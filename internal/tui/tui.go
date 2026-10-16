package tui

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Crykk/JobHunter/internal/job"
	"github.com/Crykk/JobHunter/internal/openrouter"
	"github.com/Crykk/JobHunter/internal/output"
	"github.com/Crykk/JobHunter/internal/pipeline"
	"github.com/Crykk/JobHunter/internal/profile"
	"github.com/Crykk/JobHunter/internal/settings"
)

type screen int

const (
	screenHome screen = iota
	screenRunning
	screenResults
	screenSettings
	screenAPIKey
)

const (
	defaultPages     = 3
	defaultThreshold = 0.55
	maxLogs          = 8
)

type focusField int

const (
	focusQuery focusField = iota
	focusArea
)

type menuID string

const (
	menuSearch   menuID = "search"
	menuLast     menuID = "last"
	menuSettings menuID = "settings"
	menuQuit     menuID = "quit"
)

var homeMenu = []struct {
	id   menuID
	name string
	hint string
}{
	{menuSearch, "New search", "ctrl+s"},
	{menuLast, "Open last results", "ctrl+r"},
	{menuSettings, "Settings", "ctrl+,"},
	{menuQuit, "Quit", "ctrl+q"},
}

type logMsg string

type doneMsg struct {
	res *pipeline.Result
	err error
}

type openDoneMsg struct {
	err error
}

type model struct {
	version, commit, date string

	width, height int
	screen        screen
	focus         focusField
	menuCursor    int

	query           textinput.Model
	area            textinput.Model
	apiKey          textinput.Model
	apiFromSettings bool

	errMsg   string
	progress string
	logs     []string

	matches []job.Match
	cursor  int
	offset  int
	why     string
	queryS  string
	areaS   string

	settings settings.File

	cancel context.CancelFunc
	logCh  <-chan string
	doneCh <-chan doneMsg
}

// Run starts the JobHunter TUI in the current terminal (alt screen, mouse).
func Run(version, commit, date string) error {
	p := tea.NewProgram(
		newModel(version, commit, date),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	_, err := p.Run()
	return err
}

func newModel(version, commit, date string) model {
	q := textinput.New()
	q.Placeholder = "query (e.g. python)"
	q.Prompt = "› "
	q.CharLimit = 120
	q.Width = 36

	a := textinput.New()
	a.Placeholder = "location (optional, e.g. Vantaa)"
	a.Prompt = "area "
	a.CharLimit = 80
	a.Width = 36

	k := textinput.New()
	k.Placeholder = "paste OpenRouter API key"
	k.Prompt = "key "
	k.CharLimit = 256
	k.Width = 48
	k.EchoMode = textinput.EchoPassword
	k.EchoCharacter = '•'

	cfg := settings.Load()
	_, _ = profile.Ensure()
	m := model{
		version:  version,
		commit:   commit,
		date:     date,
		width:    80,
		height:   24,
		query:    q,
		area:     a,
		apiKey:   k,
		focus:    focusQuery,
		settings: cfg,
	}
	if strings.TrimSpace(settings.ResolveAPIKey()) == "" && strings.TrimSpace(cfg.APIKey) == "" {
		m.screen = screenAPIKey
		m.query.Blur()
		m.area.Blur()
		m.apiKey.Focus()
	} else {
		m.query.Focus()
		m.area.Blur()
		m.apiKey.Blur()
	}
	return m
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.sizeInputs()
		return m, nil

	case logMsg:
		line := strings.TrimSpace(string(msg))
		if line != "" {
			m.progress = line
			m.logs = append(m.logs, line)
			if len(m.logs) > maxLogs {
				m.logs = m.logs[len(m.logs)-maxLogs:]
			}
		}
		if m.screen == screenRunning && m.logCh != nil {
			return m, waitLog(m.logCh)
		}
		return m, nil

	case doneMsg:
		m.finishSearch()
		if msg.err != nil {
			if errors.Is(msg.err, context.Canceled) {
				m.errMsg = "search cancelled"
				m.screen = screenHome
				return m.focusQuery()
			}
			m.errMsg = msg.err.Error()
			m.screen = screenHome
			return m.focusQuery()
		}
		if msg.res == nil {
			m.matches = nil
		} else {
			m.matches = msg.res.Matches
			m.queryS = msg.res.File.Query
			m.areaS = msg.res.File.Area
		}
		m.cursor = 0
		m.offset = 0
		m.setWhy()
		m.screen = screenResults
		return m, nil

	case openDoneMsg:
		if msg.err != nil {
			m.errMsg = "open url: " + msg.err.Error()
		}
		return m, nil

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m *model) finishSearch() {
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	m.logCh = nil
	m.doneCh = nil
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := msg.String()

	switch s {
	case "ctrl+c", "ctrl+q":
		if m.screen == screenRunning {
			if m.cancel != nil {
				m.cancel()
			}
			return m, nil
		}
		return m, tea.Quit
	}

	switch m.screen {
	case screenAPIKey:
		return m.handleAPIKey(msg)
	case screenSettings:
		return m.handleSettingsKey(s)
	case screenRunning:
		if s == "q" || s == "esc" {
			if m.cancel != nil {
				m.cancel()
			}
		}
		return m, nil

	case screenResults:
		switch s {
		case "esc":
			m.screen = screenHome
			m.errMsg = ""
			return m.focusQuery()
		case "q":
			return m, tea.Quit
		case "up", "k":
			m.moveCursor(-1)
			return m, nil
		case "down", "j":
			m.moveCursor(1)
			return m, nil
		case "enter", "o":
			return m, m.openSelected()
		}
		return m, nil

	default: // home
		switch s {
		case "ctrl+s":
			return m.startSearch()
		case "ctrl+,":
			return m.openSettings()
		case "ctrl+r":
			return m.loadLast()
		case "tab", "shift+tab":
			return m.toggleFocus(), nil
		case "enter":
			return m.startSearch()
		case "up":
			if m.menuCursor > 0 {
				m.menuCursor--
			}
			return m, nil
		case "down":
			if m.menuCursor < len(homeMenu)-1 {
				m.menuCursor++
			}
			return m, nil
		}
		return m.updateInputs(msg)
	}
}

func (m model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if msg.Button == tea.MouseButtonWheelUp {
		if m.screen == screenResults {
			m.moveCursor(-1)
		} else if (m.screen == screenHome || m.screen == screenSettings) && m.menuCursor > 0 {
			m.menuCursor--
		}
		return m, nil
	}
	if msg.Button == tea.MouseButtonWheelDown {
		if m.screen == screenResults {
			m.moveCursor(1)
		} else if m.screen == screenHome && m.menuCursor < len(homeMenu)-1 {
			m.menuCursor++
		}
		return m, nil
	}
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return m, nil
	}

	switch m.screen {
	case screenHome:
		if msg.X >= boxLeft {
			switch msg.Y {
			case m.homeQueryY():
				return m.focusQuery()
			case m.homeAreaY():
				return m.focusArea()
			}
		}
		if id, ok := m.menuAt(msg.X, msg.Y); ok {
			m.menuCursor = indexOfMenu(id)
			switch id {
			case menuSearch:
				return m.startSearch()
			case menuLast:
				return m.loadLast()
			case menuSettings:
				return m.openSettings()
			case menuQuit:
				return m, tea.Quit
			}
		}
	case screenSettings:
		return m.handleSettingsMouse(msg)
	case screenResults:
		if i, ok := m.resultAt(msg.Y); ok {
			m.cursor = i
			m.ensureVisible()
			m.setWhy()
			return m, m.openSelected()
		}
	}
	return m, nil
}

func (m model) startSearch() (tea.Model, tea.Cmd) {
	q := strings.TrimSpace(m.query.Value())
	if q == "" {
		m.errMsg = "type a query below, then press enter"
		m.screen = screenHome
		return m.focusQuery()
	}
	if !m.hasAPIKey() {
		return m.openAPIKey(false)
	}
	prof, err := profile.Ensure()
	if err != nil {
		m.errMsg = "profile.txt: " + err.Error()
		return m.focusQuery()
	}

	if m.cancel != nil {
		m.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.screen = screenRunning
	m.errMsg = ""
	m.progress = "starting…"
	m.logs = nil
	m.queryS = q
	m.areaS = strings.TrimSpace(m.area.Value())

	logCh := make(chan string, 64)
	doneCh := make(chan doneMsg, 1)
	m.logCh = logCh
	m.doneCh = doneCh

	opts := pipeline.Options{
		Query:       q,
		Area:        m.areaS,
		Pages:       defaultPages,
		Threshold:   defaultThreshold,
		Profile:     prof,
		Out:         output.DefaultJSON,
		FilterModel: m.settings.FilterModel,
		RankModel:   m.settings.RankModel,
		APIKey:      m.resolvedAPIKey(),
	}
	go func() {
		defer close(logCh)
		res, err := pipeline.Run(ctx, opts, &progressWriter{ch: logCh})
		doneCh <- doneMsg{res: res, err: err}
	}()
	return m, tea.Batch(waitLog(logCh), waitDone(doneCh))
}

func (m model) loadLast() (tea.Model, tea.Cmd) {
	file, err := output.LoadJSON(output.DefaultJSON)
	if err != nil {
		m.errMsg = "no last results — run a search first"
		m.screen = screenHome
		return m, nil
	}
	m.matches = file.Matches
	m.queryS = file.Query
	m.areaS = file.Area
	m.cursor = 0
	m.offset = 0
	m.errMsg = ""
	m.setWhy()
	m.screen = screenResults
	return m, nil
}

func (m model) openSelected() tea.Cmd {
	if m.cursor < 0 || m.cursor >= len(m.matches) {
		return nil
	}
	u := m.matches[m.cursor].URL
	return func() tea.Msg {
		return openDoneMsg{err: openURL(u)}
	}
}

func (m *model) moveCursor(delta int) {
	if len(m.matches) == 0 {
		return
	}
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.matches) {
		m.cursor = len(m.matches) - 1
	}
	m.ensureVisible()
	m.setWhy()
}

func (m *model) ensureVisible() {
	vis := m.resultVisible()
	if vis <= 0 {
		return
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+vis {
		m.offset = m.cursor - vis + 1
	}
}

func (m *model) setWhy() {
	if m.cursor >= 0 && m.cursor < len(m.matches) {
		m.why = m.matches[m.cursor].Why
	} else {
		m.why = ""
	}
}

func (m model) toggleFocus() model {
	if m.focus == focusQuery {
		m.focus = focusArea
		m.query.Blur()
		m.area.Focus()
	} else {
		m.focus = focusQuery
		m.area.Blur()
		m.query.Focus()
	}
	return m
}

func (m model) focusQuery() (tea.Model, tea.Cmd) {
	m.focus = focusQuery
	m.area.Blur()
	cmd := m.query.Focus()
	return m, cmd
}

func (m model) focusArea() (tea.Model, tea.Cmd) {
	m.focus = focusArea
	m.query.Blur()
	cmd := m.area.Focus()
	return m, cmd
}

func (m model) updateInputs(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	if m.focus == focusQuery {
		m.query, cmd = m.query.Update(msg)
	} else {
		m.area, cmd = m.area.Update(msg)
	}
	return m, cmd
}

func (m *model) sizeInputs() {
	box := m.boxWidth()
	qw := box - 28
	if qw < 16 {
		qw = 16
	}
	if qw > 40 {
		qw = 40
	}
	m.query.Width = qw
	m.area.Width = qw
	aw := box - 8
	if aw < 20 {
		aw = 20
	}
	if aw > 56 {
		aw = 56
	}
	m.apiKey.Width = aw
}

func (m model) hasAPIKey() bool {
	return m.resolvedAPIKey() != ""
}

func (m model) resolvedAPIKey() string {
	if k := strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY")); k != "" {
		return k
	}
	return strings.TrimSpace(m.settings.APIKey)
}

func profileOK() bool {
	_, err := os.Stat(profile.Path())
	return err == nil
}

type progressWriter struct {
	mu sync.Mutex
	ch chan string
}

func (w *progressWriter) Write(p []byte) (int, error) {
	s := strings.TrimSpace(string(p))
	if s == "" {
		return len(p), nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	select {
	case w.ch <- s:
	default:
	}
	return len(p), nil
}

var _ io.Writer = (*progressWriter)(nil)

func waitLog(ch <-chan string) tea.Cmd {
	return func() tea.Msg {
		s, ok := <-ch
		if !ok {
			return nil
		}
		return logMsg(s)
	}
}

func waitDone(ch <-chan doneMsg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return doneMsg{err: errors.New("search ended unexpectedly")}
		}
		return msg
	}
}

func indexOfMenu(id menuID) int {
	for i, it := range homeMenu {
		if it.id == id {
			return i
		}
	}
	return 0
}

func (m model) openSettings() (tea.Model, tea.Cmd) {
	m.screen = screenSettings
	m.errMsg = ""
	m.query.Blur()
	m.area.Blur()
	return m, nil
}

func (m model) closeSettings() (tea.Model, tea.Cmd) {
	if err := m.persistSettings(); err != nil {
		m.errMsg = err.Error()
	}
	m.screen = screenHome
	return m.focusQuery()
}

func (m *model) persistSettings() error {
	m.settings = settings.Normalize(m.settings)
	return settings.Save(m.settings)
}

func (m model) setFilter(id string) (tea.Model, tea.Cmd) {
	m.settings.FilterModel = settings.NormalizeFilter(id)
	m.settings.RankModel = openrouter.ArgonModel
	if err := m.persistSettings(); err != nil {
		m.errMsg = err.Error()
	} else {
		m.errMsg = ""
	}
	return m, nil
}

func (m model) handleSettingsKey(s string) (tea.Model, tea.Cmd) {
	switch s {
	case "esc", "enter":
		return m.closeSettings()
	case "left", "h", "1":
		return m.setFilter(openrouter.LunaModel)
	case "right", "l", "2":
		return m.setFilter(openrouter.JevModel)
	case "a":
		return m.openAPIKey(true)
	case "e":
		return m, openProfileEditor()
	case "f":
		return m, openProfileFolder()
	}
	return m, nil
}

func (m model) handleSettingsMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if id, ok := m.filterChipAt(msg.X, msg.Y); ok {
		return m.setFilter(id)
	}
	if m.atAPIKeyChip(msg.X, msg.Y) {
		return m.openAPIKey(true)
	}
	if side, ok := m.profileChipAt(msg.X, msg.Y); ok {
		if side == "editor" {
			return m, openProfileEditor()
		}
		return m, openProfileFolder()
	}
	return m, nil
}

func openProfileEditor() tea.Cmd {
	return func() tea.Msg {
		return openDoneMsg{err: profile.OpenEditor()}
	}
}

func openProfileFolder() tea.Cmd {
	return func() tea.Msg {
		return openDoneMsg{err: profile.OpenFolder()}
	}
}

func (m model) openAPIKey(fromSettings bool) (tea.Model, tea.Cmd) {
	m.screen = screenAPIKey
	m.apiFromSettings = fromSettings
	m.errMsg = ""
	m.query.Blur()
	m.area.Blur()
	m.apiKey.Reset()
	return m, m.apiKey.Focus()
}

func (m model) handleAPIKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		return m.saveAPIKey()
	case "esc":
		m.apiKey.Reset()
		m.apiKey.Blur()
		m.errMsg = ""
		if m.apiFromSettings {
			m.screen = screenSettings
			return m, nil
		}
		if m.hasAPIKey() {
			m.screen = screenHome
			return m.focusQuery()
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.apiKey, cmd = m.apiKey.Update(msg)
	return m, cmd
}

func (m model) saveAPIKey() (tea.Model, tea.Cmd) {
	k := strings.TrimSpace(m.apiKey.Value())
	if k == "" {
		m.errMsg = "paste your OpenRouter API key, then press enter"
		return m, nil
	}
	m.settings.APIKey = k
	if err := m.persistSettings(); err != nil {
		m.errMsg = err.Error()
		return m, nil
	}
	m.apiKey.Reset()
	m.apiKey.Blur()
	m.errMsg = ""
	if m.apiFromSettings {
		m.screen = screenSettings
		return m, nil
	}
	m.screen = screenHome
	return m.focusQuery()
}
