package workflow

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"canvasexpertse/internal/workspace"
)

func num(s string) *json.Number { n := json.Number(s); return &n }
func str(s string) *string      { return &s }

func TestParentWindowUsesEffectiveDueDatesAndCompleteInventory(t *testing.T) {
	loc, _ := time.LoadLocation("America/Chicago")
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, loc)
	start := time.Date(2026, 10, 13, 0, 0, 0, 0, loc)
	assignments := map[int64]Assignment{
		1: {ID: 1, Name: "Term 1 zero", Published: true, DueAt: str("2026-10-08T16:00:00-05:00"), Points: "10"},
		2: {ID: 2, Name: "Recent regrade, old due", Published: true, DueAt: str("2026-10-12T16:00:00-05:00"), Points: "10"},
		3: {ID: 3, Name: "Window first day", Published: true, DueAt: str("2026-10-13T16:00:00-05:00"), Points: "10"},
		4: {ID: 4, Name: "Perfect", Published: true, DueAt: str("2026-10-17T16:00:00-05:00"), Points: "10"},
		5: {ID: 5, Name: "Awaiting grade", Published: true, DueAt: str("2026-10-18T11:00:00-05:00"), Points: "10"},
		6: {ID: 6, Name: "Required ungraded", Published: true, GradingType: "not_graded", DueAt: str("2026-10-17T16:00:00-05:00")},
		7: {ID: 7, Name: "Due later", Published: true, DueAt: str("2026-10-24T16:00:00-05:00"), Points: "10"},
		8: {ID: 8, Name: "Outside term", Published: true, DueAt: str("2026-12-19T16:00:00-06:00"), Points: "10"},
	}
	var undated *string
	allDates := []AssignmentAllDateLive{{UserID: 9, DueAt: undated}}
	assignments[9] = Assignment{ID: 9, Name: "Undated override", Published: true, DueAt: str("2026-10-17T16:00:00-05:00"), AllDates: &allDates, Points: "10"}
	course := ParentReportCourse{Course: "CS8", Timezone: "America/Chicago", WorkDue: []ParentWorkDueItem{}}
	subs := []Submission{
		{UserID: 9, AssignmentID: 1, AssignmentVisible: boolPtr(true), Score: num("0"), Attempt: 1},
		{UserID: 9, AssignmentID: 2, AssignmentVisible: boolPtr(true), Score: num("0"), Attempt: 1, GradedAt: str(now.Format(time.RFC3339))},
		{UserID: 9, AssignmentID: 3, AssignmentVisible: boolPtr(true), Score: num("0"), Attempt: 1},
		{UserID: 9, AssignmentID: 4, AssignmentVisible: boolPtr(true), Score: num("10"), Attempt: 1, SubmittedAt: str("2026-10-17T15:00:00Z")},
		{UserID: 9, AssignmentID: 5, AssignmentVisible: boolPtr(true), Attempt: 1, SubmittedAt: str("2026-10-18T16:00:00Z")},
		{UserID: 9, AssignmentID: 6, AssignmentVisible: boolPtr(true)},
		{UserID: 9, AssignmentID: 7, AssignmentVisible: boolPtr(true)},
		{UserID: 9, AssignmentID: 9, AssignmentVisible: boolPtr(true)},
	}
	buildStudentReport(&course, subs, assignments, now, loc, start)
	got := map[int64]ParentWorkDueItem{}
	for _, item := range course.WorkDue {
		got[item.AssignmentID] = item
	}
	if len(got) != 4 || got[3].Status != "graded" || *got[3].Percent != "0" || got[4].Status != "graded" || *got[4].Percent != "100" || got[5].Status != "awaiting_grading" || got[6].Status != "no_grade" || got[6].Percent != nil {
		t.Fatalf("work_due = %+v", got)
	}
	for _, id := range []int64{1, 2, 7, 8, 9} {
		if _, ok := got[id]; ok {
			t.Errorf("assignment %d should not be in the term inventory", id)
		}
	}
}

func TestParentInventoryWithholdsExplicitlyUnreleasedGrade(t *testing.T) {
	loc, _ := time.LoadLocation("America/Chicago")
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, loc)
	a := Assignment{ID: 1, Name: "Private grade", Published: true, DueAt: str("2026-10-17T12:00:00-05:00"), Points: "10", SubmissionTypes: []string{"online_upload"}}
	sub := Submission{UserID: 9, AssignmentID: 1, AssignmentVisible: boolPtr(true), Attempt: 1, Score: num("0"), EnteredScore: num("8"), GradedAt: str("2026-10-18T10:00:00-05:00"), PostedAtPresent: true, Comments: []SubmissionComment{{AuthorID: 77, Comment: "Unreleased feedback"}}}
	course := ParentReportCourse{WorkDue: []ParentWorkDueItem{}}
	buildStudentReport(&course, []Submission{sub}, map[int64]Assignment{1: a}, now, loc, now.AddDate(0, 0, -14))
	if len(course.WorkDue) != 1 {
		t.Fatalf("inventory = %+v", course)
	}
	item := course.WorkDue[0]
	if item.Status != "grade_not_released" || item.Current != nil || item.Percent != nil || item.Possible != "" {
		t.Fatalf("unreleased grade inventory = %+v", item)
	}
	if raw, _ := json.Marshal(course); strings.Contains(string(raw), "Unreleased feedback") {
		t.Fatalf("unreleased feedback leaked: %s", raw)
	}
}

func TestParentScopeAutomaticallySelectsAllTermsAndClipsUpcomingWork(t *testing.T) {
	calendar := workspace.SchoolCalendar{Schema: 1, Timezone: "America/Chicago", StartDate: "2026-08-19", EndDate: "2027-05-27", GradingPeriods: []workspace.GradingPeriod{
		{Name: "Term 1", StartDate: "2026-08-19", EndDate: "2026-10-08"},
		{Name: "Term 2", StartDate: "2026-10-13", EndDate: "2026-12-18"},
		{Name: "Term 3", StartDate: "2027-01-06", EndDate: "2027-03-19"},
		{Name: "Term 4", StartDate: "2027-03-22", EndDate: "2027-05-27"},
	}}
	loc, _ := time.LoadLocation(calendar.Timezone)
	for _, tc := range []struct{ name, start, end, nearEnd string }{
		{"Term 1", "2026-08-19", "2026-10-08", "2026-10-07"},
		{"Term 2", "2026-10-13", "2026-12-18", "2026-12-17"},
		{"Term 3", "2027-01-06", "2027-03-19", "2027-03-18"},
		{"Term 4", "2027-03-22", "2027-05-27", "2027-05-26"},
	} {
		startAsOf, _ := time.ParseInLocation("2006-01-02", tc.start, loc)
		period, err := calendar.CurrentPeriod(startAsOf)
		if err != nil || period.Name != tc.name {
			t.Fatalf("start date %s selected %q, err=%v", tc.start, period.Name, err)
		}
		scope, recent, _, err := makeParentReportScope(calendar, period, startAsOf.Add(12*time.Hour))
		if err != nil || scope.PeriodName != tc.name || !recent.Equal(startAsOf) {
			t.Fatalf("%s start scope=%+v recent=%s err=%v", tc.name, scope, recent, err)
		}
		nearEnd, _ := time.ParseInLocation("2006-01-02", tc.nearEnd, loc)
		endScopePeriod, err := calendar.CurrentPeriod(nearEnd)
		if err != nil {
			t.Fatal(err)
		}
		endScope, windowStart, upcomingEnd, err := makeParentReportScope(calendar, endScopePeriod, nearEnd.Add(12*time.Hour))
		wantEnd := nearEnd.AddDate(0, 0, 2) // midnight immediately after the inclusive period end.
		if err != nil || endScope.PeriodName != tc.name || !upcomingEnd.Equal(wantEnd) || !windowStart.Equal(startAsOf) {
			t.Fatalf("%s window=%s upcoming boundary=%s want=%s scope=%+v err=%v", tc.name, windowStart, upcomingEnd, wantEnd, endScope, err)
		}
	}
	gap, _ := time.ParseInLocation(time.RFC3339, "2027-01-01T12:00:00-06:00", loc)
	if _, err := calendar.CurrentPeriod(gap); err == nil {
		t.Fatal("winter break gap must not select the previous grading period")
	}
}

func TestCompleteStudentSubmissionsResolvesMissingVisibilityRow(t *testing.T) {
	svc, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		includes := r.URL.Query()["include[]"]
		hasVisibility := false
		for _, include := range includes {
			hasVisibility = hasVisibility || include == "visibility"
		}
		if r.URL.Path != "/api/v1/courses/1/assignments/7/submissions" || !hasVisibility {
			t.Errorf("assignment-scoped read = %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"user_id":9,"assignment_id":7,"assignment_visible":true,"cached_due_date":"2026-10-17T12:00:00-05:00"}]`))
	})
	client := svc.Client
	assignments := []Assignment{{ID: 7, Name: "Catalog work", Published: true, DueAt: str("2026-10-17T12:00:00-05:00"), Points: "5"}}
	loc, _ := time.LoadLocation("America/Chicago")
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, loc)
	rows, flags := completeStudentSubmissions(t.Context(), client, 1, 9, assignments, nil, now.AddDate(0, 0, -14), now, now.AddDate(0, 0, 7))
	if len(flags) != 0 || len(rows) != 1 || rows[0].AssignmentVisible == nil || !*rows[0].AssignmentVisible {
		t.Fatalf("targeted result rows=%+v flags=%v", rows, flags)
	}
	course := ParentReportCourse{WorkDue: []ParentWorkDueItem{}}
	buildStudentReport(&course, rows, map[int64]Assignment{7: assignments[0]}, now, loc, now.AddDate(0, 0, -14))
	if len(course.WorkDue) != 1 || course.WorkDue[0].Status != "no_grade" {
		t.Fatalf("catalog assignment must remain visible without a grade: %+v", course.WorkDue)
	}
}

// One assignment-scoped read serves every student: a student missing from
// it is not assigned the work (no flag), and only a failed read falls back to
// the per-student read.
func TestCompleteStudentSubmissionsSharesAssignmentRead(t *testing.T) {
	listReads, targeted := 0, 0
	failList := false
	svc, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/courses/1/assignments/7/submissions":
			listReads++
			if failList {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Write([]byte(`[{"user_id":9,"assignment_id":7,"assignment_visible":true}]`))
		case "/api/v1/courses/1/assignments/7/submissions/9":
			targeted++
			w.Write([]byte(`{"user_id":9,"assignment_id":7,"assignment_visible":true}`))
		default:
			t.Errorf("unexpected read %s", r.URL.Path)
		}
	})
	assignments := []Assignment{{ID: 7, Name: "Club work", Published: true, DueAt: str("2026-10-17T12:00:00-05:00"), Points: "5"}}
	loc, _ := time.LoadLocation("America/Chicago")
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, loc)
	resolver := &parentSubmissionResolver{client: svc.Client, courseID: 1, batches: map[int64]map[int64]Submission{}}
	for _, student := range []int64{9, 10} {
		rows, flags := completeStudentSubmissionsWithResolver(t.Context(), resolver, student, assignments, nil, now.AddDate(0, 0, -14), now, now.AddDate(0, 0, 7))
		want := 0
		if student == 9 {
			want = 1
		}
		if len(rows) != want || len(flags) != 0 {
			t.Fatalf("student %d rows=%+v flags=%v", student, rows, flags)
		}
	}
	if listReads != 1 || targeted != 0 {
		t.Fatalf("reads: list=%d targeted=%d", listReads, targeted)
	}
	failList = true
	resolver.batches = map[int64]map[int64]Submission{}
	rows, flags := completeStudentSubmissionsWithResolver(t.Context(), resolver, 9, assignments, nil, now.AddDate(0, 0, -14), now, now.AddDate(0, 0, 7))
	if len(rows) != 1 || len(flags) != 0 || targeted != 1 {
		t.Fatalf("fallback rows=%+v flags=%v targeted=%d", rows, flags, targeted)
	}
}

func TestParentCompletionAndClassificationPreserveStudentDueDatePresence(t *testing.T) {
	for _, tc := range []struct {
		name, dateJSON string
		bulk           bool
		work, soon     int
		malformed      bool
	}{
		{name: "explicit null from bulk", dateJSON: `,"cached_due_date":null`, bulk: true},
		{name: "explicit null from targeted read", dateJSON: `,"cached_due_date":null`},
		{name: "explicit empty date", dateJSON: `,"cached_due_date":""`, bulk: true},
		{name: "malformed date stays unavailable", dateJSON: `,"cached_due_date":"not-a-date"`, bulk: true, malformed: true},
		{name: "omitted date uses catalog", bulk: true, work: 1},
		{name: "individual date takes precedence", dateJSON: `,"cached_due_date":"2026-10-19T12:00:00-05:00"`, bulk: true, soon: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := `{"user_id":9,"assignment_id":7,"assignment_visible":true,"missing":true` + tc.dateJSON + `}`
			svc, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
				if tc.bulk || r.URL.Path != "/api/v1/courses/1/assignments/7/submissions" {
					t.Errorf("unexpected read: %s", r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte("[" + body + "]"))
			})
			var bulk []Submission
			if tc.bulk {
				var row Submission
				if err := json.Unmarshal([]byte(body), &row); err != nil {
					t.Fatal(err)
				}
				bulk = []Submission{row}
			}
			loc, _ := time.LoadLocation("America/Chicago")
			now := time.Date(2026, 10, 18, 12, 0, 0, 0, loc)
			start, soonEnd := now.AddDate(0, 0, -14), now.AddDate(0, 0, 7)
			assignment := Assignment{ID: 7, Name: "Individual deadline", Published: true, DueAt: str("2026-10-17T12:00:00-05:00"), Points: "5", SubmissionTypes: []string{"online_text_entry"}}
			rows, flags := completeStudentSubmissions(t.Context(), svc.Client, 1, 9, []Assignment{assignment}, bulk, start, now, soonEnd)
			if len(rows) != 1 || (len(flags) > 0) != tc.malformed {
				t.Fatalf("completion rows=%+v flags=%v", rows, flags)
			}
			if tc.malformed && !strings.Contains(flags[0], "assignment 7 (Individual deadline): Canvas returned a malformed effective due date") {
				t.Fatalf("malformed date needs the item-specific explanation: %v", flags)
			}
			course := ParentReportCourse{WorkDue: []ParentWorkDueItem{}, upcomingEnd: soonEnd}
			buildStudentReport(&course, rows, map[int64]Assignment{7: assignment}, now, loc, start)
			if len(course.WorkDue) != tc.work || len(course.Upcoming) != tc.soon || (tc.work == 1 && course.WorkDue[0].Submission != "not_turned_in") {
				t.Fatalf("student date was not preserved in classification: %+v", course)
			}
		})
	}
}

func TestCompleteStudentSubmissionsChecksUndatedCatalogForStudentOverride(t *testing.T) {
	svc, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"user_id":9,"assignment_id":8,"assignment_visible":true,"cached_due_date":"2026-10-17T12:00:00-05:00"}`))
	})
	client := svc.Client
	loc, _ := time.LoadLocation("America/Chicago")
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, loc)
	assignments := []Assignment{{ID: 8, Name: "Individual deadline", Published: true, Points: "5"}}
	rows, flags := completeStudentSubmissions(t.Context(), client, 1, 9, assignments, nil, now.AddDate(0, 0, -14), now, now.AddDate(0, 0, 7))
	if len(flags) != 0 || len(rows) != 1 || rows[0].CachedDueDate == nil || rows[0].AssignmentVisible == nil || !*rows[0].AssignmentVisible {
		t.Fatalf("targeted read must discover individualized due date rows=%+v flags=%v", rows, flags)
	}
}

func TestCompleteStudentSubmissionsChecksOldBaseDueForStudentOverride(t *testing.T) {
	svc, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"user_id":9,"assignment_id":8,"assignment_visible":true,"cached_due_date":"2026-10-17T12:00:00-05:00"}`))
	})
	loc, _ := time.LoadLocation("America/Chicago")
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, loc)
	assignments := []Assignment{{ID: 8, Name: "Individual deadline", Published: true, DueAt: str("2026-08-17T12:00:00-05:00"), Points: "5"}}
	rows, flags := completeStudentSubmissions(t.Context(), svc.Client, 1, 9, assignments, nil, now.AddDate(0, 0, -14), now, now.AddDate(0, 0, 7))
	if len(flags) != 0 || len(rows) != 1 || rows[0].CachedDueDate == nil || rows[0].AssignmentVisible == nil || !*rows[0].AssignmentVisible {
		t.Fatalf("old catalog date must not hide an individualized in-window deadline: rows=%+v flags=%v", rows, flags)
	}
}

func TestCompleteStudentSubmissionsRejectsWrongTargetIdentity(t *testing.T) {
	svc, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"user_id":10,"assignment_id":7,"assignment_visible":true,"cached_due_date":"2026-10-17T12:00:00-05:00"}`))
	})
	client := svc.Client
	loc, _ := time.LoadLocation("America/Chicago")
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, loc)
	assignments := []Assignment{{ID: 7, Name: "Protected student row", Published: true, DueAt: str("2026-10-17T12:00:00-05:00"), Points: "5"}}
	noVisibility := []Submission{{UserID: 9, AssignmentID: 7, CachedDueDate: str("2026-10-17T12:00:00-05:00")}}
	rows, flags := completeStudentSubmissions(t.Context(), client, 1, 9, assignments, noVisibility, now.AddDate(0, 0, -14), now, now.AddDate(0, 0, 7))
	if len(rows) != 0 || len(flags) != 1 || !strings.Contains(flags[0], "identity did not match") {
		t.Fatalf("wrong-identity targeted row must be excluded: rows=%+v flags=%v", rows, flags)
	}
}

func TestWorkDueStatesGradeAndSubmissionSeparately(t *testing.T) {
	loc, _ := time.LoadLocation("America/Chicago")
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, loc)
	d := func(days int) *string { return str(now.AddDate(0, 0, days).UTC().Format(time.RFC3339)) }
	online := []string{"online_upload"}
	as := map[int64]Assignment{
		1: {ID: 1, Name: "Missing flagged", Published: true, SubmissionTypes: online, DueAt: d(-3), Points: "10", HTMLURL: "u1"},
		2: {ID: 2, Name: "Unsubmitted online", Published: true, SubmissionTypes: []string{"online_text_entry"}, DueAt: d(-2), Points: "10"},
		3: {ID: 3, Name: "Paper not marked", Published: true, SubmissionTypes: []string{"on_paper"}, DueAt: d(-2), Points: "10"},
		4: {ID: 4, Name: "Excused", Published: true, SubmissionTypes: online, DueAt: d(-2), Points: "10"},
		5: {ID: 5, Name: "Due later", Published: true, SubmissionTypes: online, DueAt: d(3), Points: "10"},
		6: {ID: 6, Name: "Corrected", Published: true, SubmissionTypes: online, DueAt: d(-5), Points: "10"},
		7: {ID: 7, Name: "Before the window", Published: true, SubmissionTypes: online, DueAt: d(-30), Points: "10"},
		9: {ID: 9, Name: "Finished, not graded yet", Published: true, SubmissionTypes: online, DueAt: d(-1), Points: "10"},
	}
	at := str("2026-10-01T00:00:00Z")
	subs := []Submission{
		{UserID: 9, AssignmentID: 1, Missing: true},
		{UserID: 9, AssignmentID: 2},
		{UserID: 9, AssignmentID: 3},
		{UserID: 9, AssignmentID: 4, Excused: true},
		{UserID: 9, AssignmentID: 5},
		{UserID: 9, AssignmentID: 6, Attempt: 2, SubmittedAt: at, Score: num("9"), EnteredScore: num("9")},
		{UserID: 9, AssignmentID: 7, Attempt: 1, SubmittedAt: at, Score: num("8")},
		{UserID: 9, AssignmentID: 9, Attempt: 1, SubmittedAt: at},
	}
	rs := ParentReportCourse{}
	buildStudentReport(&rs, subs, as, now, loc, now.AddDate(0, 0, -14))
	got := map[int64]ParentWorkDueItem{}
	for _, item := range rs.WorkDue {
		got[item.AssignmentID] = item
	}
	want := map[int64][2]string{
		1: {"no_grade", "not_turned_in"},
		2: {"no_grade", "not_turned_in"},
		3: {"no_grade", "no_online_submission"},
		6: {"graded", "turned_in"},
		9: {"awaiting_grading", "turned_in"},
	}
	if len(got) != len(want) {
		t.Fatalf("work_due = %+v (excused, later and pre-window work excluded)", rs.WorkDue)
	}
	for id, w := range want {
		if got[id].Status != w[0] || got[id].Submission != w[1] {
			t.Errorf("assignment %d = %s/%s, want %s/%s", id, got[id].Status, got[id].Submission, w[0], w[1])
		}
	}
	if *got[6].Percent != "90" || got[1].URL != "u1" {
		t.Fatalf("facts = %+v", got)
	}
}

func TestParentGradeAndSubmissionAreIndependent(t *testing.T) {
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name               string
		sub                Submission
		points             string
		status, submission string
		percent            string
	}{
		{name: "positive score resolves a stale missing flag", sub: Submission{Missing: true, Score: num("8")}, status: "graded", submission: "unknown", percent: "80"},
		{name: "entered score only", sub: Submission{Missing: true, EnteredScore: num("8")}, status: "graded", submission: "unknown", percent: "80"},
		{name: "submitted work can earn zero", sub: Submission{Attempt: 1, SubmittedAt: str("2026-10-16T12:00:00Z"), Score: num("0")}, status: "graded", submission: "turned_in", percent: "0"},
		{name: "zero with nothing turned in", sub: Submission{Missing: true, Score: num("0.0")}, status: "graded", submission: "not_turned_in", percent: "0"},
		{name: "penalty to zero after a positive entered grade", sub: Submission{Missing: true, Score: num("0"), EnteredScore: num("8")}, status: "graded", submission: "unknown", percent: "0"},
		{name: "late work keeps turned in", sub: Submission{Attempt: 1, SubmittedAt: str("2026-10-16T12:00:00Z"), Score: num("6"), PointsDeducted: num("2")}, status: "graded", submission: "turned_in", percent: "60"},
		{name: "missing without grade is not a zero", sub: Submission{Missing: true}, status: "no_grade", submission: "not_turned_in"},
		{name: "below seventy", sub: Submission{Attempt: 1, SubmittedAt: str("2026-10-16T12:00:00Z"), Score: num("34")}, points: "50", status: "graded", submission: "turned_in", percent: "68"},
		{name: "exactly seventy", sub: Submission{Attempt: 1, SubmittedAt: str("2026-10-16T12:00:00Z"), Score: num("35")}, points: "50", status: "graded", submission: "turned_in", percent: "70"},
		{name: "fractional percent", sub: Submission{Attempt: 1, SubmittedAt: str("2026-10-16T12:00:00Z"), Score: num("2")}, points: "3", status: "graded", submission: "turned_in", percent: "66.67"},
		{name: "zero points possible", sub: Submission{Attempt: 1, SubmittedAt: str("2026-10-16T12:00:00Z"), Score: num("0")}, points: "0", status: "graded", submission: "turned_in"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			points := tc.points
			if points == "" {
				points = "10"
			}
			a := Assignment{ID: 1, Name: "Assignment", Published: true, SubmissionTypes: []string{"online_upload"}, DueAt: str(now.AddDate(0, 0, -2).Format(time.RFC3339)), Points: json.Number(points)}
			sub := tc.sub
			sub.AssignmentID, sub.UserID = 1, 9
			rs := ParentReportCourse{}
			buildStudentReport(&rs, []Submission{sub}, map[int64]Assignment{1: a}, now, time.UTC, now.AddDate(0, 0, -14))
			if len(rs.WorkDue) != 1 {
				t.Fatalf("work_due = %+v", rs.WorkDue)
			}
			row := rs.WorkDue[0]
			gotPercent := ""
			if row.Percent != nil {
				gotPercent = *row.Percent
			}
			if row.Status != tc.status || row.Submission != tc.submission || gotPercent != tc.percent {
				t.Fatalf("row = %s/%s/%q, want %s/%s/%q", row.Status, row.Submission, gotPercent, tc.status, tc.submission, tc.percent)
			}
			if tc.sub.PointsDeducted != nil && (row.PointsDeducted == nil || *row.PointsDeducted != "2") {
				t.Fatalf("late deduction lost: %+v", row)
			}
		})
	}
}

func TestMissingCourseScoreIsFlaggedByEnrollmentRead(t *testing.T) {
	var e Enrollment
	if err := json.Unmarshal([]byte(`{"user_id":1,"grades":{"current_score":null}}`), &e); err != nil || e.Grades == nil || e.Grades.CurrentScore != nil {
		t.Fatalf("null current_score must decode as unavailable: %+v %v", e, err)
	}
}

func TestParentInventoryPreservesAssignmentExclusions(t *testing.T) {
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name string
		edit func(*Assignment, *Submission)
	}{
		{"excused", func(a *Assignment, sub *Submission) { sub.Excused = true }},
		{"unpublished", func(a *Assignment, sub *Submission) { a.Published = false }},
		{"invisible", func(a *Assignment, sub *Submission) { visible := false; sub.AssignmentVisible = &visible }},
		{"not graded", func(a *Assignment, sub *Submission) { a.GradingType = "not_graded" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := Assignment{ID: 1, Published: true, DueAt: str("2026-10-15T12:00:00Z"), Points: "10"}
			sub := Submission{AssignmentID: 1, Missing: true, Score: num("0")}
			tc.edit(&a, &sub)
			rs := ParentReportCourse{}
			buildStudentReport(&rs, []Submission{sub}, map[int64]Assignment{1: a}, now, time.UTC, now.AddDate(0, 0, -14))
			if tc.name == "not graded" {
				if len(rs.WorkDue) != 1 || rs.WorkDue[0].Status != "no_grade" || rs.WorkDue[0].Current != nil {
					t.Fatalf("ungraded required work must remain in inventory without a score: %+v", rs)
				}
				return
			}
			if len(rs.WorkDue) != 0 {
				t.Fatalf("excluded assignment appears: %+v", rs)
			}
		})
	}
}

func TestParentReportWritesFactsForTheWholeTerm(t *testing.T) {
	svc, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("parent report made a write: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/courses/1/enrollments":
			w.Write([]byte(`[{"user_id":9,"type":"StudentEnrollment","enrollment_state":"active","user":{"id":9,"name":"Private Name"},"grades":{"current_score":87}},{"user_id":10,"type":"StudentEnrollment","enrollment_state":"active","user":{"id":10,"name":"Other Private Name"},"grades":{"current_score":98}}]`))
		case "/api/v1/courses/1/assignments":
			w.Write([]byte(`[{"id":1,"name":"T2 graded work","published":true,"grading_type":"points","submission_types":["online_upload"],"due_at":"2026-10-15T12:00:00Z","points_possible":10},{"id":2,"name":"T1 zero","published":true,"grading_type":"points","submission_types":["online_upload"],"due_at":"2026-10-08T12:00:00Z","points_possible":10},{"id":3,"name":"T1 correction","published":true,"grading_type":"points","submission_types":["online_upload"],"due_at":"2026-10-12T12:00:00Z","points_possible":10},{"id":4,"name":"T2 first day","published":true,"grading_type":"points","submission_types":["on_paper"],"due_at":"2026-10-13T12:00:00Z","points_possible":10}]`))
		case "/api/v1/courses/1/students/submissions":
			w.Write([]byte(`[{"user_id":9,"assignment_id":1,"assignment_visible":true,"missing":true,"entered_score":8,"score":8},{"user_id":10,"assignment_id":1,"assignment_visible":true,"score":0,"attempt":1,"submitted_at":"2026-10-15T12:00:00Z"},{"user_id":9,"assignment_id":2,"assignment_visible":true,"score":0,"attempt":1,"graded_at":"2026-10-17T12:00:00Z"},{"user_id":10,"assignment_id":2,"assignment_visible":true,"score":0,"attempt":1,"graded_at":"2026-10-17T12:00:00Z"},{"user_id":9,"assignment_id":3,"assignment_visible":true,"score":8,"attempt":1,"submitted_at":"2026-10-12T12:00:00Z"},{"user_id":10,"assignment_id":3,"assignment_visible":true,"score":8,"attempt":1,"submitted_at":"2026-10-12T12:00:00Z"},{"user_id":9,"assignment_id":4,"assignment_visible":true},{"user_id":10,"assignment_id":4,"assignment_visible":true}]`))
		default:
			t.Errorf("unexpected Canvas route %s", r.URL.Path)
			http.NotFound(w, r)
		}
	})
	if err := workspace.WriteTeacherDocument(svc.Root, "context/school-calendar.md", workspace.SchoolCalendar{Schema: 1, Timezone: "America/Chicago", StartDate: "2026-08-19", EndDate: "2027-05-27", GradingPeriods: []workspace.GradingPeriod{{Name: "Term 1", StartDate: "2026-08-19", EndDate: "2026-10-08"}, {Name: "Term 2", StartDate: "2026-10-13", EndDate: "2026-12-18"}, {Name: "Term 3", StartDate: "2027-01-06", EndDate: "2027-03-19"}, {Name: "Term 4", StartDate: "2027-03-22", EndDate: "2027-05-27"}}}, ""); err != nil {
		t.Fatal(err)
	}
	if err := workspace.WriteTeacherDocument(svc.Root, "context/courses.md", svc.Config, ""); err != nil {
		t.Fatal(err)
	}
	path, packet, err := svc.ParentReport(context.Background(), []string{"TEST"}, nil, time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(packet.Students) != 2 || packet.Schema != 2 {
		t.Fatalf("packet = %+v", packet)
	}
	if packet.Scope == nil || packet.Scope.PeriodName != "Term 2" || packet.Scope.RecentStartAt != "2026-10-13T00:00:00-05:00" {
		t.Fatalf("saved scope must start at the term start, a month back: %+v", packet.Scope)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(filepath.Dir(path)) != filepath.Join(svc.Root, "courses", "TEST", "parent-updates") {
		t.Fatalf("report must remain local to UserInstall: %s", path)
	}
	for _, unwanted := range []string{"Private Name", "skip_eligible", "correction", "due_soon", "first_earned"} {
		if strings.Contains(string(raw), unwanted) {
			t.Fatalf("report must hold facts only, found %q: %s", unwanted, raw)
		}
	}
	var saved ParentReportPacket
	if err = json.Unmarshal(raw, &saved); err != nil {
		t.Fatal(err)
	}
	for _, student := range saved.Students {
		course := student.Courses[0]
		if len(course.WorkDue) != 2 || course.WorkDue[0].AssignmentID != 4 || course.WorkDue[1].AssignmentID != 1 || len(course.Flags) != 0 {
			t.Fatalf("the T2 first-day assignment and current work should be inventoried, T1 work excluded: %+v", course)
		}
		paper, work := course.WorkDue[0], course.WorkDue[1]
		if paper.Status != "no_grade" || paper.Submission != "no_online_submission" {
			t.Fatalf("paper work = %+v", paper)
		}
		if *course.CourseScore == "87" {
			if *work.Percent != "80" || work.Submission != "unknown" {
				t.Fatalf("positive grade with no upload = %+v", work)
			}
		} else if *work.Percent != "0" || work.Submission != "turned_in" {
			t.Fatalf("submitted zero = %+v", work)
		}
	}
}

func TestParentConfigKeepsLoadingExistingTeacherKeys(t *testing.T) {
	root := t.TempDir()
	body := "---\nschema: 1\nskyward_url: https://portal.example.edu\nbobcat_hour_url: https://help.example.edu\ngrade_portal_url: https://other.example.edu\ncorrection_days: 90\nskip_score: 95\nfailing_below: 70\nsign_off: Ms. Teacher\n---\n"
	if err := os.MkdirAll(filepath.Join(root, "context"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "context", "parent-updates.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	config, err := loadParentConfig(root)
	if err != nil || config.SignOff != "Ms. Teacher" {
		t.Fatalf("teacher settings are the agent's to read; the Engine must not reject them: %+v %v", config, err)
	}
}

func TestParentFalseBulkVisibilityUsesAssignmentBatchAndKeepsStudentDue(t *testing.T) {
	reads := 0
	svc, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		reads++
		if r.URL.Path != "/api/v1/courses/1/assignments/7/submissions" {
			t.Errorf("unexpected read %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"user_id":9,"assignment_id":7,"assignment_visible":true},{"user_id":10,"assignment_id":7,"assignment_visible":false}]`))
	})
	no := false
	due := "2026-10-16T12:00:00-05:00"
	assignments := []Assignment{{ID: 7, Name: "Paper", Published: true, GradingType: "points", DueAt: str("2026-10-15T12:00:00-05:00"), SubmissionTypes: []string{"on_paper"}, Points: "100"}}
	loc, _ := time.LoadLocation("America/Chicago")
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, loc)
	resolver := &parentSubmissionResolver{client: svc.Client, courseID: 1, batches: map[int64]map[int64]Submission{}}
	for _, id := range []int64{9, 10} {
		bulk := []Submission{{UserID: id, AssignmentID: 7, AssignmentVisible: &no, CachedDueDate: &due, CachedDueDatePresent: true, Score: num("0")}}
		rows, flags := completeStudentSubmissionsWithResolver(t.Context(), resolver, id, assignments, bulk, now.AddDate(0, 0, -14), now, now.AddDate(0, 0, 7))
		if len(flags) != 0 || len(rows) != 1 {
			t.Fatalf("rows=%+v flags=%v", rows, flags)
		}
		course := ParentReportCourse{WorkDue: []ParentWorkDueItem{}}
		buildStudentReport(&course, rows, map[int64]Assignment{7: assignments[0]}, now, loc, now.AddDate(0, 0, -14))
		if id == 9 && (len(course.WorkDue) != 1 || course.WorkDue[0].Submission != "no_online_submission" || !strings.Contains(course.WorkDue[0].DueAt, "2026-10-16")) {
			t.Fatalf("visible override: %+v", course)
		}
		if id == 10 && len(course.WorkDue) != 0 {
			t.Fatalf("invisible work leaked: %+v", course)
		}
	}
	if reads != 1 {
		t.Fatalf("assignment reads = %d, want one", reads)
	}
}
