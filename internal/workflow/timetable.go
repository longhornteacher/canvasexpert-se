package workflow

import (
	"context"
	"errors"
	"fmt"
	"html"
	"math/big"
	"os"
	"regexp"
	"strings"
	"time"

	"canvasexpertse/internal/canvas"
	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

// The grading timetable is a collapsed block at the end of a new assignment
// that tells students and families what happens after the due date. Every date
// counts school days with the same calendar rule as late timing.

const timetableMarker = `data-cse="grading-timetable"`

var timetableBlock = regexp.MustCompile(`(?s)<details[^>]*data-cse="grading-timetable"[^>]*>.*?</details>`)

// CanvasLatePolicy is the course late policy Canvas applies. Known is false when
// it could not be read; a course with no policy reads as Known with no deduction.
type CanvasLatePolicy struct {
	Known            bool
	DeductionEnabled bool    `json:"late_submission_deduction_enabled"`
	Deduction        float64 `json:"late_submission_deduction"`
	Interval         string  `json:"late_submission_interval"`
	MinimumEnabled   bool    `json:"late_submission_minimum_percent_enabled"`
	Minimum          float64 `json:"late_submission_minimum_percent"`
}

func (s *Service) courseLatePolicy(ctx context.Context, courseID int64) CanvasLatePolicy {
	var out struct {
		LatePolicy *CanvasLatePolicy `json:"late_policy"`
	}
	err := s.Client.Get(ctx, fmt.Sprintf("/api/v1/courses/%d/late_policy", courseID), &out)
	var ce *canvas.Error
	if errors.As(err, &ce) && ce.Kind == "not_found" {
		return CanvasLatePolicy{Known: true}
	}
	if err != nil || out.LatePolicy == nil {
		return CanvasLatePolicy{}
	}
	p := *out.LatePolicy
	p.Known = true
	return p
}

// LoadLateWorkPolicy reads late_work from the active context/grading.md. A
// missing file or block returns nil: no timetable is configured.
func LoadLateWorkPolicy(root string) (*domain.LateWorkPolicy, error) {
	var config domain.GradingConfig
	if _, err := workspace.ReadTeacherDocument(root, "context/grading.md", &config); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("context/grading.md: %w", err)
	}
	return config.LateWork, nil
}

// CourseLateWorkPolicy is the late_work that applies to one course: the block
// in context/grading.md with the course's own lines from context/courses.md
// taking their place. courseID 0 matches by nickname.
func CourseLateWorkPolicy(root string, courseID int64, nickname string) (*domain.LateWorkPolicy, error) {
	policy, err := LoadLateWorkPolicy(root)
	if err != nil {
		return nil, err
	}
	var config domain.CourseConfig
	if _, e := workspace.ReadTeacherDocument(root, "context/courses.md", &config); e != nil {
		return policy, nil
	}
	for _, c := range config.Courses {
		if (courseID != 0 && c.ID == courseID) || (courseID == 0 && nickname != "" && strings.EqualFold(c.Nickname, nickname)) {
			return policy.Merged(c.LateWork), nil
		}
	}
	return policy, nil
}

// lateWorkFor is the same policy for a course the Service already has loaded.
func (s *Service) lateWorkFor(course domain.Course) (*domain.LateWorkPolicy, error) {
	policy, err := LoadLateWorkPolicy(s.Root)
	if err != nil {
		return nil, err
	}
	return policy.Merged(course.LateWork), nil
}

// TimetableRow is one line; Extra is the date for students with the teacher's
// general extra-time allowance, empty when none applies.
type TimetableRow struct {
	Label, Date, Extra, Detail string
}

type Timetable struct {
	Rows []TimetableRow
	// DefaultLock is the lock date the Engine proposes when the draft has none.
	// With a late-work limit it is the regular cutoff (students with more time
	// get personal Canvas dates); otherwise the extra-time close date, so no
	// flagged student is locked out early.
	DefaultLock *time.Time
	// Notes explain rows left out (shown to the teacher, never to students).
	Notes []string
}

type timetableInput struct {
	Due      time.Time
	LockAt   *time.Time // the assignment's lock date, when it has one
	Points   domain.Decimal
	Calendar workspace.SchoolCalendar
	Policy   domain.LateWorkPolicy
	Late     *CanvasLatePolicy // nil: not read yet (local preview)
	// ProposeLock: with no lock date, show the close row for the lock date the
	// Engine will set (create). Otherwise an unlocked assignment has no close row.
	ProposeLock bool
}

func schoolDate(t time.Time, loc *time.Location) string {
	return t.In(loc).Format("Mon 1/02")
}

func sameLocalDay(a, b time.Time, loc *time.Location) bool {
	return a.In(loc).Format("2006-01-02") == b.In(loc).Format("2006-01-02")
}

func buildTimetable(in timetableInput) (Timetable, error) {
	var t Timetable
	cal := in.Calendar
	if err := cal.Validate(); err != nil {
		return t, fmt.Errorf("school calendar: %w", err)
	}
	loc, _ := time.LoadLocation(cal.Timezone)
	add := func(base time.Time, n int) (time.Time, error) {
		if n == 0 {
			return base, nil
		}
		return cal.AddSchoolDays(base, n)
	}
	extra := in.Policy.ExtraTimeSchoolDays
	pair := func(label string, n int, detail string) (time.Time, time.Time, error) {
		regular, err := add(in.Due, n)
		if err != nil {
			return regular, regular, err
		}
		row := TimetableRow{Label: label, Date: schoolDate(regular, loc), Detail: detail}
		late := regular
		if extra > 0 {
			extended, err := add(in.Due, extra)
			if err == nil {
				late, err = add(extended, n)
			}
			if err != nil {
				return regular, regular, err
			}
			row.Extra = schoolDate(late, loc)
		}
		t.Rows = append(t.Rows, row)
		return regular, late, nil
	}
	if _, _, err := pair("Due", 0, ""); err != nil {
		return t, err
	}
	cutoff := in.Policy.LateCutoffSchoolDays
	if cutoff != nil && *cutoff == 0 {
		// No late work at all: there is no penalty to describe.
		t.Rows = append(t.Rows, TimetableRow{Label: "Late work", Detail: "not accepted after the due date"})
	} else if penalty, ok := penaltyText(in.Late, in.Points); ok {
		if in.Late != nil && in.Late.Interval == "hour" {
			t.Rows = append(t.Rows, TimetableRow{Label: "Late penalty", Detail: penalty})
		} else if _, _, err := pair("Late penalty starts", 1, penalty); err != nil {
			return t, err
		}
	} else if in.Late != nil && !in.Late.Known {
		t.Notes = append(t.Notes, "Canvas's course late policy could not be read, so the late-penalty line was left out")
	}
	if n := in.Policy.CorrectionSchoolDays; n > 0 {
		if _, _, err := pair("Last day for corrections", n, ""); err != nil {
			return t, err
		}
	}
	if cutoff != nil {
		// The school's limit closes the assignment for everyone on the regular
		// date; a student with more time gets a personal Canvas date instead.
		lock := in.Due
		start := len(t.Rows)
		if *cutoff > 0 {
			regular, _, err := pair("Late work accepted through", *cutoff, "later work is not accepted")
			if err != nil {
				return t, err
			}
			lock = regular
		}
		switch {
		case in.LockAt == nil:
			t.DefaultLock = &lock
			if !in.ProposeLock && *cutoff > 0 {
				t.Rows = t.Rows[:start]
			}
		case !sameLocalDay(*in.LockAt, lock, loc):
			if *cutoff > 0 {
				t.Rows = t.Rows[:start]
			}
			t.Rows = append(t.Rows, TimetableRow{Label: "Assignment closes", Date: schoolDate(*in.LockAt, loc)})
		}
	} else if n := in.Policy.CloseAfterSchoolDays; n > 0 {
		start := len(t.Rows)
		_, late, err := pair("Assignment closes", n, "")
		if err != nil {
			return t, err
		}
		switch {
		case in.LockAt == nil:
			t.DefaultLock = &late
			if !in.ProposeLock {
				// No lock date in Canvas: nothing actually closes.
				t.Rows = t.Rows[:start]
			}
		case !sameLocalDay(*in.LockAt, late, loc):
			// A teacher-chosen lock date wins; show it as it is.
			t.Rows[start] = TimetableRow{Label: "Assignment closes", Date: schoolDate(*in.LockAt, loc)}
		}
	} else if in.LockAt != nil {
		t.Rows = append(t.Rows, TimetableRow{Label: "Assignment closes", Date: schoolDate(*in.LockAt, loc)})
	}
	return t, nil
}

// penaltyText describes Canvas's late deduction. Shown as points on a
// 100-point assignment, otherwise as a percent. In a local preview (late nil)
// the amount is unknown until push reads Canvas.
func penaltyText(late *CanvasLatePolicy, points domain.Decimal) (string, bool) {
	if late == nil {
		return "amount from the course's Canvas late policy, read at push", true
	}
	if !late.Known || !late.DeductionEnabled || late.Deduction <= 0 {
		return "", false
	}
	hundred := points.IsSet() && points.Rat().Cmp(big.NewRat(100, 1)) == 0
	amount := func(v float64) string {
		n := strings.TrimSuffix(strings.TrimRight(fmt.Sprintf("%.2f", v), "0"), ".")
		if hundred {
			return n + " points"
		}
		return n + "%"
	}
	unit := "school day"
	if late.Interval == "hour" {
		unit = "hour"
	}
	text := fmt.Sprintf("−%s per %s late", amount(late.Deduction), unit)
	if late.MinimumEnabled && late.Minimum > 0 {
		text += fmt.Sprintf("; lowest late grade %s", amount(late.Minimum))
	}
	return text, true
}

func (t Timetable) HTML() string {
	var b strings.Builder
	b.WriteString(`<details ` + timetableMarker + `><summary><strong>Grading timetable</strong> (late work and corrections)</summary><ul>`)
	for _, r := range t.Rows {
		b.WriteString("<li><strong>" + html.EscapeString(r.Label) + ":</strong> ")
		parts := []string{}
		if r.Date != "" {
			d := html.EscapeString(r.Date)
			if r.Extra != "" {
				d += " (extra time: " + html.EscapeString(r.Extra) + ")"
			}
			parts = append(parts, d)
		}
		if r.Detail != "" {
			parts = append(parts, html.EscapeString(r.Detail))
		}
		b.WriteString(strings.Join(parts, " — ") + "</li>")
	}
	b.WriteString("</ul><p>Dates count school days only. Corrections are available as soon as you finish; check your feedback for rationales and exemplars.</p></details>")
	return b.String()
}

// Lines is the plain-text form for the teacher check-in and validate preview.
func (t Timetable) Lines() []string {
	var lines []string
	for _, r := range t.Rows {
		line := r.Label + ":"
		if r.Date != "" {
			line += " " + r.Date
			if r.Extra != "" {
				line += " (extra time: " + r.Extra + ")"
			}
		}
		if r.Detail != "" {
			if r.Date != "" {
				line += " —"
			}
			line += " " + r.Detail
		}
		lines = append(lines, line)
	}
	return append(lines, t.Notes...)
}

// withTimetable puts the block at the end of html, replacing an earlier block
// (for example one carried over by copy-assignment).
func withTimetable(body, block string) string {
	body = strings.TrimRight(timetableBlock.ReplaceAllString(body, ""), " \t\r\n")
	if block == "" {
		return body
	}
	return body + "\n" + block
}

func hasTimetable(body string) bool { return timetableBlock.MatchString(body) }

func parseOptionalTime(v *string) (*time.Time, error) {
	if v == nil || *v == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, *v)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// planTimetable builds the timetable for an assignment's dates. A nil result
// with a note means no timetable (not configured, no due date, or a calendar
// gap); the assignment itself is unaffected. late nil builds a local preview.
func planTimetable(root string, courseID int64, nickname string, dueAt, lockAt *string, points domain.Decimal, late *CanvasLatePolicy, proposeLock bool) (*Timetable, string) {
	policy, err := CourseLateWorkPolicy(root, courseID, nickname)
	if err != nil {
		return nil, "grading timetable left out: " + err.Error()
	}
	if policy == nil {
		return nil, "grading timetable left out: late_work is not set in context/grading.md"
	}
	due, err := parseOptionalTime(dueAt)
	if err != nil || due == nil {
		return nil, "grading timetable left out: the assignment has no due date"
	}
	lock, err := parseOptionalTime(lockAt)
	if err != nil {
		return nil, "grading timetable left out: lock_at is not an RFC3339 time"
	}
	cal, err := workspace.ReadSchoolCalendar(root)
	if err != nil {
		return nil, "grading timetable left out: context/school-calendar.md could not be read (" + err.Error() + ")"
	}
	t, err := buildTimetable(timetableInput{Due: *due, LockAt: lock, Points: points, Calendar: cal, Policy: *policy, Late: late, ProposeLock: proposeLock})
	if err != nil {
		return nil, "grading timetable left out: " + err.Error()
	}
	return &t, ""
}

// PreviewTimetable is the local, no-Canvas timetable for a create draft, used
// by validate so the agent can tell the teacher the dates at check-in.
func PreviewTimetable(root string, d domain.AssignmentDocument) []string {
	if d.Operation != "create" || (d.GradingTimetable != nil && !*d.GradingTimetable) {
		return nil
	}
	t, note := planTimetable(root, d.CourseID, d.Course, d.DueAt, d.LockAt, d.Points, nil, true)
	if t == nil {
		return []string{note}
	}
	lines := t.Lines()
	if t.DefaultLock != nil {
		lines = append(lines, "lock_at will be set to "+t.DefaultLock.Format(time.RFC3339)+" (the timetable's close date) because the draft has none")
	}
	return lines
}

// createTimetable fixes the timetable for a create on its first attempt and
// sets the Engine-chosen lock date on the document.
func (s *Service) createTimetable(ctx context.Context, doc *domain.AssignmentDocument, courseID int64) *domain.TimetableRecord {
	if doc.GradingTimetable != nil && !*doc.GradingTimetable {
		return &domain.TimetableRecord{Note: "grading timetable left out: the draft opts out"}
	}
	// Settle everything local first so an unconfigured create makes no extra read.
	if t, note := planTimetable(s.Root, courseID, "", doc.DueAt, doc.LockAt, doc.Points, nil, true); t == nil {
		return &domain.TimetableRecord{Note: note}
	}
	late := s.courseLatePolicy(ctx, courseID)
	t, note := planTimetable(s.Root, courseID, "", doc.DueAt, doc.LockAt, doc.Points, &late, true)
	if t == nil {
		return &domain.TimetableRecord{Note: note}
	}
	rec := &domain.TimetableRecord{HTML: t.HTML(), Note: strings.Join(t.Notes, "; ")}
	if t.DefaultLock != nil {
		rec.LockAt = t.DefaultLock.Format(time.RFC3339)
		doc.LockAt = &rec.LockAt
	}
	return rec
}

// refreshTimetable regenerates an existing timetable when an update changes the
// due or lock date. A lock date the Engine chose for the old due date moves with
// it unless the update sets its own lock date. It returns the new body and a
// teacher-facing note; an unchanged body means nothing to refresh.
func (s *Service) refreshTimetable(ctx context.Context, d *domain.AssignmentDocument, body string) (string, string) {
	if d.Observed == nil || !hasTimetable(body) {
		return body, ""
	}
	if sameDate(d.Observed.DueAt, d.DueAt) && sameDate(d.Observed.LockAt, d.LockAt) {
		return body, ""
	}
	late := s.courseLatePolicy(ctx, d.CourseID)
	if sameDate(d.Observed.LockAt, d.LockAt) && !sameDate(d.Observed.DueAt, d.DueAt) {
		old, _ := planTimetable(s.Root, d.CourseID, "", d.Observed.DueAt, nil, d.Points, &late, false)
		lock, _ := parseOptionalTime(d.Observed.LockAt)
		if old != nil && old.DefaultLock != nil && lock != nil && old.DefaultLock.Equal(*lock) {
			moved, _ := planTimetable(s.Root, d.CourseID, "", d.DueAt, nil, d.Points, &late, false)
			if moved != nil && moved.DefaultLock != nil {
				v := moved.DefaultLock.Format(time.RFC3339)
				d.LockAt = &v
			}
		}
	}
	t, note := planTimetable(s.Root, d.CourseID, "", d.DueAt, d.LockAt, d.Points, &late, false)
	if t == nil {
		return body, note + "; the existing timetable was left as it is and may now be out of date"
	}
	return withTimetable(body, t.HTML()), "grading timetable updated for the new dates"
}
