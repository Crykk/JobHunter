package output

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Crykk/JobHunter/internal/job"
)

func TestJSONFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "matches.json")
	in := job.ResultFile{
		Query:   "python",
		Area:    "Helsinki",
		Pages:   3,
		Kept:    2,
		Profile: "profile.txt",
		Source:  "duunitori.fi",
		Matches: []job.Match{
			{ID: "1", Title: "Backend", Company: "Acme", Location: "Helsinki", URL: "https://example.com/1", Score: 88, Why: "Python and APIs"},
		},
	}
	if err := JSONFile(path, in); err != nil {
		t.Fatal(err)
	}
	out, err := LoadJSON(path)
	if err != nil {
		t.Fatal(err)
	}
	if out.Query != "python" || len(out.Matches) != 1 || out.Matches[0].Score != 88 {
		t.Fatalf("%+v", out)
	}
}

func TestLoadJSONMissing(t *testing.T) {
	_, err := LoadJSON(filepath.Join(t.TempDir(), "no.json"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadJSONEmptyMatches(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.json")
	if err := os.WriteFile(path, []byte(`{"query":"go","matches":null}`), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := LoadJSON(path)
	if err != nil {
		t.Fatal(err)
	}
	if out.Matches == nil {
		t.Fatal("matches should be empty slice")
	}
}
