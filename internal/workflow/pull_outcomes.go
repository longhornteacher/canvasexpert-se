package workflow

import (
	"context"
	"errors"
	"fmt"

	"canvasexpertse/internal/canvas"
)

// PullIncompleteError keeps successful packets usable while signaling that
// discovery or retrieval did not finish for every assignment. Err preserves
// parent cancellation or a shared/local failure when one stopped the run.
type PullIncompleteError struct {
	Failed, NotReached int
	Err                error
	StopReason         string
}

func (e *PullIncompleteError) Error() string {
	message := fmt.Sprintf("partial pull: %d failed assignment(s), %d not reached; available packets are preserved", e.Failed, e.NotReached)
	if e.StopReason != "" {
		message += "; stopped: " + e.StopReason
	}
	return message
}

func (e *PullIncompleteError) Unwrap() error { return e.Err }

// Do not copy arbitrary transport/server text into the teacher's report.
func pullFailureReason(err error) string {
	if errors.Is(err, context.Canceled) {
		return "operation cancelled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "operation time limit reached"
	}
	var reportErr *quizReportError
	if errors.As(err, &reportErr) {
		return reportErr.Error()
	}
	var canvasErr *canvas.Error
	if errors.As(err, &canvasErr) {
		if canvasErr.Status > 0 {
			return fmt.Sprintf("%s (HTTP %d)", canvasErr.Error(), canvasErr.Status)
		}
		return canvasErr.Error()
	}
	return "assignment evidence could not be retrieved, matched or saved"
}
