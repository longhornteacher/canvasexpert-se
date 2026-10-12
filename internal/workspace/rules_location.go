package workspace

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RulesLocation describes the active teacher-rules authority. InstallRoot is
// always the operational workspace; RulesRoot is InstallRoot in local mode.
type RulesLocation struct {
	InstallRoot string `json:"install_root"`
	RulesRoot   string `json:"rules_root"`
	Mode        string `json:"mode"`
}

type rulesBinding struct {
	SyncRoot string `json:"sync_root,omitempty"`
}

func rulesBindingPath(root string) string { return filepath.Join(root, ".cse", "rules-location.json") }

// IsSharedRulePath reports whether a logical Install-relative path belongs to
// the teacher rules authority. Product README.md and local course/student state
// intentionally remain in Install.
func IsSharedRulePath(relative string) bool {
	p := strings.ToLower(strings.Trim(strings.ReplaceAll(relative, "\\", "/"), "/"))
	if p == "" || strings.HasPrefix(p, "../") {
		return false
	}
	for _, part := range strings.Split(p, "/") {
		if strings.HasPrefix(part, ".") || part == "engine" {
			return false
		}
	}
	if isExecutableArtifact(p) {
		return false
	}
	if strings.HasPrefix(p, "playbooks/") {
		return true
	}
	if strings.HasPrefix(p, "context/") {
		name := strings.TrimPrefix(p, "context/")
		if name == "courses.md" || name == "students.md" {
			return false
		}
		return true
	}
	if strings.Contains(p, "/") {
		return false
	}
	if p == "readme.md" {
		return false
	}
	ext := strings.ToLower(filepath.Ext(p))
	return ext == ".md" || ext == ".txt"
}

func isExecutableArtifact(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".exe", ".cmd", ".bat", ".com", ".msi", ".msp", ".dll", ".sys", ".scr", ".ps1", ".psm1", ".sh", ".bash", ".zsh", ".fish", ".vbs", ".js", ".jse", ".wsf", ".wsh", ".jar", ".class", ".so", ".dylib", ".app":
		return true
	default:
		return false
	}
}

// GetRulesLocation reads the per-Install binding on every call. A configured
// but unavailable Sync directory is an error; callers must not fall back.
func GetRulesLocation(root string) (RulesLocation, error) {
	install, err := filepath.Abs(root)
	if err != nil {
		return RulesLocation{}, err
	}
	b, err := os.ReadFile(rulesBindingPath(install))
	if os.IsNotExist(err) {
		return RulesLocation{InstallRoot: install, RulesRoot: install, Mode: "local"}, nil
	}
	if err != nil {
		return RulesLocation{}, fmt.Errorf("read rules location: %w", err)
	}
	var binding rulesBinding
	if err := json.Unmarshal(b, &binding); err != nil {
		return RulesLocation{}, fmt.Errorf("read rules location: %w", err)
	}
	if binding.SyncRoot == "" {
		return RulesLocation{InstallRoot: install, RulesRoot: install, Mode: "local"}, nil
	}
	if !filepath.IsAbs(binding.SyncRoot) {
		return RulesLocation{InstallRoot: install, RulesRoot: binding.SyncRoot, Mode: "sync"}, fmt.Errorf("configured Sync rules directory %q is not absolute", binding.SyncRoot)
	}
	syncRoot, err := filepath.Abs(binding.SyncRoot)
	if err != nil {
		return RulesLocation{}, fmt.Errorf("resolve Sync rules directory: %w", err)
	}
	loc := RulesLocation{InstallRoot: install, RulesRoot: syncRoot, Mode: "sync"}
	info, err := os.Stat(syncRoot)
	if err != nil {
		return loc, fmt.Errorf("configured Sync rules directory %s is unavailable: %w", syncRoot, err)
	}
	if !info.IsDir() {
		return loc, fmt.Errorf("configured Sync rules path %s is not a directory", syncRoot)
	}
	return loc, nil
}

// ResolveTeacherPath maps a logical workspace path to its active authority,
// while retaining the ordinary path confinement checks at that authority.
func ResolveTeacherPath(root, relative string) (string, error) {
	install, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	base := install
	if !IsSharedRulePath(relative) {
		return ConstrainPath(base, relative)
	}
	loc, err := GetRulesLocation(install)
	if err != nil {
		return "", err
	}
	base = loc.RulesRoot
	p, err := ConstrainPath(base, relative)
	if err != nil {
		return "", err
	}
	if err := rejectLinkedPrefixes(base, relative); err != nil {
		return "", err
	}
	return p, nil
}

// SetRulesLocation explicitly binds an Install to Sync, or returns it to local
// rules. It copies only classified teacher files, archives replaced local
// content, and writes a small local-agent bootstrap while Sync is active.
func SetRulesLocation(root, syncRoot string, template fs.FS, base string) (RulesLocation, error) {
	install, err := filepath.Abs(root)
	if err != nil {
		return RulesLocation{}, err
	}
	if !isRoot(install) {
		return RulesLocation{}, fmt.Errorf("Install root %s lacks context/courses.md", install)
	}
	if strings.TrimSpace(syncRoot) != "" && !filepath.IsAbs(strings.TrimSpace(syncRoot)) {
		return RulesLocation{}, fmt.Errorf("Sync rules directory must be an absolute path")
	}
	current, err := GetRulesLocation(install)
	currentErr := err
	if strings.TrimSpace(syncRoot) == "" {
		if currentErr != nil {
			return current, currentErr
		}
		if current.Mode == "sync" {
			if template != nil {
				if _, err := ScaffoldRules(current.RulesRoot, template, base); err != nil {
					return current, err
				}
			}
			if err := copyShared(current.RulesRoot, install, true); err != nil {
				return RulesLocation{}, err
			}
			if err := os.Remove(rulesBindingPath(install)); err != nil && !os.IsNotExist(err) {
				return RulesLocation{}, err
			}
		}
		return GetRulesLocation(install)
	}
	syncPath, err := filepath.Abs(syncRoot)
	if err != nil {
		return RulesLocation{}, err
	}
	info, err := os.Stat(syncPath)
	if err != nil {
		return RulesLocation{}, fmt.Errorf("Sync rules directory %s must already exist: %w", syncPath, err)
	}
	if !info.IsDir() {
		return RulesLocation{}, fmt.Errorf("Sync rules path %s is not a directory", syncPath)
	}
	resolvedInstall, err := resolveExistingPath(install, 0)
	if err != nil {
		return RulesLocation{}, fmt.Errorf("resolve Install root: %w", err)
	}
	resolvedSync, err := resolveExistingPath(syncPath, 0)
	if err != nil {
		return RulesLocation{}, fmt.Errorf("resolve Sync rules root: %w", err)
	}
	if pathOverlaps(resolvedSync, resolvedInstall) {
		return RulesLocation{}, fmt.Errorf("Sync rules directory must be separate from Install")
	}
	if current.Mode == "sync" && samePath(current.RulesRoot, syncPath) && currentErr == nil {
		return current, nil
	}
	// An explicit new destination is the recovery path for a stale binding.
	// It does not read or replace from the unavailable old destination.
	if currentErr != nil {
		if empty, e := isEmptyDir(syncPath); e != nil {
			return RulesLocation{}, e
		} else if empty {
			return current, fmt.Errorf("configured Sync rules directory is unavailable; refusing to initialize an empty Sync directory from inactive Install copies")
		}
	}
	empty, err := isEmptyDir(syncPath)
	if err != nil {
		return RulesLocation{}, err
	}
	if empty {
		source := install
		if current.Mode == "sync" {
			source = current.RulesRoot
		}
		if err := copyShared(source, syncPath, false); err != nil {
			return RulesLocation{}, err
		}
		if template != nil {
			if _, err := ScaffoldRules(syncPath, template, base); err != nil {
				return RulesLocation{}, err
			}
		}
	} else if template != nil {
		if _, err := ScaffoldRules(syncPath, template, base); err != nil {
			return RulesLocation{}, err
		}
	}
	if err := archiveShared(install); err != nil {
		return RulesLocation{}, err
	}
	if err := writeBootstrap(install, "AGENTS.md", syncPath); err != nil {
		return RulesLocation{}, err
	}
	if err := writeBootstrap(install, "CLAUDE.md", syncPath); err != nil {
		return RulesLocation{}, err
	}
	if err := writeRulesBinding(install, syncPath); err != nil {
		return RulesLocation{}, err
	}
	return GetRulesLocation(install)
}

func samePath(a, b string) bool {
	x, _ := filepath.Abs(a)
	y, _ := filepath.Abs(b)
	return strings.EqualFold(filepath.Clean(x), filepath.Clean(y))
}

func pathOverlaps(a, b string) bool { return inside(a, b) || inside(b, a) }
func isEmptyDir(p string) (bool, error) {
	entries, e := os.ReadDir(p)
	if e != nil {
		return false, e
	}
	return len(entries) == 0, nil
}

func sharedFiles(root string) ([]string, error) {
	var files []string
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if entry.IsDir() && (strings.EqualFold(name, "playbooks") || strings.EqualFold(name, "context")) {
			folder := filepath.Join(root, name)
			err := filepath.WalkDir(folder, func(p string, d fs.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if p == folder {
					return nil
				}
				base := filepath.Base(p)
				if d.IsDir() {
					if strings.HasPrefix(base, ".") || strings.EqualFold(base, "engine") {
						return filepath.SkipDir
					}
					return nil
				}
				if d.Type()&os.ModeSymlink != 0 || strings.HasPrefix(base, ".") {
					return nil
				}
				rel, e := filepath.Rel(root, p)
				if e != nil {
					return e
				}
				rel = filepath.ToSlash(rel)
				if IsSharedRulePath(rel) {
					if e := rejectLinkedPrefixes(root, rel); e != nil {
						return e
					}
					files = append(files, rel)
				}
				return nil
			})
			if err != nil {
				return nil, err
			}
			continue
		}
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !IsSharedRulePath(name) {
			continue
		}
		if err := rejectLinkedPrefixes(root, name); err != nil {
			return nil, err
		}
		files = append(files, name)
	}
	return files, nil
}

func rejectLinkedPrefixes(root, relative string) error {
	p, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	for _, part := range strings.Split(filepath.FromSlash(relative), string(filepath.Separator)) {
		p = filepath.Join(p, part)
		info, e := os.Lstat(p)
		if os.IsNotExist(e) {
			return nil
		}
		if e != nil {
			return e
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("linked shared rules path is not accessible: %s", relative)
		}
		if info.Mode()&os.ModeIrregular != 0 {
			if _, e := os.Readlink(p); e == nil {
				return fmt.Errorf("linked shared rules path is not accessible: %s", relative)
			} else if !os.IsNotExist(e) {
				return e
			}
		}
	}
	return nil
}

func copyShared(source, dest string, archiveDiff bool) error {
	files, err := sharedFiles(source)
	if err != nil {
		return err
	}
	for _, rel := range files {
		src, err := ConstrainPath(source, rel)
		if err != nil {
			return err
		}
		dst, err := ConstrainPath(dest, rel)
		if err != nil {
			return err
		}
		if err := rejectLinkedPrefixes(source, rel); err != nil {
			return err
		}
		if err := rejectLinkedPrefixes(dest, rel); err != nil {
			return err
		}
		body, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		if old, e := os.ReadFile(dst); e == nil {
			if string(old) == string(body) {
				continue
			}
			if archiveDiff {
				if err := archiveOne(dest, rel, old); err != nil {
					return err
				}
			}
		} else if !os.IsNotExist(e) {
			return e
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(dst, body, 0644); err != nil {
			return err
		}
	}
	return nil
}

func archiveShared(root string) error {
	files, err := sharedFiles(root)
	if err != nil {
		return err
	}
	for _, rel := range files {
		p, e := ConstrainPath(root, rel)
		if e != nil {
			return e
		}
		b, e := os.ReadFile(p)
		if e != nil {
			return e
		}
		if e = archiveOne(root, rel, b); e != nil {
			return e
		}
	}
	return nil
}
func archiveOne(root, rel string, b []byte) error {
	dir := filepath.Join(root, ".cse", "rules-archive", time.Now().UTC().Format("20060102T150405.000000000Z"))
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
		return e
	}
	return os.WriteFile(p, b, 0600)
}
func writeBootstrap(root, name, syncRoot string) error {
	authority := filepath.Join(syncRoot, name)
	body := []byte("# CE-SE local agent bootstrap\n\nTeacher rules are configured in Sync at `" + syncRoot + "`. Read the authoritative instructions at `" + authority + "` and other teacher rules from that directory. This Install at `" + root + "` remains the command, course and student-work root; run the Engine here. `context/courses.md` and `context/students.md` remain local to Install. Do not use inactive local rule copies or run launchers from Sync. Never read, list or search `.cse/`; it contains private student mappings and machine state. For private setup or student pages, provide only the local URL and session status; never fetch their contents.\n")
	p := filepath.Join(root, name)
	return os.WriteFile(p, body, 0644)
}
func writeRulesBinding(root, syncRoot string) error {
	p := rulesBindingPath(root)
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(rulesBinding{SyncRoot: syncRoot}, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	tmp := p + ".tmp"
	if err = os.WriteFile(tmp, b, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// ScaffoldRules applies only starter files that belong to the shared rules
// authority; local context and product files are left in Install.
func ScaffoldRules(root string, template fs.FS, base string) ([]string, error) {
	sub, err := fs.Sub(template, base)
	if err != nil {
		return nil, err
	}
	var created []string
	err = fs.WalkDir(sub, ".", func(name string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if name == "." {
			return nil
		}
		slash := filepath.ToSlash(name)
		if d.IsDir() {
			return nil
		}
		if !IsSharedRulePath(slash) {
			return nil
		}
		p, e := ConstrainPath(root, slash)
		if e != nil {
			return e
		}
		if e := rejectLinkedPrefixes(root, slash); e != nil {
			return e
		}
		if _, e = os.Lstat(p); e == nil {
			return nil
		} else if !os.IsNotExist(e) {
			return e
		}
		b, e := fs.ReadFile(sub, name)
		if e != nil {
			return e
		}
		if e = os.MkdirAll(filepath.Dir(p), 0755); e != nil {
			return e
		}
		if e = os.WriteFile(p, b, 0644); e != nil {
			return e
		}
		created = append(created, slash)
		return nil
	})
	return created, err
}
