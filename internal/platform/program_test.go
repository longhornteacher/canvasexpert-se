package platform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLinkProgramRecordsExecutableOutsideWorkspace(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	exe, err := LinkProgram()
	if err != nil {
		t.Fatal(err)
	}
	got, err := LinkedProgram()
	if err != nil || got != exe {
		t.Fatalf("link round trip: %q %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(os.Getenv("LOCALAPPDATA"), "CanvasExpert-SE", "program.txt")); err != nil {
		t.Fatal(err)
	}
}

func TestLinkedProgramMissing(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	if _, err := LinkedProgram(); err == nil {
		t.Fatal("expected an error without a link")
	}
}

func TestOverlaps(t *testing.T) {
	root := t.TempDir()
	inner := filepath.Join(root, "cse")
	sibling := filepath.Join(root, "cse-other")
	if !Overlaps(root, inner) || !Overlaps(inner, root) || !Overlaps(inner, inner) || Overlaps(inner, sibling) {
		t.Fatal("overlap detection wrong")
	}
	engine := filepath.Join(root, BundledEngineDir)
	if !SeparateFrom(root, engine) || SeparateFrom(engine, root) || SeparateFrom(root, inner) || !SeparateFrom(inner, sibling) || !SeparateFrom(root, "") {
		t.Fatal("bundled engine rule wrong")
	}
}
