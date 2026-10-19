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
	PageID       int64  `json:"page_id"`
	Slug         string `json:"url"`
	HTMLURL      string `json:"html_url"`
	Title        string `json:"title"`
	Body         string `json:"body"`
	Published    bool   `json:"published"`
	UpdatedAt    string `json:"updated_at"`
	EditingRoles string `json:"editing_roles"`
}

type NativePageRevision struct {
	RevisionID int64  `json:"revision_id"`
	UpdatedAt  string `json:"updated_at"`
	Latest     bool   `json:"latest"`
	URL        string `json:"url,omitempty"`
	Title      string `json:"title,omitempty"`
	Body       string `json:"body,omitempty"`
}

type PagePushReport struct {
	Path, URL, Status, Message string
	PageID, ModuleItemID       int64
}

func nativePagePath(courseID, pageID int64) string {
	// Canvas interprets a bare number as a slug first. Force the numeric ID.
	return fmt.Sprintf("/api/v1/courses/%d/pages/page_id:%d", courseID, pageID)
}

func groupPagePath(groupID, pageID int64) string {
	return fmt.Sprintf("/api/v1/groups/%d/pages/page_id:%d", groupID, pageID)
}

func pageBase(courseID, groupID int64) string {
	if groupID > 0 {
		return fmt.Sprintf("/api/v1/groups/%d/pages", groupID)
	}
	return fmt.Sprintf("/api/v1/courses/%d/pages", courseID)
}

func groupEditingRoles(roles *[]string) string {
	if roles == nil {
		return "teachers"
	}
	return normalizeEditingRoles(strings.Join(*roles, ","))
}

func normalizeEditingRoles(roles string) string {
	set := map[string]bool{}
	for _, role := range strings.Split(roles, ",") {
		role = strings.TrimSpace(role)
		if role != "" {
			set[role] = true
		}
	}
	ordered := []string{}
	for _, role := range []string{"teachers", "members", "students", "public"} {
		if set[role] {
			ordered = append(ordered, role)
		}
	}
	return strings.Join(ordered, ",")
}

func (s *Service) verifyPageGroup(ctx context.Context, nickname string, groupID int64) (domain.Course, error) {
	course, err := s.Course(nickname)
	if err != nil || groupID == 0 {
		return course, err
	}
	var g canvasGroup
	if err = s.Client.Get(ctx, fmt.Sprintf("/api/v1/groups/%d", groupID), &g); err != nil {
		return course, err
	}
	if g.ID != groupID || g.ContextType != "Course" || g.CourseID != course.ID || g.GroupCategoryID <= 0 || nonCollaborative(g.NonCollaborative) {
		return course, fmt.Errorf("group %d does not belong to this course as a collaborative group", groupID)
	}
	if _, err = s.groupCategory(ctx, course.ID, g.GroupCategoryID); err != nil {
		return course, err
	}
	return course, nil
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
	return s.PullPageScoped(ctx, nickname, query, 0)
}

func (s *Service) PullPageScoped(ctx context.Context, nickname, query string, groupID int64) (string, error) {
	course, e := s.verifyPageGroup(ctx, nickname, groupID)
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
		if groupID > 0 {
			prefix = fmt.Sprintf("/groups/%d/pages/", groupID)
		}
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
	if e = s.Client.Get(ctx, pageBase(course.ID, groupID)+"/"+identifier, &p); e != nil {
		return "", e
	}
	if p.PageID <= 0 || p.Slug == "" {
		return "", fmt.Errorf("Canvas response lacks page identity")
	}
	if strings.HasPrefix(identifier, "page_id:") && identifier != fmt.Sprintf("page_id:%d", p.PageID) {
		return "", fmt.Errorf("Canvas returned a different Page ID")
	}
	if groupID == 0 {
		return s.writePulledPageWithDates(ctx, course, p)
	}
	return s.writePulledPageScoped(course, p, groupID)
}

func (s *Service) writePulledPage(course domain.Course, p NativePage) (string, error) {
	return s.writePulledPageScoped(course, p, 0)
}

func (s *Service) writePulledPageScoped(course domain.Course, p NativePage, groupID int64) (string, error) {
	pageURL := s.pageURL(course.ID, p)
	if groupID > 0 {
		pageURL = s.Client.Origin() + fmt.Sprintf("/groups/%d/pages/%s", groupID, url.PathEscape(p.Slug))
	}
	b := domain.PageSnapshot{Instance: s.Client.Origin(), CourseID: course.ID, GroupID: groupID, PageID: p.PageID, Slug: p.Slug, URL: pageURL, Title: p.Title, BodyHTML: p.Body, Published: p.Published, EditingRoles: p.EditingRoles, UpdatedAt: p.UpdatedAt}
	d := domain.PageDocument{Schema: 1, Kind: "page", Operation: "update", Course: course.Nickname, Instance: b.Instance, CourseID: b.CourseID, GroupID: groupID, PageID: b.PageID, Slug: b.Slug, URL: b.URL, Title: b.Title, BodyFormat: "html", Published: b.Published, Observed: &b}
	path, e := workspace.ConstrainPath(s.Root, filepath.Join("courses", course.Nickname, "pulled", fmt.Sprintf("page-%d-%s-%s.md", p.PageID, Slug(p.Title), time.Now().UTC().Format("20060102T150405.000000000"))))
	if e != nil {
		return "", e
	}
	return path, workspace.WriteDocument(path, &d, p.Body)
}

// writePulledPageWithDates also records the exact date-details baseline so a
// later audience or availability-date change can preserve every unrelated row.
// An unreadable baseline does not fail the pull; audience preparation reads the
// live collection itself and holds if it cannot be preserved safely.
func (s *Service) writePulledPageWithDates(ctx context.Context, course domain.Course, p NativePage) (string, error) {
	path, err := s.writePulledPageScoped(course, p, 0)
	if err != nil {
		return "", err
	}
	live, err := s.readPageDates(ctx, course.ID, p.PageID)
	if err != nil {
		return path, nil
	}
	store, err := workspace.OpenStudents(s.Root)
	if err != nil {
		return path, nil
	}
	snap, err := s.pageDatesSnapshot(live, store)
	if err != nil {
		return path, nil
	}
	var d domain.PageDocument
	body, err := workspace.ReadDocument(path, &d)
	if err != nil {
		return "", err
	}
	d.ObservedDates = &snap
	return path, workspace.WriteDocument(path, &d, body)
}

func (s *Service) PageHistory(ctx context.Context, nickname string, pageID, groupID, revisionID int64) ([]NativePageRevision, error) {
	course, err := s.verifyPageGroup(ctx, nickname, groupID)
	if err != nil {
		return nil, err
	}
	if pageID <= 0 || revisionID < 0 {
		return nil, fmt.Errorf("page ID must be positive and revision ID cannot be negative")
	}
	base := pageBase(course.ID, groupID) + fmt.Sprintf("/page_id:%d/revisions", pageID)
	if revisionID > 0 {
		var revision NativePageRevision
		if err = s.Client.Get(ctx, base+fmt.Sprintf("/%d", revisionID), &revision); err != nil {
			return nil, err
		}
		if revision.RevisionID != revisionID {
			return nil, fmt.Errorf("Canvas returned a different Page revision")
		}
		return []NativePageRevision{revision}, nil
	}
	var revisions []NativePageRevision
	if err = s.Client.List(ctx, base, &revisions); err != nil {
		return nil, err
	}
	return revisions, nil
}

func (s *Service) PushPage(ctx context.Context, path string) (report PagePushReport, err error) {
	report.Path = path
	var d domain.PageDocument
	body, err := workspace.ReadDocument(path, &d)
	if err != nil {
		return report, err
	}
	course, err := s.verifyPageGroup(ctx, d.Course, d.GroupID)
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
		d.Created = &domain.PageCreated{GroupID: d.GroupID, IntendedHTML: html, IntendedTitle: d.Title, ModuleID: d.ModuleID, ModulePosition: d.ModulePosition}
	}
	c := d.Created
	defer func() { report.PageID, report.URL, report.ModuleItemID = c.PageID, c.URL, c.ModuleItemID }()
	if c.IntendedHTML != html || c.IntendedTitle != d.Title || c.GroupID != d.GroupID || c.ModuleID != d.ModuleID || c.ModulePosition != d.ModulePosition {
		return report, fmt.Errorf("incomplete Page draft changed; recover the saved creation before editing it")
	}
	save := func() error { return workspace.WriteDocument(path, &d, body) }
	held := func(message string) (PagePushReport, error) {
		report.Status, report.Message = "held", message
		return report, nil
	}
	// Verify the selected module in this course before creating a Page.
	if d.GroupID == 0 && d.ModuleID > 0 {
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
	if c.PageID == 0 && ambiguous(c.Page) {
		// An unclear create is resolved by looking for the Page by exact title.
		if found, ok := s.findPageByTitle(ctx, course.ID, d.GroupID, d.Title); ok {
			c.PageID, c.Slug = found.PageID, found.Slug
		}
	}
	createdThisRun := false
	if c.PageID == 0 {
		c.Page = domain.ActionOutcome{Status: "pending"}
		if err = save(); err != nil {
			return report, err
		}
		var p NativePage
		err = s.Client.Write(ctx, "POST", pageBase(course.ID, d.GroupID), map[string]any{"wiki_page": map[string]any{"title": d.Title, "body": html, "published": false, "editing_roles": groupEditingRoles(d.EditingRoles)}}, &p)
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
			if d.GroupID > 0 {
				c.URL = s.Client.Origin() + fmt.Sprintf("/groups/%d/pages/%s", d.GroupID, url.PathEscape(p.Slug))
			}
			c.Page = domain.ActionOutcome{Status: "succeeded", ID: p.PageID}
			createdThisRun = true
		}
		if e := save(); e != nil {
			return report, fmt.Errorf("Page create outcome could not be persisted: %w", e)
		}
		if !definite(c.Page) {
			report.Status, report.Message = c.Page.Status, c.Page.Message
			return report, nil
		}
	}
	var p NativePage
	if createdThisRun {
		p = NativePage{PageID: c.PageID, Slug: c.Slug, HTMLURL: c.URL, Title: d.Title, Body: c.SavedHTML, Published: false, EditingRoles: groupEditingRoles(d.EditingRoles)}
	} else {
		if d.GroupID == 0 {
			p, err = s.getPage(ctx, course.ID, c.PageID)
		} else {
			err = s.Client.Get(ctx, groupPagePath(d.GroupID, c.PageID), &p)
			if err == nil && (p.PageID != c.PageID || p.Slug == "") {
				err = fmt.Errorf("Canvas group Page response did not confirm exact page identity")
			}
		}
		if err != nil {
			return report, err
		}
	}
	if c.Slug == "" {
		c.Slug, c.URL, c.SavedHTML = p.Slug, s.pageURL(course.ID, p), p.Body
		if d.GroupID > 0 {
			c.URL = s.Client.Origin() + fmt.Sprintf("/groups/%d/pages/%s", d.GroupID, url.PathEscape(p.Slug))
		}
		c.Page = domain.ActionOutcome{Status: "succeeded", ID: c.PageID}
		if err = save(); err != nil {
			return report, err
		}
	}
	if !definite(c.Page) {
		c.Slug, c.URL, c.SavedHTML = p.Slug, s.pageURL(course.ID, p), p.Body
		if d.GroupID > 0 {
			c.URL = s.Client.Origin() + fmt.Sprintf("/groups/%d/pages/%s", d.GroupID, url.PathEscape(p.Slug))
		}
		c.Page = domain.ActionOutcome{Status: "succeeded", ID: c.PageID, Message: "adopted the live Page by exact ID"}
		if err = save(); err != nil {
			return report, err
		}
	}
	if c.Complete {
		report.Status, report.Message = "completed", "already created; pull-page before editing"
		return report, nil
	}
	// Restrict the Page before module placement and publication. A failed or
	// uncertain audience stage must never publish it.
	if d.Audience != nil {
		audienceComplete, audienceErr := s.applyPageAudience(ctx, course, &d, c.PageID, save)
		if audienceErr != nil {
			return report, audienceErr
		}
		if !audienceComplete {
			report.Status, report.Message = "held", "Individual audience is incomplete; this Page has not been placed or published by this action"
			return report, nil
		}
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
			c.Module = domain.ActionOutcome{Status: "preserved", ID: c.ModuleItemID, Message: "module placement was changed or removed in Canvas; left as it is now"}
		} else {
			c.Module = domain.ActionOutcome{Status: "succeeded", ID: c.ModuleItemID}
		}
		if err = save(); err != nil {
			return report, err
		}
	} else {
		if ambiguous(c.Module) {
			if id := s.findModulePageItem(ctx, course.ID, d.ModuleID, c.Slug); id > 0 {
				c.ModuleItemID = id
				c.Module = domain.ActionOutcome{Status: "succeeded", ID: id, Message: "adopted the existing module item for this Page"}
			}
		}
	}
	if d.ModuleID > 0 && c.ModuleItemID == 0 {
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
		if !createdThisRun {
			if d.GroupID == 0 {
				p, err = s.getPage(ctx, course.ID, c.PageID)
			} else {
				err = s.Client.Get(ctx, groupPagePath(d.GroupID, c.PageID), &p)
			}
			if err != nil {
				return report, err
			}
		}
		if p.Published == d.Published || p.Published {
			c.Publication = domain.ActionOutcome{Status: "preserved"}
		} else {
			c.Publication = domain.ActionOutcome{Status: "pending"}
			if err = save(); err != nil {
				return report, err
			}
			var response NativePage
			err = s.Client.Write(ctx, "PUT", pagePathFor(course.ID, d.GroupID, c.PageID), map[string]any{"wiki_page": map[string]any{"published": d.Published}}, &response)
			if err != nil {
				c.Publication = failedOutcome(err)
			} else if response.PageID != c.PageID || response.Published != d.Published {
				c.Publication = domain.ActionOutcome{Status: "uncertain", Message: "publication response did not confirm requested Page state"}
			} else {
				c.Publication = domain.ActionOutcome{Status: "succeeded"}
			}
		}
	}
	c.Complete = definite(c.Page) && definite(c.Module) && definite(c.Publication) && (d.Audience == nil || definite(d.AudienceOutcome))
	if err = save(); err != nil {
		return report, err
	}
	report.Status = "partial"
	report.Message = c.Publication.Message
	if d.Audience != nil {
		report.Message = appendMessage(pageAudienceSummary(&d), report.Message)
	}
	if c.Complete {
		report.Status = "completed"
		if c.SavedHTML != html {
			report.Message = "Canvas normalized the HTML; created.saved_html records the actual saved body; verify it in Canvas"
		}
	}
	return report, nil
}

func pagePathFor(courseID, groupID, pageID int64) string {
	if groupID > 0 {
		return groupPagePath(groupID, pageID)
	}
	return nativePagePath(courseID, pageID)
}

// findPageByTitle returns the one Page in the selected course or group whose
// title matches exactly, so an unclear create can be adopted instead of repeated.
func (s *Service) findPageByTitle(ctx context.Context, courseID, groupID int64, title string) (NativePage, bool) {
	var pages []NativePage
	if e := s.Client.List(ctx, pageBase(courseID, groupID)+"?search_term="+url.QueryEscape(title)+"&per_page=100", &pages); e != nil {
		return NativePage{}, false
	}
	var match NativePage
	n := 0
	for _, p := range pages {
		if p.Title == title && p.PageID > 0 && p.Slug != "" {
			match, n = p, n+1
		}
	}
	return match, n == 1
}

// findModulePageItem returns the module item that already places the Page.
func (s *Service) findModulePageItem(ctx context.Context, courseID, moduleID int64, slug string) int64 {
	var items []struct {
		ID      int64  `json:"id"`
		Type    string `json:"type"`
		PageURL string `json:"page_url"`
	}
	if e := s.Client.List(ctx, fmt.Sprintf("/api/v1/courses/%d/modules/%d/items?per_page=100", courseID, moduleID), &items); e != nil {
		return 0
	}
	for _, it := range items {
		if it.Type == "Page" && it.PageURL == slug {
			return it.ID
		}
	}
	return 0
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
		case "editing_roles":
			if normalizeEditingRoles(p.EditingRoles) != normalizeEditingRoles(patch["editing_roles"].(string)) {
				return false
			}
		}
	}
	return true
}

func (s *Service) pushPageUpdate(ctx context.Context, path string, course domain.Course, d *domain.PageDocument, body string) (PagePushReport, error) {
	r := PagePushReport{Path: path, PageID: d.PageID, URL: d.URL}
	if definite(d.UpdateOutcome) && d.Audience == nil && !pageDatesChanged(d) {
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
	if d.EditingRoles != nil && groupEditingRoles(d.EditingRoles) != normalizeEditingRoles(b.EditingRoles) {
		patch["editing_roles"] = groupEditingRoles(d.EditingRoles)
	}
	var p NativePage
	if d.GroupID == 0 {
		p, e = s.getPage(ctx, course.ID, d.PageID)
	} else {
		e = s.Client.Get(ctx, groupPagePath(d.GroupID, d.PageID), &p)
		if e == nil && (p.PageID != d.PageID || p.Slug == "") {
			e = fmt.Errorf("Canvas group Page response did not confirm exact page identity")
		}
	}
	if e != nil {
		return r, e
	}
	r.URL = s.pageURL(course.ID, p)
	save := func() error { return workspace.WriteDocument(path, d, body) }
	if !pagePatchMatches(p, b.Title, b.BodyHTML, b.Published, patch) {
		if ambiguous(d.UpdateOutcome) && pagePatchMatches(p, d.Title, html, d.Published, patch) {
			d.UpdateOutcome = domain.ActionOutcome{Status: "succeeded", ID: d.PageID, Message: "intended live state recovered"}
			d.Slug, d.URL = p.Slug, r.URL
			r.Status = "completed"
			return r, save()
		}
		fresh, e := s.writePulledPageScoped(course, p, d.GroupID)
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
	if len(patch) > 0 {
		d.UpdateOutcome = domain.ActionOutcome{Status: "pending"}
		if e = save(); e != nil {
			return r, e
		}
		var response NativePage
		e = s.Client.Write(ctx, "PUT", pagePathFor(course.ID, d.GroupID, d.PageID), map[string]any{"wiki_page": patch}, &response)
		if e != nil {
			d.UpdateOutcome = failedOutcome(e)
		} else if response.PageID != d.PageID || response.Slug == "" {
			d.UpdateOutcome = domain.ActionOutcome{Status: "uncertain", Message: "update response lacks exact Page identity"}
		} else {
			d.Slug, d.URL = response.Slug, s.pageURL(course.ID, response)
			if d.GroupID > 0 {
				d.URL = s.Client.Origin() + fmt.Sprintf("/groups/%d/pages/%s", d.GroupID, url.PathEscape(response.Slug))
			}
			r.URL = d.URL
			if (patch["title"] != nil && response.Title != d.Title) || (patch["published"] != nil && response.Published != d.Published) || (patch["editing_roles"] != nil && normalizeEditingRoles(response.EditingRoles) != normalizeEditingRoles(patch["editing_roles"].(string))) {
				d.UpdateOutcome = domain.ActionOutcome{Status: "uncertain", Message: "Page update response did not confirm targeted fields"}
			} else {
				d.UpdateOutcome = domain.ActionOutcome{Status: "succeeded", ID: d.PageID}
				if patch["body"] != nil && response.Body != html {
					d.UpdateOutcome.Message = "Canvas normalized the body; observed.body_html records actual saved HTML; verify it in Canvas"
				}
				d.Observed = &domain.PageSnapshot{Instance: d.Instance, CourseID: d.CourseID, GroupID: d.GroupID, PageID: d.PageID, Slug: response.Slug, URL: d.URL, Title: response.Title, BodyHTML: response.Body, Published: response.Published, EditingRoles: response.EditingRoles, UpdatedAt: response.UpdatedAt}
			}
		}
		if err := save(); err != nil {
			return r, fmt.Errorf("Canvas Page update outcome could not be saved: %w", err)
		}
		if !definite(d.UpdateOutcome) {
			r.Status, r.Message = d.UpdateOutcome.Status, d.UpdateOutcome.Message
			return r, nil
		}
	} else if d.UpdateOutcome.Status == "" {
		d.UpdateOutcome = domain.ActionOutcome{Status: "preserved"}
	}
	// Audience and availability-date actions are separate from the content
	// patch, so an audience-only change never sends a stale body or title.
	if d.Audience != nil {
		complete, err := s.applyPageAudience(ctx, course, d, d.PageID, save)
		if err != nil {
			return r, err
		}
		if !complete {
			r.Status, r.Message = "held", "Individual audience is incomplete; the Page content edit is preserved"
			return r, nil
		}
	}
	if pageDatesChanged(d) {
		complete, err := s.applyPageDates(ctx, course, d, d.PageID, save)
		if err != nil {
			return r, err
		}
		if !complete {
			r.Status, r.Message = "held", "Availability-date change is incomplete; the Page content edit is preserved"
			return r, nil
		}
	}
	if err := save(); err != nil {
		return r, err
	}
	if d.Audience == nil && !pageDatesChanged(d) {
		// Preserve the existing content-update status wording.
		if len(patch) == 0 {
			r.Status = "completed"
		} else {
			r.Status = d.UpdateOutcome.Status
		}
		r.Message = d.UpdateOutcome.Message
		return r, nil
	}
	r.Status = "completed"
	r.Message = appendMessage(pageAudienceSummary(d), pageDatesSummary(d))
	return r, nil
}
