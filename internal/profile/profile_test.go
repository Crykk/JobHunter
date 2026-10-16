package profile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureCreatesFromExample(t *testing.T) {
	home := t.TempDir()
	t.Setenv("JOBHUNTER_HOME", home)
	t.Setenv("JOBHUNTER_PROFILE", "")
	t.Chdir(t.TempDir()) // no local profile.txt
	p, err := Ensure()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, FileName)
	if p != want {
		t.Fatalf("path %s want %s", p, want)
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "Target roles:") {
		t.Fatalf("example contents missing: %s", raw)
	}
	// second call must not overwrite edits
	if err := os.WriteFile(p, []byte("edited"), 0o644); err != nil {
		t.Fatal(err)
	}
	p2, err := Ensure()
	if err != nil || p2 != p {
		t.Fatalf("%s %v", p2, err)
	}
	got, _ := os.ReadFile(p)
	if string(got) != "edited" {
		t.Fatalf("overwrote profile: %s", got)
	}
}

func TestExampleMatchesRepo(t *testing.T) {
	root, err := os.ReadFile(filepath.Join("..", "..", "profile.example.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(example) != string(root) {
		t.Fatal("internal/profile/example.txt must match profile.example.txt")
	}
}

func TestPathPrefersCwdFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("JOBHUNTER_HOME", filepath.Join(t.TempDir(), "app"))
	t.Setenv("JOBHUNTER_PROFILE", "")
	local := filepath.Join(dir, FileName)
	if err := os.WriteFile(local, []byte("local"), 0o644); err != nil {
		t.Fatal(err)
	}
	if Path() != local {
		t.Fatalf("got %s", Path())
	}
}
