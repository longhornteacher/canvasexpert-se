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
	start := time.Date(2026, 10, 13, 0, 0, 0, 0, time.FixedZone("CDT", -5*60*60))
	soonEnd := now.In(start.Location()).AddDate(0, 0, 7)
	scope := &ParentReportScope{Timezone: "America/Chicago", PeriodName: "Term 2", PeriodStartDate: "2026-10-13", PeriodEndDate: "2026-12-18", RecentStartAt: start.Format(time.RFC3339), RecentEndAt: now.Format(time.RFC3339), DueSoonEndAt: soonEnd.Format(time.RFC3339)}
	course := func() ParentReportCourse {
		s := "80"
		return ParentReportCourse{Course: "cs8", Timezone: "America/Chicago", CourseScore: &s,
			Missing: []ParentReportItem{{AssignmentID: 1, Title: "Lab <1>", URL: "https://canvas.example.edu/a/1", DueAt: "2026-10-14T10:00:00-05:00", Basis: "canvas_missing"}},
			WorkDue: []ParentWorkDueItem{
				{ParentReportItem: ParentReportItem{AssignmentID: 1, Title: "Lab <1>", URL: "https://canvas.example.edu/a/1", DueAt: "2026-10-14T10:00:00-05:00"}, Status: "missing", Possible: "10"},
				{ParentReportItem: ParentReportItem{AssignmentID: 2, Title: "Completed project", URL: "https://canvas.example.edu/a/2", DueAt: "2026-10-15T10:00:00-05:00"}, Status: "graded", Possible: "10", Current: str("10")},
			},
		}
	}
	report := ParentReportPacket{Schema: 1, Kind: "parent-report", RunID: "R1", Courses: []string{"cs8"}, AsOf: now.Format(time.RFC3339), Scope: scope,
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
	facts := map[string]string{}
	for _, f := range ca.Facts {
		facts[f.Label] = f.Value
	}
	if facts["To"] != "ada@student.example" || facts["Cc"] != "p1@example.org, p3@example.org" || ca.Hold != "" {
		t.Fatalf("facts = %v hold = %q (student in To, enabled parents in Cc, disabled excluded)", facts, ca.Hold)
	}
	if !strings.Contains(ca.Frames[0].HTML, "Lab &lt;1&gt;") || !strings.Contains(ca.Frames[0].HTML, "Completed project") || !strings.Contains(ca.Frames[0].HTML, "Hello about Ada") {
		t.Fatalf("render must escape titles and expand the student name: %s", ca.Frames[0].HTML)
	}
	if !strings.Contains(ca.Frames[0].HTML, "only the nickname “"+a.Alias+".”") {
		t.Fatalf("email must end with the AI notice naming the alias: %s", ca.Frames[0].HTML)
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
		"skip without why":   func(d *ParentDrafts) { d.Students[1] = ParentStudentDraft{Ref: b.Ref, Decision: "skip"} },
	} {
		if err := load(mut); err == nil {
			t.Errorf("%s: expected a validation error", name)
		}
		parentFixtureRewrite(t, svc, dir)
	}
}

func TestLegacyParentRunIsPreservedAndCannotBeSentOrPrinted(t *testing.T) {
	svc, dir, _, _ := parentFixtureRun(t)
	reportPath := filepath.Join(dir, "report.json")
	raw, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatal(err)
	}
	var report ParentReportPacket
	if err = json.Unmarshal(raw, &report); err != nil {
		t.Fatal(err)
	}
	report.Scope = nil
	raw, _ = json.Marshal(report)
	if err = os.WriteFile(reportPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	draftsPath := filepath.Join(dir, "drafts.json")
	outcomesPath := filepath.Join(dir, "outcomes.json")
	if err = saveParentOutcomes(dir, ParentOutcomes{Schema: 2, Students: map[string]ParentDelivery{"s-legacy": {State: "sent", At: "2026-10-16T12:00:00Z"}}}); err != nil {
		t.Fatal(err)
	}
	draftsBefore, _ := os.ReadFile(draftsPath)
	outcomesBefore, _ := os.ReadFile(outcomesPath)
	if _, err = LoadParentRun(svc.Root, dir); err == nil || !strings.Contains(err.Error(), "pull a fresh parent report") {
		t.Fatalf("legacy run error = %v", err)
	}
	if _, _, err = svc.BuildStudentSheets(dir); err == nil || !strings.Contains(err.Error(), "pull a fresh parent report") {
		t.Fatalf("legacy print error = %v", err)
	}
	draftsAfter, _ := os.ReadFile(draftsPath)
	outcomesAfter, _ := os.ReadFile(outcomesPath)
	if string(draftsBefore) != string(draftsAfter) || string(outcomesBefore) != string(outcomesAfter) {
		t.Fatal("legacy run rejection changed saved drafts or delivery outcomes")
	}
}

func TestEmptyRecentInventoryCannotProduceSendDraft(t *testing.T) {
	svc, dir, _, _ := parentFixtureRun(t)
	var report ParentReportPacket
	raw, _ := os.ReadFile(filepath.Join(dir, "report.json"))
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatal(err)
	}
	for i := range report.Students {
		course := &report.Students[i].Courses[0]
		course.WorkDue = []ParentWorkDueItem{}
		course.Missing, course.Corrections, course.History = nil, nil, nil
	}
	raw, _ = json.Marshal(report)
	if err := os.WriteFile(filepath.Join(dir, "report.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	var drafts ParentDrafts
	raw, _ = os.ReadFile(filepath.Join(dir, "drafts.json"))
	if err := json.Unmarshal(raw, &drafts); err != nil {
		t.Fatal(err)
	}
	for i := range drafts.Students {
		drafts.Students[i].Missing = nil
	}
	raw, _ = json.Marshal(drafts)
	if err := os.WriteFile(filepath.Join(dir, "drafts.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadParentRun(svc.Root, dir); err == nil || !strings.Contains(err.Error(), "no work due in this report window") {
		t.Fatalf("empty-window send should request skip or flag: %v", err)
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
		if !strings.Contains(s.HTML, "prepared with AI tools") || !strings.Contains(s.HTML, "only your nickname, “") {
			t.Fatalf("AI notice missing: %s", s.HTML)
		}
		if !strings.Contains(s.HTML, ">Missing/0</h3>") || !strings.Contains(s.HTML, "Lab &lt;1&gt;") || !strings.Contains(s.HTML, "Completed project") {
			t.Fatalf("sheet = %s", s.HTML)
		}
	}
}

func TestParentRenderMissingZeroFirstAndKeepsHistory(t *testing.T) {
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	as := map[int64]Assignment{
		1: {ID: 1, Name: "Submitted zero", Published: true, DueAt: str("2026-10-15T12:00:00Z"), Points: "10"},
		2: {ID: 2, Name: "Positive correction", Published: true, DueAt: str("2026-10-15T12:00:00Z"), Points: "10"},
	}
	course := ParentReportCourse{Course: "TEST", Timezone: "America/Chicago"}
	buildStudentReport(&course, []Submission{
		{AssignmentID: 1, Attempt: 1, SubmittedAt: str("2026-10-15T12:00:00Z"), Score: num("0"), History: []Submission{{Attempt: 1, Score: num("0")}}},
		{AssignmentID: 2, Score: num("8")},
	}, nil, as, now, time.UTC)
	rs := ParentReportStudent{Ref: "s-1", Alias: "Alias", Courses: []ParentReportCourse{course}}
	run := &ParentRun{Report: ParentReportPacket{AsOf: now.Format(time.RFC3339), Scope: parentTestScope(now)}, students: map[string]ParentReportStudent{rs.Ref: rs}}
	sd := ParentStudentDraft{Ref: rs.Ref, Decision: "send", Missing: []ParentItemRef{{AssignmentID: 1}}, Corrections: []ParentItemRef{{AssignmentID: 2}}, History: []ParentItemRef{{AssignmentID: 1}}}
	student := workspace.Student{Name: "Ada Lovelace", FirstName: "Ada"}
	for name, rendered := range map[string]Rendered{"email": run.Render(sd, student), "student sheet": run.RenderStudent(sd, student)} {
		html := rendered.HTML
		missing := strings.Index(html, ">Missing/0</h3>")
		corrections := strings.Index(html, ">Corrections")
		history := strings.Index(html, ">Work due in this period</h3>")
		if missing < 0 || corrections <= missing || history <= corrections {
			t.Fatalf("%s: expected Missing/0, corrections, history order: %s", name, html)
		}
		if strings.Contains(html, "turn these in") || strings.Contains(html, "not turned in") || strings.Contains(html, "Nothing is missing") {
			t.Fatalf("%s: a submitted zero must not be described as unsubmitted: %s", name, html)
		}
		if strings.Contains(html[corrections:history], "Submitted zero") || !strings.Contains(html[:corrections], "Submitted zero") || !strings.Contains(html[history:], "Submitted zero") {
			t.Fatalf("%s: zero must appear in Missing/0 and history, without a correction duplicate: %s", name, html)
		}
	}
	// Existing section-membership validation rejects an agent selecting a
	// Missing/0 item as a correction, rather than inventing another policy gate.
	run.Report.RunID, run.Report.Students, run.Report.Scope = "R1", []ParentReportStudent{rs}, parentTestScope(now)
	sd.Corrections = []ParentItemRef{{AssignmentID: 1}}
	run.Drafts = ParentDrafts{Schema: 1, Kind: "parent-drafts", RunID: "R1", Students: []ParentStudentDraft{sd}}
	if err := run.validate(); err == nil || !strings.Contains(err.Error(), "report corrections list") {
		t.Fatalf("Missing/0 selected as correction must fail membership validation: %v", err)
	}
}

func parentTestScope(asOf time.Time) *ParentReportScope {
	loc, _ := time.LoadLocation("America/Chicago")
	start := time.Date(2026, 10, 13, 0, 0, 0, 0, loc)
	end := time.Date(2026, 12, 18, 0, 0, 0, 0, loc)
	recent := asOf.In(loc).AddDate(0, 0, -correctionWindowDays)
	if start.After(recent) {
		recent = start
	}
	soon := asOf.In(loc).AddDate(0, 0, dueSoonDays)
	if end.AddDate(0, 0, 1).Before(soon) {
		soon = end.AddDate(0, 0, 1)
	}
	return &ParentReportScope{Timezone: "America/Chicago", PeriodName: "Term 2", PeriodStartDate: "2026-10-13", PeriodEndDate: "2026-12-18", RecentStartAt: recent.Format(time.RFC3339), RecentEndAt: asOf.Format(time.RFC3339), DueSoonEndAt: soon.Format(time.RFC3339)}
}

func TestLoadParentRunRecomputesAdvisorySkipEligibility(t *testing.T) {
	svc, dir, _, _ := parentFixtureRun(t)
	var report ParentReportPacket
	raw, err := os.ReadFile(filepath.Join(dir, "report.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &report); err != nil {
		t.Fatal(err)
	}
	for i := range report.Students {
		report.Students[i].SkipEligible = true // stale metadata must be ignored
		report.Students[i].Courses[0].CourseScore = str("89")
	}
	report.Students[0].Courses[0].Missing = nil
	raw, _ = json.Marshal(report)
	if err = os.WriteFile(filepath.Join(dir, "report.json"), raw, 0644); err != nil {
		t.Fatal(err)
	}
	drafts := ParentDrafts{Schema: 1, Kind: "parent-drafts", RunID: "R1"}
	for _, student := range report.Students {
		drafts.Students = append(drafts.Students, ParentStudentDraft{Ref: student.Ref, Decision: "skip", Reason: "Teacher requested no email"})
	}
	raw, _ = json.Marshal(drafts)
	if err = os.WriteFile(filepath.Join(dir, "drafts.json"), raw, 0644); err != nil {
		t.Fatal(err)
	}
	run, err := LoadParentRun(svc.Root, dir)
	if err != nil {
		t.Fatal(err)
	}
	if run.Config.SkipScore != 90 || run.Report.Students[0].SkipEligible || run.Report.Students[1].SkipEligible {
		t.Fatalf("default threshold must replace stale true eligibility: %+v", run.Report.Students)
	}
	if err = workspace.WriteTeacherDocument(svc.Root, "context/parent-updates.md", ParentConfig{Schema: 1, SkipScore: 85}, ""); err != nil {
		t.Fatal(err)
	}
	run, err = LoadParentRun(svc.Root, dir)
	if err != nil {
		t.Fatal(err)
	}
	if !run.Report.Students[0].SkipEligible || run.Report.Students[1].SkipEligible || !run.students[report.Students[0].Ref].SkipEligible {
		t.Fatalf("custom threshold still requires no Missing/0: %+v", run.Report.Students)
	}
	if run.Drafts.Students[0].Decision != "skip" || run.Drafts.Students[1].Decision != "skip" {
		t.Fatal("eligibility metadata must preserve the teacher's decisions")
	}
}

func TestSkippedStudentGetsOnTrackSheetAndOnlyChosenCorrections(t *testing.T) {
	s := "96"
	now := time.Date(2026, 10, 16, 15, 0, 0, 0, time.UTC)
	first, current := "70.0", "90.0"
	rs := ParentReportStudent{Ref: "s-1", Courses: []ParentReportCourse{{Course: "cs8", Timezone: "America/Chicago", CourseScore: &s,
		Corrections: []ParentCorrection{{ParentReportItem: ParentReportItem{AssignmentID: 5, Title: "Quiz needing judgment"}}},
		WorkDue:     []ParentWorkDueItem{{ParentReportItem: ParentReportItem{AssignmentID: 6, Title: "Lab 6", URL: "u6", DueAt: "2026-10-15T18:00:00-05:00"}, Status: "graded", Possible: "100.0", First: &first, Current: &current}},
	}}}
	run := &ParentRun{Report: ParentReportPacket{AsOf: now.Format(time.RFC3339), Scope: parentTestScope(now), Students: []ParentReportStudent{rs}}, students: map[string]ParentReportStudent{"s-1": rs}}
	got := run.RenderStudent(ParentStudentDraft{Ref: "s-1", Decision: "skip", Reason: "on track"}, workspace.Student{Name: "Ada Lovelace"}).HTML
	for _, want := range []string{"Ada Lovelace", "No Missing/0 work in this report window.", "Work due in this period", "70 / 100", "90 / 100"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %s", want, got)
		}
	}
	if strings.Contains(got, "Quiz needing judgment") {
		t.Fatal("a correction the agent did not choose appeared")
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
