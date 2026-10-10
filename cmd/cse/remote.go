package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"canvasexpertse/internal/canvas"
	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/platform"
	"canvasexpertse/internal/workflow"
	"canvasexpertse/internal/workspace"
)

func runRemote(ctx context.Context, root, cmd string, args []string, out, progress io.Writer) int {
	arities := map[string][2]int{"courses": {0, 0}, "modules": {1, 1}, "push": {1, 1}, "pull-item": {2, 2}, "pull-work": {0, 2}, "post": {1, 1}, "push-quiz": {2, 2}}
	n := arities[cmd]
	if len(args) < n[0] || len(args) > n[1] {
		return fail(inputError(fmt.Errorf("wrong arguments for %s; run cse help", cmd)), progress)
	}
	var file string
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
	if cmd == "push" || cmd == "post" {
		p, e := documentPath(root, args[0])
		if e != nil {
			return fail(inputError(e), progress)
		}
		file = p
		if e = validateDocument(file, root); e != nil {
			return fail(inputError(e), progress)
		}
	}
	var config domain.CourseConfig
	if _, e := workspace.ReadDocument(filepath.Join(root, "context", "courses.md"), &config); e != nil {
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
		return fail(inputError(fmt.Errorf("context/courses.md instance differs from this machine's saved Canvas origin; update the configuration")), progress)
	}
	service := &workflow.Service{Root: root, Client: c, Config: config}
	start := time.Now()
	defer func() {
		fmt.Fprintf(progress, "%s: %s, %d HTTP requests\n", cmd, time.Since(start).Round(time.Millisecond), c.Requests())
	}()
	fmt.Fprintf(progress, "%s…\n", cmd)
	table := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	switch cmd {
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
		if report.Engine == "classic" && report.QuizID > 0 {
			fmt.Fprintf(out, "%s: Classic Quiz %d (assignment %d), %d of %d questions, %v points, published: %t\n%s\n", report.Status, report.QuizID, report.AssignmentID, report.ItemsAdded, report.ItemsTotal, report.Points, report.Published, report.URL)
		} else if report.AssignmentID > 0 {
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
	case "post":
		report, e := service.Post(ctx, file)
		partial := false
		if report.Classic {
			fmt.Fprintln(table, "Student ref\tAttempt\tQuestion\tScore/max\tStatus\tDetails")
			for _, r := range report.QuizStudents {
				for _, q := range r.Questions {
					score := "held"
					if q.Score != nil {
						score = q.Score.String() + "/" + q.PointsPossible.String()
					}
					fmt.Fprintf(table, "%s\t%d\t%d\t%s\t%s\t%s\n", r.StudentRef, r.Attempt.Attempt, q.QuestionID, score, q.Outcome.Status, compact(q.Outcome.Message))
				}
				if r.Late.Status != "not-requested" || r.CanvasState != "" {
					observation := ""
					if r.CanvasScore != nil {
						observation += " observed Canvas score: " + r.CanvasScore.String()
					}
					if r.KeptScore != nil {
						observation += " observed kept_score: " + r.KeptScore.String()
					}
					if r.CanvasState != "" {
						observation += " Canvas state: " + r.CanvasState
					}
					if r.Status == "completed" && r.CanvasState == "pending_review" {
						observation += " (requested scope complete; other writing remains pending review)"
					}
					fmt.Fprintf(table, "%s\t%d\tTiming/Canvas observations\t\t%s\t%s%s\n", r.StudentRef, r.Attempt.Attempt, r.Late.Status, compact(r.Late.Message), observation)
				}
				if r.Status != "completed" {
					partial = true
				}
			}
		} else {
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
