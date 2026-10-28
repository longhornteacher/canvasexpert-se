package domain

import (
	"fmt"
	"strings"
)

type TagSetChange struct {
	ID       int64  `yaml:"id,omitempty"`
	Name     string `yaml:"name"`
	RenameTo string `yaml:"rename_to,omitempty"`
}
type TagChange struct {
	ID       int64  `yaml:"id,omitempty"`
	Name     string `yaml:"name"`
	RenameTo string `yaml:"rename_to,omitempty"`
	Delete   bool   `yaml:"delete,omitempty"`
}
type TagMembers struct {
	Mode string              `yaml:"mode,omitempty"`
	Tags map[string][]string `yaml:",inline"`
}
type TagUsage struct {
	Kind         string `yaml:"kind"`
	ID           int64  `yaml:"id"`
	Title        string `yaml:"title"`
	AssignmentID int64  `yaml:"assignment_id,omitempty"`
}
type TagObserved struct {
	ID             int64      `yaml:"id"`
	Name           string     `yaml:"name"`
	Members        []string   `yaml:"members"`
	UnknownMembers bool       `yaml:"unknown_members,omitempty"`
	Usage          []TagUsage `yaml:"usage,omitempty"`
	UsageUnknown   bool       `yaml:"usage_unknown,omitempty"`
}
type TagsObserved struct {
	SetID   int64         `yaml:"set_id"`
	SetName string        `yaml:"set_name"`
	Tags    []TagObserved `yaml:"tags"`
}
type TagMemberOutcome struct {
	TagID      int64         `yaml:"tag_id"`
	StudentRef string        `yaml:"student_ref"`
	Operation  string        `yaml:"operation"`
	Outcome    ActionOutcome `yaml:"outcome"`
}
type TagsDocument struct {
	Schema                     int                `yaml:"schema"`
	Kind                       string             `yaml:"kind"`
	Course                     string             `yaml:"course"`
	Instance                   string             `yaml:"instance,omitempty"`
	CourseID                   int64              `yaml:"course_id,omitempty"`
	Set                        TagSetChange       `yaml:"set"`
	Tags                       []TagChange        `yaml:"tags,omitempty"`
	Members                    *TagMembers        `yaml:"members,omitempty"`
	AllowRemoveWithSubmissions bool               `yaml:"allow_remove_with_submissions,omitempty"`
	Observed                   *TagsObserved      `yaml:"observed,omitempty"`
	Structure                  ActionOutcome      `yaml:"structure,omitempty"`
	MemberOutcomes             []TagMemberOutcome `yaml:"member_outcomes,omitempty"`
}

func (d *TagsDocument) Validate() error {
	if d.Schema != 1 || d.Kind != "tags" || !validNickname(d.Course) || strings.TrimSpace(d.Set.Name) == "" || d.Set.ID < 0 || d.CourseID < 0 {
		return fmt.Errorf("tags requires schema: 1, kind: tags, safe course and a named nonnegative set identity")
	}
	if d.Instance != "" && !validInstance(d.Instance) {
		return fmt.Errorf("invalid Canvas instance")
	}
	if d.Set.RenameTo != "" && (d.Set.ID <= 0 || strings.TrimSpace(d.Set.RenameTo) == "") {
		return fmt.Errorf("set.rename_to requires an exact existing set ID")
	}
	finals, ids := map[string]bool{}, map[int64]bool{}
	for _, tag := range d.Tags {
		if strings.TrimSpace(tag.Name) == "" || tag.ID < 0 || tag.Delete && tag.RenameTo != "" {
			return fmt.Errorf("invalid tag name/ID or conflicting delete and rename")
		}
		if tag.ID > 0 && ids[tag.ID] {
			return fmt.Errorf("duplicate tag ID")
		}
		ids[tag.ID] = true
		name := tag.Name
		if tag.RenameTo != "" {
			name = tag.RenameTo
		}
		key := strings.ToLower(strings.Join(strings.Fields(name), " "))
		if finals[key] {
			return fmt.Errorf("duplicate final tag name %q", name)
		}
		finals[key] = true
	}
	if d.Members != nil {
		if d.Members.Mode != "" && d.Members.Mode != "replace" && d.Members.Mode != "add" {
			return fmt.Errorf("members.mode must be replace or add")
		}
		for name, refs := range d.Members.Tags {
			if strings.TrimSpace(name) == "" || refs == nil {
				return fmt.Errorf("members.%s must be an explicit list; use [] to clear", name)
			}
			seen := map[string]bool{}
			for _, ref := range refs {
				if strings.TrimSpace(ref) == "" || seen[ref] {
					return fmt.Errorf("duplicate or empty student in %s", name)
				}
				seen[ref] = true
			}
			for _, tag := range d.Tags {
				if tag.Delete && strings.EqualFold(tag.Name, name) {
					return fmt.Errorf("membership requested for deleted tag %q", name)
				}
			}
		}
	}
	if len(d.Tags) == 0 && (d.Members == nil || len(d.Members.Tags) == 0) && d.Set.RenameTo == "" {
		return fmt.Errorf("tags requires a structure or membership request")
	}
	if e := validateOutcome(d.Structure); e != nil {
		return e
	}
	for _, out := range d.MemberOutcomes {
		if out.TagID <= 0 || out.StudentRef == "" || (out.Operation != "add" && out.Operation != "remove") {
			return fmt.Errorf("invalid member outcome identity")
		}
		if e := validateOutcome(out.Outcome); e != nil {
			return e
		}
	}
	return nil
}
