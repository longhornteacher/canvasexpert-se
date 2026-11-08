package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"canvasexpertse/internal/render"
	"canvasexpertse/internal/workspace"
)

// STADStandingsMarker is where the Engine inserts the standings table.
const STADStandingsMarker = "{{standings}}"

// stadPageRecord is what CE-SE remembers about a course's standings Page.
type stadPageRecord struct {
	PageID    int64  `json:"page_id"`
	UpdatedAt string `json:"updated_at"` // the page's updated_at after CE-SE's last write
	URL       string `json:"url"`
}

type STADPublishReport struct {
	Status  string `json:"status"` // created, updated, unchanged, held
	PageID  int64  `json:"page_id,omitempty"`
	URL     string `json:"url,omitempty"`
	Message string `json:"message,omitempty"`
}

func stadPagesPath(root string) string {
	return filepath.Join(root, ".cse", "stad-pages.json")
}

func readSTADPages(root string) (map[string]stadPageRecord, error) {
	m := map[string]stadPageRecord{}
	b, err := os.ReadFile(stadPagesPath(root))
	if errors.Is(err, fs.ErrNotExist) {
		return m, nil
	}
	if err != nil {
		return nil, err
	}
	return m, json.Unmarshal(b, &m)
}

// STADPublish writes the course's standings Page: the agent's body with the
// computed standings table inserted at {{standings}}. The table holds team
// names, ranks and team scores only. The body is refused if it names a student
// (a student_ref or a roster alias). The Page is a managed page: CE-SE
// remembers its ID and last update, republishes without a fresh pull, and
// holds instead of overwriting when someone edited it in Canvas since.
// A new Page is created unpublished unless published is true.
func (s *Service) STADPublish(ctx context.Context, nickname, title, bodyFile string, published bool, now time.Time) (STADPublishReport, error) {
	var rep STADPublishReport
	if strings.TrimSpace(title) == "" {
		return rep, fmt.Errorf("a page title is required")
	}
	raw, err := os.ReadFile(bodyFile)
	if err != nil {
		return rep, err
	}
	body := string(raw)
	if !strings.Contains(body, STADStandingsMarker) {
		return rep, fmt.Errorf("the body must contain %s where the standings table goes", STADStandingsMarker)
	}
	if err = s.refuseStudentNames(body); err != nil {
		return rep, err
	}
	standings, err := s.STADStandings(ctx, nickname, "", "", now)
	if err != nil {
		return rep, err
	}
	course, err := s.Course(nickname)
	if err != nil {
		return rep, err
	}
	s, err = s.ForCourse(ctx, course)
	if err != nil {
		return rep, err
	}
	token := "STADSTANDINGSTABLE"
	rendered, err := render.TeachingHTML(strings.ReplaceAll(body, STADStandingsMarker, token), "markdown")
	if err != nil {
		return rep, err
	}
	table := standingsTable(standings)
	if strings.Contains(rendered, "<p>"+token+"</p>") {
		rendered = strings.Replace(rendered, "<p>"+token+"</p>", table, 1)
	} else {
		rendered = strings.Replace(rendered, token, table, 1)
	}

	pages, err := readSTADPages(s.Root)
	if err != nil {
		return rep, err
	}
	key := nickname
	record, known := pages[key]
	save := func(p NativePage) error {
		pages[key] = stadPageRecord{PageID: p.PageID, UpdatedAt: p.UpdatedAt, URL: s.pageURL(course.ID, p)}
		b, e := json.MarshalIndent(pages, "", "  ")
		if e != nil {
			return e
		}
		path := stadPagesPath(s.Root)
		if e = os.MkdirAll(filepath.Dir(path), 0o755); e != nil {
			return e
		}
		return workspace.WriteFile(path, append(b, '\n'), 0o600)
	}
	if !known {
		if existing, found := s.findPageByTitle(ctx, course.ID, 0, title); found {
			rep.Status, rep.PageID, rep.URL = "held", existing.PageID, s.pageURL(course.ID, existing)
			rep.Message = "a Page with this title already exists and CE-SE did not make it; pick another title, or remove that Page in Canvas first"
			return rep, nil
		}
		var p NativePage
		err = s.Client.Write(ctx, "POST", pageBase(course.ID, 0), map[string]any{"wiki_page": map[string]any{"title": title, "body": rendered, "published": published}}, &p)
		if err != nil {
			return rep, err
		}
		if p.PageID <= 0 {
			return rep, fmt.Errorf("Canvas accepted the Page but returned no page ID; check the course Pages list before running again")
		}
		if err = save(p); err != nil {
			return rep, fmt.Errorf("Page created (%s) but its ID could not be saved: %w", s.pageURL(course.ID, p), err)
		}
		rep.Status, rep.PageID, rep.URL = "created", p.PageID, s.pageURL(course.ID, p)
		return rep, nil
	}
	live, err := s.getPage(ctx, course.ID, record.PageID)
	if err != nil {
		return rep, fmt.Errorf("the standings Page CE-SE made earlier could not be read (%w); if it was deleted, remove %s's entry from .cse/stad-pages.json to publish a new one", err, nickname)
	}
	if live.UpdatedAt != record.UpdatedAt {
		rep.Status, rep.PageID, rep.URL = "held", live.PageID, s.pageURL(course.ID, live)
		rep.Message = "the Page was edited in Canvas since CE-SE last wrote it; nothing was changed. Pull it to keep those edits, or ask to overwrite."
		return rep, nil
	}
	if live.Body == rendered && live.Title == title {
		rep.Status, rep.PageID, rep.URL = "unchanged", live.PageID, s.pageURL(course.ID, live)
		return rep, nil
	}
	var p NativePage
	if err = s.Client.Write(ctx, "PUT", nativePagePath(course.ID, record.PageID), map[string]any{"wiki_page": map[string]any{"title": title, "body": rendered}}, &p); err != nil {
		return rep, err
	}
	if p.PageID <= 0 {
		p = live
	}
	if err = save(p); err != nil {
		return rep, fmt.Errorf("Page updated but CE-SE could not record it: %w", err)
	}
	rep.Status, rep.PageID, rep.URL = "updated", p.PageID, s.pageURL(course.ID, p)
	return rep, nil
}

// refuseStudentNames keeps individual students off a student-facing Page:
// any student_ref or roster alias in the body is refused.
func (s *Service) refuseStudentNames(body string) error {
	store, err := workspace.OpenStudents(s.Root)
	if err != nil {
		return err
	}
	for _, st := range store.Rows {
		if st.Ref != "" && strings.Contains(body, st.Ref) {
			return fmt.Errorf("the page body contains a student reference (%s); standings pages show teams only", st.Ref)
		}
		if st.Alias != "" {
			if regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(st.Alias) + `\b`).MatchString(body) {
				return fmt.Errorf("the page body contains the student alias %q; standings pages show teams only", st.Alias)
			}
		}
	}
	return nil
}

// standingsTable renders team-visible fields only.
func standingsTable(st STADStandings) string {
	var b strings.Builder
	b.WriteString("<table><thead><tr><th>Rank</th><th>Team</th><th>This week</th><th>This round</th></tr></thead><tbody>")
	for _, t := range st.Teams {
		fmt.Fprintf(&b, "<tr><td>%d</td><td>%s</td><td>%s</td><td>%s</td></tr>", t.RankRound, html.EscapeString(t.Name), html.EscapeString(t.ScoreWeek), html.EscapeString(t.ScoreRound))
	}
	b.WriteString("</tbody></table>")
	return b.String()
}
