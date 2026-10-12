package localui

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"canvasexpertse/internal/hubcoord"
)

func TestHubDiscoveryRequiresHealthyMatchingInstall(t *testing.T) {
	root, other := t.TempDir(), t.TempDir()
	h := &Hub{root: root, quit: make(chan struct{})}
	server := httptest.NewUnstartedServer(h)
	server.Start()
	defer server.Close()
	h.origin = server.URL
	if err := hubcoord.Write(root, server.URL); err != nil {
		t.Fatal(err)
	}
	if got, ok := hubcoord.Find(root); !ok || got.URL != server.URL {
		t.Fatalf("same Install did not reuse healthy hub: %+v %v", got, ok)
	}
	if _, ok := hubcoord.Find(other); ok {
		t.Fatal("different Install reused hub")
	}
	if err := hubcoord.Write(other, server.URL); err != nil {
		t.Fatal(err)
	}
	if _, ok := hubcoord.Find(other); ok {
		t.Fatal("endpoint for another Install passed identity check")
	}
	server.Close()
	if _, ok := hubcoord.Find(root); ok {
		t.Fatal("stale discovery record treated as a live hub")
	}
	// Recovery replaces only the discovery record. It does not try to signal or
	// terminate whatever process had occupied the recorded port.
	if err := hubcoord.Write(root, "http://127.0.0.1:1"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(root, ".cse", "hub.json"))
	if err != nil || len(b) == 0 {
		t.Fatal("stale record prevented replacement", err)
	}
}

func TestHubStartCoordinationIsBounded(t *testing.T) {
	root := t.TempDir()
	unlock, err := hubcoord.Acquire(root, 50*1e6)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	if _, err := hubcoord.Acquire(root, 50*1e6); err == nil {
		t.Fatal("concurrent start did not wait/fail boundedly")
	}
}
