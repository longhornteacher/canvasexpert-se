package workspace

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
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
	err = fs.WalkDir(sub, ".", func(name string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if name == "." {
			return nil
		}
		target, err := ConstrainPath(root, filepath.FromSlash(name))
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
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
	return created, err
}
