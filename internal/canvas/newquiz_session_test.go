package canvas

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"
)

// sessionServer fakes the sign-in chain and the grading reads. Each native
// launch issues a new token; expired makes the grade lookup reject the first
// token, and throttle makes the quiz session read answer 429 that many times.
func sessionServer(t *testing.T, launches *atomic.Int32, expired *atomic.Bool, throttle *atomic.Int32) (*Client, func()) {
	t.Helper()
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/courses/1/external_tools/sessionless_launch":
			w.Header().Set("Content-Type", "text/html")
			io.WriteString(w, `<script>window.ENV={"ACCOUNT_ID":7,"NEW_QUIZZES":{"signature":"sig","params":{"backend_url":"https://backend.instructure.com"}}};</script>`)
		case r.URL.Path == "/api/v1/jwts":
			io.WriteString(w, `{"token":"workflow-jwt"}`)
		case r.URL.Path == "/api/native/launch":
			n := launches.Add(1)
			fmt.Fprintf(w, `{"access_token":"native-%d","entry_path":{"resourceId":55}}`, n)
		case r.URL.Path == "/api/participant_sessions/ps1/grade":
			if expired.Load() && r.Header.Get("Authorization") == "Bearer native-1" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			io.WriteString(w, `{"host":"https://quiz.instructure.com","token":"grading-token","quiz_api_quiz_session_id":7000001}`)
		case r.URL.Path == "/api/quiz_sessions/7000001":
			if throttle.Load() > 0 {
				throttle.Add(-1)
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			io.WriteString(w, `{"id":"7000001","authoritative_result":{"id":9000001,"fudge_points":null}}`)
		case r.URL.Path == "/api/quiz_sessions/7000001/results/9000001/session_item_results":
			w.Write(fixture(t, "before_session_item_results.json"))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	dest, _ := url.Parse(ts.URL)
	c := &Client{token: "PAT-secret", limit: 4, changed: make(chan struct{}), backoff: time.Millisecond}
	c.origin, _ = url.Parse("https://canvas.instructure.com")
	c.http = &http.Client{Transport: hostRewriteTransport{destination: dest, base: ts.Client().Transport}}
	return c, ts.Close
}

func TestNewQuizSessionSignsInOncePerAssignment(t *testing.T) {
	var launches atomic.Int32
	var expired atomic.Bool
	var throttle atomic.Int32
	c, done := sessionServer(t, &launches, &expired, &throttle)
	defer done()
	ctx := context.Background()
	s, err := c.NewQuizSession(ctx, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if _, err = s.Grading(ctx, "ps1", "7000001"); err != nil {
			t.Fatal(err)
		}
		if _, err = s.ResultVersion(ctx, "ps1", "7000001"); err != nil {
			t.Fatal(err)
		}
	}
	if launches.Load() != 1 {
		t.Fatalf("signed in %d times for one assignment", launches.Load())
	}
	// An expired native token signs in again once and repeats the read.
	expired.Store(true)
	if _, err = s.Grading(ctx, "ps1", "7000001"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Grading(ctx, "ps1", "7000001"); err != nil {
		t.Fatal(err)
	}
	if launches.Load() != 2 {
		t.Fatalf("an expired token led to %d sign-ins, want 2", launches.Load())
	}
}

func TestNewQuizReadsRetryRateLimits(t *testing.T) {
	var launches atomic.Int32
	var expired atomic.Bool
	var throttle atomic.Int32
	throttle.Store(2)
	c, done := sessionServer(t, &launches, &expired, &throttle)
	defer done()
	s, err := c.NewQuizSession(context.Background(), 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Grading(context.Background(), "ps1", "7000001"); err != nil {
		t.Fatalf("a rate-limited read was not retried: %v", err)
	}
	c.mu.Lock()
	limit := c.limit
	c.mu.Unlock()
	if limit != 1 {
		t.Fatalf("a 429 left %d requests in flight allowed, want 1", limit)
	}
}
