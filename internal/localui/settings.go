package localui

import (
	"net/http"
	"strings"

	embedded "canvasexpertse"
	"canvasexpertse/internal/workspace"
	"path/filepath"
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

// saveRulesLocation explicitly binds this Install to a teacher-selected Sync
// directory. The shared rules helper validates and initializes the directory
// before persisting the local binding.
func (h *Hub) saveRulesLocation(w http.ResponseWriter, r *http.Request) {
	if !h.checkPost(w, r) || h.needWorkspace(w) {
		return
	}
	syncRoot := strings.TrimSpace(r.PostForm.Get("sync_root"))
	loc, err := workspace.SetRulesLocation(h.root, syncRoot, embedded.WorkspaceTemplate, "workspace-template")
	if err != nil {
		h.redirect(w, r, "/settings", "", "Could not change teacher rules location: "+err.Error())
		return
	}
	if loc.Mode == "sync" {
		h.redirect(w, r, "/settings", "Teacher rules now use Sync at "+loc.RulesRoot+". Courses, students and work remain in Install.", "")
		return
	}
	h.redirect(w, r, "/settings", "Teacher rules now use Install.", "")
}
