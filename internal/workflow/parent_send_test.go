package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"canvasexpertse/internal/canvas"
	"canvasexpertse/internal/outlook"
	"canvasexpertse/internal/workspace"
)

type fakeSender struct {
	sent   []outlook.Message
	drafts []outlook.Message
	next   outlook.Outcome
	err    error
}

func (f *fakeSender) Send(m outlook.Message) (outlook.Outcome, error) {
	f.sent = append(f.sent, m)
	if f.next != "" {
		return f.next, f.err
	}
	return outlook.Sent, nil
}
func (f *fakeSender) SaveDraft(m outlook.Message) error { f.drafts = append(f.drafts, m); return nil }
func (f *fakeSender) SelfAddress() (string, error)      { return "teacher@example.org", nil }

// parentFixtureRun builds a workspace with two students: one with an enabled
// parent email and a disabled one, one with no email at all.
func parentFixtureRun(t *testing.T) (*Service, string, workspace.Student, workspace.Student) {
	t.Helper()
	root := t.TempDir()
	client, err := canvas.New("https://canvas.example.edu", "test-token")
	if err != nil {
		t.Fatal(err)
	}
	svc := &Service{Root: root, Client: client}
	store, err := workspace.OpenStudents(root)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := store.Upsert(client.Origin(), 11, "Ada Lovelace", "Ada", "", "")
	b, _ := store.Upsert(client.Origin(), 12, "Bo Peep", "Bo", "", "")
	if err = store.Save(); err != nil {
		t.Fatal(err)
	}
	contacts, _ := workspace.OpenContacts(root)
	contacts.Students[workspace.ContactKey(a)] = workspace.StudentContacts{Parents: []workspace.ParentContact{
		{Email: "p1@example.org", EmailEnabled: true}, {Email: "p2@example.org", EmailEnabled: false}, {Email: "p3@example.org", EmailEnabled: true}}}
	if err = contacts.Save(); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 16, 15, 0, 0, 0, time.UTC)
	course := func() ParentReportCourse {
		s := "80"
		return ParentReportCourse{Course: "cs8", Timezone: "America/Chicago", CourseScore: &s,
			Missing: []ParentReportItem{{AssignmentID: 1, Title: "Lab <1>", URL: "https://canvas.example.edu/a/1", DueAt: "2026-10-14T10:00:00-05:00", Basis: "canvas_missing"}}}
	}
	report := ParentReportPacket{Schema: 1, Kind: "parent-report", RunID: "R1", Courses: []string{"cs8"}, AsOf: now.Format(time.RFC3339),
		Students: []ParentReportStudent{{Ref: a.Ref, Alias: a.Alias, Courses: []ParentReportCourse{course()}}, {Ref: b.Ref, Alias: b.Alias, Courses: []ParentReportCourse{course()}}}}
	dir := filepath.Join(root, "courses", "cs8", "parent-updates", "R1")
	if err = os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name string, v any) {
		b, _ := json.Marshal(v)
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("report.json", report)
	write("drafts.json", ParentDrafts{Schema: 1, Kind: "parent-drafts", RunID: "R1", Students: []ParentStudentDraft{
		{Ref: a.Ref, Decision: "send", Reason: "missing lab", Intro: "Hello about {{student_name}}", Missing: []ParentItemRef{{AssignmentID: 1}}},
		{Ref: b.Ref, Decision: "send", Reason: "missing lab", Missing: []ParentItemRef{{AssignmentID: 1}}},
	}})
	return svc, dir, a, b
}

func TestParentReviewShowsRealRecipientsAndHoldsMissingAddress(t *testing.T) {
	svc, dir, a, b := parentFixtureRun(t)
	m, err := svc.BuildParentReview(dir)
	if err != nil {
		t.Fatal(err)
	}
	cards := map[string]ReviewCard{}
	for _, c := range m.Cards {
		cards[c.Ref] = c
	}
	ca := cards[a.Ref]
	var to string
	for _, f := range ca.Facts {
		if f.Label == "To" {
			to = f.Value
		}
	}
	if to != "p1@example.org, p3@example.org" || ca.Hold != "" {
		t.Fatalf("To = %q hold = %q (disabled destination must be excluded)", to, ca.Hold)
	}
	if !strings.Contains(ca.Frames[0].HTML, "Lab &lt;1&gt;") || !strings.Contains(ca.Frames[0].HTML, "Hello about Ada") {
		t.Fatalf("render must escape titles and expand the student name: %s", ca.Frames[0].HTML)
	}
	if cards[b.Ref].Hold == "" {
		t.Fatal("student with no enabled email must be held")
	}
}

func TestSendParentUpdatesRecordsOutcomesAndNeverResends(t *testing.T) {
	svc, dir, a, _ := parentFixtureRun(t)
	f := &fakeSender{}
	status, summary, err := svc.SendParentUpdates(context.Background(), dir, map[string]bool{}, false, f)
	if err != nil || status != "completed" || len(f.sent) != 2 {
		t.Fatalf("status=%s sent=%d err=%v (one message per enabled destination)", status, len(f.sent), err)
	}
	if strings.Contains(summary, "@") {
		t.Fatalf("summary leaks an address: %s", summary)
	}
	if _, _, err = svc.SendParentUpdates(context.Background(), dir, map[string]bool{}, false, f); err != nil || len(f.sent) != 2 {
		t.Fatalf("retry resent mail: %d", len(f.sent))
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "outcomes.json"))
	if strings.Contains(string(raw), "@") || !strings.Contains(string(raw), a.Ref) {
		t.Fatalf("outcomes must be keyed by ref and hold no addresses: %s", raw)
	}
	m, _ := svc.BuildParentReview(dir)
	for _, c := range m.Cards {
		if c.Ref == a.Ref && !c.Done {
			t.Fatal("fully sent student must show as already sent")
		}
	}
}

func TestUncertainIsNotRetriedAndFailureIs(t *testing.T) {
	svc, dir, a, _ := parentFixtureRun(t)
	f := &fakeSender{next: outlook.Uncertain, err: errors.New("com error after send")}
	status, _, _ := svc.SendParentUpdates(context.Background(), dir, map[string]bool{}, false, f)
	if status != "partial" {
		t.Fatalf("status %s", status)
	}
	g := &fakeSender{}
	svc.SendParentUpdates(context.Background(), dir, map[string]bool{}, false, g)
	if len(g.sent) != 0 {
		t.Fatalf("uncertain deliveries must not be resent automatically: %d", len(g.sent))
	}
	m, _ := svc.BuildParentReview(dir)
	for _, c := range m.Cards {
		if c.Ref == a.Ref && !strings.Contains(c.Hold, "Sent Items") {
			t.Fatalf("uncertain must tell the teacher to check Sent Items: %q", c.Hold)
		}
	}
}

func TestSkippedStudentsAndDraftsOnly(t *testing.T) {
	svc, dir, a, _ := parentFixtureRun(t)
	f := &fakeSender{}
	svc.SendParentUpdates(context.Background(), dir, map[string]bool{a.Ref: true}, true, f)
	if len(f.sent) != 0 || len(f.drafts) != 0 {
		t.Fatalf("unapproved student must not be touched; the other has no address: sent=%d drafts=%d", len(f.sent), len(f.drafts))
	}
	svc.SendParentUpdates(context.Background(), dir, map[string]bool{}, true, f)
	if len(f.sent) != 0 || len(f.drafts) != 2 {
		t.Fatalf("drafts-only must not send: sent=%d drafts=%d", len(f.sent), len(f.drafts))
	}
}

func TestDraftValidation(t *testing.T) {
	svc, dir, _, b := parentFixtureRun(t)
	load := func(mut func(*ParentDrafts)) error {
		var d ParentDrafts
		raw, _ := os.ReadFile(filepath.Join(dir, "drafts.json"))
		json.Unmarshal(raw, &d)
		mut(&d)
		raw, _ = json.Marshal(d)
		os.WriteFile(filepath.Join(dir, "drafts.json"), raw, 0o644)
		_, err := LoadParentRun(svc.Root, dir)
		return err
	}
	for name, mut := range map[string]func(*ParentDrafts){
		"unknown assignment": func(d *ParentDrafts) { d.Students[0].Missing[0].AssignmentID = 99 },
		"typed score":        func(d *ParentDrafts) { d.Students[0].Missing[0].Note = "You earned 7/10" },
		"missing decision":   func(d *ParentDrafts) { d.Students = d.Students[:1] },
		"empty send":         func(d *ParentDrafts) { d.Students[1].Missing = nil },
		"skip without why":   func(d *ParentDrafts) { d.Students[1] = ParentStudentDraft{Ref: b.Ref, Decision: "skip"} },
	} {
		if err := load(mut); err == nil {
			t.Errorf("%s: expected a validation error", name)
		}
		parentFixtureRewrite(t, svc, dir)
	}
}

// parentFixtureRewrite restores a valid drafts.json between validation cases.
func parentFixtureRewrite(t *testing.T, svc *Service, dir string) {
	t.Helper()
	var r ParentReportPacket
	raw, _ := os.ReadFile(filepath.Join(dir, "report.json"))
	json.Unmarshal(raw, &r)
	d := ParentDrafts{Schema: 1, Kind: "parent-drafts", RunID: "R1", Students: []ParentStudentDraft{
		{Ref: r.Students[0].Ref, Decision: "send", Missing: []ParentItemRef{{AssignmentID: 1}}},
		{Ref: r.Students[1].Ref, Decision: "send", Missing: []ParentItemRef{{AssignmentID: 1}}}}}
	raw, _ = json.Marshal(d)
	os.WriteFile(filepath.Join(dir, "drafts.json"), raw, 0o644)
}
