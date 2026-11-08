package workflow

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"canvasexpertse/internal/workspace"
)

func TestWorkHabitsClassifiesDueItemsByWeek(t *testing.T) {
	loc := time.UTC
	from := time.Date(2026, 11, 2, 0, 0, 0, 0, loc)
	to := time.Date(2026, 11, 15, 23, 59, 59, 0, loc)
	yes := true
	online := []string{"online_upload"}
	mk := func(id int64, due string, types []string) Assignment {
		return Assignment{ID: id, Name: "A", Published: true, GradingType: "points", SubmissionTypes: types, DueAt: str(due), Points: "10"}
	}
	assignments := map[int64]Assignment{
		1: mk(1, "2026-11-03T12:00:00Z", online), // on time
		2: mk(2, "2026-11-04T12:00:00Z", online), // late
		3: mk(3, "2026-11-05T12:00:00Z", online), // missing
		4: mk(4, "2026-11-06T12:00:00Z", online), // excused
		5: mk(5, "2026-11-09T12:00:00Z", []string{"on_paper"}),
		6: mk(6, "2026-11-10T12:00:00Z", online), // stale missing flag, positive grade
		7: mk(7, "2026-11-11T12:00:00Z", online), // override moves due date into window
		8: mk(8, "2026-11-04T12:00:00Z", online), // due before enrollment
		9: mk(9, "2026-11-20T12:00:00Z", online), // outside window
	}
	submitted := func(id int64) Submission {
		return Submission{AssignmentID: id, UserID: 5, AssignmentVisible: &yes, Attempt: 1, SubmittedAt: str("2026-11-03T12:00:00Z")}
	}
	late := submitted(2)
	late.Late = true
	subs := []Submission{
		submitted(1), late,
		{AssignmentID: 3, UserID: 5, AssignmentVisible: &yes, Missing: true},
		{AssignmentID: 4, UserID: 5, AssignmentVisible: &yes, Excused: true},
		{AssignmentID: 5, UserID: 5, AssignmentVisible: &yes},
		{AssignmentID: 6, UserID: 5, AssignmentVisible: &yes, Missing: true, Score: num("8"), EnteredScore: num("8")},
		{AssignmentID: 7, UserID: 5, AssignmentVisible: &yes, CachedDueDate: str("2026-11-12T12:00:00Z"), CachedDueDatePresent: true, Missing: true},
		{AssignmentID: 8, UserID: 5, AssignmentVisible: &yes, Missing: true},
		{AssignmentID: 9, UserID: 5, AssignmentVisible: &yes, Missing: true},
	}
	enrolled := time.Date(2026, 11, 5, 0, 0, 0, 0, loc)
	got := buildWorkHabits("s-test", "Alias", subs, assignments, &enrolled, from, to, loc)

	if len(got.Weeks) != 2 || got.Weeks[0].WeekStart != "2026-11-02" || got.Weeks[1].WeekStart != "2026-11-09" {
		t.Fatalf("weeks = %+v", got.Weeks)
	}
	w1, w2 := got.Weeks[0], got.Weeks[1]
	// Items 1, 2 and 8 are due before the Nov 5 enrollment; they are listed, not counted.
	if len(got.BeforeEnrollment) != 3 {
		t.Fatalf("before_enrollment = %+v", got.BeforeEnrollment)
	}
	if w1.Due != 1 || w1.Missing != 1 || w1.Excused != 1 || w1.OnTime != 0 || w1.Late != 0 {
		t.Fatalf("week 1 = %+v", w1)
	}
	if w2.Due != 3 || w2.NoOnlineSubmission != 1 || w2.GradedNoSubmission != 1 || w2.Missing != 1 {
		t.Fatalf("week 2 = %+v", w2)
	}
}

func TestWorkHabitsCountsOnTimeAndLateAfterEnrollment(t *testing.T) {
	loc := time.UTC
	yes := true
	a1 := Assignment{ID: 1, Name: "A", Published: true, SubmissionTypes: []string{"online_upload"}, DueAt: str("2026-11-03T12:00:00Z"), Points: "10"}
	a2 := a1
	a2.ID = 2
	on := Submission{AssignmentID: 1, UserID: 5, AssignmentVisible: &yes, Attempt: 1, SubmittedAt: str("2026-11-03T11:00:00Z")}
	lt := Submission{AssignmentID: 2, UserID: 5, AssignmentVisible: &yes, Attempt: 1, SubmittedAt: str("2026-11-04T11:00:00Z"), Late: true}
	got := buildWorkHabits("s-test", "Alias", []Submission{on, lt}, map[int64]Assignment{1: a1, 2: a2}, nil,
		time.Date(2026, 11, 1, 0, 0, 0, 0, loc), time.Date(2026, 11, 8, 0, 0, 0, 0, loc), loc)
	if len(got.Weeks) != 1 || got.Weeks[0].Due != 2 || got.Weeks[0].OnTime != 1 || got.Weeks[0].Late != 1 {
		t.Fatalf("weeks = %+v", got.Weeks)
	}
}

func TestWorkHabitsWritesPseudonymizedFactsAndClipsEnrollment(t *testing.T) {
	svc, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("work habits made a write: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/courses/1/enrollments":
			w.Write([]byte(`[{"user_id":9,"type":"StudentEnrollment","enrollment_state":"active","created_at":"2026-08-01T00:00:00Z","user":{"id":9,"name":"Private Name"}},{"user_id":10,"type":"StudentEnrollment","enrollment_state":"active","created_at":"2026-11-05T00:00:00Z","user":{"id":10,"name":"New Private Name"}}]`))
		case "/api/v1/courses/1/assignments":
			w.Write([]byte(`[{"id":1,"name":"Early work","published":true,"grading_type":"points","submission_types":["online_upload"],"due_at":"2026-11-03T12:00:00Z","points_possible":10}]`))
		case "/api/v1/courses/1/students/submissions":
			w.Write([]byte(`[{"user_id":9,"assignment_id":1,"assignment_visible":true,"missing":true},{"user_id":10,"assignment_id":1,"assignment_visible":true,"missing":true}]`))
		default:
			t.Errorf("unexpected Canvas route %s", r.URL.Path)
			http.NotFound(w, r)
		}
	})
	if err := workspace.WriteTeacherDocument(svc.Root, "context/courses.md", svc.Config, ""); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	path, packet, err := svc.WorkHabits(context.Background(), "TEST", time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), time.Time{}, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(filepath.Dir(path)) != filepath.Join(svc.Root, "courses", "TEST", "work-habits") {
		t.Fatalf("record must stay local to UserInstall: %s", path)
	}
	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), "Private Name") {
		t.Fatalf("record must be pseudonymized: %s", raw)
	}
	if len(packet.Students) != 2 {
		t.Fatalf("packet = %+v", packet)
	}
	missing, clipped := 0, 0
	for _, s := range packet.Students {
		for _, w := range s.Weeks {
			missing += w.Missing
		}
		clipped += len(s.BeforeEnrollment)
	}
	if missing != 1 || clipped != 1 {
		t.Fatalf("the established student is missing the item; the new student's is before enrollment: missing=%d clipped=%d", missing, clipped)
	}
}
