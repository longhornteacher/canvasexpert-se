package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQuizReviewKeepsAnswerKeyOutOfStudentFrame(t *testing.T) {
	path := filepath.Join(t.TempDir(), "quiz.json")
	quiz := `{"version":"3.0-json","title":"Cells","instructions":"<p>Read <a href=\"{{tier:Silver}}\">help</a></p>","quiz_engine":"classic","differentiation":"hub","tiers":[{"label":"Silver","tag":"Silver","supports":"<p>x</p>"}],"items":[
	 {"id":"q1","type":"MC","prompt":"<p>Which organelle makes energy?</p><script>alert(1)</script>","points":2,"choices":[{"id":"A","text":"Nucleus"},{"id":"B","text":"Mitochondrion","correct":true}]},
	 {"id":"q2","type":"TF","prompt":"<p>Cells are alive.</p>","points":1,"answer":true}]}`
	if err := os.WriteFile(path, []byte(quiz), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := BuildReview(t.TempDir(), path)
	if err != nil {
		t.Fatal(err)
	}
	if !m.AllOrNothing || len(m.Cards) != 3 || !strings.Contains(m.Cards[0].Total, "3 points") {
		t.Fatalf("%+v", m.Cards[0])
	}
	item := m.Cards[1]
	frame := item.Frames[0].HTML
	if !strings.Contains(frame, "Nucleus") || !strings.Contains(frame, "Mitochondrion") || strings.Contains(item.Key, "Nucleus") || !strings.Contains(item.Key, "Mitochondrion") {
		t.Fatalf("frame=%q key=%q", frame, item.Key)
	}
	if strings.Contains(m.Cards[0].Frames[0].HTML, "{{tier") {
		t.Fatal("tier token should show a stand-in link")
	}
}

func TestAssignmentReviewRendersDirectionsAndRefusesUpdates(t *testing.T) {
	b, err := os.ReadFile("../../engine-docs/examples/synthetic-assignment-create.md")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "a.md")
	if err = os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := BuildReview(t.TempDir(), path)
	if err != nil || !m.AllOrNothing || len(m.Cards) == 0 || m.Cards[0].Frames[0].HTML == "" {
		t.Fatal(m, err)
	}
}
