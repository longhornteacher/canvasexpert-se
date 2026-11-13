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

type discussionEnv struct {
	server     *httptest.Server
	s          *Service
	mu         sync.Mutex
	puts       []groupPut
	updatedAt  map[int64]string // entry id -> updated_at
	checkpoint bool
	otherLink  bool
	files      map[int64]map[string]any // entry id -> its attached file
	embed      bool                     // entry 501 embeds a course file in its text
}

// newDiscussionEnv serves a graded course discussion (topic 77, assignment 10)
// with one entry per student (5 and 6) and pulls it through the real pull.
func newDiscussionEnv(t *testing.T) (*discussionEnv, domain.PacketDocument, string) {
	t.Helper()
	return newDiscussionEnvWith(t, nil)
}

func newDiscussionEnvWith(t *testing.T, setup func(*discussionEnv)) (*discussionEnv, domain.PacketDocument, string) {
	t.Helper()
	env := &discussionEnv{updatedAt: map[int64]string{501: "2026-10-10T10:00:00Z", 502: "2026-10-10T11:00:00Z"}, files: map[int64]map[string]any{}}
	if setup != nil {
		setup(env)
	}
	a := fixtureAssignment()
	a.SubmissionTypes = []string{"discussion_topic"}
	s, server := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
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
			first := map[string]any{"id": 501, "user_id": 5, "message": "<p>The first student's claim</p>", "created_at": "2026-10-10T10:00:00Z", "updated_at": env.updatedAt[501]}
			if env.embed {
				first["message"] = `<p>The first student's claim <img src="/courses/1/files/950/preview"></p>`
			}
			second := map[string]any{"id": 502, "user_id": 6, "message": "<p>The second student replies</p>", "created_at": "2026-10-10T11:00:00Z", "updated_at": env.updatedAt[502]}
			for id, entry := range map[int64]map[string]any{501: first, 502: second} {
				if f := env.files[id]; f != nil {
					f["url"] = env.server.URL + "/files/entry"
					entry["attachments"] = []any{f}
				}
			}
			write(map[string]any{"view": []any{first, second}, "new_entries": []any{}})
		case path == "/api/v1/files/950":
			write(map[string]any{"id": 950, "filename": "diagram.png", "content-type": "image/png", "url": env.server.URL + "/files/diagram"})
		case strings.HasPrefix(path, "/files/"):
			_, _ = w.Write([]byte("%PDF-1.7 entry file"))
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
	env.s, env.server = s, server
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

func (e *discussionEnv) postStatuses(t *testing.T, path string) map[string]string {
	t.Helper()
	if _, err := e.s.Post(context.Background(), path); err != nil {
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
	return status
}

// A file attached to a post is part of the evidence: replacing it, or Canvas
// reporting a new version of it, holds that student only. A file embedded in
// the post's text is saved as the student's evidence, source embedded.
func TestGradedDiscussionReplacedAttachmentHoldsOnlyThatStudent(t *testing.T) {
	for name, change := range map[string]map[string]any{
		"replaced": {"id": 901, "filename": "essay-v2.pdf", "updated_at": "2026-10-12T09:00:00Z"},
		"new version": {"id": 900, "filename": "essay.pdf", "updated_at": "2026-10-12T09:00:00Z"},
	} {
		t.Run(name, func(t *testing.T) {
			env, packet, rel := newDiscussionEnvWith(t, func(env *discussionEnv) {
				env.embed = true
				env.files[501] = map[string]any{"id": 900, "filename": "essay.pdf", "updated_at": "2026-10-10T10:00:00Z"}
			})
			var work domain.StudentWorkDocument
			if _, err := workspace.ReadDocument(filepath.Join(env.s.Root, filepath.FromSlash(packet.Students[0])), &work); err != nil {
				t.Fatal(err)
			}
			var stored domain.DiscussionPacket
			if _, err := workspace.ReadDocument(filepath.Join(env.s.Root, filepath.FromSlash(work.DiscussionEvidence.Packet)), &stored); err != nil {
				t.Fatal(err)
			}
			sources := map[string]domain.Attachment{}
			for _, e := range stored.Entries {
				for _, a := range e.Attachments {
					sources[a.Source] = a
				}
			}
			if a := sources[""]; a.ID != 900 || a.UpdatedAt == "" || !a.HasOriginal() {
				t.Fatalf("attached file identity not kept: %+v", a)
			}
			if a := sources["embedded"]; a.ID != 950 || !a.HasOriginal() || a.Name != "diagram.png" {
				t.Fatalf("embedded course file not saved as evidence: %+v", a)
			}
			path, refs := env.results(t, packet, rel, []string{"100", "60"})
			env.files[501] = change
			status := env.postStatuses(t, path)
			held := 0
			for _, ref := range refs {
				if status[ref] == "held" {
					held++
				}
			}
			if held != 1 || len(status) != 2 {
				t.Fatalf("a changed attached file should hold its author only: %v", status)
			}
		})
	}
}

// A packet saved before file identity was part of the entry signature still
// posts when nothing changed, and still holds when the file was replaced.
func TestGradedDiscussionLegacyPacketWithAttachment(t *testing.T) {
	for _, replaced := range []bool{false, true} {
		t.Run(fmt.Sprint("replaced=", replaced), func(t *testing.T) {
			env, packet, rel := newDiscussionEnvWith(t, func(env *discussionEnv) {
				env.files[501] = map[string]any{"id": 900, "filename": "essay.pdf", "updated_at": "2026-10-10T10:00:00Z"}
			})
			// Rewrite the pull as an older Engine saved it: no file versions, and
			// digests from the signature without file identity.
			for _, studentRel := range packet.Students {
				workPath := filepath.Join(env.s.Root, filepath.FromSlash(studentRel))
				var work domain.StudentWorkDocument
				body, err := workspace.ReadDocument(workPath, &work)
				if err != nil {
					t.Fatal(err)
				}
				packetPath := filepath.Join(env.s.Root, filepath.FromSlash(work.DiscussionEvidence.Packet))
				var stored domain.DiscussionPacket
				packetBody, err := workspace.ReadDocument(packetPath, &stored)
				if err != nil {
					t.Fatal(err)
				}
				var own []domain.DiscussionEntry
				for i := range stored.Entries {
					for j := range stored.Entries[i].Attachments {
						stored.Entries[i].Attachments[j].UpdatedAt = ""
					}
					if stored.Entries[i].StudentRef == work.StudentRef {
						own = append(own, stored.Entries[i])
					}
				}
				if err = workspace.WriteDocument(packetPath, &stored, packetBody); err != nil {
					t.Fatal(err)
				}
				work.DiscussionEvidence.Digest = domain.DiscussionEvidenceLegacyDigest(own)
				if err = workspace.WriteDocument(workPath, work, body); err != nil {
					t.Fatal(err)
				}
			}
			path, refs := env.results(t, packet, rel, []string{"100", "60"})
			if replaced {
				env.files[501] = map[string]any{"id": 901, "filename": "essay-v2.pdf", "updated_at": "2026-10-10T10:00:00Z"}
			}
			status := env.postStatuses(t, path)
			held, posted := 0, 0
			for _, ref := range refs {
				switch status[ref] {
				case "held":
					held++
				case "succeeded":
					posted++
				}
			}
			if replaced && (held != 1 || posted != 1) || !replaced && posted != 2 {
				t.Fatalf("legacy packet outcome replaced=%v: %v", replaced, status)
			}
		})
	}
}

func TestGradedDiscussionResultsRequireEvidence(t *testing.T) {
	topic := int64(77)
	snap := domain.AssignmentSnapshot{DiscussionTopicID: &topic}
	if err := domain.ValidateResultsEvidence(snap, nil); err == nil {
		t.Fatal("a graded discussion result without evidence validated")
	}
}
