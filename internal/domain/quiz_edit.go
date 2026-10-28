package domain

import (
	"fmt"
	"regexp"
	"strings"
)

// NewQuizEditDocument changes content, never answers, points or student results.
// Maps preserve the distinction between absent, null and empty observed fields.
type NewQuizEditDocument struct {
	Schema       int                 `yaml:"schema"`
	Kind         string              `yaml:"kind"`
	Course       string              `yaml:"course"`
	Instance     string              `yaml:"instance"`
	CourseID     int64               `yaml:"course_id"`
	AssignmentID int64               `yaml:"assignment_id"`
	Observed     NewQuizEditSnapshot `yaml:"observed"`
	Changes      map[string]any      `yaml:"changes"`
	Items        []NewQuizEditItem   `yaml:"items"`
	Outcome      ActionOutcome       `yaml:"outcome,omitempty"`
}

type NewQuizEditSnapshot struct {
	Instance     string         `yaml:"instance"`
	CourseID     int64          `yaml:"course_id"`
	AssignmentID int64          `yaml:"assignment_id"`
	Fields       map[string]any `yaml:"fields"`
}

type NewQuizEditItem struct {
	ItemID              string                  `yaml:"item_id"`
	EntryID             string                  `yaml:"entry_id"`
	Position            int                     `yaml:"position"`
	EntryType           string                  `yaml:"entry_type"`
	InteractionTypeSlug string                  `yaml:"interaction_type_slug"`
	Editable            bool                    `yaml:"editable"`
	Reason              string                  `yaml:"reason,omitempty"`
	Observed            NewQuizItemEditSnapshot `yaml:"observed"`
	Changes             map[string]any          `yaml:"changes"`
	Outcome             ActionOutcome           `yaml:"outcome,omitempty"`
}

type NewQuizItemEditSnapshot struct {
	ItemID              string         `yaml:"item_id"`
	EntryID             string         `yaml:"entry_id"`
	EntryType           string         `yaml:"entry_type"`
	InteractionTypeSlug string         `yaml:"interaction_type_slug"`
	Fields              map[string]any `yaml:"fields"`
}

var quizEditID = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func (d *NewQuizEditDocument) Validate() error {
	if d.Schema != 1 || d.Kind != "new-quiz-edit" || !validNickname(d.Course) {
		return fmt.Errorf("expected schema: 1, kind: new-quiz-edit and a safe course nickname")
	}
	b := d.Observed
	if !validInstance(d.Instance) || d.CourseID <= 0 || d.AssignmentID <= 0 || d.Instance != b.Instance || d.CourseID != b.CourseID || d.AssignmentID != b.AssignmentID {
		return fmt.Errorf("New Quiz identity differs from the pulled baseline")
	}
	if err := validateQuizTextChanges(d.Changes, false); err != nil {
		return err
	}
	if err := validateOutcome(d.Outcome); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, it := range d.Items {
		b := it.Observed
		if !quizEditID.MatchString(it.ItemID) || seen[it.ItemID] || it.ItemID != b.ItemID || it.EntryID != b.EntryID || it.EntryType != b.EntryType || it.InteractionTypeSlug != b.InteractionTypeSlug {
			return fmt.Errorf("item identity differs from the pulled baseline or is repeated: %q", it.ItemID)
		}
		seen[it.ItemID] = true
		if err := validateQuizTextChanges(it.Changes, true); err != nil {
			return fmt.Errorf("item %s: %w", it.ItemID, err)
		}
		if err := validateOutcome(it.Outcome); err != nil {
			return err
		}
	}
	return nil
}

func validateQuizTextChanges(changes map[string]any, item bool) error {
	for key, value := range changes {
		if item && key == "feedback" {
			feedback, ok := value.(map[string]any)
			if !ok {
				return fmt.Errorf("feedback changes must be a mapping of neutral, correct or incorrect strings")
			}
			for k, v := range feedback {
				if k != "neutral" && k != "correct" && k != "incorrect" {
					return fmt.Errorf("unsupported feedback change %q", k)
				}
				if _, ok := v.(string); !ok {
					return fmt.Errorf("feedback.%s must be a string; use an empty string to clear", k)
				}
			}
			continue
		}
		if key != "title" && !(item && key == "item_body") && !(!item && key == "instructions") {
			return fmt.Errorf("unsupported New Quiz content change %q", key)
		}
		text, ok := value.(string)
		if !ok || key == "title" && strings.TrimSpace(text) == "" {
			return fmt.Errorf("%s must be a string (title cannot be empty); null is not an edit", key)
		}
	}
	return nil
}
