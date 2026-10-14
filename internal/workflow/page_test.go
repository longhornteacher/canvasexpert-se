package workflow

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

func pageDraft() domain.PageDocument {
	return domain.PageDocument{Schema: 1, Kind: "page", Operation: "create", Course: "TEST", Title: "Lesson", BodyFormat: "html"}
}

func TestPageCreateUsesReturnedSlugSavedHTMLAndResumes(t *testing.T) {
	const body = `<h2>Lesson</h2><table><tr><td>1</td></tr></table><img src="/courses/1/files/9/preview">`
	const saved = `<h2>Lesson</h2><table><tbody><tr><td>1</td></tr></tbody></table><img src="/courses/1/files/9/preview" />`
	p := NativePage{PageID: 7, Slug: "actual-lesson-2", Title: "Lesson", Body: saved}
	var writes []string
	var path string
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "GET /api/v1/courses/1/modules/4":
			json.NewEncoder(w).Encode(map[string]any{"id": 4})
		case "POST /api/v1/courses/1/pages":
			writes = append(writes, "create")
			var payload struct {
				Page struct {
					Body      string `json:"body"`
					Published bool   `json:"published"`
					Roles     string `json:"editing_roles"`
				} `json:"wiki_page"`
			}
			json.NewDecoder(r.Body).Decode(&payload)
			if payload.Page.Body != body || payload.Page.Published || payload.Page.Roles != "teachers" {
				t.Errorf("unexpected page create: %#v", payload)
			}
			json.NewEncoder(w).Encode(p)
		case "GET /api/v1/courses/1/pages/page_id:7":
			json.NewEncoder(w).Encode(p)
		case "POST /api/v1/courses/1/modules/4/items":
			writes = append(writes, "place")
			var persisted domain.PageDocument
			if _, e := workspace.ReadDocument(path, &persisted); e != nil || persisted.Created.PageID != 7 || persisted.Created.Slug != p.Slug || persisted.Created.SavedHTML != saved {
				t.Errorf("page identity not persisted before placement: %#v %v", persisted, e)
			}
			var payload struct {
				Item struct {
					Type     string `json:"type"`
					PageURL  string `json:"page_url"`
					Position int    `json:"position"`
				} `json:"module_item"`
			}
			json.NewDecoder(r.Body).Decode(&payload)
			if payload.Item.Type != "Page" || payload.Item.PageURL != p.Slug || payload.Item.Position != 2 {
				t.Errorf("wrong page placement: %#v", payload)
			}
			json.NewEncoder(w).Encode(map[string]any{"id": 8})
		case "GET /api/v1/courses/1/modules/4/items/8":
			json.NewEncoder(w).Encode(map[string]any{"id": 8, "type": "Page", "page_url": p.Slug})
		case "PUT /api/v1/courses/1/pages/page_id:7":
			writes = append(writes, "publish")
			p.Published = true
			json.NewEncoder(w).Encode(p)
		default:
			t.Errorf("unexpected ordinary Page request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	})
	path = filepath.Join(s.Root, "lesson.md")
	d := pageDraft()
	d.Published = true
	d.ModuleID = 4
	d.ModulePosition = 2
	if e := workspace.WriteDocument(path, &d, body); e != nil {
		t.Fatal(e)
	}
	r, e := s.PushPage(context.Background(), path)
	if e != nil || r.Status != "completed" || r.PageID != 7 || r.ModuleItemID != 8 || !strings.HasSuffix(r.URL, "/pages/actual-lesson-2") || strings.Join(writes, ",") != "create,place,publish" {
		t.Fatalf("create %#v writes %v err %v", r, writes, e)
	}
	// A known completed record verifies by exact ID and writes nothing again.
	r, e = s.PushPage(context.Background(), path)
	if e != nil || r.Status != "completed" || len(writes) != 3 {
		t.Fatalf("resume %#v writes %v err %v", r, writes, e)
	}
}

func TestPagePullUpdatePatchesOnlyRequestedFieldsAndKeepsReturnedSlug(t *testing.T) {
	p := NativePage{PageID: 7, Slug: "lesson", Title: "Lesson", Body: "<p>Exact &amp; original.</p>"}
	var patches []map[string]any
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/courses/1/pages/page_id:7" {
			t.Errorf("Page must use forced exact ID: %s", r.URL.Path)
		}
		if r.Method == "PUT" {
			var payload struct {
				Page map[string]any `json:"wiki_page"`
			}
			json.NewDecoder(r.Body).Decode(&payload)
			patches = append(patches, payload.Page)
			p.Title = payload.Page["title"].(string)
			p.Slug = "renamed-by-canvas"
		}
		json.NewEncoder(w).Encode(p)
	})
	path, e := s.PullPage(context.Background(), "TEST", "7")
	if e != nil {
		t.Fatal(e)
	}
	var d domain.PageDocument
	body, e := workspace.ReadDocument(path, &d)
	if e != nil || body != p.Body || d.BodyFormat != "html" {
		t.Fatalf("pull lost exact HTML %q %v", body, e)
	}
	d.Title = "New lesson"
	if e = workspace.WriteDocument(path, &d, body); e != nil {
		t.Fatal(e)
	}
	// An unrelated body edit survives a title-only patch.
	p.Body = "<p>Teacher revised the body.</p>"
	r, e := s.PushPage(context.Background(), path)
	if e != nil || r.Status != "succeeded" || len(patches) != 1 || len(patches[0]) != 1 || patches[0]["title"] != "New lesson" {
		t.Fatalf("update %#v patches %v err %v", r, patches, e)
	}
	if _, e = workspace.ReadDocument(path, &d); e != nil || d.Slug != "renamed-by-canvas" || d.Observed.BodyHTML != p.Body {
		t.Fatalf("actual saved page not recorded: %#v err %v", d, e)
	}
}

func TestPageConflictPreservesUnsentBodyAndWritesFreshBaseline(t *testing.T) {
	p := NativePage{PageID: 7, Slug: "lesson", Title: "Lesson", Body: "<p>Original</p>"}
	writes := 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			writes++
		}
		json.NewEncoder(w).Encode(p)
	})
	path, e := s.PullPage(context.Background(), "TEST", "7")
	if e != nil {
		t.Fatal(e)
	}
	var d domain.PageDocument
	workspace.ReadDocument(path, &d)
	draft := "<table><tr><td>Unsent work</td></tr></table>"
	if e = workspace.WriteDocument(path, &d, draft); e != nil {
		t.Fatal(e)
	}
	before, _ := os.ReadFile(path)
	p.Body = "<p>Teacher live edit</p>"
	r, e := s.PushPage(context.Background(), path)
	after, _ := os.ReadFile(path)
	if e != nil || r.Status != "held" || writes != 0 || string(before) != string(after) || !strings.Contains(r.Message, "fresh version:") {
		t.Fatalf("conflict %#v writes %d err %v", r, writes, e)
	}
	fresh := strings.Split(r.Message, "fresh version: ")[1]
	got, e := workspace.ReadDocument(fresh, &d)
	if e != nil || got != p.Body {
		t.Fatalf("fresh baseline %q err %v", got, e)
	}
}

func TestPageIncompleteResponsesKeepKnownIDsAndNeverReplayUnknownCreate(t *testing.T) {
	for _, known := range []bool{false, true} {
		t.Run(map[bool]string{false: "unknown-id", true: "missing-slug"}[known], func(t *testing.T) {
			writes := 0
			s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "POST" {
					writes++
					if known {
						json.NewEncoder(w).Encode(NativePage{PageID: 7, Title: "Lesson", Body: "<p>Body</p>"})
					} else {
						w.Write([]byte(`{}`))
					}
					return
				}
				if r.URL.Path == "/api/v1/courses/1/pages" {
					// Exact-title lookup used to adopt a Page whose create was unclear.
					json.NewEncoder(w).Encode([]NativePage{{PageID: 7, Slug: "saved-slug", Title: "Lesson"}})
					return
				}
				if r.URL.Path != "/api/v1/courses/1/pages/page_id:7" {
					t.Errorf("unsafe lookup %s", r.URL.Path)
				}
				json.NewEncoder(w).Encode(NativePage{PageID: 7, Slug: "saved-slug", Title: "Lesson", Body: "<p>Body</p>"})
			})
			path := filepath.Join(s.Root, "lesson.md")
			d := pageDraft()
			workspace.WriteDocument(path, &d, "<p>Body</p>")
			r, e := s.PushPage(context.Background(), path)
			if e != nil || r.Status != "uncertain" {
				t.Fatalf("create %#v %v", r, e)
			}
			workspace.ReadDocument(path, &d)
			if known && d.Created.PageID != 7 {
				t.Fatal("positive returned Page ID lost")
			}
			r, e = s.PushPage(context.Background(), path)
			if e != nil || r.Status != "completed" || writes != 1 {
				t.Fatalf("resume %#v writes %d err %v", r, writes, e)
			}
		})
	}
}

func TestPageResumePreservesTeacherPublicationAndChecksModuleBeforeCreation(t *testing.T) {
	for _, wrongModule := range []bool{false, true} {
		t.Run(map[bool]string{false: "teacher-published", true: "wrong-course-module"}[wrongModule], func(t *testing.T) {
			writes := 0
			s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" {
					writes++
				}
				if strings.Contains(r.URL.Path, "modules") {
					json.NewEncoder(w).Encode(map[string]any{"id": 99})
				} else {
					json.NewEncoder(w).Encode(NativePage{PageID: 7, Slug: "lesson", Title: "Lesson", Body: "<p>Body</p>", Published: true})
				}
			})
			d := pageDraft()
			if wrongModule {
				d.ModuleID = 4
			} else {
				d.Instance = s.Client.Origin()
				d.CourseID = 1
				d.Created = &domain.PageCreated{PageID: 7, Slug: "lesson", IntendedTitle: "Lesson", IntendedHTML: "<p>Body</p>", SavedHTML: "<p>Body</p>", Page: domain.ActionOutcome{Status: "succeeded", ID: 7}}
			}
			path := filepath.Join(s.Root, "lesson.md")
			if e := workspace.WriteDocument(path, &d, "<p>Body</p>"); e != nil {
				t.Fatal(e)
			}
			r, e := s.PushPage(context.Background(), path)
			if writes != 0 || (!wrongModule && (e != nil || r.Status != "completed")) || (wrongModule && e == nil) {
				t.Fatalf("preservation %#v writes %d err %v", r, writes, e)
			}
		})
	}
}

func TestPagePullRejectsWrongCourseAndOriginBeforeRequests(t *testing.T) {
	requests := 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) { requests++ })
	for _, query := range []string{s.Client.Origin() + "/courses/2/pages/lesson", "https://different.example/courses/1/pages/lesson"} {
		if _, e := s.PullPage(context.Background(), "TEST", query); e == nil {
			t.Errorf("wrong destination accepted: %s", query)
		}
	}
	if requests != 0 {
		t.Fatal("wrong destination initiated request")
	}
}

func TestPageKnownIDConfirmsUncertainCreateWithoutRepeatingWrites(t *testing.T) {
	writes := 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			writes++
		}
		json.NewEncoder(w).Encode(NativePage{PageID: 7, Slug: "lesson", Title: "Lesson", Body: "<p>Body</p>"})
	})
	d := pageDraft()
	d.Instance = s.Client.Origin()
	d.CourseID = 1
	d.Created = &domain.PageCreated{PageID: 7, Slug: "lesson", IntendedTitle: "Lesson", IntendedHTML: "<p>Body</p>", SavedHTML: "<p>Body</p>", Page: domain.ActionOutcome{Status: "uncertain"}}
	path := filepath.Join(s.Root, "lesson.md")
	if e := workspace.WriteDocument(path, &d, "<p>Body</p>"); e != nil {
		t.Fatal(e)
	}
	r, e := s.PushPage(context.Background(), path)
	if e != nil || r.Status != "completed" || writes != 0 {
		t.Fatalf("known create recovery %#v writes %d err %v", r, writes, e)
	}
}
