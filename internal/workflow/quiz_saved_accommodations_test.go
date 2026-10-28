package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"canvasexpertse/internal/workspace"
)

func TestSavedQuizTimeExactRoundingAndLimits(t *testing.T) {
	for _, tc := range []struct {
		name, value string
		seconds     int64
		timed       bool
		want        int
		fail        bool
	}{
		{"half odd minutes", "50%", 21 * 60, true, 11, false},
		{"half fractional minute", "50%", 1201, true, 11, false},
		{"double", "100%", 21 * 60, true, 21, false},
		{"double fractional minute", "100%", 1201, true, 21, false},
		{"fixed", "15", 20 * 60, true, 15, false},
		{"untimed", "50%", 0, false, 0, false},
		{"untimed fixed", "15", 0, false, 0, false},
		{"standard", "", 1200, true, 0, false},
		{"maximum", "100%", 10080 * 60, true, 10080, false},
		{"exceeds maximum", "100%", 10080*60 + 1, true, 0, true},
		{"missing time", "50%", 0, true, 0, true},
		{"invalid setting", "25%", 1200, true, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := savedQuizExtraMinutes(tc.value, savedQuizTimer{Timed: &tc.timed, Seconds: &tc.seconds})
			if (err != nil) != tc.fail || err == nil && (got == nil && tc.want != 0 || got != nil && *got != tc.want) {
				t.Fatalf("minutes=%v err=%v expected=%d fail=%t", got, err, tc.want, tc.fail)
			}
		})
	}
	if _, err := savedQuizExtraMinutes("15", savedQuizTimer{}); err == nil {
		t.Fatal("missing timing became untimed")
	}
}

func TestFromSavedQuizAccommodationsScopePrivacyAndHold(t *testing.T) {
	var writes int
	timedQuiz := true
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost && r.URL.Path == "/api/quiz/v1/courses/1/quizzes/10/accommodations" {
			writes++
			var body []map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			// Only supplied settings are sent: never extra_attempts, never a clear.
			if len(body) != 1 || body[0]["user_id"] != float64(5) || body[0]["extra_time"] != float64(11) || body[0]["reduce_choices_enabled"] != true || len(body[0]) != 3 {
				t.Errorf("unexpected saved array %+v", body)
			}
			fmt.Fprint(w, `{"successful":[{"user_id":5}],"failed":[]}`)
			return
		}
		if r.Method != http.MethodGet {
			t.Errorf("unexpected write %s", r.URL)
			return
		}
		switch r.URL.Path {
		case "/api/quiz/v1/courses/1/quizzes/10":
			fmt.Fprintf(w, `{"id":"10","quiz_settings":{"has_time_limit":%t,"session_time_limit_in_seconds":1260}}`, timedQuiz)
		case "/api/v1/courses/1/assignments/10":
			if r.URL.Query().Get("include[]") != "assignment_visibility" {
				t.Error("effective audience was not requested")
			}
			// Canvas's source returns strings, while the public docs show numbers.
			fmt.Fprint(w, `{"id":10,"course_id":1,"is_quiz_lti_assignment":true,"published":true,"assignment_visibility":["5",6,"7"]}`)
		case "/api/v1/courses/1/enrollments":
			fmt.Fprint(w, `[{"user_id":5,"type":"StudentEnrollment","enrollment_state":"active","user":{"id":5}},{"user_id":6,"type":"StudentEnrollment","enrollment_state":"active","user":{"id":6}},{"user_id":7,"type":"StudentEnrollment","enrollment_state":"inactive","user":{"id":7}},{"user_id":8,"type":"StudentEnrollment","enrollment_state":"active","user":{"id":8}}]`)
		default:
			t.Errorf("unexpected request %s", r.URL)
		}
	})
	store, err := workspace.OpenStudents(s.Root)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []int64{5, 6, 7, 8} {
		if _, err = store.Upsert(s.Client.Origin(), id, "PRIVATE-NAME", "", "", ""); err != nil {
			t.Fatal(err)
		}
	}
	for i := range store.Rows {
		store.Rows[i].QuizTime = "50%"
		store.Rows[i].ReducedChoices = true
	}
	if err = store.Save(); err != nil {
		t.Fatal(err)
	}
	// The shared table is authoritative even when this PC's inactive table differs.
	if err = workspace.WriteDocument(filepath.Join(s.Root, "context", "courses.md"), s.Config, ""); err != nil {
		t.Fatal(err)
	}
	if _, err = workspace.SetRulesLocation(s.Root, t.TempDir(), nil, ""); err != nil {
		t.Fatal(err)
	}
	store, err = workspace.OpenStudents(s.Root)
	if err != nil {
		t.Fatal(err)
	}
	store.Rows[1].QuizTime, store.Rows[1].ReducedChoices = "", false
	if err = store.Save(); err != nil {
		t.Fatal(err)
	}
	request := QuizAccommodationRequest{FromSaved: true}
	for _, preview := range []bool{true, false} {
		report, err := s.SetQuizAccommodations(context.Background(), "TEST", 10, request, preview)
		want, wantWrites := "completed", 1
		if preview {
			want, wantWrites = "preview", 0
		}
		if err != nil || report.Status != want || report.Planned != 1 || len(report.Rows) != 0 || writes != wantWrites {
			t.Fatalf("report=%+v err=%v writes=%d", report, err, writes)
		}
		body, _ := json.Marshal(report)
		for _, private := range []string{"PRIVATE-NAME", store.Rows[0].Ref, store.Rows[0].Alias, "50%", "extra_time", "reduce_choices", "user_id"} {
			if strings.Contains(string(body), private) {
				t.Fatalf("saved settings exposed %q: %s", private, body)
			}
		}
	}
	students, err := s.savedQuizStudents(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	timed, seconds := true, int64(1260)
	plan, err := s.planSavedQuizAccommodations(context.Background(), 1, 10, savedQuizTimer{Timed: &timed, Seconds: &seconds}, students)
	if err != nil || len(plan.Changes) != 1 || plan.Changes[0].UserID != 5 || *plan.Changes[0].ExtraTime != 11 || !*plan.Changes[0].ReduceChoices {
		t.Fatalf("private plan=%+v %v", plan, err)
	}
	for _, text := range []string{`{"from_saved":true,"students":[{"student":"synthetic","extra_attempts":1}]}`, `{"from_saved":false}`} {
		if _, err := DecodeQuizAccommodations(strings.NewReader(text)); err == nil {
			t.Fatalf("invalid saved request accepted: %s", text)
		}
	}
	timedQuiz = false
	untimed := false
	plan, err = s.planSavedQuizAccommodations(context.Background(), 1, 10, savedQuizTimer{Timed: &untimed}, students)
	if err != nil || len(plan.Changes) != 1 || plan.Changes[0].ExtraTime != nil || plan.Changes[0].ReduceChoices == nil || !*plan.Changes[0].ReduceChoices {
		t.Fatalf("untimed quiz lost fewer choices or retained time: %+v %v", plan, err)
	}
	for i := range store.Rows {
		store.Rows[i].ReducedChoices = false
	}
	if err := store.Save(); err != nil {
		t.Fatal(err)
	}
	report, err := s.SetQuizAccommodations(context.Background(), "TEST", 10, request, false)
	if err != nil || report.Planned != 0 || report.Status != "completed" || writes != 1 {
		t.Fatalf("untimed quiz received time/clearing writes: %+v %v writes=%d", report, err, writes)
	}
}

func TestSavedQuizPublicationHoldResumeAndExplicitSkip(t *testing.T) {
	f := newQuizAudienceFixture(t, map[int64][]int64{60: {9001}})
	f.seedStudents(t, 9001)
	store, err := workspace.OpenStudents(f.Service.Root)
	if err != nil {
		t.Fatal(err)
	}
	store.Rows[0].QuizTime, store.Rows[0].ReducedChoices = "50%", true
	if err = store.Save(); err != nil {
		t.Fatal(err)
	}
	visibility := []int64{9001}
	f.QuizVisibility = &visibility
	path := f.writeQuiz(t, `{"title":"Private accommodation check","metadata":{"canvas":{"has_time_limit":true,"time_limit_minutes":21,"published":true}},"items":[{"id":"Q1","type":"ESSAY","prompt":"Explain.","points":5}]}`)
	// An unconfirmed POST holds publication, and every rerun resends the whole array.
	f.FailAccommodations = true
	for i := 1; i <= 2; i++ {
		report, err := f.Service.PushQuiz(context.Background(), "TEST", path)
		if err != nil || report.Status != "partial" || report.Published || f.Published || f.QuizCreates != 1 || f.ItemPosts != 1 || len(f.AccommodationBodies) != i {
			t.Fatalf("held resume=%+v err=%v creates/items=%d/%d posts=%d", report, err, f.QuizCreates, f.ItemPosts, len(f.AccommodationBodies))
		}
		rec := readSidecar(path)
		if rec.Stages == nil || rec.Stages.Accommodations == nil || rec.Stages.Accommodations.Status != "held" || rec.Stages.Accommodations.Count != 1 || rec.Stages.Accommodations.Digest == "" {
			t.Fatalf("missing private-safe stage: %+v", rec.Stages)
		}
		body, _ := os.ReadFile(sidecarPath(path))
		for _, private := range []string{store.Rows[0].Ref, store.Rows[0].Alias, "50%", "9001", "extra_time", "reduce_choices"} {
			if strings.Contains(string(body), private) {
				t.Fatalf("sidecar exposed %q: %s", private, body)
			}
		}
	}
	// Explicit skip publishes without any accommodation POST, for this run only.
	report, err := f.Service.PushQuiz(context.Background(), "TEST", path, QuizPushOptions{SkipAccommodations: true})
	if err != nil || report.Status != "completed" || !f.Published || len(f.AccommodationBodies) != 2 || readSidecar(path).Stages.Accommodations.Status != "skipped" {
		t.Fatalf("explicit skip=%+v %v", report, err)
	}
	report, err = f.Service.PushQuiz(context.Background(), "TEST", path)
	if err != nil || report.Status != "partial" || !report.Published || !f.Published || !strings.Contains(report.Message, "already published") || readSidecar(path).Stages.Accommodations.Status != "held" {
		t.Fatalf("skip became permanent or state misreported: %+v %v", report, err)
	}
	// Once Canvas accepts the absolute-value array, the stage is applied.
	f.FailAccommodations = false
	report, err = f.Service.PushQuiz(context.Background(), "TEST", path)
	last := f.AccommodationBodies[len(f.AccommodationBodies)-1]
	if err != nil || report.Status != "completed" || readSidecar(path).Stages.Accommodations.Status != "applied" || len(last) != 1 || last[0]["extra_time"] != float64(11) || last[0]["reduce_choices_enabled"] != true || len(last[0]) != 3 || strings.Contains(report.Accommodations, "9001") {
		t.Fatalf("applied run=%+v %v last=%+v", report, err, last)
	}
}

func TestSavedQuizUnpublishedAudienceFollowsRestrictedOverrides(t *testing.T) {
	f := newQuizAudienceFixture(t, map[int64][]int64{60: {9001}, 61: {9002}})
	f.seedStudents(t, 9001, 9002)
	store, err := workspace.OpenStudents(f.Service.Root)
	if err != nil {
		t.Fatal(err)
	}
	for i := range store.Rows {
		store.Rows[i].ReducedChoices = true
	}
	if err = store.Save(); err != nil {
		t.Fatal(err)
	}
	path := f.writeQuiz(t, quizAudienceFixture)
	if _, err = f.Service.PrepareAudience(context.Background(), "TEST", path, false); err != nil {
		t.Fatal(err)
	}
	report, err := f.Service.PushQuiz(context.Background(), "TEST", path)
	if err != nil || len(f.AccommodationBodies) != 1 || len(f.AccommodationBodies[0]) != 1 || f.AccommodationBodies[0][0]["user_id"] != float64(9001) {
		t.Fatalf("accommodations were not limited to the restricted audience: %+v %v bodies=%+v", report, err, f.AccommodationBodies)
	}
}

func TestSavedQuizAudienceAndMissingEvidenceHoldEvenWhenSkipping(t *testing.T) {
	f := newQuizAudienceFixture(t, map[int64][]int64{60: {9001}})
	f.seedStudents(t, 9001)
	store, err := workspace.OpenStudents(f.Service.Root)
	if err != nil {
		t.Fatal(err)
	}
	store.Rows[0].QuizTime = "50%"
	if err = store.Save(); err != nil {
		t.Fatal(err)
	}
	path := f.writeQuiz(t, quizAudienceFixture)
	f.FailAudience = http.StatusForbidden
	report, err := f.Service.PushQuiz(context.Background(), "TEST", path, QuizPushOptions{SkipAccommodations: true})
	if report.Status != "partial" || f.Published || !strings.Contains(report.Message, "audience") {
		t.Fatalf("skip bypassed audience guard: %+v %v", report, err)
	}
	// A published quiz with an unavailable effective audience must never treat
	// missing audience evidence as an empty audience and publish.
	g := newQuizAudienceFixture(t, map[int64][]int64{60: {9001}})
	g.seedStudents(t, 9001)
	g.Published = true
	store, err = workspace.OpenStudents(g.Service.Root)
	if err != nil {
		t.Fatal(err)
	}
	store.Rows[0].ReducedChoices = true
	if err = store.Save(); err != nil {
		t.Fatal(err)
	}
	path = g.writeQuiz(t, `{"title":"No audience evidence","metadata":{"canvas":{"published":true}},"items":[{"id":"Q1","type":"ESSAY","prompt":"Explain.","points":5}]}`)
	report, err = g.Service.PushQuiz(context.Background(), "TEST", path)
	if err == nil || !strings.Contains(err.Error(), "effective student audience") || report.Status != "partial" || len(g.AccommodationBodies) != 0 || readSidecar(path).Stages.Accommodations.Status != "held" {
		t.Fatalf("missing evidence allowed publication: %+v %v", report, err)
	}
}

func TestFromSavedQuizTargetEvidence(t *testing.T) {
	for _, response := range []string{
		`{"id":11,"course_id":1,"is_quiz_lti_assignment":true,"assignment_visibility":[5]}`,
		`{"id":10,"course_id":2,"is_quiz_lti_assignment":true,"assignment_visibility":[5]}`,
		`{"id":10,"course_id":1,"is_quiz_lti_assignment":false,"assignment_visibility":[5]}`,
		`{"id":10,"course_id":1,"is_quiz_lti_assignment":true,"published":true}`,
		`{"id":10,"course_id":1,"is_quiz_lti_assignment":true,"published":true,"assignment_visibility":["invalid"]}`,
		`{"id":10,"course_id":1,"is_quiz_lti_assignment":true,"published":false,"only_visible_to_overrides":true,"overrides":[{"course_section_id":3}]}`,
		`{"id":10,"course_id":1,"is_quiz_lti_assignment":true,"published":false,"only_visible_to_overrides":true,"overrides":[{"student_ids":[]}]}`,
	} {
		t.Run(response, func(t *testing.T) {
			s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Fatal("saved target check wrote to Canvas")
				}
				fmt.Fprint(w, response)
			})
			_, err := s.planSavedQuizAccommodations(context.Background(), 1, 10, savedQuizTimer{}, []workspace.Student{{UserID: 5, QuizTime: "50%"}})
			if err == nil {
				t.Fatal("unconfirmed target/audience accepted")
			}
		})
	}
}

func TestSavedQuizUntimedTimeOnlyDoesNotHoldPublication(t *testing.T) {
	f := newQuizAudienceFixture(t, map[int64][]int64{60: {9001}})
	f.seedStudents(t, 9001)
	store, err := workspace.OpenStudents(f.Service.Root)
	if err != nil {
		t.Fatal(err)
	}
	store.Rows[0].QuizTime = "50%"
	if err = store.Save(); err != nil {
		t.Fatal(err)
	}
	path := f.writeQuiz(t, `{"title":"Untimed quiz","metadata":{"canvas":{"published":true}},"items":[{"id":"Q1","type":"ESSAY","prompt":"Explain.","points":5}]}`)
	report, err := f.Service.PushQuiz(context.Background(), "TEST", path)
	if err != nil || report.Status != "completed" || !f.Published || readSidecar(path).Stages.Accommodations.Status != "not-requested" {
		t.Fatalf("irrelevant timer setting held an untimed quiz: %+v %v", report, err)
	}
}
