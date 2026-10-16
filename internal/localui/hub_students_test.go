package localui

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workflow"
	"canvasexpertse/internal/workspace"
)

func studentsHub(t *testing.T) (*Hub, string) {
	h := newTestHub()
	h.programDir = t.TempDir()
	root := makeWorkspace(t)
	// A valid HTTPS instance so the course document validates.
	path := filepath.Join(root, "context", "courses.md")
	if err := os.WriteFile(path, []byte("---\nschema: 1\ninstance: https://school.example\naliases: true\ncourses: []\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h.root = root
	h.listCoursesFn = func(context.Context) ([]workflow.Course, string, error) {
		return []workflow.Course{{ID: 11, Name: "Computer Science 8", Code: "CS8", State: "available"}, {ID: 22, Name: "Old Course", Code: "OLD", State: "available"}, {ID: 33, Name: "Same Title", Code: "A", State: "unpublished"}, {ID: 44, Name: "Same Title", Code: "B", State: "available"}}, "https://school.example", nil
	}
	h.rosterFn = func(_ context.Context, id int64) ([]workflow.Enrollment, string, error) {
		mk := func(uid int64, name, state string) workflow.Enrollment {
			return workflow.Enrollment{UserID: uid, State: state, User: workflow.CanvasUser{ID: uid, Name: name}}
		}
		return []workflow.Enrollment{mk(1, "Ana Real", "active"), mk(2, "Bo Real", "active"), mk(3, "Dropped Kid", "inactive")}, "https://school.example", nil
	}
	h.tagsFn = func(context.Context, int64) (map[int64][]string, string, error) {
		return map[int64][]string{1: {"Silver"}, 2: {"Red"}}, "https://school.example", nil
	}
	return h, root
}

func hubGet(h *Hub, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, h.origin+path, nil))
	return w
}

func TestCoursePickerSavesOnlyActiveCourses(t *testing.T) {
	h, root := studentsHub(t)
	page := hubGet(h, "/courses")
	if page.Code != 200 || !strings.Contains(page.Body.String(), "Computer Science 8") || !strings.Contains(page.Body.String(), `name="nick_11" value=""`) {
		t.Fatalf("course list missing or new nickname not empty: %d", page.Code)
	}
	body := page.Body.String()
	if strings.Contains(body, `name="nick_11" value="CS8"`) || strings.Contains(body, `name="active_11" checked`) {
		t.Fatal("new course should start with an empty nickname and unchecked")
	}
	if !strings.Contains(body, `ID 11 · available`) || !strings.Contains(body, `ID 33 · unpublished`) || !strings.Contains(body, `<tr class="unpublished">`) {
		t.Fatal("course ID/state or unpublished styling missing")
	}
	if strings.Index(body, `ID 44 · available`) > strings.Index(body, `ID 33 · unpublished`) {
		t.Fatal("available course should be listed before unpublished course")
	}
	if strings.Count(body, `Same Title <span class="course-state"`) != 2 {
		t.Fatal("same-title courses with distinct IDs must both remain visible")
	}
	if !strings.Contains(body, `aria-label="Nickname for Same Title (33)"`) || !strings.Contains(body, `aria-label="Active course Same Title (44)"`) {
		t.Fatal("course controls need labels that distinguish same-title courses")
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
	// An explicitly selected unpublished course keeps its nickname and active flag on reload/save.
	config, _, _ = h.readConfig()
	config.Courses = append(config.Courses, domain.Course{ID: 33, Nickname: "Archive", Current: true})
	if err := workspace.WriteDocument(filepath.Join(root, "context", "courses.md"), config, ""); err != nil {
		t.Fatal(err)
	}
	page = hubGet(h, "/courses")
	if !strings.Contains(page.Body.String(), `name="active_33" checked`) || !strings.Contains(page.Body.String(), `name="nick_33" value="Archive"`) {
		t.Fatal("saved unpublished selection was not restored")
	}
	hubPost(h, "/courses", url.Values{"request_secret": {"nonce"}, "id": {"11", "22", "33", "44"}, "nick_11": {"CS8"}, "nick_22": {"Old"}, "nick_33": {"Archive"}, "active_33": {"on"}}, h.origin)
	if config, _, _ = h.readConfig(); len(config.Courses) != 2 || !config.Courses[1].Current || config.Courses[1].Nickname != "Archive" {
		t.Fatalf("saved choice changed after reload/save: %+v", config.Courses)
	}
	// Selecting a new course without supplying a nickname is rejected with a useful message.
	hubPost(h, "/courses", url.Values{"request_secret": {"nonce"}, "id": {"11", "22", "33", "44"}, "nick_11": {"CS8"}, "nick_22": {"Old"}, "nick_33": {"Archive"}, "active_33": {"on"}, "active_44": {"on"}}, h.origin)
	if w := hubGet(h, "/courses"); !strings.Contains(w.Body.String(), "Every active course needs a nickname.") {
		t.Fatal("new active course without nickname should explain the correction")
	}
	_ = root
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

func TestStudentsCourseTabsAndScopedSaves(t *testing.T) {
	h, root := studentsHub(t)
	config := domain.CourseConfig{Schema: 1, Instance: "https://school.example", Aliases: true, Courses: []domain.Course{{ID: 11, Nickname: "CS8", Current: true}, {ID: 22, Nickname: "ELA7", Current: true}, {ID: 33, Nickname: "OLD"}}}
	if err := workspace.WriteDocument(filepath.Join(root, "context", "courses.md"), config, "Course notes stay here."); err != nil {
		t.Fatal(err)
	}
	h.rosterFn = func(_ context.Context, id int64) ([]workflow.Enrollment, string, error) {
		if id == 22 {
			return []workflow.Enrollment{{UserID: 2, State: "active", User: workflow.CanvasUser{Name: "Bo Real"}}, {UserID: 4, State: "active", User: workflow.CanvasUser{Name: "Cal Real"}}}, config.Instance, nil
		}
		return []workflow.Enrollment{{UserID: 1, State: "active", User: workflow.CanvasUser{Name: "Ana Real"}}, {UserID: 2, State: "active", User: workflow.CanvasUser{Name: "Bo Real"}}, {UserID: 3, State: "inactive", User: workflow.CanvasUser{Name: "Dropped Kid"}}}, config.Instance, nil
	}
	h.tagsFn = func(_ context.Context, id int64) (map[int64][]string, string, error) {
		if id == 22 {
			return map[int64][]string{2: {"Blue"}, 4: {"Silver", "<unsafe>"}}, config.Instance, nil
		}
		return map[int64][]string{1: {"Red"}}, config.Instance, nil
	}
	page := hubGet(h, "/students").Body.String()
	for _, want := range []string{`href="/students?course=11" aria-current="page"`, `href="/students?course=22"`, "Ana Real", "Red", "Extra time"} {
		if !strings.Contains(page, want) {
			t.Fatalf("student page missing %q", want)
		}
	}
	for _, absent := range []string{"Cal Real", "Dropped Kid", "OLD", "/pokemonify"} {
		if strings.Contains(page, absent) {
			t.Fatalf("student page unexpectedly includes %q", absent)
		}
	}
	page = hubGet(h, "/students?course=22").Body.String()
	if !strings.Contains(page, "Cal Real") || !strings.Contains(page, "Blue") || !strings.Contains(page, "&lt;unsafe&gt;") || strings.Contains(page, "Ana Real") {
		t.Fatal("course selection or tag scope is incorrect")
	}
	store, _ := workspace.OpenStudents(root)
	byID := map[int64]workspace.Student{}
	for i, student := range store.Rows {
		if student.UserID == 1 {
			store.Rows[i].Monitored, store.Rows[i].ExtraTime = true, true
			store.Rows[i].FirstName = "Annie"
		}
		byID[student.UserID] = store.Rows[i]
	}
	if err := store.Save(); err != nil {
		t.Fatal(err)
	}
	form := url.Values{"request_secret": {"nonce"}, "course": {"22"}, "alias_" + byID[2].Ref: {"Edited Bo"}, "alias_" + byID[4].Ref: {byID[4].Alias}, "extra_time_" + byID[4].Ref: {"on"}}
	form.Set("goes_by_"+byID[2].Ref, "  Bobby  ")
	form.Set("goes_by_"+byID[1].Ref, "Wrong course")
	for i := 0; i < 2; i++ {
		w := hubPost(h, "/students", form, h.origin)
		if w.Code != 200 || !strings.Contains(w.Body.String(), "Saved.") || !strings.Contains(w.Body.String(), `href="/students?course=22" aria-current="page"`) {
			t.Fatalf("save failed or switched course: %d %s", w.Code, w.Body.String())
		}
	}
	for _, invalid := range []string{"", strings.Repeat("x", 81), "Bo\n", "Bo\rName", "Bo\x00Name"} {
		form.Set("goes_by_"+byID[2].Ref, invalid)
		if w := hubPost(h, "/students", form, h.origin); w.Code != 400 {
			t.Fatalf("invalid Goes By accepted: %q", invalid)
		}
	}
	form.Set("goes_by_"+byID[2].Ref, "Bobby")
	form.Set("alias_"+byID[4].Ref, byID[1].Alias)
	if w := hubPost(h, "/students", form, h.origin); w.Code != 400 {
		t.Fatal("accepted alias colliding with a student in another course")
	}
	if w := hubPost(h, "/students/refresh", url.Values{"request_secret": {"nonce"}, "course": {"22"}}, h.origin); w.Code != 303 || w.Header().Get("Location") != "/students?course=22" {
		t.Fatal("refresh did not retain course selection")
	}
	store, _ = workspace.OpenStudents(root)
	for _, student := range store.Rows {
		switch student.UserID {
		case 1:
			if !student.Monitored || !student.ExtraTime || student.Alias != byID[1].Alias || student.FirstName != "Annie" {
				t.Fatal("saving another course changed Ana's settings")
			}
		case 2:
			if student.Alias != "Edited Bo" || student.Ref != byID[2].Ref || student.FirstName != "Bobby" {
				t.Fatal("refresh changed alias or identity")
			}
		case 4:
			if !student.ExtraTime || student.Alias != byID[4].Alias {
				t.Fatal("extra time not persisted or rejected save changed data")
			}
		}
	}
	// A new hub session retains course associations and tags while offline.
	restarted := newTestHub()
	restarted.root = root
	restarted.rosterFn = func(context.Context, int64) ([]workflow.Enrollment, string, error) {
		return nil, "", errors.New("offline")
	}
	page = hubGet(restarted, "/students?course=22").Body.String()
	if !strings.Contains(page, "last saved roster") || !strings.Contains(page, "Cal Real") || !strings.Contains(page, "Blue") || !strings.Contains(page, `name="extra_time_`+byID[4].Ref+`" checked`) {
		t.Fatal("saved roster or settings unavailable after restarting offline")
	}
	if w := hubGet(h, "/students?course=33"); w.Code != 404 {
		t.Fatal("inactive course tab accepted")
	}
}

func TestClassroomKeepsUnsavedEditsInCurrentTab(t *testing.T) {
	h, root := studentsHub(t)
	config := domain.CourseConfig{Schema: 1, Instance: "https://school.example", Courses: []domain.Course{{ID: 11, Nickname: "CS8", Current: true}}}
	if err := workspace.WriteDocument(filepath.Join(root, "context", "courses.md"), config, ""); err != nil {
		t.Fatal(err)
	}
	students := hubGet(h, "/students")
	page := students.Body.String()
	if csp := students.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "script-src 'self'") || strings.Contains(csp, "script-src 'unsafe-inline'") {
		t.Fatalf("student draft script is not confined by the page CSP: %s", csp)
	}
	for _, want := range []string{`data-draft-key="students:nonce"`, `name="alias_`, `name="goes_by_`, `data-draft-fields="alias_ goes_by_`, `name="note_`, `<details class="student-notes">`, `Refresh from Canvas`, `students`} {
		if !strings.Contains(page, want) {
			t.Fatalf("student page missing %q", want)
		}
	}
	if !strings.Contains(page, `table class="students"`) || strings.Contains(page, `class="student-card"`) || strings.Contains(page, `localStorage`) {
		t.Fatal("student roster should stay compact and avoid durable browser storage")
	}
	coursesResponse := hubGet(h, "/courses")
	courses := coursesResponse.Body.String()
	if csp := coursesResponse.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "script-src 'self'") || strings.Contains(csp, "connect-src") {
		t.Fatalf("course draft CSP grants unnecessary access: %s", csp)
	}
	if !strings.Contains(courses, `data-draft-key="courses:nonce"`) || !strings.Contains(courses, `name="active_11"`) || !strings.Contains(courses, `name="nick_11"`) {
		t.Fatal("course page does not expose draft-restorable controls")
	}
	if got := hubGet(h, "/_cse/drafts.js"); got.Code != 200 || !strings.Contains(got.Body.String(), "sessionStorage") || strings.Contains(got.Body.String(), "localStorage") {
		t.Fatal("tab-scoped classroom draft script missing or uses durable storage")
	}
}

func TestStudentsNoActiveCoursesAndTagFailure(t *testing.T) {
	h, root := studentsHub(t)
	if w := hubGet(h, "/students"); w.Code != 200 || !strings.Contains(w.Body.String(), "No active courses selected") {
		t.Fatal("missing empty state")
	}
	config := domain.CourseConfig{Schema: 1, Instance: "https://school.example", Courses: []domain.Course{{ID: 11, Nickname: "CS8", Current: true}}}
	if err := workspace.WriteDocument(filepath.Join(root, "context", "courses.md"), config, ""); err != nil {
		t.Fatal(err)
	}
	h.tagsFn = func(context.Context, int64) (map[int64][]string, string, error) {
		return nil, "", errors.New("Canvas denied this operation")
	}
	page := hubGet(h, "/students").Body.String()
	if !strings.Contains(page, "Ana Real") || !strings.Contains(page, "Differentiation tags could not be pulled") || !strings.Contains(page, "Unavailable") || strings.Contains(page, "No tag") {
		t.Fatal("unavailable tags must not look like absent tags or hide the roster")
	}
	if store, _ := workspace.OpenStudents(root); len(store.Rows) != 2 {
		t.Fatal("inactive students imported")
	}
	for _, path := range []string{"/students", "/students/refresh"} {
		if w := hubPost(h, path, url.Values{"request_secret": {"wrong"}, "course": {"11"}}, h.origin); w.Code != 403 {
			t.Fatal("student mutation accepted wrong secret")
		}
		if w := hubPost(h, path, url.Values{"request_secret": {"nonce"}, "course": {"11"}}, "https://foreign.example"); w.Code != 403 {
			t.Fatal("student mutation accepted foreign origin")
		}
	}
}

func TestSettingsShowInstallationAndSavePreferences(t *testing.T) {
	h, _ := studentsHub(t)
	home := hubGet(h, "/").Body.String()
	if strings.Contains(home, `name="path"`) || strings.Contains(home, "SE = ") || strings.Contains(home, "Command central") || !strings.Contains(home, "CanvasExpert: Special Edition") {
		t.Fatal("home still contains old branding or workspace controls")
	}
	page := hubGet(h, "/settings?section=preferences").Body.String()
	connections := hubGet(h, "/settings?section=connections").Body.String()
	if strings.Contains(page, `name="path"`) || !strings.Contains(page, `name="aliases" checked`) || !strings.Contains(connections, `href="/setup"`) {
		t.Fatal("settings missing basic controls")
	}
	if w := hubPost(h, "/settings", url.Values{"request_secret": {"nonce"}}, h.origin); w.Code != 303 {
		t.Fatal("settings save failed")
	}
	config, _, err := h.readConfig()
	if err != nil || config.Aliases {
		t.Fatal("alias preference not saved")
	}
	for _, path := range []string{"/pokemonify", "/pokemonify/summon", "/pokemonify/reroll", "/pokemonify/monitor"} {
		if w := hubGet(h, path); w.Code != 404 {
			t.Fatalf("removed page still served: %s", path)
		}
	}
}
