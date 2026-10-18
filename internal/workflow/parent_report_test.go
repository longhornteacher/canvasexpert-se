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

func TestCorrectionDeadlineIsEndOfDay14InCourseZone(t *testing.T) {
	loc, _ := time.LoadLocation("America/Chicago")
	due := time.Date(2026, 10, 2, 22, 0, 0, 0, loc) // Friday evening
	got := correctionDeadline(due.UTC(), loc)
	want := time.Date(2026, 10, 16, 23, 59, 59, 0, loc)
	if !got.Equal(want) {
		t.Fatalf("deadline %v, want %v", got, want)
	}
}

func TestParentWindowUsesEffectiveDueDatesAndCompleteInventory(t *testing.T) {
	loc, _ := time.LoadLocation("America/Chicago")
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, loc)
	start := time.Date(2026, 10, 13, 0, 0, 0, 0, loc)
	soonEnd := time.Date(2026, 10, 25, 0, 0, 0, 0, loc) // period end clips the seven-day horizon.
	assignments := map[int64]Assignment{
		1: {ID: 1, Name: "Term 1 zero", Published: true, DueAt: str("2026-10-08T16:00:00-05:00"), Points: "10"},
		2: {ID: 2, Name: "Recent regrade, old due", Published: true, DueAt: str("2026-10-12T16:00:00-05:00"), Points: "10"},
		3: {ID: 3, Name: "Window first day", Published: true, DueAt: str("2026-10-13T16:00:00-05:00"), Points: "10"},
		4: {ID: 4, Name: "Perfect", Published: true, DueAt: str("2026-10-17T16:00:00-05:00"), Points: "10"},
		5: {ID: 5, Name: "Awaiting grade", Published: true, DueAt: str("2026-10-18T11:00:00-05:00"), Points: "10"},
		6: {ID: 6, Name: "Required ungraded", Published: true, GradingType: "not_graded", DueAt: str("2026-10-17T16:00:00-05:00")},
		7: {ID: 7, Name: "Due soon", Published: true, DueAt: str("2026-10-24T16:00:00-05:00"), Points: "10"},
		8: {ID: 8, Name: "Outside term", Published: true, DueAt: str("2026-12-19T16:00:00-06:00"), Points: "10"},
		9: {ID: 9, Name: "Undated override", Published: true, DueAt: str("2026-10-17T16:00:00-05:00"), Points: "10"},
	}
	var undated *string
	allDates := []AssignmentAllDateLive{{UserID: 9, DueAt: undated}}
	assignments[9] = Assignment{ID: 9, Name: "Undated override", Published: true, DueAt: str("2026-10-17T16:00:00-05:00"), AllDates: &allDates, Points: "10"}
	byID := assignments
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
	versions := map[int64][]Submission{2: {{UserID: 9, AssignmentID: 2, Score: num("0"), GradedAt: str(now.Format(time.RFC3339))}}}
	buildStudentReportScoped(&course, subs, versions, byID, now, loc, start, soonEnd)
	got := map[int64]ParentWorkDueItem{}
	for _, item := range course.WorkDue {
		got[item.AssignmentID] = item
	}
	if len(got) != 4 || got[3].Status != "recorded_zero" || got[4].Status != "graded" || got[5].Status != "awaiting_grading" || got[6].Status != "no_grade" {
		t.Fatalf("work_due = %+v", got)
	}
	for _, id := range []int64{1, 2, 7, 8, 9} {
		if _, ok := got[id]; ok {
			t.Errorf("assignment %d should not be in retrospective inventory", id)
		}
	}
	if len(course.Missing) != 1 || course.Missing[0].AssignmentID != 3 || len(course.DueSoon) != 1 || course.DueSoon[0].AssignmentID != 7 || len(course.History) != 2 {
		t.Fatalf("derived lists: missing=%+v dueSoon=%+v history=%+v", course.Missing, course.DueSoon, course.History)
	}
}

func TestParentInventoryWithholdsExplicitlyUnreleasedGrade(t *testing.T) {
	loc, _ := time.LoadLocation("America/Chicago")
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, loc)
	a := Assignment{ID: 1, Name: "Private grade", Published: true, DueAt: str("2026-10-17T12:00:00-05:00"), Points: "10", SubmissionTypes: []string{"online_upload"}}
	sub := Submission{UserID: 9, AssignmentID: 1, AssignmentVisible: boolPtr(true), Attempt: 1, Score: num("0"), EnteredScore: num("8"), GradedAt: str("2026-10-18T10:00:00-05:00"), PostedAtPresent: true, Comments: []SubmissionComment{{AuthorID: 77, Comment: "Unreleased feedback"}}}
	course := ParentReportCourse{WorkDue: []ParentWorkDueItem{}}
	buildStudentReportScoped(&course, []Submission{sub}, nil, map[int64]Assignment{1: a}, now, loc, now.AddDate(0, 0, -14), now.AddDate(0, 0, 7))
	if len(course.Missing)+len(course.Corrections)+len(course.History) != 0 || len(course.WorkDue) != 1 {
		t.Fatalf("unreleased grade leaked into family sections: %+v", course)
	}
	item := course.WorkDue[0]
	if item.Status != "grade_not_released" || item.Current != nil || item.First != nil || item.Possible != "" || strings.Contains(item.Note, "Unreleased feedback") {
		t.Fatalf("unreleased grade inventory = %+v", item)
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
		endScope, _, soonEnd, err := makeParentReportScope(calendar, endScopePeriod, nearEnd.Add(12*time.Hour))
		wantEnd := nearEnd.AddDate(0, 0, 2) // midnight immediately after the inclusive period end.
		if err != nil || endScope.PeriodName != tc.name || !soonEnd.Equal(wantEnd) {
			t.Fatalf("%s upcoming boundary=%s want=%s scope=%+v err=%v", tc.name, soonEnd, wantEnd, endScope, err)
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
		if r.URL.Path != "/api/v1/courses/1/assignments/7/submissions/9" || !hasVisibility {
			t.Errorf("targeted read = %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"user_id":9,"assignment_id":7,"assignment_visible":true,"cached_due_date":"2026-10-17T12:00:00-05:00"}`))
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
	buildStudentReportScoped(&course, rows, nil, map[int64]Assignment{7: assignments[0]}, now, loc, now.AddDate(0, 0, -14), now.AddDate(0, 0, 7))
	if len(course.WorkDue) != 1 || course.WorkDue[0].Status != "no_grade" {
		t.Fatalf("catalog assignment must remain visible without a grade: %+v", course.WorkDue)
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

func parentFixture(now time.Time) (map[int64]Assignment, []Submission) {
	d := func(days int) *string { return str(now.AddDate(0, 0, days).UTC().Format(time.RFC3339)) }
	as := map[int64]Assignment{
		1: {ID: 1, Name: "Missing flagged", Published: true, SubmissionTypes: []string{"online_upload"}, DueAt: d(-3), Points: "10", HTMLURL: "u1"},
		2: {ID: 2, Name: "Unsubmitted online", Published: true, SubmissionTypes: []string{"online_text_entry"}, DueAt: d(-2), Points: "10"},
		3: {ID: 3, Name: "Paper not marked", Published: true, SubmissionTypes: []string{"on_paper"}, DueAt: d(-2), Points: "10"},
		4: {ID: 4, Name: "Excused", Published: true, SubmissionTypes: []string{"online_upload"}, DueAt: d(-2), Points: "10"},
		5: {ID: 5, Name: "Due soon", Published: true, SubmissionTypes: []string{"online_upload"}, DueAt: d(3), Points: "10"},
		6: {ID: 6, Name: "Corrected", Published: true, SubmissionTypes: []string{"online_upload"}, DueAt: d(-5), Points: "10"},
		7: {ID: 7, Name: "Too old", Published: true, SubmissionTypes: []string{"online_upload"}, DueAt: d(-30), Points: "10"},
		8: {ID: 8, Name: "Far future", Published: true, SubmissionTypes: []string{"online_upload"}, DueAt: d(20), Points: "10"},
		9: {ID: 9, Name: "Finished, no feedback yet", Published: true, SubmissionTypes: []string{"online_upload"}, DueAt: d(-1), Points: "10"},
	}
	at := str("2026-10-01T00:00:00Z")
	one := 1
	subs := []Submission{
		{UserID: 9, AssignmentID: 1, Missing: true},
		{UserID: 9, AssignmentID: 2},
		{UserID: 9, AssignmentID: 3},
		{UserID: 9, AssignmentID: 4, Excused: true},
		{UserID: 9, AssignmentID: 5},
		{UserID: 9, AssignmentID: 6, Attempt: 2, SubmittedAt: at, Score: num("9"), EnteredScore: num("9"),
			History: []Submission{{Attempt: 1, Score: num("5"), EnteredScore: num("5")}, {Attempt: 2, Score: num("9")}},
			Comments: []SubmissionComment{
				{ID: 1, Comment: "Here is why: ...", AuthorID: 77, Attempt: &one},
				{ID: 2, Comment: "my reply", AuthorID: 9}}},
		{UserID: 9, AssignmentID: 7, Attempt: 1, SubmittedAt: at, Score: num("8"), Comments: []SubmissionComment{{ID: 3, Comment: "ok", AuthorID: 77}}},
		{UserID: 9, AssignmentID: 8},
		{UserID: 9, AssignmentID: 9, Attempt: 1, SubmittedAt: at},
	}
	return as, subs
}

func TestBuildStudentReportSections(t *testing.T) {
	loc, _ := time.LoadLocation("America/Chicago")
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, loc)
	as, subs := parentFixture(now)
	rs := ParentReportCourse{}
	buildStudentReport(&rs, subs, nil, as, now, loc)

	missing := map[int64]string{}
	for _, m := range rs.Missing {
		missing[m.AssignmentID] = m.Basis
	}
	if len(missing) != 2 || missing[1] != "canvas_missing" || missing[2] != "not_submitted_canvas_not_marked_missing" {
		t.Fatalf("missing = %v (paper/excused must not appear, online past-due labelled honestly)", missing)
	}
	if len(rs.DueSoon) != 1 || rs.DueSoon[0].AssignmentID != 5 {
		t.Fatalf("due soon = %+v", rs.DueSoon)
	}
	if len(rs.Corrections) != 2 || rs.Corrections[0].AssignmentID != 6 || len(rs.Corrections[0].Comments) != 1 || rs.Corrections[1].AssignmentID != 9 || len(rs.Corrections[1].Comments) != 0 {
		t.Fatalf("corrections = %+v (finished work opens corrections without feedback; student's own comments and old work are excluded)", rs.Corrections)
	}
	if len(rs.History) != 1 {
		t.Fatalf("history = %+v", rs.History)
	}
	h := rs.History[0]
	if h.First == nil || *h.First != "5" || *h.Latest != "9" || *h.Current != "9" || h.FirstUnavailable {
		t.Fatalf("history = %+v", h)
	}
}

func TestHistoryProvenance(t *testing.T) {
	a := Assignment{ID: 1, Points: "10"}
	none := historyItem(ParentReportItem{}, Submission{Score: num("7"), EnteredScore: num("7")}, nil, a)
	if !none.FirstUnavailable || none.First != nil {
		t.Fatalf("no history must be unavailable, not entered_score: %+v", none)
	}
	one := historyItem(ParentReportItem{}, Submission{Score: num("7"), History: []Submission{{Attempt: 1, Score: num("7")}}}, nil, a)
	if one.First == nil || *one.First != "7" || *one.Latest != "7" {
		t.Fatalf("single entry = %+v", one)
	}
}

func TestMissingCourseScoreIsFlaggedByEnrollmentRead(t *testing.T) {
	var e Enrollment
	if err := json.Unmarshal([]byte(`{"user_id":1,"grades":{"current_score":null}}`), &e); err != nil || e.Grades == nil || e.Grades.CurrentScore != nil {
		t.Fatalf("null current_score must decode as unavailable: %+v %v", e, err)
	}
}

// 2026-10-16 Canvas Live: a same-attempt regrade (0 on 9/16, 80 on 10/08) left
// one submission_history entry; only the gradebook feed shows the first score.
func TestGradebookVersionsGiveFirstScoreAndRealChanges(t *testing.T) {
	loc, _ := time.LoadLocation("America/Chicago")
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, loc)
	as := map[int64]Assignment{
		1: {ID: 1, Name: "Regraded", Published: true, SubmissionTypes: []string{"on_paper"}, DueAt: str("2026-10-11T21:00:00Z"), Points: "100"},
		2: {ID: 2, Name: "Same score resaved", Published: true, SubmissionTypes: []string{"on_paper"}, DueAt: str("2026-10-11T21:00:00Z"), Points: "100"},
	}
	subs := []Submission{
		{UserID: 9, AssignmentID: 1, Score: num("80"), GradedAt: str("2026-10-08T15:00:44Z"), History: []Submission{{Score: num("80")}}},
		{UserID: 9, AssignmentID: 2, Score: num("80"), GradedAt: str("2026-10-08T15:00:44Z"), History: []Submission{{Score: num("80")}}},
	}
	versions := map[int64][]Submission{
		1: {{Score: num("0"), GradedAt: str("2026-09-16T17:40:37Z")}, {Score: num("80"), GradedAt: str("2026-10-08T15:00:44Z")}},
		2: {{Score: num("80"), GradedAt: str("2026-09-18T21:21:41Z")}, {Score: num("80"), GradedAt: str("2026-10-08T15:00:44Z")}},
	}
	rs := ParentReportCourse{}
	buildStudentReport(&rs, subs, versions, as, now, loc)
	if len(rs.History) != 1 || rs.History[0].AssignmentID != 1 {
		t.Fatalf("history = %+v (an unchanged resave is not a change)", rs.History)
	}
	if h := rs.History[0]; h.First == nil || *h.First != "0" || *h.Latest != "80" {
		t.Fatalf("history = %+v", h)
	}
}

func TestParentMissingZeroClassification(t *testing.T) {
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name       string
		sub        Submission
		old        bool
		basis      string
		correction bool
		history    bool
	}{
		{name: "positive score overrides missing and no submission", sub: Submission{Missing: true, Score: num("8")}, correction: true, history: true},
		{name: "positive score overrides absent online work", sub: Submission{Score: num("8")}, correction: true, history: true},
		{name: "entered score only overrides missing", sub: Submission{Missing: true, EnteredScore: num("8")}, correction: true},
		{name: "current zero with submitted work", sub: Submission{Attempt: 1, SubmittedAt: str("2026-10-16T12:00:00Z"), Score: num("0")}, basis: "recorded_zero", history: true},
		{name: "current zero without submitted work", sub: Submission{Missing: true, Score: num("0.0")}, basis: "recorded_zero", history: true},
		{name: "zero outside report window", sub: Submission{Score: num("0"), GradedAt: str("2026-09-01T12:00:00Z")}, old: true},
		{name: "entered zero only", sub: Submission{EnteredScore: num("0")}, basis: "recorded_zero"},
		{name: "positive entered grade with current zero", sub: Submission{Missing: true, Score: num("0"), EnteredScore: num("8")}, basis: "recorded_zero", history: true},
		{name: "ordinary missing without grade", sub: Submission{Missing: true, Comments: []SubmissionComment{{AuthorID: 77, Comment: "Please finish"}}}, basis: "canvas_missing"},
		{name: "ordinary unsubmitted without grade", sub: Submission{}, basis: "not_submitted_canvas_not_marked_missing"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			due := now.AddDate(0, 0, -2)
			if tc.old {
				due = now.AddDate(0, 0, -40)
			}
			a := Assignment{ID: 1, Name: "Assignment", Published: true, SubmissionTypes: []string{"online_upload"}, DueAt: str(due.Format(time.RFC3339)), Points: "10"}
			sub := tc.sub
			sub.AssignmentID, sub.UserID = 1, 9
			rs := ParentReportCourse{}
			buildStudentReport(&rs, []Submission{sub}, nil, map[int64]Assignment{1: a}, now, time.UTC)
			if tc.basis == "" {
				if len(rs.Missing) != 0 {
					t.Fatalf("positive grade must resolve missing: %+v", rs.Missing)
				}
			} else if len(rs.Missing) != 1 || rs.Missing[0].Basis != tc.basis {
				t.Fatalf("Missing/0 = %+v, want basis %q", rs.Missing, tc.basis)
			}
			if (len(rs.Corrections) == 1) != tc.correction || len(rs.Corrections) > 1 {
				t.Fatalf("corrections = %+v, want eligible %v", rs.Corrections, tc.correction)
			}
			if (len(rs.History) == 1) != tc.history || len(rs.History) > 1 {
				t.Fatalf("history = %+v, want retained %v", rs.History, tc.history)
			}
			if tc.history && (rs.History[0].Current == nil || *rs.History[0].Current != sub.Score.String()) {
				t.Fatalf("history must retain current Canvas score: %+v", rs.History[0])
			}
		})
	}
}

func TestParentHistoricalZeroRaisedToPositiveIsNotMissing(t *testing.T) {
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	a := Assignment{ID: 1, Published: true, SubmissionTypes: []string{"online_upload"}, DueAt: str("2026-10-15T12:00:00Z"), Points: "10"}
	sub := Submission{AssignmentID: 1, Missing: true, Score: num("8"), EnteredScore: num("9"), History: []Submission{{Attempt: 1, Score: num("0")}, {Attempt: 2, Score: num("8")}}}
	rs := ParentReportCourse{}
	buildStudentReport(&rs, []Submission{sub}, nil, map[int64]Assignment{1: a}, now, time.UTC)
	if len(rs.Missing) != 0 || len(rs.Corrections) != 1 || len(rs.History) != 1 {
		t.Fatalf("resolved report = %+v", rs)
	}
	if h := rs.History[0]; h.First == nil || *h.First != "0" || h.Latest == nil || *h.Latest != "8" || h.Current == nil || *h.Current != "8" {
		t.Fatalf("entered grade must not replace historical or current score provenance: %+v", h)
	}
}

func TestParentMissingZeroPreservesAssignmentExclusions(t *testing.T) {
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
			buildStudentReport(&rs, []Submission{sub}, nil, map[int64]Assignment{1: a}, now, time.UTC)
			if tc.name == "not graded" {
				if len(rs.Missing)+len(rs.Corrections)+len(rs.History)+len(rs.DueSoon) != 0 || len(rs.WorkDue) != 1 {
					t.Fatalf("ungraded required work must remain in inventory without classification: %+v", rs)
				}
				return
			}
			if len(rs.Missing)+len(rs.Corrections)+len(rs.History)+len(rs.DueSoon)+len(rs.WorkDue) != 0 {
				t.Fatalf("excluded assignment appears: %+v", rs)
			}
		})
	}
}

func TestParentSkipEligibilityAfterAllCoursesClassified(t *testing.T) {
	for _, tc := range []struct {
		name    string
		courses []ParentReportCourse
		skip    int
		want    bool
	}{
		{"all at threshold", []ParentReportCourse{{CourseScore: str("90"), WorkDue: []ParentWorkDueItem{}}, {CourseScore: str("97.4"), WorkDue: []ParentWorkDueItem{}}}, 90, true},
		{"empty window skips regardless of current score or due soon", []ParentReportCourse{{CourseScore: str("42"), WorkDue: []ParentWorkDueItem{}, DueSoon: []ParentReportItem{{AssignmentID: 5}}}}, 90, true},
		{"one below threshold", []ParentReportCourse{{CourseScore: str("99"), WorkDue: []ParentWorkDueItem{{Status: "graded"}}}, {CourseScore: str("89.99"), WorkDue: []ParentWorkDueItem{{Status: "graded"}}}}, 90, false},
		{"zero despite high average", []ParentReportCourse{{CourseScore: str("98"), WorkDue: []ParentWorkDueItem{{Status: "recorded_zero"}}, Missing: []ParentReportItem{{Basis: "recorded_zero"}}}}, 90, false},
		{"ordinary missing despite high average", []ParentReportCourse{{CourseScore: str("98"), WorkDue: []ParentWorkDueItem{{Status: "missing"}}, Missing: []ParentReportItem{{Basis: "canvas_missing"}}}}, 90, false},
		{"grade unavailable", []ParentReportCourse{{CourseScore: str("98"), WorkDue: []ParentWorkDueItem{{Status: "graded"}}}, {WorkDue: []ParentWorkDueItem{}}}, 90, false},
		{"no courses", nil, 90, false},
		{"custom threshold", []ParentReportCourse{{CourseScore: str("86"), WorkDue: []ParentWorkDueItem{{Status: "graded"}}}}, 85, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := parentSkipEligible(ParentReportStudent{Courses: tc.courses}, tc.skip); got != tc.want {
				t.Fatalf("skip eligible = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestParentReportWritesResolvedClassificationAndConfiguredSkip(t *testing.T) {
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
		case "/api/v1/courses/1/gradebook_history/feed":
			w.Write([]byte(`[]`))
		default:
			t.Errorf("unexpected Canvas route %s", r.URL.Path)
			http.NotFound(w, r)
		}
	})
	if err := workspace.WriteTeacherDocument(svc.Root, "context/parent-updates.md", ParentConfig{Schema: 1, SkipScore: 95}, ""); err != nil {
		t.Fatal(err)
	}
	if err := workspace.WriteTeacherDocument(svc.Root, "context/school-calendar.md", workspace.SchoolCalendar{Schema: 1, Timezone: "America/Chicago", StartDate: "2026-08-19", EndDate: "2027-05-27", GradingPeriods: []workspace.GradingPeriod{{Name: "Term 1", StartDate: "2026-08-19", EndDate: "2026-10-08"}, {Name: "Term 2", StartDate: "2026-10-13", EndDate: "2026-12-18"}, {Name: "Term 3", StartDate: "2027-01-06", EndDate: "2027-03-19"}, {Name: "Term 4", StartDate: "2027-03-22", EndDate: "2027-05-27"}}}, ""); err != nil {
		t.Fatal(err)
	}
	if err := workspace.WriteTeacherDocument(svc.Root, "context/courses.md", svc.Config, ""); err != nil {
		t.Fatal(err)
	}
	syncRoot := t.TempDir()
	if _, err := workspace.SetRulesLocation(svc.Root, syncRoot, nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := workspace.WriteTeacherDocument(svc.Root, "context/parent-updates.md", ParentConfig{Schema: 1, SkipScore: 85}, ""); err != nil {
		t.Fatal(err)
	}
	path, packet, err := svc.ParentReport(context.Background(), []string{"TEST"}, nil, time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(packet.Students) != 2 {
		t.Fatalf("students = %+v", packet.Students)
	}
	if packet.Scope == nil || packet.Scope.PeriodName != "Term 2" || packet.Scope.RecentStartAt != "2026-10-13T00:00:00-05:00" {
		t.Fatalf("saved scope = %+v", packet.Scope)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(filepath.Dir(path)) != filepath.Join(svc.Root, "courses", "TEST", "parent-updates") {
		t.Fatalf("report must remain local to UserInstall: %s", path)
	}
	if strings.Contains(string(raw), "Private Name") || !strings.Contains(string(raw), `"skip_eligible": false`) || !strings.Contains(string(raw), `"skip_eligible": true`) {
		t.Fatalf("report must expose eligibility without names: %s", raw)
	}
	var saved ParentReportPacket
	if err = json.Unmarshal(raw, &saved); err != nil {
		t.Fatal(err)
	}
	for _, student := range saved.Students {
		course := student.Courses[0]
		if len(course.WorkDue) != 2 || course.WorkDue[0].AssignmentID != 4 || course.WorkDue[1].AssignmentID != 1 {
			t.Fatalf("the T2 first-day assignment and current work should be inventoried, old T1 work excluded: %+v", course.WorkDue)
		}
		for _, row := range course.WorkDue {
			if row.Title == "T1 zero" || row.Title == "T1 correction" {
				t.Fatalf("T1 assignment leaked into T2 inventory: %+v", row)
			}
		}
		if *course.CourseScore == "87" {
			if !student.SkipEligible || len(course.Missing) != 0 || len(course.Corrections) != 1 || len(course.Flags) != 0 {
				t.Fatalf("positive grade must be resolved before drafting, with active threshold: %+v", student)
			}
		} else if student.SkipEligible || len(course.Missing) != 1 || course.Missing[0].Basis != "recorded_zero" || len(course.Corrections) != 0 || len(course.History) != 1 {
			t.Fatalf("zero must block skipping and corrections while retaining history: %+v", student)
		}
	}
}
