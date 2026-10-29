package workflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var hotspotPNG = append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{3}, 64)...)

const hotspotQuiz = `{"title":"Map","items":[
 {"id":"H1","type":"HOTSPOT","points":2,"prompt":"Click the lake.","image":"lake.png",
  "region":{"type":"oval","coordinates":[{"x":0.4,"y":0.5},{"x":0.2,"y":0.1}]}},
 {"id":"T1","type":"TF","points":1,"prompt":"Water?","answer":true}],
 "rationales":[{"item_id":"H1","rationale":"The lake is the blue oval."},{"item_id":"T1","rationale":"A lake holds water."}]}`

// hotspotQuizDir writes a quiz file and its image into one folder.
func hotspotQuizDir(t *testing.T, quiz string, image []byte) string {
	t.Helper()
	dir := t.TempDir()
	if e := os.WriteFile(filepath.Join(dir, "lake.png"), image, 0o644); e != nil {
		t.Fatal(e)
	}
	p := filepath.Join(dir, "quiz.json")
	if e := os.WriteFile(p, []byte(quiz), 0o644); e != nil {
		t.Fatal(e)
	}
	return p
}

func TestHotspotValidationRejectsBadImagesAndRegions(t *testing.T) {
	cases := map[string]struct {
		quiz  string
		image []byte
	}{
		"text file":  {hotspotQuiz, []byte("just words")},
		"oversize":   {hotspotQuiz, append(append([]byte{}, hotspotPNG...), make([]byte, 20<<20)...)},
		"escapes":    {strings.Replace(hotspotQuiz, "lake.png", "../lake.png", 1), hotspotPNG},
		"missing":    {strings.Replace(hotspotQuiz, "lake.png", "gone.png", 1), hotspotPNG},
		"no region":  {strings.Replace(hotspotQuiz, `"region"`, `"x"`, 1), hotspotPNG},
		"off image":  {strings.Replace(hotspotQuiz, `"x":0.4`, `"x":1.4`, 1), hotspotPNG},
		"two points": {strings.Replace(hotspotQuiz, `"oval"`, `"polygon"`, 1), hotspotPNG},
	}
	for name, c := range cases {
		if _, e := ReadQuizForge(hotspotQuizDir(t, c.quiz, c.image)); e == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if _, e := ReadQuizForge(hotspotQuizDir(t, hotspotQuiz, hotspotPNG)); e != nil {
		t.Fatalf("valid hot spot rejected: %v", e)
	}
}

func TestPushQuizHotspotUploadsOnceAndResumes(t *testing.T) {
	var base string
	var posted []map[string]any
	var slots, puts, creates int
	existing := 0
	failItems := true
	s, server := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/quiz/v1/courses/1/quizzes":
			_, _ = io.WriteString(w, `[]`)
		case r.Method == "POST" && r.URL.Path == "/api/quiz/v1/courses/1/quizzes":
			creates++
			_, _ = io.WriteString(w, `{"id":"55"}`)
		case r.Method == "GET" && r.URL.Path == "/api/quiz/v1/courses/1/quizzes/55/items/media_upload_url":
			slots++
			_, _ = io.WriteString(w, `{"url":"`+base+`/media/abc?X-Amz-Signature=SECRET"}`)
		case r.Method == "PUT" && r.URL.Path == "/media/abc":
			puts++
			if r.Header.Get("Authorization") != "" {
				t.Error("token reached storage")
			}
			b, _ := io.ReadAll(r.Body)
			if !bytes.Equal(b, hotspotPNG) {
				t.Error("wrong image bytes")
			}
		case r.Method == "POST" && r.URL.Path == "/api/quiz/v1/courses/1/quizzes/55/items":
			if failItems {
				failItems = false
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
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	})
	base = server.URL
	path := hotspotQuizDir(t, hotspotQuiz, hotspotPNG)

	// The image is accepted, then the item post fails: the rerun must reuse it.
	report, e := s.PushQuiz(t.Context(), "TEST", path)
	if e == nil || report.Status != "partial" || puts != 1 {
		t.Fatalf("expected partial after one upload, got %+v, %v puts=%d", report, e, puts)
	}
	if side, _ := os.ReadFile(sidecarPath(path)); strings.Contains(string(side), "SECRET") || !strings.Contains(string(side), server.URL+"/media/abc") {
		t.Errorf("sidecar must hold the unsigned URL only: %s", side)
	}
	report, e = s.PushQuiz(t.Context(), "TEST", path)
	if e != nil || report.Status != "completed" || report.ItemsAdded != 2 {
		t.Fatalf("resume wrong: %+v, %v", report, e)
	}
	if slots != 1 || puts != 1 || creates != 1 || len(posted) != 2 {
		t.Fatalf("duplicate work: slots=%d puts=%d creates=%d posted=%d", slots, puts, creates, len(posted))
	}
	entry := entryOf(posted[0])
	data := entry["interaction_data"].(map[string]any)
	value := entry["scoring_data"].(map[string]any)["value"].(map[string]any)
	if entry["interaction_type_slug"] != "hot-spot" || entry["scoring_algorithm"] != "HotSpot" || data["image_url"] != server.URL+"/media/abc" ||
		value["type"] != "oval" || len(value["coordinates"].([]any)) != 2 {
		t.Errorf("hot-spot item shape: %v", entry)
	}
	if fb, _ := entry["feedback"].(map[string]any); !strings.Contains(fmt.Sprint(fb["neutral"]), "Why this is correct:") {
		t.Errorf("hot-spot rationale not sent as correct-answer feedback: %v", entry["feedback"])
	}

	if b, _ := json.Marshal(report); strings.Contains(string(b), "SECRET") {
		t.Error("signed URL in report")
	}
}

func TestPushQuizHotspotReuploadsEditedImage(t *testing.T) {
	var base string
	var puts int
	s, server := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/items/media_upload_url"):
			_, _ = io.WriteString(w, `{"url":"`+base+`/media/abc?Signature=S"}`)
		case r.Method == "PUT":
			puts++
		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/items"):
			w.WriteHeader(422)
		case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/quizzes"):
			_, _ = io.WriteString(w, `[]`)
		case r.Method == "POST":
			_, _ = io.WriteString(w, `{"id":"55"}`)
		case r.Method == "GET":
			_, _ = io.WriteString(w, `[]`)
		}
	})
	base = server.URL
	path := hotspotQuizDir(t, hotspotQuiz, hotspotPNG)
	_, _ = s.PushQuiz(t.Context(), "TEST", path)
	_, _ = s.PushQuiz(t.Context(), "TEST", path)
	if puts != 1 {
		t.Fatalf("unchanged image uploaded %d times", puts)
	}
	edited := append(append([]byte{}, hotspotPNG...), 9)
	_ = os.WriteFile(filepath.Join(filepath.Dir(path), "lake.png"), edited, 0o644)
	_, _ = s.PushQuiz(t.Context(), "TEST", path)
	if puts != 2 {
		t.Fatalf("edited image should upload again, puts=%d", puts)
	}
}
