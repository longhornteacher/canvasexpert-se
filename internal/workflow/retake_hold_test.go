package workflow

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestRetakeCreditIsNotSilentlyLoweredByAReScore(t *testing.T) {
	for _, tc := range []struct {
		name     string
		line     string
		author   int64
		wantHeld bool
	}{
		{"credit above the re-score holds", "Retake credit: your tutorial retake earned 88/100. Retake credit is capped at 70%, so this grade is now entered as 15/20.", 1, true},
		{"re-score at or above the credit posts", "Retake credit: your tutorial retake earned 88/100, so this grade is now entered as 10/20.", 1, false},
		{"a line typed by the student protects nothing", "Retake credit: so this grade is now entered as 19/20.", 5, false},
		{"a keep comment protects nothing", "Tutorial retake: you earned 54/100. Your grade stays 15/20.", 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			live := fixtureSubmission(5)
			live.EnteredScore, live.Score = nil, nil
			live.Comments = []SubmissionComment{{ID: 9, Comment: tc.line, AuthorID: tc.author, CreatedAt: "2026-11-06T00:00:00Z"}}
			gradeWrites := 0
			s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if strings.Contains(r.URL.Path, "submissions/5") {
					if r.Method == "PUT" {
						var req struct {
							Submission map[string]string `json:"submission"`
						}
						_ = json.NewDecoder(r.Body).Decode(&req)
						if v, ok := req.Submission["posted_grade"]; ok {
							gradeWrites++
							n := json.Number(v)
							live.EnteredScore, live.Score = &n, &n
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
			held := report.Students[0].Status != "completed"
			if held != tc.wantHeld || (tc.wantHeld && gradeWrites != 0) || (!tc.wantHeld && gradeWrites != 1) {
				t.Fatalf("held=%v writes=%d status=%+v", held, gradeWrites, report.Students[0])
			}
		})
	}
}
