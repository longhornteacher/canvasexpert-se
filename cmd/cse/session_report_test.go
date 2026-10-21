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
		{Course: "CS8", AssignmentID: 1, Title: "Finished", NeedsGradingCount: &zero},
		{Course: "CS8", AssignmentID: 2, Title: "Quiz", AssignmentGroupName: "Daily", AssignmentGroupWeight: json.Number("30"), DueState: "not due yet", NeedsGradingCount: &two, Students: 1, CommentOnlyCompleted: 1},
		{Course: "CS8", AssignmentID: 3, Title: "Closed", ClosedGradingPeriod: true, SkipReason: "closed grading period, not pulled"},
	})
	text := out.String()
	for _, want := range []string{"Daily (30%)", "not due yet", "comment posted, item points not entered: 1", "closed grading period, not pulled", "2 assignments need attention; 1 other assignments"} {
		if !strings.Contains(text, want) {
			t.Errorf("report lacks %q: %s", want, text)
		}
	}
	if strings.Contains(text, "Finished") {
		t.Fatal("zero-count assignment was expanded into a row")
	}
}
