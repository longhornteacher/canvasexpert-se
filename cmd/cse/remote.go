package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"canvasexpertse/internal/canvas"
	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/localui"
	"canvasexpertse/internal/platform"
	"canvasexpertse/internal/workflow"
	"canvasexpertse/internal/workspace"
)

func runRemote(ctx context.Context, root, cmd string, args []string, out, progress io.Writer) int {
	arities := map[string][2]int{"courses": {0, 0}, "modules": {1, 1}, "push": {1, 1}, "pull-page": {2, 2}, "upload-file": {3, 12}, "pull-item": {2, 2}, "pull-work": {0, 2}, "parent-report": {1, 200}, "parent-review": {1, 1}, "student-report": {1, 1}, "post": {1, 1}, "review": {1, 2}, "push-quiz": {2, 2}, "api": {1, 1}, "quiz-points": {2, 200}}
	n := arities[cmd]
	if len(args) < n[0] || len(args) > n[1] {
		return fail(inputError(fmt.Errorf("wrong arguments for %s; run cse help", cmd)), progress)
	}
	var file string
	var uploadOptions workflow.CourseUploadOptions
	var uploadSource string
	if cmd == "upload-file" {
		var e error
		uploadSource, uploadOptions, e = parseCourseUpload(root, args)
		if e != nil {
			return fail(inputError(e), progress)
		}
	}
	if cmd == "push-quiz" {
		p, e := documentPath(root, args[1])
		if e != nil {
			return fail(inputError(e), progress)
		}
		if _, e = workflow.ReadQuizForge(p); e != nil {
			return fail(inputError(e), progress)
		}
		file = p
	}
	quizReview := false
	if cmd == "parent-review" || cmd == "student-report" {
		p, e := documentPath(root, args[0])
		if e != nil {
			return fail(inputError(e), progress)
		}
		file = p
	}
	if cmd == "review" {
		p, e := documentPath(root, args[0])
		if e != nil {
			return fail(inputError(e), progress)
		}
		course := ""
		if len(args) > 1 {
			course = args[1]
		}
		kind, e := workflow.ValidateReviewInput(root, p, course)
		if e != nil {
			return fail(inputError(e), progress)
		}
		file, quizReview = p, kind == "quiz"
	}
	if cmd == "push" || cmd == "post" {
		p, e := documentPath(root, args[0])
		if e != nil {
			return fail(inputError(e), progress)
		}
		file = p
		if e = validateDocument(file, root); e != nil {
			return fail(inputError(e), progress)
		}
		b, e := os.ReadFile(file)
		if e != nil {
			return fail(inputError(e), progress)
		}
		kind := documentKind(b)
		if cmd == "push" && kind != "assignment" && kind != "page" && kind != "module" {
			return fail(inputError(fmt.Errorf("push supports assignment, page, or module documents")), progress)
		}
		if cmd == "post" && kind != "results" {
			return fail(inputError(fmt.Errorf("post requires a results document")), progress)
		}
	}
	var config domain.CourseConfig
	if _, e := workspace.ReadTeacherDocument(root, "context/courses.md", &config); e != nil {
		return fail(inputError(e), progress)
	}
	credential, e := platform.LoadCredential()
	if e != nil {
		return fail(e, progress)
	}
	c, e := canvas.New(credential.Origin, credential.Token)
	credential.Token = ""
	if e != nil {
		return fail(e, progress)
	}
	configuredOrigin, originErr := canvas.New(config.Instance, "validate-origin-only")
	if originErr != nil || configuredOrigin.Origin() != c.Origin() {
		return fail(inputError(errors.New(workspace.OriginMismatchMessage(config.Instance, c.Origin()))), progress)
	}
	service := &workflow.Service{Root: root, Client: c, Config: config}
	start := time.Now()
	status := statusWriter(progress)
	defer func() {
		fmt.Fprintf(status, "%s: %s, %d HTTP requests\n", cmd, time.Since(start).Round(time.Millisecond), c.Requests())
	}()
	fmt.Fprintf(status, "%s…\n", cmd)
	table := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	switch cmd {
	case "upload-file":
		report, e := service.UploadCourseFile(ctx, args[0], uploadSource, uploadOptions)
		fmt.Fprintf(out, "%s: %s\n", report.Status, report.Path)
		if report.Receipt.File.ID > 0 {
			fmt.Fprintf(out, "Canvas file %d: %s (%d bytes, %s)\n%s\n", report.Receipt.File.ID, report.Receipt.File.DisplayName, report.Receipt.File.Size, report.Receipt.File.ContentType, report.Receipt.URL)
			f := report.Receipt.File
			fmt.Fprintf(out, "Access requested: %s; actual locked: %s, hidden: %s, visibility: %s, upload status: %s, lock at: %s, unlock at: %s\n", report.Receipt.Access, optionalBool(f.Locked), optionalBool(f.Hidden), knownText(f.VisibilityLevel), knownText(f.UploadStatus), knownText(f.LockAt), knownText(f.UnlockAt))
		}
		if report.Message != "" {
			fmt.Fprintln(out, report.Message)
		}
		if e != nil {
			if report.Status == "held" {
				fmt.Fprintln(progress, e)
				return 3
			}
			return fail(workflowError(e), progress)
		}
		if report.Status != "completed" {
			return 3
		}
	case "pull-page":
		p, e := service.PullPage(ctx, args[0], args[1])
		if e != nil {
			return fail(workflowError(e), progress)
		}
		fmt.Fprintln(out, p)
	case "quiz-points":
		changes := map[int]domain.Decimal{}
		for _, arg := range args[2:] {
			pos, pts, ok := strings.Cut(arg, "=")
			n, e := strconv.Atoi(pos)
			d, de := domain.ParseDecimal(pts)
			if !ok || e != nil || n <= 0 || de != nil {
				return fail(inputError(fmt.Errorf("quiz-points changes look like 2=50 (question position = points), got %q", arg)), progress)
			}
			changes[n] = d
		}
		report, e := service.QuizPoints(ctx, args[0], args[1], changes)
		if report.AssignmentID > 0 {
			fmt.Fprintf(out, "%s: New Quiz %d %s\n%s\n", report.Status, report.AssignmentID, report.Title, report.URL)
			fmt.Fprintln(table, "Position\tType\tPoints\tWas\tQuestion")
			for _, it := range report.Items {
				fmt.Fprintf(table, "%d\t%s\t%s\t%s\t%s\n", it.Position, it.Type, it.Points, it.Was, it.Stem)
			}
			_ = table.Flush()
			fmt.Fprintf(out, "Items total %s; quiz worth %s; assignment worth %s; %d changed\n", report.ItemTotal, report.QuizPoints, report.AssignmentPoints, report.Changed)
		}
		if report.Message != "" {
			fmt.Fprintln(out, report.Message)
		}
		if e != nil {
			return fail(workflowError(e), progress)
		}
		if report.Status == "held" || report.Status == "partial" {
			return 3
		}
	case "api":
		return fail(apiGet(ctx, c, args[0], out), progress)
	case "courses":
		list, e := workflow.Courses(ctx, c)
		if e != nil {
			return fail(e, progress)
		}
		fmt.Fprintln(table, "ID\tCourse\tCode\tState")
		for _, item := range list {
			fmt.Fprintf(table, "%d\t%s\t%s\t%s\n", item.ID, compact(item.Name), compact(item.Code), compact(item.State))
		}
		_ = table.Flush()
	case "modules":
		course, e := service.Course(args[0])
		if e != nil {
			return fail(inputError(e), progress)
		}
		list, e := workflow.Modules(ctx, c, course.ID)
		if e != nil {
			return fail(e, progress)
		}
		fmt.Fprintln(table, "ID\tPosition\tModule")
		for _, item := range list {
			fmt.Fprintf(table, "%d\t%d\t%s\n", item.ID, item.Position, compact(item.Name))
		}
		_ = table.Flush()
	case "pull-item":
		p, e := service.PullItem(ctx, args[0], args[1])
		if e != nil {
			return fail(e, progress)
		}
		fmt.Fprintln(out, p)
	case "push":
		b, _ := os.ReadFile(file)
		if kind := documentKind(b); kind == "page" {
			report, e := service.PushPage(ctx, file)
			fmt.Fprintf(out, "%s: Page %d %s\n", report.Status, report.PageID, report.Path)
			if report.URL != "" {
				fmt.Fprintln(out, report.URL)
			}
			if report.Message != "" {
				fmt.Fprintln(out, report.Message)
			}
			if e != nil {
				return fail(workflowError(e), progress)
			}
			if report.Status != "completed" && report.Status != "succeeded" {
				return 3
			}
			return 0
		} else if kind == "module" {
			report, e := service.PushModule(ctx, file)
			fmt.Fprintf(out, "%s: module %d, %d of %d entries, published: %t\n%s\n", report.Status, report.ModuleID, report.ItemsAdded, report.ItemsTotal, report.Published, file)
			for _, item := range report.Items {
				fmt.Fprintf(out, "Entry %d %s %d: item %d, %s %s\n", item.Index+1, item.Type, item.ContentID, item.ModuleItemID, item.Outcome.Status, item.Outcome.Message)
			}
			if report.ChildAccess != "" {
				fmt.Fprintln(out, report.ChildAccess)
			}
			if report.Message != "" {
				fmt.Fprintln(out, report.Message)
			}
			if e != nil {
				return fail(workflowError(e), progress)
			}
			if report.Status != "completed" && report.Status != "succeeded" {
				return 3
			}
			return 0
		}
		report, e := service.Push(ctx, file)
		if report.Status != "" {
			fmt.Fprintf(out, "%s: %s\n", report.Status, report.Path)
			if report.URL != "" {
				fmt.Fprintln(out, report.URL)
			}
			if report.Message != "" {
				fmt.Fprintln(out, report.Message)
			}
		}
		if e != nil {
			return fail(workflowError(e), progress)
		}
		if report.Status != "completed" && report.Status != "succeeded" {
			return 3
		}
	case "push-quiz":
		report, e := service.PushQuiz(ctx, args[0], file)
		for _, warning := range report.Warnings {
			fmt.Fprintln(progress, "Warning:", warning)
		}
		if report.Path != "" {
			fmt.Fprintln(out, "Results:", report.Path)
		}
		if report.AssignmentID > 0 {
			fmt.Fprintf(out, "%s: New Quiz %d, %d of %d items, published: %t\n%s\n", report.Status, report.AssignmentID, report.ItemsAdded, report.ItemsTotal, report.Published, report.URL)
		}
		if report.Hub != "" {
			fmt.Fprintln(out, report.Hub)
		}
		if report.Message != "" {
			fmt.Fprintln(out, report.Message)
		}
		if e != nil {
			return fail(workflowError(e), progress)
		}
	case "parent-report":
		courseList, refList := splitParentArgs(args)
		path, packet, e := service.ParentReport(ctx, courseList, refList, time.Now())
		if e != nil {
			return fail(workflowError(e), progress)
		}
		fmt.Fprintln(out, path)
		fmt.Fprintf(out, "%d students\n", len(packet.Students))
	case "pull-work":
		nickname, query := "", ""
		if len(args) > 0 {
			nickname = args[0]
		}
		if len(args) > 1 {
			query = args[1]
		}
		report, e := service.PullWork(ctx, nickname, query)
		if len(report.Assignments) > 0 {
			fmt.Fprintln(table, "Course\tAssignment\tTitle\tPulled\tLate\tMissing\tExcused\tInactive\tUnsupported\tHandled\tUnreadable attachments\tDetails")
			for _, summary := range report.Assignments {
				fmt.Fprintf(table, "%s\t%d\t%s\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%s\n", compact(summary.Course), summary.AssignmentID, compact(summary.Title), summary.Students, summary.Late, summary.Missing, summary.Excused, summary.Inactive, summary.Unsupported, summary.Handled, summary.UnreadableAttachments, compact(summary.SkipReason))
			}
			_ = table.Flush()
		}
		for _, p := range report.Packets {
			fmt.Fprintln(out, p)
		}
		fmt.Fprintf(out, "%d students in %d packets\n", report.Students, len(report.Packets))
		counts := map[string]int{}
		order := []string{}
		for _, m := range report.Skipped {
			if counts[m] == 0 {
				order = append(order, m)
			}
			counts[m]++
		}
		for _, m := range order {
			if counts[m] > 1 {
				fmt.Fprintf(progress, "Skipped: %s (%d rows)\n", m, counts[m])
			} else {
				fmt.Fprintln(progress, "Skipped:", m)
			}
		}
		for _, m := range report.Warnings {
			fmt.Fprintln(progress, "Note:", m)
		}
		fmt.Fprintf(progress, "Discovery: %s, %d HTTP requests; fetch/download: %s, %d HTTP requests\n", report.DiscoveryElapsed.Round(time.Millisecond), report.DiscoveryRequests, report.FetchElapsed.Round(time.Millisecond), report.FetchRequests)
		if e != nil {
			return fail(e, progress)
		}
	case "student-report":
		sheets, label, e := service.BuildStudentSheets(file)
		if e != nil {
			return fail(inputError(e), progress)
		}
		saved, e := localui.SaveStudentReports(ctx, sheets, label)
		if saved != "" {
			fmt.Fprintf(out, "Saved %d student reports, one per page: %s\n", len(sheets), saved)
		}
		if e != nil {
			return fail(e, progress)
		}
	case "parent-review":
		sender := workflow.OutlookSender{}
		outcome, e := localui.ReviewWithTest(ctx, filepath.Join(file, "drafts.json"), func() (workflow.ReviewModel, error) {
			return service.BuildParentReview(file)
		}, func(ctx context.Context, skip map[string]bool) (localui.ReviewPostResult, error) {
			status, summary, e := service.SendParentUpdates(ctx, file, skip, localui.ReviewActionFrom(ctx) == "drafts", sender)
			return localui.ReviewPostResult{Status: status, Summary: summary, Path: filepath.Join(file, "outcomes.json")}, e
		}, func(ctx context.Context) (string, error) {
			return service.SendParentTest(file, sender)
		}, progress, root)
		if e != nil {
			return fail(inputError(e), progress)
		}
		fmt.Fprintf(out, "Review finished. Outcome: %s\n", outcome.Status)
		if outcome.PostSummary != "" {
			fmt.Fprintln(out, "Delivery:", outcome.PostSummary)
		}
		encoded, _ := json.Marshal(struct {
			Status string                   `json:"outcome"`
			Path   string                   `json:"result_path,omitempty"`
			Revise []localui.ReviewRevision `json:"revision_notes,omitempty"`
		}{outcome.Status, outcome.Path, outcome.Revise})
		fmt.Fprintf(out, "Review result: %s\n", encoded)
	case "review":
		outcome, e := localui.Review(ctx, file, func() (workflow.ReviewModel, error) {
			if b, _ := os.ReadFile(file); documentKind(b) == "assignment" {
				var doc domain.AssignmentDocument
				if _, e := workspace.ReadDocument(file, &doc); e == nil && doc.Operation == "update" {
					return service.BuildEditReview(ctx, file)
				}
			}
			return workflow.BuildReview(root, file)
		}, func(ctx context.Context, skip map[string]bool) (localui.ReviewPostResult, error) {
			if quizReview {
				report, e := service.PushQuiz(ctx, args[1], file)
				for _, warning := range report.Warnings {
					fmt.Fprintln(progress, "Warning:", warning)
				}
				return localui.ReviewPostResult{Status: report.Status, Path: report.Path, Summary: strings.TrimSpace(fmt.Sprintf("%s: quiz %d, %d of %d items. %s %s", report.Status, report.AssignmentID, report.ItemsAdded, report.ItemsTotal, report.URL, report.Message))}, e
			}
			if b, _ := os.ReadFile(file); documentKind(b) == "assignment" {
				report, e := service.Push(ctx, file)
				return localui.ReviewPostResult{Status: report.Status, Path: report.Path, Summary: strings.TrimSpace(fmt.Sprintf("%s: %s %s", report.Status, report.URL, report.Message))}, e
			}
			service.Skip = skip
			report, e := service.Post(ctx, file)
			failed := 0
			for _, r := range report.Students {
				if r.Status != "completed" && r.Status != "succeeded" && !skip[r.StudentRef] {
					failed++
				}
			}
			summary := fmt.Sprintf("%d approved student(s) did not fully complete; run cse validate/post output or rerun review", failed)
			if failed == 0 {
				summary = "all approved students completed"
			}
			status := "completed"
			if failed > 0 || e != nil {
				status = "partial"
			}
			return localui.ReviewPostResult{Status: status, Summary: summary, Path: report.Path}, e
		}, progress, root)
		if e != nil {
			return fail(inputError(e), progress)
		}
		fmt.Fprintf(out, "Review finished. Outcome: %s\n", outcome.Status)
		if outcome.PostSummary != "" && outcome.Status != "completed" {
			fmt.Fprintln(out, "Post:", outcome.PostSummary)
		}
		fmt.Fprintln(out, "Results:", outcome.Path)
		fmt.Fprintf(out, "Approved (%d): %s\n", len(outcome.Approved), strings.Join(outcome.Approved, ", "))
		for _, r := range outcome.Revise {
			fmt.Fprintf(out, "Revise %s: %s\n", r.Ref, r.Note)
		}
		for _, h := range outcome.Held {
			fmt.Fprintln(out, "Held", h)
		}
		if len(outcome.Revise) > 0 {
			fmt.Fprintln(out, "Revision notes are saved beside the results file; fix those students, then run cse review again.")
		}
		encoded, _ := json.Marshal(outcome)
		fmt.Fprintf(out, "Review result: %s\n", encoded)
	case "post":
		report, e := service.Post(ctx, file)
		partial := false
		fmt.Fprintln(table, "Student ref\tOperation\tStatus\tRubric\tGrade\tComment\tLate\tDetails")
		for _, r := range report.Students {
			comment := r.Outcomes.Comment.Status
			if r.Operation == "revise-feedback" {
				comment = r.Revision.Status
			}
			fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", r.StudentRef, r.Operation, r.Status, r.Outcomes.Rubric.Status, r.Outcomes.Grade.Status, comment, r.Outcomes.Late.Status, compact(r.Message))
			if r.Status != "completed" && r.Status != "succeeded" {
				partial = true
			}
		}
		_ = table.Flush()
		fmt.Fprintln(out, "Results:", report.Path)
		if e != nil {
			return fail(workflowError(e), progress)
		}
		if partial {
			return 3
		}
	}
	return 0
}
func compact(s string) string { return strings.Join(strings.Fields(s), " ") }
func workflowError(e error) error {
	var ce *canvas.Error
	if errors.As(e, &ce) {
		return e
	}
	if strings.Contains(e.Error(), "held:") {
		return &cliError{3, e}
	}
	return inputError(e)
}

func documentKind(b []byte) string { k, _ := workspace.DocumentKind(b); return k }

func optionalBool(value *bool) string {
	if value == nil {
		return "unknown"
	}
	return strconv.FormatBool(*value)
}
func knownText(value string) string {
	if value == "" {
		return "not reported"
	}
	return value
}

func parseCourseUpload(root string, args []string) (string, workflow.CourseUploadOptions, error) {
	var opts workflow.CourseUploadOptions
	if len(args) < 3 {
		return "", opts, fmt.Errorf("upload-file requires COURSE FILE --access course|hidden|locked")
	}
	flags := flag.NewFlagSet("upload-file", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&opts.Access, "access", "", "file access: course, hidden (links work), or locked")
	flags.Int64Var(&opts.FolderID, "folder", 0, "existing course folder ID")
	flags.StringVar(&opts.ReceiptPath, "receipt", "", "local receipt path")
	flags.BoolVar(&opts.NewUpload, "new-upload", false, "explicitly upload new bytes without replacing a live file")
	if e := flags.Parse(args[2:]); e != nil {
		return "", opts, e
	}
	if flags.NArg() != 0 || opts.FolderID < 0 || (opts.Access != "course" && opts.Access != "hidden" && opts.Access != "locked") {
		return "", opts, fmt.Errorf("upload-file requires --access course|hidden|locked and an optional positive --folder ID")
	}
	resolve := func(path string) (string, error) {
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, path)
		}
		return filepath.Abs(path)
	}
	source, e := resolve(args[1])
	if e != nil {
		return "", opts, e
	}
	if opts.ReceiptPath != "" {
		opts.ReceiptPath, e = resolve(opts.ReceiptPath)
		if e != nil {
			return "", opts, e
		}
	}
	return source, opts, nil
}

// statusWriter returns where routine status lines ("courses…", timings) go.
// When stderr is redirected or captured (an agent shell, a pipe), those lines
// are dropped: Windows PowerShell 5.1 turns any captured native stderr into a
// NativeCommandError even for a successful command. Errors still use progress.
func statusWriter(progress io.Writer) io.Writer {
	if f, ok := progress.(*os.File); ok {
		if info, err := f.Stat(); err != nil || info.Mode()&os.ModeCharDevice == 0 {
			return io.Discard
		}
	}
	return progress
}

// splitParentArgs separates course nicknames from student refs: refs are the
// Engine's s-prefixed identifiers; everything before the first one is a course.
func splitParentArgs(args []string) (courses, refs []string) {
	for i, a := range args {
		if strings.HasPrefix(a, "s-") && i > 0 {
			return args[:i], args[i:]
		}
	}
	return args, nil
}
