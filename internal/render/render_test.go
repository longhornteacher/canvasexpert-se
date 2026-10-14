package render

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

func TestRenderFeedbackShowsAssessmentScoresThenConversation(t *testing.T) {
	rubric := domain.Rubric{Title: "Synthetic rubric", Criteria: []domain.Criterion{
		{ID: "ev", Description: "Evidence", Points: domain.MustDecimal("4")},
		{ID: "rs", Description: "Reasoning", Points: domain.MustDecimal("6")},
	}}
	interpretation := domain.Interpretation{Mapping: "repeated", Rationale: "PRIVATE rationale must not render", Components: []domain.Component{
		{Key: "one", Label: "Response 1", Weight: domain.MustDecimal("1")},
		{Key: "two", Label: "Response 2", Weight: domain.MustDecimal("2")},
	}}
	components := []domain.ComponentScore{
		{Key: "one", Evidence: "present", Criteria: []domain.CriterionScore{{CriterionID: "ev", Points: domain.MustDecimal("3")}, {CriterionID: "rs", Points: domain.MustDecimal("4")}}},
		{Key: "two", Evidence: "missing", Criteria: []domain.CriterionScore{{CriterionID: "ev", Points: domain.MustDecimal("0")}, {CriterionID: "rs", Points: domain.MustDecimal("0")}}},
	}
	feedback := "Try **a specific** next step.\nQuoted text: `a < b`"
	got, err := RenderFeedback(FeedbackInput{Rubric: rubric, Interpretation: interpretation, Components: components, Header: "full", Assessment: domain.MustDecimal("4.67"), AssignmentPoints: domain.MustDecimal("20"), Feedback: feedback})
	if err != nil {
		t.Fatal(err)
	}
	want := "Assessment: 4.67 / 20\nCriterion scores:\n- Response 1: Evidence 3/4; Reasoning 4/6\n- Response 2: Evidence 0/4; Reasoning 0/6\n\n" + feedback
	if got != want {
		t.Fatalf("unexpected rendering\n got: %s\nwant: %s", got, want)
	}
	if strings.Contains(got, "PRIVATE") || strings.Contains(got, "teacher_note") {
		t.Fatalf("private scoring context leaked: %s", got)
	}
}

func TestRenderFeedbackDirectMappingAndXSS(t *testing.T) {
	rubric := domain.Rubric{Title: "R", Criteria: []domain.Criterion{{ID: "a", Description: "Read <carefully>", Points: domain.MustDecimal("5")}, {ID: "b", Description: "Write", Points: domain.MustDecimal("5")}}}
	interpretation := domain.Interpretation{Mapping: "direct", Rationale: "private", Components: []domain.Component{
		{Key: "first", Label: "Prompt A", Weight: domain.MustDecimal("1"), CriterionIDs: []string{"a"}},
		{Key: "second", Label: "Prompt B", Weight: domain.MustDecimal("1"), CriterionIDs: []string{"b"}},
	}}
	scores := []domain.ComponentScore{
		{Key: "first", Evidence: "present", Criteria: []domain.CriterionScore{{CriterionID: "a", Points: domain.MustDecimal("4")}}},
		{Key: "second", Evidence: "present", Criteria: []domain.CriterionScore{{CriterionID: "b", Points: domain.MustDecimal("3")}}},
	}
	got, err := RenderFeedback(FeedbackInput{Rubric: rubric, Interpretation: interpretation, Components: scores, Header: "full", Assessment: domain.MustDecimal("7"), AssignmentPoints: domain.MustDecimal("10"), Feedback: "<script>alert(1)</script>"})
	if err != nil {
		t.Fatal(err)
	}
	want := "Assessment: 7 / 10\nCriterion scores:\n- Prompt A: Read <carefully> 4/5\n- Prompt B: Write 3/5\n\n<script>alert(1)</script>"
	if got != want {
		t.Fatalf("plain text criteria or exact feedback preservation incorrect:\n got: %s\nwant: %s", got, want)
	}
}

func TestRenderCommentOnlyOmitsAggregateAssessment(t *testing.T) {
	rubric := domain.Rubric{Title: "R", Criteria: []domain.Criterion{{ID: "a", Description: "Evidence", Points: domain.MustDecimal("5")}}}
	interpretation := domain.Interpretation{Mapping: "repeated", Rationale: "private", Components: []domain.Component{{Key: "one", Label: "Response", Weight: domain.MustDecimal("1")}}}
	got, err := RenderFeedback(FeedbackInput{Rubric: rubric, Interpretation: interpretation, Components: []domain.ComponentScore{{Key: "one", Evidence: "present", Criteria: []domain.CriterionScore{{CriterionID: "a", Points: domain.MustDecimal("4")}}}}, Assessment: domain.MustDecimal("16"), AssignmentPoints: domain.MustDecimal("20"), Feedback: "Good evidence.", OmitAssessment: true, Header: "full"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "Assessment:") || !strings.HasPrefix(got, "Criterion scores:\n") || !strings.HasSuffix(got, "\n\nGood evidence.") {
		t.Fatalf("comment-only text included an aggregate grade or lost feedback: %q", got)
	}
}

func TestMarkdownHTMLSafeSubsetAndHTMLText(t *testing.T) {
	got, err := MarkdownHTML("# Directions\n\nAnswer both prompts.\n\n- First\n- Second\n\n```text\nx < y\n```")
	if err != nil {
		t.Fatal(err)
	}
	want := `<h1>Directions</h1><p>Answer both prompts.</p><ul><li>First</li><li>Second</li></ul><pre><code>x &lt; y</code></pre>`
	if got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
	if text := HTMLText(`<p>Use &amp; compare</p><ul><li>One</li><li>Two</li></ul>`); text != "Use & compare\nOne\nTwo" {
		t.Fatalf("unexpected HTML text %q", text)
	}
}

func TestSyntheticExamplesMatchDocumentSchemas(t *testing.T) {
	root := filepath.Join("..", "..", "engine-docs", "examples")
	for _, name := range []string{
		"synthetic-assignment-create.md",
		"synthetic-results-two.md",
		"synthetic-results-four.md",
		"synthetic-results-choice-one.md",
	} {
		t.Run(name, func(t *testing.T) {
			source, err := os.ReadFile(filepath.Join(root, name))
			if err != nil {
				t.Fatal(err)
			}
			if name == "synthetic-assignment-create.md" {
				var doc domain.AssignmentDocument
				if _, err := workspace.ParseDocument(source, &doc); err != nil {
					t.Fatal(err)
				}
				return
			}
			var doc domain.ResultsDocument
			if _, err := workspace.ParseDocument(source, &doc); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestMarkdownHTMLRendersEmphasis(t *testing.T) {
	got, err := MarkdownHTML("Include **two responses** and *one* _conclusion_.\n\n- **Bold** item\n\nKeep 2*3*4, 5 * 3 * 2, snake_case_name, and **unclosed.\n\n[**Read** this](https://example.com/a_b_c*d*)")
	if err != nil {
		t.Fatal(err)
	}
	want := `<p>Include <strong>two responses</strong> and <em>one</em> <em>conclusion</em>.</p><ul><li><strong>Bold</strong> item</li></ul><p>Keep 2*3*4, 5 * 3 * 2, snake_case_name, and **unclosed.</p><p><a href="https://example.com/a_b_c*d*"><strong>Read</strong> this</a></p>`
	if got != want {
		t.Fatalf("got %s\nwant %s", got, want)
	}
}

func TestMarkdownHTMLRendersOnlyCourseAndWebLinks(t *testing.T) {
	got, err := MarkdownHTML("- [Silver supports](/courses/1/pages/hub-silver)\n\n[x](javascript:alert(1)) and [site](https://example.com/a?b=1&c=2)")
	if err != nil {
		t.Fatal(err)
	}
	want := `<ul><li><a href="/courses/1/pages/hub-silver">Silver supports</a></li></ul><p>[x](javascript:alert(1)) and <a href="https://example.com/a?b=1&amp;c=2">site</a></p>`
	if got != want {
		t.Fatalf("got %s\nwant %s", got, want)
	}
}

func TestHeaderLetsTheTeacherControlWhatLeadsTheComment(t *testing.T) {
	rubric := domain.Rubric{Criteria: []domain.Criterion{{ID: "a", Description: "Evidence", Points: domain.MustDecimal("4")}}}
	interpretation := domain.Interpretation{Mapping: "direct", Rationale: "private", Components: []domain.Component{{Key: "one", Label: "Response 1", CriterionIDs: []string{"a"}, Weight: domain.MustDecimal("1")}}}
	in := FeedbackInput{Rubric: rubric, Interpretation: interpretation, Components: []domain.ComponentScore{{Key: "one", Evidence: "present", Criteria: []domain.CriterionScore{{CriterionID: "a", Points: domain.MustDecimal("3")}}}}, Assessment: domain.MustDecimal("3"), AssignmentPoints: domain.MustDecimal("4"), Feedback: "32/40\nGreat work."}
	in.Header = "none"
	if got, err := RenderFeedback(in); err != nil || got != "32/40\nGreat work." {
		t.Fatalf("header none must post the feedback exactly as written: %q %v", got, err)
	}
	for _, h := range []string{"", "score"} {
		in.Header = h
		if got, err := RenderFeedback(in); err != nil || got != "Assessment: 3 / 4\n\n32/40\nGreat work." {
			t.Fatalf("header %q must lead with only the score line: %q %v", h, got, err)
		}
	}
}
