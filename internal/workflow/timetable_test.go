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
		Policy: domain.LateWorkPolicy{CorrectionSchoolDays: 4, CloseAfterSchoolDays: 11, ExtraTimeSchoolDays: 1}, Late: defaultLatePolicy, ProposeLock: true,
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
	policy := domain.LateWorkPolicy{CorrectionSchoolDays: 4, CloseAfterSchoolDays: 11}
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
	grading := domain.GradingConfig{Schema: 1, LateWork: &domain.LateWorkPolicy{CorrectionSchoolDays: 4, CloseAfterSchoolDays: 11, ExtraTimeSchoolDays: 1}}
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
