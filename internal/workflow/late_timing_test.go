package workflow

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

func timingCalendar() SchoolCalendar {
	return SchoolCalendar{Schema: 1, Timezone: "America/Chicago", StartDate: "2026-08-19", EndDate: "2027-05-27", NonSchoolDates: []string{"2026-10-09", "2026-10-12"}, NonSchoolRanges: []SchoolDateRange{{StartDate: "2026-11-23", EndDate: "2026-11-27"}}}
}
func stamp(t *testing.T, v string) time.Time {
	t.Helper()
	got, e := time.Parse(time.RFC3339, v)
	if e != nil {
		t.Fatal(e)
	}
	return got
}
func TestSchoolSecondsSkipsWholeNonSchoolDates(t *testing.T) {
	for _, test := range []struct {
		name, due, submitted string
		want                 int64
	}{
		{"weekend-holidays", "2026-10-08T18:00:00-05:00", "2026-10-13T09:00:00-05:00", 15 * 3600},
		{"weekend", "2026-10-16T18:00:00-05:00", "2026-10-19T09:00:00-05:00", 15 * 3600},
		{"school-night", "2026-10-14T18:00:00-05:00", "2026-10-15T09:00:00-05:00", 15 * 3600},
		{"break-range", "2026-11-20T18:00:00-06:00", "2026-11-30T09:00:00-06:00", 15 * 3600},
		{"early-release-full-date", "2026-12-18T12:00:00-06:00", "2026-12-18T18:00:00-06:00", 6 * 3600},
		{"on-time", "2026-10-14T18:00:00-05:00", "2026-10-14T16:00:00-05:00", 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, e := SchoolSeconds(timingCalendar(), stamp(t, test.due), stamp(t, test.submitted))
			if e != nil || got != test.want {
				t.Fatalf("seconds=%d error=%v want=%d", got, e, test.want)
			}
		})
	}
	if _, e := SchoolSeconds(timingCalendar(), stamp(t, "2027-06-01T12:00:00Z"), stamp(t, "2027-06-02T12:00:00Z")); e == nil {
		t.Fatal("uncovered dates were guessed")
	}
}

func TestLateTimingCarriesFirstGenuineSubmissionWithoutPointMath(t *testing.T) {
	for _, scenario := range []string{"late-prior", "on-time-prior", "late-current"} {
		onTime := scenario == "on-time-prior"
		t.Run(scenario, func(t *testing.T) {
			due := "2026-10-08T18:00:00-05:00"
			first := "2026-10-13T09:00:00-05:00"
			if onTime {
				first = "2026-10-08T17:00:00-05:00"
			}
			current := "2026-10-15T12:00:00-05:00"
			live := fixtureSubmission(5)
			live.Attempt = 3
			live.SubmittedAt = &current
			genuine := live
			genuine.Attempt = 2
			genuine.SubmittedAt = &first
			genuine.CachedDueDate = &due
			live.History = []Submission{genuine}
			if scenario == "late-current" {
				live.Attempt = 2
				live.SubmittedAt = &first
				live.CachedDueDate = &due
				live.History = nil
			}
			raw := json.Number("15")
			live.EnteredScore, live.Score = &raw, &raw
			late := "late"
			live.Late = true
			live.LatePolicyStatus = &late
			live.SecondsLate = 999999
			writes := 0
			s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "PUT" {
					writes++
					var req struct {
						Submission map[string]json.RawMessage `json:"submission"`
					}
					if e := json.NewDecoder(r.Body).Decode(&req); e != nil {
						t.Error(e)
					}
					for key := range req.Submission {
						if key != "late_policy_status" && key != "seconds_late_override" {
							t.Error("timing changed unrelated field", key)
						}
					}
					var status string
					_ = json.Unmarshal(req.Submission["late_policy_status"], &status)
					live.LatePolicyStatus = &status
					if onTime {
						if status != "none" || req.Submission["seconds_late_override"] != nil {
							t.Error("on-time must clear lateness without overriding raw seconds")
						}
						live.Late = false
					} else {
						var seconds int64
						_ = json.Unmarshal(req.Submission["seconds_late_override"], &seconds)
						if status != "late" || seconds != 15*3600 {
							t.Error("incorrect first-genuine lateness", seconds, status)
						}
						live.SecondsLate = seconds
					}
				}
				_ = json.NewEncoder(w).Encode(live)
			})
			if e := workspace.WriteDocument(filepath.Join(s.Root, "settings", "school-calendar.md"), timingCalendar(), "calendar"); e != nil {
				t.Fatal(e)
			}
			ref := attemptRef(genuine)
			g := &domain.GradeOperation{Attempt: attemptRef(live), LateTiming: &domain.LateForgiveness{FirstAttempt: ref, EffectiveDueAt: due, GenuineReason: "Attempt 1 was a placeholder; attempt 2 contains actual work."}}
			work := domain.StudentWorkDocument{Current: domain.AttemptWork{Ref: attemptRef(live), EffectiveDueAt: due}, Prior: []domain.AttemptWork{{Ref: ref, EffectiveDueAt: due}}}
			doc := domain.ResultsDocument{Assignment: domain.AssignmentSnapshot{CourseID: 1, AssignmentID: 10}}
			if e := s.postLate(context.Background(), doc, 5, work, g, func() error { return nil }); e != nil {
				t.Fatal(e)
			}
			if writes != 1 || g.Outcomes.Late.Status != "succeeded" || live.EnteredScore.String() != "15" {
				t.Fatalf("bad timing outcome %+v writes=%d", g.Outcomes.Late, writes)
			}
			g.Outcomes.Late.Status = "uncertain"
			if e := s.postLate(context.Background(), doc, 5, work, g, func() error { return nil }); e != nil {
				t.Fatal(e)
			}
			if writes != 1 || g.Outcomes.Late.Status != "preserved" {
				t.Fatal("uncertain timing was not recovered without replay")
			}
			// A teacher changing timing is not proof that the uncertain write failed.
			live.SecondsLate = 42
			if onTime {
				live.Late = true
			}
			g.Outcomes.Late.Status = "uncertain"
			if e := s.postLate(context.Background(), doc, 5, work, g, func() error { return nil }); e != nil {
				t.Fatal(e)
			}
			if writes != 1 || g.Outcomes.Late.Status != "uncertain" {
				t.Fatal("ambiguous timing blindly replayed")
			}
			newDue := "2026-10-07T18:00:00-05:00"
			if scenario == "late-current" {
				live.CachedDueDate = &newDue
			} else {
				live.History[0].CachedDueDate = &newDue
			}
			g.Outcomes.Late = domain.ActionOutcome{}
			if e := s.postLate(context.Background(), doc, 5, work, g, func() error { return nil }); e != nil {
				t.Fatal(e)
			}
			if writes != 1 || g.Outcomes.Late.Status != "held" || !strings.Contains(g.Outcomes.Late.Message, "due date") {
				t.Fatal("changed applicable due date was ignored")
			}
		})
	}
}
