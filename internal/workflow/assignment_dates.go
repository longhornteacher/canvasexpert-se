package workflow

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

type AssignmentDatesReport struct {
	Path    string
	Status  string
	Message string
	Rows    []domain.AssignmentOverrideRow
}

func (s *Service) PullAssignmentDates(ctx context.Context, nickname string, assignmentID int64) (AssignmentDatesReport, error) {
	course, err := s.Course(nickname)
	if err != nil {
		return AssignmentDatesReport{}, err
	}
	if assignmentID <= 0 {
		return AssignmentDatesReport{}, fmt.Errorf("assignment ID must be positive")
	}
	a, err := GetAssignment(ctx, s.Client, course.ID, assignmentID)
	if err != nil {
		return AssignmentDatesReport{}, err
	}
	if a.ID != assignmentID || a.CourseID != course.ID {
		return AssignmentDatesReport{}, fmt.Errorf("assignment is outside the configured course")
	}
	store, err := workspace.OpenStudents(s.Root)
	if err != nil {
		return AssignmentDatesReport{}, err
	}
	doc := domain.AssignmentDatesDocument{Schema: 1, Kind: "assignment-dates", Operation: "update", Course: course.Nickname, Instance: s.Client.Origin(), CourseID: course.ID, AssignmentID: a.ID, OnlyVisibleToOverrides: a.OnlyVisibleToOverrides, ObservedOnlyVisibleToOverrides: a.OnlyVisibleToOverrides, Rows: []domain.AssignmentOverrideRow{}}
	partial := false
	if a.AllDates != nil {
		for _, effective := range *a.AllDates {
			row := domain.AssignmentEffectiveDate{OverrideID: effective.ID, Base: effective.Base, GroupID: effective.GroupID, CourseSectionID: effective.CourseSectionID, DueAt: effective.DueAt, UnlockAt: effective.UnlockAt, LockAt: effective.LockAt}
			userID := effective.UserID
			if userID == 0 {
				userID = effective.StudentID
			}
			if userID > 0 {
				for _, student := range store.Rows {
					if student.Instance == s.Client.Origin() && student.UserID == userID {
						row.StudentRef = student.Ref
						break
					}
				}
				if row.StudentRef == "" {
					partial = true
					continue
				}
			}
			doc.EffectiveDates = append(doc.EffectiveDates, row)
		}
	}
	for _, live := range a.Overrides {
		row := domain.AssignmentOverrideRow{Key: fmt.Sprintf("override-%d", live.ID), Operation: "update", OverrideID: live.ID, Title: live.Title, GroupID: live.GroupID, CourseSectionID: live.CourseSectionID, Observed: &domain.AssignmentOverrideBaseline{Title: live.Title, GroupID: live.GroupID, CourseSectionID: live.CourseSectionID, DueAt: live.DueAt, UnlockAt: live.UnlockAt, LockAt: live.LockAt}}
		if len(live.StudentIDs) > 0 {
			if strings.TrimSpace(live.Title) == "" {
				partial = true
				continue
			}
			unmapped := false
			for _, id := range live.StudentIDs {
				ref := ""
				for _, student := range store.Rows {
					if student.Instance == s.Client.Origin() && student.UserID == id {
						ref = student.Ref
						break
					}
				}
				if ref == "" {
					partial = true
					unmapped = true
					break
				}
				row.StudentRefs = append(row.StudentRefs, ref)
			}
			if unmapped {
				continue
			}
			sort.Strings(row.StudentRefs)
			row.Observed.StudentRefs = append([]string(nil), row.StudentRefs...)
		}
		targets := 0
		if row.GroupID > 0 {
			targets++
		}
		if len(row.StudentRefs) > 0 {
			targets++
		}
		if row.CourseSectionID > 0 {
			targets++
		}
		if targets != 1 {
			partial = true
			continue
		}
		doc.Rows = append(doc.Rows, row)
	}
	sort.Slice(doc.Rows, func(i, j int) bool { return doc.Rows[i].OverrideID < doc.Rows[j].OverrideID })
	name := fmt.Sprintf("assignment-dates-%d-%s.md", a.ID, time.Now().UTC().Format("20060102T150405.000000000"))
	path, err := workspace.ConstrainPath(s.Root, filepath.Join("courses", course.Nickname, "pulled", name))
	if err != nil {
		return AssignmentDatesReport{}, err
	}
	if err = workspace.WriteDocument(path, doc, ""); err != nil {
		return AssignmentDatesReport{}, err
	}
	report := AssignmentDatesReport{Path: path, Status: "completed", Rows: doc.Rows}
	if partial {
		report.Status = "partial"
		report.Message = "some Canvas overrides were not included because their student target could not be mapped through existing private student refs or their target was not an exact supported scope"
	}
	return report, nil
}

func (s *Service) PushAssignmentDates(ctx context.Context, path string) (report AssignmentDatesReport, err error) {
	report.Path = path
	var doc domain.AssignmentDatesDocument
	if _, err = workspace.ReadDocument(path, &doc); err != nil {
		return report, err
	}
	if err = domain.ValidateAssignmentDates(doc); err != nil {
		return report, err
	}
	course, err := s.Course(doc.Course)
	if err != nil {
		return report, err
	}
	if doc.Instance != s.Client.Origin() || doc.CourseID != course.ID {
		return report, fmt.Errorf("assignment-dates identity differs from configured Canvas/course")
	}
	assignment, err := GetAssignment(ctx, s.Client, course.ID, doc.AssignmentID)
	if err != nil {
		return report, err
	}
	if assignment.ID != doc.AssignmentID || assignment.CourseID != course.ID {
		return report, fmt.Errorf("assignment-dates target is outside the configured course")
	}
	students, err := workspace.OpenStudents(s.Root)
	if err != nil {
		return report, err
	}
	save := func() error { return workspace.WriteDocument(path, &doc, "") }
	if doc.Rows == nil {
		return report, fmt.Errorf("assignment-dates overrides rows are required")
	}
	for i := range doc.Rows {
		row := &doc.Rows[i]
		if row.Outcome.Status == "succeeded" || row.Outcome.Status == "preserved" {
			continue
		}
		if row.Operation == "update" && !row.DueAt.Set && !row.UnlockAt.Set && !row.LockAt.Set {
			row.Outcome = domain.ActionOutcome{Status: "preserved", Message: "no date fields were changed"}
			if err = save(); err != nil {
				return report, err
			}
			continue
		}
		if row.Outcome.Status == "pending" || row.Outcome.Status == "uncertain" {
			row.Outcome = domain.ActionOutcome{Status: "held", Message: "previous write outcome is uncertain; do not replay this override row; inspect the exact override in Canvas and pull a fresh assignment-dates baseline"}
			_ = save()
			continue
		}
		userIDs, err := resolveOverrideTarget(ctx, s, course.ID, assignment, students, *row)
		if err != nil {
			row.Outcome = domain.ActionOutcome{Status: "held", Message: err.Error()}
			_ = save()
			continue
		}
		var live AssignmentOverrideLive
		if row.Operation != "create" {
			for _, candidate := range assignment.Overrides {
				if candidate.ID == row.OverrideID {
					live = candidate
					break
				}
			}
			if live.ID != row.OverrideID {
				row.Outcome = domain.ActionOutcome{Status: "held", Message: "exact override ID is no longer present; pull a fresh assignment-dates baseline"}
				_ = save()
				continue
			}
			if !overrideMatchesBaseline(live, *row.Observed, students, s.Client.Origin()) {
				row.Outcome = domain.ActionOutcome{Status: "held", Message: "live override target or dates changed after this baseline; your requested edit was preserved"}
				_ = save()
				continue
			}
		}
		if row.Outcome.Status == "pending" {
			row.Outcome = domain.ActionOutcome{Status: "held", Message: "previous override write is uncertain; inspect Canvas before retrying"}
			_ = save()
			continue
		}
		row.Outcome = domain.ActionOutcome{Status: "pending", Message: "override write may begin after intent is saved"}
		if err = save(); err != nil {
			return report, err
		}
		fields := map[string]any{}
		if row.Operation == "create" || row.Operation == "update" {
			if row.Operation == "create" {
				if row.Title != "" {
					fields["title"] = row.Title
				}
				if row.GroupID > 0 {
					fields["group_id"] = row.GroupID
				}
				if len(userIDs) > 0 {
					fields["student_ids"] = userIDs
				}
				if row.CourseSectionID > 0 {
					fields["course_section_id"] = row.CourseSectionID
				}
			}
			putOverrideDate(fields, "due_at", row.DueAt)
			putOverrideDate(fields, "unlock_at", row.UnlockAt)
			putOverrideDate(fields, "lock_at", row.LockAt)
		}
		url := fmt.Sprintf("/api/v1/courses/%d/assignments/%d/overrides", course.ID, assignment.ID)
		method := http.MethodPost
		var response struct {
			ID       int64                  `json:"id"`
			Override AssignmentOverrideLive `json:"assignment_override"`
		}
		if row.Operation == "update" {
			method = http.MethodPut
			url += "/" + strconv.FormatInt(row.OverrideID, 10)
		}
		if row.Operation == "remove" {
			method = http.MethodDelete
			url += "/" + strconv.FormatInt(row.OverrideID, 10)
		}
		body := any(nil)
		if row.Operation != "remove" {
			body = map[string]any{"assignment_override": fields}
		}
		if err = s.Client.WriteAccepted(ctx, method, url, body, &response); err != nil {
			row.Outcome = failedOutcome(err)
		} else {
			id := row.OverrideID
			if row.Operation == "create" {
				id = response.ID
				if response.Override.ID > 0 {
					id = response.Override.ID
				}
			}
			if row.Operation == "remove" {
				row.Outcome = domain.ActionOutcome{Status: "succeeded", ID: row.OverrideID}
			} else {
				row.OverrideID = id
				row.Outcome = domain.ActionOutcome{Status: "succeeded", ID: id}
			}
		}
		if err = save(); err != nil {
			return report, fmt.Errorf("Canvas accepted override write but its result could not be saved: %w", err)
		}
		if row.Outcome.Status != "succeeded" {
			report.Message = "one or more override rows need attention"
		}
	}
	if doc.OnlyVisibleToOverrides != nil {
		if assignment.OnlyVisibleToOverrides == nil || *assignment.OnlyVisibleToOverrides != *doc.ObservedOnlyVisibleToOverrides {
			report.Message = appendMessage(report.Message, "only_visible_to_overrides changed since the baseline; visibility update held")
		} else {
			allRowsComplete := true
			for _, row := range doc.Rows {
				if row.Outcome.Status != "succeeded" && row.Outcome.Status != "preserved" {
					allRowsComplete = false
				}
			}
			if *doc.OnlyVisibleToOverrides && !allRowsComplete {
				report.Message = appendMessage(report.Message, "visibility restriction was not enabled because one or more requested override rows did not complete")
			} else if *doc.OnlyVisibleToOverrides != *assignment.OnlyVisibleToOverrides {
				var response Assignment
				err = s.Client.WriteAccepted(ctx, http.MethodPut, fmt.Sprintf("/api/v1/courses/%d/assignments/%d", course.ID, assignment.ID), map[string]any{"assignment": map[string]any{"only_visible_to_overrides": *doc.OnlyVisibleToOverrides}}, &response)
				if err != nil {
					report.Message = appendMessage(report.Message, "visibility update failed: "+err.Error())
				} else {
					assignment.OnlyVisibleToOverrides = doc.OnlyVisibleToOverrides
				}
			}
		}
	}
	report.Rows = doc.Rows
	complete := true
	for _, row := range doc.Rows {
		if row.Outcome.Status != "succeeded" && row.Outcome.Status != "preserved" {
			complete = false
		}
	}
	if doc.OnlyVisibleToOverrides != nil && assignment.OnlyVisibleToOverrides != nil && *assignment.OnlyVisibleToOverrides != *doc.OnlyVisibleToOverrides {
		complete = false
	}
	report.Status = "completed"
	if !complete {
		report.Status = "partial"
	}
	if err = save(); err != nil {
		return report, err
	}
	return report, nil
}

// deliverCreateOverrides is the create-only audience stage. It runs after the
// rubric/module stages and before publication; accepted rows are never replayed.
func (s *Service) deliverCreateOverrides(ctx context.Context, courseID int64, assignment Assignment, doc *domain.AssignmentDocument, save func() error) (bool, error) {
	if len(doc.Overrides) == 0 && doc.OnlyVisibleToOverrides == nil {
		return true, nil
	}
	c := doc.Created
	if c == nil {
		return false, fmt.Errorf("assignment creation metadata is missing")
	}
	if len(c.Overrides) == 0 && len(doc.Overrides) > 0 {
		for _, row := range doc.Overrides {
			c.Overrides = append(c.Overrides, domain.AssignmentOverrideOutcome{Key: row.Key})
		}
		if err := save(); err != nil {
			return false, err
		}
	} else if len(c.Overrides) != len(doc.Overrides) {
		return false, fmt.Errorf("saved override intent differs from the create document")
	}
	store, err := workspace.OpenStudents(s.Root)
	if err != nil {
		return false, err
	}
	for i, row := range doc.Overrides {
		outcome := &c.Overrides[i]
		if outcome.Key != row.Key {
			return false, fmt.Errorf("saved override intent differs from the create document")
		}
		if definite(outcome.Outcome) {
			continue
		}
		if outcome.Outcome.Status == "pending" || outcome.Outcome.Status == "uncertain" {
			outcome.Outcome = domain.ActionOutcome{Status: "held", Message: "previous override create may have succeeded; inspect Canvas and do not replay"}
			_ = save()
			return false, nil
		}
		userIDs, resolveErr := resolveOverrideTarget(ctx, s, courseID, assignment, store, row)
		if resolveErr != nil {
			outcome.Outcome = domain.ActionOutcome{Status: "held", Message: resolveErr.Error()}
			_ = save()
			return false, nil
		}
		fields := map[string]any{}
		if row.Title != "" {
			fields["title"] = row.Title
		}
		if row.GroupID > 0 {
			fields["group_id"] = row.GroupID
		}
		if len(userIDs) > 0 {
			fields["student_ids"] = userIDs
		}
		if row.CourseSectionID > 0 {
			fields["course_section_id"] = row.CourseSectionID
		}
		putOverrideDate(fields, "due_at", row.DueAt)
		putOverrideDate(fields, "unlock_at", row.UnlockAt)
		putOverrideDate(fields, "lock_at", row.LockAt)
		outcome.Outcome = domain.ActionOutcome{Status: "pending"}
		if err = save(); err != nil {
			return false, err
		}
		var response struct {
			ID       int64                  `json:"id"`
			Override AssignmentOverrideLive `json:"assignment_override"`
		}
		err = s.Client.WriteAccepted(ctx, http.MethodPost, fmt.Sprintf("/api/v1/courses/%d/assignments/%d/overrides", courseID, assignment.ID), map[string]any{"assignment_override": fields}, &response)
		if err != nil {
			outcome.Outcome = failedOutcome(err)
		} else {
			outcome.OverrideID = response.ID
			if response.Override.ID > 0 {
				outcome.OverrideID = response.Override.ID
			}
			outcome.Outcome = domain.ActionOutcome{Status: "succeeded", ID: outcome.OverrideID}
		}
		if err = save(); err != nil {
			return false, fmt.Errorf("Canvas accepted assignment override but its outcome could not be saved: %w", err)
		}
		if !definite(outcome.Outcome) {
			return false, nil
		}
	}
	if doc.OnlyVisibleToOverrides != nil {
		if !definite(c.Visibility) {
			if c.Visibility.Status == "pending" || c.Visibility.Status == "uncertain" {
				c.Visibility = domain.ActionOutcome{Status: "held", Message: "previous visibility write is uncertain; inspect Canvas and do not replay"}
				_ = save()
				return false, nil
			}
			c.Visibility = domain.ActionOutcome{Status: "pending"}
			if err = save(); err != nil {
				return false, err
			}
			var response Assignment
			err = s.Client.WriteAccepted(ctx, http.MethodPut, fmt.Sprintf("/api/v1/courses/%d/assignments/%d", courseID, assignment.ID), map[string]any{"assignment": map[string]any{"only_visible_to_overrides": *doc.OnlyVisibleToOverrides}}, &response)
			if err != nil {
				c.Visibility = failedOutcome(err)
			} else {
				c.Visibility = domain.ActionOutcome{Status: "succeeded"}
			}
			if err = save(); err != nil {
				return false, err
			}
		}
		if !definite(c.Visibility) {
			return false, nil
		}
	}
	return true, nil
}

func createOverridesComplete(doc domain.AssignmentDocument, created *domain.CreatedResources) bool {
	if len(doc.Overrides) == 0 && doc.OnlyVisibleToOverrides == nil {
		return true
	}
	if created == nil || len(created.Overrides) != len(doc.Overrides) {
		return false
	}
	for i, row := range doc.Overrides {
		if created.Overrides[i].Key != row.Key || !definite(created.Overrides[i].Outcome) {
			return false
		}
	}
	return doc.OnlyVisibleToOverrides == nil || definite(created.Visibility)
}

func resolveOverrideTarget(ctx context.Context, s *Service, courseID int64, assignment Assignment, store *workspace.StudentStore, row domain.AssignmentOverrideRow) ([]int64, error) {
	if row.GroupID > 0 {
		if assignment.GroupCategoryID == nil || *assignment.GroupCategoryID <= 0 {
			return nil, fmt.Errorf("group override requires a group assignment")
		}
		var group canvasGroup
		if err := s.Client.Get(ctx, fmt.Sprintf("/api/v1/groups/%d", row.GroupID), &group); err != nil {
			return nil, err
		}
		if group.ID != row.GroupID || group.ContextType != "Course" || group.CourseID != courseID || group.GroupCategoryID != *assignment.GroupCategoryID || nonCollaborative(group.NonCollaborative) {
			return nil, fmt.Errorf("group override must target a collaborative group in this assignment's selected set")
		}
		return nil, nil
	}
	if row.CourseSectionID > 0 {
		var section struct {
			ID       int64 `json:"id"`
			CourseID int64 `json:"course_id"`
		}
		if err := s.Client.Get(ctx, fmt.Sprintf("/api/v1/courses/%d/sections/%d", courseID, row.CourseSectionID), &section); err != nil {
			return nil, err
		}
		if section.ID != row.CourseSectionID || section.CourseID != courseID {
			return nil, fmt.Errorf("section override target is outside the configured course")
		}
		return nil, nil
	}
	ids := make([]int64, 0, len(row.StudentRefs))
	seen := map[string]bool{}
	for _, ref := range row.StudentRefs {
		if seen[ref] {
			return nil, fmt.Errorf("student_refs contains a duplicate ref")
		}
		seen[ref] = true
		st, err := store.Resolve(s.Client.Origin(), ref)
		if err != nil {
			return nil, err
		}
		ids = append(ids, st.UserID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids, nil
}

func overrideMatchesBaseline(live AssignmentOverrideLive, base domain.AssignmentOverrideBaseline, store *workspace.StudentStore, origin string) bool {
	if live.GroupID != base.GroupID || live.CourseSectionID != base.CourseSectionID || live.Title != base.Title || !sameDate(live.DueAt, base.DueAt) || !sameDate(live.UnlockAt, base.UnlockAt) || !sameDate(live.LockAt, base.LockAt) {
		return false
	}
	ids := []int64{}
	for _, ref := range base.StudentRefs {
		st, err := store.Resolve(origin, ref)
		if err != nil {
			return false
		}
		ids = append(ids, st.UserID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	got := append([]int64(nil), live.StudentIDs...)
	sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })
	if len(ids) != len(got) {
		return false
	}
	for i := range ids {
		if ids[i] != got[i] {
			return false
		}
	}
	return true
}

func putOverrideDate(fields map[string]any, key string, date domain.OptionalDate) {
	if !date.Set {
		return
	}
	if date.Null {
		fields[key] = nil
	} else {
		fields[key] = date.Value
	}
}
func appendMessage(a, b string) string {
	if a == "" {
		return b
	}
	return a + "; " + b
}
