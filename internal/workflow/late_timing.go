package workflow

import (
	"context"
	"fmt"
	"strings"
	"time"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

// timingMatches reports whether Canvas already carries the agent's late timing.
func timingMatches(sub Submission, t domain.LateTiming) bool {
	if !sub.LateKnown || !sub.SecondsLateKnown || !sub.LatePolicyStatusKnown {
		return false
	}
	if sub.LatePolicyStatus != nil && *sub.LatePolicyStatus != t.Status {
		return false
	}
	if t.Status == "none" {
		return !sub.Late
	}
	return sub.Late && sub.SecondsLate == t.SecondsLate
}

func timingMatchesBaseline(sub Submission, t *domain.ObservedTiming) bool {
	if t == nil || !t.StatusKnown || !t.LateKnown || !t.SecondsLateKnown || !sub.LateKnown || !sub.SecondsLateKnown || !sub.LatePolicyStatusKnown {
		return false
	}
	if sub.Late != t.Late || sub.SecondsLate != t.SecondsLate {
		return false
	}
	if t.LatePolicyStatus == nil {
		if sub.LatePolicyStatus != nil {
			return false
		}
	} else if sub.LatePolicyStatus == nil || *sub.LatePolicyStatus != *t.LatePolicyStatus {
		return false
	}
	if t.CustomStatusKnown {
		return hasCustomStatus(sub) == t.HasCustomStatus
	}
	return true
}

func timingMatchesBaselineFromWork(target, observed *domain.ObservedTiming) bool {
	if target == nil || observed == nil || !knownObservedTiming(target) || !knownObservedTiming(observed) {
		return false
	}
	if target.Late != observed.Late || target.SecondsLate != observed.SecondsLate {
		return false
	}
	if target.LatePolicyStatus == nil {
		if observed.LatePolicyStatus != nil {
			return false
		}
	} else if observed.LatePolicyStatus == nil || *target.LatePolicyStatus != *observed.LatePolicyStatus {
		return false
	}
	return !target.CustomStatusKnown || (observed.CustomStatusKnown && target.HasCustomStatus == observed.HasCustomStatus)
}

func knownObservedTiming(t *domain.ObservedTiming) bool {
	return t != nil && t.StatusKnown && t.LateKnown && t.SecondsLateKnown
}

func hasCustomStatus(sub Submission) bool {
	value := strings.TrimSpace(string(sub.CustomGradeStatusID))
	return value != "" && value != "null"
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
	if live.UserID != userID || (live.AssignmentID != 0 && live.AssignmentID != assignment.AssignmentID) {
		return hold("live submission identity disagrees with the requested student and assignment")
	}
	if s.scoring != nil {
		if reason := gradeVisibleHold(*s.scoring, live); reason != "" {
			return hold(reason)
		}
	}
	if !sameAttempt(attempt, attemptRef(live)) {
		return hold("new submission before late timing adjustment")
	}
	if live.CachedDueDate == nil || !sameDate(live.CachedDueDate, &target.DueAt) {
		return hold("due date changed since the agent worked out lateness; redo timing")
	}
	if live.Excused || (live.AssignmentVisible != nil && !*live.AssignmentVisible) {
		return hold("submission is excused or the assignment is no longer visible; timing was left unchanged")
	}
	if reason := s.schoolTimeHold(live, target); reason != "" {
		return hold(reason)
	}
	if target.Observed != nil && (!live.LateKnown || !live.SecondsLateKnown || !live.LatePolicyStatusKnown) {
		return hold("Canvas did not return the current lateness fields; timing was left unchanged")
	}
	if timingMatches(live, *target) {
		*outcome = domain.ActionOutcome{Status: "preserved", Message: "requested lateness timing is present"}
		return save()
	}
	if hasCustomStatus(live) {
		return hold("Canvas has a custom grading status; late timing was left unchanged because changing it would clear that status")
	}
	if target.Observed != nil && !timingMatchesBaseline(live, target.Observed) {
		return hold("Canvas timing changed since it was pulled; refresh the timing baseline before adjusting it")
	}
	if ambiguous(*outcome) {
		outcome.Status = "uncertain"
		outcome.Message = "late timing completion remains uncertain; different live timing is not proof of failure"
		return save()
	}
	return s.writeLateTiming(ctx, assignment, userID, target, outcome, save)
}

// Callers have checked the full action audience and timing evidence. Shared
// actions must reconcile all members before reaching this single-route write.
func (s *Service) writeLateTiming(ctx context.Context, assignment domain.AssignmentSnapshot, userID int64, target *domain.LateTiming, outcome *domain.ActionOutcome, save func() error) error {
	*outcome = domain.ActionOutcome{Status: "pending"}
	if e := save(); e != nil {
		return e
	}
	fields := map[string]any{"late_policy_status": target.Status}
	if target.Status == "late" {
		fields["seconds_late_override"] = target.SecondsLate
	}
	e := s.Client.WriteAccepted(ctx, "PUT", fmt.Sprintf("/api/v1/courses/%d/assignments/%d/submissions/%d", assignment.CourseID, assignment.AssignmentID, userID), map[string]any{"submission": fields}, nil)
	if e != nil {
		*outcome = failedOutcome(e)
	} else {
		*outcome = domain.ActionOutcome{Status: "succeeded"}
	}
	return save()
}

func (s *Service) schoolTimeHold(live Submission, target *domain.LateTiming) string {
	if target == nil || target.SchoolTime == nil {
		return ""
	}
	p := target.SchoolTime
	var counted *Submission
	candidates := append([]Submission(nil), live.History...)
	if live.Attempt > 0 {
		candidates = append(candidates, live)
	}
	for i := range candidates {
		candidate := &candidates[i]
		if candidate.Attempt == p.Attempt.Attempt && candidate.SubmittedAt != nil {
			at, err := time.Parse(time.RFC3339Nano, *candidate.SubmittedAt)
			provenanceAt, provenanceErr := time.Parse(time.RFC3339Nano, p.SubmittedAt)
			if err == nil && provenanceErr == nil && at.Equal(provenanceAt) {
				counted = candidate
				break
			}
		}
	}
	if counted == nil {
		return "first genuine submission used for school-time counting is no longer present in Canvas history; refresh the pull and recompute timing"
	}
	deadline, err := time.Parse(time.RFC3339Nano, p.Deadline)
	if err != nil {
		return "school-time deadline is invalid; recompute timing"
	}
	submitted, err := time.Parse(time.RFC3339Nano, p.SubmittedAt)
	if err != nil {
		return "school-time submission time is invalid; recompute timing"
	}
	var result workspace.SchoolTimeResult
	if submitted.After(deadline) {
		calendarData, readErr := workspace.ReadSchoolCalendar(s.Root)
		if readErr != nil {
			return "school calendar is unavailable; recompute timing before posting"
		}
		if err := calendarData.Validate(); err != nil {
			return "school calendar is invalid; recompute timing before posting"
		}
		result, err = calendarData.CountSchoolTime(deadline, submitted)
		if err != nil {
			return "school calendar no longer covers this timing interval; recompute timing before posting"
		}
	} else {
		result = workspace.SchoolTimeResult{Status: "none"}
	}
	if result.Status != p.Status || result.SecondsLate != p.SecondsLate || !sameSchoolTimeDates(result.SkippedDates, p.SkippedDates) {
		return "school calendar changed since lateness was calculated; recompute timing before posting"
	}
	return ""
}

func sameSchoolTimeDates(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
