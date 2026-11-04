package workflow

import (
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"canvasexpertse/internal/workspace"
)

// The printed grade sheet is one table per course: assignment, due date and
// current grade, with a check box for work not yet due. It needs only the
// saved report.json; drafts.json (for an optional student_intro) and the
// teacher's parent-update settings (for the footer) are used when readable
// and never block printing.

func workDueStatusText(status string) string {
	switch status {
	case "awaiting_grading":
		return "Awaiting grading"
	case "grade_not_released":
		return "Grade not released"
	case "no_grade":
		return "No grade recorded"
	case "recorded_zero":
		return "Recorded zero"
	case "not_submitted":
		return "Not submitted"
	case "missing":
		return "Missing"
	case "graded":
		return "Graded"
	}
	return status
}

// loadParentRunForPrint reads only what a printed grade sheet needs.
func loadParentRunForPrint(root, dir string) (*ParentRun, error) {
	run := &ParentRun{Dir: dir, Root: root, students: map[string]ParentReportStudent{}}
	b, err := os.ReadFile(filepath.Join(dir, "report.json"))
	if err != nil {
		return nil, fmt.Errorf("read report.json: %w", err)
	}
	if err = json.Unmarshal(b, &run.Report); err != nil {
		return nil, fmt.Errorf("report.json is not valid JSON: %v", err)
	}
	for _, s := range run.Report.Students {
		run.students[s.Ref] = s
	}
	if b, err = os.ReadFile(filepath.Join(dir, "drafts.json")); err == nil {
		_ = json.Unmarshal(b, &run.Drafts) // optional: intros only
	}
	run.Config, _ = loadParentConfig(root) // optional: footer only
	return run, nil
}

// RenderStudent builds the printed grade sheet for one student.
func (r *ParentRun) RenderStudent(sd ParentStudentDraft, student workspace.Student) Rendered {
	rs := r.students[sd.Ref]
	esc := html.EscapeString
	loc := time.UTC
	if len(rs.Courses) > 0 {
		if l, e := time.LoadLocation(rs.Courses[0].Timezone); e == nil {
			loc = l
		}
	}
	day := func(v string) string {
		t, e := time.Parse(time.RFC3339, v)
		if e != nil {
			return ""
		}
		return t.In(loc).Format("Mon Jan 2")
	}
	var courses []string
	for _, c := range rs.Courses {
		courses = append(courses, c.Course)
	}
	var b strings.Builder
	b.WriteString(`<div style="font-family:Segoe UI,Arial,sans-serif;font-size:14px;line-height:1.4;color:#1f2933;max-width:640px">`)
	header := esc(student.Name) + "</b> &middot; " + esc(strings.Join(courses, ", "))
	if asOf, e := time.Parse(time.RFC3339, r.Report.AsOf); e == nil {
		header += " &middot; As of " + esc(asOf.In(loc).Format("Mon Jan 2"))
	}
	fmt.Fprintf(&b, `<p style="margin:0 0 4px;font-size:17px"><b>%s</p>`, header)
	if scope := r.Report.Scope; scope != nil && scope.PeriodName != "" {
		fmt.Fprintf(&b, `<p style="margin:0 0 8px;color:#52606d">%s · %s–%s</p>`, esc(scope.PeriodName), esc(scope.PeriodStartDate), esc(scope.PeriodEndDate))
	}
	if sd.StudentIntro != "" {
		fmt.Fprintf(&b, `<p style="margin:6px 0">%s</p>`, esc(workspace.ReplaceStudentName(sd.StudentIntro, student)))
	}
	type row struct{ due, html string }
	line := func(title, due, grade string) string {
		return fmt.Sprintf(`<tr><td %s>%s</td><td %s>%s</td><td %s>%s</td></tr>`, cellStyle, esc(title), cellStyle, esc(day(due)), cellStyle, grade)
	}
	for _, c := range rs.Courses {
		var rows []row
		for _, it := range c.WorkDue {
			grade := workDueStatusText(it.Status)
			if it.Current != nil && it.Status != "grade_not_released" {
				grade = trimPoint(*it.Current)
				if it.Possible != "" {
					grade += " / " + trimPoint(it.Possible)
				}
			}
			rows = append(rows, row{it.DueAt, line(it.Title, it.DueAt, esc(grade))})
		}
		for _, it := range c.Upcoming {
			box := "&#9744; (check this off once completed)"
			if it.TurnedIn {
				box += `<br><span style="color:#52606d">Canvas shows this turned in</span>`
			}
			rows = append(rows, row{it.DueAt, line(it.Title, it.DueAt, box)})
		}
		if len(rows) == 0 {
			continue
		}
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].due < rows[j].due })
		if len(rs.Courses) > 1 {
			fmt.Fprintf(&b, `<h2 style="margin:20px 0 0;font-size:16px;border-top:2px solid #9aa5b1;padding-top:8px">%s</h2>`, esc(c.Course))
		}
		fmt.Fprintf(&b, `<table style="border-collapse:collapse;width:100%%;margin-top:10px"><tr><th %s>Assignment</th><th %s>Due</th><th %s>Current grade</th></tr>`, headStyle, headStyle, headStyle)
		for _, rw := range rows {
			b.WriteString(rw.html)
		}
		b.WriteString(`</table>`)
	}
	portal := "the grade portal"
	if r.Config.GradePortalName != "" {
		portal = r.Config.GradePortalName
		fmt.Fprintf(&b, `<p style="margin:18px 0 4px">Official grades: %s.</p>`, esc(portal))
	}
	b.WriteString(aiNotice("This sheet was prepared with AI tools, so there may be mistakes. Scores and dates come straight from Canvas; check " + portal + " for official grades. For privacy, the AI never saw your real name."))
	b.WriteString(`</div>`)
	return Rendered{Subject: student.Name, HTML: b.String()}
}
