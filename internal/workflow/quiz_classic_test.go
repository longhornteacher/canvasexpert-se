package workflow

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

const classicFixture = `{
  "version": "3.0-json", "title": "Classic Check", "quiz_engine": "classic", "instructions": "Go — now.",
  "metadata": {"canvas": {"published": true, "module_id": 9, "time_limit_minutes": 20, "has_time_limit": true, "post_to_sis": false}},
  "items": [
    {"id": "S1", "type": "STIMULUS", "prompt": "<p>Passage.</p>"},
    {"id": "Q1", "type": "MC", "stimulus_id": "S1", "points": 2, "prompt": "<p>Which?</p>",
     "choices": [{"id":"A","text":"Yes","correct":true},{"id":"B","text":"No","correct":false}]},
    {"type": "STIMULUS_END"},
    {"id": "Q2", "type": "TF", "points": 1, "prompt": "True?", "answer": false},
    {"id": "Q3", "type": "FITB", "points": 1, "prompt": "The [blank] is a star.", "accept": ["sun"]},
    {"id": "Q4", "type": "FITB", "points": 2, "prompt": "[blank1] and [blank2]", "accept": [["a"],["b","c"]]},
    {"id": "Q5", "type": "NUMERICAL", "points": 1, "prompt": "2+2", "answer": 4, "evaluation": {"mode": "absolute_margin", "value": 0.5}},
    {"id": "Q6", "type": "MATCHING", "points": 2, "prompt": "Match", "pairs": [{"left":"x","right":"1"},{"left":"y","right":"2"}], "distractors": ["3","4"]},
    {"id": "Q7", "type": "ESSAY", "points": 10, "prompt": "Explain."},
    {"id": "Q8", "type": "FILEUPLOAD", "points": 5, "prompt": "Upload."}
  ],
  "rationales": [
    {"item_id": "Q1", "choices": [{"id":"A","rationale":"Right."}]},
    {"item_id": "Q2", "rationale": "Because."}
  ]
}`

func TestClassicTransformShapes(t *testing.T) {
	q, e := ReadQuizForge(writeQF(t, classicFixture))
	if e != nil {
		t.Fatal(e)
	}
	qs := q.classicQuestions()
	if len(qs) != 8 {
		t.Fatalf("expected 8 questions, got %d", len(qs))
	}
	get := func(i int) map[string]any { return qs[i]["question"].(map[string]any) }
	mc := get(0)
	if mc["question_type"] != "multiple_choice_question" || !strings.HasPrefix(mc["question_text"].(string), "<p>Passage.</p><p>Which?") || mc["points_possible"] != 2.0 {
		t.Errorf("mc wrong: %v", mc)
	}
	a := mc["answers"].([]any)
	a0, a1 := a[0].(map[string]any), a[1].(map[string]any)
	if a0["answer_weight"] != 100 || a0["answer_comment_html"] != "<p>Right.</p>" || a1["answer_weight"] != 0 {
		t.Errorf("mc answers wrong: %v", a)
	}
	if get(1)["neutral_comments_html"] != "<p>Because.</p>" {
		t.Errorf("tf rationale missing: %v", get(1))
	}
	if sa := get(2); sa["question_type"] != "short_answer_question" || sa["question_text"] != "The _____ is a star." {
		t.Errorf("single blank wrong: %v", sa)
	}
	multi := get(3)
	if multi["question_type"] != "fill_in_multiple_blanks_question" || len(multi["answers"].([]any)) != 3 ||
		multi["answers"].([]any)[2].(map[string]any)["blank_id"] != "blank2" {
		t.Errorf("multi blank wrong: %v", multi)
	}
	row := get(4)["answers"].([]any)[0].(map[string]any)
	if row["numerical_answer_type"] != "exact_answer" || row["answer_error_margin"] != json.Number("0.5") {
		t.Errorf("numerical wrong: %v", row)
	}
	if get(5)["matching_answer_incorrect_matches"] != "3\n4" {
		t.Errorf("distractors wrong: %v", get(5))
	}
	if get(6)["question_type"] != "essay_question" || get(6)["answers"] != nil || get(7)["question_type"] != "file_upload_question" || get(7)["points_possible"] != 5.0 {
		t.Errorf("writing questions wrong: %v %v", get(6), get(7))
	}
}

func TestClassicRefusals(t *testing.T) {
	wrap := func(item string) string {
		return `{"title":"x","quiz_engine":"classic","items":[` + item + `]}`
	}
	cases := map[string]string{
		"ordering":        wrap(`{"type":"ORDERING","prompt":"q","points":1,"items":["a","b"]}`),
		"categorization":  wrap(`{"type":"CATEGORIZATION","prompt":"q","points":1}`),
		"wordbank":        wrap(`{"type":"FITB","prompt":"[blank]","points":1,"accept":["a"],"answer_mode":"wordbank","options":["a","b"]}`),
		"case sensitive":  wrap(`{"type":"FITB","prompt":"[blank]","points":1,"accept":["a"],"case_sensitive":true}`),
		"decimal places":  wrap(`{"type":"NUMERICAL","prompt":"q","points":1,"answer":1,"evaluation":{"mode":"decimal_places","value":2}}`),
		"essay no points": wrap(`{"type":"ESSAY","prompt":"q"}`),
		"essay rationale": `{"title":"x","quiz_engine":"classic","items":[{"id":"E","type":"ESSAY","prompt":"q","points":1}],"rationales":[{"item_id":"E","rationale":"r"}]}`,
		"bad engine":      `{"title":"x","quiz_engine":"legacy","items":[{"type":"TF","prompt":"q","points":1,"answer":true}]}`,
	}
	for name, body := range cases {
		if _, e := ReadQuizForge(writeQF(t, body)); e == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	// New Quiz files still reject written responses.
	if _, e := ReadQuizForge(writeQF(t, `{"title":"x","items":[{"type":"ESSAY","prompt":"q","points":1}]}`)); e == nil {
		t.Error("New Quiz accepted an essay")
	}
	for name, s := range map[string]QuizSettings{
		"calculator": {CalculatorType: "basic"}, "cooldown": {AttemptCooldown: 5},
		"build": {BuildOnLastAttempt: true}, "first": {ScoreToKeep: "first"},
	} {
		if s.classicSettingProblem() == nil {
			t.Errorf("%s setting accepted", name)
		}
	}
}

func TestPushClassicQuizCreatesResumesAndFinishes(t *testing.T) {
	var creates, qPosts, quizPuts, assignPuts, modulePosts int
	existing := 0
	var put map[string]any
	var module map[string]any
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/quizzes":
			_, _ = io.WriteString(w, `[{"id":3,"assignment_id":4,"title":"Other quiz"}]`)
		case r.Method == "POST" && r.URL.Path == "/api/v1/courses/1/quizzes":
			creates++
			_, _ = io.WriteString(w, `{"id":55,"assignment_id":66}`)
		case r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/assignments/66":
			_, _ = io.WriteString(w, `{"id":66}`)
		case r.Method == "POST" && r.URL.Path == "/api/v1/courses/1/quizzes/55/questions":
			qPosts++
			if qPosts == 3 && existing == 0 {
				w.WriteHeader(422)
				return
			}
			_, _ = io.WriteString(w, `{"id":1}`)
		case r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/quizzes/55/questions":
			_, _ = io.WriteString(w, `[`+strings.TrimSuffix(strings.Repeat(`{},`, existing), ",")+`]`)
		case r.Method == "PUT" && r.URL.Path == "/api/v1/courses/1/quizzes/55":
			quizPuts++
			b, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(b, &put)
			_, _ = io.WriteString(w, `{"id":55,"points_possible":24,"published":true}`)
		case r.Method == "PUT" && r.URL.Path == "/api/v1/courses/1/assignments/66":
			assignPuts++
			_, _ = io.WriteString(w, `{"id":66}`)
		case r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/modules/9/items":
			_, _ = io.WriteString(w, `[]`)
		case r.Method == "POST" && r.URL.Path == "/api/v1/courses/1/modules/9/items":
			modulePosts++
			b, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(b, &module)
			_, _ = io.WriteString(w, `{"id":1}`)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	})
	path := writeQF(t, classicFixture)
	report, e := s.PushQuiz(t.Context(), "TEST", path)
	if e == nil || report.Status != "partial" || report.ItemsAdded != 2 || report.QuizID != 55 || report.AssignmentID != 66 || quizPuts != 0 {
		t.Fatalf("expected partial after two questions, got %+v, %v", report, e)
	}
	b, _ := os.ReadFile(sidecarPath(path))
	if !strings.Contains(string(b), `"quiz_id":55`) || !strings.Contains(string(b), `"assignment_id":66`) {
		t.Errorf("sidecar wrong: %s", b)
	}
	existing = 2
	report, e = s.PushQuiz(t.Context(), "TEST", path)
	if e != nil || report.Status != "completed" || report.ItemsAdded != 8 || creates != 1 || quizPuts != 1 || modulePosts != 1 || assignPuts != 1 || !report.Published || report.Points != 24 {
		t.Fatalf("resume wrong: %+v, %v creates=%d puts=%d modules=%d", report, e, creates, quizPuts, modulePosts)
	}
	f := put["quiz"].(map[string]any)
	if f["published"] != true || f["time_limit"] != 20.0 || f["allowed_attempts"] != 1.0 {
		t.Errorf("quiz settings wrong: %v", f)
	}
	if m := module["module_item"].(map[string]any); m["type"] != "Quiz" || m["content_id"] != 55.0 {
		t.Errorf("module item wrong: %v", m)
	}
}

func TestPushClassicQuizRefusesDuplicateTitleAndOldSidecar(t *testing.T) {
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			_, _ = io.WriteString(w, `[{"id":77,"assignment_id":78,"title":"Classic Check"}]`)
			return
		}
		t.Errorf("must not write: %s %s", r.Method, r.URL.Path)
	})
	path := writeQF(t, classicFixture)
	if _, e := s.PushQuiz(t.Context(), "TEST", path); e == nil || !strings.Contains(e.Error(), `"quiz_id": 77`) || !strings.Contains(e.Error(), `"assignment_id": 78`) {
		t.Fatalf("expected duplicate-title refusal, got %v", e)
	}
	_ = os.WriteFile(sidecarPath(path), []byte(`{"assignment_id":5}`), 0o644)
	if _, e := s.PushQuiz(t.Context(), "TEST", path); e == nil || !strings.Contains(e.Error(), "quiz_id") {
		t.Fatalf("expected refusal of a New Quiz record, got %v", e)
	}
}
