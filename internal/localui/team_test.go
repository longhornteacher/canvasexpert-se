package localui

import (
	"context"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"canvasexpertse/internal/canvas"
	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/platform"
	"canvasexpertse/internal/workflow"
	"canvasexpertse/internal/workspace"
)

const teamToken = "teammate-token-must-never-leak"

type teamFixture struct {
	h     *Hub
	root  string
	saved map[int64]platform.Credential
	ids   map[string]teamIdentity // by token
}

func newTeamFixture(t *testing.T) *teamFixture {
	h, root := studentsHub(t)
	f := &teamFixture{h: h, root: root, saved: map[int64]platform.Credential{}, ids: map[string]teamIdentity{
		"op-token": {ID: 1, ShortName: "Me"},
		teamToken:  {ID: 123456, ShortName: "Ms. Turner", Name: "Jane Turner", Email: "turner@school.org"},
	}}
	h.loadCredentialFn = func() (platform.Credential, error) {
		return platform.Credential{Origin: "https://school.example", Token: "op-token"}, nil
	}
	h.loadTeammateFn = func(id int64) (platform.Credential, error) {
		if c, ok := f.saved[id]; ok {
			return c, nil
		}
		return platform.Credential{}, platform.ErrNoTeammateCredential
	}
	h.saveTeammateFn = func(id int64, c platform.Credential) error { f.saved[id] = c; return nil }
	h.deleteTeammateFn = func(id int64) error { delete(f.saved, id); return nil }
	h.teammateIdentityFn = func(_ context.Context, c platform.Credential) (teamIdentity, error) {
		if id, ok := f.ids[c.Token]; ok {
			return id, nil
		}
		return teamIdentity{}, &canvas.Error{Kind: "authentication", Status: 401}
	}
	h.teammateCoursesFn = func(context.Context, platform.Credential) ([]workflow.Course, error) {
		return []workflow.Course{{ID: 11, Name: "Already Mine", Code: "MINE"}, {ID: 701, Name: "Turner Period 2", Code: "T2"}, {ID: 702, Name: "Turner Period 3", Code: "T3"}}, nil
	}
	config := domain.CourseConfig{Schema: 1, Instance: "https://school.example", Courses: []domain.Course{{ID: 11, Nickname: "CS8", Current: true}}}
	if err := workspace.WriteDocument(filepath.Join(root, "context", "courses.md"), config, "Course notes."); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *teamFixture) add(token, expires string) (code int, body, location string) {
	w := hubPost(f.h, "/team/add", url.Values{"request_secret": {"nonce"}, "token": {token}, "key_expires": {expires}}, f.h.origin)
	return w.Code, w.Body.String(), w.Header().Get("Location")
}

func (f *teamFixture) addTurner(t *testing.T) {
	t.Helper()
	if code, body, _ := f.add(teamToken, "2027-06-04"); code != 303 {
		t.Fatalf("add failed: %d %s", code, body)
	}
}

func TestAddTeammateSavesKeyListAndNotesWithoutLeakingToken(t *testing.T) {
	f := newTeamFixture(t)
	code, body, location := f.add(teamToken, "2027-06-04")
	if code != 303 || location != "/team/courses?label=turner" || strings.Contains(body+location, teamToken) {
		t.Fatalf("add: %d %q", code, location)
	}
	if f.saved[123456].Token != teamToken || f.saved[123456].Origin != "https://school.example" {
		t.Fatalf("key not saved for the configured origin: %+v", f.saved)
	}
	team, err := workspace.ReadTeam(f.root)
	want := workspace.Teammate{Label: "turner", Name: "Ms. Turner", CanvasUserID: 123456, Email: "turner@school.org", KeyExpires: "2027-06-04"}
	if err != nil || len(team.Teammates) != 1 || team.Teammates[0] != want {
		t.Fatalf("team.md: %+v %v", team, err)
	}
	if _, err = os.Stat(filepath.Join(f.root, "context", "team", "turner.md")); err != nil {
		t.Fatal("notes starter missing")
	}
	_ = filepath.WalkDir(f.root, func(p string, d fs.DirEntry, _ error) error {
		if d != nil && !d.IsDir() {
			if b, _ := os.ReadFile(p); strings.Contains(string(b), teamToken) {
				t.Errorf("token written to %s", p)
			}
		}
		return nil
	})
	if page := hubGet(f.h, "/team").Body.String(); strings.Contains(page, teamToken) || !strings.Contains(page, "Key saved on this PC") || !strings.Contains(page, "Expires 2027-06-04") {
		t.Fatal("team page missing key or expiry status")
	}
}

func TestAddTeammateRefusals(t *testing.T) {
	f := newTeamFixture(t)
	f.ids["my-own-token"] = teamIdentity{ID: 1, ShortName: "Me"}
	if code, body, _ := f.add("my-own-token", "2027-06-04"); code != 400 || !strings.Contains(body, "That&#39;s your own key") || len(f.saved) != 0 {
		t.Fatalf("own key accepted: %d", code)
	}
	if code, body, _ := f.add("unknown-token", "2027-06-04"); code != 400 || !strings.Contains(body, "didn&#39;t accept") {
		t.Fatalf("bad token accepted: %d", code)
	}
	if code, _, _ := f.add(teamToken, ""); code != 400 {
		t.Fatal("missing expiry accepted")
	}
	f.addTurner(t)
	if code, body, _ := f.add(teamToken, "2027-06-04"); code != 400 || !strings.Contains(body, "Ms. Turner is already on the team") {
		t.Fatalf("duplicate accepted: %d", code)
	}
	for _, instance := range []string{"", "https://canvas.example.edu"} {
		setInstance(t, f.root, instance)
		if code, body, _ := f.add(teamToken, "2027-06-04"); code != 400 || !strings.Contains(body, "Set up your own Canvas connection first.") {
			t.Fatalf("instance %q accepted: %d", instance, code)
		}
	}
}

// setInstance rewrites courses.md with another instance value.
func setInstance(t *testing.T, root, instance string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "context", "courses.md"), []byte("---\nschema: 1\ninstance: \""+instance+"\"\ncourses: []\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAddTeammateDropsTheKeyWhenTheListCannotBeSaved(t *testing.T) {
	f := newTeamFixture(t)
	save := f.h.saveTeammateFn
	f.h.saveTeammateFn = func(id int64, c platform.Credential) error {
		// After the key is stored, make team.md unwritable by putting a folder there.
		if err := save(id, c); err != nil {
			return err
		}
		return os.MkdirAll(filepath.Join(f.root, "context", "team.md"), 0o755)
	}
	if code, _, _ := f.add(teamToken, "2027-06-04"); code != 400 {
		t.Fatalf("expected the form to come back: %d", code)
	}
	if len(f.saved) != 0 {
		t.Fatal("key left behind after team.md failed")
	}
}

func TestChosenCoursesGetTheOwnerAndNicknamesMustBeUnique(t *testing.T) {
	f := newTeamFixture(t)
	f.addTurner(t)
	page := hubGet(f.h, "/team/courses?label=turner").Body.String()
	if !strings.Contains(page, "Turner Period 2") || strings.Contains(page, "Already Mine") || !strings.Contains(page, `name="nick_701"`) || strings.Contains(page, `name="nick_701" value=`) {
		t.Fatal("course list should hide configured courses and start with empty nicknames")
	}
	form := url.Values{"request_secret": {"nonce"}, "label": {"turner"}, "id": {"701", "702"}, "pick_701": {"on"}, "nick_701": {"cs8"}}
	if hubPost(f.h, "/team/courses", form, f.h.origin); len(readCourses(t, f.h)) != 1 {
		t.Fatal("a nickname clashing with another course was accepted")
	}
	form.Set("nick_701", "Turner P2")
	if w := hubPost(f.h, "/team/courses", form, f.h.origin); w.Code != 303 {
		t.Fatalf("save: %d", w.Code)
	}
	got := readCourses(t, f.h)
	if len(got) != 2 || got[0].Nickname != "CS8" || got[0].Owner != "" || got[1].Owner != "turner" || !got[1].Current || got[1].ID != 701 || got[1].Timezone != "" {
		t.Fatalf("courses: %+v", got)
	}
	if body := hubGet(f.h, "/courses").Body.String(); !strings.Contains(body, "<th>Owner</th>") || !strings.Contains(body, "<td>Ms. Turner</td>") || !strings.Contains(body, "<td>You</td>") {
		t.Fatal("courses page should show who owns each course")
	}
}

func readCourses(t *testing.T, h *Hub) []domain.Course {
	t.Helper()
	config, _, err := h.readConfig()
	if err != nil {
		t.Fatal(err)
	}
	return config.Courses
}

func TestRemoveDeletesKeyAndEntryAndKeepsNotesAndLog(t *testing.T) {
	f := newTeamFixture(t)
	f.addTurner(t)
	hubPost(f.h, "/team/courses", url.Values{"request_secret": {"nonce"}, "label": {"turner"}, "id": {"701"}, "pick_701": {"on"}, "nick_701": {"Turner P2"}}, f.h.origin)
	if err := workspace.AppendTeamLog(f.root, workspace.TeamLogEntry{Teammate: "turner", Course: "Turner P2", CourseID: 701, Method: "PUT", Path: "/api/v1/courses/701/x", Status: 200}); err != nil {
		t.Fatal(err)
	}
	form := url.Values{"request_secret": {"nonce"}, "label": {"turner"}}
	if w := hubPost(f.h, "/team/remove", form, f.h.origin); w.Code != 303 {
		t.Fatalf("remove: %d", w.Code)
	}
	team, _ := workspace.ReadTeam(f.root)
	if len(f.saved) != 0 || len(team.Teammates) != 0 || len(readCourses(t, f.h)) != 1 {
		t.Fatalf("remove left state behind: keys %d teammates %d courses %d", len(f.saved), len(team.Teammates), len(readCourses(t, f.h)))
	}
	if _, err := os.Stat(filepath.Join(f.root, "context", "team", "turner.md")); err != nil {
		t.Fatal("notes were deleted")
	}
	if entries, _ := workspace.ReadTeamLog(f.root, "turner", 5); len(entries) != 1 {
		t.Fatal("log was deleted")
	}
}

func TestTestKeyReportsWorkingRejectedAndMissing(t *testing.T) {
	f := newTeamFixture(t)
	f.addTurner(t)
	test := func() string {
		hubPost(f.h, "/team/test", url.Values{"request_secret": {"nonce"}, "label": {"turner"}}, f.h.origin)
		return hubGet(f.h, "/team").Body.String()
	}
	if !strings.Contains(test(), "Ms. Turner&#39;s key works.") {
		t.Fatal("working key not reported")
	}
	f.saved[123456] = platform.Credential{Origin: "https://school.example", Token: "revoked"}
	if !strings.Contains(test(), "Canvas no longer accepts Ms. Turner&#39;s key.") {
		t.Fatal("rejected key not reported")
	}
	delete(f.saved, 123456)
	if !strings.Contains(test(), "Ms. Turner&#39;s Canvas key isn&#39;t on this PC. Add it on the Team page, or run this on the team PC.") {
		t.Fatal("missing key message wrong")
	}
}

func TestExpiryTextAndHomeWarning(t *testing.T) {
	m := workspace.Teammate{Label: "turner", Name: "Ms. Turner", CanvasUserID: 1, KeyExpires: "2027-06-04"}
	at := func(s string) time.Time {
		d, _ := time.ParseInLocation("2006-01-02", s, time.Local)
		return d.Add(9 * time.Hour)
	}
	for date, want := range map[string]string{"2027-01-10": "Expires 2027-06-04", "2027-05-21": "Expires soon: 2027-06-04", "2027-06-05": "Expired 2027-06-04"} {
		if got, _ := teamKeyStatus(m, at(date)); got != want {
			t.Errorf("%s: %q, want %q", date, got, want)
		}
	}
	h := newTestHub()
	h.root = makeWorkspace(t)
	soon := m
	soon.KeyExpires = time.Now().AddDate(0, 0, 3).Format("2006-01-02")
	if err := workspace.WriteTeam(h.root, workspace.TeamConfig{Schema: 1, Teammates: []workspace.Teammate{soon}}); err != nil {
		t.Fatal(err)
	}
	if w := h.teamWarnings(); len(w) != 1 || !strings.Contains(w[0], "Ms. Turner's Canvas key expires "+soon.KeyExpires) {
		t.Fatalf("home warning: %v", w)
	}
}
