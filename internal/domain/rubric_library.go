package domain

import (
	"encoding/hex"
	"fmt"
	"strings"
)

// RubricSelection pins a reference and its resolved definition during authoring.
// Definition creation and assignment association have independent outcomes.
type RubricSelection struct {
	Library       string        `yaml:"library,omitempty"`
	RequestedID   int64         `yaml:"requested_id,omitempty"`
	LibrarySHA256 string        `yaml:"library_sha256,omitempty"`
	RubricID      int64         `yaml:"rubric_id,omitempty"`
	Definition    Rubric        `yaml:"definition"`
	Create        ActionOutcome `yaml:"create,omitempty"`
	NewVersion    bool          `yaml:"new_version,omitempty"`
}

func IsRubricReference(r *Rubric) bool {
	return r != nil && (r.Library != "" || r.ID > 0 && len(r.Criteria) == 0)
}

func ValidateAuthoredRubric(r Rubric) error {
	if !IsRubricReference(&r) {
		return ValidateRubric(r)
	}
	if r.Library != "" && (!validNickname(r.Library) || r.ID != 0) || r.ID < 0 || r.AssociationID != 0 || r.Title != "" || len(r.Criteria) != 0 || r.UseForGrading || r.UseForGradingKnown || r.FreeFormCriterionComments {
		return fmt.Errorf("rubric requires exactly one full definition, {id: positive ID}, or {library: safe filename stem}")
	}
	return nil
}

func ValidateRubricSelection(s *RubricSelection, r *Rubric) error {
	if s == nil {
		return nil
	}
	if !IsRubricReference(r) || s.Library != r.Library || s.RequestedID != r.ID || s.RubricID < 0 || (s.RequestedID > 0 && s.RubricID != s.RequestedID) {
		return fmt.Errorf("saved rubric selection differs from the requested reference; use a new draft")
	}
	if s.Library != "" {
		b, e := hex.DecodeString(s.LibrarySHA256)
		if e != nil || len(b) != 32 {
			return fmt.Errorf("saved library selection requires its SHA-256 digest")
		}
	}
	if e := ValidateRubric(s.Definition); e != nil {
		return e
	}
	if s.Definition.ID != 0 || s.Definition.AssociationID != 0 || s.Definition.UseForGrading {
		return fmt.Errorf("saved rubric definition must be independent of Canvas association identity")
	}
	if e := validateOutcome(s.Create); e != nil {
		return e
	}
	if (s.Create.Status == "succeeded" || s.Create.Status == "preserved") && s.RubricID <= 0 {
		return fmt.Errorf("known rubric selection requires its rubric ID")
	}
	return nil
}

// PortableRubric carries teaching content without Canvas resource identity or
// grading-coupling flags. Scoring snapshots still require a complete Rubric.
type PortableRubric struct {
	Title                     string      `yaml:"title"`
	FreeFormCriterionComments bool        `yaml:"free_form_criterion_comments,omitempty"`
	Criteria                  []Criterion `yaml:"criteria"`
}

type RubricLibraryDocument struct {
	Schema int            `yaml:"schema"`
	Kind   string         `yaml:"kind"`
	Rubric PortableRubric `yaml:"rubric"`
}

func (r PortableRubric) Definition() Rubric {
	return Rubric{Title: r.Title, FreeFormCriterionComments: r.FreeFormCriterionComments, Criteria: r.Criteria}
}

func (d *RubricLibraryDocument) Validate() error {
	if d.Schema != 1 || d.Kind != "rubric-library" || strings.TrimSpace(d.Rubric.Title) == "" {
		return fmt.Errorf("expected schema: 1, kind: rubric-library and a titled rubric definition")
	}
	return ValidateRubric(d.Rubric.Definition())
}
