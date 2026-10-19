package workflow

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

// prepareQuizAudienceRefresh prepares an audience-only action for a restricted
// New Quiz. It targets the exact associated assignment and never calls quiz or
// item create/update or reapplies quiz settings.
func (s *Service) prepareQuizAudienceRefresh(ctx context.Context, course domain.Course, path string) (*domain.AudienceState, error) {
	visited := map[string]bool{}
	var a *domain.Audience
	var target int64
	var savePrevious func() error
	var expectedTarget int64
	var requestedSource domain.AudienceSource
	var requestedAuthorization []string
	for {
		if visited[path] {
			return nil, fmt.Errorf("audience refresh path repeats; use the latest saved action directly")
		}
		visited[path] = true
		var err error
		a, target, savePrevious, err = s.refreshDocument(course, path)
		if err != nil {
			return nil, err
		}
		if a == nil || a.State == nil {
			return nil, fmt.Errorf("refresh requires a previously prepared audience")
		}
		old := a.State
		if expectedTarget == 0 {
			expectedTarget = target
			requestedSource = a.Source
			requestedAuthorization = append([]string(nil), a.AuthorizedRemovalRefs...)
		} else if target != expectedTarget {
			return nil, fmt.Errorf("latest refresh action points to a different quiz assignment")
		}
		if old.Instance != s.Client.Origin() || old.CourseID != course.ID || old.TargetID != target {
			return nil, fmt.Errorf("saved audience ownership differs from the exact Canvas/course/assignment")
		}
		if old.LatestRefreshPath == "" {
			break
		}
		next, e := workspace.ConstrainPath(s.Root, old.LatestRefreshPath)
		if e != nil {
			return nil, e
		}
		rel, e := filepath.Rel(s.Root, next)
		if e != nil || !strings.HasPrefix(filepath.ToSlash(rel), "courses/"+course.Nickname+"/") {
			return nil, fmt.Errorf("refresh action must stay under the configured course")
		}
		path = next
	}
	old := a.State
	preparedAudience := *a
	preparedAudience.Source = requestedSource
	preparedAudience.AuthorizedRemovalRefs = requestedAuthorization
	a = &preparedAudience
	if old.Refresh == nil && ambiguous(old.Application) || ambiguous(old.Visibility) {
		return nil, fmt.Errorf("an audience write may have reached Canvas; its saved outcome must not be discarded")
	}
	if old.Refresh != nil {
		for _, change := range old.Refresh.Changes {
			if ambiguous(change.Outcome) {
				return nil, fmt.Errorf("an audience change may have reached Canvas; continue its saved action without preparing another")
			}
		}
	}
	for _, owned := range old.Owned {
		if ambiguous(owned.Outcome) {
			return nil, fmt.Errorf("an owned override write may have reached Canvas; its state was preserved")
		}
		if definite(owned.Outcome) && owned.ID <= 0 {
			return nil, fmt.Errorf("accepted audience has no exact override ID; refresh cannot guess its ownership")
		}
	}
	live, err := s.quizAudienceTarget(ctx, course.ID, target)
	if err != nil {
		return nil, err
	}
	if err = validateAudienceOverrides(live.Overrides); err != nil {
		return nil, err
	}
	if live.OnlyVisibleToOverrides == nil {
		return nil, fmt.Errorf("quiz assignment visibility could not be read completely")
	}
	state, err := s.resolveAudience(ctx, course, a)
	if err != nil {
		return nil, err
	}
	state.TargetID = target
	state.ObservedVisibility = live.OnlyVisibleToOverrides
	state.Refresh = &domain.AudienceRefresh{}
	state.Owned = append([]domain.AudienceOverride(nil), old.Owned...)
	if *live.OnlyVisibleToOverrides {
		state.Visibility = domain.ActionOutcome{Status: "preserved", Message: "existing restriction is preserved"}
	}
	owned := []domain.AudienceOverride{}
	for _, row := range state.Owned {
		if definite(row.Outcome) && row.ID > 0 {
			owned = append(owned, row)
		}
	}
	state.Owned = owned
	state.OverrideBaseline = audienceManualFingerprint(state, live.Overrides)
	manual := []AssignmentOverrideLive{}
	ownedIDs := map[int64]bool{}
	for _, row := range state.Owned {
		ownedIDs[row.ID] = true
	}
	for _, row := range live.Overrides {
		if !ownedIDs[row.ID] {
			manual = append(manual, row)
		}
	}
	_, state.Exceptions, err = s.assignmentAudienceRefs(state, Assignment{Overrides: manual})
	if err != nil {
		return nil, err
	}
	desired := stringSet(state.StudentRefs)
	present := map[string]bool{}
	store, err := workspace.OpenStudents(s.Root)
	if err != nil {
		return nil, err
	}
	for _, row := range live.Overrides {
		for _, id := range row.StudentIDs {
			for _, student := range store.Rows {
				if student.Instance == s.Client.Origin() && student.UserID == id {
					present[student.Ref] = true
				}
			}
		}
	}
	blockedRefs := map[string]bool{}
	authorized := stringSet(a.AuthorizedRemovalRefs)
	for _, owned := range state.Owned {
		var row AssignmentOverrideLive
		for _, candidate := range live.Overrides {
			if candidate.ID == owned.ID {
				row = candidate
				break
			}
		}
		if row.ID == 0 || !overrideMatchesBaseline(row, owned.Baseline, store, s.Client.Origin()) {
			state.Refresh.Unknown = append(state.Refresh.Unknown, domain.AudienceHold{Reason: fmt.Sprintf("Owned override %d changed or is missing; preserve the teacher's Canvas edit", owned.ID)})
			for _, ref := range owned.Baseline.StudentRefs {
				blockedRefs[ref] = true
			}
			continue
		}
		after := []string{}
		removes := []string{}
		for _, ref := range owned.Baseline.StudentRefs {
			if desired[ref] {
				after = append(after, ref)
				state.Refresh.Retain = append(state.Refresh.Retain, ref)
				continue
			}
			protected, reason, readErr := s.audienceRemovalProtection(ctx, course.ID, live, ref)
			if readErr != nil {
				state.Refresh.Unknown = append(state.Refresh.Unknown, domain.AudienceHold{StudentRef: ref, Reason: readErr.Error()})
				after = append(after, ref)
				continue
			}
			if protected {
				state.Refresh.Protected = append(state.Refresh.Protected, domain.AudienceHold{StudentRef: ref, Reason: reason, Authorized: authorized[ref]})
				if !authorized[ref] {
					after = append(after, ref)
					continue
				}
			}
			removes = append(removes, ref)
			state.Refresh.Remove = append(state.Refresh.Remove, ref)
		}
		if len(removes) > 0 {
			operation := "update"
			if len(after) == 0 {
				operation = "remove"
			}
			state.Refresh.Changes = append(state.Refresh.Changes, domain.AudienceChange{Operation: operation, OverrideID: owned.ID, Before: owned.Baseline, StudentRefs: after, RemoveRefs: removes})
		}
	}
	for _, ref := range state.StudentRefs {
		if !present[ref] && !blockedRefs[ref] {
			state.Refresh.Add = append(state.Refresh.Add, ref)
		} else if !containsString(state.Refresh.Retain, ref) {
			state.Refresh.Retain = append(state.Refresh.Retain, ref)
		}
	}
	if len(state.Refresh.Add) > 0 {
		state.Refresh.Changes = append(state.Refresh.Changes, domain.AudienceChange{Operation: "create", Before: domain.AssignmentOverrideBaseline{Title: "Book club individual audience", DueAtOverridden: audienceBool(false), UnlockAtOverridden: audienceBool(false), LockAtOverridden: audienceBool(false)}, StudentRefs: append([]string(nil), state.Refresh.Add...)})
	}
	sort.Strings(state.Refresh.Add)
	sort.Strings(state.Refresh.Remove)
	sort.Strings(state.Refresh.Retain)
	state.AppliedRefs = managedAudienceRefs(state, live, store, s.Client.Origin())
	relative := filepath.ToSlash(filepath.Join("courses", course.Nickname, "drafts", fmt.Sprintf("quiz-audience-refresh-%d-%s.md", target, time.Now().UTC().Format("20060102T150405.000000000"))))
	output, err := workspace.ConstrainPath(s.Root, relative)
	if err != nil {
		return nil, err
	}
	state.PreparedPath = relative
	nextAudience := &domain.Audience{Mode: a.Mode, Policy: a.Policy, Source: a.Source, AuthorizedRemovalRefs: append([]string(nil), a.AuthorizedRemovalRefs...), State: state}
	doc := domain.AssignmentDatesDocument{Schema: 1, Kind: "assignment-dates", Operation: "update", Course: course.Nickname, Instance: s.Client.Origin(), CourseID: course.ID, AssignmentID: target, Audience: nextAudience, Rows: []domain.AssignmentOverrideRow{}}
	if err = workspace.WriteDocument(output, &doc, ""); err != nil {
		return nil, err
	}
	old.LatestRefreshPath = relative
	if err = savePrevious(); err != nil {
		return nil, fmt.Errorf("refresh draft saved at %s but the source link could not be saved: %w", relative, err)
	}
	return state, nil
}
