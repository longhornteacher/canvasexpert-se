package mcpserver

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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
		if saved.Team == nil || len(saved.Team) != 0 || !strings.Contains(result.Text, `"team":[]`) {
			t.Fatalf("an empty team must be []: %s", result.Text)
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

func TestListCoursesShowsOwnerAndTeamWithoutKeys(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "context"), 0700); err != nil {
		t.Fatal(err)
	}
	courses := `---
schema: 1
instance: https://canvas.example.edu
courses:
  - nickname: Turner P2
    id: 2
    current: true
    owner: turner
---
`
	team := `---
schema: 1
teammates:
  - label: turner
    name: Ms. Turner
    canvas_user_id: 77
    email: turner@school.org
    key_expires: 2000-01-01
---
`
	for name, body := range map[string]string{"courses.md": courses, "team.md": team} {
		if err := os.WriteFile(filepath.Join(root, "context", name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	result, err := (&Server{root: root}).listCourses(context.Background(), ListCoursesInput{})
	if err != nil {
		t.Fatal(err)
	}
	var saved configuredCourses
	if err := json.Unmarshal([]byte(result.Text), &saved); err != nil {
		t.Fatal(err)
	}
	if len(saved.Courses) != 1 || saved.Courses[0].Owner != "turner" || len(saved.Team) != 1 || saved.Team[0].Label != "turner" || !saved.Team[0].ExpiresSoon || saved.Team[0].KeyOnThisPC {
		t.Fatalf("unexpected listing: %s", result.Text)
	}
	if strings.Contains(strings.ToLower(result.Text), "token") {
		t.Fatalf("listing mentions a token: %s", result.Text)
	}
}
