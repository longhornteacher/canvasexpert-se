package domain

import "fmt"

// Audience selects a dated union of club members who work individually.
// Source IDs never become native group-assignment targets.
type Audience struct {
	Mode   string         `yaml:"mode" json:"mode"`
	Policy string         `yaml:"policy" json:"policy"`
	Source AudienceSource `yaml:"source" json:"source"`
	State  *AudienceState `yaml:"state,omitempty" json:"state,omitempty"`
}
type AudienceSource struct {
	GroupCategoryID int64   `yaml:"group_category_id" json:"group_category_id"`
	GroupIDs        []int64 `yaml:"group_ids" json:"group_ids"`
}
type AudienceState struct {
	Instance           string             `yaml:"instance" json:"instance"`
	CourseID           int64              `yaml:"course_id" json:"course_id"`
	TargetID           int64              `yaml:"target_id,omitempty" json:"target_id,omitempty"`
	Source             AudienceSource     `yaml:"source" json:"source"`
	Labels             []string           `yaml:"labels,omitempty" json:"labels,omitempty"`
	Exclusions         []string           `yaml:"exclusions,omitempty" json:"exclusions,omitempty"`
	ObservedAt         string             `yaml:"observed_at" json:"observed_at"`
	StudentRefs        []string           `yaml:"student_refs" json:"student_refs"`
	Fingerprint        string             `yaml:"fingerprint" json:"fingerprint"`
	Complete           bool               `yaml:"complete" json:"complete"`
	Reasons            []string           `yaml:"reasons,omitempty" json:"reasons,omitempty"`
	Exceptions         []string           `yaml:"exceptions,omitempty" json:"exceptions,omitempty"`
	ObservedVisibility *bool              `yaml:"observed_visibility,omitempty" json:"observed_visibility,omitempty"`
	OverrideBaseline   string             `yaml:"override_baseline,omitempty" json:"override_baseline,omitempty"`
	Owned              []AudienceOverride `yaml:"owned,omitempty" json:"owned,omitempty"`
	Application        ActionOutcome      `yaml:"application,omitempty" json:"application,omitempty"`
	Visibility         ActionOutcome      `yaml:"visibility,omitempty" json:"visibility,omitempty"`
}
type AudienceOverride struct {
	ID       int64                      `yaml:"id,omitempty" json:"id,omitempty"`
	Baseline AssignmentOverrideBaseline `yaml:"baseline" json:"baseline"`
	Outcome  ActionOutcome              `yaml:"outcome,omitempty" json:"outcome,omitempty"`
}

func ValidateAudience(a *Audience) error {
	if a == nil {
		return nil
	}
	if a.Mode != "individual" || a.Policy != "snapshot" {
		return fmt.Errorf("audience requires mode: individual and policy: snapshot")
	}
	if a.Source.GroupCategoryID <= 0 || len(a.Source.GroupIDs) == 0 {
		return fmt.Errorf("audience source requires a positive group_category_id and selected group_ids")
	}
	seen := map[int64]bool{}
	for _, id := range a.Source.GroupIDs {
		if id <= 0 || seen[id] {
			return fmt.Errorf("audience group_ids must be positive and unique")
		}
		seen[id] = true
	}
	if a.State != nil {
		if !validInstance(a.State.Instance) || a.State.CourseID <= 0 || a.State.Fingerprint == "" {
			return fmt.Errorf("audience state requires prepared Canvas/course identity and fingerprint")
		}
		if err := validateOutcome(a.State.Application); err != nil {
			return err
		}
		if err := validateOutcome(a.State.Visibility); err != nil {
			return err
		}
	}
	return nil
}
