package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

type fakePage struct {
	ID        int64
	Title     string
	Published bool
	Everyone  bool
	Groups    []int64
}

// fakeHubCanvas is a small stateful Canvas: pages, date_details and tags.
type fakeHubCanvas struct {
	mu             sync.Mutex
	pages          map[int64]*fakePage
	next           int64
	groups         map[string][]int64 // tag name -> group ids (a name twice is ambiguous)
	refuseAssign   bool
	leakOnPublish  bool
	failAssignment bool
	log            []string
	assignments    int
	assignment     map[string]any
}

func newFakeHub() *fakeHubCanvas {
	return &fakeHubCanvas{pages: map[int64]*fakePage{}, next: 100, groups: map[string][]int64{}}
}

func (f *fakeHubCanvas) count(prefix string) int {
	n := 0
	for _, l := range f.log {
		if strings.HasPrefix(l, prefix) {
			n++
		}
	}
	return n
}

func (f *fakeHubCanvas) handler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		path := r.URL.Path
		body := map[string]any{}
		if r.Method == "PUT" || r.Method == "POST" {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		write := func(v any) { w.Header().Set("Content-Type", "application/json"); _ = json.NewEncoder(w).Encode(v) }
		page := func(p *fakePage) map[string]any {
			return map[string]any{"page_id": p.ID, "url": "slug-" + strconv.FormatInt(p.ID, 10), "html_url": "https://x/courses/1/pages/slug", "title": p.Title, "published": p.Published}
		}
		switch {
		case path == "/api/v1/courses/1/group_categories":
			if len(f.groups) == 0 {
				write([]any{})
			} else {
				write([]map[string]any{{"id": 5}})
			}
		case path == "/api/v1/group_categories/5/groups":
			rows := []map[string]any{}
			for name, ids := range f.groups {
				for _, id := range ids {
					rows = append(rows, map[string]any{"id": id, "name": name})
				}
			}
			write(rows)
		case path == "/api/v1/courses/1/pages" && r.Method == "GET":
			rows := []map[string]any{}
			for _, p := range f.pages {
				if strings.Contains(strings.ToLower(p.Title), strings.ToLower(r.URL.Query().Get("search_term"))) {
					rows = append(rows, page(p))
				}
			}
			write(rows)
		case path == "/api/v1/courses/1/pages" && r.Method == "POST":
			f.log = append(f.log, "POST page")
			wp := body["wiki_page"].(map[string]any)
			f.next++
			p := &fakePage{ID: f.next, Title: wp["title"].(string), Published: wp["published"] == true, Everyone: true}
			f.pages[p.ID] = p
			write(page(p))
		case path == "/api/v1/courses/1/assignments" && r.Method == "POST":
			f.log = append(f.log, "POST assignment")
			if f.failAssignment {
				http.Error(w, "{}", 422)
				return
			}
			f.assignments++
			a := body["assignment"].(map[string]any)
			f.assignment = map[string]any{"id": 900, "course_id": 1, "name": a["name"], "points_possible": a["points_possible"], "description": a["description"], "grading_type": "points", "submission_types": a["submission_types"], "published": false, "html_url": "https://x/courses/1/assignments/900", "created_at": "2026-10-09T12:00:00Z"}
			write(f.assignment)
		case path == "/api/v1/courses/1/assignments/900" && r.Method == "GET":
			// Canvas returns course links as absolute URLs with data-api attributes.
			live := map[string]any{}
			for k, v := range f.assignment {
				live[k] = v
			}
			live["description"] = strings.ReplaceAll(live["description"].(string), `href="/courses/`, `data-api-returntype="Page" href="https://x/courses/`)
			write(live)
		default:
			rest := strings.TrimPrefix(path, "/api/v1/courses/1/pages/")
			dates := strings.HasSuffix(rest, "/date_details")
			id, err := strconv.ParseInt(strings.TrimSuffix(rest, "/date_details"), 10, 64)
			p := f.pages[id]
			if err != nil || p == nil {
				t.Errorf("unexpected request %s %s", r.Method, path)
				http.NotFound(w, r)
				return
			}
			switch {
			case dates && r.Method == "GET":
				overrides := []map[string]any{}
				for _, g := range p.Groups {
					overrides = append(overrides, map[string]any{"group_id": g})
				}
				write(map[string]any{"visible_to_everyone": p.Everyone, "overrides": overrides})
			case dates && r.Method == "PUT":
				if raw, _ := body["assignment_overrides"].([]any); len(raw) > 0 && f.refuseAssign {
					http.Error(w, `{"errors":"group_id is not valid"}`, 422)
					return
				}
				f.log = append(f.log, fmt.Sprintf("PUT dates %d", id))
				p.Everyone = body["only_visible_to_overrides"] != true
				p.Groups = nil
				for _, o := range body["assignment_overrides"].([]any) {
					p.Groups = append(p.Groups, int64(o.(map[string]any)["group_id"].(float64)))
				}
				w.WriteHeader(204)
			case r.Method == "GET":
				write(page(p))
			case r.Method == "PUT":
				pub := body["wiki_page"].(map[string]any)["published"] == true
				f.log = append(f.log, fmt.Sprintf("PUT published=%t %d", pub, id))
				p.Published = pub
				if pub && f.leakOnPublish {
					p.Everyone = true
				}
				write(page(p))
			}
		}
	}
}

func hubTiers() []HubTier {
	return []HubTier{
		{Label: "Silver", Tag: "Silver", Title: "Hub - Silver", BodyHTML: "<p>silver</p>"},
		{Label: "Red", Tag: "Red", Title: "Hub - Red", BodyHTML: "<p>red</p>"},
		{Label: "Blue", Tag: "Blue", Title: "Hub - Blue", BodyHTML: "<p>blue</p>"},
	}
}

func TestDeliverTierPagesTagResolutionAndRestrictionBeforePublish(t *testing.T) {
	f := newFakeHub()
	f.groups = map[string][]int64{"silver": {7}, "Blue": {8, 9}} // case-insensitive match; Blue is ambiguous; Red missing
	s, _ := workflowFixture(t, f.handler(t))
	pages, err := s.DeliverTierPages(context.Background(), 1, hubTiers(), true, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"Silver": "matched", "Red": "not_found", "Blue": "ambiguous"}
	for _, p := range pages {
		if p.TagStatus != want[p.Label] || !p.Published || p.PageID == 0 || p.Slug == "" {
			t.Fatalf("page %+v, want status %s", p, want[p.Label])
		}
		live := f.pages[p.PageID]
		if live.Everyone || !live.Published {
			t.Fatalf("page %s must be restricted and published: %+v", p.Label, live)
		}
		if p.Label == "Silver" && (len(live.Groups) != 1 || live.Groups[0] != 7 || p.TeacherAction != "") {
			t.Fatalf("matched page not assigned to group 7: %+v %+v", live, p)
		}
		if p.Label != "Silver" && (len(live.Groups) != 0 || !strings.Contains(p.TeacherAction, p.Title) || !strings.Contains(p.TeacherAction, p.Tag)) {
			t.Fatalf("unresolved page must be nobody-assigned with a teacher action: %+v %+v", live, p)
		}
	}
	// Ordering: every page's restriction is written before that page is published.
	for _, p := range pages {
		restrict, publish := -1, -1
		for i, l := range f.log {
			if l == fmt.Sprintf("PUT dates %d", p.PageID) && restrict < 0 {
				restrict = i
			}
			if l == fmt.Sprintf("PUT published=true %d", p.PageID) {
				publish = i
			}
		}
		if restrict < 0 || publish < restrict {
			t.Fatalf("page %d published before restriction: %v", p.PageID, f.log)
		}
	}
}

func TestDeliverTierPagesUnpublishedWhenNotRequested(t *testing.T) {
	f := newFakeHub()
	s, _ := workflowFixture(t, f.handler(t))
	pages, err := s.DeliverTierPages(context.Background(), 1, hubTiers()[:1], false, nil, nil)
	if err != nil || pages[0].Published || f.pages[pages[0].PageID].Published || f.pages[pages[0].PageID].Everyone {
		t.Fatalf("pages=%+v err=%v", pages, err)
	}
}

func TestDeliverTierPagesRefusedTagStaysRestricted(t *testing.T) {
	f := newFakeHub()
	f.groups = map[string][]int64{"Silver": {7}}
	f.refuseAssign = true
	s, _ := workflowFixture(t, f.handler(t))
	pages, err := s.DeliverTierPages(context.Background(), 1, hubTiers()[:1], true, nil, nil)
	if err != nil || pages[0].TagStatus != "unavailable" || pages[0].TeacherAction == "" {
		t.Fatalf("pages=%+v err=%v", pages, err)
	}
	if live := f.pages[pages[0].PageID]; live.Everyone || len(live.Groups) != 0 {
		t.Fatalf("refused assignment must leave the page restricted and unassigned: %+v", live)
	}
}

func TestDeliverTierPagesFailsClosedWhenPublishExposesPage(t *testing.T) {
	f := newFakeHub()
	f.leakOnPublish = true
	s, _ := workflowFixture(t, f.handler(t))
	pages, err := s.DeliverTierPages(context.Background(), 1, hubTiers()[:1], true, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "unpublished again") {
		t.Fatalf("want fail-closed error, got %v", err)
	}
	if f.pages[pages[0].PageID].Published {
		t.Fatal("page visible to everyone must be unpublished immediately")
	}
}

func TestDeliverTierPagesRefusesTitleCollisionBeforeWriting(t *testing.T) {
	f := newFakeHub()
	f.pages[50] = &fakePage{ID: 50, Title: "hub - silver", Published: true, Everyone: true}
	s, _ := workflowFixture(t, f.handler(t))
	_, err := s.DeliverTierPages(context.Background(), 1, hubTiers(), true, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "already exists") || len(f.log) != 0 {
		t.Fatalf("collision must refuse before any write: err=%v log=%v", err, f.log)
	}
}

func TestDeliverTierPagesResumeVerifiesKnownPagesAndKeepsTeacherAssignment(t *testing.T) {
	f := newFakeHub()
	s, _ := workflowFixture(t, f.handler(t))
	first, err := s.DeliverTierPages(context.Background(), 1, hubTiers()[:2], false, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	// The teacher assigns the Silver page by hand in Canvas, then the tag also appears.
	f.pages[first[0].PageID].Groups = []int64{42}
	f.groups = map[string][]int64{"Silver": {7}}
	before := f.count("POST page")
	second, err := s.DeliverTierPages(context.Background(), 1, hubTiers()[:2], true, first, nil)
	if err != nil || f.count("POST page") != before {
		t.Fatalf("resume must not create pages: err=%v log=%v", err, f.log)
	}
	if second[0].PageID != first[0].PageID || second[1].PageID != first[1].PageID {
		t.Fatalf("known ids changed: %+v vs %+v", first, second)
	}
	if got := f.pages[first[0].PageID].Groups; len(got) != 1 || got[0] != 42 || second[0].TagStatus != "manual" || second[0].TeacherAction != "" {
		t.Fatalf("teacher's assignment must be preserved: %v %+v", got, second[0])
	}
	if !second[0].Published || !second[1].Published {
		t.Fatalf("resume with publish intent must publish: %+v", second)
	}
}

func TestDeliverTierPagesDoesNotRecreateWhenKnownPageUnreadable(t *testing.T) {
	f := newFakeHub()
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			t.Error("must not create a page for a known id")
		}
		http.Error(w, "{}", 404)
	})
	known := []TierPage{{Label: "Silver", PageID: 123, Slug: "s"}}
	if _, err := s.DeliverTierPages(context.Background(), 1, hubTiers()[:1], false, known, nil); err == nil {
		t.Fatal("unreadable known page must be an error")
	}
	_ = f
}

func TestBindTierLinks(t *testing.T) {
	pages := []TierPage{{Label: "Silver", Slug: "hub-silver"}, {Label: "Red", Slug: "hub-red"}}
	got, err := BindTierLinks("[S]({{tier:Silver}}) and [R]({{tier:Red}})", pages, 1)
	if err != nil || got != "[S](/courses/1/pages/hub-silver) and [R](/courses/1/pages/hub-red)" {
		t.Fatalf("got %q %v", got, err)
	}
	if _, err = BindTierLinks("[S]({{tier:Silver}})", pages, 1); err == nil {
		t.Fatal("page without a token must be an error")
	}
	if _, err = BindTierLinks("{{tier:Silver}} {{tier:Red}} {{tier:Blue}}", pages, 1); err == nil {
		t.Fatal("token without a page must be an error")
	}
}

const hubDocBody = "Read the directions.\n\n- [Silver supports]({{tier:Silver}})\n- [Red supports]({{tier:Red}})\n"

func writeHubDoc(t *testing.T, s *Service, tiers []domain.Tier, body string) string {
	t.Helper()
	doc := domain.AssignmentDocument{Schema: 1, Kind: "assignment", Operation: "create", Course: "TEST", Title: "Hub", Points: domain.MustDecimal("10"), SubmissionTypes: []string{"online_text_entry"}, Tiers: tiers}
	path := filepath.Join(s.Root, "hub.md")
	if err := workspace.WriteDocument(path, doc, body); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestPushHubCreatesLinkedPagesAndResumesWithoutDuplicates(t *testing.T) {
	f := newFakeHub()
	f.groups = map[string][]int64{"Silver": {7}}
	f.failAssignment = true
	s, _ := workflowFixture(t, f.handler(t))
	path := writeHubDoc(t, s, []domain.Tier{
		{Label: "Silver", Tag: "Silver", Supports: "Sentence frames."},
		{Label: "Red", Tag: "Red", Supports: "Guide."},
		{Label: "Blue", Tag: "Blue"},
	}, hubDocBody)
	if _, err := workspace.ReadDocument(path, &domain.AssignmentDocument{}); err != nil {
		t.Fatalf("document should validate: %v", err)
	}
	report, err := s.Push(context.Background(), path)
	if err != nil || report.Status != "failed" || len(report.TierPages) != 2 || f.count("POST page") != 2 {
		t.Fatalf("interrupted push: %+v err=%v log=%v", report, err, f.log)
	}
	for _, want := range []string{"Hub - Silver", "Hub - Red", "matched and assigned", "not_found", "teacher action: assign page \"Hub - Red\" to tag \"Red\""} {
		if !strings.Contains(report.Message, want) {
			t.Errorf("report lacks %q:\n%s", want, report.Message)
		}
	}
	f.failAssignment = false
	report, err = s.Push(context.Background(), path)
	if err != nil || report.Status != "completed" || f.count("POST page") != 2 || f.assignments != 1 {
		t.Fatalf("resume: %+v err=%v log=%v", report, err, f.log)
	}
	moved, readErr := os.ReadFile(report.Path)
	if readErr != nil || strings.Contains(string(moved), "{{tier:") == false {
		t.Fatalf("authoring file keeps its tokens: %v", readErr)
	}
	if !strings.Contains(string(moved), "tier_pages:") {
		t.Fatal("created block must record tier pages")
	}
	silver, red := f.pages[101], f.pages[102]
	if silver.Title != "Hub - Silver" || red.Title != "Hub - Red" {
		t.Fatalf("pages: %+v %+v", silver, red)
	}
}

func TestPushHubStopsOnCollisionAndRejectsTiersOnUpdate(t *testing.T) {
	f := newFakeHub()
	f.pages[50] = &fakePage{ID: 50, Title: "Hub - Silver"}
	s, _ := workflowFixture(t, f.handler(t))
	path := writeHubDoc(t, s, []domain.Tier{{Label: "Silver", Tag: "Silver", Supports: "x"}, {Label: "Red", Tag: "Red"}}, "[S]({{tier:Silver}})")
	report, err := s.Push(context.Background(), path)
	if err != nil || report.Status != "held" || f.count("POST") != 0 || !strings.Contains(report.Message, "already exists") {
		t.Fatalf("collision: %+v err=%v log=%v", report, err, f.log)
	}
	var doc domain.AssignmentDocument
	if _, err = workspace.ReadDocument(path, &doc); err != nil || doc.Created != nil {
		t.Fatalf("nothing should be recorded after a refused collision: %v %+v", err, doc.Created)
	}
	doc.Operation = "update"
	if e := domain.ValidateAssignment(doc, "[S]({{tier:Silver}})"); e == nil || !strings.Contains(e.Error(), "only supported when creating") {
		t.Fatalf("update with tiers must be rejected: %v", e)
	}
}

func TestTierValidation(t *testing.T) {
	base := domain.AssignmentDocument{Schema: 1, Kind: "assignment", Operation: "create", Course: "TEST", Title: "Hub", Points: domain.MustDecimal("1"), SubmissionTypes: []string{"online_text_entry"}}
	for name, tc := range map[string]struct {
		tiers []domain.Tier
		body  string
		ok    bool
	}{
		"fine":                {[]domain.Tier{{Label: "A", Tag: "Silver", Supports: "s"}}, "[a]({{tier:A}})", true},
		"no page needs none":  {[]domain.Tier{{Label: "A", Tag: "Silver"}}, "plain", true},
		"missing token":       {[]domain.Tier{{Label: "A", Tag: "Silver", Supports: "s"}}, "plain", false},
		"two tokens":          {[]domain.Tier{{Label: "A", Tag: "Silver", Supports: "s"}}, "{{tier:A}} {{tier:A}}", false},
		"token no page":       {[]domain.Tier{{Label: "A", Tag: "Silver"}}, "{{tier:A}}", false},
		"unknown token":       {[]domain.Tier{{Label: "A", Tag: "Silver", Supports: "s"}}, "{{tier:A}} {{tier:Z}}", false},
		"duplicate label":     {[]domain.Tier{{Label: "A", Tag: "S", Supports: "s"}, {Label: "a", Tag: "R", Supports: "s"}}, "{{tier:A}} {{tier:a}}", false},
		"empty tag":           {[]domain.Tier{{Label: "A", Supports: "s"}}, "{{tier:A}}", false},
		"token without tiers": {nil, "{{tier:A}}", false},
	} {
		d := base
		d.Tiers = tc.tiers
		if err := domain.ValidateAssignment(d, tc.body); (err == nil) != tc.ok {
			t.Errorf("%s: ok=%v err=%v", name, tc.ok, err)
		}
	}
}
