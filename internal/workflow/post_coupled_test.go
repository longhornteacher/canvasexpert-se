package workflow

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

func TestLiveCoupledRubricPostsHonestValuesThenComputedGradeWithRecovery(t *testing.T) {
	for _, scenario := range []string{"confirmed", "rubric-response-omitted", "rubric-response-lost", "grade-response-lost", "resubmitted-between-writes"} {
		t.Run(scenario, func(t *testing.T) {
			assignment := fixtureAssignment()
			assignment.Rubric = []Criterion{
				{ID: "c1", Description: "Evidence", Points: json.Number("4"), Ratings: []Rating{{ID: "r4", Points: json.Number("4"), Description: "Complete"}, {ID: "r0", Points: json.Number("0"), Description: "Missing"}}},
				{ID: "c2", Description: "Reasoning", Points: json.Number("6"), Ratings: []Rating{{ID: "r6", Points: json.Number("6"), Description: "Complete"}, {ID: "r0", Points: json.Number("0"), Description: "Missing"}}},
			}
			assignment.RubricSettings.Points = json.Number("10")
			enabled := true
			assignment.RubricAssociation.UseForGrading = &enabled
			live := fixtureSubmission(5)
			var writes []string
			var path string
			s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
				if !strings.Contains(r.URL.Path, "submissions/") {
					if r.Method != "GET" {
						t.Error("posting changed the assignment or coupling")
					}
					_ = json.NewEncoder(w).Encode(assignment)
					return
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
					if e := json.NewDecoder(r.Body).Decode(&req); e != nil {
						t.Fatal(e)
					}
					if req.Rubric != nil {
						writes = append(writes, "rubric")
						if req.Submission != nil || decimalPointer(req.Rubric["c1"].Points).Cmp(domain.MustDecimal("2.5")) != 0 || decimalPointer(req.Rubric["c2"].Points).Cmp(domain.MustDecimal("4")) != 0 {
							t.Fatal("coupled rubric write included grade or inflated criterion values")
						}
						live.RubricAssessment = req.Rubric
						// Canvas's automatic coupled calculation overrides a grade
						// included in the same request with the raw rubric sum.
						raw := json.Number("6.5")
						live.EnteredScore, live.Score = &raw, &raw
						if scenario == "rubric-response-lost" {
							w.WriteHeader(503)
							return
						}
						if scenario == "rubric-response-omitted" {
							response := live
							response.RubricAssessment = nil
							_ = json.NewEncoder(w).Encode(response)
							return
						}
						if scenario == "resubmitted-between-writes" {
							_ = json.NewEncoder(w).Encode(live)
							live.Attempt++
							submitted := "2026-10-08T12:00:00Z"
							live.SubmittedAt = &submitted
							return
						}
					} else if req.Submission != nil {
						writes = append(writes, "grade")
						if req.Submission["posted_grade"] != "13" || live.RubricAssessment == nil {
							t.Fatal("grade was not computed after the honest rubric")
						}
						var persisted domain.ResultsDocument
						if _, e := workspace.ReadDocument(path, &persisted); e != nil {
							t.Fatal(e)
						}
						outcomes := persisted.Students[0].Grade.Outcomes
						if !definite(outcomes.Rubric) || outcomes.Grade.Status != "pending" {
							t.Fatal("rubric and pending grade outcomes were not independently persisted")
						}
						grade := json.Number("13")
						live.EnteredScore, live.Score = &grade, &grade
						if scenario == "grade-response-lost" {
							w.WriteHeader(503)
							return
						}
					} else if req.Comment.Text != "" {
						writes = append(writes, "comment")
						attempt := req.Comment.Attempt
						live.Comments = append(live.Comments, SubmissionComment{ID: 100, Comment: req.Comment.Text, Attempt: &attempt})
					}
				}
				_ = json.NewEncoder(w).Encode(live)
			})
			path = scoringFiles(t, s, 5)
			var doc domain.ResultsDocument
			body, e := workspace.ReadDocument(path, &doc)
			if e != nil {
				t.Fatal(e)
			}
			doc.Assignment, e = Snapshot(assignment, s.Client.Origin())
			if e != nil {
				t.Fatal(e)
			}
			// Coupling may change since pull without changing the scoring basis.
			doc.Assignment.Rubric.UseForGrading = false
			doc.Students[0].Grade.Assessment = domain.MustDecimal("13")
			doc.Students[0].Grade.Components[0].Criteria = []domain.CriterionScore{{CriterionID: "c1", Points: domain.MustDecimal("2.5")}, {CriterionID: "c2", Points: domain.MustDecimal("4")}}
			if e = workspace.WriteDocument(path, doc, body); e != nil {
				t.Fatal(e)
			}
			var packet domain.PacketDocument
			packetPath := filepath.Join(s.Root, filepath.FromSlash(doc.Packet))
			packetBody, e := workspace.ReadDocument(packetPath, &packet)
			if e != nil {
				t.Fatal(e)
			}
			packet.Assignment = doc.Assignment
			if e = workspace.WriteDocument(packetPath, packet, packetBody); e != nil {
				t.Fatal(e)
			}
			first, e := s.Post(context.Background(), path)
			if e != nil {
				t.Fatal(e)
			}
			if scenario == "resubmitted-between-writes" {
				if first.Students[0].Status != "held" || !reflect.DeepEqual(writes, []string{"rubric"}) {
					t.Fatalf("new attempt received an old grade or feedback: %+v %v", first, writes)
				}
				return
			}
			if strings.HasSuffix(scenario, "lost") {
				if first.Students[0].Status != "uncertain" {
					t.Fatalf("lost response was not independently uncertain: %+v", first)
				}
				second, e := s.Post(context.Background(), path)
				if e != nil || second.Students[0].Status != "completed" {
					t.Fatalf("known live outcome did not recover: %+v %v", second, e)
				}
			} else if first.Students[0].Status != "completed" {
				t.Fatalf("coupled assessment incomplete: %+v", first)
			}
			wantOrder := []string{"rubric", "grade", "comment"}
			if scenario == "rubric-response-lost" {
				wantOrder = []string{"rubric", "comment", "grade"}
			}
			if !reflect.DeepEqual(writes, wantOrder) || decimalPointer(live.EnteredScore).Cmp(domain.MustDecimal("13")) != 0 || decimalPointer(live.RubricAssessment["c1"].Points).Cmp(domain.MustDecimal("2.5")) != 0 || decimalPointer(live.RubricAssessment["c2"].Points).Cmp(domain.MustDecimal("4")) != 0 || len(live.Comments) != 1 || !*assignment.RubricAssociation.UseForGrading {
				t.Fatalf("grade, honest rubric, coupling, or action order changed: writes=%v live=%+v", writes, live)
			}
			before := s.Client.Requests()
			replay, e := s.Post(context.Background(), path)
			if e != nil || replay.Students[0].Status != "completed" || s.Client.Requests() != before {
				t.Fatal("completed split actions replayed")
			}
		})
	}
}
