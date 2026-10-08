package localui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"canvasexpertse/internal/workflow"
	"canvasexpertse/internal/workspace"
)

func pokemonHub(t *testing.T) (*Hub, string) {
	h := newTestHub()
	h.programDir = t.TempDir()
	root := makeWorkspace(t)
	// A valid HTTPS instance so the course document validates.
	path := filepath.Join(root, "settings", "courses.md")
	if err := os.WriteFile(path, []byte("---\nschema: 1\ninstance: https://school.example\naliases: true\ncourses: []\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h.setRoot(root)
	h.listCoursesFn = func(context.Context) ([]workflow.Course, string, error) {
		return []workflow.Course{{ID: 11, Name: "Computer Science 8", Code: "CS8", State: "available"}, {ID: 22, Name: "Old Course", Code: "OLD", State: "available"}}, "https://school.example", nil
	}
	h.rosterFn = func(_ context.Context, id int64) ([]workflow.Enrollment, string, error) {
		mk := func(uid int64, name, state string) workflow.Enrollment {
			return workflow.Enrollment{UserID: uid, State: state, User: workflow.CanvasUser{ID: uid, Name: name}}
		}
		return []workflow.Enrollment{mk(1, "Ana Real", "active"), mk(2, "Bo Real", "active"), mk(3, "Dropped Kid", "inactive")}, "https://school.example", nil
	}
	return h, root
}

func hubGet(h *Hub, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, h.origin+path, nil))
	return w
}

func TestCoursePickerSavesOnlyActiveCourses(t *testing.T) {
	h, root := pokemonHub(t)
	if w := hubGet(h, "/courses"); w.Code != 200 || !strings.Contains(w.Body.String(), "Computer Science 8") || !strings.Contains(w.Body.String(), `value="CS8"`) {
		t.Fatalf("course list missing: %d", w.Code)
	}
	form := url.Values{"request_secret": {"nonce"}, "id": {"11", "22"}, "nick_11": {"CS8"}, "active_11": {"on"}, "nick_22": {"OLD"}}
	if w := hubPost(h, "/courses", form, h.origin); w.Code != http.StatusSeeOther {
		t.Fatalf("save: %d", w.Code)
	}
	config, _, err := h.readConfig()
	if err != nil || len(config.Courses) != 1 || config.Courses[0].ID != 11 || !config.Courses[0].Current {
		t.Fatalf("only the ticked course should be saved: %+v %v", config.Courses, err)
	}
	// Unticking keeps the course but marks it not current; a clashing nickname is refused.
	form = url.Values{"request_secret": {"nonce"}, "id": {"11"}, "nick_11": {"CS8"}}
	hubPost(h, "/courses", form, h.origin)
	if config, _, _ = h.readConfig(); len(config.Courses) != 1 || config.Courses[0].Current {
		t.Fatal("unticked course still active")
	}
	form = url.Values{"request_secret": {"nonce"}, "id": {"11", "22"}, "nick_11": {"CS8"}, "active_11": {"on"}, "nick_22": {"cs8"}, "active_22": {"on"}}
	hubPost(h, "/courses", form, h.origin)
	if w := hubGet(h, "/courses"); !strings.Contains(w.Body.String(), "Not saved") {
		t.Fatal("duplicate nickname accepted")
	}
	_ = root
}

func TestPokemonifySummonsRerollsAndMonitors(t *testing.T) {
	h, root := pokemonHub(t)
	if w := hubPost(h, "/pokemonify/summon", url.Values{"request_secret": {"nonce"}}, h.origin); w.Code != http.StatusSeeOther {
		t.Fatalf("summon: %d", w.Code)
	}
	if w := hubGet(h, "/pokemonify"); !strings.Contains(w.Body.String(), "No active courses") {
		t.Fatal("summon without active courses should explain itself")
	}
	hubPost(h, "/courses", url.Values{"request_secret": {"nonce"}, "id": {"11"}, "nick_11": {"CS8"}, "active_11": {"on"}}, h.origin)
	hubGet(h, "/courses")
	if w := hubPost(h, "/pokemonify/summon", url.Values{"request_secret": {"nonce"}}, h.origin); w.Code != http.StatusSeeOther {
		t.Fatalf("summon: %d", w.Code)
	}
	store, _ := workspace.OpenStudents(root)
	if len(store.Rows) != 2 {
		t.Fatalf("only active students belong in the party, got %d", len(store.Rows))
	}
	page := hubGet(h, "/pokemonify").Body.String()
	for _, want := range []string{"2 new Pok", `class="ball"`, "CS8", `class="art"`, "<svg"} {
		if !strings.Contains(page, want) {
			t.Fatalf("page lacks %q", want)
		}
	}
	for _, s := range store.Rows {
		if !strings.Contains(page, ">"+s.Alias+"</h3>") {
			t.Fatalf("alias %s not shown", s.Alias)
		}
	}
	// Summoning again adds nobody new and keeps aliases.
	before := store.Rows[0].Alias
	hubPost(h, "/pokemonify/summon", url.Values{"request_secret": {"nonce"}}, h.origin)
	if again, _ := workspace.OpenStudents(root); len(again.Rows) != 2 || again.Rows[0].Alias != before {
		t.Fatal("second summon changed the party")
	}
	ref := store.Rows[0].Ref
	hubPost(h, "/pokemonify/reroll", url.Values{"request_secret": {"nonce"}, "ref": {ref}}, h.origin)
	after, _ := workspace.OpenStudents(root)
	if after.Rows[0].Alias == before || after.Rows[0].Ref != ref {
		t.Fatal("re-roll should change the alias but not the stable reference")
	}
	hubPost(h, "/pokemonify/monitor", url.Values{"request_secret": {"nonce"}, "ref": {ref}}, h.origin)
	if m, _ := workspace.OpenStudents(root); !m.Rows[0].Monitored {
		t.Fatal("monitor toggle failed")
	}
	if w := hubPost(h, "/pokemonify/reroll", url.Values{"request_secret": {"wrong"}, "ref": {ref}}, h.origin); w.Code != 403 {
		t.Fatal("reroll without the secret was accepted")
	}
}

func TestCreatureArtIsStableAndSafe(t *testing.T) {
	a, b := newCard("r", "Pikachu", "Real Name", "", false, false, 0), newCard("r2", "Pikachu", "Other", "", false, false, 3)
	if a.Art != b.Art || a.Type != b.Type || a.Level != b.Level {
		t.Fatal("a Pokémon should look the same for every student")
	}
	if strings.Contains(string(a.Art), "Real Name") || !strings.HasPrefix(string(a.Art), "<svg") {
		t.Fatal("art must be generated from the alias only")
	}
}
