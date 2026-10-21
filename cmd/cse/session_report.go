package main

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"canvasexpertse/internal/workflow"
)

func printSessionReport(out io.Writer, assignments []workflow.PullAssignmentSummary) {
	table := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	shown, skipped := 0, 0
	for _, a := range assignments {
		if a.Students == 0 && a.CommentOnlyCompleted == 0 && !a.ClosedGradingPeriod && a.SkipReason == "" && !a.NeedsGradingCountUnknown && (a.NeedsGradingCount == nil || *a.NeedsGradingCount == 0) {
			skipped++
			continue
		}
		if shown == 0 {
			fmt.Fprintln(table, "Course\tAssignment\tTitle\tGroup (weight)\tDue state\tCanvas needs grading\tPulled\tTiming candidates\tDetails")
		}
		shown++
		group := a.AssignmentGroupName
		if group == "" {
			group = "unknown"
		}
		if a.AssignmentGroupWeight != "" {
			group += " (" + a.AssignmentGroupWeight.String() + "%)"
		}
		count := "unknown"
		if a.NeedsGradingCount != nil {
			count = fmt.Sprint(*a.NeedsGradingCount)
		}
		var details []string
		if a.SkipReason != "" {
			details = append(details, a.SkipReason)
		}
		if a.NeedsGradingCountUnknown {
			details = append(details, "Canvas count unavailable; submission discovery used")
		}
		for _, reason := range []struct {
			label string
			count int
		}{
			{"inactive", a.Inactive}, {"not visible", a.NotVisible}, {"excused", a.Excused},
			{"unsupported", a.Unsupported}, {"undated", a.Undated}, {"effective due date is unavailable", a.DueUnavailable},
			{"already scored", a.Handled}, {"comment posted, item points not entered", a.CommentOnlyCompleted},
			{"missing submission", a.Missing}, {"unreadable attachments", a.UnreadableAttachments},
		} {
			if reason.count > 0 {
				details = append(details, fmt.Sprintf("%s: %d", reason.label, reason.count))
			}
		}
		if a.NeedsGradingCount != nil && *a.NeedsGradingCount != a.Students+a.CommentOnlyCompleted {
			details = append(details, "Canvas count differs from pulled count")
		}
		fmt.Fprintf(table, "%s\t%d\t%s\t%s\t%s\t%s\t%d\t%d\t%s\n", compact(a.Course), a.AssignmentID, compact(a.Title), compact(group), compact(a.DueState), count, a.Students, a.TimingCandidates, compact(strings.Join(details, "; ")))
	}
	_ = table.Flush()
	fmt.Fprintf(out, "%d assignments need attention; %d other assignments have nothing to pull.\n", shown, skipped)
}
