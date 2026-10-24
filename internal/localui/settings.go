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
			Scripts: []string{"canvas-health.js", "drafts.js", "appearance.js"}, Connect: true, Nav: nav, Handle: (*Hub).settingsPageHandler},
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
	data.SettingsNav = template.HTML(hubNavFor("Settings", settingsNavLabel(data.SettingsSection), data.Appearance))
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
		notice = "No folder picked. Nothing changed."
	}
	if query.Get("browse_error") == "1" {
		message = "The folder picker didn't open. Type or paste the folder's location in the box instead."
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
		h.settingsSection(w, 400, "Couldn't read your settings. Try again.", "", "preferences")
		return
	}
	config.Aliases = r.PostForm.Get("aliases") == "on"
	if err = workspace.WriteTeacherDocument(h.root, "context/courses.md", config, body); err != nil {
		h.settingsSection(w, 400, "Couldn't save your settings. Try again.", "", "preferences")
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
			h.redirect(w, r, settingsDraftURL("connections", r.PostForm.Get("sync_root")), "", "Couldn't switch folders: "+err.Error())
			return
		}
		if loc.Mode != "local" {
			h.redirect(w, r, settingsDraftURL("connections", r.PostForm.Get("sync_root")), "", "Couldn't switch back to this PC. Try again.")
			return
		}
		h.redirect(w, r, "/settings?section=connections&clear=sync", "Your settings are now saved on this PC only.", "")
		return
	}
	syncRoot := strings.TrimSpace(r.PostForm.Get("sync_root"))
	if r.PostForm.Get("action") == "save" && syncRoot == "" {
		h.redirect(w, r, settingsDraftURL("connections", syncRoot), "", "Choose Browse and pick a folder first.")
		return
	}
	loc, err := workspace.SetRulesLocation(h.root, syncRoot, embedded.WorkspaceTemplate, "workspace-template")
	if err != nil {
		h.redirect(w, r, settingsDraftURL("connections", syncRoot), "", "Couldn't switch folders: "+err.Error())
		return
	}
	if loc.Mode == "sync" {
		notice := "Your settings are now saved in " + loc.RulesRoot + ". On your other PC, open Settings, Connections and choose the same folder."
		if adoption := seedAdoptionNotice(h.root); adoption != "" {
			notice += " " + adoption
		}
		h.redirect(w, r, "/settings?section=connections&clear=sync", notice, "")
		return
	}
	h.redirect(w, r, "/settings?section=connections&clear=sync", "Your settings are now saved on this PC only.", "")
}

func settingsDraftURL(section, candidate string) string {
	query := url.Values{"section": {section}, "sync_root": {candidate}}
	return "/settings?" + query.Encode()
}

func inspectRulesLocation(root string) (workspace.RulesLocation, string, string, error) {
	loc, err := workspace.GetRulesLocation(root)
	if err != nil {
		return loc, "Unavailable", "CanvasExpert can't find where your settings are saved. It won't use this PC's old copy instead. Choose Browse and pick your OneDrive or Google Drive folder again.", err
	}
	if loc.Mode != "sync" {
		return loc, "This PC only", "Your settings are saved on this PC. To use them on another PC, choose a OneDrive or Google Drive folder in Settings, Connections.", nil
	}
	directory, err := os.Open(loc.RulesRoot)
	if err == nil {
		_, err = directory.ReadDir(1)
		_ = directory.Close()
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return loc, "Unavailable", "CanvasExpert can't open your settings folder, " + loc.RulesRoot + ". It won't use this PC's old copy instead. Make sure OneDrive or Google Drive is running and signed in, then choose Check the folder.", fmt.Errorf("couldn't open %s: %w", loc.RulesRoot, err)
	}
	return loc, "Available", "Your settings are saved in " + loc.RulesRoot + ". OneDrive or Google Drive copies them to your other PCs. If a change doesn't show up there, check that it's signed in on both PCs.", nil
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
	message := "Folder check: " + state + "."
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
