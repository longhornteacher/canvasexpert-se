package workflow

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"time"

	"canvasexpertse/internal/canvas"
	"canvasexpertse/internal/domain"
)

// stadScored is one scored item for one student: a percent in course order.
type stadScored struct {
	AssignmentID int64
	Title        string
	Due          string   // school date, course timezone
	Percent      *big.Rat // 0-100
	Missing      bool     // counted as 0 because the rules say missing counts as zero
}

// STADImprovementRow explains one improvement award so a teacher can see why a
// student earned what they did.
type STADImprovementRow struct {
	AssignmentID int64  `json:"assignment_id"`
	Title        string `json:"title"`
	Due          string `json:"due"`
	Percent      string `json:"percent"`
	Baseline     string `json:"baseline,omitempty"` // empty when the student has no earlier scored item
	Difference   string `json:"difference,omitempty"`
	Band         string `json:"band"`
	Points       string `json:"points"`
	Missing      bool   `json:"missing_counted_as_zero,omitempty"`
	exactPoints  *big.Rat
}

// scoreImprovement applies the improvement rules to one student's scored
// items. The baseline for an item is the mean of the N most recent scored
// items due strictly before it (fewer than N: the mean of what exists), so it
// never reaches into the future. With no earlier item a student gets
// no_baseline_points, or the second band's points (STAD starts students at
// the middle). A perfect score always earns at least the perfect points.
func scoreImprovement(items []stadScored, im STADImprovement) ([]STADImprovementRow, error) {
	n, err := ParseBaseline(im.Baseline)
	if err != nil {
		return nil, err
	}
	sorted := append([]stadScored(nil), items...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Due != sorted[j].Due {
			return sorted[i].Due < sorted[j].Due
		}
		return sorted[i].AssignmentID < sorted[j].AssignmentID
	})
	var rows []STADImprovementRow
	for i, it := range sorted {
		var earlier []stadScored
		for _, p := range sorted[:i] {
			if p.Due < it.Due {
				earlier = append(earlier, p)
			}
		}
		if len(earlier) > n {
			earlier = earlier[len(earlier)-n:]
		}
		row := STADImprovementRow{AssignmentID: it.AssignmentID, Title: it.Title, Due: it.Due, Percent: domain.RoundRat(it.Percent, 2).String(), Missing: it.Missing}
		var points *big.Rat
		if len(earlier) == 0 {
			row.Band = "no_baseline"
			switch {
			case im.NoBaselinePoints != nil:
				points = im.NoBaselinePoints.Rat()
			case len(im.Bands) > 1:
				points = im.Bands[1].Points.Rat()
			default:
				points = im.Bands[0].Points.Rat()
			}
		} else {
			sum := new(big.Rat)
			for _, p := range earlier {
				sum.Add(sum, p.Percent)
			}
			base := new(big.Rat).Quo(sum, big.NewRat(int64(len(earlier)), 1))
			diff := new(big.Rat).Sub(it.Percent, base)
			row.Baseline, row.Difference = domain.RoundRat(base, 2).String(), domain.RoundRat(diff, 2).String()
			for _, b := range im.Bands {
				if b.Otherwise || diff.Cmp(b.AtLeast.Rat()) >= 0 {
					points = b.Points.Rat()
					if b.Otherwise {
						row.Band = "otherwise"
					} else {
						row.Band = "at_least " + b.AtLeast.String()
					}
					break
				}
			}
		}
		if im.Perfect != nil && it.Percent.Cmp(im.Perfect.AtLeastPercent.Rat()) >= 0 && im.Perfect.Points.Rat().Cmp(points) > 0 {
			points, row.Band = im.Perfect.Points.Rat(), "perfect"
		}
		row.exactPoints = points
		row.Points = domain.RoundRat(points, 2).String()
		rows = append(rows, row)
	}
	return rows, nil
}

// stadScoredItems reads every active student's scored items for the course
// under the rules' evidence kind, in effective-due-date terms. Scored means
// published, visible, not excused, with a released score (or, if the rules say
// missing counts as zero, past due and not turned in). Result is keyed by
// student_ref.
func (s *Service) stadScoredItems(ctx context.Context, courseID int64, zone string, im STADImprovement, now time.Time) (map[string][]stadScored, error) {
	loc, err := time.LoadLocation(orDefault(zone, "America/Chicago"))
	if err != nil {
		return nil, fmt.Errorf("unknown course timezone %q", zone)
	}
	active, err := s.activeStudentsByUser(ctx, courseID)
	if err != nil {
		return nil, err
	}
	assignments, err := parentAssignments(ctx, s.Client, courseID)
	if err != nil {
		return nil, err
	}
	byAssignment := map[int64]Assignment{}
	for _, a := range assignments {
		byAssignment[a.ID] = a
	}
	var subs []Submission
	if im.Evidence == "rubric_row" {
		subs, err = courseSubmissionsWithRubrics(ctx, s.Client, courseID)
	} else {
		subs, err = courseSubmissions(ctx, s.Client, courseID)
	}
	if err != nil {
		return nil, err
	}
	perStudent := map[int64][]Submission{}
	for _, sub := range subs {
		perStudent[sub.UserID] = append(perStudent[sub.UserID], sub)
	}
	resolver := &parentSubmissionResolver{client: s.Client, courseID: courseID, batches: map[int64]map[int64]Submission{}}
	out := map[string][]stadScored{}
	for userID, st := range active {
		studentSubs, _ := completeStudentSubmissionsWithResolver(ctx, resolver, userID, assignments, perStudent[userID], time.Time{}, now, now)
		seen := map[int64]bool{}
		for _, sub := range studentSubs {
			a, ok := byAssignment[sub.AssignmentID]
			if !ok || !a.Published || a.GradingType == "not_graded" || sub.Excused || (sub.AssignmentVisible != nil && !*sub.AssignmentVisible) || seen[a.ID] {
				continue
			}
			if im.Evidence == "quizzes" && !(a.NewQuiz || a.QuizID != nil) {
				continue
			}
			due := effectiveDueForStudent(sub, a, userID)
			if due == nil || due.After(now) {
				continue
			}
			seen[a.ID] = true
			item := stadScored{AssignmentID: a.ID, Title: a.Name, Due: due.In(loc).Format("2006-01-02")}
			if im.Evidence == "rubric_row" {
				pct, found := rubricRowPercent(a, sub, im.RubricRow)
				if !found {
					continue
				}
				item.Percent = pct
				out[st.Ref] = append(out[st.Ref], item)
				continue
			}
			submitted := sub.SubmittedAt != nil && sub.Attempt >= 1
			row := workDueRow(ParentReportItem{AssignmentID: a.ID, Title: a.Name}, sub, a, submitted)
			switch {
			case row.Status == "graded" && row.Current != nil:
				pct, ok := percentRat(*row.Current, a.Points.String())
				if !ok {
					continue
				}
				item.Percent = pct
			case row.Submission == "not_turned_in" && row.Status == "no_grade" && im.MissingCountsAs == "zero":
				if _, ok := percentRat("0", a.Points.String()); !ok {
					continue // no points possible, nothing to score
				}
				item.Percent, item.Missing = new(big.Rat), true
			default:
				continue
			}
			out[st.Ref] = append(out[st.Ref], item)
		}
	}
	return out, nil
}

func orDefault(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

// rubricRowPercent finds the criterion whose description matches exactly and
// returns the points the student earned on it as a percent of the criterion's
// points. found is false when the assignment has no such criterion or the row
// was not assessed.
func rubricRowPercent(a Assignment, sub Submission, description string) (*big.Rat, bool) {
	want := strings.TrimSpace(description)
	for _, c := range a.Rubric {
		if strings.TrimSpace(c.Description) != want {
			continue
		}
		score, ok := sub.RubricAssessment[c.ID]
		if !ok || score.Points == nil {
			return nil, false
		}
		pct, ok := percentRat(score.Points.String(), c.Points.String())
		return pct, ok
	}
	return nil, false
}

func courseSubmissionsWithRubrics(ctx context.Context, c *canvas.Client, courseID int64) ([]Submission, error) {
	var result []Submission
	err := c.List(ctx, fmt.Sprintf("/api/v1/courses/%d/students/submissions?student_ids[]=all&include[]=submission_history&include[]=submission_comments&include[]=visibility&include[]=rubric_assessment&per_page=100", courseID), &result)
	return result, err
}
