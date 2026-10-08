package workflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/render"
	"canvasexpertse/internal/workspace"
)

type PushReport struct {
	Path, URL, Status, Message string
	AssignmentID               int64
	Requests                   int64
	Elapsed                    time.Duration
	Resources                  *domain.CreatedResources
	UpdateOutcome              domain.ActionOutcome
}

func (s *Service) Push(ctx context.Context, path string) (report PushReport, err error) {
	start := time.Now()
	requests := s.Client.Requests()
	report.Path = path
	defer func() { report.Elapsed = time.Since(start); report.Requests = s.Client.Requests() - requests }()
	var doc domain.AssignmentDocument
	body, err := workspace.ReadDocument(path, &doc)
	if err != nil {
		return report, err
	}
	defer func() {
		report.Resources = doc.Created
		report.UpdateOutcome = doc.UpdateOutcome
		if doc.Created != nil {
			report.AssignmentID = doc.Created.AssignmentID
			report.URL = doc.Created.URL
			if err != nil && doc.Created.AssignmentID > 0 && report.Status == "" {
				report.Status = "partial"
			}
		}
	}()
	course, err := s.Course(doc.Course)
	if err != nil {
		return report, err
	}
	doc.Course = course.Nickname
	if doc.Operation == "attach-rubric" {
		if doc.Instance != s.Client.Origin() || doc.CourseID != course.ID {
			return report, fmt.Errorf("rubric attachment identity differs from configured Canvas/course")
		}
		return s.pushRubricAttachment(ctx, path, &doc, body, report)
	}
	if doc.Operation == "update" {
		if doc.Instance != s.Client.Origin() || doc.CourseID != course.ID {
			return report, fmt.Errorf("update identity differs from configured Canvas/course")
		}
		return s.pushUpdate(ctx, path, &doc, body, report)
	}
	if (doc.Instance != "" && doc.Instance != s.Client.Origin()) || (doc.CourseID != 0 && doc.CourseID != course.ID) {
		return report, fmt.Errorf("create identity differs from configured Canvas instance/course")
	}
	if doc.Created != nil && (doc.Created.AssignmentID > 0 || ambiguous(doc.Created.Assignment)) && (doc.Instance == "" || doc.CourseID == 0) {
		return report, fmt.Errorf("incomplete create lacks its original Canvas instance/course; establish its exact identity before resuming")
	}
	doc.Instance = s.Client.Origin()
	doc.CourseID = course.ID
	if doc.Created != nil && doc.Created.Complete {
		report.Status = "completed"
		report.AssignmentID = doc.Created.AssignmentID
		report.URL = doc.Created.URL
		report.Message = "already created; use pull-item for future edits"
		report.Path, err = s.movePushed(path, doc.Course, doc.Title, doc.Created.AssignmentID)
		return report, err
	}
	resumingModule := doc.Created != nil && definite(doc.Created.Module) && doc.ModuleID > 0
	if err = s.require(s.Evidence.Authoring, "assignment authoring"); err != nil {
		return report, err
	}
	if doc.Rubric != nil {
		if err = s.require(s.Evidence.IndependentRubricCreate, "independent rubric attachment preserving assignment points"); err != nil {
			return report, err
		}
	}
	html, err := render.MarkdownHTML(body)
	if err != nil {
		return report, err
	}
	if doc.Created == nil {
		doc.Created = &domain.CreatedResources{StartedAt: time.Now().UTC().Format(time.RFC3339Nano), IntendedHTML: html}
	}
	c := doc.Created
	if c.IntendedHTML != html {
		return report, fmt.Errorf("incomplete create body changed; recover known creation before editing live content")
	}
	save := func() error { return workspace.WriteDocument(path, &doc, body) }
	if err = save(); err != nil {
		return report, err
	}
	if c.AssignmentID == 0 {
		if c.Assignment.Status == "uncertain" || c.Assignment.Status == "pending" {
			a, recoveryErr := s.recoverCreate(ctx, course.ID, doc, html)
			if recoveryErr != nil {
				report.Status = "held"
				report.Message = recoveryErr.Error()
				return report, nil
			}
			c.AssignmentID = a.ID
			c.URL = a.HTMLURL
			c.Assignment = domain.ActionOutcome{Status: "succeeded", ID: a.ID, Message: "uniquely matched live create after uncertain response"}
		} else {
			c.StartedAt = time.Now().UTC().Format(time.RFC3339Nano)
			c.Assignment = domain.ActionOutcome{Status: "pending", Message: "creation may begin after metadata persistence"}
			if err = save(); err != nil {
				return report, err
			}
			fields := createFields(doc, html)
			var a Assignment
			err = s.Client.Write(ctx, "POST", fmt.Sprintf("/api/v1/courses/%d/assignments", course.ID), map[string]any{"assignment": fields}, &a)
			if err != nil {
				c.Assignment = failedOutcome(err)
				if e := save(); e != nil {
					return report, e
				}
				report.Status = c.Assignment.Status
				report.Message = err.Error()
				return report, nil
			}
			if a.ID <= 0 {
				c.Assignment = domain.ActionOutcome{Status: "uncertain", Message: "create response lacks assignment ID"}
				if err = save(); err != nil {
					return report, err
				}
				report.Status = "uncertain"
				return report, nil
			}
			c.AssignmentID = a.ID
			c.URL = a.HTMLURL
			c.Assignment = domain.ActionOutcome{Status: "succeeded", ID: a.ID}
		}
		if err = save(); err != nil {
			return report, fmt.Errorf("Canvas assignment exists but local metadata could not be saved: %w", err)
		}
	}
	report.AssignmentID = c.AssignmentID
	report.URL = c.URL
	if resumingModule {
		var item struct {
			ID        int64  `json:"id"`
			Type      string `json:"type"`
			ContentID int64  `json:"content_id"`
		}
		e := s.Client.Get(ctx, fmt.Sprintf("/api/v1/courses/%d/modules/%d/items/%d", course.ID, doc.ModuleID, c.ModuleItemID), &item)
		if e != nil || item.ID != c.ModuleItemID || item.Type != "Assignment" || item.ContentID != c.AssignmentID {
			report.Status = "held"
			report.Message = "known module placement changed or was removed; incomplete draft will not recreate it automatically"
			return report, nil
		}
	}
	// Resume known resources only if their relevant live content still agrees.
	live, err := GetAssignment(ctx, s.Client, course.ID, c.AssignmentID)
	if err != nil {
		return report, err
	}
	if !sameCreateCore(live, doc, html) {
		report.Status = "held"
		report.Message = "live assignment differs from incomplete draft; pull-item and preserve the live edits"
		return report, nil
	}
	if live.Published && ((doc.Rubric != nil && !definite(c.Rubric)) || (doc.ModuleID > 0 && !definite(c.Module))) && !(ambiguous(c.Publication) && doc.Published) {
		report.Status = "held"
		report.Message = "teacher published incomplete assignment; recover its construction state before attaching required pieces"
		return report, nil
	}
	if doc.Rubric != nil && !definite(c.Rubric) {
		if len(live.Rubric) > 0 && !ambiguous(c.Rubric) {
			report.Status = "held"
			report.Message = "live assignment already has a rubric; preserve its existing association and pull-item before further edits"
			return report, nil
		}
		if c.Rubric.Status == "uncertain" || c.Rubric.Status == "pending" {
			snap, e := Snapshot(live, s.Client.Origin())
			if e != nil {
				return report, e
			}
			if snap.Rubric == nil || !sameRubricDefinition(*snap.Rubric, *doc.Rubric) || !snap.Rubric.UseForGradingKnown || snap.Rubric.UseForGrading {
				report.Status = "held"
				report.Message = "rubric creation completion remains uncertain; inspect exact assignment association before retrying"
				return report, nil
			}
			c.RubricID = snap.Rubric.ID
			c.RubricAssociationID = snap.Rubric.AssociationID
			c.Rubric = domain.ActionOutcome{Status: "succeeded", ID: c.RubricID}
		} else {
			c.Rubric = domain.ActionOutcome{Status: "pending"}
			if err = save(); err != nil {
				return report, err
			}
			var response struct {
				Rubric struct {
					ID int64 `json:"id"`
				} `json:"rubric"`
				Association struct {
					ID int64 `json:"id"`
				} `json:"rubric_association"`
			}
			err = s.Client.Write(ctx, "POST", fmt.Sprintf("/api/v1/courses/%d/rubrics", course.ID), rubricCreateFields(*doc.Rubric, c.AssignmentID), &response)
			if err != nil {
				c.Rubric = failedOutcome(err)
			} else if response.Rubric.ID <= 0 || response.Association.ID <= 0 {
				c.Rubric = domain.ActionOutcome{Status: "uncertain", Message: "rubric response lacks exact resource IDs"}
			} else {
				c.RubricID = response.Rubric.ID
				c.RubricAssociationID = response.Association.ID
				c.Rubric = domain.ActionOutcome{Status: "succeeded", ID: c.RubricID}
			}
		}
		if err = save(); err != nil {
			return report, fmt.Errorf("rubric outcome could not be persisted: %w", err)
		}
		if !definite(c.Rubric) {
			report.Status = c.Rubric.Status
			report.Message = c.Rubric.Message
			return report, nil
		}
		// Rubric attachment is the one author action known to potentially alter
		// assignment points; check that scale before making the item public.
		live, err = GetAssignment(ctx, s.Client, course.ID, c.AssignmentID)
		if err != nil {
			return report, err
		}
		if !sameCreateCore(live, doc, html) {
			report.Status = "held"
			report.Message = "rubric attachment changed assignment fields or points; item remains unpublished"
			return report, nil
		}
	} else if doc.Rubric == nil {
		c.Rubric = domain.ActionOutcome{Status: "not-requested"}
	}
	if doc.Rubric != nil && definite(c.Rubric) {
		snap, e := Snapshot(live, s.Client.Origin())
		if e != nil {
			return report, e
		}
		if snap.Rubric == nil || !sameRubricDefinition(*snap.Rubric, *doc.Rubric) || !snap.Rubric.UseForGradingKnown || snap.Rubric.UseForGrading {
			report.Status = "held"
			report.Message = "live attached rubric differs from requested independent definition; construction held before publication"
			return report, nil
		}
	}
	if doc.ModuleID > 0 && !definite(c.Module) {
		if c.Module.Status == "uncertain" || c.Module.Status == "pending" {
			var items []struct {
				ID        int64  `json:"id"`
				Type      string `json:"type"`
				ContentID int64  `json:"content_id"`
			}
			err = s.Client.List(ctx, fmt.Sprintf("/api/v1/courses/%d/modules/%d/items?per_page=100", course.ID, doc.ModuleID), &items)
			if err != nil {
				return report, err
			}
			matches := []int64{}
			for _, item := range items {
				if item.Type == "Assignment" && item.ContentID == c.AssignmentID {
					matches = append(matches, item.ID)
				}
			}
			if len(matches) != 1 {
				report.Status = "held"
				report.Message = "module placement completion remains uncertain; select the exact live module item"
				return report, nil
			}
			c.ModuleItemID = matches[0]
			c.Module = domain.ActionOutcome{Status: "succeeded", ID: matches[0]}
		} else {
			c.Module = domain.ActionOutcome{Status: "pending"}
			if err = save(); err != nil {
				return report, err
			}
			var item struct {
				ID int64 `json:"id"`
			}
			err = s.Client.Write(ctx, "POST", fmt.Sprintf("/api/v1/courses/%d/modules/%d/items", course.ID, doc.ModuleID), map[string]any{"module_item": map[string]any{"type": "Assignment", "content_id": c.AssignmentID}}, &item)
			if err != nil {
				c.Module = failedOutcome(err)
			} else if item.ID <= 0 {
				c.Module = domain.ActionOutcome{Status: "uncertain", Message: "module response lacks item ID"}
			} else {
				c.ModuleItemID = item.ID
				c.Module = domain.ActionOutcome{Status: "succeeded", ID: item.ID}
			}
		}
		if err = save(); err != nil {
			return report, err
		}
		if !definite(c.Module) {
			report.Status = c.Module.Status
			report.Message = c.Module.Message
			return report, nil
		}
	} else if doc.ModuleID == 0 {
		c.Module = domain.ActionOutcome{Status: "not-requested"}
	}
	if !definite(c.Publication) {
		if live.Published == doc.Published {
			c.Publication = domain.ActionOutcome{Status: "preserved"}
		} else if ambiguous(c.Publication) {
			report.Status = "held"
			report.Message = "publication completion remains uncertain; differing live state does not authorize replay over a teacher change"
			return report, nil
		} else {
			c.Publication = domain.ActionOutcome{Status: "pending"}
			if err = save(); err != nil {
				return report, err
			}
			var published Assignment
			err = s.Client.Write(ctx, "PUT", fmt.Sprintf("/api/v1/courses/%d/assignments/%d", course.ID, c.AssignmentID), map[string]any{"assignment": map[string]any{"published": doc.Published}}, &published)
			if err != nil {
				c.Publication = failedOutcome(err)
			} else if published.ID != c.AssignmentID || published.Published != doc.Published {
				c.Publication = domain.ActionOutcome{Status: "uncertain", Message: "publication response did not confirm requested state"}
			} else {
				c.Publication = domain.ActionOutcome{Status: "succeeded"}
			}
		}
	}
	c.Complete = definite(c.Assignment) && definite(c.Rubric) && definite(c.Module) && definite(c.Publication)
	if err = save(); err != nil {
		return report, err
	}
	report.Status = "partial"
	if c.Complete {
		report.Status = "completed"
		report.Path, err = s.movePushed(path, doc.Course, doc.Title, c.AssignmentID)
		if err != nil {
			return report, fmt.Errorf("create completed; pushed record move failed: %w", err)
		}
	} else {
		report.Message = c.Publication.Message
	}
	return report, nil
}
func (s *Service) movePushed(path, course, title string, id int64) (string, error) {
	dir, e := workspace.ConstrainPath(s.Root, filepath.Join("courses", course, "pushed"))
	if e != nil {
		return path, e
	}
	absolute, e := filepath.Abs(path)
	if e != nil {
		return path, e
	}
	if strings.EqualFold(filepath.Dir(absolute), dir) {
		return path, nil
	}
	dest, e := workspace.ConstrainPath(s.Root, filepath.Join("courses", course, "pushed", fmt.Sprintf("%d-%s-%s.md", id, Slug(title), time.Now().UTC().Format("20060102T150405.000000000"))))
	if e != nil {
		return path, e
	}
	if e = os.MkdirAll(dir, 0755); e != nil {
		return path, e
	}
	if _, e = os.Stat(dest); !os.IsNotExist(e) {
		return path, fmt.Errorf("pushed record target already exists or cannot be checked")
	}
	if e = os.Rename(absolute, dest); e != nil {
		return path, e
	}
	return dest, nil
}

func createFields(d domain.AssignmentDocument, html string) map[string]any {
	return map[string]any{"name": d.Title, "description": html, "points_possible": d.Points, "grading_type": "points", "submission_types": d.SubmissionTypes, "due_at": d.DueAt, "unlock_at": d.UnlockAt, "lock_at": d.LockAt, "published": false}
}
func sameCreateCore(a Assignment, d domain.AssignmentDocument, html string) bool {
	p, e := domain.ParseDecimal(a.Points.String())
	return e == nil && p.Cmp(d.Points) == 0 && a.Name == d.Title && a.Description == html && a.GradingType == "points" && !a.Anonymous && !a.Moderated && (a.GroupCategoryID == nil || *a.GroupCategoryID == 0) && reflect.DeepEqual(a.SubmissionTypes, d.SubmissionTypes) && sameDate(a.DueAt, d.DueAt) && sameDate(a.UnlockAt, d.UnlockAt) && sameDate(a.LockAt, d.LockAt)
}
func sameDate(a, b *string) bool {
	if a == nil || *a == "" {
		return b == nil || *b == ""
	}
	if b == nil || *b == "" {
		return false
	}
	ta, e1 := time.Parse(time.RFC3339Nano, *a)
	tb, e2 := time.Parse(time.RFC3339Nano, *b)
	return e1 == nil && e2 == nil && ta.Equal(tb)
}
func sameRubricDefinition(a, b domain.Rubric) bool {
	a.ID = 0
	b.ID = 0
	a.AssociationID = 0
	b.AssociationID = 0
	a.UseForGrading = false
	b.UseForGrading = false
	a.UseForGradingKnown = false
	b.UseForGradingKnown = false
	if a.Title != b.Title || a.FreeFormCriterionComments != b.FreeFormCriterionComments || len(a.Criteria) != len(b.Criteria) {
		return false
	}
	for i, c := range a.Criteria {
		d := b.Criteria[i]
		if c.Description != d.Description || c.LongDescription != d.LongDescription || c.UseRange != d.UseRange || c.Points.Cmp(d.Points) != 0 || len(c.Ratings) != len(d.Ratings) {
			return false
		}
		used := map[int]bool{}
		for _, rating := range c.Ratings {
			found := false
			for j, other := range d.Ratings {
				if !used[j] && rating.Points.Cmp(other.Points) == 0 && rating.Description == other.Description && rating.LongDescription == other.LongDescription {
					used[j] = true
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}
	return true
}
func rubricCreateFields(r domain.Rubric, assignmentID int64) map[string]any {
	criteria := map[string]any{}
	for i, c := range r.Criteria {
		ratings := map[string]any{}
		for j, v := range c.Ratings {
			ratings[strconv.Itoa(j)] = map[string]any{"points": v.Points, "description": v.Description, "long_description": v.LongDescription}
		}
		criteria[strconv.Itoa(i)] = map[string]any{"points": c.Points, "description": c.Description, "long_description": c.LongDescription, "ratings": ratings, "criterion_use_range": c.UseRange}
	}
	return map[string]any{"rubric": map[string]any{"title": r.Title, "free_form_criterion_comments": r.FreeFormCriterionComments, "skip_updating_points_possible": true, "criteria": criteria}, "rubric_association": map[string]any{"association_id": assignmentID, "association_type": "Assignment", "use_for_grading": false, "purpose": "grading"}}
}
func (s *Service) recoverCreate(ctx context.Context, courseID int64, d domain.AssignmentDocument, html string) (Assignment, error) {
	started, e := time.Parse(time.RFC3339Nano, d.Created.StartedAt)
	if e != nil {
		return Assignment{}, fmt.Errorf("uncertain create lacks a reliable start time; select the exact live assignment")
	}
	// Empty lookups never authorize another create. A later invocation can retry
	// this bounded read after Canvas has made the object visible.
	all, e := Assignments(ctx, s.Client, courseID)
	if e != nil {
		return Assignment{}, e
	}
	matches := []Assignment{}
	for _, a := range all {
		created, e := time.Parse(time.RFC3339Nano, a.CreatedAt)
		if e == nil && !created.Before(started.Add(-2*time.Second)) && !created.After(started.Add(2*time.Minute)) && sameCreateCore(a, d, html) && !a.Published {
			matches = append(matches, a)
		}
	}
	if len(matches) != 1 {
		return Assignment{}, fmt.Errorf("creation completion uncertain: %d plausible recent matches; do not retry create blindly", len(matches))
	}
	return matches[0], nil
}

func (s *Service) pushUpdate(ctx context.Context, path string, d *domain.AssignmentDocument, body string, report PushReport) (PushReport, error) {
	report.AssignmentID = d.AssignmentID
	if definite(d.UpdateOutcome) {
		report.Status = "completed"
		report.Message = "update already completed; pull-item before another edit"
		return report, nil
	}
	patch, err := PlanEdit(*d, body)
	if err != nil {
		return report, err
	}
	a, err := GetAssignment(ctx, s.Client, d.CourseID, d.AssignmentID)
	if err != nil {
		return report, err
	}
	report.URL = a.HTMLURL
	live, err := Snapshot(a, s.Client.Origin())
	if err != nil {
		return report, err
	}
	base := *d.Observed
	base.UpdatedAt = ""
	compare := live
	compare.UpdatedAt = ""
	if !SameSnapshot(base, compare) {
		// If the exact intended patch already exists after an uncertain write, it
		// can be recognized only when all untouched fields still match baseline.
		intended := base
		intended.Title = d.Title
		intended.Points = d.Points
		intended.DescriptionHTML = body
		intended.SubmissionTypes = d.SubmissionTypes
		intended.DueAt = d.DueAt
		intended.UnlockAt = d.UnlockAt
		intended.LockAt = d.LockAt
		intended.Published = d.Published
		if (d.UpdateOutcome.Status == "pending" || d.UpdateOutcome.Status == "uncertain") && SameSnapshot(intended, compare) {
			d.UpdateOutcome = domain.ActionOutcome{Status: "succeeded", Message: "intended live state recovered"}
			if err = workspace.WriteDocument(path, d, body); err != nil {
				return report, err
			}
			report.Status = "completed"
			return report, nil
		}
		fresh, e := s.PullItem(ctx, d.Course, fmt.Sprint(d.AssignmentID))
		if e != nil {
			return report, e
		}
		report.Status = "held"
		report.Message = "live assignment changed; unsent edit preserved; fresh version: " + fresh
		return report, nil
	}
	if d.UpdateOutcome.Status == "pending" || d.UpdateOutcome.Status == "uncertain" {
		report.Status = "held"
		report.Message = "prior update completion is uncertain; unchanged lookup does not prove it failed"
		return report, nil
	}
	if len(patch) == 0 {
		d.UpdateOutcome = domain.ActionOutcome{Status: "preserved"}
		report.Status = "completed"
		return report, workspace.WriteDocument(path, d, body)
	}
	if err = s.require(s.Evidence.Authoring, "assignment update"); err != nil {
		return report, err
	}
	d.UpdateOutcome = domain.ActionOutcome{Status: "pending"}
	if err = workspace.WriteDocument(path, d, body); err != nil {
		return report, err
	}
	var response Assignment
	err = s.Client.Write(ctx, "PUT", fmt.Sprintf("/api/v1/courses/%d/assignments/%d", d.CourseID, d.AssignmentID), map[string]any{"assignment": patch}, &response)
	if err != nil {
		d.UpdateOutcome = failedOutcome(err)
	} else if response.ID != d.AssignmentID {
		d.UpdateOutcome = domain.ActionOutcome{Status: "uncertain", Message: "update response lacks intended assignment ID"}
	} else {
		if !editPatchMatches(response, *d, body, patch) {
			d.UpdateOutcome = domain.ActionOutcome{Status: "uncertain", Message: "update response did not confirm targeted changed fields"}
		} else {
			d.UpdateOutcome = domain.ActionOutcome{Status: "succeeded"}
		}
	}
	if e := workspace.WriteDocument(path, d, body); e != nil {
		return report, fmt.Errorf("Canvas update outcome could not be saved: %w", e)
	}
	report.Status = d.UpdateOutcome.Status
	report.Message = d.UpdateOutcome.Message
	return report, nil
}
func editPatchMatches(a Assignment, d domain.AssignmentDocument, html string, patch map[string]any) bool {
	for field := range patch {
		switch field {
		case "name":
			if a.Name != d.Title {
				return false
			}
		case "description":
			if a.Description != html {
				return false
			}
		case "points_possible":
			p, e := domain.ParseDecimal(a.Points.String())
			if e != nil || p.Cmp(d.Points) != 0 {
				return false
			}
		case "submission_types":
			if !reflect.DeepEqual(a.SubmissionTypes, d.SubmissionTypes) {
				return false
			}
		case "due_at":
			if !sameDate(a.DueAt, d.DueAt) {
				return false
			}
		case "unlock_at":
			if !sameDate(a.UnlockAt, d.UnlockAt) {
				return false
			}
		case "lock_at":
			if !sameDate(a.LockAt, d.LockAt) {
				return false
			}
		case "published":
			if a.Published != d.Published {
				return false
			}
		default:
			return false
		}
	}
	return true
}
