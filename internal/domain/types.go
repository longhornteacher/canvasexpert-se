package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// All machine-written documents use schema 1. Markdown/HTML body is returned
// separately by workspace.ReadDocument so live HTML is never round-tripped.
type Course struct {
	Nickname string `yaml:"nickname" json:"nickname"`
	ID       int64  `yaml:"id" json:"id"`
	Current  bool   `yaml:"current" json:"current"`
	Timezone string `yaml:"timezone,omitempty" json:"timezone,omitempty"`
	// Owner is the label of the teammate whose course this is; empty means yours.
	Owner string `yaml:"owner,omitempty" json:"owner,omitempty"`
	// EffortFloor is the lowest grade the teacher enters for an honest attempt,
	// as a percent of the assignment's points; nil means no floor.
	EffortFloor *Decimal `yaml:"effort_floor,omitempty" json:"effort_floor,omitempty"`
	// LateWork holds this course's own late-work settings; a line set here wins
	// over the same line in context/grading.md.
	LateWork *LateWorkPolicy `yaml:"late_work,omitempty" json:"late_work,omitempty"`
}
type CourseConfig struct {
	Schema     int      `yaml:"schema"`
	Instance   string   `yaml:"instance"`
	Aliases    bool     `yaml:"aliases"`
	ModuleHint string   `yaml:"module_hint,omitempty"`
	Courses    []Course `yaml:"courses"`
}
type GradingConfig struct {
	Schema           int             `yaml:"schema"`
	Voice            string          `yaml:"voice,omitempty"`
	FeedbackGuidance string          `yaml:"feedback_guidance,omitempty"`
	LateWork         *LateWorkPolicy `yaml:"late_work,omitempty"`
	Retakes          *RetakePolicy   `yaml:"retakes,omitempty"`
}

// RetakePolicy is the teacher's Major retake policy. CapPercent is the most a
// tutorial retake can contribute to its original, and the grade below which a
// student is offered the retake. SchoolDays is the retake window counted from
// the day Canvas posted the student's grade. No block means no retake policy.
type RetakePolicy struct {
	CapPercent int `yaml:"cap_percent" json:"cap_percent"`
	SchoolDays int `yaml:"school_days" json:"school_days"`
}

// Validate keeps both numbers plausible.
func (p *RetakePolicy) Validate() error {
	if p == nil {
		return nil
	}
	if p.CapPercent < 1 || p.CapPercent > 100 {
		return fmt.Errorf("retakes.cap_percent must be between 1 and 100")
	}
	if p.SchoolDays < 1 || p.SchoolDays > 60 {
		return fmt.Errorf("retakes.school_days must be between 1 and 60 school days")
	}
	return nil
}

// LateWorkPolicy is the teacher's school-day schedule shown in a new
// assignment's grading timetable. Zero leaves that row out. The penalty and
// its floor are not here: they come from Canvas's course late policy.
type LateWorkPolicy struct {
	// CorrectionSchoolDays is the teacher's correction window. Only a positive
	// value offers corrections; there is no default. A course line of 0 turns
	// off a window set in context/grading.md for that course.
	CorrectionSchoolDays *int `yaml:"correction_school_days,omitempty" json:"correction_school_days,omitempty"`
	CloseAfterSchoolDays int  `yaml:"close_after_school_days,omitempty" json:"close_after_school_days,omitempty"`
	ExtraTimeSchoolDays  int  `yaml:"extra_time_school_days,omitempty" json:"extra_time_school_days,omitempty"`
	// LateCutoffSchoolDays is the school's late-work limit: work is not accepted
	// more than this many school days after the due date, and the assignment
	// locks then. 0 means late work is not accepted at all; nil means no limit.
	LateCutoffSchoolDays *int `yaml:"late_cutoff_school_days,omitempty" json:"late_cutoff_school_days,omitempty"`
}

// CorrectionWindow is the authorized correction window in school days; 0
// means the teacher has not offered corrections.
func (p *LateWorkPolicy) CorrectionWindow() int {
	if p == nil || p.CorrectionSchoolDays == nil || *p.CorrectionSchoolDays < 0 {
		return 0
	}
	return *p.CorrectionSchoolDays
}

// Merged returns p with every line set in course taking its place. A course's
// correction_school_days, including 0, replaces the shared one.
func (p *LateWorkPolicy) Merged(course *LateWorkPolicy) *LateWorkPolicy {
	if course == nil {
		return p
	}
	out := LateWorkPolicy{}
	if p != nil {
		out = *p
	}
	if course.CorrectionSchoolDays != nil {
		out.CorrectionSchoolDays = course.CorrectionSchoolDays
	}
	if course.CloseAfterSchoolDays > 0 {
		out.CloseAfterSchoolDays = course.CloseAfterSchoolDays
	}
	if course.ExtraTimeSchoolDays > 0 {
		out.ExtraTimeSchoolDays = course.ExtraTimeSchoolDays
	}
	if course.LateCutoffSchoolDays != nil {
		out.LateCutoffSchoolDays = course.LateCutoffSchoolDays
	}
	return &out
}

// Validate keeps each count a plausible number of school days.
func (p *LateWorkPolicy) Validate() error {
	if p == nil {
		return nil
	}
	for _, v := range []struct {
		name string
		n    int
	}{{"close_after_school_days", p.CloseAfterSchoolDays}, {"extra_time_school_days", p.ExtraTimeSchoolDays}} {
		if v.n < 0 || v.n > 60 {
			return fmt.Errorf("late_work.%s must be between 0 and 60 school days", v.name)
		}
	}
	if n := p.CorrectionSchoolDays; n != nil && (*n < 0 || *n > 60) {
		return fmt.Errorf("late_work.correction_school_days must be between 0 and 60 school days")
	}
	if n := p.LateCutoffSchoolDays; n != nil && (*n < 0 || *n > 60) {
		return fmt.Errorf("late_work.late_cutoff_school_days must be between 0 and 60 school days")
	}
	return nil
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
	Library                   string      `yaml:"library,omitempty" json:"library,omitempty"`
	ID                        int64       `yaml:"id,omitempty" json:"id,omitempty"`
	AssociationID             int64       `yaml:"association_id,omitempty" json:"association_id,omitempty"`
	Title                     string      `yaml:"title" json:"title"`
	UseForGrading             bool        `yaml:"use_for_grading" json:"use_for_grading"`
	UseForGradingKnown        bool        `yaml:"use_for_grading_known" json:"use_for_grading_known"`
	FreeFormCriterionComments bool        `yaml:"free_form_criterion_comments,omitempty" json:"free_form_criterion_comments,omitempty"`
	Criteria                  []Criterion `yaml:"criteria" json:"criteria"`
}
type AssignmentSnapshot struct {
	Instance                       string                  `yaml:"instance"`
	CourseID                       int64                   `yaml:"course_id"`
	AssignmentID                   int64                   `yaml:"assignment_id"`
	Title                          string                  `yaml:"title"`
	Points                         Decimal                 `yaml:"points"`
	DescriptionHTML                string                  `yaml:"description_html"`
	SubmissionTypes                []string                `yaml:"submission_types"`
	AssignmentGroupID              *int64                  `yaml:"assignment_group_id,omitempty"`
	DueAt                          *string                 `yaml:"due_at,omitempty"`
	UnlockAt                       *string                 `yaml:"unlock_at,omitempty"`
	LockAt                         *string                 `yaml:"lock_at,omitempty"`
	Published                      bool                    `yaml:"published"`
	UpdatedAt                      string                  `yaml:"updated_at,omitempty"`
	Rubric                         *Rubric                 `yaml:"rubric,omitempty"`
	GradingType                    string                  `yaml:"grading_type"`
	AnonymousGrading               bool                    `yaml:"anonymous_grading"`
	ModeratedGrading               bool                    `yaml:"moderated_grading"`
	GroupCategoryID                int64                   `yaml:"group_category_id,omitempty"`
	GradeGroupStudentsIndividually *bool                   `yaml:"grade_group_students_individually,omitempty"`
	DiscussionTopicID              *int64                  `yaml:"discussion_topic_id,omitempty"`
	HasSubAssignments              *bool                   `yaml:"has_sub_assignments,omitempty"`
	Checkpoints                    *[]AssignmentCheckpoint `yaml:"checkpoints,omitempty"`
	NewQuiz                        bool                    `yaml:"new_quiz,omitempty"`
}

// AssignmentCheckpoint is the assignment-linked discussion checkpoint metadata
// returned when Canvas is asked to include checkpoints.
type AssignmentCheckpoint struct {
	Tag      string   `yaml:"tag,omitempty"`
	DueAt    *string  `yaml:"due_at,omitempty"`
	UnlockAt *string  `yaml:"unlock_at,omitempty"`
	LockAt   *string  `yaml:"lock_at,omitempty"`
	Points   *Decimal `yaml:"points_possible,omitempty"`
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
	TierPages           []TierPage                  `yaml:"tier_pages,omitempty"`
	StartedAt           string                      `yaml:"create_started_at,omitempty"`
	IntendedHTML        string                      `yaml:"intended_html,omitempty"`
	SavedHTML           string                      `yaml:"saved_html,omitempty"`
	AssignmentID        int64                       `yaml:"assignment_id,omitempty"`
	RubricID            int64                       `yaml:"rubric_id,omitempty"`
	RubricAssociationID int64                       `yaml:"rubric_association_id,omitempty"`
	ModuleItemID        int64                       `yaml:"module_item_id,omitempty"`
	URL                 string                      `yaml:"url,omitempty"`
	Complete            bool                        `yaml:"complete,omitempty"`
	Assignment          ActionOutcome               `yaml:"assignment,omitempty"`
	Rubric              ActionOutcome               `yaml:"rubric,omitempty"`
	RubricSelection     *RubricSelection            `yaml:"rubric_selection,omitempty"`
	Module              ActionOutcome               `yaml:"module,omitempty"`
	Publication         ActionOutcome               `yaml:"publication,omitempty"`
	Overrides           []AssignmentOverrideOutcome `yaml:"overrides,omitempty"`
	Visibility          ActionOutcome               `yaml:"visibility,omitempty"`
	Timetable           *TimetableRecord            `yaml:"timetable,omitempty"`
}

// TimetableRecord is the grading timetable fixed on the first create attempt,
// so a resumed create sends the same body. LockAt is set when the Engine chose
// the assignment's lock date; Note explains an omitted timetable or row.
type TimetableRecord struct {
	HTML   string `yaml:"html,omitempty"`
	LockAt string `yaml:"lock_at,omitempty"`
	Note   string `yaml:"note,omitempty"`
	// DropCorrections: no new timetable was made and an earlier CE timetable in
	// the body (for example from a copy) loses its correction lines, because no
	// correction window applies here.
	DropCorrections bool `yaml:"drop_corrections,omitempty"`
}
type AssignmentOverrideOutcome struct {
	Key        string        `yaml:"key"`
	OverrideID int64         `yaml:"override_id,omitempty"`
	Outcome    ActionOutcome `yaml:"outcome,omitempty"`
}
type AssignmentDocument struct {
	Audience                       *Audience               `yaml:"audience,omitempty"`
	Schema                         int                     `yaml:"schema"`
	Kind                           string                  `yaml:"kind"`
	Operation                      string                  `yaml:"operation"`
	Course                         string                  `yaml:"course"`
	Instance                       string                  `yaml:"instance,omitempty"`
	CourseID                       int64                   `yaml:"course_id,omitempty"`
	AssignmentID                   int64                   `yaml:"assignment_id,omitempty"`
	Title                          string                  `yaml:"title"`
	BodyFormat                     string                  `yaml:"body_format,omitempty"`
	Points                         Decimal                 `yaml:"points"`
	SubmissionTypes                []string                `yaml:"submission_types"`
	AssignmentGroupID              *int64                  `yaml:"assignment_group_id,omitempty"`
	DueAt                          *string                 `yaml:"due_at,omitempty"`
	UnlockAt                       *string                 `yaml:"unlock_at,omitempty"`
	LockAt                         *string                 `yaml:"lock_at,omitempty"`
	Timezone                       string                  `yaml:"timezone,omitempty"`
	ModuleID                       int64                   `yaml:"module_id,omitempty"`
	Published                      bool                    `yaml:"published"`
	GroupCategoryID                int64                   `yaml:"group_category_id,omitempty"`
	GradeGroupStudentsIndividually *bool                   `yaml:"grade_group_students_individually,omitempty"`
	OnlyVisibleToOverrides         *bool                   `yaml:"only_visible_to_overrides,omitempty"`
	Overrides                      []AssignmentOverrideRow `yaml:"overrides,omitempty"`
	Rubric                         *Rubric                 `yaml:"rubric,omitempty"`
	Tiers                          []Tier                  `yaml:"tiers,omitempty"` // create-time differentiated hub
	GradingTimetable               *bool                   `yaml:"grading_timetable,omitempty"`
	// OmitCorrections (create only) leaves the corrections row out of the
	// timetable even where a correction window is configured. Without a
	// configured window the timetable says nothing about corrections anyway.
	OmitCorrections bool `yaml:"omit_corrections,omitempty"`
	Observed                       *AssignmentSnapshot     `yaml:"observed,omitempty"`
	Created                        *CreatedResources       `yaml:"created,omitempty"`
	UpdateOutcome                  ActionOutcome           `yaml:"update_outcome,omitempty"`
}
type PacketDocument struct {
	Schema     int                `yaml:"schema"`
	Kind       string             `yaml:"kind"`
	Course     string             `yaml:"course"`
	Assignment AssignmentSnapshot `yaml:"assignment"`
	Students   []string           `yaml:"students"`
	// DirectionsHTML is the workspace-relative file holding the assignment's
	// directions HTML. New packets keep readable directions in the document
	// body and leave assignment.description_html empty.
	DirectionsHTML string `yaml:"directions_html,omitempty"`
	// DirectionImages are Canvas images embedded in the assignment directions,
	// downloaded beside the packet so reference images can be read.
	DirectionImages []Attachment `yaml:"direction_images,omitempty"`
	// WrittenItems lists a New Quiz's written items with their Canvas points.
	WrittenItems []WrittenItem `yaml:"written_items,omitempty"`
}

// WrittenItem is one New Quiz essay or file-upload item. Responses may name
// it by either id.
type WrittenItem struct {
	ItemID  string  `yaml:"item_id"`
	EntryID string  `yaml:"entry_id,omitempty"`
	Type    string  `yaml:"type"`
	Prompt  string  `yaml:"prompt,omitempty"`
	Points  Decimal `yaml:"points"`
}
type AttemptRef struct {
	Attempt     int    `yaml:"attempt" json:"attempt"`
	SubmittedAt string `yaml:"submitted_at" json:"submitted_at"`
}
// Attachment is one student upload. Status is downloaded (original saved at
// Path; inspect it), readable (also a text preview at ReadablePath), failed
// (Reason says why the file is unavailable; no Path) or not-downloaded.
// Packets before 2026-11 used unreadable for a saved original of any
// non-text type; with a Path it still names an original to inspect.
//
// Source says where the file came from: empty for a file uploaded with the
// work, embedded for a Canvas file referenced inside the student's own rich
// text (a pasted screenshot), media for a Canvas media recording submission,
// and comment for a file on a submission comment (supplemental, never the
// answer itself; only the student's own comment files are downloaded).
type Attachment struct {
	ID           int64  `yaml:"id"`
	Name         string `yaml:"name"`
	Source       string `yaml:"source,omitempty"`
	Path         string `yaml:"path,omitempty"`
	ReadablePath string `yaml:"readable_path,omitempty"`
	ContentType  string `yaml:"content_type,omitempty"`
	Size         int64  `yaml:"size,omitempty"` // bytes; saved size when downloaded, else Canvas's report
	// UpdatedAt is Canvas's updated_at for the file when Canvas returned it.
	// With ID it identifies the exact file a discussion entry carried.
	UpdatedAt string `yaml:"updated_at,omitempty"`
	Status    string `yaml:"status"`
	Reason    string `yaml:"reason,omitempty"`
}

// HasOriginal reports whether the record names a saved original to inspect,
// including a legacy unreadable row that kept its path.
func (a Attachment) HasOriginal() bool {
	return a.Path != "" && (a.Status == "downloaded" || a.Status == "readable" || a.Status == "unreadable")
}

// HasOriginal reports whether a New Quiz answer file was saved at Path.
func (f WrittenFile) HasOriginal() bool {
	return f.Path != "" && (f.Status == "downloaded" || f.Status == "readable")
}
type ExistingComment struct {
	ID        int64  `yaml:"id"`
	Attempt   *int   `yaml:"attempt,omitempty"`
	Text      string `yaml:"text"`
	CreatedAt string `yaml:"created_at,omitempty"`
	// ByStudent is true when the student wrote this comment. Attachments are
	// the comment's files (source comment): downloaded only for the student's
	// own comment, as supplemental evidence; anyone else's are listed only.
	ByStudent   bool         `yaml:"by_student,omitempty"`
	Attachments []Attachment `yaml:"attachments,omitempty"`
}
type AttemptWork struct {
	Ref              AttemptRef        `yaml:"ref"`
	SubmissionType   string            `yaml:"submission_type,omitempty"`
	URL              string            `yaml:"url,omitempty"`
	Note             string            `yaml:"note,omitempty"`
	BodyHTML         string            `yaml:"body_html,omitempty"` // older pulls only
	BodyHTMLPath     string            `yaml:"body_html_path,omitempty"`
	ReadableText     string            `yaml:"readable_text,omitempty"`
	WrittenResponses []WrittenResponse `yaml:"written_responses,omitempty"`
	WrittenFiles     []WrittenFile     `yaml:"written_files,omitempty"`
	Attachments      []Attachment      `yaml:"attachments,omitempty"`
	Late             bool              `yaml:"late"`
	ObservedTiming   *ObservedTiming   `yaml:"observed_timing,omitempty"`
	EffectiveDueAt   string            `yaml:"effective_due_at,omitempty"`
	EnteredGrade     *Decimal          `yaml:"entered_grade,omitempty"`
	RubricAssessment []CriterionScore  `yaml:"rubric_assessment,omitempty"`
	// AutoScored is the New Quiz auto-scored subtotal from Canvas's Student
	// Analysis report at pull time. Absent when unknown; AutoScoredNote says why.
	AutoScored     *AutoScored `yaml:"auto_scored,omitempty"`
	AutoScoredNote string      `yaml:"auto_scored_note,omitempty"`
}

// AutoScored totals the auto-scored items (everything but written items) of
// one New Quiz attempt. It is Engine-owned evidence, never agent input.
type AutoScored struct {
	Earned   Decimal `yaml:"earned"`
	Possible Decimal `yaml:"possible"`
	Items    int     `yaml:"items"`
}
type WrittenResponse struct {
	ItemID string `yaml:"item_id"`
	Prompt string `yaml:"prompt,omitempty"`
	Type   string `yaml:"type"`
	HTML   string `yaml:"html,omitempty"`
	Text   string `yaml:"text,omitempty"`
}
type WrittenFile struct {
	ItemID string `yaml:"item_id"`
	Name   string `yaml:"name"`
	Path   string `yaml:"path,omitempty"`
	Size   int64  `yaml:"size,omitempty"`
	Status string `yaml:"status"`
	Reason string `yaml:"reason,omitempty"`
}
type StudentWorkDocument struct {
	Schema               int               `yaml:"schema"`
	Kind                 string            `yaml:"kind"`
	WorkPurpose          string            `yaml:"work_purpose,omitempty"` // score or score-and-late; late-only appears only in older packets
	StudentRef           string            `yaml:"student_ref"`
	Alias                string            `yaml:"alias"`
	AssignmentID         int64             `yaml:"assignment_id"`
	AssignmentVisible    *bool             `yaml:"assignment_visible,omitempty"`
	Current              AttemptWork       `yaml:"current"`
	Prior                []AttemptWork     `yaml:"prior,omitempty"`
	Comments             []ExistingComment `yaml:"comments,omitempty"`
	EnteredGradeKnown    bool              `yaml:"entered_grade_known"`
	EnteredGrade         *Decimal          `yaml:"entered_grade,omitempty"`
	ObservedEnteredScore *Decimal          `yaml:"observed_entered_score,omitempty"`
	ObservedScore        *Decimal          `yaml:"observed_score,omitempty"`
	// ObservedGradedAt is Canvas's graded_at at pull time. Any SpeedGrader
	// save moves it, so a later difference means someone graded since.
	ObservedGradedAt string `yaml:"observed_graded_at,omitempty"`
	// ObservedQuizResult is a New Quiz attempt's current result version at
	// pull time. Every SpeedGrader save, even an item-comment-only one, makes
	// a new version, so item scores hold when it has changed.
	ObservedQuizResult     string   `yaml:"observed_quiz_result,omitempty"`
	ObservedPointsDeducted *Decimal `yaml:"observed_points_deducted,omitempty"`
	EnteredGradeReason     string   `yaml:"entered_grade_reason,omitempty"`
	Excused                bool     `yaml:"excused"`
	Monitored              bool     `yaml:"monitored"`
	ExtraTime              *bool    `yaml:"extra_time,omitempty"` // private saved fact; absent in older packets
	// ExtraTimeSchoolDays is the teacher's saved late-turn-in allowance for this
	// student, in school days; absent when no number is saved. Private like ExtraTime.
	ExtraTimeSchoolDays int `yaml:"extra_time_school_days,omitempty"`
	// DiscussionEvidence references this student's own attributed posts in the
	// pulled conversation packet when the assignment is a graded discussion.
	DiscussionEvidence *DiscussionEvidenceRef `yaml:"discussion_evidence,omitempty"`
	Notes              []string               `yaml:"notes,omitempty"`
}
type Component struct {
	Key          string   `yaml:"key"`
	Label        string   `yaml:"label"`
	Weight       Decimal  `yaml:"weight"`
	CriterionIDs []string `yaml:"criterion_ids,omitempty"`
	// ItemID names the New Quiz written item this part scores (from the
	// packet's written_items), so the comment shows it in quiz points.
	ItemID string `yaml:"item_id,omitempty"`
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
	// Feedback explains this part's score under its own score line. Exemplar
	// is a model answer the Engine always shows with the part's feedback.
	Feedback string `yaml:"feedback,omitempty"`
	Exemplar string `yaml:"exemplar,omitempty"`
}


// ItemFeedback reports whether every part carries its own feedback, so the
// comment needs no closing feedback.
func ItemFeedback(components []ComponentScore) bool {
	for _, c := range components {
		if strings.TrimSpace(c.Feedback) == "" {
			return false
		}
	}
	return len(components) > 0
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
	Status      string                `yaml:"status" json:"status"` // late or none
	SecondsLate int64                 `yaml:"seconds_late,omitempty" json:"seconds_late,omitempty"`
	DueAt       string                `yaml:"due_at" json:"due_at"`
	Basis       string                `yaml:"basis" json:"basis"`
	SchoolTime  *SchoolTimeProvenance `yaml:"school_time,omitempty" json:"school_time,omitempty"`
	Observed    *ObservedTiming       `yaml:"observed,omitempty" json:"observed,omitempty"`
}

// ObservedTiming preserves the timing fields seen on the current Canvas
// attempt. StatusKnown distinguishes an unavailable/omitted field from Canvas
// explicitly returning null.
type ObservedTiming struct {
	StatusKnown       bool    `yaml:"status_known" json:"status_known"`
	LatePolicyStatus  *string `yaml:"late_policy_status" json:"late_policy_status"`
	LateKnown         bool    `yaml:"late_known" json:"late_known"`
	Late              bool    `yaml:"late" json:"late"`
	SecondsLateKnown  bool    `yaml:"seconds_late_known" json:"seconds_late_known"`
	SecondsLate       int64   `yaml:"seconds_late" json:"seconds_late"`
	CustomStatusKnown bool    `yaml:"custom_status_known" json:"custom_status_known"`
	HasCustomStatus   bool    `yaml:"has_custom_status" json:"has_custom_status"`
}

// SchoolTimeProvenance records the actual first genuine submission and
// deadline used by the agent's school-calendar calculation.
type SchoolTimeProvenance struct {
	Attempt      AttemptRef `yaml:"attempt" json:"attempt"`
	SubmittedAt  string     `yaml:"submitted_at" json:"submitted_at"`
	Deadline     string     `yaml:"deadline" json:"deadline"`
	Status       string     `yaml:"status" json:"status"`
	SecondsLate  int64      `yaml:"seconds_late,omitempty" json:"seconds_late,omitempty"`
	SkippedDates []string   `yaml:"skipped_dates,omitempty" json:"skipped_dates,omitempty"`
	// AllowanceSchoolDays and AllowanceBase show how the Engine turned an approved
	// "+N school days" into Deadline; both are empty for an absolute deadline.
	AllowanceSchoolDays int    `yaml:"allowance_school_days,omitempty" json:"allowance_school_days,omitempty"`
	AllowanceBase       string `yaml:"allowance_base,omitempty" json:"allowance_base,omitempty"`
	// AbsentDates are the student's saved absences from the Canvas due date
	// through the submission. They were treated as non-school days, and a
	// deadline that fell on one moved to the next day the student was present.
	AbsentDates []string `yaml:"absent_dates,omitempty" json:"absent_dates,omitempty"`
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

// DiscussionEvidenceRef is the pulled selection of one student's discussion
// work: their own entries (by source-qualified entry reference) and a digest
// of exactly that text. Status is complete, partial or unavailable.
type DiscussionEvidenceRef struct {
	Packet    string   `yaml:"packet"`
	TopicID   int64    `yaml:"topic_id"`
	Status    string   `yaml:"status"`
	Reasons   []string `yaml:"reasons,omitempty"`
	EntryRefs []string `yaml:"entry_refs,omitempty"`
	Digest    string   `yaml:"digest,omitempty"`
}

// EvidenceSelection is the result-side record of what the assessment relied on:
// the pulled digest of the student's own entries, and any other people's
// entries explicitly cited as context.
type EvidenceSelection struct {
	Digest      string   `yaml:"digest"`
	EntryRefs   []string `yaml:"entry_refs,omitempty"`
	ContextRefs []string `yaml:"context_refs,omitempty"`
}

type GradeOperation struct {
	Attempt    AttemptRef         `yaml:"attempt"`
	Evidence   *EvidenceSelection `yaml:"evidence,omitempty"`
	Components []ComponentScore   `yaml:"components"`
	Assessment Decimal            `yaml:"assessment"`
	// HonestEffort is the agent's judgment that this is a genuine attempt, which
	// lets the course's effort floor raise the entered grade (never the assessment).
	HonestEffort   bool             `yaml:"honest_effort,omitempty"`
	Feedback       string           `yaml:"feedback"`
	Header         string           `yaml:"header,omitempty"`
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
	Header     string              `yaml:"header,omitempty"`
	Assessment *FeedbackAssessment `yaml:"assessment,omitempty"`
	Outcome    ActionOutcome       `yaml:"outcome,omitempty"`
}
type StudentResult struct {
	StudentRef     string                `yaml:"student_ref"`
	Operation      string                `yaml:"operation"`
	Variant        string                `yaml:"variant,omitempty"`
	Grade          *GradeOperation       `yaml:"grade,omitempty"`
	CommentOnly    *CommentOnlyOperation `yaml:"comment_only,omitempty"`
	ReviseFeedback *FeedbackRevision     `yaml:"revise_feedback,omitempty"`
	LateOnly       *LateOnlyOperation    `yaml:"late_only,omitempty" json:"late_only,omitempty"`
}

// LateOnlyOperation changes Canvas lateness fields without changing the
// academic grade, rubric assessment or feedback.
type LateOnlyOperation struct {
	Attempt    AttemptRef    `yaml:"attempt" json:"attempt"`
	LateTiming LateTiming    `yaml:"late_timing" json:"late_timing"`
	Outcome    ActionOutcome `yaml:"outcome,omitempty" json:"outcome,omitempty"`
}

// CommentOnlyOperation posts an attempt-scoped comment without grade, rubric,
// or late writes. Header none with no components sends feedback without scores.
type CommentOnlyOperation struct {
	Attempt    AttemptRef         `yaml:"attempt"`
	Evidence   *EvidenceSelection `yaml:"evidence,omitempty"`
	Components []ComponentScore   `yaml:"components,omitempty"`
	Feedback   string             `yaml:"feedback"`
	Header     string             `yaml:"header,omitempty"`
	Outcome    ActionOutcome      `yaml:"outcome,omitempty"`
	// ItemScores, on a New Quiz, first writes each part's item points (the
	// numbers the comment shows) into the quiz result, then posts the comment.
	// It uses an undocumented internal New Quizzes call.
	ItemScores        bool          `yaml:"item_scores,omitempty"`
	ItemScoresOutcome ActionOutcome `yaml:"item_scores_outcome,omitempty"`
	// QuizScore is the quiz total Canvas reported once item scores landed,
	// as X/Y; the whole-assignment comment states it. The Engine owns it.
	QuizScore string `yaml:"quiz_score,omitempty"`
}

func (c CommentOnlyOperation) FeedbackOnly() bool {
	return c.Header == "none" && len(c.Components) == 0
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

// DiscussionEntryRef identifies a source entry by context, topic and entry ID.
func DiscussionEntryRef(e DiscussionEntry) string {
	return fmt.Sprintf("%s:%d:%d:%d", e.ContextType, e.ContextID, e.TopicID, e.EntryID)
}

// DiscussionEntrySignature changes whenever a post's text, deletion state,
// edit time or attached files change. Files enter as Canvas file ID plus
// Canvas's updated_at, so a replaced file never validates against old
// evidence. A post with no attached files keeps its pre-2026-11 signature.
// Files embedded in the post's text are already covered by its HTML.
func DiscussionEntrySignature(e DiscussionEntry) string {
	base := discussionEntryBase(e)
	if files := DiscussionEntryFiles(e, true); len(files) > 0 {
		base += "\x00files:" + strings.Join(files, ",")
	}
	sum := sha256.Sum256([]byte(base))
	return hex.EncodeToString(sum[:])
}

// DiscussionEntryLegacySignature is the signature packets saved before
// 2026-11 used: text, deletion state and times, without file identity.
func DiscussionEntryLegacySignature(e DiscussionEntry) string {
	sum := sha256.Sum256([]byte(discussionEntryBase(e)))
	return hex.EncodeToString(sum[:])
}

func discussionEntryBase(e DiscussionEntry) string {
	return fmt.Sprintf("%s\x00%s\x00%t\x00%s", e.UpdatedAt, e.HTML, e.Deleted, e.CreatedAt)
}

// DiscussionEntryFiles lists the sorted identities of the files attached to
// the entry itself (not those embedded in its text): the Canvas file ID, and
// with versions also Canvas's updated_at.
func DiscussionEntryFiles(e DiscussionEntry, versions bool) []string {
	var files []string
	for _, a := range e.Attachments {
		if a.Source != "" || a.ID <= 0 {
			continue
		}
		id := strconv.FormatInt(a.ID, 10)
		if versions {
			id += "@" + a.UpdatedAt
		}
		files = append(files, id)
	}
	sort.Strings(files)
	return files
}

// DiscussionEvidenceDigest is a deterministic digest of a set of entries.
func DiscussionEvidenceDigest(entries []DiscussionEntry) string {
	return discussionDigest(entries, DiscussionEntrySignature)
}

// DiscussionEvidenceLegacyDigest is the digest packets saved before 2026-11
// recorded, from legacy signatures.
func DiscussionEvidenceLegacyDigest(entries []DiscussionEntry) string {
	return discussionDigest(entries, DiscussionEntryLegacySignature)
}

func discussionDigest(entries []DiscussionEntry, sign func(DiscussionEntry) string) string {
	lines := make([]string, 0, len(entries))
	for _, e := range entries {
		lines = append(lines, DiscussionEntryRef(e)+"="+sign(e))
	}
	sort.Strings(lines)
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(sum[:])
}

// HasConsentTag reports whether an assignment name carries (CE), the mark a
// teammate adds to opt that assignment in to CE-SE scoring.
func HasConsentTag(name string) bool { return strings.Contains(strings.ToLower(name), "(ce)") }
