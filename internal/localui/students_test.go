package localui

import (
	"bytes"
	"canvasexpertse/internal/workspace"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestStudentReferenceExcludesPrivateDetails(t *testing.T) {
	data := studentsData{Hub: true, CourseID: 11, CourseName: "CS8", NotesOn: true, ContactsOn: true, Rows: []studentRow{{
		Student:  workspace.Student{Name: "Ana <Example>", Alias: "Pika & Chu", FirstName: "Private nickname", Ref: "private-ref", QuizTime: "731", ReducedChoices: true},
		Note:     "Private teacher note",
		Contacts: workspace.StudentContacts{StudentEmail: "private@example.invalid"},
	}}}
	var output bytes.Buffer
	if err := tmpl("students").Execute(&output, data); err != nil {
		t.Fatal(err)
	}
	_, reference, found := strings.Cut(output.String(), `<section class="student-reference"`)
	reference, _, _ = strings.Cut(reference, `</section>`)
	if !found || !strings.Contains(reference, `<td>Ana &lt;Example&gt;</td><td>Pika &amp; Chu</td>`) {
		t.Fatal("print reference missing escaped student/alias pair")
	}
	for _, private := range []string{"Private nickname", "private-ref", "Private teacher note", "private@example.invalid", "731", "Fewer choices", "<input", "<textarea"} {
		if strings.Contains(reference, private) {
			t.Fatalf("print reference includes %q", private)
		}
	}
}

func TestStudentQuizSettingsSaveAndValidation(t *testing.T) {
	root := t.TempDir()
	store, err := workspace.OpenStudents(root)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := store.Upsert("https://school.example", 900001, "Ana", "Ana", "", "")
	_, _ = store.Upsert(a.Instance, 900002, "Bo", "Bo", "", "")
	store.Rows[1].QuizTime = "100%"
	store.Rows[1].ReducedChoices = true
	h := &studentHandler{origin: "http://127.0.0.1:1234", secret: "nonce", store: store, hub: true, persistent: true, members: map[string][]string{a.Ref: {}}, data: studentsData{CourseID: 11}}
	form := url.Values{"request_secret": {"nonce"}, "alias_" + a.Ref: {a.Alias}, "extra_time_" + a.Ref: {"on"}, "reduced_choices_" + a.Ref: {"on"}}
	post := func() *httptest.ResponseRecorder {
		form.Set("students_version", h.store.Version)
		r := httptest.NewRequest(http.MethodPost, h.origin+"/students", strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("Origin", h.origin)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	for _, value := range []string{"50%", "100%", "minutes", ""} {
		form.Set("quiz_time_"+a.Ref, value)
		form.Set("quiz_minutes_"+a.Ref, "15")
		w := post()
		if w.Code != 200 {
			t.Fatalf("save %q: %d %s", value, w.Code, w.Body.String())
		}
		want := value
		if want == "minutes" {
			want = "15"
		}
		loaded, err := workspace.OpenStudents(root)
		if err != nil || loaded.Rows[0].QuizTime != want || !loaded.Rows[0].ReducedChoices || !loaded.Rows[0].ExtraTime || loaded.Rows[1].QuizTime != "100%" || !loaded.Rows[1].ReducedChoices {
			t.Fatalf("settings or course scope lost: %+v %v", loaded, err)
		}
		for _, field := range []string{"quiz_time_", "quiz_minutes_", "reduced_choices_"} {
			if !strings.Contains(w.Body.String(), field+a.Ref) {
				t.Fatalf("missing form setting %s", field)
			}
		}
	}
	for _, value := range []string{"", "0", "1.5", "10081", "50%"} {
		form.Set("quiz_time_"+a.Ref, "minutes")
		form.Set("quiz_minutes_"+a.Ref, value)
		form.Set("alias_"+a.Ref, "Must not save")
		if w := post(); w.Code != 400 || store.Rows[0].Alias != a.Alias || store.Rows[0].QuizTime != "" {
			t.Fatalf("invalid minutes %q changed settings", value)
		}
	}
	// An older alias-only form must leave both private settings unchanged.
	form.Del("quiz_time_" + a.Ref)
	form.Del("reduced_choices_" + a.Ref)
	form.Set("alias_"+a.Ref, "Renamed")
	store.Rows[0].QuizTime, store.Rows[0].ReducedChoices = "50%", true
	if w := post(); w.Code != 200 || store.Rows[0].QuizTime != "50%" || !store.Rows[0].ReducedChoices {
		t.Fatal("alias-only save cleared quiz settings")
	}
}

func TestStudentEditorOriginAndDuplicateAlias(t *testing.T) {
	s, e := workspace.OpenStudents(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	a, _ := s.Upsert("https://school.example", 1, "Ana", "Ana", "", "")
	b, _ := s.Upsert("https://school.example", 2, "Bob", "Bob", "", "")
	h := &studentHandler{origin: "http://127.0.0.1:1234", secret: "nonce", store: s, done: make(chan struct{})}
	form := url.Values{"request_secret": {"nonce"}, "alias_" + a.Ref: {"Same"}, "alias_" + b.Ref: {"Same"}}
	request := func(origin string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, h.origin+"/students", strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := request("http://attacker.example"); w.Code != 403 {
		t.Fatal("cross-origin allowed")
	}
	if w := request(h.origin); w.Code != 400 || s.Rows[0].Alias != a.Alias {
		t.Fatal("duplicate alias changed store")
	}
	form.Set("alias_"+a.Ref, "New Alias")
	form.Set("alias_"+b.Ref, b.Alias)
	form.Set("monitored_"+a.Ref, "on")
	if w := request(h.origin); w.Code != 200 || !s.Rows[0].Monitored || s.Rows[0].Ref != a.Ref {
		t.Fatal("edit retargeted identity")
	}
}

// An editor left open while a pull or roster refresh changes the table must not
// drop the new student, reset a newer quiz setting or disconnect a reference.
func TestStaleStudentSaveKeepsNewerRowsAndSettings(t *testing.T) {
	root := t.TempDir()
	store, err := workspace.OpenStudents(root)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := store.Upsert("https://school.example", 900001, "Ana", "Ana", "", "")
	if err = store.Save(); err != nil {
		t.Fatal(err)
	}
	h := &studentHandler{origin: "http://127.0.0.1:1234", secret: "nonce", store: store, notesRoot: root, done: make(chan struct{})}
	opened := store.Version

	// Elsewhere on this PC: a pull adds Bo and the teacher's assistant gives Ana time and a half.
	other, err := workspace.OpenStudents(root)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := other.Upsert(a.Instance, 900002, "Bo", "Bo", "", "")
	other.Rows[0].QuizTime = "50%"
	if err = other.Save(); err != nil {
		t.Fatal(err)
	}

	post := func(version string, shown ...workspace.Student) *httptest.ResponseRecorder {
		form := url.Values{"request_secret": {"nonce"}, "students_version": {version}, "alias_" + a.Ref: {"Teacher Pick"}, "quiz_time_" + a.Ref: {""}}
		for _, s := range shown {
			form.Set("alias_"+s.Ref, s.Alias)
		}
		r := httptest.NewRequest(http.MethodPost, h.origin+"/students", strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("Origin", h.origin)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	w := post(opened)
	if w.Code != 409 || !strings.Contains(w.Body.String(), "Nothing was saved") || !strings.Contains(w.Body.String(), "alias_"+b.Ref) || !strings.Contains(w.Body.String(), `data-draft-key="students:nonce"`) {
		t.Fatalf("stale save not turned away with the newest table and drafts: %d %s", w.Code, w.Body.String())
	}
	loaded, err := workspace.OpenStudents(root)
	if err != nil || len(loaded.Rows) != 2 || loaded.Rows[0].QuizTime != "50%" || loaded.Rows[0].Alias == "Teacher Pick" || loaded.Rows[1].Ref != b.Ref {
		t.Fatalf("stale save changed the table: %+v %v", loaded, err)
	}

	// Saved again from the refreshed page, the teacher's choices apply and Bo stays.
	w = post(loaded.Version, b)
	if w.Code != 200 {
		t.Fatalf("fresh save: %d %s", w.Code, w.Body.String())
	}
	loaded, err = workspace.OpenStudents(root)
	if err != nil || len(loaded.Rows) != 2 || loaded.Rows[0].Alias != "Teacher Pick" || loaded.Rows[0].QuizTime != "" || loaded.Rows[1].Ref != b.Ref {
		t.Fatalf("fresh save: %+v %v", loaded, err)
	}
}
