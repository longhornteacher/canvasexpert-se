package workflow

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
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
	t.Cleanup(func() { fixtureHonest = false })
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
	fudge, fudgeValue := "null", 0.0
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
			total := "5.0"
			if landed {
				total = fmt.Sprint(17.0 + fudgeValue)
			}
			io.WriteString(w, `{"authoritative_result":{"id":"900","fudge_points":`+fudge+`,"score":`+total+`,"points_possible":20.0}}`)
		case r.URL.Path == "/api/quiz_sessions/qs1/results/900/session_item_results":
			score, status, grader := "0.0", "ungraded", ""
			if landed {
				score, status, grader = "12.0", "graded", `,"grader_feedback":{"content":"<p>12/15<br>Clear claim. Next, add a second example.</p>"}`
			}
			io.WriteString(w, `[{"item_id":"8001","attempt":2,"score":`+score+`,"points_possible":15.0,"position":1,"graded_at":null,"grading_method":"manual_grading","feedback":{"item_feedback":{}`+grader+`},"scored_data":{"grade_status":"`+status+`","value":"<p>Current response.</p>"}}]`)
		case r.Method == http.MethodPost && r.URL.Path == "/api/quiz_sessions/qs1/results":
			*posts++
			_ = json.NewDecoder(r.Body).Decode(body)
			landed = true
			if f, ok := (*body)["fudge_points"].(float64); ok {
				fudge, fudgeValue = fmt.Sprint(f), f
			}
			if failFirstPost && *posts == 1 {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			w.WriteHeader(http.StatusCreated)
			io.WriteString(w, fmt.Sprintf(`{"id":"901","score":%v,"points_possible":20.0,"status":"graded"}`, 17.0+fudgeValue))
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
		Students: []domain.StudentResult{{StudentRef: student.Ref, Operation: "comment-only", CommentOnly: &domain.CommentOnlyOperation{Attempt: ref, ItemScores: true, HonestEffort: fixtureHonest,
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
	if grader := row["feedback"].(map[string]any)["graderFeedback"]; fmt.Sprint(grader) != "map[content:<p>12/15<br>Clear claim. Next, add a second example.</p>]" {
		t.Fatalf("item comment: %v", grader)
	}
	if *commentText != "Score: 17/20 (attempt 2)\n\nStrong evidence overall." {
		t.Fatalf("the whole-assignment comment should state Canvas's total and attempt, then the closing remarks; got %q", *commentText)
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

func TestItemScoresWholeCommentStatesTheQuizTotal(t *testing.T) {
	s, path, posts, comments, _, commentText := itemScoreFixture(t, "15", false, "")
	report, err := s.Post(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if *posts != 1 || *comments != 1 || report.Students[0].Status != "completed" || *commentText != "Score: 17/20 (attempt 2)" {
		t.Fatalf("posts=%d comments=%d text=%q report=%#v", *posts, *comments, *commentText, report.Students)
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

// A teammate's key writes item scores through the native client; the team
// log must record that write once, redacted, and nothing for token requests.
func TestTeammateItemScoreWriteIsLoggedOnceAndRedacted(t *testing.T) {
	s, _, posts, _, _, _ := itemScoreFixture(t, "15", false, "Strong evidence overall.")
	origin, _ := url.Parse(s.Client.Origin())
	insecure := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	previous := http.DefaultTransport
	http.DefaultTransport = anyHostTransport{destination: origin, base: insecure}
	t.Cleanup(func() { http.DefaultTransport = previous })
	owner := workspace.Teammate{Label: "turner", Name: "Ms. Turner", CanvasUserID: 77}
	s.InstallRoot = s.Root
	s.Keys = fakeKeys{origin: origin.String(), turner: true}
	c, err := s.teammateClient(&routeState{}, owner, domain.Course{Nickname: "TEST", ID: 1})
	if err != nil {
		t.Fatal(err)
	}
	g, err := c.NewQuizGrading(context.Background(), 1, 10, "ps1", "qs1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = g.PostScores(context.Background(), map[string]string{"8001": "12"}, nil); err != nil || *posts != 1 {
		t.Fatalf("posts=%d err=%v", *posts, err)
	}
	entries, err := workspace.ReadTeamLog(s.InstallRoot, "turner", 10)
	if err != nil || len(entries) != 1 {
		t.Fatalf("want exactly one log entry (token requests are not writes): %+v %v", entries, err)
	}
	if e := entries[0]; e.Method != "POST" || e.Path != "/api/quiz_sessions/:session/results" || e.Course != "TEST" || e.Status != 200 {
		t.Fatalf("entry: %+v", e)
	}
	raw, _ := os.ReadFile(filepath.Join(s.InstallRoot, ".cse", "team-log.jsonl"))
	for _, secret := range []string{"qs1", "ps1", turnerToken, "grading-token", "native-jwt"} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("log leaked %q: %s", secret, raw)
		}
	}
}

// fixtureHonest marks the next fixture's comment-only result honest_effort.
var fixtureHonest bool


func withFloor(s *Service, percent string) {
	f := domain.MustDecimal(percent)
	s.Config.Courses[0].EffortFloor = &f
}

func TestQuizFloorRaisesTheTotalByTheShortfallOnceAndKeepsItemScores(t *testing.T) {
		fixtureHonest = true
	s, path, posts, comments, body, commentText := itemScoreFixture(t, "15", false, "Strong evidence overall.")
	withFloor(s, "95") // minimum 19 of 20; items give 5+12 = 17
	report, err := s.Post(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	row := (*body)["results"].([]any)[0].(map[string]any)
	if *posts != 1 || (*body)["fudge_points"] != float64(2) || row["score"] != float64(12) {
		t.Fatalf("expected earned item score 12 and a 2-point adjustment: posts=%d body=%#v report=%#v", *posts, *body, report.Students)
	}
	if !strings.Contains(*commentText, "Score: 19/20") || *comments != 1 {
		t.Fatalf("comment should state Canvas's total: %q", *commentText)
	}
	if _, err = s.Post(context.Background(), path); err != nil || *posts != 1 {
		t.Fatalf("rerun must not add the floor again: posts=%d err=%v", *posts, err)
	}
}

func TestQuizFloorAddsNothingWhenTheTotalAlreadyMeetsIt(t *testing.T) {
		fixtureHonest = true
	s, path, _, _, body, _ := itemScoreFixture(t, "15", false, "Strong evidence overall.")
	withFloor(s, "50") // minimum 10; total 17
	if _, err := s.Post(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	if (*body)["fudge_points"] != nil {
		t.Fatalf("no adjustment expected: %#v", *body)
	}
}

func TestQuizFloorNeedsAnExplicitJudgmentAndTheCourseFloor(t *testing.T) {
		fixtureHonest = false
	s, path, _, _, body, _ := itemScoreFixture(t, "15", false, "Strong evidence overall.")
	withFloor(s, "95") // floor configured, no honest_effort
	if _, err := s.Post(context.Background(), path); err != nil || (*body)["fudge_points"] != nil {
		t.Fatalf("a low total alone must not raise it: %#v err=%v", *body, err)
	}
	fixtureHonest = true
	s, path, posts, _, _, _ := itemScoreFixture(t, "15", false, "Strong evidence overall.")
	withFloor(s, "95")
	s.Config.Courses[0].EffortFloor = nil
	report, err := s.Post(context.Background(), path)
	if err != nil || *posts != 0 || report.Students[0].Status != "held" || !strings.Contains(report.Students[0].Message, "effort_floor") {
		t.Fatalf("expected a hold naming effort_floor: posts=%d report=%#v err=%v", *posts, report.Students, err)
	}
}

func TestQuizFloorStaysAdviceWhenWritesAreOff(t *testing.T) {
	quizFloorWrites = false
	t.Cleanup(func() { quizFloorWrites = true })
	fixtureHonest = true
	s, path, posts, _, body, _ := itemScoreFixture(t, "15", false, "Strong evidence overall.")
	withFloor(s, "95")
	report, err := s.Post(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	msg := report.Students[0].Outcomes.Grade.Message
	if *posts != 1 || (*body)["fudge_points"] != nil || !strings.Contains(msg, "minimum 19") || !strings.Contains(msg, "enter total 19") {
		t.Fatalf("item scores should write with no fudge and the numbers reported: posts=%d body=%#v outcomes=%#v", *posts, *body, report.Students[0].Outcomes)
	}
}

func TestQuizFloorWritesOnlyWhenQuizPointsMatchTheAssignment(t *testing.T) {
	g := &canvas.NewQuizGrading{Score: "35.5", PointsPossible: "100", FudgePoints: json.RawMessage("null"), Items: []map[string]any{{"item_id": "8856743", "score": json.Number("2.5")}}}
	s := &Service{Config: domain.CourseConfig{Courses: []domain.Course{{ID: 1, EffortFloor: ptrDecimal("50")}}}}
	want := map[string]string{"8856743": "2.5"}
	plan, why := s.planQuizFloor(1, domain.MustDecimal("100"), g, want)
	if why != "" || !plan.writable || plan.raise.String() != "14.5" || plan.adjustment.String() != "14.5" {
		t.Fatalf("observed live case: %#v %q", plan, why)
	}
	if plan, _ = s.planQuizFloor(1, domain.MustDecimal("20"), g, want); plan.writable || plan.raise.String() != "14.5" {
		t.Fatalf("a quiz whose points differ from the assignment's must only advise: %#v", plan)
	}
}

func ptrDecimal(s string) *domain.Decimal { d := domain.MustDecimal(s); return &d }
