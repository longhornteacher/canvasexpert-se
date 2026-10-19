package domain

import (
	"fmt"
	"strings"
	"time"
)

// Page documents author ordinary course instruction. Hub tier pages remain a
// separate restricted workflow and never use this contract.
type PageDocument struct {
	Schema         int           `yaml:"schema"`
	Kind           string        `yaml:"kind"`
	Operation      string        `yaml:"operation"`
	Course         string        `yaml:"course"`
	Instance       string        `yaml:"instance,omitempty"`
	CourseID       int64         `yaml:"course_id,omitempty"`
	PageID         int64         `yaml:"page_id,omitempty"`
	GroupID        int64         `yaml:"group_id,omitempty"`
	Slug           string        `yaml:"slug,omitempty"`
	URL            string        `yaml:"url,omitempty"`
	Title          string        `yaml:"title"`
	BodyFormat     string        `yaml:"body_format"`
	Published      bool          `yaml:"published"`
	EditingRoles   *[]string     `yaml:"editing_roles,omitempty"`
	ModuleID       int64         `yaml:"module_id,omitempty"`
	ModulePosition int           `yaml:"module_position,omitempty"`
	Observed       *PageSnapshot `yaml:"observed,omitempty"`
	Created        *PageCreated  `yaml:"created,omitempty"`
	UpdateOutcome  ActionOutcome `yaml:"update_outcome,omitempty"`
	// Audience restricts a course Page to selected club members who work
	// individually. Group Pages and Hub tier Pages never use it.
	Audience *Audience `yaml:"audience,omitempty"`
	// ObservedDates is the pulled date-details baseline. A collection write
	// replaces every override, so unrelated rows and availability dates are
	// preserved from here.
	ObservedDates *PageDatesSnapshot `yaml:"observed_dates,omitempty"`
	// UnlockAt and LockAt are optional availability-date edits. Omitted dates
	// preserve the observed value; null clears it.
	UnlockAt *OptionalDate `yaml:"unlock_at,omitempty"`
	LockAt   *OptionalDate `yaml:"lock_at,omitempty"`
	// AudienceOutcome and DatesOutcome stay separate from UpdateOutcome so an
	// audience-only change never sends a stale body or title.
	AudienceOutcome ActionOutcome `yaml:"audience_outcome,omitempty"`
	DatesOutcome    ActionOutcome `yaml:"dates_outcome,omitempty"`
}

// PageDatesSnapshot is the exact date-details baseline for one course Page.
// Pages have availability dates and no graded due date.
type PageDatesSnapshot struct {
	OnlyVisibleToOverrides *bool             `yaml:"only_visible_to_overrides,omitempty"`
	VisibleToEveryone      *bool             `yaml:"visible_to_everyone,omitempty"`
	UnlockAt               *string           `yaml:"unlock_at,omitempty"`
	LockAt                 *string           `yaml:"lock_at,omitempty"`
	Overrides              []PageOverrideRow `yaml:"overrides,omitempty"`
}

// PageOverrideRow is one date-details override. Student targets use private
// refs; raw Canvas user IDs never enter a document.
type PageOverrideRow struct {
	ID              int64    `yaml:"id,omitempty"`
	Title           string   `yaml:"title,omitempty"`
	GroupID         int64    `yaml:"group_id,omitempty"`
	CourseSectionID int64    `yaml:"course_section_id,omitempty"`
	StudentRefs     []string `yaml:"student_refs,omitempty"`
	UnlockAt        *string  `yaml:"unlock_at,omitempty"`
	LockAt          *string  `yaml:"lock_at,omitempty"`
}

type PageSnapshot struct {
	Instance     string `yaml:"instance"`
	CourseID     int64  `yaml:"course_id"`
	GroupID      int64  `yaml:"group_id,omitempty"`
	PageID       int64  `yaml:"page_id"`
	Slug         string `yaml:"slug"`
	URL          string `yaml:"url"`
	Title        string `yaml:"title"`
	BodyHTML     string `yaml:"body_html"`
	Published    bool   `yaml:"published"`
	EditingRoles string `yaml:"editing_roles,omitempty"`
	UpdatedAt    string `yaml:"updated_at,omitempty"`
}

type PageCreated struct {
	PageID         int64         `yaml:"page_id,omitempty"`
	GroupID        int64         `yaml:"group_id,omitempty"`
	Slug           string        `yaml:"slug,omitempty"`
	URL            string        `yaml:"url,omitempty"`
	IntendedHTML   string        `yaml:"intended_html"`
	SavedHTML      string        `yaml:"saved_html"`
	IntendedTitle  string        `yaml:"intended_title"`
	ModuleID       int64         `yaml:"module_id,omitempty"`
	ModulePosition int           `yaml:"module_position,omitempty"`
	ModuleItemID   int64         `yaml:"module_item_id,omitempty"`
	Page           ActionOutcome `yaml:"page,omitempty"`
	Module         ActionOutcome `yaml:"module,omitempty"`
	Publication    ActionOutcome `yaml:"publication,omitempty"`
	Complete       bool          `yaml:"complete,omitempty"`
}

func ValidateBodyFormat(format string) error {
	if format != "" && format != "markdown" && format != "html" {
		return fmt.Errorf("body_format must be markdown or html")
	}
	return nil
}

func (d *PageDocument) ValidateBody(body string) error {
	if d.Schema != 1 || d.Kind != "page" || (d.Operation != "create" && d.Operation != "update") {
		return fmt.Errorf("expected schema: 1, kind: page and operation: create|update")
	}
	if !validNickname(d.Course) || strings.TrimSpace(d.Title) == "" {
		return fmt.Errorf("safe course nickname and page title required")
	}
	if e := ValidateBodyFormat(d.BodyFormat); e != nil {
		return e
	}
	if d.BodyFormat == "" {
		return fmt.Errorf("page body_format is required: markdown or html")
	}
	if d.Operation == "create" && strings.TrimSpace(body) == "" {
		return fmt.Errorf("page teaching body cannot be empty")
	}
	if d.Instance != "" && !validInstance(d.Instance) {
		return fmt.Errorf("instance must be an HTTPS Canvas origin")
	}
	if d.CourseID < 0 || d.PageID < 0 || d.GroupID < 0 || d.ModuleID < 0 || d.ModulePosition < 0 || (d.ModulePosition > 0 && d.ModuleID == 0) {
		return fmt.Errorf("invalid page/course/module identity or module position")
	}
	if d.GroupID > 0 && (d.ModuleID != 0 || d.ModulePosition != 0) {
		return fmt.Errorf("group Pages cannot be placed in a course module")
	}
	if d.EditingRoles != nil {
		if len(*d.EditingRoles) == 0 {
			return fmt.Errorf("editing_roles must include teachers or members")
		}
		seen := map[string]bool{}
		for _, role := range *d.EditingRoles {
			if role != "teachers" && role != "members" {
				return fmt.Errorf("editing_roles supports only teachers and members for group Pages")
			}
			if role == "members" && d.GroupID == 0 {
				return fmt.Errorf("members editing_roles requires group_id")
			}
			if seen[role] {
				return fmt.Errorf("editing_roles cannot repeat %q", role)
			}
			seen[role] = true
		}
	}
	if e := validateOutcome(d.UpdateOutcome); e != nil {
		return e
	}
	if e := validateOutcome(d.AudienceOutcome); e != nil {
		return e
	}
	if e := validateOutcome(d.DatesOutcome); e != nil {
		return e
	}
	if e := ValidateAudience(d.Audience); e != nil {
		return e
	}
	if d.Audience != nil {
		if d.GroupID > 0 {
			return fmt.Errorf("individual audience applies to course Pages, not group Pages")
		}
	}
	if d.ObservedDates != nil {
		if d.GroupID > 0 {
			return fmt.Errorf("date-details audience applies to course Pages, not group Pages")
		}
		if d.Operation == "create" && d.Audience == nil {
			return fmt.Errorf("create cannot carry a date-details baseline")
		}
		if d.ObservedDates.OnlyVisibleToOverrides != nil && d.ObservedDates.VisibleToEveryone != nil && *d.ObservedDates.OnlyVisibleToOverrides == *d.ObservedDates.VisibleToEveryone {
			return fmt.Errorf("observed date-details visibility is contradictory")
		}
		seen := map[int64]bool{}
		for i, row := range d.ObservedDates.Overrides {
			if row.ID < 0 || row.ID > 0 && seen[row.ID] {
				return fmt.Errorf("observed_dates.overrides[%d] requires a unique nonnegative override ID", i)
			}
			if row.ID > 0 {
				seen[row.ID] = true
			}
			targets := 0
			if row.GroupID > 0 {
				targets++
			}
			if row.CourseSectionID > 0 {
				targets++
			}
			if len(row.StudentRefs) > 0 {
				targets++
			}
			if targets != 1 {
				return fmt.Errorf("observed_dates.overrides[%d] requires exactly one supported target", i)
			}
			if e := validateObservedDate(row.UnlockAt); e != nil {
				return fmt.Errorf("observed_dates.overrides[%d] unlock_at: %w", i, e)
			}
			if e := validateObservedDate(row.LockAt); e != nil {
				return fmt.Errorf("observed_dates.overrides[%d] lock_at: %w", i, e)
			}
		}
		if e := validateObservedDate(d.ObservedDates.UnlockAt); e != nil {
			return fmt.Errorf("observed_dates.unlock_at: %w", e)
		}
		if e := validateObservedDate(d.ObservedDates.LockAt); e != nil {
			return fmt.Errorf("observed_dates.lock_at: %w", e)
		}
	}
	if d.UnlockAt != nil {
		if d.ObservedDates == nil {
			return fmt.Errorf("availability-date edits require the pulled date-details baseline")
		}
		if e := validateOptionalDate(d.UnlockAt); e != nil {
			return fmt.Errorf("unlock_at: %w", e)
		}
	}
	if d.LockAt != nil {
		if d.ObservedDates == nil {
			return fmt.Errorf("availability-date edits require the pulled date-details baseline")
		}
		if e := validateOptionalDate(d.LockAt); e != nil {
			return fmt.Errorf("lock_at: %w", e)
		}
	}
	if d.Operation == "create" {
		if d.Observed != nil || d.UpdateOutcome.Status != "" || d.PageID != 0 {
			return fmt.Errorf("create cannot carry an update baseline or page_id; created.page_id records a resumed create")
		}
	} else {
		b := d.Observed
		if b == nil || !validInstance(b.Instance) || b.CourseID <= 0 || b.PageID <= 0 || b.Slug == "" || strings.TrimSpace(b.Title) == "" {
			return fmt.Errorf("page update requires a pulled live baseline")
		}
		if d.Instance != b.Instance || d.CourseID != b.CourseID || d.GroupID != b.GroupID || d.PageID != b.PageID {
			return fmt.Errorf("page update identity differs from baseline")
		}
		if d.Created != nil || d.ModuleID != 0 || d.ModulePosition != 0 {
			return fmt.Errorf("page update cannot carry creation or module placement")
		}
		if d.UpdateOutcome.ID > 0 && d.UpdateOutcome.ID != d.PageID {
			return fmt.Errorf("page update outcome identity disagrees")
		}
	}
	if c := d.Created; c != nil {
		if c.PageID < 0 || c.GroupID < 0 || c.ModuleItemID < 0 || c.ModuleID < 0 || c.ModulePosition < 0 {
			return fmt.Errorf("invalid created page/module identity")
		}
		for _, outcome := range []ActionOutcome{c.Page, c.Module, c.Publication} {
			if e := validateOutcome(outcome); e != nil {
				return e
			}
		}
		if c.Page.ID > 0 && c.Page.ID != c.PageID {
			return fmt.Errorf("created page outcome identity disagrees")
		}
		if c.Page.Status == "succeeded" && c.PageID <= 0 {
			return fmt.Errorf("succeeded Page creation requires its exact page_id")
		}
		if c.Module.Status == "succeeded" && c.ModuleItemID <= 0 {
			return fmt.Errorf("succeeded module placement requires its exact module_item_id")
		}
		if c.Module.ID > 0 && c.Module.ID != c.ModuleItemID {
			return fmt.Errorf("created module item outcome identity disagrees")
		}
		if c.PageID > 0 && (d.Instance == "" || d.CourseID <= 0) {
			return fmt.Errorf("known page requires persisted origin/course")
		}
		if c.PageID > 0 && c.GroupID != d.GroupID {
			return fmt.Errorf("created Page group identity disagrees")
		}
		if c.PageID > 0 && c.Slug == "" && pageDefinite(c.Page) {
			return fmt.Errorf("confirmed page requires returned slug")
		}
		if c.Complete && (c.PageID <= 0 || !pageDefinite(c.Page) || !pageDefinite(c.Module) || !pageDefinite(c.Publication)) {
			return fmt.Errorf("complete page requires completed creation/placement/publication")
		}
		if c.Complete && d.ModuleID > 0 && (c.ModuleItemID <= 0 || c.Module.Status != "succeeded") {
			return fmt.Errorf("complete Page requires requested module placement")
		}
	}
	return nil
}

func pageDefinite(o ActionOutcome) bool {
	return o.Status == "succeeded" || o.Status == "preserved" || o.Status == "not-requested"
}

// validateObservedDate accepts an absent or RFC3339 observed date.
func validateObservedDate(d *string) error {
	if d == nil || *d == "" {
		return nil
	}
	if _, err := time.Parse(time.RFC3339, *d); err != nil {
		return fmt.Errorf("dates require RFC3339 with explicit offset")
	}
	return nil
}

// validateOptionalDate accepts an omitted date, an explicit null, or RFC3339.
func validateOptionalDate(d *OptionalDate) error {
	if d == nil || !d.Set {
		return nil
	}
	if d.Null {
		if d.Value != "" {
			return fmt.Errorf("a cleared date cannot also carry a value")
		}
		return nil
	}
	if d.Value == "" {
		return fmt.Errorf("a set date requires an RFC3339 value or null")
	}
	if _, err := time.Parse(time.RFC3339, d.Value); err != nil {
		return fmt.Errorf("dates require RFC3339 with explicit offset")
	}
	return nil
}
