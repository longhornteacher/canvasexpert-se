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
	CorrectionDays int                   `json:"correction_window_days"`
	DueSoonDays    int                   `json:"due_soon_days"`
	Students       []ParentReportStudent `json:"students"`
	Notes          []string              `json:"notes,omitempty"`
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
	Missing           []ParentReportItem  `json:"missing,omitempty"`
	Corrections       []ParentCorrection  `json:"corrections,omitempty"`
	DueSoon           []ParentReportItem  `json:"due_soon,omitempty"`
	History           []ParentHistoryItem `json:"history,omitempty"`
}

type ParentReportItem struct {
	AssignmentID int64  `json:"assignment_id"`
	Title        string `json:"title"`
	URL          string `json:"url"`
	DueAt        string `json:"due_at,omitempty"`
	// Basis explains why the work is listed: canvas_missing, or
	// not_submitted_canvas_not_marked_missing for online work past due.
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
	want := map[string]bool{}
	for _, r := range refs {
		want[r] = true
	}
	students, e := workspace.OpenStudents(s.Root)
	if e != nil {
		return "", ParentReportPacket{}, e
	}
	runID := now.UTC().Format("20060102T150405Z")
	packet := ParentReportPacket{Schema: 1, Kind: "parent-report", RunID: runID, CorrectionDays: correctionWindowDays, DueSoonDays: dueSoonDays}
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
		assignments, e := Assignments(ctx, s.Client, course.ID)
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
		for _, id := range order {
			en := byUser[id]
			rc := ParentReportCourse{Course: course.Nickname, Timezone: zone, CourseScore: en.score, CourseScoreSource: "canvas_enrollment_current_score"}
			if en.score == nil {
				rc.Flags = append(rc.Flags, "course score unavailable from Canvas")
			}
			buildStudentReport(&rc, perStudent[id], byAssignment, now, loc)
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

// courseSubmissions reads every student's submission, with history and
// comments, for the whole course in one paged request.
func courseSubmissions(ctx context.Context, c *canvas.Client, courseID int64) ([]Submission, error) {
	var result []Submission
	err := c.List(ctx, fmt.Sprintf("/api/v1/courses/%d/students/submissions?student_ids[]=all&include[]=submission_history&include[]=submission_comments&per_page=100", courseID), &result)
	return result, err
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

func buildStudentReport(rs *ParentReportCourse, subs []Submission, assignments map[int64]Assignment, now time.Time, loc *time.Location) {
	sort.Slice(subs, func(i, j int) bool { return subs[i].AssignmentID < subs[j].AssignmentID })
	for _, sub := range subs {
		a, ok := assignments[sub.AssignmentID]
		if !ok || !a.Published || sub.Excused || (sub.AssignmentVisible != nil && !*sub.AssignmentVisible) || a.GradingType == "not_graded" {
			continue
		}
		due := effectiveDue(sub, a)
		item := ParentReportItem{AssignmentID: a.ID, Title: a.Name, URL: a.HTMLURL}
		if due != nil {
			item.DueAt = due.In(loc).Format(time.RFC3339)
		}
		submitted := sub.SubmittedAt != nil && sub.Attempt >= 1
		pastDue := due != nil && !due.After(now)

		switch {
		case sub.Missing:
			item.Basis = "canvas_missing"
			rs.Missing = append(rs.Missing, item)
		case !submitted && pastDue && isOnline(a):
			item.Basis = "not_submitted_canvas_not_marked_missing"
			rs.Missing = append(rs.Missing, item)
		case !submitted && due != nil && due.After(now) && due.Before(now.AddDate(0, 0, dueSoonDays)):
			rs.DueSoon = append(rs.DueSoon, item)
		}

		inWindow := pastDue && !correctionDeadline(*due, loc).Before(now)
		changedRecently := sub.GradedAt != nil && recentTime(*sub.GradedAt, now)
		if inWindow {
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
			if len(c.Comments) > 0 || c.PerItemFeedback != "" {
				rs.Corrections = append(rs.Corrections, c)
			}
		}
		if sub.Score != nil && (inWindow || changedRecently) {
			rs.History = append(rs.History, historyItem(item, sub, a))
		}
	}
	sort.Slice(rs.Corrections, func(i, j int) bool {
		return rs.Corrections[i].CorrectionDeadline < rs.Corrections[j].CorrectionDeadline
	})
	sort.Slice(rs.DueSoon, func(i, j int) bool { return rs.DueSoon[i].DueAt < rs.DueSoon[j].DueAt })
}

func recentTime(v string, now time.Time) bool {
	t, err := time.Parse(time.RFC3339, v)
	return err == nil && t.After(now.AddDate(0, 0, -correctionWindowDays)) && !t.After(now)
}

// historyItem takes first and latest earned scores only from scored entries of
// submission_history. entered_score is never used as the original assessment.
func historyItem(item ParentReportItem, sub Submission, a Assignment) ParentHistoryItem {
	h := ParentHistoryItem{ParentReportItem: item, Possible: a.Points.String()}
	cur := sub.Score.String()
	h.Current = &cur
	var scored []string
	hist := append([]Submission(nil), sub.History...)
	sort.SliceStable(hist, func(i, j int) bool { return hist[i].Attempt < hist[j].Attempt })
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
