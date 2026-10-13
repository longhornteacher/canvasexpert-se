package localui

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestCapabilitiesPageWithoutWorkspace(t *testing.T) {
	h := newTestHub()
	w := hubGet(h, "/capabilities")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "do not show real-time Canvas health") {
		t.Fatalf("capabilities page unavailable without workspace: %d", w.Code)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, h.origin+"/capabilities", nil))
	if w.Code != http.StatusMethodNotAllowed || w.Header().Get("Allow") != "GET" {
		t.Fatalf("capabilities page accepted a write: %d", w.Code)
	}
}

func TestCapabilitiesStructureAndReviewDate(t *testing.T) {
	wording := map[capabilityStatus]string{
		capabilityGreen:  "working with recorded evidence",
		capabilityYellow: "partial or awaiting validation",
		capabilityRed:    "not implemented",
	}
	counts := map[capabilityStatus]int{}
	for _, group := range capabilityGroups {
		if _, ok := wording[group.Status]; !ok {
			t.Fatalf("group %q has unsupported status %q", group.Title, group.Status)
		}
		if strings.TrimSpace(group.Title) == "" || strings.TrimSpace(group.Summary) == "" || len(group.Items) == 0 {
			t.Fatalf("capability group is incomplete: %+v", group)
		}
		counts[group.Status] += len(group.Items)
		for _, item := range group.Items {
			if strings.TrimSpace(item.Title) == "" || strings.TrimSpace(item.Detail) == "" {
				t.Fatalf("group %q has an incomplete capability: %+v", group.Title, item)
			}
		}
	}

	h := newTestHub()
	w := hubGet(h, "/capabilities")
	body := w.Body.String()
	if strings.Contains(body, `<div class="cap-legend"`) || strings.Contains(body, `<aside class="card cap-next">`) {
		t.Fatal("capabilities page should keep the evidence list quiet and focused")
	}
	for status, count := range counts {
		if got := strings.Count(body, `class="cap-item `+string(status)+`"`); got != count {
			t.Errorf("rendered %d %s status lights; want %d", got, status, count)
		}
		if got := strings.Count(body, `<span class="cap-status-sr"> — `+wording[status]+`</span>`); got != count {
			t.Errorf("rendered %d %s status labels; want %d", got, status, count)
		}
	}

	docs, err := os.ReadFile(filepath.Join("..", "..", "docs", "engine-capabilities.md"))
	if err != nil {
		t.Fatalf("read capability evidence record: %v", err)
	}
	datePattern := regexp.MustCompile(`Last reviewed: \*\*(\d{4}-\d{2}-\d{2})\*\*`)
	match := datePattern.FindSubmatch(docs)
	if len(match) != 2 {
		t.Fatalf("capability evidence record has no structured review date")
	}
	if string(match[1]) != capabilitiesReviewed || !strings.Contains(body, `<time datetime="`+capabilitiesReviewed+`">`+capabilitiesReviewed+`</time>`) {
		t.Fatalf("review date mismatch: data=%s docs=%s", capabilitiesReviewed, match[1])
	}
}
