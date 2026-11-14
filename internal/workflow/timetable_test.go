package workflow

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

func timetableCalendar(closures ...string) workspace.SchoolCalendar {
	return workspace.SchoolCalendar{Schema: 1, Timezone: "America/Chicago", StartDate: "2026-08-19", EndDate: "2027-05-27", NonSchoolDates: closures}
}

var defaultLatePolicy = &CanvasLatePolicy{Known: true, DeductionEnabled: true, Deduction: 10, Interval: "day", MinimumEnabled: true, Minimum: 30}

func TestTimetableCountsSchoolDaysForRegularAndExtraTime(t *testing.T) {
	// Thu 11/05 due: extra time lands on Fri 11/06, and anything a school day
	// after that skips the weekend to Monday.
	tt, err := buildTimetable(timetableInput{
		Due: mustTime(t, "2026-11-05T23:59:00-06:00"), Points: domain.MustDecimal("100"), Calendar: timetableCalendar(),
		Policy: domain.LateWorkPolicy{CorrectionSchoolDays: four(), CloseAfterSchoolDays: 11, ExtraTimeSchoolDays: 1}, Late: defaultLatePolicy, ProposeLock: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"Due: Thu 11/05 (extra time: Fri 11/06)",
		"Late penalty starts: Fri 11/06 (extra time: Mon 11/09) — −10 points per school day late; lowest late grade 30 points",
		"Last day for corrections: Wed 11/11 (extra time: Thu 11/12)",
		"Assignment closes: Fri 11/20 (extra time: Mon 11/23)",
	}
	if got := tt.Lines(); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("timetable:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	// The Engine locks at the extra-time close so no flagged student is locked out.
	if tt.DefaultLock == nil || tt.DefaultLock.Format(time.RFC3339) != "2026-11-23T23:59:00-06:00" {
		t.Fatalf("default lock: %v", tt.DefaultLock)
	}
	html := tt.HTML()
	if !strings.Contains(html, timetableMarker) || !strings.HasPrefix(html, "<details") || !hasTimetable(html) {
		t.Fatalf("html block: %s", html)
	}
}

func TestTimetableSkipsClosuresAndRespectsTeacherChoices(t *testing.T) {
	due := mustTime(t, "2026-11-05T23:59:00-06:00")
	policy := domain.LateWorkPolicy{CorrectionSchoolDays: four(), CloseAfterSchoolDays: 11}
	tt, err := buildTimetable(timetableInput{Due: due, Points: domain.MustDecimal("50"), Calendar: timetableCalendar("2026-11-10"), Policy: policy, Late: &CanvasLatePolicy{Known: true, DeductionEnabled: true, Deduction: 5, Interval: "day"}, ProposeLock: true})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Join(tt.Lines(), "\n")
	for _, want := range []string{"Due: Thu 11/05\n", "−5% per school day late", "Last day for corrections: Thu 11/12", "Assignment closes: Mon 11/23"} {
		if !strings.Contains(lines, want) {
			t.Fatalf("missing %q in\n%s", want, lines)
		}
	}
	if strings.Contains(lines, "extra time") {
		t.Fatalf("no allowance configured, but extra-time dates shown:\n%s", lines)
	}

	// A teacher lock date is shown as it is; the Engine proposes nothing.
	lock := mustTime(t, "2026-12-01T23:59:00-06:00")
	tt, err = buildTimetable(timetableInput{Due: due, LockAt: &lock, Points: domain.MustDecimal("100"), Calendar: timetableCalendar(), Policy: policy, Late: &CanvasLatePolicy{Known: true}})
	if err != nil {
		t.Fatal(err)
	}
	lines = strings.Join(tt.Lines(), "\n")
	if tt.DefaultLock != nil || !strings.Contains(lines, "Assignment closes: Tue 12/01") || strings.Contains(lines, "penalty") {
		t.Fatalf("teacher lock or no-deduction course: %v\n%s", tt.DefaultLock, lines)
	}

	// An assignment with no lock date in Canvas has nothing that closes.
	tt, _ = buildTimetable(timetableInput{Due: due, Points: domain.MustDecimal("100"), Calendar: timetableCalendar(), Policy: policy, Late: &CanvasLatePolicy{}})
	lines = strings.Join(tt.Lines(), "\n")
	if strings.Contains(lines, "closes") || !strings.Contains(lines, "could not be read") {
		t.Fatalf("unlocked update or unread policy:\n%s", lines)
	}

	// A calendar that ends too early stops the timetable rather than guessing.
	short := timetableCalendar()
	short.EndDate = "2026-11-13"
	if _, err = buildTimetable(timetableInput{Due: due, Calendar: short, Policy: policy, Late: defaultLatePolicy, ProposeLock: true}); err == nil {
		t.Fatal("calendar gap was not reported")
	}
}

func TestWithTimetableReplacesAnEarlierBlock(t *testing.T) {
	old := `<p>Write.</p>` + "\n" + `<details data-cse="grading-timetable"><summary>Old</summary><ul><li>Due: Mon 1/01</li></ul></details>`
	got := withTimetable(old, `<details data-cse="grading-timetable"><summary>New</summary></details>`)
	if strings.Count(got, "<details") != 1 || !strings.Contains(got, "New") || !strings.HasPrefix(got, "<p>Write.</p>") {
		t.Fatalf("block not replaced: %s", got)
	}
}

func TestPushCreateAddsTimetableAndUpdateKeepsItCurrent(t *testing.T) {
	live := Assignment{}
	creates, policyReads := 0, 0
	var updated map[string]json.RawMessage
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/late_policy":
			policyReads++
			_, _ = w.Write([]byte(`{"late_policy":{"late_submission_deduction_enabled":true,"late_submission_deduction":10.0,"late_submission_interval":"day","late_submission_minimum_percent_enabled":true,"late_submission_minimum_percent":30.0}}`))
		case r.Method == "POST" && r.URL.Path == "/api/v1/courses/1/assignments":
			creates++
			var payload struct {
				Assignment Assignment `json:"assignment"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			live = payload.Assignment
			live.ID, live.CourseID = 10, 1
			_ = json.NewEncoder(w).Encode(live)
		case r.Method == "PUT" && r.URL.Path == "/api/v1/courses/1/assignments/10":
			var payload struct {
				Assignment map[string]json.RawMessage `json:"assignment"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			updated = payload.Assignment
			_ = json.Unmarshal(payload.Assignment["description"], &live.Description)
			var due, lock string
			_ = json.Unmarshal(payload.Assignment["due_at"], &due)
			_ = json.Unmarshal(payload.Assignment["lock_at"], &lock)
			live.DueAt, live.LockAt = &due, &lock
			_ = json.NewEncoder(w).Encode(live)
		case r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/assignments/10":
			_ = json.NewEncoder(w).Encode(live)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.String())
			http.Error(w, "unexpected request", http.StatusBadRequest)
		}
	})
	if err := workspace.WriteTeacherDocument(s.Root, "context/school-calendar.md", timetableCalendar(), "Synthetic calendar"); err != nil {
		t.Fatal(err)
	}
	grading := domain.GradingConfig{Schema: 1, LateWork: &domain.LateWorkPolicy{CorrectionSchoolDays: four(), CloseAfterSchoolDays: 11, ExtraTimeSchoolDays: 1}}
	if err := workspace.WriteTeacherDocument(s.Root, "context/grading.md", grading, "Synthetic grading rules"); err != nil {
		t.Fatal(err)
	}
	due := "2026-11-05T23:59:00-06:00"
	doc := domain.AssignmentDocument{Schema: 1, Kind: "assignment", Operation: "create", Course: "TEST", Title: "Timetabled", Points: domain.MustDecimal("100"), SubmissionTypes: []string{"online_text_entry"}, DueAt: &due}
	path := filepath.Join(s.Root, "drafts", "timetabled.md")
	if err := workspace.WriteDocument(path, doc, "Write your answer."); err != nil {
		t.Fatal(err)
	}
	preview := strings.Join(PreviewTimetable(s.Root, doc), "\n")
	if !strings.Contains(preview, "Assignment closes: Fri 11/20 (extra time: Mon 11/23)") || !strings.Contains(preview, "lock_at will be set") {
		t.Fatalf("preview:\n%s", preview)
	}
	report, err := s.Push(context.Background(), path)
	if err != nil || report.Status != "completed" || creates != 1 || policyReads != 1 {
		t.Fatalf("create: %+v %v creates=%d reads=%d", report, err, creates, policyReads)
	}
	if !strings.HasSuffix(strings.TrimSpace(live.Description), "</details>") || !strings.Contains(live.Description, "lowest late grade 30 points") {
		t.Fatalf("timetable not at the end of the body: %s", live.Description)
	}
	if live.LockAt == nil || !sameDate(live.LockAt, ptr("2026-11-23T23:59:00-06:00")) {
		t.Fatalf("lock_at: %v", live.LockAt)
	}
	if !strings.Contains(report.Timetable, "lock_at set") {
		t.Fatalf("report note: %q", report.Timetable)
	}

	// Moving the due date a week regenerates the block and moves the Engine's lock date.
	pulled, err := s.PullItem(context.Background(), "TEST", "10")
	if err != nil {
		t.Fatal(err)
	}
	var edit domain.AssignmentDocument
	body, err := workspace.ReadDocument(pulled, &edit)
	if err != nil {
		t.Fatal(err)
	}
	moved := "2026-11-12T23:59:00-06:00"
	edit.DueAt = &moved
	if err = workspace.WriteDocument(pulled, edit, body); err != nil {
		t.Fatal(err)
	}
	report, err = s.Push(context.Background(), pulled)
	if err != nil || report.Status != "succeeded" {
		t.Fatalf("update: %+v %v", report, err)
	}
	if _, ok := updated["lock_at"]; !ok || !sameDate(live.LockAt, ptr("2026-11-30T23:59:00-06:00")) {
		t.Fatalf("lock not moved with due date: %v %v", updated, live.LockAt)
	}
	if strings.Count(live.Description, "<details") != 1 || !strings.Contains(live.Description, "<strong>Due:</strong> Thu 11/12 (extra time: Fri 11/13)") {
		t.Fatalf("timetable not refreshed: %s", live.Description)
	}
}

func ptr(v string) *string { return &v }

func TestTimetableLateWorkCutoff(t *testing.T) {
	due := mustTime(t, "2026-11-05T23:59:00-06:00") // Thursday
	three, zero := 3, 0
	policy := domain.LateWorkPolicy{CorrectionSchoolDays: four(), CloseAfterSchoolDays: 11, ExtraTimeSchoolDays: 1, LateCutoffSchoolDays: &three}
	tt, err := buildTimetable(timetableInput{Due: due, Points: domain.MustDecimal("100"), Calendar: timetableCalendar(), Policy: policy, Late: defaultLatePolicy, ProposeLock: true})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Join(tt.Lines(), "\n")
	// Thu + 3 school days = Tue 11/10 (Fri, Mon, Tue).
	if !strings.Contains(lines, "Late work accepted through: Tue 11/10") {
		t.Fatalf("cutoff row missing:\n%s", lines)
	}
	if tt.DefaultLock == nil || tt.DefaultLock.Format(time.RFC3339) != "2026-11-10T23:59:00-06:00" {
		t.Fatalf("lock should be the regular cutoff, not the extra-time close: %v", tt.DefaultLock)
	}

	policy.LateCutoffSchoolDays = &zero
	tt, err = buildTimetable(timetableInput{Due: due, Points: domain.MustDecimal("100"), Calendar: timetableCalendar(), Policy: policy, Late: defaultLatePolicy, ProposeLock: true})
	if err != nil {
		t.Fatal(err)
	}
	lines = strings.Join(tt.Lines(), "\n")
	if !strings.Contains(lines, "Late work: not accepted after the due date") || strings.Contains(lines, "penalty") {
		t.Fatalf("no-late-work course:\n%s", lines)
	}
	if tt.DefaultLock == nil || tt.DefaultLock.Format(time.RFC3339) != "2026-11-05T23:59:00-06:00" {
		t.Fatalf("lock should be the due date: %v", tt.DefaultLock)
	}

	// A teacher-set lock date stays as written.
	lock := mustTime(t, "2026-12-01T23:59:00-06:00")
	tt, err = buildTimetable(timetableInput{Due: due, LockAt: &lock, Points: domain.MustDecimal("100"), Calendar: timetableCalendar(), Policy: domain.LateWorkPolicy{LateCutoffSchoolDays: &three}, Late: defaultLatePolicy})
	if err != nil {
		t.Fatal(err)
	}
	if tt.DefaultLock != nil {
		t.Fatalf("Engine proposed a lock over the teacher's: %v", tt.DefaultLock)
	}
}

func TestCourseLateWorkLineOverridesGradingMd(t *testing.T) {
	two, five := 2, 5
	base := &domain.LateWorkPolicy{LateCutoffSchoolDays: &five, CorrectionSchoolDays: four()}
	got := base.Merged(&domain.LateWorkPolicy{LateCutoffSchoolDays: &two})
	if got.LateCutoffSchoolDays == nil || *got.LateCutoffSchoolDays != 2 || got.CorrectionWindow() != 4 {
		t.Fatalf("merged=%+v", got)
	}
}

func TestTimetableOmitCorrectionsLeavesOtherRowsAlone(t *testing.T) {
	in := timetableInput{
		Due: mustTime(t, "2026-11-05T23:59:00-06:00"), Points: domain.MustDecimal("100"), Calendar: timetableCalendar(),
		Policy: domain.LateWorkPolicy{CorrectionSchoolDays: four(), CloseAfterSchoolDays: 11}, Late: defaultLatePolicy, ProposeLock: true,
	}
	with, err := buildTimetable(in)
	if err != nil {
		t.Fatal(err)
	}
	in.OmitCorrections = true
	without, err := buildTimetable(in)
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, line := range with.Lines() {
		if !strings.Contains(line, "Last day for corrections") {
			want = append(want, line)
		}
	}
	if strings.Join(without.Lines(), "\n") != strings.Join(want, "\n") || len(want) != len(with.Lines())-1 {
		t.Fatalf("omit changed other rows:\n%v\nwant:\n%v", without.Lines(), want)
	}
	html := without.HTML()
	if strings.Contains(html, "orrections") || !hasTimetable(html) {
		t.Fatalf("html still talks about corrections: %s", html)
	}
	if !strings.Contains(with.HTML(), "Corrections are accepted through the last day for corrections") || strings.Contains(with.HTML(), "as soon as you finish") {
		t.Fatalf("configured window should describe only its dates: %s", with.HTML())
	}
}

func four() *int { n := 4; return &n }

func TestTimetableWithoutCorrectionWindowSaysNothingAboutCorrections(t *testing.T) {
	three, zero := 3, 0
	due := mustTime(t, "2026-11-05T23:59:00-06:00")
	for _, policy := range []domain.LateWorkPolicy{
		{LateCutoffSchoolDays: &three, ExtraTimeSchoolDays: 1},
		{CloseAfterSchoolDays: 11},
		{CloseAfterSchoolDays: 11, CorrectionSchoolDays: &zero},
	} {
		tt, err := buildTimetable(timetableInput{Due: due, Points: domain.MustDecimal("100"), Calendar: timetableCalendar(), Policy: policy, Late: defaultLatePolicy, ProposeLock: true})
		if err != nil {
			t.Fatal(err)
		}
		html := tt.HTML()
		if !tt.NoCorrections || strings.Contains(html, "orrections") || !strings.Contains(html, "(late work)") || !strings.Contains(html, "Due:") {
			t.Fatalf("policy %+v offered corrections: %s", policy, html)
		}
	}
}

func TestCourseCorrectionWindowOverridesGradingMd(t *testing.T) {
	zero, six := 0, 6
	global := &domain.LateWorkPolicy{CorrectionSchoolDays: four(), CloseAfterSchoolDays: 11}
	if got := global.Merged(&domain.LateWorkPolicy{CorrectionSchoolDays: &zero}); got.CorrectionWindow() != 0 || got.CloseAfterSchoolDays != 11 {
		t.Fatalf("course 0 should turn the window off: %+v", got)
	}
	if got := global.Merged(&domain.LateWorkPolicy{CorrectionSchoolDays: &six}); got.CorrectionWindow() != 6 {
		t.Fatalf("course window: %+v", got)
	}
	if got := global.Merged(&domain.LateWorkPolicy{CloseAfterSchoolDays: 5}); got.CorrectionWindow() != 4 {
		t.Fatalf("an unset course line keeps the shared window: %+v", got)
	}
	var none *domain.LateWorkPolicy
	if none.CorrectionWindow() != 0 || none.Merged(&domain.LateWorkPolicy{CloseAfterSchoolDays: 5}).CorrectionWindow() != 0 {
		t.Fatal("no policy is no window")
	}
}

// oldTimetable is a block as Engine versions before 2026-11-13 wrote it.
const oldTimetable = `<details data-cse="grading-timetable"><summary><strong>Grading timetable</strong> (late work and corrections)</summary><ul><li><strong>Due:</strong> Thu 11/05</li><li><strong>Last day for corrections:</strong> Wed 11/11</li></ul><p>Dates count school days only. Corrections are available as soon as you finish; check your feedback for rationales and exemplars.</p></details>`

func TestWithoutCorrectionOfferTouchesOnlyTheTimetable(t *testing.T) {
	body := "<p>Teacher note: corrections are discussed in class.</p>\n" + oldTimetable
	got := withoutCorrectionOffer(body)
	block := timetableBlock.FindString(got)
	if strings.Contains(block, "orrections") || !strings.Contains(block, "(late work)") || !strings.Contains(block, "Due:</strong> Thu 11/05") || !strings.Contains(block, "Dates count school days only.</p>") {
		t.Fatalf("block: %s", block)
	}
	if !strings.HasPrefix(got, "<p>Teacher note: corrections are discussed in class.</p>") {
		t.Fatalf("teacher text changed: %s", got)
	}
	if !correctionsLeftOut(got) || correctionsLeftOut(body) || correctionsLeftOut("<p>No timetable.</p>") {
		t.Fatal("correctionsLeftOut reads the timetable's corrections row")
	}
}

// A copied CE timetable keeps no correction promise in a course without a
// window, on create (no new timetable made) and on a later edit.
func TestCopiedTimetableLosesUnauthorizedCorrectionOffer(t *testing.T) {
	live := Assignment{}
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "POST" && r.URL.Path == "/api/v1/courses/1/assignments":
			var payload struct {
				Assignment Assignment `json:"assignment"`
			}
			_ = json.NewDecoder(r.Body).Decode(&payload)
			live = payload.Assignment
			live.ID, live.CourseID = 10, 1
			_ = json.NewEncoder(w).Encode(live)
		case r.Method == "PUT" && r.URL.Path == "/api/v1/courses/1/assignments/10":
			var payload struct {
				Assignment map[string]json.RawMessage `json:"assignment"`
			}
			_ = json.NewDecoder(r.Body).Decode(&payload)
			if d, ok := payload.Assignment["description"]; ok {
				_ = json.Unmarshal(d, &live.Description)
			}
			if n, ok := payload.Assignment["name"]; ok {
				_ = json.Unmarshal(n, &live.Name)
			}
			_ = json.NewEncoder(w).Encode(live)
		case r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/assignments/10":
			_ = json.NewEncoder(w).Encode(live)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.String())
			http.Error(w, "unexpected request", http.StatusBadRequest)
		}
	})
	due := "2026-11-05T23:59:00-06:00"
	doc := domain.AssignmentDocument{Schema: 1, Kind: "assignment", Operation: "create", Course: "TEST", Title: "Copied", BodyFormat: "html", Points: domain.MustDecimal("100"), SubmissionTypes: []string{"online_text_entry"}, DueAt: &due}
	path := filepath.Join(s.Root, "drafts", "copied.md")
	if err := workspace.WriteDocument(path, doc, "<p>Write.</p>\n"+oldTimetable); err != nil {
		t.Fatal(err)
	}
	report, err := s.Push(context.Background(), path)
	if err != nil || report.Status != "completed" {
		t.Fatalf("create: %+v %v", report, err)
	}
	if strings.Contains(live.Description, "orrections") || !strings.Contains(live.Description, "<p>Write.</p>") || !hasTimetable(live.Description) || !strings.Contains(report.Timetable, "correction lines removed") {
		t.Fatalf("create kept the promise: %s (%q)", live.Description, report.Timetable)
	}
	// A resumed create sends the same recorded body.
	if c := report.Resources; c == nil || c.Timetable == nil || !c.Timetable.DropCorrections || c.IntendedHTML != live.Description {
		t.Fatalf("record: %+v", c)
	}

	// An assignment already in Canvas with the old block: an ordinary edit
	// (dates unchanged) drops the promise and keeps the teacher's text.
	live.Description = "<p>Write.</p>\n" + oldTimetable
	pulled, err := s.PullItem(context.Background(), "TEST", "10")
	if err != nil {
		t.Fatal(err)
	}
	var edit domain.AssignmentDocument
	body, err := workspace.ReadDocument(pulled, &edit)
	if err != nil {
		t.Fatal(err)
	}
	edit.Title = "Copied, renamed"
	if err = workspace.WriteDocument(pulled, edit, body); err != nil {
		t.Fatal(err)
	}
	report, err = s.Push(context.Background(), pulled)
	if err != nil || report.Status != "succeeded" {
		t.Fatalf("update: %+v %v", report, err)
	}
	if strings.Contains(live.Description, "orrections") || !strings.Contains(live.Description, "<p>Write.</p>") || !strings.Contains(report.Timetable, "correction lines removed") {
		t.Fatalf("update kept the promise: %s (%q)", live.Description, report.Timetable)
	}
}
