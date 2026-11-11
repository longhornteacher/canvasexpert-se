package workflow

import (
	"context"
	"fmt"
	"sort"
	"time"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

// Personal dates give one student their own Canvas due and lock dates on an
// assignment, as a single-student Canvas override. Three things earn one:
// saved extra time, an absence on the due date, and (after the due date)
// on-time work that may still be corrected once the assignment has closed to
// late work. Dates only ever move later.

const personalDatesTitle = "Personal dates"

// personalDatesLookBack is how far back a course-wide run reaches for
// assignments that may still need reopening.
const personalDatesLookBack = 21 * 24 * time.Hour

type PersonalDatesReport struct {
	Course      string                    `json:"course"`
	Preview     bool                      `json:"preview"`
	Planned     int                       `json:"planned"`
	Written     int                       `json:"written"`
	Unchanged   int                       `json:"unchanged"`
	Held        int                       `json:"held"`
	Assignments []PersonalDatesAssignment `json:"assignments,omitempty"`
	Skipped     []string                  `json:"skipped,omitempty"`
}

type PersonalDatesAssignment struct {
	AssignmentID int64              `json:"assignment_id"`
	Title        string             `json:"title"`
	DueAt        string             `json:"due_at,omitempty"`
	LockAt       string             `json:"lock_at,omitempty"`
	Rows         []PersonalDatesRow `json:"rows"`
}

type PersonalDatesRow struct {
	StudentRef string `json:"student_ref"`
	Alias      string `json:"alias"`
	Status     string `json:"status"` // planned, written, held
	Reason     string `json:"reason,omitempty"`
	// Why lists what earned the dates; private, never for students or families.
	Why    []string `json:"why,omitempty"`
	DueAt  string   `json:"due_at,omitempty"`
	LockAt string   `json:"lock_at,omitempty"`
}

// extraSchoolDays is the student's saved late-turn-in allowance for one
// assignment, in whole school days. A percent is taken of the school days from
// the day the work was assigned (Canvas "available from", else the day it was
// created) to the due date, rounded up. held explains an allowance that cannot
// be worked out.
func extraSchoolDays(st workspace.Student, a Assignment, due time.Time, cal workspace.SchoolCalendar, generalDays int) (days int, basis, held string) {
	if !st.ExtraTime {
		return 0, "", ""
	}
	switch {
	case st.ExtraTimePercent > 0:
		assigned := a.UnlockAt
		if assigned == nil && a.CreatedAt != "" {
			assigned = &a.CreatedAt
		}
		start, err := parseOptionalTime(assigned)
		if err != nil || start == nil {
			return 0, "", "the assignment has no assigned date to take the extra-time percent from"
		}
		loc, err := time.LoadLocation(cal.Timezone)
		if err != nil {
			return 0, "", "the school calendar timezone is invalid"
		}
		first := start.In(loc).AddDate(0, 0, 1).Format("2006-01-02")
		spread := len(cal.SchoolDaysBetween(first, due.In(loc).Format("2006-01-02")))
		days = (spread*st.ExtraTimePercent + 99) / 100
		return days, fmt.Sprintf("Saved extra time: %d%% of %d school days is %d school days (Students page).", st.ExtraTimePercent, spread, days), ""
	case st.ExtraTimeDays > 0:
		return st.ExtraTimeDays, fmt.Sprintf("Saved extra time: %d school days (Students page).", st.ExtraTimeDays), ""
	case generalDays > 0:
		return generalDays, fmt.Sprintf("General extra-time allowance: %d school days (context/grading.md).", generalDays), ""
	}
	return 0, "", "Extra time is ticked but no number of extra days is saved for this student or as a general allowance"
}

// firstSubmittedAt is the earliest time the student turned anything in.
func firstSubmittedAt(sub Submission) *time.Time {
	var first *time.Time
	for _, s := range append([]Submission{sub}, sub.History...) {
		t, err := parseOptionalTime(s.SubmittedAt)
		if err == nil && t != nil && (first == nil || t.Before(*first)) {
			first = t
		}
	}
	return first
}

// PersonalDates plans, and unless preview writes, personal Canvas dates. With
// assignmentID 0 it covers the course's published assignments due in the last
// three weeks or later.
func (s *Service) PersonalDates(ctx context.Context, nickname string, assignmentID int64, preview bool, now time.Time) (PersonalDatesReport, error) {
	course, err := s.Course(nickname)
	if err != nil {
		return PersonalDatesReport{}, err
	}
	outer := s
	if s, err = s.ForCourse(ctx, course); err != nil {
		return PersonalDatesReport{}, err
	}
	report := PersonalDatesReport{Course: course.Nickname, Preview: preview}
	cal, err := workspace.ReadSchoolCalendar(s.Root)
	if err == nil {
		err = cal.Validate()
	}
	if err != nil {
		return report, fmt.Errorf("the school calendar (context/school-calendar.md) is needed to set personal dates: %w", err)
	}
	policy, err := s.lateWorkFor(course)
	if err != nil {
		return report, err
	}
	if policy == nil {
		policy = &domain.LateWorkPolicy{}
	}
	_, owned, err := outer.Owned(course)
	if err != nil {
		return report, err
	}
	students, err := workspace.OpenStudents(s.Root)
	if err != nil {
		return report, err
	}
	absences, err := workspace.OpenAbsences(s.Root)
	if err != nil {
		return report, err
	}
	enrollments, err := StudentEnrollments(ctx, s.Client, course.ID)
	if err != nil {
		return report, err
	}
	var active []workspace.Student
	seen := map[int64]bool{}
	for _, enrollment := range enrollments {
		userID := enrollment.UserID
		if userID == 0 {
			userID = enrollment.User.ID
		}
		if userID <= 0 || enrollment.State != "active" || seen[userID] {
			continue
		}
		seen[userID] = true
		st, e := students.Upsert(s.Client.Origin(), userID, enrollment.User.Name, enrollment.User.ShortName, enrollment.User.SISID, enrollment.User.Login)
		if e != nil {
			return report, e
		}
		active = append(active, st)
	}
	if err = students.Save(); err != nil {
		return report, err
	}
	sort.Slice(active, func(i, j int) bool { return active[i].Alias < active[j].Alias })

	var assignments []Assignment
	if assignmentID > 0 {
		a, e := GetAssignment(ctx, s.Client, course.ID, assignmentID)
		if e != nil {
			return report, e
		}
		assignments = []Assignment{a}
	} else {
		all, e := sweepAssignments(ctx, s, course.ID)
		if e != nil {
			return report, e
		}
		for _, a := range all {
			if due, _ := parseOptionalTime(a.DueAt); a.Published && due != nil && due.After(now.Add(-personalDatesLookBack)) {
				assignments = append(assignments, a)
			}
		}
	}
	// A retake never gets personal dates of its own, and an original that has
	// one is not reopened for corrections: the retake replaces the second try.
	var retaken map[int64]bool
	if all, e := sweepAssignments(ctx, s, course.ID); e == nil {
		retaken = retakenOriginals(all)
	} else if assignmentID > 0 {
		return report, e
	}
	untagged := 0
	for _, a := range assignments {
		if isRetake(a) {
			continue
		}
		aPolicy := *policy
		if retaken[a.ID] {
			aPolicy.CorrectionSchoolDays = 0
		}
		if owned && !domain.HasConsentTag(a.Name) {
			untagged++
			continue
		}
		if a.GroupCategoryID != nil && *a.GroupCategoryID > 0 {
			report.Skipped = append(report.Skipped, fmt.Sprintf("%s: group assignments get no personal dates", a.Name))
			continue
		}
		entry, e := s.personalDatesFor(ctx, course, a, active, absences, cal, aPolicy, preview, now, &report)
		if e != nil {
			return report, e
		}
		if len(entry.Rows) > 0 {
			report.Assignments = append(report.Assignments, entry)
		}
	}
	if untagged > 0 {
		report.Skipped = append(report.Skipped, fmt.Sprintf("%d assignments not tagged (CE) were skipped.", untagged))
	}
	return report, nil
}

func (s *Service) personalDatesFor(ctx context.Context, course domain.Course, a Assignment, active []workspace.Student, absences *workspace.AbsenceStore, cal workspace.SchoolCalendar, policy domain.LateWorkPolicy, preview bool, now time.Time, report *PersonalDatesReport) (PersonalDatesAssignment, error) {
	entry := PersonalDatesAssignment{AssignmentID: a.ID, Title: a.Name}
	if a.DueAt != nil {
		entry.DueAt = *a.DueAt
	}
	if a.LockAt != nil {
		entry.LockAt = *a.LockAt
	}
	subs, err := Submissions(ctx, s.Client, course.ID, a.ID, true)
	if err != nil {
		return entry, err
	}
	byUser := map[int64]Submission{}
	for _, sub := range subs {
		byUser[sub.UserID] = sub
	}
	overridesPath := fmt.Sprintf("/api/v1/courses/%d/assignments/%d/overrides", course.ID, a.ID)
	var overrides []AssignmentOverrideLive
	if err = s.Client.List(ctx, overridesPath+"?per_page=100", &overrides); err != nil {
		return entry, err
	}
	personal := map[int64]AssignmentOverrideLive{}
	for _, o := range overrides {
		for _, id := range o.StudentIDs {
			personal[id] = o
		}
	}
	baseLock, _ := parseOptionalTime(a.LockAt)
	for _, st := range active {
		sub, has := byUser[st.UserID]
		if has && (sub.Excused || (sub.AssignmentVisible != nil && !*sub.AssignmentVisible)) {
			continue
		}
		dueText := a.DueAt
		if has {
			dueText = sub.CachedDueDate
		}
		baseDue, err := parseOptionalTime(dueText)
		if err != nil || baseDue == nil {
			continue
		}
		alias := st.Alias
		if !s.Config.Aliases {
			alias = st.Name
		}
		row := PersonalDatesRow{StudentRef: st.Ref, Alias: alias}
		hold := func(reason string) {
			row.Status, row.Reason = "held", reason
			report.Held++
			entry.Rows = append(entry.Rows, row)
		}
		ov, hasOverride := personal[st.UserID]
		curLock := baseLock
		if hasOverride && ov.LockAtOverridden != nil && *ov.LockAtOverridden {
			curLock, _ = parseOptionalTime(ov.LockAt)
		}
		absent := absences.Dates(s.Config.Instance, st.Ref)
		newDue := *baseDue
		// A personal Canvas due date already carries the student's allowance.
		if !(hasOverride && ov.DueAt != nil) {
			days, _, held := extraSchoolDays(st, a, *baseDue, cal, policy.ExtraTimeSchoolDays)
			if held != "" && st.ExtraTimePercent > 0 {
				hold(held)
				continue
			}
			if days > 0 {
				if newDue, err = cal.AddSchoolDays(*baseDue, days); err != nil {
					hold(err.Error())
					continue
				}
				row.Why = append(row.Why, "extra time")
			}
		}
		if !hasOverride || ov.DueAt == nil || ov.Title == personalDatesTitle {
			moved, e := cal.DeadlineAfterAbsences(newDue, absent)
			if e != nil {
				hold(e.Error())
				continue
			}
			if !moved.Equal(newDue) {
				newDue = moved
				row.Why = append(row.Why, "absent on the due date")
			}
		}
		// The lock only moves on an assignment that locks.
		var target *time.Time
		if curLock != nil {
			own := cal.WithAbsences(absent)
			if cutoff := policy.LateCutoffSchoolDays; cutoff != nil {
				regular, mine := *baseDue, newDue
				if *cutoff > 0 {
					var e1, e2 error
					regular, e1 = cal.AddSchoolDays(*baseDue, *cutoff)
					mine, e2 = own.AddSchoolDays(newDue, *cutoff)
					if e1 != nil || e2 != nil {
						hold("the school calendar does not cover this student's late-work window")
						continue
					}
				}
				if mine.After(regular) {
					target = &mine
					if len(row.Why) == 0 {
						row.Why = append(row.Why, "absent during the late-work window")
					}
				}
			} else if newDue.After(*curLock) {
				target = &newDue
			}
			if n := policy.CorrectionSchoolDays; n > 0 && has && now.After(newDue) {
				if first := firstSubmittedAt(sub); first != nil && !first.After(newDue) {
					end, e := own.AddSchoolDays(newDue, n)
					if e != nil {
						hold("the school calendar does not cover this student's correction window")
						continue
					}
					if end.After(*curLock) && (target == nil || end.After(*target)) {
						target = &end
						row.Why = append(row.Why, "on time: open for corrections")
					}
				}
			}
			if target != nil && !target.After(*curLock) {
				target = nil
			}
		}
		if newDue.Equal(*baseDue) && target == nil {
			report.Unchanged++
			continue
		}
		if hasOverride && len(ov.StudentIDs) > 1 {
			hold("this student shares a Canvas personal date with other students; change that date in Canvas")
			continue
		}
		fields := map[string]any{"student_ids": []int64{st.UserID}, "title": personalDatesTitle, "due_at": newDue.UTC().Format(time.RFC3339)}
		row.DueAt = newDue.Format(time.RFC3339)
		unlock := a.UnlockAt
		if hasOverride {
			fields["title"] = ov.Title
			if ov.UnlockAtOverridden != nil && *ov.UnlockAtOverridden {
				unlock = ov.UnlockAt
			}
		}
		if unlock != nil && *unlock != "" {
			fields["unlock_at"] = *unlock
		}
		if lock := target; lock != nil || curLock != nil {
			if lock == nil {
				lock = curLock
			}
			fields["lock_at"] = lock.UTC().Format(time.RFC3339)
			row.LockAt = lock.In(newDue.Location()).Format(time.RFC3339)
		}
		if preview {
			row.Status = "planned"
			report.Planned++
			entry.Rows = append(entry.Rows, row)
			continue
		}
		method, path := "POST", overridesPath
		if hasOverride {
			method, path = "PUT", fmt.Sprintf("%s/%d", overridesPath, ov.ID)
		}
		if e := s.Client.Write(ctx, method, path, map[string]any{"assignment_override": fields}, nil); e != nil {
			hold("Canvas did not accept the personal date: " + e.Error())
			continue
		}
		row.Status = "written"
		report.Written++
		entry.Rows = append(entry.Rows, row)
	}
	return entry, nil
}
