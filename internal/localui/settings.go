package localui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	embedded "canvasexpertse"
	"canvasexpertse/internal/platform"
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
		h.settingsSection(w, 400, "Could not read workspace settings.", "", "preferences")
		return
	}
	config.Aliases = r.PostForm.Get("aliases") == "on"
	if err = workspace.WriteDocument(filepath.Join(h.root, "context", "courses.md"), config, body); err != nil {
		h.settingsSection(w, 400, "Could not save workspace settings.", "", "preferences")
		return
	}
	h.redirect(w, r, "/settings?section=preferences&clear=prefs", "Settings saved.", "")
}

// saveRulesLocation explicitly binds this Install to a teacher-selected Sync
// directory. The shared rules helper validates and initializes the directory
// before persisting the local binding.
func (h *Hub) saveRulesLocation(w http.ResponseWriter, r *http.Request) {
	if !h.checkPost(w, r) || h.needWorkspace(w) {
		return
	}
	if r.PostForm.Get("action") == "local" {
		loc, err := workspace.SetRulesLocation(h.root, "", embedded.WorkspaceTemplate, "workspace-template")
		if err != nil {
			h.redirect(w, r, settingsDraftURL("connections", r.PostForm.Get("sync_root")), "", "Could not change teacher rules location: "+err.Error())
			return
		}
		if loc.Mode != "local" {
			h.redirect(w, r, settingsDraftURL("connections", r.PostForm.Get("sync_root")), "", "Teacher rules did not return to Install; check the saved rules location and try again.")
			return
		}
		h.redirect(w, r, "/settings?section=connections&clear=sync", "Teacher rules now use Install.", "")
		return
	}
	syncRoot := strings.TrimSpace(r.PostForm.Get("sync_root"))
	if r.PostForm.Get("action") == "save" && syncRoot == "" {
		h.redirect(w, r, settingsDraftURL("connections", syncRoot), "", "Choose a Sync folder first, or select Use Install.")
		return
	}
	loc, err := workspace.SetRulesLocation(h.root, syncRoot, embedded.WorkspaceTemplate, "workspace-template")
	if err != nil {
		h.redirect(w, r, settingsDraftURL("connections", syncRoot), "", "Could not change teacher rules location: "+err.Error())
		return
	}
	if loc.Mode == "sync" {
		notice := "Teacher rules now use Sync at " + loc.RulesRoot + ". Courses, students and work remain in Install."
		if adoption := seedAdoptionNotice(h.root); adoption != "" {
			notice += " " + adoption
		}
		h.redirect(w, r, "/settings?section=connections&clear=sync", notice, "")
		return
	}
	h.redirect(w, r, "/settings?section=connections&clear=sync", "Teacher rules now use Install.", "")
}

func settingsDraftURL(section, candidate string) string {
	query := url.Values{"section": {section}, "sync_root": {candidate}}
	return "/settings?" + query.Encode()
}

func inspectRulesLocation(root string) (workspace.RulesLocation, string, string, error) {
	loc, err := workspace.GetRulesLocation(root)
	if err != nil {
		return loc, "Unavailable", "The saved teacher-rules location is unavailable. CE-SE will not fall back to another copy.", err
	}
	if loc.Mode != "sync" {
		return loc, "Local only", "Teacher rules are stored in this Install.", nil
	}
	directory, err := os.Open(loc.RulesRoot)
	if err == nil {
		_, err = directory.ReadDir(1)
		_ = directory.Close()
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return loc, "Unavailable", "The saved Sync folder cannot be read. CE-SE will not fall back to another copy.", fmt.Errorf("configured Sync rules directory %s cannot be read: %w", loc.RulesRoot, err)
	}
	return loc, "Available", "The saved Sync folder is readable here. Cloud-provider delivery is not checked.", nil
}

func (h *Hub) browseRules(w http.ResponseWriter, r *http.Request) {
	if !h.checkPost(w, r) {
		return
	}
	if h.root == "" {
		http.Error(w, "Open the hub from an Install before selecting teacher rules.", http.StatusBadRequest)
		return
	}
	candidate := r.PostForm.Get("sync_root")
	h.mu.Lock()
	pick, ctx := h.pickFolderFn, h.ctx
	h.mu.Unlock()
	if pick == nil {
		pick = platform.PickFolder
	}
	if ctx == nil {
		ctx = context.Background()
	}
	selected, err := pick(ctx)
	query := url.Values{"section": {"connections"}, "sync_root": {candidate}}
	switch {
	case err != nil:
		query.Set("browse_error", "1")
	case strings.TrimSpace(selected) == "":
		query.Set("browse_cancel", "1")
	default:
		query.Set("sync_root", strings.TrimSpace(selected))
	}
	http.Redirect(w, r, "/settings?"+query.Encode(), http.StatusSeeOther)
}

func (h *Hub) testRulesLocation(w http.ResponseWriter, r *http.Request) {
	if !h.checkPost(w, r) {
		return
	}
	if h.root == "" {
		http.Error(w, "Open the hub from an Install before testing teacher rules.", http.StatusBadRequest)
		return
	}
	_, state, _, err := inspectRulesLocation(h.root)
	message := "Saved teacher-rules folder test: " + state + "."
	problem := ""
	if err != nil {
		problem = message + " " + err.Error()
		message = ""
	}
	h.mu.Lock()
	query := url.Values{"section": {"connections"}, "sync_root": {r.PostForm.Get("sync_root")}}
	h.redirect(w, r, "/settings?"+query.Encode(), message, problem)
	h.mu.Unlock()
}
