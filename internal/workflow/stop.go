package workflow

import (
	"errors"
	"sync/atomic"
)

// A batch write that is cancelled (the agent app stopped the tool call, or the time limit
// passed) must finish the write it is in the middle of and then stop before starting the next
// independent item. The Engine process sets this flag when its parent asks it to stop; batch
// loops check it only between items, so an in-flight request is never aborted mid-send.
var stopRequested atomic.Bool

// RequestStop asks running batch operations to stop before their next item.
func RequestStop() { stopRequested.Store(true) }

// StopRequested reports whether a stop was requested.
func StopRequested() bool { return stopRequested.Load() }

// ResetStop clears the flag (tests).
func ResetStop() { stopRequested.Store(false) }

// StoppedMessage tells the teacher what a stopped batch means.
const StoppedMessage = "stopped before starting %d item(s) because the operation was cancelled or ran out of time; nothing was lost. Run it again to continue with the items that were not started."

// HeldError marks an operation that held rather than failed: the CLI reports it with exit
// code 3 whatever its wording.
type HeldError struct {
	Msg string
	Err error
}

func (e *HeldError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Msg
}
func (e *HeldError) Unwrap() error { return e.Err }

// IsHeld reports whether err is, or wraps, a HeldError.
func IsHeld(err error) bool {
	var h *HeldError
	return errors.As(err, &h)
}
