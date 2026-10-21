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
	"sort"
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
	arities := map[string][2]int{"courses": {0, 0}, "modules": {1, 1}, "external-resources": {1, 5}, "groups": {1, 6}, "pull-group-set": {2, 2}, "pull-group-memberships": {2, 2}, "push": {1, 1}, "pull-page": {2, 4}, "page-history": {2, 6}, "pull-discussion": {2, 4}, "upload-file": {3, 12}, "pull-item": {2, 2}, "pull-assignment-dates": {2, 2}, "prepare-audience": {2, 3}, "pull-work": {0, 6}, "prepare-late-timing": {1, 1}, "parent-report": {1, 200}, "family-contact-prepare": {1, 1}, "parent-review": {1, 1}, "student-report": {1, 1}, "post": {1, 1}, "review": {1, 2}, "push-quiz": {2, 2}, "api": {1, 1}, "quiz-points": {2, 200}}
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
	if cmd == "parent-review" || cmd == "family-contact-prepare" || cmd == "student-report" {
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
		if cmd == "push" && kind != "assignment" && kind != "assignment-dates" && kind != "page" && kind != "discussion" && kind != "discussion-reply" && kind != "module" && kind != "group-set" && kind != "group-memberships" {
			return fail(inputError(fmt.Errorf("push supports assignment, assignment-dates, page, discussion/reply, module, group-set, or group-memberships documents")), progress)
		}
		if cmd == "post" && kind != "results" && kind != "group-results" {
			return fail(inputError(fmt.Errorf("post requires a results or group-results document")), progress)
		}
	}
	var config domain.CourseConfig
	if _, e := workspace.ReadTeacherDocument(root, "context/courses.md", &config); e != nil {
		return fail(inputError(e), progress)
	}
	if cmd == "prepare-late-timing" {
		prepared, e := (&workflow.Service{Root: root, Config: config}).PrepareLateTiming(args[0])
		if e != nil {
			return fail(inputError(e), progress)
		}
		encoded, e := json.MarshalIndent(prepared, "", "  ")
		if e != nil {
			return fail(e, progress)
		}
		fmt.Fprintln(out, string(encoded))
		return 0
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
	case "prepare-audience":
		if len(args) == 3 && args[2] != "--refresh" {
			return fail(inputError(fmt.Errorf("expected optional --refresh")), progress)
		}
		p, e := documentPath(root, args[1])
		if e != nil {
			return fail(inputError(e), progress)
		}
		state, e := service.PrepareAudience(ctx, args[0], p, len(args) == 3)
		if e != nil {
			return fail(workflowError(e), progress)
		}
		b, _ := json.MarshalIndent(state, "", "  ")
		fmt.Fprintln(out, string(b))
	case "groups":
		setID, memberID, groupID, allGroups, err := parseGroupsArgs(args[1:])
		if err != nil {
			return fail(inputError(err), progress)
		}
		if setID == 0 && memberID == 0 && groupID == 0 && !allGroups {
			sets, e := service.ListGroupSets(ctx, args[0])
			if e != nil {
				return fail(workflowError(e), progress)
			}
			b, _ := json.MarshalIndent(sets, "", "  ")
			fmt.Fprintln(out, string(b))
			return 0
		}
		if memberID > 0 {
			groupID = memberID
		}
		if setID > 0 && memberID == 0 && groupID == 0 {
			roster, e := service.ReadGroupRoster(ctx, args[0], setID)
			if e != nil {
				return fail(workflowError(e), progress)
			}
			b, _ := json.MarshalIndent(roster, "", "  ")
			fmt.Fprintln(out, string(b))
			return 0
		}
		groups, e := service.ListGroups(ctx, args[0], setID, groupID, memberID > 0)
		if e != nil {
			return fail(workflowError(e), progress)
		}
		b, _ := json.MarshalIndent(groups, "", "  ")
		fmt.Fprintln(out, string(b))
		return 0
	case "pull-group-set":
		id, e := strconv.ParseInt(args[1], 10, 64)
		if e != nil || id <= 0 {
			return fail(inputError(fmt.Errorf("group-set ID must be a positive integer")), progress)
		}
		p, e := service.PullGroupSet(ctx, args[0], id)
		if e != nil {
			return fail(workflowError(e), progress)
		}
		fmt.Fprintln(out, p)
	case "pull-group-memberships":
		id, e := strconv.ParseInt(args[1], 10, 64)
		if e != nil || id <= 0 {
			return fail(inputError(fmt.Errorf("group-set ID must be a positive integer")), progress)
		}
		p, e := service.PullGroupMemberships(ctx, args[0], id)
		if e != nil {
			return fail(workflowError(e), progress)
		}
		fmt.Fprintln(out, p)
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
		groupID, err := parseOptionalPageGroup(args[2:])
		if err != nil {
			return fail(inputError(err), progress)
		}
		p, e := service.PullPageScoped(ctx, args[0], args[1], groupID)
		if e != nil {
			return fail(workflowError(e), progress)
		}
		fmt.Fprintln(out, p)
	case "pull-discussion":
		groupID, err := parseOptionalPageGroup(args[2:])
		if err != nil {
			return fail(inputError(err), progress)
		}
		topicID, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil || topicID <= 0 {
			return fail(inputError(fmt.Errorf("topic ID must be a positive integer")), progress)
		}
		p, e := service.PullDiscussion(ctx, args[0], topicID, groupID)
		if e != nil {
			return fail(workflowError(e), progress)
		}
		fmt.Fprintln(out, p)
	case "page-history":
		pageID, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil || pageID <= 0 {
			return fail(inputError(fmt.Errorf("page ID must be a positive integer")), progress)
		}
		groupID, revisionID, err := parsePageHistoryFlags(args[2:])
		if err != nil {
			return fail(inputError(err), progress)
		}
		revisions, e := service.PageHistory(ctx, args[0], pageID, groupID, revisionID)
		if e != nil {
			return fail(workflowError(e), progress)
		}
		b, _ := json.MarshalIndent(revisions, "", "  ")
		fmt.Fprintln(out, string(b))
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
	case "external-resources":
		groupID, resourceType, e := parseExternalResourceFlags(args[1:])
		if e != nil {
			return fail(inputError(e), progress)
		}
		report, e := service.ListExternalResources(ctx, args[0], groupID, resourceType)
		if e != nil {
			return fail(workflowError(e), progress)
		}
		b, _ := json.MarshalIndent(report, "", "  ")
		fmt.Fprintln(out, string(b))
	case "pull-item":
		p, e := service.PullItem(ctx, args[0], args[1])
		if e != nil {
			return fail(e, progress)
		}
		fmt.Fprintln(out, p)
	case "pull-assignment-dates":
		id, parseErr := strconv.ParseInt(args[1], 10, 64)
		if parseErr != nil || id <= 0 {
			return fail(inputError(fmt.Errorf("assignment ID must be positive")), progress)
		}
		report, e := service.PullAssignmentDates(ctx, args[0], id)
		fmt.Fprintf(out, "%s: %d override rows, %s\n", report.Status, len(report.Rows), report.Path)
		if report.Message != "" {
			fmt.Fprintln(out, report.Message)
		}
		if e != nil {
			return fail(workflowError(e), progress)
		}
		if report.Status != "completed" {
			return 3
		}
	case "push":
		b, _ := os.ReadFile(file)
		if kind := documentKind(b); kind == "assignment-dates" {
			report, e := service.PushAssignmentDates(ctx, file)
			fmt.Fprintf(out, "%s: %d override rows, %s\n", report.Status, len(report.Rows), report.Path)
			if report.Message != "" {
				fmt.Fprintln(out, report.Message)
			}
			for _, row := range report.Rows {
				fmt.Fprintf(out, "%s: %s override %d %s\n", row.Key, row.Outcome.Status, row.OverrideID, row.Outcome.Message)
			}
			if e != nil {
				return fail(workflowError(e), progress)
			}
			if report.Status != "completed" {
				return 3
			}
			return 0
		} else if kind == "group-set" {
			report, e := service.PushGroupSet(ctx, file)
			fmt.Fprintf(out, "%s: group set %d, %s\n", report.Status, report.GroupCategoryID, report.Path)
			for _, g := range report.Groups {
				fmt.Fprintf(out, "%s: %s group %d %s\n", g.Key, g.Outcome.Status, g.GroupID, g.URL)
			}
			if e != nil {
				return fail(workflowError(e), progress)
			}
			if report.Status != "completed" {
				return 3
			}
			return 0
		} else if kind := documentKind(b); kind == "group-memberships" {
			report, e := service.PushGroupMemberships(ctx, file)
			fmt.Fprintf(out, "%s: %s\n", report.Status, report.Message)
			for _, outcome := range report.Outcomes {
				fmt.Fprintf(out, "%s: %s %s\n", outcome.StudentRef, outcome.Outcome.Status, outcome.Outcome.Message)
			}
			if report.Assigned != nil {
				fmt.Fprintf(out, "assigned student refs: %s\n", strings.Join(report.Assigned, ", "))
			}
			if e != nil {
				return fail(workflowError(e), progress)
			}
			if report.Status != "completed" {
				return 3
			}
			return 0
		} else if kind == "page" {
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
		} else if kind == "discussion" {
			report, e := service.PushDiscussion(ctx, file)
			fmt.Fprintf(out, "%s: discussion %d %s\n", report.Status, report.TopicID, report.Path)
			if report.URL != "" {
				fmt.Fprintln(out, report.URL)
			}
			if report.Message != "" {
				fmt.Fprintln(out, report.Message)
			}
			if e != nil {
				return fail(workflowError(e), progress)
			}
			if report.Status != "completed" {
				return 3
			}
			return 0
		} else if kind == "discussion-reply" {
			report, e := service.PushDiscussionReply(ctx, file)
			fmt.Fprintf(out, "%s: teacher reply on discussion %d %s\n", report.Status, report.TopicID, report.Path)
			if report.Message != "" {
				fmt.Fprintln(out, report.Message)
			}
			if e != nil {
				return fail(workflowError(e), progress)
			}
			if report.Status != "completed" {
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
		var groupID int64
		groupScoped, session := false, false
		positionals := []string{}
		for i := 0; i < len(args); i++ {
			switch args[i] {
			case "--session":
				if session {
					return fail(fmt.Errorf("--session may be supplied once"), progress)
				}
				session = true
			case "--group":
				if groupScoped || i+1 >= len(args) {
					return fail(fmt.Errorf("--group requires one positive ID"), progress)
				}
				i++
				parsed, parseErr := strconv.ParseInt(args[i], 10, 64)
				if parseErr != nil || parsed <= 0 {
					return fail(fmt.Errorf("group ID must be a positive integer"), progress)
				}
				groupID, groupScoped = parsed, true
			default:
				if strings.HasPrefix(args[i], "--") {
					return fail(fmt.Errorf("unknown pull-work option %q", args[i]), progress)
				}
				positionals = append(positionals, args[i])
			}
		}
		if len(positionals) > 2 {
			return fail(fmt.Errorf("usage: pull-work [COURSE [ASSIGNMENT]] [--group ID] [--session]"), progress)
		}
		if len(positionals) > 0 {
			nickname = positionals[0]
		}
		if len(positionals) > 1 {
			query = positionals[1]
		}
		if groupScoped && (nickname == "" || query == "") {
			return fail(fmt.Errorf("--group requires explicit course and assignment"), progress)
		}
		var report workflow.PullReport
		var e error
		options := workflow.PullWorkOptions{Session: session}
		if groupScoped {
			report, e = service.PullWorkGroupWithOptions(ctx, nickname, query, groupID, options)
		} else {
			report, e = service.PullWorkWithOptions(ctx, nickname, query, options)
		}
		if session {
			printSessionReport(out, report.Assignments)
		} else {
			if len(report.Assignments) > 0 {
				fmt.Fprintln(table, "Course\tAssignment\tTitle\tPulled\tLate\tMissing\tExcused\tInactive\tUnsupported\tHandled\tUnreadable attachments\tDetails")
				for _, summary := range report.Assignments {
					fmt.Fprintf(table, "%s\t%d\t%s\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%s\n", compact(summary.Course), summary.AssignmentID, compact(summary.Title), summary.Students, summary.Late, summary.Missing, summary.Excused, summary.Inactive, summary.Unsupported, summary.Handled, summary.UnreadableAttachments, compact(summary.SkipReason))
				}
				_ = table.Flush()
			}
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
	case "family-contact-prepare":
		audience, e := service.PrepareFamilyContact(ctx, file)
		if e != nil {
			return fail(inputError(e), progress)
		}
		encoded, e := json.MarshalIndent(audience, "", "  ")
		if e != nil {
			return fail(e, progress)
		}
		fmt.Fprintln(out, string(encoded))
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
		outcome, e := localui.ReviewWithTest(ctx, workflow.ParentReviewSource(file), func() (workflow.ReviewModel, error) {
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
			if fileKind(file) == "assignment-dates" {
				report, e := service.PushAssignmentDates(ctx, file)
				return localui.ReviewPostResult{Status: report.Status, Path: report.Path, Summary: report.Message}, e
			}
			if fileKind(file) == "page" {
				report, e := service.PushPage(ctx, file)
				return localui.ReviewPostResult{Status: report.Status, Path: report.Path, Summary: strings.TrimSpace(fmt.Sprintf("%s: Page %d %s %s", report.Status, report.PageID, report.URL, report.Message))}, e
			}
			if b, _ := os.ReadFile(file); documentKind(b) == "discussion" {
				report, e := service.PushDiscussion(ctx, file)
				return localui.ReviewPostResult{Status: report.Status, Path: report.Path, Summary: strings.TrimSpace(fmt.Sprintf("%s: discussion %d %s", report.Status, report.TopicID, report.Message))}, e
			}
			if fileKind(file) == "group-results" {
				service.Skip = skip
				report, e := service.PostGroupResults(ctx, file)
				failed := 0
				for _, r := range report.Groups {
					if !r.SharedDone && !skip[workflow.GroupCardRef(r.GroupID)] {
						failed++
					}
					for _, st := range r.Students {
						if st.Status != "completed" && !skip[workflow.GroupStudentRef(r.GroupID, st.StudentRef)] {
							failed++
						}
					}
					for ref, outcome := range r.Private {
						if outcome.Status != "succeeded" && outcome.Status != "preserved" && outcome.Status != "not-requested" && !skip[workflow.GroupPrivateRef(r.GroupID, ref)] {
							failed++
						}
					}
				}
				summary := fmt.Sprintf("%d approved group action(s) did not fully complete; run cse post output or rerun review", failed)
				if failed == 0 {
					summary = "all approved group actions completed"
				}
				status := "completed"
				if failed > 0 || e != nil {
					status = "partial"
				}
				return localui.ReviewPostResult{Status: status, Summary: summary, Path: report.Path}, e
			}
			if fileKind(file) != "results" {
				return localui.ReviewPostResult{}, fmt.Errorf("review cannot post a %q document", fileKind(file))
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
		if fileKind(file) == "group-results" {
			report, e := service.PostGroupResults(ctx, file)
			partial := false
			fmt.Fprintln(table, "Group\tOperation\tStatus\tRubric\tGrade\tShared comment\tLate\tPrivate comments\tDetails")
			for _, r := range report.Groups {
				private := []string{}
				for ref, o := range r.Private {
					private = append(private, ref+"="+o.Status)
				}
				sort.Strings(private)
				fmt.Fprintf(table, "%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", r.GroupID, r.Operation, r.Status, r.Outcomes.Rubric.Status, r.Outcomes.Grade.Status, r.Outcomes.Comment.Status, r.Outcomes.Late.Status, strings.Join(private, " "), compact(r.Message))
				for _, st := range r.Students {
					fmt.Fprintf(table, "%d\t%s\t%s\t%s\t%s\t%s\t%s\t\t%s\n", r.GroupID, st.StudentRef, st.Status, st.Outcomes.Rubric.Status, st.Outcomes.Grade.Status, st.Outcomes.Comment.Status, st.Outcomes.Late.Status, compact(st.Message))
				}
				if r.Status != "completed" {
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
			return 0
		}
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

func parseGroupsArgs(args []string) (int64, int64, int64, bool, error) {
	var setID, memberID int64
	var groupID int64
	var allGroups bool
	for i := 0; i < len(args); i++ {
		if args[i] == "--groups" {
			allGroups = true
			continue
		}
		if i+1 >= len(args) {
			return 0, 0, 0, false, fmt.Errorf("%s requires an ID", args[i])
		}
		id, e := strconv.ParseInt(args[i+1], 10, 64)
		if e != nil || id <= 0 {
			return 0, 0, 0, false, fmt.Errorf("%s requires a positive integer ID", args[i])
		}
		switch args[i] {
		case "--set":
			if setID != 0 {
				return 0, 0, 0, false, fmt.Errorf("--set may be supplied once")
			}
			setID = id
		case "--members":
			if memberID != 0 {
				return 0, 0, 0, false, fmt.Errorf("--members may be supplied once")
			}
			memberID = id
		case "--group":
			groupID = id
		default:
			return 0, 0, 0, false, fmt.Errorf("unknown groups option %q", args[i])
		}
		i++
	}
	return setID, memberID, groupID, allGroups, nil
}

func parseOptionalPageGroup(args []string) (int64, error) {
	if len(args) == 0 {
		return 0, nil
	}
	if len(args) != 2 || args[0] != "--group" {
		return 0, fmt.Errorf("use --group ID")
	}
	id, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("--group requires a positive integer ID")
	}
	return id, nil
}

func parseExternalResourceFlags(args []string) (int64, string, error) {
	groupID, resourceType := int64(0), "all"
	typeSet := false
	for i := 0; i < len(args); i += 2 {
		if i+1 >= len(args) {
			return 0, "", fmt.Errorf("%s requires a value", args[i])
		}
		switch args[i] {
		case "--group":
			id, err := strconv.ParseInt(args[i+1], 10, 64)
			if err != nil || id <= 0 || groupID != 0 {
				return 0, "", fmt.Errorf("--group requires one positive integer ID")
			}
			groupID = id
		case "--type":
			if typeSet {
				return 0, "", fmt.Errorf("--type may be supplied once")
			}
			typeSet = true
			resourceType = args[i+1]
		default:
			return 0, "", fmt.Errorf("unknown external-resources option %q", args[i])
		}
	}
	if resourceType != "all" && resourceType != "collaborations" && resourceType != "conferences" {
		return 0, "", fmt.Errorf("--type must be collaborations, conferences, or all")
	}
	return groupID, resourceType, nil
}

func parsePageHistoryFlags(args []string) (groupID, revisionID int64, err error) {
	for i := 0; i < len(args); i += 2 {
		if i+1 >= len(args) {
			return 0, 0, fmt.Errorf("%s requires a positive integer ID", args[i])
		}
		id, e := strconv.ParseInt(args[i+1], 10, 64)
		if e != nil || id <= 0 {
			return 0, 0, fmt.Errorf("%s requires a positive integer ID", args[i])
		}
		switch args[i] {
		case "--group":
			if groupID != 0 {
				return 0, 0, fmt.Errorf("--group may be supplied once")
			}
			groupID = id
		case "--revision":
			if revisionID != 0 {
				return 0, 0, fmt.Errorf("--revision may be supplied once")
			}
			revisionID = id
		default:
			return 0, 0, fmt.Errorf("unknown page-history option %q", args[i])
		}
	}
	return groupID, revisionID, nil
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

func fileKind(path string) string { b, _ := os.ReadFile(path); return documentKind(b) }
