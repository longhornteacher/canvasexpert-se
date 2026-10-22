package workflow

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

// copyFake is a small Canvas for module copies: Collab is course 1 (yours),
// Turner P2 is course 2 (owned), MINE P1 is course 3 (yours).
type copyFake struct {
	mu          sync.Mutex
	items       string
	modules     string // source module list
	selective   string
	postStatus  map[int64]int
	neverReady  map[int64]bool
	failedState map[int64]bool
	puts        map[int64]string
	posts       map[int64]string
	selected    map[int64]bool
	onPost      func(dest int64)
	destItems   string            // items of the copied module in every destination
	reads       map[string]string // other destination reads by path; absent paths 404
}

var copyPath = regexp.MustCompile(`^/api/v1/courses/(\d+)/content_migrations(?:/(\d+))?(?:/(\w+))?$`)

func newCopyFake() *copyFake {
	return &copyFake{
		modules:     `[{"id":10,"name":"Unit 3: Argument"},{"id":11,"name":"Other"}]`,
		items:       `[{"title":"Reading"},{"title":"Essay"}]`,
		selective:   `[{"title":"Other","property":"copy[context_modules][id_z]"},{"title":"Unit 3: Argument","property":"copy[context_modules][id_a]","sub_items":[{"title":"Reading","property":"copy[wiki_pages][id_b]"},{"title":"Essay","property":"copy[assignments][id_c]","sub_items":[{"property":"copy[assignment_groups][id_d]"}]}]}]`,
		postStatus:  map[int64]int{},
		neverReady:  map[int64]bool{},
		failedState: map[int64]bool{},
		puts:        map[int64]string{},
		posts:       map[int64]string{},
		selected:    map[int64]bool{},
		reads:       map[string]string{},
	}
}

func (f *copyFake) handle(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	path := r.URL.Path
	switch {
	case path == "/api/v1/courses/1/modules":
		_, _ = w.Write([]byte(f.modules))
		return
	case path == "/api/v1/courses/1/modules/10/items":
		_, _ = w.Write([]byte(f.items))
		return
	case regexp.MustCompile(`^/api/v1/courses/\d+/modules/7788/items$`).MatchString(path):
		if f.destItems == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(f.destItems))
		return
	case f.reads[path] != "":
		_, _ = w.Write([]byte(f.reads[path]))
		return
	case regexp.MustCompile(`^/api/v1/courses/\d+/modules$`).MatchString(path):
		_, _ = w.Write([]byte(`[{"id":7788,"name":"Unit 3: Argument"}]`))
		return
	}
	m := copyPath.FindStringSubmatch(path)
	if m == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	dest, _ := strconv.ParseInt(m[1], 10, 64)
	body := new(strings.Builder)
	buf := make([]byte, 4096)
	for {
		n, err := r.Body.Read(buf)
		body.Write(buf[:n])
		if err != nil {
			break
		}
	}
	switch {
	case r.Method == "POST" && m[2] == "":
		f.posts[dest] = body.String()
		if f.onPost != nil {
			f.onPost(dest)
		}
		if st := f.postStatus[dest]; st != 0 {
			w.WriteHeader(st)
			return
		}
		_, _ = w.Write([]byte(`{"id":` + strconv.FormatInt(900+dest, 10) + `}`))
	case r.Method == "PUT":
		f.puts[dest] = body.String()
		f.selected[dest] = true
		_, _ = w.Write([]byte(`{}`))
	case m[3] == "selective_data":
		_, _ = w.Write([]byte(f.selective))
	case m[3] == "migration_issues":
		_, _ = w.Write([]byte(`[{"description":"` + strings.Repeat("w", 250) + `"}]`))
	default:
		switch {
		case f.failedState[dest]:
			_, _ = w.Write([]byte(`{"workflow_state":"failed"}`))
		case f.selected[dest]:
			_, _ = w.Write([]byte(`{"workflow_state":"completed"}`))
		case f.neverReady[dest]:
			_, _ = w.Write([]byte(`{"workflow_state":"pre_processing"}`))
		default:
			_, _ = w.Write([]byte(`{"workflow_state":"waiting_for_select"}`))
		}
	}
}

func copyKit(t *testing.T, f *copyFake, turnerKey bool) *teamKit {
	t.Helper()
	k := newTeamKit(t, f.handle, []domain.Course{
		{Nickname: "Collab", ID: 1, Current: true},
		{Nickname: "Turner P2", ID: 2, Current: true, Owner: "turner"},
		{Nickname: "MINE P1", ID: 3, Current: true},
	}, 2, turnerKey)
	k.copyTiming = copyTiming{every: func(time.Duration) time.Duration { return time.Millisecond }, selectDeadline: 60 * time.Millisecond, doneDeadline: 60 * time.Millisecond}
	return k
}

func readCopyRecord(t *testing.T, k *teamKit) CopyRecord {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(k.Root, "courses", "_team-copies", "course-1-module-10.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rec CopyRecord
	if err = json.Unmarshal(b, &rec); err != nil {
		t.Fatal(err)
	}
	return rec
}

func entryFor(rec CopyRecord, id int64) *CopyDestination {
	for i := range rec.Destinations {
		if rec.Destinations[i].CourseID == id {
			return &rec.Destinations[i]
		}
	}
	return nil
}

func statusOf(out []CopyOutcome, nickname string) string {
	for _, o := range out {
		if o.Course == nickname {
			return o.Status
		}
	}
	return ""
}

func TestCopyModuleHappyPath(t *testing.T) {
	f := newCopyFake()
	k := copyKit(t, f, true)
	out, err := k.CopyModule(context.Background(), "Collab", "Unit 3: Argument", []string{"Turner P2", "MINE P1", "mine p1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 || statusOf(out, "Turner P2") != "succeeded" || statusOf(out, "MINE P1") != "succeeded" {
		t.Fatalf("outcomes: %+v", out)
	}
	if out[0].Teacher != "Ms. Turner" || out[1].Teacher != "You" || out[0].Issues != 1 {
		t.Fatalf("outcomes: %+v", out)
	}
	wantPost := `{"migration_type":"course_copy_importer","selective_import":true,"settings":{"source_course_id":"1"}}`
	wantPut := `{"copy":{"assignment_groups":{"id_d":"1"},"assignments":{"id_c":"1"},"context_modules":{"id_a":"1"},"wiki_pages":{"id_b":"1"}}}`
	for _, dest := range []int64{2, 3} {
		if f.posts[dest] != wantPost {
			t.Errorf("POST to %d: %s", dest, f.posts[dest])
		}
		if f.puts[dest] != wantPut {
			t.Errorf("PUT to %d: %s", dest, f.puts[dest])
		}
	}
	for _, r := range k.seen() {
		isOwned := strings.Contains(r.path, "/courses/2/")
		if isOwned && r.token != turnerToken {
			t.Errorf("owned course request used the wrong key: %+v", r)
		}
		if strings.Contains(r.path, "/courses/3/") && r.token != opToken {
			t.Errorf("your course request used the wrong key: %+v", r)
		}
	}
	rec := readCopyRecord(t, k)
	if rec.Source.ModuleID != 10 || rec.Source.CourseID != 1 {
		t.Fatalf("source: %+v", rec.Source)
	}
	turner := entryFor(rec, 2)
	if turner == nil || turner.Status != "succeeded" || turner.MigrationID != 902 || turner.DestModuleID != 7788 || turner.Teammate != "turner" || turner.FinishedAt == "" {
		t.Fatalf("turner entry: %+v", turner)
	}
	if len(turner.Issues) != 1 || len([]rune(turner.Issues[0])) != 200 {
		t.Fatalf("issues: %v", turner.Issues)
	}
	if mine := entryFor(rec, 3); mine == nil || mine.Teammate != "" {
		t.Fatalf("mine entry: %+v", mine)
	}
	log, _ := workspace.ReadTeamLog(k.Root, "turner", 0)
	var posts, puts int
	for _, e := range log {
		if e.CourseID != 2 {
			t.Errorf("log entry for another course: %+v", e)
		}
		if e.Method == "POST" {
			posts++
		}
		if e.Method == "PUT" {
			puts++
		}
	}
	if posts != 1 || puts != 1 {
		t.Fatalf("teammate log: %+v", log)
	}
	if all, _ := workspace.ReadTeamLog(k.Root, "", 0); len(all) != len(log) {
		t.Fatalf("your own writes must not be logged: %+v", all)
	}
}

func TestCopyModulePreflightStopsBeforeAnyDestinationRequest(t *testing.T) {
	cases := map[string]struct {
		change func(*copyFake)
		dests  []string
		want   string
	}{
		"missing module":  {func(f *copyFake) { f.modules = `[]` }, []string{"MINE P1"}, `No module titled "Unit 3: Argument" in Collab.`},
		"duplicate title": {func(f *copyFake) { f.modules = `[{"id":10,"name":"Unit 3: Argument"},{"id":12,"name":"Unit 3: Argument"}]` }, []string{"MINE P1"}, `Collab has 2 modules titled "Unit 3: Argument"; rename one first.`},
		"empty module":    {func(f *copyFake) { f.items = `[]` }, []string{"MINE P1"}, `"Unit 3: Argument" in Collab has no items yet.`},
		"consent tag":     {func(f *copyFake) { f.items = `[{"title":"Reading"},{"title":"Essay 3 (CE)"}]` }, []string{"MINE P1", "Turner P2"}, `Remove (CE) from "Essay 3 (CE)" in Collab first. Each teacher adds (CE) in their own course.`},
		"source as dest":  {func(*copyFake) {}, []string{"MINE P1", "Collab"}, "is the source"},
		"no destination":  {func(*copyFake) {}, []string{"Collab"}, "is the source"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			f := newCopyFake()
			c.change(f)
			k := copyKit(t, f, true)
			_, err := k.CopyModule(context.Background(), "Collab", "Unit 3: Argument", c.dests)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("error: %v", err)
			}
			for _, r := range k.seen() {
				if strings.Contains(r.path, "/courses/2") || strings.Contains(r.path, "/courses/3") {
					t.Errorf("destination request made: %+v", r)
				}
			}
			if len(f.posts) != 0 {
				t.Errorf("posts: %v", f.posts)
			}
		})
	}
}

func TestCopyModuleCopiesOnceForEveryPriorStatusAndSurvivesRename(t *testing.T) {
	for _, prior := range []string{"succeeded", "failed", "uncertain", "pending"} {
		t.Run(prior, func(t *testing.T) {
			f := newCopyFake()
			k := copyKit(t, f, true)
			dir := filepath.Join(k.Root, "courses", "_team-copies")
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			rec := CopyRecord{Schema: 1, Source: CopySource{Nickname: "Collab", CourseID: 1, ModuleID: 10}, Destinations: []CopyDestination{{CourseID: 2, Nickname: "Old Name", Status: prior, StartedAt: "2026-10-22T14:00:00Z"}}}
			b, _ := json.Marshal(rec)
			if err := os.WriteFile(filepath.Join(dir, "course-1-module-10.json"), b, 0600); err != nil {
				t.Fatal(err)
			}
			out, err := k.CopyModule(context.Background(), "Collab", "Unit 3: Argument", []string{"Turner P2", "MINE P1"})
			if err != nil {
				t.Fatal(err)
			}
			want := `"Unit 3: Argument" was already sent to Turner P2 on 2026-10-22 (` + prior + `). CE-SE copies a module only once. Finish it in Canvas with Import Course Content, or edit that course's copy.`
			if statusOf(out, "Turner P2") != "refused" || out[0].Message != want {
				t.Fatalf("outcome: %+v", out[0])
			}
			if statusOf(out, "MINE P1") != "succeeded" {
				t.Fatalf("a new destination still proceeds: %+v", out)
			}
			if _, posted := f.posts[2]; posted {
				t.Fatal("refused destination was posted to")
			}
		})
	}
}

func TestCopyModuleRefusedAfterNicknameRename(t *testing.T) {
	f := newCopyFake()
	k := copyKit(t, f, true)
	if _, err := k.CopyModule(context.Background(), "Collab", "Unit 3: Argument", []string{"MINE P1"}); err != nil {
		t.Fatal(err)
	}
	k.Config.Courses[0].Nickname = "Shared Collab"
	k.Config.Courses[2].Nickname = "Period 1"
	out, err := k.CopyModule(context.Background(), "Shared Collab", "Unit 3: Argument", []string{"Period 1"})
	if err != nil {
		t.Fatal(err)
	}
	if statusOf(out, "Period 1") != "refused" {
		t.Fatalf("outcome: %+v", out)
	}
}

func TestCopyModuleFailuresStayWithTheirDestination(t *testing.T) {
	cases := map[string]struct {
		change func(*copyFake)
		status string
		put    bool
	}{
		"post 422":         {func(f *copyFake) { f.postStatus[2] = 422 }, "failed", false},
		"post 500":         {func(f *copyFake) { f.postStatus[2] = 500 }, "uncertain", false},
		"never ready":      {func(f *copyFake) { f.neverReady[2] = true }, "uncertain", false},
		"no match":         {func(f *copyFake) { f.selective = `[{"title":"Other","property":"copy[context_modules][id_z]"}]` }, "failed", false},
		"migration failed": {func(f *copyFake) { f.failedState[2] = true }, "failed", false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			f := newCopyFake()
			c.change(f)
			k := copyKit(t, f, true)
			out, err := k.CopyModule(context.Background(), "Collab", "Unit 3: Argument", []string{"Turner P2", "MINE P1"})
			if err != nil {
				t.Fatal(err)
			}
			if got := statusOf(out, "Turner P2"); got != c.status {
				t.Fatalf("status %q: %+v", got, out[0])
			}
			if _, put := f.puts[2]; put != c.put {
				t.Fatalf("PUT sent: %v", put)
			}
			if name == "no match" {
				// The other destination matches nothing too; only the shared list differs.
				return
			}
			if statusOf(out, "MINE P1") != "succeeded" {
				t.Fatalf("the other destination must not be stopped: %+v", out)
			}
			rec := readCopyRecord(t, k)
			if e := entryFor(rec, 2); e == nil || e.Status != c.status || e.Message == "" {
				t.Fatalf("record: %+v", e)
			}
		})
	}
}

func TestCopyModuleWritesPendingBeforeThePostAndNeverRetries(t *testing.T) {
	f := newCopyFake()
	k := copyKit(t, f, true)
	var sawPending bool
	var posts int
	f.onPost = func(dest int64) {
		posts++
		b, err := os.ReadFile(filepath.Join(k.Root, "courses", "_team-copies", "course-1-module-10.json"))
		sawPending = err == nil && strings.Contains(string(b), `"status": "pending"`)
	}
	f.postStatus[3] = 500
	out, err := k.CopyModule(context.Background(), "Collab", "Unit 3: Argument", []string{"MINE P1"})
	if err != nil {
		t.Fatal(err)
	}
	if !sawPending || posts != 1 || out[0].Status != "uncertain" {
		t.Fatalf("pending first: %v, posts: %d, outcome: %+v", sawPending, posts, out[0])
	}
}

func TestCopyModuleMissingTeammateKeyGetsNoRecordEntry(t *testing.T) {
	f := newCopyFake()
	k := copyKit(t, f, false)
	out, err := k.CopyModule(context.Background(), "Collab", "Unit 3: Argument", []string{"Turner P2", "MINE P1"})
	if err != nil {
		t.Fatal(err)
	}
	if out[0].Status != "failed" || out[0].Message != "Ms. Turner's Canvas key isn't on this PC. Add it on the Team page, or run this on the team PC." {
		t.Fatalf("outcome: %+v", out[0])
	}
	if statusOf(out, "MINE P1") != "succeeded" {
		t.Fatalf("outcomes: %+v", out)
	}
	if entryFor(readCopyRecord(t, k), 2) != nil {
		t.Fatal("a destination that never started must have no record entry")
	}
}

func TestCopyModuleRefusedWhileLockHeldWithNoCanvasRequests(t *testing.T) {
	f := newCopyFake()
	k := copyKit(t, f, true)
	dir := filepath.Join(k.Root, "courses", "_team-copies")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".copy.lock"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	_, err := k.CopyModule(context.Background(), "Collab", "Unit 3: Argument", []string{"MINE P1"})
	if err == nil || !strings.HasPrefix(err.Error(), "Another copy is running; try again when it finishes.") {
		t.Fatalf("error: %v", err)
	}
	if n := len(k.seen()); n != 0 {
		t.Fatalf("%d Canvas requests while locked", n)
	}
	if _, statErr := os.Stat(filepath.Join(dir, ".copy.lock")); statErr != nil {
		t.Fatal("the other run's lock must stay")
	}
}

func TestCopyModuleReleasesLockWhenDone(t *testing.T) {
	f := newCopyFake()
	k := copyKit(t, f, true)
	if _, err := k.CopyModule(context.Background(), "Collab", "Unit 3: Argument", []string{"MINE P1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(k.Root, "courses", "_team-copies", ".copy.lock")); !os.IsNotExist(err) {
		t.Fatalf("lock left behind: %v", err)
	}
}

func TestSelectionBuildsNestedJSON(t *testing.T) {
	root := map[string]any{}
	for _, p := range []string{"copy[a][b]", "copy[a][c]", "copy[d]"} {
		if !addSelection(root, p) {
			t.Fatalf("%s rejected", p)
		}
	}
	b, _ := json.Marshal(root)
	if string(b) != `{"a":{"b":"1","c":"1"},"d":"1"}` {
		t.Fatal(string(b))
	}
	if addSelection(map[string]any{}, "wiki_pages[x]") {
		t.Fatal("a property outside copy[...] must be rejected")
	}
}

const checkItems = `[
{"id":1,"type":"Assignment","title":"Essay 3","content_id":50,"published":true},
{"id":2,"type":"Page","title":"Unit 3 overview","page_url":"unit-3-overview","published":false},
{"id":3,"type":"Discussion","title":"Talk","content_id":70,"published":true},
{"id":4,"type":"SubHeader","title":"Week 1","published":true}]`

func checkFake() *copyFake {
	f := newCopyFake()
	f.destItems = checkItems
	f.reads["/api/v1/courses/2/assignments/50"] = `{"description":"<p>fine</p>","only_visible_to_overrides":true,"overrides":[]}`
	f.reads["/api/v1/courses/2/pages/unit-3-overview"] = `{"body":"<a href=\"/courses/1/pages/x\">a</a> <a href=\"https://x.instructure.com/courses/1/pages/x\">b</a> /courses/12/ /courses/2/pages/ok"}`
	f.reads["/api/v1/courses/2/discussion_topics/70"] = `{"message":"hello","assignment":{"only_visible_to_overrides":true}}`
	return f
}

func TestCopyModuleAfterCopyCheck(t *testing.T) {
	f := checkFake()
	k := copyKit(t, f, true)
	out, err := k.CopyModule(context.Background(), "Collab", "Unit 3: Argument", []string{"Turner P2"})
	if err != nil {
		t.Fatal(err)
	}
	if out[0].Status != "succeeded" || out[0].Message != "" {
		t.Fatalf("outcome: %+v", out[0])
	}
	want := []CopyItemCheck{
		{Type: "Assignment", Title: "Essay 3", ID: "50", Published: true, AssignedToNobody: true},
		{Type: "Page", Title: "Unit 3 overview", ID: "unit-3-overview", CollabLinks: 2},
		{Type: "Discussion", Title: "Talk", ID: "70", Published: true, AssignedToNobody: true},
		{Type: "SubHeader", Title: "Week 1", ID: "0", Published: true},
	}
	if got := entryFor(readCopyRecord(t, k), 2).Items; !reflect.DeepEqual(got, want) || !reflect.DeepEqual(out[0].Items, want) {
		t.Fatalf("items: %+v / %+v", got, out[0].Items)
	}
	lines := CopyCheckLines(out[0])
	wantLines := []string{
		`Turner P2: "Essay 3" is assigned to no one; "Unit 3 overview" still links to Collab (2); "Talk" is assigned to no one.`,
		`Turner P2: 4 items, 3 published, 1 unpublished.`,
	}
	if !reflect.DeepEqual(lines, wantLines) {
		t.Fatalf("lines: %q", lines)
	}
}

func TestCopyModuleAfterCopyCheckOverridesAndCleanDestination(t *testing.T) {
	f := checkFake()
	f.reads["/api/v1/courses/2/assignments/50"] = `{"only_visible_to_overrides":true,"overrides":[{"id":1}]}`
	f.reads["/api/v1/courses/2/pages/unit-3-overview"] = `{"body":"ok"}`
	f.reads["/api/v1/courses/2/discussion_topics/70"] = `{"message":"hello"}`
	k := copyKit(t, f, true)
	out, _ := k.CopyModule(context.Background(), "Collab", "Unit 3: Argument", []string{"Turner P2"})
	if lines := CopyCheckLines(out[0]); !reflect.DeepEqual(lines, []string{"Turner P2: 4 items, 3 published, 1 unpublished."}) {
		t.Fatalf("lines: %q", lines)
	}
}

func TestCopyModuleAfterCopyCheckReadFailureAddsNote(t *testing.T) {
	f := checkFake()
	delete(f.reads, "/api/v1/courses/2/pages/unit-3-overview")
	k := copyKit(t, f, true)
	out, _ := k.CopyModule(context.Background(), "Collab", "Unit 3: Argument", []string{"Turner P2"})
	if out[0].Status != "succeeded" || !strings.Contains(out[0].Message, "Some copied items couldn't be checked.") || len(out[0].Items) != 4 {
		t.Fatalf("outcome: %+v", out[0])
	}
}

func TestCopyModuleNoCheckWhenDestinationModuleNotUnique(t *testing.T) {
	// Two same-titled modules in the destination: no unique module, no check.
	f2 := newCopyFake()
	f2.destItems = checkItems
	k2 := newTeamKit(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/courses/3/modules" {
			_, _ = w.Write([]byte(`[{"id":7788,"name":"Unit 3: Argument"},{"id":7789,"name":"Unit 3: Argument"}]`))
			return
		}
		f2.handle(w, r)
	}, []domain.Course{{Nickname: "Collab", ID: 1, Current: true}, {Nickname: "Turner P2", ID: 2, Current: true, Owner: "turner"}, {Nickname: "MINE P1", ID: 3, Current: true}}, 2, true)
	k2.copyTiming = copyTiming{every: func(time.Duration) time.Duration { return time.Millisecond }, selectDeadline: 60 * time.Millisecond, doneDeadline: 60 * time.Millisecond}
	out, err := k2.CopyModule(context.Background(), "Collab", "Unit 3: Argument", []string{"MINE P1"})
	if err != nil {
		t.Fatal(err)
	}
	if out[0].Status != "succeeded" || out[0].Items != nil || CopyCheckLines(out[0]) != nil {
		t.Fatalf("outcome: %+v", out[0])
	}
	for _, r := range k2.seen() {
		if strings.Contains(r.path, "/items") && strings.Contains(r.path, "/courses/3/") {
			t.Errorf("check ran: %+v", r)
		}
	}
}
