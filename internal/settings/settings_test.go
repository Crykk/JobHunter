package settings

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Crykk/JobHunter/internal/openrouter"
)

func TestNormalizeFilter(t *testing.T) {
	if NormalizeFilter("jev") != openrouter.JevModel {
		t.Fatal("jev")
	}
	if NormalizeFilter("luna") != openrouter.LunaModel {
		t.Fatal("luna")
	}
	if NormalizeFilter("typesafe/jev-1.14") != openrouter.JevModel {
		t.Fatal("slug")
	}
	if NormalizeFilter("unknown") != openrouter.LunaModel {
		t.Fatal("fallback")
	}
}

func TestNormalizeForcesArgon(t *testing.T) {
	f := Normalize(File{FilterModel: "jev", RankModel: "something-else"})
	if f.FilterModel != openrouter.JevModel {
		t.Fatalf("filter %s", f.FilterModel)
	}
	if f.RankModel != openrouter.ArgonModel {
		t.Fatalf("rank %s", f.RankModel)
	}
}

func TestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "jobhunter.settings.json")
	in := File{FilterModel: "jev"}
	if err := SavePath(path, in); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) == 0 {
		t.Fatal("empty file")
	}
	out := LoadPath(path)
	if out.FilterModel != openrouter.JevModel || out.RankModel != openrouter.ArgonModel {
		t.Fatalf("%+v", out)
	}
}

func TestLoadMissing(t *testing.T) {
	out := LoadPath(filepath.Join(t.TempDir(), "nope.json"))
	if out.FilterModel != openrouter.LunaModel {
		t.Fatalf("%+v", out)
	}
}

func TestAPIKeyRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "jobhunter.settings.json")
	if err := SavePath(path, File{FilterModel: "luna", APIKey: "  sk-or-test  "}); err != nil {
		t.Fatal(err)
	}
	out := LoadPath(path)
	if out.APIKey != "sk-or-test" {
		t.Fatalf("key %q", out.APIKey)
	}
}

func TestResolveAPIKeyEnvWins(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "sk-from-env")
	if ResolveAPIKey() != "sk-from-env" {
		t.Fatalf("%q", ResolveAPIKey())
	}
}

func TestResolveAPIKeyFromFile(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "")
	dir := t.TempDir()
	t.Chdir(dir)
	if err := Save(File{APIKey: "sk-from-file"}); err != nil {
		t.Fatal(err)
	}
	if ResolveAPIKey() != "sk-from-file" {
		t.Fatalf("%q", ResolveAPIKey())
	}
}
