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

// PageRecord describes one private page that is open in a UserInstall.
// Subject is the file or folder the page reviews, when it has one.
type PageRecord struct {
	Kind    string `json:"kind"`
	URL     string `json:"url"`
	Subject string `json:"subject,omitempty"`
}

// MarkPage records that a private page (a review, setup or students page) is
// open in this UserInstall, so an Engine update waits for the teacher to finish
// and a restarted agent can reconnect to it. The returned func removes the
// record when the page closes.
func MarkPage(root, kind, origin, subject string) func() {
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
	if subject != "" {
		subject = CanonicalRoot(subject)
	}
	path := filepath.Join(dir, strconv.Itoa(os.Getpid())+"-"+kind+".json")
	b, _ := json.Marshal(PageRecord{Kind: kind, URL: origin, Subject: subject})
	if os.WriteFile(path, b, 0600) != nil {
		return func() {}
	}
	return func() { _ = os.Remove(path) }
}

// OpenPageRecords lists the private pages still answering in this
// UserInstall. Records whose page no longer answers are removed.
func OpenPageRecords(root string) []PageRecord {
	dir := filepath.Join(CanonicalRoot(root), ".cse", pagesDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var open []PageRecord
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		b, err := os.ReadFile(path)
		var rec PageRecord
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
		open = append(open, rec)
	}
	return open
}

// OpenPages lists the kinds of private pages still answering in this UserInstall.
func OpenPages(root string) []string {
	var kinds []string
	for _, rec := range OpenPageRecords(root) {
		kinds = append(kinds, rec.Kind)
	}
	return kinds
}

// FindPage returns the open page of this kind whose subject is path, or lies
// inside path when path is a folder.
func FindPage(root, kind, path string) (PageRecord, bool) {
	want := CanonicalRoot(path)
	for _, rec := range OpenPageRecords(root) {
		if rec.Kind != kind || rec.Subject == "" {
			continue
		}
		if rec.Subject == want || strings.HasPrefix(rec.Subject, want+string(filepath.Separator)) {
			return rec, true
		}
	}
	return PageRecord{}, false
}

// PageOpen reports whether the page at this URL (its origin or a path on it)
// still answers.
func PageOpen(root, pageURL string) bool {
	for _, rec := range OpenPageRecords(root) {
		if rec.URL == pageURL || strings.HasPrefix(pageURL, rec.URL+"/") {
			return true
		}
	}
	return false
}
