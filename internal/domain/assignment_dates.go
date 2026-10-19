package domain

import (
	"fmt"
	"strings"
	"time"
)

// OptionalDate distinguishes omitted dates (preserve) from explicit null (clear).
type OptionalDate struct {
	Set   bool   `yaml:"set,omitempty"`
	Null  bool   `yaml:"null,omitempty"`
	Value string `yaml:"value,omitempty"`
}

type AssignmentOverrideBaseline struct {
	DueAtOverridden    *bool    `yaml:"due_at_overridden,omitempty" json:"due_at_overridden,omitempty"`
	UnlockAtOverridden *bool    `yaml:"unlock_at_overridden,omitempty" json:"unlock_at_overridden,omitempty"`
	LockAtOverridden   *bool    `yaml:"lock_at_overridden,omitempty" json:"lock_at_overridden,omitempty"`
	Title              string   `yaml:"title,omitempty"`
	GroupID            int64    `yaml:"group_id,omitempty"`
	StudentRefs        []string `yaml:"student_refs,omitempty"`
	CourseSectionID    int64    `yaml:"course_section_id,omitempty"`
	DueAt              *string  `yaml:"due_at,omitempty"`
	UnlockAt           *string  `yaml:"unlock_at,omitempty"`
	LockAt             *string  `yaml:"lock_at,omitempty"`
}

type AssignmentOverrideRow struct {
	Key             string                      `yaml:"key"`
	Operation       string                      `yaml:"operation"` // create, update, remove
	OverrideID      int64                       `yaml:"override_id,omitempty"`
	Title           string                      `yaml:"title,omitempty"`
	GroupID         int64                       `yaml:"group_id,omitempty"`
	StudentRefs     []string                    `yaml:"student_refs,omitempty"`
	CourseSectionID int64                       `yaml:"course_section_id,omitempty"`
	DueAt           OptionalDate                `yaml:"due_at,omitempty"`
	UnlockAt        OptionalDate                `yaml:"unlock_at,omitempty"`
	LockAt          OptionalDate                `yaml:"lock_at,omitempty"`
	Observed        *AssignmentOverrideBaseline `yaml:"observed,omitempty"`
	Outcome         ActionOutcome               `yaml:"outcome,omitempty"`
}

type AssignmentDatesDocument struct {
	Audience                       *Audience                 `yaml:"audience,omitempty"`
	Schema                         int                       `yaml:"schema"`
	Kind                           string                    `yaml:"kind"`
	Operation                      string                    `yaml:"operation"`
	Course                         string                    `yaml:"course"`
	Instance                       string                    `yaml:"instance"`
	CourseID                       int64                     `yaml:"course_id"`
	AssignmentID                   int64                     `yaml:"assignment_id"`
	OnlyVisibleToOverrides         *bool                     `yaml:"only_visible_to_overrides,omitempty"`
	ObservedOnlyVisibleToOverrides *bool                     `yaml:"observed_only_visible_to_overrides,omitempty"`
	VisibilityTarget               *bool                     `yaml:"visibility_target,omitempty"`
	VisibilityOutcome              ActionOutcome             `yaml:"visibility_outcome,omitempty"`
	EffectiveDates                 []AssignmentEffectiveDate `yaml:"effective_dates,omitempty"`
	Rows                           []AssignmentOverrideRow   `yaml:"overrides"`
}

type AssignmentEffectiveDate struct {
	OverrideID      int64   `yaml:"override_id,omitempty"`
	Base            bool    `yaml:"base,omitempty"`
	StudentRef      string  `yaml:"student_ref,omitempty"`
	GroupID         int64   `yaml:"group_id,omitempty"`
	CourseSectionID int64   `yaml:"course_section_id,omitempty"`
	DueAt           *string `yaml:"due_at,omitempty"`
	UnlockAt        *string `yaml:"unlock_at,omitempty"`
	LockAt          *string `yaml:"lock_at,omitempty"`
}

func ValidateAssignmentDates(d AssignmentDatesDocument) error {
	if err := ValidateAudience(d.Audience); err != nil {
		return err
	}
	if d.Audience != nil {
		if d.OnlyVisibleToOverrides != nil && !*d.OnlyVisibleToOverrides {
			return fmt.Errorf("individual audience cannot request visibility to Everyone")
		}
		for _, row := range d.Rows {
			if row.Operation != "update" || row.DueAt.Set || row.UnlockAt.Set || row.LockAt.Set {
				return fmt.Errorf("prepare individual audience changes separately from explicit override edits")
			}
		}
	}
	if d.Schema != 1 || d.Kind != "assignment-dates" || d.Operation != "update" {
		return fmt.Errorf("expected schema: 1, kind: assignment-dates, operation: update")
	}
	if !validNickname(d.Course) || !validInstance(d.Instance) || d.CourseID <= 0 || d.AssignmentID <= 0 {
		return fmt.Errorf("assignment-dates requires exact configured course, instance and positive IDs")
	}
	if d.OnlyVisibleToOverrides != nil && d.ObservedOnlyVisibleToOverrides == nil {
		return fmt.Errorf("changing only_visible_to_overrides requires its observed baseline")
	}
	if d.VisibilityTarget != nil && d.OnlyVisibleToOverrides == nil {
		return fmt.Errorf("visibility_target requires only_visible_to_overrides")
	}
	if err := validateOutcome(d.VisibilityOutcome); err != nil {
		return fmt.Errorf("visibility_outcome: %w", err)
	}
	if d.OnlyVisibleToOverrides != nil && d.VisibilityOutcome.Status == "not-requested" {
		return fmt.Errorf("requested visibility cannot have a not-requested outcome")
	}
	for i, row := range d.EffectiveDates {
		if row.OverrideID < 0 || row.GroupID < 0 || row.CourseSectionID < 0 {
			return fmt.Errorf("effective_dates[%d] target IDs cannot be negative", i)
		}
		if (row.OverrideID > 0) == row.Base {
			return fmt.Errorf("effective_dates[%d] must identify either an override_id or base: true", i)
		}
		for _, date := range []*string{row.DueAt, row.UnlockAt, row.LockAt} {
			if date != nil && *date != "" {
				if _, err := time.Parse(time.RFC3339, *date); err != nil {
					return fmt.Errorf("effective_dates[%d] dates require RFC3339 with explicit offset", i)
				}
			}
		}
	}
	keys := map[string]bool{}
	for i, r := range d.Rows {
		if strings.TrimSpace(r.Key) == "" || keys[r.Key] {
			return fmt.Errorf("overrides[%d] requires a unique key", i)
		}
		keys[r.Key] = true
		if r.GroupID < 0 || r.CourseSectionID < 0 || r.OverrideID < 0 {
			return fmt.Errorf("overrides[%d] IDs cannot be negative", i)
		}
		targetCount := 0
		if r.GroupID > 0 {
			targetCount++
		}
		if len(r.StudentRefs) > 0 {
			targetCount++
		}
		if r.CourseSectionID > 0 {
			targetCount++
		}
		if targetCount != 1 {
			return fmt.Errorf("overrides[%d] must choose exactly one target: group_id, student_refs, or course_section_id", i)
		}
		if len(r.StudentRefs) > 0 && strings.TrimSpace(r.Title) == "" {
			return fmt.Errorf("overrides[%d] student target requires a title", i)
		}
		if r.Operation != "create" && r.Operation != "update" && r.Operation != "remove" {
			return fmt.Errorf("overrides[%d] operation must be create, update, or remove", i)
		}
		if r.Operation == "create" {
			if r.Observed != nil {
				return fmt.Errorf("overrides[%d] create cannot carry an observed baseline", i)
			}
			if r.OverrideID != 0 && (r.Outcome.Status != "succeeded" || r.Outcome.ID <= 0 || r.Outcome.ID != r.OverrideID) {
				return fmt.Errorf("overrides[%d] create override_id is allowed only as a matching accepted outcome", i)
			}
			if r.OverrideID == 0 && r.Outcome.ID > 0 {
				return fmt.Errorf("overrides[%d] create outcome ID requires the matching saved override_id", i)
			}
			if !r.DueAt.Set && !r.UnlockAt.Set && !r.LockAt.Set && len(r.StudentRefs) == 0 {
				return fmt.Errorf("overrides[%d] create requires at least one date", i)
			}
		} else if r.OverrideID <= 0 || r.Observed == nil {
			return fmt.Errorf("overrides[%d] update/remove requires exact override_id and observed baseline", i)
		}
		if r.Operation == "remove" && (r.DueAt.Set || r.UnlockAt.Set || r.LockAt.Set) {
			return fmt.Errorf("overrides[%d] remove cannot also change dates", i)
		}
		for _, date := range []OptionalDate{r.DueAt, r.UnlockAt, r.LockAt} {
			if date.Null && !date.Set {
				return fmt.Errorf("overrides[%d] null date must also set the field", i)
			}
			if date.Set && !date.Null {
				if _, err := time.Parse(time.RFC3339, date.Value); err != nil {
					return fmt.Errorf("overrides[%d] dates require RFC3339 with explicit offset", i)
				}
			}
		}
		if r.Observed != nil {
			base := r.Observed
			if (base.GroupID > 0) != (r.GroupID > 0) || (base.CourseSectionID > 0) != (r.CourseSectionID > 0) || (len(base.StudentRefs) > 0) != (len(r.StudentRefs) > 0) {
				return fmt.Errorf("overrides[%d] target differs from observed baseline", i)
			}
			if base.GroupID != r.GroupID || base.CourseSectionID != r.CourseSectionID || !sameStrings(base.StudentRefs, r.StudentRefs) {
				return fmt.Errorf("overrides[%d] target differs from observed baseline", i)
			}
		}
		if err := validateOutcome(r.Outcome); err != nil {
			return fmt.Errorf("overrides[%d]: %w", i, err)
		}
	}
	return nil
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
