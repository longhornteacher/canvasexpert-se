package mcpserver

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"canvasexpertse/internal/playbooks"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestEngineInfoPlaybookStatusAndReplacement(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	p := filepath.Join(root, "playbooks", "setup.md")
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("teacher version"), 0644); err != nil {
		t.Fatal(err)
	}
	s := &Server{root: root}
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	s.registerOperationTools(srv)
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
	st, ct := mcp.NewInMemoryTransports()
	ss, err := srv.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	call := func(args map[string]any, wantError bool) string {
		t.Helper()
		r, e := cs.CallTool(ctx, &mcp.CallToolParams{Name: "engine_info", Arguments: args})
		if e != nil {
			t.Fatal(e)
		}
		if r.IsError != wantError {
			t.Fatalf("error=%v content=%v", r.IsError, r.Content)
		}
		if len(r.Content) == 0 {
			t.Fatal("empty result")
		}
		return r.Content[0].(*mcp.TextContent).Text
	}
	text := call(map[string]any{"operation": "playbooks-status"}, false)
	var wrapped struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(text), &wrapped); err != nil {
		t.Fatal(err)
	}
	var status struct {
		Files []playbooks.StatusRow `json:"files"`
	}
	if err := json.Unmarshal([]byte(wrapped.Text), &status); err != nil {
		t.Fatal(err)
	}
	var hash string
	for _, row := range status.Files {
		if row.Path == "playbooks/setup.md" {
			if row.State != "differs" || !row.Replaceable {
				t.Fatalf("unexpected status: %+v", row)
			}
			hash = row.SHA256
		}
	}
	if hash == "" {
		t.Fatalf("status omitted current hash: %s", text)
	}
	call(map[string]any{"operation": "replace-playbook", "path": "playbooks/setup.md", "expected_sha256": "stale"}, true)
	if b, _ := os.ReadFile(p); string(b) != "teacher version" {
		t.Fatal("stale hash changed active file")
	}
	call(map[string]any{"operation": "replace-playbook", "path": "playbooks/setup.md", "expected_sha256": hash}, false)
	seed, _ := playbooks.SeedBytes("playbooks/setup.md")
	if b, _ := os.ReadFile(p); string(b) != string(seed) {
		t.Fatal("MCP replacement did not install seed")
	}
	old, _ := filepath.Glob(filepath.Join(root, "playbook-old", "*", "playbooks", "setup.md"))
	if len(old) != 1 {
		t.Fatalf("active prior copy was not archived: %v", old)
	}
	if b, _ := os.ReadFile(old[0]); !strings.Contains(string(b), "teacher version") {
		t.Fatal("wrong archive content")
	}
}
