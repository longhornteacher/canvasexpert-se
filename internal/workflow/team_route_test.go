package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"canvasexpertse/internal/canvas"
	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/platform"
	"canvasexpertse/internal/workspace"
)

const (
	opToken     = "OP-TOKEN"
	turnerToken = "TURNER-TOKEN"
)

type teamRequest struct{ token, method, path string }

// fakeKeys is the map-backed KeySource the brief asks for.
type fakeKeys struct {
	origin string
	turner bool
}

func (k fakeKeys) Operator() (platform.Credential, error) {
	return platform.Credential{Origin: k.origin, Token: opToken}, nil
}
func (k fakeKeys) Teammate(id int64) (platform.Credential, error) {
	if id == 77 && k.turner {
		return platform.Credential{Origin: k.origin, Token: turnerToken}, nil
	}
	return platform.Credential{}, platform.ErrNoTeammateCredential
}

// teamKit is a Service plus a server that checks which key each request used.
type teamKit struct {
	*Service
	mu       sync.Mutex
	requests []teamRequest
	coTeach  bool // the operator is an active teacher in the owned course
	checkErr int  // status for the operator's teacher check; 0 means 401
	ownedID  int64
	status   *bytes.Buffer
}

func (k *teamKit) seen() []teamRequest {
	k.mu.Lock()
	defer k.mu.Unlock()
	return append([]teamRequest(nil), k.requests...)
}

// teamFixture serves MINE (course 1, yours) and THEIRS (course 2, owned by Turner).
func teamFixture(t *testing.T, handler http.HandlerFunc) *teamKit {
	t.Helper()
	return newTeamKit(t, handler, []domain.Course{{Nickname: "MINE", ID: 1, Current: true}, {Nickname: "THEIRS", ID: 2, Current: true, Owner: "turner"}}, 2, true)
}

func newTeamKit(t *testing.T, handler http.HandlerFunc, courses []domain.Course, ownedID int64, turnerKey bool) *teamKit {
	t.Helper()
	k := &teamKit{ownedID: ownedID, status: &bytes.Buffer{}}
	checkPath := "/api/v1/courses/" + strconv.FormatInt(ownedID, 10)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		k.mu.Lock()
		k.requests = append(k.requests, teamRequest{token, r.Method, r.URL.Path})
		k.mu.Unlock()
		owned := r.URL.Path == checkPath || strings.Contains(r.URL.Path, strings.TrimPrefix(checkPath, "/api/v1")+"/") || strings.HasPrefix(r.URL.Path, checkPath+"/")
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == checkPath && r.Method == "GET" && token == opToken:
			switch {
			case k.coTeach:
				_, _ = w.Write([]byte(`{"id":2,"enrollments":[{"type":"teacher","enrollment_state":"active"}]}`))
			case k.checkErr != 0:
				w.WriteHeader(k.checkErr)
			default:
				w.WriteHeader(http.StatusUnauthorized)
			}
			return
		case owned && !k.coTeach && token != turnerToken, owned && k.coTeach && token != opToken, !owned && token != opToken && strings.Contains(r.URL.Path, "/courses/"):
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		handler(w, r)
	}))
	t.Cleanup(server.Close)
	// The client has no insecure-TLS bypass; trust the fixture through the default
	// transport for the whole test, since teammate clients are built lazily.
	previous := http.DefaultTransport
	http.DefaultTransport = server.Client().Transport
	t.Cleanup(func() { http.DefaultTransport = previous })
	client, err := canvas.New(server.URL, opToken)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	k.Service = &Service{
		Root: root, Client: client, InstallRoot: root, Status: k.status,
		Config: domain.CourseConfig{Schema: 1, Instance: server.URL, Aliases: true, Courses: courses},
		Team:   workspace.TeamConfig{Schema: 1, Teammates: []workspace.Teammate{{Label: "turner", Name: "Ms. Turner", CanvasUserID: 77, KeyExpires: "2099-01-01"}}},
		Keys:   fakeKeys{origin: server.URL, turner: turnerKey},
	}
	return k
}

func notFound(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) }

// ownedRequests counts requests made with Turner's key, and with your key other
// than the teacher check itself.
func (k *teamKit) ownedRequests() (turner, operator int) {
	for _, r := range k.seen() {
		switch {
		case r.token == turnerToken:
			turner++
		case r.token == opToken && r.path != "/api/v1/courses/2":
			operator++
		}
	}
	return
}

func TestEachAreaReachesTheOwnedCourseWithTheOwnersKey(t *testing.T) {
	ctx := context.Background()
	k := teamFixture(t, notFound)
	src := filepath.Join(k.Root, "fixture.pdf")
	if err := os.WriteFile(src, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	writeDoc := func(name string, doc any) string {
		p := filepath.Join(k.Root, name)
		if err := workspace.WriteDocument(p, doc, "Body"); err != nil {
			t.Fatal(err)
		}
		return p
	}
	origin := k.Client.Origin()
	page := writeDoc("page.md", domain.PageDocument{Schema: 1, Kind: "page", Operation: "create", Course: "THEIRS", Title: "Lesson", BodyFormat: "html"})
	module := writeDoc("module.md", domain.ModuleDocument{Schema: 1, Kind: "module", Course: "THEIRS", Instance: origin, CourseID: 2, Title: "Unit", Items: []domain.ModuleItem{{Type: "SubHeader", Title: "Intro"}}})
	assignment := writeDoc("assignment.md", domain.AssignmentDocument{Schema: 1, Kind: "assignment", Operation: "create", Course: "THEIRS", Title: "Essay", Points: domain.MustDecimal("20"), SubmissionTypes: []string{"online_text_entry"}})
	commands := map[string]func() error{
		"push page":       func() error { _, e := k.PushPage(ctx, page); return e },
		"push module":     func() error { _, e := k.PushModule(ctx, module); return e },
		"push assignment": func() error { _, e := k.Push(ctx, assignment); return e },
		"pull dates":      func() error { _, e := k.PullAssignmentDates(ctx, "THEIRS", 5); return e },
		"pull item":       func() error { _, e := k.PullItem(ctx, "THEIRS", "Essay"); return e },
		"pull page":       func() error { _, e := k.PullPage(ctx, "THEIRS", "7"); return e },
		"pull discussion": func() error { _, e := k.PullDiscussion(ctx, "THEIRS", 3, 0); return e },
		"group sets":      func() error { _, e := k.ListGroupSets(ctx, "THEIRS"); return e },
		"pull group set":  func() error { _, e := k.PullGroupSet(ctx, "THEIRS", 9); return e },
		"external":        func() error { _, e := k.ListExternalResources(ctx, "THEIRS", 0, "all"); return e },
		"quiz points":     func() error { _, e := k.QuizPoints(ctx, "THEIRS", "Quiz", nil); return e },
		"pull work":       func() error { _, e := k.PullWork(ctx, "THEIRS", "Essay"); return e },
		"upload file": func() error {
			_, e := k.UploadCourseFile(ctx, "THEIRS", src, CourseUploadOptions{Access: "course"})
			return e
		},
		"test student pull": func() error { _, e := k.PullBuiltinTestStudent(ctx, "THEIRS", 5, 6); return e },
	}
	for name, run := range commands {
		before, _ := k.ownedRequests()
		_ = run()
		if after, operator := k.ownedRequests(); after == before || operator != 0 {
			t.Errorf("%s: owned-course requests with Turner's key went %d -> %d, with %d on your key", name, before, after, operator)
		}
	}
}

func TestPostToOwnedCourseUsesOwnersKeyAndLogsRedactedWrites(t *testing.T) {
	live := fixtureSubmission(5)
	assignment := fixtureAssignment()
	assignment.Name += " (CE)"
	k := newTeamKit(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/graphql" {
			_, _ = w.Write([]byte(`{"data":{"setAssignmentPostPolicy":{"postPolicy":{"postManually":true},"errors":[]}}}`))
			return
		}
		if strings.Contains(r.URL.Path, "submissions/5") {
			if r.Method == "PUT" {
				var request struct {
					Comment struct {
						Text    string `json:"text_comment"`
						Attempt int    `json:"attempt"`
					} `json:"comment"`
					Submission map[string]any `json:"submission"`
				}
				_ = json.NewDecoder(r.Body).Decode(&request)
				if request.Submission != nil {
					n := json.Number(request.Submission["posted_grade"].(string))
					live.EnteredScore, live.Score = &n, &n
				}
				if request.Comment.Text != "" {
					a := request.Comment.Attempt
					live.Comments = append(live.Comments, SubmissionComment{ID: 70, Comment: request.Comment.Text, Attempt: &a})
				}
			}
			_ = json.NewEncoder(w).Encode(live)
			return
		}
		_ = json.NewEncoder(w).Encode(assignment)
	}, []domain.Course{{Nickname: "TEST", ID: 1, Current: true, Owner: "turner"}}, 1, true)
	path := scoringFiles(t, k.Service, 5)
	report, err := k.Post(context.Background(), path)
	if err != nil || len(report.Students) != 1 || report.Students[0].Status != "completed" {
		t.Fatalf("post: %v %#v", err, report)
	}
	for _, r := range k.seen() {
		if r.path != "/api/v1/courses/1" && r.token != turnerToken {
			t.Fatalf("owned-course request used the wrong key: %+v", r)
		}
	}
	entries, err := workspace.ReadTeamLog(k.InstallRoot, "turner", 10)
	if err != nil || len(entries) == 0 {
		t.Fatalf("no action log: %v", err)
	}
	for _, e := range entries {
		if e.Course != "TEST" || e.CourseID != 1 || e.Status != 200 || (e.Path != "/api/graphql" && (e.Method != "PUT" || !strings.HasSuffix(e.Path, "/submissions/:student"))) {
			t.Errorf("unexpected log entry %+v", e)
		}
	}
	raw, _ := os.ReadFile(filepath.Join(k.InstallRoot, ".cse", "team-log.jsonl"))
	if strings.Contains(string(raw), turnerToken) || strings.Contains(string(raw), "Jordan") {
		t.Fatalf("log leaked a token or student name: %s", raw)
	}
}

func TestCoTeachingUsesYourKeyAndLogsNothing(t *testing.T) {
	k := teamFixture(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{"id":1}`)) })
	k.coTeach = true
	_, _ = k.PullAssignmentDates(context.Background(), "THEIRS", 5)
	turner, operator := k.ownedRequests()
	if turner != 0 || operator == 0 {
		t.Fatalf("co-teaching used Turner's key %d times and yours %d", turner, operator)
	}
	scoped, err := k.ForCourse(context.Background(), k.Config.Courses[1])
	if err != nil || scoped.team != nil || scoped.Client != k.Client {
		t.Fatalf("co-taught course should keep your client: %v", err)
	}
	if err := scoped.Client.Write(context.Background(), "PUT", "/api/v1/courses/2/assignments/5", map[string]any{}, nil); err != nil {
		t.Fatal(err)
	}
	if entries, _ := workspace.ReadTeamLog(k.InstallRoot, "", 10); len(entries) != 0 {
		t.Fatalf("your own write was logged: %+v", entries)
	}
}

func TestYourOwnCourseMakesNoTeacherCheck(t *testing.T) {
	k := teamFixture(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{"id":1}`)) })
	_, _ = k.PullAssignmentDates(context.Background(), "MINE", 5)
	for _, r := range k.seen() {
		if r.path == "/api/v1/courses/1" {
			t.Fatalf("unowned course made a teacher check: %+v", r)
		}
	}
	if scoped, err := k.ForCourse(context.Background(), k.Config.Courses[0]); err != nil || scoped != k.Service {
		t.Fatalf("unowned course should return the receiver: %v", err)
	}
}

func TestMissingTeammateKeyStopsBeforeAnyCourseRequest(t *testing.T) {
	k := teamFixture(t, notFound)
	k.Keys = fakeKeys{origin: k.Client.Origin()}
	_, err := k.PullAssignmentDates(context.Background(), "THEIRS", 5)
	want := "Ms. Turner's Canvas key isn't on this PC. Add it on the Team page, or run this on the team PC."
	if err == nil || err.Error() != want {
		t.Fatalf("got %v", err)
	}
	if turner, operator := k.ownedRequests(); turner+operator != 0 {
		t.Fatalf("a course request was made without the key")
	}
}

func TestRejectedTeammateKeyNamesTeammateAndCourse(t *testing.T) {
	k := teamFixture(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusUnauthorized) })
	_, err := k.PullAssignmentDates(context.Background(), "THEIRS", 5)
	var ce *canvas.Error
	if err == nil || !errors.As(err, &ce) || ce.Kind != "authentication" {
		t.Fatalf("expected the canvas error to stay reachable, got %v", err)
	}
	if !strings.Contains(err.Error(), "Canvas didn't accept Ms. Turner's key for THEIRS.") || !strings.Contains(err.Error(), "Nothing more was done in that course.") {
		t.Fatalf("message: %v", err)
	}
}

func TestTeacherCheckFailureIsNotCachedAndNeverSwitchesKeys(t *testing.T) {
	k := teamFixture(t, notFound)
	k.checkErr = http.StatusServiceUnavailable
	ctx := context.Background()
	if _, err := k.ForCourse(ctx, k.Config.Courses[1]); err == nil {
		t.Fatal("a 503 on the check must fail the course")
	}
	first := len(k.seen())
	if _, err := k.ForCourse(ctx, k.Config.Courses[1]); err == nil || len(k.seen()) == first {
		t.Fatalf("the failed check was cached: %v", err)
	}
	for _, r := range k.seen() {
		if r.token == turnerToken {
			t.Fatal("a transient error reached the teammate's key")
		}
	}
}

func TestKeysNilRoutesToYourClientButOwnershipHolds(t *testing.T) {
	k := teamFixture(t, notFound)
	k.Keys = nil
	course := k.Config.Courses[1]
	scoped, err := k.ForCourse(context.Background(), course)
	if err != nil || scoped != k.Service || len(k.seen()) != 0 {
		t.Fatalf("expected the same service and no requests: %v", err)
	}
	if owner, owned, err := k.Owned(course); err != nil || !owned || owner.Label != "turner" {
		t.Fatalf("Owned: %v %v %v", owner, owned, err)
	}
	if _, owned, err := k.Owned(k.Config.Courses[0]); err != nil || owned {
		t.Fatal("your own course reported as owned")
	}
	stray := domain.Course{Nickname: "Lee P1", ID: 3, Owner: "lee"}
	if _, _, err := k.Owned(stray); err == nil {
		t.Fatal("an owner missing from the team list must be an error, not unowned")
	}
	if _, err := k.ForCourse(context.Background(), stray); err == nil || !strings.Contains(err.Error(), `owner "lee"`) {
		t.Fatalf("ForCourse with unknown owner: %v", err)
	}
}

func TestPullWorkWithNoCoursePullsOnlyYourOwn(t *testing.T) {
	k := teamFixture(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`[]`)) })
	_, _ = k.PullWork(context.Background(), "", "")
	for _, r := range k.seen() {
		if strings.Contains(r.path, "/courses/2") {
			t.Fatalf("a teammate's course was pulled without being named: %+v", r)
		}
	}
	if len(k.seen()) == 0 {
		t.Fatal("expected your own course to be read")
	}
}

func TestExpiryNotePrintedOncePerTeammate(t *testing.T) {
	k := teamFixture(t, notFound)
	k.Team.Teammates[0].KeyExpires = "2000-01-01"
	for i := 0; i < 2; i++ {
		if _, err := k.ForCourse(context.Background(), k.Config.Courses[1]); err != nil {
			t.Fatal(err)
		}
	}
	if got := k.status.String(); got != "Note: Ms. Turner's Canvas key expires 2000-01-01.\n" {
		t.Fatalf("status %q", got)
	}
}

func TestParentReportAndFamilyContactReachOwnedCourseWithOwnersKey(t *testing.T) {
	k := teamFixture(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`[]`)) })
	calendar := workspace.SchoolCalendar{Schema: 1, Timezone: "America/Chicago", StartDate: "2026-08-19", EndDate: "2027-05-27", GradingPeriods: []workspace.GradingPeriod{{Name: "Term 1", StartDate: "2026-08-19", EndDate: "2026-10-08"}, {Name: "Term 2", StartDate: "2026-10-13", EndDate: "2026-12-18"}}}
	if err := workspace.WriteTeacherDocument(k.Root, "context/school-calendar.md", calendar, ""); err != nil {
		t.Fatal(err)
	}
	if err := workspace.WriteTeacherDocument(k.Root, "context/parent-updates.md", ParentConfig{Schema: 1, SkipScore: 95}, ""); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	_, _, _ = k.ParentReport(ctx, []string{"THEIRS"}, nil, mustTime(t, "2026-10-17T12:00:00Z"))
	if turner, operator := k.ownedRequests(); turner == 0 || operator != 0 {
		t.Fatalf("parent report: %d requests with Turner's key, %d with yours", turner, operator)
	}
	before, _ := k.ownedRequests()
	dir := filepath.Join(k.Root, "courses", "THEIRS", "family-contact", "unit")
	b, _ := json.Marshal(FamilyContact{Title: "Unit", Messages: []FamilyMessage{{Key: "m", Audience: FamilyAudience{Courses: []string{"THEIRS"}}, Subject: "S", Body: "B"}}})
	if err := workspace.WriteFile(filepath.Join(dir, "contact.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
	_, _ = k.PrepareFamilyContact(ctx, dir)
	if after, operator := k.ownedRequests(); after == before || operator != 0 {
		t.Fatalf("family contact: %d -> %d with Turner's key, %d with yours", before, after, operator)
	}
}

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
