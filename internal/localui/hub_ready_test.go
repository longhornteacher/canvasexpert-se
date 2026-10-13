package localui

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"canvasexpertse/internal/hubcoord"
	"canvasexpertse/internal/platform"
)

type readyResult struct {
	URL            string `json:"url"`
	InstallRoot    string `json:"install_root"`
	Reused         bool   `json:"reused"`
	BrowserOutcome string `json:"browser_outcome"`
}

func waitReady(t *testing.T, path string) readyResult {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(path); err == nil {
			var r readyResult
			if err := json.Unmarshal(data, &r); err != nil {
				t.Fatal(err)
			}
			return r
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("hub never published readiness")
	return readyResult{}
}

func TestInstallerReadyFileNewStartAndSameInstallReuse(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	if err := platform.CredentialSelfTest(); err != nil {
		t.Skip("credential store unavailable: ", err)
	}
	opened := 0
	oldOpen := openBrowser
	openBrowser = func(string) error { opened++; return nil }
	defer func() { openBrowser = oldOpen }()
	root := t.TempDir()
	canonical := hubcoord.CanonicalRoot(root)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	first := filepath.Join(t.TempDir(), "first.json")
	go func() {
		done <- RunHubWithOptions(ctx, root, "test", nil, HubOptions{SetupFirst: true, InstallClient: "work", ReadyFile: first})
	}()
	r1 := waitReady(t, first)
	if r1.Reused || r1.URL == "" || r1.InstallRoot != canonical || r1.BrowserOutcome != "launch_requested" {
		t.Fatalf("new hub readiness wrong: %+v (want root %s)", r1, canonical)
	}
	second := filepath.Join(t.TempDir(), "second.json")
	if err := RunHubWithOptions(context.Background(), root, "test", nil, HubOptions{SetupFirst: true, ReadyFile: second}); err != nil {
		t.Fatal(err)
	}
	r2 := waitReady(t, second)
	if !r2.Reused || r2.InstallRoot != canonical || r2.URL == "" || r2.BrowserOutcome != "launch_requested" {
		t.Fatalf("reused hub readiness wrong: %+v", r2)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
