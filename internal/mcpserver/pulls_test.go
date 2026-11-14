package mcpserver

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestLongPullReturnsPendingAndRepeatWaitsOnSameRun(t *testing.T) {
	old := pullWait
	pullWait = 20 * time.Millisecond
	defer func() { pullWait = old }()
	release := make(chan struct{})
	starts := 0
	s := &Server{ctx: context.Background(), run: func(_ context.Context, args ...string) (TextOutput, error) {
		starts++
		<-release
		return TextOutput{Text: "pulled " + args[1]}, nil
	}}

	first, err := s.longCommand(context.Background(), "pull-work", "cs8")
	if err != nil || first.State != "pending" {
		t.Fatalf("first call = %+v %v, want pending", first, err)
	}
	again, _ := s.longCommand(context.Background(), "pull-work", "cs8")
	if again.State != "pending" {
		t.Fatalf("repeat call = %+v, want pending", again)
	}
	close(release)
	done, err := s.longCommand(context.Background(), "pull-work", "cs8")
	if err != nil || done.Text != "pulled cs8" || done.State != "" {
		t.Fatalf("final call = %+v %v", done, err)
	}
	if starts != 1 {
		t.Fatalf("pull started %d times, want 1", starts)
	}
	if len(s.pulls) != 0 {
		t.Fatal("collected result was not cleared")
	}
}

func TestLongPullPartialResultKeepsTheSameRun(t *testing.T) {
	old := pullWait
	pullWait = time.Millisecond
	t.Cleanup(func() { pullWait = old })
	release := make(chan struct{})
	var starts atomic.Int32
	s := &Server{ctx: context.Background(), run: func(context.Context, ...string) (TextOutput, error) {
		starts.Add(1)
		<-release
		return TextOutput{State: "incomplete", Outcome: "partial", Text: "available packet; failed assignment 11"}, nil
	}}
	for i := 0; i < 2; i++ {
		out, err := s.longCommand(context.Background(), "pull-work", "Synthetic", "--session")
		if err != nil || out.State != "pending" {
			t.Fatalf("poll = %+v %v", out, err)
		}
	}
	close(release)
	out, err := s.longCommand(context.Background(), "pull-work", "Synthetic", "--session")
	if err != nil || out.State != "incomplete" || out.Outcome != "partial" || out.Text == "" || starts.Load() != 1 || len(s.pulls) != 0 {
		t.Fatalf("partial pull lost or repeated: %+v %v starts=%d", out, err, starts.Load())
	}
}
