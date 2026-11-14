package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"canvasexpertse/internal/workflow"
)

func printSessionReport(out io.Writer, assignments []workflow.PullAssignmentSummary) {
	table := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	shown, collapsed := 0, 0
	counts := map[string]int{}
	for _, a := range assignments {
		outcome := a.Outcome
		if outcome == "" {
			outcome = "complete"
			if a.Students == 0 {
				outcome = "filtered"
			}
			if a.SkipReason != "" {
				outcome = "skipped"
			}
		}
		counts[outcome]++
		if outcome == "skipped" && a.SkipReason == "Canvas reports no work needing grading" {
			collapsed++
			continue
		}
		if shown == 0 {
			fmt.Fprintln(table, "Course\tAssignment\tTitle\tGroup (weight)\tDue state\tCanvas needs grading\tEligible\tPulled\tTiming candidates\tOutcome\tDetails")
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
		eligible := "unknown"
		if a.CandidatesKnown {
			eligible = fmt.Sprint(a.Candidates)
		}
		var details []string
		if outcome == "failed" {
			details = append(details, a.FailurePhase+": "+a.FailureReason)
		}
		if outcome == "not-reached" {
			details = append(details, "not processed; no claim about eligible work")
		}
		if outcome == "filtered" {
			details = append(details, "no eligible work after filtering")
		}
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
			{"missing submission", a.Missing}, {"files unavailable (not downloaded)", a.UnavailableAttachments},
			{"report attempt not matched", a.UnmatchedAttempts},
		} {
			if reason.count > 0 {
				details = append(details, fmt.Sprintf("%s: %d", reason.label, reason.count))
			}
		}
		if a.NeedsGradingCount != nil && *a.NeedsGradingCount != a.Students+a.CommentOnlyCompleted {
			details = append(details, "Canvas count differs from pulled count")
		}
		fmt.Fprintf(table, "%s\t%d\t%s\t%s\t%s\t%s\t%s\t%d\t%d\t%s\t%s\n", compact(a.Course), a.AssignmentID, compact(a.Title), compact(group), compact(a.DueState), count, eligible, a.Students, a.TimingCandidates, outcome, compact(strings.Join(details, "; ")))
	}
	_ = table.Flush()
	fmt.Fprintf(out, "Assignments: %d known; %d completed; %d no eligible work; %d skipped; %d failed; %d not reached.\n", len(assignments), counts["complete"], counts["filtered"], counts["skipped"], counts["failed"], counts["not-reached"])
	if collapsed > 0 {
		fmt.Fprintf(out, "Canvas reports no work needing grading for %d skipped assignment(s); submissions were not inspected.\n", collapsed)
	}
	printPullRetries(out, assignments, true)
}

func printPullRetries(out io.Writer, assignments []workflow.PullAssignmentSummary, session bool) {
	for _, a := range assignments {
		if a.Outcome != "failed" && a.Outcome != "not-reached" {
			continue
		}
		args := map[string]any{"course": a.Course, "assignment": fmt.Sprint(a.AssignmentID)}
		if a.GroupID > 0 {
			args["group_id"] = a.GroupID
		}
		if session {
			args["session"] = true
		}
		b, _ := json.Marshal(args)
		fmt.Fprintf(out, "Retry %s / %d (%s): pull_work %s\n", compact(a.Course), a.AssignmentID, compact(a.Title), b)
	}
}
