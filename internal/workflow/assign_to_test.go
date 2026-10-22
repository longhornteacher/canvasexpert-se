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
)

// tagCanvas is a small stateful stand-in for the Canvas routes assign-to uses.
type tagCanvas struct {
	moduleRows  map[int64][]map[string]any
	pageOnly    bool
	pageRows    []map[string]any
	assignOnly  bool
	assignRows  []map[string]any
	submitted   int
	writes      []string
	nextID      int64
	stagingItem bool
}

func (c *tagCanvas) handler(t *testing.T) http.HandlerFunc {
	groups := []map[string]any{
		{"id": 21192, "name": "Alone", "course_id": 1, "context_type": "Course", "group_category_id": 5316, "non_collaborative": true, "members_count": 6},
		{"id": 21193, "name": "Ghost", "course_id": 1, "context_type": "Course", "group_category_id": 5316, "non_collaborative": true, "members_count": 5},
	}
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		write := func(v any) { _ = json.NewEncoder(w).Encode(v) }
		var body map[string]any
		if r.Method != "GET" {
			c.writes = append(c.writes, r.Method+" "+r.URL.Path)
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		p := r.URL.Path
		switch {
		case p == "/api/v1/courses/1/group_categories":
			write([]map[string]any{{"id": 5316, "name": "Book Club", "course_id": 1, "context_type": "Course", "non_collaborative": true}})
		case p == "/api/v1/group_categories/5316/groups":
			write(groups)
		case p == "/api/v1/courses/1/modules":
			ghostItems := []map[string]any{{"id": 1, "title": "Ghost Study Guide", "type": "Page", "page_url": "ghost-study-guide"}, {"id": 2, "title": "Ghost: Quiz 1", "type": "Assignment", "content_id": 700}}
			mods := []map[string]any{{"id": 10, "name": "Book Club: Ghost", "items": ghostItems}}
			if c.stagingItem {
				mods = append(mods, map[string]any{"id": 11, "name": "Staging", "items": ghostItems[:1]})
			}
			write(mods)
		case strings.HasPrefix(p, "/api/v1/courses/1/modules/") && strings.HasSuffix(p, "/assignment_overrides"):
			var id int64
			_, _ = fmt.Sscanf(p, "/api/v1/courses/1/modules/%d/assignment_overrides", &id)
			if r.Method == "PUT" {
				rows := []map[string]any{}
				for _, o := range body["overrides"].([]any) {
					m := o.(map[string]any)
					c.nextID++
					rows = append(rows, map[string]any{"id": c.nextID, "context_module_id": id, "title": "x", "group_id": m["group_id"]})
				}
				c.moduleRows[id] = rows
				w.WriteHeader(http.StatusNoContent)
				return
			}
			write(c.moduleRows[id])
		case p == "/api/v1/courses/1/pages":
			write([]map[string]any{{"page_id": 55, "url": "ghost-study-guide", "title": "Ghost Study Guide"}})
		case strings.HasSuffix(p, "/date_details") && strings.Contains(p, "/pages/"):
			if r.Method == "PUT" {
				c.pageOnly = body["only_visible_to_overrides"].(bool)
				c.pageRows = nil
				for _, o := range body["assignment_overrides"].([]any) {
					m := o.(map[string]any)
					c.nextID++
					c.pageRows = append(c.pageRows, map[string]any{"id": c.nextID, "title": "t", "group_id": m["group_id"]})
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}
			rows := c.pageRows
			if rows == nil {
				rows = []map[string]any{}
			}
			write(map[string]any{"id": 55, "only_visible_to_overrides": c.pageOnly, "overrides": rows})
		case p == "/api/v1/courses/1/assignments":
			write([]map[string]any{{"id": 700, "course_id": 1, "name": "Ghost: Quiz 1", "is_quiz_lti_assignment": true}})
		case p == "/api/v1/courses/1/assignments/700/submissions":
			subs := []map[string]any{}
			for i := 0; i < c.submitted; i++ {
				subs = append(subs, map[string]any{"id": i + 1, "user_id": i + 1, "submitted_at": "2026-10-20T10:00:00Z"})
			}
			write(subs)
		case p == "/api/v1/courses/1/assignments/700/overrides" && r.Method == "POST":
			c.nextID++
			o := body["assignment_override"].(map[string]any)
			c.assignRows = append(c.assignRows, map[string]any{"id": c.nextID, "assignment_id": 700, "title": "Ghost", "group_id": o["group_id"]})
			write(map[string]any{"id": c.nextID})
		case strings.HasPrefix(p, "/api/v1/courses/1/assignments/700/overrides/") && r.Method == "DELETE":
			var id int64
			_, _ = fmt.Sscanf(p, "/api/v1/courses/1/assignments/700/overrides/%d", &id)
			kept := []map[string]any{}
			for _, row := range c.assignRows {
				if int64(row["id"].(int64)) != id {
					kept = append(kept, row)
				}
			}
			c.assignRows = kept
			write(map[string]any{})
		case p == "/api/v1/courses/1/assignments/700" && r.Method == "PUT":
			c.assignOnly = body["assignment"].(map[string]any)["only_visible_to_overrides"].(bool)
			write(map[string]any{"id": 700})
		case p == "/api/v1/courses/1/assignments/700":
			write(map[string]any{"id": 700, "course_id": 1, "name": "Ghost: Quiz 1", "is_quiz_lti_assignment": true, "only_visible_to_overrides": c.assignOnly, "overrides": c.assignRows})
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func writeAssignDoc(t *testing.T, root, text string) string {
	t.Helper()
	path := filepath.Join(root, "courses", "TEST", "drafts", "assign.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

const assignDoc = `---
schema: 1
kind: assign-to
course: TEST
targets:
  - module: "Book Club: Ghost"
    tags: ["Book Clubs/Ghost"]
  - page: "Ghost Study Guide"
    tags: ["Ghost"]
  - quiz: "Ghost: Quiz 1"
    tags: ["Book Club/Ghost"]
---
`

func TestListTagsCountsOnly(t *testing.T) {
	c := &tagCanvas{moduleRows: map[int64][]map[string]any{}}
	s, _ := workflowFixture(t, c.handler(t))
	cat, err := s.ReadTags(context.Background(), "TEST")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(cat)
	if !strings.Contains(string(b), `"tag_set_id":5316`) || !strings.Contains(string(b), `"tag_id":21193,"name":"Ghost","member_count":5`) {
		t.Fatalf("unexpected catalog: %s", b)
	}
}

func TestAssignToPreviewThenApplyIsIdempotentAndWarns(t *testing.T) {
	c := &tagCanvas{moduleRows: map[int64][]map[string]any{}, stagingItem: true}
	s, _ := workflowFixture(t, c.handler(t))
	path := writeAssignDoc(t, s.Root, assignDoc)

	preview, err := s.PreviewAssignTo(context.Background(), path)
	if err != nil || preview.Status != "preview" || len(c.writes) != 0 {
		t.Fatalf("preview must not write: %v %#v writes=%v", err, preview, c.writes)
	}
	for _, r := range preview.Results {
		if r.Before != "Everyone" || !strings.Contains(r.After, "Ghost (5)") {
			t.Errorf("preview diff for %s: %q -> %q", r.Target, r.Before, r.After)
		}
	}

	report, err := s.PushAssignTo(context.Background(), path)
	if err != nil || report.Status != "completed" {
		t.Fatalf("push: %v %#v", err, report)
	}
	if len(c.moduleRows[10]) != 1 || c.moduleRows[10][0]["group_id"] != float64(21193) && c.moduleRows[10][0]["group_id"] != 21193.0 {
		t.Fatalf("module not assigned: %v", c.moduleRows[10])
	}
	if !c.pageOnly || len(c.pageRows) != 1 || !c.assignOnly || len(c.assignRows) != 1 {
		t.Fatalf("page/quiz not assigned: page=%v/%v quiz=%v/%v", c.pageOnly, c.pageRows, c.assignOnly, c.assignRows)
	}
	warned := false
	for _, n := range report.Results[0].Notes {
		if strings.Contains(n, `"Staging"`) {
			warned = true
		}
	}
	if !warned {
		t.Errorf("expected a warning about the Staging module, got %v", report.Results[0].Notes)
	}
	before := len(c.writes)
	again, err := s.PushAssignTo(context.Background(), path)
	if err != nil || len(c.writes) != before {
		t.Fatalf("rerun replayed writes: %v %v", err, c.writes[before:])
	}
	for _, r := range again.Results {
		if r.Status != "unchanged" {
			t.Errorf("%s: %s", r.Target, r.Status)
		}
	}
	saved, _ := os.ReadFile(path)
	if !strings.Contains(string(saved), "status: succeeded") {
		t.Errorf("outcomes were not saved:\n%s", saved)
	}
}

func TestAssignToHoldsAmbiguousAndMissingNamesAndContinues(t *testing.T) {
	c := &tagCanvas{moduleRows: map[int64][]map[string]any{}}
	s, _ := workflowFixture(t, c.handler(t))
	path := writeAssignDoc(t, s.Root, `---
schema: 1
kind: assign-to
course: TEST
targets:
  - module: "Book Club: Nope"
    tags: ["Ghost"]
  - module: "Book Club: Ghost"
    tags: ["Book Club/Mystery"]
  - module: "Book Club: Ghost"
    tags: ["Ghost"]
---
`)
	report, err := s.PushAssignTo(context.Background(), path)
	if err != nil || report.Status != "partial" {
		t.Fatalf("%v %#v", err, report)
	}
	if report.Results[0].Status != "held" || report.Results[1].Status != "held" || report.Results[2].Status != "succeeded" {
		t.Fatalf("statuses: %#v", report.Results)
	}
	if len(c.moduleRows[10]) != 1 {
		t.Fatal("independent target did not proceed")
	}
}

func TestAssignToProtectsSubmittedWork(t *testing.T) {
	c := &tagCanvas{moduleRows: map[int64][]map[string]any{}, submitted: 2}
	s, _ := workflowFixture(t, c.handler(t))
	path := writeAssignDoc(t, s.Root, `---
schema: 1
kind: assign-to
course: TEST
targets:
  - quiz: "Ghost: Quiz 1"
    tags: ["Ghost"]
---
`)
	report, err := s.PushAssignTo(context.Background(), path)
	if err != nil || report.Results[0].Status != "held" || !strings.Contains(report.Results[0].Message, "2 student(s) already submitted") || len(c.writes) != 0 {
		t.Fatalf("expected a hold with no writes: %v %#v %v", err, report.Results, c.writes)
	}
	allowed := strings.Replace(assignDocSubmitted, "ALLOW", "allow_remove_with_submissions: true", 1)
	path = writeAssignDoc(t, s.Root, allowed)
	report, err = s.PushAssignTo(context.Background(), path)
	if err != nil || report.Results[0].Status != "succeeded" || !c.assignOnly {
		t.Fatalf("authorized change did not apply: %v %#v", err, report.Results)
	}
}

const assignDocSubmitted = `---
schema: 1
kind: assign-to
course: TEST
ALLOW
targets:
  - quiz: "Ghost: Quiz 1"
    tags: ["Ghost"]
---
`

func TestAssignModulesByPattern(t *testing.T) {
	c := &tagCanvas{moduleRows: map[int64][]map[string]any{}}
	s, _ := workflowFixture(t, c.handler(t))
	report, err := s.AssignModulesByTag(context.Background(), "TEST", AssignModulesOptions{Pattern: "Book Club: {tag}", TagSet: "Book Clubs"})
	if err != nil || report.Status != "completed" || len(report.Results) != 1 || len(c.moduleRows[10]) != 1 {
		t.Fatalf("%v %#v", err, report)
	}
	if _, err := os.Stat(report.Path); err != nil {
		t.Fatalf("assign-to record was not saved: %v", err)
	}
}

func TestSetFileAccessSendsMinimalPayloadAndReadsBack(t *testing.T) {
	s, path, state := uploadWorkflowFixture(t)
	_ = path
	report, err := s.SetFileAccess(context.Background(), "TEST", 9, "hidden")
	if err != nil || report.Status != "completed" || state.file.Hidden == nil || !*state.file.Hidden {
		t.Fatalf("hidden not applied: %v %#v", err, report)
	}
	again, err := s.SetFileAccess(context.Background(), "TEST", 9, "hidden")
	if err != nil || !strings.Contains(again.Message, "nothing written") {
		t.Fatalf("second call should write nothing: %v %#v", err, again)
	}
}
