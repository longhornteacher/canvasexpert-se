package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSessionPullIsolatesAssignmentFailures(t *testing.T) {
	for _, failure := range []string{"report", "submission", "identity", "attempt"} {
		t.Run(failure, func(t *testing.T) {
			one := 1
			assignments := make([]Assignment, 3)
			for i := range assignments {
				assignments[i] = fixtureAssignment()
				assignments[i].ID = int64(10 + i)
				assignments[i].Name = fmt.Sprintf("Synthetic assignment %d", 10+i)
				assignments[i].NeedsGradingCount = &one
			}
			assignments[1].NewQuiz = failure == "report" || failure == "attempt"
			var origin string
			s, server := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case strings.HasSuffix(r.URL.Path, "/enrollments"):
					_ = json.NewEncoder(w).Encode([]Enrollment{
						{UserID: 5, Type: "StudentEnrollment", State: "active", User: CanvasUser{ID: 5}},
						{UserID: 6, Type: "StudentEnrollment", State: "active", User: CanvasUser{ID: 6}},
						{UserID: 9, Type: "StudentEnrollment", State: "inactive", User: CanvasUser{ID: 9}},
					})
				case r.URL.Path == "/api/v1/courses/1/assignments":
					_ = json.NewEncoder(w).Encode(assignments)
				case strings.HasSuffix(r.URL.Path, "/assignment_groups"):
					fmt.Fprint(w, `[]`)
				case r.URL.Path == "/api/quiz/v1/courses/1/quizzes/11/reports":
					fmt.Fprint(w, `{"progress":{"url":"/api/quiz/v1/reports/7"}}`)
				case r.URL.Path == "/api/quiz/v1/reports/7":
					if failure == "attempt" {
						fmt.Fprint(w, `{"workflow_state":"completed","results":{"url":"`+origin+`/signed/report"}}`)
					} else {
						fmt.Fprint(w, `{"workflow_state":"completed"}`)
					}
				case r.URL.Path == "/signed/report":
					fmt.Fprint(w, `[{"student_data":{"id":5,"attempt":99},"item_responses":[{"item_id":21,"answer":"Synthetic response"}]}]`)
				case strings.HasSuffix(r.URL.Path, "/items"):
					fmt.Fprint(w, `[{"id":21,"points_possible":1,"entry":{"id":121,"interaction_type_slug":"essay","item_body":"Explain"}}]`)
				default:
					var id int
					if _, err := fmt.Sscanf(r.URL.Path, "/api/v1/courses/1/assignments/%d", &id); err != nil || id < 10 || id > 12 {
						t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
						http.NotFound(w, r)
						return
					}
					if strings.HasSuffix(r.URL.Path, "/submissions") {
						if id == 11 && failure == "submission" {
							w.WriteHeader(http.StatusForbidden)
							return
						}
						present, inactive, missing := fixtureSubmission(5), fixtureSubmission(9), fixtureSubmission(6)
						present.AssignmentID, inactive.AssignmentID, missing.AssignmentID = int64(id), int64(id), int64(id)
						missing.SubmittedAt, missing.Attempt = nil, 0
						if id == 11 && failure == "identity" {
							present.AssignmentID = 999
						}
						_ = json.NewEncoder(w).Encode([]Submission{present, inactive, missing})
					} else {
						_ = json.NewEncoder(w).Encode(assignments[id-10])
					}
				}
			})
			origin = server.URL
			unsent := filepath.Join(s.Root, "courses", "TEST", "unsent-results.md")
			if err := os.MkdirAll(filepath.Dir(unsent), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(unsent, []byte("private synthetic unfinished work"), 0600); err != nil {
				t.Fatal(err)
			}
			report, err := s.PullWorkWithOptions(context.Background(), "TEST", "", PullWorkOptions{Session: true})
			var partial *PullIncompleteError
			if !errors.As(err, &partial) || partial.Failed != 1 || !report.Partial || len(report.Assignments) != 3 {
				t.Fatalf("failure was not an explicit partial result: %+v %v", report, err)
			}
			if report.Students != 2 || len(report.Packets) != 2 {
				t.Fatalf("independent packets lost: %+v", report)
			}
			for _, i := range []int{0, 2} {
				a := report.Assignments[i]
				if a.Outcome != "complete" || a.Students != 1 || !a.CandidatesKnown || a.Candidates != 1 || a.Inactive != 1 || a.Missing != 1 || len(a.Packets) != 1 {
					t.Fatalf("counts/filter reasons lost: %+v", a)
				}
				if _, err := os.Stat(a.Packets[0]); err != nil {
					t.Fatal(err)
				}
			}
			bad := report.Assignments[1]
			if bad.Outcome != "failed" || bad.FailurePhase == "" || bad.FailureReason == "" || bad.Students != 0 || len(bad.Packets) != 0 {
				t.Fatalf("failed assignment not identified: %+v", bad)
			}
			if failure == "report" && (!bad.CandidatesKnown || bad.Candidates != 1 || !strings.Contains(bad.FailureReason, "completed_without_results_url")) {
				t.Fatalf("report facts lost: %+v", bad)
			}
			if (failure == "submission" || failure == "identity") && bad.CandidatesKnown {
				t.Fatalf("failed discovery claimed eligible count: %+v", bad)
			}
			if failure == "attempt" && bad.UnmatchedAttempts != 1 {
				t.Fatalf("all-unmatched attempts became empty success: %+v", bad)
			}
			b, err := os.ReadFile(unsent)
			if err != nil || string(b) != "private synthetic unfinished work" {
				t.Fatal("unsent work changed")
			}
		})
	}
}

func TestSessionPullCancellationMarksKnownAssignmentsNotReached(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	one := 1
	a := fixtureAssignment()
	a.NeedsGradingCount = &one
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/enrollments"):
			fmt.Fprint(w, `[{"user_id":5,"type":"StudentEnrollment","enrollment_state":"active","user":{"id":5}}]`)
		case strings.HasSuffix(r.URL.Path, "/assignments"):
			_ = json.NewEncoder(w).Encode([]Assignment{a})
		case strings.HasSuffix(r.URL.Path, "/assignment_groups"):
			fmt.Fprint(w, `[]`)
		case strings.HasSuffix(r.URL.Path, "/submissions"):
			cancel()
			fmt.Fprint(w, `[]`)
		default:
			t.Errorf("unexpected fetch after cancellation: %s", r.URL.Path)
		}
	})
	report, err := s.PullWorkWithOptions(ctx, "TEST", "", PullWorkOptions{Session: true})
	if !errors.Is(err, context.Canceled) || !report.Partial || len(report.Assignments) != 1 || report.Assignments[0].Outcome != "not-reached" || report.Assignments[0].CandidatesKnown || !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("cancellation became empty success: %+v %v", report, err)
	}
}
