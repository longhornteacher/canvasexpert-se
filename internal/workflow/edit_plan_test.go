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
	if !SameGradingBasis(base, copy) {
		t.Fatal("directions change must not affect grading basis")
	}
}

func TestNewQuizEditAllowsOnlyGroupAndBaseDates(t *testing.T) {
	a, b := int64(7), int64(8)
	base := domain.AssignmentSnapshot{Instance: "https://school.instructure.com", CourseID: 1, AssignmentID: 2, Title: "Quiz", Points: domain.MustDecimal("20"), DescriptionHTML: "<p>Directions</p>", SubmissionTypes: []string{"external_tool"}, NewQuiz: true, AssignmentGroupID: &a}
	doc := domain.AssignmentDocument{Schema: 1, Kind: "assignment", Operation: "update", Instance: base.Instance, CourseID: 1, AssignmentID: 2, Course: "TEST", Title: base.Title, Points: base.Points, SubmissionTypes: base.SubmissionTypes, AssignmentGroupID: &b, Observed: &base}
	due, unlock, lock := "2026-11-20T22:00:00Z", "2026-11-20T14:00:00Z", "2026-11-20T22:00:00Z"
	doc.DueAt, doc.UnlockAt, doc.LockAt = &due, &unlock, &lock
	patch, err := PlanEdit(doc, base.DescriptionHTML)
	if err != nil || len(patch) != 4 {
		t.Fatalf("allowed fields: %v %v", patch, err)
	}
	for name, change := range map[string]func(*domain.AssignmentDocument){
		"title":       func(d *domain.AssignmentDocument) { d.Title = "Changed" },
		"points":      func(d *domain.AssignmentDocument) { d.Points = domain.MustDecimal("21") },
		"submission":  func(d *domain.AssignmentDocument) { d.SubmissionTypes = []string{"online_upload"} },
		"publication": func(d *domain.AssignmentDocument) { d.Published = true },
		"rubric":      func(d *domain.AssignmentDocument) { d.Rubric = &domain.Rubric{} },
		"module":      func(d *domain.AssignmentDocument) { d.ModuleID = 3 },
	} {
		t.Run(name, func(t *testing.T) {
			altered := doc
			change(&altered)
			if _, err := PlanEdit(altered, base.DescriptionHTML); err == nil {
				t.Fatal("unsupported quiz edit accepted")
			}
		})
	}
	if _, err = PlanEdit(doc, "<p>Changed</p>"); err == nil {
		t.Fatal("quiz description edit accepted")
	}
}

func TestAcademicBasisIgnoresPresentationAndDirectionsButRetainsScale(t *testing.T) {
	base := domain.AssignmentSnapshot{Title: "Original", Points: domain.MustDecimal("20"), DescriptionHTML: `<p>Read <a href='https://example.org/a'>A &amp; B</a>.</p><pre><code>x  = 1</code></pre>`}
	live := base
	live.Title, live.Published = "Renamed", true
	due := "2026-10-09T15:00:00Z"
	live.DueAt = &due
	live.DescriptionHTML = `<p>Read <a href="https://example.org/a">A &amp; B</a>.</p>
<pre><code>x  = 1</code></pre>`
	if !SameGradingBasis(base, live) {
		t.Fatal("presentation/scheduling invalidated assessment")
	}
	live = base
	live.Points = domain.MustDecimal("25")
	if SameGradingBasis(base, live) {
		t.Fatal("changed scale ignored")
	}
}

func TestAcademicHTMLComparison(t *testing.T) {
	for _, c := range []struct {
		a, b string
		same bool
	}{
		{`<p>Explain.</p>`, `<p >Explain.</p>`, true},
		{`a <b>x</b>`, `a<b>x</b>`, false},
		{`<span style="color:red">x</span>`, `<span style="color:blue">x</span>`, false},
	} {
		if got := academicHTML(c.a) == academicHTML(c.b); got != c.same {
			t.Errorf("%q vs %q: same=%v want %v", c.a, c.b, got, c.same)
		}
	}
}

func TestSnapshotGroupModeAndDescriptiveMetadataCompatibility(t *testing.T) {
	old := domain.AssignmentSnapshot{CourseID: 1, AssignmentID: 2, Points: domain.MustDecimal("10"), SubmissionTypes: []string{"online_text_entry"}}
	live := old
	individual := false
	live.GradeGroupStudentsIndividually = &individual
	if !SameSnapshot(old, live) || !SameGradingBasis(old, live) {
		t.Fatal("old individual snapshot should accept Canvas false group mode")
	}
	group := live
	group.GroupCategoryID = 50
	unknownGroup := group
	unknownGroup.GradeGroupStudentsIndividually = nil
	if SameSnapshot(group, unknownGroup) || SameGradingBasis(group, unknownGroup) {
		t.Fatal("missing mode on a group assignment must remain unknown")
	}
	metadata := live
	topic, checkpointed := int64(91), true
	metadata.DiscussionTopicID = &topic
	metadata.HasSubAssignments = &checkpointed
	metadata.Checkpoints = &[]domain.AssignmentCheckpoint{}
	if !SameSnapshot(live, metadata) || !SameGradingBasis(live, metadata) {
		t.Fatal("absent descriptive metadata must not invalidate an older individual snapshot")
	}
}
