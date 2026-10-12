package localui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestConnectionActionRequiresSameOriginSecret(t *testing.T) {
	h := newTestHub()
	h.root = makeWorkspace(t)
	form := url.Values{"request_secret": {"nonce"}, "client": {"work"}, "action": {"register"}}
	if w := hubPost(h, "/connect", form, "http://other.example"); w.Code != 403 {
		t.Fatal("foreign registration accepted")
	}
	form.Set("request_secret", "wrong")
	if w := hubPost(h, "/connect", form, h.origin); w.Code != 403 {
		t.Fatal("registration accepted wrong secret")
	}
}

func TestSetupOutcomeContainsOnlyCurrentNonSecretState(t *testing.T) {
	h := newTestHub()
	h.setupSaved = true
	for _, saved := range []bool{true, false} {
		h.credentialSavedFn = func() bool { return saved }
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, h.origin+"/setup/status", nil))
		var outcome map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &outcome); err != nil {
			t.Fatal(err)
		}
		if len(outcome) != 2 || outcome["credential_saved"] != saved {
			t.Fatalf("unexpected outcome: %v", outcome)
		}
		if !saved && outcome["setup_outcome"] != "not_saved" {
			t.Fatalf("forgotten credential reported saved: %v", outcome)
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("private status cached")
		}
	}
}

func TestConnectionPageExplainsDestinationVerificationAndMissingInstall(t *testing.T) {
	h := newTestHub()
	w := hubGet(h, "/connect")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "A local check cannot verify the destination app") || !strings.Contains(w.Body.String(), "webui.cmd") {
		t.Fatal("missing honest destination or Install guidance")
	}
}
