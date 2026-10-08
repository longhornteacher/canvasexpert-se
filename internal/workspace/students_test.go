package workspace

import (
	"canvassidekick/internal/domain"
	"strings"
	"testing"
)

func TestStudentRenameAndInstanceIdentity(t *testing.T) {
	root := t.TempDir()
	s, e := OpenStudents(root)
	if e != nil {
		t.Fatal(e)
	}
	a, e := s.Upsert("https://school.example", 7, "Ana Smith", "Ana", "", " ")
	if e != nil {
		t.Fatal(e)
	}
	s.Rows[0].Alias = "Custom Alias"
	s.Rows[0].Monitored = true
	b, e := s.Upsert(a.Instance, 7, "Ana Jones", "Ana", "", "")
	if e != nil || b.Ref != a.Ref || b.Alias != "Custom Alias" || !b.Monitored {
		t.Fatalf("identity changed: %#v %v", b, e)
	}
	other, e := s.Upsert("https://other.example", 7, "Other Person", "Other", "", "")
	if e != nil || other.Ref == a.Ref {
		t.Fatal("instances collided")
	}
	if e = s.Save(); e != nil {
		t.Fatal(e)
	}
	s, e = OpenStudents(root)
	if e != nil {
		t.Fatal(e)
	}
	v, e := s.Resolve(a.Instance, a.Ref)
	if e != nil || v.Name != "Ana Jones" {
		t.Fatal("save/resolve failed")
	}
	if _, e = s.Resolve(other.Instance, a.Ref); e == nil {
		t.Fatal("cross-instance resolution")
	}
}
func TestNotesAmbiguityAndPlaceholder(t *testing.T) {
	a := Student{Ref: "a", Instance: "https://school.example", Name: "Sam Lee", FirstName: "Sam", Alias: "Pikachu"}
	b := a
	b.Ref = "b"
	notes := domain.StudentNotes{Schema: 1, Notes: []domain.StudentNote{{Name: "Sam Lee", Note: "wrong if ambiguous"}, {StudentRef: "a", Note: "private accommodation"}}}
	got, w := NotesFor(notes, "ELA", a, []Student{a, b})
	if len(got) != 1 || got[0] != "private accommodation" || len(w) != 1 {
		t.Fatal("ambiguous notes attached")
	}
	text := ReplaceStudentName("{{student_name}}, your quotation says Pikachu.", a)
	if text != "Sam, your quotation says Pikachu." || strings.Contains(text, "private") {
		t.Fatal("broad replacement")
	}
}

func TestNameNoteRequiresTargetIdentityInRoster(t *testing.T) {
	a := Student{Ref: "a", Instance: "https://school.example", UserID: 1, Name: "Sam Lee"}
	b := a
	b.Ref, b.UserID = "b", 2
	notes := domain.StudentNotes{Schema: 1, Notes: []domain.StudentNote{{Name: "Sam Lee", Note: "student-specific accommodation"}}}
	for _, roster := range [][]Student{{b}, {{Ref: a.Ref, Instance: a.Instance, UserID: b.UserID, Name: a.Name}}, {}} {
		got, warnings := NotesFor(notes, "ELA", a, roster)
		if len(got) != 0 || len(warnings) != 1 {
			t.Fatalf("name note attached without matching target identity: %v %v", got, warnings)
		}
	}
	got, warnings := NotesFor(notes, "ELA", a, []Student{a})
	if len(got) != 1 || len(warnings) != 0 {
		t.Fatalf("unique target note missing: %v %v", got, warnings)
	}
}
