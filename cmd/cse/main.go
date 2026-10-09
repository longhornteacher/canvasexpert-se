package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	_ "time/tzdata"

	embedded "canvasexpertse"
	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/localui"
	"canvasexpertse/internal/platform"
	"canvasexpertse/internal/workspace"
)

var version = "0.1b"
var revision = "development"
var builtAt = "unrecorded"

const help = `CanvasExpert-SE 0.1b — the Engine: Canvas assignments and scoring

Usage: cse [--workspace PATH] COMMAND [ARGS]

  ui                            Open the local hub: Canvas setup, students, more
  link                          Record this program's location for workspace cse.cmd
  setup                         Connect Canvas privately on this machine
  students                      Edit student aliases and monitored status
  courses                       Discover authorized teacher courses
  modules COURSE                List a configured course's modules
  push FILE                     Create/resume or update an assignment
  pull-item COURSE TITLE|ID|LINK Pull actual assignment HTML for editing
  pull-work [COURSE [ASSIGNMENT]] Pull work; no scope uses current courses
  post RESULTS_FILE             Post validated grades or selected revisions
  validate FILE                 Validate a workspace document locally
  doctor                        Test native dummy credential round trip
  version                       Print version and build information
  help [formats|examples]       Print this help, the file-format reference, or the synthetic examples (no Canvas requests)

COURSE is an explicit nickname in context/courses.md. FILE paths are relative
to the workspace unless absolute. Use quotes around paths or titles with spaces.
ui/setup/students pages exist only while their command runs; Ctrl+C closes them.
Draft/show-me-first instructions stop before post. A scoring session pulls only.
Exit codes: 0 success; 2 invalid input; 3 partial/held work; 4 environment/API failure.
Read the playbooks (playbooks/setup.md, authoring.md, edit.md, score.md) as needed; use help formats and help examples for file formats.
`

// printEngineDoc prints the embedded format reference or synthetic examples.
func printEngineDoc(out io.Writer, topic string) error {
	if topic == "formats" {
		b, err := embedded.EngineDocs.ReadFile("engine-docs/formats.md")
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
	rootOpt := flags.String("workspace", "", "workspace directory")
	if e := flags.Parse(args); e != nil {
		if errors.Is(e, flag.ErrHelp) {
			fmt.Fprint(out, help)
			return 0
		}
		return 2
	}
	args = flags.Args()
	if len(args) > 0 && args[0] == "help" && len(args) == 2 && (args[1] == "formats" || args[1] == "examples") {
		if e := printEngineDoc(out, args[1]); e != nil {
			return fail(e, progress)
		}
		return 0
	}
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" {
		fmt.Fprint(out, help)
		return 0
	}
	cmd := args[0]
	args = args[1:]
	if cmd == "version" {
		if len(args) != 0 {
			return fail(inputError(fmt.Errorf("version takes no arguments")), progress)
		}
		fmt.Fprintf(out, "CanvasExpert-SE %s (SE = %s)\nrevision %s\nbuilt %s\n%s %s/%s\n", version, localui.SEExpansion(), revision, builtAt, runtime.Version(), runtime.GOOS, runtime.GOARCH)
		if info, ok := debug.ReadBuildInfo(); ok {
			for _, dep := range info.Deps {
				fmt.Fprintf(out, "dependency %s %s\n", dep.Path, dep.Version)
			}
		}
		return 0
	}
	if cmd == "doctor" {
		if len(args) != 0 {
			return fail(inputError(fmt.Errorf("doctor takes no arguments")), progress)
		}
		if e := platform.CredentialSelfTest(); e != nil {
			return fail(e, progress)
		}
		fmt.Fprintln(out, "Native dummy credential byte round trip passed; temporary credential removed.")
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
	if cmd == "ui" {
		if len(args) != 0 {
			return fail(inputError(fmt.Errorf("ui takes no arguments")), progress)
		}
		uiRoot, rootErr := workspace.ResolveRoot(*rootOpt, "")
		if rootErr != nil && *rootOpt != "" {
			return fail(inputError(rootErr), progress)
		}
		return fail(localui.RunHub(ctx, uiRoot, version, progress), progress)
	}
	if cmd == "setup" {
		if len(args) != 0 {
			return fail(inputError(fmt.Errorf("setup takes no arguments")), progress)
		}
		origin := ""
		setupRoot, rootErr := workspace.ResolveRoot(*rootOpt, "")
		var config domain.CourseConfig
		configBody := ""
		if rootErr == nil {
			var err error
			configBody, err = workspace.ReadDocument(filepath.Join(setupRoot, "context", "courses.md"), &config)
			if err != nil {
				return fail(inputError(err), progress)
			}
			origin = config.Instance
		} else if *rootOpt != "" {
			return fail(inputError(rootErr), progress)
		}
		if e := localui.SetupForOrigin(ctx, progress, origin); e != nil {
			return fail(e, progress)
		}
		if setupRoot != "" {
			credential, err := platform.LoadCredential()
			if err != nil {
				return fail(err, progress)
			}
			credential.Token = ""
			if len(config.Courses) == 0 || config.Instance == credential.Origin {
				config.Instance = credential.Origin
				if err = workspace.WriteDocument(filepath.Join(setupRoot, "context", "courses.md"), config, configBody); err != nil {
					return fail(err, progress)
				}
			} else {
				fmt.Fprintln(progress, "Saved Canvas origin differs from configured courses; update context/courses.md before remote commands.")
			}
		}
		return 0
	}
	known := map[string]bool{"students": true, "courses": true, "modules": true, "push": true, "pull-item": true, "pull-work": true, "post": true, "validate": true}
	if !known[cmd] {
		return fail(inputError(fmt.Errorf("unknown command %q; run cse help", cmd)), progress)
	}
	root, e := workspace.ResolveRoot(*rootOpt, "")
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
			e = validateDocument(p)
		}
		if e != nil {
			return fail(inputError(e), progress)
		}
		fmt.Fprintf(out, "Valid document: %s\n", p)
		return 0
	}
	return runRemote(ctx, root, cmd, args, out, progress)
}
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
func validateDocument(path string) error {
	b, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	kind, kindErr := workspace.DocumentKind(b)
	var target any
	switch kind {
	case "assignment":
		target = &domain.AssignmentDocument{}
	case "results":
		target = &domain.ResultsDocument{}
	case "packet":
		target = &domain.PacketDocument{}
	case "student-work":
		target = &domain.StudentWorkDocument{}
	default:
		switch filepath.Base(path) {
		case "courses.md":
			target = &domain.CourseConfig{}
		case "grading.md":
			target = &domain.GradingConfig{}
		case "students.md":
			target = &domain.StudentNotes{}
		default:
			return fmt.Errorf("unsupported document kind; expected assignment, results, packet, or student-work")
		}
	}
	if kindErr != nil && kind != "" {
		return kindErr
	}
	_, e = workspace.ReadDocument(path, target)
	return e
}
