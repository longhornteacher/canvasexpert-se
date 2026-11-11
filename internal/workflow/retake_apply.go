package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

const retakeCreditPrefix = "Retake credit:"
const retakeLateLine = " Your late deduction still applies."

// The amount a Retake credit line awarded, read back so a later re-score
// cannot silently take it away.
var retakeCreditAmountPattern = regexp.MustCompile(`now entered as (\d+(?:\.\d+)?)/`)

// retakeCreditAmount is the amount in the most recent Retake credit line on the
// submission written by someone other than the student. A line typed by the
// student protects nothing.
func retakeCreditAmount(sub Submission) (*big.Rat, bool) {
	var best *SubmissionComment
	for i := range sub.Comments {
		c := &sub.Comments[i]
		if c.AuthorID == 0 || c.AuthorID == sub.UserID || !strings.HasPrefix(strings.TrimSpace(c.Comment), retakeCreditPrefix) {
			continue
		}
		if best == nil || c.CreatedAt > best.CreatedAt || (c.CreatedAt == best.CreatedAt && c.ID > best.ID) {
			best = c
		}
	}
	if best == nil {
		return nil, false
	}
	m := retakeCreditAmountPattern.FindStringSubmatch(best.Comment)
	if m == nil {
		return nil, false
	}
	return new(big.Rat).SetString(m[1])
}

type RetakeApplyRow struct {
	StudentRef string `json:"student_ref"`
	Alias      string `json:"alias"`
	Status     string `json:"status"` // raise, keep (preview); written, unchanged, held
	Reason     string `json:"reason,omitempty"`
	Retake     string `json:"retake,omitempty"`
	Original   string `json:"original,omitempty"`
	Target     string `json:"target,omitempty"`
}

type RetakeApplyReport struct {
	Course       string           `json:"course"`
	Preview      bool             `json:"preview"`
	OriginalID   int64            `json:"original_id"`
	OriginalName string           `json:"original_name,omitempty"`
	RetakeID     int64            `json:"retake_id,omitempty"`
	PreviewFile  string           `json:"preview_file,omitempty"`
	Rows         []RetakeApplyRow `json:"students"`
	Notes        []string         `json:"notes,omitempty"`
}

// retakeSaved is what the teacher reviewed, so apply writes only those values.
type retakeSavedRow struct {
	Ref             string `json:"ref"`
	RetakeAttempt   int    `json:"retake_attempt"`
	RetakeScore     string `json:"retake_score"`
	OriginalAttempt int    `json:"original_attempt"`
	OriginalEntered string `json:"original_entered"`
	Target          string `json:"target"`
}

type retakeSaved struct {
	OriginalID int64            `json:"original_id"`
	RetakeID   int64            `json:"retake_id"`
	Rows       []retakeSavedRow `json:"rows"`
}

func retakePreviewPath(root, nickname string, originalID int64) (string, error) {
	return workspace.ConstrainPath(root, filepath.Join("courses", nickname, "drafts", fmt.Sprintf("retake-apply-%d.json", originalID)))
}

type retakeCalc struct {
	row     RetakeApplyRow
	user    int64
	saved   retakeSavedRow
	result  domain.RetakeResult
	sub     Submission
	earnedT string // "88/100" in the retake's own units
	genuine bool
	floored bool
}

// RetakeApply raises each student's original grade by the retake policy, never
// lowering it. Without a saved preview on this PC it runs the preview and stops.
func (s *Service) RetakeApply(ctx context.Context, nickname string, originalID int64, genuineStudents []string, preview bool) (RetakeApplyReport, error) {
	report := RetakeApplyReport{Preview: preview, OriginalID: originalID}
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
		return report, fmt.Errorf("this course is team-taught (owner set); tutorial retakes are not supported there yet, so nothing was applied")
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
	retake, err := GetAssignment(ctx, s.Client, course.ID, paired.ID)
	if err != nil {
		return report, err
	}
	report.RetakeID = retake.ID
	points, ok := new(big.Rat).SetString(original.Points.String())
	retakePoints, ok2 := new(big.Rat).SetString(retake.Points.String())
	if !ok || !ok2 || points.Sign() <= 0 || retakePoints.Sign() <= 0 {
		return report, fmt.Errorf("the original and its retake both need a point value")
	}
	originalPoints, _ := domain.ParseDecimal(original.Points.String())
	retakeMax, _ := domain.ParseDecimal(retake.Points.String())

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
	isGenuine := map[string]bool{}
	for _, g := range genuineStudents {
		for _, st := range byUser {
			if strings.EqualFold(strings.TrimSpace(g), st.Ref) || strings.EqualFold(strings.TrimSpace(g), st.Alias) {
				isGenuine[st.Ref] = true
			}
		}
	}
	retakeSubs, err := Submissions(ctx, s.Client, course.ID, retake.ID, false)
	if err != nil {
		return report, err
	}
	originalSubs, err := Submissions(ctx, s.Client, course.ID, original.ID, true)
	if err != nil {
		return report, err
	}
	origByUser := map[int64]Submission{}
	for _, sub := range originalSubs {
		origByUser[sub.UserID] = sub
	}
	sort.Slice(retakeSubs, func(i, j int) bool { return retakeSubs[i].UserID < retakeSubs[j].UserID })
	floor := s.effortFloor(course.ID)

	var calcs []*retakeCalc
	for _, rs := range retakeSubs {
		st, ok := byUser[rs.UserID]
		if !ok || rs.Excused || rs.State == "unsubmitted" || rs.SubmittedAt == nil {
			continue
		}
		alias := st.Alias
		if !s.Config.Aliases {
			alias = st.Name
		}
		row := RetakeApplyRow{StudentRef: st.Ref, Alias: alias}
		hold := func(reason string) {
			row.Status, row.Reason = "held", reason
			calcs = append(calcs, &retakeCalc{row: row, user: st.UserID})
		}
		retakeEntered, graded := enteredRat(rs)
		if !graded || rs.State != "graded" {
			hold("the retake is not fully graded (written items may still need points)")
			continue
		}
		orig, has := origByUser[st.UserID]
		entered, originalGraded := enteredRat(orig)
		switch {
		case !has || orig.Excused:
			hold("the original is excused or missing")
			continue
		case !originalGraded || orig.State != "graded":
			hold("the original has no graded attempt")
			continue
		case orig.PostedAt == nil || *orig.PostedAt == "":
			hold("the original's grade is not posted")
			continue
		}
		earnedD := domain.RoundRat(retakeEntered, 4)
		enteredD := domain.RoundRat(entered, 4)
		genuine := isGenuine[st.Ref] && floor != nil
		result := domain.RetakeTarget(originalPoints, enteredD, earnedD, retakeMax, policy.CapPercent, floor, genuine)
		row.Retake = shortRat(retakeEntered) + "/" + shortRat(retakePoints)
		row.Original = shortRat(entered) + "/" + shortRat(points)
		row.Target = shortRat(result.Target.Rat()) + "/" + shortRat(points)
		row.Status = "keep"
		if result.Raise {
			row.Status = "raise"
		}
		floored := false
		if genuine {
			pct := new(big.Rat).Mul(new(big.Rat).Quo(retakeEntered, retakePoints), big.NewRat(100, 1))
			floored = pct.Cmp(floor.Rat()) < 0
		}
		calcs = append(calcs, &retakeCalc{
			row: row, user: st.UserID, result: result, sub: orig, earnedT: row.Retake, genuine: genuine, floored: floored,
			saved: retakeSavedRow{Ref: st.Ref, RetakeAttempt: rs.Attempt, RetakeScore: earnedD.String(), OriginalAttempt: orig.Attempt, OriginalEntered: enteredD.String(), Target: result.Target.String()},
		})
	}

	savedPath, err := retakePreviewPath(s.Root, course.Nickname, original.ID)
	if err != nil {
		return report, err
	}
	var current retakeSaved
	current.OriginalID, current.RetakeID = original.ID, retake.ID
	for _, c := range calcs {
		if c.row.Status != "held" {
			current.Rows = append(current.Rows, c.saved)
		}
	}
	savePreview := func() error {
		if err := os.MkdirAll(filepath.Dir(savedPath), 0o755); err != nil {
			return err
		}
		b, _ := json.MarshalIndent(current, "", "  ")
		tmp := savedPath + ".tmp"
		if err := os.WriteFile(tmp, b, 0o644); err != nil {
			return err
		}
		return os.Rename(tmp, savedPath)
	}
	rel, _ := filepath.Rel(s.Root, savedPath)
	report.PreviewFile = filepath.ToSlash(rel)
	collect := func() {
		for _, c := range calcs {
			report.Rows = append(report.Rows, c.row)
		}
	}
	var reviewed retakeSaved
	haveSaved := false
	if b, e := os.ReadFile(savedPath); e == nil && json.Unmarshal(b, &reviewed) == nil && reviewed.OriginalID == original.ID && reviewed.RetakeID == retake.ID {
		haveSaved = true
	}
	if preview || !haveSaved {
		if err = savePreview(); err != nil {
			return report, err
		}
		collect()
		if !preview {
			report.Preview = true
			report.Notes = append(report.Notes, "No reviewed preview was saved on this PC, so this ran as a preview and wrote nothing to Canvas. Review the values, then run apply again.")
		}
		return report, nil
	}
	savedBy := map[string]retakeSavedRow{}
	for _, r := range reviewed.Rows {
		savedBy[r.Ref] = r
	}

	for _, c := range calcs {
		row := &c.row
		if row.Status == "held" {
			continue
		}
		want, ok := savedBy[c.saved.Ref]
		if !ok || want != c.saved {
			row.Status, row.Reason = "held", "changed since the reviewed preview; run a fresh preview"
			continue
		}
		attempt := c.sub.Attempt
		if attempt < 1 {
			attempt = 1
		}
		late := c.sub.PointsDeducted != nil && c.sub.PointsDeducted.String() != "0" && c.sub.PointsDeducted.String() != ""
		path := fmt.Sprintf("/api/v1/courses/%d/assignments/%d/submissions/%d", course.ID, original.ID, c.user)
		entered, _ := enteredRat(c.sub)
		keptText := fmt.Sprintf("Tutorial retake: you earned %s. Your grade stays %s/%s.", c.earnedT, shortRat(entered), shortRat(points))
		if late {
			keptText += retakeLateLine
		}
		if !c.result.Raise {
			// A student who was already given retake credit gets no keep comment.
			if _, credited := retakeCreditAmount(c.sub); credited || hasComment(c.sub, keptText) {
				row.Status = "unchanged"
				continue
			}
			if e := s.Client.Write(ctx, "PUT", path, map[string]any{"comment": map[string]any{"text_comment": keptText, "attempt": attempt}}, nil); e != nil {
				row.Status, row.Reason = "held", "the comment was not accepted: "+e.Error()
				continue
			}
			row.Status = "written"
			continue
		}
		target := shortRat(c.result.Target.Rat())
		if original.NewQuiz {
			row.Status = "held"
			row.Reason = fmt.Sprintf("a New Quiz original: add %s points to this student in SpeedGrader so the quiz total is %s/%s (gradebook points; convert if the quiz's item points differ from its gradebook points). Nothing was written.", shortRat(new(big.Rat).Sub(c.result.Target.Rat(), entered)), target, shortRat(points))
			continue
		}
		submission := map[string]any{"posted_grade": c.result.Target.String()}
		if amt, ok := retakeCreditAmount(c.sub); ok {
			if entered.Cmp(amt) >= 0 {
				row.Status = "unchanged"
				continue
			}
			// The comment landed and the grade did not: write only the grade.
			if e := s.Client.Write(ctx, "PUT", path, map[string]any{"submission": submission}, nil); e != nil {
				row.Status, row.Reason = "held", "the grade was not accepted: "+e.Error()
				continue
			}
			row.Status = "written"
			continue
		}
		text := fmt.Sprintf("%s your tutorial retake earned %s", retakeCreditPrefix, c.earnedT)
		switch {
		case c.result.Capped:
			text += fmt.Sprintf(". Retake credit is capped at %d%%, so this grade is now entered as %s/%s.", policy.CapPercent, target, shortRat(points))
		case c.floored:
			text += fmt.Sprintf(", and the honest-effort minimum of %s%% applies, so this grade is now entered as %s/%s.", floor.String(), target, shortRat(points))
		default:
			text += fmt.Sprintf(", so this grade is now entered as %s/%s.", target, shortRat(points))
		}
		if late {
			text += retakeLateLine
		}
		// Comment and grade go in one submission update, so a retry needs no record.
		if e := s.Client.Write(ctx, "PUT", path, map[string]any{"comment": map[string]any{"text_comment": text, "attempt": attempt}, "submission": submission}, nil); e != nil {
			row.Status, row.Reason = "held", "Canvas did not accept the grade and comment: "+e.Error()
			continue
		}
		row.Status = "written"
	}
	collect()
	return report, nil
}
