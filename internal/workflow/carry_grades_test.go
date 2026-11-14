package workflow

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

// Synthetic course: target 90 (100 points), sources 41 (100 points) and 42
// (50 points). Students 3-8.
type carryFixture struct {
	s       *Service
	refs    map[int64]string
	mu      sync.Mutex
	scores  map[int64]map[int64]string // assignment -> user -> entered score
	pending map[int64]bool             // user with an ungraded attempt on 41
	puts    []string
	newQuiz bool
}

func newCarryFixture(t *testing.T) *carryFixture {
	t.Helper()
	f := &carryFixture{refs: map[int64]string{}, pending: map[int64]bool{}, scores: map[int64]map[int64]string{
		41: {3: "88", 6: "70", 7: "95"},
		42: {4: "30", 6: "40", 8: "25"},
		90: {7: "97"},
	}}
	points := map[int64]int{41: 100, 42: 50, 90: 100}
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		f.mu.Lock()
		defer f.mu.Unlock()
		var id, user int64
		switch {
		case r.URL.Path == "/api/v1/courses/1/enrollments":
			var list []string
			for u := int64(3); u <= 8; u++ {
				list = append(list, fmt.Sprintf(`{"user_id":%d,"type":"StudentEnrollment","enrollment_state":"active","user":{"id":%d,"name":"Student %d"}}`, u, u, u))
			}
			_, _ = io.WriteString(w, "["+strings.Join(list, ",")+"]")
		case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/submissions"):
			_, _ = fmt.Sscanf(r.URL.Path, "/api/v1/courses/1/assignments/%d/submissions", &id)
			var subs []string
			for u := int64(3); u <= 8; u++ {
				if id == 90 && u == 8 {
					continue // the target is not assigned to student 8
				}
				score, state := "null", "unsubmitted"
				if v, ok := f.scores[id][u]; ok {
					score, state = v, "graded"
				}
				if id == 41 && f.pending[u] {
					state = "pending_review"
				}
				subs = append(subs, fmt.Sprintf(`{"id":%d,"user_id":%d,"assignment_id":%d,"workflow_state":%q,"entered_score":%s,"score":%s}`, id*100+u, u, id, state, score, score))
			}
			_, _ = io.WriteString(w, "["+strings.Join(subs, ",")+"]")
		case r.Method == "GET":
			if _, e := fmt.Sscanf(r.URL.Path, "/api/v1/courses/1/assignments/%d", &id); e != nil || points[id] == 0 {
				t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
				w.WriteHeader(404)
				return
			}
			_, _ = fmt.Fprintf(w, `{"id":%d,"course_id":1,"name":"Column %d","points_possible":%d,"grading_type":"points","published":true,"submission_types":["none"],"is_quiz_lti_assignment":%t}`, id, id, points[id], id == 90 && f.newQuiz)
		case r.Method == "PUT":
			_, _ = fmt.Sscanf(r.URL.Path, "/api/v1/courses/1/assignments/%d/submissions/%d", &id, &user)
			b, _ := io.ReadAll(r.Body)
			f.puts = append(f.puts, fmt.Sprintf("%d/%d %s", id, user, b))
			if id != 90 {
				t.Errorf("only the target may be written: %s", r.URL.Path)
			}
			grade := string(b)
			grade = grade[strings.Index(grade, `"posted_grade":"`)+len(`"posted_grade":"`):]
			f.scores[90][user] = grade[:strings.Index(grade, `"`)]
			_, _ = io.WriteString(w, `{"id":1}`)
		}
	})
	store, err := workspace.OpenStudents(s.Root)
	if err != nil {
		t.Fatal(err)
	}
	for u := int64(3); u <= 8; u++ {
		st, err := store.Upsert(s.Client.Origin(), u, fmt.Sprintf("Student %d", u), "", "", "")
		if err != nil {
			t.Fatal(err)
		}
		f.refs[u] = st.Ref
	}
	if err = store.Save(); err != nil {
		t.Fatal(err)
	}
	f.s = s
	return f
}

func (f *carryFixture) run(t *testing.T, preview bool) map[int64]CarryGradesRow {
	t.Helper()
	report, err := f.s.CarryGrades(t.Context(), "TEST", 90, []int64{42, 41}, preview)
	if err != nil {
		t.Fatal(err)
	}
	rows := map[int64]CarryGradesRow{}
	for u, ref := range f.refs {
		for _, row := range report.Rows {
			if row.StudentRef == ref {
				rows[u] = row
			}
		}
	}
	if len(rows) != 6 {
		t.Fatalf("every active student is reported: %+v", report.Rows)
	}
	return rows
}

func TestCarryGradesPreviewsThenWritesOnlyReviewedValues(t *testing.T) {
	f := newCarryFixture(t)
	// Without a saved preview, the first run is a preview.
	first, err := f.s.CarryGrades(t.Context(), "TEST", 90, []int64{41, 42}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Preview || len(f.puts) != 0 || first.Counts["write"] != 2 || first.Counts["held"] != 4 {
		t.Fatalf("first run must be a preview: %+v puts=%v", first, f.puts)
	}
	if first.Sources[0].Scored != 3 || first.Sources[1].Scored != 3 || !strings.Contains(strings.Join(first.Notes, " "), "scaled") {
		t.Fatalf("per-source counts and the scaling note: %+v", first)
	}
	rows := f.run(t, true)
	if r := rows[3]; r.Status != "write" || r.Value != "88/100" || r.SourceID != 41 {
		t.Fatalf("one source score is carried: %+v", r)
	}
	if r := rows[4]; r.Status != "write" || r.Value != "60/100" || r.Source != "30/50" {
		t.Fatalf("a 50-point source is scaled to the same percent: %+v", r)
	}
	for u, want := range map[int64]string{5: "no score", 6: "different scores", 7: "never lowered", 8: "not assigned"} {
		if r := rows[u]; r.Status != "held" || !strings.Contains(r.Reason, want) {
			t.Fatalf("student %d should be held for %q: %+v", u, want, r)
		}
	}

	// The teacher edits student 4's target grade after reviewing.
	f.scores[90][4] = "10"
	rows = f.run(t, false)
	if r := rows[4]; r.Status != "held" || !strings.Contains(r.Reason, "changed since") {
		t.Fatalf("a target edited since the preview is held: %+v", r)
	}
	if rows[3].Status != "written" || len(f.puts) != 1 || !strings.Contains(f.puts[0], `"posted_grade":"88"`) || strings.Contains(f.puts[0], "comment") {
		t.Fatalf("only the reviewed, unchanged student is written: %+v puts=%v", rows[3], f.puts)
	}
	if f.scores[90][7] != "97" || f.scores[90][4] != "10" {
		t.Fatalf("held students were left alone: %+v", f.scores[90])
	}

	// A rerun writes nothing new.
	if rows = f.run(t, false); rows[3].Status != "unchanged" || len(f.puts) != 1 {
		t.Fatalf("rerun: %+v puts=%v", rows[3], f.puts)
	}
}

func TestCarryGradesFollowsASourceUpButNeverDown(t *testing.T) {
	f := newCarryFixture(t)
	f.run(t, true)
	f.run(t, false)
	f.scores[41][3] = "92"
	if r := f.run(t, false)[3]; r.Status != "held" || f.scores[90][3] != "88" {
		t.Fatalf("a raised source needs a fresh preview first: %+v", r)
	}
	f.run(t, true)
	if r := f.run(t, false)[3]; r.Status != "written" || f.scores[90][3] != "92" {
		t.Fatalf("a raised source raises the target: %+v", r)
	}
	f.scores[41][3] = "80"
	f.run(t, true)
	if r := f.run(t, false)[3]; r.Status != "held" || f.scores[90][3] != "92" {
		t.Fatalf("a lowered source never lowers the target: %+v", r)
	}
}

func TestCarryGradesHoldsUnfinishedGradingAndRefusesUnsafeTargets(t *testing.T) {
	f := newCarryFixture(t)
	f.pending[3] = true
	if r := f.run(t, true)[3]; r.Status != "held" || !strings.Contains(r.Reason, "not fully graded") {
		t.Fatalf("a pending attempt holds: %+v", r)
	}
	f.newQuiz = true
	if _, err := f.s.CarryGrades(t.Context(), "TEST", 90, []int64{41}, true); err == nil || !strings.Contains(err.Error(), "quiz") {
		t.Fatalf("a quiz target is refused: %v", err)
	}
	if _, err := f.s.CarryGrades(t.Context(), "TEST", 41, []int64{41, 42}, true); err == nil {
		t.Fatal("the target cannot be a source")
	}
	if len(f.puts) != 0 {
		t.Fatalf("nothing may be written: %v", f.puts)
	}
}

func TestNoSubmissionAssignmentDocumentIsAccepted(t *testing.T) {
	doc := domain.AssignmentDocument{Schema: 1, Kind: "assignment", Operation: "create", Course: "TEST", Title: "Book Club Quiz 1", Points: domain.MustDecimal("100"), SubmissionTypes: []string{"none"}}
	if err := domain.ValidateAssignment(doc, "Your book quiz score."); err != nil {
		t.Fatal(err)
	}
	doc.SubmissionTypes = []string{"none", "online_upload"}
	if err := domain.ValidateAssignment(doc, "Your book quiz score."); err == nil {
		t.Fatal("none cannot be mixed with another submission type")
	}
}
