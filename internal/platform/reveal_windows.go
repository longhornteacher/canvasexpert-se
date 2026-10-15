//go:build windows

package platform

import (
	"path/filepath"

	"golang.org/x/sys/windows"
)

// RevealFile shows the prepared file without searching PATH for Explorer.
func RevealFile(path string) error {
	return revealFile(path, LaunchApp)
}

func revealFile(path string, launch func(string, ...string) error) error {
	dir, err := windows.GetWindowsDirectory()
	if err != nil {
		return err
	}
	return launch(filepath.Join(dir, "explorer.exe"), "/select,"+path)
}
