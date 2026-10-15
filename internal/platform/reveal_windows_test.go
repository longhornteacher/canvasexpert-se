//go:build windows

package platform

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRevealFileWithoutPATH(t *testing.T) {
	t.Setenv("PATH", "")
	t.Setenv("SystemRoot", t.TempDir()) // Resolve through Windows, not shell environment.
	want := `C:\Teacher folder\prepared extension.mcpb`
	launchErr := errors.New("synthetic launch failure")
	err := revealFile(want, func(exe string, args ...string) error {
		if !filepath.IsAbs(exe) || filepath.Base(exe) != "explorer.exe" {
			t.Fatalf("Explorer must use an absolute path: %q", exe)
		}
		if _, err := os.Stat(exe); err != nil {
			t.Fatalf("Windows Explorer not found: %v", err)
		}
		if len(args) != 1 || args[0] != "/select,"+want {
			t.Fatalf("file selection changed: %q", args)
		}
		return launchErr
	})
	if !errors.Is(err, launchErr) {
		t.Fatalf("launch failure was lost: %v", err)
	}
}
