package workflow

import (
	"encoding/json"
	"fmt"
	"html"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"canvasexpertse/internal/workspace"
)

// trimPoint shows Canvas's "80.0" as "80"; other decimals are kept.
func trimPoint(v string) string { return strings.TrimSuffix(v, ".0") }

// ParentConfig is context/parent-updates.md front matter.
type ParentConfig struct {
	Schema        int    `yaml:"schema"`
	// The YAML keys keep their original names for compatibility; the values
	// are each teacher's own grade portal and tutoring sign-up.
	SkywardURL      string `yaml:"skyward_url"`
	GradePortalName string `yaml:"grade_portal_name"`
	BobcatHourURL   string `yaml:"bobcat_hour_url"`
	BobcatHourLabel string `yaml:"bobcat_hour_label"`
	BobcatHourTxt   string `yaml:"bobcat_hour_text"`
	SignOff       string `yaml:"sign_off"`
	SkipScore     int    `yaml:"skip_score"`
}

// ParentDrafts is the agent's decisions and short instruction text. Assignment
// IDs refer to the report; titles, links, dates and scores come from it.
type ParentDrafts struct {
	Schema   int                  `json:"schema"`
	Kind     string               `json:"kind"`
	RunID    string               `json:"run_id"`
	Students []ParentStudentDraft `json:"students"`
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
	if _, e := workspace.ReadTeacherDocument(root, "context/parent-updates.md", &run.Config); e != nil && !os.IsNotExist(e) {
		return nil, fmt.Errorf("context/parent-updates.md: %w", e)
	}
	if run.Config.GradePortalName == "" {
		run.Config.GradePortalName = "the school's grade portal"
	}
	if run.Config.BobcatHourLabel == "" {
		run.Config.BobcatHourLabel = "Tutoring"
	}
	if run.Config.BobcatHourTxt == "" {
		run.Config.BobcatHourTxt = "Sign up for tutorials and clubs"
	}
	run.students = map[string]ParentReportStudent{}
	for _, s := range run.Report.Students {
		run.students[s.Ref] = s
	}
	if e := run.validate(); e != nil {
		return nil, e
	}
	return run, nil
}

var scoreLike = regexp.MustCompile(`\d\s*/\s*\d|%|\bout of\b|\bpoints?\b`)

func (r *ParentRun) validate() error {
	d := r.Drafts
	if d.Schema != 1 || d.Kind != "parent-drafts" {
		return fmt.Errorf("drafts.json must have schema 1 and kind parent-drafts")
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
		if e := checkFreeText(sd.Ref, sd.StudentIntro); e != nil {
			return e
		}
		switch sd.Decision {
		case "send":
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
		total := 0
		for _, sec := range sections {
			for _, it := range sec.items {
				total++
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
		if total == 0 {
			return fmt.Errorf("%s: a send decision needs at least one item", sd.Ref)
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
		history := append([]ParentHistoryItem(nil), c.History...)
		sort.SliceStable(history, func(i, j int) bool { return history[i].DueAt < history[j].DueAt })
		for _, it := range history {
			all.History = append(all.History, ParentItemRef{AssignmentID: it.AssignmentID})
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
		if missing == 0 {
			intro = strings.TrimSpace(intro + " Nothing is missing. Keep it up.")
		}
	} else {
		fmt.Fprintf(&b, `<p style="margin:0 0 4px"><b>%s</b> &middot; As of %s</p>`, esc(student.FirstName), esc(asOf.In(loc).Format("Mon Jan 2, 3:04 pm")))
	}
	if intro != "" {
		fmt.Fprintf(&b, `<p style="margin:6px 0">%s</p>`, name(intro))
	}
	if forStudent {
		fmt.Fprintf(&b, `<p style="margin:6px 0;color:#52606d">Find each assignment in Canvas by its title.</p>`)
	}
	multi := len(rs.Courses) > 1
	for _, c := range rs.Courses {
		var miss, corr, soon, hist []string
		for _, it := range sortedBy(c.Missing, sd.Missing, order) {
			miss = append(miss, fmt.Sprintf(`<tr><td %s>%s</td><td %s>%s</td><td %s>%s</td></tr>`, cellStyle, title(it), cellStyle, esc(day(it.DueAt)), cellStyle, noteOf(sd.Missing, it.AssignmentID)))
		}
		for _, it := range sortedCorr(c.Corrections, sd.Corrections, order) {
			corr = append(corr, fmt.Sprintf(`<tr><td %s>%s</td><td %s>%s</td><td %s>%s</td></tr>`, cellStyle, title(it.ParentReportItem), cellStyle, esc(day(it.CorrectionDeadline)), cellStyle, noteOf(sd.Corrections, it.AssignmentID)))
		}
		for _, it := range sortedBy(c.DueSoon, sd.DueSoon, order) {
			soon = append(soon, fmt.Sprintf(`<tr><td %s>%s</td><td %s>%s</td><td %s>%s</td></tr>`, cellStyle, title(it), cellStyle, esc(day(it.DueAt)), cellStyle, noteOf(sd.DueSoon, it.AssignmentID)))
		}
		for _, it := range sortedHist(c.History, sd.History, order) {
			hist = append(hist, fmt.Sprintf(`<tr><td %s>%s</td><td %s>%s</td><td %s>%s</td></tr>`, cellStyle, title(it.ParentReportItem), cellStyle, score(it.First, it.Possible), cellStyle, score(it.Current, it.Possible)))
		}
		if len(miss)+len(corr)+len(soon)+len(hist) == 0 {
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
			section("Missing: turn these in", []string{"Assignment", "Was due", ""}, miss)
			section("Corrections you can still make", []string{"Assignment", "Correct by", "Where to look"}, corr)
			section("Due soon", []string{"Assignment", "Due", ""}, soon)
			section("Your grade history", []string{"Assignment", "First Score", "Current Score"}, hist)
			continue
		}
		section("Missing / not turned in", []string{"Assignment", "Due", ""}, miss)
		section("Corrections available", []string{"Assignment", "Correct by", "Where to look"}, corr)
		section("Due soon", []string{"Assignment", "Due", ""}, soon)
		section("Grade history", []string{"Assignment", "First Score", "Current Score"}, hist)
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
