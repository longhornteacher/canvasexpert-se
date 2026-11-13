package workflow

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"canvasexpertse/internal/canvas"
	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

func TestSaveNewQuizFilesSanitizesNumbersCapsAndCleansFailures(t *testing.T) {
	serverURL := ""
	s, server := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/good":
			_, _ = w.Write([]byte("1234"))
		case "/too-many":
			_, _ = w.Write([]byte("12345"))
		case "/short":
			_, _ = w.Write([]byte("12"))
		default:
			http.NotFound(w, r)
		}
	})
	serverURL = server.URL
	lowUploadLimit(t, 5) // under 5 bytes
	first := s.saveNewQuizFile(context.Background(), "courses/TEST/pulled", "s-abc", 1, canvas.NewQuizFile{ItemID: "9", Name: `../answer?.docx`, Size: 4, URL: serverURL + "/good"}, 1)
	second := s.saveNewQuizFile(context.Background(), "courses/TEST/pulled", "s-abc", 1, canvas.NewQuizFile{ItemID: "9", Name: `answer?.docx`, Size: 4, URL: serverURL + "/good"}, 2)
	if first.Status != "downloaded" || second.Status != "downloaded" || first.Path == second.Path || !strings.Contains(first.Path, "answer_.docx") || !strings.Contains(second.Path, "answer_ (2).docx") {
		t.Fatalf("duplicate/sanitized files: %#v %#v", first, second)
	}
	// Each file stands alone: many files just under the limit all download
	// (no cumulative cutoff), and an unstated size (0) uses the limit while
	// streaming.
	for i := 0; i < 3; i++ {
		got := s.saveNewQuizFile(context.Background(), "courses/TEST/pulled", "s-many", 1, canvas.NewQuizFile{ItemID: "9", Name: "below.bin", URL: serverURL + "/good"}, i+1)
		if got.Status != "downloaded" || got.Size != 4 {
			t.Fatalf("file under the limit not saved: %#v", got)
		}
	}
	for _, tc := range []struct {
		name, url string
		size      int64
		reason    string
	}{
		{"declared-at-limit.bin", serverURL + "/good", 5, "files must be under 150,000,000 bytes"},
		{"streamed-at-limit.bin", serverURL + "/too-many", 0, "or larger"},
		{"too-many.bin", serverURL + "/too-many", 4, "more than the 4 bytes"},
		{"short.bin", serverURL + "/short", 4, "declared 4 bytes but sent 2"},
		{"gone.bin", serverURL + "/gone", 4, "file unavailable"},
		{"http.bin", "http://example.test/file", 4, "file unavailable"},
		{"no-url.bin", "", 4, "did not return a download address"},
	} {
		got := s.saveNewQuizFile(context.Background(), "courses/TEST/pulled", "s-abc", 1, canvas.NewQuizFile{ItemID: "9", Name: tc.name, Size: tc.size, URL: tc.url}, 1)
		if got.Status != "not-retrieved" || got.Path != "" || !strings.HasPrefix(got.Reason, "file unavailable: ") || !strings.Contains(got.Reason, tc.reason) {
			t.Fatalf("bad download accepted or unclear reason: %#v", got)
		}
	}
	var partials []string
	_ = filepath.WalkDir(s.Root, func(p string, d os.DirEntry, e error) error {
		if e == nil && !d.IsDir() && strings.HasSuffix(p, ".partial") {
			partials = append(partials, p)
		}
		return nil
	})
	if len(partials) != 0 {
		t.Fatalf("left partial files: %v", partials)
	}
	path, err := filepath.Abs(filepath.Join(s.Root, filepath.FromSlash(first.Path)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(path); err != nil {
		t.Fatalf("saved file missing: %v", err)
	}
}

func TestNewQuizResponsesMapsLatestStudentAttemptAndWrittenItems(t *testing.T) {
	var signedRequestHadAuth bool
	var server *httptest.Server
	var s *Service
	s, server = workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/reports"):
			if r.Header.Get("Authorization") == "" {
				t.Error("Canvas report request omitted its Canvas token")
			}
			_, _ = w.Write([]byte(`{"progress":{"url":"/api/quiz/v1/reports/7"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/quiz/v1/reports/7":
			_, _ = w.Write([]byte(`{"workflow_state":"completed","results":{"url":"` + server.URL + `/signed/report"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/signed/report":
			signedRequestHadAuth = r.Header.Get("Authorization") != ""
			_, _ = w.Write([]byte(`[
{"student_data":{"id":5,"attempt":1,"submitted_at":"2026-10-01T12:00:00Z"},"item_responses":[{"item_id":21,"item_type":"essay","answer":"<p>Earlier</p>"}]},
{"student_data":{"id":5,"attempt":2,"submitted_at":"2026-10-07T12:00:00Z"},"item_responses":[{"item_id":21,"item_type":"essay","answer":"<p>Current <em>typed</em></p>","score":0},{"item_id":22,"item_type":"file-upload","answer":"answer.docx"},{"item_id":23,"item_type":"choice","answer":"b","score":4.0},{"item_id":124,"item_type":"choice","answer":"a","score":2.5}]},
{"student_data":{"id":6,"attempt":1,"submitted_at":"2026-10-07T12:00:00Z"},"item_responses":[{"item_id":23,"item_type":"choice","answer":"b","score":0.0},{"item_id":24,"item_type":"choice","answer":null,"score":null}]}
]`))
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/items"):
			_, _ = w.Write([]byte(`[{"id":"21","entry_type":"Item","points_possible":10.0,"entry":{"id":"121","interaction_type_slug":"essay","item_body":"<p>Explain your reasoning.</p>"}},{"id":"22","entry_type":"Item","points_possible":5.0,"entry":{"id":"122","interaction_type_slug":"file-upload","item_body":"Upload your work."}},{"id":"23","entry_type":"Item","points_possible":4.0,"entry":{"id":"123","interaction_type_slug":"choice","item_body":"Choose one."}},{"id":"24","entry_type":"Item","points_possible":5.0,"entry":{"id":"124","interaction_type_slug":"choice","item_body":"Choose again."}},{"id":"25","entry_type":"Stimulus","points_possible":0,"entry":{"id":"125","item_body":"Passage."}}]`))
		default:
			t.Errorf("unexpected Student Analysis request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	})
	rows, written, err := newQuizResponses(context.Background(), s.Client, 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	row, ok := rows[5]
	if !ok || row.Attempt != 2 || row.SubmittedAt != "2026-10-07T12:00:00Z" || len(row.Responses) != 1 || len(row.Files) != 1 || len(row.FileItemIDs) != 2 {
		t.Fatalf("wrong New Quiz report projection: %#v", row)
	}
	if row.Responses[0].Prompt != "Explain your reasoning." || row.Responses[0].Text != "Current typed" || row.Files[0].Name != "answer.docx" || row.Files[0].Status != "not-retrieved" || row.FileItemIDs[0] != "22" {
		t.Fatalf("written items were not joined to prompts: %#v", row)
	}
	if a := row.AutoScored; a == nil || a.Earned.Cmp(domain.MustDecimal("6.5")) != 0 || a.Possible.Cmp(domain.MustDecimal("9")) != 0 || a.Items != 2 || row.AutoScoredNote != "" {
		t.Fatalf("auto-scored subtotal should sum choice items only, by either item id: %#v %q", row.AutoScored, row.AutoScoredNote)
	}
	if other := rows[6]; other.AutoScored != nil || other.AutoScoredNote != "1 of 2 auto-scored items have no score in Canvas's report" {
		t.Fatalf("a missing item score must leave the subtotal unknown: %#v %q", other.AutoScored, other.AutoScoredNote)
	}
	if len(written) != 2 || written[0].ItemID != "21" || written[0].EntryID != "121" || written[0].Type != "essay" || written[1].Type != "file-upload" {
		t.Fatalf("written items were not listed for the packet: %#v", written)
	}
	if signedRequestHadAuth {
		t.Fatal("signed Student Analysis download received the Canvas token")
	}
	if _, err = canvasAPIPath(s.Client, "https://example.invalid/api/quiz/v1/progress"); err == nil {
		t.Fatal("accepted a report progress URL outside Canvas")
	}
}

func TestPostNewQuizCommentOnlyUsesNormalReviewResultsContract(t *testing.T) {
	assignment := fixtureAssignment()
	assignment.NewQuiz = true
	quizID := int64(10)
	assignment.QuizID = &quizID
	assignmentJSON, _ := json.Marshal(assignment)
	live := fixtureSubmission(5)
	gets, writes := 0, 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/courses/1/assignments/10":
			gets++
			_, _ = w.Write(assignmentJSON)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/submissions/5"):
			gets++
			_ = json.NewEncoder(w).Encode(live)
		case r.Method == http.MethodPut:
			writes++
			var body map[string]map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if len(body) != 1 || len(body["comment"]) != 2 || body["comment"]["attempt"] != float64(2) {
				t.Errorf("New Quiz write included a non-comment action or missed the attempt: %#v", body)
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected New Quiz post request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	})
	snapshot, err := Snapshot(assignment, s.Client.Origin())
	if err != nil {
		t.Fatal(err)
	}
	studentStore, err := workspace.OpenStudents(s.Root)
	if err != nil {
		t.Fatal(err)
	}
	student, err := studentStore.Upsert(s.Client.Origin(), 5, "Synthetic Student", "Synthetic Student", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err = studentStore.Save(); err != nil {
		t.Fatal(err)
	}
	ref := attemptRef(live)
	work := domain.StudentWorkDocument{Schema: 1, Kind: "student-work", StudentRef: student.Ref, Alias: student.Alias, AssignmentID: 10, Current: domain.AttemptWork{Ref: ref, WrittenResponses: []domain.WrittenResponse{{ItemID: "21", Prompt: "Explain your reasoning.", Type: "essay", Text: "Current response."}}}, EnteredGradeKnown: true}
	workPath := "courses/TEST/" + student.Ref + ".md"
	if err = workspace.WriteDocument(filepath.Join(s.Root, filepath.FromSlash(workPath)), work, "work"); err != nil {
		t.Fatal(err)
	}
	packet := domain.PacketDocument{Schema: 1, Kind: "packet", Course: "TEST", Assignment: snapshot, Students: []string{workPath}}
	if err = workspace.WriteDocument(filepath.Join(s.Root, "courses", "TEST", "packet.md"), packet, "packet"); err != nil {
		t.Fatal(err)
	}
	results := domain.ResultsDocument{Schema: 1, Kind: "results", Packet: "courses/TEST/packet.md", Assignment: snapshot,
		Interpretation: domain.Interpretation{Mapping: "repeated", Rationale: "Use the attached rubric for the response.", Components: []domain.Component{{Key: "answer", Label: "Written response", Weight: domain.MustDecimal("1")}}},
		Students:       []domain.StudentResult{{StudentRef: student.Ref, Operation: "comment-only", CommentOnly: &domain.CommentOnlyOperation{Attempt: ref, Components: []domain.ComponentScore{{Key: "answer", Evidence: "present", Criteria: []domain.CriterionScore{{CriterionID: "c1", Points: domain.MustDecimal("80")}}}}, Feedback: "Your explanation uses relevant evidence."}}}}
	resultsPath := filepath.Join(s.Root, "courses", "TEST", "results.md")
	if err = workspace.WriteDocument(resultsPath, results, "results"); err != nil {
		t.Fatal(err)
	}
	report, err := s.Post(context.Background(), resultsPath)
	if err != nil {
		t.Fatal(err)
	}
	if writes != 1 || gets != 2 || len(report.Students) != 1 || report.Students[0].Status != "completed" {
		t.Fatalf("New Quiz comment post did not complete exactly once without readback: gets=%d writes=%d report=%#v", gets, writes, report.Students)
	}
}

func TestSameAttemptIgnoresMillisecondPrecision(t *testing.T) {
	a := domain.AttemptRef{Attempt: 1, SubmittedAt: "2026-10-13T20:31:55.560Z"}
	b := domain.AttemptRef{Attempt: 1, SubmittedAt: "2026-10-13T20:31:55Z"}
	if !sameAttempt(a, b) || !sameAttempt(b, a) {
		t.Fatal("same attempt with and without milliseconds must match")
	}
	b.Attempt = 2
	if sameAttempt(a, b) {
		t.Fatal("different attempt numbers must not match")
	}
	b = domain.AttemptRef{Attempt: 1, SubmittedAt: "2026-10-13T21:45:00Z"}
	if !sameAttempt(a, b) {
		t.Fatal("the attempt number alone identifies the attempt")
	}
}
