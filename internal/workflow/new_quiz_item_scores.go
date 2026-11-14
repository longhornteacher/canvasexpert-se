package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"math/big"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"canvasexpertse/internal/canvas"
	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/render"
	"canvasexpertse/internal/workspace"
)

// postItemScores writes each part's item points and item comment into the
// New Quiz result before any whole-assignment comment, using the internal
// call SpeedGrader uses. It sends at
// most one write per run: a pending or uncertain write is reconciled against
// the quiz result and never repeated blindly, and a grade someone entered in
// Canvas since the pull is not overwritten.
func (s *Service) postItemScores(ctx context.Context, doc *domain.ResultsDocument, c *domain.CommentOnlyOperation, v domain.ValidatedStudent, source postSource, live Submission, save func() error) error {
	o := &c.ItemScoresOutcome
	parts, err := partPoints(doc, source.writtenItems, v.Interpretation)
	if err != nil {
		return err
	}
	items := render.QuizItems(*doc.Basis(), v.Interpretation, c.Components, parts)
	want, comments := map[string]string{}, map[string]string{}
	for _, ic := range v.Interpretation.Components {
		entry := ""
		for _, item := range source.writtenItems {
			if ic.ItemID == item.ItemID || ic.ItemID == item.EntryID {
				entry = item.EntryID
				if entry == "" {
					entry = item.ItemID
				}
			}
		}
		if _, dup := want[entry]; dup {
			holdOutcome(o, fmt.Sprintf("two parts score item %s; give each written item one part", ic.ItemID))
			return save()
		}
		want[entry] = items[ic.Key].Points.String()
		if text := workspace.ReplaceStudentName(items[ic.Key].Comment, source.student); text != "" {
			comments[entry] = commentHTML(text)
		}
	}
	summary := itemScoreSummary(want)

	ps, qs := newQuizSessionIDs(live)
	if ps == "" {
		holdOutcome(o, "Canvas did not report this attempt's quiz session; enter the item points in SpeedGrader")
		return save()
	}
	g, err := s.Client.NewQuizGrading(ctx, doc.Assignment.CourseID, doc.Assignment.AssignmentID, ps, qs)
	if err != nil {
		holdOutcome(o, "could not read the current quiz result: "+err.Error())
		return save()
	}
	var floor *quizFloorPlan
	if c.HonestEffort {
		plan, why := s.planQuizFloor(doc.Assignment.CourseID, g, want)
		if why != "" {
			holdOutcome(o, why)
			return save()
		}
		floor = plan
	}
	landed := true
	for entry, score := range want {
		current, ok := g.Item(entry)
		if !ok {
			holdOutcome(o, "item "+entry+" is not in the current quiz result")
			return save()
		}
		if current.Attempt != strconv.Itoa(c.Attempt.Attempt) {
			holdOutcome(o, fmt.Sprintf("the quiz result is for attempt %s, not the pulled attempt %d; pull-work for this student", current.Attempt, c.Attempt.Attempt))
			return save()
		}
		if !sameNumber(current.Score, score) || current.Comment != comments[entry] {
			landed = false
		}
	}
	if landed && (floor == nil || floor.raise.Sign() == 0 || !quizFloorWrites) {
		*o = domain.ActionOutcome{Status: "preserved", PostedText: summary, Message: "the quiz result already shows these item points and comments" + floor.note()}
		c.QuizScore = quizTotal(g)
		return save()
	}
	if ambiguous(*o) {
		o.Status, o.Message = "uncertain", "item score write remains uncertain; check SpeedGrader before repeating it"
		return save()
	}
	if source.work.ObservedQuizResult == "" {
		holdOutcome(o, "this pull did not record the quiz result version; pull-work for this student")
		return save()
	}
	if g.ResultID != source.work.ObservedQuizResult || !sameScore(live.Score, source.work.ObservedScore) || gradedSince(live.GradedAt, source.work.ObservedGradedAt) {
		holdOutcome(o, "the quiz was graded in Canvas since the pull; not overwritten, pull-work for this student")
		return save()
	}
	*o = domain.ActionOutcome{Status: "pending", PostedText: summary}
	if err := save(); err != nil {
		return err
	}
	if floor != nil && floor.raise.Sign() > 0 && quizFloorWrites {
		g.FudgePoints = json.RawMessage(floor.adjustment.String())
		summary += floor.note()
		o.PostedText = summary
	}
	version, err := g.PostScores(ctx, want, comments)
	if err != nil {
		*o = failedOutcome(err)
		o.PostedText = summary
		return save()
	}
	*o = domain.ActionOutcome{Status: "succeeded", PostedText: summary, Message: "quiz result version " + version}
	if floor != nil && !quizFloorWrites {
		o.Message += floor.note()
	}
	c.QuizScore = quizTotal(g)
	if id, e := strconv.ParseInt(version, 10, 64); e == nil {
		o.ID = id
	}
	return save()
}

// quizFloorWrites is off until a probe on a disposable quiz establishes how
// New Quizzes scales fudge points against the quiz total, whether the total
// is before late deductions, and how a retry behaves. While off, honest_effort
// still writes the earned item points and reports the floor numbers so the
// teacher can enter the adjustment in SpeedGrader; it never writes fudge points.
var quizFloorWrites = false

// quizFloorPlan is the whole-quiz floor worked out from the live result.
type quizFloorPlan struct {
	earned, existing, minimum, projected, adjustment, raise domain.Decimal
}

// planQuizFloor works from the result as it stands: the total Canvas reports,
// with the named items' current points swapped for the points about to be
// written, so a rerun after the raise landed adds nothing. It returns a
// reason when the floor cannot be computed honestly.
func (s *Service) planQuizFloor(courseID int64, g *canvas.NewQuizGrading, want map[string]string) (*quizFloorPlan, string) {
	floor := s.effortFloor(courseID)
	if floor == nil {
		return nil, "honest_effort needs this course's effort_floor; set it in Courses, or leave honest_effort off"
	}
	total, e1 := domain.ParseDecimal(g.Score)
	possible, e2 := domain.ParseDecimal(g.PointsPossible)
	if e1 != nil || e2 != nil || possible.Sign() <= 0 {
		return nil, "Canvas did not report the quiz total; enter the points and any minimum in SpeedGrader"
	}
	existing := domain.MustDecimal("0")
	if raw := strings.TrimSpace(string(g.FudgePoints)); raw != "" && raw != "null" {
		d, err := domain.ParseDecimal(raw)
		if err != nil {
			return nil, "the quiz result's adjustment could not be read; enter the minimum in SpeedGrader"
		}
		existing = d
	}
	delta := new(big.Rat)
	for entry, score := range want {
		cur, _ := g.Item(entry)
		if d, err := domain.ParseDecimal(cur.Score); err == nil {
			delta.Sub(delta, d.Rat())
		}
		d, _ := domain.ParseDecimal(score)
		delta.Add(delta, d.Rat())
	}
	projected := domain.RoundRat(new(big.Rat).Add(total.Rat(), delta), 2)
	minimum, adjustment := domain.QuizFloor(possible, projected, existing, *floor)
	earned := domain.RoundRat(new(big.Rat).Sub(projected.Rat(), existing.Rat()), 2)
	raise := domain.RoundRat(new(big.Rat).Sub(adjustment.Rat(), existing.Rat()), 2)
	return &quizFloorPlan{earned: earned, existing: existing, minimum: minimum, projected: projected, adjustment: adjustment, raise: raise}, ""
}

// note states the floor numbers for the teacher: earned total, the adjustment
// already in the result, the course minimum and the proposed entered total.
// Canvas applies any late deduction itself; these are before it.
func (p *quizFloorPlan) note() string {
	if p == nil {
		return ""
	}
	if p.raise.Sign() == 0 {
		return fmt.Sprintf("; honest-effort floor: total %s already meets the minimum %s, nothing added", trimZeros(p.projected.String()), trimZeros(p.minimum.String()))
	}
	entered := domain.RoundRat(new(big.Rat).Add(p.projected.Rat(), p.raise.Rat()), 2)
	verb := "enter"
	if quizFloorWrites {
		verb = "entered"
	}
	return fmt.Sprintf("; honest-effort floor: earned %s, existing adjustment %s, minimum %s, %s total %s by adding %s adjustment points (before any late deduction)", trimZeros(p.earned.String()), trimZeros(p.existing.String()), trimZeros(p.minimum.String()), verb, trimZeros(entered.String()), trimZeros(p.raise.String()))
}

// quizVersionReader opens one native New Quizzes session per assignment
// during a pull and reads each attempt's current result version through it.
type quizVersionReader struct {
	session *canvas.NewQuizSession
	failed  bool
}

// quizResultVersion records an attempt's current result version, or nothing
// when it cannot be read; item scores then hold until a fresh pull.
func (s *Service) quizResultVersion(ctx context.Context, r *quizVersionReader, courseID, assignmentID int64, sub Submission) string {
	ps, qs := newQuizSessionIDs(sub)
	if ps == "" || r.failed {
		return ""
	}
	if r.session == nil {
		session, err := s.Client.NewQuizSession(ctx, courseID, assignmentID)
		if err != nil {
			r.failed = true
			return ""
		}
		r.session = session
	}
	version, err := r.session.ResultVersion(ctx, ps, qs)
	if err != nil {
		return ""
	}
	return version
}

// quizTotal is the quiz total Canvas reported, as X/Y, or empty if unknown.
func quizTotal(g *canvas.NewQuizGrading) string {
	if g.Score == "" || g.PointsPossible == "" {
		return ""
	}
	return trimZeros(g.Score) + "/" + trimZeros(g.PointsPossible)
}

// newQuizSessionIDs reads the participant and quiz session IDs from the
// launch URL Canvas reports on a New Quiz submission.
func newQuizSessionIDs(sub Submission) (participant, quiz string) {
	raw := sub.ExternalToolURL
	if raw == "" {
		raw = sub.URL
	}
	for i := 0; i < 2 && raw != ""; i++ {
		u, err := url.Parse(raw)
		if err != nil {
			return "", ""
		}
		q := u.Query()
		if ps := q.Get("participant_session_id"); ps != "" {
			return ps, q.Get("quiz_session_id")
		}
		raw = q.Get("url")
	}
	return "", ""
}

// commentHTML turns plain item feedback into the paragraphs SpeedGrader
// stores: blank lines separate paragraphs, single line breaks stay breaks.
func commentHTML(text string) string {
	var b strings.Builder
	text = strings.ReplaceAll(strings.TrimSpace(text), "\r\n", "\n")
	for _, para := range strings.Split(text, "\n\n") {
		if para = strings.TrimSpace(para); para == "" {
			continue
		}
		b.WriteString("<p>" + strings.ReplaceAll(html.EscapeString(para), "\n", "<br>") + "</p>")
	}
	return b.String()
}

func itemScoreSummary(want map[string]string) string {
	keys := make([]string, 0, len(want))
	for k := range want {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("item " + k + ": " + want[k])
	}
	return b.String()
}

func sameNumber(a, b string) bool {
	x, e1 := domain.ParseDecimal(a)
	y, e2 := domain.ParseDecimal(b)
	return e1 == nil && e2 == nil && x.Cmp(y) == 0
}

// gradedSince reports a SpeedGrader save after the pull, including one that
// changed only an item comment and left the score alone. Older pulls did not
// record graded_at; for them only the score comparison applies.
func gradedSince(live *string, pulled string) bool {
	if pulled == "" {
		return false
	}
	return live == nil || *live != pulled
}

func sameScore(live *json.Number, pulled *domain.Decimal) bool {
	if live == nil || pulled == nil {
		return live == nil && pulled == nil
	}
	return sameNumber(live.String(), pulled.String())
}

// trimZeros shows a decimal without trailing zeros: 10.0 reads 10, 7.50
// reads 7.5.
func trimZeros(s string) string {
	if strings.Contains(s, ".") {
		s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	}
	return s
}
