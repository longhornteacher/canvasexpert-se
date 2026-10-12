package installlog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppendAllowlistAndInstallRoot(t *testing.T) {
	root := t.TempDir()
	secret := "PAT_SYNTHETIC_9x7"
	response := "CANVAS_RESPONSE_SYNTHETIC_41"
	teacher := "TEACHER_PREF_SYNTHETIC_23"
	err := Append(root, "build-test", Entry{Step: "credential_validation", Client: "cowork", Outcome: "failed", ErrorCategory: "validation", InstallPath: root, RulesPath: filepath.Join(root, "playbooks")})
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(root, "install-log.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{secret, response, teacher} {
		if strings.Contains(string(b), value) {
			t.Fatalf("log leaked %q", value)
		}
	}
	if !strings.Contains(string(b), `"step":"credential_validation"`) || !strings.Contains(string(b), `"build":"build-test"`) {
		t.Fatalf("missing allowlisted fields: %s", b)
	}
}

func TestAppendRejectsRawFailureAndSecretValues(t *testing.T) {
	root := t.TempDir()
	secrets := []string{"PAT_SYNTHETIC_9x7", "CANVAS_RESPONSE_SYNTHETIC_41", "TEACHER_PREF_SYNTHETIC_23"}
	for _, secret := range secrets {
		if err := Append(root, "build", Entry{Step: "credential_validation", Outcome: secret}); err == nil {
			t.Fatal("unbounded failure text accepted")
		}
	}
	b, err := os.ReadFile(filepath.Join(root, "install-log.jsonl"))
	if err == nil {
		t.Fatalf("rejected secret was serialized: %s", b)
	}
}

func TestAppendFailureDoesNotAlterOtherSetupFile(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, "registration-result")
	if err := os.WriteFile(marker, []byte("succeeded"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Append(filepath.Join(root, "missing", "denied"), "b", Entry{Step: "registration", Outcome: "succeeded"}); err == nil {
		t.Fatal("expected logging failure")
	}
	b, err := os.ReadFile(marker)
	if err != nil || string(b) != "succeeded" {
		t.Fatal("logging failure changed setup outcome")
	}
}

func TestAppendRotatesAtSizeCap(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "install-log.jsonl")
	if err := os.WriteFile(p, make([]byte, maxBytes), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Append(root, "b", Entry{Step: "local_mcp_check", Outcome: "succeeded"}); err != nil {
		t.Fatal(err)
	}
	if st, err := os.Stat(p + ".1"); err != nil || st.Size() != maxBytes {
		t.Fatal("previous bounded log was not rotated")
	}
}
