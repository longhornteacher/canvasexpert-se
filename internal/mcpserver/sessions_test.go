package mcpserver

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

// A synthetic child stands in for the CLI: no credential store or Canvas calls.
func TestSessionChild(t *testing.T) {
	if os.Getenv("CSE_SESSION_CHILD") != "1" {
		return
	}
	kind := os.Getenv("CSE_SESSION_KIND")
	time.Sleep(80 * time.Millisecond)
	if kind == "review" {
		fmt.Fprintln(os.Stderr, "Open the local review page: http://127.0.0.1:12345/review")
	} else {
		fmt.Fprintln(os.Stderr, "Open the local setup page: http://127.0.0.1:12346/setup")
		fmt.Fprintln(os.Stdout, "private-real-name private-PAT")
		fmt.Fprintln(os.Stderr, "private-real-name private-PAT")
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(os.Getenv("CSE_SESSION_FINISH")); err == nil {
			if kind == "review" {
				fmt.Println(os.Getenv("CSE_SESSION_OUTCOME"))
			}
			os.Exit(0)
		}
		time.Sleep(5 * time.Millisecond)
	}
	os.Exit(1)
}

func sessionFixture(t *testing.T) (*Server, context.CancelFunc, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "courses"), 0700); err != nil {
		t.Fatal(err)
	}
	draft, err := os.ReadFile(filepath.Join("..", "..", "engine-docs", "examples", "synthetic-results-two.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, "courses", "results.md"), draft, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	finish := filepath.Join(root, "finish")
	s := &Server{root: root, ctx: ctx, sessionReadyWait: time.Millisecond}
	s.newSessionCommand = func(args []string) *exec.Cmd {
		cmd := exec.Command(os.Args[0], "-test.run=^TestSessionChild$")
		cmd.Env = append(os.Environ(), "CSE_SESSION_CHILD=1", "CSE_SESSION_KIND="+args[2], "CSE_SESSION_FINISH="+finish, "CSE_SESSION_OUTCOME=Revision requested: clarify feedback. Nothing posted.")
		return cmd
	}
	return s, cancel, finish
}

func awaitSession(t *testing.T, task *reviewTask, finished bool) string {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		out, err := task.status()
		if err != nil {
			t.Fatal(err)
		}
		if finished && strings.Contains(out.Text, "finished") {
			return out.Text
		}
		if !finished && strings.Contains(out.Text, "http://127.0.0.1") {
			return out.Text
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("session did not reach expected state")
	return ""
}

func TestDelayedReviewReadinessAndRevisionOutcome(t *testing.T) {
	s, _, finish := sessionFixture(t)
	out, err := s.startReview(context.Background(), ReviewInput{Path: "courses/results.md"})
	if err != nil || !strings.Contains(out.Text, "still starting") {
		t.Fatalf("%+v %v", out, err)
	}
	var task *reviewTask
	for _, task = range s.reviews {
	}
	awaitSession(t, task, false)
	if _, err := s.startReview(context.Background(), ReviewInput{Path: "courses/results.md"}); err == nil || !strings.Contains(err.Error(), task.id) {
		t.Fatal("duplicate active review accepted")
	}
	// Setup and review can coexist; private child output never enters tool results.
	if _, err := s.startLocalUI(context.Background(), LocalUIInput{Page: "setup"}); err != nil {
		t.Fatal(err)
	}
	var private *reviewTask
	for _, private = range s.sessions {
	}
	ready := awaitSession(t, private, false)
	if strings.Contains(ready, "private-") {
		t.Fatal("private contents leaked")
	}
	if err := os.WriteFile(finish, []byte("done"), 0600); err != nil {
		t.Fatal(err)
	}
	done := awaitSession(t, task, true)
	if !strings.Contains(done, "clarify feedback") || !strings.Contains(done, "Nothing posted") {
		t.Fatalf("revision outcome lost: %s", done)
	}
	if strings.Contains(done, "Open the local review page") || strings.Contains(done, "http://127.0.0.1") {
		t.Fatal("expired review URL returned", done)
	}
	if done := awaitSession(t, private, true); strings.Contains(done, "private-") {
		t.Fatal("private contents leaked after completion")
	}
	// Revised draft can launch a new review and retain its posting receipt.
	s.newSessionCommand = func(args []string) *exec.Cmd {
		cmd := exec.Command(os.Args[0], "-test.run=^TestSessionChild$")
		cmd.Env = append(os.Environ(), "CSE_SESSION_CHILD=1", "CSE_SESSION_KIND=review", "CSE_SESSION_FINISH="+finish, "CSE_SESSION_OUTCOME=Approved: synthetic student-ref 7; posted honest computed assessment 8/10.")
		return cmd
	}
	if _, err := s.startReview(context.Background(), ReviewInput{Path: "courses/results.md"}); err != nil {
		t.Fatal(err)
	}
	for _, next := range s.reviews {
		if next != task {
			if !strings.Contains(awaitSession(t, next, true), "8/10") {
				t.Fatal("posting outcome lost")
			}
		}
	}
}

func TestReviewStructuredStatusPreservesOutcomePathAndNotes(t *testing.T) {
	task := &reviewTask{id: "review-test", kind: "review", url: "http://127.0.0.1:12345/review", done: make(chan struct{})}
	pending, _ := task.status()
	if pending.State != "pending" || pending.ReviewID != task.id || pending.URL != task.url {
		t.Fatalf("%+v", pending)
	}
	task.stdout.WriteString("Review result: {\"outcome\":\"partial\",\"result_path\":\"archive/saved.md\",\"revision_notes\":[{\"ref\":\"s-a\",\"note\":\"clarify\"}]}\n")
	close(task.done)
	finished, _ := task.status()
	if finished.State != "finished" || finished.Outcome != "partial" || finished.URL != "" || finished.ResultPath != "archive/saved.md" || len(finished.RevisionNotes) != 1 || finished.RevisionNotes[0].Note != "clarify" {
		t.Fatalf("%+v", finished)
	}
	task.err = fmt.Errorf("synthetic exit failure")
	failed, _ := task.status()
	if failed.State != "failed" || failed.URL != "" {
		t.Fatalf("%+v", failed)
	}
}

func TestInvalidReviewRejectedBeforeAllocatingSession(t *testing.T) {
	s, _, _ := sessionFixture(t)
	config := domain.CourseConfig{Schema: 1, Instance: "https://canvas.example.edu", Courses: []domain.Course{{Nickname: "DEMO", ID: 11}, {Nickname: "WRONG", ID: 12}}}
	if err := workspace.WriteDocument(filepath.Join(s.root, "context", "courses.md"), config, ""); err != nil {
		t.Fatal(err)
	}
	s.newSessionCommand = func([]string) *exec.Cmd { t.Fatal("invalid review allocated child process"); return nil }
	if _, err := s.startReview(context.Background(), ReviewInput{Path: "courses/results.md", Course: "WRONG"}); err == nil || !strings.Contains(err.Error(), "differs from document course_id 11") {
		t.Fatal("course mismatch accepted")
	}
	if len(s.reviews) != 0 {
		t.Fatal("invalid input consumed active review slot")
	}
}

func TestCallCancellationKeepsReviewUntilServerShutdown(t *testing.T) {
	s, cancel, _ := sessionFixture(t)
	callCtx, callCancel := context.WithCancel(context.Background())
	callCancel()
	if _, err := s.startReview(callCtx, ReviewInput{Path: "courses/results.md"}); err != nil {
		t.Fatal(err)
	}
	var task *reviewTask
	for _, task = range s.reviews {
	}
	awaitSession(t, task, false)
	cancel()
	select {
	case <-task.done:
	case <-time.After(3 * time.Second):
		t.Fatal("child survived MCP shutdown")
	}
}

func TestPrivateURLParsing(t *testing.T) {
	for kind, line := range map[string]string{"review": "Open the local review page: http://127.0.0.1:22/review", "hub": "CanvasExpert-SE hub is running: http://127.0.0.1:44/ (Ctrl+C or the Quit button stops it)", "setup": "Open the local setup page: http://127.0.0.1:33/setup", "students": "Open the local student editor: http://127.0.0.1:55/students"} {
		if sessionURL(kind, line) == "" {
			t.Fatalf("missing %s URL", kind)
		}
	}
	if sessionURL("setup", "Open the local setup page: https://external.example/setup") != "" {
		t.Fatal("non-loopback URL accepted")
	}
}

func TestTransportShutdownSynchronouslyStopsPrivateUI(t *testing.T) {
	s, _, _ := sessionFixture(t)
	if _, err := s.startLocalUI(context.Background(), LocalUIInput{Page: "setup"}); err != nil {
		t.Fatal(err)
	}
	var task *reviewTask
	for _, task = range s.sessions {
	}
	awaitSession(t, task, false)
	s.stopSessions()
	select {
	case <-task.done:
	case <-time.After(3 * time.Second):
		t.Fatal("private UI survived transport shutdown")
	}
	status, err := s.localUIStatus(task.id)
	if err != nil || strings.Contains(status.Text, "private-") {
		t.Fatalf("private status: %s %v", status.Text, err)
	}
}
