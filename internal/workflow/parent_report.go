package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"canvasexpertse/internal/canvas"
	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

// ParentReportPacket is the pseudonymized report the agent reads. It carries no
// contacts, real names or Goes By names. A student appears once, with one
// section per selected course, so one email can cover several courses. The
// report holds Canvas facts only; what a family email says about them is the
// playbook's choice.
type ParentReportPacket struct {
	Schema   int                   `json:"schema"`
	Kind     string                `json:"kind"`
	RunID    string                `json:"run_id"`
	Courses  []string              `json:"courses"`
	AsOf     string                `json:"as_of"`
	Scope    *ParentReportScope    `json:"scope"`
	Students []ParentReportStudent `json:"students"`
	Notes    []string              `json:"notes,omitempty"`
}

// ParentReportScope records the grading period and due-date window used to
// build a report: period start through as_of. It is provenance for this saved
// run, not a live policy switch.
type ParentReportScope struct {
	Timezone        string `json:"timezone"`
	PeriodName      string `json:"period_name"`
	PeriodStartDate string `json:"period_start_date"`
	PeriodEndDate   string `json:"period_end_date"`
	RecentStartAt   string `json:"recent_start_at"`
	RecentEndAt     string `json:"recent_end_at"`
}

type ParentReportStudent struct {
	Ref     string               `json:"student_ref"`
	Alias   string               `json:"alias"`
	Courses []ParentReportCourse `json:"courses"`
}

type ParentReportCourse struct {
	Course            string              `json:"course"`
	Timezone          string              `json:"timezone"`
	CourseScore       *string             `json:"course_score"`
	CourseScoreSource string              `json:"course_score_source"`
	Flags             []string            `json:"flags,omitempty"`
	WorkDue           []ParentWorkDueItem `json:"work_due"`
	// Upcoming lists all work due after as_of through the period end, for
	// the printed grade sheet.
	Upcoming []ParentUpcomingItem `json:"upcoming,omitempty"`
	// upcomingEnd, when set, is the exclusive end of the Upcoming list.
	upcomingEnd time.Time
}

// ParentUpcomingItem is published, visible work not yet due for this student.
type ParentUpcomingItem struct {
	ParentReportItem
	TurnedIn bool `json:"turned_in,omitempty"`
}

// ParentWorkDueItem is one required assignment whose effective due date is in
// the saved window. Status is the grade state; Submission is independent
// evidence of whether work was turned in. Neither is inferred from the other.
type ParentWorkDueItem struct {
	ParentReportItem
	// Status: graded, awaiting_grading, grade_not_released or no_grade.
	Status   string  `json:"status"`
	Possible string  `json:"points_possible,omitempty"`
	Current  *string `json:"canvas_score,omitempty"`
	// Percent is canvas_score / points_possible × 100, rounded to two places
	// for reading; selection compares the exact ratio.
	Percent        *string `json:"percent,omitempty"`
	PointsDeducted *string `json:"points_deducted,omitempty"`
	// Submission: turned_in, not_turned_in, no_online_submission or unknown.
	Submission string `json:"submission"`
}

type ParentReportItem struct {
	AssignmentID int64  `json:"assignment_id"`
	Title        string `json:"title"`
	URL          string `json:"url"`
	DueAt        string `json:"due_at,omitempty"`
}

// ParentReport reads current Canvas data for the configured courses and writes
// a pseudonymized report under courses/<first nickname>/parent-updates/<run-id>/.
// refs, when given, limits the report to those student refs.
func (s *Service) ParentReport(ctx context.Context, nicknames []string, refs []string, now time.Time) (string, ParentReportPacket, error) {
	return s.ParentReportWithScope(ctx, nicknames, refs, now, true)
}

// ParentReportWithScope is kept for callers that pass the former full-period
// switch. Every report now covers the whole current grading period through now.
func (s *Service) ParentReportWithScope(ctx context.Context, nicknames []string, refs []string, now time.Time, _ bool) (string, ParentReportPacket, error) {
	if len(nicknames) == 0 {
		return "", ParentReportPacket{}, fmt.Errorf("at least one course is required")
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
	scope, periodStart, upcomingEnd, e := makeParentReportScope(calendar, period, now)
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
	packet := ParentReportPacket{Schema: 2, Kind: "parent-report", RunID: runID, Scope: scope}
	byRef := map[string]*ParentReportStudent{}
	matched := map[string]bool{}
	for _, nickname := range nicknames {
		course, e := s.Course(nickname)
		if e != nil {
			return "", ParentReportPacket{}, e
		}
		scoped, e := s.ForCourse(ctx, course)
		if e != nil {
			return "", ParentReportPacket{}, e
		}
		s := scoped
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
		resolver := &parentSubmissionResolver{client: s.Client, courseID: course.ID, batches: map[int64]map[int64]Submission{}}
		courseWorkCount := 0
		verifiedInWindow := false
		for _, id := range order {
			en := byUser[id]
			rc := ParentReportCourse{Course: course.Nickname, Timezone: zone, CourseScore: en.score, CourseScoreSource: "canvas_enrollment_current_score", WorkDue: []ParentWorkDueItem{}}
			studentSubs, incomplete := completeStudentSubmissionsWithResolver(ctx, resolver, id, assignments, perStudent[id], periodStart, now, upcomingEnd)
			rc.Flags = append(rc.Flags, incomplete...)
			for _, sub := range studentSubs {
				a, ok := byAssignment[sub.AssignmentID]
				due := effectiveDueForStudent(sub, a, id)
				if ok && a.Published && a.GradingType != "not_graded" && !sub.Excused && sub.AssignmentVisible != nil && *sub.AssignmentVisible && due != nil && !due.Before(periodStart) && !due.After(now) {
					verifiedInWindow = true
				}
			}
			rc.upcomingEnd = upcomingEnd
			buildStudentReport(&rc, studentSubs, byAssignment, now, loc, periodStart)
			courseWorkCount += len(rc.WorkDue)
			rs := byRef[en.st.Ref]
			if rs == nil {
				rs = &ParentReportStudent{Ref: en.st.Ref, Alias: en.st.Alias}
				byRef[en.st.Ref] = rs
			}
			rs.Courses = append(rs.Courses, rc)
		}
		if verifiedInWindow && courseWorkCount == 0 {
			return "", ParentReportPacket{}, fmt.Errorf("course %s has confirmed student-visible work due in the report window but no student inventory", course.Nickname)
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

// makeParentReportScope returns the saved scope, the period start (the window
// start) and the exclusive period end (the end of the upcoming list).
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
	scope := &ParentReportScope{Timezone: calendar.Timezone, PeriodName: period.Name, PeriodStartDate: period.StartDate, PeriodEndDate: period.EndDate,
		RecentStartAt: periodStart.Format(time.RFC3339), RecentEndAt: asOf.Format(time.RFC3339)}
	return scope, periodStart, periodEnd.AddDate(0, 0, 1), nil
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

// completeStudentSubmissions joins the catalog to the bulk submission feed.
// Missing rows and rows without visibility are resolved with the existing
// assignment/student read before they can affect a family report.
type parentSubmissionResolver struct {
	client   *canvas.Client
	courseID int64
	batches  map[int64]map[int64]Submission
}

func completeStudentSubmissions(ctx context.Context, c *canvas.Client, courseID, studentID int64, assignments []Assignment, bulk []Submission, recentStart, asOf, dueSoonEnd time.Time) ([]Submission, []string) {
	return completeStudentSubmissionsWithResolver(ctx, &parentSubmissionResolver{client: c, courseID: courseID, batches: map[int64]map[int64]Submission{}}, studentID, assignments, bulk, recentStart, asOf, dueSoonEnd)
}

func completeStudentSubmissionsWithResolver(ctx context.Context, resolver *parentSubmissionResolver, studentID int64, assignments []Assignment, bulk []Submission, recentStart, asOf, dueSoonEnd time.Time) ([]Submission, []string) {
	c, courseID := resolver.client, resolver.courseID
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
		if ok && sub.AssignmentVisible != nil && *sub.AssignmentVisible {
			continue
		}
		// Canvas's course-wide feed can report false visibility for every
		// student even when the assignment-scoped feed reports true, and it
		// omits rows. Resolve both from one assignment-scoped read shared by
		// every student, instead of one read per student; a student absent
		// from that complete list is not assigned the work. Only a failed
		// list read falls back to the per-student read below.
		batch, loaded := resolver.batches[a.ID]
		if !loaded {
			var rows []Submission
			path := fmt.Sprintf("/api/v1/courses/%d/assignments/%d/submissions?include[]=submission_history&include[]=submission_comments&include[]=visibility&per_page=100", courseID, a.ID)
			if err := c.List(ctx, path, &rows); err == nil {
				batch = map[int64]Submission{}
				for _, row := range rows {
					if row.UserID > 0 && (row.AssignmentID == 0 || row.AssignmentID == a.ID) {
						if row.AssignmentID == 0 {
							row.AssignmentID = a.ID
						}
						batch[row.UserID] = row
					}
				}
			}
			resolver.batches[a.ID] = batch // nil records a failed read
		}
		if batch != nil {
			resolved, found := batch[studentID]
			switch {
			case !found:
				delete(byAssignment, a.ID)
				continue
			case resolved.AssignmentVisible != nil && ok:
				sub.AssignmentVisible = resolved.AssignmentVisible
				byAssignment[a.ID] = sub
				continue
			case resolved.AssignmentVisible != nil:
				byAssignment[a.ID] = resolved
				continue
			}
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

func isOnline(a Assignment) bool {
	for _, t := range a.SubmissionTypes {
		if strings.HasPrefix(t, "online_") || t == "media_recording" || t == "student_annotation" || t == "discussion_topic" {
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

// buildStudentReport records Canvas facts for each published, visible,
// non-excused assignment: work due from windowStart through now goes in
// WorkDue, later work through the period end in Upcoming.
func buildStudentReport(rs *ParentReportCourse, subs []Submission, assignments map[int64]Assignment, now time.Time, loc *time.Location, windowStart time.Time) {
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
		item := ParentReportItem{AssignmentID: a.ID, Title: a.Name, URL: a.HTMLURL, DueAt: due.In(loc).Format(time.RFC3339)}
		submitted := sub.SubmittedAt != nil && sub.Attempt >= 1
		if !rs.upcomingEnd.IsZero() && due.After(now) && due.Before(rs.upcomingEnd) {
			rs.Upcoming = append(rs.Upcoming, ParentUpcomingItem{ParentReportItem: item, TurnedIn: submitted})
		}
		if due.Before(windowStart) || due.After(now) {
			continue
		}
		rs.WorkDue = append(rs.WorkDue, workDueRow(item, sub, a, submitted))
	}
	sort.SliceStable(rs.WorkDue, func(i, j int) bool {
		a, _ := time.Parse(time.RFC3339, rs.WorkDue[i].DueAt)
		b, _ := time.Parse(time.RFC3339, rs.WorkDue[j].DueAt)
		if !a.Equal(b) {
			return a.Before(b)
		}
		return rs.WorkDue[i].Title < rs.WorkDue[j].Title
	})
	sort.SliceStable(rs.Upcoming, func(i, j int) bool { return rs.Upcoming[i].DueAt < rs.Upcoming[j].DueAt })
}

// workDueRow states the grade and the submission evidence separately. A
// recorded zero is a graded score, not proof that nothing was turned in, and
// an unreleased or absent grade is never a zero.
func workDueRow(item ParentReportItem, sub Submission, a Assignment, submitted bool) ParentWorkDueItem {
	row := ParentWorkDueItem{ParentReportItem: item, Possible: a.Points.String()}
	// Canvas's current score (after any penalties) is the grade families see.
	current := sub.Score
	if current == nil {
		current = sub.EnteredScore
	}
	grade := decimalPointer(current)
	released := !(sub.PostedAtPresent && sub.PostedAt == nil)
	// A positive teacher-entered grade resolves a stale Canvas missing flag,
	// even when a penalty brought the current score to zero.
	entered := decimalPointer(sub.EnteredScore)
	positive := grade != nil && grade.Sign() > 0 || entered != nil && entered.Sign() > 0
	switch {
	case a.GradingType == "not_graded":
		row.Status = "no_grade"
	case !released && (grade != nil || sub.GradedAt != nil):
		row.Status, row.Possible = "grade_not_released", ""
	case grade != nil:
		row.Status = "graded"
		v := current.String()
		row.Current = &v
		if sub.PointsDeducted != nil {
			d := sub.PointsDeducted.String()
			row.PointsDeducted = &d
		}
		row.Percent = percentOf(*grade, a.Points.String())
	case submitted:
		row.Status = "awaiting_grading"
	default:
		row.Status = "no_grade"
	}
	switch {
	case submitted:
		row.Submission = "turned_in"
	case sub.Missing && !positive:
		row.Submission = "not_turned_in"
	case !isOnline(a):
		row.Submission = "no_online_submission"
	case !positive:
		// Online work past due with no Canvas submission.
		row.Submission = "not_turned_in"
	default:
		// A positive grade without an online submission: the teacher may have
		// accepted it another way.
		row.Submission = "unknown"
	}
	return row
}

// percentOf returns score/possible × 100 to two places, or nil when points
// possible is absent or zero.
func percentOf(score domain.Decimal, possible string) *string {
	p, ok := percentRat(score.String(), possible)
	if !ok {
		return nil
	}
	v := domain.RoundRat(p, 2).String() // RoundRat already drops trailing zeros
	return &v
}

// percentRat is the exact percentage, used for threshold comparisons.
func percentRat(score, possible string) (*big.Rat, bool) {
	s, ok1 := new(big.Rat).SetString(strings.TrimSpace(score))
	p, ok2 := new(big.Rat).SetString(strings.TrimSpace(possible))
	if !ok1 || !ok2 || p.Sign() <= 0 {
		return nil, false
	}
	return new(big.Rat).Mul(new(big.Rat).Quo(s, p), big.NewRat(100, 1)), true
}
