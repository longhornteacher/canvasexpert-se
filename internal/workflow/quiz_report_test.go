package workflow

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestStudentAnalysisLifecycleDiagnostics(t *testing.T) {
	oldInterval, oldTimeout := quizReportPollInterval, quizReportTimeout
	quizReportPollInterval = time.Millisecond
	t.Cleanup(func() { quizReportPollInterval, quizReportTimeout = oldInterval, oldTimeout })
	for _, tc := range []struct {
		name, progress, result, reason, phase string
		httpStatus                            int
	}{
		{"missing URL", `{"workflow_state":"completed"}`, "", "completed_without_results_url", "poll", 0},
		{"failed", `{"workflow_state":"failed","message":"SECRET_PAYLOAD"}`, "", "terminal_failure", "poll", 0},
		{"missing state", `{"results":{"url":"SECRET_URL"}}`, "", "malformed_progress", "poll", 0},
		{"invalid progress", `{"workflow_state":42}`, "", "poll_failed", "poll", 0},
		{"empty", "complete", `[]`, "empty_results", "parse", 0},
		{"malformed results", "complete", `{"student_name":"SECRET_NAME"}`, "malformed_results", "parse", 0},
		{"HTTP error", "", "", "poll_failed", "poll", 403},
		{"pending", `{"workflow_state":"running"}`, "", "pending_timeout", "poll", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			quizReportTimeout = time.Minute
			if tc.reason == "pending_timeout" {
				quizReportTimeout = -time.Second
			}
			var origin string
			s, server := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case strings.HasSuffix(r.URL.Path, "/reports"):
					fmt.Fprint(w, `{"progress":{"url":"/api/quiz/v1/reports/7?secret=SECRET_TOKEN"}}`)
				case r.URL.Path == "/api/quiz/v1/reports/7":
					if tc.httpStatus > 0 {
						w.WriteHeader(tc.httpStatus)
						fmt.Fprint(w, "SECRET_BODY")
						return
					}
					if tc.progress == "complete" {
						fmt.Fprintf(w, `{"workflow_state":"completed","results":{"url":%q}}`, origin+"/signed?token=SECRET_SIGNED")
					} else {
						fmt.Fprint(w, tc.progress)
					}
				case r.URL.Path == "/signed":
					if r.Header.Get("Authorization") != "" {
						t.Error("signed URL received credentials")
					}
					fmt.Fprint(w, tc.result)
				default:
					t.Errorf("unexpected request %s", r.URL.Path)
				}
			})
			origin = server.URL
			rows, err := readStudentAnalysis(context.Background(), s.Client, 1, 10)
			var diag *quizReportError
			if !errors.As(err, &diag) || rows != nil || diag.Reason != tc.reason || diag.Phase != tc.phase || diag.Polls != 1 || diag.Elapsed < 0 || diag.HTTPStatus != tc.httpStatus || !diag.ProgressURL {
				t.Fatalf("incorrect lifecycle facts: rows=%+v diagnostic=%+v error=%v", rows, diag, err)
			}
			if strings.Contains(err.Error(), "SECRET") || strings.Contains(err.Error(), origin) || strings.Contains(err.Error(), "/reports/7") {
				t.Fatalf("private report data leaked: %v", err)
			}
		})
	}
}

func TestStudentAnalysisPollResetsFieldsAndAcceptsUnknownPendingState(t *testing.T) {
	old := quizReportPollInterval
	quizReportPollInterval = time.Millisecond
	t.Cleanup(func() { quizReportPollInterval = old })
	for _, missingURL := range []bool{false, true} {
		t.Run(fmt.Sprint(missingURL), func(t *testing.T) {
			polls := 0
			var origin string
			s, server := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case strings.HasSuffix(r.URL.Path, "/reports"):
					fmt.Fprint(w, `{"progress":{"url":"/api/quiz/v1/reports/7"}}`)
				case r.URL.Path == "/api/quiz/v1/reports/7":
					polls++
					if polls == 1 {
						fmt.Fprint(w, `{"workflow_state":"SECRET_UNKNOWN_PENDING","results":{"url":"https://example.invalid/SECRET_STALE"}}`)
						return
					}
					if missingURL {
						fmt.Fprint(w, `{"workflow_state":"completed"}`)
					} else {
						fmt.Fprintf(w, `{"workflow_state":"completed","results":{"url":%q}}`, origin+"/signed")
					}
				case r.URL.Path == "/signed":
					fmt.Fprint(w, `[{"student_data":{"id":5,"attempt":2},"item_responses":[]}]`)
				default:
					t.Errorf("unexpected request %s", r.URL.Path)
				}
			})
			origin = server.URL
			rows, err := readStudentAnalysis(context.Background(), s.Client, 1, 10)
			if polls != 2 {
				t.Fatalf("unknown nonterminal state was not polled: %d %v", polls, err)
			}
			if missingURL {
				var diag *quizReportError
				if !errors.As(err, &diag) || diag.Reason != "completed_without_results_url" || diag.ResultsURL || strings.Contains(err.Error(), "SECRET") {
					t.Fatalf("stale fields survived: %v", err)
				}
			} else if err != nil || len(rows) != 1 {
				t.Fatalf("pending-to-complete failed: %+v %v", rows, err)
			}
		})
	}
}

func TestStudentAnalysisCancellationPreservesContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) { t.Error("cancelled report should not request Canvas") })
	_, err := readStudentAnalysis(ctx, s.Client, 1, 10)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("context cancellation lost: %v", err)
	}
}
