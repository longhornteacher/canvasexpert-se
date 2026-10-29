package workflow

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

func TestApplyBodyReplacementsRequiresExactlyOneMatchUnlessAll(t *testing.T) {
	body := "<p>Due Oct 3.</p><p>Bring a pencil.</p><p>Due Oct 3.</p>"
	if _, counts, err := ApplyBodyReplacements(body, []BodyReplacement{{Find: "Oct 9", Replace: "Oct 10"}}); err == nil || counts[0] != 0 {
		t.Fatalf("no match was accepted: %v %v", counts, err)
	}
	if _, counts, err := ApplyBodyReplacements(body, []BodyReplacement{{Find: "Due Oct 3.", Replace: "Due Oct 10."}}); err == nil || counts[0] != 2 {
		t.Fatalf("ambiguous match was accepted: %v %v", counts, err)
	}
	got, _, err := ApplyBodyReplacements(body, []BodyReplacement{{Find: "Due Oct 3.", Replace: "Due Oct 10.", All: true}, {Find: "a pencil", Replace: "your notebook"}})
	if err != nil || got != "<p>Due Oct 10.</p><p>Bring your notebook.</p><p>Due Oct 10.</p>" {
		t.Fatalf("replacements = %q, %v", got, err)
	}
	// A later failing pair means nothing is applied.
	if got, _, err := ApplyBodyReplacements(body, []BodyReplacement{{Find: "a pencil", Replace: "a pen"}, {Find: "missing", Replace: "x"}}); err == nil || got != "" {
		t.Fatalf("partial edit returned: %q %v", got, err)
	}
}

// pageReplaceFixture serves Page 7; bodies are returned by successive GETs so a
// test can model a teacher editing the Page between the read and the push.
func pageReplaceFixture(t *testing.T, bodies ...string) (*Service, *[]map[string]any) {
	var puts []map[string]any
	gets := 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "GET /api/v1/courses/1/pages/page_id:7":
			body := bodies[min(gets, len(bodies)-1)]
			gets++
			json.NewEncoder(w).Encode(NativePage{PageID: 7, Slug: "unit-3", Title: "Unit 3", Body: body, Published: true, UpdatedAt: "2026-10-01T12:00:00Z"})
		case "GET /api/v1/courses/1/pages/page_id:7/date_details":
			http.NotFound(w, r)
		case "PUT /api/v1/courses/1/pages/page_id:7":
			var payload struct {
				Page map[string]any `json:"wiki_page"`
			}
			json.NewDecoder(r.Body).Decode(&payload)
			puts = append(puts, payload.Page)
			body, _ := payload.Page["body"].(string)
			json.NewEncoder(w).Encode(NativePage{PageID: 7, Slug: "unit-3", Title: "Unit 3", Body: body, Published: true, UpdatedAt: "2026-10-02T12:00:00Z"})
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	})
	return s, &puts
}

const unitPage = "<p>Welcome.</p><p>The essay is due Oct 3.</p>"

func TestReplaceBodyPushesOnlyThePageBody(t *testing.T) {
	s, puts := pageReplaceFixture(t, unitPage)
	r, err := s.ReplaceBody(context.Background(), BodyReplaceRequest{Course: "TEST", Target: "page", Query: "7", Replacements: []BodyReplacement{{Find: "due Oct 3", Replace: "due Oct 10"}}, ExpectUpdatedAt: "2026-10-01T07:00:00-05:00", ExpectBodySHA256: BodySHA256(unitPage), Push: true})
	if err != nil || r.Status != "succeeded" {
		t.Fatalf("replace = %#v, %v", r, err)
	}
	if len(*puts) != 1 || len((*puts)[0]) != 1 || (*puts)[0]["body"] != "<p>Welcome.</p><p>The essay is due Oct 10.</p>" {
		t.Fatalf("Page write was not body-only: %#v", *puts)
	}
}

func TestReplaceBodyHoldsStaleBaselineAndUnmatchedText(t *testing.T) {
	for name, req := range map[string]BodyReplaceRequest{
		"stale updated_at": {Replacements: []BodyReplacement{{Find: "due Oct 3", Replace: "due Oct 10"}}, ExpectUpdatedAt: "2026-09-30T12:00:00Z"},
		"stale digest":     {Replacements: []BodyReplacement{{Find: "due Oct 3", Replace: "due Oct 10"}}, ExpectBodySHA256: BodySHA256("<p>older</p>")},
		"no match":         {Replacements: []BodyReplacement{{Find: "due Oct 4", Replace: "due Oct 10"}}},
		"several matches":  {Replacements: []BodyReplacement{{Find: "<p>", Replace: "<p class=\"x\">"}}},
	} {
		t.Run(name, func(t *testing.T) {
			s, puts := pageReplaceFixture(t, unitPage)
			req.Course, req.Target, req.Query, req.Push = "TEST", "page", "7", true
			r, err := s.ReplaceBody(context.Background(), req)
			if err != nil || r.Status != "held" || len(*puts) != 0 {
				t.Fatalf("replace = %#v, %v, writes %#v", r, err, *puts)
			}
			var d domain.PageDocument
			if body, e := workspace.ReadDocument(r.Path, &d); e != nil || body != unitPage {
				t.Fatalf("held pull was changed: %q %v", body, e)
			}
		})
	}
}

func TestReplaceBodyPreservesEditWhenPageChangesBeforePush(t *testing.T) {
	teacherEdit := "<p>Welcome back.</p><p>The essay is due Oct 3.</p>"
	s, puts := pageReplaceFixture(t, unitPage, teacherEdit)
	r, err := s.ReplaceBody(context.Background(), BodyReplaceRequest{Course: "TEST", Target: "page", Query: "7", Replacements: []BodyReplacement{{Find: "due Oct 3", Replace: "due Oct 10"}}, Push: true})
	if err != nil || r.Status != "held" || len(*puts) != 0 || !strings.Contains(r.Message, "unsent edit preserved") {
		t.Fatalf("replace = %#v, %v, writes %#v", r, err, *puts)
	}
	var d domain.PageDocument
	if body, e := workspace.ReadDocument(r.Path, &d); e != nil || body != "<p>Welcome.</p><p>The essay is due Oct 10.</p>" || d.Observed.BodyHTML != unitPage {
		t.Fatalf("unsent edit not preserved: %q %v", body, e)
	}
}

func TestReplaceBodyPreparesAssignmentDescriptionForReview(t *testing.T) {
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/assignments/10" {
			json.NewEncoder(w).Encode(fixtureAssignment())
			return
		}
		t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		http.NotFound(w, r)
	})
	r, err := s.ReplaceBody(context.Background(), BodyReplaceRequest{Course: "TEST", Target: "assignment", Query: "10", Replacements: []BodyReplacement{{Find: "your evidence", Replace: "your strongest evidence"}}})
	if err != nil || r.Status != "prepared" {
		t.Fatalf("replace = %#v, %v", r, err)
	}
	var d domain.AssignmentDocument
	body, err := workspace.ReadDocument(r.Path, &d)
	if err != nil {
		t.Fatal(err)
	}
	patch, err := PlanEdit(d, body)
	if err != nil || len(patch) != 1 || patch["description"] != "<p>Explain your strongest evidence.</p>" {
		t.Fatalf("assignment patch = %#v, %v", patch, err)
	}
	if _, err = s.ReplaceBody(context.Background(), BodyReplaceRequest{Course: "TEST", Target: "assignment", Query: "Evidence essay", Replacements: []BodyReplacement{{Find: "a", Replace: "b"}}}); err == nil {
		t.Fatal("assignment title accepted as an exact target")
	}
}
