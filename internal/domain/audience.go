package domain

import (
	"fmt"
	"reflect"
	"sort"
)

// Audience selects a dated union of club members who work individually.
// Source IDs never become native group-assignment targets.
type Audience struct {
	Mode                  string         `yaml:"mode" json:"mode"`
	Policy                string         `yaml:"policy" json:"policy"`
	Source                AudienceSource `yaml:"source" json:"source"`
	AuthorizedRemovalRefs []string       `yaml:"authorized_submitted_work_change_refs,omitempty" json:"authorized_submitted_work_change_refs,omitempty"`
	State                 *AudienceState `yaml:"state,omitempty" json:"state,omitempty"`
}
type AudienceSource struct {
	GroupCategoryID int64   `yaml:"group_category_id" json:"group_category_id"`
	GroupIDs        []int64 `yaml:"group_ids" json:"group_ids"`
}
type AudienceState struct {
	PreparedPath       string             `yaml:"prepared_path,omitempty" json:"path,omitempty"`
	LatestRefreshPath  string             `yaml:"latest_refresh_path,omitempty" json:"latest_refresh_path,omitempty"`
	Refresh            *AudienceRefresh   `yaml:"refresh,omitempty" json:"refresh,omitempty"`
	AppliedRefs        []string           `yaml:"applied_refs,omitempty" json:"applied_refs,omitempty"`
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
		if err := validateAudienceRefresh(a); err != nil {
			return err
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

// AudienceRefresh is an explicit, reviewable assignment-audience action.
// Owned baselines remain the authority for rows already accepted by Canvas.
type AudienceRefresh struct {
	Add       []string         `yaml:"add,omitempty" json:"add,omitempty"`
	Remove    []string         `yaml:"remove,omitempty" json:"remove,omitempty"`
	Retain    []string         `yaml:"retain,omitempty" json:"retain,omitempty"`
	Protected []AudienceHold   `yaml:"protected,omitempty" json:"protected,omitempty"`
	Unknown   []AudienceHold   `yaml:"unknown,omitempty" json:"unknown,omitempty"`
	Changes   []AudienceChange `yaml:"changes,omitempty" json:"changes,omitempty"`
}
type AudienceHold struct {
	StudentRef string `yaml:"student_ref" json:"student_ref"`
	Reason     string `yaml:"reason" json:"reason"`
	Authorized bool   `yaml:"authorized,omitempty" json:"authorized,omitempty"`
}
type AudienceChange struct {
	AppliedOperation   string                     `yaml:"applied_operation,omitempty" json:"applied_operation,omitempty"`
	AppliedStudentRefs []string                   `yaml:"applied_student_refs,omitempty" json:"applied_student_refs,omitempty"`
	AppliedRemoveRefs  []string                   `yaml:"applied_remove_refs,omitempty" json:"applied_remove_refs,omitempty"`
	Operation          string                     `yaml:"operation" json:"operation"`
	OverrideID         int64                      `yaml:"override_id,omitempty" json:"override_id,omitempty"`
	Before             AssignmentOverrideBaseline `yaml:"before" json:"before"`
	StudentRefs        []string                   `yaml:"student_refs,omitempty" json:"student_refs,omitempty"`
	RemoveRefs         []string                   `yaml:"remove_refs,omitempty" json:"remove_refs,omitempty"`
	Outcome            ActionOutcome              `yaml:"outcome,omitempty" json:"outcome,omitempty"`
}

func validateAudienceRefresh(a *Audience) error {
	st := a.State
	if st.Refresh == nil {
		return nil
	}
	if st.TargetID <= 0 {
		return fmt.Errorf("audience refresh requires an exact existing target")
	}
	desired := map[string]bool{}
	for _, ref := range st.StudentRefs {
		desired[ref] = true
	}
	ids := map[int64]bool{}
	for _, change := range st.Refresh.Changes {
		if err := validateOutcome(change.Outcome); err != nil {
			return err
		}
		if change.Operation != "create" && change.Operation != "update" && change.Operation != "remove" {
			return fmt.Errorf("audience refresh operation must be create, update or remove")
		}
		if change.Operation != "create" && !definiteAudienceChange(change.Outcome) {
			owned := false
			for _, row := range st.Owned {
				if row.ID == change.OverrideID && reflect.DeepEqual(row.Baseline, change.Before) {
					owned = true
				}
			}
			if !owned {
				return fmt.Errorf("audience refresh change requires its exact recorded owned baseline")
			}
		}
		if change.Operation != "create" {
			if change.OverrideID <= 0 || ids[change.OverrideID] {
				return fmt.Errorf("audience refresh requires unique exact owned override IDs")
			}
			ids[change.OverrideID] = true
			if len(change.Before.StudentRefs) == 0 || change.Before.GroupID != 0 || change.Before.CourseSectionID != 0 {
				return fmt.Errorf("audience refresh only changes owned student targets")
			}
		}
		if change.Operation == "create" {
			if len(change.StudentRefs) == 0 {
				return fmt.Errorf("audience addition requires student refs")
			}
			for _, ref := range change.StudentRefs {
				if !desired[ref] {
					return fmt.Errorf("audience addition is outside the prepared source")
				}
			}
			if len(change.RemoveRefs) > 0 {
				return fmt.Errorf("audience addition cannot remove students")
			}
		}
		if change.Operation == "remove" && len(change.StudentRefs) > 0 {
			return fmt.Errorf("removed audience row cannot retain student refs")
		}
		if change.Operation == "update" && len(change.StudentRefs) == 0 {
			return fmt.Errorf("empty audience row requires exact removal")
		}
		before := map[string]bool{}
		for _, ref := range change.Before.StudentRefs {
			before[ref] = true
		}
		after := map[string]bool{}
		for _, ref := range change.StudentRefs {
			if after[ref] {
				return fmt.Errorf("audience refresh student refs must be unique")
			}
			after[ref] = true
			if change.Operation != "create" && !before[ref] {
				return fmt.Errorf("owned audience update may only remove prepared students")
			}
		}
		if change.Operation != "create" {
			removed := []string{}
			for ref := range before {
				if !after[ref] {
					removed = append(removed, ref)
				}
			}
			requested := append([]string(nil), change.RemoveRefs...)
			sort.Strings(removed)
			sort.Strings(requested)
			if !reflect.DeepEqual(removed, requested) {
				return fmt.Errorf("audience removals differ from the prepared change")
			}
		}
	}
	return nil
}

func definiteAudienceChange(o ActionOutcome) bool {
	return o.Status == "succeeded" || o.Status == "preserved"
}
