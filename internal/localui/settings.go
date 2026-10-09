package localui

import (
	"net/http"
	"path/filepath"

	"canvasexpertse/internal/workspace"
)

func (h *Hub) saveSettings(w http.ResponseWriter, r *http.Request) {
	if !h.checkPost(w, r) {
		return
	}
	if h.needWorkspace(w) {
		return
	}
	config, body, err := h.readConfig()
	if err != nil {
		h.settings(w, 400, "Could not read workspace settings.", "")
		return
	}
	config.Aliases = r.PostForm.Get("aliases") == "on"
	if err = workspace.WriteDocument(filepath.Join(h.root, "context", "courses.md"), config, body); err != nil {
		h.settings(w, 400, "Could not save workspace settings.", "")
		return
	}
	h.redirect(w, r, "/settings", "Settings saved.", "")
}
