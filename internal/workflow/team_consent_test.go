package workflow

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

// consentKit serves one teammate-owned course (TEST, id 1) and records the order
// of everything that matters for hidden grades.
type consentKit struct {
	*teamKit
	emu    sync.Mutex
	events []string
	name   string   // live assignment name
	posted string   // hidden | visible | absent
	policy []string // ok | errors | 500, one per attempt; the last repeats
}

func (k *consentKit) add(e string) { k.emu.Lock(); k.events = append(k.events, e); k.emu.Unlock() }
func (k *consentKit) count(e string) (n int) {
	k.emu.Lock()
	defer k.emu.Unlock()
	for _, x := range k.events {
		if x == e {
			n++
		}
	}
	return
}
func (k *consentKit) first(e string) int {
	k.emu.Lock()
	defer k.emu.Unlock()
	for i, x := range k.events {
		if x == e {
			return i
		}
	}
	return -1
}

func postedBody(live Submission, posted string) map[string]any {
	raw, _ := json.Marshal(live)
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	switch posted {
	case "visible":
		m["posted_at"] = "2026-10-20T12:00:00Z"
	case "absent":
		delete(m, "posted_at")
	default:
		m["posted_at"] = nil
	}
	return m
}

func consentFixture(t *testing.T, name, posted string, policy ...string) *consentKit {
	t.Helper()
	k := &consentKit{name: name, posted: posted, policy: policy}
	live := fixtureSubmission(5)
	k.teamKit = newTeamKit(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/graphql":
			n := k.count("policy")
			k.add("policy")
			mode := "ok"
			if len(k.policy) > 0 {
				mode = k.policy[min(n, len(k.policy)-1)]
			}
			switch mode {
			case "500":
				w.WriteHeader(http.StatusInternalServerError)
			case "errors":
				_, _ = w.Write([]byte(`{"data":{"setAssignmentPostPolicy":{"postPolicy":null,"errors":[{"attribute":"x","message":"no"}]}}}`))
			default:
				_, _ = w.Write([]byte(`{"data":{"setAssignmentPostPolicy":{"postPolicy":{"postManually":true},"errors":[]}}}`))
			}
		case strings.Contains(r.URL.Path, "submissions/5"):
			if r.Method == "PUT" {
				k.add("put")
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
			} else {
				k.add("get-submission")
			}
			_ = json.NewEncoder(w).Encode(postedBody(live, k.posted))
		default:
			a := fixtureAssignment()
			a.Name = k.name
			_ = json.NewEncoder(w).Encode(a)
		}
	}, []domain.Course{{Nickname: "TEST", ID: 1, Current: true, Owner: "turner"}}, 1, true)
	return k
}

func resultsText(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestOwnedPostHidesGradesBeforeAnyWrite(t *testing.T) {
	k := consentFixture(t, "Essay (CE)", "hidden")
	path := scoringFiles(t, k.Service, 5)
	report, err := k.Post(context.Background(), path)
	if err != nil || len(report.Students) != 1 || report.Students[0].Status != "completed" || !report.Hidden {
		t.Fatalf("post: %v %#v", err, report)
	}
	if k.count("policy") != 1 || k.first("policy") > k.first("put") {
		t.Fatalf("policy must be sent once, before the first write: %v", k.events)
	}
	want := "TEST · Evidence essay: 1 scored, hidden from students, ready for Ms. Turner to review and post."
	if got := report.OwnerSummary(); got != want {
		t.Fatalf("summary %q", got)
	}
}

func TestOwnedPostHoldsStudentsWhoseGradeIsVisibleOrUnknown(t *testing.T) {
	for _, posted := range []string{"visible", "absent"} {
		k := consentFixture(t, "Essay (CE)", posted)
		path := scoringFiles(t, k.Service, 5)
		before := resultsText(t, path)
		report, err := k.Post(context.Background(), path)
		if err != nil || report.Students[0].Status == "completed" || report.HeldVisible != 1 || k.count("put") != 0 {
			t.Fatalf("%s: grade written or not held: %v %#v puts=%d", posted, err, report, k.count("put"))
		}
		if !strings.Contains(report.Students[0].Message, "Ms. Turner can change it in SpeedGrader") {
			t.Fatalf("%s: message %q", posted, report.Students[0].Message)
		}
		if !strings.Contains(resultsText(t, path), "explain why this evidence supports the claim") || len(before) == 0 {
			t.Fatalf("%s: scoring text was lost", posted)
		}
		if line := report.OwnerSummary(); !strings.Contains(line, "0 scored") || !strings.Contains(line, "1 held (already visible)") {
			t.Fatalf("%s: summary %q", posted, line)
		}
	}
}

func TestOwnedPostWithTagRemovedHoldsEverythingAndSendsNothing(t *testing.T) {
	k := consentFixture(t, "Essay", "hidden")
	path := scoringFiles(t, k.Service, 5)
	report, err := k.Post(context.Background(), path)
	if err != nil || report.Hidden || k.count("policy") != 0 || k.count("put") != 0 {
		t.Fatalf("untagged post wrote: %v %#v %v", err, report, k.events)
	}
	if m := report.Students[0].Message; !strings.HasPrefix(m, "not tagged (CE) in Canvas right now; nothing posted") {
		t.Fatalf("message %q", m)
	}
	if !strings.Contains(resultsText(t, path), "explain why this evidence supports the claim") {
		t.Fatal("scoring text was lost")
	}
}

func TestOwnedPostWithoutConfirmedPolicyHoldsEverything(t *testing.T) {
	k := consentFixture(t, "Essay (CE)", "hidden", "errors")
	path := scoringFiles(t, k.Service, 5)
	report, err := k.Post(context.Background(), path)
	if err != nil || report.Hidden || k.count("put") != 0 || k.count("policy") != 1 {
		t.Fatalf("unconfirmed policy still wrote: %v %#v %v", err, report, k.events)
	}
	if m := report.Students[0].Message; !strings.HasPrefix(m, "couldn't set Canvas to hide these grades; nothing posted") {
		t.Fatalf("message %q", m)
	}
}

func TestUncertainPolicyIsRetriedOnceThenPostingProceeds(t *testing.T) {
	k := consentFixture(t, "Essay (CE)", "hidden", "500", "ok")
	report, err := k.Post(context.Background(), scoringFiles(t, k.Service, 5))
	if err != nil || !report.Hidden || k.count("policy") != 2 || k.first("put") < 0 {
		t.Fatalf("retry: %v %#v %v", err, report, k.events)
	}
	if k.events[0] != "policy" || k.events[1] != "policy" {
		t.Fatalf("both policy attempts must precede any other call: %v", k.events)
	}
}

func TestProbeStyleAndCoTeachingServicesStillRequireTagAndPolicy(t *testing.T) {
	for name, keysNil := range map[string]bool{"keys nil": true, "co-teaching": false} {
		k := consentFixture(t, "Essay (CE)", "hidden")
		k.coTeach = true // your own key is routed
		if keysNil {
			k.Keys = nil
		}
		report, err := k.Post(context.Background(), scoringFiles(t, k.Service, 5))
		if err != nil || !report.Hidden || k.count("policy") != 1 || k.first("policy") > k.first("put") {
			t.Fatalf("%s: %v %#v %v", name, err, report, k.events)
		}
		untagged := consentFixture(t, "Essay", "hidden")
		untagged.coTeach = true
		if keysNil {
			untagged.Keys = nil
		}
		if _, err := untagged.Post(context.Background(), scoringFiles(t, untagged.Service, 5)); err != nil || untagged.count("put") != 0 || untagged.count("policy") != 0 {
			t.Fatalf("%s untagged: %v %v", name, err, untagged.events)
		}
	}
}

func TestOwnerMissingFromTeamIsAnInputErrorWithNoWrites(t *testing.T) {
	k := consentFixture(t, "Essay (CE)", "hidden")
	path := scoringFiles(t, k.Service, 5)
	k.Team = workspace.TeamConfig{Schema: 1}
	if _, err := k.Post(context.Background(), path); err == nil || !strings.Contains(err.Error(), `owner "turner"`) {
		t.Fatalf("err %v", err)
	}
	if k.count("put")+k.count("policy") != 0 {
		t.Fatalf("writes were sent: %v", k.events)
	}
}

func TestYourOwnCoursePostsWithoutPolicyChange(t *testing.T) {
	k := consentFixture(t, "Evidence essay", "absent")
	k.Config.Courses[0].Owner = ""
	k.Keys = nil
	k.coTeach = true // the fixture then accepts your key for course 1
	report, err := k.Post(context.Background(), scoringFiles(t, k.Service, 5))
	if err != nil || report.Students[0].Status != "completed" || k.count("policy") != 0 || report.Owner != "" || report.OwnerSummary() != "" {
		t.Fatalf("own course: %v %#v %v", err, report, k.events)
	}
	owned := consentFixture(t, "Essay (CE)", "hidden")
	if _, err := owned.Post(context.Background(), scoringFiles(t, owned.Service, 5)); err != nil || owned.count("get-submission") != k.count("get-submission") {
		t.Fatalf("owned and own posts should read submissions alike: %v %v vs %v", err, owned.events, k.events)
	}
}

func TestOwnedLateOnlyIsHeldWhenPostedWithoutAnExtraRead(t *testing.T) {
	due := "2026-10-08T18:00:00Z"
	for _, posted := range []string{"visible", "hidden"} {
		var mu sync.Mutex
		puts, gets, policies := 0, 0, 0
		k := newTeamKit(t, func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			defer mu.Unlock()
			switch {
			case r.URL.Path == "/api/graphql":
				policies++
				_, _ = w.Write([]byte(`{"data":{"setAssignmentPostPolicy":{"postPolicy":{"postManually":true},"errors":[]}}}`))
			case strings.HasSuffix(r.URL.Path, "/submissions/5") && r.Method == http.MethodPut:
				puts++
				_, _ = w.Write([]byte(`{}`))
			case strings.HasSuffix(r.URL.Path, "/submissions/5"):
				gets++
				body := map[string]any{"id": 12, "user_id": 5, "assignment_id": 10, "attempt": 2, "submitted_at": "2026-10-07T12:00:00Z", "workflow_state": "submitted", "submission_type": "online_text_entry", "cached_due_date": due, "late": false, "late_policy_status": nil, "seconds_late": 0, "excused": false, "assignment_visible": true, "submission_history": []any{}, "posted_at": nil}
				if posted == "visible" {
					body["posted_at"] = "2026-10-20T12:00:00Z"
				}
				_ = json.NewEncoder(w).Encode(body)
			default:
				a := fixtureAssignment()
				a.Name = "Essay (CE)"
				_ = json.NewEncoder(w).Encode(a)
			}
		}, []domain.Course{{Nickname: "TEST", ID: 1, Current: true, Owner: "turner"}}, 1, true)
		path := scoringFiles(t, k.Service, 5)
		var doc domain.ResultsDocument
		body, _ := workspace.ReadDocument(path, &doc)
		var packet domain.PacketDocument
		_, _ = workspace.ReadDocument(filepath.Join(k.Root, "courses", "TEST", "packet.md"), &packet)
		var work domain.StudentWorkDocument
		workPath := filepath.Join(k.Root, filepath.FromSlash(packet.Students[0]))
		_, _ = workspace.ReadDocument(workPath, &work)
		work.Current.ObservedTiming = &domain.ObservedTiming{StatusKnown: true, LateKnown: true, SecondsLateKnown: true}
		if err := workspace.WriteDocument(workPath, work, "work"); err != nil {
			t.Fatal(err)
		}
		doc.Students[0] = domain.StudentResult{StudentRef: doc.Students[0].StudentRef, Operation: "late-only", LateOnly: &domain.LateOnlyOperation{
			Attempt:    work.Current.Ref,
			LateTiming: domain.LateTiming{Status: "late", SecondsLate: 3600, DueAt: due, Basis: "one hour late", Observed: work.Current.ObservedTiming},
		}}
		if err := workspace.WriteDocument(path, &doc, body); err != nil {
			t.Fatal(err)
		}
		report, err := k.Post(context.Background(), path)
		if err != nil || gets != 1 || policies != 1 {
			t.Fatalf("%s: %v gets=%d policies=%d", posted, err, gets, policies)
		}
		if posted == "visible" && (puts != 0 || report.Students[0].Status == "completed" || report.HeldVisible != 1) {
			t.Fatalf("visible late-only row was written: %#v puts=%d", report, puts)
		}
		if posted == "hidden" && (puts != 1 || report.Students[0].Status != "completed") {
			t.Fatalf("hidden late-only row was not written: %#v puts=%d", report, puts)
		}
	}
}

func ownTurnerCourse(env *groupPostEnv) {
	env.s.Config.Courses[0].Owner = "turner"
	env.s.Team = workspace.TeamConfig{Schema: 1, Teammates: []workspace.Teammate{{Label: "turner", Name: "Ms. Turner", CanvasUserID: 77, KeyExpires: "2099-01-01"}}}
}

func TestOwnedGroupPostHoldsAGroupWithAVisibleMemberAndPostsOtherwise(t *testing.T) {
	for _, visible := range []bool{true, false} {
		env := newGroupPostEnv(t, false)
		ownTurnerCourse(env)
		env.liveName = "Essay (CE)"
		env.visible = map[int64]bool{11: visible}
		report, err := env.s.PostGroupResults(context.Background(), env.results)
		if err != nil || !report.Hidden || env.policies != 1 {
			t.Fatalf("visible=%v: %v %#v policies=%d", visible, err, report, env.policies)
		}
		if visible {
			// Only a private comment to a member whose own grade is hidden may go out.
			for _, p := range env.puts {
				if c, ok := commentOf(p); !ok || c["group_comment"] != false || p.user == 11 {
					t.Fatalf("shared work was written for a group with a visible member: %+v", p)
				}
			}
			if !strings.Contains(env.reload(t).Grade.Outcomes.Grade.Message, "a group member's grade is already visible") {
				t.Fatalf("shared grade not held for the right reason: %+v", env.reload(t).Grade.Outcomes)
			}
		} else if len(env.puts) == 0 || env.firstPutBeforePolicy {
			t.Fatalf("group was not posted, or posted before the policy: %+v", env.puts)
		}
	}
}

func TestOwnedGroupPostWithTagRemovedHoldsEverything(t *testing.T) {
	env := newGroupPostEnv(t, false)
	ownTurnerCourse(env)
	report, err := env.s.PostGroupResults(context.Background(), env.results) // live name has no tag
	if err != nil || report.Hidden || env.policies != 0 || len(env.puts) != 0 {
		t.Fatalf("untagged group post wrote: %v %#v", err, report)
	}
	if got := env.reload(t).Grade.Outcomes.Grade.Message; got != "not tagged (CE) in Canvas right now; nothing posted" {
		t.Fatalf("message %q", got)
	}
}

func TestOwnedPullRequiresTheTag(t *testing.T) {
	var mu sync.Mutex
	var paths []string
	named := func(id int64, name string) map[string]any {
		raw, _ := json.Marshal(fixtureAssignment())
		var m map[string]any
		_ = json.Unmarshal(raw, &m)
		m["id"], m["name"] = id, name
		return m
	}
	k := newTeamKit(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		switch {
		case strings.HasSuffix(r.URL.Path, "/enrollments"):
			_, _ = w.Write([]byte(`[{"user_id":5,"type":"StudentEnrollment","enrollment_state":"active","user":{"id":5,"name":"Jordan Sample"}}]`))
		case strings.HasSuffix(r.URL.Path, "/assignments"):
			_ = json.NewEncoder(w).Encode([]any{named(10, "Essay 3 (CE)"), named(11, "Essay 4"), named(12, "Quiz")})
		case strings.HasSuffix(r.URL.Path, "/assignments/11"):
			_ = json.NewEncoder(w).Encode(named(11, "Essay 4"))
		default:
			_, _ = w.Write([]byte(`[]`))
		}
	}, []domain.Course{{Nickname: "TEST", ID: 1, Current: true, Owner: "turner"}}, 1, true)
	_, err := k.PullWork(context.Background(), "TEST", "11")
	if err == nil || err.Error() != "Essay 4 in TEST isn't tagged (CE), so Ms. Turner hasn't opted it in to CE-SE scoring. Ask Ms. Turner to add (CE) to its name in Canvas." {
		t.Fatalf("explicit pull: %v", err)
	}
	mu.Lock()
	for _, p := range paths {
		if strings.Contains(p, "/submissions") {
			t.Fatalf("submissions were read for an untagged assignment: %s", p)
		}
	}
	mu.Unlock()
	report, err := k.PullWorkWithOptions(context.Background(), "TEST", "", PullWorkOptions{Session: true})
	if err != nil || len(report.Assignments) != 1 || report.Assignments[0].AssignmentID != 10 {
		t.Fatalf("session pull: %v %#v", err, report.Assignments)
	}
	if len(report.Warnings) != 1 || report.Warnings[0] != "TEST: 2 assignments not tagged (CE) were skipped." {
		t.Fatalf("warnings %v", report.Warnings)
	}
	mu.Lock()
	defer mu.Unlock()
	for _, p := range paths {
		if strings.Contains(p, "/assignments/11/") || strings.Contains(p, "/assignments/12/") {
			t.Fatalf("an untagged assignment's submissions were read: %s", p)
		}
	}
}

func TestReviewStatesWhereOwnedScoresLand(t *testing.T) {
	k := consentFixture(t, "Essay (CE)", "hidden")
	path := scoringFiles(t, k.Service, 5)
	if err := workspace.WriteTeacherDocument(k.Root, "context/courses.md", k.Config, ""); err != nil {
		t.Fatal(err)
	}
	if err := workspace.WriteTeam(k.Root, k.Team); err != nil {
		t.Fatal(err)
	}
	m, err := BuildReview(k.Root, path) // the saved title, "Evidence essay", has no tag
	if err != nil || m.Notice != "Not tagged (CE) in Canvas right now. Approving will post nothing." {
		t.Fatalf("untagged notice: %v %q", err, m.Notice)
	}
	var doc domain.ResultsDocument
	body, _ := workspace.ReadDocument(path, &doc)
	doc.Assignment.Title = "Evidence essay (CE)"
	if err := workspace.WriteDocument(path, &doc, body); err != nil {
		t.Fatal(err)
	}
	m, err = BuildReview(k.Root, path)
	if err != nil || m.Notice != "Ms. Turner's course. Scores go in hidden; Ms. Turner posts them in Canvas." {
		t.Fatalf("tagged notice: %v %q", err, m.Notice)
	}
}
