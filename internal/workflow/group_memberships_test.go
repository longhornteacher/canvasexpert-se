package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

func TestReadGroupRosterUsesStableAliasesAndNoCanvasUserData(t *testing.T) {
	members := map[int64][]canvasMembership{60: {{ID: 1, GroupID: 60, UserID: 10, WorkflowState: "accepted"}}, 61: {}}
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		write := func(v any) { _ = json.NewEncoder(w).Encode(v) }
		switch r.URL.Path {
		case "/api/v1/group_categories/50":
			write(map[string]any{"id": 50, "course_id": 1, "context_type": "Course", "name": "Clubs", "non_collaborative": false})
		case "/api/v1/group_categories/50/groups":
			write([]any{map[string]any{"id": 60, "course_id": 1, "context_type": "Course", "group_category_id": 50, "name": "A"}, map[string]any{"id": 61, "course_id": 1, "context_type": "Course", "group_category_id": 50, "name": "B"}})
		case "/api/v1/groups/60/memberships":
			write(members[60])
		case "/api/v1/groups/61/memberships":
			write(members[61])
		case "/api/v1/courses/1/enrollments":
			write([]any{map[string]any{"user_id": 10, "type": "StudentEnrollment", "enrollment_state": "active", "user": map[string]any{"id": 10, "name": "Private Student", "short_name": "Student"}}, map[string]any{"user_id": 11, "type": "StudentEnrollment", "enrollment_state": "active", "user": map[string]any{"id": 11, "name": "Private Student Two"}}})
		default:
			t.Errorf("unexpected request %s", r.URL.String())
			http.NotFound(w, r)
		}
	})
	store, _ := workspace.OpenStudents(s.Root)
	st1, _ := store.Upsert(s.Client.Origin(), 10, "Private Student", "Student", "SIS-PRIVATE", "login-private")
	_, _ = store.Upsert(s.Client.Origin(), 11, "Private Student Two", "", "", "")
	_ = store.Save()
	roster, err := s.ReadGroupRoster(context.Background(), "TEST", 50)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(roster)
	if !roster.RosterComplete || len(roster.Unassigned) != 1 || roster.Unassigned[0].Alias == "" || roster.Groups[0].Members[0].StudentRef != st1.Ref || strings.Contains(string(data), "Private Student") || strings.Contains(string(data), "login-private") || strings.Contains(string(data), "\"user_id\"") {
		t.Fatalf("unexpected roster: %+v, json=%s", roster, data)
	}
}

func TestPushMembershipMoveAndRemovalHoldSubmittedWork(t *testing.T) {
	current := map[int64][]canvasMembership{60: {{ID: 1, GroupID: 60, UserID: 10, WorkflowState: "accepted"}}, 61: {}}
	posts, deletes := 0, 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		write := func(v any) { _ = json.NewEncoder(w).Encode(v) }
		switch r.URL.Path {
		case "/api/v1/group_categories/50":
			write(map[string]any{"id": 50, "course_id": 1, "context_type": "Course", "non_collaborative": false})
		case "/api/v1/group_categories/50/groups":
			write([]any{map[string]any{"id": 60, "course_id": 1, "context_type": "Course", "group_category_id": 50}, map[string]any{"id": 61, "course_id": 1, "context_type": "Course", "group_category_id": 50}})
		case "/api/v1/groups/60/memberships":
			write(current[60])
		case "/api/v1/groups/61/memberships":
			if r.Method == http.MethodGet {
				write(current[61])
			} else {
				posts++
				write(map[string]any{"id": 3, "group_id": 61, "user_id": 10, "workflow_state": "accepted"})
			}
		case "/api/v1/courses/1/enrollments":
			write([]any{map[string]any{"user_id": 10, "type": "StudentEnrollment", "enrollment_state": "active", "user": map[string]any{"id": 10, "name": "Student"}}})
		case "/api/v1/courses/1/assignments":
			write([]any{map[string]any{"id": 90, "course_id": 1, "group_category_id": 50}})
		case "/api/v1/courses/1/assignments/90/submissions":
			write([]any{map[string]any{"user_id": 10, "submitted_at": "2026-10-01T12:00:00Z"}})
		case "/api/v1/groups/60", "/api/v1/groups/61":
			id := int64(60)
			if strings.HasSuffix(r.URL.Path, "61") {
				id = 61
			}
			write(map[string]any{"id": id, "course_id": 1, "context_type": "Course", "group_category_id": 50, "non_collaborative": false})
		case "/api/v1/groups/60/users/10":
			deletes++
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request %s", r.URL.String())
			http.NotFound(w, r)
		}
	})
	store, _ := workspace.OpenStudents(s.Root)
	st, _ := store.Upsert(s.Client.Origin(), 10, "Student", "", "", "")
	_ = store.Save()
	doc := domain.GroupMembershipDocument{Schema: 1, Kind: "group-memberships", Operation: "update", Course: "TEST", Instance: s.Client.Origin(), CourseID: 1, GroupCategoryID: 50, Observed: &domain.GroupMembershipSnapshot{Instance: s.Client.Origin(), CourseID: 1, GroupCategoryID: 50, GroupIDs: []int64{60, 61}, RosterComplete: true, Memberships: []domain.GroupMembershipObserved{{StudentRef: st.Ref, GroupID: 60, State: "accepted"}}}, Changes: []domain.GroupMembershipChange{{StudentRef: st.Ref, FromGroup: domain.NullableGroupID{Set: true, Value: 60}, ToGroup: domain.NullableGroupID{Set: true, Value: 61}}}}
	path := writeMembershipDoc(t, s, doc)
	report, err := s.PushGroupMemberships(context.Background(), path)
	if err != nil || report.Status != "partial" || report.Outcomes[0].Outcome.Status != "held" || posts != 0 || deletes != 0 {
		t.Fatalf("submitted-work move was not held: %+v err=%v post=%d delete=%d", report, err, posts, deletes)
	}
	doc.Changes[0].ToGroup = domain.NullableGroupID{Set: true, Null: true}
	path = writeMembershipDoc(t, s, doc)
	report, err = s.PushGroupMemberships(context.Background(), path)
	if err != nil || report.Status != "partial" || report.Outcomes[0].Outcome.Status != "held" || posts != 0 || deletes != 0 {
		t.Fatalf("submitted-work removal was not held: %+v err=%v post=%d delete=%d", report, err, posts, deletes)
	}
}

func TestUncertainSynchronousAllocationIsReconciledWithoutReplay(t *testing.T) {
	current := map[int64][]canvasMembership{60: {{ID: 1, GroupID: 60, UserID: 10, WorkflowState: "accepted"}}, 61: {}}
	allocations := 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		write := func(v any) { _ = json.NewEncoder(w).Encode(v) }
		switch r.URL.Path {
		case "/api/v1/group_categories/50":
			write(map[string]any{"id": 50, "course_id": 1, "context_type": "Course", "non_collaborative": false})
		case "/api/v1/group_categories/50/groups":
			write([]any{map[string]any{"id": 60, "course_id": 1, "context_type": "Course", "group_category_id": 50}, map[string]any{"id": 61, "course_id": 1, "context_type": "Course", "group_category_id": 50}})
		case "/api/v1/groups/60", "/api/v1/groups/61":
			id := int64(60)
			if strings.HasSuffix(r.URL.Path, "61") {
				id = 61
			}
			write(map[string]any{"id": id, "course_id": 1, "context_type": "Course", "group_category_id": 50, "non_collaborative": false})
		case "/api/v1/groups/60/memberships":
			write(current[60])
		case "/api/v1/groups/61/memberships":
			write(current[61])
		case "/api/v1/courses/1/enrollments":
			write([]any{map[string]any{"user_id": 10, "type": "StudentEnrollment", "enrollment_state": "active", "user": map[string]any{"id": 10, "name": "Student 10"}}, map[string]any{"user_id": 11, "type": "StudentEnrollment", "enrollment_state": "active", "user": map[string]any{"id": 11, "name": "Student 11"}}})
		case "/api/v1/group_categories/50/assign_unassigned_members":
			allocations++
			if r.URL.Query().Get("sync") != "true" {
				t.Errorf("missing sync=true: %s", r.URL.RawQuery)
			}
			current[61] = append(current[61], canvasMembership{ID: 2, GroupID: 61, UserID: 11, WorkflowState: "accepted"})
			http.Error(w, "simulated timeout after allocation", http.StatusServiceUnavailable)
		default:
			t.Errorf("unexpected request %s", r.URL.String())
			http.NotFound(w, r)
		}
	})
	store, _ := workspace.OpenStudents(s.Root)
	st10, _ := store.Upsert(s.Client.Origin(), 10, "Student 10", "", "", "")
	st11, _ := store.Upsert(s.Client.Origin(), 11, "Student 11", "", "", "")
	_ = store.Save()
	doc := domain.GroupMembershipDocument{Schema: 1, Kind: "group-memberships", Operation: "allocate-unassigned", Course: "TEST", Instance: s.Client.Origin(), CourseID: 1, GroupCategoryID: 50, Observed: &domain.GroupMembershipSnapshot{Instance: s.Client.Origin(), CourseID: 1, GroupCategoryID: 50, GroupIDs: []int64{60, 61}, RosterComplete: true, Memberships: []domain.GroupMembershipObserved{{StudentRef: st10.Ref, GroupID: 60, State: "accepted"}}, UnassignedRefs: []string{st11.Ref}}, Allocation: &domain.GroupAllocationOutcome{}}
	path := writeMembershipDoc(t, s, doc)
	first, err := s.PushGroupMemberships(context.Background(), path)
	if err != nil || first.Status != "uncertain" || allocations != 1 {
		t.Fatalf("first allocation %+v err=%v calls=%d", first, err, allocations)
	}
	second, err := s.PushGroupMemberships(context.Background(), path)
	if err != nil || second.Status != "completed" || len(second.Assigned) != 1 || second.Assigned[0] != st11.Ref || allocations != 1 {
		t.Fatalf("uncertain allocation not reconciled: %+v err=%v calls=%d", second, err, allocations)
	}
	third, err := s.PushGroupMemberships(context.Background(), path)
	if err != nil || third.Status != "completed" || len(third.Assigned) != 1 || allocations != 1 {
		t.Fatalf("completed allocation was replayed: %+v err=%v calls=%d", third, err, allocations)
	}
}

func TestSynchronousAllocationUsesReturnedMembershipsWithoutReadback(t *testing.T) {
	const userID = int64(98765)
	membershipReads := 0
	allocationCalls := 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		write := func(v any) { _ = json.NewEncoder(w).Encode(v) }
		switch r.URL.Path {
		case "/api/v1/group_categories/50":
			write(map[string]any{"id": 50, "course_id": 1, "context_type": "Course", "non_collaborative": false})
		case "/api/v1/group_categories/50/groups":
			write([]any{map[string]any{"id": 60, "course_id": 1, "context_type": "Course", "group_category_id": 50}, map[string]any{"id": 61, "course_id": 1, "context_type": "Course", "group_category_id": 50}})
		case "/api/v1/groups/60", "/api/v1/groups/61":
			id := int64(60)
			if strings.HasSuffix(r.URL.Path, "61") {
				id = 61
			}
			write(map[string]any{"id": id, "course_id": 1, "context_type": "Course", "group_category_id": 50, "non_collaborative": false})
		case "/api/v1/groups/60/memberships", "/api/v1/groups/61/memberships":
			membershipReads++
			write([]any{})
		case "/api/v1/courses/1/enrollments":
			write([]any{map[string]any{"user_id": userID, "type": "StudentEnrollment", "enrollment_state": "active", "user": map[string]any{"id": userID, "name": "Private Student Name", "short_name": "Private Alias"}}})
		case "/api/v1/group_categories/50/assign_unassigned_members":
			allocationCalls++
			if r.URL.Query().Get("sync") != "true" {
				t.Errorf("missing sync=true: %s", r.URL.RawQuery)
			}
			write([]any{map[string]any{"id": 61, "new_members": []any{map[string]any{"user_id": userID, "name": "PRIVATE CANVAS NAME", "display_name": "PRIVATE DISPLAY", "sections": []any{map[string]any{"section_id": 4, "section_code": "PRIVATE SECTION"}}}}}})
		default:
			t.Errorf("unexpected request %s", r.URL.String())
			http.NotFound(w, r)
		}
	})
	store, _ := workspace.OpenStudents(s.Root)
	student, _ := store.Upsert(s.Client.Origin(), userID, "Private Student Name", "Private Alias", "PRIVATE SIS", "private-login")
	_ = store.Save()
	doc := domain.GroupMembershipDocument{Schema: 1, Kind: "group-memberships", Operation: "allocate-unassigned", Course: "TEST", Instance: s.Client.Origin(), CourseID: 1, GroupCategoryID: 50, Observed: &domain.GroupMembershipSnapshot{Instance: s.Client.Origin(), CourseID: 1, GroupCategoryID: 50, GroupIDs: []int64{60, 61}, RosterComplete: true, UnassignedRefs: []string{student.Ref}}, Allocation: &domain.GroupAllocationOutcome{}}
	path := writeMembershipDoc(t, s, doc)
	report, err := s.PushGroupMemberships(context.Background(), path)
	if err != nil || report.Status != "completed" || len(report.Assigned) != 1 || report.Assigned[0] != student.Ref || allocationCalls != 1 || membershipReads != 4 {
		t.Fatalf("successful allocation did not use response without readback: report=%+v err=%v allocations=%d membership reads=%d", report, err, allocationCalls, membershipReads)
	}
	var saved domain.GroupMembershipDocument
	if _, err = workspace.ReadDocument(path, &saved); err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(saved)
	for _, secret := range []string{"PRIVATE CANVAS NAME", "PRIVATE DISPLAY", "PRIVATE SECTION", "private-login", "PRIVATE SIS", fmt.Sprint(userID), "user_id"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("allocation response exposed %q: %s", secret, encoded)
		}
	}
}

func TestMembershipSourceConflictHoldsOnlyThatStudent(t *testing.T) {
	current := map[int64][]canvasMembership{60: {}, 61: {}, 62: {{ID: 1, GroupID: 62, UserID: 10, WorkflowState: "accepted"}}}
	posts := 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		write := func(v any) { _ = json.NewEncoder(w).Encode(v) }
		switch r.URL.Path {
		case "/api/v1/group_categories/50":
			write(map[string]any{"id": 50, "course_id": 1, "context_type": "Course", "non_collaborative": false})
		case "/api/v1/group_categories/50/groups":
			write([]any{map[string]any{"id": 60, "course_id": 1, "context_type": "Course", "group_category_id": 50}, map[string]any{"id": 61, "course_id": 1, "context_type": "Course", "group_category_id": 50}, map[string]any{"id": 62, "course_id": 1, "context_type": "Course", "group_category_id": 50}})
		case "/api/v1/groups/60", "/api/v1/groups/61", "/api/v1/groups/62":
			var id int64
			_, _ = fmt.Sscanf(r.URL.Path, "/api/v1/groups/%d", &id)
			write(map[string]any{"id": id, "course_id": 1, "context_type": "Course", "group_category_id": 50, "non_collaborative": false})
		case "/api/v1/groups/60/memberships":
			if r.Method == http.MethodGet {
				write(current[60])
				return
			}
			posts++
			var body struct {
				UserID int64 `json:"user_id"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			current[60] = append(current[60], canvasMembership{ID: 50, GroupID: 60, UserID: body.UserID, WorkflowState: "accepted"})
			write(canvasMembership{ID: 50, GroupID: 60, UserID: body.UserID, WorkflowState: "accepted"})
		case "/api/v1/groups/61/memberships":
			write(current[61])
		case "/api/v1/groups/62/memberships":
			write(current[62])
		case "/api/v1/courses/1/enrollments":
			write([]any{map[string]any{"user_id": 10, "type": "StudentEnrollment", "enrollment_state": "active", "user": map[string]any{"id": 10, "name": "Student 10"}}, map[string]any{"user_id": 11, "type": "StudentEnrollment", "enrollment_state": "active", "user": map[string]any{"id": 11, "name": "Student 11"}}})
		case "/api/v1/courses/1/assignments":
			write([]any{})
		default:
			t.Errorf("unexpected request %s", r.URL.String())
			http.NotFound(w, r)
		}
	})
	store, _ := workspace.OpenStudents(s.Root)
	st10, _ := store.Upsert(s.Client.Origin(), 10, "Student 10", "", "", "")
	st11, _ := store.Upsert(s.Client.Origin(), 11, "Student 11", "", "", "")
	_ = store.Save()
	doc := domain.GroupMembershipDocument{Schema: 1, Kind: "group-memberships", Operation: "update", Course: "TEST", Instance: s.Client.Origin(), CourseID: 1, GroupCategoryID: 50, Observed: &domain.GroupMembershipSnapshot{Instance: s.Client.Origin(), CourseID: 1, GroupCategoryID: 50, GroupIDs: []int64{60, 61, 62}, RosterComplete: true, Memberships: []domain.GroupMembershipObserved{{StudentRef: st10.Ref, GroupID: 60, State: "accepted"}}, UnassignedRefs: []string{st11.Ref}}, Changes: []domain.GroupMembershipChange{{StudentRef: st10.Ref, FromGroup: domain.NullableGroupID{Set: true, Value: 60}, ToGroup: domain.NullableGroupID{Set: true, Value: 61}}, {StudentRef: st11.Ref, FromGroup: domain.NullableGroupID{Set: true, Null: true}, ToGroup: domain.NullableGroupID{Set: true, Value: 60}}}}
	path := writeMembershipDoc(t, s, doc)
	report, err := s.PushGroupMemberships(context.Background(), path)
	if err != nil || report.Status != "partial" || len(report.Outcomes) != 2 || report.Outcomes[0].Outcome.Status != "held" || report.Outcomes[1].Outcome.Status != "succeeded" || posts != 1 {
		t.Fatalf("per-student conflict handling failed: %+v err=%v posts=%d", report, err, posts)
	}
}

func TestPendingSignupAfterBaselineHoldsOnlyThatStudent(t *testing.T) {
	current := map[int64][]canvasMembership{
		60: {{ID: 1, GroupID: 60, UserID: 10, WorkflowState: "requested"}},
		61: {},
	}
	posts := 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		write := func(v any) { _ = json.NewEncoder(w).Encode(v) }
		switch r.URL.Path {
		case "/api/v1/group_categories/50":
			write(map[string]any{"id": 50, "course_id": 1, "context_type": "Course", "non_collaborative": false})
		case "/api/v1/group_categories/50/groups":
			write([]any{map[string]any{"id": 60, "course_id": 1, "context_type": "Course", "group_category_id": 50}, map[string]any{"id": 61, "course_id": 1, "context_type": "Course", "group_category_id": 50}})
		case "/api/v1/groups/60/memberships", "/api/v1/groups/61/memberships":
			var id int64 = 60
			if strings.Contains(r.URL.Path, "/61/") {
				id = 61
			}
			if r.Method == http.MethodGet {
				write(current[id])
				return
			}
			posts++
			var body struct {
				UserID int64 `json:"user_id"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			current[id] = append(current[id], canvasMembership{ID: 2, GroupID: id, UserID: body.UserID, WorkflowState: "accepted"})
			write(canvasMembership{ID: 2, GroupID: id, UserID: body.UserID, WorkflowState: "accepted"})
		case "/api/v1/groups/60", "/api/v1/groups/61":
			var id int64
			_, _ = fmt.Sscanf(r.URL.Path, "/api/v1/groups/%d", &id)
			write(map[string]any{"id": id, "course_id": 1, "context_type": "Course", "group_category_id": 50, "non_collaborative": false})
		case "/api/v1/courses/1/enrollments":
			write([]any{map[string]any{"user_id": 10, "type": "StudentEnrollment", "enrollment_state": "active", "user": map[string]any{"id": 10, "name": "Private Student 10"}}, map[string]any{"user_id": 11, "type": "StudentEnrollment", "enrollment_state": "active", "user": map[string]any{"id": 11, "name": "Private Student 11"}}})
		case "/api/v1/courses/1/assignments":
			write([]any{})
		default:
			t.Errorf("unexpected request %s", r.URL.String())
			http.NotFound(w, r)
		}
	})
	store, _ := workspace.OpenStudents(s.Root)
	st10, _ := store.Upsert(s.Client.Origin(), 10, "Private Student 10", "", "", "")
	st11, _ := store.Upsert(s.Client.Origin(), 11, "Private Student 11", "", "", "")
	_ = store.Save()
	roster, err := s.ReadGroupRoster(context.Background(), "TEST", 50)
	if err != nil || len(roster.PendingRequests) != 1 || roster.PendingRequests[0] != (domain.GroupMembershipPending{StudentRef: st10.Ref, GroupID: 60, State: "requested"}) {
		t.Fatalf("pending self-signup not exposed with safe identity: roster=%+v err=%v", roster, err)
	}
	encodedRoster, _ := json.Marshal(roster)
	if strings.Contains(string(encodedRoster), "Private Student") || strings.Contains(string(encodedRoster), `"user_id"`) || strings.Contains(string(encodedRoster), `"10"`) {
		t.Fatalf("pending request exposed private Canvas identity: %s", encodedRoster)
	}
	pulled, err := s.PullGroupMemberships(context.Background(), "TEST", 50)
	if err != nil {
		t.Fatal(err)
	}
	var pulledDoc domain.GroupMembershipDocument
	if _, err = workspace.ReadDocument(pulled, &pulledDoc); err != nil || len(pulledDoc.Observed.PendingRequests) != 1 || pulledDoc.Observed.PendingRequests[0].StudentRef != st10.Ref || pulledDoc.Observed.PendingRequests[0].GroupID != 60 {
		t.Fatalf("pulled baseline omitted safe pending request identity: doc=%+v err=%v", pulledDoc.Observed, err)
	}
	doc := domain.GroupMembershipDocument{Schema: 1, Kind: "group-memberships", Operation: "update", Course: "TEST", Instance: s.Client.Origin(), CourseID: 1, GroupCategoryID: 50, Observed: &domain.GroupMembershipSnapshot{Instance: s.Client.Origin(), CourseID: 1, GroupCategoryID: 50, GroupIDs: []int64{60, 61}, RosterComplete: true, UnassignedRefs: []string{st10.Ref, st11.Ref}}, Changes: []domain.GroupMembershipChange{{StudentRef: st10.Ref, FromGroup: domain.NullableGroupID{Set: true, Null: true}, ToGroup: domain.NullableGroupID{Set: true, Value: 61}}, {StudentRef: st11.Ref, FromGroup: domain.NullableGroupID{Set: true, Null: true}, ToGroup: domain.NullableGroupID{Set: true, Value: 61}}}}
	path := writeMembershipDoc(t, s, doc)
	report, err := s.PushGroupMemberships(context.Background(), path)
	if err != nil || report.Status != "partial" || len(report.Outcomes) != 2 || report.Outcomes[0].Outcome.Status != "held" || report.Outcomes[1].Outcome.Status != "succeeded" || posts != 1 {
		t.Fatalf("pending request was not held independently: report=%+v err=%v posts=%d", report, err, posts)
	}
	current[60] = append(current[60], canvasMembership{ID: 3, GroupID: 60, UserID: 99, WorkflowState: "requested"})
	incomplete, err := s.ReadGroupRoster(context.Background(), "TEST", 50)
	if err != nil || incomplete.RosterComplete || incomplete.UnresolvedCount != 1 {
		t.Fatalf("unresolved pending author did not make roster incomplete: roster=%+v err=%v", incomplete, err)
	}
}

func TestAcceptedMembershipWriteIsNotReplayedAfterMembershipChanges(t *testing.T) {
	current := map[int64][]canvasMembership{60: {{ID: 1, GroupID: 60, UserID: 10, WorkflowState: "accepted"}}, 61: {}}
	posts := 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		write := func(v any) { _ = json.NewEncoder(w).Encode(v) }
		switch r.URL.Path {
		case "/api/v1/group_categories/50":
			write(map[string]any{"id": 50, "course_id": 1, "context_type": "Course", "non_collaborative": false})
		case "/api/v1/group_categories/50/groups":
			write([]any{map[string]any{"id": 60, "course_id": 1, "context_type": "Course", "group_category_id": 50}, map[string]any{"id": 61, "course_id": 1, "context_type": "Course", "group_category_id": 50}})
		case "/api/v1/groups/60/memberships", "/api/v1/groups/61/memberships":
			var id int64 = 60
			if strings.Contains(r.URL.Path, "/61/") {
				id = 61
			}
			if r.Method == http.MethodGet {
				write(current[id])
				return
			}
			posts++
			current[60] = nil
			current[61] = []canvasMembership{{ID: 2, GroupID: 61, UserID: 10, WorkflowState: "accepted"}}
			write(current[61][0])
		case "/api/v1/groups/60", "/api/v1/groups/61":
			var id int64
			_, _ = fmt.Sscanf(r.URL.Path, "/api/v1/groups/%d", &id)
			write(map[string]any{"id": id, "course_id": 1, "context_type": "Course", "group_category_id": 50, "non_collaborative": false})
		case "/api/v1/courses/1/enrollments":
			write([]any{map[string]any{"user_id": 10, "type": "StudentEnrollment", "enrollment_state": "active", "user": map[string]any{"id": 10, "name": "Student"}}})
		case "/api/v1/courses/1/assignments":
			write([]any{})
		default:
			t.Errorf("unexpected request %s", r.URL.String())
			http.NotFound(w, r)
		}
	})
	store, _ := workspace.OpenStudents(s.Root)
	student, _ := store.Upsert(s.Client.Origin(), 10, "Student", "", "", "")
	_ = store.Save()
	doc := domain.GroupMembershipDocument{Schema: 1, Kind: "group-memberships", Operation: "update", Course: "TEST", Instance: s.Client.Origin(), CourseID: 1, GroupCategoryID: 50, Observed: &domain.GroupMembershipSnapshot{Instance: s.Client.Origin(), CourseID: 1, GroupCategoryID: 50, GroupIDs: []int64{60, 61}, RosterComplete: true, Memberships: []domain.GroupMembershipObserved{{StudentRef: student.Ref, GroupID: 60, State: "accepted"}}}, Changes: []domain.GroupMembershipChange{{StudentRef: student.Ref, FromGroup: domain.NullableGroupID{Set: true, Value: 60}, ToGroup: domain.NullableGroupID{Set: true, Value: 61}}}}
	path := writeMembershipDoc(t, s, doc)
	first, err := s.PushGroupMemberships(context.Background(), path)
	if err != nil || first.Outcomes[0].Outcome.Status != "succeeded" || posts != 1 {
		t.Fatalf("membership write failed: report=%+v err=%v posts=%d", first, err, posts)
	}
	current[60] = []canvasMembership{{ID: 1, GroupID: 60, UserID: 10, WorkflowState: "accepted"}}
	current[61] = nil
	second, err := s.PushGroupMemberships(context.Background(), path)
	if err != nil || second.Outcomes[0].Outcome.Status != "succeeded" || posts != 1 {
		t.Fatalf("accepted write replayed after membership changed: report=%+v err=%v posts=%d", second, err, posts)
	}
}

func TestUncertainMembershipWriteReconcilesWithoutReplay(t *testing.T) {
	current := map[int64][]canvasMembership{60: {{ID: 1, GroupID: 60, UserID: 10, WorkflowState: "accepted"}}, 61: {}}
	posts := 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		write := func(v any) { _ = json.NewEncoder(w).Encode(v) }
		switch r.URL.Path {
		case "/api/v1/group_categories/50":
			write(map[string]any{"id": 50, "course_id": 1, "context_type": "Course", "non_collaborative": false})
		case "/api/v1/group_categories/50/groups":
			write([]any{map[string]any{"id": 60, "course_id": 1, "context_type": "Course", "group_category_id": 50}, map[string]any{"id": 61, "course_id": 1, "context_type": "Course", "group_category_id": 50}})
		case "/api/v1/groups/60/memberships", "/api/v1/groups/61/memberships":
			var id int64 = 60
			if strings.Contains(r.URL.Path, "/61/") {
				id = 61
			}
			if r.Method == http.MethodGet {
				write(current[id])
				return
			}
			posts++
			current[60] = nil
			current[61] = []canvasMembership{{ID: 2, GroupID: 61, UserID: 10, WorkflowState: "accepted"}}
			http.Error(w, "simulated response loss after accepted membership", http.StatusServiceUnavailable)
		case "/api/v1/groups/60", "/api/v1/groups/61":
			var id int64
			_, _ = fmt.Sscanf(r.URL.Path, "/api/v1/groups/%d", &id)
			write(map[string]any{"id": id, "course_id": 1, "context_type": "Course", "group_category_id": 50, "non_collaborative": false})
		case "/api/v1/courses/1/enrollments":
			write([]any{map[string]any{"user_id": 10, "type": "StudentEnrollment", "enrollment_state": "active", "user": map[string]any{"id": 10, "name": "Student"}}})
		case "/api/v1/courses/1/assignments":
			write([]any{})
		default:
			t.Errorf("unexpected request %s", r.URL.String())
			http.NotFound(w, r)
		}
	})
	store, _ := workspace.OpenStudents(s.Root)
	student, _ := store.Upsert(s.Client.Origin(), 10, "Student", "", "", "")
	_ = store.Save()
	doc := domain.GroupMembershipDocument{Schema: 1, Kind: "group-memberships", Operation: "update", Course: "TEST", Instance: s.Client.Origin(), CourseID: 1, GroupCategoryID: 50, Observed: &domain.GroupMembershipSnapshot{Instance: s.Client.Origin(), CourseID: 1, GroupCategoryID: 50, GroupIDs: []int64{60, 61}, RosterComplete: true, Memberships: []domain.GroupMembershipObserved{{StudentRef: student.Ref, GroupID: 60, State: "accepted"}}}, Changes: []domain.GroupMembershipChange{{StudentRef: student.Ref, FromGroup: domain.NullableGroupID{Set: true, Value: 60}, ToGroup: domain.NullableGroupID{Set: true, Value: 61}}}}
	path := writeMembershipDoc(t, s, doc)
	first, err := s.PushGroupMemberships(context.Background(), path)
	if err != nil || first.Outcomes[0].Outcome.Status != "uncertain" || posts != 1 {
		t.Fatalf("uncertain membership result not saved: report=%+v err=%v posts=%d", first, err, posts)
	}
	second, err := s.PushGroupMemberships(context.Background(), path)
	if err != nil || second.Outcomes[0].Outcome.Status != "preserved" || posts != 1 {
		t.Fatalf("uncertain membership was not reconciled without replay: report=%+v err=%v posts=%d", second, err, posts)
	}
}

func writeMembershipDoc(t *testing.T, s *Service, doc domain.GroupMembershipDocument) string {
	t.Helper()
	path := filepath.Join(s.Root, "courses", "TEST", "drafts", fmt.Sprintf("memberships-%d.md", time.Now().UnixNano()))
	if err := workspace.WriteDocument(path, doc, ""); err != nil {
		t.Fatal(err)
	}
	return path
}
