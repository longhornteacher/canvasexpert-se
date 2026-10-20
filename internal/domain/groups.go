package domain

import (
	"bytes"
	"fmt"
	"gopkg.in/yaml.v3"
	"strings"
)

// GroupSetDocument creates a collaborative set or applies an exact-ID patch to
// a pulled set. Optional fields carry presence so omitted fields are preserved.
type GroupSetDocument struct {
	Schema          int               `yaml:"schema"`
	Kind            string            `yaml:"kind"`
	Operation       string            `yaml:"operation"`
	Course          string            `yaml:"course"`
	Instance        string            `yaml:"instance,omitempty"`
	CourseID        int64             `yaml:"course_id,omitempty"`
	GroupCategoryID int64             `yaml:"group_category_id,omitempty"`
	Name            string            `yaml:"name,omitempty"`
	SelfSignup      OptionalString    `yaml:"self_signup,omitempty"`
	GroupLimit      OptionalInt       `yaml:"group_limit,omitempty"`
	AutoLeader      OptionalString    `yaml:"auto_leader,omitempty"`
	Groups          []GroupSetRow     `yaml:"groups,omitempty"`
	Observed        *GroupSetSnapshot `yaml:"observed,omitempty"`
	Created         *GroupSetCreated  `yaml:"created,omitempty"`
	UpdateOutcome   ActionOutcome     `yaml:"update_outcome,omitempty"`
}

// yaml.v3 skips a field's UnmarshalYAML method for an explicit null. Preserve
// that requested clear on the containing document, while retaining strict
// field validation for the entire group-set draft.
func (d *GroupSetDocument) UnmarshalYAML(n *yaml.Node) error {
	type plain GroupSetDocument
	data, err := yaml.Marshal(n)
	if err != nil {
		return err
	}
	var decoded plain
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&decoded); err != nil {
		return err
	}
	*d = GroupSetDocument(decoded)
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i+1].Tag != "!!null" {
			continue
		}
		switch n.Content[i].Value {
		case "self_signup":
			d.SelfSignup = OptionalString{Set: true, Null: true}
		case "group_limit":
			d.GroupLimit = OptionalInt{Set: true, Null: true}
		case "auto_leader":
			d.AutoLeader = OptionalString{Set: true, Null: true}
		}
	}
	return nil
}

type OptionalString struct {
	Set   bool
	Null  bool
	Value string
}

func (o *OptionalString) UnmarshalYAML(n *yaml.Node) error {
	o.Set = true
	o.Null = n.Tag == "!!null"
	if !o.Null {
		if n.Kind != yaml.ScalarNode {
			return fmt.Errorf("expected string")
		}
		o.Value = n.Value
	}
	return nil
}
func (o OptionalString) IsZero() bool { return !o.Set }
func (o OptionalString) MarshalYAML() (any, error) {
	if !o.Set || o.Null {
		return nil, nil
	}
	return o.Value, nil
}

type OptionalInt struct {
	Set   bool
	Null  bool
	Value int
}

func (o *OptionalInt) UnmarshalYAML(n *yaml.Node) error {
	o.Set = true
	o.Null = n.Tag == "!!null"
	if !o.Null {
		if n.Kind != yaml.ScalarNode {
			return fmt.Errorf("expected integer")
		}
		if err := n.Decode(&o.Value); err != nil {
			return err
		}
	}
	return nil
}
func (o OptionalInt) IsZero() bool { return !o.Set }
func (o OptionalInt) MarshalYAML() (any, error) {
	if !o.Set || o.Null {
		return nil, nil
	}
	return o.Value, nil
}

type GroupSetRow struct {
	Key         string         `yaml:"key,omitempty"`
	GroupID     int64          `yaml:"group_id,omitempty"`
	Name        string         `yaml:"name"`
	Description *string        `yaml:"description,omitempty"`
	Observed    *GroupSnapshot `yaml:"observed,omitempty"`
}
type GroupSetSnapshot struct {
	Instance        string `yaml:"instance"`
	CourseID        int64  `yaml:"course_id"`
	GroupCategoryID int64  `yaml:"group_category_id"`
	Name            string `yaml:"name"`
	SelfSignup      string `yaml:"self_signup"`
	GroupLimit      *int   `yaml:"group_limit"`
	AutoLeader      string `yaml:"auto_leader"`
}
type GroupSnapshot struct {
	GroupID     int64  `yaml:"group_id"`
	Name        string `yaml:"name"`
	Description string `yaml:"description,omitempty"`
}
type GroupSetCreated struct {
	GroupCategoryID int64                `yaml:"group_category_id,omitempty"`
	Started         bool                 `yaml:"started,omitempty"`
	Set             ActionOutcome        `yaml:"set,omitempty"`
	Groups          []GroupCreateOutcome `yaml:"groups,omitempty"`
	Settings        ActionOutcome        `yaml:"settings,omitempty"`
}
type GroupCreateOutcome struct {
	Key     string        `yaml:"key"`
	GroupID int64         `yaml:"group_id,omitempty"`
	URL     string        `yaml:"url,omitempty"`
	Outcome ActionOutcome `yaml:"outcome,omitempty"`
}

type GroupSetSummary struct {
	ID            int64  `json:"id"`
	CourseID      int64  `json:"course_id"`
	Name          string `json:"name"`
	Collaborative bool   `json:"collaborative"`
	SelfSignup    string `json:"self_signup"`
	GroupLimit    *int   `json:"group_limit"`
	AutoLeader    string `json:"auto_leader"`
	GroupCount    int    `json:"group_count"`
}
type GroupSummary struct {
	ID              int64         `json:"id"`
	CourseID        int64         `json:"course_id"`
	GroupCategoryID int64         `json:"group_category_id"`
	Name            string        `json:"name"`
	Description     string        `json:"description,omitempty"`
	Collaborative   bool          `json:"collaborative"`
	MembersCount    int           `json:"members_count"`
	Members         []GroupMember `json:"members,omitempty"`
	RosterComplete  bool          `json:"roster_complete,omitempty"`
}
type GroupMember struct {
	StudentRef string `json:"student_ref"`
	Alias      string `json:"alias"`
	State      string `json:"state"`
}

func (d GroupSetDocument) Validate() error {
	if d.Schema != 1 || d.Kind != "group-set" || (d.Operation != "create" && d.Operation != "update") {
		return fmt.Errorf("expected schema: 1, kind: group-set and operation: create|update")
	}
	if !validNickname(d.Course) {
		return fmt.Errorf("configured course nickname required")
	}
	if d.CourseID < 0 || d.GroupCategoryID < 0 {
		return fmt.Errorf("course and group-set IDs cannot be negative")
	}
	if d.Operation == "create" {
		if strings.TrimSpace(d.Name) == "" {
			return fmt.Errorf("group-set name is required")
		}
		if d.Observed != nil || (d.GroupCategoryID != 0 && d.Created == nil) {
			return fmt.Errorf("create cannot carry an update baseline or an unrecorded group_category_id")
		}
	} else {
		if d.Observed == nil || d.Observed.GroupCategoryID <= 0 || d.Instance == "" || d.CourseID <= 0 || d.GroupCategoryID != d.Observed.GroupCategoryID || d.Instance != d.Observed.Instance || d.CourseID != d.Observed.CourseID {
			return fmt.Errorf("update requires the exact pulled group-set baseline and identity")
		}
		if !d.SelfSignup.Set && !d.GroupLimit.Set && !d.AutoLeader.Set && strings.TrimSpace(d.Name) == "" && len(d.Groups) == 0 {
			return fmt.Errorf("update has no requested changes")
		}
	}
	if d.SelfSignup.Set && !d.SelfSignup.Null && !enum(d.SelfSignup.Value, "disabled", "enabled", "restricted") {
		return fmt.Errorf("self_signup must be disabled, enabled, or restricted")
	}
	if d.AutoLeader.Set && !d.AutoLeader.Null && !enum(d.AutoLeader.Value, "none", "first", "random") {
		return fmt.Errorf("auto_leader must be none, first, or random")
	}
	if d.GroupLimit.Set && !d.GroupLimit.Null && d.GroupLimit.Value <= 0 {
		return fmt.Errorf("group_limit must be a positive integer or null")
	}
	if d.GroupLimit.Set && d.GroupLimit.Value > 0 && ((!d.SelfSignup.Set && (d.Observed == nil || d.Observed.SelfSignup == "disabled")) || (d.SelfSignup.Set && (d.SelfSignup.Null || d.SelfSignup.Value == "disabled"))) {
		return fmt.Errorf("group_limit requires self sign-up")
	}
	keys, ids := map[string]bool{}, map[int64]bool{}
	for i, g := range d.Groups {
		if g.GroupID < 0 || (g.GroupID == 0 && strings.TrimSpace(g.Name) == "") {
			return fmt.Errorf("groups[%d] requires a name for creation and nonnegative group_id", i)
		}
		if g.Key != "" {
			if keys[g.Key] {
				return fmt.Errorf("duplicate group key %q", g.Key)
			}
			keys[g.Key] = true
		}
		if g.GroupID > 0 {
			if ids[g.GroupID] {
				return fmt.Errorf("duplicate group_id %d", g.GroupID)
			}
			ids[g.GroupID] = true
			if g.Observed == nil || g.Observed.GroupID != g.GroupID {
				return fmt.Errorf("group %d update requires its exact observed baseline", g.GroupID)
			}
		} else if g.Key == "" {
			return fmt.Errorf("new groups require a stable local key")
		}
	}
	for _, o := range []ActionOutcome{d.UpdateOutcome} {
		if err := validateOutcome(o); err != nil {
			return err
		}
	}
	if d.Created != nil {
		if d.Created.GroupCategoryID < 0 {
			return fmt.Errorf("invalid created group_category_id")
		}
		seen := map[string]bool{}
		for _, g := range d.Created.Groups {
			if g.Key == "" || seen[g.Key] || g.GroupID < 0 {
				return fmt.Errorf("invalid or duplicate created group outcome")
			}
			seen[g.Key] = true
			if err := validateOutcome(g.Outcome); err != nil {
				return err
			}
		}
		for _, o := range []ActionOutcome{d.Created.Set, d.Created.Settings} {
			if err := validateOutcome(o); err != nil {
				return err
			}
		}
	}
	return nil
}
func enum(v string, allowed ...string) bool {
	for _, a := range allowed {
		if v == a {
			return true
		}
	}
	return false
}
