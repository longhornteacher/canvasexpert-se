package workflow

import (
	"context"
	"fmt"

	"canvasexpertse/internal/domain"
)

// timingMatches reports whether Canvas already carries the agent's late timing.
func timingMatches(sub Submission, t domain.LateTiming) bool {
	if sub.LatePolicyStatus == nil || *sub.LatePolicyStatus != t.Status {
		return false
	}
	if t.SecondsLate == 0 {
		return !sub.Late
	}
	return sub.SecondsLate == t.SecondsLate
}

// postLate writes the lateness the agent worked out. The Engine only checks that
// the attempt and due date are still the ones the agent used.
func (s *Service) postLate(ctx context.Context, doc domain.ResultsDocument, userID int64, g *domain.GradeOperation, save func() error) error {
	return s.postLateTiming(ctx, doc.Assignment, userID, g.Attempt, g.LateTiming, &g.Outcomes.Late, save)
}

func (s *Service) postLateTiming(ctx context.Context, assignment domain.AssignmentSnapshot, userID int64, attempt domain.AttemptRef, target *domain.LateTiming, outcome *domain.ActionOutcome, save func() error) error {
	hold := func(message string) error { holdOutcome(outcome, message); return save() }
	if target == nil {
		return nil
	}
	live, e := GetSubmission(ctx, s.Client, assignment.CourseID, assignment.AssignmentID, userID)
	if e != nil {
		return hold(e.Error())
	}
	if !sameAttempt(attempt, attemptRef(live)) {
		return hold("new submission before late timing adjustment")
	}
	if live.CachedDueDate == nil || !sameDate(live.CachedDueDate, &target.DueAt) {
		return hold("due date changed since the agent worked out lateness; redo timing")
	}
	if timingMatches(live, *target) {
		*outcome = domain.ActionOutcome{Status: "preserved", Message: "requested lateness timing is present"}
		return save()
	}
	if ambiguous(*outcome) {
		outcome.Status = "uncertain"
		outcome.Message = "late timing completion remains uncertain; different live timing is not proof of failure"
		return save()
	}
	*outcome = domain.ActionOutcome{Status: "pending"}
	if e = save(); e != nil {
		return e
	}
	fields := map[string]any{"late_policy_status": target.Status}
	if target.Status == "late" {
		fields["seconds_late_override"] = target.SecondsLate
	}
	var response Submission
	e = s.Client.Write(ctx, "PUT", fmt.Sprintf("/api/v1/courses/%d/assignments/%d/submissions/%d", assignment.CourseID, assignment.AssignmentID, userID), map[string]any{"submission": fields}, &response)
	if e != nil {
		*outcome = failedOutcome(e)
	} else if sameAttempt(attempt, attemptRef(response)) && timingMatches(response, *target) {
		*outcome = domain.ActionOutcome{Status: "succeeded"}
	} else {
		*outcome = domain.ActionOutcome{Status: "uncertain", Message: "response did not confirm requested timing on intended attempt"}
	}
	return save()
}
