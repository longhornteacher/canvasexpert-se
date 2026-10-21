package hubcoord

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenPagesListsLivePagesAndDropsClosedOnes(t *testing.T) {
	root := t.TempDir()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	release := MarkPage(root, "review", "http://"+listener.Addr().String(), filepath.Join(root, "courses", "results.md"))
	if open := OpenPages(root); len(open) != 1 || open[0] != "review" {
		t.Fatalf("open page not listed: %v", open)
	}
	if rec, ok := FindPage(root, "review", filepath.Join(root, "courses", "results.md")); !ok || rec.URL != "http://"+listener.Addr().String() {
		t.Fatalf("page not found by its file: %+v %v", rec, ok)
	}
	if _, ok := FindPage(root, "review", filepath.Join(root, "courses")); !ok {
		t.Fatal("page not found by its folder")
	}
	if _, ok := FindPage(root, "review", filepath.Join(root, "courses", "other.md")); ok {
		t.Fatal("another file matched")
	}
	_ = listener.Close()
	if open := OpenPages(root); len(open) != 0 {
		t.Fatalf("closed page still listed: %v", open)
	}
	entries, _ := os.ReadDir(filepath.Join(CanonicalRoot(root), ".cse", pagesDir))
	if len(entries) != 0 {
		t.Fatalf("stale record kept: %d", len(entries))
	}
	release() // removing an already-dropped record is harmless
	if MarkPage("", "review", "http://127.0.0.1:1", "") == nil {
		t.Fatal("empty root must return a no-op release")
	}
}
