package workflow

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"canvasexpertse/internal/workspace"
)

func TestListExternalResourcesReadsConferenceEnvelopePagesAndOnlyKnownRefs(t *testing.T) {
	var calls []string
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.URL.Path+"?"+r.URL.RawQuery)
		switch r.URL.Path {
		case "/api/v1/courses/1/collaborations":
			if r.URL.Query().Get("page") == "2" {
				_ = json.NewEncoder(w).Encode([]any{map[string]any{"id": 2, "title": "Second", "url": "https://docs.example.edu/second", "context_type": "Course", "context_id": 1}})
			} else {
				w.Header().Set("Link", "<https://"+r.Host+"/api/v1/courses/1/collaborations?page=2&per_page=100>; rel=\"next\"")
				_ = json.NewEncoder(w).Encode([]any{map[string]any{"id": 1, "title": "Missing link", "url": nil, "context_type": "Course", "context_id": 1}})
			}
		case "/api/v1/courses/1/conferences":
			if r.URL.Query().Get("page") == "2" {
				_ = json.NewEncoder(w).Encode(map[string]any{"conferences": []any{map[string]any{"id": 12, "title": "Club meeting", "join_url": "https://meet.example.edu/room", "context_type": "Course", "context_id": 1}}})
			} else {
				w.Header().Set("Link", "<https://"+r.Host+"/api/v1/courses/1/conferences?page=2&per_page=100>; rel=\"next\"")
				_ = json.NewEncoder(w).Encode(map[string]any{"conferences": []any{map[string]any{"id": 11, "title": "No link", "url": nil, "join_url": nil, "users": []int64{33}, "invitees": []int64{34}, "attendees": []int64{33}, "context_type": "Course", "context_id": 1}}})
			}
		default:
			t.Errorf("unexpected request %s", r.URL.String())
			http.NotFound(w, r)
		}
	})
	students, err := workspace.OpenStudents(s.Root)
	if err != nil {
		t.Fatal(err)
	}
	student, err := students.Upsert(s.Client.Origin(), 33, "Private Raw Name", "Private", "SIS-SECRET", "private@example.edu")
	if err != nil {
		t.Fatal(err)
	}
	if err := students.Save(); err != nil {
		t.Fatal(err)
	}

	report, err := s.ListExternalResources(context.Background(), "TEST", 0, "all")
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Resources) != 4 {
		t.Fatalf("expected two pages of each resource, got %+v", report.Resources)
	}
	if report.Resources[0].URLStatus != "unavailable; use a teacher-provided link" || report.Resources[0].URL != "" {
		t.Fatalf("missing collaboration URL invented: %+v", report.Resources[0])
	}
	if report.Resources[2].URL != "" || len(report.Resources[2].ParticipantRefs) != 1 || report.Resources[2].ParticipantRefs[0] != student.Ref {
		t.Fatalf("conference link or participant identity was not normalized safely: %+v", report.Resources[2])
	}
	encoded, _ := json.Marshal(report)
	if strings.Contains(string(encoded), "Private Raw Name") || strings.Contains(string(encoded), "SIS-SECRET") || strings.Contains(string(encoded), "private@example.edu") {
		t.Fatalf("private participant details escaped: %s", encoded)
	}
	if len(calls) != 4 {
		t.Fatalf("pagination did not fetch all pages: %v", calls)
	}
}

func TestListExternalResourcesRefusesWrongCourseGroup(t *testing.T) {
	reads := 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		reads++
		if r.URL.Path != "/api/v1/groups/99" {
			t.Errorf("unexpected scoped read %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(canvasGroup{ID: 99, CourseID: 2, GroupCategoryID: 7, ContextType: "Course"})
	})
	if _, err := s.ListExternalResources(context.Background(), "TEST", 99, "all"); err == nil || reads != 1 {
		t.Fatalf("wrong-course group was accepted: reads=%d err=%v", reads, err)
	}
}
