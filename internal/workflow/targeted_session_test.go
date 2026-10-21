package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

func TestTargetedSessionSkipsZeroAndClosedAndKeepsFutureAndUndated(t *testing.T) {
	zero, one := 0, 1
	groupID := int64(7)
	future := time.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339)
	assignments := make([]Assignment, 4)
	for i := range assignments {
		assignments[i] = fixtureAssignment()
		assignments[i].ID = int64(10 + i)
		assignments[i].AssignmentGroupID = &groupID
		assignments[i].NeedsGradingCount = &one
	}
	assignments[0].NeedsGradingCount = &zero
	assignments[1].InClosedGradingPeriod = true
	assignments[2].NeedsGradingCount = nil
	assignments[2].DueAt = &future
	assignments[3].DueAt = nil
	var started atomic.Int32
	ready := make(chan struct{})
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/courses/1/enrollments":
			_ = json.NewEncoder(w).Encode([]Enrollment{{UserID: 5, Type: "StudentEnrollment", State: "active", User: CanvasUser{ID: 5, Name: "Synthetic Student"}}})
		case "/api/v1/courses/1/assignments":
			if !strings.Contains(r.URL.RawQuery, "include[]=all_dates") {
				t.Error("assignment list omitted all_dates")
			}
			_ = json.NewEncoder(w).Encode(assignments)
		case "/api/v1/courses/1/assignment_groups":
			if strings.Contains(r.URL.RawQuery, "assignments") {
				t.Error("group read embedded assignments")
			}
			_ = json.NewEncoder(w).Encode([]AssignmentGroup{{ID: 7, Name: "Daily", Weight: "40"}})
		case "/api/v1/courses/1/assignments/12/submissions", "/api/v1/courses/1/assignments/13/submissions":
			if !strings.Contains(r.URL.RawQuery, "include[]=submission_history") {
				t.Error("session did not use detailed list")
			}
			if started.Add(1) == 2 {
				close(ready)
			}
			select {
			case <-ready:
			case <-time.After(3 * time.Second):
				t.Error("assignment submission lists did not run concurrently")
			}
			sub := fixtureSubmission(5)
			sub.AssignmentID = 12
			sub.CachedDueDate = &future
			if strings.Contains(r.URL.Path, "/13/") {
				sub.AssignmentID = 13
				sub.CachedDueDate = nil
			}
			sub.CachedDueDatePresent = true
			_ = json.NewEncoder(w).Encode([]Submission{sub})
		case "/api/v1/courses/1/assignments/12":
			_ = json.NewEncoder(w).Encode(assignments[2])
		case "/api/v1/courses/1/assignments/13":
			_ = json.NewEncoder(w).Encode(assignments[3])
		default:
			t.Errorf("unexpected request (zero/closed assignments and individual submissions must not be read): %s", r.URL.Path)
			http.NotFound(w, r)
		}
	})
	// No calendar or grading periods exist in this fixture.
	before := s.Client.Requests()
	report, err := s.PullWorkWithOptions(context.Background(), "TEST", "", PullWorkOptions{Session: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.Students != 2 || len(report.Packets) != 2 || s.Client.Requests()-before != 7 {
		t.Fatalf("targeting/request count: %+v", report)
	}
	if !report.Assignments[1].ClosedGradingPeriod || report.Assignments[1].SkipReason != "closed grading period, not pulled" {
		t.Fatalf("closed assignment not reported: %+v", report.Assignments[1])
	}
	if !report.Assignments[2].NeedsGradingCountUnknown || report.Assignments[2].DueState != "not due yet" || report.Assignments[2].Students != 1 {
		t.Fatalf("unknown-count future work lost: %+v", report.Assignments[2])
	}
	if report.Assignments[3].DueState != "undated" || report.Assignments[3].Undated != 1 || report.Assignments[3].Students != 1 {
		t.Fatalf("undated work lost: %+v", report.Assignments[3])
	}
	for _, summary := range report.Assignments {
		if summary.AssignmentGroupName != "Daily" || summary.AssignmentGroupWeight != "40" {
			t.Fatalf("group report: %+v", summary)
		}
	}
}

func TestSessionCommentOnlyCompletionRemindsUntilNewAttempt(t *testing.T) {
	assignment := fixtureAssignment()
	assignment.NewQuiz = true
	one := 1
	assignment.NeedsGradingCount = &one
	live := fixtureSubmission(5)
	var origin string
	s, server := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/enrollments"):
			_ = json.NewEncoder(w).Encode([]Enrollment{{UserID: 5, Type: "StudentEnrollment", State: "active", User: CanvasUser{ID: 5, Name: "Synthetic Student"}}})
		case r.URL.Path == "/api/v1/courses/1/assignments":
			_ = json.NewEncoder(w).Encode([]Assignment{assignment})
		case strings.HasSuffix(r.URL.Path, "/assignment_groups"):
			_ = json.NewEncoder(w).Encode([]AssignmentGroup{})
		case r.URL.Path == "/api/v1/courses/1/assignments/10":
			_ = json.NewEncoder(w).Encode(assignment)
		case r.URL.Path == "/api/v1/courses/1/assignments/10/submissions":
			_ = json.NewEncoder(w).Encode([]Submission{live})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/reports"):
			_, _ = w.Write([]byte(`{"progress":{"url":"/api/quiz/v1/reports/7"}}`))
		case r.URL.Path == "/api/quiz/v1/reports/7":
			_, _ = w.Write([]byte(`{"workflow_state":"completed","results":{"url":"` + origin + `/signed/report"}}`))
		case r.URL.Path == "/signed/report":
			_ = json.NewEncoder(w).Encode([]any{map[string]any{"student_data": map[string]any{"id": 5, "attempt": live.Attempt, "submitted_at": *live.SubmittedAt}, "item_responses": []any{map[string]any{"item_id": 21, "item_type": "essay", "answer": "A thoughtful answer"}}}})
		case strings.HasSuffix(r.URL.Path, "/items"):
			_, _ = w.Write([]byte(`[{"id":"21","entry_type":"Item","entry":{"id":"121","interaction_type_slug":"essay","item_body":"Explain."}}]`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	})
	origin = server.URL
	store, err := workspace.OpenStudents(s.Root)
	if err != nil {
		t.Fatal(err)
	}
	student, err := store.Upsert(s.Client.Origin(), 5, "Synthetic Student", "Synthetic Student", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Save(); err != nil {
		t.Fatal(err)
	}
	snapshot, err := Snapshot(assignment, s.Client.Origin())
	if err != nil {
		t.Fatal(err)
	}
	result := domain.ResultsDocument{Schema: 1, Kind: "results", Packet: "courses/TEST/packet.md", Assignment: snapshot, Students: []domain.StudentResult{{StudentRef: student.Ref, Operation: "comment-only", CommentOnly: &domain.CommentOnlyOperation{Attempt: attemptRef(live), Feedback: "Thoughtful explanation.", Outcome: domain.ActionOutcome{Status: "succeeded"}}}}}
	result.Interpretation = domain.Interpretation{Mapping: "repeated", Rationale: "Assess the written response.", Components: []domain.Component{{Key: "answer", Label: "Answer", Weight: domain.MustDecimal("1")}}}
	result.Students[0].CommentOnly.Components = []domain.ComponentScore{{Key: "answer", Evidence: "present", Criteria: []domain.CriterionScore{{CriterionID: "c1", Points: domain.MustDecimal("80")}}}}
	if err = workspace.WriteDocument(filepath.Join(s.Root, "drafts", "completed.md"), result, "Accepted quiz feedback"); err != nil {
		t.Fatal(err)
	}
	report, err := s.PullWorkWithOptions(context.Background(), "TEST", "", PullWorkOptions{Session: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.Students != 0 || report.Assignments[0].CommentOnlyCompleted != 1 || report.FetchRequests != 0 {
		t.Fatalf("completed quiz was re-pulled: %+v", report)
	}
	live.Attempt++
	report, err = s.PullWorkWithOptions(context.Background(), "TEST", "", PullWorkOptions{Session: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.Students != 1 || report.Assignments[0].CommentOnlyCompleted != 0 {
		t.Fatalf("new attempt was suppressed: %+v", report)
	}
}

func TestPullDownloadsAttachmentsConcurrentlyAtClientCap(t *testing.T) {
	count := 4
	assignment := fixtureAssignment()
	assignment.NeedsGradingCount = &count
	var inFlight, peak atomic.Int32
	full := make(chan struct{})
	var server string
	s, srv := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/courses/1/enrollments":
			var rows []Enrollment
			for user := int64(5); user < 9; user++ {
				rows = append(rows, Enrollment{UserID: user, Type: "StudentEnrollment", State: "active", User: CanvasUser{ID: user, Name: fmt.Sprintf("Synthetic %d", user)}})
			}
			_ = json.NewEncoder(w).Encode(rows)
		case r.URL.Path == "/api/v1/courses/1/assignments":
			_ = json.NewEncoder(w).Encode([]Assignment{assignment})
		case r.URL.Path == "/api/v1/courses/1/assignment_groups":
			_ = json.NewEncoder(w).Encode([]AssignmentGroup{})
		case r.URL.Path == "/api/v1/courses/1/assignments/10/submissions":
			var rows []Submission
			for user := int64(5); user < 9; user++ {
				sub := fixtureSubmission(user)
				sub.Score, sub.EnteredScore = nil, nil
				sub.Attachments = []Attachment{{ID: user*10 + 2, Filename: "current.txt", URL: fmt.Sprintf("%s/files/%d/download", server, user*10+2), ContentType: "text/plain"}}
				sub.History[0].Attachments = []Attachment{{ID: user*10 + 1, Filename: "draft.txt", URL: fmt.Sprintf("%s/files/%d/download", server, user*10+1), ContentType: "text/plain"}}
				rows = append(rows, sub)
			}
			_ = json.NewEncoder(w).Encode(rows)
		case r.URL.Path == "/api/v1/courses/1/assignments/10":
			_ = json.NewEncoder(w).Encode(assignment)
		case strings.HasPrefix(r.URL.Path, "/files/"):
			now := inFlight.Add(1)
			defer inFlight.Add(-1)
			for old := peak.Load(); now > old && !peak.CompareAndSwap(old, now); old = peak.Load() {
			}
			if now == 4 {
				close(full)
			}
			select {
			case <-full:
			case <-time.After(3 * time.Second):
				t.Error("attachment downloads did not overlap")
			}
			_, _ = w.Write([]byte("file " + strings.Split(r.URL.Path, "/")[2]))
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	})
	server = srv.URL
	report, err := s.PullWorkWithOptions(context.Background(), "TEST", "", PullWorkOptions{Session: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.Students != 4 || peak.Load() > 4 {
		t.Fatalf("students=%d peak downloads=%d: %+v", report.Students, peak.Load(), report)
	}
	files, err := filepath.Glob(filepath.Join(filepath.Dir(report.Packets[0]), "students", "*.md"))
	if err != nil || len(files) != 4 {
		t.Fatalf("student files: %v %v", files, err)
	}
	store, err := workspace.OpenStudents(s.Root)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		var doc domain.StudentWorkDocument
		if _, err := workspace.ReadDocument(file, &doc); err != nil {
			t.Fatal(err)
		}
		var user int64
		for _, row := range store.Rows {
			if row.Ref == doc.StudentRef {
				user = row.UserID
			}
		}
		if len(doc.Current.Attachments) != 1 || doc.Current.Attachments[0].ID != user*10+2 || doc.Current.Attachments[0].Status != "readable" || len(doc.Prior) != 1 || len(doc.Prior[0].Attachments) != 1 || doc.Prior[0].Attachments[0].ID != user*10+1 {
			t.Fatalf("attachments crossed students or attempts for user %d: %+v / %+v", user, doc.Current.Attachments, doc.Prior)
		}
	}
}
