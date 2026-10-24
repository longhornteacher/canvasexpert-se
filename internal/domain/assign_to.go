package domain

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Ref names existing Canvas content by title/slug or, when it is a bare YAML
// integer, by exact ID. A quoted number stays a title.
type Ref struct {
	Text string
	ID   int64
}

func (r *Ref) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind != yaml.ScalarNode {
		return fmt.Errorf("line %d: expected a title or numeric ID", n.Line)
	}
	if n.Tag == "!!int" {
		id, err := strconv.ParseInt(n.Value, 10, 64)
		if err != nil || id <= 0 {
			return fmt.Errorf("line %d: ID must be a positive integer", n.Line)
		}
		*r = Ref{ID: id}
		return nil
	}
	*r = Ref{Text: strings.TrimSpace(n.Value)}
	return nil
}

func (r Ref) MarshalYAML() (any, error) {
	if r.ID > 0 {
		return r.ID, nil
	}
	return r.Text, nil
}

func (r Ref) IsSet() bool { return r.ID > 0 || r.Text != "" }

func (r Ref) String() string {
	if r.ID > 0 {
		return strconv.FormatInt(r.ID, 10)
	}
	return r.Text
}

// AssignToDates are optional per-target dates, applied to the rows that carry
// the tag audience on assignments and quizzes.
type AssignToDates struct {
	DueAt    string `yaml:"due_at,omitempty"`
	UnlockAt string `yaml:"unlock_at,omitempty"`
	LockAt   string `yaml:"lock_at,omitempty"`
}

// AssignToOutcome is the saved result of one target, for the teacher's record.
type AssignToOutcome struct {
	Status  string   `yaml:"status,omitempty"` // unchanged, succeeded, held
	Message string   `yaml:"message,omitempty"`
	Notes   []string `yaml:"notes,omitempty"`
}

type AssignToTarget struct {
	Module     Ref            `yaml:"module,omitempty"`
	Page       Ref            `yaml:"page,omitempty"`
	Quiz       Ref            `yaml:"quiz,omitempty"`
	Assignment Ref            `yaml:"assignment,omitempty"`
	Discussion Ref            `yaml:"discussion,omitempty"`
	Tags       []string       `yaml:"tags,omitempty"`
	Everyone   *bool          `yaml:"everyone,omitempty"`
	Cascade    string         `yaml:"cascade,omitempty"` // module only: none, clear_item_overrides, apply_to_items
	Dates      *AssignToDates `yaml:"dates,omitempty"`
	Outcome    AssignToOutcome `yaml:"outcome,omitempty"`
}

// Kind and Ref return the one content kind this target names.
func (t AssignToTarget) KindRef() (string, Ref, int) {
	count := 0
	kind, ref := "", Ref{}
	for _, c := range []struct {
		kind string
		ref  Ref
	}{{"module", t.Module}, {"page", t.Page}, {"quiz", t.Quiz}, {"assignment", t.Assignment}, {"discussion", t.Discussion}} {
		if c.ref.IsSet() {
			count++
			kind, ref = c.kind, c.ref
		}
	}
	return kind, ref, count
}

// AssignToDocument sets Canvas Assign To (selective release) on existing
// content to differentiation tags. It never creates or edits tags.
type AssignToDocument struct {
	Schema                     int              `yaml:"schema"`
	Kind                       string           `yaml:"kind"`
	Course                     string           `yaml:"course"`
	Mode                       string           `yaml:"mode,omitempty"` // replace (default) or add
	Everyone                   *bool            `yaml:"everyone,omitempty"`
	AllowRemoveWithSubmissions bool             `yaml:"allow_remove_with_submissions,omitempty"`
	Targets                    []AssignToTarget `yaml:"targets"`
}

func (d AssignToDocument) EffectiveMode() string {
	if d.Mode == "" {
		return "replace"
	}
	return d.Mode
}

// EveryoneFor resolves the Everyone setting for one target: the target's own
// value, else the document's, else restricted when tags are named.
func (d AssignToDocument) EveryoneFor(t AssignToTarget) (value bool, explicit bool) {
	if t.Everyone != nil {
		return *t.Everyone, true
	}
	if d.Everyone != nil {
		return *d.Everyone, true
	}
	return len(t.Tags) == 0, false
}

func (d AssignToDocument) Validate() error {
	if d.Schema != 1 || d.Kind != "assign-to" {
		return fmt.Errorf("expected schema: 1, kind: assign-to")
	}
	if strings.TrimSpace(d.Course) == "" {
		return fmt.Errorf("assign-to requires course")
	}
	if m := d.EffectiveMode(); m != "replace" && m != "add" {
		return fmt.Errorf("mode must be replace or add")
	}
	if len(d.Targets) == 0 {
		return fmt.Errorf("assign-to requires at least one target")
	}
	for i, t := range d.Targets {
		kind, _, n := t.KindRef()
		if n != 1 {
			return fmt.Errorf("targets[%d] must name exactly one of module, page, quiz, assignment or discussion", i+1)
		}
		everyone, explicit := d.EveryoneFor(t)
		if len(t.Tags) == 0 && !everyone {
			return fmt.Errorf("targets[%d] names no tags; set everyone: true to make it visible to everyone again", i+1)
		}
		seen := map[string]bool{}
		for _, tag := range t.Tags {
			key := strings.ToLower(strings.TrimSpace(tag))
			if key == "" || seen[key] {
				return fmt.Errorf("targets[%d] has a blank or repeated tag", i+1)
			}
			seen[key] = true
		}
		if kind == "module" && everyone && len(t.Tags) > 0 {
			return fmt.Errorf("targets[%d]: a module visible to everyone cannot also list tags", i+1)
		}
		if kind != "module" && t.Cascade != "" {
			return fmt.Errorf("targets[%d]: cascade applies to module targets only", i+1)
		}
		if t.Cascade != "" && t.Cascade != "none" && t.Cascade != "clear_item_overrides" && t.Cascade != "apply_to_items" {
			return fmt.Errorf("targets[%d]: cascade must be none, clear_item_overrides or apply_to_items", i+1)
		}
		if t.Dates != nil {
			if kind != "assignment" && kind != "quiz" {
				return fmt.Errorf("targets[%d]: dates apply to assignment and quiz targets", i+1)
			}
			for _, v := range []string{t.Dates.DueAt, t.Dates.UnlockAt, t.Dates.LockAt} {
				if v != "" {
					if _, err := time.Parse(time.RFC3339, v); err != nil {
						return fmt.Errorf("targets[%d]: dates need RFC3339 with an explicit offset", i+1)
					}
				}
			}
			if len(t.Tags) == 0 {
				return fmt.Errorf("targets[%d]: dates need at least one tag to attach to", i+1)
			}
		}
		_ = explicit
	}
	return nil
}
