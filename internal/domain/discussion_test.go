package domain

import (
	"strings"
	"testing"
)

func TestDiscussionValidationScopesAnnouncementsAndGradedHold(t *testing.T) {
	d := DiscussionDocument{Schema: 1, Kind: "discussion", Operation: "create", Course: "ELA7", Title: "Week one", BodyFormat: "html"}
	if err := d.ValidateBody("<p>Prompt</p>"); err != nil {
		t.Fatal(err)
	}
	d.GroupID = 7
	d.GroupCategoryID = 8
	if err := d.ValidateBody("<p>Prompt</p>"); err == nil {
		t.Fatal("accepted mixed discussion scopes")
	}
	d.GroupCategoryID = 0
	d.IsAnnouncement = boolPtr(true)
	if err := d.ValidateBody("<p>Prompt</p>"); err == nil || !strings.Contains(err.Error(), "announcement_intent") {
		t.Fatalf("announcement without intent: %v", err)
	}
	d.AnnouncementIntent = "Share this update with club 7 members"
	assignment := map[string]any{"points": 100, "grading_type": "points", "rubric": map[string]any{"criteria": []any{}}}
	d.Assignment = &assignment
	if err := d.ValidateBody("<p>Prompt</p>"); err == nil || !strings.Contains(err.Error(), "not yet supported") {
		t.Fatalf("graded discussion was not clearly held: %v", err)
	}
}

func TestPulledAnnouncementBaselineCanBeReviewedWithoutInventedIntent(t *testing.T) {
	announcement := true
	d := DiscussionDocument{
		Schema: 1, Kind: "discussion", Operation: "update", Course: "ELA7", CourseID: 1, TopicID: 5,
		Title: "Schedule update", BodyFormat: "html", Instance: "https://canvas.example",
		IsAnnouncement: &announcement,
		Observed:       &DiscussionSnapshot{Instance: "https://canvas.example", CourseID: 1, TopicID: 5, Title: "Schedule update", IsAnnouncement: true},
	}
	if err := d.ValidateBody("<p>Updated schedule</p>"); err != nil {
		t.Fatalf("pulled announcement baseline without invented intent rejected: %v", err)
	}
	d.IsAnnouncement = nil
	if err := d.ValidateBody("<p>Updated schedule</p>"); err != nil {
		t.Fatalf("omitted announcement flag on baseline rejected: %v", err)
	}
	d.AnnouncementIntent = "Tell club members the meeting date changed."
	if err := d.ValidateBody("<p>Updated schedule</p>"); err != nil {
		t.Fatalf("explicit announcement update rejected: %v", err)
	}
}

func boolPtr(v bool) *bool { return &v }
