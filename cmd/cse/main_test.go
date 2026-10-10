package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

func TestHelpAndVersionNeedNoWorkspaceOrCanvas(t *testing.T) {
	for _, args := range [][]string{nil, {"help"}, {"--help"}, {"version"}} {
		var out, err bytes.Buffer
		if code := run(context.Background(), args, &out, &err); code != 0 || !strings.Contains(out.String(), "CanvasExpert-SE") {
			t.Fatalf("%v: %d %s", args, code, err.String())
		}
	}
}
func TestInvalidResultsAreRejectedBeforeCredentials(t *testing.T) {
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, "context"), 0755)
	_ = os.WriteFile(filepath.Join(root, "context", "courses.md"), []byte("---\nschema: 1\ninstance: https://school.example\naliases: true\ncourses: []\n---\n"), 0644)
	_ = os.WriteFile(filepath.Join(root, "bad.md"), []byte("---\nschema: 1\nkind: results\nkind: results\n---\n"), 0644)
	var out, err bytes.Buffer
	code := run(context.Background(), []string{"--workspace", root, "post", "bad.md"}, &out, &err)
	if code != 2 || strings.Contains(err.String(), "credential") {
		t.Fatalf("invalid file reached environment: %d %s", code, err.String())
	}
}

func TestHelpFormatsAndExamplesNeedNoWorkspace(t *testing.T) {
	for _, topic := range []string{"formats", "fields", "examples"} {
		var out, err bytes.Buffer
		if code := run(context.Background(), []string{"help", topic}, &out, &err); code != 0 || out.Len() == 0 {
			t.Fatalf("help %s: %d %s", topic, code, err.String())
		}
	}
}

func TestValidateResultsPreviewsComputedAssessmentTimingAndHold(t *testing.T) {
	root := t.TempDir()
	config := domain.CourseConfig{Schema: 1, Instance: "https://school.example", Courses: []domain.Course{{Nickname: "TEST", ID: 1}}}
	if e := workspace.WriteDocument(filepath.Join(root, "context", "courses.md"), config, ""); e != nil {
		t.Fatal(e)
	}
	attempt := domain.AttemptRef{Attempt: 1, SubmittedAt: "2026-10-09T12:00:00Z"}
	grade := &domain.GradeOperation{Attempt: attempt, Feedback: "Explain your evidence.", Components: []domain.ComponentScore{{Key: "answer", Evidence: "present", Criteria: []domain.CriterionScore{{CriterionID: "overall", Points: domain.MustDecimal("6.25")}}}}, LateTiming: &domain.LateTiming{Status: "late", SecondsLate: 93600, DueAt: "2026-10-08T10:00:00Z", Basis: "Teacher supplied timing"}}
	held := &domain.GradeOperation{Attempt: attempt, Feedback: "Unreadable work.", Components: []domain.ComponentScore{{Key: "answer", Evidence: "unknown"}}}
	doc := domain.ResultsDocument{Schema: 1, Kind: "results", Packet: "packet.md", Assignment: domain.AssignmentSnapshot{Instance: config.Instance, CourseID: 1, AssignmentID: 10, Title: "Synthetic reflection", Points: domain.MustDecimal("20"), GradingType: "points", SubmissionTypes: []string{"online_text_entry"}}, ScoringBasis: &domain.Rubric{Title: "Overall", Criteria: []domain.Criterion{{ID: "overall", Description: "Overall evidence", Points: domain.MustDecimal("10")}}}, Interpretation: domain.Interpretation{Mapping: "repeated", Rationale: "One response", Components: []domain.Component{{Key: "answer", Label: "Answer", Weight: domain.MustDecimal("1")}}}, Students: []domain.StudentResult{{StudentRef: "s1", Operation: "grade", Grade: grade}, {StudentRef: "s2", Operation: "grade", Grade: held}}}
	if e := workspace.WriteDocument(filepath.Join(root, "results.md"), doc, ""); e != nil {
		t.Fatal(e)
	}
	var out, progress bytes.Buffer
	if code := run(context.Background(), []string{"--workspace", root, "validate", "results.md"}, &out, &progress); code != 0 {
		t.Fatalf("validate failed: %d %s", code, progress.String())
	}
	for _, want := range []string{"Valid document:", "s1  12.5  late: 1d 2h", "s2  held: unreadable work for component answer; not scored"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q from preview: %s", want, out.String())
		}
	}
	if strings.Contains(out.String(), "s2  0") {
		t.Fatal("unknown evidence previewed as a score")
	}
	out.Reset()
	if code := run(context.Background(), []string{"--workspace", root, "validate", "context/courses.md"}, &out, &progress); code != 0 || strings.Contains(out.String(), "late:") || strings.Contains(out.String(), "held:") {
		t.Fatalf("non-results output changed: %d %s %s", code, out.String(), progress.String())
	}
}

func TestClassicExamplesAndUnknownPreview(t *testing.T) {
	root := t.TempDir()
	config := domain.CourseConfig{Schema: 1, Instance: "https://canvas.example", Courses: []domain.Course{{Nickname: "TEST", ID: 1}}}
	if e := workspace.WriteDocument(filepath.Join(root, "context", "courses.md"), config, ""); e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"classic-writing-packet.md", "classic-writing-work.md", "classic-writing-results.md"} {
		b, e := os.ReadFile(filepath.Join("..", "..", "engine-docs", "examples", name))
		if e != nil {
			t.Fatal(e)
		}
		path := filepath.Join(root, "engine-docs", "examples", name)
		if e = workspace.WriteFile(path, b, 0644); e != nil {
			t.Fatal(e)
		}
		if e = validateDocument(path, root); e != nil {
			t.Fatalf("example %s: %v", name, e)
		}
	}
	path := filepath.Join(root, "engine-docs", "examples", "classic-writing-results.md")
	var doc domain.ClassicQuizResults
	if _, e := workspace.ReadDocument(path, &doc); e != nil {
		t.Fatal(e)
	}
	doc.Students[0].Questions[2].Outcome = domain.ActionOutcome{Status: "uncertain", ID: 504}
	if e := workspace.WriteDocument(path, &doc, "synthetic example"); e != nil {
		t.Fatal(e)
	}
	var out, progress bytes.Buffer
	if code := run(context.Background(), []string{"--workspace", root, "validate", "engine-docs/examples/classic-writing-results.md"}, &out, &progress); code != 0 {
		t.Fatal(code, progress.String())
	}
	for _, want := range []string{"s-synthetic-classic | 2 | 502 | 7.5/10", "s-synthetic-classic | 2 | 503 | 0/5", "s-synthetic-classic | 2 | 504 | held:", "s-synthetic-classic | late: 1d 0h"} {
		if !strings.Contains(out.String(), want) {
			t.Fatal(want, out.String())
		}
	}
	// Source errors are rejected at the command boundary before credential access.
	doc.Students[0].Questions[0].QuestionID = 999
	if e := workspace.WriteDocument(path, &doc, ""); e != nil {
		t.Fatal(e)
	}
	out.Reset()
	progress.Reset()
	if code := run(context.Background(), []string{"--workspace", root, "post", "engine-docs/examples/classic-writing-results.md"}, &out, &progress); code != 2 || strings.Contains(progress.String(), "credential") {
		t.Fatal(code, progress.String())
	}
}
