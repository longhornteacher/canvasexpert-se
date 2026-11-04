package gitupdate

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=Test", "-c", "user.email=test@example.test", "-c", "core.autocrlf=false"}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0644); err != nil {
		t.Fatal(err)
	}
}

func sum(text string) string {
	h := sha256.Sum256([]byte(text))
	return hex.EncodeToString(h[:])
}

// release commits one userinstall release to the origin repository.
func release(t *testing.T, origin, version, playbook string) {
	t.Helper()
	write(t, filepath.Join(origin, "engine", "manifest.txt"), "version="+version+"\nidentity="+version+"\nbase_url=file:///unused\nsha256.cse.exe="+sum("engine-"+version)+"\n")
	write(t, filepath.Join(origin, "playbooks", "score.md"), playbook)
	write(t, filepath.Join(origin, ".gitignore"), "/engine/*\n!/engine/manifest.txt\n/context/\n/.cse/\n/playbook-old/\n")
	git(t, origin, "add", "-A")
	git(t, origin, "commit", "-q", "-m", version)
}

func TestStageThenApplyHoldsForOpenPagesAndEditsThenFastForwards(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	base := t.TempDir()
	origin, root := filepath.Join(base, "origin"), filepath.Join(base, "ui")
	git(t, base, "init", "-q", "-b", "userinstall", origin)
	release(t, origin, "1.0", "score v1\n")
	git(t, base, "clone", "-q", "--branch", "userinstall", origin, root)
	write(t, filepath.Join(root, "engine", "cse.exe"), "engine-1.0")

	var engineCalls [][]string
	open := []string{}
	u := New(root, func(string) []string { return open })
	u.Now = func() time.Time { return time.Date(2026, 10, 22, 8, 0, 0, 0, time.UTC) }
	u.GetEngine = func(root string, args ...string) error {
		engineCalls = append(engineCalls, args)
		staged := filepath.Join(root, "engine", "staged", "cse.exe")
		if len(args) > 0 && args[0] == "--stage" {
			write(t, staged, "engine-1.1")
			return nil
		}
		b, err := os.ReadFile(staged)
		if err != nil {
			return err
		}
		write(t, filepath.Join(root, "engine", "cse.exe"), string(b))
		return os.RemoveAll(filepath.Join(root, "engine", "staged"))
	}

	if r, err := u.Status(); err != nil || r.State != "current" || r.Current != "1.0" {
		t.Fatalf("fresh clone: %+v %v", r, err)
	}
	if r, err := u.Stage(true); err != nil || r.State != "current" {
		t.Fatalf("stage with nothing new: %+v %v", r, err)
	}
	if r, err := u.Stage(true); err != nil || r.State != "skipped" {
		t.Fatalf("second check the same day should skip: %+v %v", r, err)
	}

	release(t, origin, "1.1", "score v2\n")
	r, err := u.Stage(false)
	if err != nil || r.State != "staged" || !r.Staged || r.Available != "1.1" || r.Current != "1.0" {
		t.Fatalf("stage: %+v %v", r, err)
	}
	if b, _ := os.ReadFile(filepath.Join(root, "playbooks", "score.md")); strings.ReplaceAll(string(b), "\r\n", "\n") != "score v1\n" {
		t.Fatal("staging changed a live playbook")
	}
	if b, _ := os.ReadFile(filepath.Join(root, "engine", "cse.exe")); string(b) != "engine-1.0" {
		t.Fatal("staging changed the live Engine")
	}

	open = []string{"review"}
	if r, err = u.Apply(false); err != nil || r.State != "held" || !strings.Contains(r.Held, "review") {
		t.Fatalf("open review page must hold: %+v %v", r, err)
	}
	open = nil

	write(t, filepath.Join(root, "playbooks", "score.md"), "score v1 with a teacher edit\n")
	if r, err = u.Apply(false); err != nil || r.State != "held" || len(r.Edited) != 1 || r.Edited[0] != "playbooks/score.md" {
		t.Fatalf("edited shipped file must hold: %+v %v", r, err)
	}
	r, err = u.Apply(true)
	if err != nil || r.State != "applied" || r.Current != "1.1" || len(r.Archived) != 1 {
		t.Fatalf("apply: %+v %v", r, err)
	}
	if b, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(r.Archived[0]))); string(b) != "score v1 with a teacher edit\n" {
		t.Fatalf("teacher edit not archived: %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(root, "playbooks", "score.md")); strings.ReplaceAll(string(b), "\r\n", "\n") != "score v2\n" {
		t.Fatalf("playbook not updated: %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(root, "engine", "cse.exe")); string(b) != "engine-1.1" {
		t.Fatal("Engine not installed")
	}
	if r, err = u.Apply(false); err != nil || r.State != "current" {
		t.Fatalf("after apply: %+v %v", r, err)
	}
	if last := engineCalls[len(engineCalls)-1]; len(last) != 2 || last[0] != "--manifest" {
		t.Fatalf("apply should install from the fetched manifest before the merge: %v", last)
	}
}

func TestNotGitInstallIsRefused(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "engine", "manifest.txt"), "identity=1.0\n")
	if _, err := New(root, nil).Status(); err == nil || !strings.Contains(err.Error(), "not a git install") {
		t.Fatalf("zip install: %v", err)
	}
}

// applyFixture is a clone one release behind origin, with the new Engine staged.
func applyFixture(t *testing.T) (u Updater, root, origin string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	base := t.TempDir()
	origin, root = filepath.Join(base, "origin"), filepath.Join(base, "ui")
	git(t, base, "init", "-q", "-b", "userinstall", origin)
	release(t, origin, "1.0", "score v1\n")
	git(t, base, "clone", "-q", "--branch", "userinstall", origin, root)
	write(t, filepath.Join(root, "engine", "cse.exe"), "engine-1.0")
	release(t, origin, "1.1", "score v2\n")
	u = New(root, func(string) []string { return nil })
	u.Now = func() time.Time { return time.Date(2026, 11, 4, 8, 0, 0, 0, time.UTC) }
	u.GetEngine = func(root string, args ...string) error {
		staged := filepath.Join(root, "engine", "staged", "cse.exe")
		if len(args) > 0 && args[0] == "--stage" {
			write(t, staged, "engine-1.1")
			return nil
		}
		b, err := os.ReadFile(staged)
		if err != nil {
			return err
		}
		write(t, filepath.Join(root, "engine", "cse.exe"), string(b))
		return os.RemoveAll(filepath.Join(root, "engine", "staged"))
	}
	if _, err := u.Stage(false); err != nil {
		t.Fatal(err)
	}
	return u, root, origin
}

func playbook(root string) string {
	b, _ := os.ReadFile(filepath.Join(root, "playbooks", "score.md"))
	return strings.ReplaceAll(string(b), "\r\n", "\n")
}

func TestApplyEngineSwapFailureChangesNothing(t *testing.T) {
	u, root, _ := applyFixture(t)
	write(t, filepath.Join(root, "playbooks", "score.md"), "score v1 edited\n")
	u.GetEngine = func(string, ...string) error { return errors.New("get-engine.cmd exit 6: close the apps") }
	r, err := u.Apply(true)
	if err == nil || !strings.Contains(err.Error(), "nothing was changed") {
		t.Fatalf("want a clear swap failure, got %+v %v", r, err)
	}
	if playbook(root) != "score v1 edited\n" || len(r.Archived) != 0 {
		t.Fatalf("a failed Engine swap must not revert or archive the teacher's edit: %q %v", playbook(root), r.Archived)
	}
	if n, _ := u.Git(root, "rev-list", "--count", "HEAD..origin/userinstall"); strings.TrimSpace(n) != "1" {
		t.Fatalf("the playbooks must not have been merged: %q", n)
	}
}

func TestApplyHoldsBeforeInstallingWhenNotAFastForward(t *testing.T) {
	u, root, _ := applyFixture(t)
	write(t, filepath.Join(root, "other.txt"), "local commit\n")
	git(t, root, "add", "-A")
	git(t, root, "commit", "-q", "-m", "diverged")
	called := false
	u.GetEngine = func(string, ...string) error { called = true; return nil }
	r, err := u.Apply(false)
	if err != nil || r.State != "held" || !strings.Contains(r.Held, "fast-forward") || called {
		t.Fatalf("a diverged folder must hold before the Engine is touched: %+v %v called=%v", r, err, called)
	}
}

func TestApplyMergeFailureAfterSwapSaysTheEngineIsNew(t *testing.T) {
	u, _, _ := applyFixture(t)
	realGit := u.Git
	u.Git = func(root string, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "merge" {
			return "", errors.New("git merge: simulated failure")
		}
		return realGit(root, args...)
	}
	r, err := u.Apply(false)
	if err != nil || r.State != "held" || !strings.Contains(r.Held, "new Engine is installed") {
		t.Fatalf("want a held report naming the new Engine: %+v %v", r, err)
	}
}

func TestStaleExtensionIsRepairedNotCurrent(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	base := t.TempDir()
	origin, root := filepath.Join(base, "origin"), filepath.Join(base, "ui")
	git(t, base, "init", "-q", "-b", "userinstall", origin)
	write(t, filepath.Join(origin, "engine", "manifest.txt"), "version=1.0\nidentity=1.0\nbase_url=file:///unused\nsha256.cse.exe="+sum("engine-1.0")+"\nsha256.ext.mcpb="+sum("ext-1.0")+"\n")
	write(t, filepath.Join(origin, "playbooks", "score.md"), "score\n")
	write(t, filepath.Join(origin, ".gitignore"), "/engine/*\n!/engine/manifest.txt\n")
	git(t, origin, "add", "-A")
	git(t, origin, "commit", "-q", "-m", "1.0")
	git(t, base, "clone", "-q", "--branch", "userinstall", origin, root)
	write(t, filepath.Join(root, "engine", "cse.exe"), "engine-1.0")
	write(t, filepath.Join(root, "engine", "ext.mcpb"), "ext-stale")
	var calls int
	u := New(root, func(string) []string { return nil })
	u.GetEngine = func(root string, args ...string) error {
		calls++
		write(t, filepath.Join(root, "engine", "ext.mcpb"), "ext-1.0")
		return nil
	}
	r, err := u.Apply(false)
	if err != nil || r.State != "repaired" || calls != 1 {
		t.Fatalf("a stale extension must be repaired, not reported current: %+v %v calls=%d", r, err, calls)
	}
	if r, err = u.Apply(false); err != nil || r.State != "current" {
		t.Fatalf("after the repair: %+v %v", r, err)
	}
}
