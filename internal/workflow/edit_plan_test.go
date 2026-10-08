package workflow

import (
	"canvasexpertse/internal/domain"
	"reflect"
	"testing"
)

func TestSnapshotFormattingDoesNotInventLiveChanges(t *testing.T) {
	dueA := "2026-10-08T09:00:00-05:00"
	dueB := "2026-10-08T14:00:00Z"
	a := domain.AssignmentSnapshot{Points: domain.MustDecimal("20"), DueAt: &dueA, Rubric: &domain.Rubric{Criteria: []domain.Criterion{{ID: "c", Points: domain.MustDecimal("100"), Ratings: []domain.Rating{{ID: "r", Points: domain.MustDecimal("10")}}}}}}
	b := a
	b.Points = domain.MustDecimal("20.0")
	b.DueAt = &dueB
	b.Rubric = &domain.Rubric{Criteria: []domain.Criterion{{ID: "c", Points: domain.MustDecimal("100.000"), Ratings: []domain.Rating{{ID: "r", Points: domain.MustDecimal("10.0")}}}}}
	beforeA, beforeB := a.Rubric.Criteria[0].Points.String(), b.Rubric.Criteria[0].Points.String()
	if !SameSnapshot(a, b) {
		t.Fatal("equivalent API decimals/date caused conflict")
	}
	if a.Rubric.Criteria[0].Points.String() != beforeA || b.Rubric.Criteria[0].Points.String() != beforeB {
		t.Fatal("comparison mutated input")
	}
	b.Rubric.Criteria[0].Ratings[0].ID = "different"
	if SameSnapshot(a, b) {
		t.Fatal("criterion identity change ignored")
	}
	if reflect.DeepEqual(a, b) {
		t.Fatal("fixture unexpectedly identical")
	}
}

func TestEditPreservesHTMLAndOmittedSettings(t *testing.T) {
	base := domain.AssignmentSnapshot{Instance: "https://school.instructure.com", CourseID: 1, AssignmentID: 2, Title: "Reading", Points: domain.MustDecimal("20"), DescriptionHTML: `<div data-teacher="keep"><details><summary>Hint</summary><p>Original</p></details></div>`, SubmissionTypes: []string{"online_text_entry"}}
	doc := domain.AssignmentDocument{Schema: 1, Kind: "assignment", Operation: "update", Instance: base.Instance, CourseID: 1, AssignmentID: 2, Course: "TEST", Title: base.Title, Points: base.Points, SubmissionTypes: base.SubmissionTypes, Observed: &base}
	changed := `<div data-teacher="keep"><details><summary>Hint</summary><p>Changed</p></details></div>`
	patch, err := PlanEdit(doc, changed)
	if err != nil {
		t.Fatal(err)
	}
	if len(patch) != 1 || patch["description"] != changed {
		t.Fatalf("unexpected patch %#v", patch)
	}
	base.UpdatedAt = "later unrelated timestamp"
	copy := base
	copy.UpdatedAt = "earlier"
	if !SameGradingBasis(base, copy) {
		t.Fatal("timestamp alone invalidates work")
	}
	copy.DescriptionHTML = "teacher changed instructions"
	if SameGradingBasis(base, copy) {
		t.Fatal("directions change not detected")
	}
}
