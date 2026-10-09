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
  "version": "3.0-json", "title": "Hub Check", "quiz_engine": "classic",
  "instructions": "<p>Everyone takes this. Need help? <a href=\"{{tier:Silver}}\">Silver supports</a> <a href=\"{{tier:Red}}\">Red supports</a></p>",
  "differentiation": "hub",
  "tiers": [
    {"label": "Silver", "tag": "Silver", "supports": "<p>Sentence frames.</p><pre><code>x = 1</code></pre>"},
    {"label": "Red", "tag": "Red", "supports": "<p>Guide.</p>"}
  ],
  "metadata": {"canvas": {"published": true}},
  "items": [{"id": "Q1", "type": "TF", "points": 1, "prompt": "True?", "answer": true}]
}`

// hubQuizCanvas layers the Classic quiz endpoints over the fake pages/tags Canvas.
func hubQuizCanvas(t *testing.T, f *fakeHubCanvas, quizPosted *map[string]any, pagesBeforeQuiz *int) http.HandlerFunc {
	hub := f.handler(t)
	return func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		switch {
		case r.Method == "GET" && p == "/api/v1/courses/1/quizzes":
			_, _ = io.WriteString(w, `[]`)
		case r.Method == "POST" && p == "/api/v1/courses/1/quizzes":
			f.mu.Lock()
			*pagesBeforeQuiz = len(f.pages)
			f.mu.Unlock()
			b, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(b, quizPosted)
			_, _ = io.WriteString(w, `{"id":55,"assignment_id":66}`)
		case r.Method == "GET" && p == "/api/v1/courses/1/assignments/66":
			_, _ = io.WriteString(w, `{"id":66}`)
		case r.Method == "POST" && p == "/api/v1/courses/1/quizzes/55/questions":
			_, _ = io.WriteString(w, `{"id":1}`)
		case r.Method == "GET" && p == "/api/v1/courses/1/quizzes/55/questions":
			_, _ = io.WriteString(w, `[]`)
		case r.Method == "PUT" && p == "/api/v1/courses/1/quizzes/55":
			_, _ = io.WriteString(w, `{"points_possible":1,"published":true}`)
		default:
			hub(w, r)
		}
	}
}

func TestClassicHubCreatesRestrictedPagesBeforeQuizAndPublishesAtTheEnd(t *testing.T) {
	f := newFakeHub()
	f.groups = map[string][]int64{"Silver": {7}} // Red has no tag: page stays restricted, teacher action
	var posted map[string]any
	var pagesBeforeQuiz int
	s, _ := workflowFixture(t, hubQuizCanvas(t, f, &posted, &pagesBeforeQuiz))
	path := writeQF(t, hubQuizFixture)
	report, e := s.PushQuiz(t.Context(), "TEST", path)
	if e != nil || report.Status != "completed" {
		t.Fatalf("push: %+v %v log=%v", report, e, f.log)
	}
	if pagesBeforeQuiz != 2 {
		t.Errorf("both tier pages must exist before the quiz is created, got %d", pagesBeforeQuiz)
	}
	desc := posted["quiz"].(map[string]any)["description"].(string)
	if strings.Contains(desc, "{{tier:") || !strings.Contains(desc, "/courses/1/pages/slug-101") || !strings.Contains(desc, "/courses/1/pages/slug-102") {
		t.Errorf("description links not bound to the verified page slugs: %s", desc)
	}
	silver, red := f.pages[101], f.pages[102]
	if silver.Title != "Hub Check - Silver" || !silver.Published || silver.Everyone || len(silver.Groups) != 1 || silver.Groups[0] != 7 {
		t.Errorf("silver page wrong: %+v", silver)
	}
	if !red.Published || red.Everyone || len(red.Groups) != 0 {
		t.Errorf("unmatched tag page must be restricted with nobody assigned: %+v", red)
	}
	for _, want := range []string{"Hub Check - Silver", "teacher action: assign page \"Hub Check - Red\" to tag \"Red\""} {
		if !strings.Contains(report.Hub, want) {
			t.Errorf("hub summary lacks %q:\n%s", want, report.Hub)
		}
	}
	side, _ := os.ReadFile(sidecarPath(path))
	if !strings.Contains(string(side), `"quiz_id":55`) || !strings.Contains(string(side), "tier_pages") {
		t.Errorf("sidecar must record the quiz and pages: %s", side)
	}
	pagesBefore := len(f.pages)
	if report, e = s.PushQuiz(t.Context(), "TEST", path); e != nil || len(f.pages) != pagesBefore {
		t.Fatalf("rerun must not duplicate pages: %v pages=%d", e, len(f.pages))
	}
}

func TestClassicHubUnpublishedQuizLeavesPagesUnpublished(t *testing.T) {
	f := newFakeHub()
	f.groups = map[string][]int64{"Silver": {7}, "Red": {8}}
	var posted map[string]any
	var n int
	s, _ := workflowFixture(t, hubQuizCanvas(t, f, &posted, &n))
	body := strings.Replace(hubQuizFixture, `"metadata": {"canvas": {"published": true}},`, "", 1)
	if report, e := s.PushQuiz(t.Context(), "TEST", writeQF(t, body)); e != nil {
		t.Fatalf("%+v %v", report, e)
	}
	if f.pages[101].Published || f.pages[102].Published {
		t.Error("pages must stay unpublished while the quiz is unpublished")
	}
}

func TestHubQuizValidation(t *testing.T) {
	cases := map[string]string{
		"new quiz hub":  strings.Replace(hubQuizFixture, `"quiz_engine": "classic",`, "", 1),
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
