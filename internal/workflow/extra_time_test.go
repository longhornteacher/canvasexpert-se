package workflow

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
	"gopkg.in/yaml.v3"
)

func TestPullWorkProjectsExtraTimeFromActiveStudentAuthority(t *testing.T) {
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/courses/1/enrollments":
			_ = json.NewEncoder(w).Encode([]Enrollment{
				{UserID: 5, Type: "StudentEnrollment", State: "active", User: CanvasUser{ID: 5, Name: "Same Name"}},
				{UserID: 6, Type: "StudentEnrollment", State: "active", User: CanvasUser{ID: 6, Name: "Same Name"}},
			})
		case "/api/v1/courses/1/assignments/10":
			_ = json.NewEncoder(w).Encode(fixtureAssignment())
		case "/api/v1/courses/1/assignments/10/submissions":
			_ = json.NewEncoder(w).Encode([]Submission{fixtureSubmission(5), fixtureSubmission(6)})
		case "/api/v1/courses/1/assignments/10/submissions/5":
			_ = json.NewEncoder(w).Encode(fixtureSubmission(5))
		case "/api/v1/courses/1/assignments/10/submissions/6":
			_ = json.NewEncoder(w).Encode(fixtureSubmission(6))
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			http.NotFound(w, r)
		}
	})
	store, err := workspace.OpenStudents(s.Root)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []int64{5, 6} {
		if _, err = store.Upsert(s.Client.Origin(), id, "Same Name", "Same", "private-sis", "private-login"); err != nil {
			t.Fatal(err)
		}
	}
	store.Rows[1].ExtraTime = true // inactive local settings deliberately disagree
	if err = store.Save(); err != nil {
		t.Fatal(err)
	}
	if err = workspace.WriteDocument(filepath.Join(s.Root, "context", "courses.md"), s.Config, "Synthetic configured course."); err != nil {
		t.Fatal(err)
	}
	if _, err = workspace.SetRulesLocation(s.Root, t.TempDir(), nil, ""); err != nil {
		t.Fatal(err)
	}
	store, err = workspace.OpenStudents(s.Root)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{}
	for i := range store.Rows {
		store.Rows[i].ExtraTime = store.Rows[i].UserID == 5
		want[store.Rows[i].Ref] = store.Rows[i].ExtraTime
	}
	if err = store.Save(); err != nil {
		t.Fatal(err)
	}
	report, err := s.PullWork(context.Background(), "TEST", "10")
	if err != nil || report.Students != 2 || len(report.Packets) != 1 {
		t.Fatalf("pull=%+v err=%v", report, err)
	}
	var packet domain.PacketDocument
	if _, err = workspace.ReadDocument(report.Packets[0], &packet); err != nil {
		t.Fatal(err)
	}
	for _, rel := range packet.Students {
		path := filepath.Join(s.Root, filepath.FromSlash(rel))
		var work domain.StudentWorkDocument
		if _, err = workspace.ReadDocument(path, &work); err != nil {
			t.Fatal(err)
		}
		value, mapped := want[work.StudentRef]
		if !mapped || work.ExtraTime == nil || *work.ExtraTime != value {
			t.Fatalf("extra time used a wrong identity/authority or lost explicit false: %+v", work)
		}
		body, err := os.ReadFile(path)
		if err != nil || strings.Contains(string(body), "private-sis") || strings.Contains(string(body), "private-login") {
			t.Fatalf("student projection exposed private roster fields: err=%v", err)
		}
	}
}

func TestExtraTimePacketPresenceAndGroupTimingProjection(t *testing.T) {
	for _, tc := range []struct {
		name, field string
		value       *bool
	}{
		{name: "old packet is unknown"},
		{name: "saved false", field: "extra_time: false\n", value: boolPtr(false)},
		{name: "saved true", field: "extra_time: true\n", value: boolPtr(true)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var student domain.StudentWorkDocument
			var member domain.GroupWorkMember
			body := []byte("student_ref: s-test\n" + tc.field)
			if err := yaml.Unmarshal(body, &student); err != nil {
				t.Fatal(err)
			}
			if err := yaml.Unmarshal(body, &member); err != nil {
				t.Fatal(err)
			}
			projected := groupMemberAsStudentWork(domain.GroupWorkDocument{}, member)
			for _, value := range []*bool{student.ExtraTime, member.ExtraTime, projected.ExtraTime} {
				if (value == nil) != (tc.value == nil) || value != nil && *value != *tc.value {
					t.Fatalf("extra_time presence/value was lost: got=%v want=%v", value, tc.value)
				}
			}
			for _, document := range []any{student, member} {
				encoded, err := yaml.Marshal(document)
				if err != nil || strings.Contains(string(encoded), "extra_time:") != (tc.value != nil) {
					t.Fatalf("extra_time serialization lost presence: %s err=%v", encoded, err)
				}
			}
		})
	}
}
