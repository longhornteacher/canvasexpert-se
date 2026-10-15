package localui

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	embedded "canvasexpertse"
	"canvasexpertse/internal/platform"
	"canvasexpertse/internal/workspace"
)

// settingsData is homeData plus what only the Settings page needs.
type settingsData struct {
	homeData
	SettingsSection    string
	SettingsNav        template.HTML
	SettingsDraftClear string
	Playbook           []playbookRow
	Voice              string
	FeedbackGuidance   string
}

// settingsSections are the Settings tabs; each is a sub-tab of /settings. Agent
// (/connect) sits between Files and System in the same nav group.
var settingsSections = []struct{ Key, Label string }{
	{"connections", "Connections"}, {"preferences", "Preferences"}, {"files", "Files"}, {"system", "System"},
}

func init() {
	nav := []navSlot{{Primary: "Settings", Order: 30}}
	for i, s := range settingsSections {
		order := 31 + i
		if s.Key == "system" {
			order = 36 // after Agent (35)
		}
		nav = append(nav, navSlot{Primary: "Settings", Label: s.Label, Order: order, Href: "/settings?section=" + s.Key})
	}
	register(
		page{Path: "/settings", Methods: []string{http.MethodGet, http.MethodPost}, Template: "settings",
			Scripts: []string{"canvas-health.js", "drafts.js"}, Connect: true, Nav: nav, Handle: (*Hub).settingsPageHandler},
		page{Path: "/rules-location", Methods: []string{http.MethodPost}, Handle: (*Hub).saveRulesLocation},
		page{Path: "/rules/browse", Methods: []string{http.MethodPost}, Unlocked: true, Handle: (*Hub).browseRules},
		page{Path: "/rules/test", Methods: []string{http.MethodPost}, Unlocked: true, Handle: (*Hub).testRulesLocation},
	)
}

func (h *Hub) settingsPageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		h.saveSettings(w, r)
		return
	}
	message, notice := h.takeFlash()
	h.settingsRequest(w, r, 200, message, notice)
}

// settingsPage gathers the Settings page data for one section.
func (h *Hub) settingsPage(section string) settingsData {
	data := settingsData{homeData: h.pageData(), SettingsSection: validSettingsSection(section)}
	data.SettingsNav = template.HTML(hubNavFor("Settings", settingsNavLabel(data.SettingsSection)))
	if h.root != "" {
		data.Playbook = playbookRows(h.root)
		if g, _, err := readGrading(h.root); err == nil {
			data.Voice, data.FeedbackGuidance = g.Voice, g.FeedbackGuidance
		}
	}
	return data
}

func (h *Hub) settingsSection(w http.ResponseWriter, status int, message, notice, section string) {
	data := h.settingsPage(section)
	data.Message, data.Notice = message, notice
	h.render(w, status, tmpl("settings"), data)
}

func (h *Hub) settingsRequest(w http.ResponseWriter, r *http.Request, status int, message, notice string) {
	query := r.URL.Query()
	data := h.settingsPage(query.Get("section"))
	data.SettingsDraftClear = query.Get("clear")
	if query.Has("sync_root") {
		data.SyncRoot = query.Get("sync_root")
	}
	if query.Get("browse_cancel") == "1" {
		notice = "Folder selection canceled. The typed path remains here; the saved location is unchanged."
	}
	if query.Get("browse_error") == "1" {
		message = "The folder chooser could not open. Paste the folder path instead."
	}
	data.Message, data.Notice = message, notice
	h.render(w, status, tmpl("settings"), data)
}

func validSettingsSection(section string) string {
	for _, s := range settingsSections {
		if s.Key == section {
			return section
		}
	}
	return settingsSections[0].Key
}

func settingsNavLabel(section string) string {
	for _, s := range settingsSections {
		if s.Key == section {
			return s.Label
		}
	}
	return settingsSections[0].Label
}

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
	if err = workspace.WriteTeacherDocument(h.root, "context/courses.md", config, body); err != nil {
		h.settingsSection(w, 400, "Could not save workspace settings.", "", "preferences")
		return
	}
	h.redirect(w, r, "/settings?section=preferences&clear=prefs", "Settings saved.", "")
}

// saveRulesLocation explicitly binds this UserInstall to a teacher-selected UserSync
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
			h.redirect(w, r, settingsDraftURL("connections", r.PostForm.Get("sync_root")), "", "Teacher rules did not return to UserInstall; check the saved rules location and try again.")
			return
		}
		h.redirect(w, r, "/settings?section=connections&clear=sync", "Teacher rules now use UserInstall.", "")
		return
	}
	syncRoot := strings.TrimSpace(r.PostForm.Get("sync_root"))
	if r.PostForm.Get("action") == "save" && syncRoot == "" {
		h.redirect(w, r, settingsDraftURL("connections", syncRoot), "", "Choose a UserSync folder first, or select Use UserInstall.")
		return
	}
	loc, err := workspace.SetRulesLocation(h.root, syncRoot, embedded.WorkspaceTemplate, "workspace-template")
	if err != nil {
		h.redirect(w, r, settingsDraftURL("connections", syncRoot), "", "Could not change teacher rules location: "+err.Error())
		return
	}
	if loc.Mode == "sync" {
		notice := "Teacher setup now uses UserSync at " + loc.RulesRoot + ". Saved course setup, student settings, notes and contact details follow you between PCs. Grading and planning session files remain in UserInstall."
		if adoption := seedAdoptionNotice(h.root); adoption != "" {
			notice += " " + adoption
		}
		h.redirect(w, r, "/settings?section=connections&clear=sync", notice, "")
		return
	}
	h.redirect(w, r, "/settings?section=connections&clear=sync", "Teacher rules now use UserInstall.", "")
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
		return loc, "Local only", "Your teacher setup is kept on this PC. Add a UserSync folder to preserve reusable setup for another PC.", nil
	}
	directory, err := os.Open(loc.RulesRoot)
	if err == nil {
		_, err = directory.ReadDir(1)
		_ = directory.Close()
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return loc, "Unavailable", "The saved UserSync folder cannot be read. CE-SE will not fall back to another copy.", fmt.Errorf("configured UserSync rules directory %s cannot be read: %w", loc.RulesRoot, err)
	}
	return loc, "Available", "Your teacher setup is shared through " + loc.RulesRoot + ". Your sync app (OneDrive or Google Drive) keeps other PCs up to date; CE-SE does not check that delivery.", nil
}

func (h *Hub) browseRules(w http.ResponseWriter, r *http.Request) {
	if !h.checkPost(w, r) {
		return
	}
	if h.root == "" {
		http.Error(w, "Open the hub from a UserInstall before selecting teacher rules.", http.StatusBadRequest)
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
		http.Error(w, "Open the hub from a UserInstall before testing teacher rules.", http.StatusBadRequest)
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
