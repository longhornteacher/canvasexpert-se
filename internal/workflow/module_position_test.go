package workflow

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

func TestModulePositionCreateUpdateAndResume(t *testing.T) {
	for _, existing := range []bool{false, true} {
		name := "create"
		if existing {
			name = "update"
		}
		t.Run(name, func(t *testing.T) {
			position, writes, lists := 3, 0, 0
			s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == "POST" && r.URL.Path == "/api/v1/courses/1/modules",
					r.Method == "PUT" && r.URL.Path == "/api/v1/courses/1/modules/7":
					writes++
					var body struct {
						Module struct {
							Position int `json:"position"`
						} `json:"module"`
					}
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Fatal(err)
					}
					position = body.Module.Position
					_ = json.NewEncoder(w).Encode(canvasModule{ID: 7, CourseID: 1, Name: "Unit", Position: position})
				case r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/modules/7":
					_ = json.NewEncoder(w).Encode(canvasModule{ID: 7, CourseID: 1, Name: "Unit", Position: position})
				case r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/modules":
					lists++
					_ = json.NewEncoder(w).Encode([]canvasModule{{ID: 7, CourseID: 1, Name: "Unit", Position: position}})
				case r.Method == "POST" && r.URL.Path == "/api/v1/courses/1/modules/7/items":
					_ = json.NewEncoder(w).Encode(canvasModuleItem{ID: 101, Type: "SubHeader", Title: "Read"})
				case r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/modules/7/items/101":
					_ = json.NewEncoder(w).Encode(canvasModuleItem{ID: 101, Type: "SubHeader", Title: "Read"})
				default:
					t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
					http.NotFound(w, r)
				}
			})
			requested := 1
			d := domain.ModuleDocument{Schema: 1, Kind: "module", Course: "TEST", Title: "Unit", Position: &requested, Items: []domain.ModuleItem{{Type: "SubHeader", Title: "Read"}}}
			if existing {
				d.ModuleID = 7
			}
			path := moduleDocPath(t, s, d)
			first, err := s.PushModule(context.Background(), path)
			if err != nil || first.Status != "completed" || first.Position != 1 || writes != 1 || lists != 1 {
				t.Fatalf("position not applied and listed: %+v writes=%d lists=%d err=%v", first, writes, lists, err)
			}
			position = 2 // A later teacher reorder must survive a resume.
			if _, err := s.PushModule(context.Background(), path); err != nil || writes != 1 || lists != 1 {
				t.Fatalf("resume repeated a completed move: writes=%d lists=%d err=%v", writes, lists, err)
			}
			var saved domain.ModuleDocument
			body, err := workspace.ReadDocument(path, &saved)
			if err != nil || saved.Created.Position == nil || *saved.Created.Position != 1 {
				t.Fatalf("position receipt missing: %+v err=%v", saved.Created, err)
			}
			requested = 3
			saved.Position = &requested
			if err := workspace.WriteDocument(path, saved, body); err != nil {
				t.Fatal(err)
			}
			if _, err := s.PushModule(context.Background(), path); err != nil || writes != 2 || lists != 2 || position != 3 {
				t.Fatalf("new move not applied: writes=%d lists=%d position=%d err=%v", writes, lists, position, err)
			}
		})
	}
}

func TestModulePositionValidation(t *testing.T) {
	for _, position := range []int{0, -1} {
		d := domain.ModuleDocument{Schema: 1, Kind: "module", Course: "TEST", Title: "Unit", Position: &position, Items: []domain.ModuleItem{{Type: "SubHeader", Title: "Read"}}}
		if err := d.Validate(); err == nil {
			t.Fatalf("accepted position %d", position)
		}
	}
	position := 1
	d := domain.ModuleDocument{Schema: 1, Kind: "module", Course: "TEST", Title: "Unit", ModuleID: 7, Position: &position}
	if err := d.Validate(); err != nil {
		t.Fatalf("position-only update requires no new items: %v", err)
	}
}

func TestModulePositionRequiresListedVerification(t *testing.T) {
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/modules/7":
			_ = json.NewEncoder(w).Encode(canvasModule{ID: 7, CourseID: 1, Name: "Unit", Position: 2})
		case r.Method == "PUT" && r.URL.Path == "/api/v1/courses/1/modules/7":
			_ = json.NewEncoder(w).Encode(canvasModule{ID: 7, CourseID: 1, Name: "Unit", Position: 1})
		case r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/modules":
			_ = json.NewEncoder(w).Encode([]canvasModule{{ID: 7, CourseID: 1, Name: "Unit", Position: 2}})
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	})
	position := 1
	path := moduleDocPath(t, s, domain.ModuleDocument{Schema: 1, Kind: "module", Course: "TEST", Title: "Unit", ModuleID: 7, Position: &position})
	if _, err := s.PushModule(context.Background(), path); err == nil {
		t.Fatal("module move accepted without list confirmation")
	}
	var saved domain.ModuleDocument
	if _, err := workspace.ReadDocument(path, &saved); err != nil || saved.Created.Position != nil {
		t.Fatalf("unverified move saved as completed: %+v err=%v", saved.Created, err)
	}
}
