// Package render converts validated CanvasExpert-SE data to deterministic display text and HTML.
package render

import (
	"fmt"
	"math/big"
	"strings"

	"canvasexpertse/internal/domain"
)

// FeedbackInput contains the validated grading context and student-facing
// feedback. Teacher notes and interpretation rationale are never emitted.
type FeedbackInput struct {
	Rubric           domain.Rubric
	Interpretation   domain.Interpretation
	Components       []domain.ComponentScore
	Assessment       domain.Decimal
	AssignmentPoints domain.Decimal
	Feedback         string
	OmitAssessment   bool
	// AutoScored leads a New Quiz comment with the auto-scored subtotal from
	// the pulled report; it is never a whole-quiz total.
	AutoScored *domain.AutoScored
	// PartPoints gives a New Quiz part the Canvas points of its written item,
	// keyed by component key, so its score reads on the quiz's scale.
	PartPoints map[string]domain.Decimal
	// Late marks the overall line "(late)". The score stays the earned
	// assessment; Canvas applies any late deduction in the gradebook.
	Late bool
	// Header is the teacher's choice of what leads the comment: "" or "full"
	// (the default: all scores, then each part's rubric lines), "score" (just
	// the overall line) or "none" (the feedback exactly as written, so the
	// teacher controls any score text).
	Header string
}

// RenderFeedback produces a deterministic Canvas comment. The order is the
// scores, each part's rubric lines, and the teacher's conversational feedback.
func RenderFeedback(in FeedbackInput) (string, error) {
	if err := domain.ValidateInterpretation(in.Rubric, in.Interpretation); err != nil {
		return "", fmt.Errorf("invalid interpretation: %w", err)
	}
	if strings.TrimSpace(in.Feedback) == "" {
		return "", fmt.Errorf("conversational feedback is required")
	}
	if !in.AssignmentPoints.IsSet() || in.AssignmentPoints.Sign() <= 0 {
		return "", fmt.Errorf("positive assignment points are required")
	}
	computed, err := domain.ComputeAssessment(in.Rubric, in.Interpretation, in.Components, in.AssignmentPoints)
	if err != nil {
		return "", fmt.Errorf("invalid component scores: %w", err)
	}
	if !in.Assessment.IsSet() || computed.Posted.Cmp(in.Assessment) != 0 {
		return "", fmt.Errorf("assessment %s does not match derived value %s", in.Assessment.String(), computed.Posted.String())
	}

	overall := fmt.Sprintf("Overall: %s/%s", shortNumber(in.Assessment), shortNumber(in.AssignmentPoints))
	if in.Late {
		overall += " (late)"
	}
	switch in.Header {
	case "none":
		return in.Feedback, nil
	case "score":
		// A New Quiz comment has no honest whole-quiz total from the written
		// portion, so it always shows the full score layout.
		if !in.OmitAssessment {
			return overall + "\n\n" + in.Feedback, nil
		}
	}
	// All scores lead: the overall (or, on a New Quiz, the Canvas-scored
	// questions) and each part, then each part's rubric lines, then feedback.
	var top []string
	if !in.OmitAssessment {
		top = append(top, overall)
	} else if in.AutoScored != nil {
		top = append(top, fmt.Sprintf("Questions Canvas scored: %s/%s", shortNumber(in.AutoScored.Earned), shortNumber(in.AutoScored.Possible)))
	}
	components := make(map[string]domain.ComponentScore, len(in.Components))
	for _, c := range in.Components {
		components[c.Key] = c
	}
	rubricMax, totalWeight := new(big.Rat), new(big.Rat)
	for _, c := range in.Rubric.Criteria {
		rubricMax.Add(rubricMax, c.Points.Rat())
	}
	for _, c := range in.Interpretation.Components {
		totalWeight.Add(totalWeight, c.Weight.Rat())
	}
	// One part on an ordinary assignment is the overall line already.
	showParts := in.OmitAssessment || len(in.Interpretation.Components) > 1
	var blocks []string
	for _, component := range in.Interpretation.Components {
		score := components[component.Key]
		byID := make(map[string]domain.CriterionScore, len(score.Criteria))
		for _, criterionScore := range score.Criteria {
			byID[criterionScore.CriterionID] = criterionScore
		}
		lines := []string{component.Label}
		earned, possible := new(big.Rat), new(big.Rat)
		for _, rubricCriterion := range in.Rubric.Criteria {
			if in.Interpretation.Mapping == "direct" && !contains(component.CriterionIDs, rubricCriterion.ID) {
				continue
			}
			cs := byID[rubricCriterion.ID]
			earned.Add(earned, cs.Points.Rat())
			possible.Add(possible, rubricCriterion.Points.Rat())
			lines = append(lines, fmt.Sprintf("- %s %s/%s", rubricCriterion.Description, shortNumber(cs.Points), shortNumber(rubricCriterion.Points)))
		}
		blocks = append(blocks, strings.Join(lines, "\n"))
		if !showParts {
			continue
		}
		// A part's maximum is its share of the assignment points; on a New
		// Quiz it is the written item's Canvas points, or the rubric points
		// when the part names no item.
		max := possible
		switch {
		case !in.OmitAssessment && in.Interpretation.Mapping == "repeated" && totalWeight.Sign() > 0:
			max = new(big.Rat).Mul(in.AssignmentPoints.Rat(), new(big.Rat).Quo(component.Weight.Rat(), totalWeight))
		case !in.OmitAssessment && rubricMax.Sign() > 0:
			max = new(big.Rat).Mul(in.AssignmentPoints.Rat(), new(big.Rat).Quo(possible, rubricMax))
		case in.OmitAssessment:
			if points, ok := in.PartPoints[component.Key]; ok {
				max = points.Rat()
			}
		}
		part := new(big.Rat)
		if possible.Sign() > 0 {
			part.Mul(max, new(big.Rat).Quo(earned, possible))
		}
		top = append(top, fmt.Sprintf("%s: %s/%s", component.Label, shortNumber(domain.RoundRat(part, 2)), shortNumber(domain.RoundRat(max, 2))))
	}
	var out strings.Builder
	if len(top) > 0 {
		out.WriteString(strings.Join(top, "\n"))
		out.WriteString("\n\n")
	}
	out.WriteString(strings.Join(blocks, "\n\n"))
	out.WriteString("\n\n")
	// comment[text_comment] is plain text. Preserve the teacher's supplied
	// feedback exactly so quoted prose and code are never reinterpreted.
	out.WriteString(in.Feedback)
	return out.String(), nil
}

func contains(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

// shortNumber prints a points value the way a teacher writes it: Canvas returns
// "20.0" for whole-number maxima, which reads as noise in student feedback.
func shortNumber(d domain.Decimal) string {
	t := d.String()
	if strings.Contains(t, ".") {
		t = strings.TrimRight(strings.TrimRight(t, "0"), ".")
	}
	if t == "" || t == "-" {
		return "0"
	}
	return t
}
