package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"canvasexpertse/internal/canvas"
	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
	"golang.org/x/net/html"
)

type AssignmentCopyOptions struct {
	ModuleID          int64 `json:"module_id,omitempty"`
	AssignmentGroupID int64 `json:"assignment_group_id,omitempty"`
	Published         bool  `json:"published"`
}

type AssignmentCopyReport struct {
	Status, Message, ReceiptPath string
	AssignmentID                 int64
	URL                          string
	FilesCopied                  int
	Push                         PushReport
}

type assignmentCopyFile struct {
	ID     int64  `json:"id,omitempty"`
	Status string `json:"status"`
}

type assignmentCopyReceipt struct {
	Schema       int                          `json:"schema"`
	Instance     string                       `json:"instance"`
	SourceCourse int64                        `json:"source_course_id"`
	SourceID     int64                        `json:"source_assignment_id"`
	TargetCourse int64                        `json:"target_course_id"`
	Options      AssignmentCopyOptions        `json:"options"`
	Status       string                       `json:"status"`
	AssignmentID int64                        `json:"assignment_id,omitempty"`
	URL          string                       `json:"url,omitempty"`
	Files        map[int64]assignmentCopyFile `json:"files"`
	Document     domain.AssignmentDocument    `json:"-"`
	// The saved document freezes source content on the first run, so a resume
	// never replaces unsent work with later changes in the source course.
	DocumentPath string `json:"document_path"`
}

// CopyAssignment creates an independent assignment and copies its embedded
// Canvas files first. Existing create/rubric/module outcomes handle resumption;
// the copy receipt makes the source/destination pair stable across calls.
func (s *Service) CopyAssignment(ctx context.Context, sourceCourse, query, targetCourse string, opts AssignmentCopyOptions) (report AssignmentCopyReport, err error) {
	if opts.ModuleID < 0 || opts.AssignmentGroupID < 0 {
		return report, fmt.Errorf("module_id and assignment_group_id must be positive when supplied")
	}
	source, err := s.Course(sourceCourse)
	if err != nil {
		return report, err
	}
	target, err := s.Course(targetCourse)
	if err != nil {
		return report, err
	}
	if source.ID == target.ID {
		return report, fmt.Errorf("copy_assignment requires different source and target courses")
	}
	src, err := s.ForCourse(ctx, source)
	if err != nil {
		return report, err
	}
	dst, err := s.ForCourse(ctx, target)
	if err != nil {
		return report, err
	}
	a, err := FindAssignment(ctx, src.Client, source.ID, query)
	if err != nil {
		return report, err
	}
	if a.ID <= 0 || a.CourseID != source.ID {
		return report, fmt.Errorf("source assignment identity differs from the selected course")
	}
	dir := filepath.Join(s.Root, "courses", "_assignment-copies")
	if err = os.MkdirAll(dir, 0700); err != nil {
		return report, err
	}
	name := fmt.Sprintf("course-%d-assignment-%d-to-%d", source.ID, a.ID, target.ID)
	path := filepath.Join(dir, name+".json")
	report.ReceiptPath = path
	lock, err := os.OpenFile(path+".lock", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return report, fmt.Errorf("this assignment copy is already running; if CE-SE stopped mid-copy, inspect the receipt before removing %s", path+".lock")
	}
	defer func() {
		lock.Close()
		os.Remove(path + ".lock")
	}()
	rec := assignmentCopyReceipt{Schema: 1, Instance: dst.Client.Origin(), SourceCourse: source.ID, SourceID: a.ID, TargetCourse: target.ID, Options: opts, Files: map[int64]assignmentCopyFile{}}
	data, readErr := os.ReadFile(path)
	if readErr != nil && !os.IsNotExist(readErr) {
		return report, readErr
	}
	if readErr == nil {
		if err = json.Unmarshal(data, &rec); err != nil || rec.Schema != 1 || rec.Files == nil || rec.DocumentPath == "" {
			return report, fmt.Errorf("assignment copy receipt is unreadable; preserve it and inspect the existing copy before continuing")
		}
		if rec.Instance != dst.Client.Origin() || rec.SourceCourse != source.ID || rec.SourceID != a.ID || rec.TargetCourse != target.ID || rec.Options != opts {
			return report, fmt.Errorf("assignment copy receipt target or options differ; edit the existing copy through pull_assignment rather than creating another")
		}
		if rec.Status == "completed" {
			report.Status, report.Message, report.AssignmentID, report.URL, report.FilesCopied = "completed", "already copied; no writes made", rec.AssignmentID, rec.URL, len(rec.Files)
			return report, nil
		}
	} else {
		if reason := UnsupportedAssignment(a); reason != "" || a.NewQuiz || a.GroupCategoryID != nil && *a.GroupCategoryID > 0 || a.DiscussionTopicID != nil || a.DiscussionTopic != nil {
			return report, fmt.Errorf("copy_assignment supports ordinary individual assignments only; use the quiz/group/discussion authoring flow for this source (%s)", reason)
		}
		if _, owned, e := s.Owned(target); e != nil {
			return report, e
		} else if owned && domain.HasConsentTag(a.Name) {
			return report, fmt.Errorf("remove (CE) from the source title before copying into a teammate's course; that teacher adds their own scoring consent")
		}
		snapshot, e := Snapshot(a, src.Client.Origin())
		if e != nil {
			return report, e
		}
		doc := domain.AssignmentDocument{Schema: 1, Kind: "assignment", Operation: "create", Course: target.Nickname, Instance: dst.Client.Origin(), CourseID: target.ID, Title: a.Name, BodyFormat: "html", Points: snapshot.Points, SubmissionTypes: a.SubmissionTypes, DueAt: a.DueAt, UnlockAt: a.UnlockAt, LockAt: a.LockAt, Published: opts.Published, ModuleID: opts.ModuleID, Timezone: target.Timezone, Rubric: snapshot.Rubric}
		if opts.AssignmentGroupID > 0 {
			id := opts.AssignmentGroupID
			doc.AssignmentGroupID = &id
		}
		if doc.Rubric != nil {
			doc.Rubric.ID, doc.Rubric.AssociationID = 0, 0
			doc.Rubric.UseForGrading, doc.Rubric.UseForGradingKnown = false, true
		}
		if e = domain.ValidateAssignment(doc, a.Description); e != nil {
			return report, e
		}
		// Start directly at the stable pushed-record location. Push then leaves
		// the path unchanged, including if completion precedes receipt saving.
		rec.DocumentPath = filepath.Join("courses", target.Nickname, "pushed", name+".md")
		docPath, e := workspace.ConstrainPath(s.Root, rec.DocumentPath)
		if e != nil {
			return report, e
		}
		if _, e = os.Stat(docPath); !os.IsNotExist(e) {
			return report, fmt.Errorf("copy document already exists without its receipt; inspect %s before copying", docPath)
		}
		if e = workspace.WriteDocument(docPath, doc, a.Description); e != nil {
			return report, e
		}
	}
	save := func() error {
		b, e := json.MarshalIndent(rec, "", "  ")
		if e != nil {
			return e
		}
		return workspace.WriteFile(path, append(b, '\n'), 0600)
	}
	if err = save(); err != nil {
		return report, err
	}
	docPath, err := workspace.ConstrainPath(s.Root, rec.DocumentPath)
	if err != nil {
		return report, err
	}
	var doc domain.AssignmentDocument
	body, err := workspace.ReadDocument(docPath, &doc)
	if err != nil {
		return report, fmt.Errorf("saved copy document could not be read; preserve its receipt: %w", err)
	}
	if doc.CourseID != target.ID || doc.Instance != dst.Client.Origin() || doc.Course != target.Nickname || doc.Operation != "create" || doc.ModuleID != opts.ModuleID || doc.Published != opts.Published || (doc.AssignmentGroupID == nil && opts.AssignmentGroupID != 0) || doc.AssignmentGroupID != nil && *doc.AssignmentGroupID != opts.AssignmentGroupID {
		return report, fmt.Errorf("saved copy document identity/options differ from its receipt; inspect it before resuming")
	}
	if opts.ModuleID > 0 {
		m, e := dst.getModule(ctx, target.ID, opts.ModuleID)
		if e != nil || m.ID != opts.ModuleID || m.CourseID != 0 && m.CourseID != target.ID {
			return report, fmt.Errorf("target module is not verified in the target course")
		}
	}
	if opts.AssignmentGroupID > 0 {
		groups, e := AssignmentGroups(ctx, dst.Client, target.ID)
		if e != nil {
			return report, e
		}
		found := false
		for _, g := range groups {
			found = found || g.ID == opts.AssignmentGroupID
		}
		if !found {
			return report, fmt.Errorf("assignment_group_id %d is not in the target course", opts.AssignmentGroupID)
		}
	}
	if rec.Status != "prepared" && rec.Status != "creating" {
		// Discover every dependency before a write, then copy known IDs once.
		ids := map[int64]bool{}
		_, err = rewriteAssignmentCopyHTML(body, src.Client.Origin(), source.ID, target.ID, func(id int64) (int64, error) { ids[id] = true; return id, nil })
		if err != nil {
			return report, err
		}
		ordered := make([]int64, 0, len(ids))
		for id := range ids {
			ordered = append(ordered, id)
		}
		sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
		var folder courseAssetFolder
		if len(ordered) > 0 {
			if err = dst.Client.Get(ctx, fmt.Sprintf("/api/v1/courses/%d/folders/root", target.ID), &folder); err != nil {
				return report, err
			}
			if folder.ID <= 0 || folder.ContextID != target.ID || folder.ContextType != "Course" || folder.Locked || folder.LockAt != "" || folder.UnlockAt != "" {
				return report, fmt.Errorf("target root folder is not an accessible folder in the selected course")
			}
		}
		for _, id := range ordered {
			f := rec.Files[id]
			if f.ID == 0 {
				if f.Status == "pending" || f.Status == "uncertain" {
					return report, fmt.Errorf("file %d copy may already exist; inspect target Files and the receipt before continuing; no copy was replayed", id)
				}
				original, e := src.readCourseAsset(ctx, source.ID, id)
				if e != nil {
					return report, fmt.Errorf("source file %d is not verified in the source course: %w", id, e)
				}
				if original.Locked == nil || *original.Locked || !fileUploadReady(original) {
					return report, fmt.Errorf("source file %d is locked or unavailable; copy held before assignment creation", id)
				}
				rec.Files[id] = assignmentCopyFile{Status: "pending"}
				if err = save(); err != nil {
					return report, err
				}
				var copied canvas.File
				e = dst.Client.Write(ctx, "POST", fmt.Sprintf("/api/v1/folders/%d/copy_file", folder.ID), map[string]any{"source_file_id": strconv.FormatInt(id, 10), "on_duplicate": "rename"}, &copied)
				f = assignmentCopyFile{ID: copied.ID, Status: "copied"}
				if e != nil || copied.ID <= 0 {
					f.Status = "uncertain"
				}
				rec.Files[id] = f
				if err = save(); err != nil {
					return report, err
				}
				if e != nil {
					return report, fmt.Errorf("file %d could not be copied; assignment held: %w", id, e)
				}
				if copied.ID <= 0 {
					return report, fmt.Errorf("file %d copy did not return an ID; assignment held", id)
				}
			}
			if f.Status != "ready" {
				asset, e := dst.readCourseAsset(ctx, target.ID, f.ID)
				if e != nil {
					return report, fmt.Errorf("copied file %d is not verified in target course: %w", f.ID, e)
				}
				// Hidden files remain usable through the assignment link without
				// appearing in the course Files list before the teacher publishes.
				if !fileAccessMatches(asset, "hidden") {
					if e = dst.Client.WriteAccepted(ctx, "PUT", fmt.Sprintf("/api/v1/files/%d", f.ID), fileAccessPayload(asset, "hidden"), nil); e != nil {
						return report, e
					}
					asset, e = dst.readCourseAsset(ctx, target.ID, f.ID)
					if e != nil {
						return report, e
					}
				}
				if !fileAccessMatches(asset, "hidden") || !fileUploadReady(asset) {
					return report, fmt.Errorf("copied file %d access remains unconfirmed; assignment held", f.ID)
				}
				f.Status = "ready"
				rec.Files[id] = f
				if err = save(); err != nil {
					return report, err
				}
			}
		}
		body, err = rewriteAssignmentCopyHTML(body, src.Client.Origin(), source.ID, target.ID, func(id int64) (int64, error) {
			f := rec.Files[id]
			if f.ID <= 0 || f.Status != "ready" {
				return 0, fmt.Errorf("file %d is not ready", id)
			}
			return f.ID, nil
		})
		if err != nil {
			return report, err
		}
		if err = workspace.WriteDocument(docPath, doc, body); err != nil {
			return report, err
		}
		rec.Status = "prepared"
		if err = save(); err != nil {
			return report, err
		}
	}
	rec.Status = "creating"
	if err = save(); err != nil {
		return report, err
	}
	report.Push, err = s.Push(ctx, docPath)
	report.Status, report.Message, report.AssignmentID, report.URL, report.FilesCopied = report.Push.Status, report.Push.Message, report.Push.AssignmentID, report.Push.URL, len(rec.Files)
	rec.AssignmentID, rec.URL = report.AssignmentID, report.URL
	if report.Status == "completed" {
		live, e := GetAssignment(ctx, dst.Client, target.ID, report.AssignmentID)
		if e != nil {
			return report, e
		}
		if live.ID != report.AssignmentID || live.CourseID != target.ID || live.Published != opts.Published || opts.AssignmentGroupID > 0 && (live.AssignmentGroupID == nil || *live.AssignmentGroupID != opts.AssignmentGroupID) {
			report.Status, report.Message = "held", "copy exists but Canvas did not confirm its target, group or publication; inspect the known assignment before editing"
		} else {
			rec.Status = "completed"
		}
	}
	if e := save(); err == nil {
		err = e
	}
	return report, err
}

var assignmentCopyFilePath = regexp.MustCompile(`^/(?:api/v1/)?(?:courses/([0-9]+)/)?files/([0-9]+)(/.*)?$`)

// Rewrite only same-Canvas file references. Other course-scoped dependencies
// are held explicitly instead of creating a destination that students cannot use.
func rewriteAssignmentCopyHTML(body, origin string, source, target int64, resolve func(int64) (int64, error)) (string, error) {
	base, err := url.Parse(origin)
	if err != nil {
		return "", err
	}
	nodes, err := html.ParseFragment(strings.NewReader(body), nil)
	if err != nil {
		return "", err
	}
	var walk func(*html.Node) error
	walk = func(n *html.Node) error {
		for i := range n.Attr {
			a := &n.Attr[i]
			value := strings.TrimSpace(a.Val)
			u, e := url.Parse(value)
			if e != nil {
				continue
			}
			if u.Host != "" && (!strings.EqualFold(u.Host, base.Host) || u.Scheme != "" && u.Scheme != base.Scheme) {
				continue
			}
			if u.Scheme != "" && u.Scheme != base.Scheme {
				continue
			}
			m := assignmentCopyFilePath.FindStringSubmatch(u.Path)
			if m != nil {
				if a.Key != "src" && a.Key != "href" && a.Key != "data-api-endpoint" && a.Key != "poster" {
					return fmt.Errorf("Canvas file reference in %s requires an explicit supported link before copying", a.Key)
				}
				if m[1] != "" && m[1] != strconv.FormatInt(source, 10) {
					return fmt.Errorf("embedded file belongs to a different course; copy held")
				}
				id, _ := strconv.ParseInt(m[2], 10, 64)
				dest, e := resolve(id)
				if e != nil {
					return e
				}
				prefix := "/courses/"
				if strings.HasPrefix(u.Path, "/api/v1/") {
					prefix = "/api/v1/courses/"
				}
				u.Path = fmt.Sprintf("%s%d/files/%d%s", prefix, target, dest, m[3])
				u.RawPath = ""
				q := u.Query()
				q.Del("verifier")
				u.RawQuery = q.Encode()
				u.Scheme, u.Host, u.User = "", "", nil
				a.Val = u.String()
			} else if strings.Contains(value, fmt.Sprintf("/courses/%d/", source)) || strings.Contains(value, "/files/") && (a.Key == "srcset" || a.Key == "style") {
				return fmt.Errorf("source course link in %s cannot be copied automatically; replace it with an independent link first", a.Key)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if e := walk(c); e != nil {
				return e
			}
		}
		return nil
	}
	var out bytes.Buffer
	for _, n := range nodes {
		if err = walk(n); err != nil {
			return "", err
		}
		// ParseFragment(nil) supplies html/head/body wrappers; emit body content.
		if n.Type == html.ElementNode && n.Data == "html" {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Data == "body" {
					for child := c.FirstChild; child != nil; child = child.NextSibling {
						if err = html.Render(&out, child); err != nil {
							return "", err
						}
					}
				}
			}
		} else if err = html.Render(&out, n); err != nil {
			return "", err
		}
	}
	return out.String(), nil
}
