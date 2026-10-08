package workspace

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRejectMaliciousWindowsPaths(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{"../escape", "a\\..\\escape", "C:\\escape", "\\\\server\\share", "/absolute", "file:stream", "CON.txt", "CON .txt", "CONIN$", "CONOUT$", "folder/NUL", "COM¹", "folder/file.", "folder/file ", "a\nb", "a?b", "a|b", "a*b", "a<b"} {
		if got, e := ConstrainPath(root, p); e == nil {
			t.Fatalf("unsafe path %q accepted as %q", p, got)
		}
	}
	if _, e := ConstrainPath(root, "folder with spaces/élève.txt"); e != nil {
		t.Fatal(e)
	}
}
func TestRejectSymlinkEscape(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	link := filepath.Join(root, "redirect")
	if e := os.Symlink(outside, link); e != nil {
		if runtime.GOOS != "windows" {
			t.Skipf("symlink permission unavailable: %v", e)
		}
		// Standard Windows users may create directory junctions even when
		// symbolic links require a privilege. Both must stay inside the root.
		command := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", `$ErrorActionPreference='Stop'; New-Item -ItemType Junction -Path $env:CSE_TEST_LINK -Target $env:CSE_TEST_TARGET | Out-Null`)
		command.Env = append(os.Environ(), "CSE_TEST_LINK="+link, "CSE_TEST_TARGET="+outside)
		if output, junctionErr := command.CombinedOutput(); junctionErr != nil {
			t.Skipf("symlink/junction unavailable: %v; %s", junctionErr, output)
		}
	}
	if _, e := ConstrainPath(root, "redirect/new.txt"); e == nil {
		t.Fatal("symlink download escapes root")
	}
	if e := os.Mkdir(filepath.Join(outside, "nested"), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(outside, "nested", "existing.txt"), []byte("synthetic"), 0600); e != nil {
		t.Fatal(e)
	}
	for _, path := range []string{"redirect/nested/new.txt", "redirect/nested/existing.txt"} {
		if _, e := ConstrainPath(root, path); e == nil {
			t.Fatalf("junction ancestor escapes root: %s", path)
		}
	}
	// A teacher may intentionally locate the entire workspace through a link;
	// containment should then use that workspace's resolved root.
	if _, e := ConstrainPath(link, "nested/new.txt"); e != nil {
		t.Fatalf("linked workspace root rejected: %v", e)
	}
}
func TestSafeFilenameRemainsPortableAfterTruncation(t *testing.T) {
	root := t.TempDir()
	for _, raw := range []string{"../../CON", "NUL.txt", "COM¹.txt", "name:stream", strings.Repeat("a", 119) + "." + "more"} {
		name := SafeFilename(raw)
		if _, e := ConstrainPath(root, name); e != nil {
			t.Fatalf("sanitized %q is unsafe: %q %v", raw, name, e)
		}
	}
}
