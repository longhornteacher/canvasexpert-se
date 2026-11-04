package mcpserver

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"canvasexpertse/internal/workflow"
)

// TestCommandChild is the stand-in child Engine; it only runs when re-executed by these tests.
func TestCommandChild(t *testing.T) {
	mode := os.Getenv("CSE_CMD_MODE")
	if os.Getenv("CSE_CMD_CHILD") != "1" {
		return
	}
	if mode == "exit3" {
		fmt.Println("Results: synthetic")
		os.Exit(3)
	}
	// "batch": three independent items; a stdin close asks it to stop before the next one.
	if os.Getenv("CSE_STOP_ON_STDIN_CLOSE") == "1" {
		go func() {
			_, _ = io.Copy(io.Discard, os.Stdin)
			workflow.RequestStop()
		}()
	}
	for i := 1; i <= 3; i++ {
		if workflow.StopRequested() {
			fmt.Printf("stopped before item %d\n", i)
			os.Exit(3)
		}
		fmt.Printf("item %d started\n", i)
		time.Sleep(500 * time.Millisecond)
		fmt.Printf("item %d done\n", i)
	}
	os.Exit(0)
}

func childServer(mode string) *Server {
	return &Server{root: os.TempDir(), newChild: func([]string) *exec.Cmd {
		c := exec.Command(os.Args[0], "-test.run=^TestCommandChild$")
		c.Env = append(os.Environ(), "CSE_CMD_CHILD=1", "CSE_CMD_MODE="+mode)
		return c
	}}
}

func TestExit3IsAStructuredResultNotAToolError(t *testing.T) {
	out, err := childServer("exit3").command(context.Background(), "post", "results.md")
	if err != nil || out.Outcome != "partial" || out.State != "incomplete" || !strings.Contains(out.Text, "Results: synthetic") {
		t.Fatalf("exit 3 should return the outcomes as a result: %+v %v", out, err)
	}
	out, err = childServer("exit3").command(context.Background(), "set-file-access", "x")
	if err != nil || out.Outcome != "held" {
		t.Fatalf("a single-target command that held: %+v %v", out, err)
	}
}

// Cancelling during item 2 lets item 2 finish and be reported, and item 3 never starts.
func TestCancelFinishesTheInFlightItemAndDoesNotStartTheNext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(700 * time.Millisecond); cancel() }()
	out, err := childServer("batch").command(ctx, "post", "results.md")
	if err != nil {
		t.Fatalf("a graceful stop is a result, not an error: %v", err)
	}
	for _, want := range []string{"item 1 done", "item 2 started", "item 2 done", "stopped before item 3"} {
		if !strings.Contains(out.Text, want) {
			t.Errorf("missing %q in:\n%s", want, out.Text)
		}
	}
	if strings.Contains(out.Text, "item 3 started") || out.Outcome != "partial" {
		t.Fatalf("the next item must not start after a cancel: %+v", out)
	}
}

func TestTimeLimitStopsGracefullyAndSaysSo(t *testing.T) {
	old := commandTimeout
	commandTimeout = 700 * time.Millisecond
	t.Cleanup(func() { commandTimeout = old })
	out, err := childServer("batch").command(context.Background(), "post", "results.md")
	if err != nil || !strings.Contains(out.Text, "Stopped at the time limit") || strings.Contains(out.Text, "item 3 started") {
		t.Fatalf("time limit: %+v %v", out, err)
	}
}
