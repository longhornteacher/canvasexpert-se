package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"canvasexpertse/internal/canvas"
	"canvasexpertse/internal/workflow"
)

// This is a disposable fixture helper, not an arbitrary upload/API command.
// Canvas's documented workflow requires the file part last and no API token
// at the storage endpoint: https://developerdocs.instructure.com/services/canvas/basics/file.file_uploads
type probeUploadDirective struct {
	URL    string                     `json:"upload_url"`
	Params map[string]json.RawMessage `json:"upload_params"`
}

func (p *probe) submitFile(ctx context.Context, args []string) error {
	f := flag.NewFlagSet("submit-file", flag.ContinueOnError)
	path := f.String("file", "", "local synthetic fixture file")
	if e := f.Parse(args); e != nil {
		return e
	}
	if len(f.Args()) != 0 || *path == "" {
		return fmt.Errorf("submit-file requires --file SYNTHETIC_PATH")
	}
	name := filepath.Base(*path)
	if !strings.Contains(strings.ToLower(name), "synthetic") {
		return fmt.Errorf("fixture filename must include synthetic")
	}
	file, e := os.Open(*path)
	if e != nil {
		return fmt.Errorf("cannot open synthetic fixture file")
	}
	defer file.Close()
	info, e := file.Stat()
	if e != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > 10<<20 {
		return fmt.Errorf("synthetic fixture must be a nonempty regular file of at most 10 MiB")
	}
	data, e := io.ReadAll(io.LimitReader(file, (10<<20)+1))
	if e != nil || int64(len(data)) != info.Size() {
		return fmt.Errorf("synthetic fixture changed or could not be read")
	}
	a, e := p.assignment(ctx)
	if e != nil {
		return e
	}
	allowed := false
	for _, kind := range a.SubmissionTypes {
		allowed = allowed || kind == "online_upload"
	}
	if !allowed {
		return fmt.Errorf("exact disposable assignment does not allow online_upload")
	}
	if e = p.target(ctx); e != nil {
		return e
	}
	hash := sha256.Sum256(append([]byte(name+"\x00"), data...))
	key := fmt.Sprintf("file-upload:%x", hash)
	submitKey := fmt.Sprintf("submit-file:%x", hash)
	if old := p.state.Outcomes[submitKey]; old.Status != "" {
		if old.Status == "succeeded" {
			sub, e := p.submission(ctx)
			if e != nil {
				return e
			}
			return emit(map[string]any{"status": "already_submitted", "file_id": p.state.Outcomes[key].ID, "submission": submissionShape(sub)})
		}
		return fmt.Errorf("file submission previously %s; inspect live state before another write", old.Status)
	}
	contentType := mime.TypeByExtension(filepath.Ext(name))
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	fileID := p.state.Outcomes[key].ID
	if old := p.state.Outcomes[key]; old.Status != "" && (old.Status != "succeeded" || fileID <= 0) {
		return fmt.Errorf("file upload previously %s; unchanged failed or uncertain uploads are not replayed", old.Status)
	}
	if fileID <= 0 {
		if e = p.begin(key); e != nil {
			return e
		}
		var directive probeUploadDirective
		endpoint := p.submissionPath() + "/files?as_user_id=" + strconv.FormatInt(p.state.TestStudentID, 10)
		e = p.c.Write(ctx, "POST", endpoint, map[string]any{"name": name, "size": len(data), "content_type": contentType, "on_duplicate": "rename"}, &directive)
		if e == nil {
			var location string
			client := &http.Client{Timeout: 60 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
			fileID, location, e = sendProbeFile(ctx, client, directive, name, contentType, data)
			if e == nil && location != "" {
				var attachment workflow.Attachment
				var finalPath string
				finalPath, e = probeFinalizePath(location, p.state.TestStudentID)
				if e == nil {
					// The redirect identifies the newly created file even if
					// confirmation subsequently fails. Retain that ID for review.
					finalURL, _ := url.Parse(finalPath)
					fileID, _ = strconv.ParseInt(strings.Split(finalURL.Path, "/")[4], 10, 64)
					e = p.c.Get(ctx, finalPath, &attachment)
					if e == nil && (attachment.Size != int64(len(data)) || attachment.ID != fileID) {
						e = &canvas.Error{Kind: "uncertain", Uncertain: true}
					}
				}
				if e != nil {
					// Storage accepted bytes; a failed confirmation cannot justify
					// creating a fresh upload and losing the pending file identity.
					e = &canvas.Error{Kind: "uncertain", Uncertain: true}
				}
			}
		}
		if e == nil && fileID <= 0 {
			e = &canvas.Error{Kind: "uncertain", Uncertain: true}
		}
		if e = p.finish(key, e, fileID); e != nil {
			return e
		}
	}
	// Recheck both identities after storage work, before making the submission.
	if _, e = p.assignment(ctx); e != nil {
		return e
	}
	if e = p.target(ctx); e != nil {
		return e
	}
	if e = p.begin(submitKey); e != nil {
		return e
	}
	var sub workflow.Submission
	e = p.c.Write(ctx, "POST", fmt.Sprintf("/api/v1/courses/%d/assignments/%d/submissions?as_user_id=%d", courseID, p.state.AssignmentID, p.state.TestStudentID), map[string]any{"submission": map[string]any{"submission_type": "online_upload", "file_ids": []int64{fileID}}}, &sub)
	attached := false
	for _, attachment := range sub.Attachments {
		attached = attached || attachment.ID == fileID
	}
	if e == nil && (sub.UserID != p.state.TestStudentID || sub.AssignmentID != p.state.AssignmentID || sub.Attempt < 1 || sub.Type != "online_upload" || !attached) {
		e = &canvas.Error{Kind: "uncertain", Uncertain: true}
	}
	if e = p.finish(submitKey, e, sub.ID); e != nil {
		return e
	}
	return emit(map[string]any{"status": "succeeded", "file_id": fileID, "filename": name, "submission": submissionShape(sub)})
}

func sendProbeFile(ctx context.Context, client *http.Client, directive probeUploadDirective, name, contentType string, data []byte) (int64, string, error) {
	u, e := url.Parse(directive.URL)
	if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" || u.Opaque != "" || directive.Params == nil {
		return 0, "", &canvas.Error{Kind: "uncertain", Uncertain: true}
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	keys := make([]string, 0, len(directive.Params))
	for key := range directive.Params {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		var value string
		if key == "file" || json.Unmarshal(directive.Params[key], &value) != nil {
			return 0, "", &canvas.Error{Kind: "uncertain", Uncertain: true}
		}
		if e = writer.WriteField(key, value); e != nil {
			return 0, "", &canvas.Error{Kind: "uncertain", Uncertain: true}
		}
	}
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{"name": "file", "filename": name}))
	header.Set("Content-Type", contentType)
	part, e := writer.CreatePart(header)
	if e != nil {
		return 0, "", &canvas.Error{Kind: "uncertain", Uncertain: true}
	}
	if _, e = part.Write(data); e != nil {
		return 0, "", &canvas.Error{Kind: "uncertain", Uncertain: true}
	}
	if e = writer.Close(); e != nil {
		return 0, "", &canvas.Error{Kind: "uncertain", Uncertain: true}
	}
	req, e := http.NewRequestWithContext(ctx, "POST", u.String(), &body)
	if e != nil {
		return 0, "", &canvas.Error{Kind: "uncertain", Uncertain: true}
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, e := client.Do(req)
	if e != nil {
		return 0, "", &canvas.Error{Kind: "uncertain", Uncertain: true}
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 && resp.StatusCode < 400 || resp.StatusCode == http.StatusCreated {
		if location := resp.Header.Get("Location"); location != "" {
			ref, e := url.Parse(location)
			if e == nil {
				return 0, u.ResolveReference(ref).String(), nil
			}
			return 0, "", &canvas.Error{Kind: "uncertain", Uncertain: true}
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		kind := "uncertain"
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			kind = "forbidden"
		}
		return 0, "", &canvas.Error{Kind: kind, Status: resp.StatusCode, Uncertain: kind == "uncertain"}
	}
	var attachment workflow.Attachment
	b, e := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if e != nil || json.Unmarshal(b, &attachment) != nil || attachment.ID <= 0 || attachment.Size != int64(len(data)) {
		return 0, "", &canvas.Error{Kind: "uncertain", Uncertain: true}
	}
	return attachment.ID, "", nil
}

func probeFinalizePath(raw string, studentID int64) (string, error) {
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "https" || u.Host != strings.TrimPrefix(allowedOrigin, "https://") || u.User != nil || u.Fragment != "" || u.Opaque != "" {
		return "", &canvas.Error{Kind: "validation"}
	}
	parts := strings.Split(u.Path, "/")
	if (len(parts) != 5 && len(parts) != 6) || parts[0] != "" || parts[1] != "api" || parts[2] != "v1" || parts[3] != "files" || len(parts) == 6 && parts[5] != "create_success" {
		return "", &canvas.Error{Kind: "validation"}
	}
	id, e := strconv.ParseInt(parts[4], 10, 64)
	if e != nil || id <= 0 || studentID <= 0 {
		return "", &canvas.Error{Kind: "validation"}
	}
	query, e := url.ParseQuery(u.RawQuery)
	if e != nil || len(query["as_user_id"]) > 1 || query.Get("as_user_id") != "" && query.Get("as_user_id") != strconv.FormatInt(studentID, 10) {
		return "", &canvas.Error{Kind: "validation"}
	}
	query.Set("as_user_id", strconv.FormatInt(studentID, 10))
	u.RawQuery = query.Encode()
	return u.String(), nil
}
