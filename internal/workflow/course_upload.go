package workflow

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"canvasexpertse/internal/canvas"
	"canvasexpertse/internal/workspace"
)

type CourseUploadOptions struct {
	FolderID    int64
	Access      string // required: course, hidden (links still work), or locked
	ReceiptPath string
	NewUpload   bool // explicit fresh upload; never replaces a prior receipt/file
}

type CourseUploadReceipt struct {
	Schema            int         `json:"schema"`
	Kind              string      `json:"kind"`
	Instance          string      `json:"instance"`
	Course            string      `json:"course"`
	CourseID          int64       `json:"course_id"`
	SourcePath        string      `json:"source_path"`
	SHA256            string      `json:"sha256"`
	SourceSize        int64       `json:"source_size"`
	RequestedFolderID int64       `json:"requested_folder_id,omitempty"`
	Access            string      `json:"requested_access"`
	Status            string      `json:"status"`
	File              canvas.File `json:"file"`
	URL               string      `json:"course_file_url,omitempty"`
	APIPath           string      `json:"api_path,omitempty"`
	Message           string      `json:"message,omitempty"`
}

type CourseUploadReport struct {
	Path, Status, Message string
	Receipt               CourseUploadReceipt
}

type courseAssetFolder struct {
	ID          int64  `json:"id"`
	ContextID   int64  `json:"context_id"`
	ContextType string `json:"context_type"`
	Locked      bool   `json:"locked"`
	LockAt      string `json:"lock_at"`
	UnlockAt    string `json:"unlock_at"`
}

func (s *Service) assetFolder(ctx context.Context, courseID, folderID int64) (courseAssetFolder, error) {
	var folder courseAssetFolder
	err := s.Client.Get(ctx, fmt.Sprintf("/api/v1/courses/%d/folders/%d", courseID, folderID), &folder)
	if err == nil && (folder.ID != folderID || folder.ContextID != courseID || folder.ContextType != "Course") {
		err = fmt.Errorf("file folder does not belong to the selected course")
	}
	return folder, err
}

func fileAccessMatches(file canvas.File, access string) bool {
	return file.Locked != nil && file.Hidden != nil && *file.Locked == (access == "locked") && *file.Hidden == (access != "course") && file.VisibilityLevel == "course" && file.LockAt == "" && file.UnlockAt == ""
}

func fileUploadReady(file canvas.File) bool {
	return file.UploadStatus == "" || file.UploadStatus == "success" || file.UploadStatus == "available"
}

func (s *Service) readCourseAsset(ctx context.Context, courseID, id int64) (canvas.File, error) {
	var file canvas.File
	err := s.Client.Get(ctx, fmt.Sprintf("/api/v1/courses/%d/files/%d", courseID, id), &file)
	if err != nil {
		return file, err
	}
	if file.ID != id || file.FolderID <= 0 {
		return file, fmt.Errorf("returned file identity is not the requested course asset")
	}
	_, err = s.assetFolder(ctx, courseID, file.FolderID)
	return file, err
}

// UploadCourseFile resolves the configured course and preserves a small asset
// mapping beside the source. A prior ambiguous transfer never causes a replay.
func (s *Service) UploadCourseFile(ctx context.Context, nickname, sourcePath string, opts CourseUploadOptions) (report CourseUploadReport, err error) {
	if opts.Access != "course" && opts.Access != "hidden" && opts.Access != "locked" {
		return report, fmt.Errorf("upload requires explicit access: course, hidden (accessible through links), or locked")
	}
	if opts.FolderID < 0 {
		return report, fmt.Errorf("folder ID must be positive")
	}
	course, err := s.Course(nickname)
	if err != nil {
		return report, err
	}
	sourcePath, err = filepath.Abs(sourcePath)
	if err != nil {
		return report, err
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		return report, fmt.Errorf("cannot open source file: %w", err)
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return report, fmt.Errorf("upload source must be a regular file")
	}
	hash := sha256.New()
	if _, err = io.Copy(hash, source); err != nil {
		return report, fmt.Errorf("cannot read source file: %w", err)
	}
	digest := hex.EncodeToString(hash.Sum(nil))
	if _, err = source.Seek(0, io.SeekStart); err != nil {
		return report, err
	}
	path := opts.ReceiptPath
	if path == "" {
		path = sourcePath + ".cse-upload-" + Slug(course.Nickname) + ".json"
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return report, err
	}
	if target, e := os.Stat(path); path == sourcePath || e == nil && os.SameFile(info, target) {
		return report, fmt.Errorf("receipt path must differ from the source file")
	}
	var rec CourseUploadReceipt
	data, readErr := os.ReadFile(path)
	if readErr != nil && !os.IsNotExist(readErr) {
		return report, readErr
	}
	if readErr == nil && !opts.NewUpload {
		if json.Unmarshal(data, &rec) != nil || rec.Schema != 1 || rec.Kind != "course-file" {
			return report, fmt.Errorf("upload receipt is invalid; preserve it and use --new-upload for an intentional fresh upload")
		}
		validStage := rec.Status == "pending" || rec.Status == "uncertain" || rec.Status == "failed" || (rec.Status == "uploaded" || rec.Status == "access-pending" || rec.Status == "completed") && rec.File.ID > 0
		if !validStage || rec.File.ID < 0 {
			return report, fmt.Errorf("upload receipt has an invalid outcome/ID; inspect it before a deliberate new upload")
		}
		if rec.Instance != s.Client.Origin() || rec.CourseID != course.ID || rec.SourcePath != sourcePath || rec.RequestedFolderID != opts.FolderID || rec.Access != opts.Access {
			return report, fmt.Errorf("receipt destination, source or access differs; use --new-upload for an intentional fresh upload")
		}
		if rec.SHA256 != digest || rec.SourceSize != info.Size() {
			return report, fmt.Errorf("source bytes changed; use --new-upload to create another file and preserve the existing Canvas file")
		}
	} else {
		if readErr == nil {
			// Preserve every prior mapping, including an uncertain outcome, when
			// the teacher deliberately requests another upload.
			path = strings.TrimSuffix(path, filepath.Ext(path)) + ".new-" + time.Now().UTC().Format("20060102T150405.000000000") + ".json"
		}
		rec = CourseUploadReceipt{Schema: 1, Kind: "course-file", Instance: s.Client.Origin(), Course: course.Nickname, CourseID: course.ID, SourcePath: sourcePath, SHA256: digest, SourceSize: info.Size(), RequestedFolderID: opts.FolderID, Access: opts.Access}
	}
	report.Path = path
	defer func() {
		report.Receipt = rec
		if report.Status == "" {
			report.Status = rec.Status
		}
		report.Message = rec.Message
	}()
	save := func() error {
		if rec.File.ID > 0 {
			rec.APIPath = fmt.Sprintf("/api/v1/courses/%d/files/%d", course.ID, rec.File.ID)
			rec.URL = fmt.Sprintf("%s/courses/%d/files/%d", s.Client.Origin(), course.ID, rec.File.ID)
		}
		b, e := json.MarshalIndent(rec, "", "  ")
		if e != nil {
			return e
		}
		return workspace.WriteFile(path, append(b, '\n'), 0600)
	}
	hold := func(message string) error {
		report.Status = "held"
		rec.Message = message
		if e := save(); e != nil {
			return e
		}
		return fmt.Errorf("%s", message)
	}
	if rec.Status == "pending" || rec.Status == "uncertain" {
		// An unclear upload is resolved by looking for the same file in the course;
		// if it is not there, the upload simply runs again.
		if rec.File.ID == 0 {
			var found []canvas.File
			name := filepath.Base(sourcePath)
			if e := s.Client.List(ctx, fmt.Sprintf("/api/v1/courses/%d/files?search_term=%s&per_page=100", course.ID, url.QueryEscape(name)), &found); e == nil {
				matches := []canvas.File{}
				for _, f := range found {
					if (f.Filename == name || f.DisplayName == name) && f.Size == info.Size() && (opts.FolderID == 0 || f.FolderID == opts.FolderID) {
						matches = append(matches, f)
					}
				}
				if len(matches) == 1 {
					rec.File = matches[0]
				}
			}
		}
		rec.Status, rec.Message = "", ""
	}
	if opts.FolderID > 0 {
		folder, e := s.assetFolder(ctx, course.ID, opts.FolderID)
		if e != nil {
			if rec.File.ID > 0 {
				report.Status = "held"
				rec.Message = e.Error()
			}
			return report, e
		}
		if opts.Access == "locked" && (!folder.Locked || folder.LockAt != "" || folder.UnlockAt != "") {
			return report, fmt.Errorf("locked uploads require an existing locked course folder without scheduled unlocking")
		}
	} else if opts.Access == "locked" {
		return report, fmt.Errorf("locked uploads require --folder with an existing locked course folder to prevent exposure during transfer")
	}
	if rec.File.ID == 0 {
		if rec.Status == "failed" {
			return report, fmt.Errorf("previous upload was rejected; correct the cause and use --new-upload for a deliberate retry")
		}
		rec.Status, rec.Message = "pending", "upload may begin after receipt persistence"
		if err = save(); err != nil {
			return report, err
		}
		name := filepath.Base(sourcePath)
		contentType := mime.TypeByExtension(filepath.Ext(name))
		if contentType == "" {
			buf := make([]byte, 512)
			n, _ := source.Read(buf)
			contentType = http.DetectContentType(buf[:n])
			if _, err = source.Seek(0, io.SeekStart); err != nil {
				return report, err
			}
		}
		hash.Reset()
		file, e := s.Client.UploadCourseFile(ctx, canvas.CourseFileUpload{CourseID: course.ID, FolderID: opts.FolderID, Name: name, ContentType: contentType, Size: info.Size(), Source: io.TeeReader(source, hash), Known: func(f canvas.File) error { rec.File = f; return save() }})
		if file.ID > 0 {
			rec.File = file
		}
		if e != nil {
			rec.Status = "failed"
			var ce *canvas.Error
			if errors.As(e, &ce) && ce.Uncertain || rec.File.ID > 0 {
				rec.Status = "uncertain"
			}
			rec.Message = e.Error()
			if se := save(); se != nil {
				return report, se
			}
			return report, e
		}
		rec.Status, rec.Message = "uploaded", "uploaded ID saved; access settings still need verification"
		if err = save(); err != nil {
			return report, err
		}
	}
	live, err := s.readCourseAsset(ctx, course.ID, rec.File.ID)
	if err != nil {
		err = hold("known file could not be verified in the selected course: " + err.Error())
		return report, err
	}
	if rec.Status == "completed" {
		// A finished upload is reported as Canvas has it now; changes made in
		// Canvas since are the teacher's and are left alone.
		rec.File, rec.Message = live, "verified existing file; no upload or access write"
		if live.Size != rec.SourceSize || !fileAccessMatches(live, rec.Access) || !fileUploadReady(live) {
			rec.Message = "existing file reported as it is in Canvas now; its size, folder or access differs from the receipt and was left alone"
		}
		return report, save()
	}
	if live.Size != rec.SourceSize || rec.RequestedFolderID > 0 && live.FolderID != rec.RequestedFolderID {
		err = hold("the file Canvas has does not match this upload's size or folder; check the course Files, and use --new-upload for a fresh copy")
		return report, err
	}
	if !fileAccessMatches(live, rec.Access) {
		rec.Status = "access-pending"
		if err = save(); err != nil {
			return report, err
		}
		var updated canvas.File
		err = s.Client.Write(ctx, "PUT", fmt.Sprintf("/api/v1/files/%d", rec.File.ID), map[string]any{"locked": rec.Access == "locked", "hidden": rec.Access != "course", "visibility_level": "course", "lock_at": nil, "unlock_at": nil}, &updated)
		if err != nil {
			rec.Message = "uploaded file exists; access settings not confirmed: " + err.Error()
			if e := save(); e != nil {
				return report, e
			}
			return report, err
		}
		live, err = s.readCourseAsset(ctx, course.ID, rec.File.ID)
		if err != nil {
			return report, err
		}
	}
	if !fileAccessMatches(live, rec.Access) || !fileUploadReady(live) {
		err = hold("uploaded file exists; Canvas access/availability not yet confirmed; inspect the saved settings")
		return report, err
	}
	rec.File, rec.Status = live, "completed"
	rec.Message = "uploaded and course file access verified; browser/student access remains to be checked"
	return report, save()
}
