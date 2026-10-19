package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

type refreshFixture struct {
	Service    *Service
	Members    []int64
	Rows       []map[string]any
	Evidence   map[int64]map[string]any
	ReadFail   map[int64]bool
	Writes     []string
	Payloads   []map[string]any
	FailCreate bool
}

func newRefreshFixture(t *testing.T) *refreshFixture {
	t.Helper()
	f := &refreshFixture{Members: []int64{9001, 9002}, Evidence: map[int64]map[string]any{}, ReadFail: map[int64]bool{}}
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		write := func(v any) { _ = json.NewEncoder(w).Encode(v) }
		switch {
		case r.URL.Path == "/api/v1/group_categories/50":
			write(map[string]any{"id": 50, "course_id": 1, "context_type": "Course", "name": "Books"})
		case r.URL.Path == "/api/v1/groups/60":
			write(map[string]any{"id": 60, "course_id": 1, "context_type": "Course", "group_category_id": 50, "name": "Ghost"})
		case r.URL.Path == "/api/v1/groups/60/memberships":
			rows := []any{}
			for _, id := range f.Members {
				rows = append(rows, map[string]any{"group_id": 60, "user_id": id, "workflow_state": "accepted"})
			}
			write(rows)
		case r.URL.Path == "/api/v1/courses/1/enrollments":
			rows := []any{}
			for id := int64(9001); id <= 9005; id++ {
				rows = append(rows, map[string]any{"user_id": id, "type": "StudentEnrollment", "enrollment_state": "active", "user": map[string]any{"id": id, "name": fmt.Sprintf("Private Name %d", id)}})
			}
			write(rows)
		case strings.Contains(r.URL.Path, "/submissions/"):
			id, _ := strconv.ParseInt(filepath.Base(r.URL.Path), 10, 64)
			if f.ReadFail[id] {
				w.WriteHeader(403)
				return
			}
			sub := map[string]any{"user_id": id, "assignment_id": 10, "attempt": 1, "workflow_state": "unsubmitted", "submission_history": []any{}}
			if f.Evidence[id] != nil {
				sub = f.Evidence[id]
			}
			write(sub)
		case r.URL.Path == "/api/v1/courses/1/assignments/10" && r.Method == http.MethodGet:
			write(map[string]any{"id": 10, "course_id": 1, "name": "Response", "only_visible_to_overrides": true, "published": true, "overrides": f.Rows})
		case strings.HasPrefix(r.URL.Path, "/api/v1/courses/1/assignments/10/overrides"):
			f.Writes = append(f.Writes, r.Method+" "+r.URL.Path)
			var body map[string]map[string]any
			if r.Method != http.MethodDelete {
				_ = json.NewDecoder(r.Body).Decode(&body)
			}
			fields := body["assignment_override"]
			f.Payloads = append(f.Payloads, fields)
			if r.Method == http.MethodPost {
				if f.FailCreate {
					w.WriteHeader(503)
					return
				}
				id := int64(80 + len(f.Writes))
				fields["id"] = id
				f.Rows = append(f.Rows, fields)
				write(map[string]any{"id": id})
				return
			}
			id, _ := strconv.ParseInt(filepath.Base(r.URL.Path), 10, 64)
			for i, row := range f.Rows {
				rowID, _ := strconv.ParseInt(fmt.Sprint(row["id"]), 10, 64)
				if rowID != id {
					continue
				}
				if r.Method == http.MethodDelete {
					f.Rows = append(f.Rows[:i], f.Rows[i+1:]...)
				} else {
					for k, v := range fields {
						row[k] = v
					}
				}
				break
			}
			w.WriteHeader(204)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL)
			w.WriteHeader(404)
		}
	})
	f.Service = s
	return f
}
func (f *refreshFixture) initial(t *testing.T) string {
	t.Helper()
	s := f.Service
	path := filepath.Join(s.Root, "courses", "TEST", "drafts", "initial.md")
	d := domain.AssignmentDatesDocument{Schema: 1, Kind: "assignment-dates", Operation: "update", Course: "TEST", Instance: s.Client.Origin(), CourseID: 1, AssignmentID: 10, Audience: individualAudience(), Rows: []domain.AssignmentOverrideRow{}}
	if err := workspace.WriteDocument(path, d, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PrepareAudience(context.Background(), "TEST", path, false); err != nil {
		t.Fatal(err)
	}
	// The initial restriction is already true; initial targeting uses its existing
	// exact audience route, with one explicit restriction PUT accepted below.
	d.Audience.State = nil
	var prepared domain.AssignmentDatesDocument
	_, _ = workspace.ReadDocument(path, &prepared)
	prepared.Audience.State.Visibility = domain.ActionOutcome{Status: "preserved"}
	_ = workspace.WriteDocument(path, prepared, "")
	report, err := s.PushAssignmentDates(context.Background(), path)
	if err != nil || report.Status != "completed" {
		t.Fatalf("initial %+v %v", report, err)
	}
	// Normalize fixture numeric IDs to JSON's read shape for later route updates.
	var rows []map[string]any
	b, _ := json.Marshal(f.Rows)
	_ = json.Unmarshal(b, &rows)
	f.Rows = rows
	f.Writes = nil
	f.Payloads = nil
	return path
}
func (f *refreshFixture) ref(t *testing.T, id int64) string {
	t.Helper()
	store, e := workspace.OpenStudents(f.Service.Root)
	if e != nil {
		t.Fatal(e)
	}
	for _, student := range store.Rows {
		if student.UserID == id {
			return student.Ref
		}
	}
	t.Fatal("missing synthetic ref")
	return ""
}
func (f *refreshFixture) actionPath(state *domain.AudienceState) string {
	path, _ := workspace.ConstrainPath(f.Service.Root, state.PreparedPath)
	return path
}

func TestAudienceRefreshSafeChangesProtectedWorkAndManualExceptions(t *testing.T) {
	f := newRefreshFixture(t)
	path := f.initial(t)
	a, b, c := f.ref(t, 9001), f.ref(t, 9002), f.ref(t, 9003)
	f.Rows = append(f.Rows, map[string]any{"id": float64(70), "title": "Manual extension", "student_ids": []any{float64(9004)}, "due_at": "2026-11-01T12:00:00Z"})
	f.Members = []int64{9003}
	f.Evidence[9002] = map[string]any{"user_id": 9002, "assignment_id": 10, "workflow_state": "unsubmitted", "score": 0, "submission_history": []any{}}
	state, err := f.Service.PrepareAudience(context.Background(), "TEST", path, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Refresh.Add) != 1 || state.Refresh.Add[0] != c || len(state.Refresh.Remove) != 1 || state.Refresh.Remove[0] != a || len(state.Refresh.Protected) != 1 || state.Refresh.Protected[0].StudentRef != b {
		t.Fatalf("wrong diff %+v", state.Refresh)
	}
	model, err := BuildReview(f.Service.Root, f.actionPath(state))
	if err != nil || !strings.Contains(fmt.Sprint(model.Cards), "Protected access") {
		t.Fatalf("review %+v %v", model, err)
	}
	report, err := f.Service.PushAssignmentDates(context.Background(), f.actionPath(state))
	if err != nil || report.Status != "partial" || len(f.Writes) != 2 {
		t.Fatalf("%+v %v writes %+v", report, err, f.Writes)
	}
	var saved domain.AssignmentDatesDocument
	_, _ = workspace.ReadDocument(report.Path, &saved)
	if !reflectStrings(saved.Audience.State.AppliedRefs, []string{b, c}) {
		t.Fatalf("wrong applied refs %+v", saved.Audience.State.AppliedRefs)
	}
	if len(f.Rows) != 3 || f.Rows[1]["due_at"] != "2026-11-01T12:00:00Z" {
		t.Fatalf("manual row changed %+v", f.Rows)
	}
	_, err = f.Service.PushAssignmentDates(context.Background(), report.Path)
	if err != nil || len(f.Writes) != 2 {
		t.Fatalf("accepted change replayed %+v %v", f.Writes, err)
	}
}
func reflectStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	set := stringSet(a)
	for _, ref := range b {
		if !set[ref] {
			return false
		}
	}
	return true
}

func TestAudienceRefreshHistoryUnknownAndPlaceholderEvidence(t *testing.T) {
	for _, mode := range []string{"history", "unreadable", "missing-history", "placeholder", "new-work"} {
		t.Run(mode, func(t *testing.T) {
			f := newRefreshFixture(t)
			path := f.initial(t)
			f.Members = []int64{9002}
			switch mode {
			case "history":
				f.Evidence[9001] = map[string]any{"user_id": 9001, "assignment_id": 10, "workflow_state": "unsubmitted", "submission_history": []any{map[string]any{"user_id": 9001, "assignment_id": 10, "submitted_at": "2026-10-18T12:00:00Z"}}}
			case "unreadable":
				f.ReadFail[9001] = true
			case "missing-history":
				f.Evidence[9001] = map[string]any{"user_id": 9001, "assignment_id": 10, "workflow_state": "unsubmitted"}
			}
			state, err := f.Service.PrepareAudience(context.Background(), "TEST", path, true)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "new-work" {
				f.Evidence[9001] = map[string]any{"user_id": 9001, "assignment_id": 10, "workflow_state": "submitted", "submitted_at": "2026-10-19T12:00:00Z"}
			}
			report, err := f.Service.PushAssignmentDates(context.Background(), f.actionPath(state))
			if err != nil {
				t.Fatal(err)
			}
			if mode == "placeholder" {
				if report.Status != "completed" || len(f.Writes) != 1 || !strings.HasPrefix(f.Writes[0], "PUT ") {
					t.Fatalf("empty placeholder protected incorrectly %+v %+v", report, f.Writes)
				}
			} else if report.Status != "partial" || len(f.Writes) != 0 {
				t.Fatalf("protected removal wrote %+v %+v", report, f.Writes)
			}
		})
	}
}

func TestAudienceRefreshCompletedCreateLatestOwnershipAndDatePresence(t *testing.T) {
	f := newRefreshFixture(t)
	initial := f.initial(t)
	var source domain.AssignmentDatesDocument
	_, _ = workspace.ReadDocument(initial, &source)
	unlock := "2026-10-19T12:00:00Z"
	f.Rows[0]["due_at"] = nil
	f.Rows[0]["due_at_overridden"] = true
	f.Rows[0]["unlock_at"] = unlock
	f.Rows[0]["unlock_at_overridden"] = true
	f.Rows[0]["lock_at_overridden"] = false
	source.Audience.State.Owned[0].Baseline.DueAtOverridden = audienceBool(true)
	source.Audience.State.Owned[0].Baseline.UnlockAtOverridden = audienceBool(true)
	source.Audience.State.Owned[0].Baseline.UnlockAt = &unlock
	receipt := domain.AssignmentDocument{Schema: 1, Kind: "assignment", Operation: "create", Course: "TEST", Instance: f.Service.Client.Origin(), CourseID: 1, Title: "Response", Points: domain.MustDecimal("20"), SubmissionTypes: []string{"online_upload"}, Published: true, Audience: source.Audience, Created: &domain.CreatedResources{AssignmentID: 10, Complete: true, Assignment: domain.ActionOutcome{Status: "succeeded", ID: 10}, Rubric: domain.ActionOutcome{Status: "not-requested"}, Module: domain.ActionOutcome{Status: "not-requested"}, Publication: domain.ActionOutcome{Status: "preserved"}}}
	path := filepath.Join(f.Service.Root, "courses", "TEST", "pushed", "response.md")
	_ = workspace.WriteDocument(path, receipt, "Directions")
	f.Members = []int64{9002, 9003}
	state, err := f.Service.PrepareAudience(context.Background(), "TEST", path, true)
	if err != nil {
		t.Fatal(err)
	}
	if state.PreparedPath == "" {
		t.Fatal("no returned action path")
	}
	report, err := f.Service.PushAssignmentDates(context.Background(), f.actionPath(state))
	if err != nil || report.Status != "completed" || len(f.Writes) != 2 {
		t.Fatalf("%+v %v %+v", report, err, f.Writes)
	}
	payload := f.Payloads[0]
	if v, ok := payload["due_at"]; !ok || v != nil {
		t.Fatalf("explicit-null due date lost %+v", payload)
	}
	if payload["unlock_at"] != unlock {
		t.Fatalf("overridden unlock date lost %+v", payload)
	}
	if _, ok := payload["lock_at"]; ok {
		t.Fatalf("inherited lock date turned into override %+v", payload)
	}
	var saved domain.AssignmentDocument
	_, _ = workspace.ReadDocument(path, &saved)
	if !saved.Created.Complete || saved.Created.AssignmentID != 10 || !containsString(saved.Audience.State.Owned[0].Baseline.StudentRefs, f.ref(t, 9001)) {
		t.Fatal("original create receipt was reset")
	}
	// Refreshing the original follows its saved latest action and carries its
	// accepted new row instead of recreating the assignment or the addition.
	f.Members = []int64{9003}
	next, err := f.Service.PrepareAudience(context.Background(), "TEST", path, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(next.Owned) != 2 {
		t.Fatalf("latest ownership lost %+v", next.Owned)
	}
	report, err = f.Service.PushAssignmentDates(context.Background(), f.actionPath(next))
	if err != nil || report.Status != "completed" || len(f.Writes) != 3 || !strings.HasPrefix(f.Writes[2], "DELETE ") {
		t.Fatalf("empty owned row not removed exactly %+v %v %+v", report, err, f.Writes)
	}
}

func TestAudienceRefreshPreservesTeacherChangesAndUncertainAddition(t *testing.T) {
	for _, mode := range []string{"teacher-before-prepare", "teacher-after-prepare", "uncertain-addition", "source-changed-noop"} {
		t.Run(mode, func(t *testing.T) {
			f := newRefreshFixture(t)
			path := f.initial(t)
			if mode == "source-changed-noop" {
				state, err := f.Service.PrepareAudience(context.Background(), "TEST", path, true)
				if err != nil {
					t.Fatal(err)
				}
				f.Members = []int64{9002}
				report, err := f.Service.PushAssignmentDates(context.Background(), f.actionPath(state))
				if err != nil || report.Status != "partial" || len(f.Writes) != 0 {
					t.Fatalf("stale no-op succeeded %+v %v", report, err)
				}
				return
			}
			f.Members = []int64{9001, 9002, 9003}
			if mode == "teacher-before-prepare" {
				f.Rows[0]["student_ids"] = []any{float64(9001)}
			}
			state, err := f.Service.PrepareAudience(context.Background(), "TEST", path, true)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "teacher-before-prepare" {
				for _, ref := range state.Refresh.Add {
					if ref == f.ref(t, 9002) {
						t.Fatal("refresh would undo the teacher's removal")
					}
				}
			}
			if mode == "teacher-after-prepare" {
				f.Rows = append(f.Rows, map[string]any{"id": float64(70), "title": "New teacher exception", "student_ids": []any{float64(9005)}})
			}
			f.FailCreate = mode == "uncertain-addition"
			report, err := f.Service.PushAssignmentDates(context.Background(), f.actionPath(state))
			if err != nil {
				t.Fatal(err)
			}
			if mode == "teacher-before-prepare" {
				if report.Status != "partial" || len(f.Writes) != 1 {
					t.Fatalf("independent addition did not proceed %+v %+v", report, f.Writes)
				}
				ids := f.Payloads[0]["student_ids"].([]any)
				if len(ids) != 1 || ids[0].(float64) != 9003 {
					t.Fatalf("teacher removed student was regranted %+v", ids)
				}
			}
			if mode == "teacher-after-prepare" && len(f.Writes) != 0 {
				t.Fatal("changed manual audience overwritten")
			}
			before := len(f.Writes)
			_, err = f.Service.PushAssignmentDates(context.Background(), f.actionPath(state))
			if err != nil || len(f.Writes) != before {
				t.Fatalf("accepted/uncertain addition replayed %v %+v", err, f.Writes)
			}
			if mode == "uncertain-addition" {
				if _, err = f.Service.PrepareAudience(context.Background(), "TEST", path, true); err == nil {
					t.Fatal("uncertain addition ownership erased")
				}
			}
		})
	}
}

func TestAudienceRefreshNewProtectionStillAllowsIndependentRemoval(t *testing.T) {
	f := newRefreshFixture(t)
	path := f.initial(t)
	f.Members = []int64{9003}
	state, err := f.Service.PrepareAudience(context.Background(), "TEST", path, true)
	if err != nil {
		t.Fatal(err)
	}
	f.Evidence[9001] = map[string]any{"user_id": 9001, "assignment_id": 10, "workflow_state": "submitted", "submitted_at": "2026-10-19T12:00:00Z"}
	report, err := f.Service.PushAssignmentDates(context.Background(), f.actionPath(state))
	if err != nil || report.Status != "partial" || len(f.Writes) != 2 || !strings.HasPrefix(f.Writes[0], "PUT ") {
		t.Fatalf("safe removal blocked %+v %v %+v", report, err, f.Writes)
	}
	var saved domain.AssignmentDatesDocument
	_, _ = workspace.ReadDocument(report.Path, &saved)
	if !reflectStrings(saved.Audience.State.AppliedRefs, []string{f.ref(t, 9001), f.ref(t, 9003)}) {
		t.Fatalf("applied refs hid protected member %+v", saved.Audience.State.AppliedRefs)
	}
	change := saved.Audience.State.Refresh.Changes[0]
	if change.Operation != "remove" || change.AppliedOperation != "update" || !reflectStrings(change.AppliedRemoveRefs, []string{f.ref(t, 9002)}) {
		t.Fatalf("actual scope not saved %+v", change)
	}
	before := len(f.Writes)
	_, err = f.Service.PushAssignmentDates(context.Background(), report.Path)
	if err != nil || len(f.Writes) != before {
		t.Fatalf("narrowed accepted row replayed %v %+v", err, f.Writes)
	}
}

func TestAudienceRefreshExactOwnershipAndNoopBaselines(t *testing.T) {
	for _, mode := range []string{"linked-target", "plan-manual-row", "noop-owned-change"} {
		t.Run(mode, func(t *testing.T) {
			f := newRefreshFixture(t)
			path := f.initial(t)
			if mode == "plan-manual-row" {
				f.Members = []int64{9002}
				f.Rows = append(f.Rows, map[string]any{"id": float64(70), "title": "Manual", "student_ids": []any{float64(9005)}})
			}
			state, err := f.Service.PrepareAudience(context.Background(), "TEST", path, true)
			if err != nil {
				t.Fatal(err)
			}
			action := f.actionPath(state)
			var d domain.AssignmentDatesDocument
			_, _ = workspace.ReadDocument(action, &d)
			switch mode {
			case "linked-target":
				d.AssignmentID = 20
				d.Audience.State.TargetID = 20
				_ = workspace.WriteDocument(action, d, "")
				if _, err = f.Service.PrepareAudience(context.Background(), "TEST", path, true); err == nil {
					t.Fatal("linked same-course target change accepted")
				}
			case "plan-manual-row":
				d.Audience.State.Refresh.Changes[0].OverrideID = 70
				_ = workspace.WriteDocument(action, d, "")
				if _, err = f.Service.PushAssignmentDates(context.Background(), action); err == nil {
					t.Fatal("unowned row change accepted")
				}
			case "noop-owned-change":
				f.Rows[0]["student_ids"] = []any{float64(9001)}
				report, e := f.Service.PushAssignmentDates(context.Background(), action)
				if e != nil || report.Status != "partial" {
					t.Fatalf("no-op claimed a changed audience %+v %v", report, e)
				}
			}
			if len(f.Writes) != 0 {
				t.Fatalf("identity or stale baseline wrote %+v", f.Writes)
			}
		})
	}
}

func TestAudienceRefreshExactProtectedRemovalAuthorizationCannotForceUnknown(t *testing.T) {
	for _, unreadable := range []bool{false, true} {
		t.Run(fmt.Sprint(unreadable), func(t *testing.T) {
			f := newRefreshFixture(t)
			path := f.initial(t)
			ref := f.ref(t, 9001)
			var source domain.AssignmentDatesDocument
			_, _ = workspace.ReadDocument(path, &source)
			source.Audience.AuthorizedRemovalRefs = []string{ref}
			_ = workspace.WriteDocument(path, source, "")
			f.Members = []int64{9002}
			f.ReadFail[9001] = unreadable
			f.Evidence[9001] = map[string]any{"user_id": 9001, "assignment_id": 10, "workflow_state": "graded", "score": 0}
			state, err := f.Service.PrepareAudience(context.Background(), "TEST", path, true)
			if err != nil {
				t.Fatal(err)
			}
			report, err := f.Service.PushAssignmentDates(context.Background(), f.actionPath(state))
			if err != nil {
				t.Fatal(err)
			}
			if unreadable {
				if report.Status != "partial" || len(f.Writes) != 0 {
					t.Fatalf("unknown evidence was forced %+v %+v", report, f.Writes)
				}
			} else if report.Status != "completed" || len(f.Writes) != 1 {
				t.Fatalf("exact authorized removal held %+v %+v", report, f.Writes)
			}
		})
	}
}
