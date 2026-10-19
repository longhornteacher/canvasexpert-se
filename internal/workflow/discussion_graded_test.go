package workflow

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

func gradedDiscussionDoc(published bool) domain.DiscussionDocument {
	due := "2026-11-06T23:59:00-06:00"
	rubric := &domain.Rubric{Title: "Literary discussion", Criteria: []domain.Criterion{{ID: "evidence", Description: "Interpretation supported with evidence", Points: domain.MustDecimal("100")}}}
	return domain.DiscussionDocument{Schema: 1, Kind: "discussion", Operation: "create", Course: "TEST", GroupCategoryID: 50, Title: "Week 1", BodyFormat: "html", Published: &published,
		Assignment: &domain.DiscussionAssignment{Points: domain.MustDecimal("100"), DueAt: &due, Rubric: rubric}}
}

func TestGradedDistributedDiscussionKeepsGroupSetOnTopicAndResumesRubricOnly(t *testing.T) {
	var topicPosts, rubricPosts, publishes int
	failRubric := true
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "GET /api/v1/group_categories/50":
			json.NewEncoder(w).Encode(map[string]any{"id": 50, "course_id": 1, "context_type": "Course", "non_collaborative": false})
		case "POST /api/v1/courses/1/discussion_topics":
			topicPosts++
			var req map[string]any
			_ = json.NewDecoder(r.Body).Decode(&req)
			linked, _ := req["assignment"].(map[string]any)
			if req["group_category_id"] != float64(50) || linked == nil || linked["group_category_id"] != nil || linked["points_possible"] != float64(100) || req["published"] != false {
				t.Errorf("group set must stay on the topic and the assignment individual: %#v", req)
			}
			json.NewEncoder(w).Encode(map[string]any{"id": 77, "assignment_id": 90, "title": "Week 1", "message": "<p>Respond</p>", "group_category_id": 50})
		case "GET /api/v1/courses/1/assignments/90":
			json.NewEncoder(w).Encode(map[string]any{"id": 90, "course_id": 1, "name": "Week 1", "points_possible": 100, "grading_type": "points", "submission_types": []string{"discussion_topic"}})
		case "POST /api/v1/courses/1/rubrics":
			rubricPosts++
			if failRubric {
				http.Error(w, `{"errors":"rejected"}`, 400)
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"rubric": map[string]any{"id": 5}, "rubric_association": map[string]any{"id": 6}})
		case "PUT /api/v1/courses/1/discussion_topics/77":
			publishes++
			_, _ = w.Write([]byte("{}"))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	})
	path := filepath.Join(s.Root, "graded.md")
	d := gradedDiscussionDoc(true)
	if err := workspace.WriteDocument(path, &d, "<p>Respond</p>"); err != nil {
		t.Fatal(err)
	}
	report, err := s.PushDiscussion(context.Background(), path)
	if err != nil || report.Status != "failed" || publishes != 0 || topicPosts != 1 {
		t.Fatalf("rubric failure must hold publication: %+v err=%v posts=%d publishes=%d", report, err, topicPosts, publishes)
	}
	failRubric = false
	report, err = s.PushDiscussion(context.Background(), path)
	if err != nil || report.Status != "completed" || topicPosts != 1 || rubricPosts != 2 || publishes != 1 {
		t.Fatalf("resume repeated finished work: %+v err=%v topic=%d rubric=%d publish=%d", report, err, topicPosts, rubricPosts, publishes)
	}
	var saved domain.DiscussionDocument
	if _, err = workspace.ReadDocument(path, &saved); err != nil || saved.Created.AssignmentID != 90 || saved.Created.RubricID != 5 || !saved.Created.Complete {
		t.Fatalf("saved identities: %+v err=%v", saved.Created, err)
	}
}

func TestGradedDiscussionValidationRejectsDirectGroupAnnouncementAndUpdate(t *testing.T) {
	d := gradedDiscussionDoc(false)
	d.GroupCategoryID, d.GroupID = 0, 60
	if err := d.ValidateBody("<p>x</p>"); err == nil {
		t.Fatal("direct group topic accepted a graded assignment")
	}
	d = gradedDiscussionDoc(false)
	yes := true
	d.IsAnnouncement, d.AnnouncementIntent = &yes, "Tell everyone"
	if err := d.ValidateBody("<p>x</p>"); err == nil {
		t.Fatal("announcement accepted a graded assignment")
	}
	d = gradedDiscussionDoc(false)
	if err := d.ValidateBody("<p>x</p>"); err != nil {
		t.Fatalf("valid graded discussion rejected: %v", err)
	}
}
