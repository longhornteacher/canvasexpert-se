package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

func TestReviewInputContentAndOptionalCourse(t *testing.T) {
	root := t.TempDir()
	config := domain.CourseConfig{Schema: 1, Instance: "https://canvas.example.edu", Courses: []domain.Course{{Nickname: "DEMO", ID: 11}, {Nickname: "OTHER", ID: 12}}}
	if err := workspace.WriteDocument(filepath.Join(root, "context", "courses.md"), config, ""); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ example, kind string }{{"synthetic-assignment-create.md", "assignment"}, {"synthetic-results-two.md", "results"}, {"synthetic-results-feedback-only.md", "results"}} {
		data, err := os.ReadFile(filepath.Join("..", "..", "engine-docs", "examples", tc.example))
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, tc.kind+".txt")
		if err = os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		for _, course := range []string{"", "demo"} {
			if kind, err := ValidateReviewInput(root, path, course); err != nil || kind != tc.kind {
				t.Fatalf("%s course %q: %s %v", tc.kind, course, kind, err)
			}
		}
		if _, err = ValidateReviewInput(root, path, "OTHER"); err == nil || !strings.Contains(err.Error(), "differs") {
			t.Fatalf("wrong course accepted: %v", err)
		}
	}
	quiz := filepath.Join(root, "enveloped.txt")
	data := `Teacher draft
<QUIZFORGE_JSON>{"title":"Synthetic","items":[{"id":"q1","type":"ESSAY","prompt":"Explain.","points":2}]}</QUIZFORGE_JSON>`
	if err := os.WriteFile(quiz, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	if kind, err := ValidateReviewInput(root, quiz, "DEMO"); err != nil || kind != "quiz" {
		t.Fatalf("%s %v", kind, err)
	}
	if _, err := ValidateReviewInput(root, quiz, ""); err == nil || !strings.Contains(err.Error(), "explicit course") {
		t.Fatalf("missing quiz course accepted: %v", err)
	}
}
