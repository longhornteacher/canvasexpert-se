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

func TestPullGroupAssignmentSelectedReadsOnlySelectedMembersAndKeepsUnsentResults(t *testing.T) {
	var broadReads, otherGroupReads, fileDownloads int
	groupID, setID := int64(50), int64(60)
	mode := true
	a := fixtureAssignment()
	a.GroupCategoryID = &groupID
	a.GradeGroupStudentsIndividually = &mode
	var serverURL string
	s, server := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		write := func(v any) { _ = json.NewEncoder(w).Encode(v) }
		switch r.URL.Path {
		case "/files/shared":
			fileDownloads++
			_, _ = w.Write([]byte("shared source document"))
		case "/api/v1/courses/1/assignments/10":
			write(a)
		case "/api/v1/group_categories/50":
			write(map[string]any{"id": 50, "course_id": 1, "context_type": "Course", "name": "Clubs", "non_collaborative": false})
		case "/api/v1/groups/60":
			write(map[string]any{"id": 60, "course_id": 1, "context_type": "Course", "group_category_id": 50, "name": "Blue", "non_collaborative": false})
		case "/api/v1/groups/60/memberships":
			write([]any{map[string]any{"group_id": 60, "user_id": 10, "workflow_state": "accepted"}, map[string]any{"group_id": 60, "user_id": 11, "workflow_state": "accepted"}, map[string]any{"group_id": 60, "user_id": 12, "workflow_state": "accepted"}, map[string]any{"group_id": 60, "user_id": 13, "workflow_state": "accepted"}, map[string]any{"group_id": 60, "user_id": 14, "workflow_state": "accepted"}})
		case "/api/v1/courses/1/enrollments":
			rows := []any{}
			for id := int64(10); id <= 14; id++ {
				rows = append(rows, map[string]any{"user_id": id, "type": "StudentEnrollment", "enrollment_state": "active", "user": map[string]any{"id": id, "name": fmt.Sprintf("Private Student %d", id), "short_name": fmt.Sprintf("Student %d", id)}})
			}
			write(rows)
		case "/api/v1/courses/1/assignments/10/submissions":
			broadReads++
			t.Errorf("selected-group pull used assignment-wide submissions route")
			http.Error(w, "wrong scope", 500)
		case "/api/v1/courses/1/assignments/10/submissions/14":
			// This current member has work in a different historical group, so
			// no submission record is kept for them in the selected group.
			write(map[string]any{"user_id": 14, "assignment_id": 10, "group_id": 61, "attempt": 1})
		case "/api/v1/courses/1/assignments/10/submissions/10", "/api/v1/courses/1/assignments/10/submissions/11", "/api/v1/courses/1/assignments/10/submissions/12", "/api/v1/courses/1/assignments/10/submissions/13":
			if !strings.Contains(r.URL.RawQuery, "include[]=group") || !strings.Contains(r.URL.RawQuery, "include[]=submission_history") {
				t.Errorf("scoped submission did not request required evidence: %s", r.URL.RawQuery)
			}
			userID := int64(10)
			if strings.HasSuffix(r.URL.Path, "/11") {
				userID = 11
			}
			if strings.HasSuffix(r.URL.Path, "/12") {
				userID = 12
			}
			if strings.HasSuffix(r.URL.Path, "/13") {
				userID = 13
			}
			if userID == 12 {
				write(map[string]any{"id": 702, "user_id": userID, "assignment_id": 10, "group_id": 60, "attempt": 0, "workflow_state": "unsubmitted"})
				break
			}
			if userID == 13 {
				write(map[string]any{"id": 713, "user_id": userID, "assignment_id": 10, "attempt": 1, "submitted_at": "2026-10-16T18:00:00Z", "workflow_state": "submitted", "submission_type": "online_text_entry", "body": "unknown group identity"})
				break
			}
			grade := 15
			if userID == 11 {
				grade = 11
			}
			write(map[string]any{"id": 700 + userID, "user_id": userID, "assignment_id": 10, "group_id": 60, "attempt": 1, "submitted_at": "2026-10-16T18:00:00Z", "workflow_state": "submitted", "submission_type": "online_upload", "body": "shared work", "entered_score": grade, "attachments": []any{map[string]any{"id": 800, "filename": "shared.txt", "content_type": "text/plain", "url": serverURL + "/files/shared"}}})
		case "/api/v1/courses/1/assignments/10/users/10/group_members", "/api/v1/courses/1/assignments/10/users/11/group_members", "/api/v1/courses/1/assignments/10/users/12/group_members", "/api/v1/courses/1/assignments/10/users/13/group_members", "/api/v1/courses/1/assignments/10/users/14/group_members":
			write([]any{map[string]any{"id": "10"}, map[string]any{"id": "11"}, map[string]any{"id": "12"}, map[string]any{"id": "13"}, map[string]any{"id": "14"}})
		default:
			if strings.Contains(r.URL.Path, "/submissions/") && strings.HasSuffix(r.URL.Path, "/submissions/11") {
				otherGroupReads++
			}
			t.Errorf("unexpected request %s", r.URL.String())
			http.NotFound(w, r)
		}
	})
	serverURL = server.URL
	store, _ := workspace.OpenStudents(s.Root)
	for id := int64(10); id <= 14; id++ {
		_, _ = store.Upsert(s.Client.Origin(), id, fmt.Sprintf("Private Student %d", id), fmt.Sprintf("Student %d", id), "", "")
	}
	extraTime := map[string]bool{}
	for i := range store.Rows {
		store.Rows[i].ExtraTime = store.Rows[i].UserID == 10 || store.Rows[i].UserID == 14
		extraTime[store.Rows[i].Ref] = store.Rows[i].ExtraTime
	}
	_ = store.Save()
	unsent := filepath.Join(s.Root, "courses", "TEST", "prepared-results.md")
	if err := workspace.WriteFile(unsent, []byte("unsent teacher work"), 0644); err != nil {
		t.Fatal(err)
	}
	report, err := s.PullWorkGroup(context.Background(), "TEST", "10", setID)
	if err != nil || len(report.Packets) != 1 || broadReads != 0 || otherGroupReads != 0 || fileDownloads != 1 {
		t.Fatalf("scoped pull report=%+v err=%v broad=%d other=%d", report, err, broadReads, otherGroupReads)
	}
	var packet domain.GroupPacketDocument
	if _, err = workspace.ReadDocument(report.Packets[0], &packet); err != nil {
		t.Fatal(err)
	}
	var work domain.GroupWorkDocument
	if _, err = workspace.ReadDocument(filepath.Join(s.Root, filepath.FromSlash(packet.Groups[0].Path)), &work); err != nil {
		t.Fatal(err)
	}
	placeholderFound := false
	for _, member := range work.Members {
		if member.WorkflowState == "no submitted attempt" {
			placeholderFound = !member.Missing
		}
	}
	unknownFound := false
	for _, member := range work.Members {
		if member.WorkflowState == "submission group identity unknown" && member.SubmissionGroupID == 0 {
			unknownFound = true
		}
	}
	if len(work.Members) != 5 || !placeholderFound || !unknownFound || len(work.Artifacts) != 1 || len(work.Artifacts[0].Sources) != 2 {
		t.Fatalf("member identities/artifact dedup/placeholder mismatch: members=%+v artifacts=%+v", work.Members, work.Artifacts)
	}
	noRecordFound := false
	for _, member := range work.Members {
		want, mapped := extraTime[member.StudentRef]
		if !mapped || member.ExtraTime == nil || *member.ExtraTime != want {
			t.Fatalf("member lost its saved Extra time fact: %+v", member)
		}
		if member.WorkflowState == "no submission record returned" {
			noRecordFound = true
		}
	}
	if !noRecordFound {
		t.Fatal("fixture must exercise a known member without a returned submission")
	}
	grades := map[string]string{}
	for _, member := range work.Members {
		if member.EnteredGrade != nil {
			grades[member.StudentRef] = member.EnteredGrade.String()
		}
	}
	gradeValues := map[string]bool{}
	for _, grade := range grades {
		gradeValues[grade] = true
	}
	if len(grades) != 2 || len(gradeValues) != 2 {
		t.Fatalf("member grades were collapsed: %+v", grades)
	}
	if body, err := os.ReadFile(unsent); err != nil || string(body) != "unsent teacher work" {
		t.Fatalf("unsent results changed: %q err=%v", body, err)
	}
}

func TestPullGroupAssignmentWrongGroupSetStopsBeforeSubmissionReads(t *testing.T) {
	var submissionReads int
	groupCategoryID := int64(50)
	a := fixtureAssignment()
	a.GroupCategoryID = &groupCategoryID
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		write := func(v any) { _ = json.NewEncoder(w).Encode(v) }
		switch r.URL.Path {
		case "/api/v1/courses/1/assignments/10":
			write(a)
		case "/api/v1/group_categories/50":
			write(map[string]any{"id": 50, "course_id": 1, "context_type": "Course", "non_collaborative": false})
		case "/api/v1/courses/1/enrollments":
			write([]any{})
		case "/api/v1/groups/60":
			write(map[string]any{"id": 60, "course_id": 1, "context_type": "Course", "group_category_id": 51, "non_collaborative": false})
		default:
			if strings.Contains(r.URL.Path, "/submissions") {
				submissionReads++
			}
			t.Errorf("unexpected request before scope rejection: %s", r.URL.String())
			http.NotFound(w, r)
		}
	})
	if _, err := s.PullWorkGroup(context.Background(), "TEST", "10", 60); err == nil || !strings.Contains(err.Error(), "outside this course assignment's collaborative set") || submissionReads != 0 {
		t.Fatalf("wrong-set group was not rejected before submissions: reads=%d err=%v", submissionReads, err)
	}
}

func TestCompletedHintRecognizesIndividualGroupResultsByStudentAttempt(t *testing.T) {
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
	sub := fixtureSubmission(5)
	snapshot, err := Snapshot(fixtureAssignment(), s.Client.Origin())
	if err != nil {
		t.Fatal(err)
	}
	completed := domain.StudentResult{StudentRef: "stu_test", Operation: "grade", Grade: &domain.GradeOperation{Attempt: attemptRef(sub), Outcomes: domain.ActionOutcomes{Grade: domain.ActionOutcome{Status: "succeeded"}, Comment: domain.ActionOutcome{Status: "not-requested"}, Rubric: domain.ActionOutcome{Status: "not-requested"}, Late: domain.ActionOutcome{Status: "not-requested"}}}}
	doc := map[string]any{
		"schema": 1, "kind": "group-results", "packet": "courses/TEST/pulled/group-packet.md",
		"assignment": snapshot, "interpretation": map[string]any{"mapping": "repeated", "rationale": "synthetic"},
		"groups": []any{map[string]any{
			"group_id": 70, "artifact_key": "artifact-a", "member_refs": []string{"stu_test"}, "operation": "individual",
			"students": []any{map[string]any{
				"student_ref": "stu_test", "operation": "grade", "evidence_key": "evidence-a",
				"grade": completed.Grade, "attempt_assertion": attemptRef(sub), "future_slice_field": "ignored by this reader",
			}},
		}},
	}
	if err = workspace.WriteDocument(filepath.Join(s.Root, "courses", "TEST", "group-results.md"), doc, "synthetic"); err != nil {
		t.Fatal(err)
	}
	if !s.completedHint(10, "stu_test", sub, snapshot) {
		t.Fatal("completed individual group result was not recognized for this student and attempt")
	}
	if s.completedHint(10, "another-student", sub, snapshot) {
		t.Fatal("one member's group result hid another member's work")
	}
}

func TestBroadGroupCompletionKeepsMemberAttemptsIndependent(t *testing.T) {
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
	a := fixtureAssignment()
	individuallyGraded := true
	a.GradeGroupStudentsIndividually = &individuallyGraded
	snapshot, err := Snapshot(a, s.Client.Origin())
	if err != nil {
		t.Fatal(err)
	}
	sub1, sub2 := fixtureSubmission(5), fixtureSubmission(6)
	sub1.GroupID, sub2.GroupID = 60, 60
	store := map[int64]workspace.Student{5: {Ref: "stu_one"}, 6: {Ref: "stu_two"}}
	active := map[int64]bool{5: true, 6: true}
	grade1 := &domain.GradeOperation{Attempt: attemptRef(sub1), Outcomes: domain.ActionOutcomes{Grade: domain.ActionOutcome{Status: "succeeded"}, Comment: domain.ActionOutcome{Status: "not-requested"}, Rubric: domain.ActionOutcome{Status: "not-requested"}, Late: domain.ActionOutcome{Status: "not-requested"}}}
	grade2 := &domain.GradeOperation{Attempt: attemptRef(sub2), Outcomes: grade1.Outcomes}
	write := func(rows []domain.StudentResult) {
		t.Helper()
		doc := map[string]any{"schema": 1, "kind": "group-results", "assignment": snapshot, "groups": []any{map[string]any{"group_id": 60, "artifact_key": "artifact", "member_refs": []string{"stu_one", "stu_two"}, "operation": "individual", "students": rows}}}
		if e := workspace.WriteDocument(filepath.Join(s.Root, "courses", "TEST", "group-results.md"), doc, "synthetic"); e != nil {
			t.Fatal(e)
		}
	}
	write([]domain.StudentResult{{StudentRef: "stu_one", Operation: "grade", Grade: grade1}})
	if handled, _ := s.groupSubmissionsHandled(a, []Submission{sub1, sub2}, store, active, snapshot); handled {
		t.Fatal("one member's result hid another member's current attempt")
	}
	write([]domain.StudentResult{{StudentRef: "stu_one", Operation: "grade", Grade: grade1}, {StudentRef: "stu_two", Operation: "grade", Grade: grade2}})
	if handled, count := s.groupSubmissionsHandled(a, []Submission{sub1, sub2}, store, active, snapshot); !handled || count != 2 {
		t.Fatalf("fully handled individual group was not recognized: handled=%t count=%d", handled, count)
	}
	shared := false
	a.GradeGroupStudentsIndividually = &shared
	sub1.State, sub2.State = "graded", "graded"
	graded := true
	sub1.GradeMatchesCurrentSubmission, sub2.GradeMatchesCurrentSubmission = &graded, &graded
	if handled, count := s.groupSubmissionsHandled(a, []Submission{sub1, sub2}, store, active, snapshot); !handled || count != 2 {
		t.Fatalf("shared propagation completion was not preserved: handled=%t count=%d", handled, count)
	}
}
