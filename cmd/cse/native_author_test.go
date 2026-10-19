package main

import (
	"os"
	"path/filepath"
	"testing"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

func TestNativeDocumentValidationAndUploadFlagContract(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct {
		name string
		doc  any
		body string
	}{
		{"page.md", domain.PageDocument{Schema: 1, Kind: "page", Operation: "create", Course: "TEST", Title: "Lesson", BodyFormat: "html"}, "<p>Lesson</p>"},
		{"discussion.md", domain.DiscussionDocument{Schema: 1, Kind: "discussion", Operation: "create", Course: "TEST", Title: "Club prompt", BodyFormat: "html"}, "<p>Respond</p>"},
		{"module.md", domain.ModuleDocument{Schema: 1, Kind: "module", Course: "TEST", Title: "Unit", Items: []domain.ModuleItem{{Type: "Page", ContentID: 7}}}, "Conversion notes"},
	} {
		path := filepath.Join(root, tc.name)
		if e := workspace.WriteDocument(path, tc.doc, tc.body); e != nil {
			t.Fatal(e)
		}
		if e := validateDocument(path, root); e != nil {
			t.Fatalf("%s: %v", tc.name, e)
		}
	}
	graded := filepath.Join(root, "graded-discussion.md")
	content := "---\nschema: 1\nkind: discussion\noperation: create\ncourse: TEST\ntitle: Graded prompt\nbody_format: html\nassignment:\n  points: 100\n  grading_type: points\n  due_at: '2026-11-06T23:59:00-06:00'\n  rubric:\n    title: Evidence\n    criteria:\n      - id: evidence\n        description: Use evidence\n        points: 100\n---\n<p>Respond</p>\n"
	if err := os.WriteFile(graded, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	if err := validateDocument(graded, root); err != nil {
		t.Fatalf("graded discussion did not validate: %v", err)
	}
	source, opts, e := parseCourseUpload(root, []string{"TEST", "image.png", "--access", "hidden", "--folder", "4", "--receipt", "image.receipt.json", "--new-upload"})
	if e != nil || source != filepath.Join(root, "image.png") || opts.Access != "hidden" || opts.FolderID != 4 || !opts.NewUpload || opts.ReceiptPath != filepath.Join(root, "image.receipt.json") {
		t.Fatalf("upload flags %s %#v %v", source, opts, e)
	}
	for _, args := range [][]string{{"TEST", "image.png", "--folder", "4"}, {"TEST", "image.png", "--access", "private"}, {"TEST", "image.png", "--access", "course", "extra"}} {
		if _, _, e := parseCourseUpload(root, args); e == nil {
			t.Fatalf("invalid upload accepted: %v", args)
		}
	}
}

func TestGroupEvidenceSpecimensValidateAsTypedDocuments(t *testing.T) {
	for _, name := range []string{"synthetic-group-packet.md", "synthetic-group-work.md", "synthetic-group-results.md"} {
		path := filepath.Join("..", "..", "engine-docs", "examples", name)
		if err := validateDocument(path); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
}
