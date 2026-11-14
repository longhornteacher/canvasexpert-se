package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"canvasexpertse/internal/workflow"
)

func TestSessionReportCollapsesZeroCountsAndExplainsAttention(t *testing.T) {
	zero, two := 0, 2
	var out bytes.Buffer
	printSessionReport(&out, []workflow.PullAssignmentSummary{
		{Outcome: "skipped", SkipReason: "Canvas reports no work needing grading", Course: "CS8", AssignmentID: 1, Title: "Finished", NeedsGradingCount: &zero},
		{Course: "CS8", AssignmentID: 2, Title: "Quiz", AssignmentGroupName: "Daily", AssignmentGroupWeight: json.Number("30"), DueState: "not due yet", NeedsGradingCount: &two, Students: 1, CommentOnlyCompleted: 1},
		{Course: "CS8", AssignmentID: 3, Title: "Closed", ClosedGradingPeriod: true, SkipReason: "closed grading period, not pulled"},
	})
	text := out.String()
	for _, want := range []string{"Daily (30%)", "not due yet", "comment posted, item points not entered: 1", "closed grading period, not pulled", "3 known; 1 completed; 0 no eligible work; 2 skipped; 0 failed; 0 not reached", "submissions were not inspected"} {
		if !strings.Contains(text, want) {
			t.Errorf("report lacks %q: %s", want, text)
		}
	}
	if strings.Contains(text, "Canvas count differs") {
		t.Fatalf("pulled plus comment-posted students add up to Canvas's count: %s", text)
	}
	if strings.Contains(text, "Finished") {
		t.Fatal("zero-count assignment was expanded into a row")
	}
}

func TestSessionReportAccountsForPartialAndKeepsRetryScope(t *testing.T) {
	var out bytes.Buffer
	one := 1
	printSessionReport(&out, []workflow.PullAssignmentSummary{
		{Course: "Synthetic", AssignmentID: 10, Outcome: "complete", CandidatesKnown: true, Candidates: 1, Students: 1, NeedsGradingCount: &one},
		{Course: "Synthetic", AssignmentID: 11, GroupID: 8, Title: "Failed middle", Outcome: "failed", FailurePhase: "submission discovery", FailureReason: "Canvas denied this operation", NeedsGradingCount: &one},
		{Course: "Synthetic", AssignmentID: 12, Outcome: "filtered", CandidatesKnown: true, Inactive: 1, Missing: 1, NeedsGradingCount: &one},
		{Course: "Synthetic", AssignmentID: 13, Outcome: "not-reached", NeedsGradingCount: &one},
	})
	text := out.String()
	for _, want := range []string{"4 known; 1 completed; 1 no eligible work; 0 skipped; 1 failed; 1 not reached", "Failed middle", "submission discovery", "inactive: 1", "missing submission: 1", "not processed", `"assignment":"11"`, `"group_id":8`, `"session":true`} {
		if !strings.Contains(text, want) {
			t.Errorf("partial report lacks %q: %s", want, text)
		}
	}
	if strings.Contains(text, "have nothing to pull") {
		t.Fatalf("partial report claims completion: %s", text)
	}
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, "Failed middle") && !strings.HasPrefix(line, "Retry") && !strings.Contains(line, "unknown") {
			t.Errorf("failed discovery implies zero eligible: %s", line)
		}
	}
}
