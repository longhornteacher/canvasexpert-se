package localui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"canvasexpertse/internal/workflow"
)

func TestStudentReportDocumentPutsEachStudentOnItsOwnPage(t *testing.T) {
	doc, err := studentReportHTML([]workflow.StudentSheet{{Name: "A", HTML: "<p>one</p>"}, {Name: "B", HTML: "<p>two</p>"}})
	if err != nil {
		t.Fatal(err)
	}
	page := string(doc)
	if strings.Count(page, `<section class="sheet">`) != 2 || !strings.Contains(page, "page-break-after:always") || !strings.Contains(page, "<p>two</p>") {
		t.Fatal(page)
	}
}

func TestStudentReportNeverOverwritesAnEarlierFile(t *testing.T) {
	dir := t.TempDir()
	first := uniquePath(dir, "Student reports CS8", ".pdf")
	if err := os.WriteFile(first, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := uniquePath(dir, "Student reports CS8", ".pdf"); got != filepath.Join(dir, "Student reports CS8 (2).pdf") {
		t.Fatal(got)
	}
}

func TestDraftHandoutKeepsOnlyWhatStudentsSee(t *testing.T) {
	quiz := workflow.ReviewModel{Kind: "New Quiz", Title: "Ch 3 Quiz", AllOrNothing: true, Notice: "not a simulation", Cards: []workflow.ReviewCard{
		{Ref: "quiz", Alias: "Ch 3 Quiz", Total: "4 points, 1 questions", Facts: []workflow.ReviewFact{{Label: "Time limit", Value: "none"}}},
		{Ref: "item-1", Alias: "1. Main idea", Total: "4 pts", Key: "SECRET KEY", Frames: []workflow.ReviewFrame{{Label: "Question", HTML: "<p>What is the main idea?</p>"}, {Label: "Rationales", HTML: "<p>RATIONALE</p>"}}},
	}}
	doc, err := draftHTML(quiz, true, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	page := string(doc)
	for _, hidden := range []string{"SECRET KEY", "RATIONALE", "Time limit", "not a simulation", "Draft ·"} {
		if strings.Contains(page, hidden) {
			t.Fatalf("handout shows %q:\n%s", hidden, page)
		}
	}
	if !strings.Contains(page, "<p>What is the main idea?</p>") || !strings.Contains(page, "default-src 'none'") {
		t.Fatal(page)
	}
	teacher, _ := draftHTML(quiz, false, time.Now())
	for _, shown := range []string{"SECRET KEY", "RATIONALE", "Time limit", "Draft ·"} {
		if !strings.Contains(string(teacher), shown) {
			t.Fatalf("teacher copy lacks %q", shown)
		}
	}
}

func TestDraftScoresPutEachStudentOnAPageAndSkipHeldInHandout(t *testing.T) {
	scores := workflow.ReviewModel{Kind: "Assignment scores", Title: "Essay 1", Cards: []workflow.ReviewCard{
		{Ref: "a", Alias: "Ana", Total: "9 / 10", Comment: "Overall: 9/10", TeacherNote: "PRIVATE NOTE"},
		{Ref: "b", Alias: "Ben", Total: "7 / 10", Comment: "Overall: 7/10"},
		{Ref: "c", Alias: "Cal", Hold: "file unreadable"},
	}}
	teacher, _ := draftHTML(scores, false, time.Now())
	if strings.Count(string(teacher), `class="card page"`) != 2 || !strings.Contains(string(teacher), "Will not post: file unreadable") || !strings.Contains(string(teacher), "PRIVATE NOTE") {
		t.Fatal(string(teacher))
	}
	handout, _ := draftHTML(scores, true, time.Now())
	if strings.Contains(string(handout), "Cal") || strings.Contains(string(handout), "PRIVATE NOTE") || strings.Count(string(handout), `class="card page"`) != 1 {
		t.Fatal(string(handout))
	}
}

func TestDraftPrintsHTMLDirectionsInsteadOfEscapedSource(t *testing.T) {
	m := workflow.ReviewModel{Title: "Lab", AllOrNothing: true, Cards: []workflow.ReviewCard{{Alias: "Lab", Frames: []workflow.ReviewFrame{{Label: "Directions", HTML: "<pre>&lt;table&gt;</pre>", PrintHTML: "<table><tr><td>cell</td></tr></table>"}}}}}
	doc, _ := draftHTML(m, true, time.Now())
	if !strings.Contains(string(doc), "<td>cell</td>") || strings.Contains(string(doc), "&lt;table&gt;") {
		t.Fatal(string(doc))
	}
}
