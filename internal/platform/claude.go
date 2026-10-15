package platform

import (
	"os"
	"os/exec"
	"path/filepath"
)

// ClaudeDesktopPath returns the per-user Claude Desktop launcher when it is
// installed in its observed stable Windows location.
func ClaudeDesktopPath() (string, bool) {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		return "", false
	}
	p := filepath.Join(base, "AnthropicClaude", "claude.exe")
	if st, err := os.Stat(p); err != nil || st.IsDir() {
		return "", false
	}
	return p, true
}

// LaunchApp starts a visible local application without waiting for it. The
// arguments are passed separately so paths with spaces remain intact.
func LaunchApp(executable string, args ...string) error {
	cmd := exec.Command(executable, args...)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
