package localui

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"canvasexpertse/internal/canvas"
	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/platform"
	"canvasexpertse/internal/workspace"
)

func TestFeedbackVoiceSavesAndShows(t *testing.T) {
	h, root := studentsHub(t)
	if page := hubGet(h, "/settings?section=preferences").Body.String(); !strings.Contains(page, `name="voice"`) || !strings.Contains(page, `name="feedback_guidance"`) {
		t.Fatal("voice form missing")
	}
	form := url.Values{"request_secret": {"nonce"}, "voice": {"Warm & short.\r\nUse <first name>."}, "feedback_guidance": {"No spelling penalties."}}
	if w := hubPost(h, "/grading", form, h.origin); w.Code != 303 {
		t.Fatalf("save: %d", w.Code)
	}
	var config domain.GradingConfig
	if _, err := workspace.ReadDocument(filepath.Join(root, "context", "grading.md"), &config); err != nil || config.Voice != "Warm & short.\nUse <first name>." || config.FeedbackGuidance != "No spelling penalties." {
		t.Fatalf("grading.md not written: %+v %v", config, err)
	}
	page := hubGet(h, "/settings?section=preferences").Body.String()
	if !strings.Contains(page, "Feedback voice saved") || !strings.Contains(page, "Use &lt;first name&gt;.") {
		t.Fatal("saved voice not shown back")
	}
	form.Set("voice", strings.Repeat("x", maxContextText+1))
	hubPost(h, "/grading", form, h.origin)
	if page := hubGet(h, "/settings?section=preferences").Body.String(); !strings.Contains(page, "8,000 characters") {
		t.Fatal("oversized voice accepted")
	}
	if w := hubPost(h, "/grading", form, "http://attacker.example"); w.Code != 403 {
		t.Fatal("cross-origin grading save allowed")
	}
}

func TestFeedbackVoiceUsesConfiguredSyncAndKeepsCoursesLocal(t *testing.T) {
	h, root := studentsHub(t)
	syncRoot := t.TempDir()
	form := url.Values{"request_secret": {"nonce"}, "sync_root": {syncRoot}}
	if w := hubPost(h, "/rules-location", form, h.origin); w.Code != 303 {
		t.Fatalf("enable Sync: %d", w.Code)
	}
	connections := hubGet(h, "/settings?section=connections").Body.String()
	preferences := hubGet(h, "/settings?section=preferences").Body.String()
	if !strings.Contains(connections, syncRoot) || !strings.Contains(connections, "provider") || !strings.Contains(preferences, syncRoot) {
		t.Fatal("Settings did not show effective rules location and provider boundary")
	}
	form = url.Values{"request_secret": {"nonce"}, "voice": {"Shared voice."}, "feedback_guidance": {"Shared guidance."}}
	if w := hubPost(h, "/grading", form, h.origin); w.Code != 303 {
		t.Fatalf("save shared grading: %d", w.Code)
	}
	var got domain.GradingConfig
	if _, err := workspace.ReadDocument(filepath.Join(syncRoot, "context", "grading.md"), &got); err != nil || got.Voice != "Shared voice." {
		t.Fatalf("Sync grading was not saved: %+v %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(root, "context", "courses.md")); err != nil {
		t.Fatal("courses marker moved out of Install")
	}
	if _, err := os.Stat(filepath.Join(syncRoot, "context", "courses.md")); !os.IsNotExist(err) {
		t.Fatal("course configuration was copied into Sync")
	}
}

func TestStudentNotesSaveWithoutTouchingHandWrittenNotes(t *testing.T) {
	h, root := studentsHub(t)
	config := domain.CourseConfig{Schema: 1, Instance: "https://school.example", Courses: []domain.Course{{ID: 11, Nickname: "CS8", Current: true}}}
	if err := workspace.WriteDocument(filepath.Join(root, "context", "courses.md"), config, ""); err != nil {
		t.Fatal(err)
	}
	if page := hubGet(h, "/students").Body.String(); !strings.Contains(page, "Private notes for the agent") || !strings.Contains(page, "<details class=\"student-notes\">") {
		t.Fatal("private notes should be in a collapsed per-student section")
	}
	store, _ := workspace.OpenStudents(root)
	refs := map[int64]workspace.Student{}
	for _, s := range store.Rows {
		refs[s.UserID] = s
	}
	ana, bo := refs[1], refs[2]
	hand := domain.StudentNotes{Schema: 1, Notes: []domain.StudentNote{{Name: "Bo Real", Course: "CS8", Note: "Hand-written."}, {StudentRef: ana.Ref, Note: "Old note."}}}
	if err := workspace.WriteDocument(filepath.Join(root, "context", "students.md"), &hand, "Body stays."); err != nil {
		t.Fatal(err)
	}
	if page := hubGet(h, "/students").Body.String(); !strings.Contains(page, "Old note.") {
		t.Fatal("existing note not shown")
	}
	form := url.Values{"request_secret": {"nonce"}, "course": {"11"}, "alias_" + ana.Ref: {ana.Alias}, "alias_" + bo.Ref: {bo.Alias}, "note_" + ana.Ref: {"Reads aloud.\r\nExtra time on essays."}, "note_" + bo.Ref: {""}}
	if w := hubPost(h, "/students", form, h.origin); w.Code != 200 {
		t.Fatalf("save: %d %s", w.Code, w.Body.String())
	}
	var saved domain.StudentNotes
	body, err := workspace.ReadDocument(filepath.Join(root, "context", "students.md"), &saved)
	if err != nil || body != "Body stays." {
		t.Fatalf("notes file body lost: %q %v", body, err)
	}
	got := notesByRef(saved)
	if got[ana.Ref] != "Reads aloud.\nExtra time on essays." || got[bo.Ref] != "" || len(saved.Notes) != 2 || saved.Notes[0].Note != "Hand-written." {
		t.Fatalf("notes merged wrong: %+v", saved.Notes)
	}
	// The pulled-work path still attaches the hub note to the right student.
	if notes, _ := workspace.NotesFor(saved, "CS8", ana, store.Rows); len(notes) != 1 {
		t.Fatalf("hub note not attached: %v", notes)
	}
}

func TestStudentNotesMissingFileIsCreated(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "context"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := saveNotes(root, map[string]string{"s_1": "Hi"}); err != nil {
		t.Fatal(err)
	}
	notes, _, err := readNotes(root)
	if err != nil || len(notes.Notes) != 1 || notes.Notes[0].StudentRef != "s_1" {
		t.Fatalf("%+v %v", notes, err)
	}
}

func TestCanvasTestAndForget(t *testing.T) {
	h, _ := studentsHub(t)
	credential := platform.Credential{Origin: "https://canvas.example.edu", Token: "test-token"}
	h.loadCredentialFn = func() (platform.Credential, error) { return credential, nil }
	h.whoamiFn = func(context.Context) (string, error) { return "Ms. Teacher", nil }
	if w := hubPost(h, "/canvas/test", url.Values{"request_secret": {"nonce"}}, h.origin); w.Code != 303 {
		t.Fatalf("test should return promptly: %d", w.Code)
	}
	waitForCanvasHealth(t, h, canvasHealthConnected)
	if !h.connectionTested {
		t.Fatal("successful manual Test should preserve its connection-test handshake")
	}
	if page := hubGet(h, "/settings").Body.String(); !strings.Contains(page, "Ms. Teacher") {
		t.Fatal("validated account display missing")
	}
	h.whoamiFn = func(context.Context) (string, error) { return "", &canvas.Error{Kind: "authentication"} }
	hubPost(h, "/canvas/test", url.Values{"request_secret": {"nonce"}}, h.origin)
	waitForCanvasHealth(t, h, canvasHealthRejected)
	if h.canvasUser != "" || h.connectionTested {
		t.Fatal("rejected retry should clear account display and test handshake")
	}
	forgot := 0
	h.forgetFn = func() error { forgot++; return nil }
	hubPost(h, "/canvas/forget", url.Values{"request_secret": {"nonce"}}, h.origin)
	if forgot != 0 {
		t.Fatal("forgot without confirmation")
	}
	if w := hubPost(h, "/canvas/forget", url.Values{"request_secret": {"nonce"}, "confirm": {"on"}}, "http://attacker.example"); w.Code != 403 || forgot != 0 {
		t.Fatal("cross-origin forget allowed")
	}
	hubPost(h, "/canvas/forget", url.Values{"request_secret": {"nonce"}, "confirm": {"on"}}, h.origin)
	if page := hubGet(h, "/settings").Body.String(); forgot != 1 || !strings.Contains(page, "Token removed") {
		t.Fatal("forget did not run")
	}
	h.forgetFn = func() error { return errors.New("locked") }
	hubPost(h, "/canvas/forget", url.Values{"request_secret": {"nonce"}, "confirm": {"on"}}, h.origin)
	if page := hubGet(h, "/settings").Body.String(); !strings.Contains(page, "locked") {
		t.Fatal("forget failure hidden")
	}
}

func TestCanvasHealthNetworkFailureAndMissingCredential(t *testing.T) {
	h := newTestHub()
	credential := platform.Credential{Origin: "https://canvas.example.edu", Token: "network-token"}
	h.loadCredentialFn = func() (platform.Credential, error) { return credential, nil }
	calls := 0
	h.whoamiFn = func(context.Context) (string, error) { calls++; return "", errors.New("Canvas unavailable") }
	h.ensureCanvasHealth()
	waitForCanvasHealth(t, h, canvasHealthUnavailable)
	if calls != 1 || strings.Contains(h.canvasHealthNote, "Replace") {
		t.Fatalf("network error should remain one check without replacing valid token: calls=%d note=%q", calls, h.canvasHealthNote)
	}
	h.loadCredentialFn = func() (platform.Credential, error) {
		return platform.Credential{}, errors.New("no SE credential is saved on this machine")
	}
	h.ensureCanvasHealth()
	if h.canvasHealth != canvasHealthNotConnected || calls != 1 {
		t.Fatalf("missing token should not contact Canvas: state=%q calls=%d", h.canvasHealth, calls)
	}
}

func waitForCanvasHealth(t *testing.T, h *Hub, want string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		h.mu.Lock()
		state := h.canvasHealth
		h.mu.Unlock()
		if state == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("Canvas health did not reach %q; got %q", want, h.canvasHealth)
}

func TestCanvasHealthNavigationDoesNotWaitForCanvas(t *testing.T) {
	h := newTestHub()
	credential := platform.Credential{Origin: "https://canvas.example.edu", Token: "blocked-token"}
	h.loadCredentialFn = func() (platform.Credential, error) { return credential, nil }
	started, release := make(chan struct{}), make(chan struct{})
	h.whoamiFn = func(ctx context.Context) (string, error) {
		close(started)
		select {
		case <-release:
			return "Teacher", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	first := httptest.NewRecorder()
	h.ServeHTTP(first, httptest.NewRequest(http.MethodGet, h.origin+"/", nil))
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("Canvas check did not start")
	}
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, h.origin+"/settings", nil))
		done <- w
	}()
	select {
	case w := <-done:
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "Checking…") {
			t.Fatalf("settings navigation did not render checking state: %d", w.Code)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("navigation waited for Canvas")
	}
	close(release)
	waitForCanvasHealth(t, h, canvasHealthConnected)
}

func TestConnectionsKeepsHiddenAccountSlotWhileChecking(t *testing.T) {
	h := newTestHub()
	credential := platform.Credential{Origin: "https://canvas.example.edu", Token: "account-token"}
	h.loadCredentialFn = func() (platform.Credential, error) { return credential, nil }
	started, release := make(chan struct{}), make(chan struct{})
	h.whoamiFn = func(ctx context.Context) (string, error) {
		close(started)
		select {
		case <-release:
			return "Teacher Account", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	w := hubGet(h, "/settings")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `data-health-account-row hidden`) || !strings.Contains(w.Body.String(), `data-health-account></b>`) {
		t.Fatal("Connections page should keep an empty account slot hidden during Checking")
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("Canvas identity request did not start")
	}
	close(release)
	waitForCanvasHealth(t, h, canvasHealthConnected)
	if h.canvasUser != "Teacher Account" || !strings.Contains(canvasHealthScript, "accountRow.hidden=!s.account") {
		t.Fatal("successful check did not populate the account slot")
	}
}

func TestCanvasHealthStaleCredentialResultsAreIgnored(t *testing.T) {
	h := newTestHub()
	credential := platform.Credential{Origin: "https://canvas.example.edu", Token: "old-token"}
	h.loadCredentialFn = func() (platform.Credential, error) { return credential, nil }
	startedOld, startedNew, startedThird := make(chan struct{}), make(chan struct{}), make(chan struct{})
	releaseOld, releaseNew, releaseThird := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	h.whoamiFn = func(ctx context.Context) (string, error) {
		switch calls.Add(1) {
		case 1:
			close(startedOld)
			select {
			case <-releaseOld:
				return "Old account", nil
			case <-ctx.Done():
				return "", ctx.Err()
			}
		case 2:
			close(startedNew)
			select {
			case <-releaseNew:
				return "New account", nil
			case <-ctx.Done():
				return "", ctx.Err()
			}
		default:
			close(startedThird)
			select {
			case <-releaseThird:
				return "Removed account", nil
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}
	}
	h.ensureCanvasHealth()
	select {
	case <-startedOld:
	case <-time.After(time.Second):
		t.Fatal("old credential check did not start")
	}
	credential.Token = "new-token"
	h.ensureCanvasHealth()
	select {
	case <-startedNew:
	case <-time.After(time.Second):
		t.Fatal("replacement credential check did not start")
	}
	if h.canvasHealth != canvasHealthChecking {
		t.Fatalf("replacement should start a fresh check, got %q", h.canvasHealth)
	}
	close(releaseOld)
	time.Sleep(30 * time.Millisecond)
	if h.canvasHealth != canvasHealthChecking || h.canvasUser != "" {
		t.Fatalf("old token result replaced the current check: %q %q", h.canvasHealth, h.canvasUser)
	}
	close(releaseNew)
	waitForCanvasHealth(t, h, canvasHealthConnected)
	if h.canvasUser != "New account" {
		t.Fatalf("replacement token did not own the final result: %q", h.canvasUser)
	}
	credential.Token = "third-token"
	h.ensureCanvasHealth()
	select {
	case <-startedThird:
	case <-time.After(time.Second):
		t.Fatal("third credential check did not start")
	}
	// Removing the credential invalidates the in-flight third snapshot.
	h.loadCredentialFn = func() (platform.Credential, error) {
		return platform.Credential{}, errors.New("no SE credential is saved on this machine")
	}
	h.ensureCanvasHealth()
	if h.canvasHealth != canvasHealthNotConnected {
		t.Fatalf("removal should immediately show not connected, got %q", h.canvasHealth)
	}
	close(releaseThird)
	time.Sleep(30 * time.Millisecond)
	if h.canvasHealth != canvasHealthNotConnected || h.canvasUser != "" {
		t.Fatalf("stale result restored removed credential: %q %q", h.canvasHealth, h.canvasUser)
	}
}
