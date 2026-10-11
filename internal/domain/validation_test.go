package domain

import (
	"strings"
	"testing"
)

func validResults() ResultsDocument {
	return ResultsDocument{Schema: 1, Kind: "results", Packet: "packet.md", Assignment: AssignmentSnapshot{Instance: "https://school.instructure.com", CourseID: 1, AssignmentID: 2, Title: "Writing", Points: MustDecimal("20"), SubmissionTypes: []string{"online_text_entry"}, GradingType: "points", Rubric: rubricPointer(oneCriterion("100"))}, Interpretation: repeated("1"), Students: []StudentResult{{StudentRef: "student-a", Operation: "grade", Grade: &GradeOperation{Attempt: AttemptRef{Attempt: 2, SubmittedAt: "2026-10-08T12:00:00Z"}, Components: []ComponentScore{criterionScore("a", "80")}, Assessment: MustDecimal("16"), Feedback: "Your evidence supports the claim."}}}}
}
func rubricPointer(r Rubric) *Rubric { return &r }

func TestResultsValidateWholeFile(t *testing.T) {
	for _, mutate := range []func(*ResultsDocument){
		func(d *ResultsDocument) { d.Students = append(d.Students, d.Students[0]) },
		func(d *ResultsDocument) { d.Students[0].Grade.Components[0].Criteria[0].CriterionID = "invented" },
		func(d *ResultsDocument) { d.Students[0].Grade.Components[0].Criteria[0].Points = MustDecimal("101") },
		func(d *ResultsDocument) { d.Students[0].Grade.Assessment = MustDecimal("17") },
		func(d *ResultsDocument) { d.Assignment.GradingType = "pass_fail" },
		func(d *ResultsDocument) { d.Assignment.AnonymousGrading = true },
		func(d *ResultsDocument) { d.Assignment.GroupCategoryID = 10 },
	} {
		d := validResults()
		mutate(&d)
		if rows, e := ValidateResults(&d); e == nil || rows != nil {
			t.Fatalf("invalid document produced usable rows: %v %v", rows, e)
		}
	}
}
func TestUnknownEvidenceHoldsOneStudentAndAssessmentIsFilled(t *testing.T) {
	d := validResults()
	second := d.Students[0]
	g := *second.Grade
	g.Components = append([]ComponentScore(nil), g.Components...)
	g.Components[0].Evidence = "unknown"
	second.StudentRef, second.Grade = "student-b", &g
	d.Students = append(d.Students, second)
	want := d.Students[0].Grade.Assessment
	d.Students[0].Grade.Assessment = Decimal{}
	rows, e := ValidateResults(&d)
	if e != nil || rows[0].Hold != "" || rows[1].Hold == "" {
		t.Fatalf("holds wrong: %v %+v", e, rows)
	}
	if d.Students[0].Grade.Assessment.Cmp(want) != 0 {
		t.Fatal("omitted assessment was not filled with the computed value")
	}
}
func TestFeedbackRevisionCannotCarryGradeOrLateActions(t *testing.T) {
	d := validResults()
	attempt := d.Students[0].Grade.Attempt
	d.Assignment.GradingType = "pass_fail" // A text-only revision needs no numerical basis.
	d.Students[0] = StudentResult{StudentRef: "student-a", Operation: "revise-feedback", ReviseFeedback: &FeedbackRevision{Attempt: attempt, CommentID: 7, Feedback: "More specific feedback."}}
	if _, e := ValidateResults(&d); e != nil {
		t.Fatal(e)
	}
	d.Students[0].Grade = &GradeOperation{Attempt: attempt, LateTiming: &LateTiming{}}
	if _, e := ValidateResults(&d); e == nil {
		t.Fatal("revision accepted grade/late data")
	}
}
func TestCreateOutcomesRemainConsistent(t *testing.T) {
	d := AssignmentDocument{Schema: 1, Kind: "assignment", Operation: "create", Course: "CS8", Title: "Writing", Points: MustDecimal("20"), SubmissionTypes: []string{"online_text_entry"}, Created: &CreatedResources{AssignmentID: 2, Assignment: ActionOutcome{Status: "succeeded", ID: 2}, Publication: ActionOutcome{Status: "preserved"}, Complete: true}}
	if e := ValidateAssignment(d, "Directions"); e != nil {
		t.Fatal(e)
	}
	d.Created.Module = ActionOutcome{Status: "succeeded"}
	if e := ValidateAssignment(d, "Directions"); e == nil {
		t.Fatal("module success without id accepted")
	}
	d.Created.Module = ActionOutcome{}
	d.Rubric = rubricPointer(oneCriterion("100"))
	if e := ValidateAssignment(d, "Directions"); e == nil {
		t.Fatal("completed create missing required rubric accepted")
	}
}

func TestNewRubricRequiresIndependentGradingWithoutRewritingDraft(t *testing.T) {
	r := oneCriterion("100")
	r.UseForGrading = true
	d := AssignmentDocument{Schema: 1, Kind: "assignment", Operation: "create", Course: "CS8", Title: "Writing", Points: MustDecimal("20"), SubmissionTypes: []string{"online_text_entry"}, Rubric: &r}
	if err := ValidateAssignment(d, "Directions"); err == nil || !strings.Contains(err.Error(), "use_for_grading: false") || !strings.Contains(err.Error(), "separately computed grades") {
		t.Fatalf("new coupled rubric was not refused helpfully: %v", err)
	}
	if !d.Rubric.UseForGrading {
		t.Fatal("validation silently rewrote teacher intent")
	}
	d.Rubric.UseForGrading = false
	if err := ValidateAssignment(d, "Directions"); err != nil {
		t.Fatal(err)
	}
	// Existing snapshots, including updates, keep their observed coupling.
	d.Operation = "update"
	d.Instance, d.CourseID, d.AssignmentID = "https://school.instructure.com", 1, 2
	d.Rubric.UseForGrading = true
	d.Observed = &AssignmentSnapshot{Instance: d.Instance, CourseID: d.CourseID, AssignmentID: d.AssignmentID, Title: d.Title, Points: d.Points, SubmissionTypes: d.SubmissionTypes, Rubric: d.Rubric}
	if err := ValidateAssignment(d, "Directions"); err != nil {
		t.Fatalf("existing coupled rubric snapshot rejected: %v", err)
	}
}
func TestCanvasOriginAndCoursePathSafety(t *testing.T) {
	for _, origin := range []string{"https://:443", "https://school.instructure.com?", "https://user:secret@school.instructure.com", "https:school.instructure.com", "https://school.instructure.com/path"} {
		if validInstance(origin) {
			t.Fatalf("accepted origin %q", origin)
		}
	}
	for _, name := range []string{"../outside", "CON", "NUL.md", "COM¹", "CS8.", "CS8 ", "a?b", "a\nb"} {
		c := CourseConfig{Schema: 1, Instance: "https://school.instructure.com", Courses: []Course{{Nickname: name, ID: 1}}}
		if e := ValidateConfig(c); e == nil {
			t.Fatalf("accepted nickname %q", name)
		}
	}
}

func TestRubricRejectsNegativeResourceIDs(t *testing.T) {
	for _, mutate := range []func(*Rubric){
		func(r *Rubric) { r.ID = -1 },
		func(r *Rubric) { r.AssociationID = -1 },
	} {
		d := validResults()
		mutate(d.Assignment.Rubric)
		if rows, err := ValidateResults(&d); err == nil || rows != nil {
			t.Fatalf("invalid rubric identity accepted: %v", err)
		}
	}
}

func TestRevisionOutcomeMustReferToSelectedComment(t *testing.T) {
	d := validResults()
	attempt := d.Students[0].Grade.Attempt
	r := &FeedbackRevision{Attempt: attempt, CommentID: 7, Feedback: "More specific feedback.", Outcome: ActionOutcome{Status: "succeeded", ID: 8}}
	d.Students[0] = StudentResult{StudentRef: "student-a", Operation: "revise-feedback", ReviseFeedback: r}
	if rows, err := ValidateResults(&d); err == nil || rows != nil {
		t.Fatal("revision outcome for a different comment accepted")
	}
	r.Outcome.ID = r.CommentID
	if _, err := ValidateResults(&d); err != nil {
		t.Fatal(err)
	}
}
