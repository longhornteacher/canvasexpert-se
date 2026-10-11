package localui

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestHub() *Hub {
	h := &Hub{origin: "http://127.0.0.1:4444", secret: "nonce", quit: make(chan struct{})}
	h.setup = &setupHandler{origin: h.origin, secret: h.secret, persistent: true, hub: true, done: make(chan struct{})}
	return h
}

func hubPost(h *Hub, path string, form url.Values, origin string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPost, h.origin+path, strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Origin", origin)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func makeWorkspace(t *testing.T) string {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "context"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "context", "courses.md"), []byte("---\nschema: 1\ninstance: \"\"\ncourses: []\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestHubRejectsForeignHostAndServesNavigation(t *testing.T) {
	h := newTestHub()
	r := httptest.NewRequest(http.MethodGet, "http://rebind.example/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 404 {
		t.Fatal("foreign Host accepted")
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, h.origin+"/", nil))
	body := w.Body.String()
	if w.Code != 200 || !strings.Contains(body, `href="/students"`) || !strings.Contains(body, "CanvasExpert: Special Edition") || strings.Contains(body, "SE = ") || strings.Contains(body, "/pokemonify") {
		t.Fatal("home page incomplete")
	}
}

func TestHubFormsNeedOriginAndSecret(t *testing.T) {
	h := newTestHub()
	for _, path := range []string{"/settings", "/quit"} {
		if w := hubPost(h, path, url.Values{"request_secret": {"nonce"}, "path": {"x"}}, "http://attacker.example"); w.Code != 403 {
			t.Fatalf("%s cross-origin allowed", path)
		}
		if w := hubPost(h, path, url.Values{"request_secret": {"wrong"}, "path": {"x"}}, h.origin); w.Code != 403 {
			t.Fatalf("%s wrong secret allowed", path)
		}
	}
	select {
	case <-h.quit:
		t.Fatal("unauthenticated quit")
	default:
	}
}

func TestHubQuitSignalsShutdown(t *testing.T) {
	h := newTestHub()
	if w := hubPost(h, "/quit", url.Values{"request_secret": {"nonce"}}, h.origin); w.Code != 200 {
		t.Fatal("quit refused")
	}
	select {
	case <-h.quit:
	default:
		t.Fatal("quit not signalled")
	}
}

func TestHubCannotChangeInstallation(t *testing.T) {
	h := newTestHub()
	h.root = makeWorkspace(t)
	original := h.root
	for _, route := range []string{"/workspace", "/browse", "/browse-cancel"} {
		w := hubPost(h, route, url.Values{"request_secret": {"nonce"}, "path": {t.TempDir()}}, h.origin)
		if w.Code != http.StatusNotFound || h.root != original {
			t.Fatalf("%s changed installation or is still available: %d", route, w.Code)
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, h.origin+"/settings", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), original) || strings.Contains(w.Body.String(), `action="/workspace"`) || strings.Contains(w.Body.String(), `action="/browse"`) {
		t.Fatal("settings must show a read-only installation")
	}
}

func TestHubDevelopmentLaunchExplainsExplicitRoot(t *testing.T) {
	h := newTestHub()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, h.origin+"/settings", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "--workspace") {
		t.Fatal("missing explicit development launch guidance")
	}
}

func TestHubOpeningExistingWorkspaceAddsMissingSeedOnly(t *testing.T) {
	root := makeWorkspace(t)
	grading := filepath.Join(root, "context", "grading.md")
	edited := []byte("my own grading habits")
	if err := os.WriteFile(grading, edited, 0o644); err != nil {
		t.Fatal(err)
	}
	notice := addMissingSeed(root)
	if !strings.Contains(notice, "playbooks/score.md") {
		t.Fatalf("notice does not name the restored guide: %q", notice)
	}
	if _, err := os.Stat(filepath.Join(root, "playbooks", "score.md")); err != nil {
		t.Fatal("missing guide was not restored")
	}
	if got, _ := os.ReadFile(grading); string(got) != string(edited) {
		t.Fatal("edited settings file changed")
	}
	if again := addMissingSeed(root); again != "" {
		t.Fatalf("second open should add nothing, got %q", again)
	}
}

func TestPlaybookReplaceKeepsOldCopyAndRefusesSettings(t *testing.T) {
	h := newTestHub()
	h.root = makeWorkspace(t)
	addMissingSeed(h.root)
	guide := filepath.Join(h.root, "playbooks", "score.md")
	if err := os.WriteFile(guide, []byte("my edits"), 0o644); err != nil {
		t.Fatal(err)
	}
	form := url.Values{"request_secret": {"nonce"}, "file": {"playbooks/score.md"}, "confirm": {"on"}}
	if w := hubPost(h, "/playbook/replace", form, h.origin); w.Code != 303 {
		t.Fatalf("replace not accepted: %d", w.Code)
	}
	old, _ := filepath.Glob(filepath.Join(h.root, "playbook-old", "*", "playbooks", "score.md"))
	if len(old) != 1 {
		t.Fatalf("old copy not kept: %v", old)
	}
	if b, _ := os.ReadFile(old[0]); string(b) != "my edits" {
		t.Fatal("old copy changed")
	}
	if seed, _ := seedBytes("playbooks/score.md"); !strings.EqualFold(string(seed), readString(t, guide)) {
		t.Fatal("seed copy not written")
	}
	grading := filepath.Join(h.root, "context", "grading.md")
	if err := os.WriteFile(grading, []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	form.Set("file", "context/grading.md")
	hubPost(h, "/playbook/replace", form, h.origin)
	if readString(t, grading) != "mine" || !strings.Contains(h.flashErr, "can't be replaced") {
		t.Fatal("settings file was replaced or not refused")
	}
}

func readString(t *testing.T, p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestHowPageShowsPaths(t *testing.T) {
	h := newTestHub()
	h.programDir, h.root, h.version = `C:\Prog`, `C:\Work`, "0.1b"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, h.origin+"/how", nil))
	if body := w.Body.String(); w.Code != 200 || !strings.Contains(body, `C:\Prog`) || !strings.Contains(body, `C:\Work`) {
		t.Fatal("how page missing paths")
	}
}
