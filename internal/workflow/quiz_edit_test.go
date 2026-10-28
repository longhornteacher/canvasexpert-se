package workflow

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

type contentEditFixture struct {
	quiz            map[string]any
	items           []map[string]any
	patches         []map[string]any
	timeout         bool
	readsAfterWrite int
}

func newContentEditFixture() *contentEditFixture {
	f := &contentEditFixture{quiz: map[string]any{"id": "10", "title": "Practice", "instructions": "<p>Read.</p>"}}
	for i, typ := range []string{"Item", "Item", "Stimulus", "Bank", "BankEntry"} {
		f.items = append(f.items, map[string]any{"id": []string{"71", "72", "73", "74", "75"}[i], "position": i + 1, "entry_type": typ, "entry_editable": true, "status": "mutable", "entry": map[string]any{"id": []string{"81", "82", "83", "84", "85"}[i], "interaction_type_slug": "essay", "title": "Question", "item_body": "<p>Explain.</p>", "feedback": map[string]any{"neutral": "<p>Try.</p>", "correct": "<b>Yes</b>", "incorrect": nil, "future_key": map[string]any{"text": "Keep"}}}})
	}
	return f
}

func (f *contentEditFixture) handler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		base := "/api/quiz/v1/courses/1/quizzes/10"
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "PATCH" {
			var payload map[string]any
			if e := json.NewDecoder(r.Body).Decode(&payload); e != nil {
				t.Error(e)
			}
			f.patches = append(f.patches, payload)
			if r.URL.Path == base {
				if len(payload) != 1 {
					t.Errorf("wrong quiz wrapper: %v", payload)
				}
				for k, v := range payload["quiz"].(map[string]any) {
					f.quiz[k] = v
				}
			} else {
				for _, it := range f.items {
					if r.URL.Path == base+"/items/"+it["id"].(string) {
						wrapper := payload["item"].(map[string]any)
						if len(payload) != 1 || len(wrapper) != 1 {
							t.Errorf("unsafe wrapper: %v", payload)
						}
						entry := it["entry"].(map[string]any)
						for k, v := range wrapper["entry"].(map[string]any) {
							if k != "title" && k != "item_body" && k != "feedback" {
								t.Errorf("unrelated field written: %s", k)
							}
							entry[k] = v
						}
					}
				}
			}
			if f.timeout {
				conn, _, e := w.(http.Hijacker).Hijack()
				if e != nil {
					t.Error(e)
				} else {
					conn.Close()
				}
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != "GET" {
			t.Errorf("unexpected write %s %s", r.Method, r.URL.Path)
			w.WriteHeader(500)
			return
		}
		if len(f.patches) > 0 {
			f.readsAfterWrite++
		}
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v1/courses/1/assignments/10"):
			a := fixtureAssignment()
			a.NewQuiz = true
			json.NewEncoder(w).Encode(a)
		case r.URL.Path == base:
			json.NewEncoder(w).Encode(f.quiz)
		case r.URL.Path == base+"/items":
			if r.URL.Query().Get("page") == "2" {
				json.NewEncoder(w).Encode(f.items[2:])
				return
			}
			w.Header().Set("Link", "<https://"+r.Host+base+"/items?page=2>; rel=\"next\"")
			json.NewEncoder(w).Encode(f.items[:2])
		default:
			for _, it := range f.items {
				if r.URL.Path == base+"/items/"+it["id"].(string) {
					json.NewEncoder(w).Encode(it)
					return
				}
			}
			t.Errorf("unexpected GET %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}
}

func pullContentEdit(t *testing.T, s *Service) (string, domain.NewQuizEditDocument) {
	t.Helper()
	p, e := s.PullQuiz(context.Background(), "TEST", 10)
	if e != nil {
		t.Fatal(e)
	}
	var d domain.NewQuizEditDocument
	if _, e = workspace.ReadDocument(p, &d); e != nil {
		t.Fatal(e)
	}
	return p, d
}

func TestQuizContentEditPullNarrowPatchesAndReplay(t *testing.T) {
	f := newContentEditFixture()
	s, _ := workflowFixture(t, f.handler(t))
	oldLink := `<p><a href="` + s.Client.Origin() + `/courses/1/files/98/download?download_frd=1">Handout</a></p>`
	f.items[0]["entry"].(map[string]any)["item_body"] = oldLink
	f.items[1]["entry"].(map[string]any)["item_body"] = oldLink
	p, d := pullContentEdit(t, s)
	if len(d.Items) != 5 || !d.Items[0].Editable || d.Items[2].Editable || d.Items[3].Editable || d.Items[4].Editable {
		t.Fatalf("mixed quiz lost: %+v", d.Items)
	}
	d.Changes = map[string]any{"instructions": ""}
	d.Items[0].Changes = map[string]any{"item_body": `<p><a href="https://canvas.example.edu/courses/1/files/99/download?download_frd=1">Revised handout</a></p>`}
	d.Items[1].Changes = map[string]any{"feedback": map[string]any{"neutral": ""}}
	if e := workspace.WriteDocument(p, &d, "Unsent notes"); e != nil {
		t.Fatal(e)
	}
	before, e := os.ReadFile(p)
	if e != nil {
		t.Fatal(e)
	}
	p2, _ := pullContentEdit(t, s)
	after, _ := os.ReadFile(p)
	if p == p2 || string(before) != string(after) {
		t.Fatal("pull replaced unsent draft")
	}
	// An unrelated live title and sibling feedback change must survive.
	f.items[0]["entry"].(map[string]any)["title"] = "Teacher's new title"
	f.items[1]["entry"].(map[string]any)["feedback"].(map[string]any)["correct"] = "Teacher's new correct feedback"
	r, e := s.PushQuizEdit(context.Background(), p)
	if e != nil || r.Status != "completed" || len(f.patches) != 3 {
		t.Fatalf("push: %+v %v %v", r, e, f.patches)
	}
	if len(f.patches[0]["quiz"].(map[string]any)) != 1 {
		t.Fatal("quiz sent unrelated fields")
	}
	if f.items[1]["entry"].(map[string]any)["item_body"] != oldLink {
		t.Fatal("replacing one file reference changed another use of the original")
	}
	entry := f.patches[1]["item"].(map[string]any)["entry"].(map[string]any)
	if len(entry) != 1 || entry["item_body"] != d.Items[0].Changes["item_body"] {
		t.Fatalf("prompt sent extra fields: %v", entry)
	}
	feedback := f.items[1]["entry"].(map[string]any)["feedback"].(map[string]any)
	if feedback["correct"] != "Teacher's new correct feedback" || feedback["neutral"] != "" || feedback["incorrect"] != nil || !reflect.DeepEqual(feedback["future_key"], map[string]any{"text": "Keep"}) {
		t.Fatalf("feedback siblings lost: %v", feedback)
	}
	// No confirmation GET after the final accepted action.
	if f.readsAfterWrite != 2 {
		t.Fatalf("unexpected confirmation reads: %d", f.readsAfterWrite)
	}
	r, e = s.PushQuizEdit(context.Background(), p)
	if e != nil || r.Status != "completed" || len(f.patches) != 3 {
		t.Fatalf("replayed accepted patch: %+v %v", r, e)
	}
	for _, action := range r.Actions {
		if action.Outcome.Status != "preserved" {
			t.Fatalf("rerun: %+v", r)
		}
	}
}

func TestQuizContentEditHoldsOnlyChangedOrReadOnlyItem(t *testing.T) {
	f := newContentEditFixture()
	s, _ := workflowFixture(t, f.handler(t))
	p, d := pullContentEdit(t, s)
	d.Items[0].Changes = map[string]any{"item_body": "Revised"}
	d.Items[1].Changes = map[string]any{"title": "Independent"}
	d.Items[2].Changes = map[string]any{"title": "Unsupported"}
	f.items[0]["entry"].(map[string]any)["item_body"] = "Teacher changed this"
	if e := workspace.WriteDocument(p, &d, ""); e != nil {
		t.Fatal(e)
	}
	r, e := s.PushQuizEdit(context.Background(), p)
	if e != nil || r.Status != "partial" || len(f.patches) != 1 || len(r.Actions) != 3 || r.Actions[0].Outcome.Status != "held" || r.Actions[1].Outcome.Status != "succeeded" || r.Actions[2].Outcome.Status != "held" {
		t.Fatalf("independent outcomes: %+v %v", r, e)
	}
}

func TestQuizContentEditTimeoutRerunAndIdentity(t *testing.T) {
	f := newContentEditFixture()
	f.timeout = true
	s, _ := workflowFixture(t, f.handler(t))
	p, d := pullContentEdit(t, s)
	d.Items[0].Changes = map[string]any{"item_body": "Revised"}
	if e := workspace.WriteDocument(p, &d, ""); e != nil {
		t.Fatal(e)
	}
	r, e := s.PushQuizEdit(context.Background(), p)
	if e != nil || len(f.patches) != 1 || r.Actions[0].Outcome.Status != "uncertain" {
		t.Fatalf("timeout: %+v %v", r, e)
	}
	r, e = s.PushQuizEdit(context.Background(), p)
	if e != nil || len(f.patches) != 1 || r.Actions[0].Outcome.Status != "preserved" {
		t.Fatalf("timeout replay: %+v %v", r, e)
	}
	d.Items[0].Changes = map[string]any{"title": "New"}
	f.items[0]["entry"].(map[string]any)["id"] = "999"
	if e := workspace.WriteDocument(p, &d, ""); e != nil {
		t.Fatal(e)
	}
	r, e = s.PushQuizEdit(context.Background(), p)
	if e != nil || len(f.patches) != 1 || r.Actions[0].Outcome.Status != "held" {
		t.Fatalf("identity not held: %+v %v", r, e)
	}
	d.CourseID = 2
	d.Observed.CourseID = 2
	if e := workspace.WriteDocument(p, &d, ""); e != nil {
		t.Fatal(e)
	}
	if _, e = s.PushQuizEdit(context.Background(), p); e == nil {
		t.Fatal("wrong course accepted")
	}
}

func TestQuizContentEditRejectsUnsupportedAndNullChanges(t *testing.T) {
	f := newContentEditFixture()
	s, _ := workflowFixture(t, f.handler(t))
	_, d := pullContentEdit(t, s)
	for _, changes := range []map[string]any{{"title": nil}, {"title": ""}, {"instructions": nil}, {"points_possible": 1}} {
		d.Changes = changes
		if d.Validate() == nil {
			t.Fatalf("accepted quiz changes %v", changes)
		}
	}
	d.Changes = nil
	for _, changes := range []map[string]any{{"item_body": nil}, {"scoring_data": map[string]any{}}, {"feedback": nil}, {"feedback": map[string]any{"general": "Wrong name"}}, {"feedback": map[string]any{"correct": nil}}} {
		d.Items[0].Changes = changes
		if d.Validate() == nil {
			t.Fatalf("accepted item changes %v", changes)
		}
	}
}

func TestQuizContentEditObservedNullAndAbsentAreDistinct(t *testing.T) {
	f := newContentEditFixture()
	delete(f.quiz, "instructions")
	f.items[0]["entry"].(map[string]any)["item_body"] = nil
	f.items[1]["entry_editable"] = false
	f.items[3]["status"] = "immutable"
	s, _ := workflowFixture(t, f.handler(t))
	p, d := pullContentEdit(t, s)
	if _, ok := d.Observed.Fields["instructions"]; ok {
		t.Fatal("invented absent instructions")
	}
	if v, ok := d.Items[0].Observed.Fields["item_body"]; !ok || v != nil {
		t.Fatal("lost null baseline")
	}
	if d.Items[1].Editable || d.Items[3].Editable {
		t.Fatal("ignored Canvas editability")
	}
	d.Changes = map[string]any{"instructions": ""}
	d.Items[0].Changes = map[string]any{"item_body": ""}
	// An absent -> null change is a conflict, even when neither renders text.
	f.quiz["instructions"] = nil
	if e := workspace.WriteDocument(p, &d, ""); e != nil {
		t.Fatal(e)
	}
	r, e := s.PushQuizEdit(context.Background(), p)
	if e != nil || len(f.patches) != 1 || r.Actions[0].Outcome.Status != "held" || r.Actions[1].Outcome.Status != "succeeded" {
		t.Fatalf("presence semantics: %+v %v", r, e)
	}
}
