package workflow

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListSessionReportsAssignmentsWithoutFetchingWork(t *testing.T) {
	zero, two := 0, 2
	groupID := int64(7)
	assignments := make([]Assignment, 3)
	for i := range assignments {
		assignments[i] = fixtureAssignment()
		assignments[i].ID = int64(10 + i)
		assignments[i].AssignmentGroupID = &groupID
		assignments[i].NeedsGradingCount = &two
	}
	assignments[1].NeedsGradingCount = &zero
	assignments[2].NeedsGradingCount = nil
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/courses/1/enrollments":
			_ = json.NewEncoder(w).Encode([]Enrollment{{UserID: 5, Type: "StudentEnrollment", State: "active", User: CanvasUser{ID: 5, Name: "Synthetic Student"}}})
		case "/api/v1/courses/1/assignments":
			_ = json.NewEncoder(w).Encode(assignments)
		case "/api/v1/courses/1/assignment_groups":
			_ = json.NewEncoder(w).Encode([]AssignmentGroup{{ID: 7, Name: "Daily", Weight: "40"}})
		default:
			t.Errorf("list mode must not make per-assignment requests: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	})
	report, err := s.PullWorkWithOptions(context.Background(), "TEST", "", PullWorkOptions{Session: true, List: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.Partial || len(report.Packets) != 0 || report.Students != 0 || len(report.Assignments) != 3 {
		t.Fatalf("list report: %+v", report)
	}
	want := []string{"listed", "skipped", "listed"}
	for i, row := range report.Assignments {
		if row.Outcome != want[i] || row.AssignmentGroupName != "Daily" {
			t.Fatalf("row %d: %+v", i, row)
		}
	}
	if report.Assignments[1].SkipReason != "Canvas reports no work needing grading" || !report.Assignments[2].NeedsGradingCountUnknown {
		t.Fatalf("zero/unknown counts: %+v", report.Assignments)
	}
	if entries, _ := filepath.Glob(filepath.Join(s.Root, "courses", "TEST", "assignments", "*")); len(entries) > 0 {
		t.Fatalf("list mode wrote assignment files: %v", entries)
	}
	if _, err := os.Stat(filepath.Join(s.Root, "courses", "TEST", "assignments")); err == nil {
		t.Fatal("list mode created the assignments folder")
	}
}

func TestListRequiresSessionAndNoAssignmentOrGroup(t *testing.T) {
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("a rejected list request must not reach Canvas: %s", r.URL.Path)
		http.NotFound(w, r)
	})
	ctx := context.Background()
	if _, err := s.PullWorkWithOptions(ctx, "TEST", "", PullWorkOptions{List: true}); err == nil || !strings.Contains(err.Error(), "list applies only") {
		t.Fatalf("--list without --session: %v", err)
	}
	if _, err := s.PullWorkWithOptions(ctx, "TEST", "Quiz", PullWorkOptions{Session: true, List: true}); err == nil {
		t.Fatal("--list with an assignment was accepted")
	}
	if _, err := s.PullWorkGroupWithOptions(ctx, "TEST", "Quiz", 3, PullWorkOptions{Session: true, List: true}); err == nil {
		t.Fatal("--list with a group was accepted")
	}
}
