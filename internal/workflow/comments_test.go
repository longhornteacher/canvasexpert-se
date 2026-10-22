package workflow

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/render"
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

func TestKnownFeedbackCommentIsKeptEvenIfAttemptAssociationDiffers(t *testing.T) {
	live := fixtureSubmission(5)
	other := 1
	live.Comments = []SubmissionComment{{ID: 82, Comment: "Exact intended text", Attempt: &other}}
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) { t.Fatal("known comment reconciliation issued a request") })
	out := domain.ActionOutcome{Status: "uncertain", ID: 82, PostedText: "Exact intended text"}
	if e := s.addFeedback(context.Background(), domain.ResultsDocument{}, 5, attemptRef(live), out.PostedText, &out, live, func() error { return nil }); e != nil {
		t.Fatal(e)
	}
	if out.Status != "preserved" {
		t.Fatalf("a known comment already in Canvas should count as posted: %#v", out)
	}
}

func TestNewQuizCommentOnlyWritesOnlyAttemptScopedComment(t *testing.T) {
	writes, reads := 0, 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" {
			reads++
			_ = json.NewEncoder(w).Encode(fixtureAssignment())
			return
		}
		writes++
		if r.Method != "PUT" || !strings.HasSuffix(r.URL.Path, "/submissions/5") {
			t.Errorf("unexpected New Quiz write: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		comment := body["comment"]
		if len(body) != 1 || len(comment) != 2 || comment["text_comment"] == "" || comment["attempt"] != float64(2) {
			t.Errorf("write was not comment-only and attempt-scoped: %#v", body)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	live := fixtureSubmission(5)
	rubric := domain.Rubric{Title: "Evidence", Criteria: []domain.Criterion{{ID: "c1", Description: "Evidence", Points: domain.MustDecimal("100")}}}
	doc := &domain.ResultsDocument{Assignment: domain.AssignmentSnapshot{CourseID: 1, AssignmentID: 10, Points: domain.MustDecimal("20"), NewQuiz: true, Rubric: &rubric}, Interpretation: domain.Interpretation{Mapping: "repeated", Rationale: "Each written item uses this rubric.", Components: []domain.Component{{Key: "answer", Label: "Answer", Weight: domain.MustDecimal("1")}}}}
	row := &domain.StudentResult{Operation: "comment-only", CommentOnly: &domain.CommentOnlyOperation{Attempt: attemptRef(live), Components: []domain.ComponentScore{{Key: "answer", Evidence: "present", Criteria: []domain.CriterionScore{{CriterionID: "c1", Points: domain.MustDecimal("80")}}}}, Feedback: "Explain your evidence."}}
	validated := domain.ValidatedStudent{Interpretation: doc.Interpretation}
	work := domain.StudentWorkDocument{Current: domain.AttemptWork{Ref: attemptRef(live), AutoScored: &domain.AutoScored{Earned: domain.MustDecimal("14.00"), Possible: domain.MustDecimal("20.00"), Items: 5}}}
	source := postSource{student: workspace.Student{UserID: 5}, work: work}
	if err := s.postCommentOnly(context.Background(), doc, row, validated, source, live, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if writes != 1 || reads != 0 || row.CommentOnly.Outcome.Status != "succeeded" {
		t.Fatalf("accepted comment did not finish quietly: writes=%d reads=%d outcome=%#v", writes, reads, row.CommentOnly.Outcome)
	}
	if got := row.CommentOnly.Outcome.PostedText; !strings.HasPrefix(got, "Auto-scored items: 14/20\nCriterion scores:\n") || strings.Contains(got, "Overall") {
		t.Fatalf("New Quiz comment should lead with the pulled auto-scored subtotal and no total: %q", got)
	}
	currentAttempt := live.Attempt
	live.Comments = []SubmissionComment{{ID: 72, Comment: row.CommentOnly.Outcome.PostedText, Attempt: &currentAttempt}}
	if err := s.postCommentOnly(context.Background(), doc, row, validated, source, live, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if writes != 1 || row.CommentOnly.Outcome.Status != "succeeded" || reads != 0 {
		t.Fatalf("exact current-attempt duplicate was not preserved: writes=%d reads=%d outcome=%#v", writes, reads, row.CommentOnly.Outcome)
	}
}

func TestNewQuizCommentOnlyDuplicateCheckIsAttemptScoped(t *testing.T) {
	writes := 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		writes++
		w.WriteHeader(http.StatusNoContent)
	})
	live := fixtureSubmission(5)
	attempt := attemptRef(live)
	rubric := domain.Rubric{Title: "Evidence", Criteria: []domain.Criterion{{ID: "c1", Description: "Evidence", Points: domain.MustDecimal("100")}}}
	interpretation := domain.Interpretation{Mapping: "repeated", Rationale: "Each written item uses this rubric.", Components: []domain.Component{{Key: "answer", Label: "Answer", Weight: domain.MustDecimal("1")}}}
	doc := &domain.ResultsDocument{Assignment: domain.AssignmentSnapshot{CourseID: 1, AssignmentID: 10, Points: domain.MustDecimal("20"), NewQuiz: true, Rubric: &rubric}, Interpretation: interpretation}
	row := &domain.StudentResult{Operation: "comment-only", CommentOnly: &domain.CommentOnlyOperation{Attempt: attempt, Components: []domain.ComponentScore{{Key: "answer", Evidence: "present", Criteria: []domain.CriterionScore{{CriterionID: "c1", Points: domain.MustDecimal("80")}}}}, Feedback: "Explain your evidence."}}
	computed, _ := domain.ComputeAssessment(rubric, interpretation, row.CommentOnly.Components, doc.Assignment.Points)
	want, err := render.RenderFeedback(render.FeedbackInput{Rubric: rubric, Interpretation: interpretation, Components: row.CommentOnly.Components, Assessment: computed.Posted, AssignmentPoints: doc.Assignment.Points, Feedback: row.CommentOnly.Feedback, OmitAssessment: true})
	if err != nil {
		t.Fatal(err)
	}
	priorAttempt := 1
	live.Comments = []SubmissionComment{{ID: 71, Comment: want, Attempt: &priorAttempt}}
	live.History = nil
	// A copy on another attempt must not suppress the current attempt's comment.
	validated := domain.ValidatedStudent{Interpretation: interpretation}
	if err = s.postCommentOnly(context.Background(), doc, row, validated, postSource{student: workspace.Student{UserID: 5}}, live, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if writes != 1 || row.CommentOnly.Outcome.Status != "succeeded" {
		t.Fatalf("different-attempt duplicate incorrectly suppressed current write: writes=%d outcome=%#v", writes, row.CommentOnly.Outcome)
	}
}

func TestOrdinaryCommentOnlyAddsFollowUpWithoutTouchingTheGrade(t *testing.T) {
	var posted string
	writes := 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writes++
		var body map[string]map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if _, graded := body["submission"]; graded || len(body) != 1 {
			t.Errorf("follow-up wrote more than a comment: %#v", body)
		}
		posted, _ = body["comment"]["text_comment"].(string)
		w.WriteHeader(http.StatusNoContent)
	})
	live := fixtureSubmission(5)
	basis := domain.Rubric{Title: "Basis", Criteria: []domain.Criterion{{ID: "c1", Description: "Evidence", Points: domain.MustDecimal("10")}}}
	doc := &domain.ResultsDocument{Assignment: domain.AssignmentSnapshot{CourseID: 1, AssignmentID: 10, Points: domain.MustDecimal("20"), Rubric: &basis}, Interpretation: domain.Interpretation{Mapping: "repeated", Rationale: "One response.", Components: []domain.Component{{Key: "answer", Label: "Answer", Weight: domain.MustDecimal("1")}}}}
	row := &domain.StudentResult{Operation: "comment-only", CommentOnly: &domain.CommentOnlyOperation{Attempt: attemptRef(live), Header: "none", Components: []domain.ComponentScore{{Key: "answer", Evidence: "present", Criteria: []domain.CriterionScore{{CriterionID: "c1", Points: domain.MustDecimal("8")}}}}, Feedback: "Class exemplars: ..."}}
	if err := s.postCommentOnly(context.Background(), doc, row, domain.ValidatedStudent{Interpretation: doc.Interpretation}, postSource{student: workspace.Student{UserID: 5}}, live, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if writes != 1 || posted != "Class exemplars: ..." || row.CommentOnly.Outcome.Status != "succeeded" {
		t.Fatalf("follow-up comment not posted exactly: writes=%d text=%q outcome=%#v", writes, posted, row.CommentOnly.Outcome)
	}
	row.CommentOnly.Outcome = domain.ActionOutcome{}
	row.CommentOnly.Header = ""
	if err := s.postCommentOnly(context.Background(), doc, row, domain.ValidatedStudent{Interpretation: doc.Interpretation}, postSource{student: workspace.Student{UserID: 5}}, live, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(posted, "Overall: 16/20\n") {
		t.Fatalf("ordinary comment-only should keep the honest overall line: %q", posted)
	}
}
