package workflow

import (
	"os"
	"path/filepath"
	"testing"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
	"gopkg.in/yaml.v3"
)

func writeIndexedFixture(t *testing.T, path string, doc any) {
	t.Helper()
	header, err := yaml.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(append([]byte("---\n"), header...), []byte("---\n")...), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestResultsIndexKeepsOneOperationSnapshotAndExactCompletionIdentity(t *testing.T) {
	root := t.TempDir()
	snapshot := domain.AssignmentSnapshot{Instance: "https://canvas.example", CourseID: 2, AssignmentID: 10}
	current := domain.AttemptRef{Attempt: 2, SubmittedAt: "2026-09-11T14:00:00Z"}
	source := domain.AttemptRef{Attempt: 1, SubmittedAt: "2026-09-10T14:00:00Z"}
	due := "2026-09-10T15:00:00Z"
	timing := domain.LateTiming{Status: "none", DueAt: due, SchoolTime: &domain.SchoolTimeProvenance{Attempt: source, SubmittedAt: source.SubmittedAt, Deadline: due, Status: "none"}}
	doc := domain.ResultsDocument{Schema: 1, Kind: "results", Assignment: snapshot, Students: []domain.StudentResult{
		{StudentRef: "graded", Operation: "grade", Grade: &domain.GradeOperation{Attempt: current, Outcomes: domain.ActionOutcomes{Comment: domain.ActionOutcome{Status: "succeeded"}, Grade: domain.ActionOutcome{Status: "succeeded"}, Rubric: domain.ActionOutcome{Status: "not-requested"}, Late: domain.ActionOutcome{Status: "held"}}}},
		{StudentRef: "quiz", Operation: "comment-only", CommentOnly: &domain.CommentOnlyOperation{Attempt: current, Outcome: domain.ActionOutcome{Status: "succeeded"}}},
		{StudentRef: "pending", Operation: "comment-only", CommentOnly: &domain.CommentOnlyOperation{Attempt: current, Outcome: domain.ActionOutcome{Status: "pending"}}},
		{StudentRef: "feedback", Operation: "comment-only", CommentOnly: &domain.CommentOnlyOperation{Attempt: current, Header: "none", Feedback: "Keep revising.", Outcome: domain.ActionOutcome{Status: "succeeded"}}},
		{StudentRef: "timed", Operation: "late-only", LateOnly: &domain.LateOnlyOperation{Attempt: current, LateTiming: timing, Outcome: domain.ActionOutcome{Status: "succeeded"}}},
	}}
	path := filepath.Join(root, "drafts", "accepted")
	writeIndexedFixture(t, path, &doc)
	s := &Service{Root: root, Config: domain.CourseConfig{Instance: snapshot.Instance}}
	operation := s.withResultsIndex()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	index := operation.indexedResults()
	sub := Submission{Attempt: 2, SubmittedAt: &current.SubmittedAt, LateKnown: true, SecondsLateKnown: true, LatePolicyStatusKnown: true, CachedDueDatePresent: true, CachedDueDate: &due, History: []Submission{{Attempt: 1, SubmittedAt: &source.SubmittedAt}}}
	if operation.withResultsIndex().indexedResults() != index || s.resultsIndex != nil {
		t.Fatal("operation must reuse its own index without caching it on the reusable service")
	}
	if !index.completedHint(10, "graded", sub, snapshot) || !index.completedCommentOnlyHint(10, "quiz", sub, snapshot) || index.completedCommentOnlyHint(10, "pending", sub, snapshot) {
		t.Fatal("accepted grade/comment completion was lost or a pending comment was counted")
	}
	if index.completedCommentOnlyHint(10, "feedback", sub, snapshot) {
		t.Fatal("feedback without scores must not suppress unscored quiz work")
	}
	if !index.completedTimingHint(snapshot.Instance, 10, 2, "timed", sub, Assignment{}, nil) || !operation.acceptedTimingRecordExists(2, 10, "timed", current) || !operation.acceptedTimingOrigin(2, 10, "timed", current, source, domain.ObservedTiming{}) {
		t.Fatal("timing completion/provenance must use the indexed operation snapshot")
	}
	for _, changed := range []domain.AssignmentSnapshot{
		{Instance: "https://other.example", CourseID: 2, AssignmentID: 10},
		{Instance: snapshot.Instance, CourseID: 3, AssignmentID: 10},
	} {
		if index.completedHint(10, "graded", sub, changed) || index.completedCommentOnlyHint(10, "quiz", sub, changed) {
			t.Fatal("another origin or course inherited completion")
		}
	}
	if index.completedCommentOnlyHint(11, "quiz", sub, snapshot) || index.completedCommentOnlyHint(10, "other", sub, snapshot) || index.completedCommentOnlyHint(10, "quiz", Submission{Attempt: 3}, snapshot) {
		t.Fatal("comment completion suppressed another assignment, student or new attempt")
	}
	if s.indexedResults().completedHint(10, "graded", sub, snapshot) {
		t.Fatal("a later operation inherited stale local results")
	}
}

func TestResultsIndexGroupCompletionMatchesExistingLooseGradeReader(t *testing.T) {
	root := t.TempDir()
	snapshot := domain.AssignmentSnapshot{Instance: "https://canvas.example", CourseID: 2, AssignmentID: 10}
	grade := &domain.GradeOperation{Attempt: domain.AttemptRef{Attempt: 3}, Outcomes: domain.ActionOutcomes{Comment: domain.ActionOutcome{Status: "succeeded"}, Rubric: domain.ActionOutcome{Status: "not-requested"}, Grade: domain.ActionOutcome{Status: "succeeded"}}}
	// Unknown authored fields and a malformed unrelated action must retain the
	// forgiving academic reader's existing behavior.
	doc := map[string]any{"schema": 1, "kind": "group-results", "assignment": snapshot, "groups": []any{
		map[string]any{"group_id": 60, "operation": "individual", "member_refs": []string{"first", "second"}, "students": []any{
			map[string]any{"student_ref": "first", "operation": "grade", "grade": grade, "future_slice_field": "ignored"},
			map[string]any{"student_ref": "second", "operation": "comment-only", "comment_only": "unfinished authoring"},
		}},
	}}
	path := filepath.Join(root, "drafts", "groups.yaml")
	if err := workspace.WriteDocument(path, doc, ""); err != nil {
		t.Fatal(err)
	}
	index := loadResultsIndex(root)
	for _, ref := range []string{"first", "second"} {
		for _, attempt := range []int{3, 4} {
			sub := Submission{Attempt: attempt}
			want := groupResultsContainCompletedGrade(root, path, snapshot, 10, ref, sub)
			if got := index.completedHint(10, ref, sub, snapshot); got != want {
				t.Fatalf("group completion differs for %s attempt %d: index=%v walker=%v", ref, attempt, got, want)
			}
		}
	}
}

func TestResultsIndexSharedTimingUsesPacketMemberAttempt(t *testing.T) {
	root := t.TempDir()
	snapshot := domain.AssignmentSnapshot{Instance: "https://canvas.example", CourseID: 2, AssignmentID: 10}
	source := domain.AttemptRef{Attempt: 1, SubmittedAt: "2026-09-10T14:00:00Z"}
	due := "2026-09-10T15:00:00Z"
	packetPath, workPath := "courses/TEST/packet.md", "courses/TEST/group.md"
	packet := domain.GroupPacketDocument{Groups: []domain.GroupPacketEntry{{GroupID: 60, Path: workPath}}}
	work := domain.GroupWorkDocument{Members: []domain.GroupWorkMember{{StudentRef: "first", Attempt: 2}, {StudentRef: "second", Attempt: 3}}}
	for path, doc := range map[string]any{packetPath: &packet, workPath: &work} {
		writeIndexedFixture(t, filepath.Join(root, filepath.FromSlash(path)), doc)
	}
	provenance := domain.SchoolTimeProvenance{Attempt: source, SubmittedAt: source.SubmittedAt, Deadline: due, Status: "none"}
	timing := domain.LateTiming{Status: "none", DueAt: due, SchoolTime: &provenance}
	doc := domain.GroupResultsDocument{Schema: 1, Kind: "group-results", Packet: packetPath, Assignment: snapshot, Groups: []domain.GroupResult{{GroupID: 60, Operation: "grade", MemberRefs: []string{"first", "second"}, Grade: &domain.GroupGradeOperation{Outcomes: domain.ActionOutcomes{Comment: domain.ActionOutcome{Status: "succeeded"}, Rubric: domain.ActionOutcome{Status: "succeeded"}, Grade: domain.ActionOutcome{Status: "succeeded"}, Late: domain.ActionOutcome{Status: "succeeded"}}, LateTiming: &timing, MemberSchoolTime: map[string]domain.SchoolTimeProvenance{"first": provenance, "second": provenance}}}}}
	path := filepath.Join(root, "drafts", "group-results.md")
	writeIndexedFixture(t, path, &doc)
	index := loadResultsIndex(root)
	for _, ref := range []string{"first", "second"} {
		for _, attempt := range []int{2, 3} {
			sub := Submission{Attempt: attempt, LateKnown: true, SecondsLateKnown: true, LatePolicyStatusKnown: true, CachedDueDatePresent: true, CachedDueDate: &due, History: []Submission{{Attempt: 1}}}
			want := groupResultsContainCompletedGrade(root, path, snapshot, 10, ref, sub)
			if got := index.completedHint(10, ref, sub, snapshot); got != want {
				t.Fatalf("shared academic completion differs: %s attempt %d got=%v want=%v", ref, attempt, got, want)
			}
			if got := index.acceptedTimingRecordExists(snapshot.Instance, 2, 10, ref, attemptRef(sub)); got != want {
				t.Fatalf("shared timing record must follow the packet member attempt: %s attempt %d", ref, attempt)
			}
			if got := index.completedTimingHint(snapshot.Instance, 10, 2, ref, sub, Assignment{}, nil); got != want {
				t.Fatalf("shared completed timing must follow the packet member attempt: %s attempt %d", ref, attempt)
			}
			if got := index.acceptedTimingOrigin(snapshot.Instance, 2, 10, ref, attemptRef(sub), source, domain.ObservedTiming{}); got != want {
				t.Fatalf("shared timing origin must follow the packet member attempt: %s attempt %d", ref, attempt)
			}
		}
	}
}

// groupResultsContainCompletedGrade is the pre-index group completion reader,
// kept as an equivalence oracle. It reads academic completion independently
// of unrelated group actions. Header identity, student ref and exact packet
// attempt must match before a completed result suppresses a pull.
func groupResultsContainCompletedGrade(root, path string, snapshot domain.AssignmentSnapshot, assignmentID int64, studentRef string, sub Submission) bool {
	var doc map[string]any
	if _, err := workspace.ReadDocument(path, &doc); err != nil || yamlInt(doc["schema"]) != 1 || yamlString(doc["kind"]) != "group-results" {
		return false
	}
	assignment := yamlMap(doc["assignment"])
	if yamlString(assignment["instance"]) != snapshot.Instance || yamlInt64(assignment["course_id"]) != snapshot.CourseID || yamlInt64(assignment["assignment_id"]) != assignmentID {
		return false
	}
	for _, rawGroup := range yamlSlice(doc["groups"]) {
		group := yamlMap(rawGroup)
		if yamlString(group["operation"]) == "grade" {
			encoded, err := yaml.Marshal(group["grade"])
			var grade domain.GroupGradeOperation
			if err == nil && yaml.Unmarshal(encoded, &grade) == nil && definite(grade.Outcomes.Comment) && definite(grade.Outcomes.Rubric) && definite(grade.Outcomes.Grade) && containsString(yamlStrings(group["member_refs"]), studentRef) && groupResultCurrentAttempt(root, yamlString(doc["packet"]), yamlInt64(group["group_id"]), studentRef, attemptRef(sub)) {
				return true
			}
		}
		for _, rawStudent := range yamlSlice(group["students"]) {
			row := yamlMap(rawStudent)
			if yamlString(row["operation"]) != "grade" || yamlString(row["student_ref"]) != studentRef {
				continue
			}
			encoded, err := yaml.Marshal(row["grade"])
			if err != nil {
				continue
			}
			var grade domain.GradeOperation
			if yaml.Unmarshal(encoded, &grade) == nil && academicGradeComplete(&grade) && sameAttempt(grade.Attempt, attemptRef(sub)) {
				return true
			}
		}
	}
	return false
}
