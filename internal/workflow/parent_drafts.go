package workflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"math"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"canvasexpertse/internal/workspace"
	"github.com/yuin/goldmark"
)

// trimPoint shows Canvas's "80.0" as "80"; other decimals are kept.
func trimPoint(v string) string { return strings.TrimSuffix(v, ".0") }

// ParentConfig is context/parent-updates.md front matter.
type ParentConfig struct {
	Schema int `yaml:"schema"`
	// The YAML keys keep their original names for compatibility; the values
	// are each teacher's own grade portal and tutoring sign-up.
	SkywardURL      string `yaml:"skyward_url"`
	GradePortalName string `yaml:"grade_portal_name"`
	BobcatHourURL   string `yaml:"bobcat_hour_url"`
	BobcatHourLabel string `yaml:"bobcat_hour_label"`
	BobcatHourTxt   string `yaml:"bobcat_hour_text"`
	SignOff         string `yaml:"sign_off"`
	SkipScore       int    `yaml:"skip_score"`
	// CorrectionDays is both the report window and how long after the due date a
	// correction stays open. Zero means the default, 14.
	CorrectionDays int `yaml:"correction_days"`
}

func loadParentConfig(root string) (ParentConfig, error) {
	var config ParentConfig
	if _, err := workspace.ReadTeacherDocument(root, "context/parent-updates.md", &config); err != nil && !os.IsNotExist(err) {
		return config, fmt.Errorf("context/parent-updates.md: %w", err)
	}
	if config.SkipScore == 0 {
		config.SkipScore = 90
	}
	if config.CorrectionDays == 0 {
		config.CorrectionDays = correctionWindowDays
	}
	if config.CorrectionDays < 1 || config.CorrectionDays > 60 {
		return config, fmt.Errorf("context/parent-updates.md: correction_days must be between 1 and 60")
	}
	if config.GradePortalName == "" {
		config.GradePortalName = "the school's grade portal"
	}
	if config.BobcatHourLabel == "" {
		config.BobcatHourLabel = "Tutoring"
	}
	if config.BobcatHourTxt == "" {
		config.BobcatHourTxt = "Sign up for tutorials and clubs"
	}
	return config, nil
}

// ParentDrafts is the agent's decisions and short instruction text. Assignment
// IDs refer to the report; titles, links, dates and scores come from it.
type ParentDrafts struct {
	Schema        int                  `json:"schema"`
	Kind          string               `json:"kind"`
	RunID         string               `json:"run_id"`
	Students      []ParentStudentDraft `json:"students"`
	Layout        string               `json:"layout,omitempty"`
	Announcements map[string]string    `json:"announcements,omitempty"`
}

type ParentStudentDraft struct {
	Ref      string `json:"student_ref"`
	Decision string `json:"decision"` // send | skip | flag
	Reason   string `json:"reason,omitempty"`
	Intro    string `json:"intro,omitempty"`
	// StudentIntro opens the printable student sheet, written to the student.
	StudentIntro string          `json:"student_intro,omitempty"`
	Missing      []ParentItemRef `json:"missing,omitempty"`
	Corrections  []ParentItemRef `json:"corrections,omitempty"`
	DueSoon      []ParentItemRef `json:"due_soon,omitempty"`
	History      []ParentItemRef `json:"history,omitempty"`
}

type ParentItemRef struct {
	AssignmentID int64  `json:"assignment_id"`
	Note         string `json:"note,omitempty"`
}

// ParentRun is one loaded run folder.
type ParentRun struct {
	Dir      string
	Report   ParentReportPacket
	Drafts   ParentDrafts
	Config   ParentConfig
	Root     string
	students map[string]ParentReportStudent
	// ReplyNotes holds each student's reply-to note (HTML) for the email footer;
	// set by the Service for students in teammates' courses. Never printed.
	ReplyNotes map[string]string
}

// LoadParentRun reads report.json and drafts.json from a run folder and
// validates the drafts against the report.
func LoadParentRun(root, dir string) (*ParentRun, error) {
	run := &ParentRun{Dir: dir, Root: root}
	for name, out := range map[string]any{"report.json": &run.Report, "drafts.json": &run.Drafts} {
		b, e := os.ReadFile(dir + string(os.PathSeparator) + name)
		if e != nil {
			return nil, fmt.Errorf("read %s: %w", name, e)
		}
		if e = json.Unmarshal(b, out); e != nil {
			return nil, fmt.Errorf("%s is not valid JSON: %v", name, e)
		}
	}
	var err error
	run.Config, err = loadParentConfig(root)
	if err != nil {
		return nil, err
	}
	run.students = map[string]ParentReportStudent{}
	for i := range run.Report.Students {
		s := &run.Report.Students[i]
		s.SkipEligible = parentSkipEligible(*s, run.Config.SkipScore)
		run.students[s.Ref] = *s
	}
	if run.Drafts.Layout == "weekly_update" {
		for i := range run.Drafts.Students {
			d := &run.Drafts.Students[i]
			if d.Decision != "send" {
				continue
			}
			if student, ok := run.students[d.Ref]; ok {
				if reason := weeklyHoldReason(student); reason != "" {
					d.Decision, d.Reason = "flag", reason
				}
			}
		}
	}
	if e := run.validate(); e != nil {
		return nil, e
	}
	return run, nil
}

var scoreLike = regexp.MustCompile(`\d\s*/\s*\d|%|\bout of\b|\bpoints?\b`)

func (r *ParentRun) validate() error {
	if err := validateParentReportScope(r.Report); err != nil {
		return err
	}
	d := r.Drafts
	if d.Kind != "parent-drafts" || !((d.Schema == 1 && d.Layout == "") || (d.Schema == 2 && d.Layout == "weekly_update")) {
		return fmt.Errorf("drafts.json must have schema 1 with the default layout, or schema 2 with layout weekly_update")
	}
	if d.Layout == "weekly_update" {
		loc, err := time.LoadLocation(r.Report.Scope.Timezone)
		if err != nil {
			return err
		}
		start, err := time.ParseInLocation("2006-01-02", r.Report.Scope.PeriodStartDate, loc)
		recent, e := time.Parse(time.RFC3339, r.Report.Scope.RecentStartAt)
		if err != nil || e != nil || !start.Equal(recent) {
			return fmt.Errorf("weekly update requires a full-period parent report; pull a fresh report with full_period")
		}
		if strings.TrimSpace(r.Config.SignOff) == "" || strings.TrimSpace(r.Config.BobcatHourURL) == "" {
			return fmt.Errorf("weekly update requires sign_off and bobcat_hour_url in context/parent-updates.md")
		}
		if err := r.validateWeeklyAnnouncements(); err != nil {
			return err
		}
	}
	if d.RunID != r.Report.RunID {
		return fmt.Errorf("drafts.json run_id %q does not match the report run %q", d.RunID, r.Report.RunID)
	}
	seen := map[string]bool{}
	for _, sd := range d.Students {
		rs, ok := r.students[sd.Ref]
		if !ok {
			return fmt.Errorf("student %s is not in the report", sd.Ref)
		}
		if seen[sd.Ref] {
			return fmt.Errorf("student %s appears twice", sd.Ref)
		}
		seen[sd.Ref] = true
		hasWindowWork := false
		for _, c := range rs.Courses {
			hasWindowWork = hasWindowWork || len(c.WorkDue) > 0
		}
		if e := checkFreeText(sd.Ref, sd.StudentIntro); e != nil {
			return e
		}
		switch sd.Decision {
		case "send":
			if d.Layout == "weekly_update" {
				if len(rs.Courses) != 1 {
					return fmt.Errorf("%s: weekly update requires exactly one course per student", sd.Ref)
				}
				if len(rs.Courses[0].Flags) > 0 {
					return fmt.Errorf("%s: report has unresolved flags; mark this student flag", sd.Ref)
				}
				continue
			}
		case "skip", "flag":
			if strings.TrimSpace(sd.Reason) == "" {
				return fmt.Errorf("%s: a %s decision needs a reason", sd.Ref, sd.Decision)
			}
			continue
		default:
			return fmt.Errorf("%s: decision must be send, skip or flag", sd.Ref)
		}
		sections := []struct {
			name  string
			items []ParentItemRef
			has   func(c ParentReportCourse, id int64) bool
		}{
			{"missing", sd.Missing, func(c ParentReportCourse, id int64) bool {
				for _, i := range c.Missing {
					if i.AssignmentID == id {
						return true
					}
				}
				return false
			}},
			{"corrections", sd.Corrections, func(c ParentReportCourse, id int64) bool {
				for _, i := range c.Corrections {
					if i.AssignmentID == id {
						return true
					}
				}
				return false
			}},
			{"due_soon", sd.DueSoon, func(c ParentReportCourse, id int64) bool {
				for _, i := range c.DueSoon {
					if i.AssignmentID == id {
						return true
					}
				}
				return false
			}},
			{"history", sd.History, func(c ParentReportCourse, id int64) bool {
				for _, i := range c.History {
					if i.AssignmentID == id {
						return true
					}
				}
				return false
			}},
		}
		if hasWindowWork {
			for _, section := range []struct {
				name string
				ids  []ParentItemRef
			}{
				{"missing", sd.Missing}, {"corrections", sd.Corrections}, {"history", sd.History},
			} {
				for _, ref := range section.ids {
					present := false
					for _, c := range rs.Courses {
						for _, item := range c.WorkDue {
							present = present || item.AssignmentID == ref.AssignmentID
						}
					}
					if !present {
						return fmt.Errorf("%s: assignment %d is not in the saved work_due inventory", section.name, ref.AssignmentID)
					}
				}
			}
		}
		for _, sec := range sections {
			for _, it := range sec.items {
				found := false
				for _, c := range rs.Courses {
					found = found || sec.has(c, it.AssignmentID)
				}
				if !found {
					return fmt.Errorf("%s: assignment %d is not in this student's report %s list", sd.Ref, it.AssignmentID, sec.name)
				}
				if e := checkFreeText(sd.Ref, it.Note); e != nil {
					return e
				}
			}
		}
		if !hasWindowWork {
			return fmt.Errorf("%s: there is no work due in this report window; mark this student skip with that reason", sd.Ref)
		}
		if e := checkFreeText(sd.Ref, sd.Intro); e != nil {
			return e
		}
	}
	var missing []string
	for _, s := range r.Report.Students {
		if !seen[s.Ref] {
			missing = append(missing, s.Ref)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("every student in the report needs a decision; missing: %s", strings.Join(missing, ", "))
	}
	return nil
}

var announcementEmail = regexp.MustCompile(`(?i)\b[A-Z0-9._%+\-]+@[A-Z0-9.\-]+\.[A-Z]{2,}\b`)

func (r *ParentRun) validateWeeklyAnnouncements() error {
	for _, course := range r.Report.Courses {
		text, ok := r.Drafts.Announcements[course]
		if !ok || strings.TrimSpace(text) == "" {
			return fmt.Errorf("weekly update requires announcements for course %s", course)
		}
		if announcementEmail.MatchString(text) {
			return fmt.Errorf("weekly announcements for %s cannot contain email addresses", course)
		}
		for _, student := range r.Report.Students {
			for _, identifier := range []string{student.Ref, student.Alias} {
				if identifier != "" && regexp.MustCompile(`(?i)(^|[^\pL\pN])`+regexp.QuoteMeta(identifier)+`([^\pL\pN]|$)`).MatchString(text) {
					return fmt.Errorf("weekly announcements for %s cannot contain student identifiers", course)
				}
			}
		}
	}
	return nil
}

// An incomplete course must be held rather than producing a reassuring but
// incomplete email. Other students can still proceed through the review.
func weeklyHoldReason(s ParentReportStudent) string {
	if len(s.Courses) != 1 {
		return "Weekly update requires exactly one course for this student."
	}
	c := s.Courses[0]
	if len(c.Flags) != 0 {
		return "The report has unresolved course flags."
	}
	for _, item := range c.WorkDue {
		if item.Status == "excused" || item.Status == "grade_not_released" {
			continue
		}
		if _, err := time.Parse(time.RFC3339, item.DueAt); err != nil {
			return "An assignment has no valid due date."
		}
		u, err := url.Parse(item.URL)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
			return "An assignment has no valid link."
		}
		if item.Current != nil {
			n, err := strconv.ParseFloat(*item.Current, 64)
			if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
				return "An assignment has an invalid score."
			}
			if n > 0 {
				possible, err := strconv.ParseFloat(item.Possible, 64)
				if err != nil || possible <= 0 || math.IsNaN(possible) || math.IsInf(possible, 0) {
					return "An assignment has an invalid points possible value."
				}
			}
		}
	}
	return ""
}

func validateParentReportScope(report ParentReportPacket) error {
	scope := report.Scope
	if scope == nil {
		return fmt.Errorf("this saved parent report has no calendar scope; pull a fresh parent report before sending or printing. Existing drafts and delivery outcomes are preserved")
	}
	loc, err := time.LoadLocation(scope.Timezone)
	if err != nil || scope.Timezone == "" {
		return fmt.Errorf("saved parent report scope has an invalid timezone")
	}
	start, err := time.ParseInLocation("2006-01-02", scope.PeriodStartDate, loc)
	if err != nil {
		return fmt.Errorf("saved parent report scope has an invalid period start date")
	}
	end, err := time.ParseInLocation("2006-01-02", scope.PeriodEndDate, loc)
	if err != nil || end.Before(start) || strings.TrimSpace(scope.PeriodName) == "" {
		return fmt.Errorf("saved parent report scope has invalid period dates or name")
	}
	asOf, err := time.Parse(time.RFC3339, report.AsOf)
	if err != nil {
		return fmt.Errorf("saved parent report has an invalid as_of timestamp")
	}
	localDay := asOf.In(loc).Format("2006-01-02")
	if localDay < scope.PeriodStartDate || localDay > scope.PeriodEndDate {
		return fmt.Errorf("saved parent report as_of date does not fall within its saved grading period")
	}
	recentStart, err := time.Parse(time.RFC3339, scope.RecentStartAt)
	if err != nil {
		return fmt.Errorf("saved parent report scope has an invalid recent_start_at")
	}
	recentEnd, err := time.Parse(time.RFC3339, scope.RecentEndAt)
	if err != nil || !recentEnd.Equal(asOf) {
		return fmt.Errorf("saved parent report scope recent_end_at must match as_of")
	}
	expectedStart := asOf.In(loc).AddDate(0, 0, -daysOrDefault(report.CorrectionDays))
	if start.After(expectedStart) {
		expectedStart = start
	}
	if !recentStart.Equal(expectedStart) {
		return fmt.Errorf("saved parent report scope recent_start_at does not match the saved period and as_of")
	}
	dueSoonEnd, err := time.Parse(time.RFC3339, scope.DueSoonEndAt)
	if err != nil {
		return fmt.Errorf("saved parent report scope has an invalid due_soon_end_at")
	}
	expectedSoonEnd := asOf.In(loc).AddDate(0, 0, dueSoonDays)
	if end.AddDate(0, 0, 1).Before(expectedSoonEnd) {
		expectedSoonEnd = end.AddDate(0, 0, 1)
	}
	if !dueSoonEnd.Equal(expectedSoonEnd) {
		return fmt.Errorf("saved parent report scope due_soon_end_at does not match the saved period and as_of")
	}
	for _, student := range report.Students {
		for _, course := range student.Courses {
			if course.WorkDue == nil {
				return fmt.Errorf("saved parent report course %s is missing its work_due inventory", course.Course)
			}
			for _, item := range course.WorkDue {
				due, parseErr := time.Parse(time.RFC3339, item.DueAt)
				if parseErr != nil || due.Before(recentStart) || due.After(asOf) {
					return fmt.Errorf("saved parent report inventory item %d is outside its saved due-date window", item.AssignmentID)
				}
			}
			inventory := map[int64]bool{}
			for _, item := range course.WorkDue {
				inventory[item.AssignmentID] = true
			}
			for _, item := range course.Missing {
				if !inventory[item.AssignmentID] {
					return fmt.Errorf("saved Missing/0 item %d is outside the work_due inventory", item.AssignmentID)
				}
			}
			for _, item := range course.Corrections {
				if !inventory[item.AssignmentID] {
					return fmt.Errorf("saved correction item %d is outside the work_due inventory", item.AssignmentID)
				}
			}
			for _, item := range course.History {
				if !inventory[item.AssignmentID] {
					return fmt.Errorf("saved history item %d is outside the work_due inventory", item.AssignmentID)
				}
			}
			for _, item := range course.DueSoon {
				due, parseErr := time.Parse(time.RFC3339, item.DueAt)
				if parseErr != nil || !due.After(asOf) || !due.Before(dueSoonEnd) {
					return fmt.Errorf("saved parent report due-soon item %d is outside its saved window", item.AssignmentID)
				}
			}
		}
	}
	return nil
}

func checkFreeText(ref, text string) error {
	if scoreLike.MatchString(strings.ToLower(text)) {
		return fmt.Errorf("%s: notes cannot contain scores or point values; the Engine fills those in from Canvas", ref)
	}
	return nil
}

// Rendered is one student's email.
type Rendered struct {
	Subject, HTML string
}

const (
	cellStyle  = `style="padding:4px 10px 4px 0;vertical-align:top;border-bottom:1px solid #e4e7eb"`
	headStyle  = `style="padding:4px 10px 4px 0;text-align:left;color:#52606d;font-weight:600;border-bottom:1px solid #9aa5b1"`
	h3Style    = `style="margin:18px 0 6px;font-size:15px"`
	linkFormat = `<a href="%s">%s</a>`
)

// Render builds one student's email from the report; the agent's draft supplies
// only which assignments appear, their order and short notes.
func (r *ParentRun) Render(sd ParentStudentDraft, student workspace.Student) Rendered {
	if r.Drafts.Layout == "weekly_update" {
		return r.renderWeekly(sd)
	}
	return r.render(sd, student, false)
}

// RenderStudent builds the printable sheet handed to the student. Every
// student gets one: all missing, due-soon and history items come from the
// report, and only corrections follow the agent's draft (eligibility is
// judgment). Titles are plain text because links do not work on paper.
func (r *ParentRun) RenderStudent(sd ParentStudentDraft, student workspace.Student) Rendered {
	rs := r.students[sd.Ref]
	all := sd
	all.Missing, all.DueSoon, all.History = nil, nil, nil
	for _, c := range rs.Courses {
		missing := append([]ParentReportItem(nil), c.Missing...)
		sort.SliceStable(missing, func(i, j int) bool { return missing[i].DueAt < missing[j].DueAt })
		for _, it := range missing {
			all.Missing = append(all.Missing, ParentItemRef{AssignmentID: it.AssignmentID, Note: noteFor(sd.Missing, it.AssignmentID)})
		}
		for _, it := range c.DueSoon {
			all.DueSoon = append(all.DueSoon, ParentItemRef{AssignmentID: it.AssignmentID, Note: noteFor(sd.DueSoon, it.AssignmentID)})
		}
	}
	return r.render(all, student, true)
}

func noteFor(items []ParentItemRef, id int64) string {
	for _, it := range items {
		if it.AssignmentID == id {
			return it.Note
		}
	}
	return ""
}

func (r *ParentRun) render(sd ParentStudentDraft, student workspace.Student, forStudent bool) Rendered {
	rs := r.students[sd.Ref]
	esc := html.EscapeString
	name := func(s string) string { return esc(workspace.ReplaceStudentName(s, student)) }
	loc := time.UTC
	if len(rs.Courses) > 0 {
		if l, e := time.LoadLocation(rs.Courses[0].Timezone); e == nil {
			loc = l
		}
	}
	asOf, _ := time.Parse(time.RFC3339, r.Report.AsOf)
	day := func(v string) string {
		t, e := time.Parse(time.RFC3339, v)
		if e != nil {
			return ""
		}
		return t.In(loc).Format("Mon Jan 2")
	}
	title := func(i ParentReportItem) string {
		if i.URL == "" || forStudent {
			return esc(i.Title)
		}
		return fmt.Sprintf(linkFormat, esc(i.URL), esc(i.Title))
	}
	noteOf := func(items []ParentItemRef, id int64) string {
		for _, it := range items {
			if it.AssignmentID == id {
				return name(it.Note)
			}
		}
		return ""
	}
	order := func(items []ParentItemRef, id int64) int {
		for i, it := range items {
			if it.AssignmentID == id {
				return i
			}
		}
		return -1
	}
	score := func(v *string, possible string) string {
		if v == nil {
			return "not available"
		}
		if possible != "" {
			return esc(trimPoint(*v) + " / " + trimPoint(possible))
		}
		return esc(trimPoint(*v))
	}

	var b strings.Builder
	fmt.Fprintf(&b, `<div style="font-family:Segoe UI,Arial,sans-serif;font-size:14px;line-height:1.4;color:#1f2933;max-width:640px">`)
	intro := sd.Intro
	if forStudent {
		var courses []string
		for _, c := range rs.Courses {
			courses = append(courses, c.Course)
		}
		fmt.Fprintf(&b, `<p style="margin:0 0 4px;font-size:17px"><b>%s</b> &middot; %s &middot; As of %s</p>`, esc(student.Name), esc(strings.Join(courses, ", ")), esc(asOf.In(loc).Format("Mon Jan 2")))
		intro = sd.StudentIntro
		missing := 0
		for _, c := range rs.Courses {
			missing += len(c.Missing)
		}
		complete := true
		for _, c := range rs.Courses {
			if len(c.Flags) > 0 {
				complete = false
			}
		}
		if missing == 0 && complete {
			intro = strings.TrimSpace(intro + " No Missing/0 work in this report window.")
		}
	} else {
		fmt.Fprintf(&b, `<p style="margin:0 0 4px"><b>%s</b> &middot; As of %s</p>`, esc(student.FirstName), esc(asOf.In(loc).Format("Mon Jan 2, 3:04 pm")))
	}
	if r.Report.Scope != nil {
		scope := r.Report.Scope
		recentStart, _ := time.Parse(time.RFC3339, scope.RecentStartAt)
		recentEnd, _ := time.Parse(time.RFC3339, scope.RecentEndAt)
		fmt.Fprintf(&b, `<p style="margin:0 0 8px;color:#52606d">%s · %s–%s · Recent window %s–%s</p>`, esc(scope.PeriodName), esc(scope.PeriodStartDate), esc(scope.PeriodEndDate), esc(recentStart.In(loc).Format("Jan 2")), esc(recentEnd.In(loc).Format("Jan 2")))
	}
	if intro != "" {
		fmt.Fprintf(&b, `<p style="margin:6px 0">%s</p>`, name(intro))
	}
	if forStudent {
		fmt.Fprintf(&b, `<p style="margin:6px 0;color:#52606d">Find each assignment in Canvas by its title.</p>`)
	}
	multi := len(rs.Courses) > 1
	for _, c := range rs.Courses {
		var miss, corr, soon, dueRows []string
		for _, it := range sortedBy(c.Missing, sd.Missing, order) {
			miss = append(miss, fmt.Sprintf(`<tr><td %s>%s</td><td %s>%s</td><td %s>%s</td></tr>`, cellStyle, title(it), cellStyle, esc(day(it.DueAt)), cellStyle, noteOf(sd.Missing, it.AssignmentID)))
		}
		for _, it := range sortedCorr(c.Corrections, sd.Corrections, order) {
			corr = append(corr, fmt.Sprintf(`<tr><td %s>%s</td><td %s>%s</td><td %s>%s</td></tr>`, cellStyle, title(it.ParentReportItem), cellStyle, esc(day(it.CorrectionDeadline)), cellStyle, noteOf(sd.Corrections, it.AssignmentID)))
		}
		for _, it := range sortedBy(c.DueSoon, sd.DueSoon, order) {
			soon = append(soon, fmt.Sprintf(`<tr><td %s>%s</td><td %s>%s</td><td %s>%s</td></tr>`, cellStyle, title(it), cellStyle, esc(day(it.DueAt)), cellStyle, noteOf(sd.DueSoon, it.AssignmentID)))
		}
		for _, it := range c.WorkDue {
			current := score(it.Current, it.Possible)
			status := it.Status
			if status == "awaiting_grading" {
				status = "Awaiting grading"
			} else if status == "grade_not_released" {
				status = "Grade not released"
			} else if status == "no_grade" {
				status = "No grade recorded"
			} else if status == "recorded_zero" {
				status = "Recorded zero"
			} else if status == "not_submitted" {
				status = "Not submitted"
			} else if status == "missing" {
				status = "Missing"
			} else if status == "graded" {
				status = "Graded"
			}
			first := score(it.First, it.Possible)
			if it.First == nil {
				first = "not available"
			}
			dueRows = append(dueRows, fmt.Sprintf(`<tr><td %s>%s</td><td %s>%s</td><td %s>%s</td><td %s>%s</td><td %s>%s</td></tr>`, cellStyle, title(it.ParentReportItem), cellStyle, esc(day(it.DueAt)), cellStyle, esc(status), cellStyle, first, cellStyle, current))
		}
		if len(miss)+len(corr)+len(soon)+len(dueRows) == 0 {
			continue
		}
		if multi {
			fmt.Fprintf(&b, `<h2 style="margin:20px 0 0;font-size:16px;border-top:2px solid #9aa5b1;padding-top:8px">%s</h2>`, esc(c.Course))
		}
		section := func(heading string, cols []string, rows []string) {
			if len(rows) == 0 {
				return
			}
			fmt.Fprintf(&b, `<h3 %s>%s</h3><table style="border-collapse:collapse;width:100%%"><tr>`, h3Style, esc(heading))
			for _, col := range cols {
				fmt.Fprintf(&b, `<th %s>%s</th>`, headStyle, esc(col))
			}
			b.WriteString(`</tr>` + strings.Join(rows, "") + `</table>`)
		}
		if forStudent {
			section("Missing/0", []string{"Assignment", "Was due", ""}, miss)
			section("Corrections you can still make", []string{"Assignment", "Correct by", "Where to look"}, corr)
			section("Due soon", []string{"Assignment", "Due", ""}, soon)
			section("Work due in this period", []string{"Assignment", "Due", "Status", "First score", "Current score"}, dueRows)
			continue
		}
		section("Missing/0", []string{"Assignment", "Due", ""}, miss)
		section("Corrections available", []string{"Assignment", "Correct by", "Where to look"}, corr)
		section("Due soon", []string{"Assignment", "Due", ""}, soon)
		section("Work due in this period", []string{"Assignment", "Due", "Status", "First score", "Current score"}, dueRows)
	}
	if forStudent {
		fmt.Fprintf(&b, `<p style="margin:18px 0 4px">Official grades: %s.</p>`, esc(r.Config.GradePortalName))
		if r.Config.BobcatHourURL != "" {
			fmt.Fprintf(&b, `<p style="margin:4px 0"><b>%s:</b> %s.</p>`, esc(r.Config.BobcatHourLabel), esc(r.Config.BobcatHourTxt))
		}
		if r.Config.SignOff != "" {
			fmt.Fprintf(&b, `<p style="margin:14px 0 0">%s</p>`, esc(r.Config.SignOff))
		}
		b.WriteString(aiNotice("This report was prepared with AI tools, so there may be mistakes. Scores and dates come straight from Canvas; check " + r.Config.GradePortalName + " for official grades. For privacy, the AI never saw your real name, only your nickname, “" + rs.Alias + ".”"))
		b.WriteString(`</div>`)
		return Rendered{Subject: student.Name, HTML: b.String()}
	}
	if r.Config.SkywardURL != "" {
		fmt.Fprintf(&b, `<p style="margin:18px 0 4px"><a href="%s">%s</a> &mdash; official grades</p>`, esc(r.Config.SkywardURL), esc(r.Config.GradePortalName))
	}
	if r.Config.BobcatHourURL != "" {
		fmt.Fprintf(&b, `<p style="margin:4px 0"><b>%s:</b> <a href="%s">%s</a>.</p>`, esc(r.Config.BobcatHourLabel), esc(r.Config.BobcatHourURL), esc(r.Config.BobcatHourTxt))
	}
	b.WriteString(r.ReplyNotes[sd.Ref])
	if r.Config.SignOff != "" {
		fmt.Fprintf(&b, `<p style="margin:14px 0 0">%s</p>`, esc(r.Config.SignOff))
	}
	b.WriteString(aiNotice("This update was prepared with AI tools, so there may be mistakes. Scores and dates come straight from Canvas; check " + r.Config.GradePortalName + " for official grades. For privacy, the AI never saw your child’s real name or your contact information, only the nickname “" + rs.Alias + ".”"))
	b.WriteString(`</div>`)
	return Rendered{Subject: fmt.Sprintf("Class update for %s (%s)", student.FirstName, asOf.In(loc).Format("Jan 2")), HTML: b.String()}
}

// aiNotice closes every email and printed report: the AI's part, where the
// facts come from, and that it worked only with the student's alias.
func aiNotice(text string) string {
	return `<p style="margin:18px 0 0;padding-top:8px;border-top:1px solid #e4e7eb;font-size:12px;color:#52606d">` + html.EscapeString(text) + `</p>`
}

func sortedBy(items []ParentReportItem, picked []ParentItemRef, order func([]ParentItemRef, int64) int) []ParentReportItem {
	var out []ParentReportItem
	for _, it := range items {
		if order(picked, it.AssignmentID) >= 0 {
			out = append(out, it)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return order(picked, out[i].AssignmentID) < order(picked, out[j].AssignmentID)
	})
	return out
}
func sortedCorr(items []ParentCorrection, picked []ParentItemRef, order func([]ParentItemRef, int64) int) []ParentCorrection {
	var out []ParentCorrection
	for _, it := range items {
		if order(picked, it.AssignmentID) >= 0 {
			out = append(out, it)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CorrectionDeadline < out[j].CorrectionDeadline })
	return out
}
func sortedHist(items []ParentHistoryItem, picked []ParentItemRef, order func([]ParentItemRef, int64) int) []ParentHistoryItem {
	var out []ParentHistoryItem
	for _, it := range items {
		if order(picked, it.AssignmentID) >= 0 {
			out = append(out, it)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return order(picked, out[i].AssignmentID) < order(picked, out[j].AssignmentID)
	})
	return out
}

// renderWeekly derives all academic facts from the saved, scoped inventory.
func (r *ParentRun) renderWeekly(sd ParentStudentDraft) Rendered {
	course := r.students[sd.Ref].Courses[0]
	loc, err := time.LoadLocation(course.Timezone)
	if err != nil {
		loc = time.UTC
	}
	asOf, _ := time.Parse(time.RFC3339, r.Report.AsOf)
	day := asOf.In(loc)
	monday := day.AddDate(0, 0, -(int(day.Weekday())+6)%7)
	months := []string{"Jan.", "Feb.", "March", "April", "May", "June", "July", "Aug.", "Sept.", "Oct.", "Nov.", "Dec."}
	subject := fmt.Sprintf("ELA 7 Update – Week of %s %d", months[int(monday.Month())-1], monday.Day())
	rows := append([]ParentWorkDueItem(nil), course.WorkDue...)
	sort.SliceStable(rows, func(i, j int) bool {
		a, _ := time.Parse(time.RFC3339, rows[i].DueAt)
		b, _ := time.Parse(time.RFC3339, rows[j].DueAt)
		if !a.Equal(b) {
			return a.Before(b)
		}
		return rows[i].Title < rows[j].Title
	})
	var missing, failing []string
	for _, item := range rows {
		if item.Status == "excused" || item.Status == "grade_not_released" {
			continue
		}
		value, valid := 0.0, false
		if item.Current != nil {
			var e error
			value, e = strconv.ParseFloat(*item.Current, 64)
			valid = e == nil
		}
		isMissing := (valid && value == 0) || (!valid && item.Status == "missing")
		link := fmt.Sprintf(linkFormat, html.EscapeString(item.URL), html.EscapeString(item.Title))
		if isMissing {
			due, _ := time.Parse(time.RFC3339, item.DueAt)
			missing = append(missing, fmt.Sprintf("<li>%s – due %d/%d</li>", link, due.In(loc).Month(), due.In(loc).Day()))
			continue
		}
		if !valid || value <= 0 {
			continue
		}
		possible, e := strconv.ParseFloat(item.Possible, 64)
		if e != nil || possible <= 0 {
			continue
		}
		percent := value * 100 / possible
		if percent >= 70 {
			continue
		}
		score := ""
		if possible != 100 {
			score = strconv.FormatFloat(math.Round(percent), 'f', 0, 64)
		} else {
			score = strconv.FormatFloat(math.Round(value*10)/10, 'f', -1, 64)
		}
		marker := ""
		if item.PointsDeducted != nil {
			if n, e := strconv.ParseFloat(*item.PointsDeducted, 64); e == nil && n > 0 {
				marker = " (late points deducted)"
			}
		}
		failing = append(failing, fmt.Sprintf("<li>%s – %s%s</li>", link, score, marker))
	}
	var b strings.Builder
	b.WriteString(`<div style="font-family:Segoe UI,Arial,sans-serif;font-size:14px;line-height:1.4;color:#1f2933;max-width:640px"><p>Hello,</p><p>Here's a quick ELA update.</p>`)
	fmt.Fprintf(&b, "<p><strong>Work that needs attention (%s)</strong></p>", html.EscapeString(r.Report.Scope.PeriodName))
	if len(missing)+len(failing) == 0 {
		fmt.Fprintf(&b, "<p>No missing or failing work in %s right now.</p>", html.EscapeString(r.Report.Scope.PeriodName))
	}
	if len(missing) > 0 {
		b.WriteString("<p>Missing:</p><ul>" + strings.Join(missing, "") + "</ul>")
	}
	if len(failing) > 0 {
		b.WriteString("<p>Failing (below 70):</p><ul>" + strings.Join(failing, "") + "</ul>")
	}
	if len(missing) > 0 {
		b.WriteString("<p>All missing work should be done as soon as possible.</p>")
	}
	b.WriteString("<p><strong>Major grades this week</strong></p>")
	var md bytes.Buffer
	_ = goldmark.Convert([]byte(r.Drafts.Announcements[course.Course]), &md)
	b.WriteString(md.String())
	fmt.Fprintf(&b, `<p><strong>Extra help</strong><br>Sign up for Bobcat Hour tutorials at <a href="%s">flex.securly.com</a>.</p>`, html.EscapeString(r.Config.BobcatHourURL))
	fmt.Fprintf(&b, "<p>Thank you,<br>%s</p></div>", html.EscapeString(r.Config.SignOff))
	return Rendered{Subject: subject, HTML: b.String()}
}
