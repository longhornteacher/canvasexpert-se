package mcpserver

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// installFixture makes a UserInstall with an engine manifest, with or without .git.
func installFixture(t *testing.T, git bool) string {
	t.Helper()
	root := t.TempDir()
	for name, body := range map[string]string{
		"engine/manifest.txt": "version=1.0.0\nidentity=1.0.0\nbase_url=https://example.test/dl\nsha256.cse.exe=AB\n",
		"playbooks/score.md":  "shipped",
		"AGENTS.md":           "shipped",
		"context/grading.md":  "teacher",
	} {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if git {
		if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func writeCall(t *testing.T, root, path string) *mcp.CallToolResult {
	t.Helper()
	ctx := context.Background()
	s := &Server{root: root}
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	s.registerWorkspaceTools(server)
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
	st, ct := mcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	result, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "write_workspace_file", Arguments: map[string]any{"path": path, "content": "new text"}})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestGitInstallRefusesWritesToGitOwnedFiles(t *testing.T) {
	root := installFixture(t, true)
	for _, p := range []string{"playbooks/score.md", "AGENTS.md", "playbooks/extra/new.md", "agents.md"} {
		r := writeCall(t, root, p)
		if !r.IsError || !strings.Contains(r.Content[0].(*mcp.TextContent).Text, "Teacher_Override.md") {
			t.Errorf("%s was not refused with guidance: %v", p, r.Content)
		}
	}
	// Teacher-owned paths stay writable in a git install.
	os.WriteFile(filepath.Join(root, "Teacher_Override.md"), []byte("old"), 0o644)
	for _, p := range []string{"context/grading.md", "My Own Guide.txt"} {
		if r := writeCall(t, root, p); r.IsError && p == "My Own Guide.txt" {
			t.Errorf("teacher-created root guide refused: %v", r.Content)
		}
	}
	if r := writeCall(t, root, "context/new-note.md"); r.IsError {
		t.Errorf("context write refused: %v", r.Content)
	}
	if b, _ := os.ReadFile(filepath.Join(root, "playbooks", "score.md")); string(b) != "shipped" {
		t.Errorf("refused write still changed the file: %q", b)
	}
}

func TestZipInstallStillAllowsPlaybookWrites(t *testing.T) {
	root := installFixture(t, false)
	if r := writeCall(t, root, "playbooks/new.md"); r.IsError {
		t.Fatalf("ZIP install playbook write refused: %v", r.Content)
	}
}
