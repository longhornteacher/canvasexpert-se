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

type applyOriginal struct {
	entered  string
	deducted string
	comments []string
}

type applyFixture struct {
	s         *Service
	refs      map[int64]string
	mu        sync.Mutex
	originals map[int64]*applyOriginal
	retakes   map[int64][2]string // state, score
	puts      []string
	failPut   bool
	newQuiz   bool
}

func (f *applyFixture) originalJSON(user int64) string {
	o := f.originals[user]
	var cs []string
	for i, c := range o.comments {
		text, _ := json.Marshal(c)
		cs = append(cs, fmt.Sprintf(`{"id":%d,"comment":%s,"author_id":1,"created_at":"2026-11-0%dT00:00:00Z"}`, i+1, text, i+1))
	}
	score := o.entered
	deducted := "null"
	if o.deducted != "" {
		deducted = o.deducted
		var e, d float64
		_, _ = fmt.Sscan(o.entered, &e)
		_, _ = fmt.Sscan(o.deducted, &d)
		score = fmt.Sprint(e - d)
	}
	return fmt.Sprintf(`{"id":%d,"user_id":%d,"assignment_id":40,"attempt":1,"workflow_state":"graded","entered_score":%s,"score":%s,"points_deducted":%s,"posted_at":"2026-11-05T15:00:00Z","user":{"id":%d,"name":"Student %d"},"submission_comments":[%s]}`,
		900+user, user, o.entered, score, deducted, user, user, strings.Join(cs, ","))
}

func newApplyFixture(t *testing.T) *applyFixture {
	t.Helper()
	f := &applyFixture{refs: map[int64]string{}, originals: map[int64]*applyOriginal{
		3: {entered: "52"}, 4: {entered: "65"}, 5: {entered: "65", deducted: "20"}, 6: {entered: "25"}, 7: {entered: "30"},
	}, retakes: map[int64][2]string{3: {"graded", "88"}, 4: {"graded", "54"}, 5: {"graded", "88"}, 6: {"graded", "10"}, 7: {"pending_review", ""}}}
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
			_, _ = fmt.Fprintf(w, `{"id":40,"course_id":1,"name":"Essay Test","points_possible":100,"is_quiz_lti_assignment":%t}`, f.newQuiz)
		case r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/assignments":
			_, _ = io.WriteString(w, `[{"id":40,"course_id":1,"name":"Essay Test","points_possible":100},{"id":70,"course_id":1,"name":"TUTORIAL-ONLY RETAKE - Essay Test","points_possible":100,"description":`+string(desc)+`}]`)
		case r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/assignments/70":
			_, _ = io.WriteString(w, `{"id":70,"course_id":1,"name":"TUTORIAL-ONLY RETAKE - Essay Test","points_possible":100,"description":`+string(desc)+`}`)
		case r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/assignments/70/submissions":
			var subs []string
			for id := int64(3); id <= 7; id++ {
				state, score := f.retakes[id][0], f.retakes[id][1]
				entered := "null"
				if score != "" {
					entered = score
				}
				subs = append(subs, fmt.Sprintf(`{"id":%d,"user_id":%d,"assignment_id":70,"attempt":1,"submitted_at":"2026-11-09T15:00:00Z","workflow_state":%q,"entered_score":%s,"score":%s,"user":{"id":%d,"name":"Student %d"}}`, 700+id, id, state, entered, entered, id, id))
			}
			_, _ = io.WriteString(w, "["+strings.Join(subs, ",")+"]")
		case r.Method == "GET" && r.URL.Path == "/api/v1/courses/1/assignments/40/submissions":
			var subs []string
			for id := int64(3); id <= 7; id++ {
				subs = append(subs, f.originalJSON(id))
			}
			_, _ = io.WriteString(w, "["+strings.Join(subs, ",")+"]")
		case r.Method == "PUT" && strings.HasPrefix(r.URL.Path, "/api/v1/courses/1/assignments/40/submissions/"):
			var id int64
			_, _ = fmt.Sscanf(r.URL.Path, "/api/v1/courses/1/assignments/40/submissions/%d", &id)
			b, _ := io.ReadAll(r.Body)
			if f.failPut {
				w.WriteHeader(500)
				return
			}
			var body struct {
				Comment    map[string]any    `json:"comment"`
				Submission map[string]string `json:"submission"`
			}
			_ = json.Unmarshal(b, &body)
			f.puts = append(f.puts, fmt.Sprintf("%d %s", id, b))
			if body.Comment != nil {
				f.originals[id].comments = append(f.originals[id].comments, body.Comment["text_comment"].(string))
			}
			if g, ok := body.Submission["posted_grade"]; ok {
				f.originals[id].entered = g
			}
			_, _ = io.WriteString(w, `{"id":1}`)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	})
	if err := workspace.WriteTeacherDocument(s.Root, "context/grading.md", domain.GradingConfig{Schema: 1, Retakes: &domain.RetakePolicy{CapPercent: 70, SchoolDays: 4}}, ""); err != nil {
		t.Fatal(err)
	}
	floor := domain.MustDecimal("40")
	s.Config.Courses[0].EffortFloor = &floor
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
	if err = store.Save(); err != nil {
		t.Fatal(err)
	}
	f.s = s
	return f
}

func applyRows(r RetakeApplyReport) map[string]RetakeApplyRow {
	m := map[string]RetakeApplyRow{}
	for _, row := range r.Rows {
		m[row.StudentRef] = row
	}
	return m
}

func TestRetakeApplyPreviewsFirstThenWritesOnlyReviewedValues(t *testing.T) {
	f := newApplyFixture(t)
	// With no saved preview, apply runs the preview and writes nothing.
	first, err := f.s.RetakeApply(t.Context(), "TEST", 40, []string{f.refs[6]}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Preview || len(f.puts) != 0 || first.PreviewFile == "" {
		t.Fatalf("first run must be a preview: %+v puts=%v", first, f.puts)
	}
	rows := applyRows(first)
	if rows[f.refs[3]].Status != "raise" || rows[f.refs[3]].Target != "70/100" {
		t.Fatalf("52 with retake 88 raises to 70: %+v", rows[f.refs[3]])
	}
	if rows[f.refs[4]].Status != "keep" {
		t.Fatalf("65 with retake 54 keeps: %+v", rows[f.refs[4]])
	}
	if r := rows[f.refs[6]]; r.Status != "raise" || r.Target != "40/100" {
		t.Fatalf("genuine 10 on 25 is floored to 40 inside the formula: %+v", r)
	}
	if r := rows[f.refs[7]]; r.Status != "held" || !strings.Contains(r.Reason, "not fully graded") {
		t.Fatalf("pending written items hold: %+v", r)
	}

	// One student changes after the preview was reviewed.
	f.originals[4].entered = "66"
	second, err := f.s.RetakeApply(t.Context(), "TEST", 40, []string{f.refs[6]}, false)
	if err != nil {
		t.Fatal(err)
	}
	rows = applyRows(second)
	if r := rows[f.refs[4]]; r.Status != "held" || !strings.Contains(r.Reason, "changed since") {
		t.Fatalf("a changed student is held: %+v", r)
	}
	if rows[f.refs[3]].Status != "written" || rows[f.refs[5]].Status != "written" || rows[f.refs[6]].Status != "written" {
		t.Fatalf("the others proceed: %+v", rows)
	}
	o3 := f.originals[3]
	if o3.entered != "70" || len(o3.comments) != 1 || o3.comments[0] != "Retake credit: your tutorial retake earned 88/100. Retake credit is capped at 70%, so this grade is now entered as 70/100." {
		t.Fatalf("student 3: %+v", o3)
	}
	if got := f.originals[5].comments[0]; !strings.HasSuffix(got, "Your late deduction still applies.") || f.originals[5].entered != "70" {
		t.Fatalf("a late original shows the late line and the entered grade: %q %s", got, f.originals[5].entered)
	}
	if got := f.originals[6].comments[0]; !strings.Contains(got, "earned 10/100") || f.originals[6].entered != "40" {
		t.Fatalf("floored student: %q %s", got, f.originals[6].entered)
	}
	for _, p := range f.puts {
		if strings.Contains(p, "rubric_assessment") {
			t.Fatalf("no rubric payload may be sent: %s", p)
		}
	}
	// Comment and grade travel in one update.
	if !strings.Contains(f.puts[0], "posted_grade") || !strings.Contains(f.puts[0], "text_comment") {
		t.Fatalf("raise must be one update: %s", f.puts[0])
	}

	// A fresh preview, then apply again: the held student is written; nobody is written twice.
	before := len(f.puts)
	if _, err = f.s.RetakeApply(t.Context(), "TEST", 40, []string{f.refs[6]}, true); err != nil {
		t.Fatal(err)
	}
	third, err := f.s.RetakeApply(t.Context(), "TEST", 40, []string{f.refs[6]}, false)
	if err != nil {
		t.Fatal(err)
	}
	rows = applyRows(third)
	if rows[f.refs[3]].Status != "keep" && rows[f.refs[3]].Status != "unchanged" && rows[f.refs[3]].Status != "written" {
		t.Fatalf("unexpected: %+v", rows[f.refs[3]])
	}
	if rows[f.refs[4]].Status != "written" || len(f.originals[4].comments) != 1 || !strings.HasPrefix(f.originals[4].comments[0], "Tutorial retake: you earned 54/100. Your grade stays 66/100.") {
		t.Fatalf("keep comment: %+v %+v", rows[f.refs[4]], f.originals[4])
	}
	// Students already raised now keep (their grade equals the target), so nothing more is raised.
	if len(f.originals[3].comments) != 1 || f.originals[3].entered != "70" {
		t.Fatalf("student 3 was written twice: %+v", f.originals[3])
	}
	_ = before
	// A fourth run changes nothing.
	count := len(f.puts)
	if _, err = f.s.RetakeApply(t.Context(), "TEST", 40, []string{f.refs[6]}, true); err != nil {
		t.Fatal(err)
	}
	if _, err = f.s.RetakeApply(t.Context(), "TEST", 40, []string{f.refs[6]}, false); err != nil {
		t.Fatal(err)
	}
	if len(f.puts) != count {
		t.Fatalf("a re-run wrote again: %v", f.puts[count:])
	}
}

func TestRetakeApplyWritesOnlyTheGradeWhenTheCommentLanded(t *testing.T) {
	f := newApplyFixture(t)
	f.originals[3].comments = []string{"Retake credit: your tutorial retake earned 88/100. Retake credit is capped at 70%, so this grade is now entered as 70/100."}
	if _, err := f.s.RetakeApply(t.Context(), "TEST", 40, nil, true); err != nil {
		t.Fatal(err)
	}
	if _, err := f.s.RetakeApply(t.Context(), "TEST", 40, nil, false); err != nil {
		t.Fatal(err)
	}
	var mine []string
	for _, p := range f.puts {
		if strings.HasPrefix(p, "3 ") {
			mine = append(mine, p)
		}
	}
	if len(mine) != 1 || strings.Contains(mine[0], "text_comment") || !strings.Contains(mine[0], `"posted_grade":"70"`) || len(f.originals[3].comments) != 1 {
		t.Fatalf("expected a grade-only write, got %v / %+v", mine, f.originals[3])
	}
}

func TestRetakeApplyHoldsANewQuizOriginalWithTheNumber(t *testing.T) {
	f := newApplyFixture(t)
	f.newQuiz = true
	if _, err := f.s.RetakeApply(t.Context(), "TEST", 40, nil, true); err != nil {
		t.Fatal(err)
	}
	report, err := f.s.RetakeApply(t.Context(), "TEST", 40, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	r := applyRows(report)[f.refs[3]]
	if r.Status != "held" || !strings.Contains(r.Reason, "SpeedGrader") || !strings.Contains(r.Reason, "18") {
		t.Fatalf("held row must give the number: %+v", r)
	}
	for _, p := range f.puts {
		if strings.HasPrefix(p, "3 ") {
			t.Fatalf("nothing may be written for a held raise: %s", p)
		}
	}
}

func TestRetakeApplyRetriesAfterAFailedWriteWithoutALocalRecord(t *testing.T) {
	f := newApplyFixture(t)
	if _, err := f.s.RetakeApply(t.Context(), "TEST", 40, nil, true); err != nil {
		t.Fatal(err)
	}
	f.failPut = true
	report, err := f.s.RetakeApply(t.Context(), "TEST", 40, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if r := applyRows(report)[f.refs[3]]; r.Status != "held" || len(f.originals[3].comments) != 0 || f.originals[3].entered != "52" {
		t.Fatalf("failed write leaves nothing behind: %+v %+v", r, f.originals[3])
	}
	f.failPut = false
	if _, err = f.s.RetakeApply(t.Context(), "TEST", 40, nil, false); err != nil {
		t.Fatal(err)
	}
	if f.originals[3].entered != "70" || len(f.originals[3].comments) != 1 {
		t.Fatalf("retry: %+v", f.originals[3])
	}
}
