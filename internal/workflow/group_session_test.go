package workflow

import (
	"context"
	"testing"

	"canvasexpertse/internal/domain"
)

func TestGroupSessionDoesNotRepullCompletedIndividualGradesForTiming(t *testing.T) {
	env := newGroupEnv(t, false, true)
	for i := range env.doc.Groups[0].Students {
		row := &env.doc.Groups[0].Students[i]
		row.Grade.Outcomes = domain.ActionOutcomes{
			Grade:   domain.ActionOutcome{Status: "succeeded"},
			Rubric:  domain.ActionOutcome{Status: "succeeded"},
			Comment: domain.ActionOutcome{Status: "succeeded"},
			Late:    domain.ActionOutcome{Status: "held"},
		}
	}
	env.save(t)
	for id := int64(10); id <= 12; id++ {
		env.timing[id] = map[string]any{"late": true, "seconds_late": 3600, "late_policy_status": "late"}
	}
	report, err := env.s.PullWorkGroupWithOptions(context.Background(), "TEST", "10", 60, PullWorkOptions{Session: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Packets) != 0 || report.Students != 0 || report.Assignments[0].Handled != 3 {
		t.Fatalf("completed group was re-pulled for timing: %+v", report)
	}
	// A new member attempt still needs assessment and retains its audience.
	env.attempt[10] = 2
	report, err = env.s.PullWorkGroupWithOptions(context.Background(), "TEST", "10", 60, PullWorkOptions{Session: true})
	if err != nil || len(report.Packets) != 1 || report.Students != 1 || report.Assignments[0].Handled != 2 || report.Assignments[0].TimingCandidates != 1 {
		t.Fatalf("new group attempt not discovered: report=%+v err=%v", report, err)
	}
}
