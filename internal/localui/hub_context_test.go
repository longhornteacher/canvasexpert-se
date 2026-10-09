package localui

import (
	"context"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"canvasexpertse/internal/canvas"
	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

func TestFeedbackVoiceSavesAndShows(t *testing.T) {
	h, root := studentsHub(t)
	if page := hubGet(h, "/settings").Body.String(); !strings.Contains(page, `name="voice"`) || !strings.Contains(page, `name="feedback_guidance"`) {
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
	page := hubGet(h, "/settings").Body.String()
	if !strings.Contains(page, "Feedback voice saved") || !strings.Contains(page, "Use &lt;first name&gt;.") {
		t.Fatal("saved voice not shown back")
	}
	form.Set("voice", strings.Repeat("x", maxContextText+1))
	hubPost(h, "/grading", form, h.origin)
	if page := hubGet(h, "/settings").Body.String(); !strings.Contains(page, "8,000 characters") {
		t.Fatal("oversized voice accepted")
	}
	if w := hubPost(h, "/grading", form, "http://attacker.example"); w.Code != 403 {
		t.Fatal("cross-origin grading save allowed")
	}
}

func TestStudentNotesSaveWithoutTouchingHandWrittenNotes(t *testing.T) {
	h, root := studentsHub(t)
	config := domain.CourseConfig{Schema: 1, Instance: "https://school.example", Courses: []domain.Course{{ID: 11, Nickname: "CS8", Current: true}}}
	if err := workspace.WriteDocument(filepath.Join(root, "context", "courses.md"), config, ""); err != nil {
		t.Fatal(err)
	}
	if page := hubGet(h, "/students").Body.String(); !strings.Contains(page, "Notes for the agent") {
		t.Fatal("notes column missing")
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
	h.whoamiFn = func(context.Context) (string, error) { return "Ms. Teacher", nil }
	if w := hubPost(h, "/canvas/test", url.Values{"request_secret": {"nonce"}}, h.origin); w.Code != 303 || h.canvasUser != "Ms. Teacher" {
		t.Fatalf("test: %d %q", w.Code, h.canvasUser)
	}
	if page := hubGet(h, "/settings").Body.String(); !strings.Contains(page, "belongs to Ms. Teacher") {
		t.Fatal("success notice missing")
	}
	h.whoamiFn = func(context.Context) (string, error) { return "", &canvas.Error{Kind: "authentication"} }
	hubPost(h, "/canvas/test", url.Values{"request_secret": {"nonce"}}, h.origin)
	if page := hubGet(h, "/settings").Body.String(); !strings.Contains(page, "may have expired") || h.canvasUser != "" {
		t.Fatal("expired-token message missing")
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
