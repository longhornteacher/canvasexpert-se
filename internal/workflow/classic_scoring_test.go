package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

type classicWritingFixture struct {
	assignment Assignment
	subs       map[int64]Submission
	quizzes    map[int64]ClassicSubmission
	defs       []ClassicQuestion
	puts       int
	payloads   []string
	refuse     bool
	differ     bool
	lost       bool
}

func newClassicFixture(t *testing.T) (*Service, *classicWritingFixture) {
	t.Helper()
	f := &classicWritingFixture{subs: map[int64]Submission{}, quizzes: map[int64]ClassicSubmission{}}
	_ = json.Unmarshal([]byte(`{"id":10,"course_id":1,"name":"Mixed Classic","quiz_id":20,"points_possible":25,"grading_type":"points","submission_types":["online_quiz"],"description":"Mixed writing"}`), &f.assignment)
	_ = json.Unmarshal([]byte(`[{"id":101,"position":1,"question_name":"Choice","question_type":"multiple_choice_question","points_possible":5,"question_text":"Choose"},{"id":102,"position":2,"question_name":"Explain","question_type":"essay_question","points_possible":10,"question_text":"Explain evidence"},{"id":103,"position":3,"question_name":"Blank","question_type":"essay_question","points_possible":5,"question_text":"Explain again"},{"id":104,"position":4,"question_name":"Upload","question_type":"file_upload_question","points_possible":5,"question_text":"Upload writing"}]`), &f.defs)
	for _, user := range []int64{5, 6} {
		var sub Submission
		_ = json.Unmarshal([]byte(`{"id":500,"user_id":5,"assignment_id":10,"attempt":2,"submitted_at":"2026-10-09T12:00:00Z","cached_due_date":"2026-10-08T12:00:00Z","submission_type":"online_quiz","workflow_state":"submitted","user":{"id":5,"name":"Sample Student"},"submission_history":[{"id":999,"attempt":2,"submitted_at":"2026-10-09T12:00:00Z","submission_data":[{"question_id":"101","points":5},{"question_id":"102","points":9,"text":"<p>Current evidence</p>"},{"question_id":103,"points":null,"text":""},{"question_id":104,"points":null,"attachment_ids":[401,402]}]},{"id":888,"attempt":1,"submitted_at":"2026-10-01T12:00:00Z","submission_data":[{"question_id":102,"points":7,"text":"Prior evidence"}]}]}`), &sub)
		sub.ID = 500 + user
		sub.UserID = user
		sub.User.ID = user
		f.subs[user] = sub
		finished := "2026-10-09T12:00:00Z"
		f.quizzes[user] = ClassicSubmission{ID: ClassicID(600 + user), QuizID: 20, UserID: ClassicID(user), SubmissionID: ClassicID(sub.ID), Attempt: 2, FinishedAt: &finished, State: "pending_review"}
	}
	var origin string
	s, server := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		write := func(v any) { _ = json.NewEncoder(w).Encode(v) }
		path := r.URL.Path
		switch {
		case path == "/api/v1/courses/1/enrollments":
			write([]Enrollment{{UserID: 5, User: f.subs[5].User, Type: "StudentEnrollment", State: "active"}, {UserID: 6, User: f.subs[6].User, Type: "StudentEnrollment", State: "active"}})
		case path == "/api/v1/courses/1/assignments":
			write([]Assignment{f.assignment})
		case path == "/api/v1/courses/1/assignments/10":
			write(f.assignment)
		case strings.HasPrefix(path, "/api/v1/courses/1/assignments/10/submissions/"):
			user, _ := strconv.ParseInt(filepath.Base(path), 10, 64)
			sub := f.subs[user]
			if r.Method == "PUT" {
				var payload struct {
					Submission struct {
						Status  string `json:"late_policy_status"`
						Seconds int64  `json:"seconds_late_override"`
					} `json:"submission"`
				}
				_ = json.NewDecoder(r.Body).Decode(&payload)
				sub.LatePolicyStatus = &payload.Submission.Status
				sub.SecondsLate = payload.Submission.Seconds
				sub.Late = payload.Submission.Status == "late"
				f.subs[user] = sub
			}
			write(sub)
		case path == "/api/v1/courses/1/quizzes/20":
			write(ClassicQuiz{ID: 20, AssignmentID: 10, QuizType: "assignment", Description: "Live quiz description"})
		case path == "/api/v1/courses/1/quizzes/20/questions":
			if r.URL.Query().Get("quiz_submission_id") == "" || r.URL.Query().Get("quiz_submission_attempt") == "" {
				t.Error("missing attempt question identity")
			}
			defs := append([]ClassicQuestion(nil), f.defs...)
			if r.URL.Query().Get("quiz_submission_attempt") == "1" {
				defs[1].Text = "Prior prompt"
			}
			write(defs)
		case path == "/api/v1/courses/1/quizzes/20/submissions":
			write(map[string]any{"quiz_submissions": []ClassicSubmission{f.quizzes[5], f.quizzes[6]}})
		case strings.HasPrefix(path, "/api/v1/courses/1/quizzes/20/submissions/"):
			id, _ := strconv.ParseInt(filepath.Base(path), 10, 64)
			user := id - 600
			if r.Method == "PUT" {
				f.puts++
				var raw json.RawMessage
				_ = json.NewDecoder(r.Body).Decode(&raw)
				f.payloads = append(f.payloads, string(raw))
				if f.refuse {
					w.WriteHeader(422)
					write(map[string]string{"message": "refused"})
					return
				}
				var payload struct {
					Rows []struct {
						Attempt   int `json:"attempt"`
						Questions map[string]struct {
							Score   domain.Decimal `json:"score"`
							Comment string         `json:"comment"`
						} `json:"questions"`
					} `json:"quiz_submissions"`
				}
				_ = json.Unmarshal(raw, &payload)
				if len(payload.Rows) != 1 || payload.Rows[0].Attempt != f.quizzes[user].Attempt {
					t.Error("wrong write attempt")
				}
				if !f.differ {
					sub := f.subs[user]
					for i := range sub.History {
						if sub.History[i].Attempt == payload.Rows[0].Attempt {
							for j := range sub.History[i].SubmissionData {
								a := &sub.History[i].SubmissionData[j]
								target, ok := payload.Rows[0].Questions[fmt.Sprint(a.QuestionID)]
								if ok {
									n := json.Number(target.Score.String())
									a.Points = &n
									a.MoreComments = target.Comment
								}
							}
						}
					}
					f.subs[user] = sub
				}
				if f.lost {
					w.WriteHeader(500)
					write(map[string]string{"message": "uncertain"})
					return
				}
			}
			write(map[string]any{"quiz_submissions": []ClassicSubmission{f.quizzes[user]}})
		case path == "/api/v1/files/401":
			write(Attachment{ID: 401, Filename: "essay.txt", URL: origin + "/download", ContentType: "text/plain"})
		case path == "/api/v1/files/402":
			w.WriteHeader(404)
		case path == "/download":
			_, _ = w.Write([]byte("Upload evidence"))
		default:
			t.Errorf("unexpected endpoint %s", path)
			w.WriteHeader(404)
		}
	})
	// File metadata gets this server's safe download URL through a transport rewrite below.
	origin = server.URL
	return s, f
}
func TestClassicPullAttemptEvidenceAndPreservation(t *testing.T) {
	s, f := newClassicFixture(t)
	other := f.subs[6]
	other.User.Name = "Other Student"
	f.subs[6] = other
	notes := domain.StudentNotes{Schema: 1, Notes: []domain.StudentNote{{Name: "Sample Student", Course: "TEST", Note: "PRIVATE SENTINEL teacher context"}}}
	if e := workspace.WriteDocument(filepath.Join(s.Root, "context", "students.md"), &notes, ""); e != nil {
		t.Fatal(e)
	}
	// One metadata lookup fails while the other upload is downloaded and preserved.
	first, e := s.PullWork(context.Background(), "TEST", "10")
	if e != nil {
		t.Fatal(e)
	}
	if first.Students != 2 || len(first.Packets) != 1 {
		t.Fatal(first)
	}
	var packet domain.ClassicQuizPacket
	if _, e = workspace.ReadDocument(first.Packets[0], &packet); e != nil {
		t.Fatal(e)
	}
	var work domain.ClassicQuizWork
	if _, e = workspace.ReadDocument(filepath.Join(s.Root, packet.Students[0]), &work); e != nil {
		t.Fatal(e)
	}
	if work.SubmissionID != f.subs[5].ID || work.Current.Ref.Attempt != 2 || len(work.Current.Questions) != 4 || work.Current.Questions[1].ResponseText != "Current evidence" || !work.Current.Questions[2].ResponsePresent || work.Current.Questions[2].ObservedScore != nil || work.Prior[0].Questions[1].PromptText != "Prior prompt" || work.Prior[0].Questions[1].ResponseText != "Prior evidence" || len(work.Current.Questions[3].Attachments) != 2 || work.Current.Questions[3].Attachments[0].Status != "readable" || len(work.Notes) != 1 || !strings.Contains(work.Notes[0], "PRIVATE SENTINEL") {
		t.Fatalf("bad projection: %+v", work)
	}
	original, _ := os.ReadFile(first.Packets[0])
	second, e := s.PullWork(context.Background(), "TEST", "10")
	if e != nil {
		t.Fatal(e)
	}
	after, _ := os.ReadFile(first.Packets[0])
	if string(after) != string(original) || second.Packets[0] == first.Packets[0] {
		t.Fatal("pull replaced work")
	}
}

func classicResultFile(t *testing.T, s *Service, refs ...int64) string {
	t.Helper()
	pull, e := s.PullWork(context.Background(), "TEST", "10")
	if e != nil {
		t.Fatal(e)
	}
	var packet domain.ClassicQuizPacket
	if _, e = workspace.ReadDocument(pull.Packets[0], &packet); e != nil {
		t.Fatal(e)
	}
	rel, _ := filepath.Rel(s.Root, pull.Packets[0])
	doc := domain.ClassicQuizResults{Schema: 1, Kind: "classic-quiz-results", Packet: filepath.ToSlash(rel), Assignment: packet.Assignment, QuizID: packet.QuizID, ScoringNotes: "PRIVATE SENTINEL: prompt mapping"}
	store, e := workspace.OpenStudents(s.Root)
	if e != nil {
		t.Fatal(e)
	}
	for _, path := range packet.Students {
		var work domain.ClassicQuizWork
		if _, e = workspace.ReadDocument(filepath.Join(s.Root, path), &work); e != nil {
			t.Fatal(e)
		}
		st, e := store.Resolve(s.Client.Origin(), work.StudentRef)
		if e != nil {
			t.Fatal(e)
		}
		selected := len(refs) == 0
		for _, id := range refs {
			selected = selected || id == st.UserID
		}
		if !selected {
			continue
		}
		score := domain.MustDecimal("3.456")
		doc.Students = append(doc.Students, domain.ClassicStudentResult{StudentRef: work.StudentRef, Attempt: work.Current.Ref, TeacherNote: "PRIVATE SENTINEL", Questions: []domain.ClassicQuestionResult{{QuestionID: 102, Evidence: "present", Score: &score, Comment: "Explain how your evidence supports the claim.", TeacherNote: "PRIVATE SENTINEL"}}})
	}
	path := filepath.Join(s.Root, "results.md")
	if e = workspace.WriteDocument(path, &doc, "draft preserved body"); e != nil {
		t.Fatal(e)
	}
	return path
}
func editClassicResults(t *testing.T, path string, edit func(*domain.ClassicQuizResults)) {
	t.Helper()
	var d domain.ClassicQuizResults
	body, e := workspace.ReadDocument(path, &d)
	if e != nil {
		t.Fatal(e)
	}
	edit(&d)
	if e = workspace.WriteDocument(path, &d, body); e != nil {
		t.Fatal(e)
	}
}
func TestClassicSourceValidation(t *testing.T) {
	s, _ := newClassicFixture(t)
	path := classicResultFile(t, s, 5)
	var original domain.ClassicQuizResults
	if _, e := workspace.ReadDocument(path, &original); e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		name  string
		id    int64
		score string
		valid bool
	}{
		{"zero", 102, "0", true}, {"bounds", 102, "11", false}, {"objective", 101, "3", false}, {"invented", 999, "3", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := original
			d.Students = append([]domain.ClassicStudentResult(nil), original.Students...)
			d.Students[0].Questions = append([]domain.ClassicQuestionResult(nil), original.Students[0].Questions...)
			d.Students[0].Questions[0].QuestionID = tc.id
			p := domain.MustDecimal(tc.score)
			d.Students[0].Questions[0].Score = &p
			if (ValidateClassicResults(s.Root, &d) == nil) != tc.valid {
				t.Fatal("unexpected source validation")
			}
		})
	}
	editClassicResults(t, path, func(d *domain.ClassicQuizResults) {
		d.Students[0].Questions[0] = domain.ClassicQuestionResult{QuestionID: 104, Evidence: "unknown", TeacherNote: "Unreadable upload needs review"}
	})
	preview, e := ClassicPreview(s.Root, path)
	if e != nil || preview[0].Questions[0].Outcome.Status != "held" {
		t.Fatal(preview, e)
	}
	report, e := s.Post(context.Background(), path)
	if e != nil || report.QuizStudents[0].Questions[0].Outcome.Status != "held" {
		t.Fatal(report, e)
	}
	// Offline validation remains independent of teacher-only student identity storage.
	if e = os.Remove(filepath.Join(s.Root, ".cse", "students.csv")); e != nil {
		t.Fatal(e)
	}
	var d domain.ClassicQuizResults
	if _, e = workspace.ReadDocument(path, &d); e != nil {
		t.Fatal(e)
	}
	if e = ValidateClassicResults(s.Root, &d); e != nil {
		t.Fatal(e)
	}
}
func TestClassicHonestPostAndCompletedRepeat(t *testing.T) {
	s, f := newClassicFixture(t)
	path := classicResultFile(t, s, 5)
	report, e := s.Post(context.Background(), path)
	if e != nil {
		t.Fatal(e)
	}
	q := report.QuizStudents[0].Questions[0]
	if q.Outcome.Status != "succeeded" || q.Score.Cmp(domain.MustDecimal("3.46")) != 0 || f.puts != 1 {
		t.Fatal(report, f.puts)
	}
	for _, payload := range f.payloads {
		if strings.Contains(payload, "PRIVATE") || strings.Contains(payload, "posted_grade") || strings.Contains(payload, "rubric_assessment") || strings.Contains(payload, "fudge_points") || strings.Contains(payload, `"101"`) {
			t.Fatal("unsafe payload", payload)
		}
	}
	a := f.subs[5].History[0].SubmissionData
	if a[0].Points.String() != "5" || a[1].Points.String() != "3.46" || a[1].MoreComments != "Assessment: 3.46/10\n\nExplain how your evidence supports the claim." {
		t.Fatal(a)
	}
	requests := s.Client.Requests()
	again, e := s.Post(context.Background(), path)
	if e != nil || f.puts != 1 || s.Client.Requests() != requests || again.QuizStudents[0].Status != "completed" {
		t.Fatal(again, e)
	}
}
func TestClassicHoldsIndependentActions(t *testing.T) {
	s, f := newClassicFixture(t)
	path := classicResultFile(t, s)
	// The first student's new attempt holds that student while the second still posts.
	sub := f.subs[5]
	sub.Attempt = 3
	f.subs[5] = sub
	editClassicResults(t, path, func(d *domain.ClassicQuizResults) {
		for i := range d.Students {
			zero := domain.MustDecimal("0")
			d.Students[i].Questions = append(d.Students[i].Questions, domain.ClassicQuestionResult{QuestionID: 103, Evidence: "missing", Score: &zero, Comment: "This required response is blank."})
		}
	})
	// Changed real bounds hold only the selected first essay for the ready student.
	f.defs[1].Points = domain.MustDecimal("8")
	report, e := s.Post(context.Background(), path)
	if e != nil {
		t.Fatal(e)
	}
	if f.puts != 1 || report.QuizStudents[0].Questions[0].Outcome.Status != "held" || report.QuizStudents[1].Questions[0].Outcome.Status != "held" || report.QuizStudents[1].Questions[1].Outcome.Status != "succeeded" {
		t.Fatal(report, f.puts)
	}
}
func TestClassicUncertainReadBackAndNoBlindReplay(t *testing.T) {
	for _, different := range []bool{false, true} {
		t.Run(fmt.Sprint(different), func(t *testing.T) {
			s, f := newClassicFixture(t)
			path := classicResultFile(t, s, 5)
			f.lost = true
			f.differ = different
			report, e := s.Post(context.Background(), path)
			if e != nil {
				t.Fatal(e)
			}
			wanted := "succeeded"
			if different {
				wanted = "uncertain"
			}
			if report.QuizStudents[0].Questions[0].Outcome.Status != wanted || f.puts != 1 {
				t.Fatal(report, f.puts)
			}
			// Simulate a saved pending outcome after an interrupted confirmation.
			editClassicResults(t, path, func(d *domain.ClassicQuizResults) { d.Students[0].Questions[0].Outcome.Status = "pending" })
			again, e := s.Post(context.Background(), path)
			if e != nil || again.QuizStudents[0].Questions[0].Outcome.Status != wanted || f.puts != 1 {
				t.Fatal(again, e, f.puts)
			}
		})
	}
}
func TestClassicDiscoveryPartialCompleteAndNewAttempt(t *testing.T) {
	s, f := newClassicFixture(t)
	path := classicResultFile(t, s, 5)
	if _, e := s.Post(context.Background(), path); e != nil {
		t.Fatal(e)
	}
	pull, e := s.PullWork(context.Background(), "TEST", "")
	if e != nil {
		t.Fatal(e)
	}
	var p domain.ClassicQuizPacket
	_, _ = workspace.ReadDocument(pull.Packets[0], &p)
	var w domain.ClassicQuizWork
	_, _ = workspace.ReadDocument(filepath.Join(s.Root, p.Students[0]), &w)
	if len(w.UnhandledQuestionIDs) != 2 || w.UnhandledQuestionIDs[0] != 103 || len(w.Current.Questions) != 4 {
		t.Fatal(w)
	}
	for user, q := range f.quizzes {
		q.State = "complete"
		f.quizzes[user] = q
	}
	complete, e := s.PullWork(context.Background(), "TEST", "")
	if e != nil || len(complete.Packets) != 0 || complete.Assignments[0].Handled != 2 {
		t.Fatal(complete, e)
	}
	reassess, e := s.PullWork(context.Background(), "TEST", "10")
	if e != nil || reassess.Students != 2 {
		t.Fatal(reassess, e)
	}
	sub := f.subs[5]
	sub.Attempt = 3
	submitted := "2026-10-10T12:00:00Z"
	sub.SubmittedAt = &submitted
	row := sub.History[0]
	row.Attempt = 3
	row.SubmittedAt = &submitted
	sub.History = append(sub.History, row)
	f.subs[5] = sub
	qs := f.quizzes[5]
	qs.Attempt = 3
	qs.FinishedAt = &submitted
	qs.State = "pending_review"
	f.quizzes[5] = qs
	newer, e := s.PullWork(context.Background(), "TEST", "")
	if e != nil || newer.Students != 1 {
		t.Fatal(newer, e)
	}
	_, _ = workspace.ReadDocument(newer.Packets[0], &p)
	_, _ = workspace.ReadDocument(filepath.Join(s.Root, p.Students[0]), &w)
	if w.Current.Ref.Attempt != 3 || len(w.UnhandledQuestionIDs) != 3 {
		t.Fatal(w)
	}
}
func TestClassicTimingSeparateFromQuestionOutcomes(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(fmt.Sprint(changed), func(t *testing.T) {
			s, f := newClassicFixture(t)
			path := classicResultFile(t, s, 5)
			editClassicResults(t, path, func(d *domain.ClassicQuizResults) {
				d.Students[0].LateTiming = &domain.LateTiming{Status: "late", SecondsLate: 86400, DueAt: "2026-10-08T12:00:00Z", Basis: "PRIVATE SENTINEL calendar judgment"}
			})
			if changed {
				sub := f.subs[5]
				due := "2026-10-09T12:00:00Z"
				sub.CachedDueDate = &due
				f.subs[5] = sub
			}
			report, e := s.Post(context.Background(), path)
			if e != nil {
				t.Fatal(e)
			}
			wanted := "succeeded"
			if changed {
				wanted = "held"
			}
			if report.QuizStudents[0].Questions[0].Outcome.Status != "succeeded" || report.QuizStudents[0].Late.Status != wanted {
				t.Fatal(report)
			}
			if changed {
				for user, q := range f.quizzes {
					q.State = "complete"
					f.quizzes[user] = q
				}
				discovery, e := s.PullWork(context.Background(), "TEST", "")
				if e != nil || len(discovery.Packets) != 0 || !strings.Contains(strings.Join(discovery.Warnings, " "), path) {
					t.Fatal(discovery, e)
				}
			}
		})
	}
}
