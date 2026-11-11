package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

// Due Thu 2026-11-05 11:59 p.m. Central. Cutoff 3 school days: Tue 11/10.
const (
	pdDue    = "2026-11-06T05:59:00Z"
	pdLock   = "2026-11-11T05:59:00Z"
	pdUnlock = "2026-11-02T06:00:00Z" // assigned Monday 11/02
)

func pdSubmission(user int64, at string) string {
	if at == "" {
		return fmt.Sprintf(`{"id":%d,"user_id":%d,"assignment_id":10,"attempt":0,"cached_due_date":%q,"workflow_state":"unsubmitted","excused":false,"user":{"id":%d,"name":"Student %d"}}`, 900+user, user, pdDue, user, user)
	}
	return fmt.Sprintf(`{"id":%d,"user_id":%d,"assignment_id":10,"attempt":1,"submitted_at":%q,"cached_due_date":%q,"workflow_state":"submitted","excused":false,"user":{"id":%d,"name":"Student %d"},"submission_history":[{"attempt":1,"submitted_at":%q,"cached_due_date":%q}]}`, 900+user, user, at, pdDue, user, user, at, pdDue)
}

type pdFixture struct {
	s      *Service
	refs   map[int64]string
	mu     sync.Mutex
	writes []string
	bodies map[string]map[string]any
	over   string
	// retake, when set, adds a tutorial retake of assignment 10 to the course list.
	retake bool
}

func newPDFixture(t *testing.T, overrides string, personalDue map[int64]string) *pdFixture {
	t.Helper()
	f := &pdFixture{refs: map[int64]string{}, bodies: map[string]map[string]any{}, over: overrides}
	subs := map[int64]string{
		3: pdSubmission(3, ""),                     // 50% extra time
		4: pdSubmission(4, "2026-11-05T18:00:00Z"), // on time
		5: pdSubmission(5, "2026-11-07T18:00:00Z"), // late
		6: pdSubmission(6, ""),                     // regular student
		7: pdSubmission(7, ""),                     // absent on the due date
		8: pdSubmission(8, ""),                     // extra time, shared override
		9: pdSubmission(9, ""),                     // other half of the shared override
	}
	// Canvas reports each student's own due date as cached_due_date.
	for id, due := range personalDue {
		subs[id] = strings.ReplaceAll(subs[id], pdDue, due)
	}
	assignment := fmt.Sprintf(`{"id":10,"course_id":1,"name":"Essay","points_possible":100,"submission_types":["online_text_entry"],"published":true,"due_at":%q,"lock_at":%q,"unlock_at":%q,"grading_type":"points"}`, pdDue, pdLock, pdUnlock)
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/courses/1/enrollments":
			var list []Enrollment
			for id := int64(3); id <= 9; id++ {
				list = append(list, Enrollment{UserID: id, Type: "StudentEnrollment", State: "active", User: CanvasUser{ID: id, Name: fmt.Sprintf("Student %d", id)}})
			}
			_ = json.NewEncoder(w).Encode(list)
		case r.URL.Path == "/api/v1/courses/1/assignments":
			list := assignment
			if f.retake {
				desc, _ := json.Marshal("<p>Retake</p>" + retakeMarker(10))
				list += `,{"id":11,"course_id":1,"name":"TUTORIAL-ONLY RETAKE - Essay","description":` + string(desc) + `,"published":true,"due_at":null}`
			}
			_, _ = io.WriteString(w, "["+list+"]")
		case r.URL.Path == "/api/v1/courses/1/assignments/10":
			_, _ = io.WriteString(w, assignment)
		case r.URL.Path == "/api/v1/courses/1/assignments/10/submissions":
			var all []string
			for id := int64(3); id <= 9; id++ {
				all = append(all, subs[id])
			}
			_, _ = io.WriteString(w, "["+strings.Join(all, ",")+"]")
		case r.URL.Path == "/api/v1/courses/1/assignments/10/overrides" && r.Method == http.MethodGet:
			_, _ = io.WriteString(w, f.over)
		case strings.HasPrefix(r.URL.Path, "/api/v1/courses/1/assignments/10/overrides") && r.Method != http.MethodGet:
			var body struct {
				Override map[string]any `json:"assignment_override"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			ids, _ := body.Override["student_ids"].([]any)
			f.mu.Lock()
			f.writes = append(f.writes, r.Method+" "+r.URL.Path)
			if len(ids) == 1 {
				f.bodies[strconv.Itoa(int(ids[0].(float64)))] = body.Override
			}
			f.mu.Unlock()
			_, _ = io.WriteString(w, `{"id":99}`)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	})
	three := 3
	s.Config.Courses[0].LateWork = &domain.LateWorkPolicy{LateCutoffSchoolDays: &three, CorrectionSchoolDays: 4}
	cal := workspace.SchoolCalendar{Schema: 1, Timezone: "America/Chicago", StartDate: "2026-08-19", EndDate: "2027-05-27",
		GradingPeriods: []workspace.GradingPeriod{{Name: "Term 2", StartDate: "2026-10-13", EndDate: "2026-12-18"}}}
	if err := workspace.WriteTeacherDocument(s.Root, "context/school-calendar.md", cal, ""); err != nil {
		t.Fatal(err)
	}
	store, err := workspace.OpenStudents(s.Root)
	if err != nil {
		t.Fatal(err)
	}
	for id := int64(3); id <= 9; id++ {
		st, err := store.Upsert(s.Client.Origin(), id, fmt.Sprintf("Student %d", id), "", "", "")
		if err != nil {
			t.Fatal(err)
		}
		f.refs[id] = st.Ref
	}
	for i := range store.Rows {
		switch store.Rows[i].UserID {
		case 3:
			store.Rows[i].ExtraTime, store.Rows[i].ExtraTimePercent = true, 50
		case 8:
			store.Rows[i].ExtraTime, store.Rows[i].ExtraTimeDays = true, 1
		}
	}
	if err = store.Save(); err != nil {
		t.Fatal(err)
	}
	absences, err := workspace.OpenAbsences(s.Root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = absences.Add(s.Config.Instance, f.refs[7], "2026-11-05"); err != nil {
		t.Fatal(err)
	}
	if err = absences.Save(); err != nil {
		t.Fatal(err)
	}
	f.s = s
	return f
}

func sameInstant(t *testing.T, got, want string) bool {
	t.Helper()
	g, err := time.Parse(time.RFC3339, got)
	if err != nil {
		return false
	}
	w, _ := time.Parse(time.RFC3339, want)
	return g.Equal(w)
}

func pdRows(r PersonalDatesReport) map[string]PersonalDatesRow {
	m := map[string]PersonalDatesRow{}
	for _, a := range r.Assignments {
		for _, row := range a.Rows {
			m[row.StudentRef] = row
		}
	}
	return m
}

func TestPersonalDatesPlansAndWrites(t *testing.T) {
	shared := `[{"id":50,"title":"Two students","student_ids":[8,9],"due_at":null}]`
	f := newPDFixture(t, shared, nil)
	// Friday 11/06 noon Central, after the due date.
	now := time.Date(2026, 11, 6, 18, 0, 0, 0, time.UTC)

	preview, err := f.s.PersonalDates(context.Background(), "TEST", 10, true, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.writes) != 0 {
		t.Fatalf("preview wrote: %v", f.writes)
	}
	if preview.Planned != 3 || preview.Held != 1 || preview.Written != 0 {
		t.Fatalf("preview=%+v rows=%+v", preview, pdRows(preview))
	}
	rows := pdRows(preview)

	// 50% of 3 school days (11/03-11/05) rounds up to 2: due Mon 11/09, lock Thu 11/12.
	if r := rows[f.refs[3]]; r.Status != "planned" || !sameInstant(t, r.DueAt, "2026-11-10T05:59:00Z") || !sameInstant(t, r.LockAt, "2026-11-13T05:59:00Z") {
		t.Fatalf("percent row=%+v", r)
	}
	// On time: corrections run to Wed 11/11, past the Tue lock.
	if r := rows[f.refs[4]]; r.Status != "planned" || !sameInstant(t, r.LockAt, "2026-11-12T05:59:00Z") || strings.Join(r.Why, ",") != "on time: open for corrections" {
		t.Fatalf("on-time row=%+v", r)
	}
	// Late work is not reopened; regular and unflagged students need nothing.
	for _, id := range []int64{5, 6, 9} {
		if r, ok := rows[f.refs[id]]; ok {
			t.Fatalf("student %d should have no row: %+v", id, r)
		}
	}
	// Absent on the due date: due Fri 11/06, lock Wed 11/11.
	if r := rows[f.refs[7]]; r.Status != "planned" || !sameInstant(t, r.DueAt, "2026-11-07T05:59:00Z") || !sameInstant(t, r.LockAt, "2026-11-12T05:59:00Z") {
		t.Fatalf("absent row=%+v", r)
	}
	if r := rows[f.refs[8]]; r.Status != "held" || !strings.Contains(r.Reason, "shares") {
		t.Fatalf("shared override row=%+v", r)
	}

	written, err := f.s.PersonalDates(context.Background(), "TEST", 10, false, now)
	if err != nil {
		t.Fatal(err)
	}
	if written.Written != 3 || len(f.writes) != 3 {
		t.Fatalf("written=%+v writes=%v", written, f.writes)
	}
	for _, w := range f.writes {
		if !strings.HasPrefix(w, "POST ") {
			t.Fatalf("expected new overrides, got %s", w)
		}
	}
	if b := f.bodies["3"]; b["due_at"] != "2026-11-10T05:59:00Z" || b["lock_at"] != "2026-11-13T05:59:00Z" || b["unlock_at"] != pdUnlock {
		t.Fatalf("student 3 body=%v", b)
	}
}

func TestPersonalDatesRerunWritesNothing(t *testing.T) {
	// Canvas already holds each flagged student's personal dates.
	existing := `[
	 {"id":61,"title":"Personal dates","student_ids":[3],"due_at":"2026-11-10T05:59:00Z","lock_at":"2026-11-13T05:59:00Z","lock_at_overridden":true},
	 {"id":62,"title":"Personal dates","student_ids":[4],"due_at":"` + pdDue + `","lock_at":"2026-11-12T05:59:00Z","lock_at_overridden":true},
	 {"id":63,"title":"Personal dates","student_ids":[7],"due_at":"2026-11-07T05:59:00Z","lock_at":"2026-11-12T05:59:00Z","lock_at_overridden":true},
	 {"id":64,"title":"Personal dates","student_ids":[8],"due_at":"2026-11-07T05:59:00Z","lock_at":"2026-11-12T05:59:00Z","lock_at_overridden":true}]`
	f := newPDFixture(t, existing, map[int64]string{3: "2026-11-10T05:59:00Z", 7: "2026-11-07T05:59:00Z", 8: "2026-11-07T05:59:00Z"})
	now := time.Date(2026, 11, 6, 18, 0, 0, 0, time.UTC)
	report, err := f.s.PersonalDates(context.Background(), "TEST", 10, false, now)
	if err != nil {
		t.Fatal(err)
	}
	if report.Written != 0 || len(f.writes) != 0 || report.Planned != 0 {
		t.Fatalf("rerun wrote: report=%+v writes=%v", report, f.writes)
	}
}

func TestPersonalDatesDoesNotReopenAMajorThatHasARetake(t *testing.T) {
	now := time.Date(2026, 11, 6, 18, 0, 0, 0, time.UTC)
	plain := newPDFixture(t, "[]", nil)
	if r, err := plain.s.PersonalDates(context.Background(), "TEST", 10, true, now); err != nil || pdRows(r)[plain.refs[4]].Status != "planned" {
		t.Fatalf("without a retake the on-time student is reopened: %v %+v", err, pdRows(r))
	}
	f := newPDFixture(t, "[]", nil)
	f.retake = true
	r, err := f.s.PersonalDates(context.Background(), "TEST", 10, true, now)
	if err != nil {
		t.Fatal(err)
	}
	rows := pdRows(r)
	if row, ok := rows[f.refs[4]]; ok {
		t.Fatalf("on-time student should not be reopened when a retake exists: %+v", row)
	}
	if rows[f.refs[3]].Status != "planned" {
		t.Fatalf("extra time still applies: %+v", rows)
	}
}
