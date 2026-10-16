package workspace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"canvasexpertse/internal/domain"
)

func TestReusableTeacherSetupAcrossInstalls(t *testing.T) {
	a, b, shared := makeRulesInstall(t), makeRulesInstall(t), t.TempDir()
	config := domain.CourseConfig{Schema: 1, Instance: "https://school.example", Aliases: true, Courses: []domain.Course{{Nickname: "TEST", ID: 12, Current: true}}}
	if err := WriteTeacherDocument(a, "context/courses.md", config, "teacher setup"); err != nil {
		t.Fatal(err)
	}
	students, err := OpenStudents(a)
	if err != nil {
		t.Fatal(err)
	}
	student, err := students.Upsert(config.Instance, 42, "Synthetic Student", "Synthetic", "", "")
	if err != nil {
		t.Fatal(err)
	}
	students.Rows[0].Monitored = true
	students.Rows[0].ExtraTime = true
	if err := students.Save(); err != nil {
		t.Fatal(err)
	}
	notes := domain.StudentNotes{Schema: 1, Notes: []domain.StudentNote{{StudentRef: student.Ref, Note: "Reusable synthetic note"}}}
	if err := WriteTeacherDocument(a, "context/students.md", notes, ""); err != nil {
		t.Fatal(err)
	}
	if err := SaveRosters(a, []CourseRoster{{Instance: config.Instance, CourseID: 12, Members: map[string][]string{student.Ref: {"Blue"}}}}); err != nil {
		t.Fatal(err)
	}
	contacts, err := OpenContacts(a)
	if err != nil {
		t.Fatal(err)
	}
	contacts.Students[ContactKey(student)] = StudentContacts{Parents: []ParentContact{{Email: "parent@example.test", EmailEnabled: true}}}
	if err := contacts.Save(); err != nil {
		t.Fatal(err)
	}
	for _, relative := range []string{"courses/TEST/draft.md", ".cse/hub.json", ".cse/installation.json"} {
		p, _ := ConstrainPath(a, relative)
		if err := WriteFile(p, []byte("local only"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := SetRulesLocation(a, shared, nil, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := SetRulesLocation(b, shared, nil, ""); err != nil {
		t.Fatal(err)
	}
	var got domain.CourseConfig
	if _, err := ReadTeacherDocument(b, "context/courses.md", &got); err != nil || got.Courses[0].ID != 12 {
		t.Fatal(got, err)
	}
	restored, err := OpenStudents(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(restored.Rows) != 1 || restored.Rows[0].Ref != student.Ref || !restored.Rows[0].Monitored || !restored.Rows[0].ExtraTime {
		t.Fatal("student settings lost")
	}
	var gotNotes domain.StudentNotes
	if _, err := ReadTeacherDocument(b, "context/students.md", &gotNotes); err != nil || gotNotes.Notes[0].StudentRef != restored.Rows[0].Ref {
		t.Fatal("notes lost", err)
	}
	gotContacts, err := OpenContacts(b)
	if err != nil || len(gotContacts.Students[ContactKey(restored.Rows[0])].Parents) != 1 || !gotContacts.Students[ContactKey(restored.Rows[0])].Parents[0].EmailEnabled {
		t.Fatal("parent contacts lost", err)
	}
	rosters, err := ReadRosters(b)
	if err != nil || rosters[0].Members[student.Ref][0] != "Blue" {
		t.Fatal("roster lost", err)
	}
	for _, relative := range []string{"courses/TEST/draft.md", ".cse/hub.json", ".cse/installation.json", ".cse/rules-location.json"} {
		if _, err := os.Stat(filepath.Join(shared, relative)); !os.IsNotExist(err) {
			t.Fatal("machine/session state shared", relative, err)
		}
	}
	// Edits from the replacement PC are immediately read through the first UserInstall.
	restored.Rows[0].ExtraTime = false
	if err := restored.Save(); err != nil {
		t.Fatal(err)
	}
	again, err := OpenStudents(a)
	if err != nil || again.Rows[0].ExtraTime {
		t.Fatal("stale student settings", err)
	}
}

func TestOldRulesOnlyBindingAddsMissingSetupOnce(t *testing.T) {
	root, shared := makeRulesInstall(t), t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".cse"), 0700); err != nil {
		t.Fatal(err)
	}
	old, _ := json.Marshal(map[string]string{"sync_root": shared})
	if err := os.WriteFile(rulesBindingPath(root), old, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := GetRulesLocation(root); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(shared, "context", "courses.md")
	if b, err := os.ReadFile(p); err != nil || string(b) != "local courses" {
		t.Fatal("old setup not migrated", err)
	}
	if err := os.WriteFile(p, []byte("new shared setup"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := GetRulesLocation(root); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p); string(b) != "new shared setup" {
		t.Fatal("upgrade replaced shared data")
	}
}
