package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"canvasexpertse/internal/workspace"
)

// WorkHabitsPacket is the pseudonymized turn-in record the agent reads: per
// student, per week, which due items were on time, late, missing and so on.
// It holds Canvas facts only. What a pattern means (a push, a skill gap, a
// team placement) is the playbook's choice.
type WorkHabitsPacket struct {
	Schema   int                 `json:"schema"`
	Kind     string              `json:"kind"`
	RunID    string              `json:"run_id"`
	Course   string              `json:"course"`
	Timezone string              `json:"timezone"`
	From     string              `json:"from"`
	To       string              `json:"to"`
	AsOf     string              `json:"as_of"`
	Students []WorkHabitsStudent `json:"students"`
	Notes    []string            `json:"notes,omitempty"`
	Flags    []string            `json:"flags,omitempty"`
}

type WorkHabitsStudent struct {
	Ref   string           `json:"student_ref"`
	Alias string           `json:"alias"`
	Weeks []WorkHabitsWeek `json:"weeks"`
	// BeforeEnrollment lists items due before this student's enrollment began.
	// They are not counted anywhere: Canvas's own analytics count them as
	// missing, which makes new students look disengaged.
	BeforeEnrollment []WorkHabitItem `json:"before_enrollment,omitempty"`
}

// WorkHabitsWeek covers one Monday-to-Sunday week by effective due date, in
// the course timezone. Due is on_time + late + missing + no_online_submission
// + graded_no_submission; excused work is listed but not counted as due.
type WorkHabitsWeek struct {
	WeekStart string `json:"week_start"`
	Due       int    `json:"due"`
	OnTime    int    `json:"on_time"`
	Late      int    `json:"late"`
	Missing   int    `json:"missing"`
	Excused   int    `json:"excused"`
	// NoOnlineSubmission: work with no online turn-in to check (paper, in class).
	NoOnlineSubmission int `json:"no_online_submission"`
	// GradedNoSubmission: a positive grade with no Canvas turn-in on online
	// work; the teacher may have accepted it another way.
	GradedNoSubmission int `json:"graded_no_submission"`
	Items              struct {
		OnTime             []WorkHabitItem `json:"on_time,omitempty"`
		Late               []WorkHabitItem `json:"late,omitempty"`
		Missing            []WorkHabitItem `json:"missing,omitempty"`
		Excused            []WorkHabitItem `json:"excused,omitempty"`
		NoOnlineSubmission []WorkHabitItem `json:"no_online_submission,omitempty"`
		GradedNoSubmission []WorkHabitItem `json:"graded_no_submission,omitempty"`
	} `json:"items"`
}

type WorkHabitItem struct {
	AssignmentID int64  `json:"assignment_id"`
	Title        string `json:"title"`
	DueAt        string `json:"due_at"`
}

// WorkHabits reads current Canvas data for one course and writes a
// pseudonymized record under courses/<course>/work-habits/<run-id>/habits.json.
// from and to are school dates in the course timezone (inclusive); zero values
// default to the four weeks ending now. Work not yet due at now is never
// counted. refs, when given, limits the record to those student refs.
func (s *Service) WorkHabits(ctx context.Context, nickname string, from, to time.Time, refs []string, now time.Time) (string, WorkHabitsPacket, error) {
	packet, e := s.workHabitsPacket(ctx, nickname, from, to, refs, now)
	if e != nil {
		return "", WorkHabitsPacket{}, e
	}
	rel := filepath.Join("courses", packet.Course, "work-habits", packet.RunID, "habits.json")
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

// workHabitsPacket does the Canvas reads and classification without saving a
// file, so other tallies (STAD standings) can reuse the same facts.
func (s *Service) workHabitsPacket(ctx context.Context, nickname string, from, to time.Time, refs []string, now time.Time) (WorkHabitsPacket, error) {
	course, e := s.Course(nickname)
	if e != nil {
		return WorkHabitsPacket{}, e
	}
	scoped, e := s.ForCourse(ctx, course)
	if e != nil {
		return WorkHabitsPacket{}, e
	}
	s = scoped
	zone := course.Timezone
	if zone == "" {
		zone = "America/Chicago"
	}
	loc, e := time.LoadLocation(zone)
	if e != nil {
		return WorkHabitsPacket{}, fmt.Errorf("unknown course timezone %q", zone)
	}
	nowLocal := now.In(loc)
	if to.IsZero() {
		to = nowLocal
	} else {
		to = time.Date(to.Year(), to.Month(), to.Day(), 23, 59, 59, 0, loc)
	}
	if to.After(now) {
		to = now
	}
	if from.IsZero() {
		from = nowLocal.AddDate(0, 0, -28)
	}
	from = time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, loc)
	if to.Before(from) {
		return WorkHabitsPacket{}, fmt.Errorf("the end date is before the start date")
	}

	want := map[string]bool{}
	for _, r := range refs {
		want[r] = true
	}
	students, e := workspace.OpenStudents(s.Root)
	if e != nil {
		return WorkHabitsPacket{}, e
	}
	enrollments, e := StudentEnrollments(ctx, s.Client, course.ID)
	if e != nil {
		return WorkHabitsPacket{}, e
	}
	type enrolled struct {
		st      workspace.Student
		started *time.Time
	}
	byUser := map[int64]*enrolled{}
	var order []int64
	matched := map[string]bool{}
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
			return WorkHabitsPacket{}, e
		}
		if len(want) > 0 && !want[st.Ref] {
			continue
		}
		matched[st.Ref] = true
		var started *time.Time
		if t, err := time.Parse(time.RFC3339, en.CreatedAt); err == nil {
			started = &t
		}
		if prev := byUser[id]; prev == nil {
			order = append(order, id)
		} else if prev.started != nil && (started == nil || prev.started.Before(*started)) {
			started = prev.started // several enrollments: the earliest one counts
		}
		byUser[id] = &enrolled{st, started}
	}
	if e = students.Save(); e != nil {
		return WorkHabitsPacket{}, e
	}
	for r := range want {
		if !matched[r] {
			return WorkHabitsPacket{}, fmt.Errorf("student %s is not an active student in %s", r, course.Nickname)
		}
	}

	assignments, e := parentAssignments(ctx, s.Client, course.ID)
	if e != nil {
		return WorkHabitsPacket{}, e
	}
	byAssignment := map[int64]Assignment{}
	for _, a := range assignments {
		byAssignment[a.ID] = a
	}
	subs, e := courseSubmissions(ctx, s.Client, course.ID)
	if e != nil {
		return WorkHabitsPacket{}, e
	}
	perStudent := map[int64][]Submission{}
	for _, sub := range subs {
		perStudent[sub.UserID] = append(perStudent[sub.UserID], sub)
	}
	resolver := &parentSubmissionResolver{client: s.Client, courseID: course.ID, batches: map[int64]map[int64]Submission{}}

	packet := WorkHabitsPacket{Schema: 1, Kind: "work-habits", RunID: now.UTC().Format("20060102T150405Z"), Course: course.Nickname, Timezone: zone,
		From: from.Format("2006-01-02"), To: to.Format("2006-01-02"), AsOf: now.UTC().Format(time.RFC3339), Students: []WorkHabitsStudent{}}
	for _, id := range order {
		en := byUser[id]
		studentSubs, incomplete := completeStudentSubmissionsWithResolver(ctx, resolver, id, assignments, perStudent[id], from, to, to)
		packet.Flags = append(packet.Flags, incomplete...)
		ws := buildWorkHabits(en.st.Ref, en.st.Alias, studentSubs, byAssignment, en.started, from, to, loc)
		packet.Students = append(packet.Students, ws)
	}
	sort.Slice(packet.Students, func(i, j int) bool { return packet.Students[i].Alias < packet.Students[j].Alias })
	if len(packet.Students) == 0 {
		packet.Notes = append(packet.Notes, "no active students matched")
	}

	return packet, nil
}

// buildWorkHabits classifies each published, visible assignment whose effective
// due date falls in [from, to] for one student. The selection matches the
// parent report (effective due dates, visibility, published); excused work is
// kept so it can be listed.
func buildWorkHabits(ref, alias string, subs []Submission, assignments map[int64]Assignment, enrolledAt *time.Time, from, to time.Time, loc *time.Location) WorkHabitsStudent {
	out := WorkHabitsStudent{Ref: ref, Alias: alias, Weeks: []WorkHabitsWeek{}}
	sort.Slice(subs, func(i, j int) bool { return subs[i].AssignmentID < subs[j].AssignmentID })
	weeks := map[string]*WorkHabitsWeek{}
	seen := map[int64]bool{}
	for _, sub := range subs {
		a, ok := assignments[sub.AssignmentID]
		if !ok || !a.Published || a.GradingType == "not_graded" || (sub.AssignmentVisible != nil && !*sub.AssignmentVisible) || seen[a.ID] {
			continue
		}
		seen[a.ID] = true
		due := effectiveDueForStudent(sub, a, sub.UserID)
		if due == nil || due.Before(from) || due.After(to) {
			continue
		}
		item := WorkHabitItem{AssignmentID: a.ID, Title: a.Name, DueAt: due.In(loc).Format(time.RFC3339)}
		if enrolledAt != nil && due.Before(*enrolledAt) {
			out.BeforeEnrollment = append(out.BeforeEnrollment, item)
			continue
		}
		d := due.In(loc)
		monday := time.Date(d.Year(), d.Month(), d.Day()-(int(d.Weekday())+6)%7, 0, 0, 0, 0, loc).Format("2006-01-02")
		w := weeks[monday]
		if w == nil {
			w = &WorkHabitsWeek{WeekStart: monday}
			weeks[monday] = w
		}
		if sub.Excused {
			w.Excused++
			w.Items.Excused = append(w.Items.Excused, item)
			continue
		}
		submitted := sub.SubmittedAt != nil && sub.Attempt >= 1
		w.Due++
		switch row := workDueRow(ParentReportItem{AssignmentID: a.ID, Title: a.Name, DueAt: item.DueAt}, sub, a, submitted); {
		case row.Submission == "turned_in" && (sub.Late || sub.LatePolicyStatus != nil && *sub.LatePolicyStatus == "late"):
			w.Late++
			w.Items.Late = append(w.Items.Late, item)
		case row.Submission == "turned_in":
			w.OnTime++
			w.Items.OnTime = append(w.Items.OnTime, item)
		case row.Submission == "not_turned_in":
			w.Missing++
			w.Items.Missing = append(w.Items.Missing, item)
		case row.Submission == "no_online_submission":
			w.NoOnlineSubmission++
			w.Items.NoOnlineSubmission = append(w.Items.NoOnlineSubmission, item)
		default:
			w.GradedNoSubmission++
			w.Items.GradedNoSubmission = append(w.Items.GradedNoSubmission, item)
		}
	}
	for _, w := range weeks {
		out.Weeks = append(out.Weeks, *w)
	}
	sort.Slice(out.Weeks, func(i, j int) bool { return out.Weeks[i].WeekStart < out.Weeks[j].WeekStart })
	sort.Slice(out.BeforeEnrollment, func(i, j int) bool { return out.BeforeEnrollment[i].DueAt < out.BeforeEnrollment[j].DueAt })
	return out
}
