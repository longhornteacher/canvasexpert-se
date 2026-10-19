package workflow

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

// lateFixture serves one submission whose current attempt is due at `due`. No
// school calendar exists anywhere in the workspace.
func lateFixture(t *testing.T, due string) (*Service, *Submission, *int) {
	t.Helper()
	live := fixtureSubmission(5)
	live.CachedDueDate = &due
	raw := json.Number("15")
	live.EnteredScore, live.Score = &raw, &raw
	writes := new(int)
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PUT" {
			*writes++
			var req struct {
				Submission map[string]json.RawMessage `json:"submission"`
			}
			if e := json.NewDecoder(r.Body).Decode(&req); e != nil {
				t.Error(e)
			}
			for key := range req.Submission {
				if key != "late_policy_status" && key != "seconds_late_override" {
					t.Error("timing changed unrelated field", key)
				}
			}
			var status string
			_ = json.Unmarshal(req.Submission["late_policy_status"], &status)
			live.LatePolicyStatus = &status
			live.Late = status == "late"
			if status == "late" {
				var seconds int64
				_ = json.Unmarshal(req.Submission["seconds_late_override"], &seconds)
				live.SecondsLate = seconds
			} else if req.Submission["seconds_late_override"] != nil {
				t.Error("on-time must not override raw seconds")
			}
		}
		_ = json.NewEncoder(w).Encode(live)
	})
	return s, &live, writes
}

func postLateFor(t *testing.T, s *Service, live *Submission, g *domain.GradeOperation) {
	t.Helper()
	doc := domain.ResultsDocument{Assignment: domain.AssignmentSnapshot{CourseID: 1, AssignmentID: 10}}
	g.Attempt = attemptRef(*live)
	if e := s.postLate(context.Background(), doc, 5, g, func() error { return nil }); e != nil {
		t.Fatal(e)
	}
}

func TestLateTimingOnTimeNeedsNoCalendar(t *testing.T) {
	due := "2026-10-08T18:00:00-05:00"
	s, live, writes := lateFixture(t, due)
	late := "late"
	live.Late, live.LatePolicyStatus, live.SecondsLate = true, &late, 999
	g := &domain.GradeOperation{LateTiming: &domain.LateTiming{Status: "none", DueAt: due, Basis: "Submitted before the due time."}}
	postLateFor(t, s, live, g)
	if *writes != 1 || g.Outcomes.Late.Status != "succeeded" || live.Late || *live.LatePolicyStatus != "none" {
		t.Fatalf("bad outcome %+v writes=%d", g.Outcomes.Late, *writes)
	}
}

func TestLateTimingWritesAgentSeconds(t *testing.T) {
	due := "2026-10-08T18:00:00-05:00"
	s, live, writes := lateFixture(t, due)
	g := &domain.GradeOperation{LateTiming: &domain.LateTiming{Status: "late", SecondsLate: 97200, DueAt: due, Basis: "27 school hours."}}
	postLateFor(t, s, live, g)
	if *writes != 1 || g.Outcomes.Late.Status != "succeeded" || live.SecondsLate != 97200 || live.EnteredScore.String() != "15" {
		t.Fatalf("bad outcome %+v writes=%d", g.Outcomes.Late, *writes)
	}
	// Recovery never replays: matching live timing is preserved, differing stays uncertain.
	g.Outcomes.Late.Status = "uncertain"
	postLateFor(t, s, live, g)
	if *writes != 1 || g.Outcomes.Late.Status != "preserved" {
		t.Fatal("uncertain timing was not recovered without replay")
	}
	live.SecondsLate = 42
	g.Outcomes.Late.Status = "uncertain"
	postLateFor(t, s, live, g)
	if *writes != 1 || g.Outcomes.Late.Status != "uncertain" {
		t.Fatal("ambiguous timing blindly replayed")
	}
}

func TestLateTimingHeldWhenDueDateChanged(t *testing.T) {
	s, live, writes := lateFixture(t, "2026-10-09T18:00:00-05:00")
	g := &domain.GradeOperation{LateTiming: &domain.LateTiming{Status: "late", SecondsLate: 3600, DueAt: "2026-10-08T18:00:00-05:00", Basis: "x"}}
	postLateFor(t, s, live, g)
	if *writes != 0 || g.Outcomes.Late.Status != "held" || !strings.Contains(g.Outcomes.Late.Message, "due date") {
		t.Fatalf("changed due date was ignored: %+v", g.Outcomes.Late)
	}
}

func TestLateTimingPreservesMatchingAutomaticStatus(t *testing.T) {
	due := "2026-10-08T18:00:00-05:00"
	s, live, writes := lateFixture(t, due)
	g := &domain.GradeOperation{LateTiming: &domain.LateTiming{Status: "none", DueAt: due, Basis: "on time"}}
	postLateFor(t, s, live, g)
	if *writes != 0 || g.Outcomes.Late.Status != "preserved" {
		t.Fatalf("matching automatic timing should be preserved without a write: %+v writes=%d", g.Outcomes.Late, *writes)
	}
}

func TestLateTimingNoneMatchesCanvasRawElapsedSeconds(t *testing.T) {
	status := "none"
	live := Submission{LateKnown: true, Late: false, LatePolicyStatusKnown: true, LatePolicyStatus: &status, SecondsLateKnown: true, SecondsLate: 86400}
	if !timingMatches(live, domain.LateTiming{Status: "none"}) {
		t.Fatal("on-time status should match when Canvas retains raw elapsed seconds")
	}
}

func TestLateTimingRejectsClassicQuizAndCheckpointAssignments(t *testing.T) {
	quizID := int64(9)
	if got := lateTimingAssignmentHold(Assignment{QuizID: &quizID}, false); !strings.Contains(got, "quizzes") {
		t.Fatalf("classic quiz should be held explicitly, got %q", got)
	}
	hasSubAssignments := true
	if got := lateTimingAssignmentHold(Assignment{HasSubAssignments: &hasSubAssignments}, false); !strings.Contains(got, "checkpoint") {
		t.Fatalf("checkpoint assignment should be held explicitly, got %q", got)
	}
}

func TestLateOnlyHoldsInterveningTimingEdit(t *testing.T) {
	due := "2026-10-08T18:00:00-05:00"
	s, live, writes := lateFixture(t, due)
	manual := "late"
	live.Late, live.LatePolicyStatus, live.SecondsLate = true, &manual, 60
	baseline := &domain.ObservedTiming{StatusKnown: true, LateKnown: true, SecondsLateKnown: true, Late: false, SecondsLate: 0}
	target := &domain.LateTiming{Status: "late", SecondsLate: 3600, DueAt: due, Basis: "school time", Observed: baseline}
	outcome := domain.ActionOutcome{}
	if err := s.postLateTiming(context.Background(), domain.AssignmentSnapshot{CourseID: 1, AssignmentID: 10}, 5, attemptRef(*live), target, &outcome, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if *writes != 0 || outcome.Status != "held" || !strings.Contains(outcome.Message, "changed since it was pulled") {
		t.Fatalf("intervening timing edit was overwritten: outcome=%+v writes=%d", outcome, *writes)
	}
}

func TestLateOnlyPostsNarrowTimingPatchAndAcceptsSparseSuccess(t *testing.T) {
	due := "2026-10-08T18:00:00Z"
	puts, gets := 0, 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/courses/1/assignments/10":
			a := fixtureAssignment()
			a.Name = "A later unrelated title"
			a.Rubric[0].Description = "A later unrelated rubric edit"
			_ = json.NewEncoder(w).Encode(a)
		case "/api/v1/courses/1/assignments/10/submissions/5":
			if r.Method == http.MethodPut {
				puts++
				var payload map[string]json.RawMessage
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Fatal(err)
				}
				if len(payload) != 1 || payload["submission"] == nil {
					t.Fatalf("late-only sent fields outside submission: %s", payload)
				}
				var fields map[string]json.RawMessage
				_ = json.Unmarshal(payload["submission"], &fields)
				if len(fields) != 2 || fields["late_policy_status"] == nil || fields["seconds_late_override"] == nil {
					t.Fatalf("late-only patch was not limited to timing: %s", payload["submission"])
				}
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{}`)) // accepted sparse Canvas response
				return
			}
			gets++
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 12, "user_id": 5, "assignment_id": 10, "attempt": 2, "submitted_at": "2026-10-07T12:00:00Z", "workflow_state": "submitted", "submission_type": "online_text_entry", "cached_due_date": due, "late": false, "late_policy_status": nil, "seconds_late": 0, "excused": false, "assignment_visible": true, "submission_history": []any{}})
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.String())
			http.NotFound(w, r)
		}
	})
	s.Skip = map[string]bool{}
	path := scoringFiles(t, s, 5)
	var doc domain.ResultsDocument
	body, err := workspace.ReadDocument(path, &doc)
	if err != nil {
		t.Fatal(err)
	}
	var packet domain.PacketDocument
	_, _ = workspace.ReadDocument(filepath.Join(s.Root, "courses", "TEST", "packet.md"), &packet)
	var work domain.StudentWorkDocument
	workPath := filepath.Join(s.Root, filepath.FromSlash(packet.Students[0]))
	_, _ = workspace.ReadDocument(workPath, &work)
	statusKnown, secondsKnown := true, true
	work.Current.ObservedTiming = &domain.ObservedTiming{StatusKnown: statusKnown, LateKnown: true, SecondsLateKnown: secondsKnown, Late: false, SecondsLate: 0, LatePolicyStatus: nil}
	if err := workspace.WriteDocument(workPath, work, "work"); err != nil {
		t.Fatal(err)
	}
	doc.Students[0] = domain.StudentResult{StudentRef: doc.Students[0].StudentRef, Operation: "late-only", LateOnly: &domain.LateOnlyOperation{
		Attempt:    work.Current.Ref,
		LateTiming: domain.LateTiming{Status: "late", SecondsLate: 3600, DueAt: due, Basis: "one school hour late", Observed: work.Current.ObservedTiming},
	}}
	if err := workspace.WriteDocument(path, &doc, body); err != nil {
		t.Fatal(err)
	}
	review, err := BuildReview(s.Root, path)
	if err != nil {
		t.Fatal(err)
	}
	if len(review.Cards) != 1 || review.Cards[0].LateBasis != "one school hour late" {
		t.Fatalf("late-only review did not retain the teacher's basis: %+v", review.Cards)
	}
	var currentTiming, proposedTiming bool
	for _, fact := range review.Cards[0].Facts {
		currentTiming = currentTiming || fact.Label == "Current Canvas timing"
		proposedTiming = proposedTiming || fact.Label == "Timing action"
	}
	if !currentTiming || !proposedTiming {
		t.Fatalf("late-only review omitted current or proposed timing facts: %+v", review.Cards[0].Facts)
	}
	s.Skip[doc.Students[0].StudentRef] = true
	skipped, err := s.Post(context.Background(), path)
	if err != nil || puts != 0 || len(skipped.Students) != 1 || skipped.Students[0].Outcomes.Late.Status != "" {
		t.Fatalf("review deselection did not skip the exact timing card: report=%+v puts=%d err=%v", skipped, puts, err)
	}
	s.Skip[doc.Students[0].StudentRef] = false
	report, err := s.Post(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if puts != 1 || gets != 1 || len(report.Students) != 1 || report.Students[0].Status != "completed" || report.Students[0].Outcomes.Late.Status != "succeeded" {
		t.Fatalf("late-only result=%+v puts=%d gets=%d err=%v", report, puts, gets, err)
	}
}
