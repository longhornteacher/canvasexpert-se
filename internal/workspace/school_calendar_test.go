package workspace

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSchoolCalendarCurrentPeriodUsesInclusiveLocalDates(t *testing.T) {
	c := SchoolCalendar{
		Schema: 1, Timezone: "America/Chicago", StartDate: "2026-08-19", EndDate: "2027-05-27",
		GradingPeriods: []GradingPeriod{
			{Name: "Term 1", StartDate: "2026-08-19", EndDate: "2026-10-08"},
			{Name: "Term 2", StartDate: "2026-10-13", EndDate: "2026-12-18"},
			{Name: "Term 3", StartDate: "2027-01-06", EndDate: "2027-03-19"},
			{Name: "Term 4", StartDate: "2027-03-22", EndDate: "2027-05-27"},
		},
	}
	for _, tc := range []struct{ at, want string }{
		{"2026-10-08T23:59:00-05:00", "Term 1"},
		{"2026-10-13T00:00:00-05:00", "Term 2"},
		{"2027-01-06T00:00:00-06:00", "Term 3"},
		{"2027-03-22T00:00:00-05:00", "Term 4"},
	} {
		at, err := time.Parse(time.RFC3339, tc.at)
		if err != nil {
			t.Fatal(err)
		}
		got, err := c.CurrentPeriod(at)
		if err != nil || got.Name != tc.want {
			t.Fatalf("CurrentPeriod(%s) = %q, %v; want %q", tc.at, got.Name, err, tc.want)
		}
	}
	for _, at := range []string{"2026-10-12T12:00:00-05:00", "2027-01-05T12:00:00-06:00", "2027-03-20T12:00:00-05:00"} {
		instant, _ := time.Parse(time.RFC3339, at)
		if _, err := c.CurrentPeriod(instant); err == nil {
			t.Fatalf("CurrentPeriod(%s) should reject a calendar gap", at)
		}
	}
	// An instant still on the prior UTC date can already be the next local date.
	localNextDay := time.Date(2026, 10, 13, 5, 30, 0, 0, time.UTC)
	got, err := c.CurrentPeriod(localNextDay)
	if err != nil || got.Name != "Term 2" {
		t.Fatalf("local date should determine period, got %q, %v", got.Name, err)
	}
}

func TestSchoolCalendarValidationAllowsAbsentPeriodsAndRejectsBadPeriods(t *testing.T) {
	c := SchoolCalendar{Schema: 1, Timezone: "America/Chicago", StartDate: "2026-08-19", EndDate: "2027-05-27"}
	if err := c.Validate(); err != nil {
		t.Fatalf("empty period list should be valid: %v", err)
	}
	c.Timezone = ""
	if err := c.Validate(); err == nil {
		t.Fatal("empty timezone must be rejected")
	}
	c.Timezone = "America/Chicago"
	if _, err := c.CurrentPeriod(time.Now()); err == nil {
		t.Fatal("period resolution should require a configured containing period")
	}
	c.GradingPeriods = []GradingPeriod{
		{Name: "First", StartDate: "2026-08-19", EndDate: "2026-10-13"},
		{Name: "Second", StartDate: "2026-10-13", EndDate: "2026-12-18"},
	}
	if err := c.Validate(); err == nil {
		t.Fatal("overlapping periods should be rejected")
	}
	c.GradingPeriods = []GradingPeriod{{Name: "Bad", StartDate: "2026-02-30", EndDate: "2026-12-18"}}
	if err := c.Validate(); err == nil {
		t.Fatal("malformed period dates should be rejected")
	}
}

func TestCountSchoolTimeUsesLocalMidnightsAndInclusiveClosures(t *testing.T) {
	c := SchoolCalendar{
		Schema: 1, Timezone: "America/Chicago", StartDate: "2026-10-01", EndDate: "2026-11-30",
		NonSchoolDates:  []string{"2026-10-09", "2026-10-12"},
		NonSchoolRanges: []DateRange{{StartDate: "2026-11-23", EndDate: "2026-11-27"}},
	}
	deadline := time.Date(2026, 10, 16, 18, 0, 0, 0, time.FixedZone("CDT", -5*60*60))
	submitted := time.Date(2026, 10, 20, 9, 0, 0, 0, time.FixedZone("CDT", -5*60*60))
	got, err := c.CountSchoolTime(deadline, submitted)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "late" || got.SecondsLate != 140400 || len(got.SkippedDates) != 2 || got.SkippedDates[0] != "2026-10-17" || got.SkippedDates[1] != "2026-10-18" {
		t.Fatalf("unexpected weekend result: %+v", got)
	}

	deadline = time.Date(2026, 10, 8, 18, 0, 0, 0, time.FixedZone("CDT", -5*60*60))
	submitted = time.Date(2026, 10, 13, 9, 0, 0, 0, time.FixedZone("CDT", -5*60*60))
	got, err = c.CountSchoolTime(deadline, submitted)
	if err != nil {
		t.Fatal(err)
	}
	if got.SecondsLate != 54000 || len(got.SkippedDates) != 4 {
		t.Fatalf("expected Thu 6h + Tue 9h with closure/weekend skips: %+v", got)
	}

	deadline = time.Date(2026, 11, 20, 18, 0, 0, 0, time.FixedZone("CST", -6*60*60))
	submitted = time.Date(2026, 11, 30, 9, 0, 0, 0, time.FixedZone("CST", -6*60*60))
	got, err = c.CountSchoolTime(deadline, submitted)
	if err != nil {
		t.Fatal(err)
	}
	if got.SecondsLate != 54000 || len(got.SkippedDates) != 9 {
		t.Fatalf("inclusive Thanksgiving closure range should exclude five weekdays and both weekend days: %+v", got)
	}

	dst := SchoolCalendar{Schema: 1, Timezone: "America/Chicago", StartDate: "2026-03-06", EndDate: "2026-03-10"}
	deadline = time.Date(2026, 3, 6, 18, 0, 0, 0, time.FixedZone("CST", -6*60*60))
	submitted = time.Date(2026, 3, 9, 9, 0, 0, 0, time.FixedZone("CDT", -5*60*60))
	got, err = dst.CountSchoolTime(deadline, submitted)
	if err != nil || got.SecondsLate != 54000 || len(got.SkippedDates) != 2 {
		t.Fatalf("DST weekend should preserve 6 Friday and 9 Monday local hours: %+v, %v", got, err)
	}
}

func TestCountSchoolTimeOnTimeDoesNotRequireCalendar(t *testing.T) {
	deadline := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	got, err := (SchoolCalendar{}).CountSchoolTime(deadline, deadline)
	if err != nil || got.Status != "none" || got.SecondsLate != 0 {
		t.Fatalf("on-time calculation unexpectedly needed calendar: %+v, %v", got, err)
	}
}

func TestCountSchoolTimeAllowsEndAtMidnightAfterCalendarCoverage(t *testing.T) {
	c := SchoolCalendar{Schema: 1, Timezone: "America/Chicago", StartDate: "2026-10-16", EndDate: "2026-10-16"}
	deadline := time.Date(2026, 10, 16, 18, 0, 0, 0, time.FixedZone("CDT", -5*60*60))
	submitted := time.Date(2026, 10, 17, 0, 0, 0, 0, time.FixedZone("CDT", -5*60*60))
	got, err := c.CountSchoolTime(deadline, submitted)
	if err != nil || got.SecondsLate != 21600 {
		t.Fatalf("midnight endpoint should count only covered Friday: %+v, %v", got, err)
	}
}

func TestReadSchoolCalendarUsesActiveUserSync(t *testing.T) {
	install, shared := makeRulesInstall(t), t.TempDir()
	local := calendarFixture("Local", "2026-08-19", "2027-05-27")
	sharedCalendar := calendarFixture("UserSync", "2026-08-19", "2027-05-27")
	if err := WriteTeacherDocument(install, schoolCalendarPath, local, "local calendar body"); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(shared, "context"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := WriteTeacherDocument(shared, schoolCalendarPath, sharedCalendar, "shared calendar body"); err != nil {
		t.Fatal(err)
	}
	if _, err := SetRulesLocation(install, shared, nil, ""); err != nil {
		t.Fatal(err)
	}
	got, err := ReadSchoolCalendar(install)
	if err != nil || len(got.GradingPeriods) != 1 || got.GradingPeriods[0].Name != "UserSync" {
		t.Fatalf("active UserSync calendar was not read: %+v, %v", got, err)
	}
}

func calendarFixture(name, start, end string) SchoolCalendar {
	return SchoolCalendar{
		Schema: 1, Timezone: "America/Chicago", StartDate: start, EndDate: end,
		GradingPeriods: []GradingPeriod{{Name: name, StartDate: start, EndDate: end}},
	}
}
