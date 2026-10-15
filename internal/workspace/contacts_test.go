package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestContactFormattingAndOptionalDestinations(t *testing.T) {
	for raw, want := range map[string]string{"": "", "(512) 555-0123": "512-555-0123", "+1 512 555 0123": "512-555-0123"} {
		got, err := ContactPhone(raw)
		if err != nil || got != want {
			t.Fatalf("phone %q: %q %v", raw, got, err)
		}
	}
	for _, raw := range []string{"123", "+44 20 1234 5678", "call5125550123"} {
		if _, err := ContactPhone(raw); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
	if email, err := ContactEmail("  parent@example.com "); err != nil || email != "parent@example.com" {
		t.Fatal(email, err)
	}
	if _, err := ContactEmail("a@example.com; b@example.com"); err == nil {
		t.Fatal("accepted multiple addresses")
	}
	if _, err := ContactEmail(""); err != nil {
		t.Fatal(err)
	}
}

func TestContactsFollowSyncAndCanvasIdentity(t *testing.T) {
	root, syncRoot := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "context"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "context", "courses.md"), []byte("courses"), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := OpenContacts(root)
	if err != nil {
		t.Fatal(err)
	}
	student := Student{Ref: "local-one", Instance: "https://school.example", UserID: 42}
	s.Students[ContactKey(student)] = StudentContacts{StudentEmail: "student@example.com", Parents: []ParentContact{{Email: "parent@example.com", Text: true}}}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	if _, err := SetRulesLocation(root, syncRoot, nil, ""); err != nil {
		t.Fatal(err)
	}
	other := t.TempDir()
	if err := writeRulesBinding(other, syncRoot); err != nil {
		t.Fatal(err)
	}
	shared, err := OpenContacts(other)
	if err != nil {
		t.Fatal(err)
	}
	student.Ref = "different-local-ref"
	entry := shared.Students[ContactKey(student)]
	if entry.StudentEmail != "student@example.com" || !entry.Parents[0].Text || entry.Parents[0].Phone != "" {
		t.Fatalf("contact lost: %+v", entry)
	}
	if IsSharedRulePath(contactsFile) {
		t.Fatal("private contacts exposed as teacher rules")
	}
	if _, err := SetRulesLocation(root, "", nil, ""); err != nil {
		t.Fatal(err)
	}
	local, err := OpenContacts(root)
	if err != nil || local.Students[ContactKey(student)].StudentEmail != entry.StudentEmail {
		t.Fatal("return to local lost contacts", err)
	}
	if err := writeRulesBinding(other, filepath.Join(syncRoot, "missing")); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenContacts(other); err == nil {
		t.Fatal("unavailable UserSync silently fell back")
	}
}
