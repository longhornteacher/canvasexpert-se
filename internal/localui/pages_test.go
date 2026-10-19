package localui

import (
	"encoding/json"
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Every nav entry must lead to a registered page, and every page's CSP must
// follow its declaration.
func TestRegisteredPagesDriveNavAndCSP(t *testing.T) {
	h := newTestHub()
	for _, e := range navEntries() {
		path, _, _ := strings.Cut(e.href, "?")
		if pages[path] == nil {
			t.Errorf("nav entry %q points at unregistered %s", e.label, e.href)
		}
	}
	for path, p := range pages {
		w := httptest.NewRecorder()
		h.headers(w, httptest.NewRequest(http.MethodGet, h.origin+path, nil))
		csp := w.Header().Get("Content-Security-Policy")
		if has := strings.Contains(csp, "script-src 'self'"); has != (len(p.Scripts) > 0) {
			t.Errorf("%s: script-src %v, scripts %v", path, has, p.Scripts)
		}
		if has := strings.Contains(csp, "connect-src 'self'"); has != (p.Connect || hasAppearance(p)) {
			t.Errorf("%s: connect-src %v, Connect %v", path, has, p.Connect)
		}
	}
}

func TestCanvasHealthJSONUsesStableKeyAndSeparateLabel(t *testing.T) {
	h := newTestHub()
	h.canvasHealth = canvasHealthRejected
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, h.origin+"/_cse/canvas-health", nil))
	var got struct{ State, Label string }
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil || got.State != "rejected" || got.Label == "" || got.Label == got.State {
		t.Fatalf("state key and label not separate: %q err=%v", w.Body.String(), err)
	}
}

func TestRenderTemplateFailureIsAPlain500(t *testing.T) {
	h := newTestHub()
	w := httptest.NewRecorder()
	h.render(w, 200, template.Must(template.New("x").Parse("partial {{.Missing.Field}}")), struct{ Missing *struct{ Field string } }{})
	if w.Code != http.StatusInternalServerError || strings.Contains(w.Body.String(), "partial") {
		t.Fatalf("template error should not send a truncated page: %d %q", w.Code, w.Body.String())
	}
}

// /setup validates a token over the network, so it must not run under the hub
// lock that every other page waits on.
func TestSetupPageDoesNotNeedTheHubLock(t *testing.T) {
	h := newTestHub()
	h.mu.Lock()
	defer h.mu.Unlock()
	done := make(chan int, 1)
	go func() {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, h.origin+"/setup", nil))
		done <- w.Code
	}()
	select {
	case code := <-done:
		if code != http.StatusOK {
			t.Fatalf("setup page status %d", code)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("/setup waited for the hub lock")
	}
}
