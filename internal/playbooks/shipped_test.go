package playbooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestUpdateKeepsTeacherChoicesAndAdoptsUneditedSeeds(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC)
	states := func(r UpdateReport) map[string]string {
		m := map[string]string{}
		for _, f := range r.Files {
			m[f.Path] = f.State
		}
		return m
	}
	first, err := Update(root, now)
	if err != nil {
		t.Fatal(err)
	}
	if first.Previous != "" || states(first)["playbooks/score.md"] != "added" || states(first)[overrideFile] != "added" {
		t.Fatalf("first install should add every seed: %+v", first)
	}
	if b, err := os.ReadFile(filepath.Join(root, shippedDir, first.Current, "playbooks", "score.md")); err != nil || len(b) == 0 {
		t.Fatalf("shipped baseline not kept: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, shippedDir, first.Current, overrideFile)); !os.IsNotExist(err) {
		t.Fatal("teacher-owned files are not shipped baselines")
	}

	// Simulate an older shipped version: score.md is unedited, edit.md has a
	// teacher edit, and the override file holds a teacher choice.
	write := func(rel, body string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("shipped/old/playbooks/score.md", "old score seed")
	write("playbooks/score.md", "old score seed")
	write("shipped/old/playbooks/edit.md", "old edit seed")
	write("playbooks/edit.md", "old edit seed, plus my own rule")
	write(overrideFile, "Always skip review when I ask to post.")
	write("shipped/adopted.txt", "old\n")

	second, err := Update(root, now)
	if err != nil {
		t.Fatal(err)
	}
	got := states(second)
	if second.Previous != "old" || got["playbooks/score.md"] != "updated" || got["playbooks/edit.md"] != "kept" || got[overrideFile] != "teacher-owned" || got["TEACHER.md"] != "current" {
		t.Fatalf("unexpected update states: %+v", second)
	}
	if b, _ := os.ReadFile(filepath.Join(root, "playbooks", "edit.md")); string(b) != "old edit seed, plus my own rule" {
		t.Fatal("teacher-edited playbook was replaced")
	}
	if b, _ := os.ReadFile(filepath.Join(root, overrideFile)); string(b) != "Always skip review when I ask to post." {
		t.Fatal("override file was replaced")
	}
	if seed, _ := SeedBytes("playbooks/score.md"); func() bool { b, _ := os.ReadFile(filepath.Join(root, "playbooks", "score.md")); return string(b) != string(seed) }() {
		t.Fatal("unedited playbook did not adopt the new seed")
	}
	if old, _ := filepath.Glob(filepath.Join(root, "playbook-old", "*", "playbooks", "score.md")); len(old) != 1 {
		t.Fatal("replaced playbook was not archived")
	}
	if b, _ := os.ReadFile(filepath.Join(root, shippedDir, "adopted.txt")); strings.TrimSpace(string(b)) != second.Current {
		t.Fatal("adopted version not recorded")
	}

	// With no record of what shipped, every difference is the teacher's choice.
	if err := os.Remove(filepath.Join(root, shippedDir, "adopted.txt")); err != nil {
		t.Fatal(err)
	}
	write("playbooks/score.md", "old score seed")
	third, err := Update(root, now)
	if err != nil {
		t.Fatal(err)
	}
	if states(third)["playbooks/score.md"] != "kept" {
		t.Fatalf("difference without a baseline must be kept: %+v", third)
	}
}

func TestUpdateTreatsLineEndingOnlyDifferencesAsCurrent(t *testing.T) {
	root := t.TempDir()
	if _, err := Update(root, time.Now()); err != nil {
		t.Fatal(err)
	}
	seed, _ := SeedBytes("playbooks/score.md")
	crlf := []byte(strings.ReplaceAll(strings.ReplaceAll(string(seed), "\r\n", "\n"), "\n", "\r\n"))
	if err := os.WriteFile(filepath.Join(root, "playbooks", "score.md"), crlf, 0644); err != nil {
		t.Fatal(err)
	}
	r, err := Update(root, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range r.Files {
		if f.Path == "playbooks/score.md" && f.State != "current" {
			t.Fatalf("line-ending-only difference reported %s", f.State)
		}
	}
}
