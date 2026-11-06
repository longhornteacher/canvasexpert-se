package gitupdate

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// scriptFolder lays out a folder with the real get-engine.cmd, an installed Engine and
// extension, and a "release" of new files served over file:// for curl.
func scriptFolder(t *testing.T) (root string) {
	t.Helper()
	if runtime.GOOS != "windows" {
		t.Skip("get-engine.cmd is a Windows script")
	}
	for _, tool := range []string{"curl.exe", "certutil.exe"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skip(tool + " not available")
		}
	}
	root, assets := t.TempDir(), t.TempDir()
	script, err := os.ReadFile(filepath.Join("..", "..", "scripts", "get-engine.cmd"))
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, "get-engine.cmd"), string(script))
	write(t, filepath.Join(root, "engine", "cse.exe"), "old-engine")
	write(t, filepath.Join(root, "engine", "CanvasExpert-SE-Cowork.mcpb"), "old-extension")
	write(t, filepath.Join(assets, "cse.exe"), "new-engine")
	write(t, filepath.Join(assets, "CanvasExpert-SE-Cowork.mcpb"), "new-extension")
	base := "file:///" + strings.ReplaceAll(assets, `\`, "/")
	write(t, filepath.Join(root, "engine", "manifest.txt"), "version=2\nidentity=2\nbase_url="+base+"\nsha256.cse.exe="+sum("new-engine")+"\nsha256.CanvasExpert-SE-Cowork.mcpb="+sum("new-extension")+"\n")
	return root
}

func runScript(root string) (int, string) {
	cmd := exec.Command("cmd.exe", "/d", "/c", `.\get-engine.cmd`)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		if cmd.ProcessState != nil {
			return cmd.ProcessState.ExitCode(), string(out)
		}
		return -1, err.Error()
	}
	return 0, string(out)
}

func readText(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		return "<missing>"
	}
	return string(b)
}

func TestGetEngineInstallsEveryAssetAndKeepsPrevious(t *testing.T) {
	root := scriptFolder(t)
	if code, out := runScript(root); code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
	engine := filepath.Join(root, "engine")
	if readText(t, filepath.Join(engine, "cse.exe")) != "new-engine" || readText(t, filepath.Join(engine, "CanvasExpert-SE-Cowork.mcpb")) != "new-extension" {
		t.Fatal("new files not installed")
	}
	if readText(t, filepath.Join(engine, "cse.exe.previous")) != "old-engine" || readText(t, filepath.Join(engine, "CanvasExpert-SE-Cowork.mcpb.previous")) != "old-extension" {
		t.Fatal("previous copies not kept")
	}
}

// If any current file cannot be moved aside, nothing is left half installed: the Engine and the
// extension are exactly as they were, and an older .previous is never restored over them.
func TestGetEngineRollsBackWhenAFileCannotBeReplaced(t *testing.T) {
	root := scriptFolder(t)
	engine := filepath.Join(root, "engine")
	write(t, filepath.Join(engine, "cse.exe.previous"), "ancient-engine")
	// Go opens files without FILE_SHARE_DELETE, so holding this open blocks the rename.
	held, err := os.Open(filepath.Join(engine, "CanvasExpert-SE-Cowork.mcpb"))
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	code, out := runScript(root)
	if code != 6 {
		t.Fatalf("want exit 6, got %d: %s", code, out)
	}
	if readText(t, filepath.Join(engine, "cse.exe")) != "old-engine" {
		t.Fatalf("the Engine was not restored: %q", readText(t, filepath.Join(engine, "cse.exe")))
	}
	if readText(t, filepath.Join(engine, "CanvasExpert-SE-Cowork.mcpb")) != "old-extension" {
		t.Fatal("the extension changed")
	}
	if _, err := os.Stat(filepath.Join(engine, "cse.exe.previous")); err == nil {
		t.Fatal("a .previous copy was left behind after the rollback")
	}
}
