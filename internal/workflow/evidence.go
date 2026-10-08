package workflow

import (
	"canvasexpertse/internal/canvas"
	"canvasexpertse/internal/domain"
	"errors"
	"fmt"
)

// Capabilities records conclusions from disposable-course probes on exactly
// one Canvas origin. Its zero value permits reads and holds dependent writes.
// It is evidence, not teacher authorization: the command/host supplies intent.
// Fixture tests opt in explicitly; production must not infer it from a PAT.
type Capabilities struct {
	Instance                    string `yaml:"instance" json:"instance"`
	EvidenceReference           string `yaml:"evidence_reference" json:"evidence_reference"`
	Authoring                   bool   `yaml:"authoring" json:"authoring"`
	IndependentRubricCreate     bool   `yaml:"independent_rubric_create" json:"independent_rubric_create"`
	EnteredScore                bool   `yaml:"entered_score" json:"entered_score"`
	IndependentRubricAssessment bool   `yaml:"independent_rubric_assessment" json:"independent_rubric_assessment"`
	CoupledRubricPreservation   bool   `yaml:"coupled_rubric_preservation" json:"coupled_rubric_preservation"`
	FractionalRubricPoints      bool   `yaml:"fractional_rubric_points" json:"fractional_rubric_points"`
	CommentAttempts             bool   `yaml:"comment_attempts" json:"comment_attempts"`
	CommentEditing              bool   `yaml:"comment_editing" json:"comment_editing"`
	LateOverride                bool   `yaml:"late_override" json:"late_override"`
	EffectiveDueDate            bool   `yaml:"effective_due_date" json:"effective_due_date"`
	CompletionDiscovery         bool   `yaml:"completion_discovery" json:"completion_discovery"`
}

func (s *Service) proven(enabled bool) bool {
	return enabled && s.Evidence.Instance == s.Client.Origin() && s.Evidence.EvidenceReference != ""
}
func (s *Service) require(enabled bool, feature string) error {
	if !s.proven(enabled) {
		return fmt.Errorf("%s held: disposable-course evidence is missing for this Canvas instance", feature)
	}
	return nil
}
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
