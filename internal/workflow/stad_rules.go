package workflow

import (
	"errors"
	"fmt"
	"io/fs"
	"math/big"
	"path/filepath"
	"strings"
	"time"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

// STADRules is context/stad/<course>/rules.md: the class's point values. The
// playbook writes it from the teacher's choices; the Engine validates it and
// applies it exactly. A key left out takes the default below.
type STADRules struct {
	Schema     int              `yaml:"schema"`
	WeekStarts string           `yaml:"week_starts"`
	Effort     STADEffort       `yaml:"effort"`
	TeamBonus  STADTeamBonus    `yaml:"team_bonus"`
	Improve    *STADImprovement `yaml:"improvement,omitempty"`
	Awards     STADAwardRules   `yaml:"awards"`
	// Recognition, when set, gives each team a level by fixed thresholds
	// (Slavin's Super/Great/Good team), and the standings Page shows levels
	// in team-name order instead of ranks.
	Recognition *STADRecognition `yaml:"recognition,omitempty"`
}

type STADRecognition struct {
	Basis  string      `yaml:"basis"` // week (default) | round: which team score the levels judge
	Levels []STADLevel `yaml:"levels"`
}

// STADLevel is one recognition level; levels run highest first and a team
// takes the first it reaches. A team below every level gets none.
type STADLevel struct {
	Name    string         `yaml:"name"`
	AtLeast domain.Decimal `yaml:"at_least"`
}

type STADEffort struct {
	OnTime  domain.Decimal `yaml:"on_time"`
	Late    domain.Decimal `yaml:"late"`
	Missing domain.Decimal `yaml:"missing"`
}

type STADTeamBonus struct {
	// AllTurnedIn is added to a team's score for a week when every active
	// member turned in every due item (excused counts as turned in).
	AllTurnedIn domain.Decimal `yaml:"all_turned_in"`
}

type STADAwardRules struct {
	CapPerStudentPerWeek domain.Decimal `yaml:"cap_per_student_per_week"`
}

// STADImprovement is the optional true-STAD improvement scoring (slice 05).
type STADImprovement struct {
	Evidence         string          `yaml:"evidence"` // graded_items | quizzes | rubric_row
	RubricRow        string          `yaml:"rubric_row,omitempty"`
	Baseline         string          `yaml:"baseline"` // mean_of_last N
	MissingCountsAs  string          `yaml:"missing_counts_as"`
	Bands            []STADBand      `yaml:"bands"`
	Perfect          *STADPerfect    `yaml:"perfect,omitempty"`
	NoBaselinePoints *domain.Decimal `yaml:"no_baseline_points,omitempty"`
}

type STADBand struct {
	AtLeast   *domain.Decimal `yaml:"at_least,omitempty"`
	Otherwise bool            `yaml:"otherwise,omitempty"`
	Points    domain.Decimal  `yaml:"points"`
}

type STADPerfect struct {
	AtLeastPercent domain.Decimal `yaml:"at_least_percent"`
	Points         domain.Decimal `yaml:"points"`
}

func defaultSTADRules() STADRules {
	return STADRules{
		Schema: 1, WeekStarts: "monday",
		Effort:    STADEffort{OnTime: domain.MustDecimal("1"), Late: domain.MustDecimal("0.5"), Missing: domain.MustDecimal("0")},
		TeamBonus: STADTeamBonus{AllTurnedIn: domain.MustDecimal("2")},
		Awards:    STADAwardRules{CapPerStudentPerWeek: domain.MustDecimal("3")},
	}
}

// ReadSTADRules reads rules.md; a missing file means the defaults (effort
// only).
func ReadSTADRules(root, nickname string) (STADRules, error) {
	rules := defaultSTADRules()
	rel := filepath.ToSlash(filepath.Join("context", "stad", nickname, "rules.md"))
	if _, err := workspace.ReadTeacherDocument(root, rel, &rules); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return defaultSTADRules(), nil
		}
		return rules, fmt.Errorf("%s: %w", rel, err)
	}
	if err := rules.Validate(); err != nil {
		return rules, fmt.Errorf("%s: %w", rel, err)
	}
	return rules, nil
}

func (r STADRules) Validate() error {
	if r.WeekStarts != "monday" {
		return fmt.Errorf("week_starts: only monday is supported")
	}
	for name, d := range map[string]domain.Decimal{"effort.on_time": r.Effort.OnTime, "effort.late": r.Effort.Late, "effort.missing": r.Effort.Missing, "team_bonus.all_turned_in": r.TeamBonus.AllTurnedIn, "awards.cap_per_student_per_week": r.Awards.CapPerStudentPerWeek} {
		if d.Sign() < 0 {
			return fmt.Errorf("%s cannot be negative", name)
		}
	}
	if err := r.Recognition.validate(); err != nil {
		return err
	}
	if r.Improve == nil {
		return nil
	}
	im := r.Improve
	switch im.Evidence {
	case "graded_items", "quizzes":
	case "rubric_row":
		if strings.TrimSpace(im.RubricRow) == "" {
			return fmt.Errorf("improvement: evidence rubric_row needs rubric_row: \"<criterion description>\"")
		}
	default:
		return fmt.Errorf("improvement: evidence must be graded_items, quizzes or rubric_row, not %q", im.Evidence)
	}
	if _, err := ParseBaseline(im.Baseline); err != nil {
		return fmt.Errorf("improvement: %w", err)
	}
	if im.MissingCountsAs != "" && im.MissingCountsAs != "none" && im.MissingCountsAs != "zero" {
		return fmt.Errorf("improvement: missing_counts_as must be none or zero")
	}
	if len(im.Bands) == 0 {
		return fmt.Errorf("improvement: bands are required")
	}
	for i, b := range im.Bands {
		last := i == len(im.Bands)-1
		if b.Otherwise != last {
			return fmt.Errorf("improvement: bands must end with exactly one otherwise band, and only at the end")
		}
		if !b.Otherwise && b.AtLeast == nil {
			return fmt.Errorf("improvement: band %d needs at_least or otherwise", i+1)
		}
		if b.Points.Sign() < 0 {
			return fmt.Errorf("improvement: band %d points cannot be negative", i+1)
		}
	}
	return nil
}

func (rc *STADRecognition) validate() error {
	if rc == nil {
		return nil
	}
	if rc.Basis != "" && rc.Basis != "week" && rc.Basis != "round" {
		return fmt.Errorf("recognition: basis must be week or round")
	}
	if len(rc.Levels) == 0 {
		return fmt.Errorf("recognition: levels are required")
	}
	names := map[string]bool{}
	for i, l := range rc.Levels {
		if strings.TrimSpace(l.Name) == "" || names[l.Name] {
			return fmt.Errorf("recognition: level %d needs its own name", i+1)
		}
		names[l.Name] = true
		if !l.AtLeast.IsSet() {
			return fmt.Errorf("recognition: level %q needs at_least", l.Name)
		}
		if i > 0 && l.AtLeast.Cmp(rc.Levels[i-1].AtLeast) >= 0 {
			return fmt.Errorf("recognition: levels must run highest first; %q is not below %q", l.Name, rc.Levels[i-1].Name)
		}
	}
	return nil
}

// level is the first level the score reaches, or "".
func (rc *STADRecognition) level(score *big.Rat) string {
	for _, l := range rc.Levels {
		if score.Cmp(l.AtLeast.Rat()) >= 0 {
			return l.Name
		}
	}
	return ""
}

// ParseBaseline reads "mean_of_last N".
func ParseBaseline(s string) (int, error) {
	var n int
	var rest string
	if c, _ := fmt.Sscanf(strings.TrimSpace(s), "mean_of_last %d%s", &n, &rest); c < 1 || n < 1 || rest != "" {
		return 0, fmt.Errorf("baseline must look like \"mean_of_last 3\"")
	}
	return n, nil
}

// STADAwardsFile is context/stad/<course>/awards.md: teacher awards.
type STADAwardsFile struct {
	Schema int         `yaml:"schema"`
	Awards []STADAward `yaml:"awards"`
}

type STADAward struct {
	Date    string         `yaml:"date"`
	Student string         `yaml:"student,omitempty"` // student_ref, or
	Team    string         `yaml:"team,omitempty"`    // a team name
	Points  domain.Decimal `yaml:"points"`
	Reason  string         `yaml:"reason"`
}

func ReadSTADAwards(root, nickname string) (STADAwardsFile, error) {
	var f STADAwardsFile
	rel := filepath.ToSlash(filepath.Join("context", "stad", nickname, "awards.md"))
	if _, err := workspace.ReadTeacherDocument(root, rel, &f); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return STADAwardsFile{Schema: 1}, nil
		}
		return f, fmt.Errorf("%s: %w", rel, err)
	}
	for i, a := range f.Awards {
		if _, err := time.Parse("2006-01-02", a.Date); err != nil {
			return f, fmt.Errorf("%s: award %d needs a date like 2026-11-18", rel, i+1)
		}
		if (a.Student == "") == (a.Team == "") {
			return f, fmt.Errorf("%s: award %d must name either a student or a team", rel, i+1)
		}
		if !a.Points.IsSet() || strings.TrimSpace(a.Reason) == "" {
			return f, fmt.Errorf("%s: award %d needs points and a reason", rel, i+1)
		}
	}
	return f, nil
}
