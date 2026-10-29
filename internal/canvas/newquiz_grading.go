package canvas

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// NewQuizGrading is one attempt's current New Quiz result, read through the
// chain SpeedGrader uses. Items are the raw session_item_results rows, kept
// whole so a write can send back every field it does not change. The grading
// token stays unexported and is never persisted.
//
// This follows an undocumented internal New Quizzes protocol observed in
// SpeedGrader on 2026-10-29 (docs/new-quiz-written-work-spec.md).
type NewQuizGrading struct {
	QuizSessionID string
	ResultID      string
	// FudgePoints is the current result's fudge_points exactly as read
	// (null or a number); nil means the field was absent, so it is unknown.
	FudgePoints json.RawMessage
	Items       []map[string]any

	httpc *http.Client
	host  string
	token string
}

// NewQuizSession is one assignment's native New Quizzes session, shared by
// the students read through it. Its token stays unexported.
type NewQuizSession struct {
	n *nqNative
}

// NewQuizSession opens the native session for one assignment.
func (c *Client) NewQuizSession(ctx context.Context, courseID, assignmentID int64) (*NewQuizSession, error) {
	if courseID <= 0 || assignmentID <= 0 {
		return nil, &Error{Kind: "validation"}
	}
	n, err := c.newQuizNative(ctx, courseID, assignmentID)
	if err != nil {
		return nil, err
	}
	return &NewQuizSession{n: n}, nil
}

// ResultVersion reads the ID of the attempt's current result. Every
// SpeedGrader save creates a new one, including a save that changes only an
// item comment.
func (s *NewQuizSession) ResultVersion(ctx context.Context, participantSessionID, quizSessionID string) (string, error) {
	g, err := s.session(ctx, participantSessionID, quizSessionID)
	if err != nil {
		return "", err
	}
	return g.ResultID, nil
}

// NewQuizGrading reads the current result of the attempt behind one
// participant session. quizSessionID, when known from the Canvas submission,
// must match the session the grading token is for.
func (c *Client) NewQuizGrading(ctx context.Context, courseID, assignmentID int64, participantSessionID, quizSessionID string) (*NewQuizGrading, error) {
	if strings.TrimSpace(participantSessionID) == "" {
		return nil, &Error{Kind: "validation"}
	}
	s, err := c.NewQuizSession(ctx, courseID, assignmentID)
	if err != nil {
		return nil, err
	}
	g, err := s.session(ctx, participantSessionID, quizSessionID)
	if err != nil {
		return nil, err
	}
	var raw json.RawMessage
	if err = secureJSON(ctx, g.httpc, http.MethodGet, g.host+"/api/quiz_sessions/"+url.PathEscape(g.QuizSessionID)+"/results/"+url.PathEscape(g.ResultID)+"/session_item_results", g.token, "Signature", nil, &raw); err != nil {
		return nil, fmt.Errorf("item results step failed: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if dec.Decode(&g.Items) != nil || len(g.Items) == 0 {
		return nil, fmt.Errorf("item results step failed: %w", &Error{Kind: "validation"})
	}
	return g, nil
}

// session reads the grading token and the quiz session's current result.
func (s *NewQuizSession) session(ctx context.Context, participantSessionID, quizSessionID string) (*NewQuizGrading, error) {
	n := s.n
	if strings.TrimSpace(participantSessionID) == "" {
		return nil, &Error{Kind: "validation"}
	}
	var grade struct {
		Host      string           `json:"host"`
		Token     string           `json:"token"`
		SessionID nativeResourceID `json:"quiz_api_quiz_session_id"`
	}
	if err := secureJSON(ctx, n.httpc, http.MethodGet, n.backend+"/api/participant_sessions/"+url.PathEscape(participantSessionID)+"/grade", "Bearer "+n.token, "", nil, &grade); err != nil {
		return nil, fmt.Errorf("grading session step failed: %w", err)
	}
	host, err := trustedInstructureOrigin(grade.Host)
	if err != nil || grade.Token == "" || grade.SessionID == "" {
		return nil, fmt.Errorf("grading session step failed: %w", &Error{Kind: "validation"})
	}
	if quizSessionID != "" && string(grade.SessionID) != quizSessionID {
		return nil, fmt.Errorf("the quiz session changed since Canvas reported this attempt: %w", &Error{Kind: "validation"})
	}
	g := &NewQuizGrading{QuizSessionID: string(grade.SessionID), httpc: n.httpc, host: host, token: grade.Token}
	var session struct {
		Authoritative map[string]json.RawMessage `json:"authoritative_result"`
	}
	if err = secureJSON(ctx, n.httpc, http.MethodGet, host+"/api/quiz_sessions/"+url.PathEscape(g.QuizSessionID), grade.Token, "Signature", nil, &session); err != nil {
		return nil, fmt.Errorf("quiz session step failed: %w", err)
	}
	var id json.Number
	if json.Unmarshal(session.Authoritative["id"], &id) != nil || id == "" {
		return nil, fmt.Errorf("quiz session step failed: %w", &Error{Kind: "validation"})
	}
	g.ResultID = id.String()
	g.FudgePoints = session.Authoritative["fudge_points"]
	return g, nil
}

// NewQuizItemResult is one item's current state in the quiz result.
type NewQuizItemResult struct {
	Score, GradeStatus, Attempt string
	// Comment is the grader's item comment (HTML), empty when there is none.
	Comment string
}

// Item reads one item's current score, grade status, attempt and grader
// comment.
func (g *NewQuizGrading) Item(entryID string) (NewQuizItemResult, bool) {
	for _, it := range g.Items {
		if fmt.Sprint(it["item_id"]) != entryID {
			continue
		}
		scored, _ := it["scored_data"].(map[string]any)
		feedback, _ := it["feedback"].(map[string]any)
		grader, _ := feedback["grader_feedback"].(map[string]any)
		comment, _ := grader["content"].(string)
		return NewQuizItemResult{Score: fmt.Sprint(it["score"]), GradeStatus: fmt.Sprint(scored["grade_status"]), Attempt: fmt.Sprint(it["attempt"]), Comment: comment}, true
	}
	return NewQuizItemResult{}, false
}

// PostScores sends SpeedGrader's result write: every item of the attempt,
// with only the named items' scores changed, their grade status set to
// graded and, where given, their grader comment (HTML) set, and the fudge
// points read with the result. It returns the new result version's ID.
// There is no retry.
func (g *NewQuizGrading) PostScores(ctx context.Context, scores, comments map[string]string) (string, error) {
	if g.FudgePoints == nil {
		return "", fmt.Errorf("current fudge points were not readable: %w", &Error{Kind: "validation"})
	}
	body, err := newQuizResultBody(g.Items, scores, comments, g.FudgePoints)
	if err != nil {
		return "", err
	}
	var out struct {
		ID json.Number `json:"id"`
	}
	if err = secureJSON(context.WithoutCancel(ctx), g.httpc, http.MethodPost, g.host+"/api/quiz_sessions/"+url.PathEscape(g.QuizSessionID)+"/results", g.token, "Signature", body, &out); err != nil {
		// A lost connection or a server error after the body was sent may
		// still have created a result version.
		var ce *Error
		if errors.As(err, &ce) && (ce.Kind == "transport" || ce.Status >= 500) {
			ce.Uncertain = true
		}
		return "", fmt.Errorf("item score write failed: %w", err)
	}
	if out.ID == "" {
		return "", fmt.Errorf("item score write returned no result version: %w", &Error{Kind: "validation", Uncertain: true})
	}
	return out.ID.String(), nil
}

// snakeKey matches field names made of lowercase words joined by
// underscores. Keys that are IDs (choice UUIDs, numbered blanks) do not
// match and keep their exact spelling.
var snakeKey = regexp.MustCompile(`^[a-z]+(_[a-z]+)+$`)

func camelKey(k string) string {
	if !snakeKey.MatchString(k) {
		return k
	}
	parts := strings.Split(k, "_")
	for i := 1; i < len(parts); i++ {
		parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
	}
	return strings.Join(parts, "")
}

func camelize(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, x := range t {
			out[camelKey(k)] = camelize(x)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, x := range t {
			out[i] = camelize(x)
		}
		return out
	}
	return v
}

// newQuizResultBody builds the observed request: each read row in camelCase
// at every level, without grading_method, with errors {} and graderId null;
// gradedAt stays as read. An item comment is feedback.graderFeedback.content
// beside the untouched itemFeedback. Every named item must be in the result.
func newQuizResultBody(items []map[string]any, scores, comments map[string]string, fudge json.RawMessage) (map[string]any, error) {
	seen := map[string]bool{}
	results := make([]any, 0, len(items))
	for _, it := range items {
		row := camelize(it).(map[string]any)
		delete(row, "gradingMethod")
		row["errors"] = map[string]any{}
		row["graderId"] = nil
		id := fmt.Sprint(it["item_id"])
		if score, ok := scores[id]; ok {
			row["score"] = json.Number(score)
			scored, _ := row["scoredData"].(map[string]any)
			if scored == nil {
				scored = map[string]any{}
				row["scoredData"] = scored
			}
			scored["gradeStatus"] = "graded"
			seen[id] = true
		}
		if comment, ok := comments[id]; ok && comment != "" {
			feedback, _ := row["feedback"].(map[string]any)
			if feedback == nil {
				feedback = map[string]any{}
				row["feedback"] = feedback
			}
			feedback["graderFeedback"] = map[string]any{"content": comment}
		}
		results = append(results, row)
	}
	for id := range scores {
		if !seen[id] {
			return nil, fmt.Errorf("item %s is not in the current quiz result: %w", id, &Error{Kind: "validation"})
		}
	}
	return map[string]any{"results": results, "fudge_points": fudge}, nil
}
