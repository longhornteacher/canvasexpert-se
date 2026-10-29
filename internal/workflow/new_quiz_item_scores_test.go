package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"canvasexpertse/internal/canvas"
	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

type anyHostTransport struct {
	destination *url.URL
	base        http.RoundTripper
}

func (t anyHostTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	c := r.Clone(r.Context())
	u := *r.URL
	c.Host = u.Host
	u.Scheme, u.Host = t.destination.Scheme, t.destination.Host
	c.URL = &u
	return t.base.RoundTrip(c)
}

// itemScoreFixture fakes Canvas and the New Quizzes hosts for one student
// whose essay item 8001 (quiz item 21, 15 points) is scored 80/100 on the
// rubric, so 12 item points.
func itemScoreFixture(t *testing.T, liveScore string, failFirstPost bool, closing string, liveGradedAt ...string) (s *Service, resultsPath string, posts, comments *int, body *map[string]any, commentText *string) {
	t.Helper()
	posts, comments, body, commentText = new(int), new(int), new(map[string]any), new(string)
	assignment := fixtureAssignment()
	assignment.NewQuiz = true
	quizID := int64(10)
	assignment.QuizID = &quizID
	assignmentJSON, _ := json.Marshal(assignment)
	live := fixtureSubmission(5)
	live.Score = (*json.Number)(&liveScore)
	pulledGradedAt, pulledResult := "2026-10-07T13:00:00Z", "900"
	if liveScore == "result-changed" {
		liveScore, pulledResult = "15", "899"
	}
	live.GradedAt = &pulledGradedAt
	if len(liveGradedAt) > 0 {
		live.GradedAt = &liveGradedAt[0]
	}
	landed := false
	live.ExternalToolURL = "https://quiz-lti.instructure.com/lti/launch?participant_session_id=ps1&quiz_session_id=qs1"
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/courses/1/assignments/10":
			_, _ = w.Write(assignmentJSON)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/submissions/5"):
			_ = json.NewEncoder(w).Encode(live)
		case strings.HasSuffix(r.URL.Path, "/external_tools/sessionless_launch"):
			w.Header().Set("Content-Type", "text/html")
			io.WriteString(w, `<script>window.ENV={"ACCOUNT_ID":7,"NEW_QUIZZES":{"signature":"sig","params":{"backend_url":"https://backend.instructure.com"}}};</script>`)
		case r.URL.Path == "/api/v1/jwts":
			io.WriteString(w, `{"token":"workflow-jwt"}`)
		case r.URL.Path == "/api/native/launch":
			io.WriteString(w, `{"access_token":"native-jwt","entry_path":{"resourceId":55}}`)
		case r.URL.Path == "/api/participant_sessions/ps1/grade":
			io.WriteString(w, `{"host":"https://quiz.instructure.com","token":"grading-token","quiz_api_quiz_session_id":"qs1"}`)
		case r.URL.Path == "/api/quiz_sessions/qs1":
			io.WriteString(w, `{"authoritative_result":{"id":"900","fudge_points":null}}`)
		case r.URL.Path == "/api/quiz_sessions/qs1/results/900/session_item_results":
			score, status, grader := "0.0", "ungraded", ""
			if landed {
				score, status, grader = "12.0", "graded", `,"grader_feedback":{"content":"<p>Clear claim. Next, add a second example.</p>"}`
			}
			io.WriteString(w, `[{"item_id":"8001","attempt":2,"score":`+score+`,"points_possible":15.0,"position":1,"graded_at":null,"grading_method":"manual_grading","feedback":{"item_feedback":{}`+grader+`},"scored_data":{"grade_status":"`+status+`","value":"<p>Current response.</p>"}}]`)
		case r.Method == http.MethodPost && r.URL.Path == "/api/quiz_sessions/qs1/results":
			*posts++
			_ = json.NewDecoder(r.Body).Decode(body)
			landed = true
			if failFirstPost && *posts == 1 {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			w.WriteHeader(http.StatusCreated)
			io.WriteString(w, `{"id":"901","score":12.0,"status":"graded"}`)
		case r.Method == http.MethodPut:
			*comments++
			var put map[string]map[string]any
			_ = json.NewDecoder(r.Body).Decode(&put)
			*commentText = fmt.Sprint(put["comment"]["text_comment"])
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request %s %s host=%s", r.Method, r.URL.Path, r.Host)
			http.NotFound(w, r)
		}
	})
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	dest, _ := url.Parse(server.URL)
	previous := http.DefaultTransport
	http.DefaultTransport = anyHostTransport{destination: dest, base: server.Client().Transport}
	client, err := canvas.New(server.URL, "FIXTURE-ONLY-TOKEN")
	http.DefaultTransport = previous
	if err != nil {
		t.Fatal(err)
	}
	s = &Service{Root: t.TempDir(), Client: client, Config: domain.CourseConfig{Schema: 1, Instance: server.URL, Aliases: true, Courses: []domain.Course{{Nickname: "TEST", ID: 1, Current: true}}}}
	snapshot, err := Snapshot(assignment, s.Client.Origin())
	if err != nil {
		t.Fatal(err)
	}
	store, err := workspace.OpenStudents(s.Root)
	if err != nil {
		t.Fatal(err)
	}
	student, err := store.Upsert(s.Client.Origin(), 5, "Synthetic Student", "Synthetic Student", "", "")
	if err != nil || store.Save() != nil {
		t.Fatal(err)
	}
	ref := attemptRef(live)
	pulled := domain.MustDecimal("15")
	work := domain.StudentWorkDocument{Schema: 1, Kind: "student-work", StudentRef: student.Ref, Alias: student.Alias, AssignmentID: 10, ObservedScore: &pulled, ObservedGradedAt: pulledGradedAt, ObservedQuizResult: pulledResult, Current: domain.AttemptWork{Ref: ref, WrittenResponses: []domain.WrittenResponse{{ItemID: "21", Type: "essay", Text: "Current response."}}}, EnteredGradeKnown: true}
	workPath := "courses/TEST/" + student.Ref + ".md"
	if err = workspace.WriteDocument(filepath.Join(s.Root, filepath.FromSlash(workPath)), work, "work"); err != nil {
		t.Fatal(err)
	}
	packet := domain.PacketDocument{Schema: 1, Kind: "packet", Course: "TEST", Assignment: snapshot, Students: []string{workPath},
		WrittenItems: []domain.WrittenItem{{ItemID: "21", EntryID: "8001", Type: "essay", Points: domain.MustDecimal("15")}}}
	if err = workspace.WriteDocument(filepath.Join(s.Root, "courses", "TEST", "packet.md"), packet, "packet"); err != nil {
		t.Fatal(err)
	}
	results := domain.ResultsDocument{Schema: 1, Kind: "results", Packet: "courses/TEST/packet.md", Assignment: snapshot,
		Interpretation: domain.Interpretation{Mapping: "repeated", Rationale: "Use the rubric for the essay.", Components: []domain.Component{{Key: "essay", Label: "Q1 Essay", Weight: domain.MustDecimal("1"), ItemID: "21"}}},
		Students: []domain.StudentResult{{StudentRef: student.Ref, Operation: "comment-only", CommentOnly: &domain.CommentOnlyOperation{Attempt: ref, ItemScores: true,
			Components: []domain.ComponentScore{{Key: "essay", Evidence: "present", Criteria: []domain.CriterionScore{{CriterionID: "c1", Points: domain.MustDecimal("80")}}, Feedback: "Clear claim. Next, add a second example."}}, Feedback: closing}}}}
	resultsPath = filepath.Join(s.Root, "courses", "TEST", "results.md")
	if err = workspace.WriteDocument(resultsPath, results, "results"); err != nil {
		t.Fatal(err)
	}
	return s, resultsPath, posts, comments, body, commentText
}

func TestItemScoresPostBeforeTheCommentAndNeverRepeat(t *testing.T) {
	s, path, posts, comments, body, commentText := itemScoreFixture(t, "15", false, "Strong evidence overall.")
	report, err := s.Post(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if *posts != 1 || *comments != 1 || report.Students[0].Status != "completed" {
		t.Fatalf("posts=%d comments=%d report=%#v", *posts, *comments, report.Students)
	}
	row := (*body)["results"].([]any)[0].(map[string]any)
	if row["itemId"] != "8001" || row["score"] != float64(12) || row["scoredData"].(map[string]any)["gradeStatus"] != "graded" || (*body)["fudge_points"] != nil {
		t.Fatalf("item score body: %#v", *body)
	}
	if grader := row["feedback"].(map[string]any)["graderFeedback"]; fmt.Sprint(grader) != "map[content:<p>Clear claim. Next, add a second example.</p>]" {
		t.Fatalf("item comment: %v", grader)
	}
	if *commentText != "Strong evidence overall." {
		t.Fatalf("the whole-assignment comment should carry only the closing remarks, got %q", *commentText)
	}
	if _, err = s.Post(context.Background(), path); err != nil || *posts != 1 || *comments != 1 {
		t.Fatalf("rerun wrote again: posts=%d comments=%d err=%v", *posts, *comments, err)
	}
}

func TestItemScoresHoldWhenCanvasWasGradedSinceThePull(t *testing.T) {
	s, path, posts, comments, _, _ := itemScoreFixture(t, "14", false, "Strong evidence overall.")
	report, err := s.Post(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if *posts != 0 || *comments != 0 || report.Students[0].Status != "held" || !strings.Contains(report.Students[0].Message, "graded in Canvas since the pull") {
		t.Fatalf("expected a hold with no writes: posts=%d comments=%d report=%#v", *posts, *comments, report.Students)
	}
}

func TestUncertainItemScoreWriteReconcilesInsteadOfRepeating(t *testing.T) {
	s, path, posts, comments, _, _ := itemScoreFixture(t, "15", true, "Strong evidence overall.")
	report, err := s.Post(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if *posts != 1 || *comments != 0 || report.Students[0].Status != "uncertain" {
		t.Fatalf("a failed write must leave the comment unsent: posts=%d comments=%d report=%#v", *posts, *comments, report.Students)
	}
	report, err = s.Post(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if *posts != 1 || *comments != 1 || report.Students[0].Status != "completed" || report.Students[0].Outcomes.Grade.Status != "preserved" {
		t.Fatalf("rerun should find the landed scores and post only the comment: posts=%d comments=%d report=%#v", *posts, *comments, report.Students)
	}
}

func TestItemCommentsAloneSendNoWholeComment(t *testing.T) {
	s, path, posts, comments, _, _ := itemScoreFixture(t, "15", false, "")
	report, err := s.Post(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if *posts != 1 || *comments != 0 || report.Students[0].Status != "completed" || report.Students[0].Revision.Status != "not-requested" {
		t.Fatalf("posts=%d comments=%d report=%#v", *posts, *comments, report.Students)
	}
}

// A SpeedGrader save that changed only an item comment leaves the score
// alone but moves graded_at; the Engine must not overwrite it.
func TestItemScoresHoldWhenOnlyGradedAtMoved(t *testing.T) {
	s, path, posts, comments, _, _ := itemScoreFixture(t, "15", false, "Strong evidence overall.", "2026-10-29T16:00:00Z")
	report, err := s.Post(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if *posts != 0 || *comments != 0 || report.Students[0].Status != "held" {
		t.Fatalf("expected a hold with no writes: posts=%d comments=%d report=%#v", *posts, *comments, report.Students)
	}
}

// An item-comment-only SpeedGrader save changes neither the Canvas score nor
// graded_at (observed 2026-10-29), only the quiz result version.
func TestItemScoresHoldWhenTheQuizResultChangedSinceThePull(t *testing.T) {
	s, path, posts, comments, _, _ := itemScoreFixture(t, "result-changed", false, "Strong evidence overall.")
	report, err := s.Post(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if *posts != 0 || *comments != 0 || report.Students[0].Status != "held" {
		t.Fatalf("expected a hold with no writes: posts=%d comments=%d report=%#v", *posts, *comments, report.Students)
	}
}
