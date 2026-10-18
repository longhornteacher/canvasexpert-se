package workflow

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
	"gopkg.in/yaml.v3"
)

func TestSubmissionNullableTimingAndDueFieldsPreservePresence(t *testing.T) {
	var omitted Submission
	if err := json.Unmarshal([]byte(`{"late":false,"seconds_late":0}`), &omitted); err != nil {
		t.Fatal(err)
	}
	if omitted.CachedDueDatePresent || omitted.LatePolicyStatusKnown || !omitted.LateKnown || !omitted.SecondsLateKnown {
		t.Fatalf("omitted nullable fields lost presence: %+v", omitted)
	}
	var explicitNull Submission
	if err := json.Unmarshal([]byte(`{"cached_due_date":null,"late_policy_status":null,"custom_grade_status_id":null}`), &explicitNull); err != nil {
		t.Fatal(err)
	}
	if !explicitNull.CachedDueDatePresent || explicitNull.CachedDueDate != nil || !explicitNull.LatePolicyStatusKnown || explicitNull.LatePolicyStatus != nil || hasCustomGradeStatus(explicitNull) {
		t.Fatalf("explicit nulls were not preserved: %+v", explicitNull)
	}
	var explicitTiming Submission
	if err := json.Unmarshal([]byte(`{"late":true,"seconds_late":3600,"late_policy_status":"late","custom_grade_status_id":12}`), &explicitTiming); err != nil {
		t.Fatal(err)
	}
	observed := observedTiming(explicitTiming)
	if !explicitTiming.LatePolicyStatusKnown || !explicitTiming.SecondsLateKnown || !explicitTiming.LateKnown || !hasCustomGradeStatus(explicitTiming) || observed == nil || !observed.CustomStatusKnown || !observed.HasCustomStatus {
		t.Fatalf("returned current timing not represented: %+v / %+v", explicitTiming, observed)
	}
}

func TestSessionDueFilterUsesEffectiveStudentDateAndTermLocalDate(t *testing.T) {
	period := workspace.GradingPeriod{Name: "Term 2", StartDate: "2026-10-13", EndDate: "2026-12-18"}
	assignment := Assignment{DueAt: str("2026-10-08T23:00:00-05:00")}
	var absent Submission
	if submissionDueInPeriod(absent, assignment, period, "America/Chicago") {
		t.Fatal("assignment fallback in the preceding term should be excluded")
	}
	assignment.DueAt = str("2026-10-13T00:30:00Z") // still Oct 12 in Chicago
	if submissionDueInPeriod(absent, assignment, period, "America/Chicago") {
		t.Fatal("term membership must use local calendar date")
	}
	if err := json.Unmarshal([]byte(`{"cached_due_date":"2026-10-13T00:30:00-05:00"}`), &absent); err != nil {
		t.Fatal(err)
	}
	if !submissionDueInPeriod(absent, assignment, period, "America/Chicago") {
		t.Fatal("student-effective due date on inclusive term start should be included")
	}
	if err := json.Unmarshal([]byte(`{"cached_due_date":null}`), &absent); err != nil {
		t.Fatal(err)
	}
	if submissionDueInPeriod(absent, assignment, period, "America/Chicago") {
		t.Fatal("explicitly undated student override must not use assignment fallback")
	}
}

func TestPrepareLateTimingAcceptsExactPacketAndOnTimeNeedsNoCalendar(t *testing.T) {
	service, file, submitted := studentTimingFixture(t, &domain.ObservedTiming{StatusKnown: true, LateKnown: true, SecondsLateKnown: true, SecondsLate: 3600}, false)
	prepared, err := service.PrepareLateTiming(file)
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared.Students) != 1 || prepared.Students[0].Status != "unchanged" || prepared.Students[0].Timing == nil || prepared.Students[0].Timing.Status != "none" {
		t.Fatalf("expected an unchanged on-time proposal without a calendar: %+v", prepared)
	}
	if prepared.Students[0].Timing.SchoolTime.Attempt.SubmittedAt != submitted {
		t.Fatalf("provenance does not match the selected packet attempt: %+v", prepared.Students[0].Timing.SchoolTime)
	}
}

func TestPrepareLateTimingHoldsUnknownExplicitLateAndMismatchedAssignment(t *testing.T) {
	lateStatus := "late"
	late := observedTiming(Submission{Late: true, LateKnown: true, SecondsLate: 120, SecondsLateKnown: true, LatePolicyStatusKnown: true, LatePolicyStatus: &lateStatus})
	service, file, _ := studentTimingFixture(t, late, false)
	prepared, err := service.PrepareLateTiming(file)
	if err != nil {
		t.Fatal(err)
	}
	if got := prepared.Students[0]; got.Status != "held" || got.Timing != nil {
		t.Fatalf("unknown-origin explicit late must be held: %+v", got)
	}

	// A packet cannot lend a plausible attempt to a student document for a
	// different assignment.
	studentPath := filepath.Join(service.Root, "courses", "TEST", "unit", "student-work.md")
	var work domain.StudentWorkDocument
	if _, err = workspace.ReadDocument(studentPath, &work); err != nil {
		t.Fatal(err)
	}
	work.AssignmentID = 999
	if err = workspace.WriteDocument(studentPath, &work, ""); err != nil {
		t.Fatal(err)
	}
	if _, err = service.PrepareLateTiming(file); err == nil {
		t.Fatal("mismatched student-work assignment should be rejected")
	}
}

func TestPrepareLateTimingGroupRequiresEveryMemberAndKeepsMemberProvenance(t *testing.T) {
	service := &Service{Root: t.TempDir()}
	group := domain.GroupWorkDocument{GroupID: 44, CourseID: 2, AssignmentID: 10, RosterComplete: true, PropagationAudienceComplete: true, SharedAssessmentStatus: "ready", Assignment: domain.AssignmentSnapshot{GradeGroupStudentsIndividually: boolPtr(false)}, PropagationAudience: []string{"stu_a", "stu_b"}}
	observed := &domain.ObservedTiming{StatusKnown: true, LateKnown: true, Late: true, SecondsLateKnown: true}
	for _, ref := range group.PropagationAudience {
		group.Members = append(group.Members, domain.GroupWorkMember{StudentRef: ref, Attempt: 1, SubmittedAt: "2026-09-10T14:00:00Z", EffectiveDueAt: "2026-09-10T15:00:00Z", ObservedTiming: observed})
	}
	choice := func(ref string) LateTimingStudentDecision {
		return LateTimingStudentDecision{StudentRef: ref, FirstGenuine: domain.AttemptRef{Attempt: 1, SubmittedAt: "2026-09-10T14:00:00Z"}}
	}
	result := service.prepareGroupTiming(group, LateTimingGroupDecision{GroupID: 44, Members: []LateTimingStudentDecision{choice("stu_a"), choice("stu_b")}})
	if result.Status != "proposed" || result.Timing == nil || len(result.MemberSchoolTime) != 2 {
		t.Fatalf("expected one shared action and per-member provenance: %+v", result)
	}
	if result.MemberSchoolTime["stu_b"].Attempt.Attempt != 1 || result.ObservedMembers["stu_a"].LateKnown != true {
		t.Fatalf("shared result lost member identity or observation: %+v", result)
	}
	missing := service.prepareGroupTiming(group, LateTimingGroupDecision{GroupID: 44, Members: []LateTimingStudentDecision{choice("stu_a")}})
	if missing.Status != "held" {
		t.Fatalf("shared action requires choices for the full audience: %+v", missing)
	}
}

func TestCompletedTimingHintSkipsAcceptedNoneAfterCanvasKeepsElapsedSeconds(t *testing.T) {
	service, _, submitted := studentTimingFixture(t, nil, false)
	clientService, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) { t.Errorf("unexpected request %s", r.URL.Path) })
	service.Client = clientService.Client
	service.Config.Instance = clientService.Client.Origin()
	current := domain.AttemptRef{Attempt: 2, SubmittedAt: "2026-09-11T14:00:00Z"}
	source := domain.AttemptRef{Attempt: 1, SubmittedAt: submitted}
	due := "2026-09-10T15:00:00Z"
	points, _ := domain.ParseDecimal("100")
	snapshot := domain.AssignmentSnapshot{Instance: service.Config.Instance, CourseID: 2, AssignmentID: 10, Title: "Unit", Points: points, SubmissionTypes: []string{"online_text_entry"}}
	status := "none"
	baseline := &domain.ObservedTiming{StatusKnown: true, LateKnown: true, SecondsLateKnown: true, Late: true, SecondsLate: 3600}
	results := domain.ResultsDocument{Schema: 1, Kind: "results", Packet: "courses/TEST/unit/packet.md", Assignment: snapshot, Students: []domain.StudentResult{{StudentRef: "stu_a", Operation: "late-only", LateOnly: &domain.LateOnlyOperation{Attempt: current, LateTiming: domain.LateTiming{Status: "none", DueAt: due, Basis: "checked", SchoolTime: &domain.SchoolTimeProvenance{Attempt: source, SubmittedAt: submitted, Deadline: due, Status: "none"}, Observed: baseline}, Outcome: domain.ActionOutcome{Status: "succeeded"}}}}}
	encoded, err := yaml.Marshal(results)
	if err != nil {
		t.Fatal(err)
	}
	resultPath := filepath.Join(service.Root, "courses", "TEST", "unit", "results.md")
	if err := os.WriteFile(resultPath, append(append([]byte("---\n"), encoded...), []byte("---\n")...), 0600); err != nil {
		t.Fatal(err)
	}
	currentSubmitted, liveLate, seconds := current.SubmittedAt, false, int64(3600)
	priorSubmitted, subAttempt := submitted, current.Attempt
	sub := Submission{Attempt: subAttempt, SubmittedAt: &currentSubmitted, CachedDueDatePresent: true, CachedDueDate: &due, Late: liveLate, LateKnown: true, SecondsLate: seconds, SecondsLateKnown: true, LatePolicyStatus: &status, LatePolicyStatusKnown: true, History: []Submission{{Attempt: source.Attempt, SubmittedAt: &priorSubmitted}}}
	assignment := Assignment{ID: 10, CourseID: 2, DueAt: &due}
	if !service.sessionTimingCandidate(2, 10, "stu_a", sub) {
		var decoded domain.ResultsDocument
		_, readErr := workspace.ReadDocument(resultPath, &decoded)
		t.Fatalf("accepted CE none result should be reconsiderable on future material input changes; read=%v decoded=%+v yaml=%s", readErr, decoded.Students, encoded)
	}
	if !service.completedTimingHint(10, 2, "stu_a", sub, assignment, nil) {
		var decoded domain.ResultsDocument
		_, readErr := workspace.ReadDocument(resultPath, &decoded)
		timing := decoded.Students[0].LateOnly.LateTiming
		t.Fatalf("matching accepted none result should prevent a second session pull despite elapsed seconds: read=%v timing=%+v source=%v still=%v sub=%+v", readErr, timing, timingSourcePresent(timing, sub), timingStillMatches(timing, due, sub, nil), sub)
	}
}

func TestAcademicGradeCompletionIgnoresLateOutcome(t *testing.T) {
	grade := &domain.GradeOperation{Outcomes: domain.ActionOutcomes{Comment: domain.ActionOutcome{Status: "succeeded"}, Rubric: domain.ActionOutcome{Status: "not-requested"}, Grade: domain.ActionOutcome{Status: "succeeded"}, Late: domain.ActionOutcome{Status: "held"}}}
	if !academicGradeComplete(grade) || gradeComplete(grade) {
		t.Fatalf("academic completion must be independent from late timing: %+v", grade.Outcomes)
	}
}

func TestTimingCandidateSkipsProtectedCanvasStatuses(t *testing.T) {
	for _, status := range []string{"none", "extended"} {
		s := Submission{Late: false, SecondsLate: 3600, LatePolicyStatusKnown: true, LatePolicyStatus: &status}
		if timingCandidate(s) {
			t.Fatalf("protected status %q must not trigger a timing download", status)
		}
	}
}

func TestSessionPullSkipsAcceptedNormalizedTimingOnSecondRun(t *testing.T) {
	var live Submission
	var assignment Assignment
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/courses/1/enrollments":
			_ = json.NewEncoder(w).Encode([]Enrollment{{UserID: 5, Type: "StudentEnrollment", State: "active", User: CanvasUser{ID: 5, Name: "Synthetic Student"}}})
		case r.URL.Path == "/api/v1/courses/1/assignments":
			_ = json.NewEncoder(w).Encode([]Assignment{assignment})
		case r.URL.Path == "/api/v1/courses/1/assignments/10/submissions":
			_ = json.NewEncoder(w).Encode([]Submission{live})
		default:
			t.Errorf("unexpected session pull request %s", r.URL.Path)
			_ = json.NewEncoder(w).Encode([]any{})
		}
	})
	start := "1900-01-01"
	end := "2200-12-31"
	if err := os.MkdirAll(filepath.Join(s.Root, "context"), 0700); err != nil {
		t.Fatal(err)
	}
	calendar := workspace.SchoolCalendar{Schema: 1, Timezone: "America/Chicago", StartDate: start, EndDate: end, NonSchoolDates: []string{}, NonSchoolRanges: []workspace.DateRange{}, GradingPeriods: []workspace.GradingPeriod{{Name: "All", StartDate: start, EndDate: end}}}
	if err := workspace.WriteTeacherDocument(s.Root, "context/school-calendar.md", calendar, "Synthetic calendar"); err != nil {
		t.Fatal(err)
	}
	due := "2026-10-15T16:00:00-05:00"
	submitted := "2026-10-15T20:00:00Z"
	currentSubmitted := "2026-10-16T16:00:00Z"
	assignment = fixtureAssignment()
	assignment.DueAt = &due
	lateStatus := "none"
	live = Submission{ID: 100, UserID: 5, AssignmentID: 10, Attempt: 2, SubmittedAt: &currentSubmitted, Type: "online_text_entry", State: "graded", Late: false, LateKnown: true, LatePolicyStatus: &lateStatus, LatePolicyStatusKnown: true, SecondsLate: 86400, SecondsLateKnown: true, CachedDueDate: &due, CachedDueDatePresent: true, GradeMatchesCurrentSubmission: boolPtr(true), User: CanvasUser{ID: 5, Name: "Synthetic Student"}, History: []Submission{{Attempt: 1, SubmittedAt: &submitted}}}
	store, err := workspace.OpenStudents(s.Root)
	if err != nil {
		t.Fatal(err)
	}
	student, err := store.Upsert(s.Client.Origin(), 5, "Synthetic Student", "Synthetic Student", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(); err != nil {
		t.Fatal(err)
	}
	points, _ := domain.ParseDecimal("20")
	snapshot := domain.AssignmentSnapshot{Instance: s.Client.Origin(), CourseID: 1, AssignmentID: 10, Title: assignment.Name, Points: points, SubmissionTypes: []string{"online_text_entry"}}
	currentRef := domain.AttemptRef{Attempt: 2, SubmittedAt: currentSubmitted}
	sourceRef := domain.AttemptRef{Attempt: 1, SubmittedAt: submitted}
	observed := &domain.ObservedTiming{StatusKnown: true, LateKnown: true, SecondsLateKnown: true, SecondsLate: 86400}
	result := domain.ResultsDocument{Schema: 1, Kind: "results", Packet: "courses/TEST/unit/packet.md", Assignment: snapshot, Students: []domain.StudentResult{{StudentRef: student.Ref, Operation: "late-only", LateOnly: &domain.LateOnlyOperation{Attempt: currentRef, LateTiming: domain.LateTiming{Status: "none", DueAt: due, Basis: "verified first attempt", SchoolTime: &domain.SchoolTimeProvenance{Attempt: sourceRef, SubmittedAt: submitted, Deadline: due, Status: "none"}, Observed: observed}, Outcome: domain.ActionOutcome{Status: "succeeded"}}}}}
	if err := os.MkdirAll(filepath.Join(s.Root, "courses", "TEST", "unit"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := workspace.WriteDocument(filepath.Join(s.Root, "courses", "TEST", "unit", "results.md"), &result, "Synthetic accepted timing"); err != nil {
		t.Fatal(err)
	}
	pull, err := s.PullWorkWithOptions(context.Background(), "TEST", "", PullWorkOptions{Session: true})
	if err != nil {
		t.Fatal(err)
	}
	if pull.Students != 0 || len(pull.Packets) != 0 || pull.Assignments[0].Handled != 1 || pull.FetchRequests != 0 {
		t.Fatalf("accepted normalized timing recurred in the second session: %+v", pull)
	}
}

func TestSessionPullCreatesLateOnlyPacketAndPreparationConsumesIt(t *testing.T) {
	var assignment Assignment
	var live Submission
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/courses/1/enrollments":
			_ = json.NewEncoder(w).Encode([]Enrollment{{UserID: 5, Type: "StudentEnrollment", State: "active", User: CanvasUser{ID: 5, Name: "Synthetic Student"}}})
		case "/api/v1/courses/1/assignments":
			_ = json.NewEncoder(w).Encode([]Assignment{assignment})
		case "/api/v1/courses/1/assignments/10":
			_ = json.NewEncoder(w).Encode(assignment)
		case "/api/v1/courses/1/assignments/10/submissions/5":
			_ = json.NewEncoder(w).Encode(live)
		case "/api/v1/courses/1/assignments/10/submissions":
			_ = json.NewEncoder(w).Encode([]Submission{live})
		default:
			t.Errorf("unexpected scoring-session request %s", r.URL.Path)
			_ = json.NewEncoder(w).Encode([]any{})
		}
	})
	start, end := "1900-01-01", "2200-12-31"
	if err := os.MkdirAll(filepath.Join(s.Root, "context"), 0700); err != nil {
		t.Fatal(err)
	}
	calendar := workspace.SchoolCalendar{Schema: 1, Timezone: "America/Chicago", StartDate: start, EndDate: end, NonSchoolDates: []string{}, NonSchoolRanges: []workspace.DateRange{}, GradingPeriods: []workspace.GradingPeriod{{Name: "All", StartDate: start, EndDate: end}}}
	if err := workspace.WriteTeacherDocument(s.Root, "context/school-calendar.md", calendar, "Synthetic calendar"); err != nil {
		t.Fatal(err)
	}
	due := "2026-10-15T16:00:00-05:00"
	firstSubmitted := "2026-10-15T22:00:00Z"
	currentSubmitted := "2026-10-16T22:00:00Z"
	assignment = fixtureAssignment()
	assignment.DueAt = &due
	visible := true
	live = Submission{ID: 100, UserID: 5, AssignmentID: 10, Attempt: 2, SubmittedAt: &currentSubmitted, CachedDueDate: &due, CachedDueDatePresent: true, Type: "online_text_entry", State: "graded", Late: true, LateKnown: true, LatePolicyStatusKnown: true, SecondsLate: 7200, SecondsLateKnown: true, GradeMatchesCurrentSubmission: boolPtr(true), AssignmentVisible: &visible, Body: "The student's current evidence.", User: CanvasUser{ID: 5, Name: "Synthetic Student"}, History: []Submission{{Attempt: 1, SubmittedAt: &firstSubmitted, Type: "online_text_entry"}}}
	resultsPath := scoringFiles(t, s, 5)
	var results domain.ResultsDocument
	if _, err := workspace.ReadDocument(resultsPath, &results); err != nil {
		t.Fatal(err)
	}
	results.Students[0].Grade.Attempt = domain.AttemptRef{Attempt: 2, SubmittedAt: currentSubmitted}
	results.Students[0].Grade.Outcomes = domain.ActionOutcomes{Comment: domain.ActionOutcome{Status: "succeeded"}, Rubric: domain.ActionOutcome{Status: "not-requested"}, Grade: domain.ActionOutcome{Status: "succeeded"}, Late: domain.ActionOutcome{Status: "held"}}
	if err := workspace.WriteDocument(resultsPath, &results, "Accepted academic score; timing held for separate review."); err != nil {
		t.Fatal(err)
	}
	pull, err := s.PullWorkWithOptions(context.Background(), "TEST", "", PullWorkOptions{Session: true})
	if err != nil {
		t.Fatal(err)
	}
	if pull.Students != 1 || len(pull.Packets) != 1 {
		t.Fatalf("expected one timing-only evidence packet: %+v", pull)
	}
	var packet domain.PacketDocument
	if _, err := workspace.ReadDocument(pull.Packets[0], &packet); err != nil {
		t.Fatal(err)
	}
	var work domain.StudentWorkDocument
	studentPath, err := workspace.ConstrainPath(s.Root, filepath.FromSlash(packet.Students[0]))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.ReadDocument(studentPath, &work); err != nil {
		t.Fatal(err)
	}
	if work.WorkPurpose != "late-only" || work.Current.ObservedTiming == nil || !work.Current.ObservedTiming.StatusKnown || len(work.Prior) != 1 {
		t.Fatalf("session packet lost timing-only purpose or history: %+v", work)
	}
	decision := LateTimingDecisionsDocument{Schema: 1, Kind: "late-timing-decisions", Packet: filepath.ToSlash(mustRel(t, s.Root, pull.Packets[0])), Students: []LateTimingStudentDecision{{StudentRef: work.StudentRef, FirstGenuine: work.Prior[0].Ref}}}
	decisionPath := filepath.Join(filepath.Dir(studentPath), "late-timing-decisions.md")
	if err := workspace.WriteDocument(decisionPath, &decision, "First genuine attempt confirmed from submission history."); err != nil {
		t.Fatal(err)
	}
	prepared, err := s.PrepareLateTiming(mustRel(t, s.Root, decisionPath))
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared.Students) != 1 || prepared.Students[0].Status != "proposed" || prepared.Students[0].Timing == nil || prepared.Students[0].Timing.Status != "late" || prepared.Students[0].Timing.SecondsLate != 3600 {
		t.Fatalf("preparation did not calculate the first-genuine school-time interval: %+v", prepared)
	}
}

func TestCalculateTimingCountsAccommodatedDeadlineWithoutChangingCanvasGuard(t *testing.T) {
	attempt := domain.AttemptRef{Attempt: 1, SubmittedAt: "2026-10-16T18:30:00Z"}
	observed := &domain.ObservedTiming{StatusKnown: true, LateKnown: true, SecondsLateKnown: true}
	choice := LateTimingStudentDecision{FirstGenuine: attempt, AccommodatedDeadline: "2026-10-16T19:00:00Z", AccommodationBasis: "IEP extended deadline"}
	s := &Service{Root: t.TempDir()}
	timing, reason := s.calculateTiming(choice, "2026-10-16T18:00:00Z", observed, []domain.AttemptWork{{Ref: attempt}}, attempt)
	if reason != "" || timing == nil {
		t.Fatalf("accommodated on-time work should prepare without a calendar: reason=%q timing=%+v", reason, timing)
	}
	if timing.Status != "none" || timing.SecondsLate != 0 || timing.DueAt != "2026-10-16T18:00:00Z" || timing.Basis != "IEP extended deadline" {
		t.Fatalf("Canvas due date must remain the write guard and the accommodation basis must be kept: %+v", timing)
	}
	if timing.SchoolTime == nil || timing.SchoolTime.Deadline != "2026-10-16T19:00:00Z" || timing.SchoolTime.Attempt != attempt {
		t.Fatalf("counted deadline provenance should use the accommodated deadline: %+v", timing.SchoolTime)
	}
}

func TestPrepareLateTimingUsesEstablishedExtraTimeDeadlineOnce(t *testing.T) {
	const extended = "2026-10-16T16:00:00-05:00" // synthetic rule: one additional school day
	for _, tc := range []struct{ name, canvasDue, basis string }{
		{"allowance outside Canvas", "2026-10-15T16:00:00-05:00", "Synthetic standing allowance adds one school day; Canvas still has the base date."},
		{"Canvas already includes allowance", extended, "Synthetic standing allowance is already included in Canvas; use that deadline once."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			observed := &domain.ObservedTiming{StatusKnown: true, LateKnown: true, Late: true, SecondsLateKnown: true, SecondsLate: 400000}
			service, file, _ := studentTimingFixture(t, observed, false)
			calendar := workspace.SchoolCalendar{Schema: 1, Timezone: "America/Chicago", StartDate: "2026-08-01", EndDate: "2027-05-31", NonSchoolDates: []string{"2026-10-19"}}
			if err := workspace.WriteTeacherDocument(service.Root, "context/school-calendar.md", calendar, "Synthetic calendar with Monday off."); err != nil {
				t.Fatal(err)
			}
			studentPath := filepath.Join(service.Root, "courses", "TEST", "unit", "student-work.md")
			var work domain.StudentWorkDocument
			if _, err := workspace.ReadDocument(studentPath, &work); err != nil {
				t.Fatal(err)
			}
			work.ExtraTime = boolPtr(true)
			first := domain.AttemptRef{Attempt: 1, SubmittedAt: "2026-10-20T16:30:00-05:00"}
			work.Current.Ref = domain.AttemptRef{Attempt: 2, SubmittedAt: "2026-10-21T16:30:00-05:00"}
			work.Current.EffectiveDueAt = tc.canvasDue
			work.Prior = []domain.AttemptWork{{Ref: first, ReadableText: "First genuine academic response."}}
			if err := workspace.WriteDocument(studentPath, work, ""); err != nil {
				t.Fatal(err)
			}
			var decisions LateTimingDecisionsDocument
			decisionPath := filepath.Join(service.Root, filepath.FromSlash(file))
			if _, err := workspace.ReadDocument(decisionPath, &decisions); err != nil {
				t.Fatal(err)
			}
			decisions.Students[0].FirstGenuine = first
			decisions.Students[0].AccommodatedDeadline = extended
			decisions.Students[0].AccommodationBasis = tc.basis
			if err := workspace.WriteDocument(decisionPath, decisions, ""); err != nil {
				t.Fatal(err)
			}
			prepared, err := service.PrepareLateTiming(file)
			if err != nil || len(prepared.Students) != 1 {
				t.Fatalf("preparation=%+v err=%v", prepared, err)
			}
			row := prepared.Students[0]
			if row.Status != "proposed" || row.Timing == nil || row.Timing.Status != "late" || row.Timing.SecondsLate != 88200 || row.Timing.DueAt != tc.canvasDue || row.Timing.Basis != tc.basis {
				t.Fatalf("allowance must be counted once without replacing Canvas's write guard: %+v", row)
			}
			provenance := row.Timing.SchoolTime
			if provenance == nil || provenance.Deadline != extended || provenance.Attempt != first || len(provenance.SkippedDates) != 3 {
				t.Fatalf("weekend/holiday or first-genuine provenance lost: %+v", provenance)
			}
		})
	}
}

func mustRel(t *testing.T, root, path string) string {
	t.Helper()
	rel, err := filepath.Rel(root, path)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.ToSlash(rel)
}

func studentTimingFixture(t *testing.T, observed *domain.ObservedTiming, invisible bool) (*Service, string, string) {
	t.Helper()
	root := t.TempDir()
	unitDir := filepath.Join(root, "courses", "TEST", "unit")
	if err := os.MkdirAll(unitDir, 0700); err != nil {
		t.Fatal(err)
	}
	submitted := time.Date(2026, 9, 10, 14, 0, 0, 0, time.UTC).Format(time.RFC3339)
	visible := !invisible
	work := domain.StudentWorkDocument{Schema: 1, Kind: "student-work", StudentRef: "stu_a", AssignmentID: 10, AssignmentVisible: &visible, Current: domain.AttemptWork{Ref: domain.AttemptRef{Attempt: 1, SubmittedAt: submitted}, ObservedTiming: observed, EffectiveDueAt: "2026-09-10T15:00:00Z"}}
	studentPath := filepath.Join(unitDir, "student-work.md")
	if err := workspace.WriteDocument(studentPath, &work, ""); err != nil {
		t.Fatal(err)
	}
	points, _ := domain.ParseDecimal("100")
	assignment := domain.AssignmentSnapshot{Instance: "https://canvas.example.edu", CourseID: 2, AssignmentID: 10, Title: "Unit", Points: points, SubmissionTypes: []string{"online_text_entry"}}
	packet := domain.PacketDocument{Schema: 1, Kind: "packet", Course: "TEST", Assignment: assignment, Students: []string{"courses/TEST/unit/student-work.md"}}
	if err := workspace.WriteDocument(filepath.Join(unitDir, "packet.md"), &packet, ""); err != nil {
		t.Fatal(err)
	}
	decision := LateTimingDecisionsDocument{Schema: 1, Kind: "late-timing-decisions", Packet: "courses/TEST/unit/packet.md", Students: []LateTimingStudentDecision{{StudentRef: "stu_a", FirstGenuine: domain.AttemptRef{Attempt: 1, SubmittedAt: submitted}}}}
	decisionPath := filepath.Join(unitDir, "decisions.md")
	if err := workspace.WriteDocument(decisionPath, &decision, ""); err != nil {
		t.Fatal(err)
	}
	return &Service{Root: root, Config: domain.CourseConfig{Instance: assignment.Instance, Courses: []domain.Course{{Nickname: "TEST", ID: 2}}}}, "courses/TEST/unit/decisions.md", submitted
}
