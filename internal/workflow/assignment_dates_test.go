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

func TestPushAssignmentDatesTargetsExactOverrideAndNullClearsOnlyRequestedField(t *testing.T) {
	writes := 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/courses/1/assignments/10":
			if r.URL.Query().Get("include[]") == "" {
				t.Error("assignment read omitted overrides/all_dates includes")
			}
			_, _ = w.Write([]byte(`{"id":10,"course_id":1,"name":"Club paper","points_possible":20,"grading_type":"points","submission_types":["online_upload"],"group_category_id":50,"grade_group_students_individually":false,"overrides":[{"id":80,"title":"Club A","group_id":60,"due_at":"2026-10-20T12:00:00Z","unlock_at":"2026-10-10T12:00:00Z"}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/groups/60":
			_, _ = w.Write([]byte(`{"id":60,"course_id":1,"group_category_id":50,"context_type":"Course","non_collaborative":false}`))
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/courses/1/assignments/10/overrides/80":
			writes++
			var body struct {
				Override map[string]json.RawMessage `json:"assignment_override"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if string(body.Override["due_at"]) != "null" {
				t.Errorf("explicit null did not clear due date: %s", body.Override["due_at"])
			}
			if _, ok := body.Override["unlock_at"]; ok {
				t.Error("omitted date should preserve live unlock_at")
			}
			if _, ok := body.Override["lock_at"]; ok {
				t.Error("omitted date should preserve live lock_at")
			}
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
			w.WriteHeader(404)
		}
	})
	due := "2026-10-20T12:00:00Z"
	unlock := "2026-10-10T12:00:00Z"
	doc := domain.AssignmentDatesDocument{Schema: 1, Kind: "assignment-dates", Operation: "update", Course: "TEST", Instance: s.Client.Origin(), CourseID: 1, AssignmentID: 10, Rows: []domain.AssignmentOverrideRow{{Key: "club-a", Operation: "update", OverrideID: 80, GroupID: 60, DueAt: domain.OptionalDate{Set: true, Null: true}, Observed: &domain.AssignmentOverrideBaseline{Title: "Club A", GroupID: 60, DueAt: &due, UnlockAt: &unlock}}}}
	path := filepath.Join(s.Root, "courses", "TEST", "dates.md")
	if err := workspace.WriteDocument(path, &doc, ""); err != nil {
		t.Fatal(err)
	}
	report, err := s.PushAssignmentDates(context.Background(), path)
	if err != nil || report.Status != "completed" || writes != 1 {
		t.Fatalf("report=%+v writes=%d err=%v", report, writes, err)
	}
}

func TestPullAssignmentDatesMapsEffectiveAndOverrideAudienceToPrivateRefs(t *testing.T) {
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/courses/1/assignments/10" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
			return
		}
		_, _ = w.Write([]byte(`{"id":10,"course_id":1,"only_visible_to_overrides":false,"overrides":[{"id":80,"title":"Unmapped extension","student_ids":[9999],"due_at":"2026-10-19T12:00:00Z"},{"id":81,"title":"Jordan extension","student_ids":[9001],"due_at":"2026-10-20T12:00:00Z"}],"all_dates":[{"id":81,"user_id":9001,"due_at":"2026-10-20T12:00:00Z"}]}`))
	})
	store, err := workspace.OpenStudents(s.Root)
	if err != nil {
		t.Fatal(err)
	}
	student, err := store.Upsert(s.Client.Origin(), 9001, "Private Name", "Jordan", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Save(); err != nil {
		t.Fatal(err)
	}
	report, err := s.PullAssignmentDates(context.Background(), "TEST", 10)
	if err != nil || report.Status != "partial" {
		t.Fatalf("%+v %v", report, err)
	}
	var doc domain.AssignmentDatesDocument
	if _, err = workspace.ReadDocument(report.Path, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Rows) != 1 || doc.Rows[0].OverrideID != 81 || len(doc.Rows[0].StudentRefs) != 1 || doc.Rows[0].StudentRefs[0] != student.Ref || doc.Rows[0].Observed == nil || len(doc.EffectiveDates) != 1 || doc.EffectiveDates[0].OverrideID != 81 || doc.EffectiveDates[0].StudentRef != student.Ref {
		t.Fatalf("unexpected private mapping: %+v", doc)
	}
	b, err := os.ReadFile(report.Path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "9001") || strings.Contains(string(b), "Private Name") {
		t.Fatal("Canvas user ID or private name leaked into pulled document")
	}
}

func TestCreateOverrideAcceptedWithoutIDIsNotReplayedAndVisibilityWaits(t *testing.T) {
	posts, visibilityWrites := 0, 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/groups/60":
			_, _ = w.Write([]byte(`{"id":60,"course_id":1,"group_category_id":50,"context_type":"Course","non_collaborative":false}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/courses/1/assignments/10/overrides":
			posts++
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/courses/1/assignments/10":
			visibilityWrites++
			var payload map[string]map[string]bool
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || !payload["assignment"]["only_visible_to_overrides"] {
				t.Errorf("visibility payload = %v, err = %v", payload, err)
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
			w.WriteHeader(http.StatusNotFound)
		}
	})
	mode := false
	created := &domain.CreatedResources{AssignmentID: 10, Overrides: []domain.AssignmentOverrideOutcome{{Key: "club-a"}}}
	doc := domain.AssignmentDocument{Overrides: []domain.AssignmentOverrideRow{{Key: "club-a", Operation: "create", GroupID: 60, DueAt: domain.OptionalDate{Set: true, Value: "2026-10-25T12:00:00Z"}}}, OnlyVisibleToOverrides: boolPtr(true), GradeGroupStudentsIndividually: &mode, GroupCategoryID: 50, Created: created}
	assignment := Assignment{ID: 10, CourseID: 1, GroupCategoryID: ptrInt64(50)}
	save := func() error { return nil }
	complete, err := s.deliverCreateOverrides(context.Background(), 1, assignment, &doc, save)
	if err != nil || !complete || posts != 1 || visibilityWrites != 1 {
		t.Fatalf("first delivery complete=%v posts=%d visibility=%d err=%v", complete, posts, visibilityWrites, err)
	}
	if created.Overrides[0].OverrideID != 0 || created.Overrides[0].Outcome.Status != "succeeded" {
		t.Fatalf("accepted response without ID was not recorded as complete: %+v", created.Overrides[0])
	}
	complete, err = s.deliverCreateOverrides(context.Background(), 1, assignment, &doc, save)
	if err != nil || !complete || posts != 1 || visibilityWrites != 1 {
		t.Fatalf("resume replayed accepted writes: complete=%v posts=%d visibility=%d err=%v", complete, posts, visibilityWrites, err)
	}
}

func TestAssignmentDatesCreateResumeWithAcceptedOverrideID(t *testing.T) {
	secondPosts := 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/courses/1/assignments/10":
			_, _ = w.Write([]byte(`{"id":10,"course_id":1,"group_category_id":50,"overrides":[]}`))
		case r.Method == http.MethodGet && (r.URL.Path == "/api/v1/groups/60" || r.URL.Path == "/api/v1/groups/61"):
			id := int64(60)
			if r.URL.Path == "/api/v1/groups/61" {
				id = 61
			}
			_, _ = fmt.Fprintf(w, `{"id":%d,"course_id":1,"group_category_id":50,"context_type":"Course","non_collaborative":false}`, id)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/courses/1/assignments/10/overrides":
			var body struct {
				Override map[string]any `json:"assignment_override"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			groupID := int(body.Override["group_id"].(float64))
			if groupID == 60 {
				_, _ = w.Write([]byte(`{"id":800}`))
				return
			}
			secondPosts++
			if secondPosts == 1 {
				w.WriteHeader(http.StatusUnprocessableEntity)
				return
			}
			_, _ = w.Write([]byte(`{"id":801}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
			w.WriteHeader(http.StatusNotFound)
		}
	})
	date := func(value string) domain.OptionalDate { return domain.OptionalDate{Set: true, Value: value} }
	doc := domain.AssignmentDatesDocument{Schema: 1, Kind: "assignment-dates", Operation: "update", Course: "TEST", Instance: s.Client.Origin(), CourseID: 1, AssignmentID: 10, Rows: []domain.AssignmentOverrideRow{
		{Key: "first", Operation: "create", GroupID: 60, DueAt: date("2026-10-20T12:00:00Z")},
		{Key: "second", Operation: "create", GroupID: 61, DueAt: date("2026-10-21T12:00:00Z")},
	}}
	path := filepath.Join(s.Root, "courses", "TEST", "dates-resume.md")
	if err := workspace.WriteDocument(path, &doc, ""); err != nil {
		t.Fatal(err)
	}
	first, err := s.PushAssignmentDates(context.Background(), path)
	if err != nil || first.Status != "partial" {
		t.Fatalf("first report=%+v err=%v", first, err)
	}
	var saved domain.AssignmentDatesDocument
	if _, err = workspace.ReadDocument(path, &saved); err != nil {
		t.Fatal(err)
	}
	if err = domain.ValidateAssignmentDates(saved); err != nil {
		t.Fatalf("saved accepted create is not resumable: %v", err)
	}
	if saved.Rows[0].OverrideID != 800 || saved.Rows[0].Outcome.ID != 800 || saved.Rows[0].Outcome.Status != "succeeded" {
		t.Fatalf("first row outcome=%+v", saved.Rows[0])
	}
	fabricated := saved
	fabricated.Rows = append([]domain.AssignmentOverrideRow(nil), saved.Rows...)
	fabricated.Rows[0].Outcome = domain.ActionOutcome{}
	if err = domain.ValidateAssignmentDates(fabricated); err == nil {
		t.Fatal("create with a fabricated override_id was accepted")
	}
	second, err := s.PushAssignmentDates(context.Background(), path)
	if err != nil || second.Status != "completed" || second.Rows[0].Outcome.Status != "succeeded" || second.Rows[1].OverrideID != 801 || secondPosts != 2 {
		t.Fatalf("resume report=%+v posts-for-second=%d err=%v", second, secondPosts, err)
	}
}

func TestAssignmentDatesUncertainOverrideRemainsUnresolvedAcrossResumes(t *testing.T) {
	writes := 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/courses/1/assignments/10":
			_, _ = w.Write([]byte(`{"id":10,"course_id":1,"group_category_id":50,"overrides":[]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/groups/60":
			_, _ = w.Write([]byte(`{"id":60,"course_id":1,"group_category_id":50,"context_type":"Course","non_collaborative":false}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/courses/1/assignments/10/overrides":
			writes++
			w.WriteHeader(http.StatusRequestTimeout)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
			w.WriteHeader(http.StatusNotFound)
		}
	})
	doc := domain.AssignmentDatesDocument{Schema: 1, Kind: "assignment-dates", Operation: "update", Course: "TEST", Instance: s.Client.Origin(), CourseID: 1, AssignmentID: 10, Rows: []domain.AssignmentOverrideRow{{Key: "club", Operation: "create", GroupID: 60, DueAt: domain.OptionalDate{Set: true, Value: "2026-10-20T12:00:00Z"}}}}
	path := filepath.Join(s.Root, "courses", "TEST", "uncertain.md")
	if err := workspace.WriteDocument(path, &doc, ""); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		report, err := s.PushAssignmentDates(context.Background(), path)
		if err != nil || report.Status != "partial" {
			t.Fatalf("run %d report=%+v err=%v", i+1, report, err)
		}
	}
	var saved domain.AssignmentDatesDocument
	if _, err := workspace.ReadDocument(path, &saved); err != nil {
		t.Fatal(err)
	}
	if writes != 1 || saved.Rows[0].Outcome.Status != "uncertain" {
		t.Fatalf("writes=%d outcome=%+v", writes, saved.Rows[0].Outcome)
	}
}

func TestAssignmentDatesAcceptedVisibilityDoesNotReplayAfterExternalRestore(t *testing.T) {
	writes := 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/courses/1/assignments/10":
			_, _ = w.Write([]byte(`{"id":10,"course_id":1,"only_visible_to_overrides":false,"overrides":[]}`))
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/courses/1/assignments/10":
			writes++
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
			w.WriteHeader(http.StatusNotFound)
		}
	})
	requested, baseline := true, false
	doc := domain.AssignmentDatesDocument{Schema: 1, Kind: "assignment-dates", Operation: "update", Course: "TEST", Instance: s.Client.Origin(), CourseID: 1, AssignmentID: 10, OnlyVisibleToOverrides: &requested, ObservedOnlyVisibleToOverrides: &baseline, Rows: []domain.AssignmentOverrideRow{}}
	path := filepath.Join(s.Root, "courses", "TEST", "visibility.md")
	if err := workspace.WriteDocument(path, &doc, ""); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		report, err := s.PushAssignmentDates(context.Background(), path)
		if err != nil || report.Status != "completed" {
			t.Fatalf("run %d report=%+v err=%v", i+1, report, err)
		}
	}
	var saved domain.AssignmentDatesDocument
	if _, err := workspace.ReadDocument(path, &saved); err != nil {
		t.Fatal(err)
	}
	if writes != 1 || saved.VisibilityOutcome.Status != "succeeded" || saved.VisibilityTarget == nil || !*saved.VisibilityTarget {
		t.Fatalf("writes=%d visibility=%+v target=%v", writes, saved.VisibilityOutcome, saved.VisibilityTarget)
	}
}

func TestCreateVisibilityUncertainStaysUnresolvedAcrossResumes(t *testing.T) {
	writes := 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut && r.URL.Path == "/api/v1/courses/1/assignments/10" {
			writes++
			w.WriteHeader(http.StatusRequestTimeout)
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
		w.WriteHeader(http.StatusNotFound)
	})
	created := &domain.CreatedResources{AssignmentID: 10}
	requested, mode := true, false
	doc := domain.AssignmentDocument{OnlyVisibleToOverrides: &requested, GradeGroupStudentsIndividually: &mode, Created: created}
	assignment := Assignment{ID: 10, CourseID: 1}
	for i := 0; i < 3; i++ {
		complete, err := s.deliverCreateOverrides(context.Background(), 1, assignment, &doc, func() error { return nil })
		if err != nil || complete {
			t.Fatalf("run %d complete=%v err=%v", i+1, complete, err)
		}
	}
	if writes != 1 || created.Visibility.Status != "uncertain" {
		t.Fatalf("writes=%d visibility=%+v", writes, created.Visibility)
	}
}

func boolPtr(value bool) *bool { return &value }

func TestAssignmentDatesWrongSetStopsBeforeOverrideWrite(t *testing.T) {
	writes := 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/courses/1/assignments/10":
			_, _ = w.Write([]byte(`{"id":10,"course_id":1,"group_category_id":50,"grade_group_students_individually":false,"overrides":[]}`))
		case "/api/v1/groups/60":
			_, _ = w.Write([]byte(`{"id":60,"course_id":1,"group_category_id":51,"context_type":"Course","non_collaborative":false}`))
		default:
			if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodDelete {
				writes++
			}
			w.WriteHeader(404)
		}
	})
	doc := domain.AssignmentDatesDocument{Schema: 1, Kind: "assignment-dates", Operation: "update", Course: "TEST", Instance: s.Client.Origin(), CourseID: 1, AssignmentID: 10, Rows: []domain.AssignmentOverrideRow{{Key: "wrong", Operation: "create", GroupID: 60, DueAt: domain.OptionalDate{Set: true, Value: "2026-10-20T12:00:00Z"}}}}
	path := filepath.Join(s.Root, "courses", "TEST", "wrong.md")
	if err := workspace.WriteDocument(path, &doc, ""); err != nil {
		t.Fatal(err)
	}
	report, err := s.PushAssignmentDates(context.Background(), path)
	if err != nil || report.Status != "partial" || writes != 0 || len(report.Rows) != 1 || report.Rows[0].Outcome.Status != "held" {
		t.Fatalf("report=%+v writes=%d err=%v", report, writes, err)
	}
}
