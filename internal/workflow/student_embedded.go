package workflow

import (
	"context"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
	"golang.org/x/net/html"
)

// maxEmbeddedUploads bounds the Canvas files downloaded from one student
// body (text entry, group submission body or discussion entry). Further
// references are listed as not downloaded.
const maxEmbeddedUploads = 20

// embedScope names the Canvas contexts whose files a student's own rich text
// may embed: this course, the submitting students' own files and the
// submission's group. A reference that names any other course, user or
// group is never fetched.
type embedScope struct {
	courseID int64
	users    map[int64]bool
	groupID  int64
}

func userScope(courseID int64, users ...int64) embedScope {
	scope := embedScope{courseID: courseID, users: map[int64]bool{}}
	for _, id := range users {
		if id > 0 {
			scope.users[id] = true
		}
	}
	return scope
}

type embeddedRef struct {
	id       int64
	verifier string
}

var (
	embeddedFilePath  = regexp.MustCompile(`^/(?:api/v1/)?(?:(courses|users|groups)/([0-9]+)/)?files/([0-9]+)(?:/.*)?$`)
	embeddedMediaPath = regexp.MustCompile(`^/media_attachments_iframe/([0-9]+)/?$`)
)

// embeddedFileRefs finds Canvas file references on the course's own Canvas
// origin in student HTML (src, href, data-api-endpoint, poster), in document
// order, once per file. Links to anywhere else are left as links.
func embeddedFileRefs(body, origin string, scope embedScope) []embeddedRef {
	if !strings.Contains(body, "files/") && !strings.Contains(body, "media_attachments_iframe") {
		return nil
	}
	base, err := url.Parse(origin)
	if err != nil {
		return nil
	}
	nodes, err := html.ParseFragment(strings.NewReader(body), nil)
	if err != nil {
		return nil
	}
	var refs []embeddedRef
	seen := map[int64]bool{}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		for _, a := range n.Attr {
			if a.Key != "src" && a.Key != "href" && a.Key != "data-api-endpoint" && a.Key != "poster" {
				continue
			}
			if ref, ok := canvasFileRef(strings.TrimSpace(a.Val), base, scope); ok && !seen[ref.id] {
				seen[ref.id] = true
				refs = append(refs, ref)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	for _, n := range nodes {
		walk(n)
	}
	return refs
}

func canvasFileRef(value string, base *url.URL, scope embedScope) (embeddedRef, bool) {
	u, err := url.Parse(value)
	if err != nil || u.User != nil || u.Opaque != "" {
		return embeddedRef{}, false
	}
	if u.Scheme != "" || u.Host != "" {
		if !strings.EqualFold(u.Scheme, base.Scheme) || !strings.EqualFold(u.Host, base.Host) {
			return embeddedRef{}, false
		}
	} else if !strings.HasPrefix(u.Path, "/") {
		return embeddedRef{}, false
	}
	var raw string
	if m := embeddedFilePath.FindStringSubmatch(u.Path); m != nil {
		if m[1] != "" {
			owner, _ := strconv.ParseInt(m[2], 10, 64)
			switch m[1] {
			case "courses":
				if owner != scope.courseID {
					return embeddedRef{}, false
				}
			case "users":
				if !scope.users[owner] {
					return embeddedRef{}, false
				}
			case "groups":
				if scope.groupID <= 0 || owner != scope.groupID {
					return embeddedRef{}, false
				}
			}
		}
		raw = m[3]
	} else if m := embeddedMediaPath.FindStringSubmatch(u.Path); m != nil {
		raw = m[1]
	} else {
		return embeddedRef{}, false
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return embeddedRef{}, false
	}
	return embeddedRef{id: id, verifier: u.Query().Get("verifier")}, true
}

// embeddedUploads saves each Canvas file referenced in one student body into
// dir (workspace-relative), source embedded. skip holds file IDs already saved
// as that work's ordinary uploads; cache, when given, saves a file ID once.
func (s *Service) embeddedUploads(ctx context.Context, dir, body string, scope embedScope, skip map[int64]bool, cache map[int64]domain.Attachment) []domain.Attachment {
	var items []domain.Attachment
	count := 0
	for _, ref := range embeddedFileRefs(body, s.Client.Origin(), scope) {
		if skip[ref.id] {
			continue
		}
		count++
		var item domain.Attachment
		if cached, ok := cache[ref.id]; ok {
			item = cached
		} else if count > maxEmbeddedUploads {
			item = domain.Attachment{ID: ref.id, Name: strconv.FormatInt(ref.id, 10), Status: "not-downloaded", Reason: fmt.Sprintf("more than %d Canvas files in one answer; open the rest in SpeedGrader", maxEmbeddedUploads)}
		} else {
			item = s.embeddedUpload(ctx, dir, ref)
			if cache != nil {
				cache[ref.id] = item
			}
		}
		item.Source = "embedded"
		items = append(items, item)
	}
	return items
}

func (s *Service) embeddedUpload(ctx context.Context, dir string, ref embeddedRef) domain.Attachment {
	path := fmt.Sprintf("/api/v1/files/%d", ref.id)
	if ref.verifier != "" {
		path += "?verifier=" + url.QueryEscape(ref.verifier)
	}
	var file Attachment
	err := s.Client.Get(ctx, path, &file)
	if err == nil && file.ID == ref.id {
		rel := filepath.Join(dir, fmt.Sprintf("%d-%s", file.ID, workspace.SafeFilename(uploadName(file))))
		return s.saveStudentUpload(ctx, rel, file)
	}
	if ref.verifier == "" {
		why := "Canvas did not return the file's details"
		if err != nil {
			why += ": " + downloadFailure(err)
		}
		return domain.Attachment{ID: ref.id, Name: strconv.FormatInt(ref.id, 10), Status: "failed", Reason: unavailable(why + ". Open the answer in SpeedGrader")}
	}
	// A file in the student's own Canvas files is usually readable only through
	// the verifier Canvas wrote into the student's HTML. Use that same-origin
	// download address; the type comes from the saved bytes.
	name := fmt.Sprintf("embedded-%d", ref.id)
	download := fmt.Sprintf("%s/files/%d/download?download_frd=1&verifier=%s", strings.TrimRight(s.Client.Origin(), "/"), ref.id, url.QueryEscape(ref.verifier))
	item := s.saveStudentFile(ctx, filepath.Join(dir, name), domain.Attachment{ID: ref.id, Name: name, Status: "failed"}, download)
	if item.Status == "downloaded" {
		nameBySniffing(s.Root, &item)
	}
	return item
}

// nameBySniffing gives a saved file without a known type a content type and
// extension from its first bytes, so image tools can open it.
func nameBySniffing(root string, item *domain.Attachment) {
	path, err := workspace.ConstrainPath(root, filepath.FromSlash(item.Path))
	if err != nil {
		return
	}
	f, err := os.Open(path)
	if err != nil {
		return
	}
	head := make([]byte, 512)
	n, _ := f.Read(head)
	f.Close()
	item.ContentType = strings.TrimSpace(strings.SplitN(http.DetectContentType(head[:n]), ";", 2)[0])
	exts, _ := mime.ExtensionsByType(item.ContentType)
	if len(exts) == 0 || item.ContentType == "application/octet-stream" {
		return
	}
	if os.Rename(path, path+exts[0]) == nil {
		item.Path += exts[0]
		item.Name += exts[0]
	}
}

// mediaRecording saves a Canvas media recording submission when Canvas
// returned a direct download address on the course's own Canvas origin.
// Anything else (a player only, another host) stays a link.
func (s *Service) mediaRecording(ctx context.Context, dir string, sub Submission) *domain.Attachment {
	if sub.Type != "media_recording" || sub.MediaComment == nil || sub.MediaComment.URL == "" {
		return nil
	}
	u, err := url.Parse(sub.MediaComment.URL)
	base, baseErr := url.Parse(s.Client.Origin())
	if err != nil || baseErr != nil || !strings.EqualFold(u.Scheme, base.Scheme) || !strings.EqualFold(u.Host, base.Host) || u.User != nil {
		return nil
	}
	m := sub.MediaComment
	name := workspace.SafeFilename(m.DisplayName)
	if name == "" || name == "." {
		name = "recording"
	}
	if filepath.Ext(name) == "" {
		if exts, _ := mime.ExtensionsByType(m.ContentType); len(exts) > 0 {
			name += exts[0]
		}
	}
	item := s.saveStudentFile(ctx, filepath.Join(dir, "recording-"+name), domain.Attachment{Name: name, ContentType: m.ContentType, Status: "failed"}, sub.MediaComment.URL)
	item.Source = "media"
	return &item
}

// commentEvidence copies a submission's comments. Files on a comment the
// student wrote are saved as supplemental evidence (source comment). Files on
// anyone else's comment, or on a comment whose author Canvas did not name,
// are listed but never downloaded and never count as the student's work.
func (s *Service) commentEvidence(ctx context.Context, relDir, ref string, sub Submission) []domain.ExistingComment {
	studentID := sub.UserID
	if studentID <= 0 {
		studentID = sub.User.ID
	}
	var out []domain.ExistingComment
	for _, c := range submissionComments(sub) {
		row := domain.ExistingComment{ID: c.ID, Attempt: c.Attempt, Text: c.Comment, CreatedAt: c.CreatedAt, ByStudent: studentID > 0 && c.AuthorID == studentID}
		for _, a := range c.Attachments {
			if !row.ByStudent {
				why := "attached by the teacher or another person, not this student; not student evidence"
				if c.AuthorID <= 0 {
					why = "Canvas did not say who attached it; not student evidence"
				}
				row.Attachments = append(row.Attachments, domain.Attachment{ID: a.ID, Name: uploadName(a), Source: "comment", Status: "not-downloaded", Reason: why})
				continue
			}
			rel := filepath.Join(relDir, "attachments", ref, "comments", fmt.Sprintf("comment-%d", c.ID), fmt.Sprintf("%d-%s", a.ID, workspace.SafeFilename(uploadName(a))))
			item := s.saveStudentUpload(ctx, rel, a)
			item.Source = "comment"
			row.Attachments = append(row.Attachments, item)
		}
		out = append(out, row)
	}
	return out
}
