package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testManifest = "# comment\nversion=1.2.0\nidentity=1.2.0\nbase_url=https://example.test/dl\nsha256.cse.exe=ABCDEF\nsha256.other.mcpb=012345\n"

func gitInstallRoot(t *testing.T, git bool) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"context/courses.md":    "---\nschema: 1\n---\n",
		"context/grading.md":    "teacher grading",
		"engine/manifest.txt":   testManifest,
		"playbooks/score.md":    "shipped playbook",
		"AGENTS.md":             "shipped agents",
		"CLAUDE.md":             "shipped claude",
		"Teacher_Override.md":   "teacher override",
		"Creation Taste.txt":    "teacher guide",
		"README.md":             "product readme",
	}
	for name, body := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if git {
		if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestInstallKindAndManifest(t *testing.T) {
	if got := InstallKind(t.TempDir()); got != "" {
		t.Fatalf("empty folder kind %q", got)
	}
	if got := InstallKind(gitInstallRoot(t, false)); got != "zip" {
		t.Fatalf("zip kind %q", got)
	}
	root := gitInstallRoot(t, true)
	if got := InstallKind(root); got != "git" || !IsGitInstall(root) {
		t.Fatalf("git kind %q", got)
	}
	m, ok, err := ReadEngineManifest(root)
	if err != nil || !ok || m.Version != "1.2.0" || m.Identity != "1.2.0" || m.BaseURL != "https://example.test/dl" || m.SHA256["cse.exe"] != "abcdef" || m.SHA256["other.mcpb"] != "012345" {
		t.Fatalf("manifest %+v %v %v", m, ok, err)
	}
	// A Dev checkout has .git but no manifest, so it is not an install.
	dev := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dev, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if InstallKind(dev) != "" {
		t.Fatal("a checkout without a manifest was treated as an install")
	}
}

func TestArtifactIdentityMatchesPackager(t *testing.T) {
	if got := ArtifactIdentity("1.2.0", "release", "abc", "2026-10-21T17:51:18Z"); got != "1.2.0" {
		t.Fatalf("release identity %q", got)
	}
	if got, want := ArtifactIdentity("1.1.0-beta.1", "development", "236a20fbcc55+dirty", "2026-10-21T17:51:18Z"), "1.1.0-beta.1-dev.236a20fbcc55.dirty.20261021T175118Z"; got != want {
		t.Fatalf("dev identity %q want %q", got, want)
	}
}

func TestSharedRulePathsInGitInstall(t *testing.T) {
	git, zip := gitInstallRoot(t, true), gitInstallRoot(t, false)
	for _, rel := range []string{"playbooks/score.md", "AGENTS.md", "CLAUDE.md", "TEACHER.md"} {
		if !IsSharedRulePathIn(zip, rel) {
			t.Errorf("ZIP install: %s should follow UserSync", rel)
		}
		if IsSharedRulePathIn(git, rel) {
			t.Errorf("git install: %s must come from git, not UserSync", rel)
		}
	}
	for _, rel := range []string{"context/grading.md", "Teacher_Override.md", "Creation Taste.txt"} {
		if !IsSharedRulePathIn(git, rel) {
			t.Errorf("git install: %s should still follow UserSync", rel)
		}
	}
}

func TestSyncInGitInstallLeavesTrackedFilesAlone(t *testing.T) {
	for _, tc := range []struct {
		name string
		git  bool
	}{{"git", true}, {"zip", false}} {
		t.Run(tc.name, func(t *testing.T) {
			root := gitInstallRoot(t, tc.git)
			sync := t.TempDir()
			if _, err := SetRulesLocation(root, sync, nil, ""); err != nil {
				t.Fatal(err)
			}
			agents, _ := os.ReadFile(filepath.Join(root, "AGENTS.md"))
			_, playbookInSync := os.Stat(filepath.Join(sync, "playbooks", "score.md"))
			_, overrideInSync := os.Stat(filepath.Join(sync, "Teacher_Override.md"))
			_, contextInSync := os.Stat(filepath.Join(sync, "context", "grading.md"))
			if overrideInSync != nil || contextInSync != nil {
				t.Fatalf("teacher files must reach UserSync: override %v context %v", overrideInSync, contextInSync)
			}
			playbookPath, err := ResolveTeacherPath(root, "playbooks/score.md")
			if err != nil {
				t.Fatal(err)
			}
			contextPath, err := ResolveTeacherPath(root, "context/grading.md")
			if err != nil || !strings.HasPrefix(contextPath, sync) {
				t.Fatalf("context should resolve into UserSync: %q %v", contextPath, err)
			}
			if tc.git {
				if string(agents) != "shipped agents" || playbookInSync == nil || !strings.HasPrefix(playbookPath, root) {
					t.Fatalf("git install: AGENTS.md %q, playbook in sync %v, playbook path %q", agents, playbookInSync == nil, playbookPath)
				}
				if err := RestoreSyncBootstrap(root); err != nil {
					t.Fatal(err)
				}
				if again, _ := os.ReadFile(filepath.Join(root, "AGENTS.md")); string(again) != "shipped agents" {
					t.Fatalf("RestoreSyncBootstrap rewrote a git-owned file: %q", again)
				}
			} else {
				if !strings.HasPrefix(string(agents), "# CE-SE local agent bootstrap") || playbookInSync != nil || !strings.HasPrefix(playbookPath, sync) {
					t.Fatalf("ZIP install keeps today's behavior: AGENTS.md %q, playbook in sync %v, playbook path %q", agents, playbookInSync == nil, playbookPath)
				}
			}
		})
	}
}
