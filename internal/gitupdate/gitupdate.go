// Package gitupdate stages and applies updates for a git install (a clone of
// the userinstall branch): playbooks arrive by fast-forward pull and the Engine
// by get-engine.cmd's checksum-verified download. Judgment stays with the agent:
// it decides when to ask the teacher, and converts edited shipped files into
// Teacher_Override.md before an apply archives them.
package gitupdate

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"canvasexpertse/internal/workspace"
)

const (
	remoteBranch = "origin/userinstall"
	checkFile    = "last-update-check.txt"
)

// Report is what each operation tells the agent.
type Report struct {
	State     string   `json:"state"` // current, pending, staged, skipped, applied, repaired, held
	Current   string   `json:"current,omitempty"`
	Available string   `json:"available,omitempty"`
	Staged    bool     `json:"staged,omitempty"`
	Held      string   `json:"held,omitempty"`
	Edited    []string `json:"edited,omitempty"`
	Archived  []string `json:"archived,omitempty"`
	Message   string   `json:"message,omitempty"`
}

// Updater runs against one UserInstall. Git, GetEngine and OpenPages are
// replaceable for tests.
type Updater struct {
	Root      string
	Now       func() time.Time
	Git       func(root string, args ...string) (string, error)
	GetEngine func(root string, args ...string) error
	OpenPages func(root string) []string
}

func New(root string, openPages func(string) []string) Updater {
	return Updater{Root: root, Now: time.Now, Git: runGit, GetEngine: runGetEngine, OpenPages: openPages}
}

var errNotGit = errors.New("this UserInstall is not a git install; follow the ZIP fallback in playbooks/install.md")

// Status reports whether a fetched update is waiting, without contacting GitHub.
func (u Updater) Status() (Report, error) {
	if workspace.InstallKind(u.Root) != "git" {
		return Report{}, errNotGit
	}
	r := Report{Current: u.identity()}
	pending, err := u.pending()
	if err != nil {
		return r, err
	}
	if !pending {
		r.State = "current"
		return r, nil
	}
	r.State = "pending"
	r.Available, r.Staged = u.available()
	return r, nil
}

// Stage fetches the userinstall branch and downloads and verifies a newer
// Engine into engine/staged without changing anything live. With ifDue it
// runs at most once a day; the date is recorded first, so a failed check is
// retried the next day rather than on every session start.
func (u Updater) Stage(ifDue bool) (Report, error) {
	if workspace.InstallKind(u.Root) != "git" {
		return Report{}, errNotGit
	}
	today := u.Now().Format("2006-01-02")
	mark := filepath.Join(u.Root, "engine", checkFile)
	if ifDue {
		if b, err := os.ReadFile(mark); err == nil && strings.TrimSpace(string(b)) == today {
			r, err := u.Status()
			if err == nil && r.State == "current" {
				r.State = "skipped"
				r.Message = "already checked today"
			}
			return r, err
		}
	}
	_ = os.WriteFile(mark, []byte(today+"\n"), 0644)
	if _, err := u.Git(u.Root, "fetch", "--quiet", "origin", "userinstall"); err != nil {
		return Report{}, fmt.Errorf("could not reach the update source: %w", err)
	}
	r, err := u.Status()
	if err != nil || r.State != "pending" {
		return r, err
	}
	manifest, err := u.Git(u.Root, "show", remoteBranch+":engine/manifest.txt")
	if err != nil {
		return r, err
	}
	staged := filepath.Join(u.Root, "engine", "staged")
	if err = os.MkdirAll(staged, 0755); err != nil {
		return r, err
	}
	if err = os.WriteFile(filepath.Join(staged, "manifest.txt"), []byte(manifest), 0644); err != nil {
		return r, err
	}
	if err = u.GetEngine(u.Root, "--stage", "--manifest", filepath.Join("engine", "staged", "manifest.txt")); err != nil {
		return r, fmt.Errorf("the new Engine could not be staged: %w", err)
	}
	r.Available, r.Staged = u.available()
	r.State = "staged"
	return r, nil
}

// Apply installs a fetched update: it holds while a private page is open or a
// shipped file has local edits (unless archiveEdits, after the agent has moved
// those edits into Teacher_Override.md), fast-forwards to the fetched branch,
// installs the Engine (from the staged copy when it verifies) and checks the
// installed Engine against the new manifest.
func (u Updater) Apply(archiveEdits bool) (Report, error) {
	if workspace.InstallKind(u.Root) != "git" {
		return Report{}, errNotGit
	}
	r := Report{Current: u.identity()}
	if u.OpenPages != nil {
		if open := u.OpenPages(u.Root); len(open) > 0 {
			r.State, r.Held = "held", "a private "+strings.Join(open, ", ")+" page is open; apply after the teacher closes it"
			return r, nil
		}
	}
	pending, err := u.pending()
	if err != nil {
		return r, err
	}
	if !pending {
		if u.engineMatches() {
			r.State = "current"
			return r, nil
		}
		if err = u.GetEngine(u.Root); err != nil {
			return r, fmt.Errorf("the Engine does not match this release and could not be reinstalled: %w", err)
		}
		r.State, r.Message = "repaired", "the Engine was reinstalled to match this release"
		return r, nil
	}
	r.Available, _ = u.available()
	edited, err := u.edited()
	if err != nil {
		return r, err
	}
	if len(edited) > 0 {
		if !archiveEdits {
			r.State, r.Edited = "held", edited
			r.Held = "shipped files have local edits; move the teacher's choices into Teacher_Override.md, then apply with archive-edits"
			return r, nil
		}
		if r.Archived, err = u.archive(edited); err != nil {
			return r, err
		}
		if _, err = u.Git(u.Root, append([]string{"checkout", "--"}, edited...)...); err != nil {
			return r, err
		}
	}
	if _, err = u.Git(u.Root, "merge", "--ff-only", "--quiet", remoteBranch); err != nil {
		r.State, r.Held = "held", "the update could not be applied as a fast-forward; nothing was forced. Ask for help before changing this folder."
		return r, nil
	}
	if err = u.GetEngine(u.Root); err != nil {
		return r, fmt.Errorf("playbooks were updated but the Engine could not be installed; run .\\get-engine.cmd: %w", err)
	}
	if !u.engineMatches() {
		return r, errors.New("the installed Engine does not match the new manifest; run .\\get-engine.cmd")
	}
	r.State, r.Current = "applied", u.identity()
	r.Message = "restart the apps using CanvasExpert at a good moment so they load the new Engine"
	return r, nil
}

// pending is true when the fetched branch has commits this folder lacks.
func (u Updater) pending() (bool, error) {
	if remote, err := u.Git(u.Root, "rev-parse", "--verify", "--quiet", remoteBranch); err != nil || strings.TrimSpace(remote) == "" {
		return false, nil // nothing fetched yet
	}
	count, err := u.Git(u.Root, "rev-list", "--count", "HEAD.."+remoteBranch)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(count) != "0", nil
}

func (u Updater) identity() string {
	m, ok, err := workspace.ReadEngineManifest(u.Root)
	if err != nil || !ok {
		return ""
	}
	return m.Identity
}

// available returns the fetched release's identity and whether its Engine is
// already staged and verified.
func (u Updater) available() (string, bool) {
	text, err := u.Git(u.Root, "show", remoteBranch+":engine/manifest.txt")
	if err != nil {
		return "", false
	}
	identity, sums := parseManifest(text)
	if len(sums) == 0 {
		return identity, false
	}
	for name, want := range sums {
		if fileSHA256(filepath.Join(u.Root, "engine", "staged", name)) != want {
			return identity, false
		}
	}
	return identity, true
}

func (u Updater) engineMatches() bool {
	m, ok, err := workspace.ReadEngineManifest(u.Root)
	if err != nil || !ok || m.SHA256["cse.exe"] == "" {
		return false
	}
	return fileSHA256(filepath.Join(u.Root, "engine", "cse.exe")) == m.SHA256["cse.exe"]
}

func (u Updater) edited() ([]string, error) {
	out, err := u.Git(u.Root, "diff", "--name-only", "HEAD")
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, line := range strings.Split(out, "\n") {
		if p := strings.TrimSpace(line); p != "" {
			paths = append(paths, p)
		}
	}
	return paths, nil
}

func (u Updater) archive(paths []string) ([]string, error) {
	dir := filepath.Join("playbook-old", u.Now().Format("2006-01-02-150405"))
	var archived []string
	for _, rel := range paths {
		src, err := workspace.ConstrainPath(u.Root, filepath.FromSlash(rel))
		if err != nil {
			return archived, err
		}
		b, err := os.ReadFile(src)
		if os.IsNotExist(err) {
			continue // a deleted shipped file comes back with the checkout
		}
		if err != nil {
			return archived, err
		}
		dst := filepath.Join(u.Root, dir, filepath.FromSlash(rel))
		if err = os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return archived, err
		}
		if err = os.WriteFile(dst, b, 0644); err != nil {
			return archived, err
		}
		archived = append(archived, filepath.ToSlash(filepath.Join(dir, filepath.FromSlash(rel))))
	}
	return archived, nil
}

func parseManifest(text string) (string, map[string]string) {
	identity, sums := "", map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok || strings.HasPrefix(key, "#") {
			continue
		}
		if key == "identity" {
			identity = value
		} else if strings.HasPrefix(key, "sha256.") {
			sums[strings.TrimPrefix(key, "sha256.")] = strings.ToLower(value)
		}
	}
	return identity, sums
}

func fileSHA256(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}

func runGit(root string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return stdout.String(), fmt.Errorf("git %s: %s", args[0], msg)
		}
		return stdout.String(), fmt.Errorf("git %s: %w", args[0], err)
	}
	return stdout.String(), nil
}

func runGetEngine(root string, args ...string) error {
	cmd := exec.Command("cmd.exe", append([]string{"/d", "/c", `.\get-engine.cmd`}, args...)...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		if code := cmd.ProcessState; code != nil {
			return fmt.Errorf("get-engine.cmd exit %d: %s", code.ExitCode(), strings.TrimSpace(string(out)))
		}
		return err
	}
	return nil
}
