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

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

func TestAudiencePrepareReviewCreateAndChangedSourceHold(t *testing.T) {
	changed := false
	published := false
	creates, overrides := 0, 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		write := func(v any) { _ = json.NewEncoder(w).Encode(v) }
		switch r.URL.Path {
		case "/api/v1/group_categories/50":
			write(map[string]any{"id": 50, "course_id": 1, "context_type": "Course", "name": "Book Club"})
		case "/api/v1/groups/60", "/api/v1/groups/61":
			id := 60
			if strings.HasSuffix(r.URL.Path, "61") {
				id = 61
			}
			write(map[string]any{"id": id, "course_id": 1, "group_category_id": 50, "context_type": "Course", "name": "Ghost"})
		case "/api/v1/courses/1/enrollments":
			write([]any{map[string]any{"user_id": 9001, "type": "StudentEnrollment", "enrollment_state": "active", "user": map[string]any{"id": 9001, "name": "Private Person", "login_id": "secret@example.test"}}, map[string]any{"user_id": 9002, "type": "StudentEnrollment", "enrollment_state": "active", "user": map[string]any{"id": 9002, "name": "Other Private"}}, map[string]any{"user_id": 9003, "type": "StudentEnrollment", "enrollment_state": "active", "user": map[string]any{"id": 9003, "name": "Unassigned"}}})
		case "/api/v1/groups/60/memberships":
			rows := []any{map[string]any{"user_id": 9001, "group_id": 60, "workflow_state": "accepted"}}
			if changed {
				rows = append(rows, map[string]any{"user_id": 9003, "group_id": 60, "workflow_state": "accepted"})
			}
			write(rows)
		case "/api/v1/groups/61/memberships":
			write([]any{map[string]any{"user_id": 9001, "group_id": 61, "workflow_state": "accepted"}, map[string]any{"user_id": 9002, "group_id": 61, "workflow_state": "accepted"}})
		case "/api/v1/courses/1/assignments":
			creates++
			var b map[string]map[string]any
			_ = json.NewDecoder(r.Body).Decode(&b)
			if b["assignment"]["published"] != false || b["assignment"]["group_category_id"] != nil {
				t.Errorf("individual create became shared/published: %+v", b)
			}
			write(map[string]any{"id": 10, "course_id": 1, "name": "Response", "description": "<p>Write independently.</p>", "points_possible": 20, "submission_types": []string{"online_upload"}})
		case "/api/v1/courses/1/assignments/10":
			if r.Method == http.MethodPut {
				var b map[string]map[string]any
				_ = json.NewDecoder(r.Body).Decode(&b)
				if v, ok := b["assignment"]["published"]; ok {
					published = v.(bool)
				}
				w.WriteHeader(204)
				return
			}
			write(map[string]any{"id": 10, "course_id": 1, "name": "Response", "description": "<p>Write independently.</p>", "points_possible": 20, "grading_type": "points", "submission_types": []string{"online_upload"}, "published": published, "only_visible_to_overrides": false, "overrides": []any{}})
		case "/api/v1/courses/1/assignments/10/overrides":
			overrides++
			var b map[string]map[string]json.RawMessage
			_ = json.NewDecoder(r.Body).Decode(&b)
			var ids []int64
			_ = json.Unmarshal(b["assignment_override"]["student_ids"], &ids)
			if len(ids) != 2 || ids[0] != 9001 || ids[1] != 9002 {
				t.Errorf("wrong union %+v", ids)
			}
			if _, ok := b["assignment_override"]["due_at"]; ok {
				t.Error("audience invented a date")
			}
			write(map[string]any{"id": 80})
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL)
			w.WriteHeader(404)
		}
	})
	doc := domain.AssignmentDocument{Schema: 1, Kind: "assignment", Operation: "create", Course: "TEST", Title: "Response", Points: domain.MustDecimal("20"), SubmissionTypes: []string{"online_upload"}, Published: true, Audience: &domain.Audience{Mode: "individual", Policy: "snapshot", Source: domain.AudienceSource{GroupCategoryID: 50, GroupIDs: []int64{61, 60}}}}
	path := filepath.Join(s.Root, "courses", "TEST", "drafts", "response.md")
	if err := workspace.WriteDocument(path, &doc, "Write independently."); err != nil {
		t.Fatal(err)
	}
	state, err := s.PrepareAudience(context.Background(), "TEST", path, false)
	if err != nil || len(state.StudentRefs) != 2 || !state.Complete {
		t.Fatalf("%+v %v", state, err)
	}
	content, _ := os.ReadFile(path)
	for _, secret := range []string{"Private Person", "secret@example.test", "9001", "9002", "9003"} {
		if strings.Contains(string(content), secret) {
			t.Errorf("private roster leaked %s", secret)
		}
	}
	review, err := BuildReview(s.Root, path)
	if err != nil || len(review.Cards) == 0 || review.Cards[0].Hold != "" {
		t.Fatalf("review %+v %v", review, err)
	}
	changed = true
	held, err := s.Push(context.Background(), path)
	if err != nil || held.Status != "held" || creates != 0 {
		t.Fatalf("changed source wrote: %+v %v creates=%d", held, err, creates)
	}
	changed = false
	report, err := s.Push(context.Background(), path)
	if err != nil || report.Status != "completed" || creates != 1 || overrides != 1 || !published {
		t.Fatalf("%+v %v creates %d overrides %d published %v", report, err, creates, overrides, published)
	}
	var saved domain.AssignmentDocument
	_, _ = workspace.ReadDocument(report.Path, &saved)
	if saved.Audience.State.Owned[0].ID != 80 || saved.Audience.State.Application.Status != "succeeded" {
		t.Fatalf("ownership not saved %+v", saved.Audience.State)
	}
	if _, err = s.PrepareAudience(context.Background(), "TEST", report.Path, false); err == nil {
		t.Error("initial prepare discarded accepted ownership")
	}
}

func TestAudienceSelectedCompletenessAndIdentity(t *testing.T) {
	for _, mode := range []string{"pending", "invited-enrollment", "unresolved", "empty", "inactive", "unreadable", "tag", "wrong-course"} {
		t.Run(mode, func(t *testing.T) {
			writes := 0
			s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method != http.MethodGet {
					writes++
					w.WriteHeader(500)
					return
				}
				switch r.URL.Path {
				case "/api/v1/group_categories/50":
					fmt.Fprintf(w, `{"id":50,"course_id":1,"context_type":"Course","non_collaborative":%t}`, mode == "tag")
				case "/api/v1/groups/60":
					courseID := 1
					if mode == "wrong-course" {
						courseID = 2
					}
					fmt.Fprintf(w, `{"id":60,"course_id":%d,"group_category_id":50,"context_type":"Course"}`, courseID)
				case "/api/v1/courses/1/enrollments":
					state := "active"
					if mode == "invited-enrollment" {
						state = "invited"
					}
					if mode == "inactive" {
						state = "inactive"
					}
					fmt.Fprintf(w, `[{"user_id":9001,"type":"StudentEnrollment","enrollment_state":%q,"user":{"id":9001,"name":"Private"}}]`, state)
				case "/api/v1/groups/60/memberships":
					if mode == "unreadable" {
						w.WriteHeader(403)
						return
					}
					if mode == "empty" {
						fmt.Fprint(w, `[]`)
						return
					}
					id := 9001
					if mode == "unresolved" {
						id = 9999
					}
					state := "accepted"
					if mode == "pending" {
						state = "requested"
					}
					fmt.Fprintf(w, `[{"group_id":60,"user_id":%d,"workflow_state":%q}]`, id, state)
				default:
					t.Errorf("unrelated group read %s", r.URL)
					w.WriteHeader(404)
				}
			})
			doc := domain.AssignmentDocument{Schema: 1, Kind: "assignment", Operation: "create", Course: "TEST", Title: "Response", Points: domain.MustDecimal("20"), SubmissionTypes: []string{"online_upload"}, Published: true, Audience: &domain.Audience{Mode: "individual", Policy: "snapshot", Source: domain.AudienceSource{GroupCategoryID: 50, GroupIDs: []int64{60}}}}
			path := filepath.Join(s.Root, "courses", "TEST", "drafts", "response.md")
			_ = workspace.WriteDocument(path, doc, "Directions")
			state, err := s.PrepareAudience(context.Background(), "TEST", path, false)
			if mode == "tag" || mode == "wrong-course" {
				if err == nil {
					t.Fatal("wrong identity accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if mode == "empty" || mode == "inactive" {
				if !state.Complete {
					t.Fatalf("known empty/excluded marked incomplete %+v", state)
				}
			} else if state.Complete {
				t.Fatalf("unsupported membership marked complete %+v", state)
			}
			report, err := s.Push(context.Background(), path)
			if err != nil || report.Status != "held" || writes != 0 {
				t.Fatalf("invalid audience wrote %+v %v %d", report, err, writes)
			}
		})
	}
}

func audienceStageFixture(t *testing.T, handle func(http.ResponseWriter, *http.Request)) *Service {
	t.Helper()
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/group_categories/50":
			fmt.Fprint(w, `{"id":50,"course_id":1,"context_type":"Course","name":"Books"}`)
		case "/api/v1/groups/60":
			fmt.Fprint(w, `{"id":60,"course_id":1,"group_category_id":50,"context_type":"Course","name":"Ghost"}`)
		case "/api/v1/courses/1/enrollments":
			fmt.Fprint(w, `[{"user_id":9001,"type":"StudentEnrollment","enrollment_state":"active","user":{"id":9001,"name":"Private One"}},{"user_id":9002,"type":"StudentEnrollment","enrollment_state":"active","user":{"id":9002,"name":"Private Two"}}]`)
		case "/api/v1/groups/60/memberships":
			fmt.Fprint(w, `[{"group_id":60,"user_id":9001,"workflow_state":"accepted"},{"group_id":60,"user_id":9002,"workflow_state":"accepted"}]`)
		default:
			handle(w, r)
		}
	})
	return s
}
func individualAudience() *domain.Audience {
	return &domain.Audience{Mode: "individual", Policy: "snapshot", Source: domain.AudienceSource{GroupCategoryID: 50, GroupIDs: []int64{60}}}
}

func TestAudienceInitialExistingAssignmentPreservesManualExceptionsAndUncertainWrite(t *testing.T) {
	for _, uncertain := range []bool{false, true} {
		t.Run(fmt.Sprint(uncertain), func(t *testing.T) {
			posts, puts := 0, 0
			rows := []map[string]any{{"id": 70, "title": "Teacher extension", "student_ids": []int64{9001, 9999}, "due_at": "2026-11-01T12:00:00Z"}}
			visible := false
			s := audienceStageFixture(t, func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodGet && r.URL.Path == "/api/v1/courses/1/assignments/10":
					_ = json.NewEncoder(w).Encode(map[string]any{"id": 10, "course_id": 1, "published": true, "only_visible_to_overrides": visible, "overrides": rows})
				case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/overrides"):
					posts++
					var body map[string]map[string]any
					_ = json.NewDecoder(r.Body).Decode(&body)
					ids := body["assignment_override"]["student_ids"].([]any)
					if len(ids) != 1 || ids[0].(float64) != 9002 {
						t.Errorf("duplicated manual ad hoc student: %+v", ids)
					}
					if uncertain {
						w.WriteHeader(503)
						fmt.Fprint(w, `{"error":"unknown acceptance"}`)
						return
					}
					rows = append(rows, map[string]any{"id": 80, "title": "Book club individual audience", "student_ids": []int64{9002}})
					fmt.Fprint(w, `{"id":80}`)
				case r.Method == http.MethodPut && r.URL.Path == "/api/v1/courses/1/assignments/10":
					puts++
					visible = true
					w.WriteHeader(204)
				default:
					t.Errorf("unexpected %s %s", r.Method, r.URL)
					w.WriteHeader(404)
				}
			})
			pulled, e := s.PullAssignmentDates(context.Background(), "TEST", 10)
			if e != nil {
				t.Fatal(e)
			}
			var d domain.AssignmentDatesDocument
			if _, e = workspace.ReadDocument(pulled.Path, &d); e != nil {
				t.Fatal(e)
			}
			d.Audience = individualAudience()
			d.OnlyVisibleToOverrides = nil
			if !uncertain {
				d.OnlyVisibleToOverrides = audienceBool(true)
			}

			path := filepath.Join(s.Root, "courses", "TEST", "drafts", "audience.md")
			_ = workspace.WriteDocument(path, d, "")
			state, err := s.PrepareAudience(context.Background(), "TEST", path, false)
			if err != nil || len(state.Exceptions) != 2 {
				t.Fatalf("exceptions not prepared %+v %v", state, err)
			}
			_ = workspace.WriteDocument(filepath.Join(s.Root, "context", "courses.md"), s.Config, "")
			if _, err = ValidateReviewInput(s.Root, path, "TEST"); err != nil {
				t.Fatal(err)
			}
			model, err := BuildReview(s.Root, path)
			if err != nil || len(model.Cards) != 1 || !strings.Contains(fmt.Sprint(model.Cards[0].Facts), "exclusive club access is not achieved") {
				t.Fatalf("dishonest review %+v %v", model, err)
			}
			first, err := s.PushAssignmentDates(context.Background(), path)
			if err != nil {
				t.Fatal(err)
			}
			if uncertain {
				if first.Status != "held" || puts != 0 {
					t.Fatalf("uncertain audience changed visibility %+v", first)
				}
				if _, err = s.PrepareAudience(context.Background(), "TEST", path, false); err == nil {
					t.Error("uncertain outcome erased")
				}
			} else if first.Status != "completed" || puts != 1 {
				t.Fatalf("%+v", first)
			}
			_, err = s.PushAssignmentDates(context.Background(), path)
			if err != nil || posts != 1 {
				t.Fatalf("write replayed %d %v", posts, err)
			}
			if rows[0]["due_at"] != "2026-11-01T12:00:00Z" {
				t.Error("manual date changed")
			}
		})
	}
}

func TestAudiencePublicationResumePreservesAcceptedAudienceAndLaterTeacherChanges(t *testing.T) {
	for _, publicationStatus := range []int{400, 503} {
		t.Run(fmt.Sprint(publicationStatus), func(t *testing.T) {
			creates, overrides, publications := 0, 0, 0
			visible := false
			changed := false
			s := audienceStageFixture(t, func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodPost && r.URL.Path == "/api/v1/courses/1/assignments":
					creates++
					fmt.Fprint(w, `{"id":10,"course_id":1}`)
				case r.Method == http.MethodGet && r.URL.Path == "/api/v1/courses/1/assignments/10":
					rows := []any{}
					if overrides > 0 {
						ids := []int64{9001, 9002}
						if changed {
							ids = []int64{9001}
						}
						rows = append(rows, map[string]any{"id": 80, "title": "Book club individual audience", "student_ids": ids})
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"id": 10, "course_id": 1, "name": "Response", "description": "<p>Directions</p>", "points_possible": 20, "submission_types": []string{"online_upload"}, "only_visible_to_overrides": visible, "overrides": rows})
				case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/overrides"):
					overrides++
					fmt.Fprint(w, `{"id":80}`)
				case r.Method == http.MethodPut && r.URL.Path == "/api/v1/courses/1/assignments/10":
					var body map[string]map[string]any
					_ = json.NewDecoder(r.Body).Decode(&body)
					if body["assignment"]["published"] == true {
						publications++
						w.WriteHeader(publicationStatus)
						fmt.Fprint(w, `{"error":"publication refused"}`)
					} else {
						visible = true
						w.WriteHeader(204)
					}
				default:
					t.Errorf("unexpected %s %s", r.Method, r.URL)
					w.WriteHeader(404)
				}
			})
			d := domain.AssignmentDocument{Schema: 1, Kind: "assignment", Operation: "create", Course: "TEST", Title: "Response", Points: domain.MustDecimal("20"), SubmissionTypes: []string{"online_upload"}, Published: true, Audience: individualAudience()}
			path := filepath.Join(s.Root, "courses", "TEST", "drafts", "response.md")
			_ = workspace.WriteDocument(path, d, "Directions")
			if _, err := s.PrepareAudience(context.Background(), "TEST", path, false); err != nil {
				t.Fatal(err)
			}
			first, err := s.Push(context.Background(), path)
			if err != nil || first.Status != "partial" || publications != 1 {
				t.Fatalf("first %+v %v", first, err)
			}
			changed = publicationStatus == 400
			second, err := s.Push(context.Background(), path)
			if err != nil || second.Status != "held" || publications != 1 || creates != 1 || overrides != 1 {
				t.Fatalf("accepted restriction replayed or edited audience published %+v %v", second, err)
			}
			var saved domain.AssignmentDocument
			_, _ = workspace.ReadDocument(path, &saved)
			if saved.Audience.State.Application.Status != "succeeded" || saved.Audience.State.Visibility.Status != "succeeded" {
				t.Fatal("accepted outcomes lost")
			}
		})
	}
}

func TestAudienceExistingBaselineHoldsBeforeRetryAndVisibility(t *testing.T) {
	for _, mode := range []string{"manual-after-failure", "visibility-changed", "unsupported-course", "negative-row"} {
		t.Run(mode, func(t *testing.T) {
			posts, puts := 0, 0
			visible := false
			due := "2026-11-01T12:00:00Z"
			s := audienceStageFixture(t, func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodGet && r.URL.Path == "/api/v1/courses/1/assignments/10":
					row := map[string]any{"id": 70, "title": "Manual", "student_ids": []int64{9001}, "due_at": due}
					if mode == "unsupported-course" {
						row = map[string]any{"id": 70, "course_id": 1}
					}
					if mode == "negative-row" {
						row["unassign_item"] = true
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"id": 10, "course_id": 1, "only_visible_to_overrides": visible, "overrides": []any{row}})
				case r.Method == http.MethodPost:
					posts++
					w.WriteHeader(400)
					fmt.Fprint(w, `{"error":"override refused"}`)
				case r.Method == http.MethodPut:
					puts++
					w.WriteHeader(204)
				default:
					t.Errorf("unexpected %s %s", r.Method, r.URL)
					w.WriteHeader(404)
				}
			})
			d := domain.AssignmentDatesDocument{Schema: 1, Kind: "assignment-dates", Operation: "update", Course: "TEST", Instance: s.Client.Origin(), CourseID: 1, AssignmentID: 10, Audience: individualAudience(), Rows: []domain.AssignmentOverrideRow{}}
			path := filepath.Join(s.Root, "courses", "TEST", "drafts", "audience.md")
			_ = workspace.WriteDocument(path, d, "")
			_, err := s.PrepareAudience(context.Background(), "TEST", path, false)
			if mode == "unsupported-course" || mode == "negative-row" {
				if err == nil {
					t.Fatal("unsupported access labeled safe")
				}
				if posts != 0 || puts != 0 {
					t.Fatal("unsupported source wrote")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if mode == "visibility-changed" {
				visible = true
			} else {
				first, err := s.PushAssignmentDates(context.Background(), path)
				if err != nil || first.Status != "held" || posts != 1 {
					t.Fatalf("first %+v %v", first, err)
				}
				due = "2026-11-02T12:00:00Z"
			}
			before := posts
			report, err := s.PushAssignmentDates(context.Background(), path)
			if err != nil || report.Status != "held" || posts != before || puts != 0 {
				t.Fatalf("stale baseline wrote %+v %v posts %d", report, err, posts)
			}
		})
	}
}

func TestAudienceVisibilityRetryPreservesLaterTeacherChange(t *testing.T) {
	posts, puts := 0, 0
	visible := true
	s := audienceStageFixture(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/courses/1/assignments/10":
			rows := []any{}
			if posts > 0 {
				rows = append(rows, map[string]any{"id": 80, "title": "Book club individual audience", "student_ids": []int64{9001, 9002}})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 10, "course_id": 1, "only_visible_to_overrides": visible, "overrides": rows})
		case r.Method == http.MethodPost:
			posts++
			fmt.Fprint(w, `{"id":80}`)
		case r.Method == http.MethodPut:
			puts++
			w.WriteHeader(400)
			fmt.Fprint(w, `{"error":"visibility refused"}`)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL)
			w.WriteHeader(404)
		}
	})
	d := domain.AssignmentDatesDocument{Schema: 1, Kind: "assignment-dates", Operation: "update", Course: "TEST", Instance: s.Client.Origin(), CourseID: 1, AssignmentID: 10, Audience: individualAudience(), Rows: []domain.AssignmentOverrideRow{}}
	path := filepath.Join(s.Root, "courses", "TEST", "drafts", "audience.md")
	_ = workspace.WriteDocument(path, d, "")
	if _, err := s.PrepareAudience(context.Background(), "TEST", path, false); err != nil {
		t.Fatal(err)
	}
	first, err := s.PushAssignmentDates(context.Background(), path)
	if err != nil || first.Status != "held" || posts != 1 || puts != 1 {
		t.Fatalf("first %+v %v posts %d puts %d", first, err, posts, puts)
	}
	visible = false
	second, err := s.PushAssignmentDates(context.Background(), path)
	if err != nil || second.Status != "held" || posts != 1 || puts != 1 {
		t.Fatalf("teacher visibility overwritten %+v %v posts %d puts %d", second, err, posts, puts)
	}
}
