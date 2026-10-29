package mcpserver

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"canvasexpertse/internal/workflow"
)

func TestQuizPointsArgsExactScope(t *testing.T) {
	args, err := quizPointsArgs("ELA7", "https://canvas.example.edu/courses/1/assignments/2", []QuizPointChange{{2, "0.125"}, {4, "50"}})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"quiz-points", "ELA7", "https://canvas.example.edu/courses/1/assignments/2", "2=0.125", "4=50"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("got %q want %q", args, want)
	}
	for _, changes := range [][]QuizPointChange{{{0, "1"}}, {{1, "-1"}}, {{1, "NaN"}}, {{1, "1"}, {1, "2"}}} {
		if _, err := quizPointsArgs("ELA7", "2", changes); err == nil {
			t.Fatalf("accepted invalid changes: %+v", changes)
		}
	}
	if _, err := quizPointsArgs("", "2", nil); err == nil {
		t.Fatal("accepted missing course")
	}
	if _, err := quizPointsArgs("ELA7", "", nil); err == nil {
		t.Fatal("accepted missing quiz")
	}
}

func TestUploadArgsScopeAndOptions(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "courses", "ELA7"), 0700); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "courses", "ELA7", "asset.pdf")
	if err := os.WriteFile(source, []byte("synthetic"), 0600); err != nil {
		t.Fatal(err)
	}
	s := &Server{root: root}
	in := UploadInput{Course: "ELA7", Path: "courses/ELA7/asset.pdf", Access: "hidden", Receipt: "courses/ELA7/asset-receipt.json", FolderID: 15, NewUpload: true}
	args, err := s.uploadArgs(in)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"upload-file", "ELA7", source, "--access", "hidden", "--receipt", filepath.Join(root, "courses", "ELA7", "asset-receipt.json"), "--folder", "15", "--new-upload"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("got %q want %q", args, want)
	}
	for _, mutate := range []func(*UploadInput){
		func(v *UploadInput) { v.Course = "" },
		func(v *UploadInput) { v.Access = "public" },
		func(v *UploadInput) { v.FolderID = -1 },
		func(v *UploadInput) { v.Path = ".cse/students.csv" },
		func(v *UploadInput) { v.Path = "courses/ELA7" },
		func(v *UploadInput) { v.Receipt = "context/receipt.json" },
		func(v *UploadInput) { v.Receipt = "courses/../context/receipt.json" },
	} {
		bad := in
		mutate(&bad)
		if _, err := s.uploadArgs(bad); err == nil {
			t.Fatalf("accepted invalid upload: %+v", bad)
		}
	}
}

func TestValidateQuizLocal(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "courses"), 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "courses", "quiz.json")
	content := `{"title":"Synthetic","items":[{"id":"q1","type":"ESSAY","prompt":"Explain your reasoning.","points":5}]}`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	s := &Server{root: root}
	if _, err := s.validateQuiz("courses/quiz.json"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(content, `"title":"Synthetic"`, `"title":"Synthetic","metadata":{"canvas":{"allowed_atempts":2}}`, 1)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.validateQuiz("courses/quiz.json"); err == nil || !strings.Contains(err.Error(), "allowed_atempts") {
		t.Fatalf("setting typo passed local validation: %v", err)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(content, `"points":5`, `"points":5,"guessed_key":true`, 1)), 0600); err != nil {
		t.Fatal(err)
	}
	if result, err := s.validateQuiz("courses/quiz.json"); err != nil || !strings.Contains(result.Text, "Warning:") || !strings.Contains(result.Text, "guessed_key") {
		t.Fatalf("unknown item key warning missing: %+v %v", result, err)
	}
	if err := os.WriteFile(path, []byte(`{"title":"Invalid","items":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.validateQuiz("courses/quiz.json"); err == nil {
		t.Fatal("accepted quiz without items")
	}
	if _, err := s.validateQuiz(".cse/quiz.json"); err == nil {
		t.Fatal("accepted hidden file")
	}
}

func TestReplaceTextArgsKeepPairsInOrder(t *testing.T) {
	args, err := replaceTextArgs(ReplaceTextInput{Course: "ELA7", Target: "page", Query: "7", Replacements: []workflow.BodyReplacement{{Find: "Oct 3", Replace: "Oct 10", All: true}, {Find: "--push", Replace: ""}}, ExpectUpdatedAt: "2026-10-01T12:00:00Z", Push: true})
	want := []string{"replace-text", "ELA7", "page", "7", "--find", "Oct 3", "--replace", "Oct 10", "--all", "--find", "--push", "--replace", "", "--expect-updated-at", "2026-10-01T12:00:00Z", "--push"}
	if err != nil || !reflect.DeepEqual(args, want) {
		t.Fatalf("got %q, %v", args, err)
	}
	if _, err := replaceTextArgs(ReplaceTextInput{Course: "ELA7", Target: "quiz", Query: "7", Replacements: []workflow.BodyReplacement{{Find: "a", Replace: "b"}}}); err == nil {
		t.Fatal("accepted an unsupported target")
	}
}
