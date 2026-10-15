package localui

import (
	"canvasexpertse/internal/workspace"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestStudentContactsOptionalDestinationsAndInvalidDraft(t *testing.T) {
	root := t.TempDir()
	store, err := workspace.OpenStudents(root)
	if err != nil {
		t.Fatal(err)
	}
	student, err := store.Upsert("https://school.example", 42, "Synthetic Student", "Synthetic", "", "")
	if err != nil {
		t.Fatal(err)
	}
	h := &studentHandler{origin: "http://127.0.0.1:1234", secret: "nonce", store: store, notesRoot: root, persistent: true}
	form := url.Values{"request_secret": {"nonce"}, "alias_" + student.Ref: {student.Alias}, "student_email_" + student.Ref: {"student@example.com"}, "contact_phone_" + student.Ref: {"", "+1 (512) 555-0123"}, "contact_email_" + student.Ref: {"parent@example.com", ""}, "contact_text_" + student.Ref + "_0": {"yes"}, "contact_send_email_" + student.Ref + "_1": {"yes"}}
	post := func() *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, h.origin+"/students", strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("Origin", h.origin)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := post(); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	contacts, err := workspace.OpenContacts(root)
	if err != nil {
		t.Fatal(err)
	}
	entry := contacts.Students[workspace.ContactKey(student)]
	if len(entry.Parents) != 2 || !entry.Parents[0].Text || !entry.Parents[1].EmailEnabled || entry.Parents[1].Phone != "512-555-0123" {
		t.Fatalf("optional channels: %+v", entry)
	}
	form["contact_phone_"+student.Ref] = []string{"123", ""}
	form["contact_email_"+student.Ref] = []string{"unsaved@example.com", ""}
	if w := post(); w.Code != 400 || !strings.Contains(w.Body.String(), "unsaved@example.com") || !strings.Contains(w.Body.String(), `value="123"`) {
		t.Fatal("invalid entries were not preserved", w.Code, w.Body.String())
	}
	contacts, err = workspace.OpenContacts(root)
	if err != nil {
		t.Fatal(err)
	}
	if contacts.Students[workspace.ContactKey(student)].Parents[0].Email != "parent@example.com" {
		t.Fatal("invalid form overwrote saved contacts")
	}
	// Removing every row deliberately clears the parents without losing student email.
	delete(form, "contact_phone_"+student.Ref)
	if w := post(); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	contacts, _ = workspace.OpenContacts(root)
	if len(contacts.Students[workspace.ContactKey(student)].Parents) != 0 {
		t.Fatal("removed rows survived")
	}
}
