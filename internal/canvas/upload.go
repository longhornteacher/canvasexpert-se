package canvas

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// File contains durable metadata only: Canvas download/preview URLs can contain
// signed credentials and deliberately have no field here.
type File struct {
	ID              int64  `json:"id"`
	FolderID        int64  `json:"folder_id"`
	DisplayName     string `json:"display_name"`
	Filename        string `json:"filename"`
	ContentType     string `json:"content-type"`
	Size            int64  `json:"size"`
	Locked          *bool  `json:"locked"`
	Hidden          *bool  `json:"hidden"`
	VisibilityLevel string `json:"visibility_level"`
	UploadStatus    string `json:"upload_status,omitempty"`
	LockAt          string `json:"lock_at,omitempty"`
	UnlockAt        string `json:"unlock_at,omitempty"`
	CreatedAt       string `json:"created_at,omitempty"`
	UpdatedAt       string `json:"updated_at,omitempty"`
}

type CourseFileUpload struct {
	CourseID, FolderID int64
	Name, ContentType  string
	Size               int64
	Source             io.Reader
	// Known persists a returned ID before completion or other downstream work.
	Known func(File) error
}

type uploadDirective struct {
	URL    string                     `json:"upload_url"`
	Params map[string]json.RawMessage `json:"upload_params"`
}

func uploadUncertain() error { return &Error{Kind: "uncertain", Uncertain: true} }

// UploadCourseFile implements Canvas's direct upload protocol. Storage requests
// never carry the PAT, even for same-origin storage. Neither write is replayed.
// https://developerdocs.instructure.com/services/canvas/basics/file.file_uploads
func (c *Client) UploadCourseFile(ctx context.Context, in CourseFileUpload) (file File, err error) {
	if in.CourseID <= 0 || in.FolderID < 0 || in.Name == "" || in.Size < 0 || in.Source == nil {
		return file, &Error{Kind: "validation"}
	}
	body := map[string]any{"name": in.Name, "size": in.Size, "content_type": in.ContentType, "on_duplicate": "rename"}
	if in.FolderID > 0 {
		body["parent_folder_id"] = in.FolderID
	}
	var directive uploadDirective
	if err = c.Write(ctx, "POST", fmt.Sprintf("/api/v1/courses/%d/files", in.CourseID), body, &directive); err != nil {
		return file, err
	}
	u, e := url.Parse(directive.URL)
	if e != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" || u.Opaque != "" || directive.Params == nil || strings.Contains(directive.URL, c.token) {
		return file, uploadUncertain()
	}
	// Build the small multipart envelope in memory; stream the asset (including
	// large videos) from the caller without allocating the whole file.
	var envelope bytes.Buffer
	mw := multipart.NewWriter(&envelope)
	keys := make([]string, 0, len(directive.Params))
	for k := range directive.Params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		var value string
		if k == "file" || json.Unmarshal(directive.Params[k], &value) != nil || strings.Contains(value, c.token) {
			return file, uploadUncertain()
		}
		if e = mw.WriteField(k, value); e != nil {
			return file, uploadUncertain()
		}
	}
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{"name": "file", "filename": in.Name}))
	header.Set("Content-Type", in.ContentType)
	if _, e = mw.CreatePart(header); e != nil {
		return file, uploadUncertain()
	}
	prefix := append([]byte(nil), envelope.Bytes()...)
	if e = mw.Close(); e != nil {
		return file, uploadUncertain()
	}
	footer := envelope.Bytes()[len(prefix):]
	req, e := http.NewRequestWithContext(ctx, "POST", u.String(), io.MultiReader(bytes.NewReader(prefix), io.LimitReader(in.Source, in.Size), bytes.NewReader(footer)))
	if e != nil {
		return file, uploadUncertain()
	}
	req.ContentLength = int64(len(prefix)+len(footer)) + in.Size
	req.Header.Set("Content-Type", mw.FormDataContentType())
	// Use the configured TLS/proxy transport but no redirect handler that can
	// add credentials, no cookie jar, and no automatic POST redirect replay.
	storage := &http.Client{Transport: c.http.Transport, Timeout: 15 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	if e = c.acquire(ctx); e != nil {
		return file, e
	}
	resp, e := storage.Do(req)
	if e != nil {
		c.release()
		return file, uploadUncertain()
	}
	released := false
	defer func() {
		if !released {
			resp.Body.Close()
			c.release()
		}
	}()
	if (resp.StatusCode >= 300 && resp.StatusCode < 400 || resp.StatusCode == http.StatusCreated) && resp.Header.Get("Location") != "" {
		ref, parseErr := url.Parse(resp.Header.Get("Location"))
		if parseErr != nil {
			return file, uploadUncertain()
		}
		completion, id, parseErr := c.uploadCompletion(u.ResolveReference(ref))
		if parseErr != nil {
			return file, uploadUncertain()
		}
		file.ID = id
		if in.Known != nil {
			if err = in.Known(file); err != nil {
				return file, err
			}
		}
		resp.Body.Close()
		c.release()
		released = true
		// Completion is a GET per the current protocol prose (the historical
		// documentation's curl example still says POST).
		var completed File
		if e = c.Get(ctx, completion, &completed); e != nil || completed.ID != id || completed.Size != in.Size {
			return file, uploadUncertain()
		}
		file = completed
	} else {
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			// Exclude all returned bodies and signed URLs from errors/receipts.
			return file, statusError(resp.StatusCode, nil, true)
		}
		if e = readJSON(resp, &file); e != nil || file.ID <= 0 {
			return file, uploadUncertain()
		}
		if in.Known != nil {
			if err = in.Known(file); err != nil {
				return file, err
			}
		}
		if file.Size != in.Size {
			return file, uploadUncertain()
		}
	}
	return file, nil
}

func (c *Client) uploadCompletion(u *url.URL) (string, int64, error) {
	if !sameOrigin(c.origin, u) || u.User != nil || u.Fragment != "" || u.Opaque != "" || u.RawPath != "" {
		return "", 0, &Error{Kind: "validation"}
	}
	parts := strings.Split(u.Path, "/")
	if (len(parts) != 5 && len(parts) != 6) || parts[0] != "" || parts[1] != "api" || parts[2] != "v1" || parts[3] != "files" || len(parts) == 6 && parts[5] != "create_success" {
		return "", 0, &Error{Kind: "validation"}
	}
	id, e := strconv.ParseInt(parts[4], 10, 64)
	q, qe := url.ParseQuery(u.RawQuery)
	if e != nil || id <= 0 || qe != nil || q.Has("as_user_id") || q.Has("access_token") {
		return "", 0, &Error{Kind: "validation"}
	}
	return u.String(), id, nil
}
