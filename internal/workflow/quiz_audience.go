package workflow

import (
	"context"
	"fmt"
	"strings"

	"canvasexpertse/internal/domain"
)

// quizAudienceTarget validates the exact associated assignment for a restricted
// New Quiz. The quiz resource routes use the assignment ID, so the saved
// identity must be an individual assignment in this course.
func (s *Service) quizAudienceTarget(ctx context.Context, courseID, assignmentID int64) (Assignment, error) {
	if assignmentID <= 0 {
		return Assignment{}, fmt.Errorf("restricted quiz requires its exact associated assignment ID")
	}
	live, err := GetAssignment(ctx, s.Client, courseID, assignmentID)
	if err != nil {
		return live, err
	}
	if live.ID != assignmentID || live.CourseID != courseID {
		return live, fmt.Errorf("quiz assignment is outside the configured course")
	}
	if live.GroupCategoryID != nil && *live.GroupCategoryID > 0 {
		return live, fmt.Errorf("quiz assignment is native group work; an individual club audience cannot target it")
	}
	return live, nil
}

// prepareQuizAudience records the selected-club snapshot and the exact
// assignment override baseline in the sidecar. It writes no Canvas state.
func (s *Service) prepareQuizAudience(ctx context.Context, course domain.Course, path string, a *domain.Audience, assignmentID int64) (*domain.AudienceState, error) {
	if a == nil {
		return nil, fmt.Errorf("quiz audience intent is required")
	}
	if a.State != nil && (len(a.State.Owned) > 0 || definite(a.State.Application) || ambiguous(a.State.Application) || definite(a.State.Visibility) || ambiguous(a.State.Visibility)) {
		return nil, fmt.Errorf("audience action already began; preserve its state and use explicit refresh")
	}
	state, err := s.resolveAudience(ctx, course, a)
	if err != nil {
		return nil, err
	}
	state.TargetID = assignmentID
	if assignmentID > 0 {
		live, err := s.quizAudienceTarget(ctx, course.ID, assignmentID)
		if err != nil {
			return nil, err
		}
		if err = validateAudienceOverrides(live.Overrides); err != nil {
			return nil, err
		}
		if live.OnlyVisibleToOverrides == nil {
			return nil, fmt.Errorf("quiz assignment visibility could not be read completely; audience was not prepared")
		}
		state.ObservedVisibility = live.OnlyVisibleToOverrides
		state.OverrideBaseline = overridesFingerprint(live.Overrides)
		_, state.Exceptions, err = s.assignmentAudienceRefs(state, live)
		if err != nil {
			return nil, err
		}
	}
	a.State = state
	rec := readSidecar(path)
	rec.Audience = a
	if err = writeSidecar(path, rec); err != nil {
		return nil, err
	}
	return state, nil
}

// applyQuizAudience restricts the quiz's associated assignment before
// publication. It reuses the assignment audience path, so accepted and
// uncertain writes are never replayed.
func (s *Service) applyQuizAudience(ctx context.Context, course domain.Course, path string, a *domain.Audience, assignmentID int64, stages *quizStages) (bool, error) {
	if a == nil {
		return true, nil
	}
	if stages != nil && definite(stages.Audience) {
		return true, nil
	}
	if stages != nil && ambiguous(stages.Audience) {
		return false, nil
	}
	live, err := s.quizAudienceTarget(ctx, course.ID, assignmentID)
	if err != nil {
		return false, err
	}
	save := func() error {
		rec := readSidecar(path)
		rec.Audience = a
		if stages != nil {
			rec.Stages = stages
		}
		return writeSidecar(path, rec)
	}
	complete, err := s.applyAssignmentAudience(ctx, course, live, a, save)
	if err != nil {
		return false, err
	}
	if stages != nil {
		if complete {
			stages.Audience = domain.ActionOutcome{Status: "succeeded"}
		} else if a.State != nil {
			stages.Audience = a.State.Application
		}
		if err := save(); err != nil {
			return false, err
		}
	}
	return complete, nil
}

// quizAudienceSummary describes the applied quiz audience for a push report.
func quizAudienceSummary(a *domain.Audience) string {
	if a == nil || a.State == nil {
		return ""
	}
	st := a.State
	parts := []string{fmt.Sprintf("%d students: %s", len(st.AppliedRefs), strings.Join(st.AppliedRefs, ", "))}
	if len(st.Exceptions) > 0 {
		parts = append(parts, strings.Join(st.Exceptions, "; "))
	}
	return strings.Join(parts, "; ")
}
