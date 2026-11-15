package workflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"math/big"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"canvasexpertse/internal/workspace"
)

// trimPoint shows Canvas's "80.0" as "80"; other decimals are kept.
func trimPoint(v string) string { return strings.TrimSuffix(v, ".0") }

// ParentConfig is context/parent-updates.md front matter. These are teacher
// settings the agent reads and writes into its drafts; the Engine itself uses
// only sign_off (appended to emails) and grade_portal_name (printed sheet).
// Every key is declared so existing teacher files keep loading.
type ParentConfig struct {
	Schema          int    `yaml:"schema"`
	GradePortalURL  string `yaml:"grade_portal_url"`
	GradePortalName string `yaml:"grade_portal_name"`
	TutoringURL     string `yaml:"tutoring_url"`
	TutoringLabel   string `yaml:"tutoring_label"`
	TutoringText    string `yaml:"tutoring_text"`
	ClassName       string `yaml:"class_name"`
	FailingBelow    int    `yaml:"failing_below"`
	SignOff         string `yaml:"sign_off"`
	SkipScore       int    `yaml:"skip_score"`
	CorrectionDays  int    `yaml:"correction_days"`
	// Older key names, still accepted.
	SkywardURL      string `yaml:"skyward_url"`
	BobcatHourURL   string `yaml:"bobcat_hour_url"`
	BobcatHourLabel string `yaml:"bobcat_hour_label"`
	BobcatHourTxt   string `yaml:"bobcat_hour_text"`
}

func loadParentConfig(root string) (ParentConfig, error) {
	var config ParentConfig
	if _, err := workspace.ReadTeacherDocument(root, "context/parent-updates.md", &config); err != nil && !os.IsNotExist(err) {
		return config, fmt.Errorf("context/parent-updates.md: %w", err)
	}
	return config, nil
}

// ParentDrafts is the agent's email: its wording, the table's labels and the
// row threshold. The Engine supplies every fact in the table.
type ParentDrafts struct {
	Schema   int         `json:"schema"`
	Kind     string      `json:"kind"`
	RunID    string      `json:"run_id"`
	Subject  string      `json:"subject"`
	Body     string      `json:"body"`
	Table    ParentTable `json:"table"`
	SendWhen string      `json:"send_when,omitempty"` // rows (default) | all
	// Recipients "parents_only" leaves the student off the email.
	Recipients string               `json:"recipients,omitempty"`
	Students   []ParentStudentDraft `json:"students,omitempty"`
}

// ParentTable chooses rows and words; it carries no grade data.
type ParentTable struct {
	// BelowPercent lists rows whose released score is strictly below this
	// percent of points possible. Nil lists every row in the window.
	BelowPercent       *float64                   `json:"below_percent,omitempty"`
	From               string                     `json:"from,omitempty"`
	Through            string                     `json:"through,omitempty"`
	Courses            []string                   `json:"courses,omitempty"`
	Assignments        []ParentAssignmentSelector `json:"assignments,omitempty"`
	GradeStatuses      []string                   `json:"grade_statuses,omitempty"`
	SubmissionStatuses []string                   `json:"submission_statuses,omitempty"`
	PredicateMode      string                     `json:"predicate_mode,omitempty"` // any | all
	Columns            []string                   `json:"columns"`
	Labels             map[string]string          `json:"labels"`
	SubmissionLabels   map[string]string          `json:"submission_labels,omitempty"`
	EmptyText          string                     `json:"empty_text"`
	CourseHeadings     map[string]string          `json:"course_headings,omitempty"`
}

type ParentAssignmentSelector struct {
	Course       string `json:"course"`
	AssignmentID int64  `json:"assignment_id"`
}

// Reject misspelled selectors rather than silently broadening a message.
func (t *ParentTable) UnmarshalJSON(data []byte) error {
	type table ParentTable
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	return dec.Decode((*table)(t))
}

// ParentStudentDraft overrides the computed decision for one student. After
// loading, Drafts.Students holds every report student with its decision.
type ParentStudentDraft struct {
	Ref      string `json:"student_ref"`
	Decision string `json:"decision"` // send | skip | flag
	Reason   string `json:"reason,omitempty"`
	// StudentIntro opens the printable student sheet, written to the student.
	StudentIntro string `json:"student_intro,omitempty"`
}

var parentColumns = map[string]bool{"assignment": true, "score": true, "percent": true, "due": true, "submission": true}
var parentSubmissionValues = []string{"turned_in", "not_turned_in", "no_online_submission", "unknown"}

const assignmentTableToken = "{{assignment_table}}"

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

// LoadParentRun reads report.json and drafts.json from a run folder, validates
// the drafts and resolves every student's decision.
func LoadParentRun(root, dir string) (*ParentRun, error) {
	run := &ParentRun{Dir: dir, Root: root}
	for name, out := range map[string]any{"report.json": &run.Report, "drafts.json": &run.Drafts} {
		b, e := os.ReadFile(dir + string(os.PathSeparator) + name)
		if e != nil {
			return nil, fmt.Errorf("read %s: %w", name, e)
		}
		if name == "drafts.json" {
			if !json.Valid(b) {
				e = fmt.Errorf("invalid JSON")
			} else {
				decoder := json.NewDecoder(bytes.NewReader(b))
				decoder.DisallowUnknownFields()
				e = decoder.Decode(out)
			}
		} else {
			e = json.Unmarshal(b, out)
		}
		if e != nil {
			return nil, fmt.Errorf("%s is not valid JSON: %v", name, e)
		}
	}
	var err error
	if run.Config, err = loadParentConfig(root); err != nil {
		return nil, err
	}
	run.students = map[string]ParentReportStudent{}
	for _, s := range run.Report.Students {
		run.students[s.Ref] = s
	}
	if e := run.validate(); e != nil {
		return nil, e
	}
	run.resolveDecisions()
	return run, nil
}

// validate keeps only checks that protect families or grade honesty: the
// drafts belong to this run and its students, the table can be drawn, and the
// shared wording holds no student identifiers or addresses.
func (r *ParentRun) validate() error {
	d := r.Drafts
	if d.Kind != "parent-drafts" {
		return fmt.Errorf("drafts.json kind must be parent-drafts")
	}
	if d.Schema != 3 {
		return fmt.Errorf("this update was drafted in an older format; draft it again with the current parent-updates playbook (drafts.json schema 3)")
	}
	if r.Report.Schema != 2 {
		return fmt.Errorf("report.json is from an older Engine; pull a fresh parent report")
	}
	if d.RunID != r.Report.RunID {
		return fmt.Errorf("drafts.json run_id %q does not match the report run %q", d.RunID, r.Report.RunID)
	}
	if strings.TrimSpace(d.Subject) == "" || strings.TrimSpace(d.Body) == "" {
		return fmt.Errorf("drafts.json needs a subject and a body")
	}
	for _, m := range familyToken.FindAllStringSubmatch(d.Subject+"\n"+d.Body, -1) {
		if name := strings.TrimSpace(m[1]); name != "student_name" && name != "assignment_table" {
			return fmt.Errorf("unknown placeholder {{%s}}; use {{student_name}} or {{assignment_table}}", name)
		}
	}
	if strings.Contains(d.Subject, "assignment_table") {
		return fmt.Errorf("{{assignment_table}} belongs in the body, not the subject")
	}
	if n := strings.Count(d.Body, assignmentTableToken); n != 1 {
		return fmt.Errorf("the body must contain {{assignment_table}} exactly once, on its own line")
	}
	if d.SendWhen != "" && d.SendWhen != "rows" && d.SendWhen != "all" {
		return fmt.Errorf("send_when must be rows or all")
	}
	if d.Recipients != "" && d.Recipients != "parents_only" {
		return fmt.Errorf("recipients must be parents_only or omitted")
	}
	t := d.Table
	for _, date := range []string{t.From, t.Through} {
		if date != "" {
			if _, err := time.Parse("2006-01-02", date); err != nil {
				return fmt.Errorf("table date %q must be YYYY-MM-DD", date)
			}
		}
	}
	if t.From != "" && t.Through != "" && t.From > t.Through {
		return fmt.Errorf("table.from must not be after table.through")
	}
	if (t.From != "" || t.Through != "") && r.Report.Scope == nil {
		return fmt.Errorf("report has no date scope; pull a fresh report before selecting dates")
	}
	if (t.From != "" || t.Through != "") && r.Report.Scope != nil {
		scope := r.Report.Scope
		loc, zoneErr := time.LoadLocation(scope.Timezone)
		start, startErr := time.Parse(time.RFC3339, scope.RecentStartAt)
		end, endErr := time.Parse(time.RFC3339Nano, scope.RecentEndAt)
		if zoneErr != nil || startErr != nil || endErr != nil {
			return fmt.Errorf("report scope cannot establish the table's date coverage; pull a fresh report")
		}
		first, last := start.In(loc).Format("2006-01-02"), end.In(loc).Format("2006-01-02")
		if t.From != "" && (t.From < first || t.From > last) || t.Through != "" && (t.Through < first || t.Through > last) {
			return fmt.Errorf("table dates must fall within saved due inventory %s through %s; pull a report covering the requested range", first, last)
		}
	}
	if t.PredicateMode != "" && t.PredicateMode != "any" && t.PredicateMode != "all" {
		return fmt.Errorf("table.predicate_mode must be any or all")
	}
	for _, a := range t.Assignments {
		if strings.TrimSpace(a.Course) == "" || a.AssignmentID <= 0 {
			return fmt.Errorf("table.assignments requires course and positive assignment_id")
		}
		if !parentContains(r.Report.Courses, a.Course) {
			return fmt.Errorf("table assignment course %q is not in the report", a.Course)
		}
		found := false
		for _, student := range r.Report.Students {
			for _, course := range student.Courses {
				if course.Course == a.Course {
					for _, row := range course.WorkDue {
						if row.AssignmentID == a.AssignmentID {
							found = true
						}
					}
					for _, issue := range course.Issues {
						if issue.AssignmentID == a.AssignmentID {
							found = true
						}
					}
				}
			}
		}
		if !found {
			return fmt.Errorf("assignment %s/%d is not present in the saved due inventory; pull a report covering it", a.Course, a.AssignmentID)
		}
	}
	for _, course := range t.Courses {
		if !parentContains(r.Report.Courses, course) {
			return fmt.Errorf("table course %q is not in the report", course)
		}
	}
	for _, v := range t.GradeStatuses {
		if v != "graded" && v != "awaiting_grading" && v != "grade_not_released" && v != "no_grade" {
			return fmt.Errorf("unknown table grade status %q", v)
		}
	}
	for _, v := range t.SubmissionStatuses {
		if v != "turned_in" && v != "not_turned_in" && v != "no_online_submission" && v != "unknown" {
			return fmt.Errorf("unknown table submission status %q", v)
		}
	}
	if t.BelowPercent != nil && (*t.BelowPercent <= 0 || *t.BelowPercent > 1000) {
		return fmt.Errorf("table.below_percent must be a positive percent")
	}
	if len(t.Columns) == 0 {
		return fmt.Errorf("table.columns must name at least one of assignment, score, percent, due, submission")
	}
	seen := map[string]bool{}
	for _, c := range t.Columns {
		if !parentColumns[c] || seen[c] {
			return fmt.Errorf("table column %q is unknown or repeated; use assignment, score, percent, due, submission", c)
		}
		seen[c] = true
		if strings.TrimSpace(t.Labels[c]) == "" {
			return fmt.Errorf("table.labels needs a label for %s", c)
		}
		if c == "submission" {
			for _, v := range parentSubmissionValues {
				if strings.TrimSpace(t.SubmissionLabels[v]) == "" {
					return fmt.Errorf("table.submission_labels needs wording for %s", v)
				}
			}
		}
	}
	if strings.TrimSpace(t.EmptyText) == "" {
		return fmt.Errorf("table.empty_text is required")
	}
	if err := r.validatePrivacy(); err != nil {
		return err
	}
	overridden := map[string]bool{}
	for _, sd := range d.Students {
		if _, ok := r.students[sd.Ref]; !ok {
			return fmt.Errorf("student %s is not in the report", sd.Ref)
		}
		if overridden[sd.Ref] {
			return fmt.Errorf("student %s appears twice", sd.Ref)
		}
		overridden[sd.Ref] = true
		if sd.Decision != "send" && sd.Decision != "skip" && sd.Decision != "flag" {
			return fmt.Errorf("%s: decision must be send, skip or flag", sd.Ref)
		}
	}
	return nil
}

var announcementEmail = regexp.MustCompile(`(?i)\b[A-Z0-9._%+\-]+@[A-Z0-9.\-]+\.[A-Z]{2,}\b`)

// validatePrivacy keeps the shared wording free of student identifiers and
// addresses; names reach an email only through {{student_name}}.
func (r *ParentRun) validatePrivacy() error {
	t := r.Drafts.Table
	texts := []string{r.Drafts.Subject, r.Drafts.Body, t.EmptyText}
	for _, m := range []map[string]string{t.Labels, t.SubmissionLabels, t.CourseHeadings} {
		for _, v := range m {
			texts = append(texts, v)
		}
	}
	text := strings.Join(texts, "\n")
	if announcementEmail.MatchString(text) {
		return fmt.Errorf("parent update wording cannot contain email addresses")
	}
	for _, student := range r.Report.Students {
		for _, identifier := range []string{student.Ref, student.Alias} {
			if identifier != "" && regexp.MustCompile(`(?i)(^|[^\pL\pN])`+regexp.QuoteMeta(identifier)+`([^\pL\pN]|$)`).MatchString(text) {
				return fmt.Errorf("parent update wording cannot contain student identifiers; use {{student_name}}")
			}
		}
	}
	return nil
}

// resolveDecisions gives every report student a decision: the draft's
// override, else send_when. Incomplete evidence holds a send, so no email
// presents a partial list as complete.
func (r *ParentRun) resolveDecisions() {
	overrides := map[string]ParentStudentDraft{}
	for _, sd := range r.Drafts.Students {
		overrides[sd.Ref] = sd
	}
	var all []ParentStudentDraft
	for _, rs := range r.Report.Students {
		sd, ok := overrides[rs.Ref]
		if !ok {
			sd = ParentStudentDraft{Ref: rs.Ref, Decision: "send"}
			if r.Drafts.SendWhen != "all" && r.rowCount(rs) == 0 && r.parentHoldReason(rs) == "" {
				sd.Decision, sd.Reason = "skip", r.Drafts.Table.EmptyText
			}
		}
		if sd.Decision == "send" {
			if reason := r.parentHoldReason(rs); reason != "" {
				sd.Decision, sd.Reason = "flag", reason
			}
		}
		all = append(all, sd)
	}
	r.Drafts.Students = all
}

// parentHoldReason names evidence that would make an email incomplete or wrong.
func (r *ParentRun) parentHoldReason(s ParentReportStudent) string {
	if len(s.Courses) == 0 {
		return "The report has no course for this student."
	}
	for _, c := range s.Courses {
		if !r.parentCourseSelected(c.Course) {
			continue
		}
		if len(c.Flags) != 0 {
			if len(c.Issues) != len(c.Flags) {
				return "The report has unresolved course flags: " + strings.Join(c.Flags, "; ")
			}
			for i, issue := range c.Issues {
				if issue.Detail != c.Flags[i] || issue.AssignmentID <= 0 || r.parentAssignmentSelected(c.Course, issue.AssignmentID) {
					return "The report has unresolved course flags: " + issue.Detail
				}
			}
		}
		for _, item := range c.WorkDue {
			if !r.parentAssignmentSelected(c.Course, item.AssignmentID) {
				continue
			}
			if _, err := time.Parse(time.RFC3339, item.DueAt); err != nil {
				return "An assignment has no valid due date."
			}
			if !r.parentDateSelected(item.DueAt) {
				continue
			}
			u, err := url.Parse(item.URL)
			if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
				return "An assignment has no valid link."
			}
			if item.Current != nil {
				if _, err := strconv.ParseFloat(*item.Current, 64); err != nil {
					return "An assignment has an invalid score."
				}
			}
		}
	}
	return ""
}

func (r *ParentRun) parentCourseSelected(course string) bool {
	t := r.Drafts.Table
	if len(t.Courses) > 0 && !parentContains(t.Courses, course) {
		return false
	}
	if len(t.Assignments) == 0 {
		return true
	}
	for _, a := range t.Assignments {
		if a.Course == course {
			return true
		}
	}
	return false
}

func (r *ParentRun) parentAssignmentSelected(course string, id int64) bool {
	if len(r.Drafts.Table.Assignments) == 0 {
		return true
	}
	for _, a := range r.Drafts.Table.Assignments {
		if a.Course == course && a.AssignmentID == id {
			return true
		}
	}
	return false
}

func (r *ParentRun) parentDateSelected(raw string) bool {
	t := r.Drafts.Table
	if t.From == "" && t.Through == "" {
		return true
	}
	due, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return false
	}
	date := due.Format("2006-01-02")
	return (t.From == "" || date >= t.From) && (t.Through == "" || date <= t.Through)
}

// tableRows returns the course's rows the draft selects, in due-date order.
func (r *ParentRun) tableRows(c ParentReportCourse) []ParentWorkDueItem {
	var out []ParentWorkDueItem
	t := r.Drafts.Table
	limit := t.BelowPercent
	var threshold *big.Rat
	if limit != nil {
		threshold = new(big.Rat).SetFloat64(*limit)
	}
	for _, item := range c.WorkDue {
		if len(t.Courses) > 0 && !parentContains(t.Courses, c.Course) {
			continue
		}
		if len(t.Assignments) > 0 {
			found := false
			for _, a := range t.Assignments {
				if a.Course == c.Course && a.AssignmentID == item.AssignmentID {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		if t.From != "" || t.Through != "" {
			due, err := time.Parse(time.RFC3339, item.DueAt)
			if err != nil {
				continue
			}
			date := due.Format("2006-01-02")
			if t.From != "" && date < t.From || t.Through != "" && date > t.Through {
				continue
			}
		}
		var predicates []bool
		if len(t.GradeStatuses) > 0 {
			predicates = append(predicates, parentContains(t.GradeStatuses, item.Status))
		}
		if len(t.SubmissionStatuses) > 0 {
			predicates = append(predicates, parentContains(t.SubmissionStatuses, item.Submission))
		}
		if threshold != nil {
			match := false
			if item.Status == "graded" && item.Current != nil {
				p, ok := percentRat(*item.Current, item.Possible)
				match = ok && p.Cmp(threshold) < 0
			}
			predicates = append(predicates, match)
		}
		if len(predicates) > 0 {
			match := t.PredicateMode != "any"
			for _, p := range predicates {
				if t.PredicateMode == "any" {
					match = match || p
				} else {
					match = match && p
				}
			}
			if !match {
				continue
			}
		}
		out = append(out, item)
	}
	return out // WorkDue is already in due-date, then title, order
}

func parentContains(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}

func (r *ParentRun) rowCount(s ParentReportStudent) int {
	n := 0
	for _, c := range s.Courses {
		n += len(r.tableRows(c))
	}
	return n
}

// Rendered is one student's email.
type Rendered struct {
	Subject, HTML string
}

const (
	cellStyle  = `style="padding:4px 10px 4px 0;vertical-align:top;border-bottom:1px solid #e4e7eb"`
	headStyle  = `style="padding:4px 10px 4px 0;text-align:left;color:#52606d;font-weight:600;border-bottom:1px solid #9aa5b1"`
	linkFormat = `<a href="%s">%s</a>`
)

// Render builds one student's email: the draft's wording with the Engine's
// assignment table in place of {{assignment_table}}.
func (r *ParentRun) Render(sd ParentStudentDraft, student workspace.Student) (Rendered, error) {
	msg := FamilyMessage{Subject: r.Drafts.Subject, Body: r.Drafts.Body}
	out, _, err := renderFamily(r.Dir, msg, student.FirstName, "", r.Config.SignOff, r.ReplyNotes[sd.Ref])
	if err != nil {
		return Rendered{}, err
	}
	table := r.renderTable(r.students[sd.Ref])
	body := out.HTML
	switch {
	case strings.Count(body, "<p>"+assignmentTableToken+"</p>") == 1:
		body = strings.Replace(body, "<p>"+assignmentTableToken+"</p>", table, 1)
	case strings.Count(body, assignmentTableToken) == 1:
		return Rendered{}, fmt.Errorf("put {{assignment_table}} on its own line in the body")
	default:
		return Rendered{}, fmt.Errorf("the body must contain {{assignment_table}} exactly once")
	}
	return Rendered{Subject: out.Subject, HTML: body}, nil
}

func (r *ParentRun) renderTable(rs ParentReportStudent) string {
	t := r.Drafts.Table
	esc := html.EscapeString
	var b strings.Builder
	found := false
	multi := len(rs.Courses) > 1
	for _, c := range rs.Courses {
		rows := r.tableRows(c)
		if len(rows) == 0 {
			continue
		}
		found = true
		loc, err := time.LoadLocation(c.Timezone)
		if err != nil {
			loc = time.UTC
		}
		if multi {
			heading := c.Course
			if h := strings.TrimSpace(t.CourseHeadings[c.Course]); h != "" {
				heading = h
			}
			fmt.Fprintf(&b, `<h3 style="margin:18px 0 6px;font-size:15px">%s</h3>`, esc(heading))
		}
		b.WriteString(`<table style="border-collapse:collapse;width:100%;margin:8px 0"><tr>`)
		for _, col := range t.Columns {
			fmt.Fprintf(&b, `<th %s>%s</th>`, headStyle, esc(t.Labels[col]))
		}
		b.WriteString(`</tr>`)
		for _, it := range rows {
			b.WriteString(`<tr>`)
			for _, col := range t.Columns {
				fmt.Fprintf(&b, `<td %s>%s</td>`, cellStyle, parentCell(col, it, loc, t))
			}
			b.WriteString(`</tr>`)
		}
		b.WriteString(`</table>`)
	}
	if !found {
		return "<p>" + esc(t.EmptyText) + "</p>"
	}
	return b.String()
}

func parentCell(col string, it ParentWorkDueItem, loc *time.Location, t ParentTable) string {
	esc := html.EscapeString
	switch col {
	case "assignment":
		if it.URL == "" {
			return esc(it.Title)
		}
		return fmt.Sprintf(linkFormat, esc(it.URL), esc(it.Title))
	case "score":
		if it.Current == nil {
			return ""
		}
		if it.Possible != "" {
			return esc(trimPoint(*it.Current) + " / " + trimPoint(it.Possible))
		}
		return esc(trimPoint(*it.Current))
	case "percent":
		if it.Current == nil {
			return ""
		}
		p, ok := percentRat(*it.Current, it.Possible)
		if !ok {
			return ""
		}
		return esc(p.FloatString(0) + "%")
	case "due":
		d, err := time.Parse(time.RFC3339, it.DueAt)
		if err != nil {
			return ""
		}
		return esc(d.In(loc).Format("Mon Jan 2"))
	case "submission":
		return esc(t.SubmissionLabels[it.Submission])
	}
	return ""
}
