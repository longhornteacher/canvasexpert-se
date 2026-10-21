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
	release := MarkPage(root, "review", "http://"+listener.Addr().String())
	if open := OpenPages(root); len(open) != 1 || open[0] != "review" {
		t.Fatalf("open page not listed: %v", open)
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
	if MarkPage("", "review", "http://127.0.0.1:1") == nil {
		t.Fatal("empty root must return a no-op release")
	}
}
