package workspace

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const schoolCalendarPath = "context/school-calendar.md"

// SchoolCalendar contains the date and school-day facts used by scoring and
// reporting. Dates are local calendar dates in Timezone, and period endpoints
// are inclusive.
type SchoolCalendar struct {
	Schema          int             `yaml:"schema"`
	Timezone        string          `yaml:"timezone"`
	StartDate       string          `yaml:"start_date"`
	EndDate         string          `yaml:"end_date"`
	NonSchoolDates  []string        `yaml:"non_school_dates"`
	NonSchoolRanges []DateRange     `yaml:"non_school_ranges"`
	GradingPeriods  []GradingPeriod `yaml:"grading_periods"`
}

// DateRange is an inclusive interval of non-school local calendar dates.
type DateRange struct {
	StartDate string `yaml:"start_date"`
	EndDate   string `yaml:"end_date"`
}

// GradingPeriod is an inclusive named date range in the calendar's timezone.
type GradingPeriod struct {
	Name      string `yaml:"name"`
	StartDate string `yaml:"start_date"`
	EndDate   string `yaml:"end_date"`
}

// SchoolTimeResult reports elapsed school time after the supplied deadline.
// SkippedDates lists weekend and configured closure dates touched by the
// interval, in ascending local-date order.
type SchoolTimeResult struct {
	Status       string   `json:"status"`
	SecondsLate  int64    `json:"seconds_late"`
	SkippedDates []string `json:"skipped_dates"`
}

// ReadSchoolCalendar reads the active teacher calendar. ResolveTeacherPath,
// used by ReadTeacherDocument, keeps a configured UserSync calendar authoritative.
func ReadSchoolCalendar(root string) (SchoolCalendar, error) {
	var calendar SchoolCalendar
	if _, err := ReadTeacherDocument(root, schoolCalendarPath, &calendar); err != nil {
		return SchoolCalendar{}, err
	}
	return calendar, nil
}

// Validate checks calendar dates and any configured grading periods. An empty
// period list remains valid for explicit scoring work that needs school time.
func (c SchoolCalendar) Validate() error {
	if c.Schema != 1 {
		return fmt.Errorf("school calendar schema must be 1")
	}
	zone := strings.TrimSpace(c.Timezone)
	if zone == "" || zone != c.Timezone {
		return fmt.Errorf("school calendar timezone must be a nonempty IANA timezone without surrounding whitespace")
	}
	_, err := time.LoadLocation(zone)
	if err != nil {
		return fmt.Errorf("school calendar timezone %q is invalid: %w", c.Timezone, err)
	}
	start, err := parseCalendarDate(c.StartDate)
	if err != nil {
		return fmt.Errorf("school calendar start_date: %w", err)
	}
	end, err := parseCalendarDate(c.EndDate)
	if err != nil {
		return fmt.Errorf("school calendar end_date: %w", err)
	}
	if end.Before(start) {
		return fmt.Errorf("school calendar end_date must be on or after start_date")
	}
	for i, raw := range c.NonSchoolDates {
		d, e := parseCalendarDate(raw)
		if e != nil {
			return fmt.Errorf("non_school_dates[%d]: %w", i, e)
		}
		if d.Before(start) || d.After(end) {
			return fmt.Errorf("non_school_dates[%d] %s is outside calendar coverage", i, raw)
		}
	}
	for i, r := range c.NonSchoolRanges {
		rs, e := parseCalendarDate(r.StartDate)
		if e != nil {
			return fmt.Errorf("non_school_ranges[%d].start_date: %w", i, e)
		}
		re, e := parseCalendarDate(r.EndDate)
		if e != nil {
			return fmt.Errorf("non_school_ranges[%d].end_date: %w", i, e)
		}
		if re.Before(rs) {
			return fmt.Errorf("non_school_ranges[%d] end_date must be on or after start_date", i)
		}
		if rs.Before(start) || re.After(end) {
			return fmt.Errorf("non_school_ranges[%d] is outside calendar coverage", i)
		}
	}

	var previousEnd time.Time
	for i, p := range c.GradingPeriods {
		if strings.TrimSpace(p.Name) == "" {
			return fmt.Errorf("grading_periods[%d].name is required", i)
		}
		ps, e := parseCalendarDate(p.StartDate)
		if e != nil {
			return fmt.Errorf("grading_periods[%d].start_date: %w", i, e)
		}
		pe, e := parseCalendarDate(p.EndDate)
		if e != nil {
			return fmt.Errorf("grading_periods[%d].end_date: %w", i, e)
		}
		if pe.Before(ps) {
			return fmt.Errorf("grading_periods[%d] end_date must be on or after start_date", i)
		}
		if ps.Before(start) || pe.After(end) {
			return fmt.Errorf("grading_periods[%d] is outside calendar coverage", i)
		}
		if i > 0 && !ps.After(previousEnd) {
			return fmt.Errorf("grading_periods must be in chronological order and must not overlap")
		}
		previousEnd = pe
	}
	return nil
}

// CurrentPeriod returns the unique period containing asOf's local date.
func (c SchoolCalendar) CurrentPeriod(asOf time.Time) (GradingPeriod, error) {
	if err := c.Validate(); err != nil {
		return GradingPeriod{}, err
	}
	if len(c.GradingPeriods) == 0 {
		return GradingPeriod{}, fmt.Errorf("school calendar has no grading_periods configured")
	}
	loc, _ := time.LoadLocation(c.Timezone)
	local := asOf.In(loc)
	date := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	var matches []GradingPeriod
	for _, p := range c.GradingPeriods {
		start, _ := dateInLocation(p.StartDate, loc)
		end, _ := dateInLocation(p.EndDate, loc)
		if !date.Before(start) && !date.After(end) {
			matches = append(matches, p)
		}
	}
	if len(matches) == 0 {
		return GradingPeriod{}, fmt.Errorf("date %s in %s is not covered by a grading period", date.Format("2006-01-02"), c.Timezone)
	}
	if len(matches) != 1 {
		return GradingPeriod{}, fmt.Errorf("date %s in %s is covered by multiple grading periods", date.Format("2006-01-02"), c.Timezone)
	}
	return matches[0], nil
}

// LocalStart returns the period's inclusive local-midnight start.
func (p GradingPeriod) LocalStart(timezone string) (time.Time, error) {
	start, err := parseCalendarDate(p.StartDate)
	if err != nil {
		return time.Time{}, err
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid timezone %q: %w", timezone, err)
	}
	return time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, loc), nil
}

// LocalEndExclusive returns local midnight immediately after the inclusive end.
func (p GradingPeriod) LocalEndExclusive(timezone string) (time.Time, error) {
	end, err := parseCalendarDate(p.EndDate)
	if err != nil {
		return time.Time{}, err
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid timezone %q: %w", timezone, err)
	}
	return time.Date(end.Year(), end.Month(), end.Day()+1, 0, 0, 0, 0, loc), nil
}

// CountSchoolTime intersects the deadline-to-submission interval with weekday
// local dates that are not configured closures. On-time work is resolved before
// requiring calendar coverage, so it can be processed with no calendar.
func (c SchoolCalendar) CountSchoolTime(deadline, submitted time.Time) (SchoolTimeResult, error) {
	if submitted.IsZero() || deadline.IsZero() {
		return SchoolTimeResult{}, fmt.Errorf("deadline and submitted time are required")
	}
	if !submitted.After(deadline) {
		return SchoolTimeResult{Status: "none", SkippedDates: []string{}}, nil
	}
	if err := c.Validate(); err != nil {
		return SchoolTimeResult{}, err
	}
	loc, _ := time.LoadLocation(c.Timezone)
	calendarStart, _ := dateInLocation(c.StartDate, loc)
	calendarEnd, _ := dateInLocation(c.EndDate, loc)
	firstLocal := deadline.In(loc)
	lastLocal := submitted.In(loc)
	first := time.Date(firstLocal.Year(), firstLocal.Month(), firstLocal.Day(), 0, 0, 0, 0, loc)
	last := time.Date(lastLocal.Year(), lastLocal.Month(), lastLocal.Day(), 0, 0, 0, 0, loc)
	coverageLast := last
	if last.Equal(submitted.In(loc)) {
		coverageLast = last.AddDate(0, 0, -1)
	}
	if first.Before(calendarStart) || coverageLast.After(calendarEnd) {
		return SchoolTimeResult{}, fmt.Errorf("school calendar does not cover the counted interval %s through %s", first.Format("2006-01-02"), coverageLast.Format("2006-01-02"))
	}
	closed := make(map[string]bool, len(c.NonSchoolDates))
	for _, d := range c.NonSchoolDates {
		closed[d] = true
	}
	for _, r := range c.NonSchoolRanges {
		rs, _ := parseCalendarDate(r.StartDate)
		re, _ := parseCalendarDate(r.EndDate)
		for d := rs; !d.After(re); d = d.AddDate(0, 0, 1) {
			closed[d.Format("2006-01-02")] = true
		}
	}

	var seconds int64
	var skipped []string
	for day := first; !day.After(last); day = day.AddDate(0, 0, 1) {
		dayEnd := day.AddDate(0, 0, 1)
		intervalStart := later(deadline, day)
		intervalEnd := earlier(submitted, dayEnd)
		if !intervalEnd.After(intervalStart) {
			continue
		}
		key := day.Format("2006-01-02")
		if day.Weekday() == time.Saturday || day.Weekday() == time.Sunday || closed[key] {
			skipped = append(skipped, key)
			continue
		}
		seconds += int64(intervalEnd.Sub(intervalStart).Seconds())
	}
	status := "none"
	if seconds > 0 {
		status = "late"
	}
	sort.Strings(skipped)
	return SchoolTimeResult{Status: status, SecondsLate: seconds, SkippedDates: skipped}, nil
}

func parseCalendarDate(value string) (time.Time, error) {
	d, err := time.Parse("2006-01-02", value)
	if err != nil || d.Format("2006-01-02") != value {
		return time.Time{}, fmt.Errorf("%q must be a real YYYY-MM-DD date", value)
	}
	return d, nil
}

func dateInLocation(value string, loc *time.Location) (time.Time, error) {
	d, err := parseCalendarDate(value)
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, loc), nil
}

func later(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func earlier(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}
