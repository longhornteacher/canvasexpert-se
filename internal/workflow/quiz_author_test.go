package workflow

import (
	"fmt"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const qfFixture = `Intro text outside the envelope is ignored.
<QUIZFORGE_JSON>
{
  "version": "3.0-json", "title": "Fear Check", "instructions": "Read carefully — then answer.",
  "metadata": {"canvas": {"time_limit_minutes": 20, "has_time_limit": true, "published": true, "module_id": 9}},
  "items": [
    {"id": "S1", "type": "STIMULUS", "prompt": "<p>The lantern flickered.</p>"},
    {"id": "Q1", "type": "MC", "stimulus_id": "S1", "points": 2, "prompt": "<p>What flickered?</p>",
     "choices": [{"id":"A","text":"Lantern","correct":true},{"id":"B","text":"Sun","correct":false}]},
    {"id": "Q2", "type": "MA", "stimulus_id": "S1", "points": 2, "prompt": "<p>Pick.</p>",
     "choices": [{"text":"a","correct":true},{"text":"b","correct":true},{"text":"c","correct":false}]},
    {"type": "STIMULUS_END"},
    {"id": "Q3", "type": "TF", "points": 1, "prompt": "True?", "answer": true},
    {"id": "Q4", "type": "NUMERICAL", "points": 1, "prompt": "2+2", "answer": 4, "evaluation": {"mode": "absolute_margin", "value": 0.5}},
    {"id": "Q5", "type": "FITB", "points": 1, "prompt": "The [blank] is a star.", "accept": ["sun", "the sun"]},
    {"id": "Q6", "type": "MATCHING", "points": 2, "prompt": "Match", "pairs": [{"left":"x","right":"1"},{"left":"y","right":"2"}], "distractors": ["3"]},
    {"id": "Q7", "type": "ORDERING", "points": 2, "prompt": "Order", "items": ["first", "second"]},
    {"id": "Q8", "type": "CATEGORIZATION", "points": 2, "prompt": "Sort", "categories": ["Fish", "Bird"], "items": [{"label":"Trout","category":"Fish"},{"label":"Eagle","category":"Bird"}]}
  ],
  "rationales": [
    {"item_id": "Q1", "choices": [{"id":"A","rationale":"Yes, the lantern."},{"id":"B","rationale":"No."}]},
    {"item_id": "Q2", "choices": [{"id":"A","rationale":"Right."},{"id":"B","rationale":"Also right."},{"id":"C","rationale":"Wrong."}]},
    {"item_id": "Q3", "rationale": "Because."},
    {"item_id": "Q4", "rationale": "Two plus two is four."},
    {"item_id": "Q5", "rationale": "The sun is a star."},
    {"item_id": "Q6", "rationale": "Each letter pairs with its number."},
    {"item_id": "Q7", "rationale": "First comes before second."},
    {"item_id": "Q8", "rationale": "Trout swim; eagles fly."}
  ]
}
</QUIZFORGE_JSON>`

func writeQF(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "quiz.txt")
	if e := os.WriteFile(p, []byte(body), 0o644); e != nil {
		t.Fatal(e)
	}
	return p
}

func entryOf(p map[string]any) map[string]any {
	return p["item"].(map[string]any)["entry"].(map[string]any)
}

func TestQuizForgeTransformMatchesVerifiedShapes(t *testing.T) {
	q, e := ReadQuizForge(writeQF(t, qfFixture))
	if e != nil {
		t.Fatal(e)
	}
	items := q.itemPayloads()
	if len(items) != 8 {
		t.Fatalf("expected 8 scored items, got %d", len(items))
	}
	mc := entryOf(items[0])
	if !strings.HasPrefix(mc["item_body"].(string), "<p>The lantern flickered.</p><p>What flickered?") {
		t.Errorf("stimulus not inlined: %v", mc["item_body"])
	}
	if len(mc["answer_feedback"].(map[string]any)) != 2 {
		t.Errorf("choice rationales not mapped to answer_feedback: %v", mc["answer_feedback"])
	}
	if entryOf(items[2])["feedback"].(map[string]any)["neutral"] != `<p style="color: #2e7d32;"><strong>Why this is correct:</strong> Because.</p>` {
		t.Errorf("single rationale not mapped to feedback.neutral")
	}
	num := entryOf(items[3])
	if num["scoring_algorithm"] != "Numeric" {
		t.Errorf("numeric must use the Numeric algorithm")
	}
	if v := num["scoring_data"].(map[string]any)["value"].([]any); len(v) != 1 || v[0].(map[string]any)["type"] != "marginOfError" {
		t.Errorf("numeric must be an array of response objects: %v", v)
	}
	fitb := entryOf(items[4])
	sd := fitb["scoring_data"].(map[string]any)["value"].([]any)[0].(map[string]any)["scoring_data"].(map[string]any)
	if sd["edit_distance"] != 1 || sd["ignore_case"] != true {
		t.Errorf("FITB needs edit_distance >= 1 and case-insensitive by default: %v", sd)
	}
	if !strings.Contains(fitb["item_body"].(string), "`Blank`") {
		t.Errorf("blank token not converted: %v", fitb["item_body"])
	}
	if p := items[0]["item"].(map[string]any); p["points_possible"] != 2.0 || p["position"] != 1 {
		t.Errorf("points/position wrong: %v", p)
	}
	if strings.Contains(q.Instructions, "—") == false {
		t.Fatal("fixture should contain an em dash")
	}
	if got := normalizeStudentText(q.Instructions); got != "Read carefully - then answer." {
		t.Errorf("em dash not normalized: %q", got)
	}
}

func TestQuizForgeRejectsWhatCanvasCannotDo(t *testing.T) {
	wrap := func(item string) string {
		return `{"title":"x","items":[` + item + `]}`
	}
	cases := map[string]string{
		"essay with rationale":   `{"title":"x","items":[{"id":"E","type":"ESSAY","prompt":"q","points":5}],"rationales":[{"item_id":"E","rationale":"teacher note"}]}`,
		"no points":              wrap(`{"type":"TF","prompt":"q","answer":true}`),
		"two correct MC":         wrap(`{"type":"MC","prompt":"q","points":1,"choices":[{"text":"a","correct":true},{"text":"b","correct":true}]}`),
		"fitb 0 blanks":          wrap(`{"type":"FITB","prompt":"q","points":1,"accept":["a"]}`),
		"bad stim ref":           wrap(`{"type":"TF","prompt":"q","points":1,"answer":true,"stimulus_id":"nope"}`),
		"in-stimulus without id": `{"title":"x","items":[{"id":"S","type":"STIMULUS","prompt":"p"},{"type":"TF","prompt":"q","points":1,"answer":true},{"type":"STIMULUS_END"}]}`,
	}
	for name, body := range cases {
		if _, e := ReadQuizForge(writeQF(t, body)); e == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestPushQuizCreatesResumesAndFinishes(t *testing.T) {
	var creates, itemPosts, puts, modulePosts int
	existing := 0
	var created map[string]any
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/quiz/v1/courses/1/quizzes":
			_, _ = io.WriteString(w, `[{"id":"3","title":"Other quiz"}]`)
		case r.Method == "POST" && r.URL.Path == "/api/quiz/v1/courses/1/quizzes":
			creates++
			b, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(b, &created)
			_, _ = io.WriteString(w, `{"id":"55"}`)
		case r.Method == "POST" && r.URL.Path == "/api/quiz/v1/courses/1/quizzes/55/items":
			itemPosts++
			if itemPosts == 3 && existing == 0 {
				w.WriteHeader(422)
				return
			}
			_, _ = io.WriteString(w, `{"id":"1"}`)
		case r.Method == "GET" && r.URL.Path == "/api/quiz/v1/courses/1/quizzes/55/items":
			_, _ = io.WriteString(w, `[`+strings.TrimSuffix(strings.Repeat(`{},`, existing), ",")+`]`)
		case r.Method == "PUT" && r.URL.Path == "/api/v1/courses/1/assignments/55":
			puts++
			_, _ = io.WriteString(w, `{"id":55,"published":true}`)
		case r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/modules/9/items":
			_, _ = io.WriteString(w, `[]`)
		case r.Method == "POST" && r.URL.Path == "/api/v1/courses/1/modules/9/items":
			modulePosts++
			_, _ = io.WriteString(w, `{"id":1}`)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	})
	path := writeQF(t, qfFixture)
	report, e := s.PushQuiz(t.Context(), "TEST", path)
	if e == nil || report.Status != "partial" || report.ItemsAdded != 2 || report.AssignmentID != 55 || puts != 0 {
		t.Fatalf("expected partial after two items, got %+v, %v", report, e)
	}
	quiz := created["quiz"].(map[string]any)
	if quiz["points_possible"] != 13.0 || quiz["title"] != "Fear Check" {
		t.Errorf("quiz payload wrong: %v", quiz)
	}
	qs := quiz["quiz_settings"].(map[string]any)
	if qs["result_view_settings"].(map[string]any)["display_item_feedback"] != true || qs["session_time_limit_in_seconds"] != 1200.0 {
		t.Errorf("feedback must be visible and time limit applied: %v", qs)
	}
	existing = 2
	report, e = s.PushQuiz(t.Context(), "TEST", path)
	if e != nil || report.Status != "completed" || report.ItemsAdded != 8 || creates != 1 || puts != 1 || modulePosts != 1 || !report.Published {
		t.Fatalf("resume wrong: %+v, %v creates=%d puts=%d modules=%d", report, e, creates, puts, modulePosts)
	}
}

func TestPushQuizRefusesDuplicateTitleWithoutResumeRecord(t *testing.T) {
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			_, _ = io.WriteString(w, `[{"id":"77","title":"Fear Check"}]`)
			return
		}
		t.Errorf("must not write: %s %s", r.Method, r.URL.Path)
	})
	if _, e := s.PushQuiz(t.Context(), "TEST", writeQF(t, qfFixture)); e == nil || !strings.Contains(e.Error(), "77") {
		t.Fatalf("expected duplicate-title refusal naming the existing quiz, got %v", e)
	}
}

// Canvas Live (CS8, 2026-10-10) rejects both written items without scoring_data
// ("can't be blank") and accepts {"value": ""}.
func TestNewQuizWrittenItemsShape(t *testing.T) {
	body := `{"title":"Mixed","items":[
		{"id":"M","type":"MC","prompt":"q","points":2,"choices":[{"text":"a","correct":true},{"text":"b"}]},
		{"id":"E","type":"ESSAY","prompt":"Explain.","points":5,"rubric_hint":"TEACHER ONLY","length_guidance":"TEACHER ONLY","word_limit_max":300},
		{"id":"F","type":"FILEUPLOAD","prompt":"Upload.","points":3,"accepted_formats":[".pdf","docx"]}],
		"rationales":[{"item_id":"M","choices":[{"id":"A","rationale":"Right."},{"id":"B","rationale":"Not it."}]}]}`
	q, e := ReadQuizForge(writeQF(t, body))
	if e != nil {
		t.Fatal(e)
	}
	ps := q.itemPayloads()
	if len(ps) != 3 {
		t.Fatalf("got %d payloads", len(ps))
	}
	if got := q.scoredIDs(); strings.Join(got, ",") != "M,E,F" {
		t.Errorf("labels misaligned: %v", got)
	}
	entry := func(i int) map[string]any { return ps[i]["item"].(map[string]any)["entry"].(map[string]any) }
	essay, up := entry(1), entry(2)
	data := essay["interaction_data"].(map[string]any)
	if essay["interaction_type_slug"] != "essay" || essay["scoring_algorithm"] != "None" || fmt.Sprint(essay["scoring_data"]) != "map[value:]" ||
		data["rce"] != true || data["word_limit_enabled"] != true || data["word_limit_max"] != 300 {
		t.Errorf("essay shape: %v", essay)
	}
	data = up["interaction_data"].(map[string]any)
	if up["interaction_type_slug"] != "file-upload" || up["scoring_algorithm"] != "None" || fmt.Sprint(up["scoring_data"]) != "map[value:]" || data["restrict_types"] != true ||
		fmt.Sprint(data["valid_types"]) != "[pdf docx]" || data["files_count"] != "1" {
		t.Errorf("file upload shape: %v", up)
	}
	if b, _ := json.Marshal(ps); strings.Contains(string(b), "TEACHER ONLY") {
		t.Error("teacher-only text reached the payload")
	}
}

func TestPushQuizWithWrittenItemsKeepsOrderAndResumes(t *testing.T) {
	var posted []map[string]any
	existing := 0
	failAt := 2
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/quiz/v1/courses/1/quizzes":
			_, _ = io.WriteString(w, `[]`)
		case r.Method == "POST" && r.URL.Path == "/api/quiz/v1/courses/1/quizzes":
			_, _ = io.WriteString(w, `{"id":"55"}`)
		case r.Method == "POST" && r.URL.Path == "/api/quiz/v1/courses/1/quizzes/55/items":
			if len(posted) == failAt {
				failAt = -1
				w.WriteHeader(422)
				return
			}
			var m map[string]any
			b, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(b, &m)
			posted = append(posted, m)
			_, _ = io.WriteString(w, `{"id":"1"}`)
		case r.Method == "GET" && r.URL.Path == "/api/quiz/v1/courses/1/quizzes/55/items":
			_, _ = io.WriteString(w, `[`+strings.TrimSuffix(strings.Repeat(`{},`, existing), ",")+`]`)
		default:
			w.WriteHeader(404)
		}
	})
	path := writeQF(t, `{"title":"Mixed","items":[
		{"id":"MC","type":"MC","prompt":"q","points":2,"choices":[{"text":"a","correct":true},{"text":"b"}]},
		{"id":"E","type":"ESSAY","prompt":"Explain.","points":5},
		{"id":"F","type":"FILEUPLOAD","prompt":"Upload.","points":3}],
		"rationales":[{"item_id":"MC","choices":[{"id":"A","rationale":"Right."},{"id":"B","rationale":"Not it."}]}]}`)
	report, e := s.PushQuiz(t.Context(), "TEST", path)
	if e == nil || report.Status != "partial" || report.ItemsAdded != 2 || !strings.Contains(report.Message, "(F)") {
		t.Fatalf("expected partial naming F, got %+v, %v", report, e)
	}
	existing = 2
	report, e = s.PushQuiz(t.Context(), "TEST", path)
	if e != nil || report.ItemsAdded != 3 || len(posted) != 3 {
		t.Fatalf("resume wrong: %+v, %v posted=%d", report, e, len(posted))
	}
	slugs, total := []string{}, 0.0
	for _, p := range posted {
		it := p["item"].(map[string]any)
		slugs = append(slugs, it["entry"].(map[string]any)["interaction_type_slug"].(string))
		total += it["points_possible"].(float64)
	}
	if strings.Join(slugs, ",") != "choice,essay,file-upload" || total != 10 {
		t.Errorf("order/points wrong: %v %v", slugs, total)
	}
}
