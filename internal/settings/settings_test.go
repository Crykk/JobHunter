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
	t.Setenv("JOBHUNTER_SETTINGS", "")
	home := t.TempDir()
	t.Setenv("JOBHUNTER_HOME", home)
	t.Chdir(t.TempDir())
	if err := Save(File{APIKey: "sk-from-file"}); err != nil {
		t.Fatal(err)
	}
	if ResolveAPIKey() != "sk-from-file" {
		t.Fatalf("%q", ResolveAPIKey())
	}
	if _, err := os.Stat(filepath.Join(home, DefaultFile)); err != nil {
		t.Fatal("expected settings in app data dir")
	}
}

func TestSaveDoesNotWipeAPIKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultFile)
	if err := SavePath(path, File{FilterModel: "luna", APIKey: "sk-keep"}); err != nil {
		t.Fatal(err)
	}
	if err := SavePath(path, File{FilterModel: "jev"}); err != nil {
		t.Fatal(err)
	}
	out := LoadPath(path)
	if out.APIKey != "sk-keep" {
		t.Fatalf("wiped key: %q", out.APIKey)
	}
	if out.FilterModel != openrouter.JevModel {
		t.Fatalf("filter %s", out.FilterModel)
	}
}

func TestLoadMigratesCwdSettings(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "")
	t.Setenv("JOBHUNTER_SETTINGS", "")
	home := t.TempDir()
	t.Setenv("JOBHUNTER_HOME", home)
	cwd := t.TempDir()
	t.Chdir(cwd)
	old := filepath.Join(cwd, DefaultFile)
	if err := os.WriteFile(old, []byte(`{"filter_model":"jev","api_key":"sk-old"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := Load()
	if got.APIKey != "sk-old" || got.FilterModel != openrouter.JevModel {
		t.Fatalf("%+v", got)
	}
	if _, err := os.Stat(filepath.Join(home, DefaultFile)); err != nil {
		t.Fatal("expected migrated app-data file")
	}
}
