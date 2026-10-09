package workflow

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"canvasexpertse/internal/canvas"
	"canvasexpertse/internal/domain"
)

// Differentiated Hub: a whole-class item links to one Canvas Page per tier.
// Each page is restricted to that tier's differentiation tag (a non-collaborative
// group), is never placed in a module, and is never visible to the class. This file
// knows only pages and tags, so assignments and quizzes can both use it.

// HubTier is one tier page to deliver. Title is the exact Canvas page title.
type HubTier struct{ Label, Tag, Title, BodyHTML string }

// TierPage is the persisted record of one delivered page (see domain.TierPage).
type TierPage = domain.TierPage

type tagGroup struct {
	CategoryID, GroupID int64
	Name                string
}

type pageRead struct {
	PageID    int64  `json:"page_id"`
	URL       string `json:"url"`
	HTMLURL   string `json:"html_url"`
	Title     string `json:"title"`
	Published bool   `json:"published"`
}

type pageDates struct {
	VisibleToEveryone *bool `json:"visible_to_everyone"`
	Overrides         []struct {
		GroupID int64 `json:"group_id"`
	} `json:"overrides"`
}

func (d pageDates) restricted() bool { return d.VisibleToEveryone != nil && !*d.VisibleToEveryone }
func (d pageDates) assignedTo(group int64) bool {
	return len(d.Overrides) == 1 && d.Overrides[0].GroupID == group
}

// tagGroups lists non-collaborative groups (differentiation tags). Only category
// id, group id and name are kept; membership is never read.
func (s *Service) tagGroups(ctx context.Context, courseID int64) ([]tagGroup, error) {
	var categories []struct {
		ID int64 `json:"id"`
	}
	if err := s.Client.List(ctx, fmt.Sprintf("/api/v1/courses/%d/group_categories?collaboration_state=non_collaborative&per_page=100", courseID), &categories); err != nil {
		return nil, err
	}
	var out []tagGroup
	for _, category := range categories {
		var groups []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		}
		if err := s.Client.List(ctx, fmt.Sprintf("/api/v1/group_categories/%d/groups?per_page=100", category.ID), &groups); err != nil {
			return nil, err
		}
		for _, g := range groups {
			out = append(out, tagGroup{CategoryID: category.ID, GroupID: g.ID, Name: g.Name})
		}
	}
	return out, nil
}

// matchTag finds exactly one group by case-insensitive name.
func matchTag(groups []tagGroup, tag string) (string, tagGroup) {
	var found []tagGroup
	for _, g := range groups {
		if strings.EqualFold(strings.TrimSpace(g.Name), strings.TrimSpace(tag)) {
			found = append(found, g)
		}
	}
	switch len(found) {
	case 0:
		return "not_found", tagGroup{}
	case 1:
		return "matched", found[0]
	}
	return "ambiguous", tagGroup{}
}

func pagePath(courseID, pageID int64) string {
	return fmt.Sprintf("/api/v1/courses/%d/pages/%d", courseID, pageID)
}

func (s *Service) readPage(ctx context.Context, courseID, pageID int64) (pageRead, error) {
	var p pageRead
	if err := s.Client.Get(ctx, pagePath(courseID, pageID), &p); err != nil {
		return p, err
	}
	if p.PageID != pageID || p.URL == "" {
		return p, fmt.Errorf("Canvas returned a different page than id %d", pageID)
	}
	return p, nil
}

func (s *Service) readDates(ctx context.Context, courseID, pageID int64) (pageDates, error) {
	var d pageDates
	if err := s.Client.Get(ctx, pagePath(courseID, pageID)+"/date_details", &d); err != nil {
		return d, err
	}
	if d.VisibleToEveryone == nil {
		return d, fmt.Errorf("Canvas did not report who can see page %d", pageID)
	}
	return d, nil
}

func (s *Service) setOverrides(ctx context.Context, courseID, pageID, group int64) error {
	overrides := []map[string]any{}
	if group > 0 {
		overrides = append(overrides, map[string]any{"group_id": group})
	}
	return s.Client.Write(ctx, "PUT", pagePath(courseID, pageID)+"/date_details", map[string]any{"only_visible_to_overrides": true, "assignment_overrides": overrides}, nil)
}

func isRefused(err error) bool {
	var ce *canvas.Error
	return errors.As(err, &ce) && !ce.Uncertain && ce.Status >= 400 && ce.Status < 500
}

// unpublish is the fail-closed response to a page that is visible to the class.
func (s *Service) unpublish(ctx context.Context, courseID int64, p TierPage, why string) error {
	if err := s.Client.Write(ctx, "PUT", pagePath(courseID, p.PageID), map[string]any{"wiki_page": map[string]any{"published": false}}, nil); err != nil {
		return fmt.Errorf("page %q (id %d) %s and could NOT be unpublished (%v): unpublish it in Canvas now", p.Title, p.PageID, why, err)
	}
	return fmt.Errorf("page %q (id %d) %s; it was unpublished again", p.Title, p.PageID, why)
}

// DeliverTierPages makes every tier page exist, restricted to its tag, and (when
// publish is true) published. known carries pages from an earlier interrupted run:
// they are verified by exact id and never inferred by title. persist, when given,
// is called with the full result after each page is created or finished, so a
// caller can save ids before anything else happens. The returned slice always holds
// everything known, even with an error; an uncertain Canvas write is never retried
// here. A tag that cannot be resolved or assigned never blocks delivery: the page
// stays restricted with nobody assigned and TeacherAction says what to do.
func (s *Service) DeliverTierPages(ctx context.Context, courseID int64, tiers []HubTier, publish bool, known []TierPage, persist func([]TierPage) error) ([]TierPage, error) {
	out := make([]TierPage, len(tiers))
	seen := map[string]bool{}
	for i, t := range tiers {
		if t.Label == "" || strings.TrimSpace(t.Title) == "" || strings.TrimSpace(t.BodyHTML) == "" || strings.TrimSpace(t.Tag) == "" {
			return nil, fmt.Errorf("tier %q needs a label, tag, title and body", t.Label)
		}
		if seen[t.Label] {
			return nil, fmt.Errorf("duplicate tier label %q", t.Label)
		}
		seen[t.Label] = true
		out[i] = TierPage{Label: t.Label, Title: t.Title}
		for _, k := range known {
			if k.Label == t.Label && k.PageID > 0 {
				out[i] = k
			}
		}
		out[i].Tag, out[i].Title = t.Tag, t.Title
	}
	// Refuse a title collision before writing anything: this document did not create it.
	for i, t := range tiers {
		if out[i].PageID > 0 {
			continue
		}
		var rows []struct {
			PageID int64  `json:"page_id"`
			Title  string `json:"title"`
		}
		if err := s.Client.List(ctx, fmt.Sprintf("/api/v1/courses/%d/pages?per_page=100&search_term=%s", courseID, url.QueryEscape(t.Title)), &rows); err != nil {
			return out, fmt.Errorf("could not check for an existing page titled %q: %w", t.Title, err)
		}
		for _, row := range rows {
			if strings.EqualFold(strings.TrimSpace(row.Title), strings.TrimSpace(t.Title)) {
				return out, fmt.Errorf("a page titled %q already exists in this course (page id %d) and was not created by this document; rename or delete it, or give the tier a different title", row.Title, row.PageID)
			}
		}
	}
	groups, tagErr := s.tagGroups(ctx, courseID)
	save := func() error {
		if persist == nil {
			return nil
		}
		return persist(out)
	}
	for i, t := range tiers {
		p := &out[i]
		if p.PageID == 0 {
			var created pageRead
			err := s.Client.Write(ctx, "POST", fmt.Sprintf("/api/v1/courses/%d/pages", courseID), map[string]any{"wiki_page": map[string]any{"title": t.Title, "body": t.BodyHTML, "published": false}}, &created)
			if err != nil {
				return out, fmt.Errorf("creating page %q: %w", t.Title, err)
			}
			if created.PageID <= 0 || created.URL == "" {
				return out, fmt.Errorf("Canvas created page %q but did not return its id and slug; find it in Canvas before pushing again", t.Title)
			}
			p.PageID, p.Slug, p.URL, p.Published = created.PageID, created.URL, created.HTMLURL, created.Published
			if err = save(); err != nil {
				return out, fmt.Errorf("page %q exists (id %d) but could not be recorded locally: %w", t.Title, p.PageID, err)
			}
		}
		if err := s.finishTierPage(ctx, courseID, p, groups, tagErr, publish); err != nil {
			_ = save()
			return out, err
		}
		if err := save(); err != nil {
			return out, err
		}
	}
	return out, nil
}

func (s *Service) finishTierPage(ctx context.Context, courseID int64, p *TierPage, groups []tagGroup, tagErr error, publish bool) error {
	live, err := s.readPage(ctx, courseID, p.PageID)
	if err != nil {
		return fmt.Errorf("verifying page %q (id %d): %w", p.Title, p.PageID, err)
	}
	p.Slug, p.URL, p.Published = live.URL, live.HTMLURL, live.Published

	dates, err := s.readDates(ctx, courseID, p.PageID)
	if err != nil {
		return fmt.Errorf("reading who can see page %q: %w", p.Title, err)
	}
	if !dates.restricted() {
		if live.Published {
			return fmt.Errorf("page %q (id %d) is published and visible to the whole class; restrict or unpublish it in Canvas", p.Title, p.PageID)
		}
		if err = s.setOverrides(ctx, courseID, p.PageID, 0); err != nil {
			return fmt.Errorf("restricting page %q: %w", p.Title, err)
		}
		if dates, err = s.readDates(ctx, courseID, p.PageID); err != nil || !dates.restricted() {
			return fmt.Errorf("page %q (id %d) could not be confirmed restricted; it stays unpublished", p.Title, p.PageID)
		}
	}

	// Tag assignment. Existing assignments that are not exactly the tag are the
	// teacher's and are left alone.
	status, group := "unavailable", tagGroup{}
	if tagErr == nil {
		status, group = matchTag(groups, p.Tag)
	}
	p.GroupID = 0
	switch {
	case len(dates.Overrides) > 0 && !(status == "matched" && dates.assignedTo(group.GroupID)):
		p.TagStatus = "manual"
	case status == "matched" && dates.assignedTo(group.GroupID):
		p.TagStatus, p.GroupID = "matched", group.GroupID
	case status == "matched":
		err = s.setOverrides(ctx, courseID, p.PageID, group.GroupID)
		if err != nil && !isRefused(err) {
			return fmt.Errorf("assigning page %q to tag %q: %w", p.Title, p.Tag, err)
		}
		after, readErr := s.readDates(ctx, courseID, p.PageID)
		switch {
		case readErr == nil && !after.restricted():
			return s.unpublish(ctx, courseID, *p, "became visible to everyone while assigning its tag")
		case err == nil && readErr == nil && after.assignedTo(group.GroupID):
			p.TagStatus, p.GroupID = "matched", group.GroupID
		default:
			// Refused (a collaborative group, for example) or not confirmed: nobody assigned.
			p.TagStatus = "unavailable"
			if readErr != nil || len(after.Overrides) > 0 {
				if e := s.setOverrides(ctx, courseID, p.PageID, 0); e != nil {
					return fmt.Errorf("page %q could not be confirmed unassigned after a failed tag assignment: %w", p.Title, e)
				}
			}
		}
	default:
		p.TagStatus = status
	}
	p.TeacherAction = ""
	if p.TagStatus != "matched" && p.TagStatus != "manual" {
		reason := map[string]string{"not_found": "no differentiation tag has that name", "ambiguous": "more than one differentiation tag has that name", "unavailable": "Canvas could not read or assign that tag"}[p.TagStatus]
		p.TeacherAction = fmt.Sprintf("assign page %q to tag %q in Canvas (%s); until then nobody can see it", p.Title, p.Tag, reason)
	}

	if publish && !live.Published {
		again, err := s.readDates(ctx, courseID, p.PageID)
		if err != nil || !again.restricted() {
			return fmt.Errorf("page %q was not published because its restriction could not be re-confirmed", p.Title)
		}
		if err = s.Client.Write(ctx, "PUT", pagePath(courseID, p.PageID), map[string]any{"wiki_page": map[string]any{"published": true}}, nil); err != nil {
			return fmt.Errorf("publishing page %q: %w", p.Title, err)
		}
		after, pageErr := s.readPage(ctx, courseID, p.PageID)
		dates, dateErr := s.readDates(ctx, courseID, p.PageID)
		if pageErr != nil || dateErr != nil || !dates.restricted() {
			return s.unpublish(ctx, courseID, *p, "could not be confirmed restricted after publishing")
		}
		p.Published = after.Published
	}
	return nil
}

// BindTierLinks replaces every {{tier:LABEL}} token with the verified page path.
// It fails when a token has no page or a page has no token, so a link is never guessed.
func BindTierLinks(content string, pages []TierPage, courseID int64) (string, error) {
	byLabel := map[string]TierPage{}
	for _, p := range pages {
		if p.Slug == "" {
			return "", fmt.Errorf("tier %q page has no Canvas slug", p.Label)
		}
		byLabel[p.Label] = p
	}
	used := map[string]bool{}
	var missing []string
	bound := domain.TierToken.ReplaceAllStringFunc(content, func(token string) string {
		label := domain.TierToken.FindStringSubmatch(token)[1]
		p, ok := byLabel[label]
		if !ok {
			missing = append(missing, label)
			return token
		}
		used[label] = true
		return fmt.Sprintf("/courses/%d/pages/%s", courseID, p.Slug)
	})
	if len(missing) > 0 {
		return "", fmt.Errorf("link token(s) without a tier page: %s", strings.Join(missing, ", "))
	}
	for _, p := range pages {
		if !used[p.Label] {
			return "", fmt.Errorf("tier %q has a page but no {{tier:%s}} token in the content", p.Label, p.Label)
		}
	}
	return bound, nil
}

// HubSummary lists each page, its tag, publication, and every teacher action.
func HubSummary(pages []TierPage) string {
	var b strings.Builder
	var actions []string
	unpublished := false
	for _, p := range pages {
		state := "published"
		if !p.Published {
			state, unpublished = "unpublished", true
		}
		fmt.Fprintf(&b, "tier page %q: %q (id %d, /pages/%s), tag %q %s, %s\n", p.Label, p.Title, p.PageID, p.Slug, p.Tag, tagStatusText(p.TagStatus), state)
		if p.TeacherAction != "" {
			actions = append(actions, p.TeacherAction)
		}
	}
	for _, a := range actions {
		fmt.Fprintf(&b, "teacher action: %s\n", a)
	}
	if unpublished {
		b.WriteString("note: unpublished tier pages stay unpublished when the hub is published later; publish them with it.\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func tagStatusText(status string) string {
	switch status {
	case "matched":
		return "matched and assigned"
	case "manual":
		return "already assigned in Canvas (left as is)"
	case "":
		return "not yet resolved"
	}
	return status + " (nobody assigned)"
}
