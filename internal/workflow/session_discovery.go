package workflow

import (
	"fmt"
	"path/filepath"
	"time"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

func studentRefByCanvasID(students *workspace.StudentStore, instance string, userID int64) string {
	for _, student := range students.Rows {
		if student.Instance == instance && student.UserID == userID {
			return student.Ref
		}
	}
	return ""
}

// completedTimingHint suppresses a second timing packet only when a prior
// accepted CE-SE timing action has structured provenance that still computes
// to the live Canvas timing for the same attempt and effective due date.
func (s *Service) completedTimingHint(assignmentID, courseID int64, studentRef string, sub Submission, a Assignment, calendar *workspace.SchoolCalendar) bool {
	if studentRef == "" || !sub.LateKnown || !sub.SecondsLateKnown || !sub.LatePolicyStatusKnown {
		return false
	}
	due, known := submissionEffectiveDue(sub, a)
	if !known || due == "" {
		return false
	}
	matched := false
	_ = walkResultFiles(s.Root, func(path string) error {
		var results domain.ResultsDocument
		if _, err := workspace.ReadDocument(path, &results); err == nil && results.Schema == 1 && results.Kind == "results" && results.Assignment.Instance == s.Client.Origin() && results.Assignment.CourseID == courseID && results.Assignment.AssignmentID == assignmentID {
			for _, row := range results.Students {
				if row.StudentRef != studentRef {
					continue
				}
				var timing *domain.LateTiming
				accepted := false
				if row.Operation == "grade" && row.Grade != nil && sameAttempt(row.Grade.Attempt, attemptRef(sub)) {
					timing, accepted = row.Grade.LateTiming, row.Grade.Outcomes.Late.Status == "succeeded"
				} else if row.Operation == "late-only" && row.LateOnly != nil && sameAttempt(row.LateOnly.Attempt, attemptRef(sub)) {
					timing, accepted = &row.LateOnly.LateTiming, row.LateOnly.Outcome.Status == "succeeded"
				}
				if accepted && timing != nil && timingSourcePresent(*timing, sub) && timingStillMatches(*timing, due, sub, calendar) {
					matched = true
					return filepath.SkipAll
				}
			}
			return nil
		}
		// Group-results has its own typed contract; tolerate its group wrapper
		// while still requiring exact course, assignment, student and attempt.
		var groupResults domain.GroupResultsDocument
		if _, err := workspace.ReadDocument(path, &groupResults); err != nil || groupResults.Schema != 1 || groupResults.Kind != "group-results" || groupResults.Assignment.Instance != s.Client.Origin() || groupResults.Assignment.CourseID != courseID || groupResults.Assignment.AssignmentID != assignmentID {
			return nil
		}
		for _, group := range groupResults.Groups {
			if group.Operation == "late-only" && group.LateOnly != nil && group.LateOnly.Outcome.Status == "succeeded" && containsString(group.MemberRefs, studentRef) {
				provenance, ok := group.LateOnly.MemberSchoolTime[studentRef]
				if ok && provenance.Status == group.LateOnly.LateTiming.Status && provenance.SecondsLate == group.LateOnly.LateTiming.SecondsLate && groupResultCurrentAttempt(s.Root, groupResults.Packet, group.GroupID, studentRef, attemptRef(sub)) {
					timing := group.LateOnly.LateTiming
					timing.SchoolTime = &provenance
					if timingSourcePresent(timing, sub) && timingStillMatches(timing, due, sub, calendar) {
						matched = true
						return filepath.SkipAll
					}
				}
			}
			if group.Operation == "grade" && containsString(group.MemberRefs, studentRef) && group.Grade != nil && group.Grade.Outcomes.Late.Status == "succeeded" && group.Grade.LateTiming != nil && groupResultCurrentAttempt(s.Root, groupResults.Packet, group.GroupID, studentRef, attemptRef(sub)) {
				if provenance, ok := group.Grade.MemberSchoolTime[studentRef]; ok && provenance.Status == group.Grade.LateTiming.Status && provenance.SecondsLate == group.Grade.LateTiming.SecondsLate {
					timing := *group.Grade.LateTiming
					timing.SchoolTime = &provenance
					if timingSourcePresent(timing, sub) && timingStillMatches(timing, due, sub, calendar) {
						matched = true
						return filepath.SkipAll
					}
				}
			}
			for _, row := range group.Students {
				if row.StudentRef != studentRef {
					continue
				}
				var timing *domain.LateTiming
				accepted := false
				if row.Operation == "grade" && row.Grade != nil && sameAttempt(row.Grade.Attempt, attemptRef(sub)) {
					timing, accepted = row.Grade.LateTiming, row.Grade.Outcomes.Late.Status == "succeeded"
				} else if row.Operation == "late-only" && row.LateOnly != nil && sameAttempt(row.LateOnly.Attempt, attemptRef(sub)) {
					timing, accepted = &row.LateOnly.LateTiming, row.LateOnly.Outcome.Status == "succeeded"
				}
				if accepted && timing != nil && timingSourcePresent(*timing, sub) && timingStillMatches(*timing, due, sub, calendar) {
					matched = true
					return filepath.SkipAll
				}
			}
		}
		return nil
	})
	return matched
}

func timingSourcePresent(timing domain.LateTiming, sub Submission) bool {
	if timing.SchoolTime == nil {
		return false
	}
	if sameAttempt(timing.SchoolTime.Attempt, attemptRef(sub)) {
		return true
	}
	for _, history := range sub.History {
		if sameAttempt(timing.SchoolTime.Attempt, attemptRef(history)) {
			return true
		}
	}
	return false
}

func timingStillMatches(timing domain.LateTiming, due string, sub Submission, calendar *workspace.SchoolCalendar) bool {
	if timing.DueAt != due || timing.SchoolTime == nil || sub.LatePolicyStatusKnown && sub.LatePolicyStatus != nil && *sub.LatePolicyStatus != "" && *sub.LatePolicyStatus != timing.Status || hasCustomGradeStatus(sub) {
		return false
	}
	deadline, err := time.Parse(time.RFC3339Nano, timing.SchoolTime.Deadline)
	if err != nil {
		return false
	}
	submitted, err := time.Parse(time.RFC3339Nano, timing.SchoolTime.SubmittedAt)
	if err != nil {
		return false
	}
	var active workspace.SchoolCalendar
	if calendar != nil {
		active = *calendar
	}
	counted, err := active.CountSchoolTime(deadline, submitted)
	if err != nil || counted.Status != timing.Status || counted.SecondsLate != timing.SecondsLate || !sameStrings(counted.SkippedDates, timing.SchoolTime.SkippedDates) {
		return false
	}
	if timing.Status == "none" {
		return !sub.Late && (sub.LatePolicyStatus == nil || *sub.LatePolicyStatus == "none")
	}
	return timing.Status == "late" && sub.Late && sub.SecondsLate == timing.SecondsLate && (sub.LatePolicyStatus == nil || *sub.LatePolicyStatus == "late")
}

func sameStrings(a, b []string) bool {
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

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

// sessionTimingCandidate includes ordinary live late signals and accepted CE
// `none` results whose calendar/due inputs may now produce a different answer.
// Teacher-owned explicit statuses without matching CE provenance stay out.
func (s *Service) sessionTimingCandidate(courseID, assignmentID int64, studentRef string, sub Submission) bool {
	if timingCandidate(sub) {
		return true
	}
	if studentRef == "" || sub.Late || !sub.LatePolicyStatusKnown || sub.LatePolicyStatus == nil || *sub.LatePolicyStatus != "none" || hasCustomGradeStatus(sub) {
		return false
	}
	return s.acceptedTimingRecordExists(courseID, assignmentID, studentRef, attemptRef(sub))
}

func validateDateTime(value, label string) error {
	if _, err := time.Parse(time.RFC3339Nano, value); err != nil {
		return fmt.Errorf("%s must be RFC3339: %w", label, err)
	}
	return nil
}
