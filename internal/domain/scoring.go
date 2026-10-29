package domain

import (
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"strings"
	"time"
)

type AggregateCriterion struct {
	CriterionID string
	Points      *big.Rat // exact until the Canvas/display boundary
	RatingID    string
	Comment     string
}
type Assessment struct {
	Posted      Decimal
	Unrounded   *big.Rat
	RubricTotal *big.Rat
	Criteria    []AggregateCriterion
}
type ValidatedStudent struct {
	// Hold, when set, means this student cannot be scored honestly; the rest of
	// the file is unaffected.
	Hold           string
	Student        StudentResult
	Assessment     *Assessment
	Interpretation Interpretation
	Assignment     AssignmentSnapshot
}

func validInstance(s string) bool {
	u, e := url.Parse(s)
	return e == nil && u.Scheme == "https" && u.Hostname() != "" && u.Opaque == "" && u.User == nil && (u.Path == "" || u.Path == "/") && u.RawQuery == "" && !u.ForceQuery && u.Fragment == ""
}
func validateAttempt(a AttemptRef) error {
	if a.Attempt < 1 {
		return fmt.Errorf("attempt must be positive")
	}
	return nil
}
func ValidateRubric(r Rubric) error {
	if r.Library != "" {
		return fmt.Errorf("scoring requires a complete rubric definition, not a library reference")
	}
	if r.ID < 0 || r.AssociationID < 0 {
		return fmt.Errorf("rubric/association ids cannot be negative")
	}
	if len(r.Criteria) == 0 {
		return fmt.Errorf("rubric has no criteria")
	}
	ids := map[string]bool{}
	for i, c := range r.Criteria {
		if strings.TrimSpace(c.ID) == "" || ids[c.ID] {
			return fmt.Errorf("rubric criteria[%d]: empty or duplicate criterion id", i)
		}
		ids[c.ID] = true
		if !c.Points.IsSet() || c.Points.Sign() < 0 || strings.TrimSpace(c.Description) == "" {
			return fmt.Errorf("criterion %s: description and nonnegative points required", c.ID)
		}
		ratings := map[string]bool{}
		for _, rating := range c.Ratings {
			if rating.ID == "" || ratings[rating.ID] || !rating.Points.IsSet() || rating.Points.Sign() < 0 || rating.Points.Cmp(c.Points) > 0 {
				return fmt.Errorf("criterion %s: invalid/duplicate rating or points", c.ID)
			}
			ratings[rating.ID] = true
		}
	}
	return nil
}
func ValidateInterpretation(r Rubric, in Interpretation) error {
	if err := ValidateRubric(r); err != nil {
		return err
	}
	if in.Mapping != "repeated" && in.Mapping != "direct" {
		return fmt.Errorf("interpretation.mapping must be repeated or direct (results need an interpretation: block with mapping, rationale and components; see help examples)")
	}
	if strings.TrimSpace(in.Rationale) == "" || len(in.Components) == 0 {
		return fmt.Errorf("interpretation requires rationale and components")
	}
	criteria := map[string]bool{}
	for _, c := range r.Criteria {
		criteria[c.ID] = true
	}
	keys, mapped := map[string]bool{}, map[string]bool{}
	totalWeight := new(big.Rat)
	for i, c := range in.Components {
		if c.Key == "" || keys[c.Key] || strings.TrimSpace(c.Label) == "" {
			return fmt.Errorf("components[%d]: empty/duplicate key or missing label", i)
		}
		keys[c.Key] = true
		if !c.Weight.IsSet() || c.Weight.Sign() < 0 {
			return fmt.Errorf("component %s: weight must be nonnegative", c.Key)
		}
		totalWeight.Add(totalWeight, c.Weight.Rat())
		if in.Mapping == "repeated" && len(c.CriterionIDs) != 0 {
			return fmt.Errorf("repeated mapping cannot select criterion_ids")
		}
		if in.Mapping == "direct" {
			if c.Weight.Cmp(MustDecimal("1")) != 0 {
				return fmt.Errorf("direct mapping uses original criterion maxima; weight must be 1")
			}
			if len(c.CriterionIDs) == 0 {
				return fmt.Errorf("direct component %s requires criterion_ids", c.Key)
			}
			for _, id := range c.CriterionIDs {
				if !criteria[id] || mapped[id] {
					return fmt.Errorf("unknown or multiply mapped criterion %q", id)
				}
				mapped[id] = true
			}
		}
	}
	if totalWeight.Sign() <= 0 {
		return fmt.Errorf("component weights must have a positive total")
	}
	if in.Mapping == "direct" && len(mapped) != len(criteria) {
		return fmt.Errorf("direct mapping must cover every rubric criterion exactly once")
	}
	return nil
}

// UnknownEvidenceError marks work the agent could not read.
type UnknownEvidenceError struct{ Component string }

func (e *UnknownEvidenceError) Error() string {
	return fmt.Sprintf("component %s has unknown evidence; hold for readable evidence", e.Component)
}

func ComputeAssessment(r Rubric, in Interpretation, scores []ComponentScore, points ...Decimal) (Assessment, error) {
	if e := ValidateInterpretation(r, in); e != nil {
		return Assessment{}, e
	}
	// Optional maximum keeps this API useful for rubric-only checks; results
	// validation always supplies the live assignment's maximum explicitly.
	rubricMax := new(big.Rat)
	rubricByID := map[string]Criterion{}
	for _, c := range r.Criteria {
		rubricMax.Add(rubricMax, c.Points.Rat())
		rubricByID[c.ID] = c
	}
	if rubricMax.Sign() <= 0 {
		return Assessment{}, fmt.Errorf("unscored/zero-point rubric has no numeric assessment basis")
	}
	if len(points) > 1 {
		return Assessment{}, fmt.Errorf("at most one assignment maximum may be supplied")
	}
	p := new(big.Rat).Set(rubricMax)
	if len(points) != 0 {
		if !points[0].IsSet() {
			return Assessment{}, fmt.Errorf("assignment points required")
		}
		p = points[0].Rat()
	}
	if p.Sign() <= 0 {
		return Assessment{}, fmt.Errorf("assignment points must be positive for numeric scoring")
	}
	if len(scores) != len(in.Components) {
		return Assessment{}, fmt.Errorf("all required components must be supplied, including missing work")
	}
	byKey := map[string]ComponentScore{}
	for _, s := range scores {
		if s.Key == "" {
			return Assessment{}, fmt.Errorf("missing component key")
		}
		if _, ok := byKey[s.Key]; ok {
			return Assessment{}, fmt.Errorf("duplicate component %q", s.Key)
		}
		byKey[s.Key] = s
	}
	totalWeight := new(big.Rat)
	for _, c := range in.Components {
		totalWeight.Add(totalWeight, c.Weight.Rat())
	}
	aggregate := map[string]*big.Rat{}
	comments := map[string][]string{}
	for _, c := range r.Criteria {
		aggregate[c.ID] = new(big.Rat)
	}
	for _, component := range in.Components {
		s, ok := byKey[component.Key]
		if !ok {
			return Assessment{}, fmt.Errorf("missing required component %q", component.Key)
		}
		if s.Evidence == "unknown" {
			return Assessment{}, &UnknownEvidenceError{Component: s.Key}
		}
		if s.Evidence != "present" && s.Evidence != "missing" {
			return Assessment{}, fmt.Errorf("component %s: evidence must be present, missing, or unknown", s.Key)
		}
		expected := map[string]bool{}
		if in.Mapping == "repeated" {
			for id := range rubricByID {
				expected[id] = true
			}
		} else {
			for _, id := range component.CriterionIDs {
				expected[id] = true
			}
		}
		if len(s.Criteria) != len(expected) {
			return Assessment{}, fmt.Errorf("component %s: score every mapped criterion", s.Key)
		}
		seen := map[string]bool{}
		for _, sc := range s.Criteria {
			c, ok := rubricByID[sc.CriterionID]
			if !ok || !expected[sc.CriterionID] || seen[sc.CriterionID] {
				return Assessment{}, fmt.Errorf("component %s: unknown, unmapped, or duplicate criterion %q", s.Key, sc.CriterionID)
			}
			seen[sc.CriterionID] = true
			if !sc.Points.IsSet() || sc.Points.Sign() < 0 || sc.Points.Cmp(c.Points) > 0 {
				return Assessment{}, fmt.Errorf("component %s criterion %s: points outside 0..%s or missing", s.Key, c.ID, c.Points.String())
			}
			if s.Evidence == "missing" && sc.Points.Sign() != 0 {
				return Assessment{}, fmt.Errorf("missing component %s must explicitly score zero", s.Key)
			}
			if sc.RatingID != "" {
				matched := false
				for _, rt := range c.Ratings {
					if rt.ID == sc.RatingID && rt.Points.Cmp(sc.Points) == 0 {
						matched = true
					}
				}
				if !matched {
					return Assessment{}, fmt.Errorf("criterion %s rating %s does not truthfully match points", c.ID, sc.RatingID)
				}
			}
			value := sc.Points.Rat()
			if in.Mapping == "repeated" {
				value.Mul(value, new(big.Rat).Quo(component.Weight.Rat(), totalWeight))
			}
			aggregate[c.ID].Add(aggregate[c.ID], value)
			if strings.TrimSpace(sc.Comment) != "" {
				comment := sc.Comment
				if in.Mapping == "repeated" {
					comment = component.Label + ": " + comment
				}
				comments[c.ID] = append(comments[c.ID], comment)
			}
		}
	}
	a := Assessment{RubricTotal: rubricMax}
	total := new(big.Rat)
	for _, c := range r.Criteria {
		q := aggregate[c.ID]
		total.Add(total, q)
		ac := AggregateCriterion{CriterionID: c.ID, Points: q, Comment: strings.Join(comments[c.ID], "\n")}
		// An exact rating is optional; averages never receive invented IDs.
		for _, rt := range c.Ratings {
			if rt.Points.Rat().Cmp(q) == 0 {
				ac.RatingID = rt.ID
				break
			}
		}
		a.Criteria = append(a.Criteria, ac)
	}
	a.Unrounded = new(big.Rat).Mul(p, new(big.Rat).Quo(total, rubricMax))
	a.Posted = RoundRat(a.Unrounded, 2)
	return a, nil
}
func validateOutcome(o ActionOutcome) error {
	switch o.Status {
	case "", "pending", "succeeded", "preserved", "not-requested", "held", "failed", "uncertain":
	default:
		return fmt.Errorf("unknown outcome status %q", o.Status)
	}
	if o.ID < 0 {
		return fmt.Errorf("outcome id cannot be negative")
	}
	return nil
}

// numericGradeType reports whether Canvas takes the Engine's computed points as
// the grade. Letter and GPA-scale assignments map a points value onto the
// course's own scale. Pass/fail and not-graded have no points to post.
func numericGradeType(t string) bool {
	switch t {
	case "points", "percent", "letter_grade", "gpa_scale":
		return true
	}
	return false
}

func validateTimingSnapshot(s AssignmentSnapshot) error {
	// Timing-only actions do not interpret points, title or rubric. Keep all
	// identity, date and submission-shape checks while clearing those grading
	// requirements for validation only.
	s.Points = MustDecimal("0")
	s.Title = "timing-only"
	s.Rubric = nil
	return ValidateSnapshot(s)
}

func validateNumericTarget(s AssignmentSnapshot) error {
	if !numericGradeType(s.GradingType) {
		return fmt.Errorf("the Engine posts points grades; this assignment uses %q grading, so pull the work and enter the result in SpeedGrader", s.GradingType)
	}
	if s.AnonymousGrading || s.ModeratedGrading || s.GroupCategoryID != 0 {
		return fmt.Errorf("anonymous, moderated, or group scoring is unsupported")
	}
	return nil
}
func ValidateResults(doc *ResultsDocument) ([]ValidatedStudent, error) {
	if doc.Schema != 1 || doc.Kind != "results" {
		return nil, fmt.Errorf("expected schema: 1 and kind: results")
	}
	if doc.Packet == "" {
		return nil, fmt.Errorf("packet path required")
	}
	timingOnly := len(doc.Students) > 0
	for _, student := range doc.Students {
		if student.Operation != "late-only" {
			timingOnly = false
			break
		}
	}
	var snapshotErr error
	if timingOnly {
		snapshotErr = validateTimingSnapshot(doc.Assignment)
	} else {
		snapshotErr = ValidateSnapshot(doc.Assignment)
	}
	if snapshotErr != nil {
		return nil, snapshotErr
	}
	if len(doc.Students) == 0 {
		return nil, fmt.Errorf("results requires students")
	}
	if !timingOnly && doc.Assignment.Rubric != nil && doc.ScoringBasis != nil {
		return nil, fmt.Errorf("scoring_basis is only for assignments without a Canvas rubric; this assignment has one")
	}
	basis := doc.Basis()
	hasGrade := false
	for _, s := range doc.Students {
		if s.Operation == "grade" {
			hasGrade = true
		}
	}
	if doc.Assignment.NewQuiz && hasGrade {
		return nil, fmt.Errorf("New Quiz results are comment-only; grade writes are not supported")
	}
	if basis == nil && hasGrade {
		return nil, fmt.Errorf("assignment has no Canvas rubric: add scoring_basis (your own criteria with ids, descriptions and points) to score it")
	}
	variants := map[string]Interpretation{}
	for i, variant := range doc.Variants {
		if timingOnly {
			break
		}
		if variant.Key == "" || strings.TrimSpace(variant.Basis) == "" {
			return nil, fmt.Errorf("variants[%d]: key and established accommodation basis required", i)
		}
		if _, ok := variants[variant.Key]; ok {
			return nil, fmt.Errorf("duplicate variant %q", variant.Key)
		}
		if basis == nil {
			return nil, fmt.Errorf("variant requires a rubric or scoring_basis")
		}
		if e := ValidateInterpretation(*basis, variant.Interpretation); e != nil {
			return nil, fmt.Errorf("variant %s: %w", variant.Key, e)
		}
		variants[variant.Key] = variant.Interpretation
	}
	refs := map[string]bool{}
	validated := make([]ValidatedStudent, 0, len(doc.Students))
	for i, s := range doc.Students {
		fail := func(e error) ([]ValidatedStudent, error) {
			return nil, fmt.Errorf("students[%d] (%s): %w", i, s.StudentRef, e)
		}
		if s.StudentRef == "" || refs[s.StudentRef] {
			return fail(fmt.Errorf("missing or duplicate student_ref"))
		}
		refs[s.StudentRef] = true
		v := ValidatedStudent{Student: s, Interpretation: doc.Interpretation, Assignment: doc.Assignment}
		if s.Variant != "" {
			var ok bool
			v.Interpretation, ok = variants[s.Variant]
			if !ok {
				return fail(fmt.Errorf("unknown interpretation variant %q", s.Variant))
			}
		}
		switch s.Operation {
		case "grade":
			if e := validateNumericTarget(doc.Assignment); e != nil {
				return fail(e)
			}
			if s.Grade == nil || s.ReviseFeedback != nil || s.LateOnly != nil {
				return fail(fmt.Errorf("grade operation requires grade only"))
			}
			g := s.Grade
			if e := validateAttempt(g.Attempt); e != nil {
				return fail(e)
			}
			if e := validateEvidence(doc.Assignment, g.Evidence); e != nil {
				return fail(e)
			}
			if e := validateLateTiming(g.LateTiming); e != nil {
				return fail(e)
			}
			if g.LateTiming != nil && g.LateTiming.SchoolTime != nil && g.LateTiming.Observed == nil {
				return fail(fmt.Errorf("late_timing.school_time requires the pulled observed timing baseline"))
			}
			a, e := ComputeAssessment(*basis, v.Interpretation, g.Components, doc.Assignment.Points)
			var unknown *UnknownEvidenceError
			if errors.As(e, &unknown) {
				v.Hold = "unreadable work for component " + unknown.Component + "; not scored"
				validated = append(validated, v)
				continue
			}
			if e != nil {
				return fail(e)
			}
			if !g.Assessment.IsSet() {
				g.Assessment = a.Posted
			} else if g.Assessment.Cmp(a.Posted) != 0 {
				return fail(fmt.Errorf("assessment must equal derived posted value %s", a.Posted.String()))
			}
			if !validHeader(g.Header) {
				return fail(fmt.Errorf("header must be score, full or none"))
			}
			if strings.TrimSpace(g.Feedback) == "" && !(ItemFeedback(g.Components) && fullHeader(g.Header)) {
				return fail(fmt.Errorf("conversational feedback required"))
			}
			if prior := g.StrongestPrior; prior != nil {
				if e := validateAttempt(prior.Attempt); e != nil {
					return fail(e)
				}
				if prior.Attempt.Attempt >= g.Attempt.Attempt || strings.TrimSpace(prior.Basis) == "" {
					return fail(fmt.Errorf("strongest_prior must precede current attempt and give academic basis"))
				}
			}
			for _, o := range []ActionOutcome{g.Outcomes.Comment, g.Outcomes.Rubric, g.Outcomes.Grade, g.Outcomes.Late} {
				if e := validateOutcome(o); e != nil {
					return fail(e)
				}
			}
			v.Assessment = &a
		case "comment-only":
			if s.Grade != nil || s.ReviseFeedback != nil || s.LateOnly != nil || s.CommentOnly == nil {
				return fail(fmt.Errorf("comment-only operation requires comment_only only; grade, rubric, and late writes forbidden"))
			}
			if !numericGradeType(doc.Assignment.GradingType) || doc.Assignment.AnonymousGrading || doc.Assignment.ModeratedGrading || doc.Assignment.GroupCategoryID != 0 {
				return fail(fmt.Errorf("comment-only requires a supported individual assignment or New Quiz"))
			}
			c := s.CommentOnly
			if e := validateAttempt(c.Attempt); e != nil {
				return fail(e)
			}
			if e := validateEvidence(doc.Assignment, c.Evidence); e != nil {
				return fail(e)
			}
			if !c.FeedbackOnly() {
				if basis == nil {
					return fail(fmt.Errorf("comment-only scores require a Canvas rubric or scoring_basis; for feedback without scores use header: none and omit components"))
				}
				if _, e := ComputeAssessment(*basis, v.Interpretation, c.Components, doc.Assignment.Points); e != nil {
					var unknown *UnknownEvidenceError
					if errors.As(e, &unknown) {
						v.Hold = "unreadable work for component " + unknown.Component + "; not scored"
						validated = append(validated, v)
						continue
					}
					return fail(e)
				}
			}
			if c.ItemScores {
				if !doc.Assignment.NewQuiz || c.FeedbackOnly() {
					return fail(fmt.Errorf("item_scores needs a New Quiz and scored components"))
				}
				for _, ic := range v.Interpretation.Components {
					if ic.ItemID == "" {
						return fail(fmt.Errorf("item_scores needs an item_id on every part; part %q has none", ic.Key))
					}
				}
				if e := validateOutcome(c.ItemScoresOutcome); e != nil {
					return fail(e)
				}
			}
			if !validHeader(c.Header) {
				return fail(fmt.Errorf("header must be score, full or none"))
			}
			if strings.TrimSpace(c.Feedback) == "" && !(ItemFeedback(c.Components) && fullHeader(c.Header)) {
				return fail(fmt.Errorf("conversational feedback required"))
			}
			if e := validateOutcome(c.Outcome); e != nil {
				return fail(e)
			}
		case "revise-feedback":
			if s.Grade != nil || s.CommentOnly != nil || s.ReviseFeedback == nil || s.LateOnly != nil {
				return fail(fmt.Errorf("revise-feedback requires revise_feedback only; grade/late writes forbidden"))
			}
			r := s.ReviseFeedback
			if e := validateAttempt(r.Attempt); e != nil {
				return fail(e)
			}
			if !validHeader(r.Header) {
				return fail(fmt.Errorf("header must be score, full or none"))
			}
			if r.CommentID <= 0 || strings.TrimSpace(r.Feedback) == "" {
				return fail(fmt.Errorf("revision requires exact comment_id and feedback"))
			}
			if e := validateOutcome(r.Outcome); e != nil {
				return fail(e)
			}
			if r.Outcome.ID > 0 && r.Outcome.ID != r.CommentID {
				return fail(fmt.Errorf("revision outcome id disagrees with comment_id"))
			}
			if r.Assessment != nil {
				if basis == nil {
					return fail(fmt.Errorf("preserved score display needs a rubric or scoring_basis"))
				}
				a, e := ComputeAssessment(*basis, v.Interpretation, r.Assessment.Components, doc.Assignment.Points)
				if e != nil {
					return fail(e)
				}
				if !r.Assessment.Assessment.IsSet() {
					r.Assessment.Assessment = a.Posted
				} else if r.Assessment.Assessment.Cmp(a.Posted) != 0 {
					return fail(fmt.Errorf("preserved assessment disagrees with component scores"))
				}

				v.Assessment = &a
			}
		case "late-only":
			if s.LateOnly == nil || s.Grade != nil || s.CommentOnly != nil || s.ReviseFeedback != nil {
				return fail(fmt.Errorf("late-only operation requires late_only only"))
			}
			if doc.Assignment.NewQuiz || doc.Assignment.GroupCategoryID != 0 || doc.Assignment.AnonymousGrading || doc.Assignment.ModeratedGrading {
				return fail(fmt.Errorf("late-only requires a supported ordinary individual assignment"))
			}
			if e := validateAttempt(s.LateOnly.Attempt); e != nil {
				return fail(e)
			}
			if e := validateLateTiming(&s.LateOnly.LateTiming); e != nil {
				return fail(e)
			}
			if !observedTimingKnown(s.LateOnly.LateTiming.Observed) {
				return fail(fmt.Errorf("late-only requires the observed Canvas timing baseline from the pulled attempt"))
			}
			if e := validateOutcome(s.LateOnly.Outcome); e != nil {
				return fail(e)
			}
		default:
			return fail(fmt.Errorf("unsupported operation %q", s.Operation))
		}
		validated = append(validated, v)
	}
	return validated, nil
}

// validHeader accepts the teacher's choices for what leads a comment.
// fullHeader reports whether the header shows each part's lines, where per-part
// feedback appears.
func fullHeader(h string) bool { return h == "" || h == "full" }

func validHeader(h string) bool {
	return h == "" || h == "default" || h == "score" || h == "full" || h == "none"
}
func validateLateTiming(t *LateTiming) error {
	if t == nil {
		return nil
	}
	switch t.Status {
	case "late":
		if t.SecondsLate <= 0 {
			return fmt.Errorf("late_timing: seconds_late must be positive when status is late")
		}
	case "none":
		if t.SecondsLate != 0 {
			return fmt.Errorf("late_timing: seconds_late must be 0 or absent when status is none")
		}
	default:
		return fmt.Errorf("late_timing: status must be late or none")
	}
	if _, e := time.Parse(time.RFC3339Nano, t.DueAt); e != nil {
		return fmt.Errorf("late_timing: due_at must be RFC3339: %w", e)
	}
	if strings.TrimSpace(t.Basis) == "" {
		return fmt.Errorf("late_timing: basis is required")
	}
	if t.Observed != nil && !observedTimingKnown(t.Observed) {
		return fmt.Errorf("late_timing.observed: status and seconds_late must both be known")
	}
	if p := t.SchoolTime; p != nil {
		if e := validateAttempt(p.Attempt); e != nil {
			return fmt.Errorf("late_timing.school_time.attempt must identify a valid first submission")
		}
		if _, e := time.Parse(time.RFC3339Nano, p.SubmittedAt); e != nil {
			return fmt.Errorf("late_timing.school_time.submitted_at must be RFC3339: %w", e)
		}
		attemptSubmitted, e := time.Parse(time.RFC3339Nano, p.Attempt.SubmittedAt)
		if e != nil || p.Attempt.SubmittedAt == "" {
			return fmt.Errorf("late_timing.school_time.attempt.submitted_at must be RFC3339")
		}
		provenanceSubmitted, _ := time.Parse(time.RFC3339Nano, p.SubmittedAt)
		if !attemptSubmitted.Equal(provenanceSubmitted) {
			return fmt.Errorf("late_timing.school_time submitted_at disagrees with its attempt reference")
		}
		if _, e := time.Parse(time.RFC3339Nano, p.Deadline); e != nil {
			return fmt.Errorf("late_timing.school_time.deadline must be RFC3339: %w", e)
		}
		if p.Status != t.Status || p.SecondsLate != t.SecondsLate {
			return fmt.Errorf("late_timing.school_time result must match late_timing status and seconds_late")
		}
	}
	return nil
}

func observedTimingKnown(t *ObservedTiming) bool {
	return t != nil && t.StatusKnown && t.LateKnown && t.SecondsLateKnown
}
func (d *ResultsDocument) Validate() error { _, e := ValidateResults(d); return e }

// validateEvidence requires a graded discussion result to say which pulled
// posts it assessed; other assignments carry no discussion evidence.
func validateEvidence(a AssignmentSnapshot, ev *EvidenceSelection) error {
	if a.DiscussionTopicID == nil {
		if ev != nil {
			return fmt.Errorf("evidence is only for graded discussions; this assignment snapshot has no discussion_topic_id (omit the assignment block so the Engine fills it from the packet)")
		}
		return nil
	}
	if ev == nil || strings.TrimSpace(ev.Digest) == "" {
		return fmt.Errorf("a graded discussion result needs evidence: copy digest and entry_refs from the student's discussion_evidence in the pulled work")
	}
	return nil
}

// ValidateResultsEvidence exposes the graded-discussion evidence rule to callers.
func ValidateResultsEvidence(a AssignmentSnapshot, ev *EvidenceSelection) error {
	return validateEvidence(a, ev)
}
