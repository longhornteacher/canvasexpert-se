package canvas

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

type hostRewriteTransport struct {
	destination *url.URL
	base        http.RoundTripper
}

func (t hostRewriteTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	copy := r.Clone(r.Context())
	u := *r.URL
	originalHost := u.Host
	u.Scheme = t.destination.Scheme
	u.Host = t.destination.Host
	copy.URL = &u
	copy.Host = originalHost
	return t.base.RoundTrip(copy)
}

func TestTrustedInstructureOriginAndCanvasENV(t *testing.T) {
	for _, good := range []string{"https://quiz.instructure.com", "https://inst-fs.instructure.com/"} {
		if got, err := trustedInstructureOrigin(good); err != nil || strings.HasSuffix(got, "/") {
			t.Fatalf("trusted host %q: %q %v", good, got, err)
		}
	}
	for _, bad := range []string{"http://quiz.instructure.com", "https://instructure.com", "https://quiz.instructure.com.evil.test", "https://user@quiz.instructure.com", "https://quiz.instructure.com/path"} {
		if _, err := trustedInstructureOrigin(bad); err == nil {
			t.Fatalf("accepted unsafe host %q", bad)
		}
	}
	got, err := canvasENV([]byte(`<script>window.ENV = {"ACCOUNT_ID":7,"NEW_QUIZZES":{"signature":"s","params":{"backend_url":"https://api.instructure.com"}}};</script>`))
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if json.Unmarshal(got, &decoded) != nil {
		t.Fatal("ENV was not JSON")
	}
	if _, err = canvasENV([]byte(`window.ENV = ({ACCOUNT_ID: 7});`)); err == nil {
		t.Fatal("evaluated non-JSON JavaScript")
	}
}

func TestNewQuizFilesKeepsTokensOnTheirHostsAndChecksItemAttempt(t *testing.T) {
	for _, resource := range []string{`"resource"`, `123`} {
		t.Run(resource, func(t *testing.T) {
			var ts *httptest.Server
			ts = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case strings.HasPrefix(r.URL.Path, "/api/v1/courses/1/external_tools/sessionless_launch"):
					if r.Header.Get("Authorization") != "Bearer PAT-secret" {
						t.Errorf("launch PAT missing")
					}
					http.SetCookie(w, &http.Cookie{Name: "launch", Value: "kept", Path: "/"})
					w.Header().Set("Content-Type", "text/html")
					io.WriteString(w, `<script>window.ENV={"ACCOUNT_ID":7,"NEW_QUIZZES":{"signature":"launch-signature","params":{"backend_url":"https://backend.instructure.com"}}};</script>`)
				case r.URL.Path == "/api/v1/jwts":
					if r.Header.Get("Authorization") != "Bearer PAT-secret" || r.Header.Get("Cookie") == "" {
						t.Errorf("Canvas JWT call missing PAT/cookie: %#v", r.Header)
					}
					io.WriteString(w, `{"token":"workflow-jwt"}`)
				case r.URL.Path == "/api/native/launch":
					if r.Header.Get("Authorization") != "Bearer workflow-jwt" || r.Header.Get("Authorization") == "Bearer PAT-secret" || r.Header.Get("X-Domain") != "canvas.instructure.com" {
						t.Errorf("native launch received wrong token")
					}
					io.WriteString(w, `{"access_token":"native-jwt","entry_path":{"resourceId":`+resource+`}}`)
				case r.URL.Path == "/api/assignments/"+strings.Trim(resource, `"`)+"/participants":
					if r.Header.Get("Authorization") != "Bearer native-jwt" || r.Header.Get("Accept") != "application/json" {
						t.Errorf("participants headers wrong: %#v", r.Header)
					}
					io.WriteString(w, `{"data":[{"canvas_user_id":5,"participant_sessions":[{"id":"ps"}]}]}`)
				case r.URL.Path == "/api/participant_sessions/ps/results":
					if r.Header.Get("Authorization") != "Bearer native-jwt" {
						t.Errorf("result lookup token wrong")
					}
					io.WriteString(w, `{"host":"https://quiz.instructure.com","token":"result-token","quiz_api_quiz_session_id":"qs"}`)
				case r.URL.Path == "/api/quiz_sessions/qs":
					if r.Header.Get("Authorization") != "result-token" || r.Header.Get("AuthType") != "Signature" || r.Header.Get("Accept") != "application/json" {
						t.Errorf("quiz session headers wrong: %#v", r.Header)
					}
					io.WriteString(w, `{"authoritative_result":{"id":123}}`)
				case r.URL.Path == "/api/quiz_sessions/qs/results/123/session_item_results":
					io.WriteString(w, `[{"item_id":99,"attempt":1,"scored_data":{"value":[{"id":"f1","name":"answer.docx","url":"https://storage.example/signed","size":4}]}},{"item_id":99,"attempt":2,"scored_data":{"value":[{"id":"f2","name":"wrong-attempt.docx","url":"https://storage.example/other","size":8}]}}]`)
				case r.URL.Path == "/signed":
					if r.Header.Get("Authorization") != "" || r.Header.Get("X-CSE-Download") != "" {
						t.Errorf("signed storage request carried credentials/custom headers")
					}
					w.Write([]byte("bytes"))
				default:
					t.Errorf("unexpected request %s %s host=%s", r.Method, r.URL.Path, r.Host)
					http.NotFound(w, r)
				}
			}))
			defer ts.Close()
			dest, _ := url.Parse(ts.URL)
			c := &Client{token: "PAT-secret", limit: 4, changed: make(chan struct{})}
			c.origin, _ = url.Parse("https://canvas.instructure.com")
			var requests atomic.Int64
			c.http = &http.Client{Transport: countingTransport{hostRewriteTransport{destination: dest, base: ts.Client().Transport}, &requests}}
			files, err := c.NewQuizFiles(context.Background(), 1, 2, 5, 1, map[string]bool{"99": true})
			if err != nil || len(files) != 1 || files[0].Name != "answer.docx" || files[0].Size != 4 {
				t.Fatalf("files %#v, %v", files, err)
			}
			var body strings.Builder
			if err = c.DownloadUnauthenticated(context.Background(), c.Origin()+"/signed", &body); err != nil || body.String() != "bytes" {
				t.Fatalf("signed download %q %v", body.String(), err)
			}
		})
	}
}
