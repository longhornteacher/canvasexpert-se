package workflow

import "canvasexpertse/internal/domain"

// academicGradeComplete tracks accepted scoring/comment work independently of
// the late action, which may be held or pending for a later timing review.
func academicGradeComplete(g *domain.GradeOperation) bool {
	return g != nil && definite(g.Outcomes.Comment) && definite(g.Outcomes.Rubric) && definite(g.Outcomes.Grade)
}
