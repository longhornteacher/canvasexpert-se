package workflow

import (
	"fmt"
	"reflect"
	"time"

	"canvasexpertse/internal/domain"
)

// PlanEdit produces the smallest explicit patch against a pulled baseline.
// It performs no network calls. A caller must compare the live baseline again
// immediately before sending it; no atomic read/write guarantee is implied.
func PlanEdit(doc domain.AssignmentDocument, html string) (map[string]any, error) {
	if err := domain.ValidateAssignment(doc, html); err != nil {
		return nil, err
	}
	if doc.Operation != "update" || doc.Observed == nil {
		return nil, fmt.Errorf("edit requires a freshly pulled assignment")
	}
	base := doc.Observed
	if !sameRubric(doc.Rubric, base.Rubric) {
		return nil, fmt.Errorf("editing a live rubric definition is outside 0.1b; preserve its pulled rubric")
	}
	if doc.ModuleID != 0 {
		return nil, fmt.Errorf("module moves on existing assignments are outside 0.1b")
	}
	patch := map[string]any{}
	if doc.Title != base.Title {
		patch["name"] = doc.Title
	}
	if html != base.DescriptionHTML {
		patch["description"] = html
	}
	if doc.Points.Cmp(base.Points) != 0 {
		patch["points_possible"] = doc.Points
	}
	if !reflect.DeepEqual(doc.SubmissionTypes, base.SubmissionTypes) {
		patch["submission_types"] = doc.SubmissionTypes
	}
	if !sameDate(doc.DueAt, base.DueAt) {
		patch["due_at"] = doc.DueAt
	}
	if !sameDate(doc.UnlockAt, base.UnlockAt) {
		patch["unlock_at"] = doc.UnlockAt
	}
	if !sameDate(doc.LockAt, base.LockAt) {
		patch["lock_at"] = doc.LockAt
	}
	if doc.Published != base.Published {
		patch["published"] = doc.Published
	}
	return patch, nil
}

// SameGradingBasis ignores unrelated timestamps but includes the actual work's
// expectations, scale, rubric, and submission/grade modes.
func SameGradingBasis(a, b domain.AssignmentSnapshot) bool {
	a.UpdatedAt = ""
	b.UpdatedAt = ""
	a.Published = false
	b.Published = false
	return SameSnapshot(a, b)
}

// SameSnapshot compares numeric values and date instants without mistaking API
// formatting changes for teacher edits. Other content and identities stay exact.
func SameSnapshot(a, b domain.AssignmentSnapshot) bool {
	if a.Points.Cmp(b.Points) != 0 || !sameRubric(a.Rubric, b.Rubric) {
		return false
	}
	a.Points = domain.Decimal{}
	b.Points = domain.Decimal{}
	a.Rubric = nil
	b.Rubric = nil
	a.DueAt = canonicalDate(a.DueAt)
	b.DueAt = canonicalDate(b.DueAt)
	a.UnlockAt = canonicalDate(a.UnlockAt)
	b.UnlockAt = canonicalDate(b.UnlockAt)
	a.LockAt = canonicalDate(a.LockAt)
	b.LockAt = canonicalDate(b.LockAt)
	return reflect.DeepEqual(a, b)
}
func canonicalDate(v *string) *string {
	if v == nil || *v == "" {
		return nil
	}
	t, e := time.Parse(time.RFC3339Nano, *v)
	if e != nil {
		return v
	}
	s := t.UTC().Format(time.RFC3339Nano)
	return &s
}
func sameRubric(a, b *domain.Rubric) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	ac, bc := *a, *b
	if len(ac.Criteria) != len(bc.Criteria) {
		return false
	}
	ac.Criteria = append([]domain.Criterion(nil), ac.Criteria...)
	bc.Criteria = append([]domain.Criterion(nil), bc.Criteria...)
	for i := range ac.Criteria {
		x, y := &ac.Criteria[i], &bc.Criteria[i]
		if x.Points.Cmp(y.Points) != 0 || len(x.Ratings) != len(y.Ratings) {
			return false
		}
		x.Points = domain.Decimal{}
		y.Points = domain.Decimal{}
		x.Ratings = append([]domain.Rating(nil), x.Ratings...)
		y.Ratings = append([]domain.Rating(nil), y.Ratings...)
		for j := range x.Ratings {
			if x.Ratings[j].Points.Cmp(y.Ratings[j].Points) != 0 {
				return false
			}
			x.Ratings[j].Points = domain.Decimal{}
			y.Ratings[j].Points = domain.Decimal{}
		}
	}
	return reflect.DeepEqual(ac, bc)
}
