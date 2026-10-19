package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"canvasexpertse/internal/canvas"
	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

// Parent report policy values. The agent decides what to do with the facts;
// these only fix the date windows the report is computed over.
const (
	correctionWindowDays = 14
	dueSoonDays          = 7
	maxCommentChars      = 1500
)

// ParentReportPacket is the pseudonymized report the agent reads. It carries no
// contacts, real names or Goes By names. A student appears once, with one
// section per selected course, so one email can cover several courses.
type ParentReportPacket struct {
	Schema         int                   `json:"schema"`
	Kind           string                `json:"kind"`
	RunID          string                `json:"run_id"`
	Courses        []string              `json:"courses"`
	AsOf           string                `json:"as_of"`
	Scope          *ParentReportScope    `json:"scope"`
	CorrectionDays int                   `json:"correction_window_days"`
	DueSoonDays    int                   `json:"due_soon_days"`
	Students       []ParentReportStudent `json:"students"`
	Notes          []string              `json:"notes,omitempty"`
}

// ParentReportScope records the calendar and due-date window used to build a
// report. It is provenance for this saved run, not a live policy switch.
type ParentReportScope struct {
	Timezone        string `json:"timezone"`
	PeriodName      string `json:"period_name"`
	PeriodStartDate string `json:"period_start_date"`
	PeriodEndDate   string `json:"period_end_date"`
	RecentStartAt   string `json:"recent_start_at"`
	RecentEndAt     string `json:"recent_end_at"`
	DueSoonEndAt    string `json:"due_soon_end_at"`
}

type ParentReportStudent struct {
	Ref          string               `json:"student_ref"`
	Alias        string               `json:"alias"`
	Courses      []ParentReportCourse `json:"courses"`
	SkipEligible bool                 `json:"skip_eligible"`
}

type ParentReportCourse struct {
	Course            string              `json:"course"`
	Timezone          string              `json:"timezone"`
	CourseScore       *string             `json:"course_score"`
	CourseScoreSource string              `json:"course_score_source"`
	Flags             []string            `json:"flags,omitempty"`
	Missing           []ParentReportItem  `json:"missing,omitempty"`
	Corrections       []ParentCorrection  `json:"corrections,omitempty"`
	DueSoon           []ParentReportItem  `json:"due_soon,omitempty"`
	History           []ParentHistoryItem `json:"history,omitempty"`
	WorkDue           []ParentWorkDueItem `json:"work_due"`
}

// ParentWorkDueItem is one required assignment whose effective due date is in
// the saved retrospective window. The status describes available Canvas
// evidence without inferring submission or a grade when neither is known.
type ParentWorkDueItem struct {
	ParentReportItem
	Status   string  `json:"status"`
	Possible string  `json:"points_possible,omitempty"`
	First    *string `json:"first_earned,omitempty"`
	Current  *string `json:"canvas_score,omitempty"`
	Note     string  `json:"note,omitempty"`
}

type ParentReportItem struct {
	AssignmentID int64  `json:"assignment_id"`
	Title        string `json:"title"`
	URL          string `json:"url"`
	DueAt        string `json:"due_at,omitempty"`
	// Basis explains why the work is listed: recorded_zero, canvas_missing,
	// or not_submitted_canvas_not_marked_missing for online work past due.
	// A recorded zero does not establish whether work was submitted.
	Basis string `json:"basis,omitempty"`
}

type ParentCorrection struct {
	ParentReportItem
	CorrectionDeadline string                 `json:"correction_deadline"`
	Comments           []ParentCorrectionNote `json:"teacher_comments,omitempty"`
	// PerItemFeedback is "unverified" for New Quizzes: the Engine cannot read
	// per-item feedback, so eligibility must not be ruled out from comments.
	PerItemFeedback string `json:"per_item_feedback,omitempty"`
}

type ParentCorrectionNote struct {
	ID        int64  `json:"id"`
	Attempt   *int   `json:"attempt,omitempty"`
	CreatedAt string `json:"created_at"`
	Text      string `json:"text"`
}

type ParentHistoryItem struct {
	ParentReportItem
	Possible string  `json:"points_possible"`
	First    *string `json:"first_earned"`
	Latest   *string `json:"latest_earned"`
	Current  *string `json:"canvas_score"`
	// FirstUnavailable is set when submission history cannot establish the
	// first assessment; entered_score is never substituted.
	FirstUnavailable bool   `json:"first_unavailable,omitempty"`
	Note             string `json:"note,omitempty"`
}

// ParentReport reads current Canvas data for the configured courses and writes
// a pseudonymized report under courses/<first nickname>/parent-updates/<run-id>/.
// refs, when given, limits the report to those student refs.
func (s *Service) ParentReport(ctx context.Context, nicknames []string, refs []string, now time.Time) (string, ParentReportPacket, error) {
	if len(nicknames) == 0 {
		return "", ParentReportPacket{}, fmt.Errorf("at least one course is required")
	}
	config, e := loadParentConfig(s.Root)
	if e != nil {
		return "", ParentReportPacket{}, e
	}
	calendar, e := workspace.ReadSchoolCalendar(s.Root)
	if e != nil {
		return "", ParentReportPacket{}, fmt.Errorf("school calendar: %w", e)
	}
	if e = calendar.Validate(); e != nil {
		return "", ParentReportPacket{}, fmt.Errorf("school calendar: %w; correct context/school-calendar.md before creating a parent report", e)
	}
	period, e := calendar.CurrentPeriod(now)
	if e != nil {
		return "", ParentReportPacket{}, fmt.Errorf("school calendar: %w; correct context/school-calendar.md before creating a parent report", e)
	}
	scope, recentStart, dueSoonEnd, e := makeParentReportScope(calendar, period, now)
	if e != nil {
		return "", ParentReportPacket{}, e
	}
	want := map[string]bool{}
	for _, r := range refs {
		want[r] = true
	}
	students, e := workspace.OpenStudents(s.Root)
	if e != nil {
		return "", ParentReportPacket{}, e
	}
	runID := now.UTC().Format("20060102T150405Z")
	packet := ParentReportPacket{Schema: 1, Kind: "parent-report", RunID: runID, CorrectionDays: correctionWindowDays, DueSoonDays: dueSoonDays, Scope: scope}
	byRef := map[string]*ParentReportStudent{}
	matched := map[string]bool{}
	for _, nickname := range nicknames {
		course, e := s.Course(nickname)
		if e != nil {
			return "", ParentReportPacket{}, e
		}
		zone := course.Timezone
		if zone == "" {
			zone = "America/Chicago"
		}
		loc, e := time.LoadLocation(zone)
		if e != nil {
			return "", ParentReportPacket{}, fmt.Errorf("unknown course timezone %q", zone)
		}
		packet.Courses = append(packet.Courses, course.Nickname)
		enrollments, e := StudentEnrollments(ctx, s.Client, course.ID)
		if e != nil {
			return "", ParentReportPacket{}, e
		}
		type enrolled struct {
			st    workspace.Student
			score *string
		}
		byUser := map[int64]*enrolled{}
		var order []int64
		for _, en := range enrollments {
			if en.State != "active" || en.Type != "StudentEnrollment" {
				continue
			}
			id := en.UserID
			if id == 0 {
				id = en.User.ID
			}
			if id <= 0 {
				continue
			}
			st, e := students.Upsert(s.Client.Origin(), id, en.User.Name, en.User.ShortName, en.User.SISID, en.User.Login)
			if e != nil {
				return "", ParentReportPacket{}, e
			}
			if len(want) > 0 && !want[st.Ref] {
				continue
			}
			matched[st.Ref] = true
			var score *string
			if en.Grades != nil && en.Grades.CurrentScore != nil {
				v := en.Grades.CurrentScore.String()
				score = &v
			}
			if byUser[id] == nil {
				order = append(order, id)
			}
			byUser[id] = &enrolled{st, score}
		}
		assignments, e := parentAssignments(ctx, s.Client, course.ID)
		if e != nil {
			return "", ParentReportPacket{}, e
		}
		byAssignment := map[int64]Assignment{}
		for _, a := range assignments {
			byAssignment[a.ID] = a
		}
		subs, e := courseSubmissions(ctx, s.Client, course.ID)
		if e != nil {
			return "", ParentReportPacket{}, e
		}
		perStudent := map[int64][]Submission{}
		for _, sub := range subs {
			perStudent[sub.UserID] = append(perStudent[sub.UserID], sub)
		}
		// submission_history keeps one version per attempt, so a regrade of
		// the same attempt hides the first score. The gradebook history feed
		// keeps every grading version.
		versions := map[int64]map[int64][]Submission{}
		feed, e := gradeVersions(ctx, s.Client, course.ID)
		if e != nil {
			packet.Notes = append(packet.Notes, course.Nickname+": gradebook history unavailable; first scores come from submission history and may miss same-attempt regrades")
		}
		for _, v := range feed {
			if versions[v.UserID] == nil {
				versions[v.UserID] = map[int64][]Submission{}
			}
			versions[v.UserID][v.AssignmentID] = append(versions[v.UserID][v.AssignmentID], v)
		}
		for _, id := range order {
			en := byUser[id]
			rc := ParentReportCourse{Course: course.Nickname, Timezone: zone, CourseScore: en.score, CourseScoreSource: "canvas_enrollment_current_score", WorkDue: []ParentWorkDueItem{}}
			if en.score == nil {
				rc.Flags = append(rc.Flags, "course score unavailable from Canvas")
			}
			studentSubs, incomplete := completeStudentSubmissions(ctx, s.Client, course.ID, id, assignments, perStudent[id], recentStart, now, dueSoonEnd)
			for _, note := range incomplete {
				rc.Flags = append(rc.Flags, note)
			}
			buildStudentReportScoped(&rc, studentSubs, versions[id], byAssignment, now, loc, recentStart, dueSoonEnd)
			rs := byRef[en.st.Ref]
			if rs == nil {
				rs = &ParentReportStudent{Ref: en.st.Ref, Alias: en.st.Alias}
				byRef[en.st.Ref] = rs
			}
			rs.Courses = append(rs.Courses, rc)
		}
	}
	if e = students.Save(); e != nil {
		return "", ParentReportPacket{}, e
	}
	for r := range want {
		if !matched[r] {
			return "", ParentReportPacket{}, fmt.Errorf("student %s is not an active student in the selected courses", r)
		}
	}
	for _, rs := range byRef {
		rs.SkipEligible = parentSkipEligible(*rs, config.SkipScore)
		packet.Students = append(packet.Students, *rs)
	}
	sort.Slice(packet.Students, func(i, j int) bool { return packet.Students[i].Alias < packet.Students[j].Alias })
	if len(packet.Students) == 0 {
		packet.Notes = append(packet.Notes, "no active students matched")
	}
	packet.AsOf = now.UTC().Format(time.RFC3339)

	rel := filepath.Join("courses", packet.Courses[0], "parent-updates", runID, "report.json")
	path, e := workspace.ConstrainPath(s.Root, rel)
	if e != nil {
		return "", packet, e
	}
	b, e := json.MarshalIndent(packet, "", "  ")
	if e != nil {
		return "", packet, e
	}
	if e = workspace.WriteFile(path, append(b, '\n'), 0644); e != nil {
		return "", packet, e
	}
	return path, packet, nil
}

func makeParentReportScope(calendar workspace.SchoolCalendar, period workspace.GradingPeriod, asOf time.Time) (*ParentReportScope, time.Time, time.Time, error) {
	loc, err := time.LoadLocation(calendar.Timezone)
	if err != nil {
		return nil, time.Time{}, time.Time{}, fmt.Errorf("school calendar timezone %q is invalid; correct context/school-calendar.md", calendar.Timezone)
	}
	periodStart, err := time.ParseInLocation("2006-01-02", period.StartDate, loc)
	if err != nil {
		return nil, time.Time{}, time.Time{}, fmt.Errorf("school calendar period start date is invalid; correct context/school-calendar.md")
	}
	periodEnd, err := time.ParseInLocation("2006-01-02", period.EndDate, loc)
	if err != nil {
		return nil, time.Time{}, time.Time{}, fmt.Errorf("school calendar period end date is invalid; correct context/school-calendar.md")
	}
	recentStart := asOf.In(loc).AddDate(0, 0, -correctionWindowDays)
	if periodStart.After(recentStart) {
		recentStart = periodStart
	}
	periodExclusiveEnd := periodEnd.AddDate(0, 0, 1)
	dueSoonEnd := asOf.In(loc).AddDate(0, 0, dueSoonDays)
	if periodExclusiveEnd.Before(dueSoonEnd) {
		dueSoonEnd = periodExclusiveEnd
	}
	scope := &ParentReportScope{Timezone: calendar.Timezone, PeriodName: period.Name, PeriodStartDate: period.StartDate, PeriodEndDate: period.EndDate,
		RecentStartAt: recentStart.Format(time.RFC3339), RecentEndAt: asOf.Format(time.RFC3339), DueSoonEndAt: dueSoonEnd.Format(time.RFC3339)}
	return scope, recentStart, dueSoonEnd, nil
}

// parentAssignments loads the catalog with the existing Canvas effective-date
// details needed to distinguish base dates from individualized and undated
// overrides.
func parentAssignments(ctx context.Context, c *canvas.Client, courseID int64) ([]Assignment, error) {
	var result []Assignment
	err := c.List(ctx, fmt.Sprintf("/api/v1/courses/%d/assignments?include[]=all_dates&include[]=overrides&per_page=100", courseID), &result)
	return result, err
}

// courseSubmissions reads every student's submission, with history and
// comments, for the whole course in one paged request.
func courseSubmissions(ctx context.Context, c *canvas.Client, courseID int64) ([]Submission, error) {
	var result []Submission
	err := c.List(ctx, fmt.Sprintf("/api/v1/courses/%d/students/submissions?student_ids[]=all&include[]=submission_history&include[]=submission_comments&include[]=visibility&per_page=100", courseID), &result)
	return result, err
}

// gradeVersions reads every grading version in the course from the gradebook
// history feed, oldest first per student and assignment.
func gradeVersions(ctx context.Context, c *canvas.Client, courseID int64) ([]Submission, error) {
	var result []Submission
	if err := c.List(ctx, fmt.Sprintf("/api/v1/courses/%d/gradebook_history/feed?per_page=100", courseID), &result); err != nil {
		return nil, err
	}
	sort.SliceStable(result, func(i, j int) bool { return gradedAt(result[i]) < gradedAt(result[j]) })
	return result, nil
}

func gradedAt(s Submission) string {
	if s.GradedAt == nil {
		return ""
	}
	return *s.GradedAt
}

// scoreChangedSince reports whether a grading version at or after since set a
// score different from the version before it (a first score counts).
func scoreChangedSince(versions []Submission, since, now time.Time) bool {
	var prev *json.Number
	for _, v := range versions {
		if v.Score == nil {
			continue
		}
		if v.GradedAt != nil {
			t, err := time.Parse(time.RFC3339, *v.GradedAt)
			if err == nil && t.After(since) && !t.After(now) && (prev == nil || prev.String() != v.Score.String()) {
				return true
			}
		}
		prev = v.Score
	}
	return false
}

// effectiveDue is the student's own due date (overrides and extensions
// included) when Canvas reports one, else the assignment's.
func effectiveDue(sub Submission, a Assignment) *time.Time {
	for _, v := range []*string{sub.CachedDueDate, a.DueAt} {
		if v == nil || *v == "" {
			continue
		}
		if t, err := time.Parse(time.RFC3339, *v); err == nil {
			return &t
		}
	}
	return nil
}

// completeStudentSubmissions joins the catalog to the bulk submission feed.
// Missing rows and rows without visibility are resolved with the existing
// assignment/student read before they can affect a family report.
func completeStudentSubmissions(ctx context.Context, c *canvas.Client, courseID, studentID int64, assignments []Assignment, bulk []Submission, recentStart, asOf, dueSoonEnd time.Time) ([]Submission, []string) {
	byAssignment := make(map[int64]Submission, len(bulk))
	for _, sub := range bulk {
		byAssignment[sub.AssignmentID] = sub
	}
	var flags []string
	for _, a := range assignments {
		if !a.Published {
			continue
		}
		sub, ok := byAssignment[a.ID]
		catalogDue, malformed := effectiveAssignmentDueStatus(a, studentID)
		if ok {
			if sub.CachedDueDatePresent {
				if sub.CachedDueDate == nil {
					catalogDue, malformed = nil, false
				} else {
					if strings.TrimSpace(*sub.CachedDueDate) == "" {
						catalogDue, malformed = nil, false
					} else if d, err := time.Parse(time.RFC3339, *sub.CachedDueDate); err != nil {
						catalogDue, malformed = nil, true
					} else {
						catalogDue, malformed = &d, false
					}
				}
			}
		}
		if malformed {
			flags = append(flags, fmt.Sprintf("assignment %d (%s): Canvas returned a malformed effective due date", a.ID, a.Name))
			continue
		}
		if ok && catalogDue != nil && (catalogDue.Before(recentStart) || catalogDue.After(asOf) && !catalogDue.Before(dueSoonEnd)) {
			continue // avoid resolving unrelated older or distant future coursework.
		}
		if catalogDue == nil && ok {
			continue // the student's returned row confirms no effective date.
		}
		if ok && sub.AssignmentVisible != nil {
			continue
		}
		// A bulk row without confirmed visibility is not usable evidence. Drop it
		// before the targeted read so a failed read cannot leak it into the report.
		delete(byAssignment, a.ID)
		var resolved Submission
		path := fmt.Sprintf("/api/v1/courses/%d/assignments/%d/submissions/%d?include[]=submission_history&include[]=submission_comments&include[]=visibility", courseID, a.ID, studentID)
		if err := c.Get(ctx, path, &resolved); err != nil {
			flags = append(flags, fmt.Sprintf("assignment %d (%s): visibility or submission evidence unavailable", a.ID, a.Name))
			continue
		}
		if resolved.AssignmentID != 0 && resolved.AssignmentID != a.ID || resolved.UserID != 0 && resolved.UserID != studentID {
			flags = append(flags, fmt.Sprintf("assignment %d (%s): targeted Canvas response identity did not match this student and assignment", a.ID, a.Name))
			continue
		}
		if resolved.AssignmentID == 0 {
			resolved.AssignmentID = a.ID
		}
		if resolved.UserID == 0 {
			resolved.UserID = studentID
		}
		if resolved.AssignmentVisible == nil {
			flags = append(flags, fmt.Sprintf("assignment %d (%s): Canvas did not confirm student visibility", a.ID, a.Name))
			no := false
			resolved.AssignmentVisible = &no
		}
		byAssignment[a.ID] = resolved
	}
	result := make([]Submission, 0, len(byAssignment))
	for _, sub := range byAssignment {
		result = append(result, sub)
	}
	return result, flags
}

// effectiveAssignmentDue returns a catalog date when the assignment has a
// date, unless an explicit student-specific undated override is present.
func effectiveAssignmentDue(a Assignment, studentID int64) *time.Time {
	due, _ := effectiveAssignmentDueStatus(a, studentID)
	return due
}

func effectiveAssignmentDueStatus(a Assignment, studentID int64) (*time.Time, bool) {
	if a.AllDates != nil {
		for _, d := range *a.AllDates {
			if d.UserID == studentID || d.StudentID == studentID {
				return parseOptionalDue(d.DueAt), malformedOptionalDue(d.DueAt)
			}
		}
	}
	for _, o := range a.Overrides {
		if containsStudent(o.StudentIDs, studentID) {
			return parseOptionalDue(o.DueAt), malformedOptionalDue(o.DueAt)
		}
	}
	return parseOptionalDue(a.DueAt), malformedOptionalDue(a.DueAt)
}

func malformedOptionalDue(raw *string) bool {
	return raw != nil && strings.TrimSpace(*raw) != "" && parseOptionalDue(raw) == nil
}

func parseOptionalDue(raw *string) *time.Time {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, *raw)
	if err != nil {
		return nil
	}
	return &t
}

func containsStudent(ids []int64, id int64) bool {
	for _, candidate := range ids {
		if candidate == id {
			return true
		}
	}
	return false
}

// correctionDeadline is 11:59:59 pm, course time, on the 14th calendar day
// after the due date's local calendar day.
func correctionDeadline(due time.Time, loc *time.Location) time.Time {
	d := due.In(loc)
	return time.Date(d.Year(), d.Month(), d.Day()+correctionWindowDays, 23, 59, 59, 0, loc)
}

func isOnline(a Assignment) bool {
	for _, t := range a.SubmissionTypes {
		if strings.HasPrefix(t, "online_") || t == "media_recording" || t == "student_annotation" || t == "discussion_topic" {
			return true
		}
	}
	return false
}

func buildStudentReport(rs *ParentReportCourse, subs []Submission, versions map[int64][]Submission, assignments map[int64]Assignment, now time.Time, loc *time.Location) {
	buildStudentReportScoped(rs, subs, versions, assignments, now, loc, now.In(loc).AddDate(0, 0, -correctionWindowDays), now.In(loc).AddDate(0, 0, dueSoonDays))
}

func buildStudentReportScoped(rs *ParentReportCourse, subs []Submission, versions map[int64][]Submission, assignments map[int64]Assignment, now time.Time, loc *time.Location, recentStart, dueSoonEnd time.Time) {
	sort.Slice(subs, func(i, j int) bool { return subs[i].AssignmentID < subs[j].AssignmentID })
	seen := map[int64]bool{}
	for _, sub := range subs {
		a, ok := assignments[sub.AssignmentID]
		if !ok || !a.Published || sub.Excused || (sub.AssignmentVisible != nil && !*sub.AssignmentVisible) || seen[a.ID] {
			continue
		}
		seen[a.ID] = true
		due := effectiveDueForStudent(sub, a, sub.UserID)
		if due == nil {
			continue
		}
		item := ParentReportItem{AssignmentID: a.ID, Title: a.Name, URL: a.HTMLURL}
		item.DueAt = due.In(loc).Format(time.RFC3339)
		submitted := sub.SubmittedAt != nil && sub.Attempt >= 1
		pastDue := due != nil && !due.After(now)
		entered := sub.EnteredScore
		if entered == nil {
			entered = sub.Score
		}
		enteredGrade := decimalPointer(entered)
		positiveGrade := enteredGrade != nil && enteredGrade.Sign() > 0
		// The teacher-entered grade resolves missing status, while Canvas's
		// current score (after any penalties) determines a recorded zero.
		current := sub.Score
		if current == nil {
			current = sub.EnteredScore
		}
		currentGrade := decimalPointer(current)
		recordedZero := currentGrade != nil && currentGrade.Sign() == 0
		gradeNotReleased := sub.PostedAtPresent && sub.PostedAt == nil && (sub.Score != nil || sub.EnteredScore != nil || sub.GradedAt != nil)
		inWindow := !due.Before(recentStart) && !due.After(now)

		if inWindow && a.GradingType != "not_graded" && !gradeNotReleased {
			switch {
			case recordedZero:
				item.Basis = "recorded_zero"
				rs.Missing = append(rs.Missing, item)
			case !positiveGrade && sub.Missing:
				item.Basis = "canvas_missing"
				rs.Missing = append(rs.Missing, item)
			case !positiveGrade && !submitted && pastDue && isOnline(a):
				item.Basis = "not_submitted_canvas_not_marked_missing"
				rs.Missing = append(rs.Missing, item)
			}
		} else if a.GradingType != "not_graded" && !gradeNotReleased && !positiveGrade && !submitted && due.After(now) && due.Before(dueSoonEnd) {
			rs.DueSoon = append(rs.DueSoon, item)
		}

		vs := versions[sub.AssignmentID]
		var hist ParentHistoryItem
		if sub.Score != nil || sub.EnteredScore != nil {
			hist = historyItem(item, sub, vs, a)
		}
		if inWindow {
			status := "no_grade"
			switch {
			case a.GradingType == "not_graded":
				status = "no_grade"
			case gradeNotReleased:
				status = "grade_not_released"
			case recordedZero:
				status = "recorded_zero"
			case item.Basis == "canvas_missing":
				status = "missing"
			case item.Basis == "not_submitted_canvas_not_marked_missing":
				status = "not_submitted"
			case enteredGrade != nil && submitted:
				status = "graded"
			case submitted && enteredGrade == nil:
				status = "awaiting_grading"
			case enteredGrade != nil:
				status = "graded"
			}
			row := ParentWorkDueItem{ParentReportItem: item, Status: status, Possible: a.Points.String(), First: hist.First, Current: hist.Current}
			if gradeNotReleased {
				row.Possible, row.First, row.Current, row.Note = "", nil, nil, "Canvas has not released this grade"
			}
			if status == "no_grade" {
				row.Note = "Canvas has no grade recorded"
			}
			rs.WorkDue = append(rs.WorkDue, row)
		}
		if inWindow && item.Basis == "" && a.GradingType != "not_graded" && !gradeNotReleased {
			c := ParentCorrection{ParentReportItem: item, CorrectionDeadline: correctionDeadline(*due, loc).Format(time.RFC3339)}
			for _, cm := range sub.Comments {
				if cm.AuthorID == sub.UserID || strings.TrimSpace(cm.Comment) == "" {
					continue
				}
				text := cm.Comment
				if len(text) > maxCommentChars {
					text = text[:maxCommentChars] + "…"
				}
				c.Comments = append(c.Comments, ParentCorrectionNote{ID: cm.ID, Attempt: cm.Attempt, CreatedAt: cm.CreatedAt, Text: text})
			}
			if a.NewQuiz {
				c.PerItemFeedback = "unverified"
			}
			// Corrections open when the student finishes, not when feedback
			// is posted; comments only help the agent point to guidance.
			if submitted || sub.Score != nil || sub.EnteredScore != nil || len(c.Comments) > 0 || c.PerItemFeedback != "" {
				rs.Corrections = append(rs.Corrections, c)
			}
		}
		if sub.Score != nil && inWindow && a.GradingType != "not_graded" && !gradeNotReleased && hasGradeHistoryChange(vs) {
			rs.History = append(rs.History, hist)
		}
	}
	sort.Slice(rs.WorkDue, func(i, j int) bool { return rs.WorkDue[i].DueAt < rs.WorkDue[j].DueAt })
	sort.Slice(rs.Corrections, func(i, j int) bool {
		return rs.Corrections[i].CorrectionDeadline < rs.Corrections[j].CorrectionDeadline
	})
	sort.Slice(rs.DueSoon, func(i, j int) bool { return rs.DueSoon[i].DueAt < rs.DueSoon[j].DueAt })
}

// Gradebook history can contain a resave of the same score. Keep the report's
// change list focused on actual changes while retaining the current score in
// the complete WorkDue inventory.
func hasGradeHistoryChange(versions []Submission) bool {
	if len(versions) < 2 {
		return true
	}
	var first *string
	for _, version := range versions {
		if version.Score == nil {
			continue
		}
		score := version.Score.String()
		if first == nil {
			first = &score
			continue
		}
		if score != *first {
			return true
		}
	}
	return false
}

func effectiveDueForStudent(sub Submission, a Assignment, studentID int64) *time.Time {
	if sub.CachedDueDatePresent && sub.CachedDueDate == nil {
		return nil // An explicit student no-date response overrides the catalog.
	}
	if sub.CachedDueDate != nil {
		if strings.TrimSpace(*sub.CachedDueDate) == "" {
			return nil // Canvas explicitly reports this student's assignment as undated.
		}
		if t, err := time.Parse(time.RFC3339, *sub.CachedDueDate); err == nil {
			return &t
		}
		return nil // malformed student-effective date is not replaced by the base date.
	}
	return effectiveAssignmentDue(a, studentID)
}

// parentSkipEligible applies the teacher's threshold only after every included
// course has its Missing/0 classification. It is advisory; the agent retains
// the teacher's send/skip/flag decision.
func parentSkipEligible(student ParentReportStudent, skipScore int) bool {
	if len(student.Courses) == 0 {
		return false
	}
	allEmpty := true
	for _, course := range student.Courses {
		if len(course.Flags) > 0 {
			return false
		}
		if course.WorkDue == nil {
			return false // a missing inventory is incomplete evidence, not an empty window.
		}
		allEmpty = allEmpty && len(course.WorkDue) == 0
	}
	if allEmpty {
		return true
	}
	threshold := domain.MustDecimal(fmt.Sprint(skipScore))
	for _, course := range student.Courses {
		if course.CourseScore == nil || len(course.Missing) > 0 {
			return false
		}
		score, err := domain.ParseDecimal(*course.CourseScore)
		if err != nil || score.Cmp(threshold) < 0 {
			return false
		}
	}
	return true
}

func recentTime(v string, now time.Time) bool {
	t, err := time.Parse(time.RFC3339, v)
	return err == nil && t.After(now.AddDate(0, 0, -correctionWindowDays)) && !t.After(now)
}

// historyItem takes first and latest earned scores only from scored grading
// versions (the gradebook history feed when present, else submission_history).
// entered_score is never used as the original assessment.
func historyItem(item ParentReportItem, sub Submission, versions []Submission, a Assignment) ParentHistoryItem {
	h := ParentHistoryItem{ParentReportItem: item, Possible: a.Points.String()}
	if sub.Score != nil {
		cur := sub.Score.String()
		h.Current = &cur
	}
	var scored []string
	hist := versions
	if len(hist) == 0 {
		hist = append([]Submission(nil), sub.History...)
		sort.SliceStable(hist, func(i, j int) bool { return hist[i].Attempt < hist[j].Attempt })
	}
	for _, e := range hist {
		if e.Score != nil {
			scored = append(scored, e.Score.String())
		}
	}
	switch {
	case len(scored) == 0:
		h.FirstUnavailable = true
		h.Note = "submission history shows no earlier scored entry"
	case len(scored) == 1:
		h.First, h.Latest = &scored[0], &scored[0]
		h.Note = "one scored entry; first and latest are the same"
	default:
		h.First, h.Latest = &scored[0], &scored[len(scored)-1]
	}
	return h
}
