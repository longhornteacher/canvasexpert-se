package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	embedded "canvasexpertse"
	"canvasexpertse/internal/doctor"
	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/outlook"
	"canvasexpertse/internal/playbooks"
	"canvasexpertse/internal/workflow"
	"canvasexpertse/internal/workspace"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type PrepareAudienceInput struct {
	Course  string `json:"course" jsonschema:"Configured course nickname"`
	Path    string `json:"path" jsonschema:"Course-local assignment or assignment-dates draft"`
	Refresh bool   `json:"refresh,omitempty" jsonschema:"Explicitly prepare additions and work-preserving removals for an existing audience"`
}

type QuizInput struct {
	Course string `json:"course" jsonschema:"Configured course nickname"`
	Path   string `json:"path" jsonschema:"Workspace-relative QuizForge JSON or envelope file under courses/"`
}

type UploadInput struct {
	Course    string `json:"course" jsonschema:"Configured course nickname"`
	Path      string `json:"path" jsonschema:"Existing workspace-relative asset file; hidden and Engine files are blocked"`
	Access    string `json:"access" jsonschema:"Explicit access: course, hidden (links still work), or locked"`
	FolderID  int64  `json:"folder_id,omitempty" jsonschema:"Optional existing course folder ID"`
	Receipt   string `json:"receipt" jsonschema:"Workspace-relative reusable receipt path under courses/; preserve it for resumption"`
	NewUpload bool   `json:"new_upload,omitempty" jsonschema:"Explicitly upload fresh bytes without replacing an existing Canvas file; use after deciding a new upload is intended"`
}

type QuizPointChange struct {
	Position int    `json:"position" jsonschema:"One-based question position from list_quiz_points"`
	Points   string `json:"points" jsonschema:"Exact nonnegative decimal points, supplied as a string"`
}

type QuizPointsInput struct {
	Course string `json:"course" jsonschema:"Configured course nickname"`
	Quiz   string `json:"quiz" jsonschema:"Exact New Quiz assignment ID or Canvas link"`
}

type SetQuizPointsInput struct {
	Course  string            `json:"course" jsonschema:"Configured course nickname"`
	Quiz    string            `json:"quiz" jsonschema:"Exact New Quiz assignment ID or Canvas link"`
	Changes []QuizPointChange `json:"changes" jsonschema:"Teacher-requested question point changes; no question content edits"`
}

type CanvasGetInput struct {
	Path string `json:"path" jsonschema:"Canvas API path beginning /api/v1/ or /api/quiz/v1/; include page/per_page query parameters to paginate"`
}

type ExternalResourcesInput struct {
	Course       string `json:"course" jsonschema:"Configured course nickname"`
	GroupID      int64  `json:"group_id,omitempty" jsonschema:"Optional exact collaborative group in this course"`
	ResourceType string `json:"resource_type,omitempty" jsonschema:"collaborations, conferences, or all (default all)"`
}

type EngineInfoInput struct {
	Operation      string `json:"operation,omitempty" jsonschema:"version (default), doctor, link, outlook-status, rules-location, set-rules-location, playbooks-status, update-playbooks, or replace-playbook"`
	SyncRoot       string `json:"sync_root,omitempty" jsonschema:"Explicit teacher-rules directory for set-rules-location; empty selects local UserInstall mode"`
	Path           string `json:"path,omitempty" jsonschema:"Seed-managed workspace-relative file for replace-playbook"`
	ExpectedSHA256 string `json:"expected_sha256,omitempty" jsonschema:"Current hash from playbooks-status, or missing when explicitly adding an absent file"`
	Offline        bool   `json:"offline,omitempty" jsonschema:"For doctor only: skip Canvas and local hub network checks; those results are not_verified"`
}

func (s *Server) registerOperationTools(server *mcp.Server) {
	addActionTool(server, "prepare_audience", "Prepare a dated individual audience from selected collaborative clubs into the draft. Reads Canvas and saves stable refs, completeness and ownership state; makes no Canvas writes. Prepare before assignment review. Membership changes require explicit preparation. With refresh, returns the new assignment-dates action path with additions, removals, protected work and manual exceptions; review/push that returned path.", false, func(ctx context.Context, _ *mcp.CallToolRequest, in PrepareAudienceInput) (TextOutput, error) {
		if err := requireCourse(in.Course); err != nil {
			return TextOutput{}, err
		}
		path, err := s.coursePath(in.Path)
		if err != nil {
			return TextOutput{}, err
		}
		args := []string{"prepare-audience", in.Course, path}
		if in.Refresh {
			args = append(args, "--refresh")
		}
		return s.command(ctx, args...)
	})

	addReadTool(server, "list_external_resources", "Read collaboration and conference metadata for a configured course or exact collaborative group. Returns safe supplied links and known participant refs only; it does not create or open provider resources or read their contents.", func(ctx context.Context, _ *mcp.CallToolRequest, in ExternalResourcesInput) (TextOutput, error) {
		if err := requireCourse(in.Course); err != nil {
			return TextOutput{}, err
		}
		resourceType := in.ResourceType
		if resourceType == "" {
			resourceType = "all"
		}
		if resourceType != "all" && resourceType != "collaborations" && resourceType != "conferences" {
			return TextOutput{}, errors.New("resource_type must be collaborations, conferences, or all")
		}
		args := []string{"external-resources", in.Course}
		if in.GroupID < 0 {
			return TextOutput{}, errors.New("group_id cannot be negative")
		}
		if in.GroupID > 0 {
			args = append(args, "--group", strconv.FormatInt(in.GroupID, 10))
		}
		args = append(args, "--type", resourceType)
		return s.command(ctx, args...)
	})
	addActionTool(server, "push_document", "Create/resume or update an assignment, exact assignment-date override rows, Page, ungraded or linked-graded discussion/announcement, explicitly requested teacher discussion reply, native module, collaborative group-set or explicit group-membership plan. Assignment override rows use stable student refs, exact collaborative-group/section targets, row baselines and exact Canvas override endpoints; omitted dates are preserved and null clears. Membership writes use exact observed student refs, group IDs and per-row conflict checks. Discussion replies verify exact topic/parent scope and save accepted or uncertain outcomes without replay. Uses the document's exact scope and existing Engine conflict/outcome checks. For assignment creation/edits use start_review by default; call directly when the teacher explicitly skips review or to resume a partly created item as its playbook permits. Pages, discussions, replies, modules and group setup use this tool directly. Report returned held/partial/uncertain outcomes honestly.", true, func(ctx context.Context, _ *mcp.CallToolRequest, in PathInput) (TextOutput, error) {
		path, err := s.coursePath(in.Path)
		if err != nil {
			return TextOutput{}, err
		}
		return s.command(ctx, "push", path)
	})
	addActionTool(server, "post_results", "Post honest computed grades, rubric assessments, lateness timing, selected feedback, revise-feedback operations, or comment-only New Quiz feedback. A kind: group-results file posts one shared grade/rubric per ready club product plus whole-group and private comments, or per-student grades when Canvas grades individually; it holds any group or student whose audience, members, mode or attempt changed. New Quiz results can post only an attempt-scoped comment and never a grade, rubric, or late write. Default is start_review; direct posting requires the teacher to explicitly skip review for this task or a saved standing teacher preference to skip it. Honors document scope and current attempt checks; report held or failed items from the Engine output.", true, func(ctx context.Context, _ *mcp.CallToolRequest, in PathInput) (TextOutput, error) {
		path, err := s.coursePath(in.Path)
		if err != nil {
			return TextOutput{}, err
		}
		return s.command(ctx, "post", path)
	})
	addActionTool(server, "push_quiz", "Create or resume a Canvas New Quiz from a QuizForge file, including supported auto-scored and written items and hub differentiation. Default is start_review with course; direct creation requires the teacher to explicitly skip review; resuming a partly created quiz follows its playbook. Written work is graded by the teacher in SpeedGrader. Preserve saved IDs and report actual outcomes; this does not edit existing question content.", true, func(ctx context.Context, _ *mcp.CallToolRequest, in QuizInput) (TextOutput, error) {
		if err := requireCourse(in.Course); err != nil {
			return TextOutput{}, err
		}
		path, err := s.coursePath(in.Path)
		if err != nil {
			return TextOutput{}, err
		}
		return s.command(ctx, "push-quiz", in.Course, path)
	})
	addReadTool(server, "validate_quiz", "Validate supported QuizForge bare JSON or envelope locally without credentials or Canvas writes.", func(_ context.Context, _ *mcp.CallToolRequest, in PathInput) (TextOutput, error) {
		return s.validateQuiz(in.Path)
	})
	addActionTool(server, "upload_file", "Upload an existing workspace asset to an explicit course and access level, saving a reusable receipt under courses/. Reuses/resumes the receipt rather than replacing live files; new_upload requests fresh bytes deliberately. Hidden means links still work; locked denies access. Report returned access and any held/uncertain outcome.", true, func(ctx context.Context, _ *mcp.CallToolRequest, in UploadInput) (TextOutput, error) {
		args, err := s.uploadArgs(in)
		if err != nil {
			return TextOutput{}, err
		}
		return s.command(ctx, args...)
	})
	addReadTool(server, "list_quiz_points", "List a New Quiz's question positions and exact points through the Engine; does not change Canvas.", func(ctx context.Context, _ *mcp.CallToolRequest, in QuizPointsInput) (TextOutput, error) {
		args, err := quizPointsArgs(in.Course, in.Quiz, nil)
		if err != nil {
			return TextOutput{}, err
		}
		return s.command(ctx, args...)
	})
	addActionTool(server, "set_quiz_points", "Apply explicitly requested New Quiz question point changes by position. Uses exact decimals and existing Engine identity/readback checks; report actual item, quiz, and assignment totals and partial/held outcomes.", true, func(ctx context.Context, _ *mcp.CallToolRequest, in SetQuizPointsInput) (TextOutput, error) {
		if len(in.Changes) == 0 {
			return TextOutput{}, errors.New("changes must contain at least one question; use list_quiz_points to read")
		}
		args, err := quizPointsArgs(in.Course, in.Quiz, in.Changes)
		if err != nil {
			return TextOutput{}, err
		}
		return s.command(ctx, args...)
	})
	addReadTool(server, "canvas_get", "Read-only Canvas API GET for data no specific Engine tool covers. Returns raw JSON, potentially including real student names; keep teacher-only data out of feedback. Returns one page; supply page/per_page for more. Cannot issue arbitrary HTTP writes.", func(ctx context.Context, _ *mcp.CallToolRequest, in CanvasGetInput) (TextOutput, error) {
		if !strings.HasPrefix(in.Path, "/api/v1/") && !strings.HasPrefix(in.Path, "/api/quiz/v1/") {
			return TextOutput{}, errors.New("path must begin /api/v1/ or /api/quiz/v1/")
		}
		return s.command(ctx, "api", in.Path)
	})
	addActionTool(server, "engine_info", "Read Engine build/version; run doctor with bounded authenticated read-only Canvas and same-UserInstall hub checks (or offline:true to skip network checks); inspect/set teacher-rules authority; outlook-status reports whether classic Outlook can send parent updates (a boolean and reason, never an address); or compare and explicitly replace one seed-managed playbook. Doctor reports categorized results and never returns Canvas identity, credentials, or raw responses, and does not inspect client settings. playbooks-status returns active-rule paths and hashes; replace-playbook requires path plus expected_sha256 (current hash or missing). Replacement archives the prior file and never replaces context preferences. update-playbooks adopts this Engine's seed on install/update: it replaces only playbooks still identical to the previous shipped copy, keeps teacher-edited ones (reported kept, for agent reconciliation against shipped/<version>/), and never touches context/ or Teacher_Override.md.", false, func(ctx context.Context, _ *mcp.CallToolRequest, in EngineInfoInput) (TextOutput, error) {
		op := in.Operation
		if op == "" {
			op = "version"
		}
		if op == "outlook-status" {
			available, reason := outlook.Status()
			b, err := json.Marshal(struct {
				Available bool   `json:"outlook_available"`
				Reason    string `json:"reason,omitempty"`
			}{available, reason})
			return TextOutput{Text: string(b)}, err
		}
		if op == "rules-location" {
			location, err := workspace.GetRulesLocation(s.root)
			status := struct {
				workspace.RulesLocation
				AvailabilityError string `json:"availability_error,omitempty"`
			}{RulesLocation: location}
			if err != nil {
				status.AvailabilityError = err.Error()
			}
			b, marshalErr := json.Marshal(status)
			if marshalErr != nil {
				return TextOutput{}, marshalErr
			}
			return TextOutput{Text: string(b)}, nil
		}
		if op == "set-rules-location" {
			location, err := workspace.SetRulesLocation(s.root, in.SyncRoot, embedded.WorkspaceTemplate, "workspace-template")
			if err != nil {
				return TextOutput{}, err
			}
			b, err := json.Marshal(location)
			return TextOutput{Text: string(b)}, err
		}
		if op == "doctor" {
			exe, err := os.Executable()
			if err != nil {
				return TextOutput{}, err
			}
			build := fmt.Sprintf("version=%s revision=%s built_at=%s kind=%s", Version, Revision, BuiltAt, BuildKind)
			report := doctor.RunMode(s.root, exe, build, in.Offline)
			data, err := report.JSON()
			return TextOutput{Text: string(data)}, err
		}
		if op == "playbooks-status" {
			location, locErr := workspace.GetRulesLocation(s.root)
			status := struct {
				Location          workspace.RulesLocation `json:"location"`
				AvailabilityError string                  `json:"availability_error,omitempty"`
				Files             []playbooks.StatusRow   `json:"files"`
			}{Location: location, Files: playbooks.Status(s.root)}
			if locErr != nil {
				status.AvailabilityError = locErr.Error()
			}
			b, err := json.Marshal(status)
			return TextOutput{Text: string(b)}, err
		}
		if op == "replace-playbook" {
			if strings.TrimSpace(in.Path) == "" || strings.TrimSpace(in.ExpectedSHA256) == "" {
				return TextOutput{}, errors.New("replace-playbook requires path and expected_sha256 from playbooks-status, or expected_sha256: missing for an absent file")
			}
			if err := playbooks.Replace(s.root, in.Path, in.ExpectedSHA256, time.Now()); err != nil {
				return TextOutput{}, err
			}
			return TextOutput{Text: in.Path + " replaced with this Engine's seed; previous copy archived under playbook-old."}, nil
		}
		if op == "update-playbooks" {
			report, err := playbooks.Update(s.root, time.Now())
			if err != nil {
				return TextOutput{}, err
			}
			b, err := json.Marshal(report)
			return TextOutput{Text: string(b)}, err
		}
		if op != "version" && op != "doctor" && op != "link" {
			return TextOutput{}, errors.New("operation must be version, doctor, link, outlook-status, rules-location, set-rules-location, playbooks-status, update-playbooks, or replace-playbook")
		}
		result, err := s.command(ctx, op)
		if op == "version" {
			result.Text = "loaded_adapter: " + AdapterVersion() + "\nchild_engine:\n" + result.Text
		}
		return result, err
	})
}

func requireCourse(course string) error {
	if strings.TrimSpace(course) == "" {
		return errors.New("an explicit configured course nickname is required")
	}
	return nil
}

func (s *Server) validateQuiz(relative string) (TextOutput, error) {
	path, err := s.coursePath(relative)
	if err != nil {
		return TextOutput{}, err
	}
	q, err := workflow.ReadQuizForge(path)
	if err != nil {
		return TextOutput{}, err
	}
	output := fmt.Sprintf("Valid QuizForge quiz: %s\nTitle: %s\nItems: %d", relative, q.Title, len(q.Items))
	for _, warning := range q.Warnings {
		output += "\nWarning: " + warning
	}
	return TextOutput{Text: output}, nil
}

func (s *Server) uploadArgs(in UploadInput) ([]string, error) {
	if err := requireCourse(in.Course); err != nil {
		return nil, err
	}
	if in.Access != "course" && in.Access != "hidden" && in.Access != "locked" {
		return nil, errors.New("access must be course, hidden, or locked")
	}
	if in.FolderID < 0 {
		return nil, errors.New("folder_id must be positive when supplied")
	}
	source, err := s.readablePath(in.Path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(source)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("upload source must be a regular file")
	}
	receipt, err := s.coursePath(in.Receipt)
	if err != nil {
		return nil, fmt.Errorf("receipt: %w", err)
	}
	args := []string{"upload-file", in.Course, source, "--access", in.Access, "--receipt", receipt}
	if in.FolderID > 0 {
		args = append(args, "--folder", strconv.FormatInt(in.FolderID, 10))
	}
	if in.NewUpload {
		args = append(args, "--new-upload")
	}
	return args, nil
}

func quizPointsArgs(course, quiz string, changes []QuizPointChange) ([]string, error) {
	if err := requireCourse(course); err != nil {
		return nil, err
	}
	if strings.TrimSpace(quiz) == "" {
		return nil, errors.New("an exact quiz ID or link is required")
	}
	args := []string{"quiz-points", course, quiz}
	seen := map[int]bool{}
	for _, change := range changes {
		points, err := domain.ParseDecimal(change.Points)
		if change.Position <= 0 || err != nil || points.Sign() < 0 {
			return nil, errors.New("each change requires a positive position and exact nonnegative decimal points")
		}
		if seen[change.Position] {
			return nil, fmt.Errorf("duplicate question position %d", change.Position)
		}
		seen[change.Position] = true
		args = append(args, fmt.Sprintf("%d=%s", change.Position, points.String()))
	}
	return args, nil
}
