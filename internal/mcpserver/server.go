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
	"strconv"
	"strings"
	"sync"
	"time"

	embedded "canvasexpertse"
	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/platform"
	"canvasexpertse/internal/workflow"
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
	pulls             map[string]*pullJob
	run               func(context.Context, ...string) (TextOutput, error) // nil uses command; tests replace it
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
	GroupID    *int64 `json:"group_id,omitempty" jsonschema:"Optional exact collaborative group ID; requires course and assignment"`
	Session    bool   `json:"session,omitempty" jsonschema:"Pull unscored submitted work for a scoring session, including not-yet-due work; timing applies only to work being scored"`
}

type PrepareLateTimingInput struct {
	Packet   string                               `json:"packet" jsonschema:"Workspace-relative packet or group-packet path under courses/"`
	Students []workflow.LateTimingStudentDecision `json:"students,omitempty" jsonschema:"First-genuine choices for individual packet members"`
	Groups   []workflow.LateTimingGroupDecision   `json:"groups,omitempty" jsonschema:"Per-member first-genuine choices for group-packet entries"`
}

type CopyModuleInput struct {
	SourceCourse string   `json:"source_course" jsonschema:"Configured nickname of the course the module is copied from, usually Collab"`
	Module       string   `json:"module" jsonschema:"Exact title of the module in the source course"`
	Courses      []string `json:"courses" jsonschema:"Configured course nicknames to copy into"`
}

type ParentReportInput struct {
	Courses  []string `json:"courses" jsonschema:"Configured course nicknames; one email can cover several courses"`
	Students []string `json:"students,omitempty" jsonschema:"Optional student refs; omit for every active student"`
}

type ParentReviewInput struct {
	Folder string `json:"folder" jsonschema:"Workspace-relative run folder under courses/ holding report.json and drafts.json for grade updates, or contact.json and prepared audience.json for family contact"`
}

type FamilyContactInput struct {
	Folder string `json:"folder" jsonschema:"Workspace-relative family-contact run folder under courses/ holding contact.json and local assets/"`
}

type CourseQueryInput struct {
	Course string `json:"course" jsonschema:"Configured course nickname"`
	Query  string `json:"query" jsonschema:"Assignment title or ID, or Page ID or Canvas link"`
}

type PageQueryInput struct {
	Course  string `json:"course" jsonschema:"Configured course nickname"`
	Query   string `json:"query" jsonschema:"Exact Page ID or Canvas Page link; preserve this field name"`
	GroupID int64  `json:"group_id,omitempty" jsonschema:"Optional exact collaborative group ID for a group Page"`
}

type PageRevisionsInput struct {
	Course     string `json:"course" jsonschema:"Configured course nickname"`
	PageID     int64  `json:"page_id" jsonschema:"Exact numeric Canvas Page ID"`
	GroupID    int64  `json:"group_id,omitempty" jsonschema:"Optional exact collaborative group ID"`
	RevisionID int64  `json:"revision_id,omitempty" jsonschema:"Optional revision ID; selected revision includes its historical body"`
}

type DiscussionQueryInput struct {
	Course  string `json:"course" jsonschema:"Configured course nickname"`
	TopicID int64  `json:"topic_id" jsonschema:"Exact Canvas discussion topic ID"`
	GroupID int64  `json:"group_id,omitempty" jsonschema:"Optional exact collaborative group ID for a direct group topic"`
}

type CourseInput struct {
	Course string `json:"course" jsonschema:"Configured course nickname"`
}

type GroupSetsInput struct {
	Course string `json:"course" jsonschema:"Configured course nickname"`
}
type GroupsInput struct {
	Course         string `json:"course" jsonschema:"Configured course nickname"`
	GroupSetID     int64  `json:"group_set_id,omitempty" jsonschema:"Optional exact collaborative group-set ID"`
	GroupID        int64  `json:"group_id,omitempty" jsonschema:"Optional exact collaborative group ID"`
	IncludeMembers bool   `json:"include_members,omitempty" jsonschema:"Return stable student refs and aliases for this exact group only"`
}
type PullGroupSetInput struct {
	Course     string `json:"course" jsonschema:"Configured course nickname"`
	GroupSetID int64  `json:"group_set_id" jsonschema:"Exact collaborative group-set ID"`
}
type PullGroupMembershipsInput struct {
	Course          string `json:"course" jsonschema:"Configured course nickname"`
	GroupCategoryID int64  `json:"group_category_id" jsonschema:"Exact collaborative group-set ID"`
}

type PullAssignmentDatesInput struct {
	Course       string `json:"course" jsonschema:"Configured course nickname"`
	AssignmentID int64  `json:"assignment_id" jsonschema:"Exact numeric Canvas assignment ID"`
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
	server := mcp.NewServer(&mcp.Implementation{Name: "canvasexpert-se", Version: AdapterVersion()}, &mcp.ServerOptions{Instructions: "Before any teacher task, read AGENTS.md using read_workspace_file, then read the relevant playbook. Commands and grading/planning session files remain in UserInstall. Reusable teacher setup, including course configuration, student settings/notes, instructions, playbooks, guides and preferences, uses UserInstall by default or the explicitly configured UserSync authority. Private roster and contact files also use that authority but are never model context; UserSync is a provider-synchronized local directory, and reads always use its current contents. Check engine_info operation rules-location if the active authority is unclear. Read list_courses with its default configured source for saved nicknames and current selections; request source canvas only for connection testing or additional course discovery. Treat the structured courses list as setup status and ignore obsolete starter text in a preserved document body. Use engine_reference for Engine commands and file contracts. Keep .cse files and private teacher-only browser pages out of model context. Preserve honest computed grades, unsent work, and the teacher's requested scope. Review is the default for scores, assignment creation/edits, and quiz creation; direct writes follow an explicit teacher request to skip review."})
	addReadTool(server, "list_courses", "Read saved course nicknames, IDs, current selections and timezone locally (default source configured); source canvas discovers the authorized Canvas catalog using the saved connection. A course with an owner belongs to that teammate; the team list says whether their Canvas key is on this PC.", func(ctx context.Context, _ *mcp.CallToolRequest, in ListCoursesInput) (TextOutput, error) {
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
	addActionTool(server, "pull_work", "Pull current Canvas work using the CE-SE Engine. Set session for targeted scoring discovery (Canvas needs-grading counts) Student identity is pseudonymized by the Engine; never access .cse. Group assignments produce group-packet/group-work evidence while preserving each member's submission identity. A group_id requires an explicit assignment. A long pull returns state pending; call again with the same inputs to keep waiting on the same run.", false, func(ctx context.Context, _ *mcp.CallToolRequest, in PullWorkInput) (TextOutput, error) {
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
		if in.GroupID != nil {
			if *in.GroupID <= 0 || in.Course == "" || in.Assignment == "" {
				return TextOutput{}, errors.New("positive group_id requires explicit course and assignment")
			}
			args = append(args, "--group", strconv.FormatInt(*in.GroupID, 10))
		}
		if in.Session {
			args = append(args, "--session")
		}
		return s.longCommand(ctx, args...)
	})
	addActionTool(server, "prepare_late_timing", "Check first-genuine submission and deadline decisions against a pulled packet, compute school time from the active calendar when needed, and return proposed/unchanged/held actions. This only prepares a reviewable result; it never writes to Canvas.", false, func(_ context.Context, _ *mcp.CallToolRequest, in PrepareLateTimingInput) (TextOutput, error) {
		var config domain.CourseConfig
		if _, err := workspace.ReadTeacherDocument(s.root, "context/courses.md", &config); err != nil {
			return TextOutput{}, err
		}
		prepared, err := (&workflow.Service{Root: s.root, Config: config}).PrepareLateTimingDecisions(workflow.LateTimingDecisionsDocument{Schema: 1, Kind: "late-timing-decisions", Packet: in.Packet, Students: in.Students, Groups: in.Groups})
		if err != nil {
			return TextOutput{}, err
		}
		encoded, err := json.MarshalIndent(prepared, "", "  ")
		if err != nil {
			return TextOutput{}, err
		}
		return TextOutput{Text: string(encoded)}, nil
	})
	addActionTool(server, "pull_parent_report", "Read current Canvas facts for parent updates (course score, missing work, correction candidates, due soon, grade history) into courses/<course>/parent-updates/<run>/report.json. Students are pseudonymized; no contacts are included.", false, func(ctx context.Context, _ *mcp.CallToolRequest, in ParentReportInput) (TextOutput, error) {
		return s.longCommand(ctx, append(append([]string{"parent-report"}, in.Courses...), in.Students...)...)
	})
	addActionTool(server, "copy_module", "Copies one Collab module, by exact title, once into each listed course, using each course's teacher key. Never re-copies. State the module and the course list to the teacher before calling.", false, func(ctx context.Context, _ *mcp.CallToolRequest, in CopyModuleInput) (TextOutput, error) {
		return s.longCommand(ctx, append([]string{"copy-module", in.SourceCourse, in.Module}, in.Courses...)...)
	})
	addActionTool(server, "prepare_family_contact", "Resolve a free-form contact.json audience against current Canvas courses and collaborative groups. Save audience.json and return pseudonymized recipients, message counts, warnings and blocking conflicts. Refreshing preserves outcomes. No email is sent and no addresses or Canvas user IDs are returned.", false, func(ctx context.Context, _ *mcp.CallToolRequest, in FamilyContactInput) (TextOutput, error) {
		dir, err := s.coursePath(in.Folder)
		if err != nil {
			return TextOutput{}, err
		}
		return s.longCommand(ctx, "family-contact-prepare", dir)
	})
	addActionTool(server, "start_parent_review", "Open the private page where the teacher reviews grade-update or free-form family-contact emails with their real recipients and chooses Send or Drafts only through Outlook. Family contact requires prepare_family_contact first. Returns only the loopback URL; call review_status afterward for counts and outcomes by student reference. This tool never sends and never exposes addresses.", true, func(ctx context.Context, _ *mcp.CallToolRequest, in ParentReviewInput) (TextOutput, error) {
		dir, err := s.coursePath(in.Folder)
		if err != nil {
			return TextOutput{}, err
		}
		return s.startReviewSession(ctx, dir, []string{"parent-review", dir})
	})
	addActionTool(server, "print_student_reports", "Save one PDF of student reports from a parent-update run folder (report.json and drafts.json), one student per page, in the teacher's Downloads folder, and open it for printing. Every student gets a page. Returns the file path and count only; the PDF holds student names and is outside the workspace. Nothing is sent or written to Canvas.", false, func(ctx context.Context, _ *mcp.CallToolRequest, in ParentReviewInput) (TextOutput, error) {
		dir, err := s.coursePath(in.Folder)
		if err != nil {
			return TextOutput{}, err
		}
		return s.command(ctx, "student-report", dir)
	})
	addReadTool(server, "list_modules", "List a configured course's Canvas modules through the CE-SE Engine.", func(ctx context.Context, _ *mcp.CallToolRequest, in CourseInput) (TextOutput, error) {
		return s.command(ctx, "modules", in.Course)
	})
	addReadTool(server, "list_group_sets", "List collaborative group sets in a configured course. Differentiation tags are excluded.", func(ctx context.Context, _ *mcp.CallToolRequest, in GroupSetsInput) (TextOutput, error) {
		return s.command(ctx, "groups", in.Course)
	})
	addReadTool(server, "list_groups", "List collaborative clubs. An exact group-set read includes stable-ref memberships and unassigned students; an exact group read includes memberships only when requested. Private names, logins and contact data are never returned.", func(ctx context.Context, _ *mcp.CallToolRequest, in GroupsInput) (TextOutput, error) {
		args := []string{"groups", in.Course, "--groups"}
		if in.GroupSetID > 0 {
			args = append(args, "--set", strconv.FormatInt(in.GroupSetID, 10))
		}
		if in.GroupID > 0 {
			flag := "--group"
			if in.IncludeMembers {
				flag = "--members"
			}
			args = append(args, flag, strconv.FormatInt(in.GroupID, 10))
		}
		return s.command(ctx, args...)
	})
	addActionTool(server, "pull_group_set", "Save an editable baseline for an exact collaborative group set under courses/.", false, func(ctx context.Context, _ *mcp.CallToolRequest, in PullGroupSetInput) (TextOutput, error) {
		return s.longCommand(ctx, "pull-group-set", in.Course, strconv.FormatInt(in.GroupSetID, 10))
	})
	addActionTool(server, "pull_group_memberships", "Save an exact group-set membership baseline with stable student refs, aliases, membership states and the unassigned roster. The returned path can be edited with explicit changes or allocate-unassigned and pushed through push_document.", false, func(ctx context.Context, _ *mcp.CallToolRequest, in PullGroupMembershipsInput) (TextOutput, error) {
		return s.longCommand(ctx, "pull-group-memberships", in.Course, strconv.FormatInt(in.GroupCategoryID, 10))
	})
	addActionTool(server, "pull_assignment_dates", "Pull the exact assignment override IDs, targets, date baselines and per-student effective dates into an editable assignment-dates document. Student targets appear only through existing private student refs.", false, func(ctx context.Context, _ *mcp.CallToolRequest, in PullAssignmentDatesInput) (TextOutput, error) {
		if in.AssignmentID <= 0 {
			return TextOutput{}, errors.New("assignment_id must be positive")
		}
		return s.command(ctx, "pull-assignment-dates", in.Course, strconv.FormatInt(in.AssignmentID, 10))
	})
	addActionTool(server, "pull_assignment", "Pull an exact Canvas assignment and its edit baseline through the CE-SE Engine.", false, func(ctx context.Context, _ *mcp.CallToolRequest, in CourseQueryInput) (TextOutput, error) {
		return s.command(ctx, "pull-item", in.Course, in.Query)
	})
	addActionTool(server, "pull_page", "Pull an exact Canvas Page and its edit baseline through the CE-SE Engine. Use group_id for a group Page; query remains the exact Page ID or link.", false, func(ctx context.Context, _ *mcp.CallToolRequest, in PageQueryInput) (TextOutput, error) {
		args := []string{"pull-page", in.Course, in.Query}
		if in.GroupID != 0 {
			if in.GroupID < 0 {
				return TextOutput{}, errors.New("group_id must be positive")
			}
			args = append(args, "--group", strconv.FormatInt(in.GroupID, 10))
		}
		return s.command(ctx, args...)
	})
	addActionTool(server, "pull_discussion", "Pull an exact course, distributed club, or direct group discussion. Saves topic.md plus an as-of discussion-packet with complete/partial entry coverage, stable-ref attribution, and attachment status. A long pull returns pending; call again with the same inputs to wait on the same run.", false, func(ctx context.Context, _ *mcp.CallToolRequest, in DiscussionQueryInput) (TextOutput, error) {
		args := []string{"pull-discussion", in.Course, strconv.FormatInt(in.TopicID, 10)}
		if in.GroupID != 0 {
			if in.GroupID < 0 {
				return TextOutput{}, errors.New("group_id must be positive")
			}
			args = append(args, "--group", strconv.FormatInt(in.GroupID, 10))
		}
		return s.longCommand(ctx, args...)
	})
	addReadTool(server, "list_page_revisions", "Read revision metadata for an exact course or group Page. Set revision_id to also read that revision's historical body; this does not identify every contributor or simultaneous edits.", func(ctx context.Context, _ *mcp.CallToolRequest, in PageRevisionsInput) (TextOutput, error) {
		args := []string{"page-history", in.Course, strconv.FormatInt(in.PageID, 10)}
		if in.GroupID != 0 {
			args = append(args, "--group", strconv.FormatInt(in.GroupID, 10))
		}
		if in.RevisionID != 0 {
			args = append(args, "--revision", strconv.FormatInt(in.RevisionID, 10))
		}
		return s.command(ctx, args...)
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
	addActionTool(server, "init_results", "Start a results file for a pulled individual packet under its assignment's scoring/ folder: the packet path, each student's exact current attempt, the operation the assignment allows and any discussion-evidence digest, with an earlier results file's interpretation and scoring_basis reused. Interpretation, components and feedback are left blank and fail validation until filled. Never writes to Canvas.", false, func(ctx context.Context, _ *mcp.CallToolRequest, in PathInput) (TextOutput, error) {
		if _, err := s.coursePath(in.Path); err != nil {
			return TextOutput{}, err
		}
		return s.command(ctx, "results-init", in.Path)
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
	case "prepare_audience", "prepare_family_contact", "list_courses", "list_modules", "list_group_sets", "list_groups", "pull_group_set", "pull_group_memberships", "pull_assignment_dates", "pull_work", "pull_parent_report", "copy_module", "pull_assignment", "pull_page", "pull_discussion", "list_page_revisions", "start_review", "start_parent_review", "print_student_reports", "start_local_ui", "push_document", "post_results", "push_quiz", "upload_file", "list_quiz_points", "set_quiz_points", "canvas_get":
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
	Team        []teamStatus    `json:"team"`
}

// teamStatus tells the agent who the teammates are and whether their keys are
// saved here. It never carries a key.
type teamStatus struct {
	Label       string `json:"label"`
	Name        string `json:"name"`
	Email       string `json:"email,omitempty"`
	KeyOnThisPC bool   `json:"key_on_this_pc"`
	KeyExpires  string `json:"key_expires"`
	ExpiresSoon bool   `json:"expires_soon"`
}

func (s *Server) teamStatuses() ([]teamStatus, error) {
	out := []teamStatus{}
	path, err := s.readablePath("context/team.md")
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return out, nil
	}
	var team workspace.TeamConfig
	if _, err := workspace.ReadDocument(path, &team); err != nil {
		return nil, err
	}
	now := time.Now()
	for _, t := range team.Teammates {
		credential, keyErr := platform.LoadTeammateCredential(t.CanvasUserID)
		credential.Token = ""
		out = append(out, teamStatus{Label: t.Label, Name: t.Name, Email: t.Email, KeyOnThisPC: keyErr == nil, KeyExpires: t.KeyExpires, ExpiresSoon: t.ExpiryState(now) != workspace.KeyOK})
	}
	return out, nil
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
		team, err := s.teamStatuses()
		if err != nil {
			return TextOutput{}, err
		}
		location, _ := workspace.GetRulesLocation(s.root)
		b, err := json.Marshal(configuredCourses{Source: "configured", Root: s.root, InstallRoot: location.InstallRoot, RulesRoot: location.RulesRoot, Mode: location.Mode, Instance: config.Instance, Courses: config.Courses, Team: team})
		return TextOutput{Text: string(b)}, err
	default:
		return TextOutput{}, errors.New("source must be configured or canvas")
	}
}
