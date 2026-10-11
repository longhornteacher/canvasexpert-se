package mcpserver

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestConfiguredCoursesEmptyAndInvalidSource(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "context"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "context", "courses.md"), []byte("---\nschema: 1\ninstance: https://canvas.example.edu\ncourses: []\n---\n"), 0600); err != nil {
		t.Fatal(err)
	}
	s := &Server{root: root}
	for _, source := range []string{"", "configured"} {
		result, err := s.listCourses(context.Background(), ListCoursesInput{Source: source})
		if err != nil {
			t.Fatal(err)
		}
		var saved configuredCourses
		if err := json.Unmarshal([]byte(result.Text), &saved); err != nil {
			t.Fatal(err)
		}
		if saved.Courses == nil || len(saved.Courses) != 0 || saved.Source != "configured" || saved.Root != root {
			t.Fatalf("empty courses: %+v", saved)
		}
	}
	if _, err := s.listCourses(context.Background(), ListCoursesInput{Source: "unknown"}); err == nil {
		t.Fatal("unknown source accepted")
	}
}

func TestMCPFirstLaunchAddsSeedWithoutOverwritingTeacherFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "context"), 0700); err != nil {
		t.Fatal(err)
	}
	config := []byte("---\nschema: 1\ninstance: https://canvas.example.edu\ncourses: []\n---\nTeacher course notes.")
	if err := os.WriteFile(filepath.Join(root, "context", "courses.md"), config, 0600); err != nil {
		t.Fatal(err)
	}
	edited := []byte("Teacher grading habits.")
	if err := os.WriteFile(filepath.Join(root, "context", "grading.md"), edited, 0600); err != nil {
		t.Fatal(err)
	}
	_, close := NewServer(context.Background(), root)
	defer close()
	for name, want := range map[string][]byte{"context/courses.md": config, "context/grading.md": edited} {
		got, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil || string(got) != string(want) {
			t.Fatalf("teacher file changed: %s", name)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "playbooks", "score.md")); err != nil {
		t.Fatal("missing playbook not added")
	}
}
