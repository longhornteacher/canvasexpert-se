package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"time"
	_ "time/tzdata"

	embedded "canvasexpertse"
	"canvasexpertse/internal/doctor"
	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/installation"
	"canvasexpertse/internal/installlog"
	"canvasexpertse/internal/localui"
	"canvasexpertse/internal/mcpserver"
	"canvasexpertse/internal/platform"
	"canvasexpertse/internal/playbooks"
	"canvasexpertse/internal/workflow"
	"canvasexpertse/internal/workspace"
)

var version = embedded.ReleaseVersion()
var revision = "development"
var builtAt = "unrecorded"
var buildKind = "development"

var help = `CanvasExpert-SE ` + version + ` — the Engine: Canvas content authoring and scoring

Usage: cse [--workspace PATH] COMMAND [ARGS]

  ui                            Open the local hub: Canvas setup, students, more
  install [--client auto|work|cowork|antigravity]   Connect every detected app (or one) and check local MCP; opens private setup
  mcp                           Run the local MCP adapter over stdio
  link                          Record this program's location for workspace cse.cmd
  rules-location                Show teacher-rules authority; use --sync PATH or --local to set it
  playbooks status              Compare active teacher rules with this Engine's seed
  playbooks replace PATH --expected-sha256 HASH
                                Explicitly archive and replace one seed-managed file (HASH may be 'missing')
  setup                         Connect Canvas privately on this machine
  students                      Edit student aliases and monitored status
  courses                       Discover authorized teacher courses
  modules COURSE                List a configured course's modules
  external-resources COURSE [--group ID]  Read collaboration and conference metadata
  groups COURSE [--set ID] [--members GROUP_ID] [--groups] [--group ID]
                                List collaborative group sets/clubs; memberships require an exact group
  pull-group-set COURSE ID      Pull a collaborative group set and its editable baseline
  pull-group-memberships COURSE ID  Pull an exact group set's membership baseline
  push FILE                     Create/resume assignments, date overrides, Pages, discussions/replies, modules, group sets or membership plans
  pull-page COURSE ID|LINK       Pull exact Page HTML and baseline for editing
  pull-discussion COURSE TOPIC_ID Pull topic.md plus full/partial discussion packet (optional --group ID)
  page-history COURSE PAGE_ID   Read Page revisions (optional --group ID --revision ID)
  upload-file COURSE FILE --access course|hidden|locked [--folder ID] [--receipt PATH] [--new-upload]
                                Upload a course asset and save a reusable receipt
  pull-item COURSE TITLE|ID|LINK Pull actual assignment HTML for editing
  prepare-audience COURSE FILE [--refresh] Prepare individual club audience without Canvas writes
  pull-assignment-dates COURSE ASSIGNMENT_ID Pull exact audience override baselines for editing
  pull-work [COURSE [ASSIGNMENT]] [--group ID] [--session] Pull scoring-session work; no scope uses current courses
  prepare-late-timing FILE  Check first-genuine choices from a packet and propose late-only actions
  prepare-late-timing FILE  Check first-genuine choices from a packet and propose late-only actions
  prepare-late-timing FILE  Check first-genuine choices against pulled work and propose timing changes; never writes to Canvas
  parent-report COURSE [REF ...] Read parent-update facts (missing, corrections, due soon, history) into courses/COURSE/parent-updates/
  family-contact-prepare RUN_FOLDER Resolve a free-form contact.json audience against Canvas
  parent-review RUN_FOLDER      Open the private page to review and send a grade-update or family-contact run (Outlook)
  student-report RUN_FOLDER     Save one PDF of student reports (one per page) from a parent-update run and open it
  push-quiz COURSE FILE         Create or resume a New Quiz from a QuizForge file
  post RESULTS_FILE             Post validated grades or selected revisions
  quiz-points COURSE QUIZ [N=PTS ...] List a New Quiz's questions, or set question N to PTS points
  api PATH                      Read-only Canvas API GET when no command fits; prints the JSON
  validate FILE                 Validate a workspace document locally
  doctor [--json] [--offline]   Check this UserInstall; authenticated Canvas and hub checks unless offline
  version [--json]              Print version and build information
  review FILE [COURSE]  Open a local page where the teacher reviews scores (results file), a new assignment (create document) or a quiz (QuizForge file plus COURSE), revises some, and posts or creates the approved ones; exits when finished and prints what was posted, revised and held
  help [formats|fields|examples] Print this help, the file-format reference, Canvas field limits, or the synthetic examples (no Canvas requests)

COURSE is an explicit nickname in context/courses.md. FILE paths are relative
to the workspace unless absolute. Use quotes around paths or titles with spaces.
ui/setup/students pages exist only while their command runs; Ctrl+C closes them.
Draft/show-me-first instructions stop before post. A scoring session pulls only.
Exit codes: 0 success; 2 invalid input; 3 partial/held work; 4 environment/API failure.
Read the playbooks (playbooks/setup.md, authoring.md, native-content.md, edit.md, score.md, quizzes.md) as needed; use help formats and help examples for file formats, help fields for what Canvas keeps in each field.
`

// printEngineDoc prints the embedded format reference, field limits, or synthetic examples.
func printEngineDoc(out io.Writer, topic string) error {
	if doc := map[string]string{"formats": "formats.md", "fields": "field-limits.md"}[topic]; doc != "" {
		b, err := embedded.EngineDocs.ReadFile("engine-docs/" + doc)
		if err != nil {
			return err
		}
		_, err = out.Write(b)
		return err
	}
	entries, err := embedded.EngineDocs.ReadDir("engine-docs/examples")
	if err != nil {
		return err
	}
	for _, e := range entries {
		b, err := embedded.EngineDocs.ReadFile("engine-docs/examples/" + e.Name())
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "# %s\n\n%s\n", e.Name(), b)
	}
	return nil
}

type cliError struct {
	code int
	err  error
}

func (e *cliError) Error() string { return e.err.Error() }
func (e *cliError) Unwrap() error { return e.err }
func inputError(e error) error    { return &cliError{2, e} }
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}
func run(ctx context.Context, args []string, out, progress io.Writer) int {
	flags := flag.NewFlagSet("cse", flag.ContinueOnError)
	flags.SetOutput(progress)
	rootOpt := flags.String("workspace", "", "advanced: CE-SE installation directory for a development or extension Engine")
	if e := flags.Parse(args); e != nil {
		if errors.Is(e, flag.ErrHelp) {
			fmt.Fprint(out, help)
			return 0
		}
		return 2
	}
	args = flags.Args()
	if len(args) > 0 && args[0] == "help" && len(args) == 2 && (args[1] == "formats" || args[1] == "fields" || args[1] == "examples") {
		if e := printEngineDoc(out, args[1]); e != nil {
			return fail(e, progress)
		}
		return 0
	}
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" {
		if len(args) > 1 && args[0] == "help" {
			fmt.Fprintf(out, "No separate help for %q. Detailed help topics: formats, fields, examples. The command list below shows what each command does.\n\n", args[1])
		}
		fmt.Fprint(out, help)
		return 0
	}
	cmd := args[0]
	args = args[1:]
	if cmd == "playbooks" {
		root, e := runtimeRoot(*rootOpt)
		if e != nil {
			return fail(inputError(e), progress)
		}
		if len(args) == 1 && args[0] == "status" {
			location, locErr := workspace.GetRulesLocation(root)
			status := struct {
				Location          workspace.RulesLocation `json:"location"`
				AvailabilityError string                  `json:"availability_error,omitempty"`
				Files             []playbooks.StatusRow   `json:"files"`
			}{Location: location, Files: playbooks.Status(root)}
			if locErr != nil {
				status.AvailabilityError = locErr.Error()
			}
			b, e := json.MarshalIndent(status, "", "  ")
			if e != nil {
				return fail(e, progress)
			}
			fmt.Fprintln(out, string(b))
			return 0
		}
		if len(args) >= 2 && args[0] == "replace" {
			var rel, expected string
			if len(args) == 4 && args[2] == "--expected-sha256" {
				rel, expected = args[1], args[3]
			} else if len(args) == 4 && args[1] == "--expected-sha256" {
				expected, rel = args[2], args[3]
			}
			if strings.TrimSpace(rel) == "" || strings.TrimSpace(expected) == "" {
				return fail(inputError(fmt.Errorf("use playbooks replace PATH --expected-sha256 HASH")), progress)
			}
			if e := playbooks.Replace(root, rel, expected, time.Now()); e != nil {
				return fail(e, progress)
			}
			fmt.Fprintf(out, "%s replaced with this Engine's seed; previous copy archived under playbook-old.\n", rel)
			return 0
		}
		if len(args) == 1 && args[0] == "update" {
			report, e := playbooks.Update(root, time.Now())
			if e != nil {
				return fail(e, progress)
			}
			b, e := json.MarshalIndent(report, "", "  ")
			if e != nil {
				return fail(e, progress)
			}
			fmt.Fprintln(out, string(b))
			return 0
		}
		return fail(inputError(fmt.Errorf("use playbooks status, playbooks update, or playbooks replace PATH --expected-sha256 HASH")), progress)
	}
	if cmd == "rules-location" {
		root, e := runtimeRoot(*rootOpt)
		if e != nil {
			return fail(inputError(e), progress)
		}
		locationFlags := flag.NewFlagSet("rules-location", flag.ContinueOnError)
		locationFlags.SetOutput(progress)
		syncRoot := locationFlags.String("sync", "", "explicit shared teacher-rules directory")
		local := locationFlags.Bool("local", false, "use this UserInstall as the teacher-rules authority")
		if e := locationFlags.Parse(args); e != nil {
			return 2
		}
		if locationFlags.NArg() != 0 || (*local && *syncRoot != "") {
			return fail(inputError(fmt.Errorf("use rules-location [--sync PATH | --local]")), progress)
		}
		if *local || *syncRoot != "" {
			location, e := workspace.SetRulesLocation(root, *syncRoot, embedded.WorkspaceTemplate, "workspace-template")
			if e != nil {
				return fail(e, progress)
			}
			fmt.Fprintf(out, "mode: %s\ninstall_root: %s\nrules_root: %s\n", location.Mode, location.InstallRoot, location.RulesRoot)
			return 0
		}
		location, e := workspace.GetRulesLocation(root)
		fmt.Fprintf(out, "mode: %s\ninstall_root: %s\nrules_root: %s\n", location.Mode, location.InstallRoot, location.RulesRoot)
		if e != nil {
			fmt.Fprintf(progress, "Teacher rules location unavailable: %v\n", e)
		}
		return 0
	}
	if cmd == "mcp" {
		if len(args) != 0 {
			return fail(inputError(fmt.Errorf("mcp takes no arguments")), progress)
		}
		root, e := runtimeRoot(*rootOpt)
		if e != nil {
			return fail(inputError(e), progress)
		}
		mcpserver.Version, mcpserver.Revision, mcpserver.BuiltAt = version, revision, builtAt
		mcpserver.BuildKind = buildKind
		return fail(mcpserver.Run(ctx, root), progress)
	}
	if cmd == "version" {
		if len(args) == 1 && args[0] == "--json" {
			data, _ := json.Marshal(struct {
				Version   string `json:"version"`
				Revision  string `json:"revision"`
				BuiltAt   string `json:"built_at"`
				BuildKind string `json:"build_kind"`
			}{version, revision, builtAt, buildKind})
			fmt.Fprintln(out, string(data))
			return 0
		}
		if len(args) != 0 {
			return fail(inputError(fmt.Errorf("use version [--json]")), progress)
		}
		fmt.Fprintf(out, "CanvasExpert-SE %s (SE = %s)\nrevision %s\nbuilt %s\nbuild_kind %s\n%s %s/%s\n", version, localui.SEExpansion(), revision, builtAt, buildKind, runtime.Version(), runtime.GOOS, runtime.GOARCH)
		if info, ok := debug.ReadBuildInfo(); ok {
			for _, dep := range info.Deps {
				fmt.Fprintf(out, "dependency %s %s\n", dep.Path, dep.Version)
			}
		}
		return 0
	}
	if cmd == "doctor" {
		doctorFlags := flag.NewFlagSet("doctor", flag.ContinueOnError)
		doctorFlags.SetOutput(progress)
		jsonOutput := doctorFlags.Bool("json", false, "emit JSON checklist")
		offline := doctorFlags.Bool("offline", false, "skip Canvas and local hub network checks")
		if e := doctorFlags.Parse(args); e != nil {
			return 2
		}
		if doctorFlags.NArg() != 0 {
			return fail(inputError(fmt.Errorf("use doctor [--json] [--offline]")), progress)
		}
		root, e := runtimeRoot(*rootOpt)
		if e != nil {
			return fail(inputError(e), progress)
		}
		exe, e := os.Executable()
		if e != nil {
			return fail(e, progress)
		}
		build := fmt.Sprintf("version=%s revision=%s built_at=%s kind=%s", version, revision, builtAt, buildKind)
		report := doctor.RunMode(root, exe, build, *offline)
		if *jsonOutput {
			data, marshalErr := report.JSON()
			if marshalErr != nil {
				return fail(marshalErr, progress)
			}
			fmt.Fprintln(out, string(data))
		} else {
			fmt.Fprint(out, report.Human())
		}
		if report.Failed {
			return 4
		}
		return 0
	}
	if cmd == "link" {
		if len(args) != 0 {
			return fail(inputError(fmt.Errorf("link takes no arguments")), progress)
		}
		exe, e := platform.LinkProgram()
		if e != nil {
			return fail(e, progress)
		}
		fmt.Fprintf(out, "Linked this machine to %s\n", exe)
		return 0
	}
	if cmd == "install" {
		root, err := runtimeRoot(*rootOpt)
		if err != nil {
			return fail(inputError(err), progress)
		}
		installFlags := flag.NewFlagSet("install", flag.ContinueOnError)
		installFlags.SetOutput(progress)
		client := installFlags.String("client", "", "local destination: auto (default, every detected app), all, work, cowork or antigravity")
		replace := installFlags.Bool("replace-existing", false, "explicitly replace a CE-SE connection targeting another UserInstall")
		checkOnly := installFlags.Bool("check-only", false, "check bundled local MCP without changing client settings")
		noUI := installFlags.Bool("no-ui", false, "do not launch private setup (advanced/testing)")
		inspect := installFlags.Bool("inspect", false, "inspect client configuration-directory hints without changes")
		if err := installFlags.Parse(args); err != nil {
			return 2
		}
		if *inspect && installFlags.NArg() == 0 {
			data, _ := json.Marshal(installation.Inspect(root))
			fmt.Fprintln(out, string(data))
			return 0
		}
		if *client == "" || *client == "all" {
			*client = "auto"
		}
		if installFlags.NArg() != 0 || (*client != "auto" && *client != "work" && *client != "cowork" && *client != "antigravity") {
			return fail(inputError(fmt.Errorf("use install [--client auto|work|cowork|antigravity]")), progress)
		}
		// Launch independently of registration, including a client trust/restart requirement.
		var installErrors []error
		logEvent := func(step, outcome, category, selected string) {
			if e := installlog.Append(root, version, installlog.Entry{Step: step, Client: selected, Outcome: outcome, ErrorCategory: category, InstallPath: root}); e != nil {
				fmt.Fprintln(progress, "Install log unavailable; setup continues.")
			}
		}
		if installUIEnabled(*checkOnly, *noUI) {
			if err := reportInstallUI(ctx, root, *client, launchInstallUI, out, progress, logEvent); err != nil {
				installErrors = append(installErrors, err)
			}
		}
		if !*checkOnly {
			reportDesktopShortcut(root, ensureDesktopShortcut, out, progress)
			var results []installation.Result
			var err error
			if *client == "auto" {
				results, err = installation.RegisterAuto(root, *replace)
				if len(results) == 0 && err == nil {
					err = errors.New("no supported app was found on this computer (Claude, ChatGPT/Codex or Antigravity); install one, or name it with --client")
				}
			} else {
				var result installation.Result
				if result, err = installation.Register(root, *client, *replace); err == nil {
					results = []installation.Result{result}
				}
			}
			if err != nil {
				logEvent("registration", "failed", "registration", *client)
				installErrors = append(installErrors, err)
			}
			for _, result := range results {
				logEvent("registration", "succeeded", "", result.Client)
				if result.Client == "cowork" {
					result = installation.CoworkLaunchOutcome(result, !*noUI, installation.DefaultLauncher)
					logEvent("cowork_extension", result.CoworkLaunchOutcome, "", result.Client)
				}
				data, _ := json.Marshal(result)
				fmt.Fprintln(out, string(data))
			}
		}
		check, err := installation.Check(ctx, root)
		outcome, category := "succeeded", ""
		if err != nil {
			outcome, category = "failed", "local_check"
		}
		logEvent("local_mcp_check", outcome, category, *client)
		data, _ := json.Marshal(check)
		fmt.Fprintln(out, string(data))
		if err != nil {
			installErrors = append(installErrors, err)
		}
		return fail(errors.Join(installErrors...), progress)
	}
	if cmd == "ui" {
		uiFlags := flag.NewFlagSet("ui", flag.ContinueOnError)
		uiFlags.SetOutput(progress)
		setupFirst := uiFlags.Bool("setup-first", false, "open private Canvas setup first when no saved connection exists")
		installClient := uiFlags.String("install-client", "", "selected client for private install setup events")
		readyFile := uiFlags.String("ready-file", "", "advanced: write non-secret local startup result")
		if err := uiFlags.Parse(args); err != nil {
			return 2
		}
		if uiFlags.NArg() != 0 {
			return fail(inputError(fmt.Errorf("use ui [--setup-first]")), progress)
		}
		uiRoot, rootErr := runtimeRoot(*rootOpt)
		if rootErr != nil && *rootOpt != "" {
			return fail(inputError(rootErr), progress)
		}
		return fail(localui.RunHubWithOptions(ctx, uiRoot, version, progress, localui.HubOptions{SetupFirst: *setupFirst, ReadyFile: *readyFile, InstallClient: *installClient}), progress)
	}
	if cmd == "setup" {
		if len(args) != 0 {
			return fail(inputError(fmt.Errorf("setup takes no arguments")), progress)
		}
		origin := ""
		setupRoot, rootErr := runtimeRoot(*rootOpt)
		if rootErr == nil {
			var config domain.CourseConfig
			_, err := workspace.ReadTeacherDocument(setupRoot, "context/courses.md", &config)
			if err != nil {
				return fail(inputError(err), progress)
			}
			if !strings.Contains(config.Instance, "canvas.example.edu") {
				origin = config.Instance
			}
		} else if *rootOpt != "" {
			return fail(inputError(rootErr), progress)
		}
		if e := localui.SetupForRoot(ctx, progress, origin, setupRoot); e != nil {
			return fail(e, progress)
		}
		if setupRoot != "" {
			credential, err := platform.LoadCredential()
			if err != nil {
				return fail(err, progress)
			}
			credential.Token = ""
			changed, err := workspace.SyncInstance(setupRoot, credential.Origin)
			if err != nil {
				return fail(fmt.Errorf("Canvas credential was saved, but context/courses.md could not be updated; fix that file before remote commands: %w", err), progress)
			}
			if !changed {
				fmt.Fprintln(progress, "Canvas credential was saved, but course settings were not changed. "+workspace.OriginMismatchMessage(workspace.ConfiguredInstance(setupRoot), credential.Origin))
			}
		}
		return 0
	}
	known := map[string]bool{"students": true, "courses": true, "modules": true, "external-resources": true, "groups": true, "pull-group-set": true, "pull-group-memberships": true, "push": true, "pull-page": true, "pull-discussion": true, "page-history": true, "upload-file": true, "pull-item": true, "pull-assignment-dates": true, "prepare-audience": true, "pull-work": true, "prepare-late-timing": true, "parent-report": true, "family-contact-prepare": true, "parent-review": true, "student-report": true, "post": true, "review": true, "push-quiz": true, "validate": true, "api": true, "quiz-points": true}
	if !known[cmd] {
		return fail(inputError(fmt.Errorf("unknown command %q; run cse help", cmd)), progress)
	}
	root, e := runtimeRoot(*rootOpt)
	if e != nil {
		return fail(inputError(e), progress)
	}
	if cmd == "students" {
		if len(args) != 0 {
			return fail(inputError(fmt.Errorf("students takes no arguments")), progress)
		}
		return fail(localui.Students(ctx, root, progress), progress)
	}
	if cmd == "validate" {
		if len(args) != 1 {
			return fail(inputError(fmt.Errorf("validate requires one file")), progress)
		}
		p, e := documentPath(root, args[0])
		if e == nil {
			e = validateDocument(p, root)
		}
		if e != nil {
			return fail(inputError(e), progress)
		}
		fmt.Fprintf(out, "Valid document: %s\n", p)
		previewResults(p, out)
		return 0
	}
	return runRemote(ctx, root, cmd, args, out, progress)
}

func installUIEnabled(checkOnly, noUI bool) bool { return !checkOnly && !noUI }
func fail(e error, w io.Writer) int {
	if e == nil {
		return 0
	}
	fmt.Fprintln(w, e.Error())
	var cli *cliError
	if errors.As(e, &cli) {
		return cli.code
	}
	return 4
}
func documentPath(root, name string) (string, error) {
	if filepath.IsAbs(name) {
		absolute, e := filepath.Abs(name)
		if e != nil {
			return "", e
		}
		relative, e := filepath.Rel(root, absolute)
		if e != nil {
			return "", e
		}
		return workspace.ConstrainPath(root, relative)
	}
	return workspace.ConstrainPath(root, name)
}

// previewResults prints each student's computed assessment and late timing so the
// agent can show the teacher a preview without posting.
func previewResults(path string, out io.Writer) {
	var doc domain.ResultsDocument
	if _, e := workspace.ReadDocument(path, &doc); e != nil {
		return
	}
	rows, e := domain.ValidateResults(&doc)
	if e != nil {
		return
	}
	for _, v := range rows {
		g := v.Student.Grade
		if v.Student.Operation == "late-only" && v.Student.LateOnly != nil {
			fmt.Fprintf(out, "%s  late-only  late: %s\n", v.Student.StudentRef, lateText(v.Student.LateOnly.LateTiming))
			continue
		}
		if g == nil {
			continue
		}
		late := "—"
		if g.LateTiming != nil {
			late = lateText(*g.LateTiming)
		}
		if v.Hold != "" {
			fmt.Fprintf(out, "%s  held: %s\n", v.Student.StudentRef, v.Hold)
			continue
		}
		fmt.Fprintf(out, "%s  %s  late: %s\n", v.Student.StudentRef, g.Assessment.String(), late)
	}
}

// lateText shows seconds_late as school days (24 h each) plus hours.
func lateText(t domain.LateTiming) string {
	if t.Status != "late" {
		return "none"
	}
	hours := t.SecondsLate / 3600
	return fmt.Sprintf("%dd %dh", hours/24, hours%24)
}
func validateDocument(path string, roots ...string) error {
	b, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	kind, kindErr := workspace.DocumentKind(b)
	var target any
	switch kind {
	case "assignment":
		target = &domain.AssignmentDocument{}
	case "assignment-dates":
		target = &domain.AssignmentDatesDocument{}
	case "group-set":
		target = &domain.GroupSetDocument{}
	case "group-memberships":
		target = &domain.GroupMembershipDocument{}
	case "group-packet":
		target = &domain.GroupPacketDocument{}
	case "group-work":
		target = &domain.GroupWorkDocument{}
	case "page":
		target = &domain.PageDocument{}
	case "discussion":
		target = &domain.DiscussionDocument{}
	case "discussion-packet":
		target = &domain.DiscussionPacket{}
	case "discussion-reply":
		target = &domain.DiscussionReplyDocument{}
	case "module":
		target = &domain.ModuleDocument{}
	case "results":
		target = &domain.ResultsDocument{}
	case "group-results":
		target = &domain.GroupResultsDocument{}
	case "packet":
		target = &domain.PacketDocument{}
	case "student-work":
		target = &domain.StudentWorkDocument{}
	case "late-timing-decisions":
		target = &workflow.LateTimingDecisionsDocument{}
	default:
		switch filepath.Base(path) {
		case "courses.md":
			target = &domain.CourseConfig{}
		case "grading.md":
			target = &domain.GradingConfig{}
		case "students.md":
			target = &domain.StudentNotes{}
		default:
			return fmt.Errorf("unsupported document kind %q; expected assignment, assignment-dates, group-set, group-memberships, group-packet, group-work, page, discussion, discussion-packet, discussion-reply, module, results, group-results, packet, or student-work", kind)
		}
	}
	if kindErr != nil && kind != "" {
		return kindErr
	}
	_, e = workspace.ReadDocument(path, target)
	if e == nil {
		if d, ok := target.(*workflow.LateTimingDecisionsDocument); ok {
			e = d.Validate()
		}
		if _, ok := target.(*domain.AssignmentDocument); ok {
			var doc domain.AssignmentDocument
			body, readErr := workspace.ReadDocument(path, &doc)
			if readErr != nil {
				return readErr
			}
			e = domain.ValidateAssignment(doc, body)
		}
		if d, ok := target.(*domain.AssignmentDatesDocument); ok {
			e = domain.ValidateAssignmentDates(*d)
		}
		if d, ok := target.(*domain.GroupSetDocument); ok {
			e = d.Validate()
		}
		if d, ok := target.(*domain.GroupMembershipDocument); ok {
			e = d.Validate()
		}
		if d, ok := target.(*domain.GroupPacketDocument); ok {
			e = d.Validate()
		}
		if d, ok := target.(*domain.GroupWorkDocument); ok {
			e = d.Validate()
		}
		if d, ok := target.(*domain.GroupResultsDocument); ok {
			e = d.Validate()
		}
		if d, ok := target.(*domain.DiscussionPacket); ok {
			e = d.Validate()
		}
		if _, ok := target.(*domain.DiscussionDocument); ok && e == nil {
			var doc domain.DiscussionDocument
			body, readErr := workspace.ReadDocument(path, &doc)
			if readErr != nil {
				return readErr
			}
			e = doc.ValidateBody(body)
		}
		if _, ok := target.(*domain.DiscussionReplyDocument); ok && e == nil {
			var doc domain.DiscussionReplyDocument
			body, readErr := workspace.ReadDocument(path, &doc)
			if readErr != nil {
				return readErr
			}
			e = doc.ValidateBody(body)
		}
	}
	return e
}

func runtimeRoot(explicit string) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if err := workspace.BootstrapBundled(exe, embedded.WorkspaceTemplate, "workspace-template"); err != nil {
		return "", fmt.Errorf("prepare this CanvasExpert folder: %w", err)
	}
	return workspace.ResolveRuntimeRoot(explicit, "", exe)
}
