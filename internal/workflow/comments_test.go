package workflow

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

func TestHistoryCommentsPreserveAttemptAndRejectConflictingContexts(t *testing.T) {
	sub := fixtureSubmission(5)
	sub.Comments = []SubmissionComment{{ID: 81, Comment: "Current live text"}}
	sub.History[0].Comments = []SubmissionComment{{ID: 81, Comment: "Old history text"}, {ID: 82, Comment: "Prior-only feedback"}}
	comments := submissionComments(sub)
	if len(comments) != 2 || comments[0].Comment != "Current live text" || comments[0].Attempt == nil || *comments[0].Attempt != 1 || comments[1].Attempt == nil || *comments[1].Attempt != 1 {
		t.Fatalf("history comments lost or misassociated: %#v", comments)
	}
	sub.History = append(sub.History, Submission{Attempt: 2, Comments: []SubmissionComment{{ID: 82, Comment: "Prior-only feedback"}}})
	comments = submissionComments(sub)
	if comments[1].Attempt != nil || len(commentsForAttempt(sub, 1)) != 1 {
		t.Fatal("conflicting history contexts arbitrarily assigned a comment")
	}
}

func TestPullAndRevisePriorAttemptCommentFromHistoryOnly(t *testing.T) {
	live := fixtureSubmission(5)
	live.History[0].Comments = []SubmissionComment{{ID: 82, Comment: "Prior feedback"}}
	writes := 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "PUT":
			writes++
			if !strings.HasSuffix(r.URL.Path, "/comments/82") {
				t.Errorf("unexpected write: %s", r.URL.Path)
			}
			var request map[string]string
			_ = json.NewDecoder(r.Body).Decode(&request)
			if len(request) != 1 || request["comment"] == "" {
				t.Error("feedback revision sent another action")
			}
			live.History[0].Comments[0].Comment = request["comment"]
			_ = json.NewEncoder(w).Encode(live.History[0].Comments[0])
		case strings.HasSuffix(r.URL.Path, "/enrollments"):
			_ = json.NewEncoder(w).Encode([]Enrollment{{UserID: 5, Type: "StudentEnrollment", State: "active", User: live.User}})
		case strings.HasSuffix(r.URL.Path, "/submissions"):
			_ = json.NewEncoder(w).Encode([]Submission{live})
		case strings.Contains(r.URL.Path, "/submissions/5"):
			_ = json.NewEncoder(w).Encode(live)
		default:
			_ = json.NewEncoder(w).Encode(fixtureAssignment())
		}
	})
	report, e := s.PullWork(context.Background(), "TEST", "10")
	if e != nil || len(report.Packets) != 1 {
		t.Fatalf("pull: %#v %v", report, e)
	}
	var packet domain.PacketDocument
	if _, e = workspace.ReadDocument(report.Packets[0], &packet); e != nil {
		t.Fatal(e)
	}
	var work domain.StudentWorkDocument
	if _, e = workspace.ReadDocument(filepath.Join(s.Root, filepath.FromSlash(packet.Students[0])), &work); e != nil {
		t.Fatal(e)
	}
	if len(work.Comments) != 1 || work.Comments[0].ID != 82 || work.Comments[0].Attempt == nil || *work.Comments[0].Attempt != 1 {
		t.Fatalf("packet dropped prior comment: %#v", work.Comments)
	}
	rel, e := filepath.Rel(s.Root, report.Packets[0])
	if e != nil {
		t.Fatal(e)
	}
	results := domain.ResultsDocument{Schema: 1, Kind: "results", Packet: filepath.ToSlash(rel), Assignment: packet.Assignment,
		Students: []domain.StudentResult{{StudentRef: work.StudentRef, Operation: "revise-feedback", ReviseFeedback: &domain.FeedbackRevision{Attempt: work.Prior[0].Ref, CommentID: 82, Feedback: "Use a specific example."}}}}
	path := filepath.Join(s.Root, "courses", "TEST", "revision.md")
	if e = workspace.WriteDocument(path, results, "revision"); e != nil {
		t.Fatal(e)
	}
	post, e := s.Post(context.Background(), path)
	if e != nil || writes != 1 || len(post.Students) != 1 || post.Students[0].Status != "completed" {
		t.Fatalf("prior revision: %#v writes=%d err=%v", post, writes, e)
	}
	if decimalPointer(live.EnteredScore).String() != "15" || len(live.RubricAssessment) != 0 || live.LatePolicyStatus != nil {
		t.Fatal("prior feedback revision changed grade, rubric or lateness")
	}
}

func TestKnownFeedbackCommentWithChangedAttemptHeld(t *testing.T) {
	live := fixtureSubmission(5)
	other := 1
	live.Comments = []SubmissionComment{{ID: 82, Comment: "Exact intended text", Attempt: &other}}
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) { t.Fatal("known comment reconciliation issued a request") })
	out := domain.ActionOutcome{Status: "uncertain", ID: 82, PostedText: "Exact intended text"}
	if e := s.addFeedback(context.Background(), domain.ResultsDocument{}, 5, attemptRef(live), out.PostedText, &out, live, func() error { return nil }); e != nil {
		t.Fatal(e)
	}
	if out.Status != "held" {
		t.Fatalf("wrong-attempt known comment satisfied feedback: %#v", out)
	}
}
