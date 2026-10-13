package localui

import (
	"net/http"
	"time"

	updates "canvasexpertse/internal/playbooks"
)

type playbookRow struct {
	Path, State, SHA256 string
	Replaceable         bool
}

func playbookRows(root string) []playbookRow {
	rows := updates.Status(root)
	out := make([]playbookRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, playbookRow{Path: r.Path, State: r.State, SHA256: r.SHA256, Replaceable: r.Replaceable})
	}
	return out
}
func seedBytes(rel string) ([]byte, error) { return updates.SeedBytes(rel) }
func replacePlaybookFile(root, rel string, now time.Time) error {
	return updates.Replace(root, rel, "", now)
}
func ReplacePlaybook(root, rel, expectedSHA256 string, now time.Time) error {
	return updates.Replace(root, rel, expectedSHA256, now)
}
func (h *Hub) replacePlaybook(w http.ResponseWriter, r *http.Request) {
	if !h.checkPost(w, r) || h.needWorkspace(w) {
		return
	}
	rel := r.PostForm.Get("file")
	if r.PostForm.Get("confirm") != "on" {
		h.redirect(w, r, "/settings?section=files", "", "Tick the box to confirm before replacing a file.")
		return
	}
	if err := replacePlaybookFile(h.root, rel, time.Now()); err != nil {
		h.redirect(w, r, "/settings?section=files", "", "Could not get the new version: "+err.Error())
		return
	}
	h.redirect(w, r, "/settings?section=files", rel+" is now this version's copy. Your old one is in playbook-old.", "")
}

func (h *Hub) howPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "Method not allowed", 405)
		return
	}
	h.render(w, 200, howTemplate, h.pageData())
}

