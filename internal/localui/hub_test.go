package localui

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"canvasexpertse/internal/workspace"
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
	if err := os.MkdirAll(filepath.Join(root, "settings"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "settings", "courses.md"), []byte("---\nschema: 1\ninstance: \"\"\ncourses: []\n---\n"), 0o644); err != nil {
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
	if w.Code != 200 || !strings.Contains(body, `href="/students"`) || !strings.Contains(body, "SE = ") {
		t.Fatal("home page incomplete")
	}
}

func TestHubFormsNeedOriginAndSecret(t *testing.T) {
	h := newTestHub()
	for _, path := range []string{"/workspace", "/quit"} {
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

func TestHubStudentsNeedWorkspaceThenStayOpenAfterSave(t *testing.T) {
	h := newTestHub()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, h.origin+"/students", nil))
	if w.Code != 400 {
		t.Fatal("students page without workspace")
	}
	root := makeWorkspace(t)
	if w = hubPost(h, "/workspace", url.Values{"request_secret": {"nonce"}, "path": {root}}, h.origin); w.Code != 200 || h.root == "" {
		t.Fatalf("workspace not set: %d %s", w.Code, w.Body.String())
	}
	s, _ := workspace.OpenStudents(root)
	a, _ := s.Upsert("https://school.example", 1, "Ana", "Ana", "", "")
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	form := url.Values{"request_secret": {"nonce"}, "alias_" + a.Ref: {"Renamed"}, "monitored_" + a.Ref: {"on"}}
	for i := 0; i < 2; i++ { // the editor must accept repeated saves
		if w = hubPost(h, "/students", form, h.origin); w.Code != 200 || !strings.Contains(w.Body.String(), "Saved.") {
			t.Fatalf("save %d failed: %d", i, w.Code)
		}
	}
	if again, _ := workspace.OpenStudents(root); again.Rows[0].Alias != "Renamed" || !again.Rows[0].Monitored {
		t.Fatal("edit not persisted")
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

func TestHubRefusesWorkspaceOverlappingProgram(t *testing.T) {
	h := newTestHub()
	root := makeWorkspace(t)
	h.programDir = filepath.Join(root, "cse")
	if w := hubPost(h, "/workspace", url.Values{"request_secret": {"nonce"}, "path": {root}}, h.origin); w.Code != 400 || h.root != "" {
		t.Fatalf("program inside workspace accepted: %d", w.Code)
	}
	h.programDir = t.TempDir()
	if w := hubPost(h, "/workspace", url.Values{"request_secret": {"nonce"}, "path": {root}}, h.origin); w.Code != 200 || h.root == "" {
		t.Fatalf("separate folders rejected: %d", w.Code)
	}
}
