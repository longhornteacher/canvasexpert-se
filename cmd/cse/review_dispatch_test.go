package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReviewRejectsWrongCourseAndMissingQuizCourseBeforeCredentials(t *testing.T) {
	root := t.TempDir()
	data, err := os.ReadFile(filepath.Join("..", "..", "engine-docs", "examples", "synthetic-assignment-create.md"))
	if err != nil {
		t.Fatal(err)
	}
	assignment := filepath.Join(root, "assignment.txt")
	if err = os.WriteFile(assignment, data, 0600); err != nil {
		t.Fatal(err)
	}
	quiz := filepath.Join(root, "quiz.md")
	if err = os.WriteFile(quiz, []byte(`{"title":"Synthetic","items":[{"id":"q1","type":"ESSAY","prompt":"Explain.","points":2}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		args []string
		want string
	}{{[]string{assignment, "OTHER"}, "differs from document course"}, {[]string{quiz}, "explicit course"}} {
		var out, progress bytes.Buffer
		if code := runRemote(context.Background(), root, "review", tc.args, &out, &progress); code != 2 || !strings.Contains(progress.String(), tc.want) {
			t.Fatalf("%d: %s", code, progress.String())
		}
	}
}
