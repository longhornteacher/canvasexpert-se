package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeRootUsesBundledInstallation(t *testing.T) {
	installed := t.TempDir()
	if err := os.MkdirAll(filepath.Join(installed, "context"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installed, "context", "courses.md"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(installed, "engine", "cse.exe")
	other := t.TempDir()
	got, err := ResolveRuntimeRoot("", other, exe)
	if err != nil || got != installed {
		t.Fatalf("bundled root: %s %v", got, err)
	}
	got, err = ResolveRuntimeRoot(installed, other, exe)
	if err != nil || got != installed {
		t.Fatalf("explicit installed root: %s %v", got, err)
	}
	if _, err = ResolveRuntimeRoot(other, other, exe); err == nil || !strings.Contains(err.Error(), "installation mismatch") {
		t.Fatalf("conflict accepted: %v", err)
	}
}

func TestRuntimeRootRetainsDeveloperAndExtensionOverride(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "context"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "context", "courses.md"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, exe := range []string{filepath.Join(t.TempDir(), "dist", "cse.exe"), filepath.Join(t.TempDir(), "engine", "cse.exe")} {
		got, err := ResolveRuntimeRoot(root, t.TempDir(), exe)
		if err != nil || got != root {
			t.Fatalf("override: %s %v", got, err)
		}
	}
}
