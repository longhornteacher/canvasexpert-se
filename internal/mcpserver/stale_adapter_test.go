package mcpserver

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// After an update the running (old) adapter must not start the new Engine's children.
func TestStaleAdapterIsToldToRestartInsteadOfRunning(t *testing.T) {
	oldKind, oldVersion := BuildKind, Version
	BuildKind, Version = "release", "1.0.0"
	t.Cleanup(func() { BuildKind, Version = oldKind, oldVersion })
	root := t.TempDir()
	manifest := func(identity string) {
		p := filepath.Join(root, "engine", "manifest.txt")
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("version="+identity+"\nidentity="+identity+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	s := &Server{root: root}

	manifest("1.0.0")
	if err := s.staleAdapter([]string{"courses"}); err != nil {
		t.Fatalf("matching adapter must run: %v", err)
	}
	manifest("1.1.0")
	if _, err := s.command(context.Background(), "courses"); err == nil || !strings.Contains(err.Error(), "Close and reopen") {
		t.Fatalf("an older adapter must return the restart notice, got %v", err)
	}
	for _, allowed := range []string{"version", "update"} {
		if err := s.staleAdapter([]string{allowed}); err != nil {
			t.Fatalf("%s must stay allowed so the agent can explain: %v", allowed, err)
		}
	}
	// A development adapter, or a folder with no manifest, is never blocked.
	BuildKind = "development"
	if err := s.staleAdapter([]string{"courses"}); err != nil {
		t.Fatalf("development adapter blocked: %v", err)
	}
	BuildKind = "release"
	if err := (&Server{root: t.TempDir()}).staleAdapter([]string{"courses"}); err != nil {
		t.Fatalf("no manifest must not block: %v", err)
	}
}
