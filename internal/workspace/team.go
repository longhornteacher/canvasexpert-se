package workspace

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"canvasexpertse/internal/domain"
)

const teamPath = "context/team.md"

const teamBody = "Teammates whose Canvas keys are saved on the team PC. Keys themselves are never in this file.\n"

var teamLabel = regexp.MustCompile(`^[a-z][a-z0-9-]{0,23}$`)

// TeamConfig lists the teammates whose Canvas keys are saved on this PC.
// Keys themselves are never stored here.
type TeamConfig struct {
	Schema    int        `yaml:"schema"`
	Teammates []Teammate `yaml:"teammates"`
}

// Teammate is one colleague. Label is stable and appears only in files; Name is
// what teachers and families see.
type Teammate struct {
	Label        string `yaml:"label"`
	Name         string `yaml:"name"`
	CanvasUserID int64  `yaml:"canvas_user_id"`
	Email        string `yaml:"email,omitempty"`
	KeyExpires   string `yaml:"key_expires"`
}

// ValidTeamLabel reports whether s can be a teammate label.
func ValidTeamLabel(s string) bool { return teamLabel.MatchString(s) }

func (c TeamConfig) Validate() error {
	if c.Schema != 1 {
		return errors.New("team schema must be 1")
	}
	labels, ids := map[string]bool{}, map[int64]bool{}
	for i, t := range c.Teammates {
		switch {
		case !teamLabel.MatchString(t.Label):
			return fmt.Errorf("teammates[%d]: label must be lowercase letters, digits and hyphens, starting with a letter (24 characters at most)", i)
		case labels[t.Label]:
			return fmt.Errorf("teammates[%d]: label %q is used twice", i, t.Label)
		case t.CanvasUserID <= 0:
			return fmt.Errorf("teammates[%d]: canvas_user_id must be a positive number", i)
		case ids[t.CanvasUserID]:
			return fmt.Errorf("teammates[%d]: canvas_user_id %d is used twice", i, t.CanvasUserID)
		case strings.TrimSpace(t.Name) == "":
			return fmt.Errorf("teammates[%d]: name is required", i)
		case t.Email != "" && (!strings.Contains(t.Email, "@") || strings.ContainsAny(t.Email, " \t\r\n")):
			return fmt.Errorf("teammates[%d]: email must contain @ and no spaces", i)
		}
		if _, err := time.Parse("2006-01-02", t.KeyExpires); err != nil {
			return fmt.Errorf("teammates[%d]: key_expires must be a date like 2027-06-04", i)
		}
		labels[t.Label], ids[t.CanvasUserID] = true, true
	}
	return nil
}

// Find returns the teammate with this label, ignoring case.
func (c TeamConfig) Find(label string) (Teammate, bool) {
	for _, t := range c.Teammates {
		if strings.EqualFold(t.Label, label) {
			return t, true
		}
	}
	return Teammate{}, false
}

// Expiry states for a teammate's key, measured in the caller's local date.
const (
	KeyOK      = "ok"
	KeySoon    = "soon" // within 14 days of key_expires, or on that day
	KeyExpired = "expired"
)

// ExpiryState compares now's calendar date with key_expires. It never blocks
// anything: Canvas decides whether a key still works.
func (t Teammate) ExpiryState(now time.Time) string {
	expires, err := time.Parse("2006-01-02", t.KeyExpires)
	if err != nil {
		return KeyOK
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	switch {
	case today.After(expires):
		return KeyExpired
	case !today.Before(expires.AddDate(0, 0, -14)):
		return KeySoon
	}
	return KeyOK
}

// ReadTeam reads context/team.md through the teacher-setup authority. A missing
// file means no teammates.
func ReadTeam(root string) (TeamConfig, error) {
	p, err := ResolveTeacherPath(root, teamPath)
	if err != nil {
		return TeamConfig{}, err
	}
	if _, err := os.Stat(p); os.IsNotExist(err) {
		return TeamConfig{Schema: 1}, nil
	}
	var cfg TeamConfig
	if _, err := ReadDocument(p, &cfg); err != nil {
		return TeamConfig{}, err
	}
	return cfg, nil
}

func WriteTeam(root string, cfg TeamConfig) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	return WriteTeacherDocument(root, teamPath, cfg, teamBody)
}

// EnsureTeammateNotes creates the teammate's preference notes if absent. It
// never overwrites them.
func EnsureTeammateNotes(root string, t Teammate) error {
	if !teamLabel.MatchString(t.Label) {
		return fmt.Errorf("invalid teammate label %q", t.Label)
	}
	p, err := ResolveTeacherPath(root, "context/team/"+t.Label+".md")
	if err != nil {
		return err
	}
	if _, err := os.Stat(p); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	text := fmt.Sprintf("# %s\nPreferences for scoring, feedback, due times and publishing in %s's courses. Written by you after talking with %s.\n", t.Name, t.Name, t.Name)
	return WriteFile(p, []byte(text), 0644)
}

// CheckOwners rejects a course whose owner is not in the team list.
func CheckOwners(config domain.CourseConfig, team TeamConfig) error {
	for _, c := range config.Courses {
		if c.Owner == "" {
			continue
		}
		if _, ok := team.Find(c.Owner); !ok {
			return fmt.Errorf("course %q is owned by %q, who isn't on the Team page. Add that teammate on the Team page, or remove owner from the course in context/courses.md", c.Nickname, c.Owner)
		}
	}
	return nil
}

// MissingKeyMessage is the one message used wherever a teammate's key isn't on
// this PC (CLI, Hub and Test button).
func (t Teammate) MissingKeyMessage() string {
	return t.Name + "'s Canvas key isn't on this PC. Add it on the Team page, or run this on the team PC."
}
