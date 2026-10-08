package platform

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// LocalDir is the per-machine, unsynced folder for non-secret state such as the
// program link and the hub's last workspace. LOCALAPPDATA never roams.
func LocalDir() (string, error) {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		var err error
		if base, err = os.UserConfigDir(); err != nil {
			return "", err
		}
	}
	return filepath.Join(base, "CanvasExpert-SE"), nil
}

// LinkProgram records where this machine's cse executable lives so a synced
// workspace's cse.cmd can find it without containing the program.
func LinkProgram() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if exe, err = filepath.Abs(exe); err != nil {
		return "", err
	}
	dir, err := LocalDir()
	if err != nil {
		return "", err
	}
	if err = os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	if err = os.WriteFile(filepath.Join(dir, "program.txt"), []byte(exe+"\r\n"), 0o600); err != nil {
		return "", err
	}
	return exe, nil
}

// LinkedProgram returns the executable recorded by LinkProgram.
func LinkedProgram() (string, error) {
	dir, err := LocalDir()
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(filepath.Join(dir, "program.txt"))
	if err != nil {
		return "", errors.New("no program is linked on this machine; run webui.cmd in the program folder")
	}
	return strings.TrimSpace(string(b)), nil
}

// Overlaps reports whether either folder contains the other (or they are equal).
func Overlaps(a, b string) bool {
	return within(a, b) || within(b, a)
}

func within(parent, child string) bool {
	p, err1 := filepath.Abs(parent)
	c, err2 := filepath.Abs(child)
	if err1 != nil || err2 != nil {
		return false
	}
	rel, err := filepath.Rel(p, c)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
