package workspace

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const absencesFile = ".cse/absences.csv"

var absenceHeader = []string{"student_ref", "instance", "date"}

// Absence is one school day a student was out. It is private like the student
// table and follows the same rules authority.
type Absence struct {
	Ref, Instance, Date string
}

// AbsenceStore is the teacher-entered absence list. Version identifies the
// saved file so a long-open page can tell it changed.
type AbsenceStore struct {
	Rows    []Absence
	Version string
	path    string
}

func OpenAbsences(root string) (*AbsenceStore, error) {
	p, err := ResolveRulesPath(root, filepath.FromSlash(absencesFile))
	if err != nil {
		return nil, err
	}
	s := &AbsenceStore{path: p}
	b, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	s.Version = studentsVersion(b)
	rows, err := csv.NewReader(bytes.NewReader(b)).ReadAll()
	if err != nil || len(rows) == 0 || strings.Join(rows[0], "\x00") != strings.Join(absenceHeader, "\x00") {
		return nil, fmt.Errorf("private absence list: unsupported file")
	}
	for i, row := range rows[1:] {
		if len(row) != 3 || row[0] == "" || row[1] == "" {
			return nil, fmt.Errorf("private absence list row %d: invalid columns", i+2)
		}
		if _, err := parseCalendarDate(row[2]); err != nil {
			return nil, fmt.Errorf("private absence list row %d: %w", i+2, err)
		}
		s.Rows = append(s.Rows, Absence{Ref: row[0], Instance: row[1], Date: row[2]})
	}
	return s, nil
}

func (s *AbsenceStore) Save() error {
	sort.SliceStable(s.Rows, func(i, j int) bool {
		a, b := s.Rows[i], s.Rows[j]
		if a.Date != b.Date {
			return a.Date < b.Date
		}
		return a.Ref < b.Ref
	})
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	_ = w.Write(absenceHeader)
	for _, v := range s.Rows {
		_ = w.Write([]string{v.Ref, v.Instance, v.Date})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return err
	}
	if err := WriteFile(s.path, b.Bytes(), 0600); err != nil {
		return err
	}
	s.Version = studentsVersion(b.Bytes())
	return nil
}

// Add records one absent date; it reports false when it was already there.
func (s *AbsenceStore) Add(instance, ref, date string) (bool, error) {
	if instance == "" || ref == "" {
		return false, fmt.Errorf("absence needs a student")
	}
	if _, err := parseCalendarDate(date); err != nil {
		return false, err
	}
	for _, v := range s.Rows {
		if v.Instance == instance && v.Ref == ref && v.Date == date {
			return false, nil
		}
	}
	s.Rows = append(s.Rows, Absence{Ref: ref, Instance: instance, Date: date})
	return true, nil
}

func (s *AbsenceStore) Remove(instance, ref, date string) bool {
	for i, v := range s.Rows {
		if v.Instance == instance && v.Ref == ref && v.Date == date {
			s.Rows = append(s.Rows[:i], s.Rows[i+1:]...)
			return true
		}
	}
	return false
}

// Dates returns the student's absent dates in ascending order.
func (s *AbsenceStore) Dates(instance, ref string) []string {
	var out []string
	for _, v := range s.Rows {
		if v.Instance == instance && v.Ref == ref {
			out = append(out, v.Date)
		}
	}
	sort.Strings(out)
	return out
}

// WithAbsences returns a copy of the calendar in which the student's absent
// dates inside calendar coverage are also non-school dates. Counting starts
// after a deadline, so an absence before the due date changes nothing.
func (c SchoolCalendar) WithAbsences(dates []string) SchoolCalendar {
	if len(dates) == 0 {
		return c
	}
	out := c
	out.NonSchoolDates = append([]string(nil), c.NonSchoolDates...)
	for _, d := range dates {
		if d >= c.StartDate && d <= c.EndDate {
			out.NonSchoolDates = append(out.NonSchoolDates, d)
		}
	}
	return out
}

// DeadlineAfterAbsences moves a deadline that falls on a day the student was
// absent to the same local time on their next present school day.
func (c SchoolCalendar) DeadlineAfterAbsences(deadline time.Time, absent []string) (time.Time, error) {
	if len(absent) == 0 {
		return deadline, nil
	}
	loc, err := time.LoadLocation(c.Timezone)
	if err != nil {
		return deadline, fmt.Errorf("school calendar timezone %q is invalid: %w", c.Timezone, err)
	}
	day := deadline.In(loc).Format("2006-01-02")
	for _, d := range absent {
		if d == day {
			return c.WithAbsences(absent).AddSchoolDays(deadline, 1)
		}
	}
	return deadline, nil
}

// IsSchoolDay reports whether a YYYY-MM-DD date is a weekday inside calendar
// coverage that is not a closure. The calendar must be valid.
func (c SchoolCalendar) IsSchoolDay(date string) bool {
	d, err := parseCalendarDate(date)
	if err != nil || date < c.StartDate || date > c.EndDate {
		return false
	}
	return d.Weekday() != time.Saturday && d.Weekday() != time.Sunday && !c.closedDates()[date]
}

// SchoolDaysBetween lists the school days from one date through another, inclusive.
func (c SchoolCalendar) SchoolDaysBetween(from, to string) []string {
	start, err1 := parseCalendarDate(from)
	end, err2 := parseCalendarDate(to)
	if err1 != nil || err2 != nil {
		return nil
	}
	var out []string
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		if key := d.Format("2006-01-02"); c.IsSchoolDay(key) {
			out = append(out, key)
		}
	}
	return out
}

// WeekdayClosures counts the configured closures that fall on a weekday.
func (c SchoolCalendar) WeekdayClosures() int {
	n := 0
	for date := range c.closedDates() {
		if d, err := parseCalendarDate(date); err == nil && d.Weekday() != time.Saturday && d.Weekday() != time.Sunday {
			n++
		}
	}
	return n
}

// AbsencesBetween returns the absent dates from the first local date through
// the last, inclusive: the ones that can affect a deadline-to-submission count.
func (c SchoolCalendar) AbsencesBetween(absent []string, first, last time.Time) []string {
	loc, err := time.LoadLocation(c.Timezone)
	if err != nil {
		return nil
	}
	from, to := first.In(loc).Format("2006-01-02"), last.In(loc).Format("2006-01-02")
	var out []string
	for _, d := range absent {
		if d >= from && d <= to {
			out = append(out, d)
		}
	}
	return out
}
