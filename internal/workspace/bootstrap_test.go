package workspace

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestBootstrapBundledLaysDownStarterFilesOnlyInAnExtractedFolder(t *testing.T) {
	template := fstest.MapFS{
		"t/context/courses.md": {Data: []byte("starter courses")},
		"t/playbooks/score.md": {Data: []byte("seed score")},
	}
	root := t.TempDir()
	exe := filepath.Join(root, "engine", "cse.exe")
	if err := BootstrapBundled(exe, template, "t"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "context", "courses.md")); !os.IsNotExist(err) {
		t.Fatal("a folder without AGENTS.md is not a UserInstall")
	}
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("bootstrap"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := BootstrapBundled(exe, template, "t"); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"context/courses.md", "playbooks/score.md"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("%s not laid down: %v", rel, err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "context", "courses.md"), []byte("my courses"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := BootstrapBundled(exe, template, "t"); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(root, "context", "courses.md")); string(b) != "my courses" {
		t.Fatal("bootstrap overwrote saved courses")
	}
}
