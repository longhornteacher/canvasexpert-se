package workspace

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"canvasexpertse/internal/domain"
)

func TestStrictHeaderRejectsDuplicatesAndUnknownFields(t *testing.T) {
	for _, header := range []string{
		"schema: 1\nvoice: concise\nvoice: verbose\n",
		"schema: 1\nunrecognized: true\n",
		"schema: 1\nvoice: &voice concise\nfeedback_guidance: *voice\n",
		"schema: 1\nnotes:\n  - student_ref: a\n    student_ref: b\n    note: synthetic\n",
	} {
		var target any = &domain.GradingConfig{Schema: 1, Voice: "original"}
		if strings.Contains(header, "notes:") {
			target = &domain.StudentNotes{Schema: 1}
		}
		if _, e := ParseDocument([]byte("---\n"+header+"---\nbody"), target); e == nil {
			t.Fatalf("accepted invalid header %q", header)
		}
		if c, ok := target.(*domain.GradingConfig); ok && c.Voice != "original" {
			t.Fatal("failed parse mutated target")
		}
	}
}
func TestBodyIsPreservedAsContent(t *testing.T) {
	body := "---\r\nschema: words in the teacher's text\r\n<div data-extra=\"keep\">HTML</div>\r\n"
	var c domain.GradingConfig
	got, e := ParseDocument([]byte("\xef\xbb\xbf---\r\nschema: 1\r\n---\r\n"+body), &c)
	if e != nil {
		t.Fatal(e)
	}
	if got != body {
		t.Fatalf("body changed: %q", got)
	}
	if _, e = ParseDocument([]byte("---\nschema: 1\n---\n"), nil); e == nil {
		t.Fatal("nil target accepted")
	}
}
func TestDocumentKindUsesYAMLStrings(t *testing.T) {
	kind, e := DocumentKind([]byte("---\n\"schema\": 1\n'kind': \"results\"\n---\nkind: assignment\n"))
	if e != nil || kind != "results" {
		t.Fatalf("wrong kind %q %v", kind, e)
	}
	if _, e = DocumentKind([]byte("---\nschema: 1\nkind: assignment\nkind: results\n---\n")); e == nil {
		t.Fatal("kind discovery bypassed duplicate key validation")
	}
}
func TestInvalidDocumentDoesNotReplaceExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "draft.md")
	original := []byte("prepared work stays here")
	if e := os.WriteFile(path, original, 0600); e != nil {
		t.Fatal(e)
	}
	d := domain.AssignmentDocument{Schema: 1, Kind: "assignment", Operation: "create", Course: "CS8", Title: "Synthetic", SubmissionTypes: []string{"online_text_entry"}}
	if e := WriteDocument(path, d, "Directions"); e == nil {
		t.Fatal("missing maximum became explicit zero during serialization")
	}
	b, e := os.ReadFile(path)
	if e != nil || string(b) != string(original) {
		t.Fatalf("invalid write altered file: %q %v", b, e)
	}
}
func TestWindowsReplacementWaitsForBriefReader(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows sharing behavior")
	}
	path := filepath.Join(t.TempDir(), "result.md")
	if e := os.WriteFile(path, []byte("old result"), 0600); e != nil {
		t.Fatal(e)
	}
	reader, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	released := make(chan struct{})
	go func() { time.Sleep(25 * time.Millisecond); reader.Close(); close(released) }()
	e = WriteFile(path, []byte("known new outcomes"), 0600)
	<-released
	if e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(path)
	if e != nil || string(b) != "known new outcomes" {
		t.Fatalf("replacement failed: %q %v", b, e)
	}
}
