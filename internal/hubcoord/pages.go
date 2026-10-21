package hubcoord

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const pagesDir = "open-pages"

type pageRecord struct {
	Kind string `json:"kind"`
	URL  string `json:"url"`
}

// MarkPage records that a private page (a review, setup or students page) is
// open in this UserInstall, so an Engine update waits for the teacher to
// finish. The returned func removes the record when the page closes.
func MarkPage(root, kind, origin string) func() {
	if root == "" {
		return func() {}
	}
	if _, err := validURL(origin); err != nil {
		return func() {}
	}
	dir := filepath.Join(CanonicalRoot(root), ".cse", pagesDir)
	if os.MkdirAll(dir, 0700) != nil {
		return func() {}
	}
	path := filepath.Join(dir, strconv.Itoa(os.Getpid())+"-"+kind+".json")
	b, _ := json.Marshal(pageRecord{Kind: kind, URL: origin})
	if os.WriteFile(path, b, 0600) != nil {
		return func() {}
	}
	return func() { _ = os.Remove(path) }
}

// OpenPages lists the kinds of private pages still answering in this
// UserInstall. Records whose page no longer answers are removed.
func OpenPages(root string) []string {
	dir := filepath.Join(CanonicalRoot(root), ".cse", pagesDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var open []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		b, err := os.ReadFile(path)
		var rec pageRecord
		if err != nil || json.Unmarshal(b, &rec) != nil {
			_ = os.Remove(path)
			continue
		}
		u, err := validURL(rec.URL)
		if err != nil {
			_ = os.Remove(path)
			continue
		}
		conn, err := net.DialTimeout("tcp4", u.Host, 500*time.Millisecond)
		if err != nil {
			_ = os.Remove(path)
			continue
		}
		_ = conn.Close()
		open = append(open, rec.Kind)
	}
	return open
}
