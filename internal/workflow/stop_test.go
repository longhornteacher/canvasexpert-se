package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

// A stop request leaves every unstarted student untouched; a rerun after the stop completes them.
func TestPostStopsBeforeTheNextStudentAndARerunContinues(t *testing.T) {
	live := fixtureSubmission(5)
	live.EnteredScore, live.Score = nil, nil
	gradeWrites := 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "submissions/5") {
			if r.Method == "PUT" {
				var request struct {
					Submission map[string]any `json:"submission"`
					Comment    struct {
						Text    string `json:"text_comment"`
						Attempt int    `json:"attempt"`
					} `json:"comment"`
				}
				_ = json.NewDecoder(r.Body).Decode(&request)
				if v, ok := request.Submission["posted_grade"]; ok {
					gradeWrites++
					n := json.Number(v.(string))
					live.EnteredScore, live.Score = &n, &n
				}
				if request.Comment.Text != "" {
					a := request.Comment.Attempt
					live.Comments = append(live.Comments, SubmissionComment{ID: 70, Comment: request.Comment.Text, Attempt: &a})
				}
			}
			_ = json.NewEncoder(w).Encode(live)
			return
		}
		_ = json.NewEncoder(w).Encode(noRubricAssignment())
	})
	path := writeNoRubricResults(t, s, overallBasis())

	RequestStop()
	t.Cleanup(ResetStop)
	report, err := s.Post(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if report.StoppedBefore != 1 || gradeWrites != 0 || len(report.Students) != 1 || report.Students[0].Status == "completed" {
		t.Fatalf("a stopped post must not start the student: %+v writes=%d", report, gradeWrites)
	}

	ResetStop()
	report, err = s.Post(context.Background(), path)
	if err != nil || report.StoppedBefore != 0 || gradeWrites != 1 || report.Students[0].Status != "completed" {
		t.Fatalf("the rerun should continue where the stop left off: %+v writes=%d err=%v", report, gradeWrites, err)
	}
}

// quizCreateServer is the fake Canvas used by the marker tests. listed is what the quiz list
// returns; every create request is counted.
func quizCreateServer(t *testing.T, listed string, creates, itemPosts *int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/quiz/v1/courses/1/quizzes":
			_, _ = io.WriteString(w, listed)
		case r.Method == "POST" && r.URL.Path == "/api/quiz/v1/courses/1/quizzes":
			*creates++
			_, _ = io.WriteString(w, `{"id":"55"}`)
		case r.Method == "POST" && r.URL.Path == "/api/quiz/v1/courses/1/quizzes/55/items":
			*itemPosts++
			_, _ = io.WriteString(w, `{"id":"1"}`)
		case r.Method == "GET" && r.URL.Path == "/api/quiz/v1/courses/1/quizzes/55/items":
			_, _ = io.WriteString(w, `[]`)
		case r.Method == "PUT" && r.URL.Path == "/api/v1/courses/1/assignments/55":
			_, _ = io.WriteString(w, `{"id":55,"published":true}`)
		case r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/modules/9/items":
			_, _ = io.WriteString(w, `[]`)
		case r.Method == "POST" && r.URL.Path == "/api/v1/courses/1/modules/9/items":
			_, _ = io.WriteString(w, `{"id":1}`)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	}
}

// A stop (or kill) between the create request and the saved ID leaves a "creating" marker; the
// rerun adopts the one matching quiz instead of holding on "already exists" or duplicating it.
func TestPushQuizAdoptsTheQuizLeftByAStopBetweenCreateAndSavedID(t *testing.T) {
	var creates, itemPosts int
	s, _ := workflowFixture(t, quizCreateServer(t, `[{"id":"55","title":"Fear Check"}]`, &creates, &itemPosts))
	path := writeQF(t, qfFixture)
	if err := os.WriteFile(sidecarPath(path), []byte(`{"creating":"Fear Check"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := s.PushQuiz(t.Context(), "TEST", path)
	if err != nil || report.Status != "completed" || report.AssignmentID != 55 || creates != 0 || itemPosts != 8 {
		t.Fatalf("the earlier quiz should be adopted and finished: %+v err=%v creates=%d items=%d", report, err, creates, itemPosts)
	}
	if rec := readSidecar(path); rec.AssignmentID != 55 || rec.Creating != "" {
		t.Fatalf("the marker should be replaced by the saved ID: %+v", rec)
	}
	if _, err := os.Stat(sidecarPath(path) + ".tmp"); err == nil {
		t.Fatal("the atomic write left its temp file behind")
	}
}

func TestPushQuizMarkerWithNoMatchingQuizCreatesNormally(t *testing.T) {
	var creates, itemPosts int
	s, _ := workflowFixture(t, quizCreateServer(t, `[{"id":"3","title":"Other quiz"}]`, &creates, &itemPosts))
	path := writeQF(t, qfFixture)
	if err := os.WriteFile(sidecarPath(path), []byte(`{"creating":"Fear Check"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := s.PushQuiz(t.Context(), "TEST", path)
	if err != nil || report.Status != "completed" || creates != 1 {
		t.Fatalf("the create never landed, so it should be created once: %+v err=%v creates=%d", report, err, creates)
	}
}

func TestPushQuizMarkerWithSeveralMatchesHolds(t *testing.T) {
	var creates, itemPosts int
	s, _ := workflowFixture(t, quizCreateServer(t, `[{"id":"55","title":"Fear Check"},{"id":"56","title":"Fear Check"}]`, &creates, &itemPosts))
	path := writeQF(t, qfFixture)
	if err := os.WriteFile(sidecarPath(path), []byte(`{"creating":"Fear Check"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PushQuiz(t.Context(), "TEST", path); err == nil || !strings.Contains(err.Error(), "several") || creates != 0 || itemPosts != 0 {
		t.Fatalf("ambiguous earlier push must hold without writing: %v creates=%d items=%d", err, creates, itemPosts)
	}
}

// A typed held error maps to exit code 3 whatever its wording.
func TestHeldErrorIsRecognizedThroughWrapping(t *testing.T) {
	err := fmt.Errorf("context: %w", &HeldError{Msg: "anything at all"})
	if !IsHeld(err) || IsHeld(errors.New("held: looks held but is not typed")) {
		t.Fatal("IsHeld must follow the type, not the message")
	}
}
