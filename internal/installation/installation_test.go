package installation

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"github.com/pelletier/go-toml/v2"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCoworkPackageTargetsSelectedInstallWithoutSecondEngine(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Install with spaces")
	os.MkdirAll(filepath.Join(root, "engine"), 0700)
	f, err := os.Create(filepath.Join(root, "engine", "CanvasExpert-SE-Cowork.mcpb"))
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(f)
	manifest, _ := archive.Create("manifest.json")
	manifest.Write([]byte(`{"user_config":{"workspace":{"type":"directory"}},"server":{"mcp_config":{"command":"${__dirname}/launcher/cse-mcp-launcher.exe","args":["--install","${user_config.workspace}"]}}}`))
	launcher, _ := archive.Create("launcher/cse-mcp-launcher.exe")
	launcher.Write([]byte("synthetic launcher"))
	archive.Close()
	f.Close()
	result, err := prepareCowork(root)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != "extension-ready" {
		t.Fatal("prepared package claimed registration")
	}
	out, err := zip.OpenReader(result.ExtensionPath)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	for _, entry := range out.File {
		if entry.Name == "engine/cse.exe" {
			t.Fatal("extension carries second Engine")
		}
		if entry.Name != "manifest.json" {
			continue
		}
		reader, _ := entry.Open()
		data, _ := io.ReadAll(reader)
		reader.Close()
		var cfg map[string]any
		if err = json.Unmarshal(data, &cfg); err != nil {
			t.Fatal(err)
		}
		if cfg["user_config"] != nil {
			t.Fatal("bound extension still asks for an Install")
		}
		args := cfg["server"].(map[string]any)["mcp_config"].(map[string]any)["args"].([]any)
		if len(args) != 2 || args[0] != "--install" || args[1] != root {
			t.Fatalf("wrong target: %v", args)
		}
	}
}

func TestWorkRegistrationPreservesAndRepeatsWithoutCLI(t *testing.T) {
	t.Setenv("PATH", "")
	root := filepath.Join(t.TempDir(), "Install with spaces")
	exe := filepath.Join(root, "engine", "cse.exe")
	path := filepath.Join(t.TempDir(), "custom codex home", "config.toml")
	os.MkdirAll(filepath.Dir(path), 0700)
	original := []byte("model = 'teacher-model'\n[mcp_servers.other]\ncommand = 'other.exe'\n[mcp_servers.other.env]\nPRIVATE_VALUE = 'keep-me'\n")
	os.WriteFile(path, original, 0600)
	result, err := registerWork(root, exe, path, false)
	if err != nil {
		t.Fatal(err)
	}
	backup, err := os.ReadFile(result.BackupPath)
	if err != nil || string(backup) != string(original) {
		t.Fatal("original configuration not backed up")
	}
	data, _ := os.ReadFile(path)
	var cfg map[string]any
	if err := toml.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg["model"] != "teacher-model" {
		t.Fatal("unrelated setting lost")
	}
	servers := cfg["mcp_servers"].(map[string]any)
	if servers["other"].(map[string]any)["env"].(map[string]any)["PRIVATE_VALUE"] != "keep-me" {
		t.Fatal("unrelated server changed")
	}
	args := servers["canvasexpert-se"].(map[string]any)["args"].([]any)
	if args[1] != root {
		t.Fatalf("path with spaces changed: %v", args)
	}
	again, err := registerWork(root, exe, path, false)
	if err != nil || again.State != "settings-already-saved" || again.BackupPath != "" {
		t.Fatalf("repeat install: %+v %v", again, err)
	}
}

func TestRegisterReportsSelectedInstallRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Extracted Install")
	for path, contents := range map[string]string{
		filepath.Join(root, "engine", "cse.exe"):     "engine",
		filepath.Join(root, "AGENTS.md"):             "instructions",
		filepath.Join(root, "context", "courses.md"): "courses",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("CODEX_HOME", filepath.Join(t.TempDir(), "codex"))
	result, err := Register(root, "work", false)
	if err != nil {
		t.Fatal(err)
	}
	if result.InstallRoot != root {
		t.Fatalf("helper returned Install root %q, want %q", result.InstallRoot, root)
	}
}

func TestWorkReplacementAndMalformedPreservation(t *testing.T) {
	root := filepath.Join(t.TempDir(), "New Install")
	old := filepath.Join(t.TempDir(), "Old Install")
	path := filepath.Join(t.TempDir(), "config.toml")
	_, err := registerWork(old, filepath.Join(old, "engine", "cse.exe"), path, false)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	_, err = registerWork(root, filepath.Join(root, "engine", "cse.exe"), path, false)
	var required *ReplacementRequired
	if !errors.As(err, &required) || required.ExistingInstall != old {
		t.Fatalf("wrong Install not explained: %v", err)
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("changed config before explicit replacement")
	}
	if _, err = registerWork(root, filepath.Join(root, "engine", "cse.exe"), path, true); err != nil {
		t.Fatal(err)
	}
	bad := []byte("[broken\nsecret = 'keep'\n")
	os.WriteFile(path, bad, 0600)
	if _, err = registerWork(root, filepath.Join(root, "engine", "cse.exe"), path, true); err == nil {
		t.Fatal("malformed config accepted")
	}
	after, _ = os.ReadFile(path)
	if string(after) != string(bad) {
		t.Fatal("malformed config overwritten")
	}
}
func TestConfiguredCodexHome(t *testing.T) {
	home := filepath.Join(t.TempDir(), "Custom home")
	t.Setenv("CODEX_HOME", home)
	if workConfig() != filepath.Join(home, "config.toml") {
		t.Fatal("CODEX_HOME ignored")
	}
}

func TestCoworkLaunchOutcomesAreNarrowAndHonest(t *testing.T) {
	prepared := Result{Client: "cowork", ExtensionPath: `C:\Install with spaces\engine\prepared.mcpb`, NextAction: "Manual extension path."}
	const exe = `C:\Users\T\AppData\Local\AnthropicClaude\claude.exe`
	found := func() (string, bool) { return exe, true }
	missing := func() (string, bool) { return "", false }
	noReveal := func(string) error { t.Fatal("revealed after direct launch"); return nil }

	var gotExe string
	var gotArgs []string
	requested := CoworkLaunchOutcome(prepared, true, Launcher{Find: found, Reveal: noReveal, Launch: func(e string, a ...string) error {
		gotExe, gotArgs = e, a
		return nil
	}})
	if requested.CoworkLaunchOutcome != "launch_requested" || gotExe != exe || len(gotArgs) != 1 || gotArgs[0] != prepared.ExtensionPath {
		t.Fatalf("direct launch must pass the path as one argument: %q %q %+v", gotExe, gotArgs, requested)
	}
	if strings.Contains(strings.ToLower(requested.NextAction), "is installed") || !strings.Contains(requested.NextAction, "does not confirm") {
		t.Fatalf("misleading launch result: %+v", requested)
	}

	var revealed string
	reveal := func(p string) error { revealed = p; return nil }
	for name, l := range map[string]Launcher{
		"not found":     {Find: missing, Launch: func(string, ...string) error { t.Fatal("launched missing app"); return nil }, Reveal: reveal},
		"launch failed": {Find: found, Launch: func(string, ...string) error { return errors.New("synthetic failure PAT_SYNTHETIC") }, Reveal: reveal},
	} {
		revealed = ""
		r := CoworkLaunchOutcome(prepared, true, l)
		if r.CoworkLaunchOutcome != "file_revealed" || revealed != prepared.ExtensionPath || !strings.Contains(r.NextAction, "Settings > Extensions") || strings.Contains(r.NextAction, "PAT_SYNTHETIC") {
			t.Fatalf("%s: bad fallback: %+v", name, r)
		}
	}

	failed := CoworkLaunchOutcome(prepared, true, Launcher{Find: missing, Reveal: func(string) error { return errors.New("no explorer") }})
	if failed.CoworkLaunchOutcome != "launch_failed" || !strings.Contains(failed.NextAction, prepared.ExtensionPath) || !strings.Contains(failed.NextAction, "Settings > Extensions") {
		t.Fatalf("bad failure: %+v", failed)
	}

	never := func() { t.Fatal("no-UI opened something") }
	noUI := CoworkLaunchOutcome(prepared, false, Launcher{
		Find: func() (string, bool) { never(); return "", false }, Launch: func(string, ...string) error { never(); return nil }, Reveal: func(string) error { never(); return nil }})
	if noUI.CoworkLaunchOutcome != "prepared" {
		t.Fatalf("no-UI outcome: %+v", noUI)
	}
}

func TestCompactEngineVersionKeepsBuildIdentityOnly(t *testing.T) {
	in := "CanvasExpert-SE 0.9.1 (SE = x)\nrevision abc123+dirty\nbuilt 2026-10-12T23:09:05Z\nbuild_kind release\ngo1.27 windows/amd64\ndependency a/b v1\ndependency c/d v2\n"
	want := "version 0.9.1\nrevision abc123+dirty\nbuilt 2026-10-12T23:09:05Z\nbuild_kind release"
	if got := compactEngineVersion(in); got != want {
		t.Fatalf("got %q", got)
	}
}

func TestAntigravityRegistrationPreservesReplacesAndRepeats(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Install with spaces")
	exe := filepath.Join(root, "engine", "cse.exe")
	path := filepath.Join(t.TempDir(), ".gemini", "config", "mcp_config.json")
	os.MkdirAll(filepath.Dir(path), 0700)
	original := []byte(`{"theme":"dark","mcpServers":{"other":{"command":"other.exe","env":{"PRIVATE_VALUE":"keep-me"}}}}`)
	os.WriteFile(path, original, 0600)
	result, err := registerAntigravity(root, exe, path, false)
	if err != nil || result.State != "settings-saved" {
		t.Fatalf("%+v %v", result, err)
	}
	if backup, err := os.ReadFile(result.BackupPath); err != nil || string(backup) != string(original) {
		t.Fatal("original configuration not backed up")
	}
	var cfg map[string]any
	data, _ := os.ReadFile(path)
	if err := json.Unmarshal(data, &cfg); err != nil || cfg["theme"] != "dark" {
		t.Fatalf("unrelated setting lost: %v", err)
	}
	servers := cfg["mcpServers"].(map[string]any)
	if servers["other"].(map[string]any)["env"].(map[string]any)["PRIVATE_VALUE"] != "keep-me" {
		t.Fatal("unrelated server changed")
	}
	if args := servers["canvasexpert-se"].(map[string]any)["args"].([]any); args[1] != root || args[2] != "mcp" {
		t.Fatalf("wrong args: %v", args)
	}
	again, err := registerAntigravity(root, exe, path, false)
	if err != nil || again.State != "settings-already-saved" || again.BackupPath != "" {
		t.Fatalf("repeat install: %+v %v", again, err)
	}
	other := filepath.Join(t.TempDir(), "Other Install")
	before, _ := os.ReadFile(path)
	_, err = registerAntigravity(other, filepath.Join(other, "engine", "cse.exe"), path, false)
	var required *ReplacementRequired
	if !errors.As(err, &required) || !samePath(required.ExistingInstall, root) {
		t.Fatalf("different Install not explained: %v", err)
	}
	if after, _ := os.ReadFile(path); string(after) != string(before) {
		t.Fatal("changed before explicit replacement")
	}
	if _, err = registerAntigravity(other, filepath.Join(other, "engine", "cse.exe"), path, true); err != nil {
		t.Fatal(err)
	}
	bad := []byte(`{"mcpServers": {`)
	os.WriteFile(path, bad, 0600)
	if _, err = registerAntigravity(root, exe, path, true); err == nil {
		t.Fatal("malformed config accepted")
	}
	if after, _ := os.ReadFile(path); string(after) != string(bad) {
		t.Fatal("malformed config overwritten")
	}
}

func TestAntigravityRegistrationCreatesMissingConfig(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(t.TempDir(), "new", "mcp_config.json")
	result, err := registerAntigravity(root, filepath.Join(root, "engine", "cse.exe"), path, false)
	if err != nil || result.BackupPath != "" {
		t.Fatalf("%+v %v", result, err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("config not created")
	}
}

func TestRegisterAutoConnectsOnlyDetectedClients(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Extracted Install")
	for path, contents := range map[string]string{
		filepath.Join(root, "engine", "cse.exe"):     "engine",
		filepath.Join(root, "AGENTS.md"):             "instructions",
		filepath.Join(root, "context", "courses.md"): "courses",
	} {
		os.MkdirAll(filepath.Dir(path), 0700)
		os.WriteFile(path, []byte(contents), 0600)
	}
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)
	t.Setenv("APPDATA", filepath.Join(home, "AppData"))
	t.Setenv("CODEX_HOME", filepath.Join(home, "no-codex"))
	if results, err := RegisterAuto(root, false); err != nil || len(results) != 0 {
		t.Fatalf("nothing installed: %+v %v", results, err)
	}
	os.MkdirAll(filepath.Join(home, ".gemini"), 0700)
	os.MkdirAll(filepath.Join(home, "no-codex"), 0700)
	results, err := RegisterAuto(root, false)
	if err != nil || len(results) != 2 {
		t.Fatalf("%+v %v", results, err)
	}
	got := map[string]bool{}
	for _, r := range results {
		got[r.Client] = r.InstallRoot == root
	}
	if !got["work"] || !got["antigravity"] || got["cowork"] {
		t.Fatalf("wrong clients: %v", got)
	}
}
