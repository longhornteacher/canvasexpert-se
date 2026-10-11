package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"reflect"
	"testing"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

func moduleDocPath(t *testing.T, s *Service, d domain.ModuleDocument) string {
	t.Helper()
	path := filepath.Join(s.Root, "module.md")
	d.Instance = s.Client.Origin()
	d.CourseID = 1
	if err := workspace.WriteDocument(path, d, ""); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestPushModuleCreatesNativeOrderedItemsPublishesAndResumes(t *testing.T) {
	type itemRequest struct {
		Type      string `json:"type"`
		ContentID any    `json:"content_id"`
		PageURL   string `json:"page_url"`
		Title     string `json:"title"`
	}
	items := map[int64]canvasModuleItem{}
	var nextItem int64 = 100
	var itemTypes []string
	modulePublished := false
	moduleCreates, itemPosts, publicationPuts := 0, 0, 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "POST" && r.URL.Path == "/api/v1/courses/1/modules":
			moduleCreates++
			_ = json.NewEncoder(w).Encode(canvasModule{ID: 7, CourseID: 1, Name: "Unit"})
		case r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/modules/7":
			_ = json.NewEncoder(w).Encode(canvasModule{ID: 7, CourseID: 1, Name: "Unit", Published: modulePublished})
		case r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/pages/page_id:11":
			published, locked := true, false
			_ = json.NewEncoder(w).Encode(canvasPage{PageID: 11, URL: "unit-reading", Title: "Reading", Published: &published, Locked: &locked})
		case r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/files/22":
			locked, hidden := false, false
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 22, "folder_id": 3, "locked": locked, "hidden": hidden, "visibility_level": "course"})
		case r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/folders/3":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 3, "context_id": 1, "context_type": "Course"})
		case r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/assignments/33":
			published := false
			_ = json.NewEncoder(w).Encode(canvasModuleAssignment{ID: 33, CourseID: 1, Published: &published})
		case r.Method == "POST" && r.URL.Path == "/api/v1/courses/1/modules/7/items":
			itemPosts++
			var payload struct {
				ModuleItem itemRequest `json:"module_item"`
			}
			_ = json.NewDecoder(r.Body).Decode(&payload)
			itemTypes = append(itemTypes, payload.ModuleItem.Type)
			nextItem++
			created := canvasModuleItem{ID: nextItem, Type: payload.ModuleItem.Type, ContentID: payload.ModuleItem.ContentID, PageURL: payload.ModuleItem.PageURL, Title: payload.ModuleItem.Title, Position: itemPosts}
			items[created.ID] = created
			_ = json.NewEncoder(w).Encode(created)
		case r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/modules/7/items/101",
			r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/modules/7/items/102",
			r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/modules/7/items/103",
			r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/modules/7/items/104":
			var id int64
			_, _ = fmt.Sscanf(filepath.Base(r.URL.Path), "%d", &id)
			_ = json.NewEncoder(w).Encode(items[id])
		case r.Method == "PUT" && r.URL.Path == "/api/v1/courses/1/modules/7":
			publicationPuts++
			modulePublished = true
			_ = json.NewEncoder(w).Encode(canvasModule{ID: 7, CourseID: 1, Name: "Unit", Published: true})
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	})
	publish := true
	path := moduleDocPath(t, s, domain.ModuleDocument{Schema: 1, Kind: "module", Course: "TEST", Title: "Unit", Published: &publish,
		Items: []domain.ModuleItem{{Type: "Page", ContentID: 11}, {Type: "File", ContentID: 22}, {Type: "Assignment", ContentID: 33}, {Type: "SubHeader", Title: "Discuss"}}})
	var authored domain.ModuleDocument
	if _, err := workspace.ReadDocument(path, &authored); err != nil {
		t.Fatal(err)
	}
	if err := workspace.WriteDocument(path, authored, "Module author notes.\n"); err != nil {
		t.Fatal(err)
	}
	first, err := s.PushModule(context.Background(), path)
	if err != nil || first.Status != "completed" || !first.Published || first.ModuleID != 7 || len(first.Items) != 4 || moduleCreates != 1 || itemPosts != 4 || publicationPuts != 1 {
		t.Fatalf("module create: report=%+v creates=%d itemPosts=%d publication=%d err=%v", first, moduleCreates, itemPosts, publicationPuts, err)
	}
	if !reflect.DeepEqual(itemTypes, []string{"Page", "File", "Assignment", "SubHeader"}) {
		t.Fatalf("Canvas item order changed: %v", itemTypes)
	}
	var saved domain.ModuleDocument
	body, err := workspace.ReadDocument(path, &saved)
	if err != nil || body != "Module author notes.\n" || saved.Created.ModuleID != 7 || saved.Created.Items[0].ModuleItemID != 101 || saved.Created.Items[1].ModuleItemID != 102 || saved.Created.Items[2].ModuleItemID != 103 || saved.Created.Items[3].ModuleItemID != 104 {
		t.Fatalf("saved module IDs/body: doc=%+v body=%q err=%v", saved, body, err)
	}
	second, err := s.PushModule(context.Background(), path)
	if err != nil || second.Status != "completed" || !second.Published || moduleCreates != 1 || itemPosts != 4 || publicationPuts != 1 {
		t.Fatalf("rerun duplicated native content or failed after publication: report=%+v creates=%d itemPosts=%d publication=%d err=%v", second, moduleCreates, itemPosts, publicationPuts, err)
	}
	modulePublished = false // A later teacher edit must be preserved on another rerun.
	third, err := s.PushModule(context.Background(), path)
	if err != nil || third.Status != "held" || moduleCreates != 1 || itemPosts != 4 || publicationPuts != 1 {
		t.Fatalf("rerun overwrote later teacher publication change: report=%+v creates=%d itemPosts=%d publication=%d err=%v", third, moduleCreates, itemPosts, publicationPuts, err)
	}
}

func TestPushModuleKeepsDocumentOrderAndSendsExplicitPosition(t *testing.T) {
	var posts []map[string]any
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/modules/7":
			_ = json.NewEncoder(w).Encode(canvasModule{ID: 7, CourseID: 1, Name: "Unit", Published: false})
		case r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/modules/7/items/101":
			_ = json.NewEncoder(w).Encode(canvasModuleItem{ID: 101, Type: "SubHeader", Title: "Read"})
		case r.Method == "POST" && r.URL.Path == "/api/v1/courses/1/modules/7/items":
			var body struct {
				ModuleItem map[string]any `json:"module_item"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			posts = append(posts, body.ModuleItem)
			_ = json.NewEncoder(w).Encode(canvasModuleItem{ID: int64(101 + len(posts)), Type: body.ModuleItem["type"].(string), Title: body.ModuleItem["title"].(string)})
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	})
	path := moduleDocPath(t, s, domain.ModuleDocument{Schema: 1, Kind: "module", Course: "TEST", Title: "Unit", ModuleID: 7,
		Items: []domain.ModuleItem{{Type: "SubHeader", Title: "Read"}, {Type: "SubHeader", Title: "Practice", Position: 4}}})
	// First item is already recorded so the test focuses on ordering and the
	// explicit Canvas insertion position of the following item.
	var d domain.ModuleDocument
	if _, err := workspace.ReadDocument(path, &d); err != nil {
		t.Fatal(err)
	}
	d.Created = &domain.ModuleCreated{ModuleID: 7, Items: []domain.ModuleItemOutcome{{Index: 0, Type: "SubHeader", Title: "Read", Outcome: domain.ActionOutcome{Status: "succeeded", ID: 101}, ModuleItemID: 101}}}
	if err := workspace.WriteDocument(path, d, ""); err != nil {
		t.Fatal(err)
	}
	report, err := s.PushModule(context.Background(), path)
	if err != nil || report.Status != "completed" || len(posts) != 1 || posts[0]["title"] != "Practice" || posts[0]["position"] != float64(4) {
		t.Fatalf("module push: report=%+v posts=%v err=%v", report, posts, err)
	}
}

func TestPushModuleRejectsReferenceFromWrongCourse(t *testing.T) {
	writes := 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/modules/7":
			_ = json.NewEncoder(w).Encode(canvasModule{ID: 7, CourseID: 1})
		case r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/assignments/10":
			_ = json.NewEncoder(w).Encode(Assignment{ID: 10, CourseID: 2})
		case r.Method == "POST":
			writes++
			http.Error(w, "unexpected", http.StatusInternalServerError)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	})
	path := moduleDocPath(t, s, domain.ModuleDocument{Schema: 1, Kind: "module", Course: "TEST", Title: "Unit", ModuleID: 7,
		Items: []domain.ModuleItem{{Type: "Assignment", ContentID: 10}}})
	report, err := s.PushModule(context.Background(), path)
	if err == nil || writes != 0 {
		t.Fatalf("wrong-course reference was not held: report=%+v writes=%d err=%v", report, writes, err)
	}
}

func TestPushModuleResumesPartialItemBySavedExactID(t *testing.T) {
	posts := 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/modules/7":
			_ = json.NewEncoder(w).Encode(canvasModule{ID: 7, CourseID: 1})
		case r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/modules/7/items/101":
			_ = json.NewEncoder(w).Encode(canvasModuleItem{ID: 101, Type: "SubHeader", Title: "Start"})
		case r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/modules/7/items/202":
			_ = json.NewEncoder(w).Encode(canvasModuleItem{ID: 202, Type: "SubHeader", Title: "Continue"})
		case r.Method == "POST":
			posts++
			http.Error(w, "must not replay", http.StatusInternalServerError)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	})
	path := moduleDocPath(t, s, domain.ModuleDocument{Schema: 1, Kind: "module", Course: "TEST", Title: "Unit", ModuleID: 7,
		Items: []domain.ModuleItem{{Type: "SubHeader", Title: "Start"}, {Type: "SubHeader", Title: "Continue"}},
		Created: &domain.ModuleCreated{ModuleID: 7, Items: []domain.ModuleItemOutcome{
			{Index: 0, Type: "SubHeader", Title: "Start", ModuleItemID: 101, Outcome: domain.ActionOutcome{Status: "succeeded", ID: 101}},
			{Index: 1, Type: "SubHeader", Title: "Continue", ModuleItemID: 202, Outcome: domain.ActionOutcome{Status: "uncertain", ID: 202}},
		}}})
	report, err := s.PushModule(context.Background(), path)
	if err != nil || report.Status != "completed" || posts != 0 {
		t.Fatalf("partial placement replayed or failed to verify: report=%+v posts=%d err=%v", report, posts, err)
	}
	var saved domain.ModuleDocument
	if _, err := workspace.ReadDocument(path, &saved); err != nil || saved.Created.Items[1].ModuleItemID != 202 || saved.Created.Items[1].Outcome.Status != "succeeded" {
		t.Fatalf("verified ID was not saved: %+v err=%v", saved.Created, err)
	}
}

func TestPushModuleHoldsUncertainItemWithoutSavedID(t *testing.T) {
	posts := 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/modules/7":
			_ = json.NewEncoder(w).Encode(canvasModule{ID: 7, CourseID: 1})
		case r.Method == "POST":
			posts++
			http.Error(w, "must not replay", http.StatusInternalServerError)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	})
	path := moduleDocPath(t, s, domain.ModuleDocument{Schema: 1, Kind: "module", Course: "TEST", Title: "Unit", ModuleID: 7,
		Items: []domain.ModuleItem{{Type: "SubHeader", Title: "Continue"}}, Created: &domain.ModuleCreated{ModuleID: 7,
			Items: []domain.ModuleItemOutcome{{Index: 0, Type: "SubHeader", Title: "Continue", Outcome: domain.ActionOutcome{Status: "uncertain"}}}}})
	report, err := s.PushModule(context.Background(), path)
	if err != nil || report.Status != "held" || posts != 0 {
		t.Fatalf("uncertain item was replayed: report=%+v posts=%d err=%v", report, posts, err)
	}
}
