package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

// fakeSTADCanvas is a small stateful Canvas: one course, three students, one
// collaborative group set at a time.
type fakeSTADCanvas struct {
	mu      sync.Mutex
	setID   int64 // 0 until created
	setName string
	groups  map[int64]string // id -> name
	member  map[int64]int64  // user -> group
	nextID  int64
	writes  []string
	t       *testing.T
}

func (f *fakeSTADCanvas) handler(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	write := func(v any) { _ = json.NewEncoder(w).Encode(v) }
	group := func(id int64) map[string]any {
		return map[string]any{"id": id, "name": f.groups[id], "course_id": 1, "context_type": "Course", "group_category_id": f.setID, "non_collaborative": false}
	}
	set := func() map[string]any {
		return map[string]any{"id": f.setID, "name": f.setName, "course_id": 1, "context_type": "Course", "non_collaborative": false, "self_signup": nil}
	}
	p := r.URL.Path
	if r.Method != http.MethodGet {
		f.writes = append(f.writes, r.Method+" "+p)
	}
	switch {
	case p == "/api/v1/courses/1/enrollments":
		var list []any
		for _, id := range []int{10, 11, 12} {
			list = append(list, map[string]any{"user_id": id, "type": "StudentEnrollment", "enrollment_state": "active", "user": map[string]any{"id": id, "name": fmt.Sprintf("Student %d", id)}})
		}
		write(list)
	case p == "/api/v1/courses/1/assignments":
		write([]any{})
	case p == "/api/v1/courses/1/group_categories" && r.Method == http.MethodGet:
		if f.setID == 0 {
			write([]any{})
		} else {
			write([]any{set()})
		}
	case p == "/api/v1/courses/1/group_categories" && r.Method == http.MethodPost:
		f.setID, f.setName = 500, "STAD Teams"
		write(set())
	case p == fmt.Sprintf("/api/v1/group_categories/%d", f.setID) && f.setID != 0:
		write(set())
	case p == fmt.Sprintf("/api/v1/group_categories/%d/groups", f.setID) && r.Method == http.MethodGet:
		var list []any
		for id := range f.groups {
			list = append(list, group(id))
		}
		write(list)
	case p == fmt.Sprintf("/api/v1/group_categories/%d/groups", f.setID) && r.Method == http.MethodPost:
		_ = r.ParseForm()
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.nextID++
		id := 600 + f.nextID
		name, _ := body["name"].(string)
		if name == "" {
			name = r.Form.Get("name")
		}
		f.groups[id] = name
		write(group(id))
	case strings.HasPrefix(p, "/api/v1/groups/") && strings.HasSuffix(p, "/memberships") && r.Method == http.MethodGet:
		var gid int64
		fmt.Sscanf(p, "/api/v1/groups/%d/memberships", &gid)
		var list []any
		for u, g := range f.member {
			if g == gid {
				list = append(list, map[string]any{"id": u, "group_id": g, "user_id": u, "workflow_state": "accepted"})
			}
		}
		write(list)
	case strings.HasPrefix(p, "/api/v1/groups/") && strings.HasSuffix(p, "/memberships") && r.Method == http.MethodPost:
		var gid int64
		fmt.Sscanf(p, "/api/v1/groups/%d/memberships", &gid)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		uid := int64(0)
		if v, ok := body["user_id"].(float64); ok {
			uid = int64(v)
		}
		if uid == 0 {
			_ = r.ParseForm()
			fmt.Sscan(r.Form.Get("user_id"), &uid)
		}
		f.member[uid] = gid
		write(map[string]any{"id": uid, "group_id": gid, "user_id": uid, "workflow_state": "accepted"})
	case strings.HasPrefix(p, "/api/v1/groups/") && r.Method == http.MethodGet:
		var gid int64
		fmt.Sscanf(p, "/api/v1/groups/%d", &gid)
		write(group(gid))
	case strings.HasPrefix(p, "/api/v1/groups/") && r.Method == http.MethodPut:
		var gid int64
		fmt.Sscanf(p, "/api/v1/groups/%d", &gid)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if n, ok := body["name"].(string); ok {
			f.groups[gid] = n
		} else {
			_ = r.ParseForm()
			if n := r.Form.Get("name"); n != "" {
				f.groups[gid] = n
			}
		}
		write(group(gid))
	default:
		f.t.Errorf("unexpected Canvas request %s %s", r.Method, r.URL.String())
		http.NotFound(w, r)
	}
}

func writeTeams(t *testing.T, root string, f STADTeamsFile) {
	t.Helper()
	if err := workspace.WriteTeacherDocument(root, "context/stad/TEST/teams.md", f, ""); err != nil {
		t.Fatal(err)
	}
}

func stadFixture(t *testing.T) (*Service, *fakeSTADCanvas, [3]string) {
	f := &fakeSTADCanvas{groups: map[int64]string{}, member: map[int64]int64{}, t: t}
	svc, _ := workflowFixture(t, f.handler)
	store, _ := workspace.OpenStudents(svc.Root)
	var refs [3]string
	for i, id := range []int64{10, 11, 12} {
		st, err := store.Upsert(svc.Client.Origin(), id, fmt.Sprintf("Student %d", id), "", "", "")
		if err != nil {
			t.Fatal(err)
		}
		refs[i] = st.Ref
	}
	if err := store.Save(); err != nil {
		t.Fatal(err)
	}
	if err := workspace.WriteTeacherDocument(svc.Root, "context/courses.md", svc.Config, ""); err != nil {
		t.Fatal(err)
	}
	return svc, f, refs
}

func (f *fakeSTADCanvas) teamOf(user int64) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.groups[f.member[user]]
}

var stadNow = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)

func TestSTADPushTeamsCreatesSetPlacesStudentsAndRerunsQuietly(t *testing.T) {
	svc, canvas, refs := stadFixture(t)
	writeTeams(t, svc.Root, STADTeamsFile{Schema: 1, GroupSet: "STAD Teams", Rounds: []STADRound{{Name: "Round 1", Starts: "2026-11-02", Teams: []STADTeam{
		{Name: "Comets", Members: []string{refs[0], refs[1]}}, {Name: "Rockets", Members: []string{refs[2]}}}}}})
	rep, err := svc.STADPushTeams(context.Background(), "TEST", stadNow)
	if err != nil || rep.Status != "completed" || !rep.SetCreated || rep.Placed != 3 {
		t.Fatalf("first push = %+v err=%v", rep, err)
	}
	if canvas.teamOf(10) != "Comets" || canvas.teamOf(11) != "Comets" || canvas.teamOf(12) != "Rockets" {
		t.Fatalf("members = %v groups = %v", canvas.member, canvas.groups)
	}
	before := len(canvas.writes)
	rep, err = svc.STADPushTeams(context.Background(), "TEST", stadNow)
	if err != nil || rep.Status != "completed" || rep.Placed != 0 || len(canvas.writes) != before {
		t.Fatalf("a matching set must need no writes: %+v err=%v writes=%v", rep, err, canvas.writes[before:])
	}
}

func TestSTADPushTeamsReshufflesRenamesAndLeavesExtrasAlone(t *testing.T) {
	svc, canvas, refs := stadFixture(t)
	round1 := STADRound{Name: "Round 1", Starts: "2026-11-02", Teams: []STADTeam{{Name: "Comets", Members: []string{refs[0], refs[1]}}, {Name: "Rockets", Members: []string{refs[2]}}}}
	writeTeams(t, svc.Root, STADTeamsFile{Schema: 1, GroupSet: "STAD Teams", Rounds: []STADRound{round1}})
	if rep, err := svc.STADPushTeams(context.Background(), "TEST", stadNow); err != nil || rep.Status != "completed" {
		t.Fatalf("setup push = %+v err=%v", rep, err)
	}
	round2 := STADRound{Name: "Round 2", Starts: "2026-11-09", Teams: []STADTeam{{Name: "Comets", Members: []string{refs[0]}}, {Name: "Stars", Members: []string{refs[1], refs[2]}}}}
	writeTeams(t, svc.Root, STADTeamsFile{Schema: 1, GroupSet: "STAD Teams", Rounds: []STADRound{round1, round2}})
	rep, err := svc.STADPushTeams(context.Background(), "TEST", stadNow)
	if err != nil || rep.Status != "completed" || rep.Round != "Round 2" {
		t.Fatalf("reshuffle = %+v err=%v", rep, err)
	}
	if len(rep.Renamed) != 1 || rep.Renamed[0] != "Rockets -> Stars" || len(rep.Created) != 0 {
		t.Fatalf("the Rockets group (shared member) should become Stars: %+v", rep)
	}
	if canvas.teamOf(10) != "Comets" || canvas.teamOf(11) != "Stars" || canvas.teamOf(12) != "Stars" || len(canvas.groups) != 2 {
		t.Fatalf("members = %v groups = %v", canvas.member, canvas.groups)
	}
	if rep.Placed != 1 {
		t.Fatalf("only the one student who changed teams moves: %+v", rep)
	}
}

func TestSTADPushTeamsHoldsOnUnknownStudentBeforeAnyWrite(t *testing.T) {
	svc, canvas, refs := stadFixture(t)
	writeTeams(t, svc.Root, STADTeamsFile{Schema: 1, GroupSet: "STAD Teams", Rounds: []STADRound{{Name: "Round 1", Starts: "2026-11-02", Teams: []STADTeam{
		{Name: "Comets", Members: []string{refs[0], "s-nobody"}}}}}})
	rep, err := svc.STADPushTeams(context.Background(), "TEST", stadNow)
	if err != nil || rep.Status != "held" || len(canvas.writes) != 0 || !strings.Contains(rep.Message, "s-nobody") {
		t.Fatalf("rep = %+v err=%v writes=%v", rep, err, canvas.writes)
	}
	report, err := svc.STADTeams(context.Background(), "TEST", stadNow)
	if err != nil || report.OK() || len(report.Unassigned) != 2 {
		t.Fatalf("check = %+v err=%v", report, err)
	}
}

func TestSTADTeamsFileRefusesDuplicatesAndBadOrder(t *testing.T) {
	good := STADRound{Name: "R1", Starts: "2026-11-02", Teams: []STADTeam{{Name: "A", Members: []string{"s-1"}}}}
	for name, f := range map[string]STADTeamsFile{
		"no set":      {Schema: 1, Rounds: []STADRound{good}},
		"same ref":    {Schema: 1, GroupSet: "G", Rounds: []STADRound{{Name: "R1", Starts: "2026-11-02", Teams: []STADTeam{{Name: "A", Members: []string{"s-1"}}, {Name: "B", Members: []string{"s-1"}}}}}},
		"round order": {Schema: 1, GroupSet: "G", Rounds: []STADRound{good, {Name: "R2", Starts: "2026-10-01", Teams: good.Teams}}},
		"joined":      {Schema: 1, GroupSet: "G", Rounds: []STADRound{{Name: "R1", Starts: "2026-11-02", Teams: []STADTeam{{Name: "A", Members: []string{"s-1"}, Joined: map[string]string{"s-2": "2026-11-05"}}}}}},
	} {
		if err := f.Validate(); err == nil {
			t.Errorf("%s: expected a refusal", name)
		}
	}
}

func stadStandingsRoutes(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("standings made a write: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		enr := func(id int, created string) map[string]any {
			return map[string]any{"user_id": id, "type": "StudentEnrollment", "enrollment_state": "active", "created_at": created, "user": map[string]any{"id": id, "name": fmt.Sprintf("Student %d", id)}}
		}
		sub := func(user, a int, extra map[string]any) map[string]any {
			m := map[string]any{"user_id": user, "assignment_id": a, "assignment_visible": true}
			for k, v := range extra {
				m[k] = v
			}
			return m
		}
		turned := map[string]any{"attempt": 1, "submitted_at": "2026-11-03T15:00:00Z"}
		switch r.URL.Path {
		case "/api/v1/courses/1/enrollments":
			_ = json.NewEncoder(w).Encode([]any{enr(10, "2026-08-01T00:00:00Z"), enr(11, "2026-08-01T00:00:00Z"), enr(12, "2026-08-01T00:00:00Z"), enr(13, "2026-11-05T00:00:00Z")})
		case "/api/v1/courses/1/assignments":
			_ = json.NewEncoder(w).Encode([]any{
				map[string]any{"id": 1, "name": "First", "published": true, "grading_type": "points", "submission_types": []string{"online_upload"}, "due_at": "2026-11-03T18:00:00Z", "points_possible": 10},
				map[string]any{"id": 2, "name": "Second", "published": true, "grading_type": "points", "submission_types": []string{"online_upload"}, "due_at": "2026-11-10T18:00:00Z", "points_possible": 10}})
		case "/api/v1/courses/1/students/submissions":
			late := map[string]any{"attempt": 1, "submitted_at": "2026-11-04T15:00:00Z", "late": true}
			_ = json.NewEncoder(w).Encode([]any{
				sub(10, 1, turned), sub(11, 1, late), sub(12, 1, turned), sub(13, 1, map[string]any{"missing": true}),
				sub(10, 2, turned), sub(11, 2, map[string]any{"excused": true}), sub(12, 2, turned), sub(13, 2, turned)})
		default:
			t.Errorf("unexpected Canvas route %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}
}

// stadStandingsFixture is a course with two teams, a mid-round joiner, an
// excused member and two awards (see the standings test for the arithmetic).
func stadStandingsFixture(t *testing.T, handler http.HandlerFunc) (*Service, [4]string) {
	svc, _ := workflowFixture(t, handler)
	store, _ := workspace.OpenStudents(svc.Root)
	var refs [4]string
	for i, id := range []int64{10, 11, 12, 13} {
		st, err := store.Upsert(svc.Client.Origin(), id, fmt.Sprintf("Student %d", id), "", "", "")
		if err != nil {
			t.Fatal(err)
		}
		refs[i] = st.Ref
	}
	_ = store.Save()
	_ = workspace.WriteTeacherDocument(svc.Root, "context/courses.md", svc.Config, "")
	writeTeams(t, svc.Root, STADTeamsFile{Schema: 1, GroupSet: "STAD Teams", Rounds: []STADRound{{Name: "Round 1", Starts: "2026-11-02", Teams: []STADTeam{
		{Name: "Comets", Members: []string{refs[0], refs[1]}},
		{Name: "Rockets", Members: []string{refs[2], refs[3]}, Joined: map[string]string{refs[3]: "2026-11-05"}}}}}})
	if err := workspace.WriteTeacherDocument(svc.Root, "context/stad/TEST/awards.md", STADAwardsFile{Schema: 1, Awards: []STADAward{
		{Date: "2026-11-04", Student: refs[0], Points: domain.MustDecimal("5"), Reason: "vocabulary challenge"},
		{Date: "2026-11-10", Team: "Comets", Points: domain.MustDecimal("3"), Reason: "hallway cleanup"}}}, ""); err != nil {
		t.Fatal(err)
	}
	return svc, refs
}

func TestSTADStandingsAveragesByActiveMembersAndAppliesAwardCap(t *testing.T) {
	svc, refs := stadStandingsFixture(t, stadStandingsRoutes(t))
	_ = refs
	now := time.Date(2026, 11, 13, 18, 0, 0, 0, time.UTC)
	got, err := svc.STADStandings(context.Background(), "TEST", "", "", now)
	if err != nil {
		t.Fatal(err)
	}
	// Comets: week 1 (1 + 0.5)/2, week 2 1/2 (Student 11 excused), two weekly
	// bonuses of 2, the award capped at 3 (/2), the team award 3: 9.75.
	// Rockets: week 1 student 12 alone, 13 not yet on the team: 1; week 2 (1+1)/2; two bonuses: 6.
	want := map[string][2]string{"Comets": {"9.8", "5.5"}, "Rockets": {"6", "3"}}
	for _, row := range got.Teams {
		if row.Visibility != "team" || [2]string{row.ScoreRound, row.ScoreWeek} != want[row.Name] {
			t.Errorf("%s = round %s week %s, want %v", row.Name, row.ScoreRound, row.ScoreWeek, want[row.Name])
		}
	}
	if got.Teams[0].Name != "Comets" || got.Teams[0].RankRound != 1 || got.Teams[1].RankRound != 2 {
		t.Fatalf("ranking = %+v", got.Teams)
	}
	if len(got.Teacher.Trimmed) != 1 || got.Teacher.Trimmed[0].Given != "5" || got.Teacher.Trimmed[0].Counted != "3" {
		t.Fatalf("the cap must trim and say so: %+v", got.Teacher.Trimmed)
	}
	if got.Teacher.Visibility != "teacher" || len(got.Teacher.Students) != 4 {
		t.Fatalf("teacher view = %+v", got.Teacher)
	}
	for _, d := range got.Teacher.Teams {
		if !d.BonusProvisional {
			t.Errorf("%s: the running week's bonus should be marked provisional", d.Name)
		}
	}
}

func TestSTADRankSharesTies(t *testing.T) {
	teams := []STADTeamScore{{exactRound: big.NewRat(5, 1)}, {exactRound: big.NewRat(5, 1)}, {exactRound: big.NewRat(2, 1)}}
	rank(teams, func(x *STADTeamScore) *big.Rat { return x.exactRound }, func(x *STADTeamScore, r int) { x.RankRound = r })
	if teams[0].RankRound != 1 || teams[1].RankRound != 1 || teams[2].RankRound != 3 {
		t.Fatalf("ranks = %d %d %d", teams[0].RankRound, teams[1].RankRound, teams[2].RankRound)
	}
}

func TestSTADRulesDefaultsAndValidation(t *testing.T) {
	root := t.TempDir()
	if r, err := ReadSTADRules(root, "TEST"); err != nil || r.Effort.Late.String() != "0.5" || r.Awards.CapPerStudentPerWeek.String() != "3" {
		t.Fatalf("a missing rules file means the defaults: %+v %v", r, err)
	}
	bad := defaultSTADRules()
	bad.Improve = &STADImprovement{Evidence: "quizzes", Baseline: "mean_of_last 3", Bands: []STADBand{{Points: domain.MustDecimal("1")}}}
	if err := bad.Validate(); err == nil {
		t.Fatal("a band list with no otherwise band must be refused")
	}
	bad.Improve.Bands = []STADBand{{AtLeast: &[]domain.Decimal{domain.MustDecimal("0")}[0], Points: domain.MustDecimal("2")}, {Otherwise: true, Points: domain.MustDecimal("0")}}
	if err := bad.Validate(); err != nil {
		t.Fatalf("valid improvement rules refused: %v", err)
	}
	bad.Improve.Evidence = "vibes"
	if err := bad.Validate(); err == nil {
		t.Fatal("an unknown evidence kind must be refused")
	}
}

func TestSTADPublishCreatesUpdatesHoldsAndRefusesStudentNames(t *testing.T) {
	routes := stadStandingsRoutes(t)
	page := map[string]any{"page_id": 7, "url": "team-standings", "title": "", "body": "", "published": false, "updated_at": "2026-11-13T10:00:00Z"}
	exists := false
	posts, puts := 0, 0
	svc, refs := stadStandingsFixture(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/courses/1/pages" && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("[]"))
		case r.URL.Path == "/api/v1/courses/1/pages" && r.Method == http.MethodPost:
			posts++
			var in struct {
				WikiPage map[string]any `json:"wiki_page"`
			}
			_ = json.NewDecoder(r.Body).Decode(&in)
			page["title"], page["body"], page["published"], exists = in.WikiPage["title"], in.WikiPage["body"], in.WikiPage["published"], true
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(page)
		case r.URL.Path == "/api/v1/courses/1/pages/page_id:7" && exists:
			w.Header().Set("Content-Type", "application/json")
			if r.Method == http.MethodPut {
				puts++
				var in struct {
					WikiPage map[string]any `json:"wiki_page"`
				}
				_ = json.NewDecoder(r.Body).Decode(&in)
				page["title"], page["body"] = in.WikiPage["title"], in.WikiPage["body"]
				page["updated_at"] = fmt.Sprintf("2026-11-13T10:0%d:00Z", puts)
			}
			_ = json.NewEncoder(w).Encode(page)
		default:
			routes(w, r)
		}
	})
	body := filepath.Join(t.TempDir(), "body.md")
	write := func(text string) {
		if err := os.WriteFile(body, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Date(2026, 11, 13, 18, 0, 0, 0, time.UTC)
	ctx := context.Background()

	write("Great week, teams!\n\n{{standings}}\n\nKeep going.")
	rep, err := svc.STADPublish(ctx, "TEST", "Team Standings", body, false, now)
	if err != nil || rep.Status != "created" || posts != 1 || page["published"] != false {
		t.Fatalf("create = %+v err=%v", rep, err)
	}
	html := page["body"].(string)
	if !strings.Contains(html, "<td>Comets</td>") || !strings.Contains(html, "<td>9.8</td>") || strings.Contains(html, "{{standings}}") || strings.Contains(html, refs[0]) {
		t.Fatalf("page body = %s", html)
	}
	rep, err = svc.STADPublish(ctx, "TEST", "Team Standings", body, false, now)
	if err != nil || rep.Status != "unchanged" || puts != 0 {
		t.Fatalf("an identical republish must not write: %+v err=%v puts=%d", rep, err, puts)
	}
	write("New wording.\n\n{{standings}}")
	rep, err = svc.STADPublish(ctx, "TEST", "Team Standings", body, false, now)
	if err != nil || rep.Status != "updated" || puts != 1 {
		t.Fatalf("update = %+v err=%v", rep, err)
	}
	page["updated_at"] = "2026-11-13T11:00:00Z" // someone edits the page in Canvas
	write("Third wording.\n\n{{standings}}")
	rep, err = svc.STADPublish(ctx, "TEST", "Team Standings", body, false, now)
	if err != nil || rep.Status != "held" || puts != 1 {
		t.Fatalf("a live edit must hold the update: %+v err=%v puts=%d", rep, err, puts)
	}
	store, _ := workspace.OpenStudents(svc.Root)
	alias := store.Rows[0].Alias
	for _, text := range []string{"Shout-out to " + refs[1] + "!\n\n{{standings}}", "Shout-out to " + strings.ToLower(alias) + "!\n\n{{standings}}"} {
		write(text)
		if _, err = svc.STADPublish(ctx, "TEST", "Team Standings", body, false, now); err == nil {
			t.Fatalf("a body naming a student must be refused: %q", text)
		}
	}
	write("No table here")
	if _, err = svc.STADPublish(ctx, "TEST", "Team Standings", body, false, now); err == nil {
		t.Fatal("a body without the marker must be refused")
	}
}
