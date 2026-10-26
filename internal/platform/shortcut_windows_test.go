//go:build windows

package platform

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
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
		assertDesktopShortcutProperties(t, path, root)
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

func TestDesktopShortcutEquivalentShortAndLongPaths(t *testing.T) {
	// Always use fixtures, including the Desktop; never machine integration.
	desktop := t.TempDir()
	root := filepath.Join(t.TempDir(), "UserInstall with long folder name")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "webui.cmd"), []byte("@echo off\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	longPath, err := windows.UTF16PtrFromString(root)
	if err != nil {
		t.Fatal(err)
	}
	buffer := make([]uint16, 32768)
	n, err := windows.GetShortPathName(longPath, &buffer[0], uint32(len(buffer)))
	if err != nil {
		t.Fatal(err)
	}
	if n >= uint32(len(buffer)) {
		t.Fatal("short path exceeds buffer")
	}
	shortRoot := windows.UTF16ToString(buffer)
	if strings.EqualFold(shortRoot, root) {
		t.Skip("fixture volume does not provide an alternate short path")
	}
	for _, spelling := range []string{root, shortRoot, root} {
		path, err := createDesktopShortcutAt(spelling, desktop)
		if err != nil {
			t.Fatal(err)
		}
		assertDesktopShortcutProperties(t, path, root)
	}
}

func assertDesktopShortcutProperties(t *testing.T, path, root string) {
	t.Helper()
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
			if property.method == 10 {
				if got != property.want {
					t.Errorf("%s = %q, want %q", property.name, got, property.want)
				}
				continue
			}
			gotInfo, err := os.Stat(got)
			if err != nil {
				return err
			}
			wantInfo, err := os.Stat(property.want)
			if err != nil {
				return err
			}
			if !os.SameFile(gotInfo, wantInfo) {
				t.Errorf("%s = %q, want the same file as %q", property.name, got, property.want)
			}
		}
		buffer := make([]uint16, 32768)
		var index int32
		if err := comCall(&link.IUnknown, 16, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)), uintptr(unsafe.Pointer(&index))); err != nil {
			return err
		}
		icon := windows.UTF16ToString(buffer)
		if data, err := os.ReadFile(icon); err != nil || !bytes.Equal(data, shortcutIcon) || index != 0 {
			t.Errorf("IconLocation = %q,%d, want the embedded icon: %v", icon, index, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
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
