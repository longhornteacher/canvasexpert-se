package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

func TestPaperAssignmentCreatePullRoundTripAndGroupMove(t *testing.T) {
	groupA, groupB := int64(7), int64(8)
	live := Assignment{}
	creates, updates := 0, 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/assignment_groups/"):
			id := groupA
			if strings.HasSuffix(r.URL.Path, "/8") {
				id = groupB
			}
			_ = json.NewEncoder(w).Encode(AssignmentGroup{ID: id, Name: "Daily"})
		case r.Method == "POST" && r.URL.Path == "/api/v1/courses/1/assignments":
			creates++
			var payload struct {
				Assignment Assignment `json:"assignment"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			live = payload.Assignment
			live.ID, live.CourseID = 10, 1
			if live.Published || live.AssignmentGroupID == nil || *live.AssignmentGroupID != groupA {
				t.Errorf("wrong create target/publication: %+v", live)
			}
			_ = json.NewEncoder(w).Encode(live)
		case r.Method == "PUT":
			updates++
			var payload struct {
				Assignment map[string]json.RawMessage `json:"assignment"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			if len(payload.Assignment) != 1 {
				t.Errorf("unexpected update: %+v", payload.Assignment)
			}
			var id int64
			if err := json.Unmarshal(payload.Assignment["assignment_group_id"], &id); err != nil {
				t.Fatal(err)
			}
			live.AssignmentGroupID = &id
			_ = json.NewEncoder(w).Encode(live)
		case r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/assignments/10":
			_ = json.NewEncoder(w).Encode(live)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.String())
			http.Error(w, "unexpected request", http.StatusBadRequest)
		}
	})
	doc := domain.AssignmentDocument{Schema: 1, Kind: "assignment", Operation: "create", Course: "TEST", Title: "Paper practice", Points: domain.MustDecimal("100"), SubmissionTypes: []string{"on_paper"}, AssignmentGroupID: &groupA}
	path := filepath.Join(s.Root, "drafts", "paper.md")
	if err := workspace.WriteDocument(path, doc, "Practice on paper."); err != nil {
		t.Fatal(err)
	}
	report, err := s.Push(context.Background(), path)
	if err != nil || report.Status != "completed" || creates != 1 {
		t.Fatalf("create: %+v %v", report, err)
	}
	pulled, err := s.PullItem(context.Background(), "TEST", "10")
	if err != nil {
		t.Fatal(err)
	}
	var edit domain.AssignmentDocument
	body, err := workspace.ReadDocument(pulled, &edit)
	if err != nil {
		t.Fatal(err)
	}
	if edit.AssignmentGroupID == nil || *edit.AssignmentGroupID != groupA || edit.Observed.AssignmentGroupID == nil || *edit.Observed.AssignmentGroupID != groupA || edit.SubmissionTypes[0] != "on_paper" {
		t.Fatalf("pull lost paper/group: %+v", edit)
	}
	patch, err := PlanEdit(edit, body)
	if err != nil || len(patch) != 0 {
		t.Fatalf("round trip diff: %+v %v", patch, err)
	}
	edit.AssignmentGroupID = &groupB
	if err := workspace.WriteDocument(pulled, edit, body); err != nil {
		t.Fatal(err)
	}
	review, err := s.BuildEditReview(context.Background(), pulled)
	if err != nil || len(review.Cards) != 2 || !strings.Contains(review.Cards[1].Facts[2].Value, "course grades") {
		t.Fatalf("review: %+v %v", review, err)
	}
	report, err = s.Push(context.Background(), pulled)
	if err != nil || report.Status != "succeeded" || updates != 1 || *live.AssignmentGroupID != groupB {
		t.Fatalf("move: %+v updates=%d err=%v", report, updates, err)
	}
	// Assignment group affects course weighting, not the assessment arithmetic.
	fresh, _ := Snapshot(live, s.Client.Origin())
	if !SameGradingBasis(*edit.Observed, fresh) {
		t.Fatal("assignment group change invalidated scoring basis")
	}
}

func TestAssignmentGroupTargetConflictAndRecoveryComparisons(t *testing.T) {
	a, b, c := int64(7), int64(8), int64(9)
	live := fixtureAssignment()
	live.AssignmentGroupID = &a
	snapshot, err := Snapshot(live, "https://canvas.example.edu")
	if err != nil {
		t.Fatal(err)
	}
	doc := domain.AssignmentDocument{Schema: 1, Kind: "assignment", Operation: "update", Course: "TEST", Instance: snapshot.Instance, CourseID: 1, AssignmentID: live.ID, Title: live.Name, Points: snapshot.Points, SubmissionTypes: live.SubmissionTypes, AssignmentGroupID: &b, Rubric: snapshot.Rubric, Observed: &snapshot}
	patch, err := PlanEdit(doc, live.Description)
	if err != nil || len(patch) != 1 || patch["assignment_group_id"] != b {
		t.Fatalf("patch: %+v %v", patch, err)
	}
	changed := snapshot
	changed.AssignmentGroupID = &c
	if sameEditFields(snapshot, changed, patch) {
		t.Fatal("concurrent assignment group edit ignored")
	}
	live.AssignmentGroupID = &b
	if !editPatchMatches(live, doc, live.Description, patch) {
		t.Fatal("requested group readback refused")
	}
	live.AssignmentGroupID = &c
	if editPatchMatches(live, doc, live.Description, patch) {
		t.Fatal("wrong group readback accepted")
	}
	doc.Operation = "create"
	doc.Observed, doc.Rubric = nil, nil
	live.Rubric = nil
	if sameCreateCore(live, doc, live.Description) {
		t.Fatal("recovery matched another assignment group")
	}
	doc.AssignmentGroupID = nil
	if _, exists := createFields(doc, live.Description)["assignment_group_id"]; exists {
		t.Fatal("omission must retain Canvas default")
	}
}

func TestNewQuizGroupAndDatesPushAndConflict(t *testing.T) {
	for _, conflict := range []bool{false, true} {
		t.Run(fmt.Sprintf("conflict=%t", conflict), func(t *testing.T) {
			groupA, groupB := int64(7), int64(8)
			live := Assignment{ID: 10, CourseID: 1, Name: "Quiz", Description: "<p>Quiz</p>", Points: json.Number("20"), SubmissionTypes: []string{"external_tool"}, NewQuiz: true, AssignmentGroupID: &groupA}
			writes := 0
			s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/assignment_groups/8":
					_ = json.NewEncoder(w).Encode(AssignmentGroup{ID: 8, Name: "Major"})
				case r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/assignments/10":
					_ = json.NewEncoder(w).Encode(live)
				case r.Method == "PUT" && r.URL.Path == "/api/v1/courses/1/assignments/10":
					writes++
					var payload struct {
						Assignment map[string]json.RawMessage `json:"assignment"`
					}
					_ = json.NewDecoder(r.Body).Decode(&payload)
					if len(payload.Assignment) != 4 {
						t.Errorf("unexpected patch: %v", payload.Assignment)
					}
					_ = json.Unmarshal(payload.Assignment["assignment_group_id"], &live.AssignmentGroupID)
					_ = json.Unmarshal(payload.Assignment["due_at"], &live.DueAt)
					_ = json.Unmarshal(payload.Assignment["unlock_at"], &live.UnlockAt)
					_ = json.Unmarshal(payload.Assignment["lock_at"], &live.LockAt)
					_ = json.NewEncoder(w).Encode(live)
				default:
					t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
					http.NotFound(w, r)
				}
			})
			path, err := s.PullItem(context.Background(), "TEST", "10")
			if err != nil {
				t.Fatal(err)
			}
			var d domain.AssignmentDocument
			body, err := workspace.ReadDocument(path, &d)
			if err != nil {
				t.Fatal(err)
			}
			due, unlock, lock := "2026-11-20T22:00:00Z", "2026-11-20T14:00:00Z", "2026-11-20T22:00:00Z"
			d.AssignmentGroupID, d.DueAt, d.UnlockAt, d.LockAt = &groupB, &due, &unlock, &lock
			if err = workspace.WriteDocument(path, d, body); err != nil {
				t.Fatal(err)
			}
			if conflict {
				other := "2026-11-21T22:00:00Z"
				live.DueAt = &other
			}
			report, err := s.Push(context.Background(), path)
			if err != nil {
				t.Fatal(err)
			}
			if conflict {
				if report.Status != "held" || writes != 0 {
					t.Fatalf("conflict report=%+v writes=%d", report, writes)
				}
				var saved domain.AssignmentDocument
				if _, err := workspace.ReadDocument(path, &saved); err != nil || *saved.DueAt != due {
					t.Fatal("unsent edit lost")
				}
				return
			}
			if report.Status != "succeeded" || writes != 1 {
				t.Fatalf("report=%+v writes=%d", report, writes)
			}
		})
	}
}

func TestPaperSubmissionValidationAndWrongCourseGroup(t *testing.T) {
	doc := domain.AssignmentDocument{Schema: 1, Kind: "assignment", Operation: "create", Course: "TEST", Title: "Paper practice", Points: domain.MustDecimal("100"), SubmissionTypes: []string{"online_upload", "on_paper"}}
	if err := domain.ValidateAssignment(doc, "Practice."); err == nil || !strings.Contains(err.Error(), "on_paper must be the only") {
		t.Fatalf("mixed types: %v", err)
	}
	group := int64(7)
	doc.SubmissionTypes = []string{"on_paper"}
	doc.AssignmentGroupID = &group
	writes := 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			writes++
		}
		http.Error(w, "not in target course", http.StatusNotFound)
	})
	path := filepath.Join(s.Root, "draft.md")
	if err := workspace.WriteDocument(path, doc, "Practice."); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Push(context.Background(), path); err == nil || !strings.Contains(err.Error(), "target course 1") || writes != 0 {
		t.Fatalf("wrong-course target: writes=%d err=%v", writes, err)
	}
}
