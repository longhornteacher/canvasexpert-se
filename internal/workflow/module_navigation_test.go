package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

func TestModuleExternalUrlsWithSameTitleRemainDistinctAndResumeInOrder(t *testing.T) {
	items := map[int64]canvasModuleItem{}
	posts := 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/modules/7":
			_ = json.NewEncoder(w).Encode(canvasModule{ID: 7, CourseID: 1})
		case r.Method == "POST" && r.URL.Path == "/api/v1/courses/1/modules/7/items":
			posts++
			var body struct {
				ModuleItem struct {
					Type        string `json:"type"`
					Title       string `json:"title"`
					ExternalURL string `json:"external_url"`
					NewTab      *bool  `json:"new_tab"`
				} `json:"module_item"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			created := canvasModuleItem{ID: int64(100 + posts), Type: body.ModuleItem.Type, Title: body.ModuleItem.Title, ExternalURL: body.ModuleItem.ExternalURL, NewTab: body.ModuleItem.NewTab}
			items[created.ID] = created
			_ = json.NewEncoder(w).Encode(created)
		case r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/modules/7/items/101", r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/modules/7/items/102":
			var id int64
			_, _ = fmt.Sscanf(r.URL.Path[len(r.URL.Path)-3:], "%d", &id)
			_ = json.NewEncoder(w).Encode(items[id])
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	})
	newTab := false
	path := moduleDocPath(t, s, domain.ModuleDocument{Schema: 1, Kind: "module", Course: "TEST", Title: "Unit", ModuleID: 7, Items: []domain.ModuleItem{
		{Type: "ExternalUrl", Title: "Club notebook", ExternalURL: "https://docs.example.edu/club-one", NewTab: &newTab},
		{Type: "ExternalUrl", Title: "Club notebook", ExternalURL: "https://docs.example.edu/club-two"},
	}})
	first, err := s.PushModule(context.Background(), path)
	if err != nil || first.Status != "completed" || posts != 2 || first.Items[0].ModuleItemID != 101 || first.Items[1].ModuleItemID != 102 {
		t.Fatalf("distinct destinations not added in order: report=%+v posts=%d err=%v", first, posts, err)
	}
	second, err := s.PushModule(context.Background(), path)
	if err != nil || second.Status != "completed" || posts != 2 {
		t.Fatalf("resume duplicated module destinations: report=%+v posts=%d err=%v", second, posts, err)
	}
	var saved domain.ModuleDocument
	if _, err := workspace.ReadDocument(path, &saved); err != nil || saved.Created.Items[0].ExternalURL != "https://docs.example.edu/club-one" || saved.Created.Items[1].ExternalURL != "https://docs.example.edu/club-two" {
		t.Fatalf("destination identity was not persisted: %+v err=%v", saved.Created, err)
	}
}

func TestModuleDiscussionAndSameOriginGroupLinkAreCourseScoped(t *testing.T) {
	writes := 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/modules/7":
			_ = json.NewEncoder(w).Encode(canvasModule{ID: 7, CourseID: 1})
		case r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/discussion_topics/9":
			_ = json.NewEncoder(w).Encode(NativeDiscussion{ID: 8}) // requested ID mismatch
		case r.Method == "GET" && r.URL.Path == "/api/v1/groups/99":
			_ = json.NewEncoder(w).Encode(canvasGroup{ID: 99, CourseID: 2, GroupCategoryID: 12, ContextType: "Course"})
		case r.Method == "POST":
			writes++
			http.Error(w, "unexpected write", http.StatusInternalServerError)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	})
	path := moduleDocPath(t, s, domain.ModuleDocument{Schema: 1, Kind: "module", Course: "TEST", Title: "Unit", ModuleID: 7, Items: []domain.ModuleItem{{Type: "Discussion", ContentID: 9}}})
	if _, err := s.PushModule(context.Background(), path); err == nil {
		t.Fatal("discussion from another course was accepted")
	}
	path = moduleDocPath(t, s, domain.ModuleDocument{Schema: 1, Kind: "module", Course: "TEST", Title: "Unit", ModuleID: 7, Items: []domain.ModuleItem{{Type: "ExternalUrl", Title: "Other club", ExternalURL: s.Client.Origin() + "/groups/99/pages/notebook"}}})
	if _, err := s.PushModule(context.Background(), path); err == nil || writes != 0 {
		t.Fatalf("wrong-course group link was accepted or wrote: writes=%d err=%v", writes, err)
	}
}

func TestModuleExternalURLRejectsCredentialsAndSignedLinks(t *testing.T) {
	requests := 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		http.Error(w, "validation should stop before Canvas", http.StatusInternalServerError)
	})
	for _, raw := range []string{"javascript:alert(1)", "https://user:pass@example.edu/doc", "https://docs.example.edu/doc?access_token=secret", "https://docs.example.edu/doc?X-Amz-Signature=secret"} {
		d := domain.ModuleDocument{Schema: 1, Kind: "module", Course: "TEST", Title: "Unit", Items: []domain.ModuleItem{{Type: "ExternalUrl", Title: "Resource", ExternalURL: raw}}}
		if err := d.Validate(); err == nil {
			t.Errorf("accepted unsafe URL %q", raw)
		}
	}
	path := filepath.Join(s.Root, "unsafe.md")
	if err := os.WriteFile(path, []byte("---\nschema: 1\nkind: module\ncourse: TEST\ntitle: Unit\nmodule_id: 7\nitems:\n  - type: ExternalUrl\n    title: Resource\n    external_url: https://docs.example.edu/doc?access_token=secret\n---\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PushModule(context.Background(), path); err == nil || requests != 0 {
		t.Fatalf("invalid destination reached Canvas: requests=%d err=%v", requests, err)
	}
}
