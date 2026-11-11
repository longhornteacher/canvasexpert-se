package workflow

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"time"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

// A tutorial retake is assigned to each student who failed the original, with
// their own deadline. Everything is read from Canvas, so a rerun on any PC does
// only what is unfinished.

const retakeOverrideTitle = "Tutorial retake"

type RetakeAssignRow struct {
	StudentRef string `json:"student_ref"`
	Alias      string `json:"alias"`
	Status     string `json:"status"` // planned, assigned, complete, held, skipped
	Reason     string `json:"reason,omitempty"`
	Entered    string `json:"entered,omitempty"`
	Deadline   string `json:"deadline,omitempty"`
	Steps      string `json:"steps,omitempty"`
}

type RetakeAssignReport struct {
	Course         string            `json:"course"`
	Preview        bool              `json:"preview"`
	OriginalID     int64             `json:"original_id"`
	OriginalName   string            `json:"original_name,omitempty"`
	RetakeID       int64             `json:"retake_id,omitempty"`
	Published      bool              `json:"published"`
	Rows           []RetakeAssignRow `json:"students"`
	Accommodations string            `json:"accommodations,omitempty"`
	Notes          []string          `json:"notes,omitempty"`
}

// retakeExtraDays is a student's extra time on the retake window itself. A
// saved percent is taken of the window, rounded up; otherwise the saved days,
// otherwise the general allowance.
func retakeExtraDays(st workspace.Student, windowDays, generalDays int) (int, string) {
	if !st.ExtraTime {
		return 0, ""
	}
	switch {
	case st.ExtraTimePercent > 0:
		n := (windowDays*st.ExtraTimePercent + 99) / 100
		return n, fmt.Sprintf("%d%% extra time on a %d-day window", st.ExtraTimePercent, windowDays)
	case st.ExtraTimeDays > 0:
		return st.ExtraTimeDays, fmt.Sprintf("%d saved extra days", st.ExtraTimeDays)
	}
	return generalDays, "general extra-time allowance"
}

// enteredRat is the entered grade before any late deduction. Canvas's
// entered_score is that; without it a deducted score cannot be undone safely.
func enteredRat(sub Submission) (*big.Rat, bool) {
	n := sub.EnteredScore
	if n == nil {
		if sub.Score == nil || (sub.PointsDeducted != nil && sub.PointsDeducted.String() != "0") {
			return nil, false
		}
		n = sub.Score
	}
	return new(big.Rat).SetString(n.String())
}

// RetakeAssign plans and, unless preview, assigns the tutorial retake of an
// original assignment to every student who failed it.
func (s *Service) RetakeAssign(ctx context.Context, nickname string, originalID int64, preview bool) (RetakeAssignReport, error) {
	report := RetakeAssignReport{Preview: preview, OriginalID: originalID}
	course, err := s.Course(nickname)
	if err != nil {
		return report, err
	}
	report.Course = course.Nickname
	outer := s
	if s, err = s.ForCourse(ctx, course); err != nil {
		return report, err
	}
	if _, owned, e := outer.Owned(course); e != nil {
		return report, e
	} else if owned {
		return report, fmt.Errorf("this course is team-taught (owner set); tutorial retakes are not supported there yet, so nothing was assigned")
	}
	policy, err := LoadRetakePolicy(s.Root)
	if err != nil {
		return report, err
	}
	if policy == nil {
		return report, fmt.Errorf("no retake policy is set: add a retakes block (cap_percent, school_days) to context/grading.md")
	}
	if err = policy.Validate(); err != nil {
		return report, err
	}
	cal, err := workspace.ReadSchoolCalendar(s.Root)
	if err == nil {
		err = cal.Validate()
	}
	if err != nil {
		return report, fmt.Errorf("the school calendar (context/school-calendar.md) is needed to set retake deadlines: %w", err)
	}
	loc, _ := time.LoadLocation(cal.Timezone)
	late, _ := s.lateWorkFor(course)
	general := 0
	if late != nil {
		general = late.ExtraTimeSchoolDays
	}

	original, err := GetAssignment(ctx, s.Client, course.ID, originalID)
	if err != nil {
		return report, err
	}
	report.OriginalName = original.Name
	if isRetake(original) {
		return report, fmt.Errorf("assignment %d is itself a tutorial retake; name the original Major", originalID)
	}
	if original.GroupCategoryID != nil && *original.GroupCategoryID > 0 && (original.GradeGroupStudentsIndividually == nil || !*original.GradeGroupStudentsIndividually) {
		return report, fmt.Errorf("%q is graded as a shared group grade; one student's retake must not raise the whole group, so enter retake credit by hand", original.Name)
	}
	all, err := sweepAssignments(ctx, s, course.ID)
	if err != nil {
		return report, err
	}
	paired, err := retakeFor(all, original)
	if err != nil {
		return report, err
	}
	report.RetakeID = paired.ID
	retake, err := GetAssignment(ctx, s.Client, course.ID, paired.ID)
	if err != nil {
		return report, err
	}
	// Re-check the retake's protections from a fresh read before any student sees it.
	if err = s.ensureRetakeProtected(ctx, course.ID, retake, preview, &report); err != nil {
		return report, err
	}

	points, ok := new(big.Rat).SetString(original.Points.String())
	if !ok || points.Sign() <= 0 {
		return report, fmt.Errorf("%q has no point value to measure a failing grade against", original.Name)
	}
	capPoints := new(big.Rat).Mul(points, big.NewRat(int64(policy.CapPercent), 100))

	students, err := workspace.OpenStudents(s.Root)
	if err != nil {
		return report, err
	}
	enrollments, err := StudentEnrollments(ctx, s.Client, course.ID)
	if err != nil {
		return report, err
	}
	byUser := map[int64]workspace.Student{}
	for _, en := range enrollments {
		id := en.UserID
		if id == 0 {
			id = en.User.ID
		}
		if id <= 0 || en.State != "active" {
			continue
		}
		st, e := students.Upsert(s.Client.Origin(), id, en.User.Name, en.User.ShortName, en.User.SISID, en.User.Login)
		if e != nil {
			return report, e
		}
		byUser[id] = st
	}
	if err = students.Save(); err != nil {
		return report, err
	}
	subs, err := Submissions(ctx, s.Client, course.ID, original.ID, true)
	if err != nil {
		return report, err
	}
	var overrides []AssignmentOverrideLive
	if err = s.Client.List(ctx, fmt.Sprintf("/api/v1/courses/%d/assignments/%d/overrides?per_page=100", course.ID, retake.ID), &overrides); err != nil {
		return report, err
	}
	hasOverride := map[int64]bool{}
	for _, o := range overrides {
		for _, id := range o.StudentIDs {
			hasOverride[id] = true
		}
	}

	type chosen struct {
		row      int
		student  workspace.Student
		sub      Submission
		deadline time.Time
	}
	var picks []chosen
	sort.Slice(subs, func(i, j int) bool { return subs[i].UserID < subs[j].UserID })
	for _, sub := range subs {
		st, ok := byUser[sub.UserID]
		if !ok {
			continue
		}
		alias := st.Alias
		if !s.Config.Aliases {
			alias = st.Name
		}
		row := RetakeAssignRow{StudentRef: st.Ref, Alias: alias}
		finish := func(status, reason string) {
			row.Status, row.Reason = status, reason
			report.Rows = append(report.Rows, row)
		}
		entered, graded := enteredRat(sub)
		switch {
		case sub.Excused:
			continue
		case !graded || sub.State != "graded":
			finish("skipped", "no graded attempt on the original")
			continue
		case entered.Cmp(capPoints) >= 0:
			continue
		case sub.PostedAt == nil || *sub.PostedAt == "":
			finish("skipped", "the original's grade is not posted yet")
			continue
		}
		row.Entered = shortRat(entered) + "/" + shortRat(points)
		posted, e := time.Parse(time.RFC3339, *sub.PostedAt)
		if e != nil {
			finish("held", "the posted date could not be read")
			continue
		}
		local := posted.In(loc)
		base := time.Date(local.Year(), local.Month(), local.Day(), 23, 59, 0, 0, loc)
		extra, basis := retakeExtraDays(st, policy.SchoolDays, general)
		deadline, e := cal.AddSchoolDays(base, policy.SchoolDays+extra)
		if e != nil {
			finish("held", "the school calendar does not cover this student's retake window")
			continue
		}
		row.Deadline = deadline.Format("2006-01-02 15:04")
		if basis != "" {
			row.Steps = "extra time: " + basis
		}
		finish("planned", "")
		picks = append(picks, chosen{row: len(report.Rows) - 1, student: st, sub: sub, deadline: deadline})
	}
	if preview {
		return report, nil
	}

	assigned := 0
	for _, pk := range picks {
		row := &report.Rows[pk.row]
		var done []string
		if !hasOverride[pk.student.UserID] {
			fields := map[string]any{
				"student_ids": []int64{pk.student.UserID}, "title": retakeOverrideTitle,
				"due_at": pk.deadline.UTC().Format(time.RFC3339), "lock_at": pk.deadline.UTC().Format(time.RFC3339),
			}
			if e := s.Client.Write(ctx, "POST", fmt.Sprintf("/api/v1/courses/%d/assignments/%d/overrides", course.ID, retake.ID), map[string]any{"assignment_override": fields}, nil); e != nil {
				row.Status, row.Reason = "held", "Canvas did not accept the retake date: "+e.Error()
				continue
			}
			done = append(done, "assigned")
		}
		assigned++
		text := "Tutorial retake open until " + pk.deadline.Format("01/02") + "."
		if !hasComment(pk.sub, text) {
			attempt := pk.sub.Attempt
			if attempt < 1 {
				attempt = 1
			}
			path := fmt.Sprintf("/api/v1/courses/%d/assignments/%d/submissions/%d", course.ID, original.ID, pk.student.UserID)
			if e := s.Client.Write(ctx, "PUT", path, map[string]any{"comment": map[string]any{"text_comment": text, "attempt": attempt}}, nil); e != nil {
				row.Status, row.Reason = "held", "assigned, but the comment on the original was not posted: "+e.Error()
				continue
			}
			done = append(done, "comment")
		}
		row.Status = "assigned"
		if len(done) == 0 {
			row.Status = "complete"
		}
		row.Steps = strings.Trim(strings.Join(done, ", ")+"; "+row.Steps, "; ")
	}
	if assigned > 0 {
		report.Accommodations = "saved quiz accommodations applied"
		acc, e := s.SetQuizAccommodations(ctx, course.Nickname, retake.ID, QuizAccommodationRequest{FromSaved: true}, false)
		if e != nil {
			report.Accommodations = "NOT applied: " + e.Error()
			report.Notes = append(report.Notes, "Saved accommodations failed; do not give assigned students the access code until that is fixed.")
		} else if acc.Failed > 0 || acc.Uncertain > 0 {
			report.Accommodations = fmt.Sprintf("%d applied, %d failed, %d uncertain", acc.Completed, acc.Failed, acc.Uncertain)
			report.Notes = append(report.Notes, "Some accommodations did not land; do not give those students the access code until it is fixed (inspect Moderate).")
		}
	}
	report.Published = retake.Published
	if assigned > 0 && !report.Published {
		var a Assignment
		if e := s.Client.Write(ctx, "PUT", fmt.Sprintf("/api/v1/courses/%d/assignments/%d", course.ID, retake.ID), map[string]any{"assignment": map[string]any{"published": true}}, &a); e != nil {
			return report, fmt.Errorf("students were assigned but publishing the retake failed: %w", e)
		}
		report.Published = true
	}
	return report, nil
}

func shortRat(r *big.Rat) string {
	d := domain.RoundRat(r, 2).String()
	if strings.Contains(d, ".") {
		d = strings.TrimRight(strings.TrimRight(d, "0"), ".")
	}
	return d
}

func hasComment(sub Submission, text string) bool {
	for _, c := range sub.Comments {
		if strings.TrimSpace(c.Comment) == text {
			return true
		}
	}
	return false
}

// ensureRetakeProtected checks, from a fresh read, that the retake is visible
// only to overrides, requires an access code, does not count and is not sent to
// the SIS. Missing assignment settings are restored; a missing access code is
// not (the Engine does not know it), so it stops before any override or publish.
func (s *Service) ensureRetakeProtected(ctx context.Context, courseID int64, retake Assignment, preview bool, report *RetakeAssignReport) error {
	var quiz struct {
		Settings struct {
			Require bool `json:"require_student_access_code"`
		} `json:"quiz_settings"`
	}
	if err := s.Client.Get(ctx, fmt.Sprintf("/api/quiz/v1/courses/%d/quizzes/%d", courseID, retake.ID), &quiz); err != nil {
		return err
	}
	if !quiz.Settings.Require {
		return fmt.Errorf("the retake (assignment %d) does not require an access code; set one in Canvas before anything is assigned", retake.ID)
	}
	fix := map[string]any{}
	if retake.OnlyVisibleToOverrides == nil || !*retake.OnlyVisibleToOverrides {
		fix["only_visible_to_overrides"] = true
	}
	if retake.OmitFromFinalGrade == nil || !*retake.OmitFromFinalGrade {
		fix["omit_from_final_grade"] = true
	}
	if retake.PostToSIS == nil || *retake.PostToSIS {
		fix["post_to_sis"] = false
	}
	if len(fix) == 0 {
		return nil
	}
	if preview {
		report.Notes = append(report.Notes, "The retake's protections are incomplete; assigning will restore them first.")
		return nil
	}
	if err := s.Client.Write(ctx, "PUT", fmt.Sprintf("/api/v1/courses/%d/assignments/%d", courseID, retake.ID), map[string]any{"assignment": fix}, nil); err != nil {
		return fmt.Errorf("the retake's protections could not be restored, so nothing was assigned or published: %w", err)
	}
	return nil
}
