package workflow

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

const hubQuizFixture = `{
  "version": "3.0-json", "title": "Hub Check",
  "instructions": "<p>Everyone takes this. Need help? <a href=\"{{tier:Silver}}\">Silver supports</a> <a href=\"{{tier:Red}}\">Red supports</a></p>",
  "differentiation": "hub",
  "tiers": [
    {"label": "Silver", "tag": "Silver", "supports": "<p>Sentence frames.</p><pre><code>x = 1</code></pre>"},
    {"label": "Red", "tag": "Red", "supports": "<p>Guide.</p>"}
  ],
  "metadata": {"canvas": {"published": true}},
  "items": [{"id": "Q1", "type": "TF", "points": 1, "prompt": "True?", "answer": true}],
  "rationales": [{"item_id": "Q1", "rationale": "It is true."}]
}`

func TestRetiredClassicEngineIsRefused(t *testing.T) {
	body := strings.Replace(hubQuizFixture, `"title": "Hub Check",`, `"title": "Hub Check", "quiz_engine": "classic",`, 1)
	if _, e := ReadQuizForge(writeQF(t, body)); e == nil || !strings.Contains(e.Error(), "retired") {
		t.Fatalf("classic engine should be refused as retired: %v", e)
	}
}

func TestHubQuizValidation(t *testing.T) {
	cases := map[string]string{
		"bridge":        strings.Replace(hubQuizFixture, `"hub"`, `"bridge"`, 1),
		"missing token": strings.Replace(hubQuizFixture, `<a href=\"{{tier:Red}}\">Red supports</a>`, "", 1),
		"unknown token": strings.Replace(hubQuizFixture, `{{tier:Red}}`, `{{tier:Gold}}`, 1),
		"no tag":        strings.Replace(hubQuizFixture, `"tag": "Red",`, "", 1),
		"no supports":   strings.Replace(hubQuizFixture, `"supports": "<p>Guide.</p>"`, `"supports": ""`, 1),
		"stray token":   strings.Replace(hubQuizFixture, `"differentiation": "hub",`, "", 1),
	}
	for name, body := range cases {
		if _, e := ReadQuizForge(writeQF(t, body)); e == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestNewQuizHubDeliversPagesBeforeQuizAndKeepsSidecar(t *testing.T) {
	for _, publish := range []bool{true, false} {
		f := newFakeHub()
		f.groups = map[string][]int64{"Silver": {7}, "Red": {8}}
		hub := f.handler(t)
		var posted map[string]any
		var pagesBeforeQuiz int
		var pagePublishedAtQuiz bool
		const base = "/api/quiz/v1/courses/1/quizzes"
		s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
			p := r.URL.Path
			switch {
			case r.Method == "GET" && p == base:
				_, _ = io.WriteString(w, `[]`)
			case r.Method == "POST" && p == base:
				f.mu.Lock()
				pagesBeforeQuiz = len(f.pages)
				pagePublishedAtQuiz = f.pages[101].Published || f.pages[102].Published
				f.mu.Unlock()
				b, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(b, &posted)
				_, _ = io.WriteString(w, `{"id":"55"}`)
			case r.Method == "POST" && p == base+"/55/items":
				_, _ = io.WriteString(w, `{"id":"1"}`)
			case r.Method == "PUT" && p == "/api/v1/courses/1/assignments/55":
				_, _ = io.WriteString(w, `{"id":55,"published":true}`)
			default:
				hub(w, r)
			}
		})
		body := hubQuizFixture
		if !publish {
			body = strings.Replace(body, `"metadata": {"canvas": {"published": true}},`, "", 1)
		}
		path := writeQF(t, body)
		report, e := s.PushQuiz(t.Context(), "TEST", path)
		if e != nil || report.Status != "completed" || report.AssignmentID != 55 {
			t.Fatalf("publish=%v push: %+v %v", publish, report, e)
		}
		if pagesBeforeQuiz != 2 || pagePublishedAtQuiz {
			t.Errorf("pages must exist, unpublished, before the quiz is created: n=%d published=%v", pagesBeforeQuiz, pagePublishedAtQuiz)
		}
		in := posted["quiz"].(map[string]any)["instructions"].(string)
		if strings.Contains(in, "{{tier:") || !strings.Contains(in, "/courses/1/pages/slug-101") || !strings.Contains(in, "/courses/1/pages/slug-102") {
			t.Errorf("instructions links not bound: %s", in)
		}
		if f.pages[101].Published != publish || f.pages[102].Published != publish {
			t.Errorf("pages must publish only with the quiz (publish=%v)", publish)
		}
		if !strings.Contains(report.Hub, "Hub Check - Silver") {
			t.Errorf("hub summary missing: %s", report.Hub)
		}
		side, _ := os.ReadFile(sidecarPath(path))
		if !strings.Contains(string(side), `"assignment_id":55`) || !strings.Contains(string(side), "tier_pages") {
			t.Errorf("sidecar must keep both assignment_id and tier_pages: %s", side)
		}
	}
}
