package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ResolveRuntimeRoot binds a bundled Engine to its extracted UserInstall.
// The ZIP runs in place: setup registers this root without copying it elsewhere.
// Development and extension binaries have no adjacent teacher configuration and
// retain the explicit root (or CLI working-directory discovery).
func ResolveRuntimeRoot(explicit, start, executable string) (string, error) {
	programDir := filepath.Dir(executable)
	installed := filepath.Dir(programDir)
	info, err := os.Stat(filepath.Join(installed, "context", "courses.md"))
	if strings.EqualFold(filepath.Base(programDir), "engine") && err == nil && !info.IsDir() {
		if explicit != "" {
			requested, err := filepath.Abs(explicit)
			if err != nil {
				return "", err
			}
			same := strings.EqualFold(filepath.Clean(requested), filepath.Clean(installed))
			if !same {
				a, aerr := os.Stat(requested)
				b, berr := os.Stat(installed)
				same = aerr == nil && berr == nil && os.SameFile(a, b)
			}
			if !same {
				return "", fmt.Errorf("CE-SE installation mismatch: this bundled Engine uses %s; --workspace requested %s. Use the Engine in the intended installation", installed, requested)
			}
		}
		return ResolveRoot(installed, "")
	}
	return ResolveRoot(explicit, start)
}

func ResolveRoot(explicit, start string) (string, error) {
	if explicit != "" {
		p, e := filepath.Abs(explicit)
		if e != nil {
			return "", e
		}
		if !isRoot(p) {
			return "", fmt.Errorf("workspace root %s lacks context/courses.md", p)
		}
		return p, nil
	}
	if start == "" {
		var e error
		start, e = os.Getwd()
		if e != nil {
			return "", e
		}
	}
	p, e := filepath.Abs(start)
	if e != nil {
		return "", e
	}
	if info, e := os.Stat(p); e == nil && !info.IsDir() {
		p = filepath.Dir(p)
	}
	for {
		if isRoot(p) {
			return p, nil
		}
		parent := filepath.Dir(p)
		if parent == p {
			break
		}
		p = parent
	}
	return "", fmt.Errorf("no workspace found above %s; use --workspace", start)
}
func isRoot(p string) bool {
	migrateLegacyFolders(p)
	i, e := os.Stat(filepath.Join(p, "context", "courses.md"))
	return e == nil && !i.IsDir()
}

var deviceName = regexp.MustCompile(`(?i)^(CON|PRN|AUX|NUL|CONIN\$|CONOUT\$|COM[1-9¹²³]|LPT[1-9¹²³])(?: *\..*)?$`)

func inside(root, p string) bool {
	rel, e := filepath.Rel(root, p)
	return e == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}
func ConstrainPath(root, relative string) (string, error) {
	if relative == "" || filepath.IsAbs(relative) || filepath.VolumeName(relative) != "" || strings.HasPrefix(relative, "/") || strings.HasPrefix(relative, "\\") {
		return "", fmt.Errorf("expected a relative workspace path")
	}
	// Reject Windows path hazards even on cross-compiling development hosts.
	for _, part := range strings.FieldsFunc(relative, func(r rune) bool { return r == '/' || r == '\\' }) {
		if part == ".." || strings.ContainsAny(part, `<>:"|?*`) || strings.IndexFunc(part, func(r rune) bool { return r < 32 }) >= 0 || strings.HasSuffix(part, " ") || strings.HasSuffix(part, ".") && part != "." || deviceName.MatchString(part) {
			return "", fmt.Errorf("unsafe path component %q", part)
		}
	}
	r, e := filepath.Abs(root)
	if e != nil {
		return "", e
	}
	p := filepath.Join(r, filepath.FromSlash(strings.ReplaceAll(relative, "\\", "/")))
	if !inside(r, p) {
		return "", fmt.Errorf("path escapes workspace")
	}
	resolvedRoot, e := resolveExistingPath(r, 0)
	if e != nil {
		return "", e
	}
	// Resolve the nearest existing ancestor to catch symlink/junction escapes
	// for both existing files and newly created downloads.
	ancestor := p
	for {
		if _, e := os.Lstat(ancestor); e == nil {
			resolved, e := resolveExistingPath(ancestor, 0)
			if e != nil {
				return "", e
			}
			if !inside(resolvedRoot, resolved) {
				return "", fmt.Errorf("path resolves outside workspace")
			}
			break
		} else if !os.IsNotExist(e) {
			return "", e
		}
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			return "", fmt.Errorf("no existing path ancestor")
		}
		ancestor = parent
	}
	return p, nil
}

// Go's current Windows Lstat marks directory junctions as ModeIrregular,
// which EvalSymlinks does not follow. Readlink supports both junctions and
// symlinks; inspect each existing prefix so either kind is resolved before
// comparing containment. Non-surrogate OneDrive reparse points remain valid.
func resolveExistingPath(path string, links int) (string, error) {
	if links > 255 {
		return "", fmt.Errorf("too many path links")
	}
	path = filepath.Clean(path)
	volume := filepath.VolumeName(path)
	rest := strings.TrimPrefix(path[len(volume):], string(filepath.Separator))
	parts := strings.Split(rest, string(filepath.Separator))
	prefix := volume + string(filepath.Separator)
	for i, part := range parts {
		if part == "" {
			continue
		}
		prefix = filepath.Join(prefix, part)
		info, e := os.Lstat(prefix)
		if e != nil {
			return "", e
		}
		if info.Mode()&(os.ModeSymlink|os.ModeIrregular) == 0 {
			continue
		}
		target, e := os.Readlink(prefix)
		if e != nil {
			// Other reparse tags (including cloud placeholders) return ENOENT
			// from Readlink; Lstat above established that this path exists.
			if info.Mode()&os.ModeSymlink == 0 && os.IsNotExist(e) {
				continue
			}
			return "", e
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(prefix), target)
		}
		for _, tail := range parts[i+1:] {
			target = filepath.Join(target, tail)
		}
		return resolveExistingPath(target, links+1)
	}
	return filepath.EvalSymlinks(path)
}

// SafeFilename produces one portable filename component from a remote label.
func SafeFilename(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r < 32 || strings.ContainsRune(`<>:"/\|?*`, r) {
			b.WriteByte('_')
		} else {
			b.WriteRune(r)
		}
	}
	v := strings.Trim(b.String(), " .")
	if v == "" {
		v = "item"
	}
	if deviceName.MatchString(v) {
		v = "_" + v
	}
	r := []rune(v)
	if len(r) > 120 {
		v = strings.TrimRight(string(r[:120]), " .")
	}
	return v
}

// migrateLegacyFolders renames the folders of an older workspace (settings/ is
// now context/, guides/ is now playbooks/). It only moves a folder when the new
// name is absent, so nothing is overwritten.
func migrateLegacyFolders(p string) {
	if _, e := os.Stat(filepath.Join(p, "settings", "courses.md")); e != nil {
		return
	}
	for _, m := range [][2]string{{"settings", "context"}, {"guides", "playbooks"}} {
		if _, e := os.Stat(filepath.Join(p, m[1])); os.IsNotExist(e) {
			_ = os.Rename(filepath.Join(p, m[0]), filepath.Join(p, m[1]))
		}
	}
}
