//go:build windows

package platform

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopShortcutRoundTripRepeatAndConflict(t *testing.T) {
	// Always use a fixture Desktop; never the machine's actual Desktop.
	desktop := filepath.Join(t.TempDir(), "Desktop with spaces 教師")
	if err := os.MkdirAll(desktop, 0700); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "UserInstall with spaces 教師")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "webui.cmd"), []byte("@echo off\r\n"), 0600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", "")
	for range 2 {
		path, err := createDesktopShortcutAt(root, desktop)
		if err != nil {
			t.Fatal(err)
		}
		if path != filepath.Join(desktop, "CanvasExpert WebUI.lnk") {
			t.Fatalf("wrong shortcut name: %s", path)
		}
		if err := withShellShortcut(path, true, func(link *shellLink, _ *persistFile) error {
			for _, property := range []struct {
				method     int
				name, want string
			}{
				{3, "TargetPath", filepath.Join(root, "webui.cmd")}, {8, "WorkingDirectory", root}, {10, "Arguments", ""},
			} {
				got, err := link.text(property.method, property.method == 3)
				if err != nil {
					return err
				}
				if got != property.want {
					t.Errorf("%s = %q, want %q", property.name, got, property.want)
				}
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(desktop)
	if err != nil || len(entries) != 1 {
		t.Fatalf("repeat duplicated shortcut: %v %v", entries, err)
	}
	other := filepath.Join(t.TempDir(), "Another UserInstall")
	if err := os.MkdirAll(other, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(other, "webui.cmd"), []byte("@echo off\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(desktop, "CanvasExpert WebUI.lnk")
	before, _ := os.ReadFile(path)
	if _, err := createDesktopShortcutAt(other, desktop); err == nil || !strings.Contains(err.Error(), "left unchanged") {
		t.Fatalf("conflict was not reported: %v", err)
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("conflict replaced the existing shortcut")
	}
}

func TestDesktopShortcutMissingLauncherDoesNotWrite(t *testing.T) {
	desktop := t.TempDir()
	if _, err := createDesktopShortcutAt(t.TempDir(), desktop); err == nil {
		t.Fatal("missing webui.cmd accepted")
	}
	entries, err := os.ReadDir(desktop)
	if err != nil || len(entries) != 0 {
		t.Fatalf("missing launcher wrote a shortcut: %v %v", entries, err)
	}
}
