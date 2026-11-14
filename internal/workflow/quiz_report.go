package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"canvasexpertse/internal/canvas"
)

var quizReportPollInterval = time.Second
var quizReportTimeout = 5 * time.Minute

// quizReportError contains lifecycle facts only. Server payloads, addresses,
// student data and underlying transport messages must never enter Error().
type quizReportError struct {
	Phase, Reason, State    string
	Polls                   int
	Elapsed                 time.Duration
	ProgressURL, ResultsURL bool
	HTTPStatus              int
	Cause                   error
}

func (e *quizReportError) Error() string {
	status := ""
	if e.HTTPStatus > 0 {
		status = fmt.Sprintf("; HTTP %d", e.HTTPStatus)
	}
	return fmt.Sprintf("Student Analysis report: %s (phase=%s; state=%s; polls=%d; elapsed_ms=%d; progress_url_present=%t; results_url_present=%t%s)", e.Reason, e.Phase, e.State, e.Polls, e.Elapsed.Milliseconds(), e.ProgressURL, e.ResultsURL, status)
}

func (e *quizReportError) Unwrap() error { return e.Cause }

func readStudentAnalysis(ctx context.Context, c *canvas.Client, courseID, assignmentID int64) ([]quizReportRow, error) {
	start := time.Now()
	d := quizReportError{Phase: "request", State: "not-reported"}
	fail := func(reason string, cause error) ([]quizReportRow, error) {
		if ctx.Err() != nil {
			cause = ctx.Err()
			reason = "cancelled"
		}
		d.Reason, d.Cause, d.Elapsed = reason, cause, time.Since(start)
		var ce *canvas.Error
		if errors.As(cause, &ce) {
			d.HTTPStatus = ce.Status
		}
		return nil, &d
	}
	var launched struct {
		Progress struct {
			URL string `json:"url"`
		} `json:"progress"`
	}
	path := fmt.Sprintf("/api/quiz/v1/courses/%d/quizzes/%d/reports", courseID, assignmentID)
	if err := c.Write(ctx, "POST", path, map[string]any{"quiz_report": map[string]string{"report_type": "student_analysis", "format": "json"}}, &launched); err != nil {
		return fail("request_failed", err)
	}
	d.ProgressURL = launched.Progress.URL != ""
	progressURL, err := canvasAPIPath(c, launched.Progress.URL)
	if err != nil || !d.ProgressURL {
		return fail("invalid_progress_url", err)
	}
	d.Phase = "poll"
	deadline := start.Add(quizReportTimeout)
	var resultsURL string
	for {
		// A field absent from a later response must not retain its old value.
		var progress quizReportProgress
		d.Polls++
		d.ResultsURL = false
		if err := c.Get(ctx, progressURL, &progress); err != nil {
			return fail("poll_failed", err)
		}
		d.ResultsURL = progress.Results.URL != ""
		switch progress.WorkflowState {
		case "completed", "failed", "error", "queued", "pending", "running":
			d.State = progress.WorkflowState
		default:
			d.State = "unrecognized"
			if progress.WorkflowState == "" {
				return fail("malformed_progress", nil)
			}
		}
		if d.State == "completed" {
			if !d.ResultsURL {
				return fail("completed_without_results_url", nil)
			}
			resultsURL = progress.Results.URL
			break
		}
		if d.State == "failed" || d.State == "error" {
			return fail("terminal_failure", nil)
		}
		if time.Now().After(deadline) {
			return fail("pending_timeout", nil)
		}
		t := time.NewTimer(quizReportPollInterval)
		select {
		case <-ctx.Done():
			t.Stop()
			return fail("cancelled", ctx.Err())
		case <-t.C:
		}
	}
	d.Phase = "download"
	var buf bytes.Buffer
	if err := c.DownloadUnauthenticated(ctx, resultsURL, &buf); err != nil {
		return fail("download_failed", err)
	}
	d.Phase = "parse"
	var rows []quizReportRow
	if err := json.Unmarshal(buf.Bytes(), &rows); err != nil {
		return fail("malformed_results", nil)
	}
	if len(rows) == 0 {
		return fail("empty_results", nil)
	}
	return rows, nil
}
