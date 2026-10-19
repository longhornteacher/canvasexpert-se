package workspace

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Scaffold creates the starter workspace inside root from the template files
// found under base in template. It only adds: any file that already exists is
// left untouched. It returns the relative paths it created.
func Scaffold(root string, template fs.FS, base string) ([]string, error) {
	sub, err := fs.Sub(template, base)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("create %s: %w", root, err)
	}
	var created []string
	var firstErr error
	err = fs.WalkDir(sub, ".", func(name string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if name == "." {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		logical := filepath.ToSlash(name)
		target, err := ResolveTeacherPath(root, logical)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			return nil
		}
		if _, err := os.Lstat(target); err == nil {
			return nil
		} else if !os.IsNotExist(err) {
			return err
		}
		body, err := fs.ReadFile(sub, name)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, body, 0o644); err != nil {
			return err
		}
		created = append(created, path.Clean(name))
		return nil
	})
	if err != nil {
		return created, err
	}
	return created, firstErr
}

// BootstrapBundled prepares a freshly extracted UserInstall for its bundled
// Engine. The ZIP carries no context/ or playbooks, so a folder holding
// engine/<exe> and AGENTS.md but no context/courses.md gets the starter files
// laid down here. Existing files are never touched.
func BootstrapBundled(executable string, template fs.FS, base string) error {
	programDir := filepath.Dir(executable)
	if !strings.EqualFold(filepath.Base(programDir), "engine") {
		return nil
	}
	installed := filepath.Dir(programDir)
	if isRoot(installed) {
		return nil
	}
	if info, err := os.Stat(filepath.Join(installed, "AGENTS.md")); err != nil || info.IsDir() {
		return nil
	}
	// courses.md marks the root; write it first so teacher paths resolve.
	body, err := fs.ReadFile(template, path.Join(base, "context", "courses.md"))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(installed, "context"), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(installed, "context", "courses.md"), body, 0o644); err != nil {
		return err
	}
	_, err = Scaffold(installed, template, base)
	return err
}
