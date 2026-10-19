package domain

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// GroupMembershipDocument is an exact, observed membership plan. The Engine
// records per-student outcomes so accepted and uncertain writes are not lost.
type GroupMembershipDocument struct {
	Schema                   int                      `yaml:"schema"`
	Kind                     string                   `yaml:"kind"`
	Operation                string                   `yaml:"operation"`
	Course                   string                   `yaml:"course"`
	Instance                 string                   `yaml:"instance,omitempty"`
	CourseID                 int64                    `yaml:"course_id,omitempty"`
	GroupCategoryID          int64                    `yaml:"group_category_id"`
	Changes                  []GroupMembershipChange  `yaml:"changes,omitempty"`
	Observed                 *GroupMembershipSnapshot `yaml:"observed"`
	AuthorizedWorkChangeRefs []string                 `yaml:"authorized_submitted_work_change_refs,omitempty"`
	Outcomes                 []GroupMembershipOutcome `yaml:"outcomes,omitempty"`
	Allocation               *GroupAllocationOutcome  `yaml:"allocation,omitempty"`
}

// NullableGroupID distinguishes an explicitly observed/desired null from an
// omitted field. Null source means observed unassigned; null destination means
// explicit removal.
type NullableGroupID struct {
	Set   bool
	Null  bool
	Value int64
}

func (v *NullableGroupID) UnmarshalYAML(n *yaml.Node) error {
	v.Set = true
	v.Null = n.Tag == "!!null"
	if v.Null {
		return nil
	}
	if n.Kind != yaml.ScalarNode {
		return fmt.Errorf("expected group ID or null")
	}
	return n.Decode(&v.Value)
}
func (v NullableGroupID) IsZero() bool { return !v.Set }
func (v NullableGroupID) MarshalYAML() (any, error) {
	if !v.Set || v.Null {
		return nil, nil
	}
	return v.Value, nil
}

type GroupMembershipChange struct {
	StudentRef string          `yaml:"student_ref"`
	FromGroup  NullableGroupID `yaml:"from_group_id"`
	ToGroup    NullableGroupID `yaml:"to_group_id"`
}

func (c *GroupMembershipChange) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind != yaml.MappingNode {
		return fmt.Errorf("membership change must be a mapping")
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		key, value := n.Content[i].Value, n.Content[i+1]
		switch key {
		case "student_ref":
			if err := value.Decode(&c.StudentRef); err != nil {
				return err
			}
		case "from_group_id":
			if err := decodeNullableGroupID(value, &c.FromGroup); err != nil {
				return err
			}
		case "to_group_id":
			if err := decodeNullableGroupID(value, &c.ToGroup); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown membership change field %q", key)
		}
	}
	return nil
}

func decodeNullableGroupID(n *yaml.Node, out *NullableGroupID) error {
	out.Set = true
	out.Null = n.Tag == "!!null"
	if out.Null {
		return nil
	}
	if n.Kind != yaml.ScalarNode {
		return fmt.Errorf("expected group ID or null")
	}
	return n.Decode(&out.Value)
}

type GroupMembershipSnapshot struct {
	Instance        string                    `yaml:"instance"`
	CourseID        int64                     `yaml:"course_id"`
	GroupCategoryID int64                     `yaml:"group_category_id"`
	GroupIDs        []int64                   `yaml:"group_ids"`
	RosterComplete  bool                      `yaml:"roster_complete"`
	Memberships     []GroupMembershipObserved `yaml:"memberships,omitempty"`
	PendingRequests []GroupMembershipPending  `yaml:"pending_requests,omitempty"`
	UnassignedRefs  []string                  `yaml:"unassigned_student_refs,omitempty"`
}
type GroupMembershipObserved struct {
	StudentRef string `yaml:"student_ref"`
	GroupID    int64  `yaml:"group_id"`
	State      string `yaml:"state"`
}
type GroupMembershipPending struct {
	StudentRef string `yaml:"student_ref" json:"student_ref"`
	GroupID    int64  `yaml:"group_id" json:"group_id"`
	State      string `yaml:"state" json:"state"`
}
type GroupMembershipOutcome struct {
	StudentRef    string        `yaml:"student_ref"`
	ActualGroupID *int64        `yaml:"actual_group_id,omitempty"`
	ActualState   string        `yaml:"actual_state,omitempty"`
	Outcome       ActionOutcome `yaml:"outcome"`
}
type GroupAllocationOutcome struct {
	Started               bool          `yaml:"started,omitempty"`
	StartedUnassignedRefs []string      `yaml:"started_unassigned_student_refs,omitempty"`
	Outcome               ActionOutcome `yaml:"outcome,omitempty"`
	AssignedRefs          []string      `yaml:"assigned_student_refs,omitempty"`
	UnmappedAssignedCount int           `yaml:"unmapped_assigned_count,omitempty"`
	Message               string        `yaml:"message,omitempty"`
}

type GroupRoster struct {
	Course          string                   `json:"course"`
	CourseID        int64                    `json:"course_id"`
	GroupCategoryID int64                    `json:"group_category_id"`
	GroupSetName    string                   `json:"group_set_name"`
	RosterComplete  bool                     `json:"roster_complete"`
	Groups          []GroupSummary           `json:"groups"`
	Unassigned      []GroupMember            `json:"unassigned_students"`
	PendingRequests []GroupMembershipPending `json:"pending_requests,omitempty"`
	UnresolvedCount int                      `json:"unresolved_membership_count,omitempty"`
	PendingCount    int                      `json:"pending_membership_count,omitempty"`
}

func (d GroupMembershipDocument) Validate() error {
	if d.Schema != 1 || d.Kind != "group-memberships" || (d.Operation != "update" && d.Operation != "allocate-unassigned") {
		return fmt.Errorf("expected schema: 1, kind: group-memberships and operation: update|allocate-unassigned")
	}
	if !validNickname(d.Course) || d.GroupCategoryID <= 0 || d.CourseID <= 0 || d.Instance == "" || d.Observed == nil {
		return fmt.Errorf("group-memberships requires configured course, exact set identity, and observed baseline")
	}
	b := d.Observed
	if b.Instance != d.Instance || b.CourseID != d.CourseID || b.GroupCategoryID != d.GroupCategoryID || !b.RosterComplete {
		return fmt.Errorf("group-memberships baseline identity must match and roster must be complete")
	}
	groupIDs := map[int64]bool{}
	for _, id := range b.GroupIDs {
		if id <= 0 || groupIDs[id] {
			return fmt.Errorf("group-memberships baseline has invalid or duplicate group IDs")
		}
		groupIDs[id] = true
	}
	if len(groupIDs) == 0 {
		return fmt.Errorf("group-memberships baseline requires existing groups")
	}
	observed := map[string]GroupMembershipObserved{}
	for _, m := range b.Memberships {
		if strings.TrimSpace(m.StudentRef) == "" || m.GroupID <= 0 || !groupIDs[m.GroupID] || (m.State != "accepted" && m.State != "invited") {
			return fmt.Errorf("group-memberships baseline contains an invalid membership")
		}
		if _, ok := observed[m.StudentRef]; ok {
			return fmt.Errorf("group-memberships baseline contains a student in multiple groups")
		}
		observed[m.StudentRef] = m
	}
	pending := map[string]map[int64]bool{}
	for _, m := range b.PendingRequests {
		if strings.TrimSpace(m.StudentRef) == "" || m.GroupID <= 0 || !groupIDs[m.GroupID] || m.State != "requested" {
			return fmt.Errorf("group-memberships baseline contains an invalid pending request")
		}
		if pending[m.StudentRef] == nil {
			pending[m.StudentRef] = map[int64]bool{}
		}
		if pending[m.StudentRef][m.GroupID] {
			return fmt.Errorf("group-memberships baseline contains a duplicate pending request")
		}
		pending[m.StudentRef][m.GroupID] = true
	}
	unassigned := map[string]bool{}
	for _, ref := range b.UnassignedRefs {
		_, hasMembership := observed[ref]
		if strings.TrimSpace(ref) == "" || unassigned[ref] || hasMembership || len(pending[ref]) > 0 {
			return fmt.Errorf("group-memberships baseline has invalid unassigned references")
		}
		unassigned[ref] = true
	}
	if d.Operation == "update" {
		if d.Allocation != nil {
			return fmt.Errorf("update cannot carry an allocation outcome")
		}
		seen := map[string]bool{}
		for i, c := range d.Changes {
			if strings.TrimSpace(c.StudentRef) == "" || seen[c.StudentRef] || !c.FromGroup.Set || !c.ToGroup.Set {
				return fmt.Errorf("changes[%d] requires a unique student_ref and explicit from/to group IDs (null allowed)", i)
			}
			seen[c.StudentRef] = true
			if c.FromGroup.Null {
				if !unassigned[c.StudentRef] {
					return fmt.Errorf("changes[%d] null from_group_id requires the student to be observed unassigned", i)
				}
			} else if c.FromGroup.Value <= 0 || !groupIDs[c.FromGroup.Value] || observed[c.StudentRef].GroupID != c.FromGroup.Value {
				return fmt.Errorf("changes[%d] from_group_id does not match the observed membership", i)
			}
			if c.ToGroup.Null {
				if c.FromGroup.Null {
					return fmt.Errorf("changes[%d] cannot remove an observed unassigned student", i)
				}
			} else if c.ToGroup.Value <= 0 || !groupIDs[c.ToGroup.Value] || (!c.FromGroup.Null && c.ToGroup.Value == c.FromGroup.Value) {
				return fmt.Errorf("changes[%d] requires a different destination group in the selected set", i)
			}
		}
		authorized := map[string]bool{}
		for _, ref := range d.AuthorizedWorkChangeRefs {
			if !seen[ref] || authorized[ref] {
				return fmt.Errorf("submitted-work authorization must name a changed student at most once")
			}
			authorized[ref] = true
		}
		outcomeRefs := map[string]bool{}
		for _, o := range d.Outcomes {
			if !seen[o.StudentRef] || outcomeRefs[o.StudentRef] {
				return fmt.Errorf("outcome has no matching requested change")
			}
			outcomeRefs[o.StudentRef] = true
			if err := validateOutcome(o.Outcome); err != nil {
				return err
			}
		}
		return nil
	}
	if len(d.Changes) != 0 || d.Allocation == nil || len(b.UnassignedRefs) == 0 {
		return fmt.Errorf("allocate-unassigned requires observed unassigned students and no explicit changes")
	}
	refs := map[string]bool{}
	for _, ref := range b.UnassignedRefs {
		if refs[ref] {
			return fmt.Errorf("duplicate unassigned student reference")
		}
		refs[ref] = true
	}
	if err := validateOutcome(d.Allocation.Outcome); err != nil {
		return err
	}
	assigned := map[string]bool{}
	for _, ref := range d.Allocation.AssignedRefs {
		if strings.TrimSpace(ref) == "" || assigned[ref] {
			return fmt.Errorf("allocation outcome contains an invalid or duplicate student reference")
		}
		assigned[ref] = true
	}
	return nil
}
