package workflow

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"canvasexpertse/internal/canvas"
	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

type canvasMembership struct {
	ID            int64  `json:"id"`
	GroupID       int64  `json:"group_id"`
	UserID        int64  `json:"user_id"`
	WorkflowState string `json:"workflow_state"`
}

type groupRosterLive struct {
	Roster   domain.GroupRoster
	Groups   []canvasGroup
	ByRef    map[string]domain.GroupMembershipObserved
	ByUserID map[int64]workspace.Student
	ByRefID  map[string]int64
	Pending  map[int64]bool
}

type allocatedGroupResponse struct {
	ID         int64 `json:"id"`
	NewMembers []struct {
		UserID int64 `json:"user_id"`
	} `json:"new_members"`
}

func (s *Service) groupRosterLive(ctx context.Context, nickname string, setID int64) (*groupRosterLive, error) {
	course, err := s.Course(nickname)
	if err != nil {
		return nil, err
	}
	set, err := s.groupCategory(ctx, course.ID, setID)
	if err != nil {
		return nil, err
	}
	var groups []canvasGroup
	if err = s.Client.List(ctx, fmt.Sprintf("/api/v1/group_categories/%d/groups?per_page=100", setID), &groups); err != nil {
		return nil, err
	}
	for _, g := range groups {
		if g.ID <= 0 || g.ContextType != "Course" || g.CourseID != course.ID || g.GroupCategoryID != setID || nonCollaborative(g.NonCollaborative) {
			return nil, fmt.Errorf("Canvas returned a group outside collaborative set %d", setID)
		}
	}
	enrollments, err := StudentEnrollments(ctx, s.Client, course.ID)
	if err != nil {
		return nil, err
	}
	students, err := workspace.OpenStudents(s.Root)
	if err != nil {
		return nil, err
	}
	byUser := map[int64]workspace.Student{}
	eligible := map[int64]bool{}
	refIDs := map[string]int64{}
	for _, enrollment := range enrollments {
		id := enrollment.UserID
		if id == 0 {
			id = enrollment.User.ID
		}
		if id <= 0 || enrollment.Type != "StudentEnrollment" {
			continue
		}
		st, e := students.Upsert(s.Client.Origin(), id, enrollment.User.Name, enrollment.User.ShortName, enrollment.User.SISID, enrollment.User.Login)
		if e != nil {
			return nil, fmt.Errorf("course roster could not be mapped to stable student references: %w", e)
		}
		byUser[id] = st
		refIDs[st.Ref] = id
		if enrollment.State == "active" || enrollment.State == "invited" {
			eligible[id] = true
		}
	}
	if err = students.Save(); err != nil {
		return nil, fmt.Errorf("save stable student references: %w", err)
	}
	live := &groupRosterLive{Roster: domain.GroupRoster{Course: course.Nickname, CourseID: course.ID, GroupCategoryID: setID, GroupSetName: set.Name, RosterComplete: true}, Groups: groups, ByRef: map[string]domain.GroupMembershipObserved{}, ByUserID: byUser, ByRefID: refIDs, Pending: map[int64]bool{}}
	groupIndex := map[int64]int{}
	for i, g := range groups {
		groupIndex[g.ID] = i
		live.Roster.Groups = append(live.Roster.Groups, domain.GroupSummary{ID: g.ID, CourseID: course.ID, GroupCategoryID: setID, Name: g.Name, Description: g.Description, Collaborative: true, MembersCount: g.MembersCount, RosterComplete: true})
	}
	assigned := map[int64]bool{}
	for _, g := range groups {
		var memberships []canvasMembership
		if err = s.Client.List(ctx, fmt.Sprintf("/api/v1/groups/%d/memberships?per_page=100", g.ID), &memberships); err != nil {
			return nil, err
		}
		for _, m := range memberships {
			if m.UserID <= 0 {
				live.Roster.RosterComplete = false
				live.Roster.UnresolvedCount++
				continue
			}
			if m.WorkflowState == "requested" {
				live.Pending[m.UserID] = true
				live.Roster.PendingCount++
				continue
			}
			if m.WorkflowState != "accepted" && m.WorkflowState != "invited" {
				continue
			}
			st, ok := byUser[m.UserID]
			if !ok {
				live.Roster.RosterComplete = false
				live.Roster.UnresolvedCount++
				continue
			}
			if prior, exists := live.ByRef[st.Ref]; exists && prior.GroupID != g.ID {
				live.Roster.RosterComplete = false
				live.Roster.UnresolvedCount++
				continue
			}
			assigned[m.UserID] = true
			live.ByRef[st.Ref] = domain.GroupMembershipObserved{StudentRef: st.Ref, GroupID: g.ID, State: m.WorkflowState}
			ms := domain.GroupMember{StudentRef: st.Ref, Alias: st.Alias, State: m.WorkflowState}
			idx := groupIndex[g.ID]
			live.Roster.Groups[idx].Members = append(live.Roster.Groups[idx].Members, ms)
		}
	}
	for id, st := range byUser {
		if !eligible[id] || assigned[id] || live.Pending[id] {
			continue
		}
		live.Roster.Unassigned = append(live.Roster.Unassigned, domain.GroupMember{StudentRef: st.Ref, Alias: st.Alias, State: "unassigned"})
	}
	for i := range live.Roster.Groups {
		live.Roster.Groups[i].RosterComplete = live.Roster.RosterComplete
		sort.Slice(live.Roster.Groups[i].Members, func(a, b int) bool {
			return live.Roster.Groups[i].Members[a].StudentRef < live.Roster.Groups[i].Members[b].StudentRef
		})
	}
	sort.Slice(live.Roster.Unassigned, func(i, j int) bool {
		return live.Roster.Unassigned[i].StudentRef < live.Roster.Unassigned[j].StudentRef
	})
	return live, nil
}

func (s *Service) ReadGroupRoster(ctx context.Context, nickname string, setID int64) (domain.GroupRoster, error) {
	live, err := s.groupRosterLive(ctx, nickname, setID)
	if err != nil {
		return domain.GroupRoster{}, err
	}
	return live.Roster, nil
}

func (s *Service) PullGroupMemberships(ctx context.Context, nickname string, setID int64) (string, error) {
	live, err := s.groupRosterLive(ctx, nickname, setID)
	if err != nil {
		return "", err
	}
	if !live.Roster.RosterComplete {
		return "", fmt.Errorf("membership roster is incomplete (%d unresolved memberships); no editable plan was saved", live.Roster.UnresolvedCount)
	}
	snapshot := &domain.GroupMembershipSnapshot{Instance: s.Client.Origin(), CourseID: live.Roster.CourseID, GroupCategoryID: setID, RosterComplete: true}
	for _, g := range live.Groups {
		snapshot.GroupIDs = append(snapshot.GroupIDs, g.ID)
	}
	for _, m := range live.ByRef {
		snapshot.Memberships = append(snapshot.Memberships, m)
	}
	for _, m := range live.Roster.Unassigned {
		snapshot.UnassignedRefs = append(snapshot.UnassignedRefs, m.StudentRef)
	}
	sort.Slice(snapshot.Memberships, func(i, j int) bool { return snapshot.Memberships[i].StudentRef < snapshot.Memberships[j].StudentRef })
	doc := domain.GroupMembershipDocument{Schema: 1, Kind: "group-memberships", Operation: "update", Course: live.Roster.Course, Instance: s.Client.Origin(), CourseID: live.Roster.CourseID, GroupCategoryID: setID, Observed: snapshot}
	name := fmt.Sprintf("group-memberships-%d-%s.md", setID, time.Now().UTC().Format("20060102T150405.000000000"))
	path, err := workspace.ConstrainPath(s.Root, filepath.Join("courses", live.Roster.Course, "pulled", name))
	if err != nil {
		return "", err
	}
	if err = workspace.WriteDocument(path, doc, ""); err != nil {
		return "", err
	}
	return path, nil
}

type GroupMembershipReport struct {
	Status   string
	Message  string
	Outcomes []domain.GroupMembershipOutcome
	Assigned []string
}

func (s *Service) PushGroupMemberships(ctx context.Context, path string) (report GroupMembershipReport, err error) {
	var doc domain.GroupMembershipDocument
	if _, err = workspace.ReadDocument(path, &doc); err != nil {
		return report, err
	}
	if err = doc.Validate(); err != nil {
		return report, err
	}
	course, err := s.Course(doc.Course)
	if err != nil {
		return report, err
	}
	if doc.Instance != s.Client.Origin() || doc.CourseID != course.ID {
		return report, fmt.Errorf("group-memberships document belongs to a different Canvas instance or course")
	}
	set, err := s.groupCategory(ctx, course.ID, doc.GroupCategoryID)
	if err != nil {
		return report, err
	}
	if set.ID != doc.GroupCategoryID {
		return report, fmt.Errorf("selected group set changed identity")
	}
	save := func() error { return workspace.WriteDocument(path, &doc, "") }
	live, err := s.groupRosterLive(ctx, course.Nickname, set.ID)
	if err != nil {
		return report, err
	}
	if !live.Roster.RosterComplete {
		return report, fmt.Errorf("current group roster is incomplete; membership writes are held")
	}
	if !sameGroupIDs(doc.Observed.GroupIDs, live.Roster.Groups) {
		return report, fmt.Errorf("group set changed since the membership baseline; refresh the plan before writing")
	}
	students, err := workspace.OpenStudents(s.Root)
	if err != nil {
		return report, err
	}
	// Resolve every ref and every group before the first write. This makes a
	// cross-course or tag-group plan a zero-write failure.
	userIDs := map[string]int64{}
	for _, c := range doc.Changes {
		st, e := students.Resolve(s.Client.Origin(), c.StudentRef)
		if e != nil {
			return report, e
		}
		if _, enrolled := live.ByUserID[st.UserID]; !enrolled {
			return report, fmt.Errorf("student_ref %s is not currently enrolled in the selected course", c.StudentRef)
		}
		userIDs[c.StudentRef] = st.UserID
	}
	for _, id := range doc.Observed.GroupIDs {
		var group canvasGroup
		if err = s.Client.Get(ctx, fmt.Sprintf("/api/v1/groups/%d", id), &group); err != nil {
			return report, err
		}
		if group.ID != id || group.CourseID != course.ID || group.ContextType != "Course" || group.GroupCategoryID != set.ID || nonCollaborative(group.NonCollaborative) {
			return report, fmt.Errorf("group %d does not belong to this collaborative course set", id)
		}
	}
	if doc.Operation == "allocate-unassigned" {
		return s.allocateUnassigned(ctx, path, &doc, live)
	}
	assignments, err := Assignments(ctx, s.Client, course.ID)
	if err != nil {
		return report, err
	}
	for i := range doc.Changes {
		change := doc.Changes[i]
		out := s.membershipOutcome(&doc, change.StudentRef)
		if out == nil {
			doc.Outcomes = append(doc.Outcomes, domain.GroupMembershipOutcome{StudentRef: change.StudentRef})
			out = &doc.Outcomes[len(doc.Outcomes)-1]
		}
		uid := userIDs[change.StudentRef]
		// Membership can change while another row is being processed. Read this
		// student's source again immediately before deciding on its mutation.
		rowLive, readErr := s.groupRosterLive(ctx, course.Nickname, set.ID)
		if readErr != nil || !rowLive.Roster.RosterComplete {
			out.Outcome = domain.ActionOutcome{Status: "held", Message: "could not re-read a complete current membership roster; this row is preserved"}
			_ = save()
			continue
		}
		current, assigned := rowLive.ByRef[change.StudentRef]
		currentGroup := int64(0)
		if assigned {
			currentGroup = current.GroupID
		}
		destination := int64(0)
		if !change.ToGroup.Null {
			destination = change.ToGroup.Value
		}
		if currentGroup == destination && (destination > 0 || (!assigned && change.ToGroup.Null)) {
			out.Outcome = domain.ActionOutcome{Status: "preserved", Message: "student is already in the requested membership state"}
			if err = save(); err != nil {
				return report, err
			}
			continue
		}
		from := int64(0)
		if !change.FromGroup.Null {
			from = change.FromGroup.Value
		}
		if currentGroup != from {
			out.Outcome = domain.ActionOutcome{Status: "held", Message: fmt.Sprintf("membership changed since baseline (currently group %d)", currentGroup)}
			_ = save()
			continue
		}
		if from > 0 || destination > 0 {
			hasWork, workErr := hasSubmittedSetWork(ctx, s, course.ID, set.ID, uid, assignments)
			if workErr != nil {
				out.Outcome = domain.ActionOutcome{Status: "held", Message: "could not verify submitted work for this move; membership is preserved"}
				_ = save()
				continue
			}
			if hasWork && !containsRef(doc.AuthorizedWorkChangeRefs, change.StudentRef) {
				out.Outcome = domain.ActionOutcome{Status: "held", Message: "student has submitted work in this group set; explicitly authorize this student's membership change before changing access"}
				_ = save()
				continue
			}
		}
		if destination > 0 {
			var group canvasGroup
			if err = s.Client.Get(ctx, fmt.Sprintf("/api/v1/groups/%d", destination), &group); err != nil {
				out.Outcome = failedOutcome(err)
				_ = save()
				continue
			}
			if group.ID != destination || group.CourseID != course.ID || group.GroupCategoryID != set.ID || group.ContextType != "Course" || nonCollaborative(group.NonCollaborative) {
				out.Outcome = domain.ActionOutcome{Status: "held", Message: "destination group no longer belongs to the selected collaborative set"}
				_ = save()
				continue
			}
			out.Outcome = domain.ActionOutcome{Status: "pending"}
			if err = save(); err != nil {
				return report, err
			}
			var actual canvasMembership
			err = s.Client.Write(ctx, "POST", fmt.Sprintf("/api/v1/groups/%d/memberships", destination), map[string]any{"user_id": uid}, &actual)
			if err == nil && actual.UserID != 0 && actual.UserID != uid {
				err = fmt.Errorf("Canvas returned a membership for a different student")
			}
			if err == nil && actual.GroupID != 0 && actual.GroupID != destination {
				err = fmt.Errorf("Canvas returned a membership in a different group")
			}
			if err == nil {
				id := destination
				out.ActualGroupID = &id
				out.ActualState = actual.WorkflowState
			}
		} else {
			out.Outcome = domain.ActionOutcome{Status: "pending"}
			if err = save(); err != nil {
				return report, err
			}
			err = s.Client.Write(ctx, "DELETE", fmt.Sprintf("/api/v1/groups/%d/users/%d", from, uid), nil, nil)
			if err == nil {
				zero := int64(0)
				out.ActualGroupID = &zero
				out.ActualState = "unassigned"
			}
		}
		if err != nil {
			if uncertainWrite(err) {
				out.Outcome = domain.ActionOutcome{Status: "uncertain", Message: "Canvas membership write is uncertain; reread current membership before retrying"}
			} else {
				out.Outcome = failedOutcome(err)
			}
			_ = save()
			continue
		}
		out.Outcome = domain.ActionOutcome{Status: "succeeded"}
		if err = save(); err != nil {
			return report, err
		}
	}
	report.Outcomes = doc.Outcomes
	report.Status = groupMembershipStatus(doc.Outcomes)
	return report, nil
}

func (s *Service) membershipOutcome(doc *domain.GroupMembershipDocument, ref string) *domain.GroupMembershipOutcome {
	for i := range doc.Outcomes {
		if doc.Outcomes[i].StudentRef == ref {
			return &doc.Outcomes[i]
		}
	}
	return nil
}

func containsRef(refs []string, target string) bool {
	for _, ref := range refs {
		if ref == target {
			return true
		}
	}
	return false
}

func sameGroupIDs(observed []int64, current []domain.GroupSummary) bool {
	if len(observed) != len(current) {
		return false
	}
	ids := make(map[int64]bool, len(observed))
	for _, id := range observed {
		ids[id] = true
	}
	for _, group := range current {
		if !ids[group.ID] {
			return false
		}
		delete(ids, group.ID)
	}
	return len(ids) == 0
}

func uncertainWrite(err error) bool {
	var ce *canvas.Error
	return errors.As(err, &ce) && ce.Uncertain
}

func groupMembershipStatus(outcomes []domain.GroupMembershipOutcome) string {
	status := "completed"
	for _, o := range outcomes {
		switch o.Outcome.Status {
		case "held", "failed", "uncertain":
			return "partial"
		}
	}
	return status
}

func hasSubmittedSetWork(ctx context.Context, s *Service, courseID, setID, userID int64, assignments []Assignment) (bool, error) {
	for _, a := range assignments {
		if a.GroupCategoryID == nil || *a.GroupCategoryID != setID || a.ID <= 0 {
			continue
		}
		submissions, err := Submissions(ctx, s.Client, courseID, a.ID, false)
		if err != nil {
			return false, err
		}
		for _, submission := range submissions {
			if submission.UserID == userID && submission.SubmittedAt != nil && *submission.SubmittedAt != "" {
				return true, nil
			}
		}
	}
	return false, nil
}

func (s *Service) allocateUnassigned(ctx context.Context, path string, doc *domain.GroupMembershipDocument, before *groupRosterLive) (GroupMembershipReport, error) {
	report := GroupMembershipReport{}
	if doc.Allocation == nil {
		doc.Allocation = &domain.GroupAllocationOutcome{}
	}
	if doc.Allocation.Outcome.Status == "succeeded" {
		assigned := append([]string(nil), doc.Allocation.AssignedRefs...)
		report.Status, report.Assigned, report.Message = "completed", assigned, doc.Allocation.Message
		return report, nil
	}
	// The previous broad preflight is not the allocation audience. Read again
	// just before the non-idempotent synchronous endpoint.
	current, err := s.groupRosterLive(ctx, doc.Course, doc.GroupCategoryID)
	if err != nil {
		return report, err
	}
	if !current.Roster.RosterComplete {
		return report, fmt.Errorf("current group roster is incomplete; allocation is held")
	}
	before = current
	if doc.Allocation.Outcome.Status == "pending" || doc.Allocation.Outcome.Status == "uncertain" {
		after, err := s.groupRosterLive(ctx, doc.Course, doc.GroupCategoryID)
		if err != nil {
			return report, err
		}
		started := map[string]bool{}
		for _, ref := range doc.Allocation.StartedUnassignedRefs {
			started[ref] = true
		}
		assigned := newlyAssignedRefsFromSet(started, after)
		if len(assigned) > 0 {
			doc.Allocation.AssignedRefs = assigned
			doc.Allocation.Outcome = domain.ActionOutcome{Status: "succeeded"}
			doc.Allocation.Message = "resolved from current group membership after an uncertain allocation"
			if err = workspace.WriteDocument(path, doc, ""); err != nil {
				return report, err
			}
			report.Status, report.Assigned, report.Message = "completed", assigned, doc.Allocation.Message
			return report, nil
		}
		doc.Allocation.Outcome = domain.ActionOutcome{Status: "uncertain", Message: "Canvas allocation outcome remains unclear; do not rerun automatically"}
		_ = workspace.WriteDocument(path, doc, "")
		report.Status, report.Message = "uncertain", doc.Allocation.Outcome.Message
		return report, nil
	}
	if !sameGroupIDs(doc.Observed.GroupIDs, before.Roster.Groups) {
		doc.Allocation.Outcome = domain.ActionOutcome{Status: "held", Message: "group set changed since the baseline; refresh the allocation plan before retrying"}
		_ = workspace.WriteDocument(path, doc, "")
		report.Status, report.Message = "held", doc.Allocation.Outcome.Message
		return report, nil
	}
	observed := map[string]bool{}
	for _, ref := range doc.Observed.UnassignedRefs {
		observed[ref] = true
	}
	for _, member := range before.Roster.Unassigned {
		if !observed[member.StudentRef] {
			doc.Allocation.Outcome = domain.ActionOutcome{Status: "held", Message: "new unassigned students appeared after the baseline; refresh the plan before allocation"}
			_ = workspace.WriteDocument(path, doc, "")
			report.Status, report.Message = "held", doc.Allocation.Outcome.Message
			return report, nil
		}
	}
	for _, m := range before.Roster.Unassigned {
		doc.Allocation.StartedUnassignedRefs = append(doc.Allocation.StartedUnassignedRefs, m.StudentRef)
	}
	// Students assigned since the baseline (including a late self-sign-up) are
	// preserved and omitted from Canvas's allocate-unassigned audience.
	doc.Allocation.Started = true
	doc.Allocation.Outcome = domain.ActionOutcome{Status: "pending"}
	if err := workspace.WriteDocument(path, doc, ""); err != nil {
		return report, err
	}
	pathURL := fmt.Sprintf("/api/v1/group_categories/%d/assign_unassigned_members?sync=true", doc.GroupCategoryID)
	var created []allocatedGroupResponse
	if err := s.Client.Write(ctx, "POST", pathURL, map[string]any{"sync": true}, &created); err != nil {
		if uncertainWrite(err) {
			doc.Allocation.Outcome = domain.ActionOutcome{Status: "uncertain", Message: "Canvas allocation is uncertain; reread current membership before any retry"}
		} else {
			doc.Allocation.Outcome = failedOutcome(err)
		}
		_ = workspace.WriteDocument(path, doc, "")
		if uncertainWrite(err) {
			report.Status, report.Message = "uncertain", doc.Allocation.Outcome.Message
		} else {
			report.Status, report.Message = doc.Allocation.Outcome.Status, err.Error()
		}
		return report, nil
	}
	assignedSet := map[string]bool{}
	returnedMembers := 0
	for _, group := range created {
		for _, member := range group.NewMembers {
			returnedMembers++
			if student, ok := before.ByUserID[member.UserID]; ok {
				assignedSet[student.Ref] = true
			} else {
				doc.Allocation.UnmappedAssignedCount++
			}
		}
	}
	assigned := make([]string, 0, len(assignedSet))
	for ref := range assignedSet {
		assigned = append(assigned, ref)
	}
	sort.Strings(assigned)
	doc.Allocation.AssignedRefs = assigned
	doc.Allocation.Outcome = domain.ActionOutcome{Status: "succeeded"}
	doc.Allocation.Message = fmt.Sprintf("Canvas assigned %d student(s)", returnedMembers)
	if doc.Allocation.UnmappedAssignedCount > 0 {
		doc.Allocation.Message += fmt.Sprintf("; %d could not be mapped to a current student reference", doc.Allocation.UnmappedAssignedCount)
	}
	if err = workspace.WriteDocument(path, doc, ""); err != nil {
		return report, err
	}
	report.Status, report.Assigned, report.Message = "completed", assigned, doc.Allocation.Message
	return report, nil
}

func newlyAssignedRefsFromSet(before map[string]bool, live *groupRosterLive) []string {
	assigned := []string{}
	for ref := range before {
		if _, ok := live.ByRef[ref]; ok {
			assigned = append(assigned, ref)
		}
	}
	sort.Strings(assigned)
	return assigned
}
