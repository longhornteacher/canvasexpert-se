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
	want := "Overall: 4.67/20\nResponse 1: 4.67/6.67\nResponse 2: 0/13.33\n\nResponse 1\n- Evidence 3/4\n- Reasoning 4/6\n\nResponse 2\n- Evidence 0/4\n- Reasoning 0/6\n\n" + feedback
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
	want := "Overall: 7/10\nPrompt A: 4/5\nPrompt B: 3/5\n\nPrompt A\n- Read <carefully> 4/5\n\nPrompt B\n- Write 3/5\n\n<script>alert(1)</script>"
	if got != want {
		t.Fatalf("plain text criteria or exact feedback preservation incorrect:\n got: %s\nwant: %s", got, want)
	}
}

func TestRenderCommentOnlyOmitsAggregateAssessment(t *testing.T) {
	rubric := domain.Rubric{Title: "R", Criteria: []domain.Criterion{{ID: "a", Description: "Evidence", Points: domain.MustDecimal("5")}}}
	interpretation := domain.Interpretation{Mapping: "repeated", Rationale: "private", Components: []domain.Component{{Key: "one", Label: "Response", Weight: domain.MustDecimal("1")}}}
	for _, header := range []string{"", "full"} {
		got, err := RenderFeedback(FeedbackInput{Rubric: rubric, Interpretation: interpretation, Components: []domain.ComponentScore{{Key: "one", Evidence: "present", Criteria: []domain.CriterionScore{{CriterionID: "a", Points: domain.MustDecimal("4")}}}}, Assessment: domain.MustDecimal("16"), AssignmentPoints: domain.MustDecimal("20"), Feedback: "Good evidence.", OmitAssessment: true, Header: header})
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(got, "Assessment:") || !strings.HasPrefix(got, "Response: 4/5\n\nResponse\n- Evidence 4/5\n\n") || !strings.HasSuffix(got, "\n\nGood evidence.") {
			t.Fatalf("header %q: comment-only text included an aggregate grade or lost feedback: %q", header, got)
		}
	}
}

func TestRenderNewQuizLeadsWithAutoScoredSubtotal(t *testing.T) {
	rubric := domain.Rubric{Title: "R", Criteria: []domain.Criterion{{ID: "a", Description: "Evidence", Points: domain.MustDecimal("5")}}}
	interpretation := domain.Interpretation{Mapping: "repeated", Rationale: "private", Components: []domain.Component{{Key: "one", Label: "Response", Weight: domain.MustDecimal("1")}}}
	auto := &domain.AutoScored{Earned: domain.MustDecimal("6.50"), Possible: domain.MustDecimal("9.00"), Items: 2}
	in := FeedbackInput{Rubric: rubric, Interpretation: interpretation, Components: []domain.ComponentScore{{Key: "one", Evidence: "present", Criteria: []domain.CriterionScore{{CriterionID: "a", Points: domain.MustDecimal("4")}}}}, Assessment: domain.MustDecimal("16"), AssignmentPoints: domain.MustDecimal("20"), Feedback: "Good evidence.", OmitAssessment: true, AutoScored: auto, PartPoints: map[string]domain.Decimal{"one": domain.MustDecimal("15.0")}}
	for _, header := range []string{"", "full", "score"} {
		in.Header = header
		got, err := RenderFeedback(in)
		if err != nil {
			t.Fatal(err)
		}
		if got != "Questions Canvas scored: 6.5/9\nResponse: 12/15\n\nResponse\n- Evidence 4/5\n\nGood evidence." {
			t.Fatalf("header %q: %q", header, got)
		}
	}
	in.Header = "none"
	if got, _ := RenderFeedback(in); got != "Good evidence." {
		t.Fatalf("header none must post the feedback exactly: %q", got)
	}
}

func TestRenderItemFeedbackBlocksAndExemplarThreshold(t *testing.T) {
	rubric := domain.Rubric{Title: "R", Criteria: []domain.Criterion{{ID: "a", Description: "Explanation", Points: domain.MustDecimal("10")}}}
	interpretation := domain.Interpretation{Mapping: "repeated", Rationale: "private", Components: []domain.Component{{Key: "q11", Label: "Q11", Weight: domain.MustDecimal("1")}, {Key: "q12", Label: "Q12", Weight: domain.MustDecimal("1")}}}
	part := func(key, points, feedback string) domain.ComponentScore {
		return domain.ComponentScore{Key: key, Evidence: "present", Criteria: []domain.CriterionScore{{CriterionID: "a", Points: domain.MustDecimal(points)}}, Feedback: feedback, Exemplar: "A locked backup keeps a clean copy."}
	}
	auto := &domain.AutoScored{Earned: domain.MustDecimal("54"), Possible: domain.MustDecimal("60"), Items: 10}
	// 9/10 is exactly 90%, so it gets no exemplar; 8.9/10 is under and does.
	in := FeedbackInput{Rubric: rubric, Interpretation: interpretation, Components: []domain.ComponentScore{part("q11", "9", "Clear analogy."), part("q12", "8.9", "Name the second effect.")}, Assessment: domain.MustDecimal("0"), AssignmentPoints: domain.MustDecimal("60"), OmitAssessment: true, AutoScored: auto, PartPoints: map[string]domain.Decimal{"q11": domain.MustDecimal("12"), "q12": domain.MustDecimal("14")}}
	computed, err := domain.ComputeAssessment(rubric, interpretation, in.Components, in.AssignmentPoints)
	if err != nil {
		t.Fatal(err)
	}
	in.Assessment = computed.Posted
	got, err := RenderFeedback(in)
	if err != nil {
		t.Fatal(err)
	}
	want := "Questions Canvas scored: 54/60\n\nQ11 10.8/12\nClear analogy.\n\nQ12 12.46/14\nName the second effect. (Hand-copy exemplar: A locked backup keeps a clean copy.)"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	in.Header = "none"
	if _, err := RenderFeedback(in); err == nil {
		t.Fatal("header none with only per-part feedback must ask for the closing feedback")
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
	if text := HTMLText(`<p>Use &amp; compare</p><ul><li>One</li><li>Two</li></ul>`); text != "Use & compare\n- One\n- Two" {
		t.Fatalf("unexpected HTML text %q", text)
	}
}

func TestHTMLTextKeepsCodeLinksAndImages(t *testing.T) {
	source := `<p>See <a href="https://example.test/guide">the guide</a> and <a href="https://example.test/x">https://example.test/x</a>.</p>` +
		`<pre><code>def f(x):
    if x &lt; 2:
        return 1</code></pre>` +
		`<p><img src="/files/9/preview" alt="Reference code"><img src="/files/10/preview"></p>` +
		`<table><tr><th>A</th><th>B</th></tr><tr><td>1</td><td>2</td></tr></table><script>alert(1)</script><p>End<br>line</p>`
	want := "See the guide (https://example.test/guide) and https://example.test/x.\n" +
		"```\ndef f(x):\n    if x < 2:\n        return 1\n```\n" +
		"[image: Reference code] [image]\nA | B\n1 | 2\nEnd\nline"
	if got := HTMLText(source); got != want {
		t.Fatalf("got %q\nwant %q", got, want)
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
	in.Header = "score"
	if got, err := RenderFeedback(in); err != nil || got != "Overall: 3/4\n\n32/40\nGreat work." {
		t.Fatalf("header score must lead with only the overall line: %q %v", got, err)
	}
	for _, h := range []string{"", "full"} {
		in.Header = h
		if got, err := RenderFeedback(in); err != nil || got != "Overall: 3/4\n\nResponse 1\n- Evidence 3/4\n\n32/40\nGreat work." {
			t.Fatalf("header %q must lead with the overall and item lines: %q %v", h, got, err)
		}
	}
	in.Header, in.Late = "score", true
	if got, err := RenderFeedback(in); err != nil || got != "Overall: 3/4 (late)\n\n32/40\nGreat work." {
		t.Fatalf("late work must mark the earned overall line: %q %v", got, err)
	}
}

func TestQuizItemsCommentLeadsWithItemScoreAndAddsExemplarBelowNinety(t *testing.T) {
	rubric := domain.Rubric{Criteria: []domain.Criterion{{ID: "c", Description: "Claim", Points: domain.MustDecimal("2")}}}
	interp := domain.Interpretation{Mapping: "repeated", Components: []domain.Component{{Key: "q9", Label: "Q9", Weight: domain.MustDecimal("1")}}}
	parts := map[string]domain.Decimal{"q9": domain.MustDecimal("10")}
	score := func(points string) []domain.ComponentScore {
		return []domain.ComponentScore{{Key: "q9", Evidence: "present", Criteria: []domain.CriterionScore{{CriterionID: "c", Points: domain.MustDecimal(points)}}, Feedback: "Clear claim. Next, add a reason.", Exemplar: "A model sentence."}}
	}
	if got := QuizItems(rubric, interp, score("1"), parts)["q9"]; got.Points.String() != "5.00" && got.Points.String() != "5" || got.Comment != "5/10\nClear claim. Next, add a reason.\nExemplar: A model sentence." {
		t.Fatalf("below 90%%: %#v", got)
	}
	if got := QuizItems(rubric, interp, score("2"), parts)["q9"]; got.Comment != "10/10\nClear claim. Next, add a reason." {
		t.Fatalf("full credit should have no exemplar: %#v", got)
	}
}
