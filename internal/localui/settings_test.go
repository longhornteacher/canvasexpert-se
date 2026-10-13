package localui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	embedded "canvasexpertse"
	"canvasexpertse/internal/workspace"
)

func TestRulesBrowsePopulatesDraftWithoutBindingAndCancellationKeepsIt(t *testing.T) {
	h := newTestHub()
	h.root = makeWorkspace(t)
	candidate := filepath.Join(t.TempDir(), "typed rules path")
	selected := t.TempDir()
	h.pickFolderFn = func(context.Context) (string, error) { return selected, nil }
	w := hubPost(h, "/rules/browse", url.Values{"request_secret": {h.secret}, "sync_root": {candidate}}, h.origin)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("browse returned %d", w.Code)
	}
	page := hubGet(h, w.Header().Get("Location")).Body.String()
	if !strings.Contains(w.Header().Get("Location"), "section=connections") {
		t.Fatal("Browse did not return to the Connections section")
	}
	if !strings.Contains(page, `name="sync_root" value="`+selected+`"`) {
		t.Fatalf("chosen path was not populated: %s", page)
	}
	if _, err := os.Stat(filepath.Join(h.root, ".cse", "rules-location.json")); !os.IsNotExist(err) {
		t.Fatalf("Browse changed saved authority: %v", err)
	}
	h.pickFolderFn = func(context.Context) (string, error) { return "", nil }
	w = hubPost(h, "/rules/browse", url.Values{"request_secret": {h.secret}, "sync_root": {candidate}}, h.origin)
	page = hubGet(h, w.Header().Get("Location")).Body.String()
	if !strings.Contains(page, `name="sync_root" value="`+candidate+`"`) || !strings.Contains(page, "Folder selection canceled") {
		t.Fatal("cancel did not preserve the typed draft")
	}
	if _, err := os.Stat(filepath.Join(h.root, ".cse", "rules-location.json")); !os.IsNotExist(err) {
		t.Fatalf("cancel changed saved authority: %v", err)
	}
	h.pickFolderFn = func(context.Context) (string, error) { return "", os.ErrPermission }
	w = hubPost(h, "/rules/browse", url.Values{"request_secret": {h.secret}, "sync_root": {candidate}}, h.origin)
	page = hubGet(h, w.Header().Get("Location")).Body.String()
	if !strings.Contains(page, `name="sync_root" value="`+candidate+`"`) || !strings.Contains(page, "Paste the folder path instead") {
		t.Fatal("chooser failure did not preserve paste fallback")
	}
	w = hubPost(h, "/rules-location", url.Values{"request_secret": {h.secret}, "action": {"save"}, "sync_root": {selected}}, h.origin)
	loc, err := workspace.GetRulesLocation(h.root)
	if w.Code != http.StatusSeeOther || err != nil || loc.Mode != "sync" || loc.RulesRoot != selected {
		t.Fatalf("explicit save did not bind the browsed folder: status=%d location=%+v err=%v", w.Code, loc, err)
	}
}

func TestSettingsSectionsKeepFormsAndUseTabScopedDrafts(t *testing.T) {
	h := newTestHub()
	h.root = makeWorkspace(t)
	for _, section := range []struct {
		name, current, heading string
	}{
		{"connections", `aria-current="page">Connections</a>`, "Canvas connection"},
		{"preferences", `aria-current="page">Preferences</a>`, "Feedback voice"},
		{"files", `aria-current="page">Files</a>`, "Playbooks and context files"},
		{"system", `aria-current="page">System</a>`, "Quit hub"},
	} {
		page := hubGet(h, "/settings?section="+section.name).Body.String()
		if !strings.Contains(page, section.current) || !strings.Contains(page, section.heading) {
			t.Fatalf("Settings section %q is not selected or missing its content", section.name)
		}
	}
	preferences := hubGet(h, "/settings?section=preferences")
	if csp := preferences.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "script-src 'self'") {
		t.Fatalf("Settings scripts are not allowed by CSP: %s", csp)
	}
	if !strings.Contains(preferences.Body.String(), `src="/_cse/settings-drafts.js"`) || !strings.Contains(preferences.Body.String(), `data-draft-key="nonce"`) {
		t.Fatal("Settings draft retention is not attached to the current hub session")
	}
	script := hubGet(h, "/_cse/settings-drafts.js").Body.String()
	if !strings.Contains(script, "sessionStorage") || strings.Contains(script, "localStorage") || strings.Contains(script, "request_secret") {
		t.Fatal("Settings drafts must be tab-scoped and exclude request secrets")
	}
	if strings.Contains(hubGet(h, "/settings?section=connections").Body.String(), `name="voice"`) || strings.Contains(hubGet(h, "/settings?section=files").Body.String(), `name="voice"`) {
		t.Fatal("Settings sections should not expose unrelated forms")
	}
}

func TestRulesBrowseDoesNotHoldHubMutex(t *testing.T) {
	h := newTestHub()
	h.root = makeWorkspace(t)
	started, release := make(chan struct{}), make(chan struct{})
	h.pickFolderFn = func(context.Context) (string, error) {
		close(started)
		<-release
		return "", nil
	}
	browseDone := make(chan struct{})
	go func() {
		defer close(browseDone)
		hubPost(h, "/rules/browse", url.Values{"request_secret": {h.secret}}, h.origin)
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("folder chooser did not start")
	}
	navDone := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		navDone <- hubGet(h, "/settings")
	}()
	select {
	case response := <-navDone:
		if response.Code != http.StatusOK {
			t.Fatalf("settings navigation returned %d", response.Code)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("settings navigation waited for the native chooser")
	}
	close(release)
	select {
	case <-browseDone:
	case <-time.After(time.Second):
		t.Fatal("browse did not finish after chooser returned")
	}
}

func TestRulesTestChecksSavedAuthorityWithoutWritesOrDraftBinding(t *testing.T) {
	h := newTestHub()
	h.root = makeWorkspace(t)
	syncRoot := t.TempDir()
	if _, err := workspace.SetRulesLocation(h.root, syncRoot, embedded.WorkspaceTemplate, "workspace-template"); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(syncRoot, "context", "test-sentinel.txt")
	if err := os.WriteFile(sentinel, []byte("keep this"), 0o600); err != nil {
		t.Fatal(err)
	}
	bindingPath := filepath.Join(h.root, ".cse", "rules-location.json")
	bindingBefore, err := os.ReadFile(bindingPath)
	if err != nil {
		t.Fatal(err)
	}
	candidate := t.TempDir()
	w := hubPost(h, "/rules/test", url.Values{"request_secret": {h.secret}, "sync_root": {candidate}}, h.origin)
	page := hubGet(h, w.Header().Get("Location")).Body.String()
	if !strings.Contains(page, "Saved teacher-rules folder test: Available") || !strings.Contains(page, `name="sync_root" value="`+candidate+`"`) {
		t.Fatalf("test did not report saved-folder result while preserving the draft: %s", page)
	}
	bindingAfter, err := os.ReadFile(bindingPath)
	if err != nil || string(bindingAfter) != string(bindingBefore) {
		t.Fatalf("Test changed saved authority: %v", err)
	}
	if got, err := os.ReadFile(sentinel); err != nil || string(got) != "keep this" {
		t.Fatalf("Test changed Sync contents: %q %v", got, err)
	}
}

func TestUseInstallKeepsExplicitReturnSemantics(t *testing.T) {
	h := newTestHub()
	h.root = makeWorkspace(t)
	syncRoot := t.TempDir()
	if _, err := workspace.SetRulesLocation(h.root, syncRoot, embedded.WorkspaceTemplate, "workspace-template"); err != nil {
		t.Fatal(err)
	}
	w := hubPost(h, "/rules-location", url.Values{"request_secret": {h.secret}, "action": {"local"}, "sync_root": {syncRoot}}, h.origin)
	loc, err := workspace.GetRulesLocation(h.root)
	if w.Code != http.StatusSeeOther || err != nil || loc.Mode != "local" {
		t.Fatalf("Use Install failed to return to local rules: status=%d location=%+v err=%v", w.Code, loc, err)
	}
}
