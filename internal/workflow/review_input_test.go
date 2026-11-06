package workflow

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReviewInputKindUsesContents(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct{ example, kind string }{{"synthetic-assignment-create.md", "assignment"}, {"synthetic-results-two.md", "results"}, {"synthetic-results-feedback-only.md", "results"}} {
		data, err := os.ReadFile(filepath.Join("..", "..", "engine-docs", "examples", tc.example))
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, tc.kind+".txt")
		if err = os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		if kind, err := ReviewInputKind(path); err != nil || kind != tc.kind {
			t.Fatalf("%s: %s %v", tc.kind, kind, err)
		}
	}
	quiz := filepath.Join(root, "enveloped.txt")
	data := `Teacher draft
<QUIZFORGE_JSON>{"title":"Synthetic","items":[{"id":"q1","type":"ESSAY","prompt":"Explain.","points":2}]}</QUIZFORGE_JSON>`
	if err := os.WriteFile(quiz, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	if kind, err := ReviewInputKind(quiz); err != nil || kind != "quiz" {
		t.Fatalf("%s %v", kind, err)
	}
}
