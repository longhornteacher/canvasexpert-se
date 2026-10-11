package mcpserver

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestWorkspaceBoundaries(t *testing.T) {
	s := &Server{root: t.TempDir()}
	for _, path := range []string{"courses/../context/outside.md", "courses/.private/file.md", "courses/engine/file.md", ".cse/students.csv", "engine/cse.exe"} {
		if _, err := s.readablePath(path); err == nil {
			t.Errorf("readablePath accepted %q", path)
		}
		if _, err := s.writeDraft(WriteFileInput{Path: path, Content: "draft"}); err == nil {
			t.Errorf("writeDraft accepted %q", path)
		}
	}
	if _, err := s.coursePath("courses\\example\\draft.md"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.writeDraft(WriteFileInput{Path: "context/outside.md", Content: "draft"}); err == nil {
		t.Fatal("accepted context draft")
	}
}

func TestQuizForgeDraftFormats(t *testing.T) {
	s := &Server{root: t.TempDir()}
	for _, ext := range []string{"json", "txt"} {
		if _, err := s.writeDraft(WriteFileInput{Path: "courses/test/quiz." + ext, Content: "synthetic quiz"}); err != nil {
			t.Fatalf("%s draft: %v", ext, err)
		}
	}
}

func TestWorkspaceToolsRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := &Server{root: t.TempDir()}
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
	tools, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools.Tools) != 4 {
		t.Fatalf("got %d tools", len(tools.Tools))
	}
	call := func(name string, args map[string]any, wantError bool) *mcp.CallToolResult {
		t.Helper()
		result, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatal(err)
		}
		if result.IsError != wantError {
			t.Fatalf("%s isError=%v content=%v", name, result.IsError, result.Content)
		}
		return result
	}
	call("write_workspace_file", map[string]any{"path": "context/teacher.md", "content": "teacher instructions"}, false)
	call("write_workspace_file", map[string]any{"path": "Creation Taste.txt", "content": "teacher preferences"}, false)
	call("write_workspace_file", map[string]any{"path": "Creation Taste.txt", "content": "unguarded replacement"}, true)
	call("write_workspace_file", map[string]any{"path": "Creation Taste.txt", "content": "updated preferences", "expected_sha256": fmt.Sprintf("%x", sha256.Sum256([]byte("teacher preferences")))}, false)
	call("write_workspace_file", map[string]any{"path": "script.ps1", "content": "script"}, true)
	call("write_workspace_file", map[string]any{"path": "context/teacher.md", "content": "unsolicited replacement"}, true)
	call("write_workspace_file", map[string]any{"path": "courses/test/draft.md", "content": "wrong writer"}, true)
	b := []byte{0, 1, 2, 254, 255}
	encoded := base64.StdEncoding.EncodeToString(b)
	call("write_asset", map[string]any{"path": "courses/test/assets/example.bin", "base64": encoded}, false)
	out := call("read_workspace_asset", map[string]any{"path": "courses/test/assets/example.bin"}, false)
	text := out.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, encoded) || !strings.Contains(text, fmt.Sprintf("%x", sha256.Sum256(b))) {
		t.Fatalf("asset round trip failed: %s", text)
	}
	call("write_asset", map[string]any{"path": "courses/test/assets/bad.bin", "base64": "invalid"}, true)
	call("read_workspace_asset", map[string]any{"path": ".cse/private.bin"}, true)
}

func TestWorkspaceRootReferences(t *testing.T) {
	s := &Server{root: t.TempDir()}
	refs := []string{"README.txt", "Creation Taste.txt", "CS8 WeeklyPractice_BuildGuide.md"}
	for _, name := range append(refs, "script.ps1", "program.exe", ".private.md") {
		if err := os.WriteFile(filepath.Join(s.root, name), []byte("synthetic teacher reference"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(s.root, "fake.txt"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range refs {
		p, err := s.readablePath(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		b, err := readLimited(p, maxWorkspaceFile)
		if err != nil || string(b) != "synthetic teacher reference" {
			t.Fatalf("reference read %s: %q %v", name, b, err)
		}
	}
	out, err := s.listWorkspaceFiles(ListFilesInput{})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range refs {
		if !strings.Contains(out.Text, name) {
			t.Errorf("root discovery omitted %s: %s", name, out.Text)
		}
	}
	for _, name := range []string{"script.ps1", "program.exe", ".private.md", "fake.txt", "README.txt/child.md"} {
		if _, err := s.readablePath(name); err == nil {
			t.Errorf("accepted %s", name)
		}
		if strings.Contains(out.Text, name) {
			t.Errorf("listed %s: %s", name, out.Text)
		}
	}
}
func TestWorkspaceLinkPrivacy(t *testing.T) {
	root := t.TempDir()
	s := &Server{root: root}
	if err := os.MkdirAll(filepath.Join(root, "courses"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{filepath.Join(root, ".cse"), filepath.Join(root, "engine"), t.TempDir()} {
		if err := os.MkdirAll(target, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(target, "private.md"), []byte("synthetic private"), 0600); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(root, "courses", "alias")
		var err error
		if runtime.GOOS == "windows" {
			err = exec.Command("cmd", "/c", "mklink", "/J", link, target).Run()
		} else {
			err = os.Symlink(target, link)
		}
		if err != nil {
			t.Fatal(err)
		}
		if _, err = s.readablePath("courses/alias/private.md"); err == nil {
			t.Fatal("accepted private linked read")
		}
		if _, err = s.writeDraft(WriteFileInput{Path: "courses/alias/new.md", Content: "draft"}); err == nil {
			t.Fatal("accepted linked write")
		}
		out, err := s.listWorkspaceFiles(ListFilesInput{Path: "courses", Recursive: true})
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out.Text, "alias") {
			t.Fatalf("listed private link: %s", out.Text)
		}
		if err = os.Remove(link); err != nil {
			t.Fatal(err)
		}
	}
}
func TestWorkspaceDraftConcurrentReplacement(t *testing.T) {
	s := &Server{root: t.TempDir()}
	path := "courses/test/draft.md"
	if _, err := s.writeDraft(WriteFileInput{Path: path, Content: "original"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.writeDraft(WriteFileInput{Path: path, Content: "overwrite"}); err == nil {
		t.Fatal("missing hash accepted")
	}
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte("original")))
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, content := range []string{"one", "two"} {
		wg.Add(1)
		go func(content string) {
			defer wg.Done()
			<-start
			_, err := s.writeDraft(WriteFileInput{Path: path, Content: content, ExpectedSHA256: hash})
			results <- err
		}(content)
	}
	close(start)
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("want one successful replacement, got %d", success)
	}
}
func TestWorkspaceDiscovery(t *testing.T) {
	root := t.TempDir()
	s := &Server{root: root}
	for _, path := range []string{"AGENTS.md", "context/courses.md", "playbooks/scoring.md", "courses/test/draft.md", "courses/.secret/private.md", ".cse/private.md", "engine/cse.md"} {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	out, err := s.listWorkspaceFiles(ListFilesInput{Recursive: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"AGENTS.md", "context/courses.md", "playbooks/scoring.md", "courses/test/draft.md"} {
		if !strings.Contains(out.Text, path) {
			t.Errorf("missing %s in %s", path, out.Text)
		}
	}
	for _, path := range []string{".secret", ".cse", "engine"} {
		if strings.Contains(out.Text, path) {
			t.Errorf("private path listed: %s", out.Text)
		}
	}
	if _, err := s.listWorkspaceFiles(ListFilesInput{Path: ".cse"}); err == nil {
		t.Fatal("accepted private listing")
	}
}
