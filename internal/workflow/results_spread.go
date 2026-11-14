package workflow

import (
	"fmt"
	"math/big"
	"sort"
	"strings"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/render"
	"canvasexpertse/internal/workspace"
)

// ResultsSpread summarizes a valid results file's scores for the check-in, so
// the agent quotes the Engine's numbers instead of working them out: how many
// students are scored and held, and the spread of earned scores in assignment
// points (New Quizzes: per written part, in item points). It returns "" when
// the file has no scores to summarize. It reads only the results file, its
// packet and the course settings.
func ResultsSpread(root, path string) (string, error) {
	var doc domain.ResultsDocument
	if _, err := ReadResultsDocument(root, path, &doc); err != nil {
		return "", err
	}
	rows, err := domain.ValidateResults(&doc)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	held := 0
	if doc.Assignment.NewQuiz {
		var packet domain.PacketDocument
		if p, e := workspace.ConstrainPath(root, doc.Packet); e == nil {
			_, _ = workspace.ReadDocument(p, &packet)
		}
		parts := map[string][]*big.Rat{}
		maxima := map[string]domain.Decimal{}
		scored, marked := 0, 0
		for _, v := range rows {
			c := v.Student.CommentOnly
			if c == nil || len(c.Components) == 0 {
				continue
			}
			if v.Hold != "" {
				held++
				continue
			}
			points, e := partPoints(&doc, packet.WrittenItems, v.Interpretation)
			if e != nil {
				continue
			}
			scored++
			if c.HonestEffort {
				marked++
			}
			for key, item := range render.QuizItems(*doc.Basis(), v.Interpretation, c.Components, points) {
				parts[key] = append(parts[key], item.Points.Rat())
				maxima[key] = points[key]
			}
		}
		if scored == 0 {
			return "", nil
		}
		fmt.Fprintf(&b, "Score spread by written part (item points): %d scored, %d held.\n", scored, held)
		for _, c := range doc.Interpretation.Components {
			values := parts[c.Key]
			if len(values) == 0 {
				continue
			}
			label := c.Label
			if label == "" {
				label = c.Key
			}
			fmt.Fprintf(&b, "  %s (out of %s): %s\n", label, trimZeros(maxima[c.Key].String()), spreadLine(values))
		}
		if marked > 0 {
			fmt.Fprintf(&b, "  marked honest_effort: %d (the Engine works out any floor from Canvas's quiz total when it posts)\n", marked)
		}
		return b.String(), nil
	}
	floor := (*domain.Decimal)(nil)
	var config domain.CourseConfig
	if _, e := workspace.ReadTeacherDocument(root, "context/courses.md", &config); e == nil {
		for _, course := range config.Courses {
			if course.ID == doc.Assignment.CourseID {
				floor = course.EffortFloor
			}
		}
	}
	var values []*big.Rat
	raised := 0
	for _, v := range rows {
		var earned domain.Decimal
		honest := false
		switch {
		case v.Student.Grade != nil:
			earned, honest = v.Student.Grade.Assessment, v.Student.Grade.HonestEffort
		case v.Student.CommentOnly != nil && len(v.Student.CommentOnly.Components) > 0 && v.Assessment != nil:
			earned = v.Assessment.Posted
		default:
			continue
		}
		if v.Hold != "" {
			held++
			continue
		}
		values = append(values, earned.Rat())
		if domain.EnteredGrade(earned, doc.Assignment.Points, floor, honest).Cmp(earned) != 0 {
			raised++
		}
	}
	if len(values) == 0 {
		return "", nil
	}
	points := doc.Assignment.Points
	fmt.Fprintf(&b, "Score spread (earned, out of %s): %d scored, %d held.\n", trimZeros(points.String()), len(values), held)
	fmt.Fprintf(&b, "  %s\n", spreadLine(values))
	if points.IsSet() && points.Sign() > 0 {
		fmt.Fprintf(&b, "  by percent: %s\n", percentBands(values, points.Rat()))
	}
	if raised > 0 {
		fmt.Fprintf(&b, "  entered at the class honest-effort minimum: %d (earned scores above stay as earned)\n", raised)
	}
	return b.String(), nil
}

// spreadLine gives the median, the middle half (from four scores up) and the
// lowest and highest score.
func spreadLine(values []*big.Rat) string {
	sorted := append([]*big.Rat(nil), values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Cmp(sorted[j]) < 0 })
	n := len(sorted)
	median := new(big.Rat).Set(sorted[n/2])
	if n%2 == 0 {
		median.Add(sorted[n/2-1], sorted[n/2]).Quo(median, big.NewRat(2, 1))
	}
	show := func(r *big.Rat) string { return trimZeros(domain.RoundRat(r, 2).String()) }
	line := "median " + show(median)
	if n >= 4 {
		line += "; middle half " + show(sorted[n/4]) + "–" + show(sorted[(3*n-1)/4])
	}
	return line + "; lowest " + show(sorted[0]) + ", highest " + show(sorted[n-1])
}

// percentBands counts scores by ten-point percent bands of the assignment's
// points, highest first, leaving out empty bands.
func percentBands(values []*big.Rat, max *big.Rat) string {
	labels := []string{"90–100%", "80s", "70s", "60s", "below 60%"}
	counts := make([]int, len(labels))
	for _, v := range values {
		pct := new(big.Rat).Quo(new(big.Rat).Mul(v, big.NewRat(100, 1)), max)
		band := len(labels) - 1
		for i, cut := range []int64{90, 80, 70, 60} {
			if pct.Cmp(big.NewRat(cut, 1)) >= 0 {
				band = i
				break
			}
		}
		counts[band]++
	}
	var out []string
	for i, label := range labels {
		if counts[i] > 0 {
			out = append(out, fmt.Sprintf("%s: %d", label, counts[i]))
		}
	}
	return strings.Join(out, ", ")
}
