package workflow

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/render"
	"canvasexpertse/internal/workspace"
)

type NativePage struct {
	PageID    int64  `json:"page_id"`
	Slug      string `json:"url"`
	HTMLURL   string `json:"html_url"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	Published bool   `json:"published"`
	UpdatedAt string `json:"updated_at"`
}

type PagePushReport struct {
	Path, URL, Status, Message string
	PageID, ModuleItemID       int64
}

func nativePagePath(courseID, pageID int64) string {
	// Canvas interprets a bare number as a slug first. Force the numeric ID.
	return fmt.Sprintf("/api/v1/courses/%d/pages/page_id:%d", courseID, pageID)
}

func (s *Service) pageURL(courseID int64, p NativePage) string {
	if u, e := url.Parse(p.HTMLURL); e == nil && u.User == nil && u.RawQuery == "" && u.Fragment == "" && u.Scheme+"://"+u.Host == s.Client.Origin() && u.Path == fmt.Sprintf("/courses/%d/pages/%s", courseID, p.Slug) {
		return p.HTMLURL
	}
	// The API's url is the actual slug, never a slug guessed from its title.
	return s.Client.Origin() + fmt.Sprintf("/courses/%d/pages/", courseID) + url.PathEscape(p.Slug)
}

func (s *Service) getPage(ctx context.Context, courseID, pageID int64) (NativePage, error) {
	var p NativePage
	e := s.Client.Get(ctx, nativePagePath(courseID, pageID), &p)
	if e == nil && (p.PageID != pageID || p.Slug == "") {
		e = fmt.Errorf("Canvas page response did not confirm exact page identity")
	}
	return p, e
}

// PullPage writes a fresh file and never replaces the teacher's unsent edits.
func (s *Service) PullPage(ctx context.Context, nickname, query string) (string, error) {
	course, e := s.Course(nickname)
	if e != nil {
		return "", e
	}
	identifier := ""
	if id, err := strconv.ParseInt(query, 10, 64); err == nil && id > 0 {
		identifier = "page_id:" + strconv.FormatInt(id, 10)
	} else {
		u, err := url.Parse(query)
		if err != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Scheme+"://"+u.Host != s.Client.Origin() {
			return "", fmt.Errorf("pull-page requires a positive page ID or a Page link in this Canvas origin/course")
		}
		prefix := fmt.Sprintf("/courses/%d/pages/", course.ID)
		if !strings.HasPrefix(u.Path, prefix) {
			return "", fmt.Errorf("Page link belongs to a different course")
		}
		slug := strings.TrimPrefix(u.Path, prefix)
		if slug == "" || strings.Contains(slug, "/") {
			return "", fmt.Errorf("invalid Page link")
		}
		identifier = url.PathEscape(slug)
	}
	var p NativePage
	if e = s.Client.Get(ctx, fmt.Sprintf("/api/v1/courses/%d/pages/%s", course.ID, identifier), &p); e != nil {
		return "", e
	}
	if p.PageID <= 0 || p.Slug == "" {
		return "", fmt.Errorf("Canvas response lacks page identity")
	}
	if strings.HasPrefix(identifier, "page_id:") && identifier != fmt.Sprintf("page_id:%d", p.PageID) {
		return "", fmt.Errorf("Canvas returned a different Page ID")
	}
	return s.writePulledPage(course, p)
}

func (s *Service) writePulledPage(course domain.Course, p NativePage) (string, error) {
	b := domain.PageSnapshot{Instance: s.Client.Origin(), CourseID: course.ID, PageID: p.PageID, Slug: p.Slug, URL: s.pageURL(course.ID, p), Title: p.Title, BodyHTML: p.Body, Published: p.Published, UpdatedAt: p.UpdatedAt}
	d := domain.PageDocument{Schema: 1, Kind: "page", Operation: "update", Course: course.Nickname, Instance: b.Instance, CourseID: b.CourseID, PageID: b.PageID, Slug: b.Slug, URL: b.URL, Title: b.Title, BodyFormat: "html", Published: b.Published, Observed: &b}
	path, e := workspace.ConstrainPath(s.Root, filepath.Join("courses", course.Nickname, "pulled", fmt.Sprintf("page-%d-%s-%s.md", p.PageID, Slug(p.Title), time.Now().UTC().Format("20060102T150405.000000000"))))
	if e != nil {
		return "", e
	}
	return path, workspace.WriteDocument(path, &d, p.Body)
}

func (s *Service) PushPage(ctx context.Context, path string) (report PagePushReport, err error) {
	report.Path = path
	var d domain.PageDocument
	body, err := workspace.ReadDocument(path, &d)
	if err != nil {
		return report, err
	}
	course, err := s.Course(d.Course)
	if err != nil {
		return report, err
	}
	if (d.Instance != "" && d.Instance != s.Client.Origin()) || (d.CourseID != 0 && d.CourseID != course.ID) {
		return report, fmt.Errorf("page identity differs from configured Canvas/course")
	}
	if d.Operation == "update" {
		return s.pushPageUpdate(ctx, path, course, &d, body)
	}
	if d.Created != nil && (d.Created.PageID > 0 || ambiguous(d.Created.Page)) && (d.Instance == "" || d.CourseID == 0) {
		return report, fmt.Errorf("incomplete Page creation lacks its persisted origin/course")
	}
	d.Instance, d.CourseID, d.Course = s.Client.Origin(), course.ID, course.Nickname
	html, err := render.TeachingHTML(body, d.BodyFormat)
	if err != nil {
		return report, err
	}
	if d.Created == nil {
		d.Created = &domain.PageCreated{IntendedHTML: html, IntendedTitle: d.Title, ModuleID: d.ModuleID, ModulePosition: d.ModulePosition}
	}
	c := d.Created
	defer func() { report.PageID, report.URL, report.ModuleItemID = c.PageID, c.URL, c.ModuleItemID }()
	if c.IntendedHTML != html || c.IntendedTitle != d.Title || c.ModuleID != d.ModuleID || c.ModulePosition != d.ModulePosition {
		return report, fmt.Errorf("incomplete Page draft changed; recover the saved creation before editing it")
	}
	save := func() error { return workspace.WriteDocument(path, &d, body) }
	held := func(message string) (PagePushReport, error) {
		report.Status, report.Message = "held", message
		return report, nil
	}
	// Verify the selected module in this course before creating a Page.
	if d.ModuleID > 0 {
		var module struct {
			ID int64 `json:"id"`
		}
		if err = s.Client.Get(ctx, fmt.Sprintf("/api/v1/courses/%d/modules/%d", course.ID, d.ModuleID), &module); err != nil {
			return report, err
		}
		if module.ID != d.ModuleID {
			return report, fmt.Errorf("module response did not confirm selected course/module")
		}
	}
	if c.PageID == 0 {
		if ambiguous(c.Page) {
			return held("Page creation completion is uncertain; use a bounded live Page lookup in this course and record the exact returned ID/slug; do not replay create")
		}
		c.Page = domain.ActionOutcome{Status: "pending"}
		if err = save(); err != nil {
			return report, err
		}
		var p NativePage
		err = s.Client.Write(ctx, "POST", fmt.Sprintf("/api/v1/courses/%d/pages", course.ID), map[string]any{"wiki_page": map[string]any{"title": d.Title, "body": html, "published": false, "editing_roles": "teachers"}}, &p)
		if p.PageID > 0 {
			c.PageID, c.Slug, c.SavedHTML = p.PageID, p.Slug, p.Body
			if p.Slug != "" {
				c.URL = s.pageURL(course.ID, p)
			}
		}
		if err != nil {
			c.Page = failedOutcome(err)
		} else if p.PageID <= 0 || p.Slug == "" {
			c.Page = domain.ActionOutcome{Status: "uncertain", Message: "Page response lacks exact returned ID/slug; known ID retained for exact lookup"}
		} else {
			c.PageID, c.Slug, c.URL, c.SavedHTML = p.PageID, p.Slug, s.pageURL(course.ID, p), p.Body
			c.Page = domain.ActionOutcome{Status: "succeeded", ID: p.PageID}
		}
		if e := save(); e != nil {
			return report, fmt.Errorf("Page create outcome could not be persisted: %w", e)
		}
		if !definite(c.Page) {
			report.Status, report.Message = c.Page.Status, c.Page.Message
			return report, nil
		}
	}
	p, err := s.getPage(ctx, course.ID, c.PageID)
	if err != nil {
		return report, err
	}
	if c.Slug == "" {
		if p.Title != c.IntendedTitle || p.Body != c.SavedHTML {
			return held("known Page create response was incomplete; exact ID retained, but content must be inspected before resuming")
		}
		c.Slug, c.URL = p.Slug, s.pageURL(course.ID, p)
		c.Page = domain.ActionOutcome{Status: "succeeded", ID: c.PageID}
		if err = save(); err != nil {
			return report, err
		}
	}
	if !definite(c.Page) {
		if p.Title != c.IntendedTitle || p.Body != c.SavedHTML || p.Slug != c.Slug {
			return held("known Page create response was uncertain; exact ID retained, but differing live content must be inspected before resuming")
		}
		c.Page = domain.ActionOutcome{Status: "succeeded", ID: c.PageID, Message: "confirmed saved Page by exact ID"}
		if err = save(); err != nil {
			return report, err
		}
	}
	if c.Complete {
		report.Status, report.Message = "completed", "already created; pull-page before editing"
		return report, nil
	}
	if p.Title != c.IntendedTitle || p.Body != c.SavedHTML || p.Slug != c.Slug {
		return held("live Page differs from incomplete draft; pull-page to preserve teacher edits")
	}
	if p.Published && !d.Published {
		return held("teacher published the incomplete Page; preserve live publication instead of unpublishing it")
	}
	if p.Published && d.ModuleID > 0 && !definite(c.Module) {
		return held("Page was published before requested placement; preserve live state and recover construction")
	}
	if d.ModuleID == 0 {
		c.Module = domain.ActionOutcome{Status: "not-requested"}
	} else if c.ModuleItemID > 0 {
		var item struct {
			ID      int64  `json:"id"`
			Type    string `json:"type"`
			PageURL string `json:"page_url"`
		}
		if e := s.Client.Get(ctx, fmt.Sprintf("/api/v1/courses/%d/modules/%d/items/%d", course.ID, d.ModuleID, c.ModuleItemID), &item); e != nil {
			return report, e
		}
		if item.ID != c.ModuleItemID || item.Type != "Page" || item.PageURL != c.Slug {
			return held("known Page module placement changed; it will not be recreated automatically")
		}
		c.Module = domain.ActionOutcome{Status: "succeeded", ID: c.ModuleItemID}
		if err = save(); err != nil {
			return report, err
		}
	} else {
		if ambiguous(c.Module) {
			return held("Page module placement completion is uncertain; identify the exact live module item before retrying")
		}
		c.Module = domain.ActionOutcome{Status: "pending"}
		if err = save(); err != nil {
			return report, err
		}
		fields := map[string]any{"type": "Page", "page_url": c.Slug}
		if d.ModulePosition > 0 {
			fields["position"] = d.ModulePosition
		}
		var item struct {
			ID int64 `json:"id"`
		}
		err = s.Client.Write(ctx, "POST", fmt.Sprintf("/api/v1/courses/%d/modules/%d/items", course.ID, d.ModuleID), map[string]any{"module_item": fields}, &item)
		if item.ID > 0 {
			c.ModuleItemID = item.ID
		}
		if err != nil {
			c.Module = failedOutcome(err)
		} else if item.ID <= 0 {
			c.Module = domain.ActionOutcome{Status: "uncertain", Message: "module response lacks item ID"}
		} else {
			c.ModuleItemID = item.ID
			c.Module = domain.ActionOutcome{Status: "succeeded", ID: item.ID}
		}
		if e := save(); e != nil {
			return report, e
		}
		if !definite(c.Module) {
			report.Status, report.Message = c.Module.Status, c.Module.Message
			return report, nil
		}
		var confirmed struct {
			ID      int64  `json:"id"`
			Type    string `json:"type"`
			PageURL string `json:"page_url"`
		}
		if err = s.Client.Get(ctx, fmt.Sprintf("/api/v1/courses/%d/modules/%d/items/%d", course.ID, d.ModuleID, c.ModuleItemID), &confirmed); err != nil {
			return report, err
		}
		if confirmed.ID != c.ModuleItemID || confirmed.Type != "Page" || confirmed.PageURL != c.Slug {
			return held("Page placement response did not confirm requested content; exact item ID retained, publication held")
		}
	}
	if !definite(c.Publication) {
		// Read again after placement; never publish over a teacher's concurrent body edit.
		p, err = s.getPage(ctx, course.ID, c.PageID)
		if err != nil {
			return report, err
		}
		if p.Title != c.IntendedTitle || p.Body != c.SavedHTML || p.Slug != c.Slug {
			return held("live Page changed before publication; incomplete draft preserved")
		}
		if p.Published && !d.Published {
			return held("teacher published the incomplete Page; preserve live publication instead of unpublishing it")
		}
		if p.Published == d.Published {
			c.Publication = domain.ActionOutcome{Status: "preserved"}
		} else if ambiguous(c.Publication) {
			return held("Page publication completion is uncertain; preserve the differing live state")
		} else {
			c.Publication = domain.ActionOutcome{Status: "pending"}
			if err = save(); err != nil {
				return report, err
			}
			var response NativePage
			err = s.Client.Write(ctx, "PUT", nativePagePath(course.ID, c.PageID), map[string]any{"wiki_page": map[string]any{"published": d.Published}}, &response)
			if err != nil {
				c.Publication = failedOutcome(err)
			} else if response.PageID != c.PageID || response.Published != d.Published {
				c.Publication = domain.ActionOutcome{Status: "uncertain", Message: "publication response did not confirm requested Page state"}
			} else {
				c.Publication = domain.ActionOutcome{Status: "succeeded"}
			}
		}
	}
	c.Complete = definite(c.Page) && definite(c.Module) && definite(c.Publication)
	if err = save(); err != nil {
		return report, err
	}
	report.Status = "partial"
	report.Message = c.Publication.Message
	if c.Complete {
		report.Status = "completed"
		if c.SavedHTML != html {
			report.Message = "Canvas normalized the HTML; created.saved_html records the actual saved body; verify it in Canvas"
		}
	}
	return report, nil
}

func pagePatchMatches(p NativePage, title, html string, published bool, patch map[string]any) bool {
	for field := range patch {
		switch field {
		case "title":
			if p.Title != title {
				return false
			}
		case "body":
			if p.Body != html {
				return false
			}
		case "published":
			if p.Published != published {
				return false
			}
		}
	}
	return true
}

func (s *Service) pushPageUpdate(ctx context.Context, path string, course domain.Course, d *domain.PageDocument, body string) (PagePushReport, error) {
	r := PagePushReport{Path: path, PageID: d.PageID, URL: d.URL}
	if definite(d.UpdateOutcome) {
		r.Status, r.Message = "completed", "update already completed; pull-page before another edit"
		return r, nil
	}
	html, e := render.TeachingHTML(body, d.BodyFormat)
	if e != nil {
		return r, e
	}
	b := d.Observed
	patch := map[string]any{}
	if d.Title != b.Title {
		patch["title"] = d.Title
	}
	if html != b.BodyHTML {
		patch["body"] = html
	}
	if d.Published != b.Published {
		patch["published"] = d.Published
	}
	p, e := s.getPage(ctx, course.ID, d.PageID)
	if e != nil {
		return r, e
	}
	r.URL = s.pageURL(course.ID, p)
	if !pagePatchMatches(p, b.Title, b.BodyHTML, b.Published, patch) {
		if ambiguous(d.UpdateOutcome) && pagePatchMatches(p, d.Title, html, d.Published, patch) {
			d.UpdateOutcome = domain.ActionOutcome{Status: "succeeded", ID: d.PageID, Message: "intended live state recovered"}
			d.Slug, d.URL = p.Slug, r.URL
			r.Status = "completed"
			return r, workspace.WriteDocument(path, d, body)
		}
		fresh, e := s.writePulledPage(course, p)
		if e != nil {
			return r, e
		}
		r.Status, r.Message = "held", "live Page changed; unsent edit preserved; fresh version: "+fresh
		return r, nil
	}
	if ambiguous(d.UpdateOutcome) {
		r.Status, r.Message = "held", "prior Page update completion is uncertain; unchanged read does not authorize replay"
		return r, nil
	}
	if len(patch) == 0 {
		d.UpdateOutcome = domain.ActionOutcome{Status: "preserved"}
		r.Status = "completed"
		return r, workspace.WriteDocument(path, d, body)
	}
	d.UpdateOutcome = domain.ActionOutcome{Status: "pending"}
	if e = workspace.WriteDocument(path, d, body); e != nil {
		return r, e
	}
	var response NativePage
	e = s.Client.Write(ctx, "PUT", nativePagePath(course.ID, d.PageID), map[string]any{"wiki_page": patch}, &response)
	if e != nil {
		d.UpdateOutcome = failedOutcome(e)
	} else if response.PageID != d.PageID || response.Slug == "" {
		d.UpdateOutcome = domain.ActionOutcome{Status: "uncertain", Message: "update response lacks exact Page identity"}
	} else {
		d.Slug, d.URL = response.Slug, s.pageURL(course.ID, response)
		r.URL = d.URL
		if (patch["title"] != nil && response.Title != d.Title) || (patch["published"] != nil && response.Published != d.Published) {
			d.UpdateOutcome = domain.ActionOutcome{Status: "uncertain", Message: "Page update response did not confirm targeted fields"}
		} else {
			d.UpdateOutcome = domain.ActionOutcome{Status: "succeeded", ID: d.PageID}
			if patch["body"] != nil && response.Body != html {
				d.UpdateOutcome.Message = "Canvas normalized the body; observed.body_html records actual saved HTML; verify it in Canvas"
			}
			d.Observed = &domain.PageSnapshot{Instance: d.Instance, CourseID: d.CourseID, PageID: d.PageID, Slug: response.Slug, URL: d.URL, Title: response.Title, BodyHTML: response.Body, Published: response.Published, UpdatedAt: response.UpdatedAt}
		}
	}
	if err := workspace.WriteDocument(path, d, body); err != nil {
		return r, fmt.Errorf("Canvas Page update outcome could not be saved: %w", err)
	}
	r.Status, r.Message = d.UpdateOutcome.Status, d.UpdateOutcome.Message
	return r, nil
}
