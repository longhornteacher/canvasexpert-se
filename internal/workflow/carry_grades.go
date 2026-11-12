package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

type CarryGradesRow struct {
	StudentRef string `json:"student_ref"`
	Alias      string `json:"alias"`
	Status     string `json:"status"` // write, unchanged, held (preview); written, unchanged, held
	Reason     string `json:"reason,omitempty"`
	SourceID   int64  `json:"source_id,omitempty"`
	Source     string `json:"source,omitempty"`  // "44/50" in the source's own points
	Current    string `json:"current,omitempty"` // the target's entered grade now
	Value      string `json:"value,omitempty"`   // the grade to enter on the target
}

type CarryGradesSource struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Points string `json:"points"`
	Scored int    `json:"scored"` // students with an entered score here
}

type CarryGradesReport struct {
	Course      string              `json:"course"`
	Preview     bool                `json:"preview"`
	TargetID    int64               `json:"target_id"`
	TargetName  string              `json:"target_name,omitempty"`
	Sources     []CarryGradesSource `json:"sources,omitempty"`
	Counts      map[string]int      `json:"counts,omitempty"`
	PreviewFile string              `json:"preview_file,omitempty"`
	Rows        []CarryGradesRow    `json:"students"`
	Notes       []string            `json:"notes,omitempty"`
}

// carrySavedRow is what the teacher reviewed for one student: the value to
// enter and the target's entered grade at that moment.
type carrySavedRow struct {
	Ref     string `json:"ref"`
	Value   string `json:"value"`
	Current string `json:"current"`
}

type carrySaved struct {
	TargetID  int64           `json:"target_id"`
	SourceIDs []int64         `json:"source_ids"`
	Rows      []carrySavedRow `json:"rows"`
}

func carryPreviewPath(root, nickname string, targetID int64) (string, error) {
	return workspace.ConstrainPath(root, filepath.Join("courses", nickname, "drafts", fmt.Sprintf("carry-grades-%d.json", targetID)))
}

// CarryGrades copies each student's entered score from the one source
// assignment they have a score on into the target column, scaled to the
// target's points. It never lowers a target grade, never enters a grade for a
// student without exactly one source value, and writes only values saved by a
// reviewed preview on this PC.
func (s *Service) CarryGrades(ctx context.Context, nickname string, targetID int64, sourceIDs []int64, preview bool) (CarryGradesReport, error) {
	report := CarryGradesReport{Preview: preview, TargetID: targetID}
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
		return report, fmt.Errorf("this course is team-taught (owner set); carrying grades is not supported there yet, so nothing was written")
	}
	if len(sourceIDs) == 0 {
		return report, fmt.Errorf("name at least one source assignment")
	}
	sourceIDs = append([]int64(nil), sourceIDs...)
	sort.Slice(sourceIDs, func(i, j int) bool { return sourceIDs[i] < sourceIDs[j] })
	for i, id := range sourceIDs {
		if id == targetID {
			return report, fmt.Errorf("assignment %d is both the target and a source", id)
		}
		if i > 0 && sourceIDs[i-1] == id {
			return report, fmt.Errorf("source assignment %d is listed twice", id)
		}
	}
	target, err := GetAssignment(ctx, s.Client, course.ID, targetID)
	if err != nil {
		return report, err
	}
	report.TargetName = target.Name
	switch {
	case target.NewQuiz || target.QuizID != nil:
		return report, fmt.Errorf("%q is a quiz; Canvas computes its grade from the quiz, so carry grades into an ordinary column (an on-paper or no-submission assignment)", target.Name)
	case target.Anonymous || target.Moderated:
		return report, fmt.Errorf("%q uses anonymous or moderated grading, which isn't supported", target.Name)
	case target.GradingType != "" && target.GradingType != "points":
		return report, fmt.Errorf("%q is graded as %s; the target column must be graded in points", target.Name, target.GradingType)
	case target.GroupCategoryID != nil && *target.GroupCategoryID > 0 && (target.GradeGroupStudentsIndividually == nil || !*target.GradeGroupStudentsIndividually):
		return report, fmt.Errorf("%q is graded as a shared group grade; one student's score must not be entered for the whole group", target.Name)
	case target.InClosedGradingPeriod:
		return report, fmt.Errorf("%q is in a closed grading period", target.Name)
	}
	targetPoints, ok := new(big.Rat).SetString(target.Points.String())
	if !ok || targetPoints.Sign() <= 0 {
		return report, fmt.Errorf("%q needs a point value above zero", target.Name)
	}

	students, err := workspace.OpenStudents(s.Root)
	if err != nil {
		return report, err
	}
	enrollments, err := StudentEnrollments(ctx, s.Client, course.ID)
	if err != nil {
		return report, err
	}
	byUser := map[int64]workspace.Student{}
	var users []int64
	for _, en := range enrollments {
		id := en.UserID
		if id == 0 {
			id = en.User.ID
		}
		if _, seen := byUser[id]; id <= 0 || en.State != "active" || seen {
			continue
		}
		st, e := students.Upsert(s.Client.Origin(), id, en.User.Name, en.User.ShortName, en.User.SISID, en.User.Login)
		if e != nil {
			return report, e
		}
		byUser[id] = st
		users = append(users, id)
	}
	if err = students.Save(); err != nil {
		return report, err
	}
	sort.Slice(users, func(i, j int) bool { return users[i] < users[j] })

	type found struct {
		sourceID int64
		text     string
		value    domain.Decimal
	}
	scores := map[int64][]found{}
	pending, excused := map[int64]bool{}, map[int64]bool{}
	scaled := false
	for _, id := range sourceIDs {
		source, e := GetAssignment(ctx, s.Client, course.ID, id)
		if e != nil {
			return report, e
		}
		points, ok := new(big.Rat).SetString(source.Points.String())
		if !ok || points.Sign() <= 0 {
			return report, fmt.Errorf("source %q needs a point value above zero", source.Name)
		}
		scaled = scaled || points.Cmp(targetPoints) != 0
		subs, e := Submissions(ctx, s.Client, course.ID, id, false)
		if e != nil {
			return report, e
		}
		info := CarryGradesSource{ID: id, Name: source.Name, Points: shortRat(points)}
		for _, sub := range subs {
			if _, enrolled := byUser[sub.UserID]; !enrolled {
				continue
			}
			if sub.Excused {
				excused[sub.UserID] = true
				continue
			}
			entered, graded := enteredRat(sub)
			if !graded {
				// A submitted attempt with no score yet is unfinished grading.
				if sub.State == "pending_review" || (sub.State == "submitted" && sub.SubmittedAt != nil) {
					pending[sub.UserID] = true
				}
				continue
			}
			if sub.State == "pending_review" {
				pending[sub.UserID] = true
				continue
			}
			info.Scored++
			value := domain.RoundRat(new(big.Rat).Mul(new(big.Rat).Quo(entered, points), targetPoints), 2)
			scores[sub.UserID] = append(scores[sub.UserID], found{id, shortRat(entered) + "/" + shortRat(points), value})
		}
		report.Sources = append(report.Sources, info)
	}

	readTarget := func() (map[int64]Submission, error) {
		subs, e := Submissions(ctx, s.Client, course.ID, target.ID, false)
		m := map[int64]Submission{}
		for _, sub := range subs {
			m[sub.UserID] = sub
		}
		return m, e
	}
	targetSubs, err := readTarget()
	if err != nil {
		return report, err
	}

	type calc struct {
		row   CarryGradesRow
		user  int64
		saved carrySavedRow
		value domain.Decimal
	}
	var calcs []*calc
	for _, user := range users {
		st := byUser[user]
		alias := st.Alias
		if !s.Config.Aliases {
			alias = st.Name
		}
		c := &calc{row: CarryGradesRow{StudentRef: st.Ref, Alias: alias}, user: user}
		calcs = append(calcs, c)
		hold := func(reason string) { c.row.Status, c.row.Reason = "held", reason }
		got := scores[user]
		differ := false
		for _, g := range got[min(1, len(got)):] {
			differ = differ || g.value.Cmp(got[0].value) != 0
		}
		switch {
		case pending[user]:
			hold("a source attempt is not fully graded yet")
			continue
		case differ:
			var parts []string
			for _, g := range got {
				parts = append(parts, fmt.Sprintf("%s on %d", g.text, g.sourceID))
			}
			hold("different scores on more than one source (" + strings.Join(parts, ", ") + "); choose one and enter it yourself")
			continue
		case len(got) == 0 && excused[user]:
			hold("excused on the source; nothing was entered")
			continue
		case len(got) == 0:
			hold("no score on any source; nothing was entered")
			continue
		}
		c.value = got[0].value
		c.row.SourceID, c.row.Source, c.row.Value = got[0].sourceID, got[0].text, shortRat(c.value.Rat())+"/"+shortRat(targetPoints)
		sub, assigned := targetSubs[user]
		if !assigned {
			hold("the target column is not assigned to this student")
			continue
		}
		if sub.Excused {
			hold("excused on the target column")
			continue
		}
		c.row.Status = "write"
		c.saved = carrySavedRow{Ref: st.Ref, Value: c.value.String()}
		if current, has := enteredRat(sub); has {
			c.row.Current = shortRat(current) + "/" + shortRat(targetPoints)
			c.saved.Current = domain.RoundRat(current, 4).String()
			switch cmp := current.Cmp(c.value.Rat()); {
			case cmp == 0:
				c.row.Status = "unchanged"
			case cmp > 0:
				hold(fmt.Sprintf("the target already has %s, which is higher than the source's %s; a grade is never lowered here", c.row.Current, c.row.Value))
			}
		}
	}

	if scaled {
		report.Notes = append(report.Notes, fmt.Sprintf("At least one source has different points than the target (%s), so those scores are scaled to keep the same percent.", shortRat(targetPoints)))
	}
	report.Notes = append(report.Notes,
		"Each value is the score entered on the source, before any late deduction. No late deduction is applied on the target.",
		"Grades are written the way SpeedGrader writes them: they stay hidden if the target's posting policy is manual, and students see them at once if it is automatic.",
		"The source columns still count toward the course grade until they are moved to a no-weight assignment group.")
	if !target.Published {
		report.Notes = append(report.Notes, "The target is unpublished. Canvas may refuse grades on an unpublished assignment; publish it when you are ready to write.")
	}

	savedPath, err := carryPreviewPath(s.Root, course.Nickname, target.ID)
	if err != nil {
		return report, err
	}
	rel, _ := filepath.Rel(s.Root, savedPath)
	report.PreviewFile = filepath.ToSlash(rel)
	finish := func() {
		report.Counts = map[string]int{}
		for _, c := range calcs {
			report.Rows = append(report.Rows, c.row)
			report.Counts[c.row.Status]++
		}
	}
	var reviewed carrySaved
	haveSaved := false
	if b, e := os.ReadFile(savedPath); e == nil && json.Unmarshal(b, &reviewed) == nil && reviewed.TargetID == target.ID && fmt.Sprint(reviewed.SourceIDs) == fmt.Sprint(sourceIDs) {
		haveSaved = true
	}
	if preview || !haveSaved {
		current := carrySaved{TargetID: target.ID, SourceIDs: sourceIDs}
		for _, c := range calcs {
			if c.row.Status == "write" {
				current.Rows = append(current.Rows, c.saved)
			}
		}
		if err = os.MkdirAll(filepath.Dir(savedPath), 0o755); err != nil {
			return report, err
		}
		b, _ := json.MarshalIndent(current, "", "  ")
		if err = os.WriteFile(savedPath+".tmp", b, 0o644); err != nil {
			return report, err
		}
		if err = os.Rename(savedPath+".tmp", savedPath); err != nil {
			return report, err
		}
		if !preview {
			report.Preview = true
			report.Notes = append(report.Notes, "No reviewed preview for this target and these sources was saved on this PC, so this ran as a preview and wrote nothing to Canvas. Review the values, then run it again.")
		}
		finish()
		return report, nil
	}

	savedBy := map[string]carrySavedRow{}
	for _, r := range reviewed.Rows {
		savedBy[r.Ref] = r
	}
	wrote := false
	for _, c := range calcs {
		if c.row.Status != "write" {
			continue
		}
		// A changed source score or a grade edited on the target since the
		// preview both show up as a different saved row.
		if want, ok := savedBy[c.saved.Ref]; !ok || want != c.saved {
			c.row.Status, c.row.Reason = "held", "the source score or the target grade changed since the reviewed preview; run a fresh preview"
			continue
		}
		path := fmt.Sprintf("/api/v1/courses/%d/assignments/%d/submissions/%d", course.ID, target.ID, c.user)
		if e := s.Client.Write(ctx, "PUT", path, map[string]any{"submission": map[string]any{"posted_grade": c.value.String()}}, nil); e != nil {
			c.row.Status, c.row.Reason = "held", "Canvas did not accept the grade: "+e.Error()
			continue
		}
		c.row.Status, wrote = "written", true
	}
	if wrote {
		after, e := readTarget()
		for _, c := range calcs {
			if c.row.Status != "written" {
				continue
			}
			entered, has := enteredRat(after[c.user])
			if e != nil || !has || entered.Cmp(c.value.Rat()) != 0 {
				c.row.Status, c.row.Reason = "held", "Canvas accepted the grade but a fresh read did not show it; check this student in the gradebook"
			}
		}
	}
	finish()
	return report, nil
}
