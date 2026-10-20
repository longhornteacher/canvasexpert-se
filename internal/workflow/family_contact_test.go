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
	"time"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

type familyAudienceFixture struct {
	service      *Service
	dir          string
	doc          FamilyContact
	members      map[int64][]canvasMembership
	refs         map[int64]string
	wrongCourse  bool
	privateError bool
}

func newFamilyAudienceFixture(t *testing.T) *familyAudienceFixture {
	t.Helper()
	f := &familyAudienceFixture{members: map[int64][]canvasMembership{}, refs: map[int64]string{}}
	names := []string{"Ghost", "MexiKid", "Book Three", "Book Four", "Book Five", "The Tequila Worm"}
	f.service, _ = workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if f.privateError {
			http.Error(w, `{"name":"Private Full Name", "user_id":987654321}`, http.StatusForbidden)
			return
		}
		if r.Method != http.MethodGet {
			t.Errorf("audience preparation made a write: %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		write := func(v any) { _ = json.NewEncoder(w).Encode(v) }
		for course := int64(1); course <= 2; course++ {
			set, first := 49+course, 50+10*course
			switch r.URL.Path {
			case fmt.Sprintf("/api/v1/courses/%d/enrollments", course):
				rows := []map[string]any{}
				for i := int64(0); i < 5; i++ {
					id := course*100 + i
					rows = append(rows, map[string]any{"user_id": id, "type": "StudentEnrollment", "enrollment_state": "active", "user": map[string]any{"id": id, "name": fmt.Sprintf("Private Full Name %d", id), "login_id": "private-login", "sis_user_id": "private-sis"}})
				}
				rows = append(rows, map[string]any{"user_id": 300, "type": "StudentEnrollment", "enrollment_state": "active", "user": map[string]any{"id": 300, "name": "Shared Private Student"}})
				if course == 1 {
					rows = append(rows, map[string]any{"user_id": 999, "type": "StudentEnrollment", "enrollment_state": "active", "user": map[string]any{"id": 999, "name": "Unassigned Private Student"}}, map[string]any{"user_id": 998, "type": "StudentEnrollment", "enrollment_state": "inactive", "user": map[string]any{"id": 998, "name": "Inactive Private Student"}})
				}
				write(rows)
				return
			case fmt.Sprintf("/api/v1/group_categories/%d", set):
				actualCourse := course
				if f.wrongCourse {
					actualCourse = 999
				}
				write(map[string]any{"id": set, "course_id": actualCourse, "context_type": "Course", "name": "Book clubs", "non_collaborative": false})
				return
			case fmt.Sprintf("/api/v1/group_categories/%d/groups", set):
				groups := []map[string]any{}
				for i, name := range names {
					groups = append(groups, map[string]any{"id": first + int64(i), "course_id": course, "group_category_id": set, "context_type": "Course", "name": name})
				}
				write(groups)
				return
			}
		}
		for group, rows := range f.members {
			if r.URL.Path == fmt.Sprintf("/api/v1/groups/%d/memberships", group) {
				write(rows)
				return
			}
		}
		t.Errorf("unexpected family audience request: %s", r.URL.Path)
		http.NotFound(w, r)
	})
	f.service.Config.Courses = append(f.service.Config.Courses, domain.Course{Nickname: "SECOND", ID: 2, Current: true})
	store, err := workspace.OpenStudents(f.service.Root)
	if err != nil {
		t.Fatal(err)
	}
	for course := int64(1); course <= 2; course++ {
		for i := int64(0); i < 6; i++ {
			group := 50 + 10*course + i
			f.members[group] = []canvasMembership{}
			if i < 5 {
				id := 100*course + i
				f.members[group] = append(f.members[group], canvasMembership{ID: group * 10, GroupID: group, UserID: id, WorkflowState: "accepted"})
				st, e := store.Upsert(f.service.Client.Origin(), id, "Private Full Name", "Private", "private-sis", "private-login")
				if e != nil {
					t.Fatal(e)
				}
				f.refs[id] = st.Ref
			}
			if i == 0 {
				f.members[group] = append(f.members[group], canvasMembership{ID: group*10 + 1, GroupID: group, UserID: 300, WorkflowState: "accepted"})
			}
		}
	}
	for _, id := range []int64{300, 999} {
		st, e := store.Upsert(f.service.Client.Origin(), id, "Private Full Name", "Private", "", "")
		if e != nil {
			t.Fatal(e)
		}
		f.refs[id] = st.Ref
	}
	if err = store.Save(); err != nil {
		t.Fatal(err)
	}
	f.doc = FamilyContact{Title: "Unit 2 book clubs"}
	for i := 0; i < 5; i++ {
		name := names[i]
		if i == 1 {
			name = " mEXi KiD "
		}
		first := FamilyGroupTarget{Course: "TEST", GroupSet: 50, Name: name}
		second := FamilyGroupTarget{Course: "SECOND", GroupSet: 51, ID: 70 + int64(i)}
		f.doc.Messages = append(f.doc.Messages, FamilyMessage{Key: fmt.Sprintf("book-%d", i), Audience: FamilyAudience{Groups: []FamilyGroupTarget{first, second}}, Subject: "Book clubs for {{student_name}}", Body: "Reading {{group_name}} this week."})
	}
	f.dir = filepath.Join(f.service.Root, "courses", "TEST", "family-contact", "unit-2")
	f.save(t)
	return f
}

func (f *familyAudienceFixture) save(t *testing.T) {
	t.Helper()
	b, err := json.Marshal(f.doc)
	if err != nil {
		t.Fatal(err)
	}
	if err = workspace.WriteFile(filepath.Join(f.dir, "contact.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestFamilyAudienceTwoCourseBookClubPreparationAndPrivacy(t *testing.T) {
	f := newFamilyAudienceFixture(t)
	snapshot, err := f.service.PrepareFamilyContact(context.Background(), f.dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Blocking) != 0 || len(snapshot.Students) != 11 || snapshot.Counts["book-0"] != 3 {
		t.Fatalf("unexpected book club audience: %+v", snapshot)
	}
	for i := 1; i < 5; i++ {
		if snapshot.Counts[fmt.Sprintf("book-%d", i)] != 2 {
			t.Fatalf("wrong count: %+v", snapshot.Counts)
		}
	}
	warnings := strings.Join(snapshot.Warnings, "\n")
	if strings.Count(warnings, "The Tequila Worm") != 2 || !strings.Contains(warnings, f.refs[999]) {
		t.Fatalf("missing empty groups or unmatched student: %s", warnings)
	}
	b, err := os.ReadFile(filepath.Join(f.dir, "audience.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{"Private", "private-login", "private-sis", "user_id", "\"100\"", "\"300\"", f.service.Client.Origin()} {
		if strings.Contains(string(b), private) {
			t.Fatalf("private identity leaked: %s", private)
		}
	}
	if _, _, err = f.service.CheckFamilyAudience(context.Background(), f.dir); err != nil {
		t.Fatal(err)
	}
	marker := []byte("preserve accepted outcomes")
	if err = os.WriteFile(filepath.Join(f.dir, "outcomes.json"), marker, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = f.service.PrepareFamilyContact(context.Background(), f.dir); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(f.dir, "outcomes.json"))
	if string(got) != string(marker) {
		t.Fatal("preparation changed outcomes")
	}
}

func TestFamilyAudienceDuplicateMessagesAreReviewableAndBlockDelivery(t *testing.T) {
	f := newFamilyAudienceFixture(t)
	f.doc.Messages = append(f.doc.Messages, FamilyMessage{Key: "reminder", Audience: FamilyAudience{Students: []string{f.refs[100]}}, Subject: "Reminder", Body: "Please bring a book."})
	f.save(t)
	snapshot, err := f.service.PrepareFamilyContact(context.Background(), f.dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Blocking) != 1 || !strings.Contains(snapshot.Blocking[0], f.refs[100]) {
		t.Fatalf("duplicate not reported: %+v", snapshot)
	}
	if _, saved, e := LoadFamilyAudience(f.service.Root, f.dir); e != nil || len(saved.Blocking) != 1 {
		t.Fatalf("blocking review unavailable: %+v %v", saved, e)
	}
	if _, _, err = f.service.CheckFamilyAudience(context.Background(), f.dir); err == nil {
		t.Fatal("duplicate message audience allowed delivery")
	}
}

func TestFamilyAudienceWholeCourseExplicitRefAndRepeatedSelectorsDeduplicate(t *testing.T) {
	f := newFamilyAudienceFixture(t)
	f.doc.Messages = []FamilyMessage{{Key: "all", Audience: FamilyAudience{Courses: []string{"TEST", "TEST", "SECOND"}, Students: []string{f.refs[100], f.refs[100]}, Groups: []FamilyGroupTarget{{Course: "TEST", GroupSet: 50, ID: 60}}}, Subject: "Reminder", Body: "Hello {{student_name}}."}}
	f.save(t)
	snapshot, err := f.service.PrepareFamilyContact(context.Background(), f.dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Blocking) != 0 || len(snapshot.Students) != 12 || snapshot.Counts["all"] != 12 {
		t.Fatalf("selectors did not deduplicate: %+v", snapshot)
	}
	for _, row := range snapshot.Students {
		if row.Ref == f.refs[100] && row.Group != "Ghost" {
			t.Fatalf("group placeholder lost: %+v", row)
		}
	}
}

func TestFamilyAudienceRejectsContentDriftMembershipDriftAndSnapshotEdits(t *testing.T) {
	f := newFamilyAudienceFixture(t)
	prepare := func() {
		t.Helper()
		if _, err := f.service.PrepareFamilyContact(context.Background(), f.dir); err != nil {
			t.Fatal(err)
		}
	}
	prepare()
	contactPath := filepath.Join(f.dir, "contact.json")
	newer := time.Now().Add(time.Second)
	if err := os.Chtimes(contactPath, newer, newer); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LoadFamilyAudience(f.service.Root, f.dir); err == nil {
		t.Fatal("touched contact allowed old audience")
	}
	older := time.Now().Add(-time.Minute)
	if err := os.Chtimes(contactPath, older, older); err != nil {
		t.Fatal(err)
	}
	prepare()
	f.doc.Messages[0].Subject = "Updated subject"
	f.save(t)
	if _, _, err := f.service.CheckFamilyAudience(context.Background(), f.dir); err == nil {
		t.Fatal("changed content allowed delivery")
	}
	prepare()
	f.members[60] = f.members[60][1:]
	if _, _, err := f.service.CheckFamilyAudience(context.Background(), f.dir); err == nil {
		t.Fatal("changed membership allowed delivery")
	}
	prepare()
	_, saved, err := LoadFamilyAudience(f.service.Root, f.dir)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Students[0].Message == "book-0" {
		saved.Students[0].Message = "book-1"
	} else {
		saved.Students[0].Message = "book-0"
	}
	b, _ := json.Marshal(saved)
	if err = os.WriteFile(filepath.Join(f.dir, "audience.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err = f.service.CheckFamilyAudience(context.Background(), f.dir); err == nil {
		t.Fatal("edited audience rows allowed delivery")
	}
	prepare()
	_, saved, err = LoadFamilyAudience(f.service.Root, f.dir)
	if err != nil {
		t.Fatal(err)
	}
	saved.Counts["book-0"]++
	b, _ = json.Marshal(saved)
	if err = os.WriteFile(filepath.Join(f.dir, "audience.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err = f.service.CheckFamilyAudience(context.Background(), f.dir); err == nil {
		t.Fatal("edited audience counts allowed delivery")
	}
}

func TestFamilyAudienceCanvasFailuresDoNotExposePrivateResponse(t *testing.T) {
	f := newFamilyAudienceFixture(t)
	f.privateError = true
	_, err := f.service.PrepareFamilyContact(context.Background(), f.dir)
	if err == nil {
		t.Fatal("failed Canvas read accepted")
	}
	if strings.Contains(err.Error(), "Private") || strings.Contains(err.Error(), "987654321") || strings.Contains(err.Error(), "user_id") {
		t.Fatalf("Canvas error exposed private response: %v", err)
	}
}

func TestFamilyAudienceRefusesWrongCourseGroupAndLocalRunEscape(t *testing.T) {
	f := newFamilyAudienceFixture(t)
	f.wrongCourse = true
	if _, err := f.service.PrepareFamilyContact(context.Background(), f.dir); err == nil {
		t.Fatal("cross-course group accepted")
	}
	for _, dir := range []string{t.TempDir(), filepath.Join(f.service.Root, "context", "family-contact", "run"), filepath.Join(f.service.Root, "courses", "TEST", "pulled", "run")} {
		if _, err := LoadFamilyContact(f.service.Root, dir); err == nil {
			t.Fatalf("unconstrained run accepted: %s", dir)
		}
	}
	link := filepath.Join(f.service.Root, "courses", "TEST", "family-contact", "redirect")
	if err := os.Symlink(t.TempDir(), link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := LoadFamilyContact(f.service.Root, link); err == nil {
		t.Fatal("run symlink escape accepted")
	}
}
