package workflow

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

func TestDistributedDiscussionCreateScopesGroupSetOnTopicAndPullsBaseline(t *testing.T) {
	var methods []string
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "GET /api/v1/group_categories/50":
			json.NewEncoder(w).Encode(map[string]any{"id": 50, "course_id": 1, "context_type": "Course", "non_collaborative": false})
		case "POST /api/v1/courses/1/discussion_topics":
			methods = append(methods, r.Method+" "+r.URL.Path)
			var req struct {
				Topic map[string]any `json:"discussion_topic"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			if req.Topic["group_category_id"] != float64(50) || req.Topic["published"] != false {
				t.Errorf("unexpected distributed topic fields: %#v", req.Topic)
			}
			json.NewEncoder(w).Encode(map[string]any{"id": 77, "title": "Common prompt", "message": "<p>Read and respond</p>", "group_category_id": 50, "group_topic_children": []any{map[string]any{"group_id": 60, "id": 78}}})
		case "PUT /api/v1/courses/1/discussion_topics/77":
			methods = append(methods, r.Method+" "+r.URL.Path)
			var req struct {
				Topic map[string]any `json:"discussion_topic"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			if req.Topic["published"] != true {
				t.Errorf("publication payload=%#v", req.Topic)
			}
			_, _ = w.Write([]byte("not-json"))
		case "GET /api/v1/courses/1/discussion_topics/77":
			json.NewEncoder(w).Encode(map[string]any{"id": 77, "title": "Common prompt", "message": "<p>Read and respond</p>", "group_category_id": 50, "discussion_type": "threaded", "published": false})
		case "GET /api/v1/group_categories/50/groups":
			json.NewEncoder(w).Encode([]any{map[string]any{"id": 60, "course_id": 1, "group_category_id": 50, "context_type": "Course", "non_collaborative": false}})
		case "GET /api/v1/courses/1/enrollments":
			json.NewEncoder(w).Encode([]any{})
		case "GET /api/v1/groups/60/discussion_topics/78":
			json.NewEncoder(w).Encode(map[string]any{"id": 78, "title": "Club prompt", "message": "Club thread"})
		case "GET /api/v1/groups/60/discussion_topics/78/view":
			json.NewEncoder(w).Encode(map[string]any{"view": []any{}, "new_entries": []any{}})
		default:
			t.Errorf("unexpected Canvas request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	})
	path := filepath.Join(s.Root, "draft.md")
	d := domain.DiscussionDocument{Schema: 1, Kind: "discussion", Operation: "create", Course: "TEST", GroupCategoryID: 50, Title: "Common prompt", BodyFormat: "html"}
	published := true
	d.Published = &published
	if err := workspace.WriteDocument(path, &d, "<p>Read and respond</p>"); err != nil {
		t.Fatal(err)
	}
	report, err := s.PushDiscussion(context.Background(), path)
	if err != nil || report.Status != "completed" || report.TopicID != 77 || len(methods) != 2 || methods[0] != "POST /api/v1/courses/1/discussion_topics" || methods[1] != "PUT /api/v1/courses/1/discussion_topics/77" {
		t.Fatalf("report=%+v methods=%v err=%v", report, methods, err)
	}
	var saved domain.DiscussionDocument
	if _, err = workspace.ReadDocument(path, &saved); err != nil || saved.Created.GroupTopicChildren[0] != (domain.DiscussionChild{GroupID: 60, TopicID: 78}) || saved.Created.Publication.Status != "succeeded" || !saved.Created.Complete {
		t.Fatalf("child identities not persisted: %+v %v", saved.Created, err)
	}
	pulled, err := s.PullDiscussion(context.Background(), "TEST", 77, 0)
	if err != nil {
		t.Fatal(err)
	}
	var baseline domain.DiscussionDocument
	body, err := workspace.ReadDocument(pulled, &baseline)
	if err != nil || body != "<p>Read and respond</p>" || baseline.GroupCategoryID != 50 || baseline.Observed == nil {
		t.Fatalf("bad pulled discussion %q %+v %v", body, baseline, err)
	}
}

func TestUncertainDiscussionCreateDistinguishesAnnouncementFromDiscussion(t *testing.T) {
	announcement := true
	d := domain.DiscussionDocument{Schema: 1, Kind: "discussion", Operation: "create", Course: "TEST", Title: "Schedule update", BodyFormat: "html", IsAnnouncement: &announcement, AnnouncementIntent: "Share the schedule update with the club members."}
	posts := 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "POST /api/v1/courses/1/discussion_topics":
			posts++
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]any{})
		case "GET /api/v1/courses/1/discussion_topics":
			now := time.Now().UTC().Format(time.RFC3339Nano)
			json.NewEncoder(w).Encode([]map[string]any{
				{"id": 90, "title": "Schedule update", "message": "<p>Club news</p>", "is_announcement": false, "created_at": now},
				{"id": 91, "title": "Schedule update", "message": "<p>Club news</p>", "is_announcement": true, "created_at": now},
			})
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	})
	path := filepath.Join(s.Root, "announcement.md")
	if err := workspace.WriteDocument(path, &d, "<p>Club news</p>"); err != nil {
		t.Fatal(err)
	}
	report, err := s.PushDiscussion(context.Background(), path)
	if err != nil || report.Status != "uncertain" {
		t.Fatalf("first report=%+v err=%v", report, err)
	}
	report, err = s.PushDiscussion(context.Background(), path)
	if err != nil || report.Status != "completed" || report.TopicID != 91 || posts != 1 {
		t.Fatalf("recovery report=%+v posts=%d err=%v", report, posts, err)
	}
}

func TestAcceptedDiscussionCreateWithoutIDIsHeldAndNeverReplayed(t *testing.T) {
	posts, lists := 0, 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "POST" {
			posts++
			_, _ = w.Write([]byte("not-json"))
			return
		}
		if r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/discussion_topics" {
			lists++
			json.NewEncoder(w).Encode([]any{})
			return
		}
		t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		http.NotFound(w, r)
	})
	path := filepath.Join(s.Root, "accepted-no-id.md")
	d := domain.DiscussionDocument{Schema: 1, Kind: "discussion", Operation: "create", Course: "TEST", Title: "No ID", BodyFormat: "html"}
	if err := workspace.WriteDocument(path, &d, "<p>Body</p>"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		report, err := s.PushDiscussion(context.Background(), path)
		if err != nil || report.Status != "held" {
			t.Fatalf("run %d report=%+v err=%v", i, report, err)
		}
	}
	if posts != 1 || lists != 0 {
		t.Fatalf("accepted write without ID was replayed or read back: posts=%d lists=%d", posts, lists)
	}
}

func TestDiscussionSettingsOnlyUpdateIgnoresChangedBodyAndPreservesIt(t *testing.T) {
	pinned := true
	base := domain.DiscussionSnapshot{CourseID: 1, TopicID: 5, Title: "Prompt", BodyHTML: "<p>Old</p>"}
	d := domain.DiscussionDocument{Schema: 1, Kind: "discussion", Operation: "update", Course: "TEST", CourseID: 1, TopicID: 5, Title: "Prompt", BodyFormat: "html", Pinned: &pinned, Observed: &base}
	var patch map[string]any
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" {
			json.NewEncoder(w).Encode(map[string]any{"id": 5, "title": "Prompt", "message": "<p>New student reply context</p>", "pinned": false})
			return
		}
		var req struct {
			Topic map[string]any `json:"discussion_topic"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		patch = req.Topic
		_, _ = w.Write([]byte("not-json"))
	})
	base.Instance = s.Client.Origin()
	d.Instance = base.Instance
	d.Observed = &base
	path := filepath.Join(s.Root, "update.md")
	if err := workspace.WriteDocument(path, &d, "<p>Old</p>"); err != nil {
		t.Fatal(err)
	}
	r, err := s.PushDiscussion(context.Background(), path)
	if err != nil || r.Status != "succeeded" || len(patch) != 1 || patch["pinned"] != true {
		t.Fatalf("report=%+v patch=%#v err=%v", r, patch, err)
	}
}
