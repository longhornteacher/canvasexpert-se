package workflow

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

func (s *Service) refreshDocument(course domain.Course, path string) (*domain.Audience, int64, func() error, error) {
	kind, err := ReviewInputKind(path)
	if err != nil {
		return nil, 0, nil, err
	}
	switch kind {
	case "assignment":
		var d domain.AssignmentDocument
		body, e := workspace.ReadDocument(path, &d)
		if e != nil {
			return nil, 0, nil, e
		}
		if e = domain.ValidateAssignment(d, body); e != nil {
			return nil, 0, nil, e
		}
		if d.Course != course.Nickname || d.Instance != s.Client.Origin() || d.CourseID != course.ID || d.Created == nil || d.Created.AssignmentID <= 0 {
			return nil, 0, nil, fmt.Errorf("refresh requires a saved individual assignment identity in this Canvas/course")
		}
		return d.Audience, d.Created.AssignmentID, func() error { return workspace.WriteDocument(path, &d, body) }, nil
	case "assignment-dates":
		var d domain.AssignmentDatesDocument
		body, e := workspace.ReadDocument(path, &d)
		if e != nil {
			return nil, 0, nil, e
		}
		if e = domain.ValidateAssignmentDates(d); e != nil {
			return nil, 0, nil, e
		}
		if d.Course != course.Nickname || d.Instance != s.Client.Origin() || d.CourseID != course.ID {
			return nil, 0, nil, fmt.Errorf("refresh draft differs from configured Canvas/course")
		}
		return d.Audience, d.AssignmentID, func() error { return workspace.WriteDocument(path, &d, body) }, nil
	}
	return nil, 0, nil, fmt.Errorf("refresh requires an assignment receipt or assignment-dates audience")
}

func (s *Service) prepareAssignmentAudienceRefresh(ctx context.Context, course domain.Course, path string) (*domain.AudienceState, error) {
	visited := map[string]bool{}
	var a *domain.Audience
	var target int64
	var savePrevious func() error
	var expectedTarget int64
	var requestedSource domain.AudienceSource
	var requestedAuthorization []string
	// A receipt links to its latest Engine-authored action, keeping accepted
	// ownership in the documents already used for preparation and push.
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
			return nil, fmt.Errorf("latest refresh action points to a different assignment")
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
	live, err := GetAssignment(ctx, s.Client, course.ID, target)
	if err != nil {
		return nil, err
	}
	if live.ID != target || live.CourseID != course.ID || live.GroupCategoryID != nil && *live.GroupCategoryID > 0 {
		return nil, fmt.Errorf("refresh target must be the exact individual assignment in this course")
	}
	if err = validateAudienceOverrides(live.Overrides); err != nil {
		return nil, err
	}
	if live.OnlyVisibleToOverrides == nil {
		return nil, fmt.Errorf("assignment visibility could not be read completely")
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
	// Only rows with accepted exact IDs are owned. A prior refused create has no
	// row to edit and can be replaced by a freshly prepared addition.
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
	relative := filepath.ToSlash(filepath.Join("courses", course.Nickname, "drafts", fmt.Sprintf("audience-refresh-%d-%s.md", target, time.Now().UTC().Format("20060102T150405.000000000"))))
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
		return nil, fmt.Errorf("refresh draft saved at %s but the source receipt link could not be saved: %w", relative, err)
	}
	return state, nil
}

func (s *Service) audienceRemovalProtection(ctx context.Context, courseID int64, assignment Assignment, ref string) (bool, string, error) {
	store, err := workspace.OpenStudents(s.Root)
	if err != nil {
		return false, "", fmt.Errorf("private student mapping could not be read; removal held")
	}
	student, err := store.Resolve(s.Client.Origin(), ref)
	if err != nil {
		return false, "", fmt.Errorf("student reference could not be resolved; removal held")
	}
	sub, err := GetSubmission(ctx, s.Client, courseID, assignment.ID, student.UserID)
	if err != nil {
		return false, "", fmt.Errorf("submission and grade evidence could not be read; removal held")
	}
	if sub.UserID != student.UserID || sub.AssignmentID != assignment.ID {
		return false, "", fmt.Errorf("submission evidence has a different student or assignment; removal held")
	}
	if submissionProtectsAudience(sub, assignment.NewQuiz) {
		return true, "Submission, attempt, grade or saved work keeps this student's access", nil
	}
	for _, prior := range sub.History {
		if prior.UserID != 0 && prior.UserID != student.UserID || prior.AssignmentID != 0 && prior.AssignmentID != assignment.ID {
			return false, "", fmt.Errorf("submission history identity could not be established; removal held")
		}
		if submissionProtectsAudience(prior, assignment.NewQuiz) {
			return true, "Earlier submission, attempt, grade or saved work keeps this student's access", nil
		}
	}
	if !sub.HistoryPresent {
		return false, "", fmt.Errorf("submission history was not included; removal held")
	}
	if sub.State != "unsubmitted" {
		return false, "", fmt.Errorf("submission state is not a known empty placeholder; removal held")
	}
	return false, "", nil
}
func submissionProtectsAudience(sub Submission, quiz bool) bool {
	return sub.SubmittedAt != nil && *sub.SubmittedAt != "" || sub.GradedAt != nil && *sub.GradedAt != "" || sub.Score != nil || sub.EnteredScore != nil || sub.Grade != nil && strings.TrimSpace(*sub.Grade) != "" || len(sub.RubricAssessment) > 0 || strings.TrimSpace(sub.Body) != "" || len(sub.Attachments) > 0 || sub.URL != "" || sub.MediaComment != nil || len(sub.DiscussionEntries) > 0 || sub.State == "submitted" || sub.State == "graded" || sub.State == "pending_review" || quiz && sub.Attempt > 0
}

func (s *Service) applyAssignmentAudienceRefresh(ctx context.Context, course domain.Course, assignment Assignment, a *domain.Audience, save func() error) (bool, error) {
	st := a.State
	plan := st.Refresh
	if definite(st.Application) && definite(st.Visibility) {
		return true, nil
	}
	if err := s.checkAudience(ctx, course, a, assignment.ID); err != nil {
		st.Application = domain.ActionOutcome{Status: "held", Message: err.Error()}
		return false, save()
	}
	if st.TargetID != assignment.ID || st.Instance != s.Client.Origin() || st.CourseID != course.ID || assignment.CourseID != course.ID || assignment.GroupCategoryID != nil && *assignment.GroupCategoryID > 0 {
		return false, fmt.Errorf("refresh targets a different Canvas/course/assignment")
	}
	if err := validateAudienceOverrides(assignment.Overrides); err != nil {
		return false, err
	}
	if !refreshVisibilityMatches(st, assignment.OnlyVisibleToOverrides) || audienceManualFingerprint(st, assignment.Overrides) != st.OverrideBaseline {
		st.Application = domain.ActionOutcome{Status: "held", Message: "assignment visibility or manual exceptions changed after preparation; prepare refresh again"}
		return false, save()
	}
	if len(plan.Changes) == 0 {
		if err := s.acceptedAudienceBaseline(a, assignment); err != nil {
			st.Application = domain.ActionOutcome{Status: "held", Message: err.Error()}
			return false, save()
		}
	}
	for i := range plan.Changes {
		change := &plan.Changes[i]
		if definite(change.Outcome) {
			continue
		}
		if ambiguous(change.Outcome) {
			continue
		}
		if err := s.checkAudience(ctx, course, a, assignment.ID); err != nil {
			change.Outcome = domain.ActionOutcome{Status: "held", Message: err.Error()}
			if err = save(); err != nil {
				return false, err
			}
			continue
		}
		live, err := GetAssignment(ctx, s.Client, course.ID, assignment.ID)
		if err != nil {
			return false, err
		}
		if live.ID != assignment.ID || live.CourseID != course.ID || live.GroupCategoryID != nil && *live.GroupCategoryID > 0 {
			return false, fmt.Errorf("refresh assignment identity changed")
		}
		if err = validateAudienceOverrides(live.Overrides); err != nil {
			return false, err
		}
		if !refreshVisibilityMatches(st, live.OnlyVisibleToOverrides) || audienceManualFingerprint(st, live.Overrides) != st.OverrideBaseline {
			change.Outcome = domain.ActionOutcome{Status: "held", Message: "assignment visibility or manual exceptions changed after preparation; prepare refresh again"}
			if err = save(); err != nil {
				return false, err
			}
			continue
		}
		var row AssignmentOverrideLive
		if change.Operation != "create" {
			owned := false
			for _, candidate := range st.Owned {
				if candidate.ID == change.OverrideID && definite(candidate.Outcome) && reflect.DeepEqual(candidate.Baseline, change.Before) {
					owned = true
				}
			}
			if !owned {
				change.Outcome = domain.ActionOutcome{Status: "held", Message: "prepared change does not match its accepted owned baseline"}
				if err = save(); err != nil {
					return false, err
				}
				continue
			}
		}
		if change.Operation != "create" {
			for _, candidate := range live.Overrides {
				if candidate.ID == change.OverrideID {
					row = candidate
					break
				}
			}
			store, e := workspace.OpenStudents(s.Root)
			if e != nil {
				return false, e
			}
			if row.ID == 0 || !overrideMatchesBaseline(row, change.Before, store, s.Client.Origin()) {
				change.Outcome = domain.ActionOutcome{Status: "held", Message: "owned override changed after preparation; the teacher's edit is preserved"}
				if err = save(); err != nil {
					return false, err
				}
				continue
			}
		}
		actual := *change
		actual.StudentRefs = append([]string(nil), change.StudentRefs...)
		actual.RemoveRefs = nil
		retained := stringSet(actual.StudentRefs)
		for _, ref := range change.RemoveRefs {
			protected, reason, e := s.audienceRemovalProtection(ctx, course.ID, live, ref)
			if e != nil {
				plan.Unknown = append(plan.Unknown, domain.AudienceHold{StudentRef: ref, Reason: e.Error()})
				retained[ref] = true
				continue
			}
			if protected && !stringSet(a.AuthorizedRemovalRefs)[ref] {
				plan.Protected = append(plan.Protected, domain.AudienceHold{StudentRef: ref, Reason: reason})
				retained[ref] = true
				continue
			}
			actual.RemoveRefs = append(actual.RemoveRefs, ref)
		}
		actual.StudentRefs = nil
		for ref := range retained {
			actual.StudentRefs = append(actual.StudentRefs, ref)
		}
		sort.Strings(actual.StudentRefs)
		if change.Operation != "create" {
			if len(actual.RemoveRefs) == 0 {
				change.Outcome = domain.ActionOutcome{Status: "preserved", ID: change.OverrideID, Message: "all prepared removals now require preserved access"}
				change.AppliedOperation = "preserve"
				change.AppliedStudentRefs = append([]string(nil), change.Before.StudentRefs...)
				if err = save(); err != nil {
					return false, err
				}
				continue
			}
			actual.Operation = "update"
			if len(actual.StudentRefs) == 0 {
				actual.Operation = "remove"
			}
		}
		change.AppliedOperation = actual.Operation
		change.AppliedStudentRefs = append([]string(nil), actual.StudentRefs...)
		change.AppliedRemoveRefs = append([]string(nil), actual.RemoveRefs...)
		fields := map[string]any{}
		method := http.MethodPut
		url := fmt.Sprintf("/api/v1/courses/%d/assignments/%d/overrides", course.ID, assignment.ID)
		if actual.Operation == "create" {
			method = http.MethodPost
			fields["title"] = change.Before.Title
		} else {
			url += fmt.Sprintf("/%d", change.OverrideID)
			if actual.Operation == "remove" {
				method = http.MethodDelete
			} else {
				preserveOverrideDates(fields, row)
			}
		}
		if actual.Operation == "create" {
			store, e := workspace.OpenStudents(s.Root)
			if e != nil {
				return false, e
			}
			ids, e := resolveOverrideTarget(ctx, s, course.ID, live, store, domain.AssignmentOverrideRow{StudentRefs: actual.StudentRefs})
			if e != nil {
				return false, e
			}
			present := map[int64]bool{}
			for _, current := range live.Overrides {
				for _, id := range current.StudentIDs {
					present[id] = true
				}
			}
			conflict := false
			for _, id := range ids {
				if present[id] {
					conflict = true
				}
			}
			if conflict {
				change.Outcome = domain.ActionOutcome{Status: "held", Message: "a prepared addition now has an existing student override; preserve it and prepare refresh again"}
				if e = save(); e != nil {
					return false, e
				}
				continue
			}
		}
		if actual.Operation != "remove" {
			store, e := workspace.OpenStudents(s.Root)
			if e != nil {
				return false, e
			}
			ids, e := resolveOverrideTarget(ctx, s, course.ID, live, store, domain.AssignmentOverrideRow{StudentRefs: actual.StudentRefs})
			if e != nil {
				return false, e
			}
			fields["student_ids"] = ids
		}
		change.Outcome = domain.ActionOutcome{Status: "pending"}
		if err = save(); err != nil {
			return false, err
		}
		var response struct {
			ID       int64                  `json:"id"`
			Override AssignmentOverrideLive `json:"assignment_override"`
		}
		body := any(nil)
		if actual.Operation != "remove" {
			body = map[string]any{"assignment_override": fields}
		}
		err = s.Client.WriteAccepted(ctx, method, url, body, &response)
		if err != nil {
			change.Outcome = failedOutcome(err)
		} else {
			if actual.Operation == "create" {
				change.OverrideID = response.ID
				if response.Override.ID > 0 {
					change.OverrideID = response.Override.ID
				}
			}
			change.Outcome = domain.ActionOutcome{Status: "succeeded", ID: change.OverrideID}
			actual.OverrideID = change.OverrideID
			actual.Outcome = change.Outcome
			applyAudienceOwnership(st, actual)
		}
		if err = save(); err != nil {
			return false, fmt.Errorf("Canvas audience action completed but its state could not be saved: %w", err)
		}
	}
	all := true
	for _, change := range plan.Changes {
		if !definite(change.Outcome) {
			all = false
		}
	}
	if all && !definite(st.Visibility) {
		if ambiguous(st.Visibility) {
			return false, nil
		}
		if err := s.checkAudience(ctx, course, a, assignment.ID); err != nil {
			st.Application = domain.ActionOutcome{Status: "held", Message: err.Error()}
			return false, save()
		}
		live, err := GetAssignment(ctx, s.Client, course.ID, assignment.ID)
		if err != nil {
			return false, err
		}
		if live.ID != assignment.ID || live.CourseID != course.ID || !refreshVisibilityMatches(st, live.OnlyVisibleToOverrides) || audienceManualFingerprint(st, live.Overrides) != st.OverrideBaseline {
			st.Application = domain.ActionOutcome{Status: "held", Message: "visibility or preserved exceptions changed; refresh restriction held"}
			return false, save()
		}
		st.Visibility = domain.ActionOutcome{Status: "pending"}
		if err = save(); err != nil {
			return false, err
		}
		err = s.Client.WriteAccepted(ctx, http.MethodPut, fmt.Sprintf("/api/v1/courses/%d/assignments/%d", course.ID, assignment.ID), map[string]any{"assignment": map[string]any{"only_visible_to_overrides": true}}, nil)
		if err != nil {
			st.Visibility = failedOutcome(err)
		} else {
			st.Visibility = domain.ActionOutcome{Status: "succeeded"}
		}
		if err = save(); err != nil {
			return false, err
		}
	}
	held := len(plan.Unknown) > 0
	for _, p := range plan.Protected {
		if !p.Authorized {
			held = true
		}
	}
	if all && definite(st.Visibility) && !held {
		st.Application = domain.ActionOutcome{Status: "succeeded"}
	} else {
		st.Application = domain.ActionOutcome{Status: "held", Message: "safe changes are retained; protected, unknown or unfinished audience changes remain"}
	}
	return definite(st.Application) && definite(st.Visibility), save()
}

func applyAudienceOwnership(st *domain.AudienceState, change domain.AudienceChange) {
	applied := stringSet(st.AppliedRefs)
	for _, ref := range change.Before.StudentRefs {
		delete(applied, ref)
	}
	for _, ref := range change.StudentRefs {
		applied[ref] = true
	}
	st.AppliedRefs = nil
	for ref := range applied {
		st.AppliedRefs = append(st.AppliedRefs, ref)
	}
	sort.Strings(st.AppliedRefs)

	if change.Operation == "create" {
		base := change.Before
		base.StudentRefs = append([]string(nil), change.StudentRefs...)
		st.Owned = append(st.Owned, domain.AudienceOverride{ID: change.OverrideID, Baseline: base, Outcome: change.Outcome})
		return
	}
	for i := range st.Owned {
		if st.Owned[i].ID != change.OverrideID {
			continue
		}
		if change.Operation == "remove" {
			st.Owned = append(st.Owned[:i], st.Owned[i+1:]...)
		} else {
			st.Owned[i].Baseline = change.Before
			st.Owned[i].Baseline.StudentRefs = append([]string(nil), change.StudentRefs...)
			st.Owned[i].Outcome = change.Outcome
		}
		return
	}
}
func managedAudienceRefs(st *domain.AudienceState, live Assignment, store *workspace.StudentStore, origin string) []string {
	ids := map[int64]bool{}
	for _, row := range st.Owned {
		ids[row.ID] = true
	}
	refs := []string{}
	for _, row := range live.Overrides {
		if ids[row.ID] {
			for _, id := range row.StudentIDs {
				for _, student := range store.Rows {
					if student.Instance == origin && student.UserID == id {
						refs = append(refs, student.Ref)
					}
				}
			}
		}
	}
	sort.Strings(refs)
	return refs
}
func stringSet(refs []string) map[string]bool {
	set := map[string]bool{}
	for _, ref := range refs {
		set[ref] = true
	}
	return set
}

func audienceRefreshSummary(st *domain.AudienceState) string {
	if st == nil || st.Refresh == nil {
		return ""
	}
	accepted, unfinished := 0, 0
	for _, change := range st.Refresh.Changes {
		if definite(change.Outcome) {
			accepted++
		} else {
			unfinished++
		}
	}
	parts := []string{fmt.Sprintf("%d audience changes completed; %d unfinished. Applied managed students: %s", accepted, unfinished, strings.Join(st.AppliedRefs, ", "))}
	for _, hold := range st.Refresh.Protected {
		if !hold.Authorized {
			parts = append(parts, hold.StudentRef+": protected access retained")
		}
	}
	for _, hold := range st.Refresh.Unknown {
		parts = append(parts, hold.StudentRef+": "+hold.Reason)
	}
	if st.Application.Message != "" {
		parts = append(parts, st.Application.Message)
	}
	return strings.Join(parts, "; ")
}

func refreshVisibilityMatches(st *domain.AudienceState, live *bool) bool {
	if live == nil {
		return false
	}
	if definite(st.Visibility) {
		return *live
	}
	return st.ObservedVisibility != nil && *live == *st.ObservedVisibility
}
