package playbooks

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	_ "embed"

	embedded "canvasexpertse"
	"canvasexpertse/internal/workspace"
)

// shippedDir keeps each adopted seed in UserInstall so the agent can compare
// the previous shipped file, the teacher's active file and the new seed.
const shippedDir = "shipped"

// UpdateRow reports what an update did with one seed file:
//   - added: it was missing and the seed was written
//   - current: it already matched the new seed
//   - updated: it still matched the previous shipped copy, so the new seed
//     replaced it (the old copy is archived under playbook-old)
//   - kept: the teacher's version differs from what shipped, or no previous
//     shipped copy exists; it stays active for the agent to reconcile
//   - teacher-owned: context/ and Teacher_Override.md are never replaced
type UpdateRow struct {
	Path  string `json:"path"`
	State string `json:"state"`
}

type UpdateReport struct {
	Previous string      `json:"previous_shipped,omitempty"`
	Current  string      `json:"current_shipped"`
	Files    []UpdateRow `json:"files"`
}

// SeedLabel names this Engine's seed by release version and content hash, so
// development builds with the same version string stay distinct.
func SeedLabel() string {
	h := sha256.New()
	for _, rel := range seedPaths() {
		b, _ := seedBytes(rel)
		fmt.Fprintf(h, "%s\x00%d\x00", rel, len(b))
		h.Write(b)
	}
	return fmt.Sprintf("%s-%x", embedded.ReleaseVersion(), h.Sum(nil)[:4])
}

// Update adopts this Engine's seed without overriding teacher choices. Every
// difference from the previous shipped copy counts as the teacher's choice.
func Update(root string, now time.Time) (UpdateReport, error) {
	if err := workspace.RestoreSyncBootstrap(root); err != nil {
		return UpdateReport{}, err
	}
	label := SeedLabel()
	report := UpdateReport{Current: label}
	adoptedPath, err := workspace.ConstrainPath(root, path.Join(shippedDir, "adopted.txt"))
	if err != nil {
		return report, err
	}
	if b, e := os.ReadFile(adoptedPath); e == nil {
		report.Previous = strings.TrimSpace(string(b))
	} else if !os.IsNotExist(e) {
		return report, e
	}
	for _, rel := range seedPaths() {
		seed, err := seedBytes(rel)
		if err != nil {
			return report, err
		}
		active, err := workspace.ResolveTeacherPath(root, rel)
		if err != nil {
			return report, err
		}
		have, readErr := os.ReadFile(active)
		row := UpdateRow{Path: rel}
		switch {
		case os.IsNotExist(readErr):
			if err = os.MkdirAll(filepath.Dir(active), 0755); err != nil {
				return report, err
			}
			if err = os.WriteFile(active, seed, 0644); err != nil {
				return report, err
			}
			row.State = "added"
		case readErr != nil:
			return report, readErr
		case contextPath(rel):
			row.State = "teacher-owned"
		case bytes.Equal(have, seed):
			row.State = "current"
		case report.Previous != "" && matchesShipped(root, report.Previous, rel, have), pastSeed(rel, have):
			if err = Replace(root, rel, fmt.Sprintf("%x", sha256.Sum256(have)), now); err != nil {
				return report, err
			}
			row.State = "updated"
		default:
			row.State = "kept"
		}
		report.Files = append(report.Files, row)
	}
	if err = writeShipped(root, label); err != nil {
		return report, err
	}
	return report, os.WriteFile(adoptedPath, []byte(label+"\n"), 0644)
}

func matchesShipped(root, label, rel string, have []byte) bool {
	p, err := workspace.ConstrainPath(root, path.Join(shippedDir, label, rel))
	if err != nil {
		return false
	}
	old, err := os.ReadFile(p)
	return err == nil && bytes.Equal(old, have)
}

func writeShipped(root, label string) error {
	for _, rel := range seedPaths() {
		if contextPath(rel) {
			continue
		}
		b, err := seedBytes(rel)
		if err != nil {
			return err
		}
		p, err := workspace.ConstrainPath(root, path.Join(shippedDir, label, rel))
		if err != nil {
			return err
		}
		if err = os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			return err
		}
		if err = os.WriteFile(p, b, 0644); err != nil {
			return err
		}
	}
	readme, err := workspace.ConstrainPath(root, path.Join(shippedDir, "README.txt"))
	if err != nil {
		return err
	}
	const note = "Reference copies of CE-SE's shipped playbooks, one folder per version. They are inactive: never follow them as current rules. During an update the agent compares the previous shipped copy, the teacher's active file and the new shipped copy; adopted.txt names the version last adopted.\n"
	return os.WriteFile(readme, []byte(note), 0644)
}

// pastSeeds fingerprints every committed version of each Engine-managed rules
// file up to the release that introduced shipped/ (generated once by
// `go run internal/playbooks/pastseeds_gen.go`). It lets installs that predate
// shipped/ recognize unedited older copies; later versions are covered by the
// shipped/ baseline, so the list needs no regular regeneration.
//
//go:embed past_seeds.txt
var pastSeeds string

func pastSeed(rel string, have []byte) bool {
	line := fmt.Sprintf("%s %x", rel, sha256.Sum256(bytes.ReplaceAll(have, []byte("\r\n"), []byte("\n"))))
	for _, known := range strings.Split(strings.ReplaceAll(pastSeeds, "\r\n", "\n"), "\n") {
		if known == line {
			return true
		}
	}
	return false
}
