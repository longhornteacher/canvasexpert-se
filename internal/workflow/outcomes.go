package workflow

import (
	"canvasexpertse/internal/canvas"
	"canvasexpertse/internal/domain"
	"errors"
)

func definite(o domain.ActionOutcome) bool {
	return o.Status == "succeeded" || o.Status == "preserved" || o.Status == "not-requested"
}
func gradeComplete(g *domain.GradeOperation) bool {
	return g != nil && definite(g.Outcomes.Comment) && definite(g.Outcomes.Rubric) && definite(g.Outcomes.Grade) && definite(g.Outcomes.Late)
}
func failedOutcome(err error) domain.ActionOutcome {
	status := "failed"
	var ce *canvas.Error
	if errors.As(err, &ce) && ce.Uncertain {
		status = "uncertain"
	}
	return domain.ActionOutcome{Status: status, Message: err.Error()}
}
