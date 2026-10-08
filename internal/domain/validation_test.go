package domain

import "testing"

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
		func(d *ResultsDocument) { d.Students[0].Grade.Components[0].Evidence = "unknown" },
	} {
		d := validResults()
		mutate(&d)
		if rows, e := ValidateResults(&d); e == nil || rows != nil {
			t.Fatalf("invalid document produced usable rows: %v %v", rows, e)
		}
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
	d.Students[0].Grade = &GradeOperation{Attempt: attempt, LateForgiveness: &LateForgiveness{}}
	if _, e := ValidateResults(&d); e == nil {
		t.Fatal("revision accepted grade/late data")
	}
}
func TestLateForgivenessMustActuallyPrecedeCurrentAttempt(t *testing.T) {
	d := validResults()
	d.Students[0].Grade.LateForgiveness = &LateForgiveness{FirstAttempt: AttemptRef{Attempt: 1, SubmittedAt: "2026-10-09T12:00:00Z"}, EffectiveDueAt: "2026-10-10T12:00:00Z", GenuineReason: "Complete response."}
	if _, e := ValidateResults(&d); e == nil {
		t.Fatal("future first attempt accepted")
	}
}
func TestGradeRecoveryTargetPreservesPrecision(t *testing.T) {
	d := validResults()
	target := MustDecimal("17.00400")
	d.Students[0].Grade.DesiredEnteredGrade = &target
	if _, e := ValidateResults(&d); e != nil {
		t.Fatal(e)
	}
	if d.Students[0].Grade.DesiredEnteredGrade.String() != "17.00400" {
		t.Fatal("recovery target precision changed")
	}
	for _, invalid := range []Decimal{{}, MustDecimal("-1"), MustDecimal("15.99")} {
		d.Students[0].Grade.DesiredEnteredGrade = &invalid
		if _, e := ValidateResults(&d); e == nil {
			t.Fatal("invalid recovery target accepted")
		}
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

func TestRevisionRetainedGradeMustBeExplicit(t *testing.T) {
	d := validResults()
	g := d.Students[0].Grade
	g.Components[0].Criteria[0].Points = MustDecimal("0")
	r := &FeedbackRevision{Attempt: g.Attempt, CommentID: 7, Feedback: "More specific feedback.", Assessment: &FeedbackAssessment{Assessment: MustDecimal("0"), RetainedGrade: &Decimal{}, Components: g.Components}}
	d.Students[0] = StudentResult{StudentRef: "student-a", Operation: "revise-feedback", ReviseFeedback: r}
	if _, err := ValidateResults(&d); err == nil {
		t.Fatal("unset retained grade accepted as an entered zero")
	}
	zero := MustDecimal("0")
	r.Assessment.RetainedGrade = &zero
	if _, err := ValidateResults(&d); err != nil {
		t.Fatal(err)
	}
}
