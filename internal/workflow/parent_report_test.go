package workflow

import (
	"encoding/json"
	"testing"
	"time"
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
	}
	return as, subs
}

func TestBuildStudentReportSections(t *testing.T) {
	loc, _ := time.LoadLocation("America/Chicago")
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, loc)
	as, subs := parentFixture(now)
	rs := ParentReportCourse{}
	buildStudentReport(&rs, subs, as, now, loc)

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
	if len(rs.Corrections) != 1 || rs.Corrections[0].AssignmentID != 6 || len(rs.Corrections[0].Comments) != 1 {
		t.Fatalf("corrections = %+v (student's own comments and old work are excluded)", rs.Corrections)
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
	none := historyItem(ParentReportItem{}, Submission{Score: num("7"), EnteredScore: num("7")}, a)
	if !none.FirstUnavailable || none.First != nil {
		t.Fatalf("no history must be unavailable, not entered_score: %+v", none)
	}
	one := historyItem(ParentReportItem{}, Submission{Score: num("7"), History: []Submission{{Attempt: 1, Score: num("7")}}}, a)
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
