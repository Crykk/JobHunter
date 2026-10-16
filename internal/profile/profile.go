package profile

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

//go:embed example.txt
var example []byte

const FileName = "profile.txt"

// Dir is the app data folder (Windows: %LOCALAPPDATA%\jobhunter).
func Dir() string {
	if d := strings.TrimSpace(os.Getenv("JOBHUNTER_HOME")); d != "" {
		return d
	}
	if runtime.GOOS == "windows" {
		if local := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); local != "" {
			return filepath.Join(local, "jobhunter")
		}
	}
	if xdg := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); xdg != "" {
		return filepath.Join(xdg, "jobhunter")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "jobhunter"
	}
	return filepath.Join(home, ".local", "share", "jobhunter")
}

// Path is the profile file to edit. ./profile.txt wins if it already exists.
func Path() string {
	if p := strings.TrimSpace(os.Getenv("JOBHUNTER_PROFILE")); p != "" {
		return p
	}
	cwd, err := os.Getwd()
	if err == nil {
		local := filepath.Join(cwd, FileName)
		if _, err := os.Stat(local); err == nil {
			return local
		}
	}
	return filepath.Join(Dir(), FileName)
}

// Ensure creates profile.txt from the bundled example if it is missing.
func Ensure() (string, error) {
	p := Path()
	if _, err := os.Stat(p); err == nil {
		return p, nil
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(p, example, 0o644); err != nil {
		return "", err
	}
	return p, nil
}

// OpenEditor opens the profile in the OS default app (Notepad on typical Windows).
func OpenEditor() error {
	p, err := Ensure()
	if err != nil {
		return err
	}
	return openFile(p)
}

// OpenFolder opens the folder that contains the profile.
func OpenFolder() error {
	p, err := Ensure()
	if err != nil {
		return err
	}
	return openDir(filepath.Dir(p), p)
}

func openFile(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	return cmd.Start()
}

func openDir(dir, selectFile string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", "/select,", selectFile)
	case "darwin":
		cmd = exec.Command("open", "-R", selectFile)
	default:
		cmd = exec.Command("xdg-open", dir)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open folder: %w", err)
	}
	return nil
}
