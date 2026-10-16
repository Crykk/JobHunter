package tui

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// openURL opens u with the OS default handler (not a bundled browser).
func openURL(u string) error {
	u = strings.TrimSpace(u)
	if u == "" {
		return fmt.Errorf("empty url")
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", u)
	case "darwin":
		cmd = exec.Command("open", u)
	default:
		cmd = exec.Command("xdg-open", u)
	}
	return cmd.Start()
}
