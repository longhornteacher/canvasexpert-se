package workflow

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"canvasexpertse/internal/canvas"
	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

func workflowFixture(t *testing.T, handler http.HandlerFunc) (*Service, *httptest.Server) {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	// The public client deliberately has no insecure-TLS/test bypass. Trust the
	// fixture's normal TLS certificate through its transport at construction.
	previous := http.DefaultTransport
	http.DefaultTransport = server.Client().Transport
	client, e := canvas.New(server.URL, "FIXTURE-ONLY-TOKEN")
	http.DefaultTransport = previous
	if e != nil {
		t.Fatal(e)
	}
	root := t.TempDir()
	s := &Service{Root: root, Client: client, Config: domain.CourseConfig{Schema: 1, Instance: server.URL, Aliases: true, Courses: []domain.Course{{Nickname: "TEST", ID: 1, Current: true}}}}
	return s, server
}
func fixtureAssignment() Assignment {
	var a Assignment
	_ = json.Unmarshal([]byte(`{"id":10,"course_id":1,"name":"Evidence essay","points_possible":20,"description":"<p>Explain your evidence.</p>","grading_type":"points","submission_types":["online_text_entry"],"published":false,"rubric_settings":{"id":20,"title":"Evidence"},"rubric_association":{"id":30,"rubric_id":20,"use_for_grading":false},"rubric":[{"id":"c1","description":"Evidence","points":100,"ratings":[{"id":"r100","description":"Complete","points":100},{"id":"r0","description":"Missing","points":0}]}]}`), &a)
	return a
}
func fixtureSubmission(user int64) Submission {
	var s Submission
	_ = json.Unmarshal([]byte(`{"id":12,"user_id":5,"assignment_id":10,"attempt":2,"body":"<p>Current writing.</p>","submitted_at":"2026-10-07T12:00:00Z","submission_type":"online_text_entry","score":15,"entered_score":15,"workflow_state":"submitted","user":{"id":5,"name":"Jordan Sample","short_name":"Jordan"},"submission_history":[{"attempt":1,"submitted_at":"2026-10-01T12:00:00Z","body":"<p>Earlier writing.</p>","cached_due_date":"2026-10-02T12:00:00Z","submission_type":"online_text_entry"}]}`), &s)
	s.UserID = user
	s.User.ID = user
	return s
}
func scoringFiles(t *testing.T, s *Service, users ...int64) string {
	t.Helper()
	snapshot, e := Snapshot(fixtureAssignment(), s.Client.Origin())
	if e != nil {
		t.Fatal(e)
	}
	store, e := workspace.OpenStudents(s.Root)
	if e != nil {
		t.Fatal(e)
	}
	packet := domain.PacketDocument{Schema: 1, Kind: "packet", Course: "TEST", Assignment: snapshot}
	results := domain.ResultsDocument{Schema: 1, Kind: "results", Packet: "courses/TEST/packet.md", Assignment: snapshot, Interpretation: domain.Interpretation{Mapping: "repeated", Rationale: "Each required response uses the evidence rubric.", Components: []domain.Component{{Key: "answer", Label: "Answer", Weight: domain.MustDecimal("1")}}}}
	for _, user := range users {
		st, e := store.Upsert(s.Client.Origin(), user, "Jordan Sample", "Jordan", "", "")
		if e != nil {
			t.Fatal(e)
		}
		sub := fixtureSubmission(user)
		work := domain.StudentWorkDocument{Schema: 1, Kind: "student-work", StudentRef: st.Ref, Alias: st.Alias, AssignmentID: 10, Current: domain.AttemptWork{Ref: attemptRef(sub), BodyHTML: sub.Body}, Prior: []domain.AttemptWork{{Ref: attemptRef(sub.History[0]), BodyHTML: sub.History[0].Body, EffectiveDueAt: *sub.History[0].CachedDueDate}}, EnteredGradeKnown: true, EnteredGrade: decimalPointer(sub.EnteredScore)}
		rel := "courses/TEST/" + st.Ref + ".md"
		if e = workspace.WriteDocument(filepath.Join(s.Root, filepath.FromSlash(rel)), work, "work"); e != nil {
			t.Fatal(e)
		}
		packet.Students = append(packet.Students, rel)
		g := &domain.GradeOperation{Attempt: work.Current.Ref, Assessment: domain.MustDecimal("12"), Feedback: "{{student_name}}, explain why this evidence supports the claim.", TeacherNote: "PRIVATE: never student feedback", Components: []domain.ComponentScore{{Key: "answer", Evidence: "present", Criteria: []domain.CriterionScore{{CriterionID: "c1", Points: domain.MustDecimal("60")}}}}}
		results.Students = append(results.Students, domain.StudentResult{StudentRef: st.Ref, Operation: "grade", Grade: g})
	}
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

func TestHonestAssessmentReplacesHigherGradeAndCompletedReplay(t *testing.T) {
	live := fixtureSubmission(5)
	assignment := fixtureAssignment()
	writes := 0
	gradeWrites := 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "submissions/5") {
			if r.Method == "PUT" {
				writes++
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
				if request.Submission != nil {
					gradeWrites++
					n := json.Number(request.Submission["posted_grade"].(string))
					live.EnteredScore, live.Score = &n, &n
				}
				if request.Rubric != nil {
					live.RubricAssessment = request.Rubric
				}
				if request.Comment.Text != "" {
					if strings.Contains(request.Comment.Text, "PRIVATE") || strings.Contains(request.Comment.Text, "<p>") {
						t.Error("feedback leaked private note or HTML")
					}
					a := request.Comment.Attempt
					live.Comments = append(live.Comments, SubmissionComment{ID: 70, Comment: request.Comment.Text, Attempt: &a})
				}
			}
			_ = json.NewEncoder(w).Encode(live)
			return
		}
		_ = json.NewEncoder(w).Encode(assignment)
	})
	path := scoringFiles(t, s, 5)
	assignment.Name = "Teacher renamed it"
	assignment.Description = "<p>Explain your evidence.</p>\n"
	due := "2026-10-10T12:00:00Z"
	assignment.DueAt = &due
	// Obsolete metadata must have no bearing on supported posting.
	if e := os.WriteFile(filepath.Join(s.Root, ".cse", "capabilities.json"), []byte("{malformed"), 0644); e != nil {
		t.Fatal(e)
	}
	report, e := s.Post(context.Background(), path)
	if e != nil {
		t.Fatal(e)
	}
	if len(report.Students) != 1 || report.Students[0].Status != "completed" || gradeWrites != 1 || writes != 2 {
		t.Fatalf("unexpected posting: %#v gradeWrites=%d writes=%d", report, gradeWrites, writes)
	}
	if decimalPointer(live.RubricAssessment["c1"].Points).String() != "60" || decimalPointer(live.EnteredScore).String() != "12" {
		t.Fatal("rubric assessment or entered grade dishonest")
	}
	if !strings.Contains(live.Comments[0].Comment, "12 / 20") || strings.Contains(live.Comments[0].Comment, "retained") {
		t.Fatal("feedback did not report only the computed assessment")
	}
	before := s.Client.Requests()
	live.Comments[0].Comment = "Teacher edited"
	live.EnteredScore = nil
	if _, e = s.Post(context.Background(), path); e != nil {
		t.Fatal(e)
	}
	if s.Client.Requests() != before || writes != 2 {
		t.Fatal("completed replay touched Canvas")
	}
}

func TestInvalidLaterStudentCriterionSendsNoRequests(t *testing.T) {
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) { t.Error("invalid file reached HTTP fixture") })
	path := scoringFiles(t, s, 5, 6)
	b, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	text := string(b)
	key := "criterion_id: c1"
	i := strings.LastIndex(text, key)
	if i < 0 {
		t.Fatal("missing score fixture")
	}
	text = text[:i] + strings.Replace(text[i:], key, "criterion_id: unrelated", 1)
	if e = os.WriteFile(path, []byte(text), 0644); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Post(context.Background(), path); e == nil {
		t.Fatal("wrong criterion accepted")
	}
	if s.Client.Requests() != 0 {
		t.Fatal("invalid later row allowed earlier writes")
	}
}

func TestAssessmentPostsRegardlessOfExistingGrade(t *testing.T) {
	for _, test := range []struct {
		name, value string
		gradeWrites int
	}{{"empty", "", 1}, {"zero", "0", 1}, {"equal", "12", 1}, {"lower", "8", 1}, {"precise-higher", "15.123456", 1}} {
		t.Run(test.name, func(t *testing.T) {
			live := fixtureSubmission(5)
			if test.value == "" {
				live.Score = nil
				live.EnteredScore = nil
			} else {
				number := json.Number(test.value)
				live.Score = &number
				live.EnteredScore = &number
			}
			gradeWrites := 0
			s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
				if strings.Contains(r.URL.Path, "submissions/") {
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
						if req.Rubric != nil {
							live.RubricAssessment = req.Rubric
						}
						if grade, ok := req.Submission["posted_grade"]; ok {
							gradeWrites++
							n := json.Number(grade)
							live.Score = &n
							live.EnteredScore = &n
						}
						if req.Comment.Text != "" {
							a := req.Comment.Attempt
							live.Comments = append(live.Comments, SubmissionComment{ID: 123, Comment: req.Comment.Text, Attempt: &a})
						}
					}
					_ = json.NewEncoder(w).Encode(live)
				} else {
					_ = json.NewEncoder(w).Encode(fixtureAssignment())
				}
			})
			path := scoringFiles(t, s, 5)
			report, e := s.Post(context.Background(), path)
			if e != nil {
				t.Fatal(e)
			}
			if report.Students[0].Status != "completed" || gradeWrites != test.gradeWrites {
				t.Fatalf("unexpected decision: %#v writes%d", report, gradeWrites)
			}
			expected := test.value
			if test.gradeWrites == 1 {
				expected = "12"
			}
			if live.EnteredScore == nil || live.EnteredScore.String() != expected {
				t.Fatal("computed assessment was not posted")
			}
		})
	}
}

func TestResubmissionHoldsOnlyAffectedStudent(t *testing.T) {
	lives := map[string]Submission{"5": fixtureSubmission(5), "6": fixtureSubmission(6)}
	l := lives["5"]
	l.Attempt = 3
	lives["5"] = l
	writes5, writes6 := 0, 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "submissions/") {
			user := filepath.Base(r.URL.Path)
			live := lives[user]
			if r.Method == "PUT" {
				if user == "5" {
					writes5++
				} else {
					writes6++
				}
				var req struct {
					Rubric     map[string]RubricScore `json:"rubric_assessment"`
					Submission map[string]string      `json:"submission"`
					Comment    struct {
						Text    string `json:"text_comment"`
						Attempt int    `json:"attempt"`
					} `json:"comment"`
				}
				_ = json.NewDecoder(r.Body).Decode(&req)
				if value, ok := req.Submission["posted_grade"]; ok {
					n := json.Number(value)
					live.EnteredScore, live.Score = &n, &n
				}
				if req.Rubric != nil {
					live.RubricAssessment = req.Rubric
				}
				if req.Comment.Text != "" {
					a := req.Comment.Attempt
					live.Comments = append(live.Comments, SubmissionComment{ID: 80, Comment: req.Comment.Text, Attempt: &a})
				}
				lives[user] = live
			}
			_ = json.NewEncoder(w).Encode(live)
		} else {
			_ = json.NewEncoder(w).Encode(fixtureAssignment())
		}
	})
	path := scoringFiles(t, s, 5, 6)
	r, e := s.Post(context.Background(), path)
	if e != nil {
		t.Fatal(e)
	}
	if writes5 != 0 || writes6 != 2 || r.Students[0].Status != "held" || r.Students[1].Status != "completed" {
		t.Fatalf("row isolation failed: %#v writes %d/%d", r, writes5, writes6)
	}
}

func TestLostCommentResponseRecoveredWithoutDuplicate(t *testing.T) {
	live := fixtureSubmission(5)
	commentWrites := 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "submissions/5") {
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
				if value, ok := req.Submission["posted_grade"]; ok {
					n := json.Number(value)
					live.EnteredScore, live.Score = &n, &n
				}
				if req.Rubric != nil {
					live.RubricAssessment = req.Rubric
				}
				if req.Comment.Text != "" {
					commentWrites++
					a := req.Comment.Attempt
					live.Comments = append(live.Comments, SubmissionComment{ID: 90, Comment: req.Comment.Text, Attempt: &a})
					w.WriteHeader(500)
					return
				}
			}
			_ = json.NewEncoder(w).Encode(live)
		} else {
			_ = json.NewEncoder(w).Encode(fixtureAssignment())
		}
	})
	path := scoringFiles(t, s, 5)
	first, e := s.Post(context.Background(), path)
	if e != nil {
		t.Fatal(e)
	}
	if first.Students[0].Status != "uncertain" {
		t.Fatalf("lost response status %s", first.Students[0].Status)
	}
	second, e := s.Post(context.Background(), path)
	if e != nil {
		t.Fatal(e)
	}
	if commentWrites != 1 || second.Students[0].Status != "completed" {
		t.Fatalf("duplicate or incomplete: %#v %d", second, commentWrites)
	}
	var doc domain.ResultsDocument
	if _, e = workspace.ReadDocument(path, &doc); e != nil {
		t.Fatal(e)
	}
	if doc.Students[0].Grade.Outcomes.Comment.ID != 0 {
		t.Fatal("matching live text incorrectly claimed comment ownership")
	}
}

func TestUncertainCreateEmptyLookupNeverCreatesAgain(t *testing.T) {
	creates := 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			creates++
			w.WriteHeader(500)
			return
		}
		_, _ = io.WriteString(w, "[]")
	})
	doc := domain.AssignmentDocument{Schema: 1, Kind: "assignment", Operation: "create", Course: "TEST", Title: "Same title", Points: domain.MustDecimal("20"), SubmissionTypes: []string{"online_text_entry"}}
	path := filepath.Join(s.Root, "draft.md")
	if e := workspace.WriteDocument(path, doc, "Explain."); e != nil {
		t.Fatal(e)
	}
	first, e := s.Push(context.Background(), path)
	if e != nil {
		t.Fatal(e)
	}
	if first.Status != "uncertain" {
		t.Fatal("uncertain write not retained")
	}
	second, e := s.Push(context.Background(), path)
	if e != nil {
		t.Fatal(e)
	}
	if creates != 1 || second.Status != "held" {
		t.Fatal("empty lookup triggered blind create")
	}
	var persisted domain.AssignmentDocument
	if _, e = workspace.ReadDocument(path, &persisted); e != nil {
		t.Fatal(e)
	}
	if persisted.Instance != s.Client.Origin() || persisted.CourseID != 1 {
		t.Fatal("create did not persist its target instance/course")
	}
	before := s.Client.Requests()
	s.Config.Courses[0].ID = 2
	if _, e = s.Push(context.Background(), path); e == nil {
		t.Fatal("incomplete create retargeted another course after configuration change")
	}
	if s.Client.Requests() != before {
		t.Fatal("cross-course resume made Canvas request")
	}
}

func TestFeedbackRevisionChangesOnlySelectedComment(t *testing.T) {
	live := fixtureSubmission(5)
	attempt := 2
	live.Comments = []SubmissionComment{{ID: 81, Comment: "Teacher-authored context", Attempt: &attempt}, {ID: 82, Comment: "Old feedback", Attempt: &attempt}}
	writes := 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PUT" {
			writes++
			if !strings.HasSuffix(r.URL.Path, "/comments/82") {
				t.Error("revision touched grading or wrong comment route")
			}
			var request map[string]json.RawMessage
			_ = json.NewDecoder(r.Body).Decode(&request)
			if len(request) != 1 || request["comment"] == nil {
				t.Error("revision contains grading/rubric/late fields")
			}
			var text string
			_ = json.Unmarshal(request["comment"], &text)
			live.Comments[1].Comment = text
			_ = json.NewEncoder(w).Encode(live.Comments[1])
			return
		}
		if strings.Contains(r.URL.Path, "submissions/") {
			_ = json.NewEncoder(w).Encode(live)
		} else {
			_ = json.NewEncoder(w).Encode(fixtureAssignment())
		}
	})
	path := scoringFiles(t, s, 5)
	var results domain.ResultsDocument
	body, e := workspace.ReadDocument(path, &results)
	if e != nil {
		t.Fatal(e)
	}
	var packet domain.PacketDocument
	if _, e = workspace.ReadDocument(filepath.Join(s.Root, filepath.FromSlash(results.Packet)), &packet); e != nil {
		t.Fatal(e)
	}
	var work domain.StudentWorkDocument
	workPath := filepath.Join(s.Root, filepath.FromSlash(packet.Students[0]))
	workBody, e := workspace.ReadDocument(workPath, &work)
	if e != nil {
		t.Fatal(e)
	}
	work.Comments = []domain.ExistingComment{{ID: 81, Text: live.Comments[0].Comment, Attempt: &attempt}, {ID: 82, Text: live.Comments[1].Comment, Attempt: &attempt}}
	if e = workspace.WriteDocument(workPath, work, workBody); e != nil {
		t.Fatal(e)
	}
	results.Students[0].Grade = nil
	results.Students[0].Operation = "revise-feedback"
	results.Students[0].ReviseFeedback = &domain.FeedbackRevision{Attempt: work.Current.Ref, CommentID: 82, Feedback: "Try explaining your evidence in one clear sentence."}
	if e = workspace.WriteDocument(path, results, body); e != nil {
		t.Fatal(e)
	}
	report, e := s.Post(context.Background(), path)
	if e != nil {
		t.Fatal(e)
	}
	if writes != 1 || report.Students[0].Status != "completed" || live.Comments[0].Comment != "Teacher-authored context" || decimalPointer(live.EnteredScore).String() != "15" {
		t.Fatal("selective revision failed")
	}
	before := s.Client.Requests()
	if _, e = s.Post(context.Background(), path); e != nil {
		t.Fatal(e)
	}
	if s.Client.Requests() != before {
		t.Fatal("completed revision replay contacted Canvas")
	}
}

func TestCreateRequiredRubricFailureResumesSameAssignmentAndMovesRecord(t *testing.T) {
	creates, rubricCalls, publishes := 0, 0, 0
	live := fixtureAssignment()
	live.Rubric = nil
	live.RubricAssociation = nil
	live.RubricSettings.ID = 0
	live.Description = "<p>Explain.</p>"
	live.Name = "Draft"
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/assignments") {
			creates++
			_ = json.NewEncoder(w).Encode(live)
			return
		}
		if r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/rubrics") {
			rubricCalls++
			var req map[string]map[string]json.RawMessage
			_ = json.NewDecoder(r.Body).Decode(&req)
			if string(req["rubric_association"]["use_for_grading"]) != "false" || string(req["rubric"]["skip_updating_points_possible"]) != "true" {
				t.Error("rubric scales or coupling are unsafe")
			}
			if rubricCalls == 1 {
				w.WriteHeader(422)
				return
			}
			live.Rubric = fixtureAssignment().Rubric
			live.RubricSettings = fixtureAssignment().RubricSettings
			live.RubricAssociation = fixtureAssignment().RubricAssociation
			_, _ = io.WriteString(w, `{"rubric":{"id":20},"rubric_association":{"id":30}}`)
			return
		}
		if r.Method == "PUT" {
			publishes++
			live.Published = true
		}
		_ = json.NewEncoder(w).Encode(live)
	})
	rubric := domain.Rubric{Title: "Evidence", Criteria: []domain.Criterion{{ID: "draft-c1", Description: "Evidence", Points: domain.MustDecimal("100"), Ratings: []domain.Rating{{ID: "draft-r100", Description: "Complete", Points: domain.MustDecimal("100")}, {ID: "draft-r0", Description: "Missing", Points: domain.MustDecimal("0")}}}}}
	doc := domain.AssignmentDocument{Schema: 1, Kind: "assignment", Operation: "create", Course: "TEST", Title: "Draft", Points: domain.MustDecimal("20"), SubmissionTypes: []string{"online_text_entry"}, Published: true, Rubric: &rubric}
	path := filepath.Join(s.Root, "draft.md")
	if e := workspace.WriteDocument(path, doc, "Explain."); e != nil {
		t.Fatal(e)
	}
	first, e := s.Push(context.Background(), path)
	if e != nil {
		t.Fatal(e)
	}
	if creates != 1 || publishes != 0 || first.Status != "failed" || live.Published {
		t.Fatalf("partial create unsafe: %#v", first)
	}
	second, e := s.Push(context.Background(), path)
	if e != nil {
		t.Fatal(e)
	}
	if creates != 1 || rubricCalls != 2 || publishes != 1 || second.Status != "completed" {
		t.Fatalf("resume failed: %#v", second)
	}
	if second.Path == path || !strings.Contains(second.Path, "pushed") {
		t.Fatal("completed draft not moved")
	}
	if _, e = os.Stat(path); !os.IsNotExist(e) {
		t.Fatal("original complete draft remains")
	}
	before := s.Client.Requests()
	if _, e = s.Push(context.Background(), second.Path); e != nil {
		t.Fatal(e)
	}
	if s.Client.Requests() != before {
		t.Fatal("completed create replay touched Canvas")
	}
}

func TestStaleLiveEditPreservesUnsentEditAndFreshHTML(t *testing.T) {
	live := fixtureAssignment()
	writes := 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			writes++
		}
		_ = json.NewEncoder(w).Encode(live)
	})
	path, e := s.PullItem(context.Background(), "TEST", "10")
	if e != nil {
		t.Fatal(e)
	}
	var doc domain.AssignmentDocument
	_, e = workspace.ReadDocument(path, &doc)
	if e != nil {
		t.Fatal(e)
	}
	doc.Title = "My intended edit"
	if e = workspace.WriteDocument(path, doc, "<section data-unknown=\"preserve\">Local edit</section>"); e != nil {
		t.Fatal(e)
	}
	live.Description = "<details>Teacher live change</details>"
	report, e := s.Push(context.Background(), path)
	if e != nil {
		t.Fatal(e)
	}
	if writes != 0 || report.Status != "held" || !strings.Contains(report.Message, "fresh version") {
		t.Fatalf("stale edit overwritten: %#v", report)
	}
	b, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(string(b), "Local edit") {
		t.Fatal("unsent edit lost")
	}
}

func TestTargetedEditWritesChangedFieldAndCompletedReplay(t *testing.T) {
	live := fixtureAssignment()
	writes := 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PUT" {
			writes++
			var req struct {
				Assignment map[string]json.RawMessage `json:"assignment"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			if len(req.Assignment) != 1 || req.Assignment["description"] == nil {
				t.Error("targeted edit sent unrelated defaults")
			}
			_ = json.Unmarshal(req.Assignment["description"], &live.Description)
			live.UpdatedAt = "2026-10-08T15:00:00Z"
		}
		_ = json.NewEncoder(w).Encode(live)
	})
	path, e := s.PullItem(context.Background(), "TEST", "10")
	if e != nil {
		t.Fatal(e)
	}
	var doc domain.AssignmentDocument
	if _, e = workspace.ReadDocument(path, &doc); e != nil {
		t.Fatal(e)
	}
	if e = workspace.WriteDocument(path, doc, "<section data-preserved=\"yes\">New directions.</section>"); e != nil {
		t.Fatal(e)
	}
	due := "2026-10-10T12:00:00Z"
	live.DueAt = &due
	live.Name = "Teacher renamed it"
	report, e := s.Push(context.Background(), path)
	if e != nil {
		t.Fatal(e)
	}
	if report.Status != "succeeded" || writes != 1 {
		t.Fatalf("targeted edit failed: %#v", report)
	}
	if live.Name != "Teacher renamed it" || live.DueAt == nil || *live.DueAt != due {
		t.Fatal("unrelated live settings were replaced")
	}
	before := s.Client.Requests()
	live.Description = "Teacher changed after successful update"
	again, e := s.Push(context.Background(), path)
	if e != nil {
		t.Fatal(e)
	}
	if again.Status != "completed" || s.Client.Requests() != before || writes != 1 {
		t.Fatal("completed update restored old content")
	}
}

func TestLateTimingHeldWhenDueDateChangedWhileOrdinaryGradingProceeds(t *testing.T) {
	live := fixtureSubmission(5)
	late := "late"
	live.Late = true
	live.LatePolicyStatus = &late
	lateWrites := 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "submissions/") {
			if r.Method == "PUT" {
				var req struct {
					Rubric     map[string]RubricScore     `json:"rubric_assessment"`
					Submission map[string]json.RawMessage `json:"submission"`
					Comment    struct {
						Text    string `json:"text_comment"`
						Attempt int    `json:"attempt"`
					} `json:"comment"`
				}
				_ = json.NewDecoder(r.Body).Decode(&req)
				if value, ok := req.Submission["posted_grade"]; ok {
					n := json.Number(value)
					live.EnteredScore, live.Score = &n, &n
				}
				if req.Rubric != nil {
					live.RubricAssessment = req.Rubric
				}
				if req.Comment.Text != "" {
					a := req.Comment.Attempt
					live.Comments = append(live.Comments, SubmissionComment{ID: 100, Comment: req.Comment.Text, Attempt: &a})
				}
				if value, ok := req.Submission["posted_grade"]; ok {
					var grade string
					_ = json.Unmarshal(value, &grade)
					n := json.Number(grade)
					live.EnteredScore, live.Score = &n, &n
				}
				if req.Submission["late_policy_status"] != nil {
					lateWrites++
					if len(req.Submission) != 1 || string(req.Submission["late_policy_status"]) != `"none"` {
						t.Error("late write changed academic score or policy")
					}
					status := "none"
					live.LatePolicyStatus = &status
				}
			}
			_ = json.NewEncoder(w).Encode(live)
		} else {
			_ = json.NewEncoder(w).Encode(fixtureAssignment())
		}
	})
	path := scoringFiles(t, s, 5)
	var doc domain.ResultsDocument
	body, e := workspace.ReadDocument(path, &doc)
	if e != nil {
		t.Fatal(e)
	}
	doc.Students[0].Grade.LateTiming = &domain.LateTiming{Status: "none", DueAt: "2020-01-01T00:00:00Z", Basis: "Agent counted against an older due date."}
	if e = workspace.WriteDocument(path, doc, body); e != nil {
		t.Fatal(e)
	}
	report, e := s.Post(context.Background(), path)
	if e != nil {
		t.Fatal(e)
	}
	if lateWrites != 0 || report.Students[0].Outcomes.Late.Status != "held" || !definite(report.Students[0].Outcomes.Comment) || !definite(report.Students[0].Outcomes.Rubric) || !definite(report.Students[0].Outcomes.Grade) {
		t.Fatalf("late timing hold failed: %#v", report)
	}
	if _, e = s.Post(context.Background(), path); e != nil {
		t.Fatal(e)
	}
	if lateWrites != 0 {
		t.Fatal("completed override repeated")
	}
}

func TestPullWorkFullRosterNotesAndAttachmentsAndPreservesResults(t *testing.T) {
	live := fixtureSubmission(5)
	live.Attempt = 2
	name := "../../unsafe student.py"
	attachmentBytes := []byte("print('student work')\r\n")
	var serverURL string
	s, server := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/files/code" {
			_, _ = w.Write(attachmentBytes)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/enrollments") {
			_, _ = io.WriteString(w, `[{"user_id":5,"type":"StudentEnrollment","enrollment_state":"active","user":{"id":5,"name":"Jordan Sample","short_name":"Jordan"}},{"user_id":6,"type":"StudentEnrollment","enrollment_state":"active","user":{"id":6,"name":"Jordan Sample","short_name":"Jordan"}},{"user_id":7,"type":"StudentEnrollment","enrollment_state":"inactive","user":{"id":7,"name":"Inactive Example"}}]`)
			return
		}
		if strings.Contains(r.URL.Path, "submissions") {
			withFile := live
			withFile.Attachments = []Attachment{{ID: 500, Filename: name, ContentType: "application/octet-stream", URL: serverURL + "/files/code"}}
			if strings.HasSuffix(r.URL.Path, "/submissions") {
				unsubmitted := fixtureSubmission(6)
				unsubmitted.SubmittedAt = nil
				inactive := fixtureSubmission(7)
				_ = json.NewEncoder(w).Encode([]Submission{withFile, unsubmitted, inactive})
			} else {
				_ = json.NewEncoder(w).Encode(withFile)
			}
			return
		}
		_ = json.NewEncoder(w).Encode(fixtureAssignment())
	})
	serverURL = server.URL
	notes := domain.StudentNotes{Schema: 1, Notes: []domain.StudentNote{{Name: "Jordan Sample", Course: "TEST", Note: "Ambiguous private note"}}}
	if e := workspace.WriteDocument(filepath.Join(s.Root, "context", "students.md"), notes, "notes"); e != nil {
		t.Fatal(e)
	}
	prepared := filepath.Join(s.Root, "courses", "TEST", "prepared-results.md")
	if e := workspace.WriteFile(prepared, []byte("unsent teacher work"), 0644); e != nil {
		t.Fatal(e)
	}
	report, e := s.PullWork(context.Background(), "TEST", "10")
	if e != nil {
		t.Fatal(e)
	}
	if report.Students != 1 || len(report.Packets) != 1 || len(report.Warnings) == 0 {
		t.Fatalf("bad pull: %#v", report)
	}
	var packet domain.PacketDocument
	if _, e = workspace.ReadDocument(report.Packets[0], &packet); e != nil {
		t.Fatal(e)
	}
	var work domain.StudentWorkDocument
	if _, e = workspace.ReadDocument(filepath.Join(s.Root, filepath.FromSlash(packet.Students[0])), &work); e != nil {
		t.Fatal(e)
	}
	if len(work.Notes) != 0 || len(work.Prior) != 1 || len(work.Current.Attachments) != 1 {
		t.Fatal("notes misassigned or history/attachments lost")
	}
	attachment := work.Current.Attachments[0]
	if attachment.Status != "readable" || attachment.ReadablePath == "" {
		t.Fatal("plain text code upload labelled unreadable")
	}
	bytes, e := os.ReadFile(filepath.Join(s.Root, filepath.FromSlash(attachment.Path)))
	if e != nil {
		t.Fatal(e)
	}
	if string(bytes) != string(attachmentBytes) {
		t.Fatal("download bytes changed")
	}
	if !strings.Contains(attachment.Path, "attachments") || strings.Contains(attachment.Path, "../") {
		t.Fatal("unsafe attachment path")
	}
	b, e := os.ReadFile(prepared)
	if e != nil || string(b) != "unsent teacher work" {
		t.Fatal("prepared result overwritten")
	}
	again, e := s.PullWork(context.Background(), "TEST", "10")
	if e != nil {
		t.Fatal(e)
	}
	if again.Packets[0] == report.Packets[0] {
		t.Fatal("pull reused job path")
	}
}

func TestCoupledRubricPostsComputedAssessment(t *testing.T) {
	live := fixtureSubmission(5)
	assignment := fixtureAssignment()
	enabled := true
	assignment.RubricAssociation.UseForGrading = &enabled
	writes := 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "submissions/") {
			if r.Method == "PUT" {
				writes++
				var req struct {
					Rubric     map[string]RubricScore `json:"rubric_assessment"`
					Submission map[string]string      `json:"submission"`
					Comment    struct {
						Text    string `json:"text_comment"`
						Attempt int    `json:"attempt"`
					} `json:"comment"`
				}
				_ = json.NewDecoder(r.Body).Decode(&req)
				if req.Rubric != nil {
					live.RubricAssessment = req.Rubric
					if req.Submission["posted_grade"] != "12" {
						t.Error("combined write omitted computed assessment")
					}
				}
				if value, ok := req.Submission["posted_grade"]; ok {
					n := json.Number(value)
					live.EnteredScore, live.Score = &n, &n
				}
				if req.Comment.Text != "" {
					attempt := req.Comment.Attempt
					live.Comments = append(live.Comments, SubmissionComment{ID: 100, Comment: req.Comment.Text, Attempt: &attempt})
				}
			}
			_ = json.NewEncoder(w).Encode(live)
		} else {
			_ = json.NewEncoder(w).Encode(assignment)
		}
	})
	path := scoringFiles(t, s, 5)
	var doc domain.ResultsDocument
	body, e := workspace.ReadDocument(path, &doc)
	if e != nil {
		t.Fatal(e)
	}
	doc.Assignment.Rubric.UseForGrading = true
	if e = workspace.WriteDocument(path, doc, body); e != nil {
		t.Fatal(e)
	}
	var packet domain.PacketDocument
	packetPath := filepath.Join(s.Root, filepath.FromSlash(doc.Packet))
	packetBody, e := workspace.ReadDocument(packetPath, &packet)
	if e != nil {
		t.Fatal(e)
	}
	packet.Assignment.Rubric.UseForGrading = true
	if e = workspace.WriteDocument(packetPath, packet, packetBody); e != nil {
		t.Fatal(e)
	}
	report, e := s.Post(context.Background(), path)
	if e != nil {
		t.Fatal(e)
	}
	if report.Students[0].Status != "completed" || writes != 2 || len(live.Comments) != 1 || decimalPointer(live.EnteredScore).String() != "12" {
		t.Fatalf("coupled operation was not held: %#v", report)
	}
	again, e := s.Post(context.Background(), path)
	if e != nil {
		t.Fatal(e)
	}
	if writes != 2 || again.Students[0].Status != "completed" || len(live.Comments) != 1 {
		t.Fatal("held coupled operation wrote on retry")
	}
}

func TestCreateRecoveryAcceptsEquivalentDatesDecimalsAndGeneratedIDs(t *testing.T) {
	a := fixtureAssignment()
	utc := "2026-10-08T15:00:00Z"
	local := "2026-10-08T10:00:00-05:00"
	a.DueAt = &utc
	d := domain.AssignmentDocument{Title: a.Name, Points: domain.MustDecimal("20.0"), SubmissionTypes: a.SubmissionTypes, DueAt: &local}
	if !sameCreateCore(a, d, a.Description) {
		t.Fatal("same date instant was rejected because of Canvas UTC normalization")
	}
	snapshot, e := Snapshot(a, "https://canvas.example")
	if e != nil {
		t.Fatal(e)
	}
	rubric := *snapshot.Rubric
	rubric.Criteria = append([]domain.Criterion(nil), rubric.Criteria...)
	rubric.Criteria[0].ID = "draft-id"
	rubric.Criteria[0].Points = domain.MustDecimal("100.0")
	rubric.Criteria[0].Ratings = append([]domain.Rating(nil), rubric.Criteria[0].Ratings...)
	for i := range rubric.Criteria[0].Ratings {
		rubric.Criteria[0].Ratings[i].ID = "draft"
	}
	if !sameRubricDefinition(*snapshot.Rubric, rubric) {
		t.Fatal("same numeric rubric definition with generated IDs rejected")
	}
}

func TestRepresentativeSyntheticClassPullPost(t *testing.T) {
	const size = 30
	lives := map[int64]Submission{}
	enrollments := []Enrollment{}
	for i := int64(1); i <= size; i++ {
		sub := fixtureSubmission(i)
		sub.User.Name = "Synthetic Student " + strconv.FormatInt(i, 10)
		lives[i] = sub
		enrollments = append(enrollments, Enrollment{UserID: i, Type: "StudentEnrollment", State: "active", User: sub.User})
	}
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/enrollments") {
			_ = json.NewEncoder(w).Encode(enrollments)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/assignments") {
			_ = json.NewEncoder(w).Encode([]Assignment{fixtureAssignment()})
			return
		}
		if strings.HasSuffix(r.URL.Path, "/submissions") {
			list := []Submission{}
			for i := int64(1); i <= size; i++ {
				list = append(list, lives[i])
			}
			_ = json.NewEncoder(w).Encode(list)
			return
		}
		if strings.Contains(r.URL.Path, "/submissions/") {
			user, e := strconv.ParseInt(filepath.Base(r.URL.Path), 10, 64)
			if e != nil {
				t.Error(e)
			}
			sub := lives[user]
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
				if value, ok := req.Submission["posted_grade"]; ok {
					n := json.Number(value)
					sub.EnteredScore, sub.Score = &n, &n
				}
				if req.Rubric != nil {
					sub.RubricAssessment = req.Rubric
					sub.State = "graded"
					matches := true
					sub.GradeMatchesCurrentSubmission = &matches
				}
				if req.Comment.Text != "" {
					a := req.Comment.Attempt
					sub.Comments = append(sub.Comments, SubmissionComment{ID: 1000 + user, Comment: req.Comment.Text, Attempt: &a})
				}
				lives[user] = sub
			}
			_ = json.NewEncoder(w).Encode(sub)
			return
		}
		_ = json.NewEncoder(w).Encode(fixtureAssignment())
	})
	pull, e := s.PullWork(context.Background(), "TEST", "10")
	if e != nil {
		t.Fatal(e)
	}
	if pull.Students != size {
		t.Fatalf("class pull size: %d", pull.Students)
	}
	var packet domain.PacketDocument
	if _, e = workspace.ReadDocument(pull.Packets[0], &packet); e != nil {
		t.Fatal(e)
	}
	packetRel, e := filepath.Rel(s.Root, pull.Packets[0])
	if e != nil {
		t.Fatal(e)
	}
	doc := domain.ResultsDocument{Schema: 1, Kind: "results", Packet: filepath.ToSlash(packetRel), Assignment: packet.Assignment, Interpretation: domain.Interpretation{Mapping: "repeated", Rationale: "The one required response uses the evidence rubric.", Components: []domain.Component{{Key: "answer", Label: "Answer", Weight: domain.MustDecimal("1")}}}}
	for _, rel := range packet.Students {
		var work domain.StudentWorkDocument
		if _, e = workspace.ReadDocument(filepath.Join(s.Root, filepath.FromSlash(rel)), &work); e != nil {
			t.Fatal(e)
		}
		doc.Students = append(doc.Students, domain.StudentResult{StudentRef: work.StudentRef, Operation: "grade", Grade: &domain.GradeOperation{Attempt: work.Current.Ref, Assessment: domain.MustDecimal("12"), Feedback: "Explain how your evidence supports the claim.", Components: []domain.ComponentScore{{Key: "answer", Evidence: "present", Criteria: []domain.CriterionScore{{CriterionID: "c1", Points: domain.MustDecimal("60")}}}}}})
	}
	resultsPath := filepath.Join(s.Root, "courses", "TEST", "class-results.md")
	if e = workspace.WriteDocument(resultsPath, doc, "synthetic class"); e != nil {
		t.Fatal(e)
	}
	post, e := s.Post(context.Background(), resultsPath)
	if e != nil {
		t.Fatal(e)
	}
	if len(post.Students) != size {
		t.Fatal("class post lost rows")
	}
	for _, row := range post.Students {
		if row.Status != "completed" {
			t.Fatalf("class row not complete: %#v", row)
		}
	}
	discovery, e := s.PullWork(context.Background(), "TEST", "")
	if e != nil {
		t.Fatal(e)
	}
	if discovery.Students != 0 {
		t.Fatal("completed assessment recurred in discovery")
	}
	sub := lives[1]
	sub.Attempt = 3
	stamp := "2026-10-08T12:00:00Z"
	sub.SubmittedAt = &stamp
	sub.State = "submitted"
	matches := false
	sub.GradeMatchesCurrentSubmission = &matches
	lives[1] = sub
	newAttempt, e := s.PullWork(context.Background(), "TEST", "")
	if e != nil {
		t.Fatal(e)
	}
	if newAttempt.Students != 1 {
		t.Fatal("new attempt was hidden by completed prior operation")
	}
	t.Logf("SYNTHETIC HTTP fixture only: %d students, text current+prior attempts, one rubric criterion, posted computed assessment; pull=%s/%d requests; post=%s/%d requests; handled discovery=%s/%d requests; new-attempt discovery=%s/%d requests", size, pull.Elapsed, pull.Requests, post.Elapsed, post.Requests, discovery.Elapsed, discovery.Requests, newAttempt.Elapsed, newAttempt.Requests)
}
