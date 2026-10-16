package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/Crykk/JobHunter/internal/job"
	"github.com/Crykk/JobHunter/internal/openrouter"
	"github.com/Crykk/JobHunter/internal/profile"
	"github.com/Crykk/JobHunter/internal/settings"
)

const padTop = 1
const boxLeft = 2

var (
	colTitle  = lipgloss.Color("#E6C07B")
	colDim    = lipgloss.Color("#8A8A8A")
	colBorder = lipgloss.Color("#5A5A5A")
	colOK     = lipgloss.Color("#9ECE6A")
	colBad    = lipgloss.Color("#F7768E")
	colSelect = lipgloss.Color("#7AA2F7")
	colFg     = lipgloss.Color("#C0CAF5")
	colHint   = lipgloss.Color("#565F89")
)

func (m model) boxWidth() int {
	w := m.width - boxLeft*2
	if w > 92 {
		w = 92
	}
	if w < 40 {
		w = max(20, m.width-2)
	}
	return w
}

func (m model) boxHeight() int {
	h := m.height - 8
	if h < 12 {
		h = max(8, m.height-5)
	}
	if h > 28 {
		h = 28
	}
	return h
}

func (m model) menuStartY() int {
	// y=0 pad, y=1 top border, y=2 title, y=3 subtitle, y=4 blank, y=5 first item
	return padTop + 4
}

// homeInputsStartY is the query row under the box (status, optional error, then a blank).
func (m model) homeInputsStartY() int {
	y := padTop + m.boxHeight() + 2
	if m.errMsg != "" {
		y++
	}
	return y + 1
}

func (m model) homeQueryY() int { return m.homeInputsStartY() }
func (m model) homeAreaY() int  { return m.homeInputsStartY() + 1 }

func (m model) resultStartY() int {
	// skip Results title, count, blank — same offset as menuStartY content block
	return m.menuStartY() + 3
}

func (m model) resultVisible() int {
	inner := m.boxHeight() - 2
	// title, subtitle, blank
	vis := inner - 3
	if vis < 1 {
		return 1
	}
	return vis
}

func (m model) menuAt(x, y int) (menuID, bool) {
	start := m.menuStartY()
	if x < boxLeft+1 || x >= boxLeft+m.boxWidth()-1 {
		return "", false
	}
	i := y - start
	if i < 0 || i >= len(homeMenu) {
		return "", false
	}
	return homeMenu[i].id, true
}

func (m model) resultAt(y int) (int, bool) {
	start := m.resultStartY()
	i := y - start
	if i < 0 || i >= m.resultVisible() {
		return 0, false
	}
	idx := m.offset + i
	if idx < 0 || idx >= len(m.matches) {
		return 0, false
	}
	return idx, true
}

func (m model) View() string {
	if m.width == 0 {
		return "loading…"
	}
	var b strings.Builder
	for i := 0; i < padTop; i++ {
		b.WriteByte('\n')
	}

	innerW := m.boxWidth() - 2
	var inner []string
	inner = append(inner, m.titleLine(innerW))
	inner = append(inner, m.subtitleLine(innerW))
	inner = append(inner, "")
	switch m.screen {
	case screenRunning:
		inner = append(inner, m.runningLines(innerW)...)
	case screenResults:
		inner = append(inner, m.resultLines(innerW)...)
	case screenSettings:
		inner = append(inner, m.settingsLines(innerW)...)
	case screenAPIKey:
		inner = append(inner, m.apiKeyLines(innerW)...)
	default:
		inner = append(inner, m.menuLines(innerW)...)
	}

	boxH := m.boxHeight()
	for len(inner) < boxH-2 {
		inner = append(inner, "")
	}
	if len(inner) > boxH-2 {
		inner = inner[:boxH-2]
	}

	border := lipgloss.NewStyle().Foreground(colBorder)
	top := border.Render("┌" + strings.Repeat("─", innerW) + "┐")
	bot := border.Render("└" + strings.Repeat("─", innerW) + "┘")
	side := border.Render("│")
	pad := strings.Repeat(" ", boxLeft)
	b.WriteString(pad + top + "\n")
	for _, line := range inner {
		b.WriteString(pad + side + padLine(line, innerW) + side + "\n")
	}
	b.WriteString(pad + bot + "\n")

	b.WriteString("\n")
	b.WriteString(pad + m.statusLine() + "\n")
	if m.screen == screenResults && m.why != "" {
		b.WriteString(pad + lipgloss.NewStyle().Foreground(colDim).Render(clipRunes("why: "+m.why, m.boxWidth())) + "\n")
	}
	if m.errMsg != "" {
		b.WriteString(pad + lipgloss.NewStyle().Foreground(colBad).Render(clipRunes(m.errMsg, m.boxWidth())) + "\n")
	}

	if m.screen == screenAPIKey {
		b.WriteString("\n")
		b.WriteString(pad + m.apiKey.View() + "\n")
		b.WriteString(pad + lipgloss.NewStyle().Foreground(colHint).Render("paste key  enter save  ctrl+q quit") + "\n")
	} else if m.screen == screenSettings {
		b.WriteString(pad + lipgloss.NewStyle().Foreground(colHint).Render("←/→ models  a api key  e editor  f folder  esc back") + "\n")
	} else if m.screen != screenResults && m.screen != screenRunning {
		b.WriteString("\n")
		b.WriteString(pad + m.query.View() + "\n")
		b.WriteString(pad + m.area.View() + "\n")
		b.WriteString(pad + lipgloss.NewStyle().Foreground(colHint).Render("enter search  click or tab location  click menu  ctrl+q quit") + "\n")
	} else if m.screen == screenResults {
		b.WriteString(pad + lipgloss.NewStyle().Foreground(colHint).Render("↑↓ select  enter/click open  esc home  ctrl+q quit") + "\n")
	} else {
		b.WriteString(pad + lipgloss.NewStyle().Foreground(colHint).Render("q / esc / ctrl+c cancel") + "\n")
	}
	return b.String()
}

func (m model) titleLine(width int) string {
	name := "JobHunter"
	if m.screen == screenSettings {
		name = "Settings"
	}
	if m.screen == screenAPIKey {
		name = "OpenRouter API key"
	}
	left := lipgloss.NewStyle().Bold(true).Foreground(colTitle).Render(name)
	right := lipgloss.NewStyle().Foreground(colDim).Render(m.version)
	return spread(left, right, width)
}

func (m model) subtitleLine(width int) string {
	s := "Duunitori → " + settings.FilterLabel(m.settings.FilterModel) + " → " + settings.RankLabel()
	if m.screen == screenSettings {
		s = "Models, API key, and profile.txt"
	}
	if m.screen == screenAPIKey {
		s = "Paste a key from openrouter.ai — stored only on this machine"
	}
	return clipRunes(lipgloss.NewStyle().Foreground(colFg).Render(s), width)
}

func (m model) settingsChipY() int {
	return m.settingsLineY(1)
}

func (m model) filterChipAt(x, y int) (string, bool) {
	if y != m.settingsChipY() {
		return "", false
	}
	innerX := x - (boxLeft + 1)
	if innerX < 0 {
		return "", false
	}
	innerW := m.boxWidth() - 2
	if innerX >= innerW {
		return "", false
	}
	if innerX < innerW/2 {
		return openrouter.LunaModel, true
	}
	return openrouter.JevModel, true
}

func (m model) settingsLines(width int) []string {
	onLuna := settings.NormalizeFilter(m.settings.FilterModel) == openrouter.LunaModel
	label := lipgloss.NewStyle().Foreground(colDim)
	luna := chip("Luna 6 Decisions", onLuna)
	jev := chip("Jev", !onLuna)
	argon := chip("Gemini 4 Argon", true)
	locked := lipgloss.NewStyle().Foreground(colHint).Render("only this model for now")
	keyStatus := lipgloss.NewStyle().Foreground(colBad).Render("not set")
	if m.hasAPIKey() {
		keyStatus = lipgloss.NewStyle().Foreground(colOK).Render("saved")
	}
	paste := chip("Paste API key", false)
	openEd := chip("Open in editor", false)
	openDir := chip("Open folder", false)
	profPath := clipRunes(profile.Path(), width)
	return []string{
		label.Render("First pass — list cards"),
		spread(luna, jev, width),
		label.Render("Second pass — full ads"),
		argon,
		locked,
		label.Render("OpenRouter API key  ") + keyStatus,
		paste,
		label.Render("Profile"),
		lipgloss.NewStyle().Foreground(colFg).Render(profPath),
		spread(openEd, openDir, width),
	}
}

func (m model) apiKeyChipY() int {
	return m.settingsLineY(6)
}

func (m model) profileChipY() int {
	return m.settingsLineY(9)
}

func (m model) settingsLineY(i int) int {
	// title, subtitle, blank, then settingsLines[i]
	return padTop + 1 + 2 + i
}

func (m model) atAPIKeyChip(x, y int) bool {
	if y != m.apiKeyChipY() {
		return false
	}
	innerX := x - (boxLeft + 1)
	return innerX >= 0 && innerX < m.boxWidth()-2
}

func (m model) profileChipAt(x, y int) (string, bool) {
	if y != m.profileChipY() {
		return "", false
	}
	innerX := x - (boxLeft + 1)
	if innerX < 0 {
		return "", false
	}
	innerW := m.boxWidth() - 2
	if innerX >= innerW {
		return "", false
	}
	if innerX < innerW/2 {
		return "editor", true
	}
	return "folder", true
}

func (m model) apiKeyLines(width int) []string {
	hint := lipgloss.NewStyle().Foreground(colDim)
	return []string{
		hint.Render("The key is hidden as you paste. It is not shown again."),
		hint.Render("Get one at https://openrouter.ai/keys"),
	}
}

func chip(label string, on bool) string {
	st := lipgloss.NewStyle().Padding(0, 1)
	if on {
		return st.Foreground(lipgloss.Color("#1A1B26")).Background(colSelect).Bold(true).Render(label)
	}
	return st.Foreground(colFg).Background(lipgloss.Color("#1F2335")).Render(label)
}

func (m model) menuLines(width int) []string {
	var lines []string
	nameSt := lipgloss.NewStyle().Foreground(colFg)
	selSt := lipgloss.NewStyle().Foreground(colSelect).Bold(true)
	hintSt := lipgloss.NewStyle().Foreground(colHint)
	for i, it := range homeMenu {
		label := it.name
		if i == m.menuCursor {
			label = selSt.Render("▸ " + it.name)
		} else {
			label = nameSt.Render("  " + it.name)
		}
		hint := hintSt.Render(it.hint)
		lines = append(lines, spread(label, hint, width))
	}
	return lines
}

func (m model) runningLines(width int) []string {
	head := lipgloss.NewStyle().Foreground(colTitle).Render("Searching…")
	q := fmt.Sprintf("%s", m.queryS)
	if m.areaS != "" {
		q += " · " + m.areaS
	}
	lines := []string{
		head,
		lipgloss.NewStyle().Foreground(colDim).Render(clipRunes(q, width)),
		"",
	}
	st := lipgloss.NewStyle().Foreground(colFg)
	for _, l := range m.logs {
		lines = append(lines, st.Render(clipRunes(l, width)))
	}
	if m.progress != "" && (len(m.logs) == 0 || m.logs[len(m.logs)-1] != m.progress) {
		lines = append(lines, st.Render(clipRunes(m.progress, width)))
	}
	return lines
}

func (m model) resultLines(width int) []string {
	head := "Results"
	if m.queryS != "" {
		head += " · " + m.queryS
		if m.areaS != "" {
			head += " (" + m.areaS + ")"
		}
	}
	lines := []string{
		lipgloss.NewStyle().Foreground(colTitle).Render(clipRunes(head, width)),
		lipgloss.NewStyle().Foreground(colDim).Render(fmt.Sprintf("%d matches", len(m.matches))),
		"",
	}
	if len(m.matches) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(colDim).Render("No matching jobs."))
		return lines
	}
	vis := m.resultVisible()
	end := m.offset + vis
	if end > len(m.matches) {
		end = len(m.matches)
	}
	for i := m.offset; i < end; i++ {
		lines = append(lines, formatMatch(m.matches[i], i == m.cursor, width))
	}
	return lines
}

func formatMatch(mt job.Match, selected bool, width int) string {
	row := fmt.Sprintf("%3d  %s  %s  %s", mt.Score, mt.Title, mt.Company, mt.Location)
	row = clipRunes(row, width)
	if selected {
		return lipgloss.NewStyle().Foreground(colSelect).Bold(true).Render("▸ " + strings.TrimPrefix(row, " "))
	}
	return lipgloss.NewStyle().Foreground(colFg).Render("  " + row)
}

func (m model) statusLine() string {
	key := lipgloss.NewStyle().Foreground(colBad).Render("API key missing")
	if m.hasAPIKey() {
		key = lipgloss.NewStyle().Foreground(colOK).Render("API key ok")
	}
	prof := lipgloss.NewStyle().Foreground(colBad).Render("profile.txt missing")
	if _, err := os.Stat(profile.Path()); err == nil {
		prof = lipgloss.NewStyle().Foreground(colOK).Render("profile.txt ok")
	}
	return key + lipgloss.NewStyle().Foreground(colHint).Render("    ") + prof
}

func spread(left, right string, width int) string {
	lw := lipgloss.Width(left)
	rw := lipgloss.Width(right)
	space := width - lw - rw
	if space < 1 {
		return clipRunes(left+" "+right, width)
	}
	return left + strings.Repeat(" ", space) + right
}

func padLine(s string, width int) string {
	w := lipgloss.Width(s)
	if w > width {
		return clipRunes(s, width)
	}
	return s + strings.Repeat(" ", width-w)
}

func clipRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= n {
		return s
	}
	var b strings.Builder
	w := 0
	for _, r := range s {
		rw := 1
		if r == '\n' {
			break
		}
		if w+rw > n-1 {
			break
		}
		b.WriteRune(r)
		w += rw
	}
	return b.String() + "…"
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
