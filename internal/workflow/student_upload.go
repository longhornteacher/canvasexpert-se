package workflow

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"canvasexpertse/internal/canvas"
	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/render"
	"canvasexpertse/internal/workspace"
)

// maxStudentUpload: each student upload must be strictly smaller than this
// (fewer than 150,000,000 bytes). Every check uses this one boundary; tests
// lower it.
var maxStudentUpload int64 = 150_000_000

const studentUploadLimitText = "150,000,000 bytes"

// maxTextPreview bounds the optional text preview of a saved text upload.
// Larger text files stay downloaded originals to open directly.
var maxTextPreview int64 = 4 << 20

// previewSlot runs one text preview at a time, so concurrent downloads of
// large text files never hold several full copies in memory together.
var previewSlot = make(chan struct{}, 1)

func uploadName(a Attachment) string {
	if a.Filename != "" {
		return a.Filename
	}
	return a.DisplayName
}

func unavailable(why string) string { return "file unavailable: " + why }

// saveStudentUpload saves one student upload's original bytes at rel
// (workspace-relative). Every failure is this file's alone: it gets status
// failed, a specific reason and no path, and no partial file is left behind.
func (s *Service) saveStudentUpload(ctx context.Context, rel string, a Attachment) domain.Attachment {
	item := domain.Attachment{ID: a.ID, Name: uploadName(a), ContentType: a.ContentType, UpdatedAt: a.UpdatedAt, Status: "failed"}
	if a.Size > 0 {
		item.Size = a.Size
	}
	if a.ID <= 0 {
		item.Reason = unavailable("Canvas did not return the file's ID, so the original was not downloaded")
		return item
	}
	return s.saveStudentFile(ctx, rel, item, a.URL)
}

// saveStudentFile downloads one piece of student evidence described by item
// (Name, ContentType and Canvas's Size when known) from url to rel. It applies
// the shared size cap, temp-file/rename and failure rules.
func (s *Service) saveStudentFile(ctx context.Context, rel string, item domain.Attachment, url string) domain.Attachment {
	fail := func(why string) domain.Attachment {
		item.Path, item.ReadablePath, item.Status, item.Reason = "", "", "failed", unavailable(why)
		return item
	}
	switch {
	case url == "":
		return fail("Canvas did not return a download address for it")
	case item.Size >= maxStudentUpload:
		return fail(fmt.Sprintf("Canvas reports %d bytes; files must be under %s. Open it in SpeedGrader", item.Size, studentUploadLimitText))
	}
	path, err := workspace.ConstrainPath(s.Root, rel)
	if err != nil {
		return fail("no safe place in UserInstall to save it")
	}
	if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fail("its folder could not be created in UserInstall")
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".cse-download-*")
	if err != nil {
		return fail("a temporary file could not be created in UserInstall")
	}
	tmp := f.Name()
	n, err := s.Client.DownloadLimited(ctx, url, f, maxStudentUpload-1)
	if closeErr := f.Close(); err == nil && closeErr != nil {
		err = &canvas.Error{Kind: "local"}
	}
	if err != nil {
		_ = os.Remove(tmp)
		return fail(downloadFailure(err))
	}
	if err = os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fail("the original could not be saved in UserInstall")
	}
	item.Path, item.Size, item.Status, item.Reason = filepath.ToSlash(rel), n, "downloaded", ""
	if strings.HasPrefix(item.ContentType, "text/") || plainTextExtension(item.Name) {
		s.textPreview(&item, path, rel, item.ContentType)
	}
	return item
}

// textPreview adds a cheap UTF-8 text sidecar. Any failure leaves the item a
// downloaded original; the extension list only chooses whether to try.
func (s *Service) textPreview(item *domain.Attachment, path, rel, contentType string) {
	if item.Size > maxTextPreview {
		item.Reason = "no text preview for a text file this large; open the original"
		return
	}
	previewSlot <- struct{}{}
	defer func() { <-previewSlot }()
	f, err := os.Open(path)
	if err != nil {
		return
	}
	b, err := io.ReadAll(io.LimitReader(f, maxTextPreview+1))
	f.Close()
	if err != nil || int64(len(b)) > maxTextPreview {
		return
	}
	if !utf8.Valid(b) || strings.ContainsRune(string(b), 0) {
		item.Reason = "not UTF-8 text, so no text preview; open the original"
		return
	}
	readable := string(b)
	b = nil
	if strings.Contains(contentType, "html") || strings.EqualFold(filepath.Ext(item.Name), ".html") || strings.EqualFold(filepath.Ext(item.Name), ".htm") {
		readable = render.HTMLText(readable)
	}
	textRel := rel + ".readable.txt"
	textPath, err := workspace.ConstrainPath(s.Root, textRel)
	if err != nil || workspace.WriteFile(textPath, []byte(readable), 0644) != nil {
		return
	}
	item.ReadablePath, item.Status, item.Reason = filepath.ToSlash(textRel), "readable", ""
}

// downloadFailure names why a retrieval failed, keeping a size rejection
// apart from a network failure.
func downloadFailure(err error) string {
	var ce *canvas.Error
	if errors.As(err, &ce) {
		switch ce.Kind {
		case "too_large":
			return "it is " + studentUploadLimitText + " or larger, the download limit. Open it in SpeedGrader"
		case "local":
			return "the download could not be written in UserInstall (disk full or no permission)"
		case "canceled":
			return "the pull stopped before this download finished"
		case "not_found":
			return "Canvas no longer has this file"
		case "forbidden", "authentication":
			return "Canvas denied access to the file"
		case "throttled":
			return "Canvas rate limit reached; pull again later"
		case "transport":
			return "the network connection failed or stalled before the download finished"
		}
		if ce.Status > 0 {
			return fmt.Sprintf("Canvas answered HTTP %d", ce.Status)
		}
	}
	return err.Error()
}

// fileAvailability counts saved originals to inspect and unavailable files
// in one attempt. A legacy unreadable row counts as an original only when it
// is still a regular file inside UserInstall (root empty skips that check).
func fileAvailability(root string, w domain.AttemptWork) (originals, missing int) {
	exists := func(rel string) bool {
		if root == "" {
			return true
		}
		path, err := workspace.ConstrainPath(root, filepath.FromSlash(rel))
		if err != nil {
			return false
		}
		info, err := os.Lstat(path)
		return err == nil && info.Mode().IsRegular()
	}
	for _, a := range w.Attachments {
		if a.HasOriginal() && (a.Status != "unreadable" || exists(a.Path)) {
			originals++
		} else if a.Source != "embedded" {
			// An unavailable file referenced in the answer text is reported on
			// its own row; it does not add a hold hint.
			missing++
		}
	}
	for _, f := range w.WrittenFiles {
		if f.HasOriginal() {
			originals++
		} else if f.Status != "" {
			missing++
		}
	}
	return originals, missing
}
