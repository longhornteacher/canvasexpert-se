package workflow

import (
	"context"

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
	hold := func(message string) error { holdOutcome(&g.Outcomes.Late, message); return save() }
	target := g.LateTiming
	if target == nil {
		return nil
	}
	live, e := GetSubmission(ctx, s.Client, doc.Assignment.CourseID, doc.Assignment.AssignmentID, userID)
	if e != nil {
		return hold(e.Error())
	}
	if !sameAttempt(g.Attempt, attemptRef(live)) {
		return hold("new submission before late timing adjustment")
	}
	if live.CachedDueDate == nil || !sameDate(live.CachedDueDate, &target.DueAt) {
		return hold("due date changed since the agent worked out lateness; redo timing")
	}
	if timingMatches(live, *target) {
		g.Outcomes.Late = domain.ActionOutcome{Status: "preserved", Message: "requested lateness timing is present"}
		return save()
	}
	if ambiguous(g.Outcomes.Late) {
		g.Outcomes.Late.Status = "uncertain"
		g.Outcomes.Late.Message = "late timing completion remains uncertain; different live timing is not proof of failure"
		return save()
	}
	g.Outcomes.Late = domain.ActionOutcome{Status: "pending"}
	if e = save(); e != nil {
		return e
	}
	fields := map[string]any{"late_policy_status": target.Status}
	if target.Status == "late" {
		fields["seconds_late_override"] = target.SecondsLate
	}
	var response Submission
	e = s.Client.Write(ctx, "PUT", submissionPath(doc, userID), map[string]any{"submission": fields}, &response)
	if e != nil {
		g.Outcomes.Late = failedOutcome(e)
	} else if sameAttempt(g.Attempt, attemptRef(response)) && timingMatches(response, *target) {
		g.Outcomes.Late = domain.ActionOutcome{Status: "succeeded"}
	} else {
		g.Outcomes.Late = domain.ActionOutcome{Status: "uncertain", Message: "response did not confirm requested timing on intended attempt"}
	}
	return save()
}
