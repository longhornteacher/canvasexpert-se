package installation

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"github.com/pelletier/go-toml/v2"
	"io"
	"os"
	"path/filepath"
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
