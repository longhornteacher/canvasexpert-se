package workflow

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"canvasexpertse/internal/canvas"
	"canvasexpertse/internal/workspace"
)

const (
	notTaggedHold   = "not tagged (CE) in Canvas right now; nothing posted"
	cantHideHold    = "couldn't set Canvas to hide these grades; nothing posted"
	visibleMarker   = "already visible"
	groupVisibleMsg = "a group member's grade is already visible to the student; %s can change it in SpeedGrader"
)

// forScoring marks a Service copy as writing grades in a teammate's course, so
// every grade, rubric, comment and timing write is held unless Canvas is
// confirmed to hide the grades.
func (s *Service) forScoring(owner workspace.Teammate) *Service {
	cp := *s
	cp.scoring = &owner
	return &cp
}

// hideGrades sets the assignment to manual posting and succeeds only when
// Canvas confirms it. An uncertain answer is retried once; the mutation is
// idempotent.
func (s *Service) hideGrades(ctx context.Context, assignmentID int64) error {
	err := s.Client.SetAssignmentPostManually(ctx, assignmentID)
	var ce *canvas.Error
	if err != nil && errors.As(err, &ce) && ce.Uncertain {
		err = s.Client.SetAssignmentPostManually(ctx, assignmentID)
	}
	return err
}

// gradeVisibleHold is why a student's grade must not be touched in a
// teammate's course: Canvas shows it, or didn't say that it doesn't.
func gradeVisibleHold(owner workspace.Teammate, live Submission) string {
	if live.PostedAtPresent && live.PostedAt == nil {
		return ""
	}
	return fmt.Sprintf("grade %s to the student; %s can change it in SpeedGrader", visibleMarker, owner.Name)
}

// countVisibleHolds counts students held because their grade is visible.
func countVisibleHolds(students []StudentPostReport) int {
	n := 0
	for _, r := range students {
		if r.Status != "completed" && strings.Contains(r.Message, visibleMarker) {
			n++
		}
	}
	return n
}

// OwnerSummary is the one-line result for a teammate's assignment. It is empty
// for your own courses.
func (r PostReport) OwnerSummary() string {
	if r.Owner == "" {
		return ""
	}
	done := 0
	for _, st := range r.Students {
		if st.Status == "completed" {
			done++
		}
	}
	label := r.Course + " · " + r.Assignment
	if !r.Hidden {
		return fmt.Sprintf("%s: nothing was posted to students; see the held reasons above.", label)
	}
	line := fmt.Sprintf("%s: %d scored, hidden from students, ready for %s to review and post.", label, done, r.Owner)
	if r.HeldVisible > 0 {
		line += fmt.Sprintf(" %d held (already visible).", r.HeldVisible)
	}
	return line
}

// OwnerSummary is the one-line result for a teammate's group assignment. It is
// empty for your own courses.
func (r GroupPostReport) OwnerSummary() string {
	if r.Owner == "" {
		return ""
	}
	done := 0
	for _, g := range r.Groups {
		if g.Status == "completed" {
			done++
		}
	}
	label := r.Course + " · " + r.Assignment
	if !r.Hidden {
		return fmt.Sprintf("%s: nothing was posted to students; see the held reasons above.", label)
	}
	return fmt.Sprintf("%s: %d groups scored, hidden from students, ready for %s to review and post.", label, done, r.Owner)
}
