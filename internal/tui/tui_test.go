package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Crykk/JobHunter/internal/job"
	"github.com/Crykk/JobHunter/internal/openrouter"
	"github.com/Crykk/JobHunter/internal/output"
	"github.com/Crykk/JobHunter/internal/settings"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "jobhunter-tui")
	if err != nil {
		os.Exit(1)
	}
	_ = os.Setenv("JOBHUNTER_HOME", dir)
	_ = os.Setenv("OPENROUTER_API_KEY", "sk-test-tui")
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

func TestCtrlQQuits(t *testing.T) {
	m := newModel("dev", "deadbeef", "now")
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlQ})
	if cmd == nil {
		t.Fatal("expected quit cmd")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("got %T", msg)
	}
}

func TestEnterEmptyQuery(t *testing.T) {
	m := newModel("dev", "x", "y")
	got, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm := got.(model)
	if mm.errMsg == "" {
		t.Fatal("expected query hint")
	}
	if mm.screen != screenHome {
		t.Fatalf("screen %v", mm.screen)
	}
}

func TestLoadLastMissing(t *testing.T) {
	t.Chdir(t.TempDir())
	m := newModel("dev", "x", "y")
	got, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	mm := got.(model)
	if mm.screen != screenHome {
		t.Fatalf("screen %v", mm.screen)
	}
	if mm.errMsg == "" {
		t.Fatal("expected missing last-results error")
	}
}

func TestLoadLastJSON(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := output.JSONFile(output.DefaultJSON, job.ResultFile{
		Query: "python",
		Area:  "Helsinki",
		Matches: []job.Match{
			{ID: "1", Title: "Go dev", Company: "Acme", Location: "Helsinki", URL: "https://example.com/1", Score: 90, Why: "Go fit"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	m := newModel("dev", "x", "y")
	got, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	mm := got.(model)
	if mm.screen != screenResults {
		t.Fatalf("screen %v err=%s", mm.screen, mm.errMsg)
	}
	if len(mm.matches) != 1 || mm.matches[0].Score != 90 {
		t.Fatalf("%+v", mm.matches)
	}
	if mm.why != "Go fit" {
		t.Fatalf("why %q", mm.why)
	}
}

func TestHomeInputYMatchesView(t *testing.T) {
	m := newModel("dev", "x", "y")
	m.width, m.height = 80, 24
	lines := strings.Split(m.View(), "\n")
	qy, ay := m.homeQueryY(), m.homeAreaY()
	if qy < 0 || ay >= len(lines) {
		t.Fatalf("input y query=%d area=%d lines=%d", qy, ay, len(lines))
	}
	if !strings.Contains(lines[qy], "query") {
		t.Fatalf("query y=%d was %q", qy, lines[qy])
	}
	if !strings.Contains(lines[ay], "location") && !strings.Contains(lines[ay], "area") {
		t.Fatalf("area y=%d was %q", ay, lines[ay])
	}
}

func TestClickAreaFocusesAndTypes(t *testing.T) {
	m := newModel("dev", "x", "y")
	m.width, m.height = 80, 24
	got, _ := m.Update(tea.MouseMsg{
		X:      boxLeft + 4,
		Y:      m.homeAreaY(),
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	})
	mm := got.(model)
	if mm.focus != focusArea {
		t.Fatalf("focus %v", mm.focus)
	}
	got, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Vantaa")})
	mm = got.(model)
	if mm.area.Value() != "Vantaa" {
		t.Fatalf("area %q", mm.area.Value())
	}
	if mm.query.Value() != "" {
		t.Fatalf("query leaked %q", mm.query.Value())
	}
}

func TestClickQueryKeepsFocus(t *testing.T) {
	m := newModel("dev", "x", "y")
	m.width, m.height = 80, 24
	got, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	mm := got.(model)
	if mm.focus != focusArea {
		t.Fatalf("tab should move to area, got %v", mm.focus)
	}
	got, _ = mm.Update(tea.MouseMsg{
		X:      boxLeft + 4,
		Y:      mm.homeQueryY(),
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	})
	mm = got.(model)
	if mm.focus != focusQuery {
		t.Fatalf("focus %v", mm.focus)
	}
}

func TestMenuClickQuit(t *testing.T) {
	m := newModel("dev", "x", "y")
	m.width, m.height = 80, 24
	y := m.menuStartY() + 3 // Quit
	got, cmd := m.Update(tea.MouseMsg{
		X:      boxLeft + 4,
		Y:      y,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	})
	_ = got
	if cmd == nil {
		t.Fatal("expected quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("expected QuitMsg")
	}
}

func TestMenuAt(t *testing.T) {
	m := newModel("dev", "x", "y")
	m.width, m.height = 80, 24
	id, ok := m.menuAt(boxLeft+4, m.menuStartY())
	if !ok || id != menuSearch {
		t.Fatalf("got %s %v", id, ok)
	}
	id, ok = m.menuAt(boxLeft+4, m.menuStartY()+1)
	if !ok || id != menuLast {
		t.Fatalf("got %s %v", id, ok)
	}
}

func TestResultClickOpensCmd(t *testing.T) {
	m := newModel("dev", "x", "y")
	m.width, m.height = 80, 24
	m.screen = screenResults
	m.matches = []job.Match{{ID: "1", Title: "T", URL: "https://example.com/job", Score: 70}}
	y := m.resultStartY()
	got, cmd := m.Update(tea.MouseMsg{
		X:      boxLeft + 4,
		Y:      y,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	})
	mm := got.(model)
	if mm.cursor != 0 {
		t.Fatalf("cursor %d", mm.cursor)
	}
	if cmd == nil {
		t.Fatal("expected open cmd")
	}
}

func TestEscFromResults(t *testing.T) {
	m := newModel("dev", "x", "y")
	m.screen = screenResults
	got, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if got.(model).screen != screenHome {
		t.Fatal("expected home")
	}
}

func TestOpenSettingsAndPickJev(t *testing.T) {
	t.Chdir(t.TempDir())
	m := newModel("dev", "x", "y")
	m.width, m.height = 80, 24
	y := m.menuStartY() + 2 // Settings
	got, _ := m.Update(tea.MouseMsg{
		X:      boxLeft + 4,
		Y:      y,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	})
	mm := got.(model)
	if mm.screen != screenSettings {
		t.Fatalf("screen %v", mm.screen)
	}
	chipY := mm.settingsChipY()
	got, _ = mm.Update(tea.MouseMsg{
		X:      boxLeft + 1 + (mm.boxWidth()-2)/2 + 2,
		Y:      chipY,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	})
	mm = got.(model)
	if settings.NormalizeFilter(mm.settings.FilterModel) != openrouter.JevModel {
		t.Fatalf("filter %s", mm.settings.FilterModel)
	}
	if mm.settings.RankModel != openrouter.ArgonModel {
		t.Fatalf("rank %s", mm.settings.RankModel)
	}
	got, _ = mm.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if got.(model).screen != screenHome {
		t.Fatal("expected home")
	}
}

func TestAPIKeyPromptWhenMissing(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "")
	t.Chdir(t.TempDir())
	m := newModel("dev", "x", "y")
	if m.screen != screenAPIKey {
		t.Fatalf("screen %v", m.screen)
	}
	got, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm := got.(model)
	if mm.errMsg == "" {
		t.Fatal("expected paste hint")
	}
	if mm.screen != screenAPIKey {
		t.Fatalf("screen %v", mm.screen)
	}
}

func TestPasteAPIKeySaves(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "")
	t.Chdir(t.TempDir())
	m := newModel("dev", "x", "y")
	m.apiKey.SetValue("sk-or-pasted")
	got, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm := got.(model)
	if mm.screen != screenHome {
		t.Fatalf("screen %v err=%s", mm.screen, mm.errMsg)
	}
	if mm.settings.APIKey != "sk-or-pasted" {
		t.Fatal("expected key in memory")
	}
	if settings.Load().APIKey != "sk-or-pasted" {
		t.Fatal("expected key in settings file")
	}
}

func TestProfileChipHit(t *testing.T) {
	m := newModel("dev", "x", "y")
	m.width, m.height = 80, 24
	m.screen = screenSettings
	y := m.profileChipY()
	side, ok := m.profileChipAt(boxLeft+2, y)
	if !ok || side != "editor" {
		t.Fatalf("editor %s %v", side, ok)
	}
	side, ok = m.profileChipAt(boxLeft+1+(m.boxWidth()-2)/2+2, y)
	if !ok || side != "folder" {
		t.Fatalf("folder %s %v", side, ok)
	}
}

func TestOpenURLEmpty(t *testing.T) {
	if err := openURL(""); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadJSONUsedByLast(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "matches.json")
	if err := os.WriteFile(path, []byte(`{"query":"x","matches":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := output.LoadJSON(path)
	if err != nil || file.Query != "x" {
		t.Fatalf("%v %+v", err, file)
	}
}
