package workflow

import (
	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func noRubricAssignment() Assignment {
	var a Assignment
	_ = json.Unmarshal([]byte(`{"id":10,"course_id":1,"name":"Reflection","points_possible":20,"description":"<p>Reflect on the unit.</p>","grading_type":"points","submission_types":["online_text_entry"],"published":true}`), &a)
	return a
}

// writeNoRubricResults builds a results file for an assignment that has no
// Canvas rubric. basis is the agent's own scoring_basis (nil to omit it).
func writeNoRubricResults(t *testing.T, s *Service, basis *domain.Rubric) string {
	t.Helper()
	snapshot, e := Snapshot(noRubricAssignment(), s.Client.Origin())
	if e != nil {
		t.Fatal(e)
	}
	if snapshot.Rubric != nil {
		t.Fatal("fixture must have no Canvas rubric")
	}
	store, e := workspace.OpenStudents(s.Root)
	if e != nil {
		t.Fatal(e)
	}
	st, e := store.Upsert(s.Client.Origin(), 5, "Jordan Sample", "Jordan", "", "")
	if e != nil {
		t.Fatal(e)
	}
	sub := fixtureSubmission(5)
	sub.EnteredScore, sub.Score = nil, nil
	work := domain.StudentWorkDocument{Schema: 1, Kind: "student-work", StudentRef: st.Ref, Alias: st.Alias, AssignmentID: 10, Current: domain.AttemptWork{Ref: attemptRef(sub), BodyHTML: sub.Body}, EnteredGradeKnown: true}
	rel := "courses/TEST/" + st.Ref + ".md"
	if e = workspace.WriteDocument(filepath.Join(s.Root, filepath.FromSlash(rel)), work, "work"); e != nil {
		t.Fatal(e)
	}
	packet := domain.PacketDocument{Schema: 1, Kind: "packet", Course: "TEST", Assignment: snapshot, Students: []string{rel}}
	g := &domain.GradeOperation{Attempt: work.Current.Ref, Assessment: domain.MustDecimal("12"), Feedback: "{{student_name}}, your reflection names a real change in your thinking.", Components: []domain.ComponentScore{{Key: "reflection", Evidence: "present", Criteria: []domain.CriterionScore{{CriterionID: "overall", Points: domain.MustDecimal("6")}}}}}
	results := domain.ResultsDocument{Schema: 1, Kind: "results", Packet: "courses/TEST/packet.md", Assignment: snapshot, ScoringBasis: basis,
		Interpretation: domain.Interpretation{Mapping: "repeated", Rationale: "One reflection, scored on overall quality.", Components: []domain.Component{{Key: "reflection", Label: "Reflection", Weight: domain.MustDecimal("1")}}},
		Students:       []domain.StudentResult{{StudentRef: st.Ref, Operation: "grade", Grade: g}}}
	if e = store.Save(); e != nil {
		t.Fatal(e)
	}
	if e = workspace.WriteDocument(filepath.Join(s.Root, "courses", "TEST", "packet.md"), packet, "packet"); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(s.Root, "courses", "TEST", "results.md")
	if e = workspace.WriteDocument(path, results, "results"); e != nil {
		t.Fatal(e)
	}
	return path
}

func overallBasis() *domain.Rubric {
	return &domain.Rubric{Title: "Overall quality", Criteria: []domain.Criterion{{ID: "overall", Description: "Overall quality", Points: domain.MustDecimal("10")}}}
}

func TestNoRubricAssignmentScoresWithAgentBasisAndWritesGradeAndCommentOnly(t *testing.T) {
	live := fixtureSubmission(5)
	live.EnteredScore, live.Score = nil, nil
	rubricWrites, gradeWrites := 0, 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "submissions/5") {
			if r.Method == "PUT" {
				var request struct {
					Rubric     map[string]RubricScore `json:"rubric_assessment"`
					Submission map[string]any         `json:"submission"`
					Comment    struct {
						Text    string `json:"text_comment"`
						Attempt int    `json:"attempt"`
					} `json:"comment"`
				}
				if e := json.NewDecoder(r.Body).Decode(&request); e != nil {
					t.Error(e)
				}
				if request.Rubric != nil {
					rubricWrites++
				}
				if v, ok := request.Submission["posted_grade"]; ok {
					gradeWrites++
					n := json.Number(v.(string))
					live.EnteredScore, live.Score = &n, &n
				}
				if request.Comment.Text != "" {
					a := request.Comment.Attempt
					live.Comments = append(live.Comments, SubmissionComment{ID: 70, Comment: request.Comment.Text, Attempt: &a})
				}
			}
			_ = json.NewEncoder(w).Encode(live)
			return
		}
		_ = json.NewEncoder(w).Encode(noRubricAssignment())
	})
	path := writeNoRubricResults(t, s, overallBasis())
	report, e := s.Post(context.Background(), path)
	if e != nil {
		t.Fatal(e)
	}
	if len(report.Students) != 1 || report.Students[0].Status != "completed" {
		t.Fatalf("expected completed row, got %#v", report)
	}
	if rubricWrites != 0 || gradeWrites != 1 {
		t.Fatalf("no-rubric post must write only grade+comment: rubricWrites=%d gradeWrites=%d", rubricWrites, gradeWrites)
	}
	if live.EnteredScore == nil || live.EnteredScore.String() != "12" {
		t.Fatalf("expected entered grade 12, got %v", live.EnteredScore)
	}
	if len(live.Comments) != 1 || !strings.HasPrefix(live.Comments[0].Comment, "Assessment: 12 / 20\n\n") {
		t.Fatalf("feedback should lead with the score line by default: %#v", live.Comments)
	}
	if report.Students[0].Outcomes.Rubric.Status != "not-requested" {
		t.Fatalf("rubric outcome should be not-requested, got %q", report.Students[0].Outcomes.Rubric.Status)
	}
	// Historical completion survives teacher output and academic-basis edits,
	// while the next submission remains new work.
	var saved domain.ResultsDocument
	if _, e = workspace.ReadDocument(path, &saved); e != nil {
		t.Fatal(e)
	}
	changed, e := Snapshot(noRubricAssignment(), s.Client.Origin())
	if e != nil {
		t.Fatal(e)
	}
	changed.DescriptionHTML = "Teacher changed directions after posting"
	live.Comments[0].Comment = "Teacher revised feedback"
	if !s.completedHint(10, saved.Students[0].StudentRef, live, changed) {
		t.Fatal("completed no-rubric attempt recurred after teacher edits")
	}
	live.State = "graded"
	matches := true
	live.GradeMatchesCurrentSubmission = &matches
	if !liveHandled(live) {
		t.Fatal("live no-rubric completion was missed")
	}
	live.Attempt++
	matches = false
	if s.completedHint(10, saved.Students[0].StudentRef, live, changed) || liveHandled(live) {
		t.Fatal("new no-rubric attempt was hidden")
	}

}

func TestLostAssessmentResponseRecoversOnlyExactScore(t *testing.T) {
	live := fixtureSubmission(5)
	gradeWrites := 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "submissions/5") {
			if r.Method == "PUT" {
				var req struct {
					Submission map[string]string `json:"submission"`
					Comment    struct {
						Text    string `json:"text_comment"`
						Attempt int    `json:"attempt"`
					} `json:"comment"`
				}
				_ = json.NewDecoder(r.Body).Decode(&req)
				if value, ok := req.Submission["posted_grade"]; ok {
					gradeWrites++
					n := json.Number(value)
					live.EnteredScore, live.Score = &n, &n
					w.WriteHeader(500)
					return
				}
				if req.Comment.Text != "" {
					attempt := req.Comment.Attempt
					live.Comments = append(live.Comments, SubmissionComment{ID: 90, Comment: req.Comment.Text, Attempt: &attempt})
				}
			}
			_ = json.NewEncoder(w).Encode(live)
			return
		}
		_ = json.NewEncoder(w).Encode(noRubricAssignment())
	})
	path := writeNoRubricResults(t, s, overallBasis())
	first, e := s.Post(context.Background(), path)
	if e != nil {
		t.Fatal(e)
	}
	if first.Students[0].Outcomes.Grade.Status != "uncertain" {
		t.Fatal("lost grade response was not uncertain")
	}
	second, e := s.Post(context.Background(), path)
	if e != nil {
		t.Fatal(e)
	}
	if gradeWrites != 1 || second.Students[0].Status != "completed" {
		t.Fatal("exact assessment was not recovered without replay")
	}
	var saved domain.ResultsDocument
	body, e := workspace.ReadDocument(path, &saved)
	if e != nil {
		t.Fatal(e)
	}
	saved.Students[0].Grade.Outcomes.Grade.Status = "uncertain"
	if e = workspace.WriteDocument(path, saved, body); e != nil {
		t.Fatal(e)
	}
	higher := json.Number("14")
	live.EnteredScore = &higher
	third, e := s.Post(context.Background(), path)
	if e != nil {
		t.Fatal(e)
	}
	if gradeWrites != 1 || third.Students[0].Outcomes.Grade.Status != "uncertain" {
		t.Fatal("higher score or falsely proved uncertain assessment completed")
	}
}
