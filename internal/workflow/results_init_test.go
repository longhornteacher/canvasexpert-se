package workflow

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

func TestInitResultsReusesBasisFromResultsWithOmittedSnapshot(t *testing.T) {
	s, _ := workflowFixture(t, nil)
	path := scoringFiles(t, s, 5)
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	bare := regexp.MustCompile(`(?m)^assignment:\r?\n(?:[ \t].*\r?\n|\r?\n)*`).ReplaceAll(original, nil)
	if len(bare) == len(original) {
		t.Fatal("fixture has no assignment snapshot to omit")
	}
	if err = os.WriteFile(path, bare, 0600); err != nil {
		t.Fatal(err)
	}
	var prior domain.ResultsDocument
	if _, err = ReadResultsDocument(s.Root, path, &prior); err != nil {
		t.Fatalf("handwritten results should still validate: %v", err)
	}
	rel, err := InitResults(s.Root, "courses/TEST/packet.md", time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	scaffold, err := os.ReadFile(filepath.Join(s.Root, filepath.FromSlash(rel)))
	if err != nil || !strings.Contains(string(scaffold), "reused from courses/TEST/results.md") || !strings.Contains(string(scaffold), prior.Interpretation.Rationale) {
		t.Fatalf("new scaffold lost the existing scoring interpretation: %s %v", scaffold, err)
	}
}

func TestInitResultsFillsMechanicsAndLeavesJudgmentBlank(t *testing.T) {
	live := fixtureSubmission(5)
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/enrollments"):
			_ = json.NewEncoder(w).Encode([]Enrollment{{UserID: 5, Type: "StudentEnrollment", State: "active", User: live.User}})
		case strings.HasSuffix(r.URL.Path, "/submissions"):
			_ = json.NewEncoder(w).Encode([]Submission{live})
		case strings.Contains(r.URL.Path, "/submissions/5"):
			_ = json.NewEncoder(w).Encode(live)
		default:
			_ = json.NewEncoder(w).Encode(fixtureAssignment())
		}
	})
	report, err := s.PullWork(context.Background(), "TEST", "10")
	if err != nil || len(report.Packets) != 1 {
		t.Fatalf("pull: %#v %v", report, err)
	}
	packetRel, _ := filepath.Rel(s.Root, report.Packets[0])
	at := time.Date(2026, 10, 22, 9, 5, 0, 0, time.UTC)
	rel, err := InitResults(s.Root, filepath.ToSlash(packetRel), at)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(rel, "/assignments/10-evidence-essay/scoring/results-20261022-0905.md") {
		t.Fatalf("results path: %s", rel)
	}
	path := filepath.Join(s.Root, filepath.FromSlash(rel))
	b, _ := os.ReadFile(path)
	text := string(b)
	for _, want := range []string{"packet: " + filepath.ToSlash(packetRel), "assignment:", "assignment_id: 10", "operation: grade", "attempt: {attempt: 2, submitted_at: \"2026-10-07T12:00:00Z\"}", "# criteria: c1 Evidence (100)", "criteria: [{criterion_id:", "# work: courses/", "header: none"} {
		if !strings.Contains(text, want) {
			t.Fatalf("scaffold lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "Jordan") {
		t.Fatal("scaffold carried a student name")
	}
	var doc domain.ResultsDocument
	if _, err = workspace.ReadDocument(path, &doc); err == nil {
		t.Fatal("an unfilled scaffold must not validate")
	}
	if _, err = InitResults(s.Root, filepath.ToSlash(packetRel), at); err == nil {
		t.Fatal("a second scaffold in the same minute must not overwrite the first")
	}

	// Filling the judgment makes the same rows valid.
	filled := strings.Replace(text, "interpretation:\n  mapping: \"\" # repeated or direct\n  rationale: \"\"\n  components: [] # {key, label, weight} per required response unit\n",
		"interpretation:\n  mapping: repeated\n  rationale: One response scored with the evidence rubric.\n  components: [{key: answer, label: Answer, weight: 1}]\n", 1)
	filled = strings.Replace(filled, "      components: []\n      feedback: \"\"\n", "      components: [{key: answer, evidence: present, criteria: [{criterion_id: c1, points: 60}]}]\n      feedback: Name the evidence that supports your claim.\n", 1)
	if err = os.WriteFile(path, []byte(filled), 0644); err != nil {
		t.Fatal(err)
	}
	doc = domain.ResultsDocument{}
	if _, err = workspace.ReadDocument(path, &doc); err != nil {
		t.Fatalf("filled scaffold should validate: %v\n%s", err, filled)
	}

	// A later scaffold reuses that interpretation.
	later, err := InitResults(s.Root, filepath.ToSlash(packetRel), at.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(filepath.Join(s.Root, filepath.FromSlash(later)))
	if !strings.Contains(string(b), "reused from "+rel) || !strings.Contains(string(b), "rationale: One response scored with the evidence rubric.") {
		t.Fatalf("earlier interpretation not reused:\n%s", b)
	}
}
