package mcpserver

import (
	"context"
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
