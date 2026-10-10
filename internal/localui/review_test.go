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

	"canvasexpertse/internal/workflow"
)

func reviewModelFixture() workflow.ReviewModel {
	return workflow.ReviewModel{Kind: "Assignment scores", Title: "Essay", Cards: []workflow.ReviewCard{
		{Ref: "s-a", Attempt: 1, Total: "9 / 10", Comment: "Assessment: 9 / 10\n\nNice <b>work</b>", TeacherNote: "private-note", Digest: "d1"},
		{Ref: "s-b", Attempt: 1, Total: "5 / 10", Comment: "Needs more", Digest: "d2"},
		{Ref: "s-c", Attempt: 1, Hold: "unreadable evidence", Digest: "d3"},
	}}
}

func reviewTestHandler(t *testing.T, posted *map[string]bool) *reviewHandler {
	model := reviewModelFixture()
	return &reviewHandler{origin: "http://127.0.0.1:1234", secret: "secret", path: filepath.Join(t.TempDir(), "results.md"), ctx: context.Background(), done: make(chan struct{}),
		build: func() (workflow.ReviewModel, error) { return model, nil },
		post: func(_ context.Context, skip map[string]bool) (string, error) {
			*posted = skip
			return "ok", nil
		}}
}

func reviewForm(h *reviewHandler, token, action string, extra url.Values) *http.Request {
	m := reviewModelFixture()
	f := url.Values{"request_secret": {h.secret}, "token": {token}, "action": {action}, "ref_0": {"s-a"}, "ref_1": {"s-b"}, "ref_2": {"s-c"}, "status_0": {"approved"}, "status_1": {"approved"}}
	for k, v := range extra {
		f[k] = v
	}
	_ = m
	r := httptest.NewRequest(http.MethodPost, h.origin+"/submit", strings.NewReader(f.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Origin", h.origin)
	return r
}

func TestReviewPageShowsPlainTextAndMarksPrivateNotes(t *testing.T) {
	var skip map[string]bool
	h := reviewTestHandler(t, &skip)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, h.origin+"/review", nil))
	body := w.Body.String()
	if w.Code != 200 || !strings.Contains(body, "Nice &lt;b&gt;work&lt;/b&gt;") || !strings.Contains(body, "Teacher only (not posted)") || !strings.Contains(body, "Held, will not post: unreadable evidence") {
		t.Fatal(body)
	}
}

func TestReviewPostSkipsRevisedAndHeldAndSavesNotes(t *testing.T) {
	var skip map[string]bool
	h := reviewTestHandler(t, &skip)
	token := reviewModelFixture().ReviewDigest()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, reviewForm(h, token, "post", url.Values{"status_1": {"revise"}, "note_1": {"too harsh"}}))
	if w.Code != 200 || h.outcome == nil || !h.outcome.Posted {
		t.Fatal(w.Code, w.Body.String())
	}
	if !skip["s-b"] || !skip["s-c"] || skip["s-a"] {
		t.Fatalf("wrong skip set: %v", skip)
	}
	if len(h.outcome.Revise) != 1 || h.outcome.Revise[0].Note != "too harsh" || len(h.outcome.Held) != 1 {
		t.Fatalf("%+v", h.outcome)
	}
	st, err := workflow.LoadReviewState(h.path)
	if err != nil || st.Items["s-b"].Status != "revise" || st.Items["s-b"].Note != "too harsh" {
		t.Fatal(st, err)
	}
	if _, err := os.Stat(h.path); !os.IsNotExist(err) {
		t.Fatal("review must not create or touch the results file")
	}
}

func TestReviewRefusesChangedFileAndSendRevisionsPostsNothing(t *testing.T) {
	var skip map[string]bool
	h := reviewTestHandler(t, &skip)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, reviewForm(h, "stale-token", "post", nil))
	if w.Code != 409 || skip != nil || h.outcome != nil {
		t.Fatal("stale page must not post", w.Code)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, reviewForm(h, reviewModelFixture().ReviewDigest(), "revise", nil))
	if w.Code != 200 || skip != nil || h.outcome.Posted {
		t.Fatal("send revisions must not post")
	}
}

func TestReviewRejectsWrongSecret(t *testing.T) {
	var skip map[string]bool
	h := reviewTestHandler(t, &skip)
	r := reviewForm(h, reviewModelFixture().ReviewDigest(), "post", url.Values{"request_secret": {"nope"}})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 || skip != nil {
		t.Fatal(w.Code)
	}
}

func TestReviewCreateWholeRefusesPostWithRevisionAndSandboxesFrames(t *testing.T) {
	model := workflow.ReviewModel{Kind: "New assignment", Title: "T", AllOrNothing: true, Notice: "limits", Cards: []workflow.ReviewCard{
		{Ref: "a", Frames: []workflow.ReviewFrame{{Label: "Directions", HTML: "<p onclick=x>Hi</p>"}}, Key: "B", Digest: "1"},
		{Ref: "b", Digest: "2"}}}
	called := false
	h := &reviewHandler{origin: "http://127.0.0.1:1234", secret: "secret", path: filepath.Join(t.TempDir(), "r.md"), ctx: context.Background(), done: make(chan struct{}),
		build: func() (workflow.ReviewModel, error) { return model, nil },
		post:  func(context.Context, map[string]bool) (string, error) { called = true; return "", nil }}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, h.origin+"/review", nil))
	body := w.Body.String()
	if !strings.Contains(body, `<iframe sandbox srcdoc="&lt;p onclick=x&gt;Hi`) || !strings.Contains(body, "Answer key (teacher only)") || !strings.Contains(body, "Create in Canvas") {
		t.Fatal(body)
	}
	f := url.Values{"request_secret": {"secret"}, "token": {model.ReviewDigest()}, "action": {"post"}, "ref_0": {"a"}, "ref_1": {"b"}, "status_0": {"revise"}, "status_1": {"approved"}}
	r := httptest.NewRequest(http.MethodPost, h.origin+"/submit", strings.NewReader(f.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Origin", h.origin)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 409 || called || h.outcome != nil {
		t.Fatal("a created-whole item with a revision must not post", w.Code)
	}
}
