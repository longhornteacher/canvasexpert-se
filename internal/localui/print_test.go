package localui

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestPrintPageOneSheetPerPageAndDoneNeedsSecret(t *testing.T) {
	h := &printHandler{origin: "http://127.0.0.1:1234", secret: "nonce", sheets: []template.HTML{"<p>one</p>", "<p>two</p>"}, done: make(chan struct{})}
	r := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:1234/print", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	page := w.Body.String()
	if w.Code != 200 || strings.Count(page, `<section class="sheet">`) != 2 || !strings.Contains(page, "page-break-after:always") {
		t.Fatalf("page = %d %s", w.Code, page)
	}
	post := func(secret string) int {
		r := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:1234/done", strings.NewReader(url.Values{"request_secret": {secret}}.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("Origin", h.origin)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	if post("wrong") != http.StatusForbidden {
		t.Fatal("Done accepted a wrong secret")
	}
	select {
	case <-h.done:
		t.Fatal("closed without the secret")
	default:
	}
	if post("nonce") != 200 {
		t.Fatal("Done refused the page's secret")
	}
	<-h.done
}
