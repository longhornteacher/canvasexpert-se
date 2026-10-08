package localui

import (
	"context"
	"time"
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

func TestHubCreatesWorkspaceInEmptyOrNewFolder(t *testing.T) {
	for _, name := range []string{"", "new-child"} {
		h := newTestHub()
		h.programDir = t.TempDir()
		folder := filepath.Join(t.TempDir(), name)
		quoted := `"` + folder + `"` // pasted paths often carry quotes
		w := hubPost(h, "/workspace", url.Values{"request_secret": {"nonce"}, "path": {quoted}}, h.origin)
		if w.Code != 200 || h.root == "" || !strings.Contains(w.Body.String(), "Workspace created") {
			t.Fatalf("%q: not created: %d %s", name, w.Code, w.Body.String())
		}
		for _, want := range []string{"settings/courses.md", ".claude/settings.json", "cse.cmd", "AGENTS.md"} {
			if _, err := os.Stat(filepath.Join(folder, filepath.FromSlash(want))); err != nil {
				t.Fatalf("starter file missing: %s", want)
			}
		}
		if _, err := os.Stat(filepath.Join(folder, "cse.exe")); err == nil {
			t.Fatal("program copied into workspace")
		}
	}
}

func TestHubAsksBeforeUsingNonEmptyFolder(t *testing.T) {
	h := newTestHub()
	h.programDir = t.TempDir()
	folder := t.TempDir()
	keep := filepath.Join(folder, "mine.txt")
	if err := os.WriteFile(keep, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	form := url.Values{"request_secret": {"nonce"}, "path": {folder}}
	if w := hubPost(h, "/workspace", form, h.origin); w.Code != 200 || h.root != "" || !strings.Contains(w.Body.String(), "Set up a workspace here?") {
		t.Fatal("non-empty folder was not confirmed first")
	}
	if _, err := os.Stat(filepath.Join(folder, "settings")); err == nil {
		t.Fatal("scaffolded before confirmation")
	}
	form.Set("confirm", "1")
	if w := hubPost(h, "/workspace", form, h.origin); w.Code != 200 || h.root == "" {
		t.Fatal("confirmed setup failed")
	}
	if b, _ := os.ReadFile(keep); string(b) != "keep" {
		t.Fatal("existing file changed")
	}
}

func TestHubRejectsRelativeAndFilePaths(t *testing.T) {
	h := newTestHub()
	file := filepath.Join(t.TempDir(), "f.txt")
	_ = os.WriteFile(file, []byte("x"), 0o644)
	for _, p := range []string{"relative/dir", file, "  "} {
		if w := hubPost(h, "/workspace", url.Values{"request_secret": {"nonce"}, "path": {p}}, h.origin); w.Code != 400 {
			t.Fatalf("%q accepted: %d", p, w.Code)
		}
	}
}

func TestHubBrowseUsesChosenFolder(t *testing.T) {
	h := newTestHub()
	h.programDir = t.TempDir()
	folder := t.TempDir()
	done := make(chan struct{})
	h.chooser = func(context.Context) (string, error) { defer close(done); return folder, nil }
	w := hubPost(h, "/browse", url.Values{"request_secret": {"nonce"}}, h.origin)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `http-equiv="refresh"`) {
		t.Fatal("browse did not show the waiting page")
	}
	<-done
	for i := 0; i < 50 && func() bool { h.mu.Lock(); defer h.mu.Unlock(); return h.picked == nil }(); i++ {
		time.Sleep(20 * time.Millisecond)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, h.origin+"/", nil))
	if h.root == "" || !strings.Contains(w.Body.String(), "Workspace created") {
		t.Fatalf("chosen folder not adopted: %s", w.Body.String())
	}
}
