package localui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"canvasexpertse/internal/canvas"
	"canvasexpertse/internal/platform"
)

func TestNormalizeCanvasOrigin(t *testing.T) {
	good := map[string]string{
		"school.instructure.com":                             "https://school.instructure.com",
		"  School.Instructure.com/  ":                        "https://School.Instructure.com",
		"https://school.instructure.com/courses/12?x=1#frag": "https://school.instructure.com",
		"canvas.myschool.org:8443/profile/settings":          "https://canvas.myschool.org:8443",
		"https://canvas.example":                             "https://canvas.example",
	}
	for in, want := range good {
		got := normalizeCanvasOrigin(in)
		if got != want {
			t.Errorf("%q: got %q want %q", in, got, want)
		}
		if _, err := canvas.New(got, "t"); err != nil {
			t.Errorf("%q normalized to %q but was rejected: %v", in, got, err)
		}
	}
	for _, in := range []string{"http://school.instructure.com", "javascript:alert(1)", "ftp://x.example", "https://user:pw@school.instructure.com/a", "user:pw@school.instructure.com", "mailto:a@b.c", ""} {
		if _, err := canvas.New(normalizeCanvasOrigin(in), "t"); err == nil {
			t.Errorf("%q should be rejected", in)
		}
	}
}

func TestSetupAcceptsPastedCanvasLinkWithoutEchoingIt(t *testing.T) {
	var got platform.Credential
	h := &setupHandler{origin: "http://127.0.0.1:1234", secret: "secret", ctx: context.Background(), done: make(chan struct{}),
		validate: func(_ context.Context, c platform.Credential) error { got = c; return nil },
		save:     func(platform.Credential) error { return nil }}
	form := url.Values{"origin": {"https://school.instructure.com/courses/9?session=SECRETQ"}, "token": {"PAT"}, "request_secret": {"secret"}}
	r := httptest.NewRequest(http.MethodPost, h.origin+"/setup", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Origin", h.origin)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if got.Origin != "https://school.instructure.com" || strings.Contains(w.Body.String(), "SECRETQ") {
		t.Fatalf("origin %q body leaked or wrong", got.Origin)
	}
}
