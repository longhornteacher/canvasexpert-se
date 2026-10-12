package workspace

import (
	"path/filepath"
	"strings"
	"testing"

	"canvasexpertse/internal/domain"
)

func TestSyncInstanceUpdatesStarterAndPreservesCurrentDocument(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "context", "courses.md")
	config := domain.CourseConfig{Schema: 1, Instance: "https://canvas.example.edu", Courses: []domain.Course{{Nickname: "CS", ID: 12, Current: true}}}
	body := "# Teacher notes\nKeep this text.\n"
	if err := WriteDocument(path, config, body); err != nil {
		t.Fatal(err)
	}
	changed, err := SyncInstance(root, "https://school.instructure.com/")
	if err != nil || !changed {
		t.Fatalf("sync = %v, %v", changed, err)
	}
	var got domain.CourseConfig
	gotBody, err := ReadDocument(path, &got)
	if err != nil || got.Instance != "https://school.instructure.com/" || len(got.Courses) != 1 || got.Courses[0].ID != 12 || gotBody != body {
		t.Fatalf("document not preserved: %#v body=%q err=%v", got, gotBody, err)
	}
}

func TestSyncInstanceDoesNotRebindConfiguredCourses(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "context", "courses.md")
	config := domain.CourseConfig{Schema: 1, Instance: "https://old.instructure.com", Courses: []domain.Course{{Nickname: "CS", ID: 12, Current: true}}}
	if err := WriteDocument(path, config, "# Keep\n"); err != nil {
		t.Fatal(err)
	}
	changed, err := SyncInstance(root, "https://new.instructure.com")
	var got domain.CourseConfig
	_, readErr := ReadDocument(path, &got)
	if err != nil || readErr != nil || changed || got.Instance != config.Instance {
		t.Fatalf("different origin rebound configured courses: changed=%v config=%#v err=%v read=%v", changed, got, err, readErr)
	}
}

func TestSyncInstanceRereadsDocumentAtWriteBoundary(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "context", "courses.md")
	if err := WriteDocument(path, domain.CourseConfig{Schema: 1, Instance: "https://old.instructure.com"}, "# Before setup\n"); err != nil {
		t.Fatal(err)
	}
	// Simulate a teacher edit made while the setup form was open.
	if err := WriteDocument(path, domain.CourseConfig{Schema: 1, Instance: "https://school.instructure.com", Courses: []domain.Course{{Nickname: "New edit", ID: 99, Current: true}}}, "# Edited during setup\n"); err != nil {
		t.Fatal(err)
	}
	changed, err := SyncInstance(root, "https://school.instructure.com")
	var got domain.CourseConfig
	body, readErr := ReadDocument(path, &got)
	if err != nil || readErr != nil || !changed || len(got.Courses) != 1 || got.Courses[0].ID != 99 || !strings.Contains(body, "Edited during setup") {
		t.Fatalf("concurrent edit lost: changed=%v config=%#v body=%q err=%v read=%v", changed, got, body, err, readErr)
	}
}

func TestSyncInstanceReportsUnreadableConfiguration(t *testing.T) {
	root := t.TempDir()
	if _, err := SyncInstance(root, "https://school.instructure.com"); err == nil {
		t.Fatal("expected configuration read failure")
	}
}
