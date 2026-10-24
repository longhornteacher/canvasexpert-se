package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

type groupPostEnv struct {
	server   *httptest.Server
	s        *Service
	results  string
	doc      domain.GroupResultsDocument
	mu       sync.Mutex
	puts     []groupPut
	audience []string // live group_members ids
	failText string   // comment text whose write should fail with 400
	attempt  map[int64]int
	timing   map[int64]map[string]any
	shared   *bool // live grade_group_students_individually, as Canvas currently reports it
	// Teammate-course checks: the live assignment name, members whose grade
	// Canvas already shows, post-policy calls seen, and whether a grade write
	// preceded the first one.
	liveName             string
	visible              map[int64]bool
	policies             int
	firstPutBeforePolicy bool
}

type groupPut struct {
	user int64
	body map[string]any
}

func (e *groupPostEnv) putsTo(user int64) []groupPut {
	var out []groupPut
	for _, p := range e.puts {
		if p.user == user {
			out = append(out, p)
		}
	}
	return out
}

// newGroupPostEnv pulls a shared three-member group through the real pull code
// and returns a group-results document for it.
func newGroupPostEnv(t *testing.T, coupled bool) *groupPostEnv {
	return newGroupEnv(t, coupled, false)
}

func newGroupEnv(t *testing.T, coupled, individual bool) *groupPostEnv {
	t.Helper()
	env := &groupPostEnv{liveName: "Evidence essay", audience: []string{"10", "11", "12"}, attempt: map[int64]int{10: 1, 11: 1, 12: 1}, timing: map[int64]map[string]any{}}
	setID := int64(50)
	mode := individual
	env.shared = &mode
	a := fixtureAssignment()
	a.GroupCategoryID = &setID
	a.GradeGroupStudentsIndividually = &mode
	a.SubmissionTypes = []string{"online_text_entry"}
	if coupled {
		use := true
		a.RubricAssociation.UseForGrading = &use
	}
	s, server := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		write := func(v any) { _ = json.NewEncoder(w).Encode(v) }
		sub := func(user int64) map[string]any {
			result := map[string]any{"id": 700 + user, "user_id": user, "assignment_id": 10, "group_id": 60, "attempt": env.attempt[user], "submitted_at": "2026-10-16T18:00:00Z", "workflow_state": "submitted", "submission_type": "online_text_entry", "body": "<p>Our club's shared interpretation.</p>", "cached_due_date": "2026-10-15T12:00:00Z", "late": false, "late_policy_status": nil, "seconds_late": 0, "assignment_visible": true, "posted_at": nil}
			if env.visible[user] {
				result["posted_at"] = "2026-10-20T12:00:00Z"
			}
			for key, value := range env.timing[user] {
				result[key] = value
			}
			return result
		}
		path := r.URL.Path
		switch {
		case path == "/api/graphql":
			env.mu.Lock()
			env.policies++
			env.firstPutBeforePolicy = env.firstPutBeforePolicy || (env.policies == 1 && len(env.puts) > 0)
			env.mu.Unlock()
			write(map[string]any{"data": map[string]any{"setAssignmentPostPolicy": map[string]any{"postPolicy": map[string]any{"postManually": true}, "errors": []any{}}}})
		case path == "/api/v1/courses/1/assignments/10":
			live := a
			live.Name = env.liveName
			live.GradeGroupStudentsIndividually = env.shared
			write(live)
		case path == "/api/v1/group_categories/50":
			write(map[string]any{"id": 50, "course_id": 1, "context_type": "Course", "name": "Clubs", "non_collaborative": false})
		case path == "/api/v1/groups/60":
			write(map[string]any{"id": 60, "course_id": 1, "context_type": "Course", "group_category_id": 50, "name": "Blue", "non_collaborative": false})
		case path == "/api/v1/groups/60/memberships":
			write([]any{map[string]any{"group_id": 60, "user_id": 10, "workflow_state": "accepted"}, map[string]any{"group_id": 60, "user_id": 11, "workflow_state": "accepted"}, map[string]any{"group_id": 60, "user_id": 12, "workflow_state": "accepted"}})
		case path == "/api/v1/courses/1/enrollments":
			rows := []any{}
			for id := int64(10); id <= 13; id++ {
				rows = append(rows, map[string]any{"user_id": id, "type": "StudentEnrollment", "enrollment_state": "active", "user": map[string]any{"id": id, "name": fmt.Sprintf("Private Student %d", id), "short_name": fmt.Sprintf("Student %d", id)}})
			}
			write(rows)
		case path == "/api/v1/courses/1/assignment_groups":
			write([]any{map[string]any{"id": 1, "name": "Daily", "group_weight": 30}})
		case strings.HasSuffix(path, "/group_members"):
			rows := []any{}
			for _, id := range env.audience {
				rows = append(rows, map[string]any{"id": id})
			}
			write(rows)
		case strings.Contains(path, "/submissions/"):
			var user int64
			_, _ = fmt.Sscanf(path[strings.LastIndex(path, "/")+1:], "%d", &user)
			if r.Method == http.MethodPut {
				raw, _ := io.ReadAll(r.Body)
				var body map[string]any
				_ = json.Unmarshal(raw, &body)
				env.mu.Lock()
				env.puts = append(env.puts, groupPut{user: user, body: body})
				env.mu.Unlock()
				if c, ok := body["comment"].(map[string]any); ok && env.failText != "" && c["text_comment"] == env.failText {
					http.Error(w, `{"errors":"rejected"}`, 400)
					return
				}
			}
			write(sub(user))
		case path == "/api/v1/courses/1/assignments/10/submissions":
			write([]any{sub(10), sub(11), sub(12)})
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.String())
			http.NotFound(w, r)
		}
	})
	env.s, env.server = s, server
	s.Skip = map[string]bool{}
	store, _ := workspace.OpenStudents(s.Root)
	for id := int64(10); id <= 13; id++ {
		_, _ = store.Upsert(s.Client.Origin(), id, fmt.Sprintf("Private Student %d", id), fmt.Sprintf("Student %d", id), "", "")
	}
	_ = store.Save()
	report, err := s.PullWorkGroup(context.Background(), "TEST", "10", 60)
	if err != nil || len(report.Packets) != 1 {
		t.Fatalf("pull report=%+v err=%v", report, err)
	}
	var packet domain.GroupPacketDocument
	if _, err = workspace.ReadDocument(report.Packets[0], &packet); err != nil {
		t.Fatal(err)
	}
	var work domain.GroupWorkDocument
	if _, err = workspace.ReadDocument(filepath.Join(s.Root, filepath.FromSlash(packet.Groups[0].Path)), &work); err != nil {
		t.Fatal(err)
	}
	if (work.SharedAssessmentStatus != "ready" && !individual) || len(work.Artifacts) != 1 {
		t.Fatalf("pulled group is not ready: %+v", work)
	}
	refs := append([]string(nil), work.PropagationAudience...)
	if individual && work.SharedAssessmentStatus != "not-applicable" {
		t.Fatalf("individual pull status %s", work.SharedAssessmentStatus)
	}
	rel, _ := filepath.Rel(s.Root, report.Packets[0])
	snapshot, err := Snapshot(a, s.Client.Origin())
	if err != nil {
		t.Fatal(err)
	}
	score := domain.CriterionScore{CriterionID: "c1", Points: domain.MustDecimal("80")}
	env.doc = domain.GroupResultsDocument{Schema: 1, Kind: "group-results", Packet: filepath.ToSlash(rel), Assignment: snapshot,
		Interpretation: domain.Interpretation{Mapping: "repeated", Rationale: "One shared product.", Components: []domain.Component{{Key: "answer", Label: "Answer", Weight: domain.MustDecimal("1")}}},
		Groups: []domain.GroupResult{{GroupID: 60, ArtifactKey: work.Artifacts[0].Key, MemberRefs: refs, Operation: "grade",
			Grade:              &domain.GroupGradeOperation{Components: []domain.ComponentScore{{Key: "answer", Evidence: "present", Criteria: []domain.CriterionScore{score}}}, FeedbackAudience: "group", Feedback: "Your interpretation connects a claim to evidence."},
			IndividualFeedback: []domain.IndividualFeedback{{StudentRef: refs[0], Text: "PRIVATE-SENTINEL your next question is strong."}}}}}
	if individual {
		row := &env.doc.Groups[0]
		row.Operation, row.Grade, row.IndividualFeedback = "individual", nil, nil
		for i, ref := range refs {
			pts := []string{"100", "60", "80"}[i]
			row.Students = append(row.Students, domain.StudentResult{StudentRef: ref, Operation: "grade", Grade: &domain.GradeOperation{
				Attempt:    domain.AttemptRef{Attempt: 1, SubmittedAt: "2026-10-16T18:00:00Z"},
				Components: []domain.ComponentScore{{Key: "answer", Evidence: "present", Criteria: []domain.CriterionScore{{CriterionID: "c1", Points: domain.MustDecimal(pts)}}}},
				Feedback:   "PRIVATE-" + ref + " your own reasoning."}})
		}
	}
	env.results = filepath.Join(s.Root, "courses", "TEST", "group-results.md")
	env.save(t)
	return env
}

func (e *groupPostEnv) save(t *testing.T) {
	t.Helper()
	if err := workspace.WriteDocument(e.results, &e.doc, "results"); err != nil {
		t.Fatal(err)
	}
}

func (e *groupPostEnv) reload(t *testing.T) domain.GroupResult {
	t.Helper()
	var doc domain.GroupResultsDocument
	if _, err := workspace.ReadDocument(e.results, &doc); err != nil {
		t.Fatal(err)
	}
	return doc.Groups[0]
}

func commentOf(p groupPut) (map[string]any, bool) {
	c, ok := p.body["comment"].(map[string]any)
	return c, ok
}

func TestPostGroupResultsSendsOneSharedSequenceAndScopedComments(t *testing.T) {
	env := newGroupPostEnv(t, false)
	if _, err := env.s.PostGroupResults(context.Background(), env.results); err != nil {
		t.Fatal(err)
	}
	row := env.reload(t)
	if !groupComplete(row) || row.Representative == "" {
		t.Fatalf("group did not complete: %+v", row)
	}
	var shared, private, gradeWrites int
	for _, p := range env.puts {
		if c, ok := commentOf(p); ok {
			text, _ := c["text_comment"].(string)
			if c["group_comment"] == true {
				shared++
				if strings.Contains(text, "PRIVATE-SENTINEL") {
					t.Fatalf("private text reached the shared comment")
				}
			} else if c["group_comment"] == false {
				private++
				if !strings.Contains(text, "PRIVATE-SENTINEL") {
					t.Fatalf("private comment text was not sent: %q", text)
				}
			} else {
				t.Fatalf("comment without explicit group_comment: %+v", c)
			}
			continue
		}
		gradeWrites++
		if _, rubric := p.body["rubric_assessment"]; !rubric {
			t.Fatalf("grade write lacks rubric: %+v", p.body)
		}
	}
	if shared != 1 || private != 1 || gradeWrites != 1 {
		t.Fatalf("expected one grade/rubric, one shared and one private write; got %d/%d/%d: %+v", gradeWrites, shared, private, env.puts)
	}
	// An unrelated member never receives the private text.
	before := len(env.puts)
	if _, err := env.s.PostGroupResults(context.Background(), env.results); err != nil || len(env.puts) != before {
		t.Fatalf("completed work was replayed: err=%v puts=%d->%d", err, before, len(env.puts))
	}
}

func TestPostGroupResultsCoupledRubricPostsRubricThenGrade(t *testing.T) {
	env := newGroupPostEnv(t, true)
	if _, err := env.s.PostGroupResults(context.Background(), env.results); err != nil {
		t.Fatal(err)
	}
	var order []string
	for _, p := range env.puts {
		if _, ok := commentOf(p); ok {
			continue
		}
		if _, rubric := p.body["rubric_assessment"]; rubric {
			if _, graded := p.body["submission"]; graded {
				t.Fatalf("coupled rubric and grade were sent together")
			}
			order = append(order, "rubric")
		} else {
			order = append(order, "grade")
		}
	}
	if strings.Join(order, ",") != "rubric,grade" || !groupComplete(env.reload(t)) {
		t.Fatalf("order=%v", order)
	}
}

func TestPostGroupResultsHoldsChangedAudienceOrSubsetWithoutWriting(t *testing.T) {
	env := newGroupPostEnv(t, false)
	env.audience = []string{"10", "11", "12", "13"}
	if _, err := env.s.PostGroupResults(context.Background(), env.results); err != nil {
		t.Fatal(err)
	}
	for _, p := range env.puts {
		if _, ok := commentOf(p); !ok {
			t.Fatalf("a propagated write was sent for a changed audience: %+v", p)
		}
	}
	if row := env.reload(t); row.Grade.Outcomes.Grade.Status != "held" {
		t.Fatalf("grade was not held: %+v", row.Grade.Outcomes)
	}

	subset := newGroupPostEnv(t, false)
	subset.doc.Groups[0].MemberRefs = subset.doc.Groups[0].MemberRefs[:2]
	subset.save(t)
	if _, err := subset.s.PostGroupResults(context.Background(), subset.results); err != nil {
		t.Fatal(err)
	}
	for _, p := range subset.puts {
		if _, ok := commentOf(p); !ok {
			t.Fatalf("a subset request became a group grade: %+v", p)
		}
	}
}

func TestPostGroupResultsHoldsChangedAttempt(t *testing.T) {
	env := newGroupPostEnv(t, false)
	env.attempt[11] = 2
	if _, err := env.s.PostGroupResults(context.Background(), env.results); err != nil {
		t.Fatal(err)
	}
	for _, p := range env.puts {
		if _, ok := commentOf(p); !ok {
			t.Fatalf("grade written despite a new member attempt")
		}
	}
	if status := env.reload(t).Grade.Outcomes.Grade.Status; status != "held" {
		t.Fatalf("status=%s", status)
	}
}

func TestPostGroupLateOnlyRenewsFullAudienceAndWritesTimingOnce(t *testing.T) {
	env := newGroupPostEnv(t, false)
	prepareGroupLateOnly(t, env)
	if _, err := env.s.PostGroupResults(context.Background(), env.results); err != nil {
		t.Fatal(err)
	}
	if len(env.puts) != 1 || len(env.putsTo(10))+len(env.putsTo(11))+len(env.putsTo(12)) != 1 {
		t.Fatalf("shared timing should make one write for the representative: %+v", env.puts)
	}
	fields, ok := env.puts[0].body["submission"].(map[string]any)
	if !ok || len(fields) != 2 || fields["late_policy_status"] != "late" || fields["seconds_late_override"] != float64(3600) {
		t.Fatalf("unexpected shared timing patch: %#v", env.puts[0].body)
	}
}

func TestPostGroupLateOnlyHoldsWhenAudienceIsIncomplete(t *testing.T) {
	env := newGroupPostEnv(t, false)
	prepareGroupLateOnly(t, env)
	env.audience = []string{"10", "11"}
	if report, err := env.s.PostGroupResults(context.Background(), env.results); err != nil || len(env.puts) != 0 || len(report.Groups) != 1 || report.Groups[0].Outcomes.Late.Status != "held" {
		t.Fatalf("incomplete live group audience should hold the one shared action: report=%+v puts=%+v err=%v", report, env.puts, err)
	}
}

func TestPostGroupLateOnlyReconcilesUncertainWriteWhenEveryMemberMatches(t *testing.T) {
	env := newGroupPostEnv(t, false)
	prepareGroupLateOnly(t, env)
	row := &env.doc.Groups[0]
	row.LateOnly.Outcome = domain.ActionOutcome{Status: "uncertain"}
	// All three live submissions show the accepted target, but the earlier
	// write response was ambiguous. Reconcile without sending another PUT.
	for user := int64(10); user <= 12; user++ {
		env.timing[user] = map[string]any{"late": true, "late_policy_status": "late", "seconds_late": 3600}
	}
	env.save(t)
	report, err := env.s.PostGroupResults(context.Background(), env.results)
	if err != nil || len(env.puts) != 0 || len(report.Groups) != 1 || report.Groups[0].Outcomes.Late.Status != "preserved" {
		t.Fatalf("accepted uncertain write should reconcile only when all members match: report=%+v puts=%+v err=%v", report, env.puts, err)
	}
}

func TestSharedGradeTimingHoldDoesNotBlockAcademicActions(t *testing.T) {
	env := newGroupPostEnv(t, false)
	var packet domain.GroupPacketDocument
	if _, err := workspace.ReadDocument(filepath.Join(env.s.Root, filepath.FromSlash(env.doc.Packet)), &packet); err != nil {
		t.Fatal(err)
	}
	var work domain.GroupWorkDocument
	if _, err := workspace.ReadDocument(filepath.Join(env.s.Root, filepath.FromSlash(packet.Groups[0].Path)), &work); err != nil {
		t.Fatal(err)
	}
	var baseline *domain.ObservedTiming
	for _, member := range work.Members {
		if member.StudentRef == env.doc.Groups[0].Representative || baseline == nil {
			baseline = member.ObservedTiming
		}
	}
	if baseline == nil {
		t.Fatal("group pull has no timing baseline")
	}
	env.doc.Groups[0].Grade.LateTiming = &domain.LateTiming{Status: "late", SecondsLate: 3600, DueAt: "2026-10-15T12:00:00Z", Basis: "one elapsed hour late", Observed: baseline}
	env.timing[11] = map[string]any{"late": true, "late_policy_status": "late", "seconds_late": 60}
	env.save(t)
	report, err := env.s.PostGroupResults(context.Background(), env.results)
	if err != nil || len(report.Groups) != 1 {
		t.Fatalf("post report=%+v err=%v", report, err)
	}
	outcomes := report.Groups[0].Outcomes
	if !definite(outcomes.Rubric) || !definite(outcomes.Grade) || !definite(outcomes.Comment) || outcomes.Late.Status != "held" {
		t.Fatalf("per-member timing change should hold only lateness: %+v", outcomes)
	}
}

func TestPostGroupIndividualLateOnlyHoldsOnlyMemberWithNewAttempt(t *testing.T) {
	env := newGroupEnv(t, false, true)
	var packet domain.GroupPacketDocument
	if _, err := workspace.ReadDocument(filepath.Join(env.s.Root, filepath.FromSlash(env.doc.Packet)), &packet); err != nil {
		t.Fatal(err)
	}
	var work domain.GroupWorkDocument
	if _, err := workspace.ReadDocument(filepath.Join(env.s.Root, filepath.FromSlash(packet.Groups[0].Path)), &work); err != nil {
		t.Fatal(err)
	}
	byRef := map[string]domain.GroupWorkMember{}
	for _, member := range work.Members {
		byRef[member.StudentRef] = member
	}
	row := &env.doc.Groups[0]
	row.Operation, row.Grade, row.IndividualFeedback = "individual", nil, nil
	row.Students = nil
	for _, ref := range row.MemberRefs {
		member := byRef[ref]
		row.Students = append(row.Students, domain.StudentResult{StudentRef: ref, Operation: "late-only", LateOnly: &domain.LateOnlyOperation{
			Attempt:    domain.AttemptRef{Attempt: member.Attempt, SubmittedAt: member.SubmittedAt},
			LateTiming: domain.LateTiming{Status: "late", SecondsLate: 3600, DueAt: member.EffectiveDueAt, Basis: "one elapsed hour late", Observed: member.ObservedTiming},
		}})
	}
	env.doc.Assignment.Title = ""
	env.doc.Assignment.Points = domain.Decimal{}
	env.doc.Assignment.Rubric = nil
	env.doc.ScoringBasis = nil
	env.attempt[11] = 2
	env.save(t)
	report, err := env.s.PostGroupResults(context.Background(), env.results)
	if err != nil {
		t.Fatal(err)
	}
	if len(env.puts) != 2 {
		t.Fatalf("only the two unchanged members should receive timing writes: %+v", env.puts)
	}
	statuses := map[string]string{}
	for _, student := range report.Groups[0].Students {
		statuses[student.StudentRef] = student.Status
	}
	held, completed := 0, 0
	for _, status := range statuses {
		if status == "held" {
			held++
		} else if status == "completed" {
			completed++
		}
	}
	if held != 1 || completed != 2 {
		t.Fatalf("individual timing hold was not scoped to one member: %+v", statuses)
	}
}

func prepareGroupLateOnly(t *testing.T, env *groupPostEnv) {
	t.Helper()
	var work domain.GroupWorkDocument
	var packet domain.GroupPacketDocument
	if _, err := workspace.ReadDocument(filepath.Join(env.s.Root, filepath.FromSlash(env.doc.Packet)), &packet); err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.ReadDocument(filepath.Join(env.s.Root, filepath.FromSlash(packet.Groups[0].Path)), &work); err != nil {
		t.Fatal(err)
	}
	baselines := map[string]domain.ObservedTiming{}
	for _, member := range work.Members {
		if member.ObservedTiming == nil {
			t.Fatalf("member %s has no pulled timing baseline", member.StudentRef)
		}
		baselines[member.StudentRef] = *member.ObservedTiming
	}
	row := &env.doc.Groups[0]
	row.Operation, row.Grade, row.IndividualFeedback = "late-only", nil, nil
	row.ArtifactKey = ""
	row.LateOnly = &domain.GroupLateOnly{LateTiming: domain.LateTiming{Status: "late", SecondsLate: 3600, DueAt: "2026-10-15T12:00:00Z", Basis: "one elapsed hour late"}, ObservedMembers: baselines}
	env.doc.Assignment.Title = ""
	env.doc.Assignment.Points = domain.Decimal{}
	env.doc.Assignment.Rubric = nil
	env.doc.ScoringBasis = nil
	env.save(t)
}

func TestPostGroupResultsSkipKeysSuppressExactActions(t *testing.T) {
	env := newGroupPostEnv(t, false)
	env.s.Skip = map[string]bool{GroupCardRef(60): true}
	if _, err := env.s.PostGroupResults(context.Background(), env.results); err != nil {
		t.Fatal(err)
	}
	for _, p := range env.puts {
		if c, ok := commentOf(p); !ok || c["group_comment"] != false {
			t.Fatalf("deselected group card still wrote shared work: %+v", p)
		}
	}
	privateWrites := len(env.puts)
	env.s.Skip = map[string]bool{GroupPrivateRef(60, env.doc.Groups[0].MemberRefs[0]): true}
	if _, err := env.s.PostGroupResults(context.Background(), env.results); err != nil {
		t.Fatal(err)
	}
	if len(env.puts) == privateWrites {
		t.Fatalf("shared work was not posted once its card was approved")
	}
	for _, p := range env.puts[privateWrites:] {
		if c, ok := commentOf(p); ok && c["group_comment"] == false {
			t.Fatalf("deselected private comment was posted")
		}
	}
}

func TestPostGroupResultsFailedCommentResumesWithoutRepeatingGrade(t *testing.T) {
	env := newGroupPostEnv(t, false)
	env.failText = "placeholder"
	text, err := GroupSharedText(env.doc, env.doc.Groups[0], func() domain.ValidatedGroup {
		v, e := domain.ValidateGroupResults(&env.doc)
		if e != nil {
			t.Fatal(e)
		}
		return v[0]
	}(), false)
	if err != nil {
		t.Fatal(err)
	}
	env.failText = text
	if _, err = env.s.PostGroupResults(context.Background(), env.results); err != nil {
		t.Fatal(err)
	}
	row := env.reload(t)
	if row.Grade.Outcomes.Grade.Status != "succeeded" || row.Grade.Outcomes.Comment.Status != "failed" {
		t.Fatalf("outcomes: %+v", row.Grade.Outcomes)
	}
	env.failText = ""
	gradeWritesBefore := 0
	for _, p := range env.puts {
		if _, ok := commentOf(p); !ok {
			gradeWritesBefore++
		}
	}
	if _, err = env.s.PostGroupResults(context.Background(), env.results); err != nil {
		t.Fatal(err)
	}
	gradeWritesAfter := 0
	for _, p := range env.puts {
		if _, ok := commentOf(p); !ok {
			gradeWritesAfter++
		}
	}
	if gradeWritesAfter != gradeWritesBefore || !groupComplete(env.reload(t)) {
		t.Fatalf("grade repeated or comment not completed: %d -> %d", gradeWritesBefore, gradeWritesAfter)
	}
}

func TestGroupResultsRequireKnownSharedMode(t *testing.T) {
	env := newGroupPostEnv(t, false)
	yes := true
	env.doc.Assignment.GradeGroupStudentsIndividually = &yes
	if _, err := domain.ValidateGroupResults(&env.doc); err == nil {
		t.Fatal("individual group grading must not validate as shared")
	}
	env.doc.Assignment.GradeGroupStudentsIndividually = nil
	if _, err := domain.ValidateGroupResults(&env.doc); err == nil {
		t.Fatal("unknown group mode must not validate as shared")
	}
}

func TestGroupResultsReviewCardsKeepPrivateTextSeparate(t *testing.T) {
	env := newGroupPostEnv(t, false)
	model, err := BuildReview(env.s.Root, env.results)
	if err != nil || len(model.Cards) != 2 {
		t.Fatalf("cards=%d err=%v", len(model.Cards), err)
	}
	shared, private := model.Cards[0], model.Cards[1]
	if shared.Ref != GroupCardRef(60) || strings.Contains(shared.Comment, "PRIVATE-SENTINEL") || shared.Hold != "" {
		t.Fatalf("shared card: %+v", shared)
	}
	if !strings.HasPrefix(private.Ref, "group:60:private:") || !strings.Contains(private.Comment, "PRIVATE-SENTINEL") {
		t.Fatalf("private card: %+v", private)
	}
}

func TestPostGroupResultsIndividualModeGradesEachMemberPrivately(t *testing.T) {
	env := newGroupEnv(t, false, true)
	if _, err := env.s.PostGroupResults(context.Background(), env.results); err != nil {
		t.Fatal(err)
	}
	if !groupComplete(env.reload(t)) {
		t.Fatalf("not complete: %+v", env.reload(t))
	}
	grades := map[string]bool{}
	routes := map[int64]bool{}
	for _, p := range env.puts {
		if c, ok := commentOf(p); ok {
			if c["group_comment"] != false {
				t.Fatalf("individual comment lacked explicit group_comment=false: %+v", c)
			}
			continue
		}
		routes[p.user] = true
		if sub, ok := p.body["submission"].(map[string]any); ok {
			grades[fmt.Sprint(sub["posted_grade"])] = true
		}
	}
	if len(routes) != 3 || len(grades) != 3 {
		t.Fatalf("expected three distinct grades on three routes: %v %v", routes, grades)
	}
}

func TestGroupIndividualFeedbackOnlyKeepsItsPrivateAudience(t *testing.T) {
	env := newGroupEnv(t, false, true)
	student := &env.doc.Groups[0].Students[0]
	student.Operation = "comment-only"
	student.CommentOnly = &domain.CommentOnlyOperation{Attempt: student.Grade.Attempt, Header: "none", Feedback: "Private feedback without scores."}
	student.Grade = nil
	env.save(t)
	model, err := BuildReview(env.s.Root, env.results)
	if err != nil || len(model.Cards) == 0 || model.Cards[0].Comment != student.CommentOnly.Feedback || model.Cards[0].Hold != "" || model.Cards[0].Total != "" {
		t.Fatalf("feedback-only member review: %+v %v", model, err)
	}
	if _, err = env.s.PostGroupResults(context.Background(), env.results); err != nil {
		t.Fatal(err)
	}
	var recipient int64
	for _, p := range env.puts {
		if c, ok := commentOf(p); ok && c["text_comment"] == student.CommentOnly.Feedback {
			if c["group_comment"] != false || recipient != 0 {
				t.Fatalf("feedback audience was not one private student: %+v", env.puts)
			}
			recipient = p.user
		}
	}
	if recipient == 0 || len(env.putsTo(recipient)) != 1 {
		t.Fatalf("feedback-only member received more than a comment: %+v", env.puts)
	}
}

func TestPostGroupResultsIndividualAttemptHoldsOnlyThatStudent(t *testing.T) {
	env := newGroupEnv(t, false, true)
	env.attempt[11] = 2
	if _, err := env.s.PostGroupResults(context.Background(), env.results); err != nil {
		t.Fatal(err)
	}
	if len(env.putsTo(11)) != 0 || len(env.putsTo(10)) == 0 || len(env.putsTo(12)) == 0 {
		t.Fatalf("a new attempt did not hold only that student: %+v", env.puts)
	}
}

func TestPostGroupResultsIndividualHoldsEveryoneWhenModeSwitchesToShared(t *testing.T) {
	env := newGroupEnv(t, false, true)
	shared := false
	env.shared = &shared
	if _, err := env.s.PostGroupResults(context.Background(), env.results); err != nil {
		t.Fatal(err)
	}
	if len(env.puts) != 0 {
		t.Fatalf("writes were sent after the live mode switched: %+v", env.puts)
	}
}

func TestPostGroupResultsIndividualSkipAndSharedFeedbackAndCompletion(t *testing.T) {
	env := newGroupEnv(t, false, true)
	row := &env.doc.Groups[0]
	row.SharedFeedback = &domain.GroupCommentOnly{Components: row.Students[0].Grade.Components, Feedback: "The club's discussion was thoughtful."}
	env.save(t)
	first, second := row.Students[0].StudentRef, row.Students[1].StudentRef
	env.s.Skip = map[string]bool{GroupStudentRef(60, second): true, GroupStudentRef(60, row.Students[2].StudentRef): true, GroupCardRef(60): true}
	if _, err := env.s.PostGroupResults(context.Background(), env.results); err != nil {
		t.Fatal(err)
	}
	snapshot, _ := Snapshot(fixtureAssignment(), env.s.Client.Origin())
	snapshot.GroupCategoryID = 50
	sub := Submission{Attempt: 1}
	at := "2026-10-16T18:00:00Z"
	sub.SubmittedAt = &at
	if !env.s.completedHint(10, first, sub, env.doc.Assignment) || env.s.completedHint(10, second, sub, env.doc.Assignment) {
		t.Fatalf("completion must follow the student: first hidden=%v second hidden=%v", env.s.completedHint(10, first, sub, env.doc.Assignment), env.s.completedHint(10, second, sub, env.doc.Assignment))
	}
	for _, p := range env.puts {
		if c, ok := commentOf(p); ok && c["group_comment"] == true {
			t.Fatalf("shared feedback posted although its card was deselected")
		}
	}
	env.s.Skip = map[string]bool{}
	if _, err := env.s.PostGroupResults(context.Background(), env.results); err != nil {
		t.Fatal(err)
	}
	shared := 0
	for _, p := range env.puts {
		if c, ok := commentOf(p); ok && c["group_comment"] == true {
			shared++
		}
	}
	if shared != 1 || !groupComplete(env.reload(t)) {
		t.Fatalf("shared feedback count=%d complete=%v", shared, groupComplete(env.reload(t)))
	}
}

func TestGroupResultsRejectsSharedRowsInIndividualMode(t *testing.T) {
	env := newGroupEnv(t, false, true)
	env.doc.Groups[0].Operation = "grade"
	if _, err := domain.ValidateGroupResults(&env.doc); err == nil {
		t.Fatal("a shared grade row validated against an individual-mode assignment")
	}
}
