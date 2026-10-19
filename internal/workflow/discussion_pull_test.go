package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

func TestPullDiscussionPacketMergesFullRepliesAndKeepsAuthorsPrivate(t *testing.T) {
	var replies []map[string]any
	for i := 100; i < 112; i++ {
		message := "reply"
		if i == 100 {
			message = "cached version"
		}
		replies = append(replies, map[string]any{"id": i, "user_id": 12, "parent_id": 1, "message": message, "created_at": "2026-10-01T12:00:00Z", "updated_at": "2026-10-01T12:01:00Z"})
	}
	viewReplies := append([]map[string]any(nil), replies[:10]...)
	updated := map[string]any{"id": 100, "user_id": 12, "parent_id": 1, "message": "newer version", "created_at": "2026-10-01T12:00:00Z", "updated_at": "2026-10-01T12:03:00Z"}
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "GET /api/v1/courses/1/discussion_topics/20":
			json.NewEncoder(w).Encode(map[string]any{"id": 20, "title": "Thread", "message": "Prompt", "assignment_id": 30})
		case "GET /api/v1/courses/1/discussion_topics/20/view":
			json.NewEncoder(w).Encode(map[string]any{"view": []any{map[string]any{"id": 1, "user_id": 12, "message": "<p>Opening post</p>", "created_at": "2026-10-01T11:00:00Z", "replies_count": 12, "has_more_replies": true, "replies": viewReplies}}, "new_entries": []any{updated}})
		case "GET /api/v1/courses/1/discussion_topics/20/entries/1/replies":
			json.NewEncoder(w).Encode(replies)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.String())
			http.NotFound(w, r)
		}
	})
	students, err := workspace.OpenStudents(s.Root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = students.Upsert(s.Client.Origin(), 12, "Private Real Name", "Private", "SIS-SECRET", "private@example.edu"); err != nil {
		t.Fatal(err)
	}
	if err = students.Save(); err != nil {
		t.Fatal(err)
	}
	pulled, err := s.PullDiscussion(context.Background(), "TEST", 20, 0)
	if err != nil {
		t.Fatal(err)
	}
	var baseline domain.DiscussionDocument
	body, err := workspace.ReadDocument(pulled, &baseline)
	if err != nil || body != "Prompt" || baseline.TopicID != 20 {
		t.Fatalf("baseline path=%s body=%q doc=%+v err=%v", pulled, body, baseline, err)
	}
	var packet domain.DiscussionPacket
	packetPath := filepath.Join(filepath.Dir(pulled), "discussion-packet.yaml")
	if _, err = workspace.ReadDocument(packetPath, &packet); err != nil {
		t.Fatal(err)
	}
	if packet.Kind != "discussion-packet" || packet.Status != "complete" || packet.AssignmentID != 30 || len(packet.Entries) != 13 {
		t.Fatalf("packet status=%s assignment=%d entries=%d", packet.Status, packet.AssignmentID, len(packet.Entries))
	}
	seen := map[int64]bool{}
	var merged domain.DiscussionEntry
	for _, entry := range packet.Entries {
		if seen[entry.EntryID] {
			t.Fatalf("duplicate source entry %d", entry.EntryID)
		}
		seen[entry.EntryID] = true
		if entry.EntryID == 100 {
			merged = entry
		}
	}
	if merged.HTML != "newer version" || merged.ParentEntryID != 1 || merged.StudentRef == "" || merged.Alias == "" || merged.AuthorKind != "student" {
		t.Fatalf("newest entry or mapped author missing: %+v", merged)
	}
	file, err := os.ReadFile(packetPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"Private Real Name", "SIS-SECRET", "private@example.edu", "user_id:"} {
		if bytes.Contains(file, []byte(secret)) {
			t.Fatalf("packet leaked %q", secret)
		}
	}
}

func TestAcceptedDiscussionReplyTargetsParentAndNeverReadsBackOrReplays(t *testing.T) {
	reads, parentReads, posts := 0, 0, 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/discussion_topics/20" {
			reads++
			json.NewEncoder(w).Encode(map[string]any{"id": 20, "title": "Thread", "message": "Prompt"})
			return
		}
		if r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/discussion_topics/20/entries/90/replies" {
			parentReads++
			json.NewEncoder(w).Encode([]any{})
			return
		}
		if r.Method == "POST" && r.URL.Path == "/api/v1/courses/1/discussion_topics/20/entries/90/replies" {
			posts++
			var request map[string]string
			_ = json.NewDecoder(r.Body).Decode(&request)
			if request["message"] != "<p>Teacher reply</p>" {
				t.Errorf("reply payload=%#v", request)
			}
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"id": 91})
			return
		}
		t.Errorf("unexpected request %s %s", r.Method, r.URL.String())
		http.NotFound(w, r)
	})
	path := filepath.Join(s.Root, "reply.md")
	doc := domain.DiscussionReplyDocument{Schema: 1, Kind: "discussion-reply", Operation: "create", Course: "TEST", ContextType: "Course", ContextID: 1, TopicID: 20, ParentEntryID: 90, BodyFormat: "html"}
	if err := workspace.WriteDocument(path, &doc, "<p>Teacher reply</p>"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		report, err := s.PushDiscussionReply(context.Background(), path)
		if err != nil || report.Status != "completed" {
			t.Fatalf("attempt %d report=%+v err=%v", i, report, err)
		}
	}
	var saved domain.DiscussionReplyDocument
	if _, err := workspace.ReadDocument(path, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Outcome.Status != "succeeded" || saved.Outcome.ID != 91 || posts != 1 || reads != 1 || parentReads != 1 {
		t.Fatalf("saved=%+v posts=%d topic reads=%d parent reads=%d", saved.Outcome, posts, reads, parentReads)
	}
}

func TestUncertainDiscussionReplyIsPersistedAndNeverReplayed(t *testing.T) {
	reads, posts := 0, 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/discussion_topics/20" {
			reads++
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"id": 20, "title": "Thread"})
			return
		}
		if r.Method == "POST" && r.URL.Path == "/api/v1/courses/1/discussion_topics/20/entries" {
			posts++
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Errorf("hijack: %v", err)
				return
			}
			_ = conn.Close() // simulate a timeout after Canvas may have accepted the request
			return
		}
		t.Errorf("unexpected request %s %s", r.Method, r.URL.String())
		http.NotFound(w, r)
	})
	path := filepath.Join(s.Root, "uncertain-reply.md")
	doc := domain.DiscussionReplyDocument{Schema: 1, Kind: "discussion-reply", Operation: "create", Course: "TEST", ContextType: "Course", ContextID: 1, TopicID: 20, BodyFormat: "markdown"}
	if err := workspace.WriteDocument(path, &doc, "A teacher reply"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		report, err := s.PushDiscussionReply(context.Background(), path)
		if err != nil || report.Status != "uncertain" {
			t.Fatalf("attempt %d report=%+v err=%v", i, report, err)
		}
	}
	var saved domain.DiscussionReplyDocument
	if _, err := workspace.ReadDocument(path, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Outcome.Status != "uncertain" || posts != 1 || reads != 1 {
		t.Fatalf("outcome=%+v posts=%d reads=%d", saved.Outcome, posts, reads)
	}
}

func TestCrossTopicDiscussionParentIsRefusedBeforeReplyAttempt(t *testing.T) {
	posts, parents := 0, 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/discussion_topics/20" {
			json.NewEncoder(w).Encode(map[string]any{"id": 20, "title": "Thread", "message": "Prompt"})
			return
		}
		if r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/discussion_topics/20/entries/99/replies" {
			parents++
			http.NotFound(w, r)
			return
		}
		if r.Method == "POST" {
			posts++
		}
		t.Errorf("unexpected request %s %s", r.Method, r.URL.String())
		http.NotFound(w, r)
	})
	path := filepath.Join(s.Root, "cross-topic-reply.md")
	doc := domain.DiscussionReplyDocument{Schema: 1, Kind: "discussion-reply", Operation: "create", Course: "TEST", ContextType: "Course", ContextID: 1, TopicID: 20, ParentEntryID: 99, BodyFormat: "markdown"}
	if err := workspace.WriteDocument(path, &doc, "Reply"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PushDiscussionReply(context.Background(), path); err == nil {
		t.Fatal("cross-topic parent was accepted")
	}
	var saved domain.DiscussionReplyDocument
	if _, err := workspace.ReadDocument(path, &saved); err != nil {
		t.Fatal(err)
	}
	if posts != 0 || parents == 0 || saved.Outcome.Status != "" {
		t.Fatalf("posts=%d parent reads=%d outcome=%+v", posts, parents, saved.Outcome)
	}
}

func TestDistributedDiscussionPullKeepsIndependentCoverageAndSkipsWrongCourseChild(t *testing.T) {
	var wrongChildReads int
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "GET /api/v1/courses/1/discussion_topics/20":
			json.NewEncoder(w).Encode(map[string]any{"id": 20, "title": "Distributed", "message": "Prompt", "group_category_id": 50, "group_topic_children": []any{map[string]any{"group_id": 60, "id": 78}, map[string]any{"group_id": 61, "id": 79}, map[string]any{"group_id": 999, "id": 99}}})
		case "GET /api/v1/group_categories/50":
			json.NewEncoder(w).Encode(map[string]any{"id": 50, "course_id": 1, "context_type": "Course", "non_collaborative": false})
		case "GET /api/v1/group_categories/50/groups":
			json.NewEncoder(w).Encode([]any{map[string]any{"id": 60, "course_id": 1, "group_category_id": 50, "context_type": "Course", "non_collaborative": false}, map[string]any{"id": 61, "course_id": 1, "group_category_id": 50, "context_type": "Course", "non_collaborative": false}})
		case "GET /api/v1/courses/1/enrollments":
			json.NewEncoder(w).Encode([]any{})
		case "GET /api/v1/groups/60/discussion_topics/78":
			json.NewEncoder(w).Encode(map[string]any{"id": 78, "title": "Club A"})
		case "GET /api/v1/groups/60/discussion_topics/78/view":
			json.NewEncoder(w).Encode(map[string]any{"view": []any{map[string]any{"id": 5, "user_id": 700, "message": "A's written contribution"}}, "new_entries": []any{}})
		case "GET /api/v1/groups/61/discussion_topics/79":
			http.Error(w, "restricted", http.StatusForbidden)
		case "GET /api/v1/groups/999/discussion_topics/99":
			wrongChildReads++
			t.Errorf("wrong-course child route was read")
			http.NotFound(w, r)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.String())
			http.NotFound(w, r)
		}
	})
	pulled, err := s.PullDiscussion(context.Background(), "TEST", 20, 0)
	if err != nil {
		t.Fatal(err)
	}
	var packet domain.DiscussionPacket
	if _, err = workspace.ReadDocument(filepath.Join(filepath.Dir(pulled), "discussion-packet.yaml"), &packet); err != nil {
		t.Fatal(err)
	}
	if packet.Status != "partial" || wrongChildReads != 0 || len(packet.Sources) != 3 || len(packet.Entries) != 1 || packet.Entries[0].AuthorKind != "unknown" {
		t.Fatalf("packet=%+v wrong-child-reads=%d", packet, wrongChildReads)
	}
	if packet.Sources[0].Status != "complete" || packet.Sources[1].Status != "partial" || packet.Sources[2].Status != "partial" {
		t.Fatalf("independent coverage not preserved: %+v", packet.Sources)
	}
}
