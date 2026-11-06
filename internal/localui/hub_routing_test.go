package localui

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

// A course whose list or owner cannot be read must never be fetched with your own key.
func TestClientForCourseFailsClosedWhenCoursesCannotBeRead(t *testing.T) {
	f := newTeamFixture(t)
	if err := os.Remove(filepath.Join(f.root, "context", "courses.md")); err != nil {
		t.Fatal(err)
	}
	c, err := f.h.clientForCourse(context.Background(), 701)
	if err == nil || c != nil {
		t.Fatalf("want an error and no client, got client=%v err=%v", c, err)
	}
}

func TestClientForCourseFailsClosedWhenOwnerIsNotOnTheTeam(t *testing.T) {
	f := newTeamFixture(t)
	config := domain.CourseConfig{Schema: 1, Instance: "https://school.example", Courses: []domain.Course{{ID: 701, Nickname: "T2", Current: true, Owner: "turner"}}}
	if err := workspace.WriteDocument(filepath.Join(f.root, "context", "courses.md"), config, "Course notes."); err != nil {
		t.Fatal(err)
	}
	c, err := f.h.clientForCourse(context.Background(), 701)
	if err == nil || c != nil {
		t.Fatalf("want an error and no client, got client=%v err=%v", c, err)
	}
}

func TestClientForCourseStillServesYourOwnCourse(t *testing.T) {
	f := newTeamFixture(t)
	c, err := f.h.clientForCourse(context.Background(), 11)
	if err != nil || c == nil {
		t.Fatalf("own course: %v", err)
	}
}
