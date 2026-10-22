package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"canvasexpertse/internal/domain"
)

func goodTeammate() Teammate {
	return Teammate{Label: "turner", Name: "Ms. Turner", CanvasUserID: 123456, Email: "turner@school.org", KeyExpires: "2027-06-04"}
}

func TestTeamRoundTripAndMissingFile(t *testing.T) {
	root := t.TempDir()
	cfg, err := ReadTeam(root)
	if err != nil || cfg.Schema != 1 || len(cfg.Teammates) != 0 {
		t.Fatalf("missing team.md should mean no teammates: %+v %v", cfg, err)
	}
	cfg.Teammates = []Teammate{goodTeammate()}
	if err := WriteTeam(root, cfg); err != nil {
		t.Fatal(err)
	}
	got, err := ReadTeam(root)
	if err != nil || len(got.Teammates) != 1 || got.Teammates[0] != goodTeammate() {
		t.Fatalf("round trip: %+v %v", got, err)
	}
	if found, ok := got.Find("TURNER"); !ok || found.CanvasUserID != 123456 {
		t.Fatal("Find should ignore case")
	}
}

func TestTeamValidateRejectsEachBadField(t *testing.T) {
	cases := map[string]func(*Teammate){
		"bad label":    func(m *Teammate) { m.Label = "Ms Turner" },
		"digit label":  func(m *Teammate) { m.Label = "1turner" },
		"zero id":      func(m *Teammate) { m.CanvasUserID = 0 },
		"empty name":   func(m *Teammate) { m.Name = " " },
		"bad date":     func(m *Teammate) { m.KeyExpires = "June 4" },
		"missing date": func(m *Teammate) { m.KeyExpires = "" },
		"email no at":  func(m *Teammate) { m.Email = "turner" },
		"email spaces": func(m *Teammate) { m.Email = "a b@school.org" },
	}
	for name, mutate := range cases {
		m := goodTeammate()
		mutate(&m)
		if (TeamConfig{Schema: 1, Teammates: []Teammate{m}}).Validate() == nil {
			t.Errorf("%s accepted", name)
		}
	}
	dupLabel, dupID := goodTeammate(), goodTeammate()
	dupLabel.CanvasUserID = 2
	dupID.Label = "other"
	for name, list := range map[string][]Teammate{"duplicate label": {goodTeammate(), dupLabel}, "duplicate id": {goodTeammate(), dupID}} {
		if (TeamConfig{Schema: 1, Teammates: list}).Validate() == nil {
			t.Errorf("%s accepted", name)
		}
	}
	if (TeamConfig{Schema: 2}).Validate() == nil {
		t.Error("schema 2 accepted")
	}
	noEmail := goodTeammate()
	noEmail.Email = ""
	if err := (TeamConfig{Schema: 1, Teammates: []Teammate{noEmail}}).Validate(); err != nil {
		t.Errorf("email is optional: %v", err)
	}
}

func TestTeammateNotesAreNotOverwritten(t *testing.T) {
	root := t.TempDir()
	m := goodTeammate()
	if err := EnsureTeammateNotes(root, m); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(root, "context", "team", "turner.md")
	b, err := os.ReadFile(p)
	if err != nil || !strings.Contains(string(b), "Ms. Turner's courses") {
		t.Fatalf("starter missing: %q %v", b, err)
	}
	if err := os.WriteFile(p, []byte("my notes"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := EnsureTeammateNotes(root, m); err != nil {
		t.Fatal(err)
	}
	if b, _ = os.ReadFile(p); string(b) != "my notes" {
		t.Fatal("existing notes were overwritten")
	}
}

func TestCheckOwnersNamesTheCourse(t *testing.T) {
	team := TeamConfig{Schema: 1, Teammates: []Teammate{goodTeammate()}}
	ok := domain.CourseConfig{Courses: []domain.Course{{Nickname: "Mine"}, {Nickname: "Turner P2", Owner: "turner"}}}
	if err := CheckOwners(ok, team); err != nil {
		t.Fatal(err)
	}
	bad := domain.CourseConfig{Courses: []domain.Course{{Nickname: "Lee P1", Owner: "lee"}}}
	if err := CheckOwners(bad, team); err == nil || !strings.Contains(err.Error(), "Lee P1") {
		t.Fatalf("unknown owner should name the course: %v", err)
	}
}

func TestTeammateExpiryStates(t *testing.T) {
	m := goodTeammate() // expires 2027-06-04
	day := func(s string) time.Time {
		d, _ := time.ParseInLocation("2006-01-02", s, time.Local)
		return d.Add(15 * time.Hour)
	}
	for date, want := range map[string]string{"2027-05-20": KeyOK, "2027-05-21": KeySoon, "2027-06-04": KeySoon, "2027-06-05": KeyExpired} {
		if got := m.ExpiryState(day(date)); got != want {
			t.Errorf("%s: %s, want %s", date, got, want)
		}
	}
}

func TestTeamLogRedactsAndReadsNewestFirst(t *testing.T) {
	for in, want := range map[string]string{
		"/api/v1/courses/7/assignments/3/submissions/123?x=1": "/api/v1/courses/7/assignments/3/submissions/:student",
		"/api/v1/users/9/profile":                             "/api/v1/users/:student/profile",
		"/api/v1/courses/7/students/4":                        "/api/v1/courses/7/students/:student",
		"/api/v1/users/self":                                  "/api/v1/users/self",
		"/api/v1/courses/7/submissions/update_grades":         "/api/v1/courses/7/submissions/update_grades",
	} {
		if got := RedactTeamLogPath(in); got != want {
			t.Errorf("%s -> %s, want %s", in, got, want)
		}
	}
	root := t.TempDir()
	for _, who := range []string{"turner", "lee", "turner"} {
		if err := AppendTeamLog(root, TeamLogEntry{Teammate: who, Course: "C", CourseID: 1, Method: "PUT", Path: "/api/v1/courses/1/submissions/55", Status: 200}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := ReadTeamLog(root, "turner", 10)
	if err != nil || len(got) != 2 || got[0].Path != "/api/v1/courses/1/submissions/:student" {
		t.Fatalf("log read: %+v %v", got, err)
	}
	if one, _ := ReadTeamLog(root, "", 1); len(one) != 1 || one[0].Teammate != "turner" {
		t.Fatal("limit or ordering wrong")
	}
}
