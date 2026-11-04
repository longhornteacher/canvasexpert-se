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
	// The values are each teacher's own grade portal and tutoring sign-up. The
	// generic keys are canonical; the older skyward_url and bobcat_hour_* keys
	// still work and are folded into them by loadParentConfig.
	GradePortalURL  string `yaml:"grade_portal_url"`
	GradePortalName string `yaml:"grade_portal_name"`
	TutoringURL     string `yaml:"tutoring_url"`
	TutoringLabel   string `yaml:"tutoring_label"`
	TutoringText    string `yaml:"tutoring_text"`
	// ClassName names the class in the weekly update; blank gives a generic wording.
	ClassName string `yaml:"class_name"`
	// FailingBelow is the percent under which the weekly update lists work as
	// failing. Zero means the default, 70.
	FailingBelow int    `yaml:"failing_below"`
	SignOff      string `yaml:"sign_off"`
	SkipScore    int    `yaml:"skip_score"`

	// Older key names, read only when the matching generic key is blank.
	SkywardURL      string `yaml:"skyward_url"`
	BobcatHourURL   string `yaml:"bobcat_hour_url"`
	BobcatHourLabel string `yaml:"bobcat_hour_label"`
	BobcatHourTxt   string `yaml:"bobcat_hour_text"`
	// CorrectionDays is both the report window and how long after the due date a
	// correction stays open. Zero means the default, 14.
	CorrectionDays int `yaml:"correction_days"`
}

func loadParentConfig(root string) (ParentConfig, error) {
	var config ParentConfig
	if _, err := workspace.ReadTeacherDocument(root, "context/parent-updates.md", &config); err != nil && !os.IsNotExist(err) {
		return config, fmt.Errorf("context/parent-updates.md: %w", err)
	}
	for _, pair := range []struct {
		current *string
		legacy  string
		now     string
		old     string
	}{
		{&config.GradePortalURL, config.SkywardURL, "grade_portal_url", "skyward_url"},
		{&config.TutoringURL, config.BobcatHourURL, "tutoring_url", "bobcat_hour_url"},
		{&config.TutoringLabel, config.BobcatHourLabel, "tutoring_label", "bobcat_hour_label"},
		{&config.TutoringText, config.BobcatHourTxt, "tutoring_text", "bobcat_hour_text"},
	} {
		switch {
		case *pair.current == "":
			*pair.current = pair.legacy
		case pair.legacy != "" && pair.legacy != *pair.current:
			return config, fmt.Errorf("context/parent-updates.md: %s and %s are both set to different values; keep only %s", pair.now, pair.old, pair.now)
		}
	}
	if config.FailingBelow == 0 {
		config.FailingBelow = 70
	}
	if config.FailingBelow < 1 || config.FailingBelow > 100 {
		return config, fmt.Errorf("context/parent-updates.md: failing_below must be between 1 and 100")
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
	if config.TutoringLabel == "" {
		config.TutoringLabel = "Tutoring"
	}
	if config.TutoringText == "" {
		config.TutoringText = "Sign up for tutorials and clubs"
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

// validate keeps only checks that protect families or grade honesty: the
// drafts belong to this run, name students in it, and say send/skip/flag; a
// weekly update covers the whole period (its "no missing work in Term 2"
// line would otherwise be false) and its announcements hold no student or
// address details. Anything the drafts cannot match is simply not shown.
func (r *ParentRun) validate() error {
	d := r.Drafts
	if d.Kind != "parent-drafts" || !((d.Schema == 1 && d.Layout == "") || (d.Schema == 2 && d.Layout == "weekly_update")) {
		return fmt.Errorf("drafts.json must have schema 1 with the default layout, or schema 2 with layout weekly_update")
	}
	if d.RunID != r.Report.RunID {
		return fmt.Errorf("drafts.json run_id %q does not match the report run %q", d.RunID, r.Report.RunID)
	}
	if d.Layout == "weekly_update" {
		if !r.fullPeriod() {
			return fmt.Errorf("weekly update requires a full-period parent report; pull a fresh report with full_period")
		}
		if err := r.validateWeeklyAnnouncements(); err != nil {
			return err
		}
	}
	seen := map[string]bool{}
	for _, sd := range d.Students {
		if _, ok := r.students[sd.Ref]; !ok {
			return fmt.Errorf("student %s is not in the report", sd.Ref)
		}
		if seen[sd.Ref] {
			return fmt.Errorf("student %s appears twice", sd.Ref)
		}
		seen[sd.Ref] = true
		if sd.Decision != "send" && sd.Decision != "skip" && sd.Decision != "flag" {
			return fmt.Errorf("%s: decision must be send, skip or flag", sd.Ref)
		}
	}
	return nil
}

func (r *ParentRun) fullPeriod() bool {
	scope := r.Report.Scope
	if scope == nil {
		return false
	}
	loc, err := time.LoadLocation(scope.Timezone)
	if err != nil {
		return false
	}
	start, err := time.ParseInLocation("2006-01-02", scope.PeriodStartDate, loc)
	recent, e := time.Parse(time.RFC3339, scope.RecentStartAt)
	return err == nil && e == nil && start.Equal(recent)
}

var announcementEmail = regexp.MustCompile(`(?i)\b[A-Z0-9._%+\-]+@[A-Z0-9.\-]+\.[A-Z]{2,}\b`)

func (r *ParentRun) validateWeeklyAnnouncements() error {
	for _, course := range r.Report.Courses {
		text := r.Drafts.Announcements[course]
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
	return r.render(sd, student)
}

func noteFor(items []ParentItemRef, id int64) string {
	for _, it := range items {
		if it.AssignmentID == id {
			return it.Note
		}
	}
	return ""
}

func (r *ParentRun) render(sd ParentStudentDraft, student workspace.Student) Rendered {
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
		if i.URL == "" {
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
	fmt.Fprintf(&b, `<p style="margin:0 0 4px"><b>%s</b> &middot; As of %s</p>`, esc(student.FirstName), esc(asOf.In(loc).Format("Mon Jan 2, 3:04 pm")))
	if r.Report.Scope != nil {
		scope := r.Report.Scope
		recentStart, _ := time.Parse(time.RFC3339, scope.RecentStartAt)
		recentEnd, _ := time.Parse(time.RFC3339, scope.RecentEndAt)
		fmt.Fprintf(&b, `<p style="margin:0 0 8px;color:#52606d">%s · %s–%s · Recent window %s–%s</p>`, esc(scope.PeriodName), esc(scope.PeriodStartDate), esc(scope.PeriodEndDate), esc(recentStart.In(loc).Format("Jan 2")), esc(recentEnd.In(loc).Format("Jan 2")))
	}
	if intro != "" {
		fmt.Fprintf(&b, `<p style="margin:6px 0">%s</p>`, name(intro))
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
			status := workDueStatusText(it.Status)
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
		section("Missing/0", []string{"Assignment", "Due", ""}, miss)
		section("Corrections available", []string{"Assignment", "Correct by", "Where to look"}, corr)
		section("Due soon", []string{"Assignment", "Due", ""}, soon)
		section("Work due in this period", []string{"Assignment", "Due", "Status", "First score", "Current score"}, dueRows)
	}
	if r.Config.GradePortalURL != "" {
		fmt.Fprintf(&b, `<p style="margin:18px 0 4px"><a href="%s">%s</a> &mdash; official grades</p>`, esc(r.Config.GradePortalURL), esc(r.Config.GradePortalName))
	}
	if r.Config.TutoringURL != "" {
		fmt.Fprintf(&b, `<p style="margin:4px 0"><b>%s:</b> <a href="%s">%s</a>.</p>`, esc(r.Config.TutoringLabel), esc(r.Config.TutoringURL), esc(r.Config.TutoringText))
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

// failingBelow is the configured cutoff, or 70 when the config was built without loadParentConfig.
func (r *ParentRun) failingBelow() int {
	if r.Config.FailingBelow == 0 {
		return 70
	}
	return r.Config.FailingBelow
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
	className := strings.TrimSpace(r.Config.ClassName)
	subject := fmt.Sprintf("Class update – Week of %s %d", months[int(monday.Month())-1], monday.Day())
	greetingClass := "class"
	if className != "" {
		subject = fmt.Sprintf("%s Update – Week of %s %d", className, months[int(monday.Month())-1], monday.Day())
		greetingClass = className
	}
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
		if percent >= float64(r.failingBelow()) {
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
	b.WriteString(`<div style="font-family:Segoe UI,Arial,sans-serif;font-size:14px;line-height:1.4;color:#1f2933;max-width:640px"><p>Hello,</p><p>Here's a quick ` + html.EscapeString(greetingClass) + ` update.</p>`)
	fmt.Fprintf(&b, "<p><strong>Work that needs attention (%s)</strong></p>", html.EscapeString(r.Report.Scope.PeriodName))
	if len(missing)+len(failing) == 0 {
		fmt.Fprintf(&b, "<p>No missing or failing work in %s right now.</p>", html.EscapeString(r.Report.Scope.PeriodName))
	}
	if len(missing) > 0 {
		b.WriteString("<p>Missing:</p><ul>" + strings.Join(missing, "") + "</ul>")
	}
	if len(failing) > 0 {
		fmt.Fprintf(&b, "<p>Failing (below %d):</p><ul>%s</ul>", r.failingBelow(), strings.Join(failing, ""))
	}
	if len(missing) > 0 {
		b.WriteString("<p>All missing work should be done as soon as possible.</p>")
	}
	b.WriteString("<p><strong>Major grades this week</strong></p>")
	var md bytes.Buffer
	_ = goldmark.Convert([]byte(r.Drafts.Announcements[course.Course]), &md)
	b.WriteString(md.String())
	if r.Config.TutoringURL != "" {
		fmt.Fprintf(&b, `<p><strong>Extra help</strong><br>%s: <a href="%s">%s</a>.</p>`, html.EscapeString(r.Config.TutoringLabel), html.EscapeString(r.Config.TutoringURL), html.EscapeString(r.Config.TutoringText))
	}
	fmt.Fprintf(&b, "<p>Thank you,<br>%s</p></div>", html.EscapeString(r.Config.SignOff))
	return Rendered{Subject: subject, HTML: b.String()}
}
