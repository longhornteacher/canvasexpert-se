package localui

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

func absencesHub(t *testing.T) (*Hub, string, map[int64]workspace.Student) {
	t.Helper()
	h, root := studentsHub(t)
	config := domain.CourseConfig{Schema: 1, Instance: "https://school.example", Aliases: true, Courses: []domain.Course{{ID: 11, Nickname: "CS8", Current: true}}}
	if err := workspace.WriteDocument(filepath.Join(root, "context", "courses.md"), config, ""); err != nil {
		t.Fatal(err)
	}
	if w := hubGet(h, "/students?course=11"); w.Code != 200 || !strings.Contains(w.Body.String(), "Ana Real") {
		t.Fatalf("roster load: %d", w.Code)
	}
	// Monday 2026-11-09 is a closure.
	calendar := "---\nschema: 1\ntimezone: America/Chicago\nstart_date: \"2026-08-19\"\nend_date: \"2027-05-27\"\nnon_school_dates:\n  - \"2026-11-09\"\n---\n"
	if err := os.WriteFile(filepath.Join(root, "context", "school-calendar.md"), []byte(calendar), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := workspace.OpenStudents(root)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[int64]workspace.Student{}
	for _, v := range store.Rows {
		byID[v.UserID] = v
	}
	return h, root, byID
}

func absenceVersion(t *testing.T, root string) string {
	t.Helper()
	store, err := workspace.OpenAbsences(root)
	if err != nil {
		t.Fatal(err)
	}
	return store.Version
}

func TestAbsencesPageSavesClearsRangesAndRemoves(t *testing.T) {
	h, root, byID := absencesHub(t)
	ana, bo := byID[1], byID[2]
	page := hubGet(h, "/absences?course=11&date=2026-11-06").Body.String()
	for _, want := range []string{"Ana Real", "Bo Real", "Out on Fri 11/6/2026", "1 weekday closures", `href="/absences?course=11&amp;date=2026-11-06" aria-current="page"`} {
		if !strings.Contains(page, want) {
			t.Fatalf("absences page missing %q", want)
		}
	}
	if strings.Contains(page, "Dropped Kid") {
		t.Fatal("inactive student listed")
	}
	post := func(form url.Values) (int, string) {
		form.Set("request_secret", "nonce")
		form.Set("course", "11")
		form.Set("absences_version", absenceVersion(t, root))
		w := hubPost(h, "/absences", form, h.origin)
		return w.Code, hubGet(h, w.Header().Get("Location")).Body.String()
	}
	dates := func(v workspace.Student) string {
		store, err := workspace.OpenAbsences(root)
		if err != nil {
			t.Fatal(err)
		}
		return strings.Join(store.Dates(v.Instance, v.Ref), ",")
	}
	// One day: ticked students are saved.
	if code, body := post(url.Values{"date": {"2026-11-06"}, "absent_" + ana.Ref: {"on"}}); code != 303 || !strings.Contains(body, "Saved: 1 added, 0 cleared.") || dates(ana) != "2026-11-06" {
		t.Fatalf("single day: %d %s", code, dates(ana))
	}
	// The same day again with Ana unticked and Bo ticked clears Ana.
	if _, body := post(url.Values{"date": {"2026-11-06"}, "absent_" + bo.Ref: {"on"}}); !strings.Contains(body, "Saved: 1 added, 1 cleared.") || dates(ana) != "" || dates(bo) != "2026-11-06" {
		t.Fatalf("clear: ana=%q bo=%q", dates(ana), dates(bo))
	}
	// A range adds every school day and skips the weekend and the closure; nothing is cleared.
	if _, body := post(url.Values{"date": {"2026-11-05"}, "through": {"2026-11-11"}, "absent_" + ana.Ref: {"on"}}); !strings.Contains(body, "Saved: 4 added, 0 cleared.") || dates(ana) != "2026-11-05,2026-11-06,2026-11-10,2026-11-11" || dates(bo) != "2026-11-06" {
		t.Fatalf("range: ana=%q bo=%q", dates(ana), dates(bo))
	}
	// A day off saves nothing and says why.
	if _, body := post(url.Values{"date": {"2026-11-09"}, "absent_" + bo.Ref: {"on"}}); !strings.Contains(body, "not a school day") || dates(bo) != "2026-11-06" {
		t.Fatalf("closure day was saved: %q", dates(bo))
	}
	// A form opened before another save is turned away.
	stale := url.Values{"request_secret": {"nonce"}, "course": {"11"}, "absences_version": {"old"}, "date": {"2026-11-12"}, "absent_" + bo.Ref: {"on"}}
	if w := hubPost(h, "/absences", stale, h.origin); w.Code != 303 || dates(bo) != "2026-11-06" {
		t.Fatal("stale form changed the absence list")
	}
	if body := hubGet(h, "/absences?course=11").Body.String(); !strings.Contains(body, "nothing was saved") {
		t.Fatal("stale save was not explained")
	}
	remove := url.Values{"request_secret": {"nonce"}, "course": {"11"}, "view": {"2026-11-06"}, "ref": {bo.Ref}, "date": {"2026-11-06"}}
	if w := hubPost(h, "/absences/remove", remove, h.origin); w.Code != 303 || w.Header().Get("Location") != "/absences?course=11&date=2026-11-06" || dates(bo) != "" {
		t.Fatalf("remove: %d %q", w.Code, dates(bo))
	}
	if w := hubPost(h, "/absences", url.Values{"request_secret": {"wrong"}, "course": {"11"}}, h.origin); w.Code != 403 {
		t.Fatalf("wrong secret accepted: %d", w.Code)
	}
	if b, err := os.ReadFile(filepath.Join(root, ".cse", "absences.csv")); err != nil || strings.Contains(string(b), "Ana Real") {
		t.Fatalf("absence file missing or holds a real name: %v", err)
	}
}

func TestStudentsPageSavesExtraDays(t *testing.T) {
	h, root, byID := absencesHub(t)
	ana, bo := byID[1], byID[2]
	save := func(anaDays string, ticked bool) int {
		form := url.Values{"request_secret": {"nonce"}, "course": {"11"}, "students_version": {studentsVersionNow(t, root)},
			"alias_" + ana.Ref: {ana.Alias}, "alias_" + bo.Ref: {bo.Alias}, "extra_days_" + ana.Ref: {anaDays}, "extra_days_" + bo.Ref: {""}}
		if ticked {
			form.Set("extra_time_"+ana.Ref, "on")
		}
		return hubPost(h, "/students", form, h.origin).Code
	}
	if code := save("2", true); code != 200 {
		t.Fatalf("save: %d", code)
	}
	for _, bad := range []string{"-1", "31", "two", "1.5"} {
		if code := save(bad, true); code != 400 {
			t.Fatalf("accepted extra days %q", bad)
		}
	}
	if code := save("2", false); code != 400 {
		t.Fatal("accepted extra days without Extra time ticked")
	}
	store, err := workspace.OpenStudents(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range store.Rows {
		if v.UserID == 1 && (!v.ExtraTime || v.ExtraTimeDays != 2) || v.UserID == 2 && (v.ExtraTime || v.ExtraTimeDays != 0) {
			t.Fatalf("saved settings: %+v", v)
		}
	}
	if page := hubGet(h, "/students?course=11").Body.String(); !strings.Contains(page, `name="extra_days_`+ana.Ref+`" value="2"`) {
		t.Fatal("saved extra days not shown back")
	}
}

func TestStudentsPageSavesExtraPercent(t *testing.T) {
	h, root, byID := absencesHub(t)
	ana, bo := byID[1], byID[2]
	save := func(days, percent string, ticked bool) int {
		form := url.Values{"request_secret": {"nonce"}, "course": {"11"}, "students_version": {studentsVersionNow(t, root)},
			"alias_" + ana.Ref: {ana.Alias}, "alias_" + bo.Ref: {bo.Alias}, "extra_days_" + ana.Ref: {days}, "extra_days_" + bo.Ref: {""},
			"extra_percent_" + ana.Ref: {percent}, "extra_percent_" + bo.Ref: {""}}
		if ticked {
			form.Set("extra_time_"+ana.Ref, "on")
		}
		return hubPost(h, "/students", form, h.origin).Code
	}
	if code := save("", "50", true); code != 200 {
		t.Fatalf("save: %d", code)
	}
	for _, bad := range []string{"-1", "201", "half", "1.5"} {
		if code := save("", bad, true); code != 400 {
			t.Fatalf("accepted percent %q", bad)
		}
	}
	if save("", "50", false) != 400 || save("2", "50", true) != 400 {
		t.Fatal("accepted percent without Extra time, or together with days")
	}
	// An older form without the percent box keeps the saved value.
	old := url.Values{"request_secret": {"nonce"}, "course": {"11"}, "students_version": {studentsVersionNow(t, root)},
		"alias_" + ana.Ref: {ana.Alias}, "alias_" + bo.Ref: {bo.Alias}, "extra_time_" + ana.Ref: {"on"}}
	if code := hubPost(h, "/students", old, h.origin).Code; code != 200 {
		t.Fatalf("older form: %d", code)
	}
	store, err := workspace.OpenStudents(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range store.Rows {
		if v.UserID == 1 && v.ExtraTimePercent != 50 {
			t.Fatalf("saved percent lost: %+v", v)
		}
	}
}
