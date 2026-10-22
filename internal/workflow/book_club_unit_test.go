package workflow

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

// bookClubUnit is one stateful Canvas course used by the integrated book-club
// unit checks: a shared existing module, two collaborative clubs, an unassigned
// student, and the three Ghost resources (Page, individual response, New Quiz).
type bookClubUnit struct {
	Service *Service
	// Members maps a club ID to its accepted member user IDs.
	Members map[int64][]int64
	// ModuleItems is the live shared module, including entries this unit did not
	// create.
	ModuleItems []canvasModuleItem
	// PageDates is the live date-details collection for the created Page.
	PageDates pageDateDetails
	// AssignmentOverrides is the live override collection for the response.
	AssignmentOverrides []AssignmentOverrideLive
	// QuizOverrides is the live override collection for the quiz assignment.
	QuizOverrides []AssignmentOverrideLive
	// Submissions maps a user ID to the response submission body.
	Submissions map[int64]Submission
	// FailQuizPublish refuses the quiz publication with this status.
	FailQuizPublish int
	// Writes records every Canvas write in order.
	Writes []string
	// nextItem is the next module item ID.
	nextItem int64
	// pagePublished and assignmentPublished track live publication.
	pagePublished, assignmentPublished, quizPublished bool
	// pageVisible and assignmentVisible track only_visible_to_overrides.
	pageVisible, assignmentVisible, quizVisible bool
}

const bookClubPageBody = "# Ghost reading calendar\n\n| Week | Reading |\n|---|---|\n| 1 | Chapters 1-3 |\n"

const bookClubQuiz = `{
  "version": "3.0-json", "title": "Ghost reading check",
  "metadata": {"canvas": {"published": true, "module_id": 4, "audience": {"mode": "individual", "policy": "snapshot", "source": {"group_category_id": 50, "group_ids": [60]}}}},
  "items": [
    {"id": "Q1", "type": "MC", "points": 2, "prompt": "Which?", "choices": [{"id":"A","text":"Lantern","correct":true},{"id":"B","text":"Sun","correct":false}]}
  ],
  "rationales": [{"item_id": "Q1", "choices": [{"id":"A","rationale":"The lantern is the detail."},{"id":"B","rationale":"The sun is not in the story."}]}]
}`

func newBookClubUnit(t *testing.T) *bookClubUnit {
	t.Helper()
	u := &bookClubUnit{
		Members:     map[int64][]int64{60: {9001, 9002}, 61: {9003, 9004}},
		Submissions: map[int64]Submission{},
		nextItem:    200,
		PageDates:   pageDateDetails{ID: 7, OnlyVisibleToOverrides: boolPtr(false), VisibleToEveryone: boolPtr(true), Overrides: []pageDateOverride{}},
	}
	// One unrelated entry already exists in the shared module.
	u.ModuleItems = []canvasModuleItem{{ID: 100, Position: 1, Type: "SubHeader", Title: "Book Club"}}
	u.Service, _ = workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		write := func(v any) { _ = json.NewEncoder(w).Encode(v) }
		switch {
		case r.URL.Path == "/api/v1/group_categories/50":
			write(map[string]any{"id": 50, "course_id": 1, "context_type": "Course", "name": "Book Club"})
		case strings.HasPrefix(r.URL.Path, "/api/v1/groups/") && strings.HasSuffix(r.URL.Path, "/memberships"):
			id := int64(60)
			if strings.Contains(r.URL.Path, "/61/") {
				id = 61
			}
			rows := []any{}
			for _, user := range u.Members[id] {
				rows = append(rows, map[string]any{"user_id": user, "group_id": id, "workflow_state": "accepted"})
			}
			write(rows)
		case strings.HasPrefix(r.URL.Path, "/api/v1/groups/"):
			id := int64(60)
			if strings.Contains(r.URL.Path, "/61") {
				id = 61
			}
			name := "Ghost"
			if id == 61 {
				name = "Alone"
			}
			write(map[string]any{"id": id, "course_id": 1, "group_category_id": 50, "context_type": "Course", "name": name})
		case r.URL.Path == "/api/v1/courses/1/enrollments":
			rows := []any{}
			for _, user := range []int64{9001, 9002, 9003, 9004, 9005} {
				rows = append(rows, map[string]any{"user_id": user, "type": "StudentEnrollment", "enrollment_state": "active", "user": map[string]any{"id": user, "name": "Private"}})
			}
			write(rows)

		// Shared existing module.
		case r.URL.Path == "/api/v1/courses/1/modules/4":
			write(map[string]any{"id": 4, "course_id": 1, "name": "Book Club", "published": true})
		case r.URL.Path == "/api/v1/courses/1/modules/4/items" && r.Method == http.MethodGet:
			write(u.ModuleItems)
		case r.URL.Path == "/api/v1/courses/1/modules/4/items" && r.Method == http.MethodPost:
			var payload struct {
				ModuleItem struct {
					Type      string `json:"type"`
					ContentID any    `json:"content_id"`
					PageURL   string `json:"page_url"`
					Title     string `json:"title"`
				} `json:"module_item"`
			}
			_ = json.NewDecoder(r.Body).Decode(&payload)
			u.Writes = append(u.Writes, "module-item:"+payload.ModuleItem.Type)
			u.nextItem++
			created := canvasModuleItem{ID: u.nextItem, Position: len(u.ModuleItems) + 1, Type: payload.ModuleItem.Type, ContentID: payload.ModuleItem.ContentID, PageURL: payload.ModuleItem.PageURL, Title: payload.ModuleItem.Title}
			u.ModuleItems = append(u.ModuleItems, created)
			write(created)
		case strings.HasPrefix(r.URL.Path, "/api/v1/courses/1/modules/4/items/"):
			id, _ := strconv.ParseInt(filepath.Base(r.URL.Path), 10, 64)
			for _, item := range u.ModuleItems {
				if item.ID == id {
					write(item)
					return
				}
			}
			w.WriteHeader(404)

		// Course Page.
		case r.URL.Path == "/api/v1/courses/1/pages" && r.Method == http.MethodPost:
			u.Writes = append(u.Writes, "page-create")
			var payload struct {
				Page struct {
					Title     string `json:"title"`
					Published bool   `json:"published"`
				} `json:"wiki_page"`
			}
			_ = json.NewDecoder(r.Body).Decode(&payload)
			if payload.Page.Published {
				t.Error("audience Page was created published")
			}
			write(map[string]any{"page_id": 7, "url": "ghost-calendar", "title": payload.Page.Title, "body": "<h1>Ghost reading calendar</h1>", "published": false})
		case r.URL.Path == "/api/v1/courses/1/pages/page_id:7":
			if r.Method == http.MethodPut {
				var payload struct {
					Page map[string]any `json:"wiki_page"`
				}
				_ = json.NewDecoder(r.Body).Decode(&payload)
				if v, ok := payload.Page["published"].(bool); ok {
					u.Writes = append(u.Writes, "page-publish")
					u.pagePublished = v
				}
			}
			write(map[string]any{"page_id": 7, "url": "ghost-calendar", "title": "Ghost calendar", "body": "<h1>Ghost reading calendar</h1>", "published": u.pagePublished})
		case r.URL.Path == "/api/v1/courses/1/pages/page_id:7/date_details":
			if r.Method == http.MethodPut {
				u.Writes = append(u.Writes, "page-dates")
				var payload struct {
					OnlyVisible *bool            `json:"only_visible_to_overrides"`
					Overrides   []map[string]any `json:"assignment_overrides"`
				}
				_ = json.NewDecoder(r.Body).Decode(&payload)
				u.PageDates.OnlyVisibleToOverrides = payload.OnlyVisible
				u.PageDates.VisibleToEveryone = boolPtr(payload.OnlyVisible == nil || !*payload.OnlyVisible)
				next := []pageDateOverride{}
				for _, row := range payload.Overrides {
					out := pageDateOverride{}
					if v, ok := row["id"].(float64); ok {
						out.ID = int64(v)
					}
					if v, ok := row["title"].(string); ok {
						out.Title = v
					}
					if ids, ok := row["student_ids"].([]any); ok {
						for _, id := range ids {
							out.StudentIDs = append(out.StudentIDs, int64(id.(float64)))
						}
					}
					next = append(next, out)
				}
				u.PageDates.Overrides = next
				u.pageVisible = payload.OnlyVisible != nil && *payload.OnlyVisible
				w.WriteHeader(http.StatusNoContent)
				return
			}
			write(u.PageDates)

		// Individual response assignment.
		case r.URL.Path == "/api/v1/courses/1/assignments" && r.Method == http.MethodPost:
			u.Writes = append(u.Writes, "assignment-create")
			var payload struct {
				Assignment map[string]any `json:"assignment"`
			}
			_ = json.NewDecoder(r.Body).Decode(&payload)
			if payload.Assignment["published"] != false || payload.Assignment["group_category_id"] != nil {
				t.Errorf("individual response became shared or published: %+v", payload.Assignment)
			}
			write(map[string]any{"id": 10, "course_id": 1, "name": "Ghost individual reading response", "description": "<p>Write independently.</p>", "points_possible": 20, "grading_type": "points", "submission_types": []string{"online_upload"}, "published": false})
		case r.URL.Path == "/api/v1/courses/1/assignments/10":
			if r.Method == http.MethodPut {
				var payload struct {
					Assignment map[string]any `json:"assignment"`
				}
				_ = json.NewDecoder(r.Body).Decode(&payload)
				if v, ok := payload.Assignment["published"].(bool); ok {
					u.Writes = append(u.Writes, "assignment-publish")
					u.assignmentPublished = v
				} else if v, ok := payload.Assignment["only_visible_to_overrides"].(bool); ok {
					u.Writes = append(u.Writes, "assignment-visibility")
					u.assignmentVisible = v
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}
			write(map[string]any{"id": 10, "course_id": 1, "name": "Ghost individual reading response", "description": "<p>Write independently.</p>", "points_possible": 20, "grading_type": "points", "submission_types": []string{"online_upload"}, "published": u.assignmentPublished, "only_visible_to_overrides": u.assignmentVisible, "overrides": u.AssignmentOverrides})
		case r.URL.Path == "/api/v1/courses/1/assignments/10/overrides" && r.Method == http.MethodPost:
			u.Writes = append(u.Writes, "assignment-override")
			var payload struct {
				Override struct {
					StudentIDs []int64 `json:"student_ids"`
				} `json:"assignment_override"`
			}
			_ = json.NewDecoder(r.Body).Decode(&payload)
			u.AssignmentOverrides = append(u.AssignmentOverrides, AssignmentOverrideLive{ID: 80, Title: "Book club individual audience", StudentIDs: payload.Override.StudentIDs, DueAtOverridden: audienceBool(false), UnlockAtOverridden: audienceBool(false), LockAtOverridden: audienceBool(false)})
			write(map[string]any{"id": 80})
		case strings.HasPrefix(r.URL.Path, "/api/v1/courses/1/assignments/10/overrides/"):
			u.Writes = append(u.Writes, "assignment-override-"+r.Method)
			var payload struct {
				Override struct {
					StudentIDs []int64 `json:"student_ids"`
				} `json:"assignment_override"`
			}
			if r.Method != http.MethodDelete {
				_ = json.NewDecoder(r.Body).Decode(&payload)
			}
			id, _ := strconv.ParseInt(filepath.Base(r.URL.Path), 10, 64)
			for i := range u.AssignmentOverrides {
				if u.AssignmentOverrides[i].ID != id {
					continue
				}
				if r.Method == http.MethodDelete {
					u.AssignmentOverrides = append(u.AssignmentOverrides[:i], u.AssignmentOverrides[i+1:]...)
				} else {
					u.AssignmentOverrides[i].StudentIDs = payload.Override.StudentIDs
				}
				break
			}
			w.WriteHeader(http.StatusNoContent)
		case strings.HasPrefix(r.URL.Path, "/api/v1/courses/1/assignments/10/submissions/"):
			user, _ := strconv.ParseInt(filepath.Base(r.URL.Path), 10, 64)
			sub, ok := u.Submissions[user]
			if !ok {
				sub = Submission{UserID: user, AssignmentID: 10, State: "unsubmitted", HistoryPresent: true}
			}
			sub.UserID, sub.AssignmentID = user, 10
			write(sub)

		// New Quiz.
		case r.URL.Path == "/api/quiz/v1/courses/1/quizzes" && r.Method == http.MethodGet:
			write([]any{})
		case r.URL.Path == "/api/quiz/v1/courses/1/quizzes" && r.Method == http.MethodPost:
			u.Writes = append(u.Writes, "quiz-create")
			write(map[string]any{"id": "55"})
		case r.URL.Path == "/api/quiz/v1/courses/1/quizzes/55/items" && r.Method == http.MethodGet:
			write([]any{})
		case r.URL.Path == "/api/quiz/v1/courses/1/quizzes/55/items" && r.Method == http.MethodPost:
			u.Writes = append(u.Writes, "quiz-item")
			write(map[string]any{"id": "1"})
		case r.URL.Path == "/api/v1/courses/1/assignments/55":
			if r.Method == http.MethodPut {
				var payload struct {
					Assignment map[string]any `json:"assignment"`
				}
				_ = json.NewDecoder(r.Body).Decode(&payload)
				if v, ok := payload.Assignment["published"].(bool); ok {
					if v && u.FailQuizPublish > 0 {
						w.WriteHeader(u.FailQuizPublish)
						return
					}
					u.Writes = append(u.Writes, "quiz-publish")
					u.quizPublished = v
				} else if v, ok := payload.Assignment["only_visible_to_overrides"].(bool); ok {
					u.Writes = append(u.Writes, "quiz-visibility")
					u.quizVisible = v
					w.WriteHeader(http.StatusNoContent)
					return
				}
			}
			write(map[string]any{"id": 55, "course_id": 1, "name": "Ghost reading check", "published": u.quizPublished, "only_visible_to_overrides": u.quizVisible, "overrides": u.QuizOverrides, "is_quiz_lti_assignment": true})
		case r.URL.Path == "/api/v1/courses/1/assignments/55/overrides" && r.Method == http.MethodPost:
			u.Writes = append(u.Writes, "quiz-override")
			var payload struct {
				Override struct {
					StudentIDs []int64 `json:"student_ids"`
				} `json:"assignment_override"`
			}
			_ = json.NewDecoder(r.Body).Decode(&payload)
			u.QuizOverrides = append(u.QuizOverrides, AssignmentOverrideLive{ID: 90, Title: "Book club individual audience", StudentIDs: payload.Override.StudentIDs, DueAtOverridden: audienceBool(false), UnlockAtOverridden: audienceBool(false), LockAtOverridden: audienceBool(false)})
			write(map[string]any{"id": 90})
		case strings.HasPrefix(r.URL.Path, "/api/v1/courses/1/assignments/55/overrides/"):
			u.Writes = append(u.Writes, "quiz-override-"+r.Method)
			var payload struct {
				Override struct {
					StudentIDs []int64 `json:"student_ids"`
				} `json:"assignment_override"`
			}
			if r.Method != http.MethodDelete {
				_ = json.NewDecoder(r.Body).Decode(&payload)
			}
			id, _ := strconv.ParseInt(filepath.Base(r.URL.Path), 10, 64)
			for i := range u.QuizOverrides {
				if u.QuizOverrides[i].ID != id {
					continue
				}
				if r.Method == http.MethodDelete {
					u.QuizOverrides = append(u.QuizOverrides[:i], u.QuizOverrides[i+1:]...)
				} else {
					u.QuizOverrides[i].StudentIDs = payload.Override.StudentIDs
				}
				break
			}
			w.WriteHeader(http.StatusNoContent)
		case strings.HasPrefix(r.URL.Path, "/api/v1/courses/1/assignments/55/submissions/"):
			user, _ := strconv.ParseInt(filepath.Base(r.URL.Path), 10, 64)
			sub, ok := u.Submissions[user]
			if !ok {
				sub = Submission{UserID: user, AssignmentID: 55, State: "unsubmitted", HistoryPresent: true}
			}
			sub.UserID, sub.AssignmentID = user, 55
			write(sub)

		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	})
	return u
}

func (u *bookClubUnit) seedStudents(t *testing.T, users ...int64) {
	t.Helper()
	store, err := workspace.OpenStudents(u.Service.Root)
	if err != nil {
		t.Fatal(err)
	}
	for _, user := range users {
		if _, err = store.Upsert(u.Service.Client.Origin(), user, "Private", "", "", ""); err != nil {
			t.Fatal(err)
		}
	}
	if err = store.Save(); err != nil {
		t.Fatal(err)
	}
}

func (u *bookClubUnit) draft(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(u.Service.Root, "courses", "TEST", "drafts", name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func (u *bookClubUnit) ghostAudience() *domain.Audience {
	return &domain.Audience{Mode: "individual", Policy: "snapshot", Source: domain.AudienceSource{GroupCategoryID: 50, GroupIDs: []int64{60}}}
}

// TestBookClubUnitBuildsThreeIndividualResourcesInSharedModule is the integrated
// end-to-end check: one Ghost audience produces a course Page, an individual
// response and a New Quiz, each placed natively in the existing shared module
// while the unrelated entry and the Alone club stay untouched.
func TestBookClubUnitBuildsThreeIndividualResourcesInSharedModule(t *testing.T) {
	u := newBookClubUnit(t)
	u.seedStudents(t, 9001, 9002, 9003, 9004, 9005)
	ctx := context.Background()

	// 1. Course Page for Ghost.
	pagePath := u.draft(t, "ghost-calendar.md")
	page := domain.PageDocument{Schema: 1, Kind: "page", Operation: "create", Course: "TEST", Title: "Ghost calendar", BodyFormat: "markdown", Published: true, ModuleID: 4, Audience: u.ghostAudience()}
	if err := workspace.WriteDocument(pagePath, &page, bookClubPageBody); err != nil {
		t.Fatal(err)
	}
	if _, err := u.Service.PrepareAudience(ctx, "TEST", pagePath, false); err != nil {
		t.Fatalf("page prepare: %v", err)
	}
	pageReport, err := u.Service.PushPage(ctx, pagePath)
	if err != nil || pageReport.Status != "completed" || pageReport.PageID != 7 {
		t.Fatalf("page push %+v %v", pageReport, err)
	}

	// 2. Individual reading response for Ghost.
	responsePath := u.draft(t, "ghost-response.md")
	response := domain.AssignmentDocument{Schema: 1, Kind: "assignment", Operation: "create", Course: "TEST", Title: "Ghost individual reading response", Points: domain.MustDecimal("20"), SubmissionTypes: []string{"online_upload"}, Published: true, ModuleID: 4, Audience: u.ghostAudience()}
	if err := workspace.WriteDocument(responsePath, &response, "Write your own response."); err != nil {
		t.Fatal(err)
	}
	if _, err := u.Service.PrepareAudience(ctx, "TEST", responsePath, false); err != nil {
		t.Fatalf("response prepare: %v", err)
	}
	responseReport, err := u.Service.Push(ctx, responsePath)
	if err != nil || responseReport.Status != "completed" || responseReport.AssignmentID != 10 {
		t.Fatalf("response push %+v %v", responseReport, err)
	}

	// 3. New Quiz for Ghost.
	quizPath := u.draft(t, "ghost-quiz.txt")
	if err := os.WriteFile(quizPath, []byte(bookClubQuiz), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := u.Service.PrepareAudience(ctx, "TEST", quizPath, false); err != nil {
		t.Fatalf("quiz prepare: %v", err)
	}
	quizReport, err := u.Service.PushQuiz(ctx, "TEST", quizPath)
	if err != nil || quizReport.Status != "completed" || quizReport.AssignmentID != 55 || !quizReport.Published {
		t.Fatalf("quiz push %+v %v", quizReport, err)
	}

	// Every resource is restricted to the same two Ghost students, and the
	// source club IDs never became native group settings.
	ghost := []int64{9001, 9002}
	for name, ids := range map[string][]int64{
		"page":     u.PageDates.Overrides[0].StudentIDs,
		"response": u.AssignmentOverrides[0].StudentIDs,
		"quiz":     u.QuizOverrides[0].StudentIDs,
	} {
		if len(ids) != 2 || ids[0] != ghost[0] || ids[1] != ghost[1] {
			t.Fatalf("%s audience is not the Ghost union: %+v", name, ids)
		}
	}
	if !u.pageVisible || !u.assignmentVisible || !u.quizVisible {
		t.Fatalf("a resource was not restricted to overrides: page=%t response=%t quiz=%t", u.pageVisible, u.assignmentVisible, u.quizVisible)
	}
	if !u.pagePublished || !u.assignmentPublished || !u.quizPublished {
		t.Fatalf("a resource was not published after its accepted restriction: page=%t response=%t quiz=%t", u.pagePublished, u.assignmentPublished, u.quizPublished)
	}

	// The shared module keeps its unrelated entry and gains exactly three native
	// entries in the order the teacher asked for them.
	if len(u.ModuleItems) != 4 {
		t.Fatalf("shared module entries: %+v", u.ModuleItems)
	}
	if u.ModuleItems[0].ID != 100 || u.ModuleItems[0].Type != "SubHeader" || u.ModuleItems[0].Title != "Book Club" {
		t.Fatalf("unrelated module entry was changed: %+v", u.ModuleItems[0])
	}
	want := []string{"Page", "Assignment", "Assignment"}
	for i, item := range u.ModuleItems[1:] {
		if item.Type != want[i] {
			t.Fatalf("module entry %d is %s, want %s: %+v", i+2, item.Type, want[i], u.ModuleItems)
		}
	}
	if u.ModuleItems[1].PageURL != "ghost-calendar" {
		t.Fatalf("Page entry does not reference the created Page: %+v", u.ModuleItems[1])
	}
	if contentString(u.ModuleItems[2].ContentID) != "10" || contentString(u.ModuleItems[3].ContentID) != "55" {
		t.Fatalf("assignment entries do not reference the response and quiz assignment: %+v", u.ModuleItems[2:])
	}

	// The Alone club and the unassigned student never appear in any audience.
	for _, ids := range [][]int64{u.PageDates.Overrides[0].StudentIDs, u.AssignmentOverrides[0].StudentIDs, u.QuizOverrides[0].StudentIDs} {
		for _, id := range ids {
			if id == 9003 || id == 9004 || id == 9005 {
				t.Fatalf("audience leaked a non-Ghost student: %+v", ids)
			}
		}
	}
}

// TestBookClubUnitQuizFailureResumesWithoutDuplicatingWork proves the staged
// failure/resume requirement: a refused quiz publication leaves the quiz
// unpublished, and the rerun repeats no accepted question, settings, audience or
// module work.
func TestBookClubUnitQuizFailureResumesWithoutDuplicatingWork(t *testing.T) {
	u := newBookClubUnit(t)
	u.seedStudents(t, 9001, 9002)
	ctx := context.Background()
	u.FailQuizPublish = http.StatusInternalServerError

	quizPath := u.draft(t, "ghost-quiz.txt")
	if err := os.WriteFile(quizPath, []byte(bookClubQuiz), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := u.Service.PrepareAudience(ctx, "TEST", quizPath, false); err != nil {
		t.Fatal(err)
	}
	first, err := u.Service.PushQuiz(ctx, "TEST", quizPath)
	if err == nil || first.Status != "partial" || first.Published {
		t.Fatalf("expected a held publication, got %+v %v", first, err)
	}
	if u.quizPublished {
		t.Fatal("quiz was published despite the refused publication")
	}
	creates, items, overrides := countWrites(u.Writes, "quiz-create"), countWrites(u.Writes, "quiz-item"), countWrites(u.Writes, "quiz-override")
	if creates != 1 || items != 1 || overrides != 1 {
		t.Fatalf("first run wrote %d/%d/%d", creates, items, overrides)
	}

	u.FailQuizPublish = 0
	second, err := u.Service.PushQuiz(ctx, "TEST", quizPath)
	if err != nil || second.Status != "completed" || !second.Published {
		t.Fatalf("resume %+v %v", second, err)
	}
	if countWrites(u.Writes, "quiz-create") != creates || countWrites(u.Writes, "quiz-item") != items || countWrites(u.Writes, "quiz-override") != overrides {
		t.Fatalf("resume repeated accepted stages: %v", u.Writes)
	}
	if len(u.QuizOverrides) != 1 || len(u.QuizOverrides[0].StudentIDs) != 2 {
		t.Fatalf("resume duplicated the audience row: %+v", u.QuizOverrides)
	}
}

// TestBookClubUnitRefreshProtectsWorkAndKeepsIndependentContent proves the
// membership refresh requirement: a Ghost student with real work keeps access
// while the safe removal of another student still proceeds, and the refresh
// changes no Page body, quiz question or settings.
func TestBookClubUnitRefreshProtectsWorkAndKeepsIndependentContent(t *testing.T) {
	u := newBookClubUnit(t)
	u.seedStudents(t, 9001, 9002)
	ctx := context.Background()

	responsePath := u.draft(t, "ghost-response.md")
	response := domain.AssignmentDocument{Schema: 1, Kind: "assignment", Operation: "create", Course: "TEST", Title: "Ghost individual reading response", Points: domain.MustDecimal("20"), SubmissionTypes: []string{"online_upload"}, Published: true, ModuleID: 4, Audience: u.ghostAudience()}
	if err := workspace.WriteDocument(responsePath, &response, "Write your own response."); err != nil {
		t.Fatal(err)
	}
	if _, err := u.Service.PrepareAudience(ctx, "TEST", responsePath, false); err != nil {
		t.Fatal(err)
	}
	report, err := u.Service.Push(ctx, responsePath)
	if err != nil || report.Status != "completed" {
		t.Fatalf("response push %+v %v", report, err)
	}

	// 9001 submitted real work; 9002 has an empty placeholder. Ghost now keeps
	// only 9002, so 9001 is a protected removal and 9002 stays.
	u.Submissions[9001] = Submission{UserID: 9001, AssignmentID: 10, State: "submitted", Body: "<p>My reading response.</p>", HistoryPresent: true}
	u.Members[60] = []int64{9002}
	before := len(u.Writes)

	state, err := u.Service.PrepareAudience(ctx, "TEST", report.Path, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Refresh.Protected) != 1 || state.Refresh.Protected[0].StudentRef == "" {
		t.Fatalf("submitted work was not protected: %+v", state.Refresh)
	}
	action, err := workspace.ConstrainPath(u.Service.Root, state.PreparedPath)
	if err != nil {
		t.Fatal(err)
	}
	refresh, err := u.Service.PushAssignmentDates(ctx, action)
	if err != nil || refresh.Status != "partial" {
		t.Fatalf("refresh %+v %v", refresh, err)
	}
	if len(u.AssignmentOverrides) != 1 || len(u.AssignmentOverrides[0].StudentIDs) != 2 {
		t.Fatalf("protected student lost access: %+v", u.AssignmentOverrides)
	}
	// The refresh touched only the exact override row.
	for _, write := range u.Writes[before:] {
		if !strings.HasPrefix(write, "assignment-override") {
			t.Fatalf("refresh wrote unrelated content: %v", u.Writes[before:])
		}
	}
}

func countWrites(writes []string, prefix string) int {
	n := 0
	for _, write := range writes {
		if write == prefix {
			n++
		}
	}
	return n
}
