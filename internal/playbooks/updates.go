package playbooks

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	embedded "canvasexpertse"
	"canvasexpertse/internal/workspace"
)

type StatusRow struct {
	Path        string `json:"path"`
	State       string `json:"state"`
	SHA256      string `json:"sha256,omitempty"`
	Replaceable bool   `json:"replaceable"`
}

func seedPaths() []string {
	var out []string
	_ = fs.WalkDir(embedded.WorkspaceTemplate, "workspace-template", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel := strings.TrimPrefix(name, "workspace-template/")
		if rel == "AGENTS.md" || rel == "CLAUDE.md" || rel == "TEACHER.md" || rel == overrideFile || strings.HasPrefix(rel, "playbooks/") || strings.HasPrefix(rel, "context/") {
			out = append(out, rel)
		}
		return nil
	})
	sort.Strings(out)
	return out
}
// overrideFile holds the teacher's overrides of shipped defaults. Like
// context/, it is teacher-owned: seeded when missing, never replaced.
const overrideFile = "Teacher_Override.md"

func contextPath(rel string) bool { return strings.HasPrefix(rel, "context/") || rel == overrideFile }
func seedBytes(rel string) ([]byte, error) {
	return fs.ReadFile(embedded.WorkspaceTemplate, path.Join("workspace-template", rel))
}

func SeedBytes(rel string) ([]byte, error) { return seedBytes(rel) }

func Status(root string) []StatusRow {
	var rows []StatusRow
	for _, rel := range seedPaths() {
		row := StatusRow{Path: rel}
		seed, err := seedBytes(rel)
		active, pathErr := workspace.ResolveTeacherPath(root, rel)
		var have []byte
		var readErr error
		if pathErr != nil {
			readErr = pathErr
		} else {
			have, readErr = os.ReadFile(active)
		}
		switch {
		case err != nil:
			continue
		case pathErr != nil:
			row.State = "unavailable"
		case os.IsNotExist(readErr):
			row.State, row.Replaceable = "missing", !contextPath(rel)
		case readErr != nil:
			row.State = "unavailable"
		case bytes.Equal(have, seed):
			row.State, row.SHA256 = "same", fmt.Sprintf("%x", sha256.Sum256(have))
		default:
			row.State, row.Replaceable, row.SHA256 = "differs", !contextPath(rel), fmt.Sprintf("%x", sha256.Sum256(have))
		}
		rows = append(rows, row)
	}
	return rows
}

// Replace adopts one seed file. expectedSHA256 is the active file's hash, or
// "missing" for an explicit add. An empty precondition is reserved for the
// Settings UI's existing confirm flow.
func Replace(root, rel, expectedSHA256 string, now time.Time) error {
	rel = path.Clean(strings.ReplaceAll(rel, "\\", "/"))
	known := false
	for _, p := range seedPaths() {
		known = known || p == rel
	}
	if !known || contextPath(rel) {
		return fmt.Errorf("%s can't be replaced from the hub", rel)
	}
	seed, err := seedBytes(rel)
	if err != nil {
		return err
	}
	target, err := workspace.ResolveTeacherPath(root, rel)
	if err != nil {
		return err
	}
	current, statErr := os.ReadFile(target)
	if statErr == nil {
		if expectedSHA256 != "" && !strings.EqualFold(expectedSHA256, fmt.Sprintf("%x", sha256.Sum256(current))) {
			return fmt.Errorf("%s changed since status was read; reread playbook status before replacing", rel)
		}
		if expectedSHA256 == "missing" {
			return fmt.Errorf("%s exists; reread playbook status before replacing", rel)
		}
		oldRel := path.Join("playbook-old", now.Format("2006-01-02"), rel)
		dest, err := workspace.ResolveRulesPath(root, filepath.FromSlash(oldRel))
		if err != nil {
			return err
		}
		ext := filepath.Ext(dest)
		base := strings.TrimSuffix(dest, ext)
		for n := 2; ; n++ {
			_, e := os.Lstat(dest)
			if os.IsNotExist(e) {
				break
			}
			if e != nil {
				return e
			}
			dest = fmt.Sprintf("%s-%d%s", base, n, ext)
		}
		if err = os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return err
		}
		if err = os.WriteFile(dest, current, 0644); err != nil {
			return err
		}
		archiveRoot, err := workspace.ResolveRulesPath(root, filepath.Join("playbook-old", now.Format("2006-01-02")))
		if err != nil {
			return err
		}
		if err = writeInactiveMarker(archiveRoot); err != nil {
			return err
		}
	} else if os.IsNotExist(statErr) {
		if expectedSHA256 != "" && expectedSHA256 != "missing" {
			return fmt.Errorf("%s is missing; reread playbook status and use the explicit missing precondition to add it", rel)
		}
	} else {
		return statErr
	}
	if err = os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	tmp := target + ".cse-update-tmp"
	if err = os.WriteFile(tmp, seed, 0644); err != nil {
		return err
	}
	if err = os.Rename(tmp, target); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// writeInactiveMarker labels only the date-stamped archive directory just
// created by Replace. It never scans or labels arbitrary teacher folders.
func writeInactiveMarker(dir string) error {
	const marker = "INACTIVE.txt"
	content := "Preserved history: this folder contains a previous CE-SE instruction or playbook copy. It is inactive and is not a source of current rules. Read the active AGENTS.md from the effective teacher-rules location (UserInstall by default, configured UserSync otherwise). To compare and adopt a current seed, use Settings → Your playbooks and context or `cse playbooks status`, then explicitly replace only the selected file.\n"
	p := filepath.Join(dir, marker)
	if _, err := os.Lstat(p); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.WriteFile(p, []byte(content), 0644)
}
