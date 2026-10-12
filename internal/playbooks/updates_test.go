package playbooks

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReplaceUsesHashAndExplicitMissingPrecondition(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "playbooks"), 0755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(root, "playbooks", "score.md")
	if err := os.WriteFile(p, []byte("teacher edits"), 0644); err != nil {
		t.Fatal(err)
	}
	err := Replace(root, "playbooks/score.md", "stale", time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC))
	if err == nil || !strings.Contains(err.Error(), "reread") {
		t.Fatalf("expected conflict, got %v", err)
	}
	if b, _ := os.ReadFile(p); string(b) != "teacher edits" {
		t.Fatal("stale hash changed active content")
	}
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte("teacher edits")))
	if err := Replace(root, "playbooks/score.md", hash, time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	old, err := filepath.Glob(filepath.Join(root, "playbook-old", "2026-10-12", "playbooks", "score.md"))
	if err != nil || len(old) != 1 {
		t.Fatalf("previous file not archived: %v %v", old, err)
	}
	if b, _ := os.ReadFile(old[0]); string(b) != "teacher edits" {
		t.Fatal("archive differs from previous file")
	}
	marker := filepath.Join(root, "playbook-old", "2026-10-12", "INACTIVE.txt")
	if b, err := os.ReadFile(marker); err != nil || !strings.Contains(string(b), "Preserved history") || !strings.Contains(string(b), "cse playbooks status") {
		t.Fatalf("inactive archive marker missing or incomplete: %q, %v", b, err)
	}
	for _, rel := range seedPaths() {
		if strings.Contains(rel, "playbook-old") || strings.HasSuffix(rel, "INACTIVE.txt") {
			t.Fatalf("archive marker became a seed candidate: %q", rel)
		}
	}
	for _, row := range Status(root) {
		if row.Path == "playbook-old/2026-10-12/INACTIVE.txt" {
			t.Fatal("archive marker became an adoption candidate")
		}
	}
	if err := Replace(root, "playbook-old/2026-10-12/INACTIVE.txt", "missing", time.Now()); err == nil {
		t.Fatal("archive marker was accepted as a replaceable seed")
	}
	missing := filepath.Join(root, "playbooks", "score.md")
	if err := os.Remove(missing); err != nil {
		t.Fatal(err)
	}
	if err := Replace(root, "playbooks/score.md", "stale", time.Now()); err == nil {
		t.Fatal("missing file accepted without explicit precondition")
	}
	if err := Replace(root, "playbooks/score.md", "missing", time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(missing); err != nil {
		t.Fatal("explicit missing replacement did not add seed", err)
	}
}

func TestReplaceProtectsContextAndUnavailableSync(t *testing.T) {
	root := t.TempDir()
	ctx := filepath.Join(root, "context", "grading.md")
	if err := os.MkdirAll(filepath.Dir(ctx), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ctx, []byte("teacher preference"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := Replace(root, "context/grading.md", "missing", time.Now()); err == nil {
		t.Fatal("context preference was replaceable")
	}
	if b, _ := os.ReadFile(ctx); string(b) != "teacher preference" {
		t.Fatal("context preference changed")
	}
	if err := os.MkdirAll(filepath.Join(root, ".cse"), 0700); err != nil {
		t.Fatal(err)
	}
	binding := []byte(fmt.Sprintf(`{"sync_root":%q}`, filepath.Join(root, "unavailable-sync")))
	if err := os.WriteFile(filepath.Join(root, ".cse", "rules-location.json"), binding, 0600); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(root, "playbooks", "score.md")
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("local copy"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := Replace(root, "playbooks/score.md", "missing", time.Now()); err == nil {
		t.Fatal("replacement fell back from unavailable Sync")
	}
	if b, _ := os.ReadFile(p); string(b) != "local copy" {
		t.Fatal("unavailable Sync changed inactive local copy")
	}
	rows := Status(root)
	if len(rows) == 0 || rows[0].State != "unavailable" {
		t.Fatal("status did not report unavailable active authority")
	}
}
