package workflow

import (
	"strings"
	"testing"
	"time"

	"canvasexpertse/internal/workspace"
)

func TestReportListsUpcomingWork(t *testing.T) {
	loc, _ := time.LoadLocation("America/Chicago")
	now := time.Date(2026, 11, 4, 13, 48, 0, 0, loc)
	start := time.Date(2026, 10, 13, 0, 0, 0, 0, loc)
	periodEnd := time.Date(2026, 12, 19, 0, 0, 0, 0, loc)
	a := func(id int64, published bool, due string) Assignment {
		return Assignment{ID: id, Name: "A", Published: published, DueAt: str(due), Points: "100", SubmissionTypes: []string{"online_upload"}}
	}
	as := map[int64]Assignment{
		1: a(1, true, "2026-10-20T16:00:00-05:00"),  // past, graded
		2: a(2, true, "2026-11-06T16:00:00-06:00"),  // upcoming, turned in early
		3: a(3, true, "2026-12-01T16:00:00-06:00"),  // upcoming, base date superseded by override below
		4: a(4, false, "2026-11-10T16:00:00-06:00"), // unpublished
		5: a(5, true, "2026-11-10T16:00:00-06:00"),  // not visible to this student
		6: a(6, true, "2027-01-10T16:00:00-06:00"),  // after the period
	}
	yes, no := boolPtr(true), boolPtr(false)
	subs := []Submission{
		{UserID: 9, AssignmentID: 1, AssignmentVisible: yes, Attempt: 1, SubmittedAt: str("2026-10-19T10:00:00-05:00"), Score: num("85"), EnteredScore: num("85")},
		{UserID: 9, AssignmentID: 2, AssignmentVisible: yes, Attempt: 1, SubmittedAt: str("2026-11-03T10:00:00-06:00")},
		{UserID: 9, AssignmentID: 3, AssignmentVisible: yes, CachedDueDate: str("2026-11-20T16:00:00-06:00")},
		{UserID: 9, AssignmentID: 4, AssignmentVisible: yes},
		{UserID: 9, AssignmentID: 5, AssignmentVisible: no},
		{UserID: 9, AssignmentID: 6, AssignmentVisible: yes},
	}
	course := ParentReportCourse{WorkDue: []ParentWorkDueItem{}, upcomingEnd: periodEnd}
	buildStudentReport(&course, subs, as, now, loc, start)
	if len(course.WorkDue) != 1 || course.WorkDue[0].AssignmentID != 1 {
		t.Fatalf("work_due = %+v", course.WorkDue)
	}
	if len(course.Upcoming) != 2 || course.Upcoming[0].AssignmentID != 2 || !course.Upcoming[0].TurnedIn ||
		course.Upcoming[1].AssignmentID != 3 || course.Upcoming[1].TurnedIn || !strings.HasPrefix(course.Upcoming[1].DueAt, "2026-11-20") {
		t.Fatalf("upcoming = %+v", course.Upcoming)
	}
}

func TestGradeSheetRendering(t *testing.T) {
	now := time.Date(2026, 11, 4, 19, 48, 0, 0, time.UTC)
	work := func(id int64, title, due, status string, score *string) ParentWorkDueItem {
		return ParentWorkDueItem{ParentReportItem: ParentReportItem{AssignmentID: id, Title: title, DueAt: due}, Status: status, Possible: "100", Current: score}
	}
	course := ParentReportCourse{Course: "ELA7", Timezone: "America/Chicago",
		WorkDue: []ParentWorkDueItem{
			work(2, "Essay", "2026-10-28T16:00:00-05:00", "graded", str("85")),
			{ParentReportItem: ParentReportItem{AssignmentID: 1, Title: "Vocab", DueAt: "2026-10-20T16:00:00-05:00"}, Status: "no_grade", Possible: "100", Submission: "not_turned_in"},
			work(3, "Hidden", "2026-10-30T16:00:00-05:00", "grade_not_released", nil),
		},
		Upcoming: []ParentUpcomingItem{{ParentReportItem: ParentReportItem{AssignmentID: 4, Title: "Compare ECR", DueAt: "2026-11-06T16:00:00-06:00"}, TurnedIn: true}},
	}
	student := ParentReportStudent{Ref: "s-1", Alias: "Wren", Courses: []ParentReportCourse{course}}
	run := &ParentRun{Report: ParentReportPacket{AsOf: now.Format(time.RFC3339), Students: []ParentReportStudent{student}}, students: map[string]ParentReportStudent{"s-1": student}}
	html := run.RenderStudent(ParentStudentDraft{Ref: "s-1", StudentIntro: "Hi {{student_name}}, here is your Term 2 grade sheet."}, workspace.Student{Name: "Ada Lovelace", FirstName: "Ada"}).HTML
	order := []string{"Vocab", "Not turned in", "Essay", "85 / 100", "Hidden", "Grade not released", "Compare ECR", "(check this off once completed)", "Canvas shows this turned in"}
	at := 0
	for _, s := range order {
		i := strings.Index(html[at:], s)
		if i < 0 {
			t.Fatalf("%q missing or out of order in %s", s, html)
		}
		at += i
	}
	if !strings.Contains(html, "Ada Lovelace") || !strings.Contains(html, "Hi Ada,") || strings.Contains(html, "<a href") {
		t.Fatal(html)
	}
}
