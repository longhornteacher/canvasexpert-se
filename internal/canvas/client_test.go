package canvas

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func testClient(t *testing.T, server *httptest.Server) *Client {
	t.Helper()
	c, err := New(server.URL, "SYNTHETIC-PAT")
	if err != nil {
		t.Fatal(err)
	}
	c.http.Transport = countingTransport{server.Client().Transport, &c.requests}
	c.backoff = time.Millisecond
	return c
}
func requireKind(t *testing.T, err error, kind string) *Error {
	t.Helper()
	var ce *Error
	if !errors.As(err, &ce) || ce.Kind != kind {
		t.Fatalf("expected %s, got %v", kind, err)
	}
	if strings.Contains(err.Error(), "SYNTHETIC-PAT") {
		t.Fatal("error exposed credential")
	}
	return ce
}

func TestNewRejectsUnsafeOrigins(t *testing.T) {
	for _, origin := range []string{"http://canvas.example", "https://user:pass@canvas.example", "https://canvas.example/path", "https://canvas.example?secret=yes", "https://canvas.example?", "https://:443", "https://canvas.example#fragment", "//canvas.example"} {
		if _, err := New(origin, "SYNTHETIC-PAT"); err == nil {
			t.Fatal("accepted unsafe origin")
		}
	}
	if _, err := New("https://canvas.example", "token\r\nInjected: yes"); err == nil {
		t.Fatal("accepted header injection")
	}
}

func TestListRelativePaginationUsesRedirectedResponseURL(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/original" {
			http.Redirect(w, r, "/api/v1/courses", http.StatusFound)
			return
		}
		if r.URL.Path != "/api/v1/courses" {
			t.Errorf("unexpected pagination path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("page") == "2" {
			_, _ = io.WriteString(w, `[{"id":2}]`)
			return
		}
		w.Header().Set("Link", `<?page=2>; rel="next"`)
		_, _ = io.WriteString(w, `[{"id":1}]`)
	}))
	defer server.Close()
	c := testClient(t, server)
	var got []struct{ ID int }
	if err := c.List(context.Background(), "/api/v1/original", &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != 1 || got[1].ID != 2 || c.Requests() != 3 {
		t.Fatalf("redirected pagination failed: %#v, requests %d", got, c.Requests())
	}
}

func TestRetryAfterDurationCannotOverflow(t *testing.T) {
	h := make(http.Header)
	h.Set("Retry-After", "9223372036854775807")
	if got := retryDelay(h, time.Millisecond); got != 30*time.Second {
		t.Fatalf("large Retry-After bypassed bounded backoff: %s", got)
	}
}

func TestListCompletesPagination(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer SYNTHETIC-PAT" {
			t.Error("missing authentication")
		}
		if r.URL.Query().Get("page") == "2" {
			_, _ = io.WriteString(w, `[{"id":2}]`)
			return
		}
		w.Header().Set("Link", `</api/v1/courses?page=2>; rel="next", </api/v1/courses?page=2>; rel="last"`)
		_, _ = io.WriteString(w, `[{"id":1}]`)
	}))
	defer server.Close()
	c := testClient(t, server)
	var got []struct {
		ID int `json:"id"`
	}
	if err := c.List(context.Background(), "/api/v1/courses", &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != 1 || got[1].ID != 2 || c.Requests() != 2 {
		t.Fatal("pagination incomplete")
	}
}

func TestHostilePaginationRejectedBeforeSend(t *testing.T) {
	var foreignCalls atomic.Int32
	foreign := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { foreignCalls.Add(1) }))
	defer foreign.Close()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Link", "<"+foreign.URL+`/api/v1/courses>; rel="next"`)
		_, _ = io.WriteString(w, `[{"id":1}]`)
	}))
	defer server.Close()
	c := testClient(t, server)
	got := []int{9}
	requireKind(t, c.List(context.Background(), "/api/v1/courses", &got), "validation")
	if foreignCalls.Load() != 0 || len(got) != 1 || got[0] != 9 {
		t.Fatal("hostile pagination was followed or output changed")
	}
}

func TestDownloadForeignRedirectReceivesNoPAT(t *testing.T) {
	var foreignCalls atomic.Int32
	foreign := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		foreignCalls.Add(1)
		if r.Header.Get("Authorization") != "" {
			t.Error("PAT reached foreign host")
		}
		_, _ = w.Write([]byte{0, 255, 128, 65})
	}))
	defer foreign.Close()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer SYNTHETIC-PAT" {
			t.Error("initial Canvas request was not authenticated")
		}
		http.Redirect(w, r, foreign.URL+"/file", 302)
	}))
	defer server.Close()
	c := testClient(t, server)
	var dst bytes.Buffer
	if err := c.Download(context.Background(), server.URL+"/files/1/download", &dst); err != nil {
		t.Fatal(err)
	}
	if foreignCalls.Load() != 1 || !bytes.Equal(dst.Bytes(), []byte{0, 255, 128, 65}) || c.Requests() != 2 {
		t.Fatal("download did not preserve bytes")
	}
}

func TestAPIRejectsForeignRedirect(t *testing.T) {
	var foreignCalls atomic.Int32
	foreign := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { foreignCalls.Add(1) }))
	defer foreign.Close()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, foreign.URL+"/api/v1/users/self", 302)
	}))
	defer server.Close()
	c := testClient(t, server)
	var got any
	requireKind(t, c.Get(context.Background(), "/api/v1/users/self", &got), "validation")
	if foreignCalls.Load() != 0 {
		t.Fatal("foreign API redirect followed")
	}
}

func TestRateLimitSafeReadRetry(t *testing.T) {
	for _, status := range []int{429, 403} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if calls.Add(1) == 1 {
					w.Header().Set("Retry-After", "0")
					w.WriteHeader(status)
					_, _ = io.WriteString(w, "Rate Limit Exceeded SYNTHETIC-PAT")
					return
				}
				_, _ = io.WriteString(w, `{"id":1}`)
			}))
			defer server.Close()
			c := testClient(t, server)
			var got map[string]int
			if err := c.Get(context.Background(), "/api/v1/users/self", &got); err != nil {
				t.Fatal(err)
			}
			if calls.Load() != 2 || got["id"] != 1 || c.limit != 1 {
				t.Fatal("safe retry or throttling failed")
			}
		})
	}
}

func TestWriteTimeoutUncertainAndNeverReplayed(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		time.Sleep(80 * time.Millisecond)
		_, _ = io.WriteString(w, `{"id":7}`)
	}))
	defer server.Close()
	c := testClient(t, server)
	c.http.Timeout = 20 * time.Millisecond
	var got any
	err := c.Write(context.Background(), "POST", "/api/v1/courses/1/assignments", map[string]any{"assignment": map[string]any{"name": "Synthetic"}}, &got)
	ce := requireKind(t, err, "uncertain")
	if !ce.Uncertain || calls.Load() != 1 || c.Requests() != 1 {
		t.Fatal("ambiguous write replayed or not classified")
	}
}

func TestWriteRateLimitNeverRetried(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(429) }))
	defer server.Close()
	c := testClient(t, server)
	requireKind(t, c.Write(context.Background(), "PUT", "/api/v1/courses/1/assignments/2", map[string]string{"name": "Synthetic"}, nil), "throttled")
	if calls.Load() != 1 {
		t.Fatal("write was replayed")
	}
}

func TestErrorClassificationAndSanitization(t *testing.T) {
	for status, kind := range map[int]string{401: "authentication", 403: "forbidden", 404: "not_found", 422: "validation", 405: "unsupported", 500: "uncertain"} {
		ce := statusError(status, []byte("SYNTHETIC-PAT private body"), true)
		if ce.Kind != kind || strings.Contains(ce.Error(), "SYNTHETIC-PAT") {
			t.Fatal("unsafe or incorrect classification")
		}
	}
}

func TestFourRequestConcurrencyLimit(t *testing.T) {
	var active, maxActive atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := active.Add(1)
		defer active.Add(-1)
		for prev := maxActive.Load(); n > prev; prev = maxActive.Load() {
			if maxActive.CompareAndSwap(prev, n) {
				break
			}
		}
		time.Sleep(25 * time.Millisecond)
		_, _ = io.WriteString(w, `{"id":1}`)
	}))
	defer server.Close()
	c := testClient(t, server)
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var got any
			if err := c.Get(context.Background(), "/api/v1/users/self", &got); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if maxActive.Load() > 4 || maxActive.Load() < 2 {
		t.Fatalf("unexpected concurrency %d", maxActive.Load())
	}
}

func TestPaginationLoopRejected(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Link", `</api/v1/courses>; rel="next"`)
		_, _ = io.WriteString(w, `[]`)
	}))
	defer server.Close()
	c := testClient(t, server)
	var got []any
	requireKind(t, c.List(context.Background(), "/api/v1/courses", &got), "validation")
	if c.Requests() != 1 {
		t.Fatal("pagination loop sent a repeated request")
	}
}

func TestCanceledReadAndInvalidPathSendNothing(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("unexpected request") }))
	defer server.Close()
	c := testClient(t, server)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	requireKind(t, c.Get(ctx, "/api/v1/users/self", nil), "canceled")
	requireKind(t, c.Get(context.Background(), "https://attacker.example/api/v1/users/self", nil), "validation")
	if c.Requests() != 0 {
		t.Fatal("invalid request sent")
	}
}

func TestListFieldEnvelopePagination(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "2" {
			_, _ = io.WriteString(w, `{"quiz_submissions":[{"id":2}]}`)
			return
		}
		w.Header().Set("Link", `<?page=2>; rel="next"`)
		_, _ = io.WriteString(w, `{"quiz_submissions":[{"id":1}]}`)
	}))
	defer server.Close()
	c := testClient(t, server)
	var rows []struct{ ID int }
	if e := c.ListField(context.Background(), "/api/v1/quizzes/1/submissions", "quiz_submissions", &rows); e != nil || len(rows) != 2 || rows[1].ID != 2 {
		t.Fatalf("rows=%v error=%v", rows, e)
	}
	for _, body := range []string{`{}`, `{"quiz_submissions":null}`, `{"quiz_submissions":{}}`} {
		bad := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, body) }))
		e := testClient(t, bad).ListField(context.Background(), "/api/v1/quizzes/1/submissions", "quiz_submissions", &rows)
		bad.Close()
		requireKind(t, e, "validation")
		if len(rows) != 2 {
			t.Fatal("failed response changed output")
		}
	}
}
