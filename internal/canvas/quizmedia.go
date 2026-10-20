package canvas

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// MaxQuizMediaBytes is the largest New Quiz item image the Engine will upload.
const MaxQuizMediaBytes = 20 << 20

var s3Host = regexp.MustCompile(`(?i)^[a-z0-9.-]+\.s3[a-z0-9.-]*\.amazonaws\.com(\.cn)?$`)

// QuizMediaType returns the content type for a supported image from its leading
// bytes (PNG, JPEG, GIF or WebP); the file name is never trusted.
func QuizMediaType(head []byte) string {
	switch ct := http.DetectContentType(head); ct {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
		return ct
	}
	return ""
}

// UploadQuizMedia stores one image through the New Quizzes media flow and
// returns its unsigned URL for interaction_data.image_url. Canvas hands out a
// presigned PUT URL; it goes only to Canvas's own host or S3, never carries the
// PAT, follows no redirect, and is never returned or put in an error. The PUT is
// not replayed: after an uncertain outcome the caller may upload again, which
// can only orphan an unused image.
func (c *Client) UploadQuizMedia(ctx context.Context, courseID, assignmentID int64, data []byte) (string, error) {
	contentType := QuizMediaType(data)
	if courseID <= 0 || assignmentID <= 0 || len(data) == 0 || len(data) > MaxQuizMediaBytes || contentType == "" {
		return "", &Error{Kind: "validation"}
	}
	var slot struct {
		URL string `json:"url"`
	}
	if err := c.Get(ctx, fmt.Sprintf("/api/quiz/v1/courses/%d/quizzes/%d/items/media_upload_url", courseID, assignmentID), &slot); err != nil {
		return "", err
	}
	u, err := c.mediaDestination(slot.URL)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u.String(), bytes.NewReader(data))
	if err != nil {
		return "", uploadUncertain()
	}
	req.Header.Set("Content-Type", contentType)
	storage := &http.Client{Transport: c.http.Transport, Timeout: 5 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	if err = c.acquire(ctx); err != nil {
		return "", err
	}
	defer c.release()
	resp, err := storage.Do(req)
	if err != nil {
		return "", uploadUncertain()
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		return "", statusError(resp.StatusCode, nil, true)
	}
	return (&url.URL{Scheme: "https", Host: u.Host, Path: u.Path}).String(), nil
}

// mediaDestination accepts only a signed HTTPS URL on the Canvas host or S3.
func (c *Client) mediaDestination(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || strings.ContainsAny(raw, " \t\r\n") || u.Scheme != "https" || u.User != nil || u.Fragment != "" || u.Opaque != "" || u.Path == "" || u.Hostname() == "" {
		return nil, &Error{Kind: "validation"}
	}
	q := u.Query()
	if !q.Has("X-Amz-Signature") && !q.Has("Signature") {
		return nil, &Error{Kind: "validation"}
	}
	canvasHost := strings.EqualFold(u.Hostname(), c.origin.Hostname()) && effectivePort(u) == effectivePort(c.origin)
	if !canvasHost && !(s3Host.MatchString(u.Hostname()) && effectivePort(u) == "443") {
		return nil, &Error{Kind: "validation"}
	}
	return u, nil
}
