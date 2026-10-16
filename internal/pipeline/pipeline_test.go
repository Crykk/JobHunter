package pipeline

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunRequiresQuery(t *testing.T) {
	_, err := Run(context.Background(), Options{Profile: "profile.txt"}, nil)
	if err == nil || !strings.Contains(err.Error(), "query is required") {
		t.Fatalf("got %v", err)
	}
}

func TestRunRequiresProfile(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "sk-or-test")
	_, err := Run(context.Background(), Options{
		Query:   "python",
		Profile: filepath.Join(t.TempDir(), "no-such-profile.txt"),
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "read profile") {
		t.Fatalf("got %v", err)
	}
}

func TestRunRequiresAPIKey(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "")
	dir := t.TempDir()
	t.Chdir(dir)
	profile := filepath.Join(dir, "profile.txt")
	if err := os.WriteFile(profile, []byte("Name: Test"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Run(context.Background(), Options{Query: "python", Profile: profile}, nil)
	if err == nil || !strings.Contains(err.Error(), "API key") {
		t.Fatalf("got %v", err)
	}
}

func TestRunHonorsCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Run(ctx, Options{Query: "python"}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}
