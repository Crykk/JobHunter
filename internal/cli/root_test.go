package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootHelp(t *testing.T) {
	out, errBuf := &bytes.Buffer{}, &bytes.Buffer{}
	cmd := newRoot("1.0.0", "abc", "today", out, errBuf)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "jobhunter") {
		t.Fatalf("help %s", out.String())
	}
}

func TestSearchRequiresQuery(t *testing.T) {
	out, errBuf := &bytes.Buffer{}, &bytes.Buffer{}
	cmd := newRoot("dev", "none", "unknown", out, errBuf)
	cmd.SetArgs([]string{"search"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected missing --query error")
	}
}

func TestRootNoArgsHelpWhenNotTTY(t *testing.T) {
	out, errBuf := &bytes.Buffer{}, &bytes.Buffer{}
	cmd := newRoot("1.0.0", "abc", "today", out, errBuf)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if !strings.Contains(s, "jobhunter") {
		t.Fatalf("help %s", s)
	}
}

func TestVersion(t *testing.T) {
	out, errBuf := &bytes.Buffer{}, &bytes.Buffer{}
	cmd := newRoot("9.9.9", "deadbeef", "now", out, errBuf)
	cmd.SetArgs([]string{"version"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "9.9.9") {
		t.Fatalf("%s", out.String())
	}
}
