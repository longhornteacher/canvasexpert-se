package playbooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGitInstallUpdateStatusAndReplaceChangeNothing(t *testing.T) {
	root := t.TempDir()
	for name, body := range map[string]string{
		"context/courses.md":  "---\nschema: 1\n---\n",
		"engine/manifest.txt": "version=1.2.0\nidentity=1.2.0\n",
		"playbooks/score.md":  "git-delivered, edited text",
	} {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	report, err := Update(root, time.Now())
	if err != nil || report.ManagedBy != "git" || len(report.Files) != 0 || report.Note == "" {
		t.Fatalf("update report %+v %v", report, err)
	}
	for _, path := range []string{"shipped", "playbook-old"} {
		if _, err := os.Stat(filepath.Join(root, path)); err == nil {
			t.Fatalf("%s was created in a git install", path)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(root, "playbooks", "score.md")); string(b) != "git-delivered, edited text" {
		t.Fatalf("playbook was modified: %q", b)
	}
	if rows := Status(root); len(rows) != 0 {
		t.Fatalf("status rows in a git install: %v", rows)
	}
	if err := Replace(root, "playbooks/score.md", "", time.Now()); err == nil || !strings.Contains(err.Error(), "git") {
		t.Fatalf("replace in a git install: %v", err)
	}
}
