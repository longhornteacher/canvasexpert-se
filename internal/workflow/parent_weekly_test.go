package workflow

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"canvasexpertse/internal/workspace"
)

func TestWeeklyUpdateRendering(t *testing.T) {
	now := time.Date(2026, 10, 26, 21, 0, 0, 0, time.UTC)
	row := func(id int64, title, status, score string) ParentWorkDueItem {
		item := ParentWorkDueItem{ParentReportItem: ParentReportItem{AssignmentID: id, Title: title, URL: "https://example.edu/courses/1/assignments/1", DueAt: "2026-10-20T21:00:00Z"}, Status: status, Possible: "100"}
		if score != "" {
			item.Current = str(score)
		}
		return item
	}
	rows := []ParentWorkDueItem{row(1, "Zero submitted", "recorded_zero", "0"), row(2, "Zero late", "recorded_zero", "0"), row(3, "Blank", "missing", ""), row(4, "Fifty-five", "graded", "55"), row(5, "Decimal", "graded", "69.5"), row(6, "Seventy", "graded", "70"), row(7, "Late", "graded", "62"), row(8, "Hidden", "grade_not_released", "40"), row(9, "Stale missing", "missing", "45"), row(10, "Excused", "excused", "0")}
	rows[1].PointsDeducted = str("8")
	rows[6].PointsDeducted = str("8")
	course := ParentReportCourse{Course: "ELA7", Timezone: "America/Chicago", WorkDue: rows}
	student := ParentReportStudent{Ref: "s-123", Alias: "Wren", Courses: []ParentReportCourse{course}}
	run := &ParentRun{Report: ParentReportPacket{AsOf: now.Format(time.RFC3339), Scope: parentTestScope(now)}, Drafts: ParentDrafts{Layout: "weekly_update", Announcements: map[string]string{"ELA7": "- **Thursday:** [Essay](https://example.edu/courses/1/assignments/2)."}}, Config: ParentConfig{SignOff: "Your teacher", ClassName: "Science 6", TutoringURL: "https://tutoring.example/signup", TutoringLabel: "Tutoring", TutoringText: "Sign up for tutorials and clubs"}, students: map[string]ParentReportStudent{student.Ref: student}}
	rendered := run.Render(ParentStudentDraft{Ref: student.Ref}, workspace.Student{Name: "Secret Name", FirstName: "Secret"})
	if rendered.Subject != "Science 6 Update – Week of Oct. 26" || !strings.Contains(rendered.HTML, "Here's a quick Science 6 update") || !strings.Contains(rendered.HTML, `<a href="https://tutoring.example/signup">Sign up for tutorials and clubs</a>`) || !strings.Contains(rendered.HTML, "Failing (below 70)") {
		t.Fatal(rendered.Subject)
	}
	for _, s := range []string{"Zero submitted</a> – due 10/20", "Zero late</a> – due 10/20", "Blank</a> – due 10/20", "Fifty-five</a> – 55", "Decimal</a> – 69.5", "Late</a> – 62 (late points deducted)", "Stale missing</a> – 45", "<strong>Thursday:</strong>"} {
		if !strings.Contains(rendered.HTML, s) {
			t.Errorf("missing %q in %s", s, rendered.HTML)
		}
	}
	for _, s := range []string{"Seventy</a>", "Hidden</a>", "Excused</a>", "Secret", "Wren", "s-123", "Zero late</a> – 0", "Work due in this period"} {
		if strings.Contains(rendered.HTML, s) || strings.Contains(rendered.Subject, s) {
			t.Errorf("unexpected %q", s)
		}
	}
	course.WorkDue = []ParentWorkDueItem{row(6, "Seventy", "graded", "70")}
	student.Courses = []ParentReportCourse{course}
	run.students[student.Ref] = student
	empty := run.Render(ParentStudentDraft{Ref: student.Ref}, workspace.Student{}).HTML
	if !strings.Contains(empty, "No missing or failing work in Term 2 right now.") || strings.Contains(empty, "Missing:</p>") {
		t.Fatal(empty)
	}
}

func TestWeeklyAnnouncementIdentifiers(t *testing.T) {
	run := &ParentRun{Report: ParentReportPacket{Courses: []string{"ELA7"}, Students: []ParentReportStudent{{Ref: "s-123", Alias: "Wren"}}}, Drafts: ParentDrafts{Announcements: map[string]string{"ELA7": "- A shared announcement"}}}
	if e := run.validateWeeklyAnnouncements(); e != nil {
		t.Fatal(e)
	}
	for _, text := range []string{"Hello Wren", "For s-123", "contact teacher@example.edu"} {
		run.Drafts.Announcements["ELA7"] = text
		if run.validateWeeklyAnnouncements() == nil {
			t.Errorf("accepted %q", text)
		}
	}
}

func TestWeeklyReviewUsesParentsOnlyAndHoldsIncomplete(t *testing.T) {
	svc, dir, a, b := parentFixtureRun(t)
	config := "---\nschema: 1\nsign_off: Your teacher\ntutoring_url: https://tutoring.example/signup\n---\n"
	if err := os.MkdirAll(filepath.Join(svc.Root, "context"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(svc.Root, "context", "parent-updates.md"), []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	var report ParentReportPacket
	data, _ := os.ReadFile(filepath.Join(dir, "report.json"))
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatal(err)
	}
	report.Students[1].Courses[0].Flags = []string{"incomplete"}
	data, _ = json.Marshal(report)
	if err := os.WriteFile(filepath.Join(dir, "report.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	drafts := ParentDrafts{Schema: 2, Kind: "parent-drafts", RunID: "R1", Layout: "weekly_update", Announcements: map[string]string{"cs8": "- Announcement"}, Students: []ParentStudentDraft{{Ref: a.Ref, Decision: "send"}, {Ref: b.Ref, Decision: "send"}}}
	data, _ = json.Marshal(drafts)
	if err := os.WriteFile(filepath.Join(dir, "drafts.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	review, err := svc.BuildParentReview(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, card := range review.Cards {
		if card.Ref == a.Ref {
			if len(card.Frames) != 1 || !strings.Contains(card.Frames[0].HTML, "Here's a quick class update") {
				t.Fatalf("unexpected preview %+v", card)
			}
			for _, fact := range card.Facts {
				if strings.Contains(fact.Value, "ada@student.example") {
					t.Fatal("student recipient included")
				}
			}
		}
		if card.Ref == b.Ref && card.Hold == "" {
			t.Fatal("incomplete student not held")
		}
	}
	sender := &fakeSender{}
	_, _, err = svc.SendParentUpdates(context.Background(), dir, nil, true, sender)
	if err != nil {
		t.Fatal(err)
	}
	if len(sender.sent) != 0 || len(sender.drafts) != 1 || strings.Join(sender.drafts[0].To, ",") != "p1@example.org,p3@example.org" || len(sender.drafts[0].Cc) != 0 {
		t.Fatalf("drafts=%+v sent=%+v", sender.drafts, sender.sent)
	}
}

func weeklyTestRun(config ParentConfig) (*ParentRun, ParentStudentDraft) {
	now := time.Date(2026, 10, 26, 21, 0, 0, 0, time.UTC)
	item := ParentWorkDueItem{ParentReportItem: ParentReportItem{AssignmentID: 1, Title: "Sixty-six", URL: "https://example.edu/courses/1/assignments/1", DueAt: "2026-10-20T21:00:00Z"}, Status: "graded", Possible: "100", Current: str("66")}
	course := ParentReportCourse{Course: "SCI6", Timezone: "America/Chicago", WorkDue: []ParentWorkDueItem{item}}
	student := ParentReportStudent{Ref: "s-1", Alias: "Wren", Courses: []ParentReportCourse{course}}
	run := &ParentRun{Report: ParentReportPacket{AsOf: now.Format(time.RFC3339), Scope: parentTestScope(now)}, Drafts: ParentDrafts{Layout: "weekly_update", Announcements: map[string]string{"SCI6": "- Quiz Friday."}}, Config: config, students: map[string]ParentReportStudent{student.Ref: student}}
	return run, ParentStudentDraft{Ref: student.Ref}
}

func TestWeeklyUpdateGenericDefaultsAndConfiguredCutoff(t *testing.T) {
	// Blank class name and tutoring URL: generic wording, no Extra help block.
	run, draft := weeklyTestRun(ParentConfig{SignOff: "Your teacher"})
	got := run.Render(draft, workspace.Student{})
	if got.Subject != "Class update – Week of Oct. 26" || !strings.Contains(got.HTML, "Here's a quick class update.") || strings.Contains(got.HTML, "Extra help") {
		t.Fatalf("generic render: %s / %s", got.Subject, got.HTML)
	}
	if !strings.Contains(got.HTML, "Sixty-six</a> – 66") {
		t.Fatalf("66%% should fail at the default cutoff: %s", got.HTML)
	}
	// failing_below: 65 moves a 66% row out of Failing.
	run, draft = weeklyTestRun(ParentConfig{SignOff: "Your teacher", FailingBelow: 65})
	got = run.Render(draft, workspace.Student{})
	if strings.Contains(got.HTML, "Sixty-six") || strings.Contains(got.HTML, "Failing (below") {
		t.Fatalf("cutoff 65: %s", got.HTML)
	}
}

func parentConfigFrom(t *testing.T, yaml string) (ParentConfig, error) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "context"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "context", "parent-updates.md"), []byte("---\nschema: 1\n"+yaml+"---\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return loadParentConfig(root)
}

func TestParentConfigOldKeysStillWorkAndConflictsFail(t *testing.T) {
	old, err := parentConfigFrom(t, "skyward_url: https://portal.example/\nbobcat_hour_url: https://tutoring.example/signup\nbobcat_hour_label: Tutorials\nbobcat_hour_text: Join us\n")
	if err != nil || old.GradePortalURL != "https://portal.example/" || old.TutoringURL != "https://tutoring.example/signup" || old.TutoringLabel != "Tutorials" || old.TutoringText != "Join us" || old.FailingBelow != 70 {
		t.Fatalf("old keys: %+v %v", old, err)
	}
	same, err := parentConfigFrom(t, "tutoring_url: https://tutoring.example/signup\nbobcat_hour_url: https://tutoring.example/signup\n")
	if err != nil || same.TutoringURL != "https://tutoring.example/signup" {
		t.Fatalf("agreeing old and new keys: %+v %v", same, err)
	}
	_, err = parentConfigFrom(t, "tutoring_url: https://tutoring.example/a\nbobcat_hour_url: https://tutoring.example/b\n")
	if err == nil || !strings.Contains(err.Error(), "tutoring_url") || !strings.Contains(err.Error(), "bobcat_hour_url") {
		t.Fatalf("conflict not named: %v", err)
	}
	if _, err = parentConfigFrom(t, "failing_below: 150\n"); err == nil {
		t.Fatal("failing_below out of range accepted")
	}
}
