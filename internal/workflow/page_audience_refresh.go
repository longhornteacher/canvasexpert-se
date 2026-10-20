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

// preparePageAudienceRefresh prepares a separate Page update action that
// recomputes the managed student row from current club membership. Pages have
// no submission evidence route, so removals are not work-protected; manual
// rows, availability dates and teacher edits are always preserved.
func (s *Service) preparePageAudienceRefresh(ctx context.Context, course domain.Course, path string) (*domain.AudienceState, error) {
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
			return nil, fmt.Errorf("latest refresh action points to a different Page")
		}
		if old.Instance != s.Client.Origin() || old.CourseID != course.ID || old.TargetID != target {
			return nil, fmt.Errorf("saved audience ownership differs from the exact Canvas/course/Page")
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
	state, err := s.resolveAudience(ctx, course, a)
	if err != nil {
		return nil, err
	}
	state.TargetID = target
	state.ObservedVisibility = snap.OnlyVisibleToOverrides
	state.Refresh = &domain.AudienceRefresh{}
	state.Owned = append([]domain.AudienceOverride(nil), old.Owned...)
	if snap.OnlyVisibleToOverrides != nil && *snap.OnlyVisibleToOverrides {
		state.Visibility = domain.ActionOutcome{Status: "preserved", Message: "existing restriction is preserved"}
	}
	// Resolve the managed row by its exact saved student set. Canvas documents
	// 204 without override IDs, so this preflight is the only identity evidence.
	appliedIDs, err := s.pageStudentIDs(store, old.AppliedRefs)
	if err != nil {
		return nil, err
	}
	managed := []domain.PageOverrideRow{}
	if len(appliedIDs) > 0 {
		want := idSet(appliedIDs)
		for _, row := range snap.Overrides {
			if len(row.StudentRefs) == 0 || row.GroupID > 0 || row.CourseSectionID > 0 {
				continue
			}
			ids, err := s.pageStudentIDs(store, row.StudentRefs)
			if err != nil {
				return nil, err
			}
			if sameIDSet(ids, want) {
				managed = append(managed, row)
			}
		}
		if len(managed) == 0 {
			state.Refresh.Unknown = append(state.Refresh.Unknown, domain.AudienceHold{Reason: "the managed student row changed or is missing; preserve the teacher's Canvas edit and prepare again after checking the Page"})
		} else if len(managed) > 1 {
			state.Refresh.Unknown = append(state.Refresh.Unknown, domain.AudienceHold{Reason: "more than one Page override matches the managed student set; ownership is ambiguous"})
			managed = nil
		}
	}
	managedIDs := map[int64]bool{}
	for _, row := range managed {
		managedIDs[row.ID] = true
	}
	state.OverrideBaseline = pageDatesFingerprint(snap)
	state.Exceptions = pageAudienceExceptions(snap, managedIDs, stringSet(state.StudentRefs))
	desired := stringSet(state.StudentRefs)
	// A student already covered by a preserved manual row needs no managed row.
	manual := map[string]bool{}
	for _, row := range snap.Overrides {
		if managedIDs[row.ID] {
			continue
		}
		for _, ref := range row.StudentRefs {
			manual[ref] = true
		}
	}
	present := map[string]bool{}
	for _, row := range managed {
		for _, ref := range row.StudentRefs {
			present[ref] = true
		}
	}
	for _, ref := range state.StudentRefs {
		switch {
		case present[ref]:
			state.Refresh.Retain = append(state.Refresh.Retain, ref)
		case manual[ref]:
			state.Refresh.Retain = append(state.Refresh.Retain, ref)
		default:
			state.Refresh.Add = append(state.Refresh.Add, ref)
		}
	}
	after := []string{}
	for _, ref := range state.StudentRefs {
		if present[ref] {
			after = append(after, ref)
		}
	}
	removes := []string{}
	for ref := range present {
		if !desired[ref] {
			removes = append(removes, ref)
			state.Refresh.Remove = append(state.Refresh.Remove, ref)
		}
	}
	sort.Strings(removes)
	// Ambiguous or missing ownership holds every change: the collection write
	// would otherwise replace a row the Engine cannot identify.
	ambiguousOwnership := len(state.Refresh.Unknown) > 0
	if len(managed) == 1 && len(removes) > 0 && !ambiguousOwnership {
		operation := "update"
		if len(after) == 0 {
			operation = "remove"
		}
		state.Refresh.Changes = append(state.Refresh.Changes, domain.AudienceChange{Operation: operation, OverrideID: managed[0].ID, Before: domain.AssignmentOverrideBaseline{Title: managed[0].Title, StudentRefs: append([]string(nil), managed[0].StudentRefs...)}, StudentRefs: after, RemoveRefs: removes})
	}
	if len(state.Refresh.Add) > 0 && !ambiguousOwnership {
		state.Refresh.Changes = append(state.Refresh.Changes, domain.AudienceChange{Operation: "create", Before: domain.AssignmentOverrideBaseline{Title: "Book club individual audience", DueAtOverridden: audienceBool(false), UnlockAtOverridden: audienceBool(false), LockAtOverridden: audienceBool(false)}, StudentRefs: append([]string(nil), state.Refresh.Add...)})
	}
	sort.Strings(state.Refresh.Add)
	sort.Strings(state.Refresh.Remove)
	sort.Strings(state.Refresh.Retain)
	state.AppliedRefs = append([]string(nil), old.AppliedRefs...)
	relative := filepath.ToSlash(filepath.Join("courses", course.Nickname, "drafts", fmt.Sprintf("page-audience-refresh-%d-%s.md", target, time.Now().UTC().Format("20060102T150405.000000000"))))
	output, err := workspace.ConstrainPath(s.Root, relative)
	if err != nil {
		return nil, err
	}
	state.PreparedPath = relative
	nextAudience := &domain.Audience{Mode: a.Mode, Policy: a.Policy, Source: a.Source, AuthorizedRemovalRefs: append([]string(nil), a.AuthorizedRemovalRefs...), State: state}
	// The refresh action carries the live Page as an unchanged content baseline,
	// so it is a valid update that changes only the audience.
	livePage, err := s.getPage(ctx, course.ID, target)
	if err != nil {
		return nil, err
	}
	pageURL := s.pageURL(course.ID, livePage)
	baseline := domain.PageSnapshot{Instance: s.Client.Origin(), CourseID: course.ID, PageID: livePage.PageID, Slug: livePage.Slug, URL: pageURL, Title: livePage.Title, BodyHTML: livePage.Body, Published: livePage.Published, EditingRoles: livePage.EditingRoles, UpdatedAt: livePage.UpdatedAt}
	doc := domain.PageDocument{Schema: 1, Kind: "page", Operation: "update", Course: course.Nickname, Instance: s.Client.Origin(), CourseID: course.ID, PageID: target, Slug: livePage.Slug, URL: pageURL, Title: livePage.Title, BodyFormat: "html", Published: livePage.Published, Observed: &baseline, Audience: nextAudience, ObservedDates: &snap}
	if err = workspace.WriteDocument(output, &doc, livePage.Body); err != nil {
		return nil, err
	}
	old.LatestRefreshPath = relative
	if err = savePrevious(); err != nil {
		return nil, fmt.Errorf("refresh draft saved at %s but the source link could not be saved: %w", relative, err)
	}
	return state, nil
}

func idSet(ids []int64) map[int64]bool {
	set := map[int64]bool{}
	for _, id := range ids {
		set[id] = true
	}
	return set
}

func sameIDSet(a []int64, b map[int64]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for _, id := range a {
		if !b[id] {
			return false
		}
	}
	return true
}
