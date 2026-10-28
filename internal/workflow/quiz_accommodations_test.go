package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"canvasexpertse/internal/workspace"
)

func accommodationChange(ref string) QuizAccommodationChange {
	minutes, attempts, choices := 0, 1, false
	return QuizAccommodationChange{Student: ref, ExtraTimeMinutes: &minutes, ExtraAttempts: &attempts, ReduceChoices: &choices}
}

func TestQuizAccommodationsExactBulkPreviewHoldAndPartialOutcomes(t *testing.T) {
	posts := 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/quiz/v1/courses/1/quizzes/10":
			fmt.Fprint(w, `{"id":"10"}`)
		case "/api/v1/courses/1/assignments/10":
			fmt.Fprint(w, `{"id":10,"course_id":1,"is_quiz_lti_assignment":true}`)
		case "/api/v1/courses/1/enrollments":
			fmt.Fprint(w, `[{"user_id":5,"type":"StudentEnrollment","enrollment_state":"active","user":{"id":5}},{"user_id":6,"type":"StudentEnrollment","enrollment_state":"active","user":{"id":6}}]`)
		case "/api/quiz/v1/courses/1/quizzes/10/accommodations":
			if r.Method != "POST" {
				t.Fatal("accommodation confirmation read")
			}
			posts++
			var body []map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body) != 2 {
				t.Fatalf("not a top-level array: %+v %v", body, err)
			}
			if body[0]["user_id"] != float64(5) || body[1]["user_id"] != float64(6) || body[0]["extra_time"] != float64(0) || body[0]["reduce_choices_enabled"] != false || body[0]["extra_attempts"] != float64(1) || len(body[0]) != 4 {
				t.Fatalf("incorrect wire values: %+v", body)
			}
			fmt.Fprint(w, `{"successful":[{"user_id":5}],"failed":[{"user_id":6,"error":"PRIVATE-NAME token=PRIVATE"}]}`)
		default:
			t.Errorf("unexpected route %s", r.URL)
		}
	})
	_, one, two := tagManageFiles(t, s)
	request := QuizAccommodationRequest{Students: []QuizAccommodationChange{accommodationChange(one), accommodationChange(two)}}
	ctx := context.Background()
	preview, err := s.SetQuizAccommodations(ctx, "TEST", 10, request, true)
	if err != nil || preview.Status != "preview" || posts != 0 || preview.Rows[0].Settings.ExtraTimeMinutes == nil || *preview.Rows[0].Settings.ReduceChoices {
		t.Fatalf("preview: %+v %v posts=%d", preview, err, posts)
	}
	request.Students[0].ExtraTimeMinutes = nil
	held, err := s.SetQuizAccommodations(ctx, "TEST", 10, request, false)
	if err != nil || held.Status != "held" || posts != 0 {
		t.Fatalf("omission was guessed: %+v %v", held, err)
	}
	request.Students[0] = accommodationChange(one)
	report, err := s.SetQuizAccommodations(ctx, "TEST", 10, request, false)
	data, _ := json.Marshal(report)
	if err != nil || report.Completed != 1 || report.Failed != 1 || report.Status != "partial" || posts != 1 || strings.Contains(string(data), "PRIVATE") || strings.Contains(string(data), "user_id") {
		t.Fatalf("partial/private result: %s %v posts=%d", data, err, posts)
	}
}

func TestQuizAccommodationsGuardAndUncertainResponses(t *testing.T) {
	for _, mode := range []string{"wrong-quiz", "wrong-instance", "inactive", "duplicate", "identity", "500"} {
		t.Run(mode, func(t *testing.T) {
			posts := 0
			s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/api/quiz/v1/courses/1/quizzes/10":
					if mode == "wrong-quiz" {
						fmt.Fprint(w, `{"id":11,"course_id":2}`)
					} else {
						fmt.Fprint(w, `{"id":10,"course_id":1}`)
					}
				case "/api/v1/courses/1/assignments/10":
					fmt.Fprint(w, `{"id":10,"course_id":1,"is_quiz_lti_assignment":true}`)
				case "/api/v1/courses/1/enrollments":
					state := "active"
					if mode == "inactive" {
						state = "inactive"
					}
					fmt.Fprintf(w, `[{"user_id":5,"type":"StudentEnrollment","enrollment_state":%q,"user":{"id":5}},{"user_id":6,"type":"StudentEnrollment","enrollment_state":"active","user":{"id":6}}]`, state)
				case "/api/quiz/v1/courses/1/quizzes/10/accommodations":
					posts++
					if mode == "500" {
						http.Error(w, "PRIVATE", 500)
						return
					}
					fmt.Fprint(w, `{"successful":[{"user_id":5},{"user_id":5},{"user_id":999}],"failed":[]}`)
				default:
					t.Errorf("unexpected %s", r.URL)
				}
			})
			_, one, _ := tagManageFiles(t, s)
			request := QuizAccommodationRequest{Students: []QuizAccommodationChange{accommodationChange(one)}}
			if mode == "wrong-instance" {
				s.Config.Instance = "https://other.example.edu"
			}
			if mode == "ambiguous" || mode == "duplicate" {
				store, err := workspace.OpenStudents(s.Root)
				if err != nil {
					t.Fatal(err)
				}
				store.Rows[0].Alias, store.Rows[1].Alias = "Robin", "Robin"
				if mode == "duplicate" {
					store.Rows[1].Alias = "Other"
				}
				if err = store.Save(); err != nil {
					t.Fatal(err)
				}
				if mode == "ambiguous" {
					request.Students[0].Student = "Robin"
				} else {
					request.Students = append(request.Students, accommodationChange("Robin"))
				}
			}
			report, err := s.SetQuizAccommodations(context.Background(), "TEST", 10, request, false)
			if mode == "identity" || mode == "500" {
				if err != nil || posts != 1 || report.Uncertain != 1 || report.Completed != 0 {
					t.Fatalf("uncertain write retried/misreported: %+v %v posts=%d", report, err, posts)
				}
			} else if err == nil || posts != 0 {
				t.Fatalf("guard failed: %+v %v posts=%d", report, err, posts)
			}
		})
	}
}

func TestQuizAccommodationStrictInput(t *testing.T) {
	for _, text := range []string{
		`{"students":[{"student":"s1","extra_time_minutes":0,"reduce_choices":false}]}`,
		`{"students":[{"student":"s1","extra_time_minutes":0,"unknown":false}]}`,
		`{"students":[{"student":"s1"}]}`,
		`{"students":[{"student":"s1","extra_time_minutes":10081}]}`,
		`{"students":[{"student":"s1","extra_attempts":-1}]}`,
		`{"students":[{"student":"s1","extra_time_minutes":1.5}]}`,
		`{"students":[{"student":"s1","extra_attempts":1},{"student":"S1","extra_attempts":1}]}`,
	} {
		request, err := DecodeQuizAccommodations(strings.NewReader(text))
		if strings.Contains(text, `"reduce_choices":false`) {
			if err != nil || request.Students[0].ExtraTimeMinutes == nil || *request.Students[0].ExtraTimeMinutes != 0 || request.Students[0].ExtraAttempts != nil || request.Students[0].ReduceChoices == nil {
				t.Fatalf("presence lost: %+v %v", request, err)
			}
		} else if err == nil {
			t.Fatalf("invalid input accepted: %s", text)
		}
	}
}
