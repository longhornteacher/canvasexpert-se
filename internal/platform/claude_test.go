package platform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClaudeDesktopPathFindsStableLauncher(t *testing.T) {
	base := t.TempDir()
	t.Setenv("LOCALAPPDATA", base)
	if _, ok := ClaudeDesktopPath(); ok {
		t.Fatal("found launcher that does not exist")
	}
	dir := filepath.Join(base, "AnthropicClaude")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "claude.exe")
	if err := os.WriteFile(want, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, ok := ClaudeDesktopPath(); !ok || got != want {
		t.Fatalf("got %q %v", got, ok)
	}
}

func TestClaudeDesktopPathFindsStoreAlias(t *testing.T) {
	base := t.TempDir()
	t.Setenv("LOCALAPPDATA", base)
	dir := filepath.Join(base, "Microsoft", "WindowsApps")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "claude-desktop.exe")
	if err := os.WriteFile(want, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, ok := ClaudeDesktopPath(); !ok || got != want {
		t.Fatalf("got %q %v", got, ok)
	}
}
