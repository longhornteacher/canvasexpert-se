package workflow

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

type retakeFixture struct {
	s         *Service
	refs      map[int64]string
	mu        sync.Mutex
	overrides []string // JSON of each override written
	comments  map[int64][]string
	puts      []string
	published bool
	protected bool
	hasCode   bool
}

func retakeSubmission(user int64, state, entered, score, deducted, posted string, comments []string) string {
	var cs []string
	for i, c := range comments {
		text, _ := json.Marshal(c)
		cs = append(cs, fmt.Sprintf(`{"id":%d,"comment":%s,"author_id":1,"created_at":"2026-11-06T00:00:00Z"}`, i+1, text))
	}
	postedJSON := "null"
	if posted != "" {
		postedJSON = `"` + posted + `"`
	}
	enteredJSON, scoreJSON, deductedJSON := "null", "null", "null"
	if entered != "" {
		enteredJSON, scoreJSON = entered, score
	}
	if deducted != "" {
		deductedJSON = deducted
	}
	return fmt.Sprintf(`{"id":%d,"user_id":%d,"assignment_id":40,"attempt":1,"workflow_state":%q,"entered_score":%s,"score":%s,"points_deducted":%s,"posted_at":%s,"excused":false,"user":{"id":%d,"name":"Student %d"},"submission_comments":[%s]}`,
		900+user, user, state, enteredJSON, scoreJSON, deductedJSON, postedJSON, user, user, strings.Join(cs, ","))
}

func newRetakeFixture(t *testing.T) *retakeFixture {
	t.Helper()
	f := &retakeFixture{refs: map[int64]string{}, comments: map[int64][]string{}, protected: true, hasCode: true}
	desc, _ := json.Marshal("<p>Retake</p>" + retakeMarker(40))
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		f.mu.Lock()
		defer f.mu.Unlock()
		switch {
		case r.URL.Path == "/api/v1/courses/1/enrollments":
			var list []Enrollment
			for id := int64(3); id <= 7; id++ {
				list = append(list, Enrollment{UserID: id, Type: "StudentEnrollment", State: "active", User: CanvasUser{ID: id, Name: fmt.Sprintf("Student %d", id)}})
			}
			_ = json.NewEncoder(w).Encode(list)
		case r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/assignments/40":
			_, _ = io.WriteString(w, `{"id":40,"course_id":1,"name":"Essay Test","points_possible":100,"published":true}`)
		case r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/assignments":
			_, _ = io.WriteString(w, `[{"id":40,"course_id":1,"name":"Essay Test","points_possible":100},{"id":70,"course_id":1,"name":"TUTORIAL-ONLY RETAKE - Essay Test","description":`+string(desc)+`}]`)
		case r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/assignments/70":
			_, _ = fmt.Fprintf(w, `{"id":70,"course_id":1,"name":"TUTORIAL-ONLY RETAKE - Essay Test","description":%s,"published":%t,"only_visible_to_overrides":%t,"omit_from_final_grade":%t,"post_to_sis":%t}`, desc, f.published, f.protected, f.protected, !f.protected)
		case r.Method == "GET" && r.URL.Path == "/api/quiz/v1/courses/1/quizzes/70":
			_, _ = fmt.Fprintf(w, `{"id":"70","quiz_settings":{"require_student_access_code":%t}}`, f.hasCode)
		case r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/assignments/40/submissions":
			subs := []string{
				retakeSubmission(3, "graded", "52", "52", "", "2026-11-05T15:00:00Z", nil),
				retakeSubmission(4, "graded", "75", "75", "", "2026-11-05T15:00:00Z", nil),
				retakeSubmission(5, "graded", "65", "45", "20", "2026-11-05T15:00:00Z", f.comments[5]),
				retakeSubmission(6, "graded", "40", "40", "", "", nil),
				retakeSubmission(7, "unsubmitted", "", "", "", "", nil),
			}
			subs[0] = retakeSubmission(3, "graded", "52", "52", "", "2026-11-05T15:00:00Z", f.comments[3])
			_, _ = io.WriteString(w, "["+strings.Join(subs, ",")+"]")
		case r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/assignments/70/overrides":
			_, _ = io.WriteString(w, "["+strings.Join(f.overrides, ",")+"]")
		case r.Method == "POST" && r.URL.Path == "/api/v1/courses/1/assignments/70/overrides":
			b, _ := io.ReadAll(r.Body)
			var body struct {
				Override map[string]any `json:"assignment_override"`
			}
			_ = json.Unmarshal(b, &body)
			ids, _ := json.Marshal(body.Override["student_ids"])
			f.overrides = append(f.overrides, fmt.Sprintf(`{"id":%d,"title":"Tutorial retake","student_ids":%s,"due_at":%q,"lock_at":%q}`, len(f.overrides)+1, ids, body.Override["due_at"], body.Override["lock_at"]))
			_, _ = io.WriteString(w, `{"id":1}`)
		case r.Method == "PUT" && strings.HasPrefix(r.URL.Path, "/api/v1/courses/1/assignments/40/submissions/"):
			var id int64
			_, _ = fmt.Sscanf(r.URL.Path, "/api/v1/courses/1/assignments/40/submissions/%d", &id)
			b, _ := io.ReadAll(r.Body)
			var body struct {
				Comment map[string]any `json:"comment"`
			}
			_ = json.Unmarshal(b, &body)
			f.comments[id] = append(f.comments[id], body.Comment["text_comment"].(string))
			_, _ = io.WriteString(w, `{"id":1}`)
		case r.Method == "PUT" && r.URL.Path == "/api/v1/courses/1/assignments/70":
			b, _ := io.ReadAll(r.Body)
			f.puts = append(f.puts, string(b))
			if strings.Contains(string(b), `"published":true`) {
				f.published = true
			}
			if strings.Contains(string(b), "only_visible_to_overrides") {
				f.protected = true
			}
			_, _ = io.WriteString(w, `{"id":70,"published":true}`)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	})
	cal := workspace.SchoolCalendar{Schema: 1, Timezone: "America/Chicago", StartDate: "2026-08-19", EndDate: "2027-05-27"}
	if err := workspace.WriteTeacherDocument(s.Root, "context/school-calendar.md", cal, ""); err != nil {
		t.Fatal(err)
	}
	grading := domain.GradingConfig{Schema: 1, Retakes: &domain.RetakePolicy{CapPercent: 70, SchoolDays: 4}}
	if err := workspace.WriteTeacherDocument(s.Root, "context/grading.md", grading, ""); err != nil {
		t.Fatal(err)
	}
	store, err := workspace.OpenStudents(s.Root)
	if err != nil {
		t.Fatal(err)
	}
	for id := int64(3); id <= 7; id++ {
		st, err := store.Upsert(s.Client.Origin(), id, fmt.Sprintf("Student %d", id), "", "", "")
		if err != nil {
			t.Fatal(err)
		}
		f.refs[id] = st.Ref
	}
	for i := range store.Rows {
		if store.Rows[i].UserID == 3 {
			store.Rows[i].ExtraTime, store.Rows[i].ExtraTimePercent = true, 50
		}
	}
	if err = store.Save(); err != nil {
		t.Fatal(err)
	}
	f.s = s
	return f
}

func retakeRows(r RetakeAssignReport) map[string]RetakeAssignRow {
	m := map[string]RetakeAssignRow{}
	for _, row := range r.Rows {
		m[row.StudentRef] = row
	}
	return m
}

func TestRetakeAssignSelectsFailingPostedStudentsAndResumes(t *testing.T) {
	f := newRetakeFixture(t)
	preview, err := f.s.RetakeAssign(t.Context(), "TEST", 40, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.overrides) != 0 || len(f.puts) != 0 || len(f.comments) != 0 {
		t.Fatalf("preview wrote: %v %v %v", f.overrides, f.puts, f.comments)
	}
	rows := retakeRows(preview)
	if rows[f.refs[3]].Status != "planned" || rows[f.refs[5]].Status != "planned" {
		t.Fatalf("52 and 65-shown-45-late must be selected: %+v", rows)
	}
	if _, ok := rows[f.refs[4]]; ok {
		t.Fatalf("75 passes and gets no retake: %+v", rows[f.refs[4]])
	}
	if r := rows[f.refs[6]]; r.Status != "skipped" || !strings.Contains(r.Reason, "not posted") {
		t.Fatalf("unposted: %+v", r)
	}
	if r := rows[f.refs[7]]; r.Status != "skipped" || !strings.Contains(r.Reason, "no graded attempt") {
		t.Fatalf("no graded attempt: %+v", r)
	}
	// Posted Thu 11/05 + 4 school days is Wed 11/11; 50% of 4 days adds 2 more, to Fri 11/13.
	if rows[f.refs[5]].Deadline != "2026-11-11 23:59" || rows[f.refs[3]].Deadline != "2026-11-13 23:59" {
		t.Fatalf("deadlines: %+v %+v", rows[f.refs[5]], rows[f.refs[3]])
	}

	report, err := f.s.RetakeAssign(t.Context(), "TEST", 40, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.overrides) != 2 || !f.published || !report.Published {
		t.Fatalf("overrides=%d published=%v report=%+v", len(f.overrides), f.published, report)
	}
	if got := f.comments[5]; len(got) != 1 || got[0] != "Tutorial retake open until 11/11." {
		t.Fatalf("comment: %v", got)
	}
	if got := f.comments[3]; len(got) != 1 || got[0] != "Tutorial retake open until 11/13." {
		t.Fatalf("extra-time comment: %v", got)
	}
	for _, o := range f.overrides {
		if strings.Contains(o, "TUTORIAL") {
			t.Fatal("override leaked something odd")
		}
	}

	// A rerun does only what is unfinished: nothing.
	puts := len(f.puts)
	again, err := f.s.RetakeAssign(t.Context(), "TEST", 40, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.overrides) != 2 || len(f.comments[5]) != 1 || len(f.comments[3]) != 1 || len(f.puts) != puts {
		t.Fatalf("rerun wrote again: overrides=%d comments=%v puts=%d", len(f.overrides), f.comments, len(f.puts))
	}
	if r := retakeRows(again)[f.refs[5]]; r.Status != "complete" {
		t.Fatalf("rerun row: %+v", r)
	}
}

func TestRetakeAssignRestoresProtectionBeforePublishing(t *testing.T) {
	f := newRetakeFixture(t)
	f.protected = false
	if _, err := f.s.RetakeAssign(t.Context(), "TEST", 40, false); err != nil {
		t.Fatal(err)
	}
	if len(f.puts) < 2 || !strings.Contains(f.puts[0], "only_visible_to_overrides") || !strings.Contains(f.puts[len(f.puts)-1], `"published":true`) {
		t.Fatalf("protection must be restored before the publish: %v", f.puts)
	}
}

func TestRetakeAssignStopsWithoutAnAccessCode(t *testing.T) {
	f := newRetakeFixture(t)
	f.hasCode = false
	if _, err := f.s.RetakeAssign(t.Context(), "TEST", 40, false); err == nil || !strings.Contains(err.Error(), "access code") {
		t.Fatalf("expected access-code stop, got %v", err)
	}
	if len(f.overrides) != 0 || len(f.puts) != 0 {
		t.Fatalf("nothing may be written: %v %v", f.overrides, f.puts)
	}
}

func TestRetakeAssignNeedsAPolicy(t *testing.T) {
	f := newRetakeFixture(t)
	if err := workspace.WriteTeacherDocument(f.s.Root, "context/grading.md", domain.GradingConfig{Schema: 1}, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := f.s.RetakeAssign(t.Context(), "TEST", 40, true); err == nil || !strings.Contains(err.Error(), "no retake policy") {
		t.Fatalf("got %v", err)
	}
}
