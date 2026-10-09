package workflow

import (
	"context"
	"fmt"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

// pushRubricAttachment adds a new independent rubric to a freshly observed
// assignment with no rubric. It never updates assignment fields or edits a
// rubric definition. Recovery uses this assignment's one live association.
func (s *Service) pushRubricAttachment(ctx context.Context, path string, d *domain.AssignmentDocument, body string, report PushReport) (PushReport, error) {
	report.AssignmentID = d.AssignmentID
	if d.Instance != s.Client.Origin() {
		return report, fmt.Errorf("rubric attachment targets another Canvas instance")
	}
	course, e := s.Course(d.Course)
	if e != nil {
		return report, e
	}
	if d.CourseID != course.ID {
		return report, fmt.Errorf("rubric attachment targets another configured course")
	}
	if d.Created != nil && d.Created.Complete {
		report.Status = "completed"
		report.URL = d.Created.URL
		report.Message = "rubric attachment already completed; pull-item before another operation"
		return report, nil
	}
	live, e := GetAssignment(ctx, s.Client, d.CourseID, d.AssignmentID)
	if e != nil {
		return report, e
	}
	if live.CourseID == 0 {
		live.CourseID = d.CourseID
	}
	report.URL = live.HTMLURL
	snapshot, e := Snapshot(live, s.Client.Origin())
	if e != nil {
		return report, e
	}
	baseline := *d.Observed
	baseline.UpdatedAt = ""
	compare := snapshot
	compare.UpdatedAt = ""
	compare.Rubric = nil
	if !SameGradingBasis(baseline, compare) {
		report.Status = "held"
		report.Message = "live assignment changed since pull; rubric attachment held and assignment preserved"
		return report, nil
	}
	save := func() error { return workspace.WriteDocument(path, d, body) }
	if d.Created == nil {
		d.Created = &domain.CreatedResources{AssignmentID: d.AssignmentID, URL: live.HTMLURL, Assignment: domain.ActionOutcome{Status: "preserved", ID: d.AssignmentID}, Publication: domain.ActionOutcome{Status: "preserved"}, Module: domain.ActionOutcome{Status: "not-requested"}}
	}
	c := d.Created
	confirm := func(actual *domain.Rubric) bool {
		if actual == nil || !actual.UseForGradingKnown || actual.UseForGrading || actual.ID <= 0 || actual.AssociationID <= 0 || !sameRubricDefinition(*actual, *d.Rubric) {
			return false
		}
		if (c.RubricID > 0 && c.RubricID != actual.ID) || (c.RubricAssociationID > 0 && c.RubricAssociationID != actual.AssociationID) {
			return false
		}
		c.RubricID = actual.ID
		c.RubricAssociationID = actual.AssociationID
		c.Rubric = domain.ActionOutcome{Status: "preserved", ID: actual.ID, Message: "exact independent live rubric association confirmed"}
		c.Complete = true
		return true
	}
	if snapshot.Rubric != nil {
		if (ambiguous(c.Rubric) || definite(c.Rubric)) && confirm(snapshot.Rubric) {
			report.Status = "completed"
			return report, save()
		}
		report.Status = "held"
		report.Message = "assignment already has a rubric; preserve its existing association and definition"
		return report, nil
	}
	if ambiguous(c.Rubric) || definite(c.Rubric) {
		report.Status = "uncertain"
		report.Message = "prior rubric attachment completion remains uncertain; absent lookup does not authorize replay"
		return report, nil
	}
	c.Rubric = domain.ActionOutcome{Status: "pending"}
	if e = save(); e != nil {
		return report, e
	}
	var response struct {
		Rubric struct {
			ID int64 `json:"id"`
		} `json:"rubric"`
		Association struct {
			ID int64 `json:"id"`
		} `json:"rubric_association"`
	}
	e = s.Client.Write(ctx, "POST", fmt.Sprintf("/api/v1/courses/%d/rubrics", d.CourseID), rubricCreateFields(*d.Rubric, d.AssignmentID), &response)
	if e != nil {
		c.Rubric = failedOutcome(e)
		report.Status = c.Rubric.Status
		report.Message = e.Error()
		if persistErr := save(); persistErr != nil {
			return report, fmt.Errorf("rubric attachment outcome could not be saved: %w", persistErr)
		}
		return report, nil
	}
	if response.Rubric.ID <= 0 || response.Association.ID <= 0 {
		c.Rubric = domain.ActionOutcome{Status: "uncertain", Message: "rubric response lacks exact rubric/association IDs"}
		report.Status = "uncertain"
		report.Message = c.Rubric.Message
		return report, save()
	}
	c.RubricID = response.Rubric.ID
	c.RubricAssociationID = response.Association.ID
	c.Rubric = domain.ActionOutcome{Status: "succeeded", ID: c.RubricID}
	if e = save(); e != nil {
		return report, fmt.Errorf("Canvas attached rubric but local resource IDs could not be saved: %w", e)
	}
	// The rubric response omits assignment settings. Re-read only to confirm
	// this potentially scale-changing association preserved the existing item.
	live, e = GetAssignment(ctx, s.Client, d.CourseID, d.AssignmentID)
	if e != nil {
		return report, e
	}
	if live.CourseID == 0 {
		live.CourseID = d.CourseID
	}
	snapshot, e = Snapshot(live, s.Client.Origin())
	if e != nil {
		return report, e
	}
	compare = snapshot
	compare.Rubric = nil
	compare.UpdatedAt = ""
	if !SameGradingBasis(baseline, compare) || !confirm(snapshot.Rubric) {
		report.Status = "held"
		report.Message = "rubric resource IDs saved, but live attachment or preserved assignment settings do not match; no repair write performed"
		return report, nil
	}
	report.Status = "completed"
	return report, save()
}
