package workflow

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

const quizAudienceFixture = `{
  "version": "3.0-json", "title": "Ghost reading check",
  "metadata": {"canvas": {"published": true, "audience": {"mode": "individual", "policy": "snapshot", "source": {"group_category_id": 50, "group_ids": [60]}}}},
  "items": [
    {"id": "Q1", "type": "MC", "points": 2, "prompt": "Which?", "choices": [{"id":"A","text":"Lantern","correct":true},{"id":"B","text":"Sun","correct":false}]}
  ]
}`

type quizAudienceFixtureState struct {
	Service   *Service
	Members   map[int64][]int64
	Published bool
	// Overrides is the live assignment override collection.
	Overrides []AssignmentOverrideLive
	// Visible is the live only_visible_to_overrides setting.
	Visible bool
	// FailAudience refuses the override write with this status.
	FailAudience int
	// FailPublish refuses publication with this status.
	FailPublish int
	// QuizCreates, ItemPosts, SettingsPuts and Publishes count writes.
	QuizCreates, ItemPosts, SettingsPuts, Publishes int
	// Submissions maps a user ID to a submission body; empty means no work.
	Submissions map[int64]Submission
}

func newQuizAudienceFixture(t *testing.T, members map[int64][]int64) *quizAudienceFixtureState {
	t.Helper()
	f := &quizAudienceFixtureState{Members: members, Submissions: map[int64]Submission{}}
	f.Service, _ = workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
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
			for _, user := range f.Members[id] {
				rows = append(rows, map[string]any{"user_id": user, "group_id": id, "workflow_state": "accepted"})
			}
			write(rows)
		case strings.HasPrefix(r.URL.Path, "/api/v1/groups/"):
			id := int64(60)
			if strings.Contains(r.URL.Path, "/61") {
				id = 61
			}
			write(map[string]any{"id": id, "course_id": 1, "group_category_id": 50, "context_type": "Course", "name": "Ghost"})
		case r.URL.Path == "/api/v1/courses/1/enrollments":
			rows := []any{}
			for _, users := range f.Members {
				for _, user := range users {
					rows = append(rows, map[string]any{"user_id": user, "type": "StudentEnrollment", "enrollment_state": "active", "user": map[string]any{"id": user, "name": "Private"}})
				}
			}
			write(rows)
		case r.Method == http.MethodGet && r.URL.Path == "/api/quiz/v1/courses/1/quizzes":
			write([]any{})
		case r.Method == http.MethodPost && r.URL.Path == "/api/quiz/v1/courses/1/quizzes":
			f.QuizCreates++
			write(map[string]any{"id": "55"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/quiz/v1/courses/1/quizzes/55/items":
			write([]any{})
		case r.Method == http.MethodPost && r.URL.Path == "/api/quiz/v1/courses/1/quizzes/55/items":
			f.ItemPosts++
			write(map[string]any{"id": "1"})
		case r.URL.Path == "/api/v1/courses/1/assignments/55":
			if r.Method == http.MethodPut {
				var payload struct {
					Assignment map[string]any `json:"assignment"`
				}
				_ = json.NewDecoder(r.Body).Decode(&payload)
				if v, ok := payload.Assignment["published"].(bool); ok {
					if v && f.FailPublish > 0 {
						w.WriteHeader(f.FailPublish)
						return
					}
					f.Published = v
				} else if v, ok := payload.Assignment["only_visible_to_overrides"].(bool); ok {
					f.Visible = v
				} else {
					f.SettingsPuts++
				}
			}
			write(map[string]any{"id": 55, "course_id": 1, "name": "Ghost reading check", "published": f.Published, "only_visible_to_overrides": f.Visible, "overrides": f.Overrides, "is_quiz_lti_assignment": true})
		case r.URL.Path == "/api/v1/courses/1/assignments/55/overrides":
			if f.FailAudience > 0 {
				w.WriteHeader(f.FailAudience)
				return
			}
			var payload struct {
				Override struct {
					StudentIDs []int64 `json:"student_ids"`
				} `json:"assignment_override"`
			}
			_ = json.NewDecoder(r.Body).Decode(&payload)
			f.Overrides = append(f.Overrides, AssignmentOverrideLive{ID: 80, Title: "Book club individual audience", StudentIDs: payload.Override.StudentIDs, DueAtOverridden: audienceBool(false), UnlockAtOverridden: audienceBool(false), LockAtOverridden: audienceBool(false)})
			write(map[string]any{"id": 80})
		case strings.HasPrefix(r.URL.Path, "/api/v1/courses/1/assignments/55/overrides/"):
			var payload struct {
				Override struct {
					StudentIDs []int64 `json:"student_ids"`
				} `json:"assignment_override"`
			}
			_ = json.NewDecoder(r.Body).Decode(&payload)
			for i := range f.Overrides {
				if f.Overrides[i].ID == 80 {
					f.Overrides[i].StudentIDs = payload.Override.StudentIDs
				}
			}
			w.WriteHeader(http.StatusNoContent)
		case strings.HasPrefix(r.URL.Path, "/api/v1/courses/1/assignments/55/submissions/"):
			user := int64(0)
			_, _ = fmtSscan(strings.TrimPrefix(r.URL.Path, "/api/v1/courses/1/assignments/55/submissions/"), &user)
			sub, ok := f.Submissions[user]
			if !ok {
				sub = Submission{UserID: user, AssignmentID: 55, State: "unsubmitted", HistoryPresent: true}
			}
			sub.UserID, sub.AssignmentID = user, 55
			write(sub)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/courses/1/modules/9/items":
			write([]any{})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/courses/1/modules/9/items":
			write(map[string]any{"id": 1})
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	})
	return f
}

func fmtSscan(s string, out *int64) (int, error) {
	var n int64
	for _, r := range s {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int64(r-'0')
	}
	*out = n
	return 1, nil
}

func (f *quizAudienceFixtureState) seedStudents(t *testing.T, users ...int64) {
	t.Helper()
	store, err := workspace.OpenStudents(f.Service.Root)
	if err != nil {
		t.Fatal(err)
	}
	for _, user := range users {
		if _, err = store.Upsert(f.Service.Client.Origin(), user, "Private", "", "", ""); err != nil {
			t.Fatal(err)
		}
	}
	if err = store.Save(); err != nil {
		t.Fatal(err)
	}
}

func (f *quizAudienceFixtureState) writeQuiz(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(f.Service.Root, "courses", "TEST", "drafts", "ghost-quiz.txt")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestQuizAudienceRestrictsAssociatedAssignmentBeforePublication(t *testing.T) {
	f := newQuizAudienceFixture(t, map[int64][]int64{60: {9001, 9002}})
	f.seedStudents(t, 9001, 9002)
	path := f.writeQuiz(t, quizAudienceFixture)
	state, err := f.Service.PrepareAudience(context.Background(), "TEST", path, false)
	if err != nil || len(state.StudentRefs) != 2 || !state.Complete {
		t.Fatalf("prepare %+v %v", state, err)
	}
	review, err := BuildReview(f.Service.Root, path)
	if err != nil || len(review.Cards) == 0 {
		t.Fatalf("review %v", err)
	}
	report, err := f.Service.PushQuiz(context.Background(), "TEST", path)
	if err != nil || report.Status != "completed" || !report.Published {
		t.Fatalf("push %+v %v", report, err)
	}
	if f.QuizCreates != 1 || f.ItemPosts != 1 {
		t.Fatalf("quiz/item writes: %d/%d", f.QuizCreates, f.ItemPosts)
	}
	if len(f.Overrides) != 1 || len(f.Overrides[0].StudentIDs) != 2 {
		t.Fatalf("audience not applied: %+v", f.Overrides)
	}
	if !f.Visible {
		t.Fatal("assignment was not restricted to overrides")
	}
	if report.Audience == "" {
		t.Fatal("report did not describe the applied audience")
	}
	// The sidecar records origin/course and the accepted stages.
	rec := readSidecar(path)
	if rec.Instance != f.Service.Client.Origin() || rec.CourseID != 1 || rec.AssignmentID != 55 {
		t.Fatalf("sidecar identity: %+v", rec)
	}
	if rec.Stages == nil || rec.Stages.Audience.Status != "succeeded" || rec.Stages.Publication.Status != "succeeded" {
		t.Fatalf("accepted stages not saved: %+v", rec.Stages)
	}
}

func TestQuizAudienceFailureKeepsQuizUnpublished(t *testing.T) {
	f := newQuizAudienceFixture(t, map[int64][]int64{60: {9001}})
	f.seedStudents(t, 9001)
	f.FailAudience = http.StatusForbidden
	path := f.writeQuiz(t, quizAudienceFixture)
	if _, err := f.Service.PrepareAudience(context.Background(), "TEST", path, false); err != nil {
		t.Fatal(err)
	}
	report, err := f.Service.PushQuiz(context.Background(), "TEST", path)
	if err != nil || report.Status != "partial" || report.Published {
		t.Fatalf("push %+v %v", report, err)
	}
	if f.Publishes != 0 || f.Published {
		t.Fatal("quiz was published despite a refused audience")
	}
	rec := readSidecar(path)
	if rec.AssignmentID != 55 || rec.Stages == nil || rec.Stages.Items.Status != "succeeded" {
		t.Fatalf("accepted item stage not retained: %+v", rec)
	}
	if rec.Stages.Audience.Status == "succeeded" {
		t.Fatal("refused audience recorded as accepted")
	}
}

func TestQuizAudienceResumeDoesNotDuplicateAcceptedStages(t *testing.T) {
	f := newQuizAudienceFixture(t, map[int64][]int64{60: {9001}})
	f.seedStudents(t, 9001)
	f.FailPublish = http.StatusInternalServerError
	path := f.writeQuiz(t, quizAudienceFixture)
	if _, err := f.Service.PrepareAudience(context.Background(), "TEST", path, false); err != nil {
		t.Fatal(err)
	}
	if report, err := f.Service.PushQuiz(context.Background(), "TEST", path); err == nil || report.Status != "partial" {
		t.Fatalf("expected publication failure, got %+v %v", report, err)
	}
	creates, items, settings := f.QuizCreates, f.ItemPosts, f.SettingsPuts
	f.FailPublish = 0
	report, err := f.Service.PushQuiz(context.Background(), "TEST", path)
	if err != nil || report.Status != "completed" || !report.Published {
		t.Fatalf("resume %+v %v", report, err)
	}
	if f.QuizCreates != creates || f.ItemPosts != items || f.SettingsPuts != settings {
		t.Fatalf("resume repeated accepted stages: quiz %d->%d items %d->%d settings %d->%d", creates, f.QuizCreates, items, f.ItemPosts, settings, f.SettingsPuts)
	}
	if len(f.Overrides) != 1 {
		t.Fatalf("resume duplicated the audience row: %+v", f.Overrides)
	}
}

func TestQuizAudienceRefreshProtectsAttemptsAndSendsNoQuizWrites(t *testing.T) {
	f := newQuizAudienceFixture(t, map[int64][]int64{60: {9001, 9002}})
	f.seedStudents(t, 9001, 9002)
	path := f.writeQuiz(t, quizAudienceFixture)
	if _, err := f.Service.PrepareAudience(context.Background(), "TEST", path, false); err != nil {
		t.Fatal(err)
	}
	if report, err := f.Service.PushQuiz(context.Background(), "TEST", path); err != nil || report.Status != "completed" {
		t.Fatalf("initial push %+v %v", report, err)
	}
	creates, items, settings := f.QuizCreates, f.ItemPosts, f.SettingsPuts
	// 9001 has a genuine attempt; 9002 has none. Membership drops 9001.
	attempt := 2
	f.Submissions[9001] = Submission{UserID: 9001, AssignmentID: 55, State: "submitted", Attempt: attempt, HistoryPresent: true}
	f.Members[60] = []int64{9002}
	state, err := f.Service.PrepareAudience(context.Background(), "TEST", path, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Refresh.Protected) != 1 || state.Refresh.Protected[0].StudentRef == "" {
		t.Fatalf("attempt was not protected: %+v", state.Refresh)
	}
	action := filepath.Join(f.Service.Root, filepath.FromSlash(state.PreparedPath))
	report, err := f.Service.PushAssignmentDates(context.Background(), action)
	if err != nil || report.Status != "partial" {
		t.Fatalf("refresh %+v %v", report, err)
	}
	if f.QuizCreates != creates || f.ItemPosts != items || f.SettingsPuts != settings {
		t.Fatal("audience-only refresh wrote quiz content or settings")
	}
	if len(f.Overrides) != 1 || len(f.Overrides[0].StudentIDs) != 2 {
		t.Fatalf("protected student lost access: %+v", f.Overrides)
	}
}

func TestQuizAudienceRejectsWrongOriginAndGroupAssignment(t *testing.T) {
	f := newQuizAudienceFixture(t, map[int64][]int64{60: {9001}})
	f.seedStudents(t, 9001)
	path := f.writeQuiz(t, quizAudienceFixture)
	// A sidecar bound to another Canvas must not be reused.
	if err := os.WriteFile(sidecarPath(path), []byte(`{"assignment_id":55,"instance":"https://other.example","course_id":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Service.PushQuiz(context.Background(), "TEST", path); err == nil || !strings.Contains(err.Error(), "different Canvas") {
		t.Fatalf("wrong-origin sidecar accepted: %v", err)
	}
	if f.QuizCreates != 0 {
		t.Fatal("wrong-origin sidecar caused a write")
	}
}

func TestQuizAudienceSettingsRejectInvalidIntent(t *testing.T) {
	body := `{"title":"Bad","metadata":{"canvas":{"audience":{"mode":"shared","policy":"snapshot","source":{"group_category_id":50,"group_ids":[60]}}}},"items":[{"id":"Q1","type":"MC","points":1,"prompt":"q","choices":[{"text":"a","correct":true}]}]}`
	if _, err := ReadQuizForge(writeQF(t, body)); err == nil {
		t.Fatal("invalid audience mode accepted")
	}
	body = `{"title":"Bad","metadata":{"canvas":{"audience":{"mode":"individual","policy":"snapshot","source":{"group_category_id":50,"group_ids":[]}}}},"items":[{"id":"Q1","type":"MC","points":1,"prompt":"q","choices":[{"text":"a","correct":true}]}]}`
	if _, err := ReadQuizForge(writeQF(t, body)); err == nil {
		t.Fatal("empty audience source accepted")
	}
}

func TestQuizAudienceSidecarKeepsOldRecordsUsable(t *testing.T) {
	f := newQuizAudienceFixture(t, map[int64][]int64{60: {9001}})
	f.seedStudents(t, 9001)
	path := f.writeQuiz(t, strings.Replace(quizAudienceFixture, `"audience": {"mode": "individual", "policy": "snapshot", "source": {"group_category_id": 50, "group_ids": [60]}}`, `"published": true`, 1))
	// An old sidecar has only assignment_id and tier pages.
	if err := os.WriteFile(sidecarPath(path), []byte(`{"assignment_id":55,"tier_pages":[{"label":"Core","title":"Core page","page_id":7}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := f.Service.PushQuiz(context.Background(), "TEST", path)
	if err != nil || report.Status != "completed" {
		t.Fatalf("old sidecar unusable: %+v %v", report, err)
	}
	rec := readSidecar(path)
	if rec.AssignmentID != 55 || len(rec.TierPages) != 1 || rec.TierPages[0].PageID != 7 {
		t.Fatalf("old sidecar fields lost: %+v", rec)
	}
	if rec.Stages != nil {
		t.Fatal("ordinary quiz gained restricted stages")
	}
}

var _ = io.Discard
var _ = domain.ActionOutcome{}
