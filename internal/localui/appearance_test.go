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

	"canvasexpertse/internal/platform"
	"canvasexpertse/internal/workflow"
	"canvasexpertse/internal/workspace"
)

func TestAppearanceOptionalDocumentAndSharedAuthority(t *testing.T) {
	a, b, shared := makeWorkspace(t), makeWorkspace(t), t.TempDir()
	doc, _, err := readAppearance(a)
	if err != nil || doc.Mode != "" {
		t.Fatal(doc, err)
	}
	if _, err := os.Stat(filepath.Join(a, appearancePath)); !os.IsNotExist(err) {
		t.Fatal("opening created appearance document", err)
	}
	for _, mode := range []string{"light", "dark"} {
		if err := saveAppearance(a, mode); err != nil {
			t.Fatal(err)
		}
		got, _, err := readAppearance(a)
		if err != nil || got.Mode != mode {
			t.Fatal(got, err)
		}
	}
	if err := workspace.WriteTeacherDocument(a, appearancePath, appearanceDocument{Schema: 1, Mode: "dark"}, "Teacher annotation\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.SetRulesLocation(a, shared, nil, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.SetRulesLocation(b, shared, nil, ""); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(shared, "context/courses.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err := saveAppearance(b, "light"); err != nil {
		t.Fatal(err)
	}
	got, body, err := readAppearance(a)
	if err != nil || got.Mode != "light" || body != "Teacher annotation\n" {
		t.Fatal(got, body, err)
	}
	after, _ := os.ReadFile(filepath.Join(shared, "context/courses.md"))
	if string(before) != string(after) {
		t.Fatal("appearance changed courses")
	}
	// Simulate the provider directory becoming unavailable. Local originals must
	// neither be read as preference authority nor receive a fallback write.
	if err := os.Rename(shared, shared+"-offline"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Rename(shared+"-offline", shared) })
	if _, _, err := readAppearance(a); err == nil {
		t.Fatal("unavailable UserSync treated as an absent optional file")
	}
	if err := saveAppearance(a, "light"); err == nil {
		t.Fatal("saved without authority")
	}
	var local appearanceDocument
	if _, err := workspace.ReadDocument(filepath.Join(a, appearancePath), &local); err != nil || local.Mode != "dark" {
		t.Fatal("local original changed", local, err)
	}
	if got := appearanceFor(a, "nonce"); got.Mode != "" || got.Problem == "" {
		t.Fatal("read failure must use system and report separately", got)
	}
}

func appearanceRequest(origin, secret, mode string) *http.Request {
	f := url.Values{"request_secret": {secret}, "mode": {mode}}
	r := httptest.NewRequest(http.MethodPost, origin+"/_cse/appearance", strings.NewReader(f.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Origin", origin)
	return r
}

func TestAppearanceRouteIsolatedAndValidated(t *testing.T) {
	h := newTestHub()
	h.root = makeWorkspace(t)
	before, _ := os.ReadFile(filepath.Join(h.root, "context/courses.md"))
	for _, tc := range []struct {
		origin, secret, mode string
		code                 int
	}{
		{"http://attacker.example", h.secret, "dark", 403},
		{h.origin, "wrong", "dark", 403},
		{h.origin, h.secret, "system", 400},
		{h.origin, h.secret, "dark", 200},
	} {
		r := appearanceRequest(h.origin, tc.secret, tc.mode)
		r.Header.Set("Origin", tc.origin)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.code {
			t.Fatal(w.Code, w.Body.String())
		}
		if tc.code != 200 {
			if _, err := os.Stat(filepath.Join(h.root, appearancePath)); !os.IsNotExist(err) {
				t.Fatal("invalid request saved preference")
			}
		} else if strings.TrimSpace(w.Body.String()) != `{"mode":"dark"}` {
			t.Fatal(w.Body.String())
		}
	}
	after, _ := os.ReadFile(filepath.Join(h.root, "context/courses.md"))
	if string(before) != string(after) {
		t.Fatal("appearance changed unrelated configuration")
	}
	for _, path := range []string{"/capabilities", "/settings?section=preferences", "/connect"} {
		w := hubGet(h, path)
		if w.Code != 200 || !strings.Contains(w.Body.String(), `data-appearance="dark"`) || strings.Count(w.Body.String(), `data-appearance-toggle`) < 1 {
			t.Fatal(path, w.Code, w.Body.String())
		}
	}
}

func TestStandaloneAppearanceBypassesAllReviewCallbacks(t *testing.T) {
	root := makeWorkspace(t)
	var skipped map[string]bool
	h := reviewTestHandler(t, &skipped)
	h.root = root
	builds, posts, tests := 0, 0, 0
	h.build = func() (workflow.ReviewModel, error) { builds++; return reviewModelFixture(), nil }
	h.post = func(context.Context, map[string]bool) (ReviewPostResult, error) {
		posts++
		return ReviewPostResult{}, nil
	}
	h.test = func(context.Context) (string, error) { tests++; return "", nil }
	w := httptest.NewRecorder()
	h.ServeHTTP(w, appearanceRequest(h.origin, h.secret, "dark"))
	if w.Code != 200 || builds != 0 || posts != 0 || tests != 0 || h.outcome != nil {
		t.Fatal("appearance invoked review action", w.Code, builds, posts, tests)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, h.origin+"/review", nil))
	if !strings.Contains(w.Body.String(), `data-appearance="dark"`) || !strings.Contains(w.Body.String(), `type="button" role="switch"`) || !strings.Contains(w.Header().Get("Content-Security-Policy"), "connect-src 'self'") {
		t.Fatal(w.Body.String())
	}
	// A separate server with a different port starts with the same preference.
	second := reviewTestHandler(t, &skipped)
	second.root = root
	second.origin = "http://127.0.0.1:5678"
	w = httptest.NewRecorder()
	second.ServeHTTP(w, httptest.NewRequest(http.MethodGet, second.origin+"/review", nil))
	if !strings.Contains(w.Body.String(), `data-appearance="dark"`) {
		t.Fatal("new server lost preference")
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, reviewForm(h, reviewModelFixture().ReviewDigest(), "revise", url.Values{"status_1": {"revise"}, "note_1": {"Retain my revision"}}))
	if w.Code != 200 || h.outcome == nil || posts != 0 {
		t.Fatal(w.Code, w.Body.String())
	}
	assertTerminalAppearance(t, w.Body.String())
	select {
	case <-h.done:
	default:
		t.Fatal("review did not finish")
	}
}

func assertTerminalAppearance(t *testing.T, body string) {
	t.Helper()
	if !strings.Contains(body, `data-appearance="dark"`) || strings.Contains(body, `<script`) || strings.Contains(body, `data-appearance-toggle`) {
		t.Fatal("terminal page depends on live server or loses palette", body)
	}
}

func TestSetupAndStudentsUseExplicitAppearanceRoot(t *testing.T) {
	root := makeWorkspace(t)
	if err := saveAppearance(root, "dark"); err != nil {
		t.Fatal(err)
	}
	setupCalls := 0
	s := &setupHandler{root: root, origin: "http://127.0.0.1:1234", secret: "nonce", ctx: context.Background(), done: make(chan struct{}), validate: func(context.Context, platform.Credential) error { setupCalls++; return nil }, save: func(platform.Credential) error { setupCalls++; return nil }}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, appearanceRequest(s.origin, s.secret, "dark"))
	if w.Code != 200 || setupCalls != 0 {
		t.Fatal("appearance changed credentials")
	}
	w = httptest.NewRecorder()
	s.ServeHTTP(w, httptest.NewRequest(http.MethodGet, s.origin+"/setup", nil))
	if !strings.Contains(w.Body.String(), `data-appearance="dark"`) || !strings.Contains(w.Body.String(), `data-appearance-toggle`) {
		t.Fatal(w.Body.String())
	}
	w = httptest.NewRecorder()
	s.ServeHTTP(w, formRequest(s.origin, s.secret, "synthetic-token"))
	assertTerminalAppearance(t, w.Body.String())
	select {
	case <-s.done:
	default:
		t.Fatal("setup did not finish")
	}
	store, err := workspace.OpenStudents(root)
	if err != nil {
		t.Fatal(err)
	}
	student := &studentHandler{origin: s.origin, secret: s.secret, notesRoot: root, store: store, done: make(chan struct{})}
	w = httptest.NewRecorder()
	student.ServeHTTP(w, appearanceRequest(student.origin, student.secret, "dark"))
	if w.Code != 200 || student.complete {
		t.Fatal("appearance saved students")
	}
	w = httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, student.origin+"/students", strings.NewReader("request_secret=nonce"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Origin", student.origin)
	student.ServeHTTP(w, r)
	assertTerminalAppearance(t, w.Body.String())
	select {
	case <-student.done:
	default:
		t.Fatal("students did not finish")
	}
}
