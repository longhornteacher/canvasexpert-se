package platform

import (
	"os"
	"os/exec"
	"path/filepath"
)

// ClaudeDesktopPath returns the per-user Claude Desktop launcher. Both observed
// Windows installs are checked: the older per-user installer, and the Store
// (MSIX) package, which is started through its app execution alias.
func ClaudeDesktopPath() (string, bool) {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		return "", false
	}
	for _, p := range []string{
		filepath.Join(base, "AnthropicClaude", "claude.exe"),
		filepath.Join(base, "Microsoft", "WindowsApps", "claude-desktop.exe"),
	} {
		// An execution alias is a reparse point; Lstat reports it without resolving it.
		if st, err := os.Lstat(p); err == nil && !st.IsDir() {
			return p, true
		}
	}
	return "", false
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
