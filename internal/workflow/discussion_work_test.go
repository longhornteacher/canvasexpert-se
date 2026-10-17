package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

type discussionEnv struct {
	s          *Service
	mu         sync.Mutex
	puts       []groupPut
	updatedAt  map[int64]string // entry id -> updated_at
	checkpoint bool
	otherLink  bool
}

// newDiscussionEnv serves a graded course discussion (topic 77, assignment 10)
// with one entry per student (5 and 6) and pulls it through the real pull.
func newDiscussionEnv(t *testing.T) (*discussionEnv, domain.PacketDocument, string) {
	t.Helper()
	env := &discussionEnv{updatedAt: map[int64]string{501: "2026-10-10T10:00:00Z", 502: "2026-10-10T11:00:00Z"}}
	a := fixtureAssignment()
	a.SubmissionTypes = []string{"discussion_topic"}
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		write := func(v any) { _ = json.NewEncoder(w).Encode(v) }
		sub := func(user int64) map[string]any {
			return map[string]any{"id": 700 + user, "user_id": user, "assignment_id": 10, "attempt": 1, "submitted_at": "2026-10-10T10:00:00Z", "workflow_state": "submitted", "submission_type": "discussion_topic", "cached_due_date": "2026-10-15T12:00:00Z",
				"user": map[string]any{"id": user, "name": fmt.Sprintf("Private Student %d", user), "short_name": fmt.Sprintf("Student %d", user)}}
		}
		path := r.URL.Path
		switch {
		case path == "/api/v1/courses/1/assignments" || path == "/api/v1/courses/1/assignments/10":
			live := map[string]any{}
			raw, _ := json.Marshal(a)
			_ = json.Unmarshal(raw, &live)
			live["discussion_topic"] = map[string]any{"id": 77}
			if env.checkpoint {
				live["has_sub_assignments"] = true
			}
			if path == "/api/v1/courses/1/assignments" {
				write([]any{live})
			} else {
				write(live)
			}
		case path == "/api/v1/courses/1/enrollments":
			write([]any{map[string]any{"user_id": 5, "type": "StudentEnrollment", "enrollment_state": "active", "user": map[string]any{"id": 5, "name": "Private Student 5", "short_name": "Student 5"}}, map[string]any{"user_id": 6, "type": "StudentEnrollment", "enrollment_state": "active", "user": map[string]any{"id": 6, "name": "Private Student 6", "short_name": "Student 6"}}})
		case path == "/api/v1/courses/1/assignments/10/submissions":
			write([]any{sub(5), sub(6)})
		case path == "/api/v1/courses/1/discussion_topics/77":
			linked := 10
			if env.otherLink {
				linked = 99
			}
			write(map[string]any{"id": 77, "assignment_id": linked, "title": "Week 1", "message": "<p>Prompt</p>"})
		case path == "/api/v1/courses/1/discussion_topics/77/view":
			write(map[string]any{"view": []any{
				map[string]any{"id": 501, "user_id": 5, "message": "<p>The first student's claim</p>", "created_at": "2026-10-10T10:00:00Z", "updated_at": env.updatedAt[501]},
				map[string]any{"id": 502, "user_id": 6, "message": "<p>The second student replies</p>", "created_at": "2026-10-10T11:00:00Z", "updated_at": env.updatedAt[502]},
			}, "new_entries": []any{}})
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
			}
			write(sub(user))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.String())
			http.NotFound(w, r)
		}
	})
	env.s = s
	report, err := s.PullWork(context.Background(), "TEST", "10")
	if err != nil || len(report.Packets) != 1 || report.Students != 2 {
		t.Fatalf("pull report=%+v err=%v", report, err)
	}
	var packet domain.PacketDocument
	if _, err = workspace.ReadDocument(report.Packets[0], &packet); err != nil {
		t.Fatal(err)
	}
	rel, _ := filepath.Rel(s.Root, report.Packets[0])
	return env, packet, filepath.ToSlash(rel)
}

func (e *discussionEnv) results(t *testing.T, packet domain.PacketDocument, packetRel string, scores []string) (string, []string) {
	t.Helper()
	doc := domain.ResultsDocument{Schema: 1, Kind: "results", Packet: packetRel, Assignment: packet.Assignment,
		Interpretation: domain.Interpretation{Mapping: "repeated", Rationale: "Discussion posts against the rubric.", Components: []domain.Component{{Key: "answer", Label: "Answer", Weight: domain.MustDecimal("1")}}}}
	refs := []string{}
	for i, rel := range packet.Students {
		var work domain.StudentWorkDocument
		if _, err := workspace.ReadDocument(filepath.Join(e.s.Root, filepath.FromSlash(rel)), &work); err != nil {
			t.Fatal(err)
		}
		if work.DiscussionEvidence == nil || work.DiscussionEvidence.Digest == "" || len(work.DiscussionEvidence.EntryRefs) != 1 {
			t.Fatalf("student work lacks own-entry evidence: %+v", work.DiscussionEvidence)
		}
		refs = append(refs, work.StudentRef)
		doc.Students = append(doc.Students, domain.StudentResult{StudentRef: work.StudentRef, Operation: "grade", Grade: &domain.GradeOperation{
			Attempt:    work.Current.Ref,
			Evidence:   &domain.EvidenceSelection{Digest: work.DiscussionEvidence.Digest, EntryRefs: work.DiscussionEvidence.EntryRefs},
			Components: []domain.ComponentScore{{Key: "answer", Evidence: "present", Criteria: []domain.CriterionScore{{CriterionID: "c1", Points: domain.MustDecimal(scores[i])}}}},
			Feedback:   "Your own reasoning stands out."}})
	}
	path := filepath.Join(e.s.Root, "courses", "TEST", "discussion-results.md")
	if err := workspace.WriteDocument(path, &doc, "results"); err != nil {
		t.Fatal(err)
	}
	return path, refs
}

func TestGradedDiscussionAssessesStudentsIndividuallyWithoutGroupComments(t *testing.T) {
	env, packet, rel := newDiscussionEnv(t)
	path, _ := env.results(t, packet, rel, []string{"100", "60"})
	if _, err := env.s.Post(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	grades := map[string]bool{}
	for _, p := range env.puts {
		if c, ok := commentOf(p); ok {
			if _, set := c["group_comment"]; set {
				t.Fatalf("discussion comment carried group_comment: %+v", c)
			}
			continue
		}
		if sub, ok := p.body["submission"].(map[string]any); ok {
			grades[fmt.Sprint(sub["posted_grade"])] = true
		}
	}
	if len(grades) != 2 {
		t.Fatalf("students should receive different grades: %v (%+v)", grades, env.puts)
	}
}

func TestGradedDiscussionChangedPostHoldsOnlyThatStudent(t *testing.T) {
	env, packet, rel := newDiscussionEnv(t)
	path, refs := env.results(t, packet, rel, []string{"100", "60"})
	env.updatedAt[501] = "2026-10-12T09:00:00Z" // the first student edited their post
	if _, err := env.s.Post(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	var doc domain.ResultsDocument
	if _, err := workspace.ReadDocument(path, &doc); err != nil {
		t.Fatal(err)
	}
	status := map[string]string{}
	for _, r := range doc.Students {
		status[r.StudentRef] = r.Grade.Outcomes.Grade.Status
	}
	held, posted := 0, 0
	for _, ref := range refs {
		if status[ref] == "held" {
			held++
		}
		if status[ref] == "succeeded" {
			posted++
		}
	}
	if held != 1 || posted != 1 {
		t.Fatalf("one changed post should hold one student only: %v", status)
	}
}

func TestGradedDiscussionCheckpointHoldsGradeButNotPull(t *testing.T) {
	env, packet, rel := newDiscussionEnv(t)
	path, _ := env.results(t, packet, rel, []string{"100", "60"})
	var doc domain.ResultsDocument
	if _, err := workspace.ReadDocument(path, &doc); err != nil {
		t.Fatal(err)
	}
	yes := true
	doc.Assignment.HasSubAssignments = &yes
	if err := workspace.WriteDocument(path, &doc, "results"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.s.Post(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	if len(env.puts) != 0 {
		t.Fatalf("a checkpointed discussion received writes: %+v", env.puts)
	}
}

func TestGradedDiscussionRelinkedTopicBlocksWrites(t *testing.T) {
	env, packet, rel := newDiscussionEnv(t)
	path, _ := env.results(t, packet, rel, []string{"100", "60"})
	env.otherLink = true
	if _, err := env.s.Post(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	if len(env.puts) != 0 {
		t.Fatalf("writes were sent although the topic no longer links to the assignment: %+v", env.puts)
	}
}

func TestGradedDiscussionResultsRequireEvidence(t *testing.T) {
	topic := int64(77)
	snap := domain.AssignmentSnapshot{DiscussionTopicID: &topic}
	if err := domain.ValidateResultsEvidence(snap, nil); err == nil {
		t.Fatal("a graded discussion result without evidence validated")
	}
}
