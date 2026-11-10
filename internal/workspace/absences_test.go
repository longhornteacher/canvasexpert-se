package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func absenceCalendar() SchoolCalendar {
	// Monday 2026-11-09 is a closure; 11/07-11/08 is a weekend.
	return SchoolCalendar{Schema: 1, Timezone: "America/Chicago", StartDate: "2026-08-19", EndDate: "2027-05-27", NonSchoolDates: []string{"2026-11-09"}}
}

func TestAbsenceStoreSavesRemovesAndRejectsBadDates(t *testing.T) {
	root := t.TempDir()
	store, err := OpenAbsences(root)
	if err != nil || len(store.Rows) != 0 {
		t.Fatalf("empty store: %+v %v", store, err)
	}
	for _, date := range []string{"2026-11-06", "2026-11-05", "2026-11-06"} {
		if _, err = store.Add("https://school.example", "s-a", date); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = store.Add("https://school.example", "s-a", "11/06/2026"); err == nil {
		t.Fatal("accepted a date that is not YYYY-MM-DD")
	}
	if _, err = store.Add("https://school.example", "s-b", "2026-11-06"); err != nil {
		t.Fatal(err)
	}
	if err = store.Save(); err != nil {
		t.Fatal(err)
	}
	again, err := OpenAbsences(root)
	if err != nil || again.Version == "" || strings.Join(again.Dates("https://school.example", "s-a"), ",") != "2026-11-05,2026-11-06" {
		t.Fatalf("reopened store: %+v %v", again, err)
	}
	if !again.Remove("https://school.example", "s-a", "2026-11-05") || again.Remove("https://school.example", "s-a", "2026-11-05") {
		t.Fatal("remove should succeed once")
	}
	if got := again.Dates("https://other.example", "s-a"); len(got) != 0 {
		t.Fatalf("absences crossed Canvas instances: %v", got)
	}
	if err = os.WriteFile(filepath.Join(root, ".cse", "absences.csv"), []byte("student_ref,instance,date\ns-a,https://school.example,tomorrow\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = OpenAbsences(root); err == nil {
		t.Fatal("accepted a malformed saved date")
	}
}

func TestAbsencesMoveDeadlineAndAreNotCountedAsLateTime(t *testing.T) {
	cal := absenceCalendar()
	loc, _ := time.LoadLocation(cal.Timezone)
	due := time.Date(2026, 11, 5, 16, 0, 0, 0, loc) // Thursday 4 p.m.

	// Absent on the due date: due the next day back, same time.
	moved, err := cal.DeadlineAfterAbsences(due, []string{"2026-11-05"})
	if err != nil || !moved.Equal(time.Date(2026, 11, 6, 16, 0, 0, 0, loc)) {
		t.Fatalf("deadline=%v err=%v", moved, err)
	}
	// Absent Thursday and Friday, then a weekend and a closure: back on Tuesday.
	moved, err = cal.DeadlineAfterAbsences(due, []string{"2026-11-05", "2026-11-06"})
	if err != nil || !moved.Equal(time.Date(2026, 11, 10, 16, 0, 0, 0, loc)) {
		t.Fatalf("deadline across weekend and closure=%v err=%v", moved, err)
	}
	// An absence before the due date changes nothing.
	if same, err := cal.DeadlineAfterAbsences(due, []string{"2026-11-04"}); err != nil || !same.Equal(due) {
		t.Fatalf("an earlier absence moved the deadline: %v %v", same, err)
	}

	// Absent the day after the due date: that day is not late time.
	submitted := time.Date(2026, 11, 10, 9, 0, 0, 0, loc) // Tuesday 9 a.m.
	plain, err := cal.CountSchoolTime(due, submitted)
	if err != nil {
		t.Fatal(err)
	}
	with, err := cal.WithAbsences([]string{"2026-11-06", "2026-11-04"}).CountSchoolTime(due, submitted)
	if err != nil {
		t.Fatal(err)
	}
	// Thursday 8h + Friday 24h + Tuesday 9h, against Thursday 8h + Tuesday 9h.
	if plain.SecondsLate != (8+24+9)*3600 || with.SecondsLate != (8+9)*3600 {
		t.Fatalf("plain=%d with absences=%d", plain.SecondsLate, with.SecondsLate)
	}
	if got := cal.AbsencesBetween([]string{"2026-11-04", "2026-11-06", "2026-11-12"}, due, submitted); len(got) != 1 || got[0] != "2026-11-06" {
		t.Fatalf("absences in the counted window: %v", got)
	}
	// Dates outside calendar coverage are ignored instead of breaking validation.
	if err := cal.WithAbsences([]string{"2025-01-01"}).Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestSchoolDaysBetweenSkipsWeekendsAndClosures(t *testing.T) {
	cal := absenceCalendar()
	got := cal.SchoolDaysBetween("2026-11-05", "2026-11-11")
	if strings.Join(got, ",") != "2026-11-05,2026-11-06,2026-11-10,2026-11-11" {
		t.Fatalf("school days: %v", got)
	}
	if cal.IsSchoolDay("2026-11-09") || cal.IsSchoolDay("2026-11-07") || cal.IsSchoolDay("2028-01-03") || !cal.IsSchoolDay("2026-11-10") {
		t.Fatal("school-day check is wrong")
	}
	if cal.WeekdayClosures() != 1 {
		t.Fatalf("closures=%d", cal.WeekdayClosures())
	}
}

func TestStudentTableGainsExtraTimeDays(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".cse", "students.csv")
	old := strings.Join(quizStudentHeader, ",") + "\ns-test,https://school.example,7,Ana,Ana,Pikachu,,,false,true,teacher,,false\n"
	if err := WriteFile(path, []byte(old), 0600); err != nil {
		t.Fatal(err)
	}
	store, err := OpenStudents(root)
	if err != nil || len(store.Rows) != 1 || store.Rows[0].ExtraTimeDays != 0 || !store.Rows[0].ExtraTime {
		t.Fatalf("older table: %+v %v", store, err)
	}
	store.Rows[0].ExtraTimeDays = 2
	if err = store.Save(); err != nil {
		t.Fatal(err)
	}
	again, err := OpenStudents(root)
	if err != nil || again.Rows[0].ExtraTimeDays != 2 || again.Rows[0].Ref != "s-test" || again.Rows[0].Alias != "Pikachu" {
		t.Fatalf("saved days: %+v %v", again, err)
	}
	again.Rows[0].ExtraTimeDays = MaxExtraTimeDays + 1
	if err = again.Save(); err == nil {
		t.Fatal("saved an out-of-range number of extra days")
	}
}
