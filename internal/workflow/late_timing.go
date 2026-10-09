package workflow

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

type SchoolCalendar struct {
	Schema          int               `yaml:"schema"`
	Timezone        string            `yaml:"timezone"`
	StartDate       string            `yaml:"start_date"`
	EndDate         string            `yaml:"end_date"`
	NonSchoolDates  []string          `yaml:"non_school_dates"`
	NonSchoolRanges []SchoolDateRange `yaml:"non_school_ranges"`
}
type SchoolDateRange struct {
	StartDate string `yaml:"start_date"`
	EndDate   string `yaml:"end_date"`
}

// SchoolSeconds counts elapsed clock time on student school dates, not class hours.
// Local midnight boundaries deliberately use AddDate so DST dates keep their real length.
func SchoolSeconds(calendar SchoolCalendar, due, submitted time.Time) (int64, error) {
	if calendar.Schema != 1 {
		return 0, fmt.Errorf("school calendar requires schema 1")
	}
	loc, e := time.LoadLocation(calendar.Timezone)
	if e != nil || calendar.Timezone == "" {
		return 0, fmt.Errorf("school calendar requires a valid timezone")
	}
	parse := func(v string) (time.Time, error) { return time.ParseInLocation("2006-01-02", v, loc) }
	start, e := parse(calendar.StartDate)
	if e != nil {
		return 0, fmt.Errorf("invalid calendar start_date")
	}
	end, e := parse(calendar.EndDate)
	if e != nil || end.Before(start) {
		return 0, fmt.Errorf("invalid calendar end_date")
	}
	end = end.AddDate(0, 0, 1)
	if due.Before(start) || !due.Before(end) || submitted.Before(start) || !submitted.Before(end) {
		return 0, fmt.Errorf("lateness dates fall outside school calendar coverage %s through %s", calendar.StartDate, calendar.EndDate)
	}
	excluded := map[string]bool{}
	for _, v := range calendar.NonSchoolDates {
		day, e := parse(v)
		if e != nil {
			return 0, fmt.Errorf("invalid non_school_dates date %q", v)
		}
		excluded[day.Format("2006-01-02")] = true
	}
	for _, r := range calendar.NonSchoolRanges {
		first, e := parse(r.StartDate)
		if e != nil {
			return 0, fmt.Errorf("invalid non_school_ranges start_date")
		}
		last, e := parse(r.EndDate)
		if e != nil || last.Before(first) || first.Before(start) || !last.Before(end) {
			return 0, fmt.Errorf("invalid or uncovered non_school_ranges interval")
		}
		for day := first; !day.After(last); day = day.AddDate(0, 0, 1) {
			excluded[day.Format("2006-01-02")] = true
		}
	}
	if !submitted.After(due) {
		return 0, nil
	}
	local := due.In(loc)
	day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	var elapsed time.Duration
	for day.Before(submitted) {
		next := day.AddDate(0, 0, 1)
		if day.Weekday() != time.Saturday && day.Weekday() != time.Sunday && !excluded[day.Format("2006-01-02")] {
			from, to := day, next
			if due.After(from) {
				from = due
			}
			if submitted.Before(to) {
				to = submitted
			}
			if to.After(from) {
				elapsed += to.Sub(from)
			}
		}
		day = next
	}
	return int64(elapsed / time.Second), nil
}

func timingMatches(sub Submission, target domain.LateTimingTarget) bool {
	if sub.LatePolicyStatus == nil || *sub.LatePolicyStatus != target.Status {
		return false
	}
	if target.Seconds == 0 {
		return !sub.Late
	}
	return sub.SecondsLate == target.Seconds
}

func (s *Service) postLate(ctx context.Context, doc domain.ResultsDocument, userID int64, work domain.StudentWorkDocument, g *domain.GradeOperation, save func() error) error {
	hold := func(message string) error { holdOutcome(&g.Outcomes.Late, message); return save() }
	timing := g.Timing()
	if timing == nil {
		return nil
	}
	first, e := time.Parse(time.RFC3339Nano, timing.FirstAttempt.SubmittedAt)
	if e != nil || timing.FirstAttempt.Attempt < 1 || strings.TrimSpace(timing.GenuineReason) == "" {
		return hold("late timing requires the selected genuine submission identity, timestamp and reason")
	}
	due, e := time.Parse(time.RFC3339Nano, timing.EffectiveDueAt)
	if e != nil {
		return hold("late timing requires the student's applicable cached due date")
	}
	found := false
	for _, attempt := range append([]domain.AttemptWork{work.Current}, work.Prior...) {
		if sameAttempt(attempt.Ref, timing.FirstAttempt) && sameDate(&attempt.EffectiveDueAt, &timing.EffectiveDueAt) {
			found = true
		}
	}
	if !found {
		return hold("selected genuine submission or its applicable due date is absent from pulled work")
	}
	var calendar SchoolCalendar
	if _, e := workspace.ReadDocument(filepath.Join(s.Root, "settings", "school-calendar.md"), &calendar); e != nil {
		return hold("school calendar unavailable: " + e.Error())
	}
	seconds, e := SchoolSeconds(calendar, due, first)
	if e != nil {
		return hold(e.Error())
	}
	target := domain.LateTimingTarget{Seconds: seconds, Status: "late"}
	if seconds == 0 {
		target.Status = "none"
	}
	live, e := GetSubmission(ctx, s.Client, doc.Assignment.CourseID, doc.Assignment.AssignmentID, userID)
	if e != nil {
		return hold(e.Error())
	}
	if !sameAttempt(g.Attempt, attemptRef(live)) {
		return hold("new submission before late timing adjustment")
	}
	found = false
	for _, attempt := range append([]Submission{live}, live.History...) {
		if sameAttempt(attemptRef(attempt), timing.FirstAttempt) && attempt.CachedDueDate != nil && sameDate(attempt.CachedDueDate, &timing.EffectiveDueAt) {
			found = true
		}
	}
	if !found {
		return hold("genuine submission or applicable due date changed since pull; refresh timing evidence")
	}
	if ambiguous(g.Outcomes.Late) && g.TimingTarget != nil && *g.TimingTarget != target {
		return hold("calendar or timing inputs changed after an uncertain write; inspect Canvas before retrying")
	}
	if timingMatches(live, target) {
		g.Outcomes.Late = domain.ActionOutcome{Status: "preserved", Message: "requested lateness timing is present"}
		return save()
	}
	if ambiguous(g.Outcomes.Late) {
		g.Outcomes.Late.Status = "uncertain"
		g.Outcomes.Late.Message = "late timing completion remains uncertain; different live timing is not proof of failure"
		return save()
	}
	g.TimingTarget = &target
	g.Outcomes.Late = domain.ActionOutcome{Status: "pending"}
	if e = save(); e != nil {
		return e
	}
	fields := map[string]any{"late_policy_status": target.Status}
	if seconds > 0 {
		fields["seconds_late_override"] = seconds
	}
	var response Submission
	e = s.Client.Write(ctx, "PUT", submissionPath(doc, userID), map[string]any{"submission": fields}, &response)
	if e != nil {
		g.Outcomes.Late = failedOutcome(e)
	} else if sameAttempt(g.Attempt, attemptRef(response)) && timingMatches(response, target) {
		g.Outcomes.Late = domain.ActionOutcome{Status: "succeeded"}
	} else {
		g.Outcomes.Late = domain.ActionOutcome{Status: "uncertain", Message: "response did not confirm requested timing on intended attempt"}
	}
	return save()
}
