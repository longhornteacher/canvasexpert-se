// Package render converts validated CanvasExpert-SE data to deterministic display text and HTML.
package render

import (
	"fmt"
	"html"
	"regexp"
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
	// Header is the teacher's choice of what leads the comment: "" or "default"
	// (score and criterion lines), "score" (only the score line) or "none" (the
	// feedback exactly as written, so the teacher controls any score text).
	Header string
}

// RenderFeedback produces a deterministic Canvas comment. The order is the
// computed assessment, component criteria, and
// the teacher's conversational feedback.
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

	switch in.Header {
	case "none":
		return in.Feedback, nil
	case "score":
		return fmt.Sprintf("Assessment: %s / %s\n\n%s", in.Assessment.String(), in.AssignmentPoints.String(), in.Feedback), nil
	}
	var out strings.Builder
	if !in.OmitAssessment {
		fmt.Fprintf(&out, "Assessment: %s / %s", in.Assessment.String(), in.AssignmentPoints.String())
		out.WriteString("\n")
	}
	out.WriteString("Criterion scores:")
	components := make(map[string]domain.ComponentScore, len(in.Components))
	for _, c := range in.Components {
		components[c.Key] = c
	}
	for _, component := range in.Interpretation.Components {
		score := components[component.Key]
		byID := make(map[string]domain.CriterionScore, len(score.Criteria))
		for _, criterionScore := range score.Criteria {
			byID[criterionScore.CriterionID] = criterionScore
		}
		fmt.Fprintf(&out, "\n- %s: ", component.Label)
		printed := 0
		for _, rubricCriterion := range in.Rubric.Criteria {
			if in.Interpretation.Mapping == "direct" && !contains(component.CriterionIDs, rubricCriterion.ID) {
				continue
			}
			if printed > 0 {
				out.WriteString("; ")
			}
			cs := byID[rubricCriterion.ID]
			fmt.Fprintf(&out, "%s %s/%s", rubricCriterion.Description, cs.Points.String(), rubricCriterion.Points.String())
			printed++
		}
	}
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

var blockTag = regexp.MustCompile(`(?i)^/?(p|div|section|article|li|ul|ol|h[1-6]|blockquote|pre|br|tr|td|th)(\s|/|$)`)

// HTMLText extracts readable text from Canvas HTML without interpreting it.
// Block elements become line breaks and entities are decoded.
func HTMLText(source string) string {
	var out strings.Builder
	inTag := false
	var tag strings.Builder
	for _, r := range source {
		if !inTag && r == '<' {
			inTag = true
			tag.Reset()
			continue
		}
		if inTag {
			if r == '>' {
				if blockTag.MatchString(strings.TrimSpace(tag.String())) {
					out.WriteRune('\n')
				}
				inTag = false
			} else {
				tag.WriteRune(r)
			}
			continue
		}
		out.WriteRune(r)
	}
	lines := strings.Split(html.UnescapeString(out.String()), "\n")
	clean := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.Join(strings.Fields(line), " ")
		if line != "" {
			clean = append(clean, line)
		}
	}
	return strings.Join(clean, "\n")
}
