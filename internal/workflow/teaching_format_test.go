package workflow

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/render"
	"canvasexpertse/internal/workspace"
)

func TestAssignmentHTMLCreateUsesCanvasNormalizedBodyForCompletionAndSafeReview(t *testing.T) {
	const body = `<table><tr><td>Instructions</td></tr></table><img src="/courses/1/files/9/preview"><script>bad()</script>`
	const saved = `<table><tbody><tr><td>Instructions</td></tr></tbody></table><img src="/courses/1/files/9/preview" />`
	a := fixtureAssignment()
	a.Name = "Activity"
	a.Description = saved
	a.Rubric = nil
	a.RubricAssociation = nil
	writes := 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			writes++
			var payload struct {
				Assignment struct {
					Description string `json:"description"`
					Published   bool   `json:"published"`
				} `json:"assignment"`
			}
			json.NewDecoder(r.Body).Decode(&payload)
			if payload.Assignment.Description != body || payload.Assignment.Published {
				t.Errorf("HTML altered or creation published: %#v", payload)
			}
		} else if r.Method == "PUT" {
			writes++
			a.Published = true
		}
		json.NewEncoder(w).Encode(a)
	})
	d := domain.AssignmentDocument{Schema: 1, Kind: "assignment", Operation: "create", Course: "TEST", Title: "Activity", BodyFormat: "html", Points: domain.MustDecimal("20"), SubmissionTypes: []string{"online_text_entry"}, Published: true}
	path := filepath.Join(s.Root, "activity.md")
	if e := workspace.WriteDocument(path, &d, body); e != nil {
		t.Fatal(e)
	}
	m, e := BuildReview(s.Root, path)
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(m.Notice, "escaped HTML source") || strings.Contains(m.Cards[0].Frames[0].HTML, "<script>") || !strings.Contains(m.Cards[0].Frames[0].HTML, "&lt;script&gt;") {
		t.Fatalf("unsafe or misleading raw HTML review: %#v", m)
	}
	r, e := s.Push(context.Background(), path)
	if e != nil || r.Status != "completed" || writes != 2 {
		t.Fatalf("normalized HTML create %#v writes %d err %v", r, writes, e)
	}
	if _, e = workspace.ReadDocument(r.Path, &d); e != nil || d.Created.SavedHTML != saved || d.Created.IntendedHTML != body {
		t.Fatalf("normalization evidence lost: %#v err %v", d.Created, e)
	}
}

func TestTeachingFormatsPreserveLegacyMarkdownAndRejectUnsupportedDocuments(t *testing.T) {
	const body = "## Directions\n\nUse **evidence**. <table>"
	old, e := render.MarkdownHTML(body)
	if e != nil {
		t.Fatal(e)
	}
	got, e := render.TeachingHTML(body, "")
	if e != nil || got != old || !strings.Contains(got, "&lt;table&gt;") {
		t.Fatalf("legacy Markdown changed: %q %v", got, e)
	}
	d := pageDraft()
	d.BodyFormat = "richtext"
	if e := d.ValidateBody("body"); e == nil {
		t.Fatal("unsupported Page format accepted")
	}
	a := domain.AssignmentDocument{Schema: 1, Kind: "assignment", Operation: "create", Course: "TEST", Title: "Activity", BodyFormat: "richtext", Points: domain.MustDecimal("20"), SubmissionTypes: []string{"online_text_entry"}}
	if e := domain.ValidateAssignment(a, "body"); e == nil {
		t.Fatal("unsupported assignment format accepted")
	}
	a.BodyFormat = "markdown"
	a.Operation = "update"
	if e := domain.ValidateAssignment(a, "body"); e == nil || !strings.Contains(e.Error(), "existing assignment documents use HTML") {
		t.Fatalf("assignment update interpretation changed: %v", e)
	}
}
