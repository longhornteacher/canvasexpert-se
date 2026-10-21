package mcpserver

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	embedded "canvasexpertse"
	"canvasexpertse/internal/workspace"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestWorkspaceBoundaries(t *testing.T) {
	s := &Server{root: t.TempDir()}
	for _, path := range []string{"courses/../context/outside.md", "courses/.private/file.md", "courses/engine/file.md", ".cse/students.csv", ".cse/student-contacts.json", "engine/cse.exe"} {
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

func TestBarePlaybookNameFallsBackToPlaybooks(t *testing.T) {
	s := &Server{root: t.TempDir()}
	if err := os.MkdirAll(filepath.Join(s.root, "playbooks"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.root, "playbooks", "group-work.md"), []byte("playbook"), 0600); err != nil {
		t.Fatal(err)
	}
	p, err := s.readablePath("group-work.md")
	if err != nil || filepath.Base(filepath.Dir(p)) != "playbooks" {
		t.Fatalf("bare playbook name not resolved: %q %v", p, err)
	}
	if err := os.WriteFile(filepath.Join(s.root, "group-work.md"), []byte("root"), 0600); err != nil {
		t.Fatal(err)
	}
	if p, _ = s.readablePath("group-work.md"); filepath.Base(filepath.Dir(p)) == "playbooks" {
		t.Fatalf("root file should win over the playbooks fallback: %q", p)
	}
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

func TestSharedRulesRouteAcrossInstallRoots(t *testing.T) {
	installA, installB, syncRoot := t.TempDir(), t.TempDir(), t.TempDir()
	for _, root := range []string{installA, installB} {
		if err := os.MkdirAll(filepath.Join(root, "context"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(root, "courses"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "context", "courses.md"), []byte("local course config"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(installA, "context", "grading.md"), []byte("custom grading voice"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.SetRulesLocation(installA, syncRoot, embedded.WorkspaceTemplate, "workspace-template"); err != nil {
		t.Fatal(err)
	}
	// PC B sees the existing authority as-is and keeps its different local copy inactive.
	if err := os.WriteFile(filepath.Join(installB, "context", "grading.md"), []byte("different local copy"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.SetRulesLocation(installB, syncRoot, embedded.WorkspaceTemplate, "workspace-template"); err != nil {
		t.Fatal(err)
	}
	a, b := &Server{root: installA}, &Server{root: installB}
	pa, err := a.readablePath("context/grading.md")
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(pa)
	if err != nil || string(content) != "custom grading voice" {
		t.Fatalf("initial shared content %q, %v", content, err)
	}
	oldHash := fmt.Sprintf("%x", sha256.Sum256(content))
	if _, err := a.writeWorkspaceBytes(pa, []byte("updated through PC A"), oldHash); err != nil {
		t.Fatal(err)
	}
	pb, err := b.readablePath("context/grading.md")
	if err != nil {
		t.Fatal(err)
	}
	content, err = os.ReadFile(pb)
	if err != nil || string(content) != "updated through PC A" {
		t.Fatalf("PC B stale read %q, %v", content, err)
	}
	if _, err := b.writeWorkspaceBytes(pb, []byte("stale overwrite"), oldHash); err == nil {
		t.Fatal("stale hash was accepted")
	}
	localCourses, err := b.readablePath("context/courses.md")
	if err != nil {
		t.Fatal(err)
	}
	content, err = os.ReadFile(localCourses)
	if err != nil || string(content) != "local course config" {
		t.Fatalf("course config left UserInstall: %q, %v", content, err)
	}
	if _, err := b.readablePath(".cse/rules-location.json"); err == nil {
		t.Fatal("private binding became readable")
	}
	listed, err := b.listWorkspaceFiles(ListFilesInput{Path: "context", Recursive: true})
	if err != nil {
		t.Fatal(err)
	}
	if count := strings.Count(listed.Text, "context/grading.md"); count != 1 {
		t.Fatalf("shared path listed %d times: %s", count, listed.Text)
	}
}

func TestMCPSharedRulesRefreshesAcrossConnectedClients(t *testing.T) {
	ctx := context.Background()
	installA, installB, syncRoot := t.TempDir(), t.TempDir(), t.TempDir()
	for _, root := range []string{installA, installB} {
		if err := os.MkdirAll(filepath.Join(root, "context"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(root, "courses"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "context", "courses.md"), []byte("local"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(installA, "context", "grading.md"), []byte("initial shared voice"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.SetRulesLocation(installA, syncRoot, embedded.WorkspaceTemplate, "workspace-template"); err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.SetRulesLocation(installB, syncRoot, embedded.WorkspaceTemplate, "workspace-template"); err != nil {
		t.Fatal(err)
	}
	connect := func(root string) (*mcp.ClientSession, func()) {
		t.Helper()
		server, closeServer := NewServer(ctx, root)
		client := mcp.NewClient(&mcp.Implementation{Name: "client", Version: "test"}, nil)
		st, ct := mcp.NewInMemoryTransports()
		ss, err := server.Connect(ctx, st, nil)
		if err != nil {
			t.Fatal(err)
		}
		cs, err := client.Connect(ctx, ct, nil)
		if err != nil {
			t.Fatal(err)
		}
		return cs, func() { _ = cs.Close(); _ = ss.Close(); closeServer() }
	}
	a, closeA := connect(installA)
	defer closeA()
	b, closeB := connect(installB)
	defer closeB()
	if _, err := a.ListTools(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := b.ListTools(ctx, nil); err != nil {
		t.Fatal(err)
	}
	call := func(client *mcp.ClientSession, name string, args map[string]any, wantError bool) string {
		t.Helper()
		result, err := client.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatal(err)
		}
		if result.IsError != wantError {
			t.Fatalf("%s isError=%v: %v", name, result.IsError, result.Content)
		}
		if wantError {
			return ""
		}
		text := result.Content[0].(*mcp.TextContent).Text
		var structured map[string]any
		if json.Unmarshal([]byte(text), &structured) == nil {
			if value, ok := structured["text"].(string); ok {
				return value
			}
		}
		return text
	}
	readA := call(a, "read_workspace_file", map[string]any{"path": "context/grading.md"}, false)
	if !strings.Contains(readA, "authority_root: "+syncRoot) || !strings.Contains(readA, "initial shared voice") {
		t.Fatalf("unexpected authoritative read: %s", readA)
	}
	oldHash := fmt.Sprintf("%x", sha256.Sum256([]byte("initial shared voice")))
	call(a, "write_workspace_file", map[string]any{"path": "context/grading.md", "content": "changed from connected client A", "expected_sha256": oldHash}, false)
	readB := call(b, "read_workspace_file", map[string]any{"path": "context/grading.md"}, false)
	if !strings.Contains(readB, "changed from connected client A") {
		t.Fatalf("client B did not observe current rules: %s", readB)
	}
	call(b, "write_workspace_file", map[string]any{"path": "context/grading.md", "content": "stale", "expected_sha256": oldHash}, true)
	listing := call(b, "list_workspace_files", map[string]any{"path": "context", "recursive": true}, false)
	var listed struct {
		Entries []struct {
			Path string `json:"path"`
		} `json:"entries"`
		InstallRoot string `json:"install_root"`
		RulesRoot   string `json:"rules_root"`
		Mode        string `json:"mode"`
	}
	if err := json.Unmarshal([]byte(listing), &listed); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, entry := range listed.Entries {
		if entry.Path == "context/grading.md" {
			count++
		}
	}
	if count != 1 || listed.InstallRoot != installB || listed.RulesRoot != syncRoot || listed.Mode != "sync" {
		t.Fatalf("listing authority/contents incorrect: %s", listing)
	}
	playbooks := call(b, "list_workspace_files", map[string]any{"path": "playbooks/", "recursive": true}, false)
	if !strings.Contains(playbooks, "playbooks/setup.md") {
		t.Fatalf("explicit shared playbook listing failed without a local playbooks folder: %s", playbooks)
	}
	call(b, "list_workspace_files", map[string]any{"path": "/playbooks"}, true)
	contextSlash := call(b, "list_workspace_files", map[string]any{"path": "context\\", "recursive": true}, false)
	if !strings.Contains(contextSlash, "context/grading.md") {
		t.Fatalf("context directory with trailing separator omitted shared rules: %s", contextSlash)
	}
	if err := os.Rename(syncRoot, syncRoot+".offline"); err != nil {
		t.Fatal(err)
	}
	call(b, "list_workspace_files", map[string]any{"path": "playbooks/"}, true)
	fullListing := call(b, "list_workspace_files", map[string]any{}, false)
	if !strings.Contains(fullListing, "availability_error") || strings.Contains(fullListing, "playbooks/setup.md") {
		t.Fatalf("missing UserSync listing fell back to inactive local rules: %s", fullListing)
	}
	if err := os.Rename(syncRoot+".offline", syncRoot); err != nil {
		t.Fatal(err)
	}
	call(b, "read_workspace_file", map[string]any{"path": ".cse/rules-location.json"}, true)
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
