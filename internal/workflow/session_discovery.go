package workflow

import (
	"fmt"
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
