package workspace

import (
	"canvasexpertse/internal/domain"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQuizSettingsKeepEstablishedAliasesAndPrivateAuthority(t *testing.T) {
	root := makeRulesInstall(t)
	path := filepath.Join(root, ".cse", "students.csv")
	old := strings.Join(aliasSourceStudentHeader, ",") + "\ns-existing,https://school.example,7,Ana Smith,Ana,Pikachu,,,true,true,teacher\n"
	if err := WriteFile(path, []byte(old), 0600); err != nil {
		t.Fatal(err)
	}
	store, err := OpenStudents(root)
	if err != nil {
		t.Fatal(err)
	}
	v := store.Rows[0]
	if v.Alias != "Pikachu" || v.AliasSource != AliasFromTeacher || v.QuizTime != "" || v.ReducedChoices || !v.ExtraTime {
		t.Fatalf("11-column identity/settings changed: %+v", v)
	}
	if b, _ := os.ReadFile(path); string(b) != old {
		t.Fatal("opening an established table rewrote it")
	}
	shared := t.TempDir()
	if _, err := SetRulesLocation(root, shared, nil, ""); err != nil {
		t.Fatal(err)
	}
	store, err = OpenStudents(root)
	if err != nil {
		t.Fatal(err)
	}
	store.Rows[0].QuizTime, store.Rows[0].ReducedChoices = "50%", true
	if _, err := store.Upsert(v.Instance, v.UserID, "Updated Name", "Updated", "", ""); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(); err != nil {
		t.Fatal(err)
	}
	again, err := OpenStudents(root)
	if err != nil {
		t.Fatal(err)
	}
	got := again.Rows[0]
	if got.QuizTime != "50%" || !got.ReducedChoices || got.Alias != v.Alias || got.Ref != v.Ref || !got.ExtraTime {
		t.Fatalf("refresh/save lost private settings: %+v", got)
	}
	if b, _ := os.ReadFile(path); string(b) != old {
		t.Fatal("shared save changed inactive local table")
	}
	if b, err := os.ReadFile(filepath.Join(shared, ".cse", "students.csv")); err != nil || !strings.Contains(string(b), ",50%,true") {
		t.Fatalf("shared settings not saved: %v", err)
	}
	b, err := json.Marshal(got)
	if err != nil || strings.Contains(string(b), "QuizTime") || strings.Contains(string(b), "ReducedChoices") || strings.Contains(string(b), "50%") {
		t.Fatalf("private quiz settings serialized: %s %v", b, err)
	}
}

func TestQuizSettingValidation(t *testing.T) {
	for _, value := range []string{"", "50%", "100%", "1", "15", "10080"} {
		if !ValidQuizTime(value) {
			t.Fatalf("valid quiz_time rejected: %q", value)
		}
	}
	for _, value := range []string{"0", "-1", "10081", "1.5", "25%", " 15", "+15", "standard", "99999999999999999999"} {
		if ValidQuizTime(value) {
			t.Fatalf("invalid quiz_time accepted: %q", value)
		}
	}
	for _, tc := range []struct{ time, choices, field string }{
		{"25%", "false", "quiz_time"}, {"15", "sometimes", "reduced_choices"},
	} {
		t.Run(tc.field, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, ".cse", "students.csv")
			data := strings.Join(studentHeader, ",") + "\ns-test,https://school.example,7,Ana,Ana,Pikachu,,,false,false,teacher," + tc.time + "," + tc.choices + "\n"
			if err := WriteFile(path, []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := OpenStudents(root); err == nil || !strings.Contains(err.Error(), "row 2: invalid "+tc.field) {
				t.Fatalf("missing row/field error: %v", err)
			}
			if b, _ := os.ReadFile(path); string(b) != data {
				t.Fatal("invalid private settings were replaced")
			}
		})
	}
}

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
	s.Rows[0].FirstName = "Annie"
	s.Rows[0].Monitored = true
	s.Rows[0].ExtraTime = true
	b, e := s.Upsert(a.Instance, 7, "Ana Jones", "Ana", "", "")
	if e != nil || b.Ref != a.Ref || b.Alias != "Custom Alias" || b.FirstName != "Annie" || !b.Monitored || !b.ExtraTime {
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
	if e != nil || v.Name != "Ana Jones" || v.FirstName != "Annie" || !v.ExtraTime {
		t.Fatal("save/resolve failed")
	}
	if _, e = s.Resolve(other.Instance, a.Ref); e == nil {
		t.Fatal("cross-instance resolution")
	}
}

func TestLegacyStudentTableGainsExtraTimeWithoutChangingIdentity(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".cse", "students.csv")
	legacy := strings.Join(legacyStudentHeader, ",") + "\ns-existing,https://school.example,7,Ana Smith,Ana,Pikachu,sis7,ana,true\n"
	if err := WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := OpenStudents(root)
	if err != nil || len(store.Rows) != 1 || store.Rows[0].ExtraTime || !store.Rows[0].Monitored {
		t.Fatalf("legacy table not readable: %v", err)
	}
	store.Rows[0].ExtraTime = true
	if err := store.Save(); err != nil {
		t.Fatal(err)
	}
	store, err = OpenStudents(root)
	want := idAlias("https://school.example", 7, map[string]bool{})
	if err != nil || store.Rows[0].Ref != "s-existing" || store.Rows[0].Alias != want || !store.Rows[0].ExtraTime {
		t.Fatalf("migration lost student identity or flag: %v", err)
	}
	b, _ := os.ReadFile(path)
	if !strings.HasPrefix(string(b), strings.Join(studentHeader, ",")) {
		t.Fatal("new column not saved")
	}
}

// The incident: a second install without the shared table gave the same
// students different names. Canvas-ID aliases make a fresh table agree, the
// one-time rename keeps typed names, and later teacher renames stick.
func TestAliasesComeFromCanvasIDOnEveryInstall(t *testing.T) {
	const school = "https://school.example"
	ids := []int64{11, 12, 13, 14, 15}
	root := t.TempDir()
	rows := strings.Join(extraTimeStudentHeader, ",") + "\n" +
		"s-a,https://school.example,11,A One,A,Pikachu,,,false,false\n" +
		"s-b,https://school.example,12,B Two,B,Ninetales,,,true,false\n" +
		"s-c,https://school.example,13,C Three,C,Captain C,,,false,true\n"
	if err := WriteFile(filepath.Join(root, ".cse", "students.csv"), []byte(rows), 0o600); err != nil {
		t.Fatal(err)
	}
	migrated, err := OpenStudents(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range ids[3:] {
		if _, err = migrated.Upsert(school, id, "New Student", "", "", ""); err != nil {
			t.Fatal(err)
		}
	}
	fresh, _ := OpenStudents(t.TempDir())
	for i := len(ids) - 1; i >= 0; i-- {
		if _, err = fresh.Upsert(school, ids[i], "Same Student", "", "", ""); err != nil {
			t.Fatal(err)
		}
	}
	alias := func(s *StudentStore, id int64) Student {
		for _, v := range s.Rows {
			if v.UserID == id {
				return v
			}
		}
		t.Fatalf("student %d missing", id)
		return Student{}
	}
	if v := alias(migrated, 13); v.Alias != "Captain C" || v.AliasSource != AliasFromTeacher {
		t.Fatalf("typed alias replaced: %+v", v)
	}
	for _, id := range []int64{11, 12, 14, 15} {
		m, f := alias(migrated, id), alias(fresh, id)
		if m.Alias != f.Alias || m.AliasSource != AliasFromCanvasID {
			t.Fatalf("student %d: migrated %q, fresh install %q", id, m.Alias, f.Alias)
		}
	}
	if err = migrated.Save(); err != nil {
		t.Fatal(err)
	}
	reopened, _ := OpenStudents(root)
	reopened.Rows[0].Alias, reopened.Rows[0].AliasSource = "Teacher Pick", AliasFromTeacher
	if err = reopened.Save(); err != nil {
		t.Fatal(err)
	}
	if again, _ := OpenStudents(root); again.Rows[0].Alias != "Teacher Pick" || alias(again, 12).Alias != alias(migrated, 12).Alias {
		t.Fatal("aliases changed after the one-time rename")
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
	a.FirstName = "Sammy"
	text := ReplaceStudentName("{{student_name}}, your quotation says Pikachu.", a)
	if text != "Sammy, your quotation says Pikachu." || strings.Contains(text, "private") {
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
