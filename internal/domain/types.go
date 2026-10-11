package domain

import "strings"

// All machine-written documents use schema 1. Markdown/HTML body is returned
// separately by workspace.ReadDocument so live HTML is never round-tripped.
type Course struct {
	Nickname string `yaml:"nickname" json:"nickname"`
	ID       int64  `yaml:"id" json:"id"`
	Current  bool   `yaml:"current" json:"current"`
	Timezone string `yaml:"timezone,omitempty" json:"timezone,omitempty"`
}
type CourseConfig struct {
	Schema     int      `yaml:"schema"`
	Instance   string   `yaml:"instance"`
	Aliases    bool     `yaml:"aliases"`
	ModuleHint string   `yaml:"module_hint,omitempty"`
	Courses    []Course `yaml:"courses"`
}
type GradingConfig struct {
	Schema           int    `yaml:"schema"`
	Voice            string `yaml:"voice,omitempty"`
	FeedbackGuidance string `yaml:"feedback_guidance,omitempty"`
}
type StudentNotes struct {
	Schema int           `yaml:"schema"`
	Notes  []StudentNote `yaml:"notes"`
}
type StudentNote struct {
	StudentRef string `yaml:"student_ref,omitempty"`
	Name       string `yaml:"name,omitempty"`
	Course     string `yaml:"course,omitempty"`
	Note       string `yaml:"note"`
}
type Rating struct {
	ID              string  `yaml:"id" json:"id"`
	Points          Decimal `yaml:"points" json:"points"`
	Description     string  `yaml:"description" json:"description"`
	LongDescription string  `yaml:"long_description,omitempty" json:"long_description,omitempty"`
}
type Criterion struct {
	UseRange        bool     `yaml:"criterion_use_range,omitempty" json:"criterion_use_range,omitempty"`
	ID              string   `yaml:"id" json:"id"`
	Description     string   `yaml:"description" json:"description"`
	LongDescription string   `yaml:"long_description,omitempty" json:"long_description,omitempty"`
	Points          Decimal  `yaml:"points" json:"points"`
	Ratings         []Rating `yaml:"ratings,omitempty" json:"ratings,omitempty"`
}
type Rubric struct {
	ID                        int64       `yaml:"id,omitempty" json:"id,omitempty"`
	AssociationID             int64       `yaml:"association_id,omitempty" json:"association_id,omitempty"`
	Title                     string      `yaml:"title" json:"title"`
	UseForGrading             bool        `yaml:"use_for_grading" json:"use_for_grading"`
	UseForGradingKnown        bool        `yaml:"use_for_grading_known" json:"use_for_grading_known"`
	FreeFormCriterionComments bool        `yaml:"free_form_criterion_comments,omitempty" json:"free_form_criterion_comments,omitempty"`
	Criteria                  []Criterion `yaml:"criteria" json:"criteria"`
}
type AssignmentSnapshot struct {
	Instance         string   `yaml:"instance"`
	CourseID         int64    `yaml:"course_id"`
	AssignmentID     int64    `yaml:"assignment_id"`
	Title            string   `yaml:"title"`
	Points           Decimal  `yaml:"points"`
	DescriptionHTML  string   `yaml:"description_html"`
	SubmissionTypes  []string `yaml:"submission_types"`
	DueAt            *string  `yaml:"due_at,omitempty"`
	UnlockAt         *string  `yaml:"unlock_at,omitempty"`
	LockAt           *string  `yaml:"lock_at,omitempty"`
	Published        bool     `yaml:"published"`
	UpdatedAt        string   `yaml:"updated_at,omitempty"`
	Rubric           *Rubric  `yaml:"rubric,omitempty"`
	GradingType      string   `yaml:"grading_type"`
	AnonymousGrading bool     `yaml:"anonymous_grading"`
	ModeratedGrading bool     `yaml:"moderated_grading"`
	GroupCategoryID  int64    `yaml:"group_category_id,omitempty"`
}

// Tier is one differentiated-hub tier of a create document: a supplement page
// restricted to a Canvas differentiation tag. Empty Supports means no page.
type Tier struct {
	Label    string `yaml:"label"`
	Tag      string `yaml:"tag"`
	Title    string `yaml:"title,omitempty"`
	Supports string `yaml:"supports,omitempty"`
}

// PageTitle is the Canvas page title: explicit, or "<base> - <tag>".
func (t Tier) PageTitle(base string) string {
	if strings.TrimSpace(t.Title) != "" {
		return strings.TrimSpace(t.Title)
	}
	return strings.TrimSpace(base) + " - " + strings.TrimSpace(t.Tag)
}

// TierPage records one delivered tier page. It never holds membership.
type TierPage struct {
	Label         string `yaml:"label" json:"label"`
	Tag           string `yaml:"tag" json:"tag"`
	Title         string `yaml:"title" json:"title"`
	PageID        int64  `yaml:"page_id,omitempty" json:"page_id,omitempty"`
	Slug          string `yaml:"slug,omitempty" json:"slug,omitempty"`
	URL           string `yaml:"url,omitempty" json:"url,omitempty"`
	GroupID       int64  `yaml:"group_id,omitempty" json:"group_id,omitempty"`
	TagStatus     string `yaml:"tag_status,omitempty" json:"tag_status,omitempty"` // matched, not_found, ambiguous, unavailable, manual
	Published     bool   `yaml:"published,omitempty" json:"published,omitempty"`
	TeacherAction string `yaml:"teacher_action,omitempty" json:"teacher_action,omitempty"`
}

type CreatedResources struct {
	TierPages           []TierPage    `yaml:"tier_pages,omitempty"`
	StartedAt           string        `yaml:"create_started_at,omitempty"`
	IntendedHTML        string        `yaml:"intended_html,omitempty"`
	SavedHTML           string        `yaml:"saved_html,omitempty"`
	AssignmentID        int64         `yaml:"assignment_id,omitempty"`
	RubricID            int64         `yaml:"rubric_id,omitempty"`
	RubricAssociationID int64         `yaml:"rubric_association_id,omitempty"`
	ModuleItemID        int64         `yaml:"module_item_id,omitempty"`
	URL                 string        `yaml:"url,omitempty"`
	Complete            bool          `yaml:"complete,omitempty"`
	Assignment          ActionOutcome `yaml:"assignment,omitempty"`
	Rubric              ActionOutcome `yaml:"rubric,omitempty"`
	Module              ActionOutcome `yaml:"module,omitempty"`
	Publication         ActionOutcome `yaml:"publication,omitempty"`
}
type AssignmentDocument struct {
	Schema          int                 `yaml:"schema"`
	Kind            string              `yaml:"kind"`
	Operation       string              `yaml:"operation"`
	Course          string              `yaml:"course"`
	Instance        string              `yaml:"instance,omitempty"`
	CourseID        int64               `yaml:"course_id,omitempty"`
	AssignmentID    int64               `yaml:"assignment_id,omitempty"`
	Title           string              `yaml:"title"`
	BodyFormat      string              `yaml:"body_format,omitempty"`
	Points          Decimal             `yaml:"points"`
	SubmissionTypes []string            `yaml:"submission_types"`
	DueAt           *string             `yaml:"due_at,omitempty"`
	UnlockAt        *string             `yaml:"unlock_at,omitempty"`
	LockAt          *string             `yaml:"lock_at,omitempty"`
	Timezone        string              `yaml:"timezone,omitempty"`
	ModuleID        int64               `yaml:"module_id,omitempty"`
	Published       bool                `yaml:"published"`
	Rubric          *Rubric             `yaml:"rubric,omitempty"`
	Tiers           []Tier              `yaml:"tiers,omitempty"` // create-time differentiated hub
	Observed        *AssignmentSnapshot `yaml:"observed,omitempty"`
	Created         *CreatedResources   `yaml:"created,omitempty"`
	UpdateOutcome   ActionOutcome       `yaml:"update_outcome,omitempty"`
}
type PacketDocument struct {
	Schema     int                `yaml:"schema"`
	Kind       string             `yaml:"kind"`
	Course     string             `yaml:"course"`
	Assignment AssignmentSnapshot `yaml:"assignment"`
	Students   []string           `yaml:"students"`
}
type AttemptRef struct {
	Attempt     int    `yaml:"attempt" json:"attempt"`
	SubmittedAt string `yaml:"submitted_at" json:"submitted_at"`
}
type Attachment struct {
	ID           int64  `yaml:"id"`
	Name         string `yaml:"name"`
	Path         string `yaml:"path,omitempty"`
	ReadablePath string `yaml:"readable_path,omitempty"`
	Status       string `yaml:"status"`
	Reason       string `yaml:"reason,omitempty"`
}
type ExistingComment struct {
	ID        int64  `yaml:"id"`
	Attempt   *int   `yaml:"attempt,omitempty"`
	Text      string `yaml:"text"`
	CreatedAt string `yaml:"created_at,omitempty"`
}
type AttemptWork struct {
	Ref              AttemptRef       `yaml:"ref"`
	BodyHTML         string           `yaml:"body_html,omitempty"`
	ReadableText     string           `yaml:"readable_text,omitempty"`
	Attachments      []Attachment     `yaml:"attachments,omitempty"`
	Late             bool             `yaml:"late"`
	EffectiveDueAt   string           `yaml:"effective_due_at,omitempty"`
	EnteredGrade     *Decimal         `yaml:"entered_grade,omitempty"`
	RubricAssessment []CriterionScore `yaml:"rubric_assessment,omitempty"`
}
type StudentWorkDocument struct {
	Schema                 int               `yaml:"schema"`
	Kind                   string            `yaml:"kind"`
	StudentRef             string            `yaml:"student_ref"`
	Alias                  string            `yaml:"alias"`
	AssignmentID           int64             `yaml:"assignment_id"`
	Current                AttemptWork       `yaml:"current"`
	Prior                  []AttemptWork     `yaml:"prior,omitempty"`
	Comments               []ExistingComment `yaml:"comments,omitempty"`
	EnteredGradeKnown      bool              `yaml:"entered_grade_known"`
	EnteredGrade           *Decimal          `yaml:"entered_grade,omitempty"`
	ObservedEnteredScore   *Decimal          `yaml:"observed_entered_score,omitempty"`
	ObservedScore          *Decimal          `yaml:"observed_score,omitempty"`
	ObservedPointsDeducted *Decimal          `yaml:"observed_points_deducted,omitempty"`
	EnteredGradeReason     string            `yaml:"entered_grade_reason,omitempty"`
	Excused                bool              `yaml:"excused"`
	Monitored              bool              `yaml:"monitored"`
	Notes                  []string          `yaml:"notes,omitempty"`
}
type Component struct {
	Key          string   `yaml:"key"`
	Label        string   `yaml:"label"`
	Weight       Decimal  `yaml:"weight"`
	CriterionIDs []string `yaml:"criterion_ids,omitempty"`
}
type Interpretation struct {
	Mapping    string      `yaml:"mapping"` // repeated or direct
	Rationale  string      `yaml:"rationale"`
	Components []Component `yaml:"components"`
}
type CriterionScore struct {
	CriterionID string  `yaml:"criterion_id" json:"criterion_id"`
	Points      Decimal `yaml:"points" json:"points"`
	RatingID    string  `yaml:"rating_id,omitempty" json:"rating_id,omitempty"`
	Comment     string  `yaml:"comment,omitempty" json:"comment,omitempty"`
}
type ComponentScore struct {
	Key      string           `yaml:"key"`
	Evidence string           `yaml:"evidence"` // present, missing, unknown
	Criteria []CriterionScore `yaml:"criteria"`
}
type PriorComparison struct {
	Attempt         AttemptRef `yaml:"attempt"`
	Basis           string     `yaml:"basis"` // academic evidence, never teacher grade
	ComparisonLimit string     `yaml:"comparison_limit,omitempty"`
}

// LateTiming is the agent's own lateness decision. Basis is for the teacher;
// the Engine never parses it. DueAt guards against a deadline moved after the
// agent counted.
type LateTiming struct {
	Status      string `yaml:"status"` // late or none
	SecondsLate int64  `yaml:"seconds_late,omitempty"`
	DueAt       string `yaml:"due_at"`
	Basis       string `yaml:"basis"`
}

type ActionOutcome struct {
	Status     string `yaml:"status,omitempty"` // pending,succeeded,preserved,not-requested,held,failed,uncertain
	ID         int64  `yaml:"id,omitempty"`
	Message    string `yaml:"message,omitempty"`
	PostedText string `yaml:"posted_text,omitempty"`
}
type ActionOutcomes struct {
	Comment ActionOutcome `yaml:"comment,omitempty"`
	Rubric  ActionOutcome `yaml:"rubric,omitempty"`
	Grade   ActionOutcome `yaml:"grade,omitempty"`
	Late    ActionOutcome `yaml:"late,omitempty"`
}
type GradeOperation struct {
	Attempt        AttemptRef       `yaml:"attempt"`
	Components     []ComponentScore `yaml:"components"`
	Assessment     Decimal          `yaml:"assessment"`
	Feedback       string           `yaml:"feedback"`
	TeacherNote    string           `yaml:"teacher_note,omitempty"`
	StrongestPrior *PriorComparison `yaml:"strongest_prior,omitempty"`
	LateTiming     *LateTiming      `yaml:"late_timing,omitempty"`
	Outcomes       ActionOutcomes   `yaml:"outcomes,omitempty"`
}

// FeedbackAssessment supplies a preserved display basis, never write instructions.
type FeedbackAssessment struct {
	Assessment Decimal          `yaml:"assessment"`
	Components []ComponentScore `yaml:"components"`
}
type FeedbackRevision struct {
	Attempt    AttemptRef          `yaml:"attempt"`
	CommentID  int64               `yaml:"comment_id"`
	Feedback   string              `yaml:"feedback"`
	Assessment *FeedbackAssessment `yaml:"assessment,omitempty"`
	Outcome    ActionOutcome       `yaml:"outcome,omitempty"`
}
type StudentResult struct {
	StudentRef     string            `yaml:"student_ref"`
	Operation      string            `yaml:"operation"`
	Variant        string            `yaml:"variant,omitempty"`
	Grade          *GradeOperation   `yaml:"grade,omitempty"`
	ReviseFeedback *FeedbackRevision `yaml:"revise_feedback,omitempty"`
}
type ResultsDocument struct {
	Schema         int                `yaml:"schema"`
	Kind           string             `yaml:"kind"`
	Packet         string             `yaml:"packet"`
	Assignment     AssignmentSnapshot `yaml:"assignment"`
	Interpretation Interpretation     `yaml:"interpretation"`
	// ScoringBasis is the agent's own criteria for an assignment with no Canvas
	// rubric. It drives the arithmetic and the feedback score lines only; it is
	// never written to Canvas as a rubric.
	ScoringBasis *Rubric                 `yaml:"scoring_basis,omitempty"`
	Variants     []InterpretationVariant `yaml:"variants,omitempty"`
	Students     []StudentResult         `yaml:"students"`
}
type InterpretationVariant struct {
	Key            string         `yaml:"key"`
	Basis          string         `yaml:"basis"` // private established teacher accommodation
	Interpretation Interpretation `yaml:"interpretation"`
}

// Basis returns the criteria scoring is computed against: the live Canvas rubric
// when the assignment has one, otherwise the agent-supplied scoring basis.
func (d ResultsDocument) Basis() *Rubric {
	if d.Assignment.Rubric != nil {
		return d.Assignment.Rubric
	}
	return d.ScoringBasis
}
