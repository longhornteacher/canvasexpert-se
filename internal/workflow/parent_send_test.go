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
	contacts.Students[workspace.ContactKey(a)] = workspace.StudentContacts{StudentEmail: "ada@student.example", Parents: []workspace.ParentContact{
		{Email: "p1@example.org", EmailEnabled: true}, {Email: "p2@example.org", EmailEnabled: false}, {Email: "p3@example.org", EmailEnabled: true}}}
	if err = contacts.Save(); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 16, 15, 0, 0, 0, time.UTC)
	course := func() ParentReportCourse {
		s := "80"
		return ParentReportCourse{Course: "cs8", Timezone: "America/Chicago", CourseScore: &s,
			WorkDue: []ParentWorkDueItem{
				{ParentReportItem: ParentReportItem{AssignmentID: 1, Title: "Lab <1>", URL: "https://canvas.example.edu/a/1", DueAt: "2026-10-14T10:00:00-05:00"}, Status: "graded", Possible: "10", Current: str("0"), Percent: str("0"), Submission: "not_turned_in"},
				{ParentReportItem: ParentReportItem{AssignmentID: 2, Title: "Completed project", URL: "https://canvas.example.edu/a/2", DueAt: "2026-10-15T10:00:00-05:00"}, Status: "graded", Possible: "10", Current: str("10"), Percent: str("100"), Submission: "turned_in"},
			},
		}
	}
	report := ParentReportPacket{Schema: 2, Kind: "parent-report", RunID: "R1", Courses: []string{"cs8"}, AsOf: now.Format(time.RFC3339), Scope: parentTestScope(now),
		Students: []ParentReportStudent{{Ref: a.Ref, Alias: a.Alias, Courses: []ParentReportCourse{course()}}, {Ref: b.Ref, Alias: b.Alias, Courses: []ParentReportCourse{course()}}}}
	dir := filepath.Join(root, "courses", "cs8", "parent-updates", "R1")
	if err = os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(report)
	if err = os.WriteFile(filepath.Join(dir, "report.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	writeParentDrafts(t, dir, parentTestDrafts())
	return svc, dir, a, b
}

// parentTestDrafts is a schema 3 draft as the seed playbook writes it.
func parentTestDrafts() ParentDrafts {
	seventy := 70.0
	return ParentDrafts{Schema: 3, Kind: "parent-drafts", RunID: "R1",
		Subject: "{{student_name}}: Term 2 assignments below 70",
		Body:    "Hello,\n\nHere are {{student_name}}'s Term 2 assignments below 70%.\n\n{{assignment_table}}\n\n[Grade portal](https://portal.example.edu) for official grades.",
		Table: ParentTable{BelowPercent: &seventy, Columns: []string{"assignment", "score", "due", "submission"},
			Labels:           map[string]string{"assignment": "Assignment", "score": "Score", "due": "Due date", "submission": "Turned in?"},
			SubmissionLabels: map[string]string{"turned_in": "Turned in", "not_turned_in": "Not turned in", "no_online_submission": "No online submission required", "unknown": "Status unavailable"},
			EmptyText:        "No assignments below 70% this term."},
	}
}

func writeParentDrafts(t *testing.T, dir string, d ParentDrafts) {
	t.Helper()
	raw, _ := json.Marshal(d)
	if err := os.WriteFile(filepath.Join(dir, "drafts.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
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
	facts := map[string]string{}
	for _, f := range ca.Facts {
		facts[f.Label] = f.Value
	}
	if facts["To"] != "ada@student.example" || facts["Cc"] != "p1@example.org, p3@example.org" || ca.Hold != "" {
		t.Fatalf("facts = %v hold = %q (student in To, enabled parents in Cc, disabled excluded)", facts, ca.Hold)
	}
	html := ca.Frames[0].HTML
	if !strings.Contains(html, "Lab &lt;1&gt;") || !strings.Contains(html, "Here are Ada") || !strings.Contains(ca.Frames[0].Label, "Ada: Term 2") {
		t.Fatalf("render must escape titles and expand the student name: %s %s", ca.Frames[0].Label, html)
	}
	if strings.Contains(html, "Completed project") || strings.Contains(html, "AI") || strings.Contains(html, a.Alias) {
		t.Fatalf("email must list only below-threshold work and carry no AI notice or alias: %s", html)
	}
	if cards[b.Ref].Hold == "" {
		t.Fatal("student with no enabled email must be held")
	}
}

func TestSendParentUpdatesRecordsOutcomesAndNeverResends(t *testing.T) {
	svc, dir, a, _ := parentFixtureRun(t)
	f := &fakeSender{}
	status, summary, err := svc.SendParentUpdates(context.Background(), dir, map[string]bool{}, false, f)
	if err != nil || status != "completed" || len(f.sent) != 1 {
		t.Fatalf("status=%s sent=%d err=%v (one message per student with a parent address)", status, len(f.sent), err)
	}
	if m := f.sent[0]; strings.Join(m.To, ",") != "ada@student.example" || strings.Join(m.Cc, ",") != "p1@example.org,p3@example.org" {
		t.Fatalf("to=%v cc=%v", m.To, m.Cc)
	}
	if strings.Contains(summary, "@") {
		t.Fatalf("summary leaks an address: %s", summary)
	}
	if _, _, err = svc.SendParentUpdates(context.Background(), dir, map[string]bool{}, false, f); err != nil || len(f.sent) != 1 {
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

func TestParentsOnlyRecipients(t *testing.T) {
	svc, dir, _, _ := parentFixtureRun(t)
	d := parentTestDrafts()
	d.Recipients = "parents_only"
	writeParentDrafts(t, dir, d)
	f := &fakeSender{}
	if _, _, err := svc.SendParentUpdates(context.Background(), dir, map[string]bool{}, false, f); err != nil || len(f.sent) != 1 {
		t.Fatalf("sent=%d err=%v", len(f.sent), err)
	}
	if m := f.sent[0]; strings.Contains(strings.Join(append(m.To, m.Cc...), ","), "student") {
		t.Fatalf("parents_only must leave the student off: to=%v cc=%v", m.To, m.Cc)
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
	if len(f.sent) != 0 || len(f.drafts) != 1 {
		t.Fatalf("drafts-only must not send: sent=%d drafts=%d", len(f.sent), len(f.drafts))
	}
}

func TestDraftValidation(t *testing.T) {
	svc, dir, a, b := parentFixtureRun(t)
	try := func(mut func(*ParentDrafts)) error {
		d := parentTestDrafts()
		mut(&d)
		writeParentDrafts(t, dir, d)
		_, err := LoadParentRun(svc.Root, dir)
		return err
	}
	for name, mut := range map[string]func(*ParentDrafts){
		"older schema":    func(d *ParentDrafts) { d.Schema = 1 },
		"other run":       func(d *ParentDrafts) { d.RunID = "R2" },
		"unknown student": func(d *ParentDrafts) { d.Students = []ParentStudentDraft{{Ref: "s-nobody", Decision: "skip"}} },
		"twice": func(d *ParentDrafts) {
			d.Students = []ParentStudentDraft{{Ref: a.Ref, Decision: "skip"}, {Ref: a.Ref, Decision: "skip"}}
		},
		"bad decision":        func(d *ParentDrafts) { d.Students = []ParentStudentDraft{{Ref: b.Ref, Decision: "maybe"}} },
		"no table":            func(d *ParentDrafts) { d.Body = "Hello" },
		"table twice":         func(d *ParentDrafts) { d.Body += "\n\n{{assignment_table}}" },
		"unknown placeholder": func(d *ParentDrafts) { d.Body += " {{grade}}" },
		"missing label":       func(d *ParentDrafts) { delete(d.Table.Labels, "due") },
		"missing status word": func(d *ParentDrafts) { delete(d.Table.SubmissionLabels, "unknown") },
		"unknown column":      func(d *ParentDrafts) { d.Table.Columns = append(d.Table.Columns, "first_score") },
		"no empty text":       func(d *ParentDrafts) { d.Table.EmptyText = "" },
		"alias in wording":    func(d *ParentDrafts) { d.Body = "Hi " + a.Alias + "\n\n{{assignment_table}}" },
		"address in wording":  func(d *ParentDrafts) { d.Body += "\n\nWrite to me@school.example" },
		"bad send_when":       func(d *ParentDrafts) { d.SendWhen = "sometimes" },
	} {
		if err := try(mut); err == nil {
			t.Errorf("%s: expected a validation error", name)
		}
	}
	if err := try(func(d *ParentDrafts) { d.Students = []ParentStudentDraft{{Ref: b.Ref, Decision: "skip"}} }); err != nil {
		t.Errorf("a skip needs no reason: %v", err)
	}
}

func TestParentTableTokenMustStandAlone(t *testing.T) {
	svc, dir, a, _ := parentFixtureRun(t)
	d := parentTestDrafts()
	d.Body = "Hello, here is the list: {{assignment_table}}"
	writeParentDrafts(t, dir, d)
	run, err := LoadParentRun(svc.Root, dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = run.Render(run.Drafts.Students[0], a); err == nil || !strings.Contains(err.Error(), "own line") {
		t.Fatalf("inline table token must be refused: %v", err)
	}
}

func TestParentDecisionsFollowSendWhenAndHoldIncompleteEvidence(t *testing.T) {
	svc, dir, a, b := parentFixtureRun(t)
	var report ParentReportPacket
	raw, _ := os.ReadFile(filepath.Join(dir, "report.json"))
	json.Unmarshal(raw, &report)
	// Ada's zero is raised to exactly 70%: no row qualifies, so she is skipped.
	report.Students[0].Courses[0].WorkDue[0].Current = str("7")
	// Bo's course has an unresolved flag: a send is held, never sent partial.
	report.Students[1].Courses[0].Flags = []string{"assignment 3 (Essay): visibility or submission evidence unavailable"}
	raw, _ = json.Marshal(report)
	os.WriteFile(filepath.Join(dir, "report.json"), raw, 0o644)
	run, err := LoadParentRun(svc.Root, dir)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]ParentStudentDraft{}
	for _, sd := range run.Drafts.Students {
		got[sd.Ref] = sd
	}
	if got[a.Ref].Decision != "skip" || got[a.Ref].Reason != "No assignments below 70% this term." {
		t.Fatalf("exactly 70%% is not below 70: %+v", got[a.Ref])
	}
	if got[b.Ref].Decision != "flag" || !strings.Contains(got[b.Ref].Reason, "Essay") {
		t.Fatalf("incomplete evidence must hold: %+v", got[b.Ref])
	}
	d := parentTestDrafts()
	d.SendWhen = "all"
	writeParentDrafts(t, dir, d)
	run, _ = LoadParentRun(svc.Root, dir)
	r, err := run.Render(run.Drafts.Students[0], workspace.Student{FirstName: "Ada"})
	if err != nil || run.Drafts.Students[0].Decision != "send" || !strings.Contains(r.HTML, "No assignments below 70% this term.") {
		t.Fatalf("send_when all sends the empty text: %+v %v %s", run.Drafts.Students[0], err, r.HTML)
	}
}

func TestParentTableRowsColumnsAndCourses(t *testing.T) {
	loc := "America/Chicago"
	row := func(id int64, title, due, status, score, possible, submission string) ParentWorkDueItem {
		it := ParentWorkDueItem{ParentReportItem: ParentReportItem{AssignmentID: id, Title: title, URL: "https://canvas.example.edu/a/" + title, DueAt: due}, Status: status, Possible: possible, Submission: submission}
		if score != "" {
			it.Current = str(score)
		}
		return it
	}
	rs := ParentReportStudent{Ref: "s-1", Courses: []ParentReportCourse{
		{Course: "ELA7", Timezone: loc, WorkDue: []ParentWorkDueItem{
			row(1, "Essay", "2026-10-14T23:59:00-05:00", "graded", "34", "50", "turned_in"),
			row(2, "Quiz", "2026-10-15T23:59:00-05:00", "graded", "35", "50", "turned_in"),
			row(3, "Hidden", "2026-10-15T23:59:00-05:00", "grade_not_released", "", "", "turned_in"),
			row(4, "Paper", "2026-10-16T08:00:00-05:00", "graded", "5", "10", "no_online_submission"),
			row(5, "Waiting", "2026-10-16T08:00:00-05:00", "awaiting_grading", "", "10", "turned_in"),
		}},
		{Course: "CS8", Timezone: loc, WorkDue: []ParentWorkDueItem{row(6, "Lab", "2026-10-13T23:59:00-05:00", "graded", "60", "100", "unknown")}},
	}}
	d := parentTestDrafts()
	d.Table.Columns = []string{"assignment", "percent", "due", "submission"}
	d.Table.Labels["percent"] = "Percent"
	d.Table.CourseHeadings = map[string]string{"ELA7": "English 7"}
	run := &ParentRun{Drafts: d, students: map[string]ParentReportStudent{"s-1": rs}}
	got := run.renderTable(rs)
	for _, want := range []string{">English 7</h3>", ">CS8</h3>", ">Essay</a>", "68%", "Wed Oct 14", "No online submission required", "Status unavailable", ">Paper</a>", ">Lab</a>", ">Percent</th>"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %s", want, got)
		}
	}
	for _, unwanted := range []string{">Quiz<", ">Hidden<", ">Waiting<", "Score"} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("unexpected %q in %s", unwanted, got)
		}
	}
	if strings.Index(got, "Essay") > strings.Index(got, "Paper") {
		t.Fatalf("rows keep due-date order: %s", got)
	}
	d.Table.BelowPercent = nil
	run.Drafts = d
	if all := run.renderTable(rs); !strings.Contains(all, ">Quiz<") || !strings.Contains(all, ">Waiting<") {
		t.Fatalf("no threshold lists every row: %s", all)
	}
}

func TestPrintNeedsOnlyTheReport(t *testing.T) {
	svc, dir, a, _ := parentFixtureRun(t)
	reportPath := filepath.Join(dir, "report.json")
	raw, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatal(err)
	}
	var report ParentReportPacket
	if err = json.Unmarshal(raw, &report); err != nil {
		t.Fatal(err)
	}
	report.Scope = nil // an older report without saved scope
	raw, _ = json.Marshal(report)
	if err = os.WriteFile(reportPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "drafts.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	sheets, _, err := svc.BuildStudentSheets(dir)
	if err != nil || len(sheets) != 2 || sheets[0].Name != a.Name || !strings.Contains(sheets[0].HTML, "Completed project") {
		t.Fatalf("print blocked or incomplete: %v %+v", err, sheets)
	}
	if err = os.Remove(filepath.Join(dir, "drafts.json")); err != nil {
		t.Fatal(err)
	}
	if sheets, _, err = svc.BuildStudentSheets(dir); err != nil || len(sheets) != 2 {
		t.Fatalf("print without drafts.json: %v", err)
	}
}

func TestStudentSheetsCoverEveryStudentWithoutLinksOrAddresses(t *testing.T) {
	svc, dir, a, b := parentFixtureRun(t)
	sheets, label, err := svc.BuildStudentSheets(dir)
	if err != nil {
		t.Fatal(err)
	}
	if label != "cs8 · Term 2 2026-10-13–2026-12-18 2026-10-16" {
		t.Fatalf("label = %q", label)
	}
	if len(sheets) != 2 || sheets[0].Name != a.Name || sheets[1].Name != b.Name {
		t.Fatalf("sheets = %+v (one per student, sorted by name)", sheets)
	}
	for _, s := range sheets {
		if strings.Contains(s.HTML, "href") || strings.Contains(s.HTML, "@") {
			t.Fatalf("paper sheet has links or addresses: %s", s.HTML)
		}
		if strings.Contains(s.HTML, "AI") {
			t.Fatalf("sheet carries an AI notice: %s", s.HTML)
		}
		if !strings.Contains(s.HTML, ">Current grade</th>") || !strings.Contains(s.HTML, "Lab &lt;1&gt;") || !strings.Contains(s.HTML, "0 / 10") || !strings.Contains(s.HTML, "10 / 10") {
			t.Fatalf("sheet = %s", s.HTML)
		}
	}
}

func parentTestScope(asOf time.Time) *ParentReportScope {
	loc, _ := time.LoadLocation("America/Chicago")
	start := time.Date(2026, 10, 13, 0, 0, 0, 0, loc)
	return &ParentReportScope{Timezone: "America/Chicago", PeriodName: "Term 2", PeriodStartDate: "2026-10-13", PeriodEndDate: "2026-12-18", RecentStartAt: start.Format(time.RFC3339), RecentEndAt: asOf.Format(time.RFC3339)}
}

func TestGradeSheetShowsCurrentGradeOnly(t *testing.T) {
	s := "96"
	now := time.Date(2026, 10, 16, 15, 0, 0, 0, time.UTC)
	current := "90.0"
	rs := ParentReportStudent{Ref: "s-1", Courses: []ParentReportCourse{{Course: "cs8", Timezone: "America/Chicago", CourseScore: &s,
		WorkDue: []ParentWorkDueItem{
			{ParentReportItem: ParentReportItem{AssignmentID: 6, Title: "Lab 6", URL: "u6", DueAt: "2026-10-15T18:00:00-05:00"}, Status: "graded", Possible: "100.0", Current: &current, Submission: "turned_in"},
			{ParentReportItem: ParentReportItem{AssignmentID: 7, Title: "Lab 7", URL: "u7", DueAt: "2026-10-15T18:00:00-05:00"}, Status: "no_grade", Possible: "10", Submission: "not_turned_in"},
		},
	}}}
	run := &ParentRun{Report: ParentReportPacket{AsOf: now.Format(time.RFC3339), Scope: parentTestScope(now), Students: []ParentReportStudent{rs}}, students: map[string]ParentReportStudent{"s-1": rs}}
	got := run.RenderStudent(ParentStudentDraft{Ref: "s-1"}, workspace.Student{Name: "Ada Lovelace"}).HTML
	for _, want := range []string{"Ada Lovelace", "Lab 6", "Thu Oct 15", "90 / 100", "Not turned in"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %s", want, got)
		}
	}
	if strings.Contains(got, "href") {
		t.Fatalf("unexpected link in %s", got)
	}
}

func TestParentsAreToWhenNoStudentEmail(t *testing.T) {
	svc, _, a, _ := parentFixtureRun(t)
	contacts, _ := workspace.OpenContacts(svc.Root)
	c := contacts.Students[workspace.ContactKey(a)]
	c.StudentEmail = ""
	contacts.Students[workspace.ContactKey(a)] = c
	if err := contacts.Save(); err != nil {
		t.Fatal(err)
	}
	to, cc, parents, err := parentRecipients(svc.Root, a)
	if err != nil || strings.Join(to, ",") != "p1@example.org,p3@example.org" || len(cc) != 0 || parents != 2 {
		t.Fatalf("to=%v cc=%v parents=%d err=%v", to, cc, parents, err)
	}
}
