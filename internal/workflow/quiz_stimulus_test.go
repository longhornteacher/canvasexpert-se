package workflow

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"canvasexpertse/internal/canvas"
	"canvasexpertse/internal/domain"
)

type fakeEntry struct {
	ID        string `json:"id"`
	EntryType string `json:"entry_type"`
	Stimulus  string `json:"stimulus_quiz_entry_id"`
	Position  int    `json:"position"`
	body      string
	title     string
	layout    string
}

// A quiz with a STIMULUS gets a real Canvas stimulus through the quiz
// editor's session, and the questions that name it are attached to it. A
// failed attachment is completed by the rerun without re-posting anything.
func TestPushQuizCreatesRealStimuliAndResumesAnAttachment(t *testing.T) {
	var entries []*fakeEntry
	creates, stimuli, patches := 0, 0, 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		build := r.Host == "quiz.instructure.com"
		if build && (r.Header.Get("Authorization") != "build-token" || r.Header.Get("AuthType") != "Signature") {
			t.Errorf("quiz editor call without its own token: %s %s", r.Method, r.URL.Path)
		}
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/quiz/v1/courses/1/quizzes":
			io.WriteString(w, `[]`)
		case r.Method == "POST" && r.URL.Path == "/api/quiz/v1/courses/1/quizzes":
			creates++
			io.WriteString(w, `{"id":"55"}`)
		case r.Method == "GET" && r.URL.Path == "/api/quiz/v1/courses/1/quizzes/55/items":
			_ = json.NewEncoder(w).Encode(entries)
		case r.Method == "POST" && r.URL.Path == "/api/quiz/v1/courses/1/quizzes/55/items":
			var p map[string]map[string]any
			_ = json.NewDecoder(r.Body).Decode(&p)
			if _, ok := p["item"]["stimulus_quiz_entry_id"]; ok {
				t.Errorf("the public item create ignores stimulus links; it must not be relied on")
			}
			e := &fakeEntry{ID: fmt.Sprintf("i%d", len(entries)+1), EntryType: "Item", Position: int(p["item"]["position"].(float64)), body: p["item"]["entry"].(map[string]any)["item_body"].(string)}
			entries = append(entries, e)
			_ = json.NewEncoder(w).Encode(e)
		case strings.HasSuffix(r.URL.Path, "/external_tools/sessionless_launch"):
			w.Header().Set("Content-Type", "text/html")
			io.WriteString(w, `<script>window.ENV={"ACCOUNT_ID":7,"NEW_QUIZZES":{"signature":"sig","params":{"backend_url":"https://backend.instructure.com"}}};</script>`)
		case r.URL.Path == "/api/v1/jwts":
			io.WriteString(w, `{"token":"workflow-jwt"}`)
		case r.URL.Path == "/api/native/launch":
			io.WriteString(w, `{"access_token":"native-jwt","entry_path":{"resourceId":555}}`)
		case r.URL.Path == "/api/assignments/555":
			if r.URL.Query().Get("scope") != "quiz.build" || r.Header.Get("Authorization") != "Bearer native-jwt" {
				t.Errorf("editor session must ask for the quiz.build scope with the native token")
			}
			io.WriteString(w, `{"host":"https://quiz.instructure.com","token":"build-token","quiz_api_quiz_id":901}`)
		case build && r.Method == "POST" && r.URL.Path == "/api/quizzes/901/stimuli":
			stimuli++
			var p map[string]map[string]any
			_ = json.NewDecoder(r.Body).Decode(&p)
			entries = append(entries, &fakeEntry{title: p["stimulus"]["title"].(string), body: p["stimulus"]["body"].(string), layout: p["stimulus"]["orientation"].(string)})
			io.WriteString(w, `{"id":"st1"}`)
		case build && r.Method == "POST" && r.URL.Path == "/api/quizzes/901/quiz_entries":
			var p map[string]map[string]any
			_ = json.NewDecoder(r.Body).Decode(&p)
			e := entries[len(entries)-1]
			if p["quiz_entry"]["entry_type"] != "Stimulus" || p["quiz_entry"]["entry_id"] != "st1" {
				t.Errorf("stimulus entry wrong: %v", p)
			}
			e.ID, e.EntryType, e.Position = "e1", "Stimulus", int(p["quiz_entry"]["position"].(float64))
			_ = json.NewEncoder(w).Encode(e)
		case build && r.Method == "PATCH" && strings.HasPrefix(r.URL.Path, "/api/quizzes/901/quiz_entries/"):
			patches++
			if patches == 1 {
				w.WriteHeader(422)
				return
			}
			var p map[string]map[string]any
			_ = json.NewDecoder(r.Body).Decode(&p)
			id := strings.TrimPrefix(r.URL.Path, "/api/quizzes/901/quiz_entries/")
			for _, e := range entries {
				if e.ID == id {
					e.Stimulus = p["quiz_entry"]["stimulus_quiz_entry_id"].(string)
					_ = json.NewEncoder(w).Encode(e)
					return
				}
			}
			t.Errorf("attached an unknown entry %s", id)
		case r.Method == "PUT" && r.URL.Path == "/api/v1/courses/1/assignments/55":
			io.WriteString(w, `{"id":55,"published":true}`)
		case r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/modules/9/items":
			io.WriteString(w, `[]`)
		case r.Method == "POST" && r.URL.Path == "/api/v1/courses/1/modules/9/items":
			io.WriteString(w, `{"id":1}`)
		default:
			t.Errorf("unexpected %s %s host=%s", r.Method, r.URL.Path, r.Host)
			w.WriteHeader(404)
		}
	})
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	dest, _ := url.Parse(server.URL)
	previous := http.DefaultTransport
	http.DefaultTransport = anyHostTransport{destination: dest, base: server.Client().Transport}
	client, err := canvas.New(server.URL, "FIXTURE-ONLY-TOKEN")
	http.DefaultTransport = previous
	if err != nil {
		t.Fatal(err)
	}
	s := &Service{Root: t.TempDir(), Client: client, Config: domain.CourseConfig{Schema: 1, Instance: server.URL, Aliases: true, Courses: []domain.Course{{Nickname: "TEST", ID: 1, Current: true}}}}
	path := writeQF(t, strings.Replace(qfFixture, `"type": "STIMULUS", "prompt"`, `"type": "STIMULUS", "title": "The Lantern", "layout": "below", "prompt"`, 1))

	report, err := s.PushQuiz(t.Context(), "TEST", path)
	if err == nil || report.Status != "partial" || !strings.Contains(report.Message, "not attached to its passage") || len(entries) != 2 {
		t.Fatalf("a refused attachment must stop with a partial report: %+v %v entries=%d", report, err, len(entries))
	}
	report, err = s.PushQuiz(t.Context(), "TEST", path)
	if err != nil || report.Status != "completed" || report.ItemsAdded != 8 || creates != 1 || stimuli != 1 || len(entries) != 9 || len(report.Warnings) != 0 {
		t.Fatalf("rerun should attach the first question and finish: %+v %v creates=%d stimuli=%d entries=%d", report, err, creates, stimuli, len(entries))
	}
	stim := entries[0]
	if stim.EntryType != "Stimulus" || stim.Position != 1 || stim.title != "The Lantern" || stim.layout != "top" || stim.body != "<p>The lantern flickered.</p>" {
		t.Errorf("stimulus wrong: %+v", stim)
	}
	for i, e := range entries[1:] {
		if e.Position != i+2 {
			t.Errorf("entry %s at position %d, want %d", e.ID, e.Position, i+2)
		}
		if strings.Contains(e.body, "lantern flickered") {
			t.Errorf("passage still copied into %s: %s", e.ID, e.body)
		}
		if want := map[bool]string{true: "e1", false: ""}[i < 2]; e.Stimulus != want {
			t.Errorf("entry %s attached to %q, want %q", e.ID, e.Stimulus, want)
		}
	}
}
