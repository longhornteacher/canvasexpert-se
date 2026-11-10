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
	newChild          func(args []string) *exec.Cmd                        // tests replace the child Engine
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

type LateSweepInput struct {
	Course string `json:"course" jsonschema:"Configured course nickname"`
	Period string `json:"period,omitempty" jsonschema:"Grading period name from context/school-calendar.md; default the current period"`
}

type PersonalDatesInput struct {
	Course       string `json:"course" jsonschema:"Configured course nickname"`
	AssignmentID int64  `json:"assignment_id,omitempty" jsonschema:"Canvas assignment ID; omit to cover the course's published assignments due in the last three weeks or later"`
	Preview      bool   `json:"preview,omitempty" jsonschema:"Report the planned dates without writing to Canvas"`
}

type CopyAssignmentInput struct {
	SourceCourse      string `json:"source_course" jsonschema:"Configured source course nickname"`
	Query             string `json:"query" jsonschema:"Source assignment title or exact ID"`
	TargetCourse      string `json:"target_course" jsonschema:"Configured destination course nickname"`
	ModuleID          int64  `json:"module_id,omitempty" jsonschema:"Optional existing destination module ID"`
	AssignmentGroupID int64  `json:"assignment_group_id,omitempty" jsonschema:"Optional destination assignment group ID"`
	Published         bool   `json:"published,omitempty" jsonschema:"Publish only when requested; default false"`
}

type CopyModuleInput struct {
	SourceCourse string   `json:"source_course" jsonschema:"Configured nickname of the course the module is copied from, usually Collab"`
	Module       string   `json:"module" jsonschema:"Exact title of the module in the source course"`
	Courses      []string `json:"courses" jsonschema:"Configured course nicknames to copy into"`
}

type ParentReportInput struct {
	Courses    []string `json:"courses" jsonschema:"Configured course nicknames; one email can cover several courses"`
	Students   []string `json:"students,omitempty" jsonschema:"Optional student refs; omit for every active student"`
	FullPeriod bool     `json:"full_period,omitempty" jsonschema:"Accepted for compatibility; every report now covers the whole current grading period"`
}

type WorkHabitsInput struct {
	Course   string   `json:"course" jsonschema:"Configured course nickname"`
	From     string   `json:"from,omitempty" jsonschema:"First due date, YYYY-MM-DD in the course timezone; default four weeks ago"`
	To       string   `json:"to,omitempty" jsonschema:"Last due date, YYYY-MM-DD; default today. Work not yet due is never counted"`
	Students []string `json:"students,omitempty" jsonschema:"Optional student refs; omit for every active student"`
}

type STADCourseInput struct {
	Course string `json:"course" jsonschema:"Configured course nickname"`
}

type STADStandingsInput struct {
	Course string `json:"course" jsonschema:"Configured course nickname"`
	Week   string `json:"week,omitempty" jsonschema:"Any date in the week to report, YYYY-MM-DD; default the latest week of the round"`
	Round  string `json:"round,omitempty" jsonschema:"Round name from teams.md; default the current round"`
}

type STADPublishInput struct {
	Course    string `json:"course" jsonschema:"Configured course nickname"`
	PageTitle string `json:"page_title" jsonschema:"Title of the standings Page"`
	Body      string `json:"body" jsonschema:"Workspace-relative Markdown file for the page body, containing {{standings}} where the computed table goes; it must not name any student"`
	Published bool   `json:"published,omitempty" jsonschema:"Publish a new Page only when the teacher asks; default unpublished"`
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

type PullQuizInput struct {
	Course       string `json:"course" jsonschema:"Configured course nickname"`
	AssignmentID int64  `json:"assignment_id" jsonschema:"Exact positive New Quiz assignment ID"`
}

type PageQueryInput struct {
	Course  string `json:"course" jsonschema:"Configured course nickname"`
	Query   string `json:"query" jsonschema:"Exact Page ID or Canvas Page link; preserve this field name"`
	GroupID int64  `json:"group_id,omitempty" jsonschema:"Optional exact collaborative group ID for a group Page"`
}

type ReplaceTextInput struct {
	Course           string                     `json:"course" jsonschema:"Configured course nickname"`
	Target           string                     `json:"target" jsonschema:"page or assignment"`
	Query            string                     `json:"query" jsonschema:"Exact Page ID or Canvas Page link; for an assignment, its exact assignment ID"`
	Replacements     []workflow.BodyReplacement `json:"replacements" jsonschema:"Exact find/replace pairs applied in order to the stored Canvas HTML; each find must match once unless all is true"`
	ExpectUpdatedAt  string                     `json:"expect_updated_at,omitempty" jsonschema:"Optional baseline: updated_at from a prior pull or replace report; holds if Canvas changed since"`
	ExpectBodySHA256 string                     `json:"expect_body_sha256,omitempty" jsonschema:"Optional baseline: body_sha256 from a prior replace report; holds if the body changed since"`
	Push             bool                       `json:"push,omitempty" jsonschema:"Send the edit now. Pages: true. Assignments: true only after the teacher's go-ahead in the conversation, or when the request already asked for the change"`
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

type ValidateDocumentInput struct {
	Path string `json:"path" jsonschema:"Workspace-relative document path"`
	Show string `json:"show,omitempty" jsonschema:"Optional student_ref in a results file; return only their exact rendered feedback sample or hold reason"`
}

type HelpInput struct {
	Topic string `json:"topic,omitempty" jsonschema:"Optional reference topic: formats, formats-assignment, formats-page, formats-module, formats-quiz, formats-inbox, fields, or examples"`
}

type DraftPDFInput struct {
	Path    string `json:"path" jsonschema:"Workspace-relative assignment, Page, QuizForge quiz or results file"`
	Handout bool   `json:"handout,omitempty" jsonschema:"Keep only what students see (directions, questions, posted feedback) for printed copies"`
}

type ReviewStatusInput struct {
	ReviewID string `json:"review_id" jsonschema:"ID returned by start_parent_review"`
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
	server := mcp.NewServer(&mcp.Implementation{Name: "canvasexpert-se", Version: AdapterVersion()}, &mcp.ServerOptions{Instructions: "Before any teacher task, read AGENTS.md using read_workspace_file, then read the relevant playbook. Commands and grading/planning session files remain in UserInstall. Reusable teacher setup, including course configuration, student settings/notes, instructions, playbooks, guides and preferences, uses UserInstall by default or the explicitly configured UserSync authority. Private roster and contact files also use that authority but are never model context; UserSync is a provider-synchronized local directory, and reads always use its current contents. Check engine_info operation rules-location if the active authority is unclear. Read list_courses with its default configured source for saved nicknames and current selections; request source canvas only for connection testing or additional course discovery. Treat the structured courses list as setup status and ignore obsolete starter text in a preserved document body. Use engine_reference for Engine commands and file contracts. Keep .cse files and private teacher-only browser pages out of model context. Students appear as a stable student_ref and a saved alias (a Pokemon name) while the teacher has student names hidden; refer to students by alias and ask the teacher for the alias rather than searching Canvas for a real name. Preserve honest computed grades, unsent work, and the teacher's requested scope. Before posting scores, creating assignments or quizzes, or editing assignments, report the work in the conversation and wait for the teacher's go-ahead there, unless the request already included the write."})
	addReadTool(server, "list_courses", "Read saved course nicknames, IDs, current selections and timezone locally (default source configured); source canvas discovers the authorized Canvas catalog using the saved connection. A course with an owner belongs to that teammate; the team list says whether their Canvas key is on this PC.", func(ctx context.Context, _ *mcp.CallToolRequest, in ListCoursesInput) (TextOutput, error) {
		return s.listCourses(ctx, in)
	})
	addReadTool(server, "engine_reference", "Read CE-SE command help, document formats, Canvas field limits, or synthetic examples. Use formats-assignment, formats-page, formats-module, formats-quiz, or formats-inbox for a smaller focused contract.", func(ctx context.Context, _ *mcp.CallToolRequest, in HelpInput) (TextOutput, error) {
		if in.Topic == "" {
			return s.command(ctx, "help")
		}
		if !embedded.IsEngineReferenceTopic(in.Topic) {
			return TextOutput{}, errors.New("topic must be formats, formats-assignment, formats-page, formats-module, formats-quiz, formats-inbox, fields, or examples")
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
	addActionTool(server, "late_sweep", "Recompute a course's late marks in school time. For every submission Canvas shows as late on an ordinary individual assignment due in the grading period, the Engine times the earliest attempt against the student's deadline, skipping weekends, calendar closures and the teacher's saved absences, and adding saved extra-time days. Returns proposed, unchanged and held rows by student_ref and alias, and saves one late-only results file per assignment under courses/<course>/late-sweeps/ to send with post_results after the teacher's go-ahead. Read-only: it never writes to Canvas. A long sweep returns state pending; call again with the same inputs.", false, func(ctx context.Context, _ *mcp.CallToolRequest, in LateSweepInput) (TextOutput, error) {
		args := []string{"late-sweep", in.Course}
		if in.Period != "" {
			args = append(args, "--period", in.Period)
		}
		return s.longCommand(ctx, args...)
	})
	addActionTool(server, "personal_dates", "Give students their own Canvas due and lock dates on an assignment, as single-student Canvas overrides. A student with saved extra time gets a later due date (saved school days, or a saved percent of the school days from the assigned date to the due date, rounded up); a student absent on the due date is due the next day back; where the course has a late-work limit the lock date follows the student's own due date; and after the due date, students whose work was on time are reopened through the correction window. Dates only move later, a student who already has a personal Canvas due date keeps it, and group assignments are skipped. Returns planned, written and held rows by student_ref and alias; the why list is private. Use preview first, then write after the teacher's go-ahead.", false, func(ctx context.Context, _ *mcp.CallToolRequest, in PersonalDatesInput) (TextOutput, error) {
		args := []string{"personal-dates", in.Course}
		if in.AssignmentID > 0 {
			args = append(args, strconv.FormatInt(in.AssignmentID, 10))
		}
		if in.Preview {
			args = append(args, "--preview")
		}
		return s.longCommand(ctx, args...)
	})
	addActionTool(server, "pull_parent_report", "Read current Canvas facts for parent updates into courses/<course>/parent-updates/<run>/report.json: for each student and course, every assignment due this grading period so far with its released score, percent, student due date, direct link and turned-in evidence, plus upcoming work for printed sheets. Facts only; the playbook writes the email. Students are pseudonymized; no contacts are included.", false, func(ctx context.Context, _ *mcp.CallToolRequest, in ParentReportInput) (TextOutput, error) {
		args := append(append([]string{"parent-report"}, in.Courses...), in.Students...)
		if in.FullPeriod {
			args = append(args, "--full-period")
		}
		return s.longCommand(ctx, args...)
	})
	addActionTool(server, "work_habits", "Read each active student's turn-in record for a course into courses/<course>/work-habits/<run>/habits.json: per week, counts and item lists of due, on_time, late, missing, excused and no_online_submission work. Work due before a student enrolled is listed apart and never counted. Facts only; read-only; students are pseudonymized. Used for sorting groups or teams, parent-update context and who is behind.", false, func(ctx context.Context, _ *mcp.CallToolRequest, in WorkHabitsInput) (TextOutput, error) {
		args := []string{"work-habits", in.Course}
		if in.From != "" {
			args = append(args, "--from", in.From)
		}
		if in.To != "" {
			args = append(args, "--to", in.To)
		}
		return s.longCommand(ctx, append(args, in.Students...)...)
	})
	addActionTool(server, "stad_teams", "Check the course's classroom team file (context/stad/<course>/teams.md) against its active students: current round, any student_ref that is not active, and active students on no team. Read-only.", false, func(ctx context.Context, _ *mcp.CallToolRequest, in STADCourseInput) (TextOutput, error) {
		return s.longCommand(ctx, "stad", "teams", in.Course)
	})
	addActionTool(server, "stad_standings", "Tally classroom team points for a round from Canvas turn-in facts, rules.md and awards.md: team scores (average over active members) with ranks and ties, plus teacher-only per-student detail and any award the weekly cap trimmed. Fields tagged visibility team are safe for students; teacher fields are not. Read-only.", false, func(ctx context.Context, _ *mcp.CallToolRequest, in STADStandingsInput) (TextOutput, error) {
		args := []string{"stad", "standings", in.Course}
		if in.Week != "" {
			args = append(args, "--week", in.Week)
		}
		if in.Round != "" {
			args = append(args, "--round", in.Round)
		}
		return s.longCommand(ctx, args...)
	})
	addActionTool(server, "stad_publish", "Create or update the course's team standings Page: your body with the computed standings table (team names, ranks, team scores only) inserted at {{standings}}. Refuses a body that names a student (reference or alias). Remembers the Page and holds instead of overwriting if someone edited it in Canvas since. A new Page is unpublished unless the teacher asked to publish. Show the teacher the wording first.", false, func(ctx context.Context, _ *mcp.CallToolRequest, in STADPublishInput) (TextOutput, error) {
		args := []string{"stad", "publish", in.Course, "--page-title", in.PageTitle, "--body", in.Body}
		if in.Published {
			args = append(args, "--published")
		}
		return s.longCommand(ctx, args...)
	})
	addActionTool(server, "stad_push_teams", "Make the Canvas group set named in teams.md match the current team round: creates the set and one group per team when missing, renames a group whose team was renamed, and places each member. Adds and moves only; never deletes a group or removes a student. Show the teacher the teams first and wait for their go-ahead.", false, func(ctx context.Context, _ *mcp.CallToolRequest, in STADCourseInput) (TextOutput, error) {
		return s.longCommand(ctx, "stad", "push-teams", in.Course)
	})
	addActionTool(server, "copy_module", "Copies one Collab module, by exact title, once into each listed course, using each course's teacher key. Never re-copies. State the module and the course list to the teacher before calling.", false, func(ctx context.Context, _ *mcp.CallToolRequest, in CopyModuleInput) (TextOutput, error) {
		return s.longCommand(ctx, append([]string{"copy-module", in.SourceCourse, in.Module}, in.Courses...)...)
	})
	addActionTool(server, "copy_assignment", "Copy a plain assignment between configured courses, copying embedded Canvas files and preserving directions, points, dates and any rubric. Defaults unpublished. Optional module and assignment group IDs belong to the destination. Reuse the saved receipt to resume without duplicate assignments; no submissions, grades or overrides are copied.", false, func(ctx context.Context, _ *mcp.CallToolRequest, in CopyAssignmentInput) (TextOutput, error) {
		if in.ModuleID < 0 || in.AssignmentGroupID < 0 {
			return TextOutput{}, fmt.Errorf("module_id and assignment_group_id must be positive when supplied")
		}
		args := []string{"copy-assignment", in.SourceCourse, in.Query, in.TargetCourse}
		if in.ModuleID != 0 {
			args = append(args, "--module-id", strconv.FormatInt(in.ModuleID, 10))
		}
		if in.AssignmentGroupID != 0 {
			args = append(args, "--assignment-group-id", strconv.FormatInt(in.AssignmentGroupID, 10))
		}
		if in.Published {
			args = append(args, "--published")
		}
		return s.longCommand(ctx, args...)
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
	addActionTool(server, "print_student_reports", "Save one PDF of student reports from a parent-update run folder (only report.json is needed; drafts.json adds optional intros), one student per page: assignment, due date and current grade, with a check box for work not yet due, in the teacher's Downloads folder, and open it for printing. Every student gets a page. Returns the file path and count only; the PDF holds student names and is outside the workspace. Nothing is sent or written to Canvas.", false, func(ctx context.Context, _ *mcp.CallToolRequest, in ParentReviewInput) (TextOutput, error) {
		dir, err := s.coursePath(in.Folder)
		if err != nil {
			return TextOutput{}, err
		}
		return s.command(ctx, "student-report", dir)
	})
	addActionTool(server, "save_draft_pdf", "Save a printable Draft PDF of an assignment, Page, QuizForge quiz or results file in the teacher's Downloads folder and open it, laid out for paper from the shared draft rendering. The teacher copy includes settings, answer keys and private notes; handout: true keeps only what students see, for physical copies. Returns the file path only; a results PDF can hold student names and work and is outside the workspace. Nothing is written to Canvas.", false, func(ctx context.Context, _ *mcp.CallToolRequest, in DraftPDFInput) (TextOutput, error) {
		path, err := s.coursePath(in.Path)
		if err != nil {
			return TextOutput{}, err
		}
		if in.Handout {
			return s.command(ctx, "draft-pdf", path, "--handout")
		}
		return s.command(ctx, "draft-pdf", path)
	})
	addReadTool(server, "list_modules", "List a configured course's Canvas modules through the CE-SE Engine.", func(ctx context.Context, _ *mcp.CallToolRequest, in CourseInput) (TextOutput, error) {
		return s.command(ctx, "modules", in.Course)
	})
	addReadTool(server, "list_group_sets", "List collaborative group sets in a configured course. Differentiation tags are excluded; use list_tags for those.", func(ctx context.Context, _ *mcp.CallToolRequest, in GroupSetsInput) (TextOutput, error) {
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
	addActionTool(server, "pull_assignment", "Pull an exact Canvas assignment and its edit baseline through the CE-SE Engine. Pass the assignment title, ID, or Canvas link in query (not assignment or assignment_id).", false, func(ctx context.Context, _ *mcp.CallToolRequest, in CourseQueryInput) (TextOutput, error) {
		return s.command(ctx, "pull-item", in.Course, in.Query)
	})
	addActionTool(server, "pull_quiz", "Pull a New Quiz's exact rich HTML and paginated items into a fresh new-quiz-edit draft. Edit changes maps, validate_document, then push_document. Stimulus and bank content stays read-only; this does not pull student work.", false, func(ctx context.Context, _ *mcp.CallToolRequest, in PullQuizInput) (TextOutput, error) {
		if in.AssignmentID <= 0 {
			return TextOutput{}, errors.New("assignment_id must be positive")
		}
		return s.command(ctx, "pull-quiz", in.Course, strconv.FormatInt(in.AssignmentID, 10))
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
	addActionTool(server, "replace_text", "Make a small exact correction in an existing course Page body or assignment description without regenerating it. Reads the live item into a fresh pulled file, holds if the optional baseline no longer matches or any find text matches zero or several places (unless all), and saves an edit of only the body. With push, the existing Page/assignment push sends it and keeps the edit if Canvas changed again; without push, the returned path waits for push_document after the teacher's go-ahead. Report held outcomes and their fresh version honestly.", true, func(ctx context.Context, _ *mcp.CallToolRequest, in ReplaceTextInput) (TextOutput, error) {
		args, err := replaceTextArgs(in)
		if err != nil {
			return TextOutput{}, err
		}
		return s.command(ctx, args...)
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
	addActionTool(server, "init_results", "Start a results file for a pulled individual packet under its assignment's scoring/ folder: the packet path and assignment snapshot, component/criterion field examples, each student's work-file path, exact current attempt, allowed operation and any discussion-evidence digest. An earlier interpretation and scoring_basis are reused; remaining judgment is blank and must be filled. For feedback without scores use comment-only with header none and omit components. Never writes to Canvas.", false, func(ctx context.Context, _ *mcp.CallToolRequest, in PathInput) (TextOutput, error) {
		if _, err := s.coursePath(in.Path); err != nil {
			return TextOutput{}, err
		}
		return s.command(ctx, "results-init", in.Path)
	})
	addReadTool(server, "validate_document", "Validate a CE-SE document offline. Optional show returns only one selected student's ref, alias, hold status and exact rendered comment; no submission or private teacher notes.", func(ctx context.Context, _ *mcp.CallToolRequest, in ValidateDocumentInput) (TextOutput, error) {
		if _, err := s.coursePath(in.Path); err != nil {
			return TextOutput{}, err
		}
		args := []string{"validate", in.Path}
		if in.Show != "" {
			args = append(args, "--show", in.Show)
		}
		return s.command(ctx, args...)
	})
	addReadTool(server, "review_status", "Check whether the teacher finished private family review; returns delivery outcomes or revision notes.", func(_ context.Context, _ *mcp.CallToolRequest, in ReviewStatusInput) (TextOutput, error) {
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
	case "set_quiz_accommodations", "set_tags", "list_tags", "assign_to", "assign_modules_by_tag", "audience_report", "set_file_access", "list_external_resources", "copy_assignment", "engine_info", "prepare_audience", "prepare_family_contact", "list_courses", "list_modules", "list_group_sets", "list_groups", "pull_group_set", "pull_group_memberships", "pull_assignment_dates", "pull_work", "pull_parent_report", "work_habits", "late_sweep", "personal_dates", "stad_teams", "stad_standings", "stad_publish", "stad_push_teams", "copy_module", "pull_assignment", "pull_quiz", "list_rubrics", "pull_rubric", "pull_page", "replace_text", "pull_discussion", "list_page_revisions", "inbox_list", "inbox_read", "inbox_reply", "inbox_set_state", "start_parent_review", "print_student_reports", "start_local_ui", "push_document", "post_results", "push_quiz", "upload_file", "list_quiz_points", "set_quiz_points", "canvas_get":
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

// staleAdapterNotice is returned instead of running a command when this adapter is older than
// the Engine now installed. After an update, os.Executable still names the old path, which now
// holds the new Engine, so an old adapter would start new child Engines (mixed versions) until
// the app is restarted. Status and update commands stay allowed so the agent can still explain.
const staleAdapterNotice = "CanvasExpert was updated. Close and reopen this app so it loads the new version; nothing was run."

func (s *Server) staleAdapter(args []string) error {
	if BuildKind != "release" || len(args) == 0 || args[0] == "version" || args[0] == "update" {
		return nil
	}
	m, ok, err := workspace.ReadEngineManifest(s.root)
	if err != nil || !ok || m.Identity == "" {
		return nil
	}
	if workspace.ArtifactIdentity(Version, BuildKind, Revision, BuiltAt) != m.Identity {
		return errors.New(staleAdapterNotice)
	}
	return nil
}

func (s *Server) command(ctx context.Context, args ...string) (TextOutput, error) {
	if err := s.staleAdapter(args); err != nil {
		return TextOutput{}, err
	}
	full := append([]string{"--workspace", s.root}, args...)
	// A stalled Canvas call must not hold the client's tool call forever.
	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()
	graceful := len(args) > 0 && gracefulStop[args[0]]
	var cmd *exec.Cmd
	switch {
	case s.newChild != nil:
		cmd = s.newChild(full)
	default:
		exe, err := os.Executable()
		if err != nil {
			return TextOutput{}, err
		}
		if graceful {
			cmd = exec.Command(exe, full...)
		} else {
			cmd = exec.CommandContext(ctx, exe, full...)
		}
	}
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	var err error
	if graceful {
		err = runGraceful(ctx, cmd)
	} else {
		err = cmd.Run()
	}
	result := strings.TrimSpace(stdout.String())
	if stderr.Len() > 0 {
		if result != "" {
			result += "\n"
		}
		result += strings.TrimSpace(stderr.String())
	}
	// A graceful stop that ran out of time is a normal, resumable result.
	if graceful && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		note := "Stopped at the time limit after finishing the item in progress; nothing was lost. Run the same tool again to continue with the items that were not started."
		if result != "" {
			result += "\n"
		}
		return TextOutput{Text: result + note, State: "incomplete", Outcome: exit3Outcome(args)}, nil
	}
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 3 {
			// Exit 3 means some items were held, stopped or not completed: the outcomes are the
			// result, not a tool failure.
			return TextOutput{Text: result, State: "incomplete", Outcome: exit3Outcome(args)}, nil
		}
		return TextOutput{}, fmt.Errorf("CE-SE command failed: %w\n%s", err, result)
	}
	return TextOutput{Text: result}, nil
}

// gracefulStop lists the batch writers whose child Engine finishes the item in flight and stops
// before the next one when its stdin is closed. Other commands are killed on cancel as before.
var gracefulStop = map[string]bool{"post": true, "push-quiz": true}

// stopGrace is how long a stopping child may finish its item before it is killed.
var stopGrace = 2 * time.Minute

// runGraceful runs cmd with a stdin pipe the child watches. When ctx ends (the client cancelled
// or the time limit passed) the pipe is closed so the child stops between items; a child that
// does not exit within stopGrace is killed, and the receipts reconcile on the next run.
func runGraceful(ctx context.Context, cmd *exec.Cmd) error {
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	if cmd.Env == nil {
		cmd.Env = os.Environ()
	}
	cmd.Env = append(cmd.Env, "CSE_STOP_ON_STDIN_CLOSE=1")
	if err = cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err = <-done:
		return err
	case <-ctx.Done():
	}
	_ = stdin.Close()
	select {
	case err = <-done:
		return err
	case <-time.After(stopGrace):
		_ = cmd.Process.Kill()
		return <-done
	}
}

// exit3Outcome names exit code 3: batch commands report partial progress, single-target
// commands report that they held.
func exit3Outcome(args []string) string {
	if len(args) > 0 {
		switch args[0] {
		case "post", "push", "push-quiz", "set-tags", "assign-to", "assign-modules", "quiz-accommodations", "upload-file", "prepare-audience", "inbox-set-state":
			return "partial"
		}
	}
	return "held"
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
	// FoundSync lists shared folders this PC could use but doesn't yet.
	FoundSync []string `json:"found_sync,omitempty"`
	SyncNote  string   `json:"sync_note,omitempty"`
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
		found := workspace.DiscoverUserSync(s.root)
		b, err := json.Marshal(configuredCourses{Source: "configured", Root: s.root, InstallRoot: location.InstallRoot, RulesRoot: location.RulesRoot, Mode: location.Mode, Instance: config.Instance, Courses: config.Courses, Team: team, FoundSync: found, SyncNote: workspace.UnconnectedSyncNote(found)})
		return TextOutput{Text: string(b)}, err
	default:
		return TextOutput{}, errors.New("source must be configured or canvas")
	}
}

// replaceTextArgs builds the replace-text command; arguments go to the Engine
// directly, never through a shell.
func replaceTextArgs(in ReplaceTextInput) ([]string, error) {
	if err := requireCourse(in.Course); err != nil {
		return nil, err
	}
	if in.Target != "page" && in.Target != "assignment" {
		return nil, errors.New("target must be page or assignment")
	}
	if len(in.Replacements) == 0 {
		return nil, errors.New("at least one replacement is required")
	}
	args := []string{"replace-text", in.Course, in.Target, in.Query}
	for _, r := range in.Replacements {
		args = append(args, "--find", r.Find, "--replace", r.Replace)
		if r.All {
			args = append(args, "--all")
		}
	}
	if in.ExpectUpdatedAt != "" {
		args = append(args, "--expect-updated-at", in.ExpectUpdatedAt)
	}
	if in.ExpectBodySHA256 != "" {
		args = append(args, "--expect-sha256", in.ExpectBodySHA256)
	}
	if in.Push {
		args = append(args, "--push")
	}
	return args, nil
}
