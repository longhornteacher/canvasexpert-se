package workflow

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

const retakeQuizFile = `<QUIZFORGE_JSON>
{"version":"3.0-json","title":"Ignored title","instructions":"<p>Tutorial.</p>",
 "metadata":{"canvas":{"retake_for":40,"access_code":"TUTORIAL"}},
 "items":[{"id":"Q1","type":"TF","points":1,"prompt":"True?","answer":true}],
 "rationales":[{"item_id":"Q1","rationale":"Because."}]}
</QUIZFORGE_JSON>`

func TestPushQuizCreatesProtectedRetake(t *testing.T) {
	var created, assignmentPut map[string]any
	creates := 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/assignments/40":
			_, _ = io.WriteString(w, `{"id":40,"course_id":1,"name":"Essay Test","submission_types":["online_text_entry"]}`)
		case r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/assignments":
			_, _ = io.WriteString(w, `[{"id":40,"course_id":1,"name":"Essay Test"}]`)
		case r.Method == "GET" && r.URL.Path == "/api/quiz/v1/courses/1/quizzes":
			_, _ = io.WriteString(w, `[]`)
		case r.Method == "POST" && r.URL.Path == "/api/quiz/v1/courses/1/quizzes":
			creates++
			b, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(b, &created)
			_, _ = io.WriteString(w, `{"id":"55"}`)
		case r.Method == "POST" && r.URL.Path == "/api/quiz/v1/courses/1/quizzes/55/items":
			_, _ = io.WriteString(w, `{"id":"1"}`)
		case r.Method == "GET" && r.URL.Path == "/api/quiz/v1/courses/1/quizzes/55/items":
			_, _ = io.WriteString(w, `[]`)
		case r.Method == "PUT" && r.URL.Path == "/api/v1/courses/1/assignments/55":
			b, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(b, &assignmentPut)
			_, _ = io.WriteString(w, `{"id":55,"published":false}`)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	})
	report, err := s.PushQuiz(t.Context(), "TEST", writeQF(t, retakeQuizFile))
	if err != nil || report.Status != "completed" || report.Published {
		t.Fatalf("retake push: %+v %v", report, err)
	}
	quiz := created["quiz"].(map[string]any)
	if quiz["title"] != "TUTORIAL-ONLY RETAKE - Essay Test" {
		t.Errorf("title is derived from the original: %v", quiz["title"])
	}
	instructions, _ := quiz["instructions"].(string)
	if retakeOriginalID(instructions) != 40 || !strings.HasPrefix(instructions, "<p>Tutorial.</p>") {
		t.Errorf("instructions must keep their text and carry the marker: %q", instructions)
	}
	qs := quiz["quiz_settings"].(map[string]any)
	if qs["require_student_access_code"] != true || qs["student_access_code"] != "TUTORIAL" {
		t.Errorf("access code not required: %v", qs)
	}
	fields := assignmentPut["assignment"].(map[string]any)
	if fields["only_visible_to_overrides"] != true || fields["omit_from_final_grade"] != true || fields["post_to_sis"] != false {
		t.Errorf("retake not protected: %v", fields)
	}
	if _, published := fields["published"]; published {
		t.Errorf("a retake is never published by its own create: %v", fields)
	}
}

func TestRetakeSettingsRefuseWhatBreaksProtection(t *testing.T) {
	for name, canvas := range map[string]string{
		"no code":   `{"retake_for":40}`,
		"published": `{"retake_for":40,"access_code":"x","published":true}`,
		"module":    `{"retake_for":40,"access_code":"x","module_id":3}`,
		"attempts":  `{"retake_for":40,"access_code":"x","allow_multiple_attempts":true,"allowed_attempts":2}`,
	} {
		body := `{"version":"3.0-json","title":"x","metadata":{"canvas":` + canvas + `},"items":[{"id":"Q1","type":"TF","points":1,"prompt":"q","answer":true}],"rationales":[{"item_id":"Q1","rationale":"r"}]}`
		if _, err := ReadQuizForge(writeQF(t, body)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestRetakePairingByMarkerAndByUniqueTitle(t *testing.T) {
	marked := Assignment{ID: 70, Name: "TUTORIAL-ONLY RETAKE - Essay", Description: "<p>x</p>" + retakeMarker(40)}
	orig := Assignment{ID: 40, Name: "Essay"}
	other := Assignment{ID: 41, Name: "Essay"}
	if got, err := retakeFor([]Assignment{orig, marked}, orig); err != nil || got.ID != 70 {
		t.Fatalf("marker pairing: %v %v", got, err)
	}
	// Two originals with one name: the marker still finds its own, and the other has none.
	if got, err := retakeFor([]Assignment{orig, other, marked}, orig); err != nil || got.ID != 70 {
		t.Fatalf("renamed twin: %v %v", got, err)
	}
	if _, err := retakeFor([]Assignment{orig, other, marked}, other); err == nil {
		t.Fatal("the twin must not claim the retake")
	}
	// A retake that lost its marker pairs by title only while the name is unique.
	bare := Assignment{ID: 71, Name: "TUTORIAL-ONLY RETAKE - Essay"}
	if got, err := retakeFor([]Assignment{orig, bare}, orig); err != nil || got.ID != 71 {
		t.Fatalf("title fallback: %v %v", got, err)
	}
	if _, err := retakeFor([]Assignment{orig, other, bare}, orig); err == nil {
		t.Fatal("title fallback must refuse an ambiguous original name")
	}
	if _, err := retakeFor([]Assignment{orig, marked, bare}, orig); err == nil {
		t.Fatal("two retakes for one original must hold")
	}
}
