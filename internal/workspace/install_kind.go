package workspace

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// EngineManifest is engine/manifest.txt: what Engine the shipped playbooks were
// released with, and the checksums get-engine.cmd verifies. It is a plain
// key=value file written by scripts/package-userinstall.ps1.
type EngineManifest struct {
	Version  string
	Identity string
	BaseURL  string
	SHA256   map[string]string
}

// ReadEngineManifest reads engine/manifest.txt. ok is false when the file does
// not exist, which is the case for Dev checkouts and installs that predate it.
func ReadEngineManifest(root string) (EngineManifest, bool, error) {
	f, err := os.Open(filepath.Join(root, "engine", "manifest.txt"))
	if os.IsNotExist(err) {
		return EngineManifest{}, false, nil
	}
	if err != nil {
		return EngineManifest{}, false, err
	}
	defer f.Close()
	m := EngineManifest{SHA256: map[string]string{}}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		key, value, found := strings.Cut(line, "=")
		if line == "" || strings.HasPrefix(line, "#") || !found {
			continue
		}
		switch {
		case key == "version":
			m.Version = value
		case key == "identity":
			m.Identity = value
		case key == "base_url":
			m.BaseURL = value
		case strings.HasPrefix(key, "sha256."):
			m.SHA256[strings.TrimPrefix(key, "sha256.")] = strings.ToLower(value)
		}
	}
	return m, true, scanner.Err()
}

// InstallKind says how this UserInstall receives updates: "git" for a clone of
// the userinstall branch, "zip" for an extracted ZIP, "" for anything else (a Dev
// checkout, or a folder that predates the manifest).
func InstallKind(root string) string {
	if _, ok, err := ReadEngineManifest(root); err != nil || !ok {
		return ""
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); err == nil {
		return "git"
	}
	return "zip"
}

// IsGitInstall reports whether git delivers this UserInstall's playbooks.
func IsGitInstall(root string) bool { return InstallKind(root) == "git" }

// gitOwnedRootFiles are the tracked root files of the userinstall branch. In a git
// install they come from git on every PC, never from UserSync.
var gitOwnedRootFiles = map[string]bool{
	"agents.md": true, "claude.md": true, "teacher.md": true,
	"teaching-assistant.md": true, "readme.md": true,
}

// IsSharedRulePathIn is IsSharedRulePath for a particular UserInstall. In a git
// install, playbooks and the shipped root files belong to git, so only context/,
// Teacher_Override.md and the teacher's own root guides follow UserSync.
func IsSharedRulePathIn(root, relative string) bool {
	if !IsSharedRulePath(relative) {
		return false
	}
	if !IsGitInstall(root) {
		return true
	}
	p := strings.ToLower(strings.Trim(strings.ReplaceAll(relative, "\\", "/"), "/"))
	return !strings.HasPrefix(p, "playbooks/") && !gitOwnedRootFiles[p]
}

// ArtifactIdentity is the manifest `identity` for a build: the version for a
// release, otherwise the version with the revision and build time, matching
// scripts/package-userinstall.ps1.
func ArtifactIdentity(version, buildKind, revision, builtAt string) string {
	if buildKind == "release" {
		return version
	}
	stamp := builtAt
	if t, err := time.Parse(time.RFC3339, builtAt); err == nil {
		stamp = t.UTC().Format("20060102T150405Z")
	}
	return version + "-dev." + strings.ReplaceAll(revision, "+", ".") + "." + stamp
}
