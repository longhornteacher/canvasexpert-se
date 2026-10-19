package workflow

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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

func audienceHash(v any) string {
	b, _ := json.Marshal(v)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
func sortedAudienceSource(src domain.AudienceSource) domain.AudienceSource {
	src.GroupIDs = append([]int64(nil), src.GroupIDs...)
	sort.Slice(src.GroupIDs, func(i, j int) bool { return src.GroupIDs[i] < src.GroupIDs[j] })
	return src
}

// resolveAudience reads only selected clubs. An unrelated club cannot hold this item.
func (s *Service) resolveAudience(ctx context.Context, course domain.Course, a *domain.Audience) (*domain.AudienceState, error) {
	if err := domain.ValidateAudience(a); err != nil {
		return nil, err
	}
	set, err := s.groupCategory(ctx, course.ID, a.Source.GroupCategoryID)
	if err != nil {
		return nil, err
	}
	st := &domain.AudienceState{Instance: s.Client.Origin(), CourseID: course.ID, Source: sortedAudienceSource(a.Source), ObservedAt: time.Now().UTC().Format(time.RFC3339Nano), Complete: true, StudentRefs: []string{}, Labels: []string{fmt.Sprintf("%s (set %d)", set.Name, set.ID)}}
	enrollments, err := StudentEnrollments(ctx, s.Client, course.ID)
	if err != nil {
		return nil, fmt.Errorf("course enrollment could not be read; audience was not prepared")
	}
	store, err := workspace.OpenStudents(s.Root)
	if err != nil {
		return nil, err
	}
	active := map[int64]bool{}
	states := map[int64][]string{}
	refs := map[int64]string{}
	for _, e := range enrollments {
		id := e.UserID
		if id == 0 {
			id = e.User.ID
		}
		if id <= 0 || e.Type != "StudentEnrollment" {
			continue
		}
		states[id] = append(states[id], e.State)
		if e.State == "active" {
			active[id] = true
		}
		student, e2 := store.Upsert(s.Client.Origin(), id, e.User.Name, e.User.ShortName, e.User.SISID, e.User.Login)
		if e2 != nil {
			return nil, fmt.Errorf("course enrollment could not be mapped privately")
		}
		refs[id] = student.Ref
	}
	if err = store.Save(); err != nil {
		return nil, err
	}
	selected := map[string]bool{}
	facts := []string{}
	for _, id := range st.Source.GroupIDs {
		var group canvasGroup
		if err = s.Client.Get(ctx, fmt.Sprintf("/api/v1/groups/%d", id), &group); err != nil {
			return nil, fmt.Errorf("selected club %d could not be read", id)
		}
		if group.ID != id || group.ContextType != "Course" || group.CourseID != course.ID || group.GroupCategoryID != a.Source.GroupCategoryID || nonCollaborative(group.NonCollaborative) {
			return nil, fmt.Errorf("selected club %d is outside this course's collaborative set", id)
		}
		st.Labels = append(st.Labels, fmt.Sprintf("%s (club %d)", group.Name, id))
		var memberships []canvasMembership
		if err = s.Client.List(ctx, fmt.Sprintf("/api/v1/groups/%d/memberships?per_page=100", id), &memberships); err != nil {
			st.Complete = false
			st.Reasons = append(st.Reasons, fmt.Sprintf("Club %d membership could not be read completely", id))
			facts = append(facts, fmt.Sprintf("%d:unreadable", id))
			continue
		}
		for _, m := range memberships {
			ref := refs[m.UserID]
			es := append([]string(nil), states[m.UserID]...)
			sort.Strings(es)
			// The fingerprint may include private IDs inside a hash; returned facts never do.
			facts = append(facts, fmt.Sprintf("%d:%d:%s:%s", id, m.UserID, m.WorkflowState, strings.Join(es, ",")))
			reason := ""
			switch {
			case m.GroupID != 0 && m.GroupID != id:
				reason = "membership returned outside selected club"
			case m.UserID <= 0 || ref == "":
				reason = "unresolved member"
			case m.WorkflowState != "accepted":
				reason = "membership is " + safeMembershipState(m.WorkflowState)
			case !active[m.UserID]:
				knownExcluded := len(es) > 0
				for _, state := range es {
					if state != "completed" && state != "inactive" && state != "deleted" {
						knownExcluded = false
					}
				}
				if knownExcluded {
					st.Exclusions = append(st.Exclusions, fmt.Sprintf("%s in club %d has no active enrollment and is excluded", ref, id))
					continue
				}
				reason = "member has no active student enrollment"
			}
			if reason != "" {
				st.Complete = false
				label := ref
				if label == "" {
					label = "unknown student"
				}
				st.Reasons = append(st.Reasons, fmt.Sprintf("Club %d: %s (%s)", id, reason, label))
				continue
			}
			selected[ref] = true
		}
	}
	for ref := range selected {
		st.StudentRefs = append(st.StudentRefs, ref)
	}
	sort.Strings(st.StudentRefs)
	sort.Strings(facts)
	sort.Strings(st.Reasons)
	st.Fingerprint = audienceHash(facts)
	return st, nil
}
func safeMembershipState(state string) string {
	switch state {
	case "requested", "invited", "accepted", "deleted", "rejected":
		return state
	}
	return "unsupported"
}

// PrepareAudience records a snapshot without writing to Canvas. Explicit refresh
// returns an assignment-dates action; initial preparation preserves ownership.
func (s *Service) PrepareAudience(ctx context.Context, nickname, path string, refresh bool) (*domain.AudienceState, error) {

	course, err := s.Course(nickname)
	if err != nil {
		return nil, err
	}
	if filepath.IsAbs(path) {
		path, err = filepath.Rel(s.Root, path)
		if err != nil {
			return nil, err
		}
	}
	constrained, err := workspace.ConstrainPath(s.Root, path)
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(s.Root, constrained)
	if err != nil || !strings.HasPrefix(filepath.ToSlash(rel), "courses/"+course.Nickname+"/") {
		return nil, fmt.Errorf("audience draft must be under courses/%s/", course.Nickname)
	}
	if refresh {
		kind, e := ReviewInputKind(constrained)
		if e != nil {
			return nil, e
		}
		switch kind {
		case "page":
			return s.preparePageAudienceRefresh(ctx, course, constrained)
		case "quiz":
			return s.prepareQuizAudienceRefresh(ctx, course, constrained)
		}
		return s.prepareAssignmentAudienceRefresh(ctx, course, constrained)
	}
	kind, err := ReviewInputKind(constrained)
	if err != nil {
		return nil, err
	}
	var a *domain.Audience
	var target int64
	var save func() error
	switch kind {
	case "assignment":
		var d domain.AssignmentDocument
		body, e := workspace.ReadDocument(constrained, &d)
		if e != nil {
			return nil, e
		}
		if e = domain.ValidateAssignment(d, body); e != nil {
			return nil, e
		}
		if d.Course != course.Nickname || (d.Instance != "" && d.Instance != s.Client.Origin()) || (d.CourseID != 0 && d.CourseID != course.ID) {
			return nil, fmt.Errorf("audience draft differs from configured course/Canvas")
		}
		a = d.Audience
		if d.Created != nil {
			target = d.Created.AssignmentID
		}
		save = func() error { return workspace.WriteDocument(constrained, &d, body) }
	case "assignment-dates":
		var d domain.AssignmentDatesDocument
		body, e := workspace.ReadDocument(constrained, &d)
		if e != nil {
			return nil, e
		}
		if e = domain.ValidateAssignmentDates(d); e != nil {
			return nil, e
		}
		if d.Course != course.Nickname || d.Instance != s.Client.Origin() || d.CourseID != course.ID {
			return nil, fmt.Errorf("audience draft differs from configured course/Canvas")
		}
		a = d.Audience
		target = d.AssignmentID
		save = func() error { return workspace.WriteDocument(constrained, &d, body) }
	case "page":
		var d domain.PageDocument
		body, e := workspace.ReadDocument(constrained, &d)
		if e != nil {
			return nil, e
		}
		if e = d.ValidateBody(body); e != nil {
			return nil, e
		}
		if d.Course != course.Nickname || (d.Instance != "" && d.Instance != s.Client.Origin()) || (d.CourseID != 0 && d.CourseID != course.ID) {
			return nil, fmt.Errorf("audience draft differs from configured course/Canvas")
		}
		if d.Audience == nil {
			return nil, fmt.Errorf("draft requires an audience intent")
		}
		return s.preparePageAudience(ctx, course, &d, func() error { return workspace.WriteDocument(constrained, &d, body) })
	case "quiz":
		q, e := ReadQuizForge(constrained)
		if e != nil {
			return nil, e
		}
		set, e := q.settings()
		if e != nil {
			return nil, e
		}
		if set.Audience == nil {
			return nil, fmt.Errorf("draft requires an audience intent")
		}
		rec := readSidecar(constrained)
		if rec.Instance != "" && (rec.Instance != s.Client.Origin() || rec.CourseID != 0 && rec.CourseID != course.ID) {
			return nil, fmt.Errorf("saved quiz sidecar belongs to a different Canvas/course")
		}
		return s.prepareQuizAudience(ctx, course, constrained, set.Audience, rec.AssignmentID)
	default:
		return nil, fmt.Errorf("audience preparation supports assignments, assignment-dates, course Pages and quizzes")
	}
	if a == nil {
		return nil, fmt.Errorf("draft requires an audience intent")
	}
	if a.State != nil && (len(a.State.Owned) > 0 || definite(a.State.Application) || ambiguous(a.State.Application) || definite(a.State.Visibility) || ambiguous(a.State.Visibility)) {
		return nil, fmt.Errorf("audience action already began; preserve its state and use explicit refresh")
	}
	state, err := s.resolveAudience(ctx, course, a)
	if err != nil {
		return nil, err
	}
	state.TargetID = target
	if target > 0 {
		live, e := GetAssignment(ctx, s.Client, course.ID, target)
		if e != nil {
			return nil, e
		}
		if live.ID != target || live.CourseID != course.ID || live.GroupCategoryID != nil && *live.GroupCategoryID > 0 {
			return nil, fmt.Errorf("audience target must be an individual assignment in this course")
		}
		if err = validateAudienceOverrides(live.Overrides); err != nil {
			return nil, err
		}
		if live.OnlyVisibleToOverrides == nil {
			return nil, fmt.Errorf("assignment visibility could not be read completely; audience was not prepared")
		}
		state.ObservedVisibility = live.OnlyVisibleToOverrides
		state.OverrideBaseline = overridesFingerprint(live.Overrides)
		_, state.Exceptions, err = s.assignmentAudienceRefs(state, live)
		if err != nil {
			return nil, err
		}
	}
	a.State = state
	if err = save(); err != nil {
		return nil, err
	}
	return state, nil
}

func (s *Service) checkAudience(ctx context.Context, course domain.Course, a *domain.Audience, target int64) error {
	if a == nil {
		return nil
	}
	st := a.State
	if st == nil {
		return fmt.Errorf("prepare this audience before review or push")
	}
	if st.Instance != s.Client.Origin() || st.CourseID != course.ID || st.TargetID != 0 && st.TargetID != target || !reflect.DeepEqual(sortedAudienceSource(a.Source), st.Source) {
		return fmt.Errorf("audience preparation belongs to different source, Canvas, course or target")
	}
	if !st.Complete {
		return fmt.Errorf("audience is incomplete: %s", strings.Join(st.Reasons, "; "))
	}
	if len(st.StudentRefs) == 0 && st.Refresh == nil {
		return fmt.Errorf("selected clubs have no eligible members; keep this draft and prepare again after members are added")
	}
	fresh, err := s.resolveAudience(ctx, course, a)
	if err != nil {
		return err
	}
	if !fresh.Complete || fresh.Fingerprint != st.Fingerprint || !reflect.DeepEqual(fresh.StudentRefs, st.StudentRefs) {
		return fmt.Errorf("selected membership or enrollment changed; explicitly prepare the audience again")
	}
	return nil
}

// applyAssignmentAudience owns only the exact override it creates. Existing
// ad hoc exceptions keep their students and dates, and are never adopted.
func (s *Service) applyAssignmentAudience(ctx context.Context, course domain.Course, assignment Assignment, a *domain.Audience, save func() error) (bool, error) {
	if a == nil {
		return true, nil
	}
	st := a.State
	if st != nil && st.Refresh != nil {
		return s.applyAssignmentAudienceRefresh(ctx, course, assignment, a, save)
	}
	if st != nil && ambiguous(st.Application) && len(st.Owned) > 0 {
		all := true
		for _, owned := range st.Owned {
			if !definite(owned.Outcome) {
				all = false
			}
		}
		if all {
			st.Application = domain.ActionOutcome{Status: "succeeded"}
			if err := save(); err != nil {
				return false, err
			}
		}
	}
	if err := validateAudienceOverrides(assignment.Overrides); err != nil {
		return false, err
	}
	if st != nil && definite(st.Application) && definite(st.Visibility) {
		return true, nil
	}
	if st != nil && definite(st.Application) {
		if err := s.acceptedAudienceBaseline(a, assignment); err != nil {
			return false, err
		}
	}
	if err := s.checkAudience(ctx, course, a, assignment.ID); err != nil {
		if st != nil {
			if !ambiguous(st.Application) && !definite(st.Application) {
				st.Application = domain.ActionOutcome{Status: "held", Message: err.Error()}
			}
			_ = save()
		}
		return false, nil
	}
	if assignment.ID <= 0 || assignment.CourseID != course.ID || assignment.GroupCategoryID != nil && *assignment.GroupCategoryID > 0 {
		return false, fmt.Errorf("audience requires an exact individual assignment")
	}
	if st.TargetID == 0 {
		st.TargetID = assignment.ID
		if err := save(); err != nil {
			return false, err
		}
	}
	if !definite(st.Visibility) && !ambiguous(st.Visibility) && st.ObservedVisibility != nil && (assignment.OnlyVisibleToOverrides == nil || *assignment.OnlyVisibleToOverrides != *st.ObservedVisibility) {
		st.Application = domain.ActionOutcome{Status: "held", Message: "assignment visibility changed since preparation; prepare again"}
		return false, save()
	}
	if st.OverrideBaseline != "" && st.OverrideBaseline != audienceManualFingerprint(st, assignment.Overrides) {
		if !ambiguous(st.Application) && !definite(st.Application) {
			st.Application = domain.ActionOutcome{Status: "held", Message: "assignment audience changed since preparation; prepare again"}
		}
		return false, save()
	}
	if !definite(st.Application) {
		if ambiguous(st.Application) {
			return false, nil
		}
		if len(st.Owned) == 0 {
			if st.OverrideBaseline == "" {
				st.OverrideBaseline = overridesFingerprint(assignment.Overrides)
			}
			refs, exceptions, err := s.assignmentAudienceRefs(st, assignment)
			if err != nil {
				return false, err
			}
			st.Exceptions = exceptions
			if len(refs) > 0 {
				st.Owned = []domain.AudienceOverride{{Baseline: domain.AssignmentOverrideBaseline{Title: "Book club individual audience", StudentRefs: refs, DueAtOverridden: audienceBool(false), UnlockAtOverridden: audienceBool(false), LockAtOverridden: audienceBool(false)}}}
			}
		}
		for i := range st.Owned {
			row := &st.Owned[i]
			if definite(row.Outcome) {
				continue
			}
			if ambiguous(row.Outcome) {
				return false, nil
			}
			store, err := workspace.OpenStudents(s.Root)
			if err != nil {
				return false, err
			}
			ids, err := resolveOverrideTarget(ctx, s, course.ID, assignment, store, domain.AssignmentOverrideRow{StudentRefs: row.Baseline.StudentRefs})
			if err != nil {
				return false, err
			}
			row.Outcome = domain.ActionOutcome{Status: "pending"}
			st.Application = domain.ActionOutcome{Status: "pending"}
			if err = save(); err != nil {
				return false, err
			}
			var result struct {
				ID       int64                  `json:"id"`
				Override AssignmentOverrideLive `json:"assignment_override"`
			}
			err = s.Client.WriteAccepted(ctx, http.MethodPost, fmt.Sprintf("/api/v1/courses/%d/assignments/%d/overrides", course.ID, assignment.ID), map[string]any{"assignment_override": map[string]any{"title": row.Baseline.Title, "student_ids": ids}}, &result)
			if err != nil {
				row.Outcome = failedOutcome(err)
				st.Application = row.Outcome
				_ = save()
				return false, nil
			}
			row.ID = result.ID
			if result.Override.ID > 0 {
				row.ID = result.Override.ID
			}
			row.Outcome = domain.ActionOutcome{Status: "succeeded", ID: row.ID}
			if err = save(); err != nil {
				return false, err
			}
		}
		st.Application = domain.ActionOutcome{Status: "succeeded"}
		if err := save(); err != nil {
			return false, err
		}
	}
	if !definite(st.Visibility) {
		if ambiguous(st.Visibility) {
			return false, nil
		}
		st.Visibility = domain.ActionOutcome{Status: "pending"}
		if err := save(); err != nil {
			return false, err
		}
		err := s.Client.WriteAccepted(ctx, http.MethodPut, fmt.Sprintf("/api/v1/courses/%d/assignments/%d", course.ID, assignment.ID), map[string]any{"assignment": map[string]any{"only_visible_to_overrides": true}}, nil)
		if err != nil {
			st.Visibility = failedOutcome(err)
		} else {
			st.Visibility = domain.ActionOutcome{Status: "succeeded"}
		}
		if err = save(); err != nil {
			return false, err
		}
	}
	return definite(st.Application) && definite(st.Visibility), nil
}

func (s *Service) assignmentAudienceRefs(st *domain.AudienceState, assignment Assignment) ([]string, []string, error) {
	store, err := workspace.OpenStudents(s.Root)
	if err != nil {
		return nil, nil, err
	}
	manual := map[int64]bool{}
	exceptions := []string{}
	desiredIDs := map[int64]bool{}
	refs := []string{}
	for _, row := range assignment.Overrides {
		for _, id := range row.StudentIDs {
			manual[id] = true
		}
		if row.GroupID > 0 || row.CourseSectionID > 0 {
			exceptions = append(exceptions, fmt.Sprintf("Override %d preserves additional group or section access", row.ID))
		}
	}
	for _, ref := range st.StudentRefs {
		student, err := store.Resolve(s.Client.Origin(), ref)
		if err != nil {
			return nil, nil, err
		}
		desiredIDs[student.UserID] = true
		if manual[student.UserID] {
			exceptions = append(exceptions, ref+" retains an existing student exception")
		} else {
			refs = append(refs, ref)
		}
	}
	for id := range manual {
		if !desiredIDs[id] {
			exceptions = append(exceptions, "Existing student override preserves access beyond the selected clubs; exclusive club access is not achieved")
			break
		}
	}
	return refs, exceptions, nil
}

// acceptedAudienceBaseline holds unfinished dependent actions when later teacher
// changes invalidate a completed restriction. It never restores those edits.
func (s *Service) acceptedAudienceBaseline(a *domain.Audience, live Assignment) error {
	if a == nil || a.State == nil {
		return nil
	}
	st := a.State
	if definite(st.Visibility) && (live.OnlyVisibleToOverrides == nil || !*live.OnlyVisibleToOverrides) {
		return fmt.Errorf("Canvas visibility changed after the accepted restriction; publication held and the teacher's change preserved")
	}
	if err := validateAudienceOverrides(live.Overrides); err != nil {
		return err
	}
	if st.OverrideBaseline != "" && audienceManualFingerprint(st, live.Overrides) != st.OverrideBaseline {
		return fmt.Errorf("other assignment audience exceptions changed after preparation; publication held without changing them")
	}
	store, err := workspace.OpenStudents(s.Root)
	if err != nil {
		return err
	}
	for _, owned := range st.Owned {
		if !definite(owned.Outcome) {
			continue
		}
		if owned.ID == 0 {
			return fmt.Errorf("accepted audience has no saved override ID; publication resume needs an exact identity before continuing")
		}
		found := false
		for _, row := range live.Overrides {
			if row.ID == owned.ID {
				found = true
				if !overrideMatchesBaseline(row, owned.Baseline, store, s.Client.Origin()) {
					return fmt.Errorf("Canvas audience changed after its accepted write; publication held and the teacher's change preserved")
				}
			}
		}
		if !found {
			return fmt.Errorf("accepted audience override is no longer present; publication held")
		}
	}
	return nil
}

func audienceBool(v bool) *bool { return &v }
func overridesFingerprint(rows []AssignmentOverrideLive) string {
	copyRows := append([]AssignmentOverrideLive{}, rows...)
	for i := range copyRows {
		copyRows[i].StudentIDs = append([]int64{}, copyRows[i].StudentIDs...)
		sort.Slice(copyRows[i].StudentIDs, func(a, b int) bool { return copyRows[i].StudentIDs[a] < copyRows[i].StudentIDs[b] })
	}
	sort.Slice(copyRows, func(i, j int) bool { return copyRows[i].ID < copyRows[j].ID })
	return audienceHash(copyRows)
}

func audienceManualFingerprint(st *domain.AudienceState, rows []AssignmentOverrideLive) string {
	ownedIDs := map[int64]bool{}
	for _, owned := range st.Owned {
		if owned.ID > 0 && definite(owned.Outcome) {
			ownedIDs[owned.ID] = true
		}
	}
	manual := []AssignmentOverrideLive{}
	for _, row := range rows {
		if !ownedIDs[row.ID] {
			manual = append(manual, row)
		}
	}
	return overridesFingerprint(manual)
}
func validateAudienceOverrides(rows []AssignmentOverrideLive) error {
	for _, row := range rows {
		if row.CourseID != 0 || len(row.NoopID) > 0 && string(row.NoopID) != "null" || row.UnassignItem != nil && *row.UnassignItem {
			return fmt.Errorf("override %d has a course-wide, negative or unsupported target; its access cannot be preserved as an individual club restriction", row.ID)
		}
		targets := 0
		if row.GroupID > 0 {
			targets++
		}
		if row.CourseSectionID > 0 {
			targets++
		}
		if len(row.StudentIDs) > 0 {
			targets++
		}
		if row.ID <= 0 || targets != 1 {
			return fmt.Errorf("override %d has an unsupported audience target; restriction held", row.ID)
		}
	}
	return nil
}
