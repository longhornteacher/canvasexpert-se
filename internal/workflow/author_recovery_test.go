package workflow

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/render"
	"canvasexpertse/internal/workspace"
)

func TestCreateResumeRecognizesCanvasHTMLEntitiesAndPreservesLiveEdits(t *testing.T) {
	const body = "Explain the author's evidence."
	intended, e := render.MarkdownHTML(body)
	if e != nil {
		t.Fatal(e)
	}
	for _, recovery := range []string{"known-id", "uncertain-response"} {
		for _, tc := range []struct {
			name, liveHTML, status string
		}{
			{"entity-normalized", "<p>Explain the author's evidence.</p>", "completed"},
			{"teacher-content-edit", "<p>Explain the author's reasoning.</p>", "held"},
			{"teacher-markup-edit", "<p>Explain the author's <strong>evidence</strong>.</p>", "held"},
		} {
			t.Run(recovery+"/"+tc.name, func(t *testing.T) {
				live := fixtureAssignment()
				live.Name, live.Description, live.CreatedAt = "Draft", tc.liveHTML, "2026-10-08T12:00:00Z"
				live.Rubric, live.RubricAssociation = nil, nil
				live.RubricSettings.ID = 0
				writes := 0
				s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
					if r.Method != "GET" {
						writes++
						t.Errorf("recovery replayed a write: %s %s", r.Method, r.URL.Path)
					}
					if r.URL.Path == "/api/v1/courses/1/assignments" {
						_ = json.NewEncoder(w).Encode([]Assignment{live})
					} else {
						_ = json.NewEncoder(w).Encode(live)
					}
				})
				doc := domain.AssignmentDocument{Schema: 1, Kind: "assignment", Operation: "create", Course: "TEST", Instance: s.Client.Origin(), CourseID: 1,
					Title: "Draft", Points: domain.MustDecimal("20"), SubmissionTypes: []string{"online_text_entry"},
					Created: &domain.CreatedResources{AssignmentID: live.ID, StartedAt: live.CreatedAt, IntendedHTML: intended,
						Assignment: domain.ActionOutcome{Status: "succeeded", ID: live.ID}}}
				if recovery == "uncertain-response" {
					doc.Created.AssignmentID = 0
					doc.Created.Assignment = domain.ActionOutcome{Status: "uncertain", Message: "response lost"}
				}
				path := filepath.Join(s.Root, "draft.md")
				if e := workspace.WriteDocument(path, doc, body); e != nil {
					t.Fatal(e)
				}
				report, e := s.Push(context.Background(), path)
				if e != nil || report.Status != tc.status || writes != 0 {
					t.Fatalf("create recovery: %#v writes=%d err=%v", report, writes, e)
				}
				if tc.status == "completed" && report.AssignmentID != live.ID {
					t.Fatalf("recovery lost exact existing ID: %#v", report)
				}
			})
		}
	}
}

func TestIncompleteCreatePreservesTeacherRubricAndUnknownPublication(t *testing.T) {
	for _, operation := range []string{"rubric", "publication"} {
		t.Run(operation, func(t *testing.T) {
			live := fixtureAssignment()
			live.Name, live.Description = "Draft", "<p>Explain.</p>"
			writes := 0
			s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" {
					writes++
					t.Errorf("recovery overwrote live state: %s %s", r.Method, r.URL.Path)
				}
				_ = json.NewEncoder(w).Encode(live)
			})
			doc := domain.AssignmentDocument{Schema: 1, Kind: "assignment", Operation: "create", Course: "TEST", Instance: s.Client.Origin(), CourseID: 1,
				Title: "Draft", Points: domain.MustDecimal("20"), SubmissionTypes: []string{"online_text_entry"}, Published: true,
				Created: &domain.CreatedResources{AssignmentID: 10, StartedAt: "2026-10-08T12:00:00Z", IntendedHTML: "<p>Explain.</p>",
					Assignment: domain.ActionOutcome{Status: "succeeded", ID: 10}, Module: domain.ActionOutcome{Status: "not-requested"}}}
			if operation == "rubric" {
				snapshot, e := Snapshot(live, s.Client.Origin())
				if e != nil {
					t.Fatal(e)
				}
				doc.Rubric = snapshot.Rubric
				doc.Created.Rubric = domain.ActionOutcome{Status: "failed", Message: "known rejected attachment"}
			} else {
				live.Rubric, live.RubricAssociation = nil, nil
				live.RubricSettings.ID = 0
				doc.Created.Rubric = domain.ActionOutcome{Status: "not-requested"}
				doc.Created.Publication = domain.ActionOutcome{Status: "uncertain", Message: "response lost"}
			}
			path := filepath.Join(s.Root, "draft.md")
			if e := workspace.WriteDocument(path, doc, "Explain."); e != nil {
				t.Fatal(e)
			}
			report, e := s.Push(context.Background(), path)
			if e != nil || report.Status != "held" || writes != 0 {
				t.Fatalf("unsafe recovery: %#v writes=%d err=%v", report, writes, e)
			}
		})
	}
}
