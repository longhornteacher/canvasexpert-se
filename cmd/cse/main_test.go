package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHelpAndVersionNeedNoWorkspaceOrCanvas(t *testing.T) {
	for _, args := range [][]string{nil, {"help"}, {"--help"}, {"version"}} {
		var out, err bytes.Buffer
		if code := run(context.Background(), args, &out, &err); code != 0 || !strings.Contains(out.String(), "CanvasExpert-SE") {
			t.Fatalf("%v: %d %s", args, code, err.String())
		}
	}
}
func TestInvalidResultsAreRejectedBeforeCredentials(t *testing.T) {
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, "context"), 0755)
	_ = os.WriteFile(filepath.Join(root, "context", "courses.md"), []byte("---\nschema: 1\ninstance: https://school.example\naliases: true\ncourses: []\n---\n"), 0644)
	_ = os.WriteFile(filepath.Join(root, "bad.md"), []byte("---\nschema: 1\nkind: results\nkind: results\n---\n"), 0644)
	var out, err bytes.Buffer
	code := run(context.Background(), []string{"--workspace", root, "post", "bad.md"}, &out, &err)
	if code != 2 || strings.Contains(err.String(), "credential") {
		t.Fatalf("invalid file reached environment: %d %s", code, err.String())
	}
}

func TestHelpFormatsAndExamplesNeedNoWorkspace(t *testing.T) {
	for _, topic := range []string{"formats", "examples"} {
		var out, err bytes.Buffer
		if code := run(context.Background(), []string{"help", topic}, &out, &err); code != 0 || out.Len() == 0 {
			t.Fatalf("help %s: %d %s", topic, code, err.String())
		}
	}
}
