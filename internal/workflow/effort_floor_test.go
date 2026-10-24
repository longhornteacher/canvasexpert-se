package workflow

import (
	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// markHonestEffort sets the agent's honest-effort judgment on the fixture row.
func markHonestEffort(t *testing.T, path string, honest bool) {
	t.Helper()
	var doc domain.ResultsDocument
	body, e := workspace.ReadDocument(path, &doc)
	if e != nil {
		t.Fatal(e)
	}
	doc.Students[0].Grade.HonestEffort = honest
	if e = workspace.WriteDocument(path, doc, body); e != nil {
		t.Fatal(e)
	}
}

func TestEffortFloorEntersFloorAndKeepsEarnedScoreInComment(t *testing.T) {
	for _, tc := range []struct {
		name    string
		floor   string
		honest  bool
		entered string
		note    bool
	}{
		{"honest attempt below floor", "75", true, "15", true},
		{"not marked honest", "75", false, "12", false},
		{"earned above floor", "50", true, "12", false},
		{"no floor set", "", true, "12", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			live := fixtureSubmission(5)
			live.EnteredScore, live.Score = nil, nil
			s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
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
						if v, ok := req.Submission["posted_grade"]; ok {
							n := json.Number(v)
							live.EnteredScore, live.Score = &n, &n
						}
						if req.Comment.Text != "" {
							a := req.Comment.Attempt
							live.Comments = append(live.Comments, SubmissionComment{ID: 70, Comment: req.Comment.Text, Attempt: &a})
						}
					}
					_ = json.NewEncoder(w).Encode(live)
					return
				}
				_ = json.NewEncoder(w).Encode(noRubricAssignment())
			})
			if tc.floor != "" {
				f := domain.MustDecimal(tc.floor)
				s.Config.Courses[0].EffortFloor = &f
			}
			path := writeNoRubricResults(t, s, overallBasis())
			markHonestEffort(t, path, tc.honest)
			report, e := s.Post(context.Background(), path)
			if e != nil {
				t.Fatal(e)
			}
			if report.Students[0].Status != "completed" {
				t.Fatalf("expected completed row, got %#v", report.Students[0])
			}
			if live.EnteredScore == nil || live.EnteredScore.String() != tc.entered {
				t.Fatalf("entered grade %v, want %s", live.EnteredScore, tc.entered)
			}
			comment := live.Comments[0].Comment
			if !strings.HasPrefix(comment, "Overall: 12/20\n") {
				t.Fatalf("comment must keep the earned score: %q", comment)
			}
			if got := strings.Contains(comment, "Entered as 15/20 (class minimum for an honest attempt)"); got != tc.note {
				t.Fatalf("entered line present=%v, want %v: %q", got, tc.note, comment)
			}
			var saved domain.ResultsDocument
			if _, e = workspace.ReadDocument(path, &saved); e != nil {
				t.Fatal(e)
			}
			if saved.Students[0].Grade.Assessment.String() != "12" {
				t.Fatalf("assessment changed to %s", saved.Students[0].Grade.Assessment.String())
			}
		})
	}
}

func TestEffortFloorRecoversLostResponseAgainstEnteredGrade(t *testing.T) {
	live := fixtureSubmission(5)
	live.EnteredScore, live.Score = nil, nil
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
				if v, ok := req.Submission["posted_grade"]; ok {
					gradeWrites++
					n := json.Number(v)
					live.EnteredScore, live.Score = &n, &n
					w.WriteHeader(500)
					return
				}
				if req.Comment.Text != "" {
					a := req.Comment.Attempt
					live.Comments = append(live.Comments, SubmissionComment{ID: 90, Comment: req.Comment.Text, Attempt: &a})
				}
			}
			_ = json.NewEncoder(w).Encode(live)
			return
		}
		_ = json.NewEncoder(w).Encode(noRubricAssignment())
	})
	f := domain.MustDecimal("75")
	s.Config.Courses[0].EffortFloor = &f
	path := writeNoRubricResults(t, s, overallBasis())
	markHonestEffort(t, path, true)
	if _, e := s.Post(context.Background(), path); e != nil {
		t.Fatal(e)
	}
	second, e := s.Post(context.Background(), path)
	if e != nil {
		t.Fatal(e)
	}
	if gradeWrites != 1 || second.Students[0].Status != "completed" || second.Students[0].Outcomes.Grade.Status != "preserved" {
		t.Fatalf("entered floor was not recovered without replay: writes=%d %#v", gradeWrites, second.Students[0])
	}
}
