package main

import (
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
