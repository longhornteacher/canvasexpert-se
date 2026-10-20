package workflow

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

// preparePageAudience records the selected-club snapshot and the exact
// date-details baseline for a course Page. It writes only the course-local
// draft and never changes Canvas.
func (s *Service) preparePageAudience(ctx context.Context, course domain.Course, d *domain.PageDocument, save func() error) (*domain.AudienceState, error) {
	if d.Audience == nil {
		return nil, fmt.Errorf("draft requires an audience intent")
	}
	if d.GroupID > 0 {
		return nil, fmt.Errorf("individual audience applies to course Pages, not group Pages")
	}
	if d.Audience.State != nil && (len(d.Audience.State.Owned) > 0 || definite(d.Audience.State.Application) || ambiguous(d.Audience.State.Application) || definite(d.Audience.State.Visibility) || ambiguous(d.Audience.State.Visibility)) {
		return nil, fmt.Errorf("audience action already began; preserve its state and use explicit refresh")
	}
	state, err := s.resolveAudience(ctx, course, d.Audience)
	if err != nil {
		return nil, err
	}
	target := d.PageID
	if target == 0 && d.Created != nil {
		target = d.Created.PageID
	}
	state.TargetID = target
	if target > 0 {
		live, err := s.readPageDates(ctx, course.ID, target)
		if err != nil {
			return nil, err
		}
		store, err := workspace.OpenStudents(s.Root)
		if err != nil {
			return nil, err
		}
		snap, err := s.pageDatesSnapshot(live, store)
		if err != nil {
			return nil, err
		}
		state.ObservedVisibility = snap.OnlyVisibleToOverrides
		state.OverrideBaseline = pageDatesFingerprint(snap)
		state.Exceptions = pageAudienceExceptions(snap, pageAudienceOwnedIDs(d.Audience), stringSet(state.StudentRefs))
		d.ObservedDates = &snap
	}
	d.Audience.State = state
	if err := save(); err != nil {
		return nil, err
	}
	return state, nil
}

// applyPageAudience restricts a course Page to the prepared individual
// audience. The date-details collection is replaced as a whole, so every
// unrelated row and availability date is preserved from the observed baseline.
func (s *Service) applyPageAudience(ctx context.Context, course domain.Course, d *domain.PageDocument, pageID int64, save func() error) (bool, error) {
	a := d.Audience
	if a == nil {
		return true, nil
	}
	st := a.State
	if st == nil {
		d.AudienceOutcome = domain.ActionOutcome{Status: "held", Message: "prepare this audience before review or push"}
		return false, save()
	}
	if definite(d.AudienceOutcome) {
		return true, nil
	}
	if ambiguous(d.AudienceOutcome) {
		return false, nil
	}
	if st.Refresh != nil {
		return s.applyPageAudienceRefresh(ctx, course, d, pageID, save)
	}
	if pageID <= 0 {
		d.AudienceOutcome = domain.ActionOutcome{Status: "held", Message: "Page identity is required before applying its audience"}
		return false, save()
	}
	if st.TargetID == 0 {
		st.TargetID = pageID
		if err := save(); err != nil {
			return false, err
		}
	}
	if st.TargetID != pageID || st.Instance != s.Client.Origin() || st.CourseID != course.ID {
		return false, fmt.Errorf("audience preparation belongs to a different Canvas/course/Page")
	}
	if err := s.checkAudience(ctx, course, a, pageID); err != nil {
		d.AudienceOutcome = domain.ActionOutcome{Status: "held", Message: err.Error()}
		return false, save()
	}
	live, err := s.readPageDates(ctx, course.ID, pageID)
	if err != nil {
		d.AudienceOutcome = domain.ActionOutcome{Status: "held", Message: err.Error()}
		return false, save()
	}
	store, err := workspace.OpenStudents(s.Root)
	if err != nil {
		return false, err
	}
	snap, err := s.pageDatesSnapshot(live, store)
	if err != nil {
		d.AudienceOutcome = domain.ActionOutcome{Status: "held", Message: err.Error()}
		return false, save()
	}
	if d.ObservedDates == nil {
		d.ObservedDates = &snap
	}
	if pageDatesFingerprint(snap) != pageDatesFingerprint(*d.ObservedDates) {
		d.AudienceOutcome = domain.ActionOutcome{Status: "held", Message: "Page audience or availability dates changed after preparation; prepare again"}
		return false, save()
	}
	ownedIDs := pageAudienceOwnedIDs(a)
	refs, exceptions := pageAudienceRefs(st, snap, ownedIDs)
	st.Exceptions = append(append([]string(nil), exceptions...), pageAudienceExceptions(snap, ownedIDs, stringSet(st.StudentRefs))...)
	rows := pageAudienceRows(snap, ownedIDs)
	payload := []map[string]any{}
	for _, row := range rows {
		ids, err := s.pageStudentIDs(store, row.StudentRefs)
		if err != nil {
			d.AudienceOutcome = domain.ActionOutcome{Status: "held", Message: err.Error()}
			return false, save()
		}
		payload = append(payload, pageOverridePayload(row, ids))
	}
	if len(refs) > 0 {
		ids, err := s.pageStudentIDs(store, refs)
		if err != nil {
			d.AudienceOutcome = domain.ActionOutcome{Status: "held", Message: err.Error()}
			return false, save()
		}
		payload = append(payload, pageOverridePayload(domain.PageOverrideRow{Title: "Book club individual audience", StudentRefs: refs}, ids))
	}
	d.AudienceOutcome = domain.ActionOutcome{Status: "pending"}
	if err := save(); err != nil {
		return false, err
	}
	if err := s.writePageDates(ctx, course.ID, pageID, true, snap.UnlockAt, snap.LockAt, payload); err != nil {
		d.AudienceOutcome = failedOutcome(err)
		return false, save()
	}
	// Canvas documents 204 with no body. The accepted collection is recorded
	// without a confirmation read; a later identity-dependent action resolves
	// its own exact rows.
	st.Owned = nil
	if len(refs) > 0 {
		st.Owned = []domain.AudienceOverride{{Baseline: domain.AssignmentOverrideBaseline{Title: "Book club individual audience", StudentRefs: append([]string(nil), refs...)}}}
	}
	st.AppliedRefs = append([]string(nil), refs...)
	sort.Strings(st.AppliedRefs)
	st.Application = domain.ActionOutcome{Status: "succeeded"}
	st.Visibility = domain.ActionOutcome{Status: "succeeded"}
	d.AudienceOutcome = domain.ActionOutcome{Status: "succeeded"}
	return true, save()
}

// applyPageAudienceRefresh applies a prepared Page membership refresh. The
// date-details collection is indivisible, so preserved rows and the managed row
// are submitted together; a changed managed row holds instead of being replaced.
func (s *Service) applyPageAudienceRefresh(ctx context.Context, course domain.Course, d *domain.PageDocument, pageID int64, save func() error) (bool, error) {
	a := d.Audience
	st := a.State
	plan := st.Refresh
	if definite(st.Application) && definite(st.Visibility) {
		return true, nil
	}
	if ambiguous(st.Application) {
		return false, nil
	}
	if err := s.checkAudience(ctx, course, a, pageID); err != nil {
		st.Application = domain.ActionOutcome{Status: "held", Message: err.Error()}
		return false, save()
	}
	live, err := s.readPageDates(ctx, course.ID, pageID)
	if err != nil {
		st.Application = domain.ActionOutcome{Status: "held", Message: err.Error()}
		return false, save()
	}
	store, err := workspace.OpenStudents(s.Root)
	if err != nil {
		return false, err
	}
	snap, err := s.pageDatesSnapshot(live, store)
	if err != nil {
		st.Application = domain.ActionOutcome{Status: "held", Message: err.Error()}
		return false, save()
	}
	if d.ObservedDates == nil {
		d.ObservedDates = &snap
	}
	if pageDatesFingerprint(snap) != pageDatesFingerprint(*d.ObservedDates) {
		st.Application = domain.ActionOutcome{Status: "held", Message: "Page audience or availability dates changed after preparation; prepare refresh again"}
		return false, save()
	}
	managedIDs := map[int64]bool{}
	for _, change := range plan.Changes {
		if change.Operation != "create" {
			managedIDs[change.OverrideID] = true
		}
	}
	rows := pageAudienceRows(snap, managedIDs)
	payload := []map[string]any{}
	for _, row := range rows {
		ids, err := s.pageStudentIDs(store, row.StudentRefs)
		if err != nil {
			st.Application = domain.ActionOutcome{Status: "held", Message: err.Error()}
			return false, save()
		}
		payload = append(payload, pageOverridePayload(row, ids))
	}
	applied := []string{}
	for _, change := range plan.Changes {
		if definite(change.Outcome) {
			applied = append(applied, change.StudentRefs...)
			continue
		}
		if ambiguous(change.Outcome) {
			return false, nil
		}
		if change.Operation != "create" {
			found := false
			for _, row := range snap.Overrides {
				if row.ID == change.OverrideID {
					found = true
					if !sameRefSet(row.StudentRefs, change.Before.StudentRefs) {
						change.Outcome = domain.ActionOutcome{Status: "held", Message: "managed Page override changed after preparation; the teacher's edit is preserved"}
						if err := save(); err != nil {
							return false, err
						}
						found = false
					}
				}
			}
			if !found {
				if change.Outcome.Status == "" {
					change.Outcome = domain.ActionOutcome{Status: "held", Message: "managed Page override is missing; prepare refresh again"}
					if err := save(); err != nil {
						return false, err
					}
				}
				continue
			}
		}
		if len(change.StudentRefs) > 0 {
			ids, err := s.pageStudentIDs(store, change.StudentRefs)
			if err != nil {
				change.Outcome = domain.ActionOutcome{Status: "held", Message: err.Error()}
				if err := save(); err != nil {
					return false, err
				}
				continue
			}
			payload = append(payload, pageOverridePayload(domain.PageOverrideRow{Title: change.Before.Title, StudentRefs: change.StudentRefs}, ids))
		}
		change.AppliedOperation = change.Operation
		change.AppliedStudentRefs = append([]string(nil), change.StudentRefs...)
		change.AppliedRemoveRefs = append([]string(nil), change.RemoveRefs...)
		applied = append(applied, change.StudentRefs...)
	}
	st.Application = domain.ActionOutcome{Status: "pending"}
	if err := save(); err != nil {
		return false, err
	}
	if err := s.writePageDates(ctx, course.ID, pageID, true, snap.UnlockAt, snap.LockAt, payload); err != nil {
		st.Application = failedOutcome(err)
		return false, save()
	}
	for i := range plan.Changes {
		if plan.Changes[i].Outcome.Status == "" {
			plan.Changes[i].Outcome = domain.ActionOutcome{Status: "succeeded"}
		}
	}
	sort.Strings(applied)
	st.AppliedRefs = applied
	st.Owned = nil
	if len(applied) > 0 {
		st.Owned = []domain.AudienceOverride{{Baseline: domain.AssignmentOverrideBaseline{Title: "Book club individual audience", StudentRefs: append([]string(nil), applied...)}}}
	}
	st.Visibility = domain.ActionOutcome{Status: "succeeded"}
	held := false
	for _, change := range plan.Changes {
		if !definite(change.Outcome) {
			held = true
		}
	}
	if held || len(plan.Unknown) > 0 {
		st.Application = domain.ActionOutcome{Status: "held", Message: "safe changes are retained; protected, unknown or unfinished audience changes remain"}
	} else {
		st.Application = domain.ActionOutcome{Status: "succeeded"}
	}
	return definite(st.Application) && definite(st.Visibility), save()
}

func sameRefSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	set := stringSet(a)
	for _, ref := range b {
		if !set[ref] {
			return false
		}
	}
	return true
}

func (s *Service) applyPageDates(ctx context.Context, course domain.Course, d *domain.PageDocument, pageID int64, save func() error) (bool, error) {
	if !pageDatesChanged(d) {
		if d.DatesOutcome.Status == "" {
			d.DatesOutcome = domain.ActionOutcome{Status: "not-requested"}
		}
		return true, nil
	}
	if definite(d.DatesOutcome) {
		return true, nil
	}
	if ambiguous(d.DatesOutcome) {
		return false, nil
	}
	if d.ObservedDates == nil {
		d.DatesOutcome = domain.ActionOutcome{Status: "held", Message: "availability-date edits require the pulled date-details baseline"}
		return false, save()
	}
	live, err := s.readPageDates(ctx, course.ID, pageID)
	if err != nil {
		d.DatesOutcome = domain.ActionOutcome{Status: "held", Message: err.Error()}
		return false, save()
	}
	store, err := workspace.OpenStudents(s.Root)
	if err != nil {
		return false, err
	}
	snap, err := s.pageDatesSnapshot(live, store)
	if err != nil {
		d.DatesOutcome = domain.ActionOutcome{Status: "held", Message: err.Error()}
		return false, save()
	}
	if pageDatesFingerprint(snap) != pageDatesFingerprint(*d.ObservedDates) {
		d.DatesOutcome = domain.ActionOutcome{Status: "held", Message: "Page audience or availability dates changed after preparation; pull a fresh baseline"}
		return false, save()
	}
	payload := []map[string]any{}
	for _, row := range snap.Overrides {
		ids, err := s.pageStudentIDs(store, row.StudentRefs)
		if err != nil {
			d.DatesOutcome = domain.ActionOutcome{Status: "held", Message: err.Error()}
			return false, save()
		}
		payload = append(payload, pageOverridePayload(row, ids))
	}
	onlyVisible := snap.OnlyVisibleToOverrides != nil && *snap.OnlyVisibleToOverrides
	unlock := pageDateValue(d.UnlockAt, snap.UnlockAt)
	lock := pageDateValue(d.LockAt, snap.LockAt)
	d.DatesOutcome = domain.ActionOutcome{Status: "pending"}
	if err := save(); err != nil {
		return false, err
	}
	if err := s.writePageDates(ctx, course.ID, pageID, onlyVisible, unlock, lock, payload); err != nil {
		d.DatesOutcome = failedOutcome(err)
		return false, save()
	}
	d.DatesOutcome = domain.ActionOutcome{Status: "succeeded"}
	return true, save()
}

// pageAudienceSummary describes the applied audience for a push report.
func pageAudienceSummary(d *domain.PageDocument) string {
	if d.Audience == nil || d.Audience.State == nil {
		return ""
	}
	st := d.Audience.State
	parts := []string{fmt.Sprintf("%d students: %s", len(st.AppliedRefs), strings.Join(st.AppliedRefs, ", "))}
	if len(st.Exceptions) > 0 {
		parts = append(parts, strings.Join(st.Exceptions, "; "))
	}
	return strings.Join(parts, "; ")
}
