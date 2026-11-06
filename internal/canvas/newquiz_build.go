package canvas

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// NewQuizBuild is an authoring session on one New Quiz, opened the way the
// New Quizzes editor opens it. The public items API rejects stimulus entries
// and ignores stimulus links, so stimuli use the editor's quiz service:
// undocumented, read from the editor's public script bundle and checked in
// Canvas Live 2026-11-05. Its token stays unexported.
type NewQuizBuild struct {
	n      *nqNative
	host   string
	token  string
	quizID string
}

// OpenNewQuizBuild opens the editor session for one New Quiz assignment.
func (c *Client) OpenNewQuizBuild(ctx context.Context, courseID, assignmentID int64) (*NewQuizBuild, error) {
	if courseID <= 0 || assignmentID <= 0 {
		return nil, &Error{Kind: "validation"}
	}
	n, err := c.newQuizNative(ctx, courseID, assignmentID)
	if err != nil {
		return nil, c.mapNative(err)
	}
	var sess struct {
		Host   string           `json:"host"`
		Token  string           `json:"token"`
		QuizID nativeResourceID `json:"quiz_api_quiz_id"`
	}
	if err = secureJSON(ctx, n.httpc, http.MethodGet, n.backend+"/api/assignments/"+url.PathEscape(n.resource)+"?scope=quiz.build", "Bearer "+n.token, "", nil, &sess); err != nil {
		return nil, c.mapNative(fmt.Errorf("quiz editor session step failed: %w", err))
	}
	host, err := trustedInstructureOrigin(sess.Host)
	if err != nil || sess.Token == "" || sess.QuizID == "" {
		return nil, fmt.Errorf("quiz editor session step failed: %w", &Error{Kind: "validation"})
	}
	return &NewQuizBuild{n: n, host: host, token: sess.Token, quizID: string(sess.QuizID)}, nil
}

// CreateStimulus adds a stimulus at a quiz position and returns its quiz entry
// ID. orientation is "left" (questions to the right) or "top" (questions
// below); Canvas shows "left" when its orientation feature is off. A stimulus object created
// before a failed entry request is not part of the quiz and is harmless.
func (b *NewQuizBuild) CreateStimulus(ctx context.Context, title, body, orientation string, position int) (string, error) {
	if orientation != "left" && orientation != "top" {
		return "", &Error{Kind: "validation"}
	}
	var stim struct {
		ID nativeResourceID `json:"id"`
	}
	stimulus := map[string]any{"title": title, "body": body, "instructions": "", "orientation": orientation, "stimulus_type": "text", "passage": false, "source_url": ""}
	if err := b.write(ctx, http.MethodPost, "/stimuli", map[string]any{"stimulus": stimulus}, &stim); err != nil {
		return "", err
	}
	if stim.ID == "" {
		return "", fmt.Errorf("stimulus create returned no ID: %w", &Error{Kind: "validation", Uncertain: true})
	}
	var entry struct {
		ID nativeResourceID `json:"id"`
	}
	if err := b.write(ctx, http.MethodPost, "/quiz_entries", map[string]any{"quiz_entry": map[string]any{"entry_type": "Stimulus", "entry_id": string(stim.ID), "position": position}}, &entry); err != nil {
		return "", err
	}
	if entry.ID == "" {
		return "", fmt.Errorf("stimulus entry returned no ID: %w", &Error{Kind: "validation", Uncertain: true})
	}
	return string(entry.ID), nil
}

// LinkToStimulus attaches an existing question entry to a stimulus entry.
func (b *NewQuizBuild) LinkToStimulus(ctx context.Context, itemEntryID, stimulusEntryID string, position int) error {
	if strings.TrimSpace(itemEntryID) == "" || strings.TrimSpace(stimulusEntryID) == "" {
		return &Error{Kind: "validation"}
	}
	var out struct {
		Stimulus nativeResourceID `json:"stimulus_quiz_entry_id"`
	}
	if err := b.write(ctx, http.MethodPatch, "/quiz_entries/"+url.PathEscape(itemEntryID), map[string]any{"quiz_entry": map[string]any{"stimulus_quiz_entry_id": stimulusEntryID, "position": position}}, &out); err != nil {
		return err
	}
	if string(out.Stimulus) != stimulusEntryID {
		return fmt.Errorf("question was not attached to its stimulus: %w", &Error{Kind: "validation", Uncertain: true})
	}
	return nil
}

func (b *NewQuizBuild) write(ctx context.Context, method, suffix string, body, out any) error {
	path := "/api/quizzes/" + url.PathEscape(b.quizID) + suffix
	err := secureJSON(context.WithoutCancel(ctx), b.n.httpc, method, b.host+path, b.token, "Signature", body, out)
	if err != nil {
		var ce *Error
		if errors.As(err, &ce) && (ce.Kind == "transport" || writeStatusUncertain(ce.Status) || (ce.Status >= 200 && ce.Status < 300)) {
			ce.Uncertain = true
		}
	}
	if c := b.n.client; c != nil && c.OnWrite != nil {
		status, uncertain := 200, false
		var ce *Error
		if errors.As(err, &ce) {
			status, uncertain = ce.Status, ce.Uncertain
		} else if err != nil {
			status = 0
		}
		c.OnWrite(method, path, status, uncertain)
	}
	if err != nil {
		return b.n.client.mapNative(fmt.Errorf("quiz editor write failed: %w", err))
	}
	return nil
}
