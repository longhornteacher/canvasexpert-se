package canvas

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/newquiz_item_scores/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func decodeAny(t *testing.T, b []byte) any {
	t.Helper()
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

// gradingServer fakes Canvas, the New Quizzes backend and the quiz API on one
// TLS server. The launch answers with JSON naming a one-time URL, as Canvas
// did on 2026-10-28.
func gradingServer(t *testing.T, authoritative string, posted *[]byte) (*Client, func()) {
	return gradingServerWith(t, authoritative, posted, nil)
}

// gradingServerWith lets a test choose the answer to the results POST.
func gradingServerWith(t *testing.T, authoritative string, posted *[]byte, respond func(http.ResponseWriter)) (*Client, func()) {
	t.Helper()
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v1/courses/1/external_tools/sessionless_launch"):
			io.WriteString(w, `{"id":1,"name":"Quizzes","url":"https://canvas.instructure.com/courses/1/external_tools/9?display=borderless&verifier=v"}`)
		case r.URL.Path == "/courses/1/external_tools/9":
			w.Header().Set("Content-Type", "text/html")
			io.WriteString(w, `<script>window.ENV={"ACCOUNT_ID":7,"NEW_QUIZZES":{"signature":"sig","params":{"backend_url":"https://backend.instructure.com"}}};</script>`)
		case r.URL.Path == "/api/v1/jwts":
			io.WriteString(w, `{"token":"workflow-jwt"}`)
		case r.URL.Path == "/api/native/launch":
			io.WriteString(w, `{"access_token":"native-jwt","entry_path":{"resourceId":55}}`)
		case r.URL.Path == "/api/participant_sessions/ps1/grade":
			if r.Header.Get("Authorization") != "Bearer native-jwt" || r.Host != "backend.instructure.com" {
				t.Errorf("grade lookup sent the wrong token or host: %s", r.Host)
			}
			io.WriteString(w, `{"host":"https://quiz.instructure.com","token":"grading-token","quiz_api_quiz_session_id":7000001}`)
		case r.URL.Path == "/api/quiz_sessions/7000001":
			if r.Header.Get("Authorization") != "grading-token" || r.Header.Get("AuthType") != "Signature" || r.Host != "quiz.instructure.com" {
				t.Errorf("quiz session read sent the wrong token or host")
			}
			io.WriteString(w, `{"id":"7000001","authoritative_result":`+authoritative+`}`)
		case r.URL.Path == "/api/quiz_sessions/7000001/results/9000001/session_item_results":
			w.Write(fixture(t, "before_session_item_results.json"))
		case r.Method == http.MethodPost && r.URL.Path == "/api/quiz_sessions/7000001/results":
			if r.Header.Get("Authorization") != "grading-token" || r.Header.Get("AuthType") != "Signature" || r.Host != "quiz.instructure.com" {
				t.Errorf("result write sent the wrong token or host")
			}
			*posted, _ = io.ReadAll(r.Body)
			if respond != nil {
				respond(w)
				return
			}
			w.WriteHeader(http.StatusCreated)
			w.Write(fixture(t, "post_results_response.json"))
		default:
			t.Errorf("unexpected request %s %s host=%s", r.Method, r.URL.Path, r.Host)
			http.NotFound(w, r)
		}
	}))
	dest, _ := url.Parse(ts.URL)
	c := &Client{token: "PAT-secret", limit: 4, changed: make(chan struct{})}
	c.origin, _ = url.Parse("https://canvas.instructure.com")
	c.http = &http.Client{Transport: hostRewriteTransport{destination: dest, base: ts.Client().Transport}}
	return c, ts.Close
}

func TestNewQuizGradingPostsTheObservedBody(t *testing.T) {
	var posted []byte
	c, done := gradingServer(t, `{"id":9000001,"fudge_points":0.5}`, &posted)
	defer done()
	g, err := c.NewQuizGrading(context.Background(), 1, 2, "ps1", "7000001")
	if err != nil {
		t.Fatal(err)
	}
	if it, ok := g.Item("1002"); !ok || it.Score != "4.0" || it.GradeStatus != "graded" || it.Attempt != "1" || it.Comment != "" {
		t.Fatalf("read item 1002: %#v %v", it, ok)
	}
	version, err := g.PostScores(context.Background(), map[string]string{"1002": "5"}, map[string]string{"1002": "<p>Synthetic item comment.</p>"})
	if err != nil || version != "9000002" || g.Score != "10.5" || g.PointsPossible != "10.0" {
		t.Fatalf("post: %q %v total %s/%s", version, err, g.Score, g.PointsPossible)
	}
	// SpeedGrader's captured request: score, item comment and fudge points.
	want := decodeAny(t, fixture(t, "post_results_request.json")).(map[string]any)
	if got := decodeAny(t, posted); !reflect.DeepEqual(got, want) {
		gotJSON, _ := json.MarshalIndent(got, "", " ")
		t.Fatalf("posted body differs from the observed request:\n%s", gotJSON)
	}
}

func TestNewQuizGradingHoldsWhatItCannotConfirm(t *testing.T) {
	var posted []byte
	c, done := gradingServer(t, `{"id":9000001}`, &posted)
	defer done()
	if _, err := c.NewQuizGrading(context.Background(), 1, 2, "ps1", "7000099"); err == nil {
		t.Fatal("a different quiz session than Canvas reported was accepted")
	}
	g, err := c.NewQuizGrading(context.Background(), 1, 2, "ps1", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = g.PostScores(context.Background(), map[string]string{"1002": "5"}, nil); err == nil || posted != nil {
		t.Fatal("wrote without knowing the current fudge points")
	}
	g.FudgePoints = json.RawMessage("null")
	if _, err = g.PostScores(context.Background(), map[string]string{"4040": "5"}, nil); err == nil || posted != nil {
		t.Fatal("wrote a score for an item that is not in the result")
	}
}

func TestCamelKeyLeavesIDsAlone(t *testing.T) {
	for in, want := range map[string]string{"item_id": "itemId", "scored_data": "scoredData", "result_score": "resultScore", "score": "score", "381d812b-16f9-4dd1-a794-102b61b4c765": "381d812b-16f9-4dd1-a794-102b61b4c765", "blank_1": "blank_1"} {
		if got := camelKey(in); got != want {
			t.Errorf("camelKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNewQuizGradingClassifiesWriteOutcomesLikeOrdinaryWrites(t *testing.T) {
	cases := []struct {
		name      string
		respond   func(http.ResponseWriter)
		uncertain bool
		kind      string
	}{
		{"408", func(w http.ResponseWriter) { w.WriteHeader(408) }, true, ""},
		{"302 without Location", func(w http.ResponseWriter) { w.WriteHeader(302) }, true, ""},
		{"500", func(w http.ResponseWriter) { w.WriteHeader(500) }, true, ""},
		{"2xx invalid JSON", func(w http.ResponseWriter) { w.WriteHeader(201); io.WriteString(w, "not json") }, true, ""},
		{"2xx unreadable body", func(w http.ResponseWriter) {
			w.Header().Set("Content-Length", "100")
			w.WriteHeader(201)
			io.WriteString(w, "{")
		}, true, ""},
		{"400", func(w http.ResponseWriter) { w.WriteHeader(400) }, false, ""},
		{"403", func(w http.ResponseWriter) { w.WriteHeader(403) }, false, ""},
		{"401", func(w http.ResponseWriter) { w.WriteHeader(401) }, false, "authentication"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var posted []byte
			c, done := gradingServerWith(t, `{"id":9000001,"fudge_points":0.5}`, &posted, tc.respond)
			defer done()
			c.MapError = func(err error) error { return fmt.Errorf("mapped: %w", err) }
			var logged []string
			c.OnWrite = func(method, path string, status int, uncertain bool) {
				logged = append(logged, fmt.Sprint(method, " ", path, " ", uncertain))
			}
			g, err := c.NewQuizGrading(context.Background(), 1, 2, "ps1", "7000001")
			if err != nil {
				t.Fatal(err)
			}
			_, err = g.PostScores(context.Background(), map[string]string{"1002": "5"}, nil)
			var ce *Error
			if !errors.As(err, &ce) {
				t.Fatalf("want a canvas error, got %v", err)
			}
			if !strings.HasPrefix(err.Error(), "mapped:") {
				t.Fatalf("error not mapped: %v", err)
			}
			if ce.Uncertain != tc.uncertain || (tc.kind != "" && ce.Kind != tc.kind) {
				t.Fatalf("uncertain=%v kind=%s, want uncertain=%v kind=%s", ce.Uncertain, ce.Kind, tc.uncertain, tc.kind)
			}
			if len(logged) != 1 || !strings.HasPrefix(logged[0], "POST /api/quiz_sessions/7000001/results ") {
				t.Fatalf("audit hook calls: %v", logged)
			}
		})
	}
}

func TestNewQuizGradingAuditsASuccessfulWrite(t *testing.T) {
	var posted []byte
	c, done := gradingServer(t, `{"id":9000001,"fudge_points":0.5}`, &posted)
	defer done()
	var calls int
	c.OnWrite = func(method, path string, status int, uncertain bool) {
		calls++
		if method != "POST" || status != 200 || uncertain {
			t.Errorf("audit: %s %s %d %v", method, path, status, uncertain)
		}
	}
	g, err := c.NewQuizGrading(context.Background(), 1, 2, "ps1", "7000001")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = g.PostScores(context.Background(), map[string]string{"1002": "5"}, nil); err != nil || calls != 1 {
		t.Fatalf("err=%v audit calls=%d", err, calls)
	}
}
