package mcpserver

import (
	"context"
	"crypto/sha256"
	"encoding/json"

	"errors"
	"fmt"

	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	embedded "canvasexpertse"
	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const maxWorkspaceFile = 1 << 20

var commandTimeout = 15 * time.Minute

type Server struct {
	root              string
	ctx               context.Context
	mu                sync.Mutex
	reviews           map[string]*reviewTask
	sessions          map[string]*reviewTask
	newSessionCommand func([]string) *exec.Cmd
	sessionReadyWait  time.Duration
}

type TextOutput struct {
	Text                 string           `json:"text" jsonschema:"Result text"`
	State                string           `json:"state,omitempty" jsonschema:"Local session state: pending, finished, or failed"`
	ReviewID             string           `json:"review_id,omitempty"`
	SessionID            string           `json:"session_id,omitempty"`
	URL                  string           `json:"url,omitempty" jsonschema:"Local page URL; returned only while the session is pending"`
	Outcome              string           `json:"outcome,omitempty" jsonschema:"Review write outcome: completed, partial, or held"`
	ResultPath           string           `json:"result_path,omitempty"`
	RevisionNotes        []ReviewRevision `json:"revision_notes,omitempty"`
	SetupStatusAvailable bool             `json:"setup_status_available,omitempty"`
	CredentialSaved      *bool            `json:"credential_saved,omitempty"`
	ValidationSucceeded  *bool            `json:"validation_succeeded,omitempty"`
	ConnectionTested     *bool            `json:"connection_tested,omitempty"`
	SetupOutcome         string           `json:"setup_outcome,omitempty"`
}

type ReviewRevision struct {
	Ref  string `json:"ref"`
	Note string `json:"note"`
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
	server, close := NewServer(ctx, root)
	defer close()
	return server.Run(ctx, &mcp.StdioTransport{})
}

// NewServer creates the same adapter for stdio and in-process transports.
// Close stops any child review or private UI sessions.
func NewServer(ctx context.Context, root string) (*mcp.Server, func()) {
	ctx, cancel := context.WithCancel(ctx)
	if _, err := workspace.Scaffold(root, embedded.WorkspaceTemplate, "workspace-template"); err != nil {
		fmt.Fprintf(os.Stderr, "Could not add some CE-SE starter files: %v\n", err)
	}
	s := &Server{root: root, ctx: ctx, reviews: map[string]*reviewTask{}}
	server := mcp.NewServer(&mcp.Implementation{Name: "canvasexpert-se", Version: AdapterVersion()}, &mcp.ServerOptions{Instructions: "Before any teacher task, read AGENTS.md using read_workspace_file, then read the relevant playbook. Commands, courses, student-specific context and work remain in Install. Teacher instructions, playbooks, guides and preferences use Install by default or the explicitly configured Sync authority; Sync is a provider-synchronized local directory, and reads always use its current contents. Check engine_info operation rules-location if the active authority is unclear. Read list_courses with its default configured source for saved nicknames and current selections; request source canvas only for connection testing or additional course discovery. Treat the structured courses list as setup status and ignore obsolete starter text in a preserved document body. Use engine_reference for Engine commands and file contracts. Keep .cse files and private teacher-only browser pages out of model context. Preserve honest computed grades, unsent work, and the teacher's requested scope. Review is the default for scores, assignment creation/edits, and quiz creation; direct writes follow an explicit teacher request to skip review."})
	addReadTool(server, "list_courses", "Read saved course nicknames, IDs, current selections and timezone locally (default source configured); source canvas discovers the authorized Canvas catalog using the saved connection.", func(ctx context.Context, _ *mcp.CallToolRequest, in ListCoursesInput) (TextOutput, error) {
		return s.listCourses(ctx, in)
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
	addReadTool(server, "read_workspace_file", "Read a root .md/.txt teacher reference, workspace instruction, playbook, context file, or course work file through its active authority. Returns authority metadata and the file hash. Hidden folders, .cse, and Engine files are blocked.", func(_ context.Context, _ *mcp.CallToolRequest, in ReadFileInput) (TextOutput, error) {
		path, err := s.readablePath(in.Path)
		if err != nil {
			return TextOutput{}, err
		}
		b, err := readLimited(path, maxWorkspaceFile)
		if err != nil {
			return TextOutput{}, err
		}
		if len(b) > maxWorkspaceFile {
			return TextOutput{}, fmt.Errorf("file exceeds the %d-byte read limit", maxWorkspaceFile)
		}
		h := sha256.Sum256(b)
		location, authority, locationErr := s.rulesMetadata(in.Path)
		if locationErr != nil {
			return TextOutput{}, fmt.Errorf("teacher rules location unavailable: %w", locationErr)
		}
		return TextOutput{Text: fmt.Sprintf("path: %s\nmode: %s\ninstall_root: %s\nrules_root: %s\nauthority_root: %s\nsha256: %x\n\n%s", filepath.ToSlash(in.Path), location.Mode, location.InstallRoot, location.RulesRoot, authority, h, string(b))}, nil
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
	s.registerOperationTools(server)
	s.registerWorkspaceTools(server)
	s.registerSessionTools(server)
	return server, func() { cancel(); s.stopSessions() }
}

func addReadTool[In any](server *mcp.Server, name, description string, fn func(context.Context, *mcp.CallToolRequest, In) (TextOutput, error)) {
	addTool(server, name, description, mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: boolPointer(externalTool(name))}, fn)
}

func addActionTool[In any](server *mcp.Server, name, description string, destructive bool, fn func(context.Context, *mcp.CallToolRequest, In) (TextOutput, error)) {
	addTool(server, name, description, mcp.ToolAnnotations{DestructiveHint: &destructive, OpenWorldHint: boolPointer(externalTool(name)), ReadOnlyHint: false}, fn)
}

func externalTool(name string) bool {
	switch name {
	case "list_courses", "list_modules", "pull_work", "pull_assignment", "pull_page", "start_review", "start_local_ui", "push_document", "post_results", "push_quiz", "upload_file", "list_quiz_points", "set_quiz_points", "canvas_get":
		return true
	default:
		return false
	}
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
	// A stalled Canvas call must not hold the client's tool call forever.
	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()
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

type ListCoursesInput struct {
	Source string `json:"source,omitempty" jsonschema:"configured (default) reads saved selections without Canvas; canvas discovers the live catalog"`
}

type configuredCourses struct {
	Source      string          `json:"source"`
	Root        string          `json:"root"`
	InstallRoot string          `json:"install_root"`
	RulesRoot   string          `json:"rules_root"`
	Mode        string          `json:"mode"`
	Instance    string          `json:"instance"`
	Courses     []domain.Course `json:"courses"`
}

func (s *Server) listCourses(ctx context.Context, in ListCoursesInput) (TextOutput, error) {
	switch in.Source {
	case "canvas":
		return s.command(ctx, "courses")
	case "", "configured":
		path, err := s.readablePath("context/courses.md")
		if err != nil {
			return TextOutput{}, err
		}
		var config domain.CourseConfig
		if _, err := workspace.ReadDocument(path, &config); err != nil {
			return TextOutput{}, err
		}
		if config.Courses == nil {
			config.Courses = []domain.Course{}
		}
		location, _ := workspace.GetRulesLocation(s.root)
		b, err := json.Marshal(configuredCourses{Source: "configured", Root: s.root, InstallRoot: location.InstallRoot, RulesRoot: location.RulesRoot, Mode: location.Mode, Instance: config.Instance, Courses: config.Courses})
		return TextOutput{Text: string(b)}, err
	default:
		return TextOutput{}, errors.New("source must be configured or canvas")
	}
}
