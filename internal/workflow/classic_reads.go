package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"canvasexpertse/internal/canvas"
	"canvasexpertse/internal/domain"
)

// ClassicID accepts Canvas numeric and quoted numeric IDs without projecting them.
type ClassicID int64

func (id *ClassicID) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		*id = 0
		return nil
	}
	n, e := strconv.ParseInt(strings.Trim(string(b), `"`), 10, 64)
	if e != nil {
		return fmt.Errorf("invalid Canvas numeric id")
	}
	*id = ClassicID(n)
	return nil
}

type ClassicQuiz struct {
	ID            ClassicID `json:"id"`
	AssignmentID  ClassicID `json:"assignment_id"`
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	QuizType      string    `json:"quiz_type"`
	ScoringPolicy string    `json:"scoring_policy"`
}
type ClassicSubmission struct {
	ID           ClassicID    `json:"id"`
	QuizID       ClassicID    `json:"quiz_id"`
	UserID       ClassicID    `json:"user_id"`
	SubmissionID ClassicID    `json:"submission_id"`
	Attempt      int          `json:"attempt"`
	FinishedAt   *string      `json:"finished_at"`
	State        string       `json:"workflow_state"`
	Score        *json.Number `json:"score"`
	KeptScore    *json.Number `json:"kept_score"`
}
type ClassicQuestion struct {
	ID       ClassicID      `json:"id"`
	Position int            `json:"position"`
	Name     string         `json:"question_name"`
	Type     string         `json:"question_type"`
	Text     string         `json:"question_text"`
	Points   domain.Decimal `json:"points_possible"`
}
type ClassicAnswer struct {
	QuestionID    ClassicID    `json:"question_id"`
	Points        *json.Number `json:"points"`
	Text          string       `json:"text"`
	AttachmentIDs []ClassicID  `json:"attachment_ids"`
	MoreComments  string       `json:"more_comments"`
}

func classicPath(course, quiz int64) string {
	return fmt.Sprintf("/api/v1/courses/%d/quizzes/%d", course, quiz)
}
func getClassicQuiz(ctx context.Context, c *canvas.Client, course, quiz int64) (ClassicQuiz, error) {
	var q ClassicQuiz
	e := c.Get(ctx, classicPath(course, quiz), &q)
	return q, e
}
func classicSubmissions(ctx context.Context, c *canvas.Client, course, quiz int64) ([]ClassicSubmission, error) {
	var rows []ClassicSubmission
	e := c.ListField(ctx, classicPath(course, quiz)+"/submissions?per_page=100", "quiz_submissions", &rows)
	return rows, e
}
func getClassicSubmission(ctx context.Context, c *canvas.Client, course, quiz, id int64) (ClassicSubmission, error) {
	var response struct {
		Rows []ClassicSubmission `json:"quiz_submissions"`
	}
	e := c.Get(ctx, fmt.Sprintf("%s/submissions/%d", classicPath(course, quiz), id), &response)
	if e != nil {
		return ClassicSubmission{}, e
	}
	if len(response.Rows) != 1 || int64(response.Rows[0].ID) != id {
		return ClassicSubmission{}, fmt.Errorf("quiz submission response identity is unavailable or ambiguous")
	}
	return response.Rows[0], nil
}
func classicQuestions(ctx context.Context, c *canvas.Client, course, quiz, id int64, attempt int) ([]ClassicQuestion, error) {
	var rows []ClassicQuestion
	e := c.List(ctx, fmt.Sprintf("%s/questions?quiz_submission_id=%d&quiz_submission_attempt=%d&per_page=100", classicPath(course, quiz), id, attempt), &rows)
	return rows, e
}
func classicJoin(q ClassicSubmission, sub Submission, quiz, assignment, user int64) string {
	if int64(q.QuizID) != quiz || int64(q.UserID) != user || sub.UserID != user || sub.AssignmentID != assignment || sub.ID <= 0 || int64(q.SubmissionID) != sub.ID {
		return "assignment/quiz submission identity disagrees"
	}
	if q.Attempt != sub.Attempt || q.Attempt < 1 || sub.SubmittedAt == nil || q.FinishedAt == nil || *q.FinishedAt == "" || (q.State != "pending_review" && q.State != "complete") {
		return "current quiz attempt is not finished or disagrees with submitted assignment attempt"
	}
	if sub.Excused {
		return "excused work skipped"
	}
	if sub.AssignmentVisible != nil && !*sub.AssignmentVisible {
		return "work not currently assigned to student"
	}
	if sub.Type != "online_quiz" {
		return "submission is not Classic quiz work"
	}
	return ""
}

// History row IDs are deliberately irrelevant: exact attempt and timestamp select evidence.
func classicResponse(sub Submission, ref domain.AttemptRef) (Submission, string) {
	var found *Submission
	for _, row := range append([]Submission{sub}, sub.History...) {
		if row.Attempt != ref.Attempt || len(row.SubmissionData) == 0 {
			continue
		}
		if !sameAttempt(attemptRef(row), ref) {
			return Submission{}, "response history timestamp disagrees with intended attempt"
		}
		if found != nil && !sameClassicAnswers(found.SubmissionData, row.SubmissionData) {
			return Submission{}, "ambiguous response history for intended attempt"
		}
		copy := row
		found = &copy
	}
	if found == nil {
		return Submission{}, "response data unavailable for intended attempt"
	}
	return *found, ""
}
func sameClassicAnswers(a, b []ClassicAnswer) bool {
	aa, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return string(aa) == string(bb)
}
func classicAnswers(rows []ClassicAnswer) (map[int64]ClassicAnswer, map[int64]bool) {
	out := map[int64]ClassicAnswer{}
	ambiguous := map[int64]bool{}
	for _, a := range rows {
		id := int64(a.QuestionID)
		if _, ok := out[id]; ok {
			ambiguous[id] = true
		}
		out[id] = a
	}
	return out, ambiguous
}
