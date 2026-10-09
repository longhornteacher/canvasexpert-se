package workflow

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

// gradingServer serves the assignment as `assignment` and one live submission per user.
func gradingServer(t *testing.T, assignment *Assignment, users ...int64) (*Service, map[int64]*Submission) {
	t.Helper()
	live := map[int64]*Submission{}
	for _, u := range users {
		sub := fixtureSubmission(u)
		live[u] = &sub
	}
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		for u, sub := range live {
			if !strings.Contains(r.URL.Path, "submissions/"+strconv.FormatInt(u, 10)) {
				continue
			}
			if r.Method == "PUT" {
				var req struct {
					Rubric     map[string]RubricScore `json:"rubric_assessment"`
					Submission map[string]string      `json:"submission"`
					Comment    struct {
						Text    string `json:"text_comment"`
						Attempt int    `json:"attempt"`
					} `json:"comment"`
				}
				_ = json.NewDecoder(r.Body).Decode(&req)
				if v, ok := req.Submission["posted_grade"]; ok {
					n := json.Number(v)
					sub.EnteredScore, sub.Score = &n, &n
				}
				if req.Rubric != nil {
					sub.RubricAssessment = req.Rubric
				}
				if req.Comment.Text != "" {
					a := req.Comment.Attempt
					sub.Comments = append(sub.Comments, SubmissionComment{ID: 100 + u, Comment: req.Comment.Text, Attempt: &a})
				}
			}
			_ = json.NewEncoder(w).Encode(sub)
			return
		}
		_ = json.NewEncoder(w).Encode(assignment)
	})
	return s, live
}

func TestPostIgnoresDirectionsButHoldsOnPoints(t *testing.T) {
	live := fixtureAssignment()
	live.Description = "<p>A completely different paragraph.</p>"
	s, subs := gradingServer(t, &live, 5)
	path := scoringFiles(t, s, 5)
	report, e := s.Post(context.Background(), path)
	if e != nil {
		t.Fatal(e)
	}
	if o := report.Students[0].Outcomes.Grade; o.Status != "succeeded" || subs[5].EnteredScore.String() != "12" {
		t.Fatalf("directions change blocked grading: %+v", o)
	}
	live = fixtureAssignment()
	live.Points = json.Number("40")
	s, _ = gradingServer(t, &live, 5)
	path = scoringFiles(t, s, 5)
	report, e = s.Post(context.Background(), path)
	if e != nil {
		t.Fatal(e)
	}
	if o := report.Students[0].Outcomes.Grade; o.Status != "held" || !strings.Contains(o.Message, "grading basis changed") {
		t.Fatalf("points change did not hold: %+v", o)
	}
}

func TestPostFillsAssessmentAndHoldsUnreadableStudentOnly(t *testing.T) {
	live := fixtureAssignment()
	s, subs := gradingServer(t, &live, 5, 6)
	path := scoringFiles(t, s, 5, 6)
	var doc domain.ResultsDocument
	body, e := workspace.ReadDocument(path, &doc)
	if e != nil {
		t.Fatal(e)
	}
	doc.Students[0].Grade.Assessment = domain.Decimal{}
	doc.Students[1].Grade.Assessment = domain.Decimal{}
	doc.Students[1].Grade.Components[0].Evidence = "unknown"
	if e = workspace.WriteDocument(path, doc, body); e != nil {
		t.Fatal(e)
	}
	report, e := s.Post(context.Background(), path)
	if e != nil {
		t.Fatal(e)
	}
	if report.Students[0].Outcomes.Grade.Status != "succeeded" || subs[5].EnteredScore.String() != "12" {
		t.Fatalf("readable student not posted: %+v", report.Students[0].Outcomes)
	}
	if o := report.Students[1].Outcomes.Grade; o.Status != "held" || !strings.Contains(o.Message, "unreadable") || subs[6].EnteredScore.String() != "15" {
		t.Fatalf("unreadable student not held: %+v", o)
	}
	var saved domain.ResultsDocument
	if _, e = workspace.ReadDocument(path, &saved); e != nil {
		t.Fatal(e)
	}
	if saved.Students[0].Grade.Assessment.String() != "12" {
		t.Fatal("computed assessment was not saved into the results file")
	}
}
