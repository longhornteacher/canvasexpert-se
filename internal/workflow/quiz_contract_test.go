package workflow

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestQuizForgeUnknownKeysAndInlineRationales(t *testing.T) {
	base := `{"quizforge_version":"3.0-json","notes":"teacher","title":"Check","items":[{"id":"q1","type":"MC","points":2,"prompt":"Choose.","metadata":{"author":"synthetic"},"rubric_hint":"teacher","notes":"teacher","length_guidance":"teacher","guessed_item":"ignored","choices":[{"id":"a","text":"A","correct":true,"metadata":{},"notes":"teacher","guessed_choice":"ignored"},{"id":"b","text":"B"}]}],"rationales":[{"item_id":"q1","choices":[{"id":"a","rationale":"Right."},{"id":"b","rationale":"Not it."}]}]}`
	q, err := ReadQuizForge(writeQF(t, base))
	if err != nil {
		t.Fatal(err)
	}
	if len(q.Warnings) != 2 || !strings.Contains(q.Warnings[0], "q1") || !strings.Contains(q.Warnings[0], "guessed_item") || !strings.Contains(q.Warnings[1], "choice 1 (a)") || !strings.Contains(q.Warnings[1], "guessed_choice") {
		t.Fatalf("warnings: %v", q.Warnings)
	}
	for _, body := range []string{
		strings.Replace(base, `"guessed_item":"ignored"`, `"rationale":"Feedback"`, 1),
		strings.Replace(base, `"guessed_choice":"ignored"`, `"rationale":"Feedback"`, 1),
	} {
		if _, err := ReadQuizForge(writeQF(t, body)); err == nil || !strings.Contains(err.Error(), "top-level rationales") || !strings.Contains(err.Error(), "q1") || !strings.Contains(err.Error(), "engine_reference") {
			t.Fatalf("inline rationale diagnostic: %v", err)
		}
	}
}

func TestQuizForgeSettingsValidationAndEffectiveReview(t *testing.T) {
	quiz := func(settings string) string {
		return `{"title":"Settings","metadata":{"canvas":` + settings + `},"items":[{"id":"q","type":"TF","points":1,"prompt":"True?","answer":true}],"rationales":[{"item_id":"q","rationale":"It is true."}]}`
	}
	for _, settings := range []string{
		`{"allowed_atempts":2}`, `{"allowed_attempts":1}`, `{"allowed_attempts":2}`,
		`{"allow_multiple_attempts":true,"allowed_attempts":1}`,
		`{"allow_multiple_attempts":true,"allowed_attempts":2.5}`,
		`{"allow_multiple_attempts":true,"allowed_attempts":"guess"}`,
		`{"allow_multiple_attempts":true,"score_to_keep":"guess"}`,
	} {
		if _, err := ReadQuizForge(writeQF(t, quiz(settings))); err == nil {
			t.Errorf("accepted %s", settings)
		}
	}
	for _, tc := range []struct{ settings, attempts, limit string }{
		{`{}`, "1 (single attempt)", "none"},
		{`{"allow_multiple_attempts":true,"allowed_attempts":2,"has_time_limit":true,"time_limit_minutes":10}`, "2; score to keep: highest", "10 minutes"},
		{`{"allow_multiple_attempts":true,"allowed_attempts":"unlimited"}`, "unlimited; score to keep: highest", "none"},
		{`{"time_limit_minutes":10}`, "1 (single attempt)", "none"},
	} {
		m, err := buildQuizReview(writeQF(t, quiz(tc.settings)))
		if err != nil {
			t.Fatal(err)
		}
		facts := map[string]string{}
		for _, f := range m.Cards[0].Facts {
			facts[f.Label] = f.Value
		}
		if facts["Attempts"] != tc.attempts || facts["Time limit"] != tc.limit {
			t.Fatalf("effective settings for %s: %v", tc.settings, facts)
		}
	}
}

func TestEmbeddedQuizForgeExampleRationalesInPayloadAndReview(t *testing.T) {
	path := "../../engine-docs/examples/synthetic-quizforge-mixed.md"
	q, err := ReadQuizForge(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(q.Warnings) != 0 {
		t.Fatalf("canonical example warnings: %v", q.Warnings)
	}
	items := q.itemPayloads()
	if len(items) != 3 || len(entryOf(items[0])["answer_feedback"].(map[string]any)) != 2 || entryOf(items[1])["feedback"].(map[string]any)["neutral"] != `<p style="color: #2e7d32;"><strong>Why this is correct:</strong> Setting gives the time and place of events.</p>` {
		t.Fatalf("canonical feedback payloads: %v", items)
	}
	m, err := buildQuizReview(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Cards[1].Frames) != 2 || len(m.Cards[2].Frames) != 2 || len(m.Cards[3].Frames) != 1 {
		t.Fatalf("review frames: %+v", m.Cards)
	}
	feedback := m.Cards[1].Frames[1]
	if !strings.Contains(feedback.Label, "Student feedback") || !strings.Contains(feedback.HTML, "A library at sunset describes") || strings.Contains(m.Cards[1].Frames[0].HTML, "A library at sunset describes") || strings.Contains(m.Cards[1].Key, "A library at sunset describes") {
		t.Fatalf("feedback not separate from question/key: %+v", m.Cards[1])
	}
	b, _ := json.Marshal(items)
	if strings.Contains(string(b), "Teacher grades") || strings.Contains(string(b), "Assess a clear explanation") {
		t.Fatal("teacher notes reached Canvas payload")
	}
}

func TestPushQuizModuleFailureKeepsUnpublishedAndResumes(t *testing.T) {
	var creates, publishes int
	moduleAlreadyPlaced := false
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/quiz/v1/courses/1/quizzes":
			io.WriteString(w, `[]`)
		case r.Method == "POST" && r.URL.Path == "/api/quiz/v1/courses/1/quizzes":
			creates++
			io.WriteString(w, `{"id":"55"}`)
		case r.Method == "POST" && r.URL.Path == "/api/quiz/v1/courses/1/quizzes/55/items":
			io.WriteString(w, `{"id":"1"}`)
		case r.Method == "GET" && r.URL.Path == "/api/quiz/v1/courses/1/quizzes/55/items":
			io.WriteString(w, `[{}]`)
		case r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/modules/9/items":
			if moduleAlreadyPlaced {
				io.WriteString(w, `[{"type":"Assignment","content_id":55}]`)
			} else {
				io.WriteString(w, `[]`)
			}
		case r.Method == "POST" && r.URL.Path == "/api/v1/courses/1/modules/9/items":
			w.WriteHeader(422)
		case r.Method == "PUT" && r.URL.Path == "/api/v1/courses/1/assignments/55":
			var payload struct {
				Assignment map[string]any `json:"assignment"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			if payload.Assignment["published"] == true {
				if !moduleAlreadyPlaced {
					t.Error("published before requested module completed")
				}
				publishes++
			}
			io.WriteString(w, `{"id":55,"published":true}`)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	})
	path := writeQF(t, `{"title":"Module check","metadata":{"canvas":{"published":true,"module_id":9}},"items":[{"id":"TF","type":"TF","prompt":"True?","points":1,"answer":true}],"rationales":[{"item_id":"TF","rationale":"It is true."}]}`)
	report, err := s.PushQuiz(t.Context(), "TEST", path)
	if err == nil || report.Status != "partial" || report.Published || publishes != 0 || report.Path != sidecarPath(path) || readSidecar(path).AssignmentID != 55 {
		t.Fatalf("failed module result: %+v %v, publishes=%d", report, err, publishes)
	}
	if _, err := os.Stat(report.Path); err != nil {
		t.Fatal(err)
	}
	moduleAlreadyPlaced = true
	report, err = s.PushQuiz(t.Context(), "TEST", path)
	if err != nil || report.Status != "completed" || !report.Published || creates != 1 || publishes != 1 {
		t.Fatalf("resume: %+v %v creates=%d publishes=%d", report, err, creates, publishes)
	}
}
