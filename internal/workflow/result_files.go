package workflow

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"canvasexpertse/internal/workspace"
)

// walkResultFiles discovers local result documents by their frontmatter
// content. It never resolves teacher-rule paths into UserSync or follows links,
// hidden/private directories, or Engine folders.
func walkResultFiles(root string, visit func(string) error) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry == nil {
			return nil
		}
		if path == root {
			return nil
		}
		excluded := strings.HasPrefix(entry.Name(), ".") || strings.EqualFold(entry.Name(), "engine") || entry.Type()&os.ModeSymlink != 0
		if entry.Type()&os.ModeIrregular != 0 {
			// Junctions are links; ordinary provider placeholders are not.
			_, linkErr := os.Readlink(path)
			excluded = excluded || linkErr == nil || !os.IsNotExist(linkErr)
		}
		if excluded {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		if _, err := workspace.ConstrainPath(root, rel); err != nil {
			return nil
		}
		// Posting is content-based, even for extensionless CLI files. A tiny
		// frontmatter check avoids parsing arbitrary assets and binaries.
		file, err := os.Open(path)
		if err != nil {
			return nil
		}
		prefix := make([]byte, 8)
		n, _ := file.Read(prefix)
		_ = file.Close()
		prefix = bytes.TrimPrefix(prefix[:n], []byte{0xef, 0xbb, 0xbf})
		if !bytes.HasPrefix(prefix, []byte("---\n")) && !bytes.HasPrefix(prefix, []byte("---\r\n")) {
			return nil
		}
		return visit(path)
	})
}
