package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func useSyncSearchRoots(t *testing.T, roots ...string) {
	t.Helper()
	old := syncSearchRoots
	syncSearchRoots = func() []string { return roots }
	resetSyncDiscovery()
	t.Cleanup(func() { syncSearchRoots = old; resetSyncDiscovery() })
}

func TestSecondPCFindsSharedFolderAndStopsAfterConnecting(t *testing.T) {
	cloud := t.TempDir()
	shared := filepath.Join(cloud, "School", "CanvasExpert")
	if err := os.MkdirAll(shared, 0755); err != nil {
		t.Fatal(err)
	}
	first := makeRulesInstall(t)
	if _, err := SetRulesLocation(first, shared, nil, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(shared, syncMarkerFile)); err != nil {
		t.Fatal("binding did not mark the shared folder", err)
	}
	// A look-alike UserInstall in the same drive is not offered.
	decoy := filepath.Join(cloud, "Other")
	for _, p := range []string{"context", "engine"} {
		if err := os.MkdirAll(filepath.Join(decoy, p), 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(decoy, "context", "courses.md"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	useSyncSearchRoots(t, cloud)

	second := makeRulesInstall(t)
	found := DiscoverUserSync(second)
	if len(found) != 1 || !samePath(found[0], shared) {
		t.Fatalf("found = %v, want %s", found, shared)
	}
	if note := UnconnectedSyncNote(found); !strings.Contains(note, shared) || !strings.Contains(note, "parent contacts") {
		t.Fatalf("note = %q", note)
	}
	if got := DiscoverUserSync(first); len(got) != 0 {
		t.Fatalf("connected PC still offered %v", got)
	}
	if _, err := SetRulesLocation(second, found[0], nil, ""); err != nil {
		t.Fatal(err)
	}
	if got := DiscoverUserSync(second); len(got) != 0 {
		t.Fatalf("offered again after connecting: %v", got)
	}
}

func TestSharedFolderFromBeforeTheMarkerIsRecognized(t *testing.T) {
	cloud := t.TempDir()
	shared := filepath.Join(cloud, "CE")
	if err := os.MkdirAll(filepath.Join(shared, "context"), 0755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"context/courses.md": "x", "AGENTS.md": "x"} {
		if err := os.WriteFile(filepath.Join(shared, filepath.FromSlash(name)), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	useSyncSearchRoots(t, cloud)
	if found := DiscoverUserSync(makeRulesInstall(t)); len(found) != 1 || !samePath(found[0], shared) {
		t.Fatalf("found = %v", found)
	}
}
