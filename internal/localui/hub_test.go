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
	if w.Code != 200 || !strings.Contains(body, `href="/students"`) || !strings.Contains(body, "CanvasExpert: Special Edition") || strings.Contains(body, "SE = ") || strings.Contains(body, "/pokemonify") {
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
	release := make(chan struct{})
	finished := make(chan struct{})
	h.chooser = func(context.Context) (string, error) {
		defer close(finished)
		<-release
		return folder, nil
	}
	w := hubPost(h, "/browse", url.Values{"request_secret": {"nonce"}}, h.origin)
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/settings" {
		t.Fatalf("browse must redirect to settings, got %d", w.Code)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, h.origin+"/settings", nil))
	if !strings.Contains(w.Body.String(), `http-equiv="refresh"`) {
		t.Fatal("home did not show the waiting page while the chooser is open")
	}
	close(release)
	<-finished
	for i := 0; i < 100; i++ {
		h.mu.Lock()
		ready := h.picked != nil
		h.mu.Unlock()
		if ready {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, h.origin+"/settings", nil))
	if h.root == "" || !strings.Contains(w.Body.String(), "Workspace created") {
		t.Fatalf("chosen folder not adopted: %s", w.Body.String())
	}
}

func TestHubOpeningExistingWorkspaceAddsMissingSeedOnly(t *testing.T) {
	root := makeWorkspace(t)
	grading := filepath.Join(root, "settings", "grading.md")
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
	grading := filepath.Join(h.root, "settings", "grading.md")
	if err := os.WriteFile(grading, []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	form.Set("file", "settings/grading.md")
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
