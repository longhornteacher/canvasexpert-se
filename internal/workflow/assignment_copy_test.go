package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	"canvasexpertse/internal/domain"
)

func TestAssignmentCopyRewritesFilesPlacesUnpublishedAndDoesNotReplay(t *testing.T) {
	for _, withRubric := range []bool{false, true} {
		t.Run(fmt.Sprintf("rubric-%v", withRubric), func(t *testing.T) {
			source := fixtureAssignment()
			source.Name, source.Points, source.SubmissionTypes, source.Published = "Paper practice", "100", []string{"on_paper"}, true
			source.Description = `<p>Practice.</p><img src="/courses/1/files/99/preview?verifier=private" data-api-endpoint="/api/v1/courses/1/files/99"/><a href="/files/99/download">Sheet</a>`
			if !withRubric {
				source.Rubric, source.RubricAssociation = nil, nil
			}
			var target Assignment
			fileWrites, assignmentWrites, moduleWrites, rubricWrites := 0, 0, 0, 0
			s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
				encode := func(v any) { _ = json.NewEncoder(w).Encode(v) }
				switch r.Method + " " + r.URL.Path {
				case "GET /api/v1/courses/1/assignments/10":
					encode(source)
				case "GET /api/v1/courses/1/files/99":
					encode(map[string]any{"id": 99, "folder_id": 100, "locked": false, "hidden": false, "visibility_level": "course"})
				case "GET /api/v1/courses/1/folders/100":
					encode(map[string]any{"id": 100, "context_id": 1, "context_type": "Course"})
				case "GET /api/v1/courses/2/folders/root", "GET /api/v1/courses/2/folders/200":
					encode(map[string]any{"id": 200, "context_id": 2, "context_type": "Course"})
				case "GET /api/v1/courses/2/modules/7":
					encode(map[string]any{"id": 7, "course_id": 2, "name": "Practice"})
				case "GET /api/v1/courses/2/assignment_groups":
					encode([]map[string]any{{"id": 8, "name": "Daily"}})
				case "GET /api/v1/courses/2/assignment_groups/8":
					encode(map[string]any{"id": 8, "name": "Daily"})
				case "POST /api/v1/folders/200/copy_file":
					fileWrites++
					var body map[string]any
					_ = json.NewDecoder(r.Body).Decode(&body)
					if body["source_file_id"] != "99" || body["on_duplicate"] != "rename" {
						t.Errorf("copy fields: %#v", body)
					}
					encode(map[string]any{"id": 199, "folder_id": 200})
				case "GET /api/v1/courses/2/files/199":
					encode(map[string]any{"id": 199, "folder_id": 200, "locked": false, "hidden": true, "visibility_level": "course"})
				case "POST /api/v1/courses/2/assignments":
					assignmentWrites++
					var body struct {
						Assignment map[string]json.RawMessage `json:"assignment"`
					}
					_ = json.NewDecoder(r.Body).Decode(&body)
					for _, forbidden := range []string{"assignment_overrides", "group_category_id", "rubric_association", "score", "submissions"} {
						if _, ok := body.Assignment[forbidden]; ok {
							t.Errorf("copied forbidden field %s", forbidden)
						}
					}
					b, _ := json.Marshal(body.Assignment)
					_ = json.Unmarshal(b, &target)
					target.ID, target.CourseID, target.HTMLURL = 30, 2, "/courses/2/assignments/30"
					if target.Published || target.AssignmentGroupID == nil || *target.AssignmentGroupID != 8 || strings.Contains(target.Description, "/courses/1/") || strings.Contains(target.Description, "verifier") || !strings.Contains(target.Description, "/courses/2/files/199/preview") || !strings.Contains(target.Description, "/api/v1/courses/2/files/199") {
						t.Errorf("wrong copied assignment: %#v", target)
					}
					encode(target)
				case "GET /api/v1/courses/2/assignments/30":
					encode(target)
				case "POST /api/v1/courses/2/rubrics":
					rubricWrites++
					var body map[string]map[string]any
					_ = json.NewDecoder(r.Body).Decode(&body)
					if body["rubric_association"]["use_for_grading"] != false {
						t.Errorf("rubric copy coupled grading")
					}
					target.Rubric, target.RubricAssociation, target.RubricSettings = source.Rubric, &RubricAssociation{ID: 330, RubricID: 320, UseForGrading: boolPtr(false)}, source.RubricSettings
					target.RubricSettings.ID = 320
					encode(map[string]any{"rubric": map[string]any{"id": 320}, "rubric_association": map[string]any{"id": 330}})
				case "POST /api/v1/courses/2/modules/7/items":
					moduleWrites++
					var body map[string]map[string]any
					_ = json.NewDecoder(r.Body).Decode(&body)
					if body["module_item"]["content_id"] != float64(30) {
						t.Errorf("wrong module content: %#v", body)
					}
					encode(map[string]any{"id": 70})
				default:
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
					http.Error(w, "unexpected", 500)
				}
			})
			s.Config.Courses = append(s.Config.Courses, domain.Course{Nickname: "TARGET", ID: 2})
			opts := AssignmentCopyOptions{ModuleID: 7, AssignmentGroupID: 8}
			first, err := s.CopyAssignment(context.Background(), "TEST", "10", "TARGET", opts)
			if err != nil || first.Status != "completed" || first.AssignmentID != 30 || fileWrites != 1 || assignmentWrites != 1 || moduleWrites != 1 || rubricWrites != map[bool]int{false: 0, true: 1}[withRubric] {
				t.Fatalf("copy: %#v error=%v writes=%d/%d/%d/%d", first, err, fileWrites, assignmentWrites, moduleWrites, rubricWrites)
			}
			if _, err = os.Stat(first.ReceiptPath); err != nil {
				t.Fatal(err)
			}
			second, err := s.CopyAssignment(context.Background(), "TEST", "10", "TARGET", opts)
			if err != nil || second.Status != "completed" || second.AssignmentID != 30 || fileWrites != 1 || assignmentWrites != 1 || moduleWrites != 1 {
				t.Fatalf("copy replayed: %#v error=%v", second, err)
			}
			_, err = s.CopyAssignment(context.Background(), "TEST", "10", "TARGET", AssignmentCopyOptions{ModuleID: 7, AssignmentGroupID: 9})
			if err == nil || !strings.Contains(err.Error(), "options differ") {
				t.Fatalf("changed destination was accepted: %v", err)
			}
		})
	}
}

func TestAssignmentCopyHoldsUncertainFileBeforeAssignmentAndDoesNotReplay(t *testing.T) {
	source := fixtureAssignment()
	source.Rubric = nil
	source.RubricAssociation = nil
	source.Description = `<img src="/courses/1/files/99/preview"/>`
	writes := 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		var v any
		switch r.Method + " " + r.URL.Path {
		case "GET /api/v1/courses/1/assignments/10":
			v = source
		case "GET /api/v1/courses/1/files/99":
			v = map[string]any{"id": 99, "folder_id": 100, "locked": false}
		case "GET /api/v1/courses/1/folders/100":
			v = map[string]any{"id": 100, "context_id": 1, "context_type": "Course"}
		case "GET /api/v1/courses/2/folders/root":
			v = map[string]any{"id": 200, "context_id": 2, "context_type": "Course"}
		case "POST /api/v1/folders/200/copy_file":
			writes++
			v = map[string]any{}
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected", 500)
			return
		}
		_ = json.NewEncoder(w).Encode(v)
	})
	s.Config.Courses = append(s.Config.Courses, domain.Course{Nickname: "TARGET", ID: 2})
	for i := 0; i < 2; i++ {
		_, err := s.CopyAssignment(context.Background(), "TEST", "10", "TARGET", AssignmentCopyOptions{})
		if err == nil {
			t.Fatal("uncertain copy proceeded")
		}
	}
	if writes != 1 {
		t.Fatalf("uncertain file copy replayed %d times", writes)
	}
}

func TestAssignmentCopyHTMLDependencies(t *testing.T) {
	for _, input := range []string{`<a href="/courses/1/pages/intro">Intro</a>`, `<img srcset="/courses/1/files/99/preview 1x"/>`, `<img src="/courses/4/files/99/preview"/>`} {
		if _, err := rewriteAssignmentCopyHTML(input, "https://canvas.example.edu", 1, 2, func(id int64) (int64, error) { return 199, nil }); err == nil {
			t.Errorf("unsafe dependency was accepted: %s", input)
		}
	}
	input := `<p>Keep <strong>formatting</strong>.</p><img src="https://outside.example/courses/1/files/99/preview"/><img src="https://canvas.example.edu/files/99/preview?verifier=secret#x"/>`
	got, err := rewriteAssignmentCopyHTML(input, "https://canvas.example.edu", 1, 2, func(id int64) (int64, error) { return 199, nil })
	if err != nil || strings.Contains(got, "secret") || !strings.Contains(got, `<strong>formatting</strong>`) || !strings.Contains(got, "https://outside.example/courses/1/files/99/preview") || !strings.Contains(got, "/courses/2/files/199/preview#x") {
		t.Fatalf("rewritten HTML: %s err=%v", got, err)
	}
}
