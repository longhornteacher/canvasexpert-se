package workflow

import (
	"strings"
	"testing"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

func allowanceService(t *testing.T) *Service {
	t.Helper()
	s := &Service{Root: t.TempDir()}
	calendar := workspace.SchoolCalendar{Schema: 1, Timezone: "America/Chicago", StartDate: "2026-08-01", EndDate: "2027-05-31", NonSchoolDates: []string{"2026-10-19"}}
	if err := workspace.WriteTeacherDocument(s.Root, "context/school-calendar.md", calendar, "Synthetic calendar with Monday off."); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestAllowanceSchoolDaysMatchesTheHandSuppliedDeadline(t *testing.T) {
	s := allowanceService(t)
	first := domain.AttemptRef{Attempt: 1, SubmittedAt: "2026-10-20T16:30:00-05:00"}
	observed := &domain.ObservedTiming{StatusKnown: true, LateKnown: true, SecondsLateKnown: true}
	// Canvas still has the base date (Thu); the documented rule adds one school day (Fri).
	canvasDue := "2026-10-15T16:00:00-05:00"
	byHand := LateTimingStudentDecision{FirstGenuine: first, AccommodatedDeadline: "2026-10-16T16:00:00-05:00", AccommodationBasis: "Synthetic standing allowance."}
	byEngine := LateTimingStudentDecision{FirstGenuine: first, AllowanceSchoolDays: 1, AccommodationBasis: "Synthetic standing allowance."}
	want, reason := s.calculateTiming(byHand, canvasDue, observed, []domain.AttemptWork{{Ref: first}}, first)
	if reason != "" {
		t.Fatal(reason)
	}
	got, reason := s.calculateTiming(byEngine, canvasDue, observed, []domain.AttemptWork{{Ref: first}}, first)
	if reason != "" || got == nil {
		t.Fatalf("allowance: %q", reason)
	}
	if got.SecondsLate != want.SecondsLate || got.Status != want.Status || got.DueAt != canvasDue || got.SchoolTime.Deadline != want.SchoolTime.Deadline {
		t.Fatalf("engine-counted allowance differs from the hand-supplied deadline:\n got %+v\nwant %+v", got.SchoolTime, want.SchoolTime)
	}
	if got.SchoolTime.AllowanceSchoolDays != 1 || got.SchoolTime.AllowanceBase == "" {
		t.Fatalf("provenance should show the base and N: %+v", got.SchoolTime)
	}
}

func TestAllowanceDecisionValidation(t *testing.T) {
	first := domain.AttemptRef{Attempt: 1, SubmittedAt: "2026-10-20T16:30:00-05:00"}
	base := LateTimingStudentDecision{StudentRef: "stu_a", FirstGenuine: first}
	for name, tc := range map[string]struct {
		mutate func(*LateTimingStudentDecision)
		want   string
	}{
		"both fields": {func(d *LateTimingStudentDecision) {
			d.AllowanceSchoolDays, d.AccommodatedDeadline, d.AccommodationBasis = 1, "2026-10-16T16:00:00-05:00", "x"
		}, "not both"},
		"missing basis":     {func(d *LateTimingStudentDecision) { d.AllowanceSchoolDays = 1 }, "accommodation_basis is required"},
		"base without days": {func(d *LateTimingStudentDecision) { d.AllowanceBase = "2026-10-16T16:00:00-05:00" }, "requires allowance_school_days"},
		"negative days":     {func(d *LateTimingStudentDecision) { d.AllowanceSchoolDays = -1 }, "positive"},
	} {
		d := base
		tc.mutate(&d)
		if err := validateLateTimingDecision(d); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: %v", name, err)
		}
	}
	ok := base
	ok.AllowanceSchoolDays, ok.AccommodationBasis = 1, "Synthetic standing allowance."
	if err := validateLateTimingDecision(ok); err != nil {
		t.Fatal(err)
	}
}

func TestSharedGroupWithDifferingAllowancesHolds(t *testing.T) {
	s := allowanceService(t)
	group := domain.GroupWorkDocument{GroupID: 44, CourseID: 2, AssignmentID: 10, RosterComplete: true, PropagationAudienceComplete: true, SharedAssessmentStatus: "ready", Assignment: domain.AssignmentSnapshot{GradeGroupStudentsIndividually: boolPtr(false)}, PropagationAudience: []string{"stu_a", "stu_b"}}
	observed := &domain.ObservedTiming{StatusKnown: true, LateKnown: true, Late: true, SecondsLateKnown: true}
	for _, ref := range group.PropagationAudience {
		group.Members = append(group.Members, domain.GroupWorkMember{StudentRef: ref, Attempt: 1, SubmittedAt: "2026-09-14T14:00:00Z", EffectiveDueAt: "2026-09-10T15:00:00Z", ObservedTiming: observed})
	}
	choice := func(ref string, days int) LateTimingStudentDecision {
		return LateTimingStudentDecision{StudentRef: ref, FirstGenuine: domain.AttemptRef{Attempt: 1, SubmittedAt: "2026-09-14T14:00:00Z"}, AllowanceSchoolDays: days, AccommodationBasis: "Synthetic allowance."}
	}
	same := s.prepareGroupTiming(group, LateTimingGroupDecision{GroupID: 44, Members: []LateTimingStudentDecision{choice("stu_a", 1), choice("stu_b", 1)}})
	if same.Status != "proposed" {
		t.Fatalf("equal allowances should share one decision: %+v", same)
	}
	differ := s.prepareGroupTiming(group, LateTimingGroupDecision{GroupID: 44, Members: []LateTimingStudentDecision{choice("stu_a", 1), choice("stu_b", 2)}})
	if differ.Status != "held" {
		t.Fatalf("differing member allowances must hold shared timing: %+v", differ)
	}
}
