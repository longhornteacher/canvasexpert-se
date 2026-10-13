package localui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
		if has := strings.Contains(csp, "connect-src 'self'"); has != p.Connect {
			t.Errorf("%s: connect-src %v, Connect %v", path, has, p.Connect)
		}
	}
}
