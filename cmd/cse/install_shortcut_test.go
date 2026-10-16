package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallShortcutModesAndWarningContinue(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"engine/cse.exe", "AGENTS.md", "context/courses.md"} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	codex := t.TempDir()
	t.Setenv("CODEX_HOME", codex)
	previous := ensureDesktopShortcut
	t.Cleanup(func() { ensureDesktopShortcut = previous })
	calls := 0
	ensureDesktopShortcut = func(got string) (string, error) {
		calls++
		if got != root {
			t.Fatalf("shortcut for wrong UserInstall: %s", got)
		}
		return "", errors.New("fixture Desktop unavailable")
	}
	for _, args := range [][]string{
		{"install", "--inspect"},
		{"install", "--check-only", "--no-ui"},
		{"install", "--client", "invalid", "--no-ui"},
		{"install", "--no-ui", "extra"},
	} {
		var out, progress bytes.Buffer
		run(context.Background(), append([]string{"--workspace", root}, args...), &out, &progress)
		if calls != 0 {
			t.Fatalf("%v tried to create a shortcut", args)
		}
	}
	var out, progress bytes.Buffer
	run(context.Background(), []string{"--workspace", root, "install", "--client", "work", "--no-ui"}, &out, &progress)
	if calls != 1 || !strings.Contains(progress.String(), "webui.cmd") || !strings.Contains(progress.String(), "Setup continues") {
		t.Fatalf("missing nonfatal shortcut warning: %d %s", calls, progress.String())
	}
	if !strings.Contains(out.String(), "settings-saved") || !strings.Contains(out.String(), "local-check-failed") {
		t.Fatalf("shortcut failure stopped registration/local check: %s", out.String())
	}
}

func TestDesktopShortcutSuccessOutput(t *testing.T) {
	var out, progress bytes.Buffer
	reportDesktopShortcut("fixture", func(string) (string, error) {
		return "fixture Desktop/CanvasExpert WebUI.lnk", nil
	}, &out, &progress)
	if !strings.Contains(out.String(), `"desktop_shortcut":"fixture Desktop/CanvasExpert WebUI.lnk"`) || progress.Len() != 0 {
		t.Fatalf("unexpected shortcut result: %s %s", out.String(), progress.String())
	}
}
