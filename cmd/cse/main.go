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

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/localui"
	"canvasexpertse/internal/platform"
	"canvasexpertse/internal/workspace"
)

var version = "0.1b"
var revision = "development"
var builtAt = "unrecorded"

const help = `CanvasExpert-SE 0.1b — portable Canvas assignments and scoring

Usage: cse [--workspace PATH] COMMAND [ARGS]

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
  help                          Print this help, without Canvas requests

COURSE is an explicit nickname in settings/courses.md. FILE paths are relative
to the workspace unless absolute. Use quotes around paths or titles with spaces.
setup/students pages exist only while their command runs; Ctrl+C closes them.
Draft/show-me-first instructions stop before post. A scoring session pulls only.
Exit codes: 0 success; 2 invalid input; 3 partial/held work; 4 environment/API failure.
Read guides/setup.md, authoring.md, edit.md, score.md, and formats.md as needed.
`

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
			configBody, err = workspace.ReadDocument(filepath.Join(setupRoot, "settings", "courses.md"), &config)
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
				if err = workspace.WriteDocument(filepath.Join(setupRoot, "settings", "courses.md"), config, configBody); err != nil {
					return fail(err, progress)
				}
			} else {
				fmt.Fprintln(progress, "Saved Canvas origin differs from configured courses; update settings/courses.md before remote commands.")
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
