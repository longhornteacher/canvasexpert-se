package localui

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"canvasexpertse/internal/canvas"
	"canvasexpertse/internal/platform"
)

func formRequest(origin, secret, token string) *http.Request {
	form := url.Values{"origin": {"https://canvas.example"}, "token": {token}, "request_secret": {secret}}
	r := httptest.NewRequest(http.MethodPost, origin+"/setup", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Origin", origin)
	return r
}

func TestSetupDistinguishesRejectedTokenFromConnectionFailure(t *testing.T) {
	for _, test := range []struct{ kind, want string }{
		{"authentication", "accept this token"},
		{"network", "token is probably fine"},
	} {
		saved := false
		h := &setupHandler{origin: "http://127.0.0.1:1234", secret: "secret", ctx: context.Background(), validate: func(context.Context, platform.Credential) error { return &canvas.Error{Kind: test.kind} }, save: func(platform.Credential) error { saved = true; return nil }}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, formRequest(h.origin, h.secret, "PRIVATE-PAT"))
		if saved || strings.Contains(w.Body.String(), "PRIVATE-PAT") || !strings.Contains(w.Body.String(), test.want) || !strings.Contains(w.Body.String(), "Nothing was changed") {
			t.Fatalf("unsafe or misleading %s result: %s", test.kind, w.Body.String())
		}
	}
}
func TestSetupRejectsCrossOriginAndWrongSecret(t *testing.T) {
	called := false
	h := &setupHandler{origin: "http://127.0.0.1:1234", secret: "secret", ctx: context.Background(), done: make(chan struct{}), validate: func(context.Context, platform.Credential) error { called = true; return nil }, save: func(platform.Credential) error { called = true; return nil }}
	for _, test := range []struct{ origin, secret string }{{"http://attacker.example", "secret"}, {h.origin, "wrong"}} {
		r := formRequest(h.origin, test.secret, "PRIVATE-PAT")
		r.Header.Set("Origin", test.origin)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 403 || called || strings.Contains(w.Body.String(), "PRIVATE-PAT") {
			t.Fatal("unsafe form handling")
		}
	}
}

func TestSetupSameOriginReferrerPolicySupportsBrowserFormPost(t *testing.T) {
	h := &setupHandler{origin: "http://127.0.0.1:1234", secret: "secret"}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, h.origin+"/setup", nil))
	if w.Header().Get("Referrer-Policy") != "same-origin" {
		t.Fatal("no-referrer may turn same-origin browser form POST Origin into null")
	}
	if w.Code != 200 {
		t.Fatal("setup page unavailable")
	}
}
func TestSetupValidationFailureDoesNotEchoOrSaveToken(t *testing.T) {
	saved := false
	h := &setupHandler{origin: "http://127.0.0.1:1234", secret: "secret", ctx: context.Background(), done: make(chan struct{}), validate: func(context.Context, platform.Credential) error { return errors.New("PRIVATE-PAT") }, save: func(platform.Credential) error { saved = true; return nil }}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, formRequest(h.origin, h.secret, "PRIVATE-PAT"))
	if w.Code != 400 || saved || strings.Contains(w.Body.String(), "PRIVATE-PAT") {
		t.Fatal("token exposed or saved without validation")
	}
}
func TestSetupStoresOnlyValidatedCredentialAndCompletes(t *testing.T) {
	validated, saved := false, false
	h := &setupHandler{origin: "http://127.0.0.1:1234", secret: "secret", ctx: context.Background(), done: make(chan struct{}), validate: func(_ context.Context, c platform.Credential) error { validated = c.Token == "PRIVATE-PAT"; return nil }, save: func(c platform.Credential) error {
		saved = validated && c.Origin == "https://canvas.example"
		return nil
	}}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, formRequest(h.origin, h.secret, "PRIVATE-PAT"))
	if w.Code != 200 || !saved || strings.Contains(w.Body.String(), "PRIVATE-PAT") {
		t.Fatal("setup did not complete safely")
	}
	select {
	case <-h.done:
	default:
		t.Fatal("setup did not signal completion")
	}
}
