package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

// Due Friday 2026-11-06 at 11:59 p.m. Central; Monday 11/09 is a closure.
const sweepDue = "2026-11-07T05:59:00Z"

func sweepSubmission(user int64, late bool, attempts ...string) string {
	history := []string{}
	for i, at := range attempts {
		history = append(history, fmt.Sprintf(`{"id":%d,"user_id":%d,"assignment_id":10,"attempt":%d,"submitted_at":%q,"cached_due_date":%q,"workflow_state":"submitted","late":%t}`, 900+user, user, i+1, at, sweepDue, late))
	}
	last := attempts[len(attempts)-1]
	seconds := 0
	if late {
		seconds = 100000
	}
	return fmt.Sprintf(`{"id":%d,"user_id":%d,"assignment_id":10,"attempt":%d,"submitted_at":%q,"cached_due_date":%q,"workflow_state":"submitted","submission_type":"online_text_entry","late":%t,"seconds_late":%d,"late_policy_status":null,"missing":false,"excused":false,"user":{"id":%d,"name":"Student %d"},"submission_history":[%s]}`,
		900+user, user, len(attempts), last, sweepDue, late, seconds, user, user, strings.Join(history, ","))
}

func TestLateSweepProposesSchoolTimeCorrectionsAndPostsThem(t *testing.T) {
	subs := map[int64]string{
		3: sweepSubmission(3, true, "2026-11-08T18:00:00Z"),                         // extra days, but a personal Canvas date
		4: sweepSubmission(4, false, "2026-11-06T18:00:00Z"),                        // on time
		5: sweepSubmission(5, true, "2026-11-08T18:00:00Z"),                         // Sunday noon
		6: sweepSubmission(6, true, "2026-11-10T21:00:00Z"),                         // Tuesday 3 p.m., one extra day
		7: sweepSubmission(7, true, "2026-11-10T16:00:00Z"),                         // Tuesday 10 a.m., absent on the due date
		8: sweepSubmission(8, true, "2026-11-08T18:00:00Z"),                         // extra time with no number saved
		9: sweepSubmission(9, true, "2026-11-07T16:00:00Z", "2026-11-11T16:00:00Z"), // two attempts
	}
	var mu sync.Mutex
	writes := map[int64]map[string]any{}
	assignment := `{"id":10,"course_id":1,"name":"Essay","points_possible":100,"submission_types":["online_text_entry"],"published":true,"due_at":"` + sweepDue + `","grading_type":"points","overrides":[{"id":1,"student_ids":[3],"due_at":"` + sweepDue + `"}]}`
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		const prefix = "/api/v1/courses/1/assignments/10/submissions/"
		switch {
		case r.URL.Path == "/api/v1/courses/1/enrollments":
			var list []Enrollment
			for id := int64(3); id <= 9; id++ {
				list = append(list, Enrollment{UserID: id, Type: "StudentEnrollment", State: "active", User: CanvasUser{ID: id, Name: fmt.Sprintf("Student %d", id)}})
			}
			_ = json.NewEncoder(w).Encode(list)
		case r.URL.Path == "/api/v1/courses/1/assignments":
			_, _ = io.WriteString(w, "["+assignment+"]")
		case r.URL.Path == "/api/v1/courses/1/assignments/10":
			_, _ = io.WriteString(w, assignment)
		case r.URL.Path == "/api/v1/courses/1/assignments/10/submissions":
			var all []string
			for id := int64(3); id <= 9; id++ {
				all = append(all, subs[id])
			}
			_, _ = io.WriteString(w, "["+strings.Join(all, ",")+"]")
		case strings.HasPrefix(r.URL.Path, prefix):
			id, _ := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, prefix), 10, 64)
			if r.Method == http.MethodPut {
				var body struct {
					Submission map[string]any `json:"submission"`
				}
				_ = json.NewDecoder(r.Body).Decode(&body)
				mu.Lock()
				writes[id] = body.Submission
				mu.Unlock()
			} else if r.Method != http.MethodGet {
				t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			}
			_, _ = io.WriteString(w, subs[id])
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	})
	calendar := workspace.SchoolCalendar{Schema: 1, Timezone: "America/Chicago", StartDate: "2026-08-19", EndDate: "2027-05-27", NonSchoolDates: []string{"2026-11-09"},
		GradingPeriods: []workspace.GradingPeriod{{Name: "Term 2", StartDate: "2026-10-13", EndDate: "2026-12-18"}}}
	if err := workspace.WriteTeacherDocument(s.Root, "context/school-calendar.md", calendar, ""); err != nil {
		t.Fatal(err)
	}
	store, err := workspace.OpenStudents(s.Root)
	if err != nil {
		t.Fatal(err)
	}
	refs := map[int64]string{}
	for id := int64(3); id <= 9; id++ {
		st, err := store.Upsert(s.Client.Origin(), id, fmt.Sprintf("Student %d", id), "", "", "")
		if err != nil {
			t.Fatal(err)
		}
		refs[id] = st.Ref
	}
	for i := range store.Rows {
		switch store.Rows[i].UserID {
		case 3:
			store.Rows[i].ExtraTime, store.Rows[i].ExtraTimeDays = true, 2
		case 6:
			store.Rows[i].ExtraTime, store.Rows[i].ExtraTimeDays = true, 1
		case 8:
			store.Rows[i].ExtraTime = true
		}
	}
	if err = store.Save(); err != nil {
		t.Fatal(err)
	}
	absences, err := workspace.OpenAbsences(s.Root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = absences.Add(s.Config.Instance, refs[7], "2026-11-06"); err != nil {
		t.Fatal(err)
	}
	if err = absences.Save(); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 11, 12, 15, 0, 0, 0, time.UTC)
	report, err := s.LateSweep(context.Background(), "TEST", "", now)
	if err != nil {
		t.Fatal(err)
	}
	if len(writes) != 0 {
		t.Fatalf("the sweep wrote to Canvas: %v", writes)
	}
	if report.Period != "Term 2" || report.Proposed != 5 || report.Held != 1 || report.Unchanged != 0 || report.CheckFirst != 1 || len(report.Assignments) != 1 || report.Folder == "" {
		t.Fatalf("report=%+v", report)
	}
	rows := map[string]LateSweepRow{}
	for _, row := range report.Assignments[0].Rows {
		rows[row.StudentRef] = row
	}
	if _, listed := rows[refs[4]]; listed || len(rows) != 6 {
		t.Fatalf("on-time work was listed or a late row is missing: %+v", rows)
	}
	// Sunday submission: only Friday's last minute is school time.
	if r := rows[refs[5]]; r.Status != "proposed" || r.NewStatus != "late" || r.SchoolSecondsLate != 60 || len(r.SkippedDates) != 2 {
		t.Fatalf("weekend row=%+v", r)
	}
	// One saved extra day: Friday plus one school day is Tuesday, past the closure.
	if r := rows[refs[6]]; r.Status != "proposed" || r.NewStatus != "none" || r.ExtraSchoolDays != 1 || r.Deadline != "2026-11-10T23:59:00-06:00" {
		t.Fatalf("extra-days row=%+v", r)
	}
	// Absent on the due date: due the next day back.
	if r := rows[refs[7]]; r.Status != "proposed" || r.NewStatus != "none" || len(r.AbsentDates) != 1 || r.AbsentDates[0] != "2026-11-06" || r.Deadline != "2026-11-10T23:59:00-06:00" {
		t.Fatalf("absence row=%+v", r)
	}
	if r := rows[refs[8]]; r.Status != "held" || !strings.Contains(r.Reason, "no number of extra days") {
		t.Fatalf("missing-days row=%+v", r)
	}
	// Two attempts: the first is timed and the row is marked for a look.
	if r := rows[refs[9]]; r.Status != "proposed" || !r.CheckFirstAttempt || r.SchoolSecondsLate != 60 {
		t.Fatalf("two-attempt row=%+v", r)
	}
	// A personal Canvas date already carries the allowance.
	if r := rows[refs[3]]; r.Status != "proposed" || r.ExtraSchoolDays != 0 || r.Note == "" || r.SchoolSecondsLate != 60 {
		t.Fatalf("personal-date row=%+v", r)
	}

	resultsPath := filepath.Join(s.Root, filepath.FromSlash(report.Assignments[0].Results))
	var results domain.ResultsDocument
	if _, err = ReadResultsDocument(s.Root, resultsPath, &results); err != nil {
		t.Fatal(err)
	}
	if _, err = domain.ValidateResults(&results); err != nil || len(results.Students) != 5 {
		t.Fatalf("results=%d err=%v", len(results.Students), err)
	}
	for _, row := range results.Students {
		if row.StudentRef == refs[9] && (row.LateOnly.Attempt.Attempt != 2 || row.LateOnly.LateTiming.SchoolTime.Attempt.Attempt != 1) {
			t.Fatalf("two-attempt write must target the current attempt and time the first: %+v", row.LateOnly)
		}
	}

	// The absence is removed before posting: that student's write is held.
	absences, _ = workspace.OpenAbsences(s.Root)
	absences.Remove(s.Config.Instance, refs[7], "2026-11-06")
	if err = absences.Save(); err != nil {
		t.Fatal(err)
	}
	posted, err := s.Post(context.Background(), resultsPath)
	if err != nil {
		t.Fatal(err)
	}
	outcomes := map[string]StudentPostReport{}
	for _, row := range posted.Students {
		outcomes[row.StudentRef] = row
	}
	if o := outcomes[refs[7]]; o.Outcomes.Late.Status != "held" || !strings.Contains(o.Outcomes.Late.Message, "absences changed") {
		t.Fatalf("changed absence was not held: %+v", o)
	}
	if _, wrote := writes[7]; wrote || len(writes) != 4 {
		t.Fatalf("writes=%v", writes)
	}
	if w := writes[5]; w["late_policy_status"] != "late" || w["seconds_late_override"] != float64(60) {
		t.Fatalf("weekend write=%v", w)
	}
	if w := writes[6]; w["late_policy_status"] != "none" {
		t.Fatalf("extra-days write=%v", w)
	}
}

func TestLateSweepNeedsACalendarAndAKnownPeriod(t *testing.T) {
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request %s", r.URL.Path)
	})
	if _, err := s.LateSweep(context.Background(), "TEST", "", time.Now()); err == nil || !strings.Contains(err.Error(), "school calendar") {
		t.Fatalf("err=%v", err)
	}
	calendar := workspace.SchoolCalendar{Schema: 1, Timezone: "America/Chicago", StartDate: "2026-08-19", EndDate: "2027-05-27"}
	if err := workspace.WriteTeacherDocument(s.Root, "context/school-calendar.md", calendar, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.LateSweep(context.Background(), "TEST", "Term 9", time.Now()); err == nil || !strings.Contains(err.Error(), "Term 9") {
		t.Fatalf("err=%v", err)
	}
}
