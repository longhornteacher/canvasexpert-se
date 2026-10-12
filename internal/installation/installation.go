// Package installation connects one extracted Install to a selected local client.
// It never reads credentials or calls Canvas.
package installation

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/pelletier/go-toml/v2"
)

type Result struct {
	Client              string `json:"client"`
	InstallRoot         string `json:"install_root"`
	State               string `json:"state"`
	ConfigPath          string `json:"config_path,omitempty"`
	BackupPath          string `json:"backup_path,omitempty"`
	ExtensionPath       string `json:"extension_path,omitempty"`
	NextAction          string `json:"next_action"`
	ExistingInstall     string `json:"existing_install,omitempty"`
	CoworkLaunchOutcome string `json:"cowork_launch_outcome,omitempty"`
}
type CheckResult struct {
	State                 string `json:"state"`
	ToolCount             int    `json:"tool_count"`
	AdapterVersion        string `json:"adapter_version,omitempty"`
	InstructionsRead      bool   `json:"instructions_read"`
	ConfiguredCoursesRead bool   `json:"configured_courses_read"`
	DestinationVerified   bool   `json:"destination_verified"`
	EngineStarts          bool   `json:"engine_starts"`
	ChildEngineVersion    string `json:"child_engine_version,omitempty"`
}
type Availability struct {
	WorkConfigFound      bool   `json:"work_config_found"`
	CoworkConfigFound    bool   `json:"cowork_config_found"`
	WorkConfigPath       string `json:"work_config_path"`
	AvailabilityEvidence string `json:"availability_evidence"`
}
type ReplacementRequired struct{ ExistingInstall, InstallRoot string }

func (e *ReplacementRequired) Error() string {
	return fmt.Sprintf("CE-SE is already registered against %s. Replacing that connection with %s needs an explicit replacement choice; neither Install's files will be changed", e.ExistingInstall, e.InstallRoot)
}

func workConfig() string {
	base := os.Getenv("CODEX_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".codex")
	}
	return filepath.Join(base, "config.toml")
}
func Inspect(root string) Availability {
	config := workConfig()
	_, err := os.Stat(filepath.Dir(config))
	_, claudeErr := os.Stat(filepath.Join(os.Getenv("APPDATA"), "Claude"))
	return Availability{WorkConfigFound: err == nil, CoworkConfigFound: claudeErr == nil, WorkConfigPath: config, AvailabilityEvidence: "configuration-directory-only; app availability unconfirmed"}
}
func validate(root string) (string, string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return "", "", err
	}
	exe := filepath.Join(root, "engine", "cse.exe")
	for _, p := range []string{exe, filepath.Join(root, "AGENTS.md"), filepath.Join(root, "context", "courses.md")} {
		st, err := os.Stat(p)
		if err != nil || st.IsDir() {
			return "", "", fmt.Errorf("selected Install is missing %s", p)
		}
	}
	return root, exe, nil
}

func Register(root, client string, replace bool) (Result, error) {
	root, exe, err := validate(root)
	if err != nil {
		return Result{}, err
	}
	switch client {
	case "work":
		result, err := registerWork(root, exe, workConfig(), replace)
		result.InstallRoot = root
		return result, err
	case "cowork":
		result, err := prepareCowork(root)
		result.InstallRoot = root
		return result, err
	default:
		return Result{}, errors.New("choose client work or cowork")
	}
}

// CoworkLaunchOutcome records only the OS request outcome. It cannot establish
// that Claude accepted the extension or connected to its destination.
func CoworkLaunchOutcome(result Result, automatic bool, open func(string) error) Result {
	if result.Client != "cowork" || result.ExtensionPath == "" {
		return result
	}
	result.CoworkLaunchOutcome = "prepared"
	if automatic {
		if open == nil || open(result.ExtensionPath) != nil {
			result.CoworkLaunchOutcome = "launch_failed"
			result.NextAction += " Automatic opening failed; manually open the prepared extension at " + result.ExtensionPath + "."
		} else {
			result.CoworkLaunchOutcome = "launch_requested"
		}
	}
	result.NextAction += " OS launch does not confirm trust or installation; accept Claude Desktop's prompt and restart if requested."
	return result
}

func registerWork(root, exe, path string, replace bool) (Result, error) {
	result := Result{Client: "work", State: "settings-saved", ConfigPath: path, NextAction: "Reconnect or restart local Work/Codex, then ask the destination agent to read CE-SE AGENTS.md and configured courses. A local check does not verify that app's connection."}
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return result, fmt.Errorf("cannot read client configuration: %w", err)
	}
	config := map[string]any{}
	if len(data) > 0 {
		if err := toml.Unmarshal(data, &config); err != nil {
			return result, errors.New("client configuration is malformed; left it unchanged. Repair it in the client settings, then retry installation")
		}
	}
	servers, ok := config["mcp_servers"].(map[string]any)
	if config["mcp_servers"] != nil && !ok {
		return result, errors.New("client mcp_servers configuration is not a table; left it unchanged")
	}
	if servers == nil {
		servers = map[string]any{}
		config["mcp_servers"] = servers
	}
	wanted := map[string]any{"command": exe, "args": []any{"--workspace", root, "mcp"}}
	if old, exists := servers["canvasexpert-se"]; exists {
		oldMap, ok := old.(map[string]any)
		if !ok {
			return result, errors.New("existing CE-SE entry is not a table; left client configuration unchanged")
		}
		oldRoot := registeredRoot(oldMap)
		if !samePath(oldRoot, root) && !replace {
			return result, &ReplacementRequired{ExistingInstall: oldRoot, InstallRoot: root}
		}
		if samePath(oldRoot, root) {
			// Preserve teacher-selected tool/approval/env settings for this same Install.
			unchanged := reflect.DeepEqual(oldMap["command"], wanted["command"]) && reflect.DeepEqual(oldMap["args"], wanted["args"])
			if unchanged {
				result.State = "settings-already-saved"
				return result, nil
			}
			for k, v := range oldMap {
				if k != "command" && k != "args" && k != "url" {
					wanted[k] = v
				}
			}
		}
		result.ExistingInstall = oldRoot
	}
	servers["canvasexpert-se"] = wanted
	updated, err := toml.Marshal(config)
	if err != nil {
		return result, fmt.Errorf("cannot encode client configuration: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return result, err
	}
	if data != nil {
		backup := path + ".cse-backup-" + time.Now().UTC().Format("20060102T150405.000000000")
		if err := os.WriteFile(backup, data, 0600); err != nil {
			return result, fmt.Errorf("could not preserve client configuration backup: %w", err)
		}
		result.BackupPath = backup
	}
	if err := writeFile(path, updated); err != nil {
		return result, err
	}
	return result, nil
}
func registeredRoot(entry map[string]any) string {
	args, _ := entry["args"].([]any)
	for i, a := range args {
		if a == "--workspace" && i+1 < len(args) {
			if p, ok := args[i+1].(string); ok {
				return p
			}
		}
	}
	if command, ok := entry["command"].(string); ok {
		if filepath.IsAbs(command) && strings.EqualFold(filepath.Base(command), "cse.exe") {
			return filepath.Dir(filepath.Dir(command))
		}
	}
	return "an unrecognized target (existing CE-SE entry)"
}
func samePath(a, b string) bool { return strings.EqualFold(filepath.Clean(a), filepath.Clean(b)) }
func writeFile(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".cse-install-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

// prepareCowork binds the packaged thin launcher to this Install. The client,
// not CE-SE, owns extension trust, installation and destination tool discovery.
func prepareCowork(root string) (Result, error) {
	source := filepath.Join(root, "engine", "CanvasExpert-SE-Cowork.mcpb")
	result := Result{Client: "cowork", State: "extension-ready", ExtensionPath: filepath.Join(root, "engine", "CanvasExpert-SE-Cowork-this-install.mcpb"), NextAction: "Open the prepared extension in Claude Desktop Settings > Extensions and accept its trust prompt. Check CE-SE tools in the actual Cowork conversation; destination support still needs a client pilot."}
	archive, err := zip.OpenReader(source)
	if err != nil {
		return result, fmt.Errorf("packaged Cowork extension unavailable: %w", err)
	}
	defer archive.Close()
	f, err := os.CreateTemp(filepath.Join(root, "engine"), ".cse-extension-*")
	if err != nil {
		return result, err
	}
	name := f.Name()
	defer os.Remove(name)
	output := zip.NewWriter(f)
	for _, entry := range archive.File {
		r, err := entry.Open()
		if err != nil {
			output.Close()
			f.Close()
			return result, err
		}
		data, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			output.Close()
			f.Close()
			return result, err
		}
		if entry.Name == "manifest.json" {
			var manifest map[string]any
			if err = json.Unmarshal(data, &manifest); err != nil {
				output.Close()
				f.Close()
				return result, err
			}
			server, ok := manifest["server"].(map[string]any)
			if !ok {
				output.Close()
				f.Close()
				return result, errors.New("packaged extension has no server")
			}
			cfg, ok := server["mcp_config"].(map[string]any)
			if !ok {
				output.Close()
				f.Close()
				return result, errors.New("packaged extension has no MCP configuration")
			}
			cfg["args"] = []string{"--install", root}
			delete(manifest, "user_config")
			data, err = json.MarshalIndent(manifest, "", "  ")
			if err != nil {
				output.Close()
				f.Close()
				return result, err
			}
		}
		w, err := output.Create(entry.Name)
		if err != nil {
			output.Close()
			f.Close()
			return result, err
		}
		if _, err = w.Write(data); err != nil {
			output.Close()
			f.Close()
			return result, err
		}
	}
	if err = output.Close(); err != nil {
		f.Close()
		return result, err
	}
	if err = f.Close(); err != nil {
		return result, err
	}
	return result, os.Rename(name, result.ExtensionPath)
}

func Check(ctx context.Context, root string) (CheckResult, error) {
	result := CheckResult{State: "local-check-failed"}
	root, exe, err := validate(root)
	if err != nil {
		return result, err
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, "--workspace", root, "mcp")
	cmd.Stderr = io.Discard
	hide(cmd)
	client := mcp.NewClient(&mcp.Implementation{Name: "cse-local-install-check", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd, TerminateDuration: time.Second}, nil)
	if err != nil {
		return result, fmt.Errorf("Engine MCP initialization failed: %w", err)
	}
	defer session.Close()
	result.EngineStarts = true
	result.AdapterVersion = session.InitializeResult().ServerInfo.Version
	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		return result, err
	}
	result.ToolCount = len(tools.Tools)
	for _, call := range []struct {
		name string
		args map[string]any
	}{{"read_workspace_file", map[string]any{"path": "AGENTS.md"}}, {"list_courses", map[string]any{"source": "configured"}}} {
		response, err := session.CallTool(ctx, &mcp.CallToolParams{Name: call.name, Arguments: call.args})
		if err != nil || response.IsError {
			return result, fmt.Errorf("local MCP %s check failed", call.name)
		}
		if call.name == "read_workspace_file" {
			if !strings.Contains(replyText(response), "install_root: "+root+"\n") {
				return result, errors.New("local instructions check returned a different Install")
			}
			result.InstructionsRead = true
		} else {
			var metadata struct {
				InstallRoot string `json:"install_root"`
			}
			if json.Unmarshal([]byte(replyText(response)), &metadata) != nil || !samePath(metadata.InstallRoot, root) {
				return result, errors.New("configured courses check returned a different Install")
			}
			result.ConfiguredCoursesRead = true
		}
	}
	versionReply, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "engine_info", Arguments: map[string]any{"operation": "version"}})
	if err != nil || versionReply.IsError {
		return result, errors.New("child Engine version check failed")
	}
	result.ChildEngineVersion = replyText(versionReply)
	result.State = "local-check-passed"
	return result, nil
}

func replyText(response *mcp.CallToolResult) string {
	b, _ := json.Marshal(response.StructuredContent)
	var out struct {
		Text string `json:"text"`
	}
	if json.Unmarshal(b, &out) == nil && out.Text != "" {
		return out.Text
	}
	for _, content := range response.Content {
		if text, ok := content.(*mcp.TextContent); ok {
			if json.Unmarshal([]byte(text.Text), &out) == nil {
				return out.Text
			}
		}
	}
	return ""
}
