package workflow

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"canvasexpertse/internal/domain"
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
