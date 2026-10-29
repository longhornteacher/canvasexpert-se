package canvas

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

// Native launch returns numeric resource IDs live; the string form remains supported.
type nativeResourceID string

func (id *nativeResourceID) UnmarshalJSON(data []byte) error {
	var text string
	if len(data) > 0 && data[0] == '"' {
		if err := json.Unmarshal(data, &text); err != nil {
			return err
		}
	} else {
		var n json.Number
		if err := json.Unmarshal(data, &n); err != nil {
			return err
		}
		if value, err := n.Int64(); err != nil || value <= 0 {
			return &Error{Kind: "validation"}
		}
		text = n.String()
	}
	if strings.TrimSpace(text) == "" {
		return &Error{Kind: "validation"}
	}
	*id = nativeResourceID(text)
	return nil
}

// NewQuizFile is metadata for one actual uploaded file. URL is a signed
// storage URL and must only be used with Download (which sends no credentials
// to a different origin).
type NewQuizFile struct {
	ItemID string
	Name   string
	Size   int64
	URL    string
}

// NewQuizFiles follows the native SpeedGrader read path for one Canvas user
// and attempt. PAT-authenticated requests stay on Canvas; derived tokens are
// sent only to the validated backend and quiz hosts supplied by Canvas.
// participantSessionID, from the submission's launch URL, skips the
// participants lookup, which returned 404 live on 2026-10-28.
func (c *Client) NewQuizFiles(ctx context.Context, courseID, assignmentID, userID int64, attempt int, itemIDs map[string]bool, participantSessionID string) ([]NewQuizFile, error) {
	if courseID <= 0 || assignmentID <= 0 || userID <= 0 || attempt < 1 || len(itemIDs) == 0 {
		return nil, &Error{Kind: "validation"}
	}
	n, err := c.newQuizNative(ctx, courseID, assignmentID)
	if err != nil {
		return nil, err
	}
	httpc, backend, nativeToken := n.httpc, n.backend, n.token
	participantSession := participantSessionID
	if participantSession == "" {
		type participant struct {
			CanvasUserID int64 `json:"canvas_user_id"`
			Sessions     []struct {
				ID string `json:"id"`
			} `json:"participant_sessions"`
		}
		var participantRaw json.RawMessage
		if err = secureJSON(ctx, httpc, http.MethodGet, backend+"/api/assignments/"+url.PathEscape(n.resource)+"/participants?no_pagination=true", "Bearer "+nativeToken, "application/json", nil, &participantRaw); err != nil {
			return nil, fmt.Errorf("participants step failed: %w", err)
		}
		var participants []participant
		if len(participantRaw) > 0 && participantRaw[0] == '[' {
			err = json.Unmarshal(participantRaw, &participants)
		} else {
			var envelope struct {
				Data []participant `json:"data"`
			}
			err = json.Unmarshal(participantRaw, &envelope)
			participants = envelope.Data
		}
		if err != nil {
			return nil, fmt.Errorf("participants step failed: %w", &Error{Kind: "validation"})
		}
		for _, p := range participants {
			if p.CanvasUserID == userID && len(p.Sessions) > 0 {
				participantSession = p.Sessions[0].ID
				break
			}
		}
		if participantSession == "" {
			return nil, fmt.Errorf("participants step found no matching student session: %w", &Error{Kind: "not_found"})
		}
	}
	var result struct {
		Host         string           `json:"host"`
		Token        string           `json:"token"`
		SessionID    nativeResourceID `json:"quiz_api_quiz_session_id"`
		OldSessionID nativeResourceID `json:"quiz_session_id"`
	}
	// The grading read (what SpeedGrader uses) carries each file's URL in the
	// item results; the older results read is the fallback.
	if err = secureJSON(ctx, httpc, http.MethodGet, backend+"/api/participant_sessions/"+url.PathEscape(participantSession)+"/grade", "Bearer "+nativeToken, "", nil, &result); err != nil {
		if err = secureJSON(ctx, httpc, http.MethodGet, backend+"/api/participant_sessions/"+url.PathEscape(participantSession)+"/results", "Bearer "+nativeToken, "application/json", nil, &result); err != nil {
			return nil, fmt.Errorf("participant results step failed: %w", err)
		}
	}
	quizHost, err := trustedInstructureOrigin(result.Host)
	if err != nil || result.Token == "" {
		return nil, fmt.Errorf("quiz host step failed: %w", &Error{Kind: "validation"})
	}
	// Canvas sends these session IDs as numbers live; strings stay accepted.
	quizSession := string(result.SessionID)
	if quizSession == "" {
		quizSession = string(result.OldSessionID)
	}
	if quizSession == "" {
		return nil, fmt.Errorf("participant results step failed: %w", &Error{Kind: "validation"})
	}
	var session struct {
		Authoritative struct {
			ID json.Number `json:"id"`
		} `json:"authoritative_result"`
	}
	if err = secureJSON(ctx, httpc, http.MethodGet, quizHost+"/api/quiz_sessions/"+url.PathEscape(quizSession), result.Token, "Signature", nil, &session); err != nil {
		return nil, fmt.Errorf("quiz session step failed: %w", err)
	}
	if session.Authoritative.ID == "" {
		return nil, fmt.Errorf("quiz session step failed: %w", &Error{Kind: "validation"})
	}
	var itemResults []struct {
		ItemID  json.Number `json:"item_id"`
		Attempt int         `json:"attempt"`
		// Value is a file list on upload items and text on essays, so it
		// is decoded only for the requested upload items.
		Scored struct {
			Value json.RawMessage `json:"value"`
		} `json:"scored_data"`
	}
	if err = secureJSON(ctx, httpc, http.MethodGet, quizHost+"/api/quiz_sessions/"+url.PathEscape(quizSession)+"/results/"+url.PathEscape(session.Authoritative.ID.String())+"/session_item_results", result.Token, "Signature", nil, &itemResults); err != nil {
		return nil, fmt.Errorf("item results step failed: %w", err)
	}
	var files []NewQuizFile
	matchedAttempt := false
	attemptMismatch := false
	for _, it := range itemResults {
		itemID := it.ItemID.String()
		if !itemIDs[itemID] {
			continue
		}
		if it.Attempt != attempt {
			attemptMismatch = true
			continue
		}
		matchedAttempt = true
		var uploaded []struct {
			ID   json.RawMessage `json:"id"`
			Name string          `json:"name"`
			URL  string          `json:"url"`
			Size int64           `json:"size"`
		}
		if json.Unmarshal(it.Scored.Value, &uploaded) != nil {
			continue
		}
		for _, f := range uploaded {
			if f.Name != "" && f.URL != "" && f.Size >= 0 {
				files = append(files, NewQuizFile{ItemID: itemID, Name: f.Name, Size: f.Size, URL: f.URL})
			}
		}
	}
	if !matchedAttempt {
		if attemptMismatch {
			return nil, fmt.Errorf("item results attempt did not match requested attempt")
		}
		return nil, fmt.Errorf("item results step had no matching file-upload item")
	}
	return files, nil
}

// nqNative is a native New Quizzes session for one assignment: the cookie
// jar, the validated backend host and the derived native token. The token
// never leaves this process and is sent only to the backend host.
type nqNative struct {
	httpc    *http.Client
	backend  string
	token    string
	resource string
}

// newQuizNative follows Canvas's sessionless launch to a native New Quizzes
// token. PAT-authenticated requests stay on Canvas.
func (c *Client) newQuizNative(ctx context.Context, courseID, assignmentID int64) (*nqNative, error) {
	jar, _ := cookiejar.New(nil)
	httpc := &http.Client{Transport: c.http.Transport, Jar: jar, Timeout: 45 * time.Second}
	httpc.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 || req.URL.Scheme != "https" || req.URL.User != nil || !sameOrigin(c.origin, req.URL) {
			return &Error{Kind: "validation"}
		}
		return nil
	}
	launchPath := fmt.Sprintf("/api/v1/courses/%d/external_tools/sessionless_launch?launch_type=assessment&assignment_id=%d", courseID, assignmentID)
	launchURL, _ := c.apiURL(launchPath)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, launchURL.String(), nil)
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "text/html,application/json")
	resp, err := httpc.Do(req)
	if err != nil {
		return nil, &Error{Kind: "transport"}
	}
	page, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20+1))
	resp.Body.Close()
	if err != nil || len(page) > 8<<20 || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &Error{Kind: "validation"}
	}
	// Canvas may answer with JSON naming a one-time launch URL on its own
	// origin instead of the launch page itself; follow that URL once.
	var launch struct {
		URL string `json:"url"`
	}
	if json.Unmarshal(page, &launch) == nil && launch.URL != "" {
		next, perr := url.Parse(launch.URL)
		if perr != nil || next.Scheme != "https" || next.User != nil || !sameOrigin(c.origin, next) {
			return nil, fmt.Errorf("launch URL step failed: %w", &Error{Kind: "validation"})
		}
		req, _ = http.NewRequestWithContext(ctx, http.MethodGet, next.String(), nil)
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Accept", "text/html")
		if resp, err = httpc.Do(req); err != nil {
			return nil, &Error{Kind: "transport"}
		}
		page, err = io.ReadAll(io.LimitReader(resp.Body, 8<<20+1))
		resp.Body.Close()
		if err != nil || len(page) > 8<<20 || resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, &Error{Kind: "validation"}
		}
	}
	env, err := canvasENV(page)
	if err != nil {
		return nil, fmt.Errorf("launch ENV step failed: %w", err)
	}
	var config struct {
		NewQuizzes struct {
			Params    json.RawMessage `json:"params"`
			Signature string          `json:"signature"`
		} `json:"NEW_QUIZZES"`
		AccountID json.Number `json:"ACCOUNT_ID"`
	}
	if json.Unmarshal(env, &config) != nil || len(config.NewQuizzes.Params) == 0 || config.NewQuizzes.Signature == "" || config.AccountID == "" {
		return nil, fmt.Errorf("launch ENV step failed: %w", &Error{Kind: "validation"})
	}
	var params struct {
		BackendURL string `json:"backend_url"`
	}
	if json.Unmarshal(config.NewQuizzes.Params, &params) != nil {
		return nil, fmt.Errorf("launch params step failed: %w", &Error{Kind: "validation"})
	}
	backend, err := trustedInstructureOrigin(params.BackendURL)
	if err != nil {
		return nil, fmt.Errorf("backend host step failed: %w", err)
	}
	var jwt struct {
		Token string `json:"token"`
	}
	jwtPath := "/api/v1/jwts?canvas_audience=false&workflows[]=new_quizzes_native_launch&context_id=" + url.QueryEscape(config.AccountID.String()) + "&context_type=account"
	if err = secureJSON(ctx, httpc, http.MethodPost, c.Origin()+jwtPath, "Bearer "+c.token, "", map[string]any{}, &jwt); err != nil || jwt.Token == "" {
		return nil, fmt.Errorf("Canvas workflow token step failed: %w", &Error{Kind: "validation"})
	}
	var native struct {
		Token       string `json:"token"`
		JWT         string `json:"jwt"`
		AccessToken string `json:"access_token"`
		Entry       struct {
			ResourceID nativeResourceID `json:"resourceId"`
		} `json:"entry_path"`
	}
	if err = secureJSON(ctx, httpc, http.MethodPost, backend+"/api/native/launch", "Bearer "+jwt.Token, "", map[string]any{"params": json.RawMessage(config.NewQuizzes.Params), "signature": config.NewQuizzes.Signature}, &native, c.origin.Host); err != nil {
		return nil, fmt.Errorf("native launch step failed: %w", err)
	}
	nativeToken := native.Token
	if nativeToken == "" {
		nativeToken = native.JWT
	}
	if nativeToken == "" {
		nativeToken = native.AccessToken
	}
	if nativeToken == "" || native.Entry.ResourceID == "" {
		return nil, fmt.Errorf("native launch step failed: %w", &Error{Kind: "validation"})
	}
	return &nqNative{httpc: httpc, backend: backend, token: nativeToken, resource: string(native.Entry.ResourceID)}, nil
}

func trustedInstructureOrigin(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" || u.User != nil || u.Host == "" || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || !strings.HasSuffix(strings.ToLower(u.Hostname()), ".instructure.com") {
		return "", &Error{Kind: "validation"}
	}
	u.Path = ""
	return strings.TrimRight(u.String(), "/"), nil
}

func secureJSON(ctx context.Context, hc *http.Client, method, rawURL, authorization, authType string, body any, out any, domainHeader ...string) error {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Host == "" {
		return &Error{Kind: "validation"}
	}
	var data []byte
	if body != nil {
		data, err = json.Marshal(body)
		if err != nil {
			return &Error{Kind: "validation"}
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), bytes.NewReader(data))
	if err != nil {
		return &Error{Kind: "validation"}
	}
	req.Header.Set("Accept", "application/json")
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	if authType != "" {
		req.Header.Set("AuthType", authType)
	}
	if len(domainHeader) > 0 && domainHeader[0] != "" {
		req.Header.Set("X-Domain", domainHeader[0])
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := hc.Do(req)
	if err != nil {
		return &Error{Kind: "transport"}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return &Error{Kind: "validation", Status: resp.StatusCode}
	}
	data, err = io.ReadAll(io.LimitReader(resp.Body, maxJSON+1))
	if err != nil || len(data) > maxJSON {
		return &Error{Kind: "validation"}
	}
	if json.Unmarshal(data, out) != nil {
		return &Error{Kind: "validation"}
	}
	return nil
}

// canvasENV extracts and decodes the JSON object assigned to the launch page's
// ENV variable. It never evaluates JavaScript.
func canvasENV(page []byte) (json.RawMessage, error) {
	s := string(page)
	marker := strings.Index(s, "ENV")
	if marker < 0 {
		return nil, &Error{Kind: "validation"}
	}
	start := strings.Index(s[marker:], "{")
	if start < 0 {
		return nil, &Error{Kind: "validation"}
	}
	start += marker + 0
	// Find the first object after ENV, balancing braces while respecting JSON strings.
	open := strings.Index(s[marker:], "{") + marker
	depth := 0
	quoted, escape := false, false
	for i := open; i < len(s); i++ {
		ch := s[i]
		if quoted {
			if escape {
				escape = false
			} else if ch == '\\' {
				escape = true
			} else if ch == '"' {
				quoted = false
			}
			continue
		}
		if ch == '"' {
			quoted = true
			continue
		}
		if ch == '{' {
			depth++
		}
		if ch == '}' {
			depth--
			if depth == 0 {
				raw := json.RawMessage(s[open : i+1])
				var v any
				if json.Unmarshal(raw, &v) != nil {
					return nil, &Error{Kind: "validation"}
				}
				return raw, nil
			}
		}
	}
	return nil, &Error{Kind: "validation"}
}
