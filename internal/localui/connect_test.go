package localui

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"canvasexpertse/internal/installation"

	"canvasexpertse/internal/platform"
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
	for _, saved := range []bool{true, false} {
		h.credentialSavedFn = func() bool { return saved }
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, h.origin+"/setup/status", nil))
		var outcome map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &outcome); err != nil {
			t.Fatal(err)
		}
		if len(outcome) != 4 || outcome["credential_saved"] != saved {
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

func TestSetupStatusTracksValidationSaveTestAndRemoval(t *testing.T) {
	h := newTestHub()
	saved := false
	h.credentialSavedFn = func() bool { return saved }
	h.setup.ctx = context.Background()
	h.setup.validate = func(context.Context, platform.Credential) error { return nil }
	h.setup.save = func(platform.Credential) error { saved = true; return nil }
	h.setup.onValidation = func(valid bool) { h.setupValidated, h.setupSaved, h.connectionTested = valid, false, false }
	h.setup.onSaved = func() { h.setupSaved = true }
	h.setup.after = func(platform.Credential) string { h.connectionTested = true; return "" }
	read := func() map[string]any {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, h.origin+"/setup/status", nil))
		var state map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &state); err != nil {
			t.Fatal(err)
		}
		return state
	}
	assert := func(wantOutcome string, validated, tested, credentialSaved bool) {
		t.Helper()
		state := read()
		if state["setup_outcome"] != wantOutcome || state["validation_succeeded"] != validated || state["connection_tested"] != tested || state["credential_saved"] != credentialSaved {
			t.Fatalf("unexpected status: %v", state)
		}
	}
	assert("not_saved", false, false, false)
	form := url.Values{"request_secret": {"nonce"}, "origin": {"https://canvas.example"}, "token": {"PRIVATE-PAT"}}
	if w := hubPost(h, "/setup", form, h.origin); w.Code != 200 {
		t.Fatalf("setup failed: %d", w.Code)
	}
	assert("saved_this_session", true, true, true)

	// A later validated replacement that cannot be saved must not claim a new
	// completed connection; the previously stored credential remains present.
	h.setup.save = func(platform.Credential) error { return errors.New("credential store unavailable") }
	if w := hubPost(h, "/setup", form, h.origin); w.Code != 500 {
		t.Fatalf("expected save failure, got %d", w.Code)
	}
	assert("saved_not_tested", true, false, true)

	// Restart semantics are the same as a saved credential with no test in this
	// process, even when the preceding process had tested it.
	h.setupValidated, h.setupSaved, h.connectionTested = false, false, false
	assert("saved_not_tested", false, false, true)

	h.forgetFn = func() error { saved = false; return nil }
	form = url.Values{"request_secret": {"nonce"}, "confirm": {"on"}}
	if w := hubPost(h, "/canvas/forget", form, h.origin); w.Code != 303 {
		t.Fatalf("forget failed: %d", w.Code)
	}
	assert("not_saved", false, false, false)
}

func TestFailedCanvasTestClearsTestedStatus(t *testing.T) {
	h := newTestHub()
	h.connectionTested = true
	h.canvasUser = "must not survive a failed check"
	h.whoamiFn = func(context.Context) (string, error) { return "", errors.New("Canvas unavailable") }
	form := url.Values{"request_secret": {"nonce"}}
	if w := hubPost(h, "/canvas/test", form, h.origin); w.Code != http.StatusSeeOther {
		t.Fatalf("test failed unexpectedly: %d", w.Code)
	}
	if h.connectionTested || h.canvasUser != "" {
		t.Fatal("failed connection test left a success display")
	}
}

func TestConnectionPageExplainsDestinationVerificationAndMissingInstall(t *testing.T) {
	h := newTestHub()
	w := hubGet(h, "/connect")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "does not check the destination app") || !strings.Contains(w.Body.String(), "webui.cmd") || !strings.Contains(w.Body.String(), `aria-current="page">Agent</a>`) {
		t.Fatal("missing honest destination or UserInstall guidance")
	}
}

func TestConnectionOutcomeLabelsDescribeLocalEvidence(t *testing.T) {
	cases := []struct {
		result installation.Result
		want   string
	}{
		{installation.Result{State: "settings-saved"}, "Work registration saved"},
		{installation.Result{State: "settings-already-saved"}, "Work already points to this UserInstall"},
		{installation.Result{State: "extension-ready", CoworkLaunchOutcome: "launch_requested"}, "Cowork extension prepared; opening was requested"},
		{installation.Result{State: "extension-ready", CoworkLaunchOutcome: "file_revealed"}, "Cowork extension prepared; file shown in File Explorer for manual install"},
		{installation.Result{State: "extension-ready", CoworkLaunchOutcome: "launch_failed"}, "Cowork extension prepared; automatic opening did not work"},
	}
	for _, tc := range cases {
		if got := connectionResultLabel(tc.result); got != tc.want {
			t.Errorf("connectionResultLabel(%+v) = %q, want %q", tc.result, got, tc.want)
		}
	}
	if got := connectionCheckLabel(installation.CheckResult{State: "local-check-passed"}); got != "Local checks passed" {
		t.Fatalf("successful local check label = %q", got)
	}
	if got := connectionCheckLabel(installation.CheckResult{State: "local-check-failed"}); got != "Local check did not pass" {
		t.Fatalf("failed local check label = %q", got)
	}
}

func TestConnectionShortcutWarningContinuesAndReadOnlyDoesNotCreate(t *testing.T) {
	h := newTestHub()
	h.root = makeWorkspace(t)
	for _, name := range []string{"engine/cse.exe", "AGENTS.md"} {
		path := filepath.Join(h.root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("CODEX_HOME", t.TempDir())
	calls := 0
	h.desktopShortcutFn = func(root string) (string, error) {
		calls++
		if root != h.root {
			t.Fatal("shortcut used wrong UserInstall")
		}
		return "", errors.New("fixture Desktop unavailable")
	}
	hubGet(h, "/connect")
	form := url.Values{"request_secret": {"nonce"}, "client": {"invalid"}, "action": {"register"}}
	hubPost(h, "/connect", form, h.origin)
	form.Set("action", "check")
	hubPost(h, "/connect", form, h.origin)
	if calls != 0 {
		t.Fatal("read-only or invalid connection action created a shortcut")
	}
	form.Set("action", "register")
	form.Set("client", "work")
	w := hubPost(h, "/connect", form, h.origin)
	if calls != 1 || h.connection.Result.State != "settings-saved" || !strings.Contains(w.Body.String(), "fixture Desktop unavailable") || !strings.Contains(w.Body.String(), "webui.cmd") {
		t.Fatalf("shortcut failure stopped registration or hid warning: %d %+v %s", calls, h.connection.Result, w.Body.String())
	}
}
