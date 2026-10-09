package localui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCapabilitiesPageWithoutWorkspace(t *testing.T) {
	h := newTestHub()
	w := hubGet(h, "/capabilities")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "not real-time Canvas health") {
		t.Fatalf("capabilities page unavailable without workspace: %d", w.Code)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, h.origin+"/capabilities", nil))
	if w.Code != http.StatusMethodNotAllowed || w.Header().Get("Allow") != "GET" {
		t.Fatalf("capabilities page accepted a write: %d", w.Code)
	}
}
