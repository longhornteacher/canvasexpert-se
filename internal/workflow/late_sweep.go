package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

// LateSweepReport is the read-only result of recomputing a course's late
// marks in school time. Proposed rows are saved as ordinary late-only results
// files, one per assignment, for the existing post path.
type LateSweepReport struct {
	Course      string                `json:"course"`
	Period      string                `json:"period"`
	Folder      string                `json:"folder,omitempty"`
	Proposed    int                   `json:"proposed"`
	Unchanged   int                   `json:"unchanged"`
	Held        int                   `json:"held"`
	CheckFirst  int                   `json:"check_first_attempt"`
	Assignments []LateSweepAssignment `json:"assignments,omitempty"`
	Skipped     []string              `json:"skipped,omitempty"`
}

type LateSweepAssignment struct {
	AssignmentID int64  `json:"assignment_id"`
	Title        string `json:"title"`
	// Results is the late-only results file to post after the teacher's
	// go-ahead; empty when no row needs a change.
	Results string         `json:"results,omitempty"`
	Rows    []LateSweepRow `json:"rows"`
}

type LateSweepRow struct {
	StudentRef string `json:"student_ref"`
	Alias      string `json:"alias"`
	Status     string `json:"status"` // proposed, unchanged, held
	Reason     string `json:"reason,omitempty"`
	// CanvasSecondsLate is what Canvas shows now; SchoolSecondsLate is the
	// school-time count (0 with NewStatus none means on time).
	CanvasSecondsLate int64    `json:"canvas_seconds_late"`
	NewStatus         string   `json:"new_status,omitempty"`
	SchoolSecondsLate int64    `json:"school_seconds_late,omitempty"`
	CanvasDue         string   `json:"canvas_due,omitempty"`
	Deadline          string   `json:"deadline,omitempty"`
	ExtraSchoolDays   int      `json:"extra_school_days,omitempty"`
	AbsentDates       []string `json:"absent_dates,omitempty"`
	SkippedDates      []string `json:"skipped_dates,omitempty"`
	// CheckFirstAttempt marks a student with more than one attempt: the sweep
	// timed the earliest one without reading it.
	CheckFirstAttempt bool   `json:"check_first_attempt,omitempty"`
	Note              string `json:"note,omitempty"`
}

func sweepAssignments(ctx context.Context, s *Service, courseID int64) ([]Assignment, error) {
	var result []Assignment
	err := s.Client.List(ctx, fmt.Sprintf("/api/v1/courses/%d/assignments?per_page=100&include[]=all_dates&include[]=overrides", courseID), &result)
	return result, err
}

func inWindow(value *string, start, end time.Time) bool {
	if value == nil {
		return false
	}
	t, err := time.Parse(time.RFC3339Nano, *value)
	return err == nil && !t.Before(start) && t.Before(end)
}

// dueInWindow reports whether any of the assignment's due dates falls in the period.
func dueInWindow(a Assignment, start, end time.Time) bool {
	if inWindow(a.DueAt, start, end) {
		return true
	}
	if a.AllDates != nil {
		for _, d := range *a.AllDates {
			if inWindow(d.DueAt, start, end) {
				return true
			}
		}
	}
	for _, o := range a.Overrides {
		if inWindow(o.DueAt, start, end) {
			return true
		}
	}
	return false
}

// personalDueDate reports whether an override names this student and sets a due date.
func personalDueDate(a Assignment, userID int64) bool {
	for _, o := range a.Overrides {
		if o.DueAt == nil {
			continue
		}
		for _, id := range o.StudentIDs {
			if id == userID {
				return true
			}
		}
	}
	return false
}

// timingAttempt keeps what a timing decision needs and none of the student's work.
func timingAttempt(sub Submission) domain.AttemptWork {
	w := domain.AttemptWork{Ref: attemptRef(sub), SubmissionType: sub.Type, Late: sub.Late, ObservedTiming: observedTiming(sub)}
	if sub.CachedDueDate != nil {
		w.EffectiveDueAt = *sub.CachedDueDate
	}
	return w
}

// LateSweep recomputes lateness for every submission Canvas shows as late on
// ordinary individual assignments due in a grading period. It reads Canvas and
// writes local files only.
func (s *Service) LateSweep(ctx context.Context, nickname, periodName string, now time.Time) (LateSweepReport, error) {
	course, err := s.Course(nickname)
	if err != nil {
		return LateSweepReport{}, err
	}
	outer := s
	if s, err = s.ForCourse(ctx, course); err != nil {
		return LateSweepReport{}, err
	}
	s = s.withResultsIndex()
	report := LateSweepReport{Course: course.Nickname}
	calendar, err := workspace.ReadSchoolCalendar(s.Root)
	if err == nil {
		err = calendar.Validate()
	}
	if err != nil {
		return report, fmt.Errorf("the school calendar (context/school-calendar.md) is needed to fix late marks: %w", err)
	}
	var period workspace.GradingPeriod
	if periodName == "" {
		if period, err = calendar.CurrentPeriod(now); err != nil {
			return report, fmt.Errorf("%w; name a grading period from context/school-calendar.md", err)
		}
	} else {
		found := false
		for _, p := range calendar.GradingPeriods {
			if strings.EqualFold(p.Name, periodName) {
				period, found = p, true
			}
		}
		if !found {
			return report, fmt.Errorf("grading period %q is not in context/school-calendar.md", periodName)
		}
	}
	report.Period = period.Name
	start, _ := period.LocalStart(calendar.Timezone)
	end, _ := period.LocalEndExclusive(calendar.Timezone)
	policy, err := s.lateWorkFor(course)
	if err != nil {
		return report, err
	}
	generalDays := 0
	if policy != nil {
		generalDays = policy.ExtraTimeSchoolDays
	}
	_, owned, err := outer.Owned(course)
	if err != nil {
		return report, err
	}
	students, err := workspace.OpenStudents(s.Root)
	if err != nil {
		return report, err
	}
	enrollments, err := StudentEnrollments(ctx, s.Client, course.ID)
	if err != nil {
		return report, err
	}
	active := map[int64]workspace.Student{}
	for _, enrollment := range enrollments {
		userID := enrollment.UserID
		if userID == 0 {
			userID = enrollment.User.ID
		}
		if userID <= 0 || enrollment.State != "active" {
			continue
		}
		st, e := students.Upsert(s.Client.Origin(), userID, enrollment.User.Name, enrollment.User.ShortName, enrollment.User.SISID, enrollment.User.Login)
		if e != nil {
			return report, e
		}
		active[userID] = st
	}
	// Persist new stable references before writing any outward projection.
	if err = students.Save(); err != nil {
		return report, err
	}
	assignments, err := sweepAssignments(ctx, s, course.ID)
	if err != nil {
		return report, err
	}
	stamp := now.UTC().Format("20060102T150405")
	sweepDir := filepath.Join("courses", course.Nickname, "late-sweeps", stamp)
	untagged := 0
	for _, a := range assignments {
		if !a.Published || !dueInWindow(a, start, end) {
			continue
		}
		if owned && !domain.HasConsentTag(a.Name) {
			untagged++
			continue
		}
		if a.CourseID == 0 {
			a.CourseID = course.ID
		}
		reason := unpullableAssignment(a)
		if reason == "" {
			reason = lateTimingAssignmentHold(a, false)
		}
		if reason != "" {
			report.Skipped = append(report.Skipped, fmt.Sprintf("%s: %s", a.Name, reason))
			continue
		}
		subs, e := Submissions(ctx, s.Client, course.ID, a.ID, true)
		if e != nil {
			return report, e
		}
		snapshot, e := TimingSnapshot(a, s.Client.Origin())
		if e != nil {
			report.Skipped = append(report.Skipped, fmt.Sprintf("%s: %v", a.Name, e))
			continue
		}
		snapshot.DescriptionHTML = ""
		relDir := filepath.Join(sweepDir, fmt.Sprintf("%d-%s", a.ID, Slug(a.Name)))
		packetRel := filepath.ToSlash(filepath.Join(relDir, "packet.md"))
		packet := domain.PacketDocument{Schema: 1, Kind: "packet", Course: course.Nickname, Assignment: snapshot}
		results := domain.ResultsDocument{Schema: 1, Kind: "results", Packet: packetRel, Assignment: snapshot}
		entry := LateSweepAssignment{AssignmentID: a.ID, Title: a.Name}
		works := map[string]domain.StudentWorkDocument{}
		for _, sub := range subs {
			st, ok := active[sub.UserID]
			if !ok || !sub.Late || unsupportedSubmission(sub) != "" || !inWindow(sub.CachedDueDate, start, end) {
				continue
			}
			alias := st.Alias
			if !s.Config.Aliases {
				alias = st.Name
			}
			work := domain.StudentWorkDocument{Schema: 1, Kind: "student-work", StudentRef: st.Ref, Alias: alias, AssignmentID: a.ID, AssignmentVisible: sub.AssignmentVisible, Excused: sub.Excused, Monitored: st.Monitored, ExtraTime: &st.ExtraTime, ExtraTimeSchoolDays: st.ExtraTimeDays, Current: timingAttempt(sub)}
			first := work.Current
			for _, prior := range priorAttempts(sub) {
				attempt := timingAttempt(prior)
				work.Prior = append(work.Prior, attempt)
				if attempt.Ref.Attempt < first.Ref.Attempt {
					first = attempt
				}
			}
			sort.Slice(work.Prior, func(i, j int) bool { return work.Prior[i].Ref.Attempt < work.Prior[j].Ref.Attempt })
			row := LateSweepRow{StudentRef: st.Ref, Alias: alias, CanvasSecondsLate: sub.SecondsLate, CanvasDue: work.Current.EffectiveDueAt, CheckFirstAttempt: len(work.Prior) > 0}
			choice := LateTimingStudentDecision{StudentRef: st.Ref, FirstGenuine: first.Ref}
			held := ""
			if st.ExtraTime {
				if personalDueDate(a, sub.UserID) {
					row.Note = "This student has a personal Canvas due date, so extra days were not added again."
				} else if due, e := parseOptionalTime(sub.CachedDueDate); e == nil && due != nil {
					choice.AllowanceSchoolDays, choice.AccommodationBasis, held = extraSchoolDays(st, a, *due, calendar, generalDays)
				}
			}
			var prepared PreparedLateTiming
			if held != "" {
				prepared = PreparedLateTiming{StudentRef: st.Ref, Status: "held", Reason: held}
			} else {
				prepared = s.prepareStudentTiming(work, choice, course.ID)
			}
			row.Status, row.Reason, row.ExtraSchoolDays = prepared.Status, prepared.Reason, choice.AllowanceSchoolDays
			if t := prepared.Timing; t != nil {
				if choice.AccommodationBasis == "" {
					t.Basis = "Earliest submission compared with the Canvas due date, counted in school time."
				}
				row.NewStatus, row.SchoolSecondsLate = t.Status, t.SecondsLate
				if t.SchoolTime != nil {
					row.Deadline, row.AbsentDates, row.SkippedDates = t.SchoolTime.Deadline, t.SchoolTime.AbsentDates, t.SchoolTime.SkippedDates
				}
			}
			switch row.Status {
			case "proposed":
				report.Proposed++
				works[st.Ref] = work
				results.Students = append(results.Students, domain.StudentResult{StudentRef: st.Ref, Operation: "late-only", LateOnly: &domain.LateOnlyOperation{Attempt: work.Current.Ref, LateTiming: *prepared.Timing}})
				if row.CheckFirstAttempt {
					report.CheckFirst++
				}
			case "unchanged":
				report.Unchanged++
			default:
				report.Held++
			}
			entry.Rows = append(entry.Rows, row)
		}
		if len(entry.Rows) == 0 {
			continue
		}
		if len(results.Students) > 0 {
			if _, e := domain.ValidateResults(&results); e != nil {
				return report, fmt.Errorf("%s: prepared late marks did not validate: %w", a.Name, e)
			}
			for _, r := range results.Students {
				file, e := workspace.ConstrainPath(s.Root, filepath.Join(relDir, "students", r.StudentRef+".md"))
				if e != nil {
					return report, e
				}
				if e = workspace.WriteDocument(file, works[r.StudentRef], "Timing facts only; the student's work was not pulled.\n"); e != nil {
					return report, e
				}
				rel, _ := filepath.Rel(s.Root, file)
				packet.Students = append(packet.Students, filepath.ToSlash(rel))
			}
			packetFile, e := workspace.ConstrainPath(s.Root, filepath.FromSlash(packetRel))
			if e != nil {
				return report, e
			}
			if e = workspace.WriteDocument(packetFile, packet, "Late-mark sweep: timing facts only.\n"); e != nil {
				return report, e
			}
			resultsFile, e := workspace.ConstrainPath(s.Root, filepath.Join(relDir, "results.md"))
			if e != nil {
				return report, e
			}
			if e = workspace.WriteDocument(resultsFile, results, "Prepared by the late-mark sweep. Post only after the teacher's go-ahead; remove a row the teacher holds back.\n"); e != nil {
				return report, e
			}
			entry.Results = filepath.ToSlash(filepath.Join(relDir, "results.md"))
		}
		report.Assignments = append(report.Assignments, entry)
	}
	if untagged > 0 {
		report.Skipped = append(report.Skipped, fmt.Sprintf("%d assignments not tagged (CE) were skipped.", untagged))
	}
	if report.Proposed > 0 {
		report.Folder = filepath.ToSlash(sweepDir)
		encoded, e := json.MarshalIndent(report, "", "  ")
		if e != nil {
			return report, e
		}
		file, e := workspace.ConstrainPath(s.Root, filepath.Join(sweepDir, "sweep.json"))
		if e != nil {
			return report, e
		}
		if e = workspace.WriteFile(file, append(encoded, '\n'), 0600); e != nil {
			return report, e
		}
	}
	return report, nil
}
