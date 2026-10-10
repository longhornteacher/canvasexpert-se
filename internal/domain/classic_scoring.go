package domain

import (
	"fmt"
	"strings"
	"time"
)

type ClassicQuizPacket struct {
	Schema     int                `yaml:"schema"`
	Kind       string             `yaml:"kind"`
	Course     string             `yaml:"course"`
	Assignment AssignmentSnapshot `yaml:"assignment"`
	QuizID     int64              `yaml:"quiz_id"`
	Students   []string           `yaml:"students"`
}
type ClassicQuizWork struct {
	Schema               int                  `yaml:"schema"`
	Kind                 string               `yaml:"kind"`
	StudentRef           string               `yaml:"student_ref"`
	Alias                string               `yaml:"alias"`
	AssignmentID         int64                `yaml:"assignment_id"`
	QuizID               int64                `yaml:"quiz_id"`
	SubmissionID         int64                `yaml:"submission_id"`
	QuizSubmissionID     int64                `yaml:"quiz_submission_id"`
	Current              ClassicAttemptWork   `yaml:"current"`
	Prior                []ClassicAttemptWork `yaml:"prior,omitempty"`
	UnhandledQuestionIDs []int64              `yaml:"unhandled_question_ids"`
	Notes                []string             `yaml:"notes,omitempty"`
	Monitored            bool                 `yaml:"monitored"`
}
type ClassicAttemptWork struct {
	Ref            AttemptRef            `yaml:"ref"`
	Questions      []ClassicQuestionWork `yaml:"questions"`
	Late           bool                  `yaml:"late"`
	EffectiveDueAt string                `yaml:"effective_due_at,omitempty"`
	CanvasScore    *Decimal              `yaml:"canvas_score,omitempty"`
	Warnings       []string              `yaml:"warnings,omitempty"`
}
type ClassicQuestionWork struct {
	QuestionID      int64        `yaml:"question_id"`
	Position        int          `yaml:"position"`
	Name            string       `yaml:"name"`
	QuestionType    string       `yaml:"question_type"`
	PromptHTML      string       `yaml:"prompt_html"`
	PromptText      string       `yaml:"prompt_text"`
	PointsPossible  Decimal      `yaml:"points_possible"`
	ResponsePresent bool         `yaml:"response_present"`
	ResponseHTML    string       `yaml:"response_html,omitempty"`
	ResponseText    string       `yaml:"response_text,omitempty"`
	Attachments     []Attachment `yaml:"attachments,omitempty"`
	ObservedScore   *Decimal     `yaml:"observed_score,omitempty"`
	ObservedComment string       `yaml:"observed_comment,omitempty"`
	Warning         string       `yaml:"warning,omitempty"`
}
type ClassicQuizResults struct {
	Schema       int                    `yaml:"schema"`
	Kind         string                 `yaml:"kind"`
	Packet       string                 `yaml:"packet"`
	Assignment   AssignmentSnapshot     `yaml:"assignment"`
	QuizID       int64                  `yaml:"quiz_id"`
	ScoringNotes string                 `yaml:"scoring_notes"`
	Students     []ClassicStudentResult `yaml:"students"`
}
type ClassicStudentResult struct {
	StudentRef     string                  `yaml:"student_ref"`
	Attempt        AttemptRef              `yaml:"attempt"`
	Questions      []ClassicQuestionResult `yaml:"questions"`
	StrongestPrior *PriorComparison        `yaml:"strongest_prior,omitempty"`
	TeacherNote    string                  `yaml:"teacher_note,omitempty"`
	LateTiming     *LateTiming             `yaml:"late_timing,omitempty"`
	LateOutcome    ActionOutcome           `yaml:"late_outcome,omitempty"`
}
type ClassicQuestionResult struct {
	QuestionID  int64         `yaml:"question_id"`
	Evidence    string        `yaml:"evidence"`
	Score       *Decimal      `yaml:"score,omitempty"`
	Comment     string        `yaml:"comment,omitempty"`
	TeacherNote string        `yaml:"teacher_note,omitempty"`
	Outcome     ActionOutcome `yaml:"outcome,omitempty"`
}

func ClassicManual(t string) bool { return t == "essay_question" || t == "file_upload_question" }
func ValidateClassicTarget(s AssignmentSnapshot) error {
	if e := ValidateSnapshot(s); e != nil {
		return e
	}
	if (s.GradingType != "points" && s.GradingType != "percent") || s.AnonymousGrading || s.ModeratedGrading || s.GroupCategoryID != 0 {
		return fmt.Errorf("Classic scoring requires individual numeric nonanonymous, nonmoderated grading")
	}
	if len(s.SubmissionTypes) != 1 || s.SubmissionTypes[0] != "online_quiz" {
		return fmt.Errorf("Classic scoring requires online_quiz")
	}
	return nil
}
func (d *ClassicQuizPacket) Validate() error {
	if d.Schema != 1 || d.Kind != "classic-quiz-packet" || d.QuizID <= 0 {
		return fmt.Errorf("invalid Classic packet identity/schema")
	}
	if e := (&PacketDocument{Schema: 1, Kind: "packet", Course: d.Course, Assignment: d.Assignment, Students: d.Students}).Validate(); e != nil {
		return e
	}
	return ValidateClassicTarget(d.Assignment)
}
func (a *ClassicAttemptWork) Validate() error {
	if e := validateAttempt(a.Ref); e != nil {
		return e
	}
	ids := map[int64]bool{}
	for _, q := range a.Questions {
		if q.QuestionID <= 0 || ids[q.QuestionID] || q.QuestionType == "" || !q.PointsPossible.IsSet() || q.PointsPossible.Sign() < 0 {
			return fmt.Errorf("invalid/duplicate Classic question or maximum")
		}
		ids[q.QuestionID] = true
	}
	return nil
}
func (d *ClassicQuizWork) Validate() error {
	if d.Schema != 1 || d.Kind != "classic-quiz-work" || d.StudentRef == "" || d.AssignmentID <= 0 || d.QuizID <= 0 || d.SubmissionID <= 0 || d.QuizSubmissionID <= 0 {
		return fmt.Errorf("invalid Classic work identity/schema")
	}
	if e := d.Current.Validate(); e != nil {
		return e
	}
	last := 0
	now, _ := time.Parse(time.RFC3339Nano, d.Current.Ref.SubmittedAt)
	for _, a := range d.Prior {
		if e := a.Validate(); e != nil {
			return e
		}
		t, _ := time.Parse(time.RFC3339Nano, a.Ref.SubmittedAt)
		if a.Ref.Attempt <= last || a.Ref.Attempt >= d.Current.Ref.Attempt || !t.Before(now) {
			return fmt.Errorf("prior attempts must be ascending and preceding")
		}
		last = a.Ref.Attempt
	}
	ids := map[int64]bool{}
	for _, id := range d.UnhandledQuestionIDs {
		found := false
		for _, q := range d.Current.Questions {
			if q.QuestionID == id && ClassicManual(q.QuestionType) {
				found = true
			}
		}
		if !found || ids[id] {
			return fmt.Errorf("invalid/duplicate unhandled question")
		}
		ids[id] = true
	}
	return nil
}
func (d *ClassicQuizResults) Validate() error {
	if d.Schema != 1 || d.Kind != "classic-quiz-results" || d.Packet == "" || d.QuizID <= 0 || strings.TrimSpace(d.ScoringNotes) == "" || len(d.Students) == 0 {
		return fmt.Errorf("Classic results require identity, packet, scoring_notes and students")
	}
	if e := ValidateClassicTarget(d.Assignment); e != nil {
		return e
	}
	students := map[string]bool{}
	for _, s := range d.Students {
		if s.StudentRef == "" || students[s.StudentRef] || len(s.Questions) == 0 {
			return fmt.Errorf("invalid/duplicate Classic student or empty questions")
		}
		students[s.StudentRef] = true
		if e := validateAttempt(s.Attempt); e != nil {
			return e
		}
		if e := validateLateTiming(s.LateTiming); e != nil {
			return e
		}
		if e := validateOutcome(s.LateOutcome); e != nil {
			return e
		}
		if s.LateTiming != nil && s.LateOutcome.Status == "not-requested" {
			return fmt.Errorf("requested timing cannot be not-requested")
		}
		if s.StrongestPrior != nil {
			if e := validateAttempt(s.StrongestPrior.Attempt); e != nil {
				return e
			}
			if strings.TrimSpace(s.StrongestPrior.Basis) == "" {
				return fmt.Errorf("strongest prior basis required")
			}
		}
		ids := map[int64]bool{}
		for _, q := range s.Questions {
			if q.QuestionID <= 0 || ids[q.QuestionID] {
				return fmt.Errorf("invalid/duplicate question result")
			}
			ids[q.QuestionID] = true
			if e := validateOutcome(q.Outcome); e != nil {
				return e
			}
			if q.Outcome.Status == "not-requested" || (q.Outcome.ID != 0 && q.Outcome.ID != q.QuestionID) {
				return fmt.Errorf("invalid question outcome identity/status")
			}
			switch q.Evidence {
			case "present", "missing":
				if q.Score == nil || !q.Score.IsSet() || q.Score.Sign() < 0 || strings.TrimSpace(q.Comment) == "" {
					return fmt.Errorf("assessed question requires nonnegative score and comment")
				}
			case "unknown":
				if q.Outcome.Status == "succeeded" || q.Outcome.Status == "preserved" {
					return fmt.Errorf("unknown evidence cannot carry a completed question assessment")
				}
				if q.Score != nil || q.Comment != "" || strings.TrimSpace(q.TeacherNote) == "" {
					return fmt.Errorf("unknown question requires private explanation, no score/comment")
				}
			default:
				return fmt.Errorf("question evidence must be present, missing, or unknown")
			}
		}
	}
	return nil
}
func NormalizeClassicScore(score *Decimal, max Decimal) error {
	if score == nil {
		return nil
	}
	if score.Cmp(max) > 0 || score.Sign() < 0 {
		return fmt.Errorf("question score outside 0..%s", max.String())
	}
	normalized := RoundRat(score.Rat(), 2)
	if normalized.Cmp(max) > 0 || normalized.Sign() < 0 {
		return fmt.Errorf("rounded question score outside 0..%s", max.String())
	}
	*score = normalized
	return nil
}
func ClassicQuestionComment(score, max Decimal, comment string) string {
	return "Assessment: " + score.String() + "/" + max.String() + "\n\n" + comment
}
