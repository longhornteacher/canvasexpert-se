package domain

import "testing"

func TestAuthorRubricReferencesDoNotWeakenScoringBasis(t *testing.T) {
	for _, r := range []Rubric{{ID: 900001}, {Library: "SCR 4-point"}} {
		if e := ValidateAuthoredRubric(r); e != nil {
			t.Fatal(e)
		}
		if e := ValidateRubric(r); e == nil {
			t.Fatal("reference accepted as a complete scoring basis")
		}
	}
	for _, r := range []Rubric{{Library: "../SCR"}, {Library: "SCR", ID: 900001}, {ID: 900001, Title: "Mixed"}, {Library: "SCR", UseForGrading: true}, {Library: "SCR", Criteria: []Criterion{{ID: "c"}}}} {
		if e := ValidateAuthoredRubric(r); e == nil {
			t.Fatalf("invalid/mixed reference accepted: %+v", r)
		}
	}
}
