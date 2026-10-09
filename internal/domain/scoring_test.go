package domain

import (
	"encoding/json"
	"math/big"
	"strings"
	"testing"
)

func oneCriterion(max string) Rubric {
	return Rubric{Title: "Writing", Criteria: []Criterion{{ID: "quality", Description: "Quality", Points: MustDecimal(max)}}}
}
func repeated(weights ...string) Interpretation {
	in := Interpretation{Mapping: "repeated", Rationale: "Assignment requires each response."}
	for i, w := range weights {
		key := string(rune('a' + i))
		in.Components = append(in.Components, Component{Key: key, Label: key, Weight: MustDecimal(w)})
	}
	return in
}
func criterionScore(key, points string) ComponentScore {
	return ComponentScore{Key: key, Evidence: "present", Criteria: []CriterionScore{{CriterionID: "quality", Points: MustDecimal(points)}}}
}

func TestAssignmentRubricScaleAndExactWeights(t *testing.T) {
	a, e := ComputeAssessment(oneCriterion("100"), repeated("1", "2"), []ComponentScore{criterionScore("a", "80"), criterionScore("b", "50")}, MustDecimal("20"))
	if e != nil {
		t.Fatal(e)
	}
	if a.Posted.String() != "12" || a.Criteria[0].Points.Cmp(big.NewRat(60, 1)) != 0 {
		t.Fatalf("wanted 12/20 and 60/100, got %+v", a)
	}

}
func TestFractionalRepeatedAndDirect(t *testing.T) {
	r := oneCriterion("3.5")
	a, e := ComputeAssessment(r, repeated("0.5", "1.25"), []ComponentScore{criterionScore("a", "1.25"), criterionScore("b", "2.75")}, MustDecimal("20"))
	// (1.25*2/7 + 2.75*5/7)/3.5*20 = 650/49.
	if e != nil {
		t.Fatal(e)
	}
	if a.Unrounded.Cmp(big.NewRat(650, 49)) != 0 || a.Posted.String() != "13.27" {
		t.Fatalf("wrong exact weighted assessment: %s (%s)", a.Unrounded, a.Posted)
	}
	r = Rubric{Criteria: []Criterion{{ID: "a", Description: "First", Points: MustDecimal("1.5")}, {ID: "b", Description: "Second", Points: MustDecimal("2.5")}}}
	in := Interpretation{Mapping: "direct", Rationale: "Separate criteria already identify tasks.", Components: []Component{{Key: "first", Label: "First", Weight: MustDecimal("1"), CriterionIDs: []string{"a"}}, {Key: "second", Label: "Second", Weight: MustDecimal("1"), CriterionIDs: []string{"b"}}}}
	scores := []ComponentScore{{Key: "first", Evidence: "present", Criteria: []CriterionScore{{CriterionID: "a", Points: MustDecimal(".75")}}}, {Key: "second", Evidence: "present", Criteria: []CriterionScore{{CriterionID: "b", Points: MustDecimal("1.25")}}}}
	a, e = ComputeAssessment(r, in, scores, MustDecimal("20"))
	if e != nil {
		t.Fatal(e)
	}
	if a.Posted.String() != "10" {
		t.Fatalf("direct mapping double counted: %s", a.Posted)
	}
}
func TestZeroWeightRequiresPositiveTotal(t *testing.T) {
	if _, e := ComputeAssessment(oneCriterion("100"), repeated("0", "1"), []ComponentScore{criterionScore("a", "0"), criterionScore("b", "80")}, MustDecimal("20")); e != nil {
		t.Fatal(e)
	}
	if e := ValidateInterpretation(oneCriterion("100"), repeated("0", "0")); e == nil {
		t.Fatal("zero total accepted")
	}
}
func TestRubricOnlyMaximumRemainsExact(t *testing.T) {
	a, e := ComputeAssessment(oneCriterion(".123456789012345"), repeated("1"), []ComponentScore{criterionScore("a", ".123456789012345")})
	if e != nil {
		t.Fatal(e)
	}
	if a.Unrounded.Cmp(MustDecimal(".123456789012345").Rat()) != 0 {
		t.Fatalf("rubric maximum rounded early: %s", a.Unrounded)
	}
}
func TestOverallUsesUnroundedCriteria(t *testing.T) {
	r := Rubric{Criteria: []Criterion{{ID: "a", Description: "First", Points: MustDecimal("1")}, {ID: "b", Description: "Second", Points: MustDecimal("1")}}}
	scores := []ComponentScore{}
	for i, key := range []string{"a", "b", "c"} {
		points := "0"
		if i == 0 {
			points = "1"
		}
		scores = append(scores, ComponentScore{Key: key, Evidence: "present", Criteria: []CriterionScore{{CriterionID: "a", Points: MustDecimal(points)}, {CriterionID: "b", Points: MustDecimal(points)}}})
	}
	a, e := ComputeAssessment(r, repeated("1", "1", "1"), scores, MustDecimal("100"))
	if e != nil {
		t.Fatal(e)
	}
	if a.Posted.String() != "33.33" || a.Criteria[0].Points.Cmp(big.NewRat(1, 3)) != 0 {
		t.Fatalf("overall used rounded criterion displays: %s (%s)", a.Posted, a.Criteria[0].Points)
	}
}
func TestCriterionExplanationAndTruthfulRatings(t *testing.T) {
	r := oneCriterion("100")
	r.Criteria[0].Ratings = []Rating{{ID: "full", Points: MustDecimal("100")}, {ID: "partial", Points: MustDecimal("50")}}
	scores := []ComponentScore{criterionScore("a", "80"), criterionScore("b", "50")}
	scores[0].Criteria[0].Comment = "Evidence is specific."
	scores[1].Criteria[0].Comment = "Reasoning needs support."
	a, e := ComputeAssessment(r, repeated("1", "2"), scores, MustDecimal("20"))
	if e != nil {
		t.Fatal(e)
	}
	if a.Criteria[0].RatingID != "" || a.Criteria[0].Comment != "a: Evidence is specific.\nb: Reasoning needs support." {
		t.Fatalf("false rating or missing rubric feedback: %+v", a.Criteria[0])
	}
	scores[0].Criteria[0].RatingID = "full"
	if _, e = ComputeAssessment(r, repeated("1", "2"), scores); e == nil {
		t.Fatal("accepted full rating attached to 80 points")
	}
}
func TestDecimalJSONAndBoundaries(t *testing.T) {
	for _, raw := range []string{"+001.50", ".5", "-.25", "1."} {
		b, e := json.Marshal(MustDecimal(raw))
		if e != nil || !json.Valid(b) {
			t.Fatalf("invalid JSON for %q: %s %v", raw, b, e)
		}
		var d Decimal
		if e = json.Unmarshal(b, &d); e != nil || d.Cmp(MustDecimal(raw)) != 0 {
			t.Fatalf("decimal changed: %q %s %v", raw, b, e)
		}
	}
	for _, raw := range []string{".nan", ".inf", "NaN", "1/3", ""} {
		if _, e := ParseDecimal(raw); e == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
	if got := RoundRat(big.NewRat(-1005, 1000), 2).String(); got != "-1.01" {
		t.Fatalf("half-away rounding: %s", got)
	}
}

func noRubricResults(basis *Rubric) *ResultsDocument {
	snap := AssignmentSnapshot{Instance: "https://canvas.example", CourseID: 1, AssignmentID: 10, Title: "Reflection", Points: MustDecimal("20"), SubmissionTypes: []string{"online_text_entry"}, GradingType: "points"}
	attempt := AttemptRef{Attempt: 1, SubmittedAt: "2026-10-07T12:00:00Z"}
	grade := &GradeOperation{Attempt: attempt, Assessment: MustDecimal("12"), Feedback: "Specific, kind feedback.", Components: []ComponentScore{{Key: "a", Evidence: "present", Criteria: []CriterionScore{{CriterionID: "overall", Points: MustDecimal("6")}}}}}
	return &ResultsDocument{Schema: 1, Kind: "results", Packet: "packet.md", Assignment: snap, ScoringBasis: basis, Interpretation: Interpretation{Mapping: "repeated", Rationale: "One response.", Components: []Component{{Key: "a", Label: "Response", Weight: MustDecimal("1")}}}, Students: []StudentResult{{StudentRef: "s1", Operation: "grade", Grade: grade}}}
}
func overallBasis() *Rubric {
	return &Rubric{Title: "Overall", Criteria: []Criterion{{ID: "overall", Description: "Overall quality", Points: MustDecimal("10")}}}
}
func TestNoCanvasRubricScoresAgainstAgentScoringBasis(t *testing.T) {
	validated, e := ValidateResults(noRubricResults(overallBasis()))
	if e != nil {
		t.Fatal(e)
	}
	if validated[0].Assessment.Posted.String() != "12" {
		t.Fatalf("wanted 12/20 from 6/10 on a 20-point assignment, got %s", validated[0].Assessment.Posted)
	}
}
func TestNoCanvasRubricWithoutScoringBasisGivesActionableError(t *testing.T) {
	if _, e := ValidateResults(noRubricResults(nil)); e == nil || !strings.Contains(e.Error(), "add scoring_basis") {
		t.Fatalf("expected guidance to add scoring_basis, got %v", e)
	}
}
func TestScoringBasisRejectedWhenCanvasRubricExists(t *testing.T) {
	doc := noRubricResults(overallBasis())
	doc.Assignment.Rubric = overallBasis()
	if _, e := ValidateResults(doc); e == nil || !strings.Contains(e.Error(), "only for assignments without a Canvas rubric") {
		t.Fatalf("expected conflict error, got %v", e)
	}
}
