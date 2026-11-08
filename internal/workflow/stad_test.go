package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

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
