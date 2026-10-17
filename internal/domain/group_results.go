package domain

import (
	"errors"
	"fmt"
	"strings"
)

// GroupResultsDocument carries one shared assessment per club product. All
// member identities and attempts come from the referenced group packet; nothing
// here authors a raw Canvas user ID.
type GroupResultsDocument struct {
	Schema         int                `yaml:"schema"`
	Kind           string             `yaml:"kind"`
	Packet         string             `yaml:"packet"`
	Assignment     AssignmentSnapshot `yaml:"assignment"`
	Interpretation Interpretation     `yaml:"interpretation"`
	ScoringBasis   *Rubric            `yaml:"scoring_basis,omitempty"`
	// Variants are established accommodation interpretations for individual rows.
	Variants []InterpretationVariant `yaml:"variants,omitempty"`
	Groups   []GroupResult           `yaml:"groups"`
}

// Basis returns the live Canvas rubric when present, else the agent's scoring basis.
func (d GroupResultsDocument) Basis() *Rubric {
	if d.Assignment.Rubric != nil {
		return d.Assignment.Rubric
	}
	return d.ScoringBasis
}

// GroupResult is one group's operation. Representative is Engine-authored: the
// member whose submission route carried the propagated grade/rubric.
type GroupResult struct {
	GroupID            int64                `yaml:"group_id"`
	ArtifactKey        string               `yaml:"artifact_key"`
	MemberRefs         []string             `yaml:"member_refs"`
	Operation          string               `yaml:"operation"` // grade, comment-only, or individual
	Representative     string               `yaml:"representative,omitempty"`
	Grade              *GroupGradeOperation `yaml:"grade,omitempty"`
	CommentOnly        *GroupCommentOnly    `yaml:"comment_only,omitempty"`
	IndividualFeedback []IndividualFeedback `yaml:"individual_feedback,omitempty"`
	// Students and SharedFeedback belong to operation: individual (live
	// grade_group_students_individually: true). Each student keeps an ordinary
	// grade/comment-only operation; SharedFeedback is a separate whole-group comment.
	Students       []StudentResult   `yaml:"students,omitempty"`
	SharedFeedback *GroupCommentOnly `yaml:"shared_feedback,omitempty"`
}

// GroupGradeOperation has no single member attempt; attempts come from the packet.
type GroupGradeOperation struct {
	Components       []ComponentScore `yaml:"components"`
	Assessment       Decimal          `yaml:"assessment"`
	FeedbackAudience string           `yaml:"feedback_audience"` // group or none
	Feedback         string           `yaml:"feedback,omitempty"`
	Header           string           `yaml:"header,omitempty"`
	TeacherNote      string           `yaml:"teacher_note,omitempty"`
	StrongestPrior   *PriorComparison `yaml:"strongest_prior,omitempty"`
	LateTiming       *LateTiming      `yaml:"late_timing,omitempty"`
	Outcomes         ActionOutcomes   `yaml:"outcomes,omitempty"`
}

// GroupCommentOnly posts a whole-group comment with criterion lines, no grade.
type GroupCommentOnly struct {
	Components []ComponentScore `yaml:"components"`
	Feedback   string           `yaml:"feedback"`
	Header     string           `yaml:"header,omitempty"`
	Outcome    ActionOutcome    `yaml:"outcome,omitempty"`
}

// IndividualFeedback is private student-facing text for one member.
type IndividualFeedback struct {
	StudentRef string        `yaml:"student_ref"`
	Text       string        `yaml:"text"`
	Outcome    ActionOutcome `yaml:"outcome,omitempty"`
}

// ValidatedGroup is a row's validated arithmetic. Hold means this group cannot
// be scored honestly; other groups are unaffected.
type ValidatedGroup struct {
	Hold       string
	Assessment *Assessment
	// Students are the validated individual rows (operation: individual).
	Students []ValidatedStudent
	// SharedFeedback is the arithmetic behind an individual row's optional group
	// comment; SharedHold says why that comment alone cannot be posted.
	SharedFeedback *Assessment
	SharedHold     string
}

const studentNamePlaceholder = "{{student_name}}"

func sharedText(s string) error {
	if strings.Contains(s, studentNamePlaceholder) {
		return fmt.Errorf("shared text goes to the whole group and cannot use %s", studentNamePlaceholder)
	}
	return nil
}

func sharedComponentText(components []ComponentScore) error {
	for _, c := range components {
		for _, sc := range c.Criteria {
			if err := sharedText(sc.Comment); err != nil {
				return err
			}
		}
	}
	return nil
}

func (d *GroupResultsDocument) Validate() error { _, e := ValidateGroupResults(d); return e }

func ValidateGroupResults(doc *GroupResultsDocument) ([]ValidatedGroup, error) {
	if doc.Schema != 1 || doc.Kind != "group-results" {
		return nil, fmt.Errorf("expected schema: 1 and kind: group-results")
	}
	if strings.TrimSpace(doc.Packet) == "" {
		return nil, fmt.Errorf("packet path required")
	}
	if e := ValidateSnapshot(doc.Assignment); e != nil {
		return nil, e
	}
	a := doc.Assignment
	if a.GroupCategoryID <= 0 || a.NewQuiz || a.AnonymousGrading || a.ModeratedGrading || !numericGradeType(a.GradingType) {
		return nil, fmt.Errorf("group-results requires a supported (non-anonymous, non-moderated, points-grade) group assignment")
	}
	if a.GradeGroupStudentsIndividually == nil {
		return nil, fmt.Errorf("group grading mode (grade_group_students_individually) is unknown; pull the assignment again")
	}
	individualMode := *a.GradeGroupStudentsIndividually
	if len(doc.Groups) == 0 {
		return nil, fmt.Errorf("group-results requires groups")
	}
	if a.Rubric != nil && doc.ScoringBasis != nil {
		return nil, fmt.Errorf("scoring_basis is only for assignments without a Canvas rubric; this assignment has one")
	}
	basis := doc.Basis()
	if basis == nil {
		return nil, fmt.Errorf("assignment has no Canvas rubric: add scoring_basis (your own criteria with ids, descriptions and points) to score it")
	}
	seen := map[int64]bool{}
	validated := make([]ValidatedGroup, 0, len(doc.Groups))
	for i := range doc.Groups {
		g := &doc.Groups[i]
		fail := func(e error) ([]ValidatedGroup, error) {
			return nil, fmt.Errorf("groups[%d] (group %d): %w", i, g.GroupID, e)
		}
		if g.GroupID <= 0 || seen[g.GroupID] {
			return fail(fmt.Errorf("group_id must be a positive Canvas group ID, once per document"))
		}
		seen[g.GroupID] = true
		if strings.TrimSpace(g.ArtifactKey) == "" {
			return fail(fmt.Errorf("artifact_key required"))
		}
		if len(g.MemberRefs) == 0 {
			return fail(fmt.Errorf("member_refs must list every affected member"))
		}
		members := map[string]bool{}
		for _, ref := range g.MemberRefs {
			if strings.TrimSpace(ref) == "" || members[ref] {
				return fail(fmt.Errorf("member_refs must be nonempty and unique"))
			}
			members[ref] = true
		}
		if g.Representative != "" && !members[g.Representative] {
			return fail(fmt.Errorf("representative is not one of member_refs"))
		}
		feedbackRefs := map[string]bool{}
		for _, f := range g.IndividualFeedback {
			if !members[f.StudentRef] || feedbackRefs[f.StudentRef] {
				return fail(fmt.Errorf("individual_feedback needs one row per listed member"))
			}
			feedbackRefs[f.StudentRef] = true
			if strings.TrimSpace(f.Text) == "" {
				return fail(fmt.Errorf("individual_feedback text required"))
			}
			if e := validateOutcome(f.Outcome); e != nil {
				return fail(e)
			}
		}
		v := ValidatedGroup{}
		if (g.Operation == "individual") != individualMode {
			return fail(fmt.Errorf("operation %q does not match the assignment's group grading mode: use individual only when grade_group_students_individually is true, grade or comment-only when it is false", g.Operation))
		}
		if g.Operation != "individual" && (len(g.Students) > 0 || g.SharedFeedback != nil) {
			return fail(fmt.Errorf("students and shared_feedback belong only to operation: individual"))
		}
		switch g.Operation {
		case "individual":
			if g.Grade != nil || g.CommentOnly != nil || len(g.IndividualFeedback) > 0 || len(g.Students) == 0 {
				return fail(fmt.Errorf("individual operation requires students only; each student's comment is their own feedback"))
			}
			tmp := &ResultsDocument{Schema: 1, Kind: "results", Packet: doc.Packet, Assignment: a, Interpretation: doc.Interpretation, ScoringBasis: doc.ScoringBasis, Variants: doc.Variants, Students: g.Students}
			// Ordinary per-student validation applies to the student's own operation.
			tmp.Assignment.GroupCategoryID, tmp.Assignment.GradeGroupStudentsIndividually = 0, nil
			rows, e := ValidateResults(tmp)
			if e != nil {
				return fail(e)
			}
			for _, r := range g.Students {
				if !members[r.StudentRef] || r.Operation == "revise-feedback" {
					return fail(fmt.Errorf("student %s must be one of member_refs and use grade or comment-only", r.StudentRef))
				}
			}
			v.Students = rows
			if sf := g.SharedFeedback; sf != nil {
				if !validHeader(sf.Header) || strings.TrimSpace(sf.Feedback) == "" {
					return fail(fmt.Errorf("shared_feedback needs feedback and a valid header"))
				}
				if e := sharedText(sf.Feedback); e != nil {
					return fail(e)
				}
				if e := sharedComponentText(sf.Components); e != nil {
					return fail(e)
				}
				if e := validateOutcome(sf.Outcome); e != nil {
					return fail(e)
				}
				a2, e := ComputeAssessment(*basis, doc.Interpretation, sf.Components, a.Points)
				var unknown *UnknownEvidenceError
				if errors.As(e, &unknown) {
					v.SharedHold = "unreadable work for component " + unknown.Component + "; shared feedback not scored"
				} else if e != nil {
					return fail(e)
				} else {
					v.SharedFeedback = &a2
				}
			}
		case "grade":
			if g.Grade == nil || g.CommentOnly != nil {
				return fail(fmt.Errorf("grade operation requires grade only"))
			}
			gr := g.Grade
			if e := validateLateTiming(gr.LateTiming); e != nil {
				return fail(e)
			}
			if gr.FeedbackAudience != "group" && gr.FeedbackAudience != "none" {
				return fail(fmt.Errorf("feedback_audience must be group or none"))
			}
			if gr.FeedbackAudience == "none" && strings.TrimSpace(gr.Feedback) != "" {
				return fail(fmt.Errorf("feedback_audience none carries no shared feedback; use individual_feedback for private text"))
			}
			if gr.FeedbackAudience == "group" && strings.TrimSpace(gr.Feedback) == "" {
				return fail(fmt.Errorf("conversational shared feedback required"))
			}
			if e := sharedText(gr.Feedback); e != nil {
				return fail(e)
			}
			if e := sharedComponentText(gr.Components); e != nil {
				return fail(e)
			}
			if !validHeader(gr.Header) {
				return fail(fmt.Errorf("header must be score, full or none"))
			}
			if p := gr.StrongestPrior; p != nil {
				if e := validateAttempt(p.Attempt); e != nil {
					return fail(e)
				}
				if strings.TrimSpace(p.Basis) == "" {
					return fail(fmt.Errorf("strongest_prior needs an academic basis"))
				}
			}
			for _, o := range []ActionOutcome{gr.Outcomes.Comment, gr.Outcomes.Rubric, gr.Outcomes.Grade, gr.Outcomes.Late} {
				if e := validateOutcome(o); e != nil {
					return fail(e)
				}
			}
			a2, e := ComputeAssessment(*basis, doc.Interpretation, gr.Components, a.Points)
			var unknown *UnknownEvidenceError
			if errors.As(e, &unknown) {
				v.Hold = "unreadable work for component " + unknown.Component + "; not scored"
				validated = append(validated, v)
				continue
			}
			if e != nil {
				return fail(e)
			}
			if !gr.Assessment.IsSet() {
				gr.Assessment = a2.Posted
			} else if gr.Assessment.Cmp(a2.Posted) != 0 {
				return fail(fmt.Errorf("assessment must equal derived posted value %s", a2.Posted.String()))
			}
			v.Assessment = &a2
		case "comment-only":
			if g.CommentOnly == nil || g.Grade != nil {
				return fail(fmt.Errorf("comment-only operation requires comment_only only; grade, rubric, and late writes forbidden"))
			}
			c := g.CommentOnly
			if !validHeader(c.Header) {
				return fail(fmt.Errorf("header must be score, full or none"))
			}
			if strings.TrimSpace(c.Feedback) == "" {
				return fail(fmt.Errorf("conversational shared feedback required"))
			}
			if e := sharedText(c.Feedback); e != nil {
				return fail(e)
			}
			if e := sharedComponentText(c.Components); e != nil {
				return fail(e)
			}
			if e := validateOutcome(c.Outcome); e != nil {
				return fail(e)
			}
			a2, e := ComputeAssessment(*basis, doc.Interpretation, c.Components, a.Points)
			var unknown *UnknownEvidenceError
			if errors.As(e, &unknown) {
				v.Hold = "unreadable work for component " + unknown.Component + "; not scored"
				validated = append(validated, v)
				continue
			}
			if e != nil {
				return fail(e)
			}
			v.Assessment = &a2
		default:
			return fail(fmt.Errorf("unsupported operation %q", g.Operation))
		}
		validated = append(validated, v)
	}
	return validated, nil
}
