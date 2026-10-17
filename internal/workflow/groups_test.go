package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

func TestPushGroupSetPersistsIDsAndResumesWithoutDuplicateCreates(t *testing.T) {
	sets, groups := 0, 0
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		write := func(v any) { _ = json.NewEncoder(w).Encode(v) }
		switch r.URL.Path {
		case "/api/v1/courses/1/group_categories":
			if r.Method == http.MethodPost {
				sets++
				write(map[string]any{"id": 50, "course_id": 1, "context_type": "Course", "name": "Fall clubs", "non_collaborative": false})
				return
			}
			write([]any{})
		case "/api/v1/group_categories/50":
			write(map[string]any{"id": 50, "course_id": 1, "context_type": "Course", "name": "Fall clubs", "non_collaborative": false, "self_signup": nil, "auto_leader": nil})
		case "/api/v1/group_categories/50/groups":
			if r.Method == http.MethodPost {
				var b struct {
					Group map[string]any `json:"group"`
				}
				_ = json.NewDecoder(r.Body).Decode(&b)
				groups++
				write(map[string]any{"id": int64(60 + groups), "course_id": 1, "context_type": "Course", "group_category_id": 50, "name": b.Group["name"], "html_url": fmt.Sprintf("https://canvas.example/groups/%d", 60+groups)})
				return
			}
			write([]any{})
		case "/api/v1/groups/61", "/api/v1/groups/62":
			id := int64(61)
			if strings.HasSuffix(r.URL.Path, "62") {
				id = 62
			}
			write(map[string]any{"id": id, "course_id": 1, "context_type": "Course", "group_category_id": 50, "name": "A"})
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}
	s, _ := workflowFixture(t, handler)
	doc := domain.GroupSetDocument{Schema: 1, Kind: "group-set", Operation: "create", Course: "TEST", Name: "Fall clubs", Groups: []domain.GroupSetRow{{Key: "club-a", Name: "Club A"}, {Key: "club-b", Name: "Club B"}}}
	path := filepath.Join(s.Root, "courses", "TEST", "drafts", "clubs.md")
	if err := workspace.WriteDocument(path, doc, ""); err != nil {
		t.Fatal(err)
	}
	first, err := s.PushGroupSet(context.Background(), path)
	if err != nil || first.Status != "completed" || first.GroupCategoryID != 50 || len(first.Groups) != 2 {
		t.Fatalf("first push %+v %v", first, err)
	}
	var saved domain.GroupSetDocument
	if _, err = workspace.ReadDocument(path, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Created.GroupCategoryID != 50 || saved.Created.Groups[0].GroupID != 61 || saved.Created.Groups[1].GroupID != 62 {
		t.Fatalf("exact outcomes not saved: %+v", saved.Created)
	}
	second, err := s.PushGroupSet(context.Background(), path)
	if err != nil || second.Status != "completed" || sets != 1 || groups != 2 {
		t.Fatalf("resume duplicated create: %+v err=%v sets=%d groups=%d", second, err, sets, groups)
	}
}

func TestPullGroupSetRefusesWrongCourseBeforeSaving(t *testing.T) {
	writes := 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		writes++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":5,"course_id":2,"context_type":"Course","non_collaborative":false}`))
	})
	if _, err := s.PullGroupSet(context.Background(), "TEST", 5); err == nil {
		t.Fatal("wrong-course set accepted")
	}
	entries, err := os.ReadDir(filepath.Join(s.Root, "courses"))
	if err == nil && len(entries) > 0 {
		t.Fatal("wrong target created a local baseline")
	}
	_ = writes
}

func TestListGroupSetsAndGroupsFollowCanvasPagination(t *testing.T) {
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		write := func(v string) { _, _ = w.Write([]byte(v)) }
		switch r.URL.Path {
		case "/api/v1/courses/1/group_categories":
			if r.URL.Query().Get("collaboration_state") != "collaborative" {
				t.Errorf("missing collaborative filter: %s", r.URL.RawQuery)
			}
			write(`[ {"id":7,"course_id":1,"context_type":"Course","name":"Clubs","non_collaborative":false} ]`)
		case "/api/v1/group_categories/7":
			write(`{"id":7,"course_id":1,"context_type":"Course","name":"Clubs","non_collaborative":false}`)
		case "/api/v1/group_categories/7/groups":
			if r.URL.Query().Get("page") == "2" {
				write(`[{"id":72,"course_id":1,"context_type":"Course","group_category_id":7,"name":"Later club","non_collaborative":false}]`)
				return
			}
			w.Header().Set("Link", `</api/v1/group_categories/7/groups?page=2>; rel="next"`)
			write(`[{"id":71,"course_id":1,"context_type":"Course","group_category_id":7,"name":"First club","non_collaborative":false}]`)
		default:
			t.Errorf("unexpected group API path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	})
	sets, err := s.ListGroupSets(context.Background(), "TEST")
	if err != nil || len(sets) != 1 || sets[0].GroupCount != 2 {
		t.Fatalf("sets=%+v err=%v", sets, err)
	}
	groups, err := s.ListGroups(context.Background(), "TEST", 7, 0, false)
	if err != nil || len(groups) != 2 || groups[1].ID != 72 {
		t.Fatalf("groups=%+v err=%v", groups, err)
	}
}

func TestGroupSetNameConflictPreservesRequestedDraftAndDoesNotWrite(t *testing.T) {
	writes := 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":8,"course_id":1,"context_type":"Course","name":"Live changed","non_collaborative":false}`))
		if r.Method != http.MethodGet {
			writes++
		}
	})
	doc := domain.GroupSetDocument{Schema: 1, Kind: "group-set", Operation: "update", Course: "TEST", Instance: s.Client.Origin(), CourseID: 1, GroupCategoryID: 8, Name: "Teacher's desired name", Observed: &domain.GroupSetSnapshot{Instance: s.Client.Origin(), CourseID: 1, GroupCategoryID: 8, Name: "Baseline name", SelfSignup: "disabled", AutoLeader: "none"}}
	path := filepath.Join(s.Root, "courses", "TEST", "drafts", "conflict.md")
	if err := workspace.WriteDocument(path, doc, ""); err != nil {
		t.Fatal(err)
	}
	report, err := s.PushGroupSet(context.Background(), path)
	if err != nil || report.Status != "held" || writes != 0 {
		t.Fatalf("conflict result %+v err=%v writes=%d", report, err, writes)
	}
	var saved domain.GroupSetDocument
	if _, err = workspace.ReadDocument(path, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Name != doc.Name || saved.Observed.Name != doc.Observed.Name {
		t.Fatalf("unsent draft or baseline changed: %+v", saved)
	}
}
