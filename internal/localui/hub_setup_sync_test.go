package localui

import (
	"context"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/platform"
	"canvasexpertse/internal/workspace"
)

// setupHub returns a hub whose setup form runs the real post-save callbacks
// with mocked validation and credential storage.
func setupHub(t *testing.T, root string, saveErr error) (*Hub, *[]platform.Credential) {
	t.Helper()
	h := newTestHub()
	h.root = root
	h.installClient = "work"
	saved := &[]platform.Credential{}
	h.setup.ctx = context.Background()
	h.setup.validate = func(context.Context, platform.Credential) error { return nil }
	h.setup.save = func(c platform.Credential) error {
		if saveErr != nil {
			return saveErr
		}
		*saved = append(*saved, c)
		return nil
	}
	wireSetupCallbacks(h, "test", io.Discard)
	return h, saved
}

func submitSetup(h *Hub, address string) string {
	w := hubPost(h, "/setup", url.Values{"request_secret": {"nonce"}, "origin": {address}, "token": {"PRIVATE-PAT"}}, h.origin)
	return w.Body.String()
}

func TestHubSetupRecordsOriginInFreshStarterAndKeepsTeacherContent(t *testing.T) {
	starter, err := os.ReadFile(filepath.Join("..", "..", "workspace-template", "context", "courses.md"))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	path := filepath.Join(root, "context", "courses.md")
	if err = os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, starter, 0o644); err != nil {
		t.Fatal(err)
	}
	var before domain.CourseConfig
	bodyBefore, err := workspace.ReadDocument(path, &before)
	if err != nil {
		t.Fatal(err)
	}
	h, saved := setupHub(t, root, nil)
	page := submitSetup(h, "https://Pearland.Instructure.com/")
	var got domain.CourseConfig
	body, err := workspace.ReadDocument(path, &got)
	if err != nil || got.Instance != "https://pearland.instructure.com" || body != bodyBefore || got.Schema != before.Schema || len(*saved) != 1 {
		t.Fatalf("origin not persisted cleanly: %#v saved=%d err=%v", got, len(*saved), err)
	}
	if !strings.Contains(page, "CE-SE settings now use this Canvas address") || strings.Contains(page, "PRIVATE-PAT") {
		t.Fatalf("unexpected setup page: %s", page)
	}
	log, _ := os.ReadFile(filepath.Join(root, "install-log.jsonl"))
	if !strings.Contains(string(log), `"origin_configuration"`) || !strings.Contains(string(log), `"updated"`) || strings.Contains(string(log), "PRIVATE-PAT") {
		t.Fatalf("install log missing origin outcome: %s", log)
	}
}

func TestHubSetupKeepsPopulatedDifferentOriginAndNamesBothOrigins(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "context", "courses.md")
	config := domain.CourseConfig{Schema: 1, Instance: "https://old.instructure.com", Courses: []domain.Course{{Nickname: "CS", ID: 12, Current: true}}}
	if err := workspace.WriteDocument(path, config, "Teacher notes\n"); err != nil {
		t.Fatal(err)
	}
	h, saved := setupHub(t, root, nil)
	page := submitSetup(h, "https://new.instructure.com")
	var got domain.CourseConfig
	body, err := workspace.ReadDocument(path, &got)
	if err != nil || got.Instance != config.Instance || len(got.Courses) != 1 || body != "Teacher notes\n" || len(*saved) != 1 {
		t.Fatalf("populated configuration was adopted or lost: %#v err=%v", got, err)
	}
	for _, want := range []string{"https://old.instructure.com", "https://new.instructure.com", "cse setup"} {
		if !strings.Contains(page, want) {
			t.Fatalf("notice missing %q: %s", want, page)
		}
	}
	for _, leak := range []string{"PRIVATE-PAT"} {
		if strings.Contains(page, leak) {
			t.Fatalf("notice exposes %q", leak)
		}
	}
}

func TestHubSetupReportsUnwritableConfigurationAndSaveFailure(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "context"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "context", "courses.md"), []byte("not a course file"), 0o644); err != nil {
		t.Fatal(err)
	}
	h, saved := setupHub(t, root, nil)
	if page := submitSetup(h, "https://school.instructure.com"); len(*saved) != 1 || !strings.Contains(page, "credential was saved, but context/courses.md could not be updated") {
		t.Fatalf("partial failure not visible: %s", page)
	}
	h, saved = setupHub(t, root, errors.New("store down"))
	if page := submitSetup(h, "https://school.instructure.com"); len(*saved) != 0 || !strings.Contains(page, "could not save the credential") || strings.Contains(page, "CE-SE settings now use") {
		t.Fatalf("credential failure misreported: %s", page)
	}
}
