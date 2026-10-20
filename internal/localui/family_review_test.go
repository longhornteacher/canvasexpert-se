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

func TestFamilyReviewSharedPreviewsAndSkipOnlyBatch(t *testing.T) {
	model := workflow.ReviewModel{Kind: "Family contact", Title: "Synthetic book clubs", Email: true,
		Frames:    []workflow.ReviewFrame{{Label: "Ghost · 2 students", HTML: `<p>Dear {{student_name}}</p><img alt="Synthetic cover" src="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aRXsAAAAASUVORK5CYII=" width="180" height="240">`, Files: []string{"synthetic-reading-schedule.pdf"}}},
		Cards:     []workflow.ReviewCard{{Ref: "s-a", Alias: "Student A", Facts: []workflow.ReviewFact{{Label: "To", Value: "synthetic-student@example.invalid"}, {Label: "Cc", Value: "synthetic-parent@example.invalid"}}, Digest: "card-1"}},
		PostLabel: "Send approved emails", Actions: []workflow.ReviewAction{{Value: "drafts", Label: "Fill Outlook Drafts only"}}, ContentDigest: "attachment-1"}
	var skipped map[string]bool
	h := &reviewHandler{origin: "http://127.0.0.1:1234", secret: "secret", path: filepath.Join(t.TempDir(), "contact.json"), ctx: context.Background(), done: make(chan struct{}),
		build: func() (workflow.ReviewModel, error) { return model, nil },
		post: func(_ context.Context, skip map[string]bool) (ReviewPostResult, error) {
			skipped = skip
			return ReviewPostResult{Status: "completed", Summary: "0 sent"}, nil
		}}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, h.origin+"/review", nil))
	for _, want := range []string{"Ghost · 2 students", "data:image/png;base64,", "synthetic-parent@example.invalid", `value="skip"`, "Fill Outlook Drafts only", "Files attached:", "synthetic-reading-schedule.pdf"} {
		if !strings.Contains(w.Body.String(), want) {
			t.Fatalf("missing %q", want)
		}
	}
	if csp := w.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "img-src data:") || strings.Contains(csp, "img-src https:") {
		t.Fatalf("inline preview policy: %s", csp)
	}
	// Optional local visual fixture: all values here are deliberately synthetic.
	if target := os.Getenv("CSE_FAMILY_PREVIEW_HTML"); target != "" {
		if err := os.WriteFile(target, w.Body.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
	}
	f := url.Values{"request_secret": {h.secret}, "token": {model.ReviewDigest()}, "action": {"drafts"}, "ref_0": {"s-a"}, "status_0": {"skip"}}
	r := httptest.NewRequest(http.MethodPost, h.origin+"/submit", strings.NewReader(f.Encode()))
	r.Header.Set("Origin", h.origin)
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 || !skipped["s-a"] || len(h.outcome.Revise) != 0 {
		t.Fatalf("skip-only batch: code %d, skipped %v, outcome %+v", w.Code, skipped, h.outcome)
	}
	v, err := h.view("", nil)
	if err != nil || v.Cards[0].Status != "skip" {
		t.Fatalf("saved skip selection lost: %+v %v", v, err)
	}
}

func TestFamilyReviewRefusesChangedAttachment(t *testing.T) {
	var skipped map[string]bool
	h := reviewTestHandler(t, &skipped)
	m := reviewModelFixture()
	m.Email, m.ContentDigest = true, "original-attachment"
	h.build = func() (workflow.ReviewModel, error) { return m, nil }
	token := m.ReviewDigest()
	m.ContentDigest = "edited-attachment"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, reviewForm(h, token, "post", nil))
	if w.Code != 409 || skipped != nil {
		t.Fatal("changed attachment was approved from stale page")
	}
}
