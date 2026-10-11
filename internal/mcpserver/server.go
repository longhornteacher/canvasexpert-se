package mcpserver

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"canvasexpertse/internal/workspace"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const maxWorkspaceFile = 1 << 20

type Server struct {
	root    string
	ctx     context.Context
	mu      sync.Mutex
	reviews map[string]*reviewTask
}

type reviewTask struct {
	id     string
	url    string
	done   chan struct{}
	mu     sync.Mutex
	stdout strings.Builder
	stderr strings.Builder
	err    error
}

type TextOutput struct {
	Text string `json:"text" jsonschema:"Result text"`
}

type ReadFileInput struct {
	Path string `json:"path" jsonschema:"Workspace-relative path; .cse and hidden folders are not accessible"`
}

type WriteFileInput struct {
	Path           string `json:"path" jsonschema:"Workspace-relative draft path under courses/"`
	Content        string `json:"content" jsonschema:"Complete UTF-8 file contents"`
	ExpectedSHA256 string `json:"expected_sha256,omitempty" jsonschema:"Required to replace an existing file; use the hash returned by read_workspace_file"`
}

type PullWorkInput struct {
	Course     string `json:"course,omitempty" jsonschema:"Configured course nickname; omit to pull current courses"`
	Assignment string `json:"assignment,omitempty" jsonschema:"Optional assignment title or ID"`
}

type CourseQueryInput struct {
	Course string `json:"course" jsonschema:"Configured course nickname"`
	Query  string `json:"query" jsonschema:"Assignment title or ID, or Page ID or Canvas link"`
}

type CourseInput struct {
	Course string `json:"course" jsonschema:"Configured course nickname"`
}

type PathInput struct {
	Path string `json:"path" jsonschema:"Workspace-relative document path"`
}

type HelpInput struct {
	Topic string `json:"topic,omitempty" jsonschema:"Optional reference topic: formats, fields, or examples"`
}

type ReviewInput struct {
	Path   string `json:"path" jsonschema:"Workspace-relative results, assignment, or QuizForge file"`
	Course string `json:"course,omitempty" jsonschema:"Required for a QuizForge quiz review"`
}

type ReviewStatusInput struct {
	ReviewID string `json:"review_id" jsonschema:"ID returned by start_review"`
}

func Run(ctx context.Context, root string) error {
	s := &Server{root: root, ctx: ctx, reviews: map[string]*reviewTask{}}
	server := mcp.NewServer(&mcp.Implementation{Name: "canvasexpert-se", Version: "0.1b"}, nil)
	addReadTool(server, "list_courses", "List the teacher's configured Canvas courses by nickname and current IDs.", func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (TextOutput, error) {
		return s.command(ctx, "courses")
	})
	addReadTool(server, "engine_reference", "Read CE-SE command help, document formats, Canvas field limits, or synthetic examples.", func(ctx context.Context, _ *mcp.CallToolRequest, in HelpInput) (TextOutput, error) {
		if in.Topic == "" {
			return s.command(ctx, "help")
		}
		if in.Topic != "formats" && in.Topic != "fields" && in.Topic != "examples" {
			return TextOutput{}, errors.New("topic must be formats, fields, or examples")
		}
		return s.command(ctx, "help", in.Topic)
	})
	addActionTool(server, "pull_work", "Pull current Canvas work using the CE-SE Engine. Student identity is pseudonymized by the Engine; never access .cse.", false, func(ctx context.Context, _ *mcp.CallToolRequest, in PullWorkInput) (TextOutput, error) {
		args := []string{"pull-work"}
		if in.Course != "" {
			args = append(args, in.Course)
		}
		if in.Assignment != "" {
			if in.Course == "" {
				return TextOutput{}, errors.New("course is required when assignment is specified")
			}
			args = append(args, in.Assignment)
		}
		return s.command(ctx, args...)
	})
	addReadTool(server, "list_modules", "List a configured course's Canvas modules through the CE-SE Engine.", func(ctx context.Context, _ *mcp.CallToolRequest, in CourseInput) (TextOutput, error) {
		return s.command(ctx, "modules", in.Course)
	})
	addActionTool(server, "pull_assignment", "Pull an exact Canvas assignment and its edit baseline through the CE-SE Engine.", false, func(ctx context.Context, _ *mcp.CallToolRequest, in CourseQueryInput) (TextOutput, error) {
		return s.command(ctx, "pull-item", in.Course, in.Query)
	})
	addActionTool(server, "pull_page", "Pull an exact Canvas Page and its edit baseline through the CE-SE Engine.", false, func(ctx context.Context, _ *mcp.CallToolRequest, in CourseQueryInput) (TextOutput, error) {
		return s.command(ctx, "pull-page", in.Course, in.Query)
	})
	addReadTool(server, "read_workspace_file", "Read a workspace instruction, playbook, context file, or course work file. Hidden folders, .cse, and Engine files are blocked.", func(_ context.Context, _ *mcp.CallToolRequest, in ReadFileInput) (TextOutput, error) {
		path, err := s.readablePath(in.Path)
		if err != nil {
			return TextOutput{}, err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return TextOutput{}, err
		}
		if len(b) > maxWorkspaceFile {
			return TextOutput{}, fmt.Errorf("file exceeds the %d-byte read limit", maxWorkspaceFile)
		}
		h := sha256.Sum256(b)
		return TextOutput{Text: fmt.Sprintf("path: %s\nsha256: %x\n\n%s", filepath.ToSlash(in.Path), h, string(b))}, nil
	})
	addActionTool(server, "write_draft", "Create or update a draft under courses/. Existing files require their current SHA-256 to prevent overwriting newer unsent work. This tool never writes to Canvas.", true, func(_ context.Context, _ *mcp.CallToolRequest, in WriteFileInput) (TextOutput, error) {
		return s.writeDraft(in)
	})
	addReadTool(server, "validate_document", "Validate a CE-SE workspace document without writing to Canvas.", func(ctx context.Context, _ *mcp.CallToolRequest, in PathInput) (TextOutput, error) {
		if _, err := s.coursePath(in.Path); err != nil {
			return TextOutput{}, err
		}
		return s.command(ctx, "validate", in.Path)
	})
	addActionTool(server, "start_review", "Start the CE-SE local review page for a draft. Returns its loopback URL. Canvas is not changed until the teacher approves on that page. Call review_status afterward to receive the outcome or revision notes.", true, func(ctx context.Context, _ *mcp.CallToolRequest, in ReviewInput) (TextOutput, error) {
		return s.startReview(ctx, in)
	})
	addReadTool(server, "review_status", "Check whether the teacher finished a CE-SE review; returns approval/post outcome or teacher revision notes.", func(_ context.Context, _ *mcp.CallToolRequest, in ReviewStatusInput) (TextOutput, error) {
		return s.reviewStatus(in.ReviewID)
	})
	return server.Run(ctx, &mcp.StdioTransport{})
}

func addReadTool[In any](server *mcp.Server, name, description string, fn func(context.Context, *mcp.CallToolRequest, In) (TextOutput, error)) {
	addTool(server, name, description, mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true}, fn)
}

func addActionTool[In any](server *mcp.Server, name, description string, destructive bool, fn func(context.Context, *mcp.CallToolRequest, In) (TextOutput, error)) {
	addTool(server, name, description, mcp.ToolAnnotations{DestructiveHint: &destructive, OpenWorldHint: boolPointer(name == "start_review" || name == "pull_work" || name == "pull_assignment" || name == "pull_page"), ReadOnlyHint: false}, fn)
}

func boolPointer(v bool) *bool { return &v }

func addTool[In any](server *mcp.Server, name, description string, annotations mcp.ToolAnnotations, fn func(context.Context, *mcp.CallToolRequest, In) (TextOutput, error)) {
	mcp.AddTool(server, &mcp.Tool{Name: name, Description: description, Annotations: &annotations}, func(ctx context.Context, req *mcp.CallToolRequest, in In) (*mcp.CallToolResult, TextOutput, error) {
		out, err := fn(ctx, req, in)
		return nil, out, err
	})
}

func (s *Server) command(ctx context.Context, args ...string) (TextOutput, error) {
	exe, err := os.Executable()
	if err != nil {
		return TextOutput{}, err
	}
	full := append([]string{"--workspace", s.root}, args...)
	cmd := exec.CommandContext(ctx, exe, full...)
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err = cmd.Run()
	result := strings.TrimSpace(stdout.String())
	if stderr.Len() > 0 {
		if result != "" {
			result += "\n"
		}
		result += strings.TrimSpace(stderr.String())
	}
	if err != nil {
		return TextOutput{}, fmt.Errorf("CE-SE command failed: %w\n%s", err, result)
	}
	return TextOutput{Text: result}, nil
}

func (s *Server) safePath(relative string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(strings.ReplaceAll(relative, "\\", "/")))
	if clean == "." || filepath.IsAbs(clean) {
		return "", errors.New("expected a workspace-relative path")
	}
	for _, part := range strings.Split(filepath.ToSlash(clean), "/") {
		if strings.HasPrefix(part, ".") {
			return "", errors.New("hidden workspace paths are not accessible")
		}
		if strings.EqualFold(part, "engine") {
			return "", errors.New("Engine files are not accessible through workspace file tools")
		}
	}
	return workspace.ConstrainPath(s.root, clean)
}

func (s *Server) readablePath(relative string) (string, error) {
	path, err := s.safePath(relative)
	if err != nil {
		return "", err
	}
	first := strings.Split(filepath.ToSlash(filepath.Clean(filepath.FromSlash(relative))), "/")[0]
	if !strings.EqualFold(first, "AGENTS.md") && !strings.EqualFold(first, "CLAUDE.md") && !strings.EqualFold(first, "context") && !strings.EqualFold(first, "playbooks") && !strings.EqualFold(first, "courses") {
		return "", errors.New("read access is limited to AGENTS.md, CLAUDE.md, context/, playbooks/, and courses/")
	}
	return path, nil
}

func (s *Server) coursePath(relative string) (string, error) {
	path, err := s.safePath(relative)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(strings.ToLower(filepath.ToSlash(relative)), "courses/") {
		return "", errors.New("document path must be under courses/")
	}
	return path, nil
}

func (s *Server) writeDraft(in WriteFileInput) (TextOutput, error) {
	path, err := s.coursePath(in.Path)
	if err != nil {
		return TextOutput{}, err
	}
	if len(in.Content) > maxWorkspaceFile {
		return TextOutput{}, fmt.Errorf("draft exceeds the %d-byte write limit", maxWorkspaceFile)
	}
	if !strings.EqualFold(filepath.Ext(path), ".md") && !strings.EqualFold(filepath.Ext(path), ".yaml") && !strings.EqualFold(filepath.Ext(path), ".yml") {
		return TextOutput{}, errors.New("drafts must use .md, .yaml, or .yml files")
	}
	if existing, readErr := os.ReadFile(path); readErr == nil {
		if in.ExpectedSHA256 == "" {
			return TextOutput{}, errors.New("file already exists; read it and provide its sha256 to replace it")
		}
		h := sha256.Sum256(existing)
		if !strings.EqualFold(in.ExpectedSHA256, hex.EncodeToString(h[:])) {
			return TextOutput{}, errors.New("file changed since it was read; reread it before editing")
		}
	} else if !os.IsNotExist(readErr) {
		return TextOutput{}, readErr
	} else if in.ExpectedSHA256 != "" {
		return TextOutput{}, errors.New("file no longer exists; omit expected_sha256 to create it again")
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return TextOutput{}, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".cse-draft-*")
	if err != nil {
		return TextOutput{}, err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err = tmp.Chmod(0600); err == nil {
		_, err = io.WriteString(tmp, in.Content)
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return TextOutput{}, err
	}
	if err = os.Rename(tmpName, path); err != nil {
		return TextOutput{}, err
	}
	h := sha256.Sum256([]byte(in.Content))
	return TextOutput{Text: fmt.Sprintf("Draft saved: %s\nsha256: %x", filepath.ToSlash(in.Path), h)}, nil
}

func (s *Server) startReview(ctx context.Context, in ReviewInput) (TextOutput, error) {
	path, err := s.coursePath(in.Path)
	if err != nil {
		return TextOutput{}, err
	}
	args := []string{"--workspace", s.root, "review", path}
	if in.Course != "" {
		args = append(args, in.Course)
	}
	exe, err := os.Executable()
	if err != nil {
		return TextOutput{}, err
	}
	cmd := exec.Command(exe, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return TextOutput{}, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return TextOutput{}, err
	}
	var nonce [16]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		return TextOutput{}, err
	}
	task := &reviewTask{id: hex.EncodeToString(nonce[:]), done: make(chan struct{})}
	s.mu.Lock()
	for _, current := range s.reviews {
		select {
		case <-current.done:
		default:
			s.mu.Unlock()
			return TextOutput{}, errors.New("a CE-SE review is already running; check it with review_status")
		}
	}
	if err = cmd.Start(); err != nil {
		s.mu.Unlock()
		return TextOutput{}, err
	}
	s.reviews[task.id] = task
	s.mu.Unlock()
	stdoutDone := make(chan struct{})
	go func() { defer close(stdoutDone); _, _ = io.Copy(&task.stdout, stdout) }()
	urlReady := make(chan string, 1)
	stderrDone := make(chan struct{})
	go func() {
		defer close(stderrDone)
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			task.mu.Lock()
			task.stderr.WriteString(line + "\n")
			task.mu.Unlock()
			const prefix = "Open the local review page: "
			if strings.HasPrefix(line, prefix) {
				select {
				case urlReady <- strings.TrimSpace(strings.TrimPrefix(line, prefix)):
				default:
				}
			}
		}
	}()
	go func() {
		cmdErr := cmd.Wait()
		<-stdoutDone
		<-stderrDone
		task.mu.Lock()
		task.err = cmdErr
		task.mu.Unlock()
		close(task.done)
	}()
	go func() {
		select {
		case <-s.ctx.Done():
			_ = cmd.Process.Kill()
		case <-task.done:
		}
	}()
	select {
	case raw := <-urlReady:
		task.mu.Lock()
		task.url = raw
		task.mu.Unlock()
		return TextOutput{Text: fmt.Sprintf("Review is ready. Ask the teacher to open %s. No Canvas changes occur unless they approve on the page. review_id: %s", raw, task.id)}, nil
	case <-task.done:
		if task.err != nil {
			return TextOutput{}, fmt.Errorf("review exited before the page became ready: %w\n%s", task.err, task.output())
		}
		return TextOutput{Text: "Review exited before its page became ready. " + task.output()}, nil
	case <-ctx.Done():
		return TextOutput{Text: "Review started; check review_status for review_id " + task.id}, nil
	case <-time.After(30 * time.Second):
		return TextOutput{Text: "Review is still starting; check review_status for review_id " + task.id}, nil
	}
}

func (t *reviewTask) output() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return strings.TrimSpace(t.stdout.String() + "\n" + t.stderr.String())
}

func (s *Server) reviewStatus(id string) (TextOutput, error) {
	s.mu.Lock()
	task := s.reviews[id]
	s.mu.Unlock()
	if task == nil {
		return TextOutput{}, errors.New("unknown review_id")
	}
	select {
	case <-task.done:
		if task.err != nil {
			return TextOutput{}, fmt.Errorf("review command failed: %w\n%s", task.err, task.output())
		}
		return TextOutput{Text: "Review finished.\n" + task.output()}, nil
	default:
		task.mu.Lock()
		url := task.url
		task.mu.Unlock()
		return TextOutput{Text: fmt.Sprintf("Review is waiting for the teacher at %s. review_id: %s", url, task.id)}, nil
	}
}
