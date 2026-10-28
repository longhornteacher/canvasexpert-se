package workflow

import (
	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

type manageTagsFixture struct {
	setName                                                                           string
	groups                                                                            map[int64]string
	members                                                                           map[int64]map[int64]bool
	structure, adds, removes                                                          int
	failAdd, uncertainStructure, submitted, moduleUse, unknownCoverage, collaborative bool
	uncertainAdd                                                                      bool
}

func newManageTagsFixture() *manageTagsFixture {
	return &manageTagsFixture{setName: "Reading level", groups: map[int64]string{71: "Old", 72: "Core"}, members: map[int64]map[int64]bool{71: {5: true}, 72: {}}}
}
func (f *manageTagsFixture) group(id int64) map[string]any {
	return map[string]any{"id": id, "name": f.groups[id], "course_id": 1, "context_type": "Course", "group_category_id": 50, "non_collaborative": !f.collaborative, "members_count": len(f.members[id])}
}
func (f *manageTagsFixture) handler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		p := r.URL.Path
		switch {
		case r.Method == "GET" && p == "/api/v1/courses/1/group_categories":
			if f.setName == "" {
				fmt.Fprint(w, `[]`)
			} else {
				json.NewEncoder(w).Encode([]any{map[string]any{"id": 50, "name": f.setName, "course_id": 1, "context_type": "Course", "non_collaborative": !f.collaborative}})
			}
		case r.Method == "GET" && p == "/api/v1/group_categories/50":
			json.NewEncoder(w).Encode(map[string]any{"id": 50, "name": f.setName, "course_id": 1, "context_type": "Course", "non_collaborative": !f.collaborative})
		case r.Method == "GET" && p == "/api/v1/group_categories/50/groups":
			all := []any{}
			for id := range f.groups {
				all = append(all, f.group(id))
			}
			json.NewEncoder(w).Encode(all)
		case r.Method == "GET" && p == "/api/v1/courses/1/enrollments":
			fmt.Fprint(w, `[{"user_id":5,"type":"StudentEnrollment","enrollment_state":"active","user":{"id":5}},{"user_id":6,"type":"StudentEnrollment","enrollment_state":"active","user":{"id":6}}]`)
		case r.Method == "GET" && p == "/api/v1/courses/1/bulk_user_tags":
			result := map[string][]int64{}
			for _, key := range r.URL.Query()["user_ids[]"] {
				id, _ := strconv.ParseInt(key, 10, 64)
				result[key] = []int64{}
				for group, members := range f.members {
					if members[id] {
						result[key] = append(result[key], group)
					}
				}
			}
			json.NewEncoder(w).Encode(result)
		case r.Method == "GET" && p == "/api/v1/courses/1/assignments":
			if f.submitted || f.moduleUse {
				fmt.Fprint(w, `[{"id":10,"course_id":1,"name":"Work","points_possible":10,"submission_types":["online_text_entry"]}]`)
			} else {
				fmt.Fprint(w, `[]`)
			}
		case r.Method == "GET" && p == "/api/v1/courses/1/assignments/10/overrides":
			if f.submitted && !f.moduleUse {
				fmt.Fprint(w, `[{"id":1,"group_id":71}]`)
			} else {
				fmt.Fprint(w, `[]`)
			}
		case r.Method == "GET" && strings.HasPrefix(p, "/api/v1/courses/1/assignments/10/submissions/"):
			id, _ := strconv.ParseInt(strings.TrimPrefix(p, "/api/v1/courses/1/assignments/10/submissions/"), 10, 64)
			if f.submitted {
				fmt.Fprintf(w, `{"user_id":%d,"assignment_id":10,"attempt":1,"submitted_at":"2026-10-28T12:00:00Z","body":"Saved work","workflow_state":"submitted","submission_history":[]}`, id)
			} else {
				fmt.Fprintf(w, `{"user_id":%d,"assignment_id":10,"attempt":0,"workflow_state":"unsubmitted","submission_history":[]}`, id)
			}
		case r.Method == "GET" && p == "/api/v1/courses/1/pages":
			if f.unknownCoverage {
				http.Error(w, "PRIVATE", 403)
			} else {
				fmt.Fprint(w, `[]`)
			}
		case r.Method == "GET" && p == "/api/v1/courses/1/discussion_topics":
			fmt.Fprint(w, `[]`)
		case r.Method == "GET" && p == "/api/v1/courses/1/modules":
			if f.moduleUse {
				fmt.Fprint(w, `[{"id":90,"name":"Unit","items_count":1}]`)
			} else {
				fmt.Fprint(w, `[]`)
			}
		case r.Method == "GET" && p == "/api/v1/courses/1/modules/90/assignment_overrides":
			fmt.Fprint(w, `[{"id":1,"group_id":71}]`)
		case r.Method == "GET" && p == "/api/v1/courses/1/modules/90/items":
			fmt.Fprint(w, `[{"id":99,"type":"Assignment","content_id":10,"title":"Work"}]`)
		case r.Method == "GET" && strings.HasPrefix(p, "/api/v1/groups/"):
			id, _ := strconv.ParseInt(strings.TrimPrefix(p, "/api/v1/groups/"), 10, 64)
			if _, ok := f.groups[id]; !ok {
				http.NotFound(w, r)
				return
			}
			json.NewEncoder(w).Encode(f.group(id))
		case r.Method == "POST" && p == "/api/v1/courses/1/group_categories/bulk_manage_differentiation_tag":
			f.structure++
			var request struct {
				Category struct {
					ID   int64  `json:"id"`
					Name string `json:"name"`
				} `json:"group_category"`
				Operations map[string][]managedTag `json:"operations"`
			}
			if e := json.NewDecoder(r.Body).Decode(&request); e != nil {
				t.Error(e)
			}
			if request.Category.Name != "" {
				f.setName = request.Category.Name
			}
			created, updated := []any{}, []any{}
			for _, tag := range request.Operations["delete"] {
				delete(f.groups, tag.ID)
				delete(f.members, tag.ID)
			}
			for _, tag := range request.Operations["create"] {
				id := int64(100 + len(f.groups))
				for f.groups[id] != "" {
					id++
				}
				f.groups[id] = tag.Name
				f.members[id] = map[int64]bool{}
				created = append(created, map[string]any{"group": f.group(id)})
			}
			for _, tag := range request.Operations["update"] {
				f.groups[tag.ID] = tag.Name
				updated = append(updated, map[string]any{"group": f.group(tag.ID)})
			}
			if f.uncertainStructure {
				http.Error(w, "uncertain", 502)
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"group_category": map[string]any{"id": 50, "non_collaborative": true}, "created": created, "updated": updated, "deleted": []any{}})
		case r.Method == "POST" && strings.HasSuffix(p, "/memberships"):
			f.adds++
			parts := strings.Split(p, "/")
			group, _ := strconv.ParseInt(parts[4], 10, 64)
			if f.failAdd {
				http.Error(w, "PRIVATE", 400)
				return
			}
			var request struct {
				Members []int64 `json:"members"`
			}
			json.NewDecoder(r.Body).Decode(&request)
			for _, id := range request.Members {
				f.members[group][id] = true
			}
			if f.uncertainAdd {
				http.Error(w, "uncertain", 502)
				return
			}
			fmt.Fprint(w, `{}`)
		case r.Method == "DELETE" && strings.Contains(p, "/users/"):
			f.removes++
			parts := strings.Split(p, "/")
			group, _ := strconv.ParseInt(parts[4], 10, 64)
			user, _ := strconv.ParseInt(parts[6], 10, 64)
			delete(f.members[group], user)
			fmt.Fprint(w, `{}`)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL)
			http.NotFound(w, r)
		}
	}
}
func tagManageFiles(t *testing.T, s *Service) (string, string, string) {
	t.Helper()
	store, e := workspace.OpenStudents(s.Root)
	if e != nil {
		t.Fatal(e)
	}
	one, e := store.Upsert(s.Client.Origin(), 5, "Private One", "Private", "", "")
	if e != nil {
		t.Fatal(e)
	}
	two, e := store.Upsert(s.Client.Origin(), 6, "Private Two", "Private", "", "")
	if e != nil {
		t.Fatal(e)
	}
	if e = store.Save(); e != nil {
		t.Fatal(e)
	}
	return filepath.Join(s.Root, "courses", "TEST", "tags.md"), one.Ref, two.Ref
}
func writeTags(t *testing.T, path string, d domain.TagsDocument) {
	t.Helper()
	if e := workspace.WriteDocument(path, &d, "Unsent teacher notes.\n"); e != nil {
		t.Fatal(e)
	}
}

func TestSetTagsPreviewThenMoveAndIdempotentRerun(t *testing.T) {
	f := newManageTagsFixture()
	s, _ := workflowFixture(t, f.handler(t))
	path, one, _ := tagManageFiles(t, s)
	d := domain.TagsDocument{Schema: 1, Kind: "tags", Course: "TEST", Set: domain.TagSetChange{Name: "Reading levels"}, Members: &domain.TagMembers{Tags: map[string][]string{"Old": {}, "Core": {one}}}}
	writeTags(t, path, d)
	preview, e := s.SetTags(context.Background(), path, true)
	if e != nil || preview.Status != "preview" || f.adds+f.removes+f.structure != 0 {
		t.Fatalf("preview wrote: %+v %v", preview, e)
	}
	r, e := s.SetTags(context.Background(), path, false)
	if e != nil || r.Status != "completed" || f.adds != 1 || f.removes != 1 || f.members[71][5] || !f.members[72][5] {
		t.Fatalf("move: %+v %v", r, e)
	}
	r, e = s.SetTags(context.Background(), path, false)
	if e != nil || r.Status != "completed" || f.adds != 1 || f.removes != 1 {
		t.Fatalf("rerun repeated writes: %+v %v", r, e)
	}
	b, _ := os.ReadFile(path)
	if strings.Contains(string(b), "user_id") || strings.Contains(string(b), "Private One") || !strings.Contains(string(b), "Unsent teacher notes.") {
		t.Fatalf("public draft lost privacy/notes: %s", b)
	}
}
func TestSetTagsFailedDestinationProtectsOldMembership(t *testing.T) {
	f := newManageTagsFixture()
	f.failAdd = true
	s, _ := workflowFixture(t, f.handler(t))
	path, one, _ := tagManageFiles(t, s)
	writeTags(t, path, domain.TagsDocument{Schema: 1, Kind: "tags", Course: "TEST", Set: domain.TagSetChange{Name: "Reading level"}, Members: &domain.TagMembers{Tags: map[string][]string{"Old": {}, "Core": {one}}}})
	r, e := s.SetTags(context.Background(), path, false)
	if e != nil || r.Status != "partial" || f.removes != 0 || !f.members[71][5] {
		t.Fatalf("failed destination removed source: %+v %v", r, e)
	}
	f.failAdd = false
	r, e = s.SetTags(context.Background(), path, false)
	if e != nil || r.Status != "completed" || f.removes != 1 || f.members[71][5] || !f.members[72][5] {
		t.Fatalf("failed add retry: %+v %v", r, e)
	}
}
func TestSetTagsSubmittedModuleWorkHoldsAtomicDeleteButAddsIndependently(t *testing.T) {
	f := newManageTagsFixture()
	f.submitted, f.moduleUse = true, true
	s, _ := workflowFixture(t, f.handler(t))
	path, _, two := tagManageFiles(t, s)
	writeTags(t, path, domain.TagsDocument{Schema: 1, Kind: "tags", Course: "TEST", Set: domain.TagSetChange{Name: "Reading level"}, Tags: []domain.TagChange{{Name: "Old", Delete: true}, {Name: "New"}}, Members: &domain.TagMembers{Tags: map[string][]string{"Core": {two}}}})
	r, e := s.SetTags(context.Background(), path, false)
	if e != nil || r.Status != "partial" || r.Structure.Status != "held" || f.structure != 0 || f.groups[71] != "Old" || !f.members[72][6] {
		t.Fatalf("unsafe atomic delete or independent add blocked: %+v %v", r, e)
	}
}
func TestSetTagsUncertainStructureResolvesNamesWithoutDuplicate(t *testing.T) {
	f := newManageTagsFixture()
	f.setName = ""
	f.groups = map[int64]string{}
	f.members = map[int64]map[int64]bool{}
	f.uncertainStructure = true
	s, _ := workflowFixture(t, f.handler(t))
	path, one, _ := tagManageFiles(t, s)
	writeTags(t, path, domain.TagsDocument{Schema: 1, Kind: "tags", Course: "TEST", Set: domain.TagSetChange{Name: "New set"}, Tags: []domain.TagChange{{Name: "Support"}}, Members: &domain.TagMembers{Tags: map[string][]string{"Support": {one}}}})
	r, e := s.SetTags(context.Background(), path, false)
	if e != nil || r.Status != "partial" || f.structure != 1 || f.adds != 0 {
		t.Fatalf("uncertain structure: %+v %v", r, e)
	}
	f.uncertainStructure = false
	r, e = s.SetTags(context.Background(), path, false)
	if e != nil || r.Status != "completed" || f.structure != 1 || f.adds != 1 || len(f.groups) != 1 {
		t.Fatalf("uncertain structure duplicated: %+v %v", r, e)
	}
}
func TestSetTagsUnknownCoverageAndChangedMembershipHoldOnlyRemoval(t *testing.T) {
	f := newManageTagsFixture()
	f.unknownCoverage = true
	s, _ := workflowFixture(t, f.handler(t))
	path, one, two := tagManageFiles(t, s)
	writeTags(t, path, domain.TagsDocument{Schema: 1, Kind: "tags", Course: "TEST", Set: domain.TagSetChange{Name: "Reading level"}, Members: &domain.TagMembers{Tags: map[string][]string{"Old": {}, "Core": {two}}}})
	r, e := s.SetTags(context.Background(), path, false)
	if e != nil || r.Status != "partial" || f.removes != 0 || !f.members[72][6] {
		t.Fatalf("unknown coverage lost work or blocked safe add: %+v %v", r, e)
	}
	f.unknownCoverage = false
	d := domain.TagsDocument{Schema: 1, Kind: "tags", Course: "TEST", Set: domain.TagSetChange{Name: "Reading level"}, Members: &domain.TagMembers{Tags: map[string][]string{"Old": {one}}}}
	writeTags(t, path, d)
	if _, e = s.SetTags(context.Background(), path, true); e != nil {
		t.Fatal(e)
	}
	f.members[71][6] = true
	r, e = s.SetTags(context.Background(), path, false)
	if e != nil || r.Status != "partial" || !f.members[71][6] {
		t.Fatalf("changed membership overwritten: %+v %v", r, e)
	}
}
func TestSetTagsRejectsCollaborativeAndWrongCourseBeforeWrites(t *testing.T) {
	f := newManageTagsFixture()
	f.collaborative = true
	s, _ := workflowFixture(t, f.handler(t))
	path, _, _ := tagManageFiles(t, s)
	writeTags(t, path, domain.TagsDocument{Schema: 1, Kind: "tags", Course: "TEST", Set: domain.TagSetChange{ID: 50, Name: "Reading level"}, Tags: []domain.TagChange{{Name: "New"}}})
	if _, e := s.SetTags(context.Background(), path, false); e == nil || f.structure+f.adds+f.removes != 0 {
		t.Fatal("collaborative set accepted")
	}
}

func TestSetTagsUncertainAddIsObservedWithoutReplay(t *testing.T) {
	f := newManageTagsFixture()
	f.uncertainAdd = true
	s, _ := workflowFixture(t, f.handler(t))
	path, one, _ := tagManageFiles(t, s)
	writeTags(t, path, domain.TagsDocument{Schema: 1, Kind: "tags", Course: "TEST", Set: domain.TagSetChange{Name: "Reading level"}, Members: &domain.TagMembers{Tags: map[string][]string{"Old": {}, "Core": {one}}}})
	r, e := s.SetTags(context.Background(), path, false)
	if e != nil || r.Status != "partial" || f.removes != 0 {
		t.Fatalf("uncertain destination did not protect old membership: %+v %v", r, e)
	}
	f.uncertainAdd = false
	r, e = s.SetTags(context.Background(), path, false)
	if e != nil || r.Status != "completed" || f.adds != 1 || f.removes != 1 {
		t.Fatalf("uncertain add was replayed: %+v %v", r, e)
	}
	for _, out := range r.Members {
		if out.Outcome.Status == "uncertain" || out.Outcome.Status == "failed" {
			t.Fatalf("resolved membership still reported as unfinished: %+v", r)
		}
	}
}
