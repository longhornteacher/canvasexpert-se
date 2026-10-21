package workflow

import (
	"path/filepath"
	"strconv"
	"strings"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/render"
	"canvasexpertse/internal/workspace"
)

// writeHTMLSidecar saves raw Canvas HTML beside a pulled document, so the
// agent-facing file carries readable text and the markup stays available when
// the text loses something (layout, embedded media). It returns the
// workspace-relative path, or "" for empty HTML.
func (s *Service) writeHTMLSidecar(relPath, body string) (string, error) {
	if strings.TrimSpace(body) == "" {
		return "", nil
	}
	file, err := workspace.ConstrainPath(s.Root, relPath)
	if err != nil {
		return "", err
	}
	if err = workspace.WriteFile(file, []byte(body), 0644); err != nil {
		return "", err
	}
	rel, err := filepath.Rel(s.Root, file)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(rel), nil
}

// packetDirections moves an assignment's directions HTML out of a packet
// snapshot into directions.html and returns the readable directions for the
// packet body. Grading-basis checks ignore description_html, so the packet
// keeps every field the Engine compares.
func (s *Service) packetDirections(relDir string, snapshot *domain.AssignmentSnapshot) (path, body string, err error) {
	html := snapshot.DescriptionHTML
	if path, err = s.writeHTMLSidecar(filepath.Join(relDir, "directions.html"), html); err != nil {
		return "", "", err
	}
	snapshot.DescriptionHTML = ""
	return path, render.HTMLText(html), nil
}

// moveAttemptHTML replaces each attempt's inline body_html with a sidecar file
// under the student's folder; readable_text stays in the student file.
func (s *Service) moveAttemptHTML(relDir, ref string, attempts ...*domain.AttemptWork) error {
	for _, attempt := range attempts {
		if attempt.BodyHTML == "" {
			continue
		}
		path, err := s.writeHTMLSidecar(filepath.Join(relDir, "students", ref, "attempt-"+strconv.Itoa(attempt.Ref.Attempt)+".html"), attempt.BodyHTML)
		if err != nil {
			return err
		}
		attempt.BodyHTMLPath, attempt.BodyHTML = path, ""
	}
	return nil
}
