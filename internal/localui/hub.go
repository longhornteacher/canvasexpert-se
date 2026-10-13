package localui

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	embedded "canvasexpertse"
	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/hubcoord"
	"canvasexpertse/internal/installlog"
	"canvasexpertse/internal/platform"
	"canvasexpertse/internal/workflow"
	"canvasexpertse/internal/workspace"
)

// preferredHubAddr is tried first so a bookmark keeps working; the hub falls
// back to any free loopback port when it is taken.
const preferredHubAddr = "127.0.0.1:47831"

var homeTemplate = template.Must(template.New("home").Parse(`<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>CanvasExpert: Special Edition</title><style>` + funkyCSS + `</style><main class="wide">` + hubNavHome + `
<section class="home-status-grid" aria-label="Connection status">
<section class="card home-status" id="home-canvas" data-canvas-health data-state="{{.CanvasHealth}}"><h2><span class="status-mark" data-health-mark aria-hidden="true"></span>Canvas</h2><p class="health-state" data-health-state aria-live="polite">{{.CanvasHealth}}</p><p class="health-note" data-health-note>{{.CanvasHealthNote}}</p>
<div class="inline">{{if .Connected}}<form method="post" action="/canvas/test"><input type="hidden" name="request_secret" value="{{.Secret}}"><button type="submit" data-canvas-test class="small" {{if eq .CanvasHealth "Checking…"}}disabled{{end}}>Test</button></form><a class="go" href="/settings?section=connections">Connection</a><a class="go" href="/setup">Replace token</a>{{else}}<a class="go" href="/setup">Connect Canvas</a><a class="go" href="/settings?section=connections">Connection</a>{{end}}</div>
<details><summary>Get an Access token</summary><ol><li>Sign in to Canvas and open Account → Settings.</li><li>Under Approved Integrations, choose New Access Token.</li><li>Copy the token when Canvas shows it. Enter it under Settings → Connections.</li></ol><p>The token is kept in Windows Credential Manager on this computer.</p></details></section>
		<section class="card home-status" id="home-sync"><h2><span class="status-mark" aria-hidden="true"></span>Sync</h2><p class="health-state">{{.SyncHealth}}</p><p class="health-note">{{.SyncHealthNote}}</p>{{if eq .RulesMode "sync"}}<p><code>{{.RulesRoot}}</code></p>{{end}}<div class="inline"><a class="go" href="/settings?section=connections#rules">Browse</a><form method="post" action="/rules/test"><input type="hidden" name="request_secret" value="{{.Secret}}"><input type="hidden" name="sync_root" value="{{.SyncRoot}}"><button class="small secondary" type="submit">Test saved folder</button></form><a class="go" href="/settings?section=connections">Manage teacher rules</a></div></section>
</section>
{{if .Message}}<p class="error">{{.Message}}</p>{{end}}{{if .Notice}}<p class="ok">{{.Notice}}</p>{{end}}
<section class="home-courses"><div class="section-head"><h2>Active courses <span class="count">{{.ActiveCourses}}</span></h2><a href="/courses">Manage</a></div>
{{if .CourseRows}}<ul class="home-course-list">{{range .CourseRows}}<li><a href="/students?course={{.ID}}">{{.Nickname}}</a><span>Canvas course {{.ID}}</span></li>{{end}}</ul>{{else}}<p class="health-note">No active courses selected. Choose courses to show them here.</p><a class="go" href="/courses">Choose courses</a>{{end}}</section>
<p class="home-agent"><span>Agent setup</span><a href="/connect">Connect this Install to your teaching app</a></p>
<script src="/_cse/canvas-health.js" defer></script></main></html>`))

type homeData struct {
	Secret, Message, Notice, CanvasOrigin, Root string
	Program, Version                            string
	CanvasHealth, CanvasHealthNote              string
	SyncHealth, SyncHealthNote                  string
	ActiveCourses                               int
	CourseRows                                  []homeCourse
	Connected                                   bool
	ConnectionTested                            bool
	Preferences, Aliases                        bool
	Students, Monitored                         int
	CanvasUser, Voice, FeedbackGuidance         string
	SettingsSection                             string
	SettingsNav                                 template.HTML
	SettingsDraftClear                          string
	RulesMode, RulesRoot, SyncRoot, RulesError  string
	Playbook                                    []playbookRow
}

type homeCourse struct {
	ID       int64
	Nickname string
}

// Hub is one long-lived loopback server holding every local page. Pages share
// a single per-run request secret and the same origin checks as the one-shot
// commands.
type Hub struct {
	origin, secret string
	installClient  string
	programDir     string
	version        string
	ctx            context.Context
	// flash carries one notice or problem across a post-redirect-get.
	flash, flashErr string
	// loaded tracks the courses refreshed from Canvas during this hub session.
	loaded map[int64]bool
	// Test seams; nil means use the saved Canvas credential.
	listCoursesFn    func(context.Context) ([]workflow.Course, string, error)
	rosterFn         func(context.Context, int64) ([]workflow.Enrollment, string, error)
	tagsFn           func(context.Context, int64) (map[int64][]string, string, error)
	whoamiFn         func(context.Context) (string, error)
	loadCredentialFn func() (platform.Credential, error)
	savedOriginFn    func() string
	forgetFn         func() error
	pickFolderFn     func(context.Context) (string, error)
	// canvasUser is the account name from the last successful connection test.
	canvasUser                                              string
	setupValidated                                          bool
	connectionTested                                        bool
	canvasHealth, canvasHealthNote, canvasHealthFingerprint string
	canvasHealthGeneration                                  uint64
	canvasHealthTimeout                                     time.Duration
	// seedNotice reports starter files added when the installation opened;
	// Home shows it once.
	seedNotice        string
	mu                sync.Mutex
	root              string
	setup             *setupHandler
	quit              chan struct{}
	quitOnce          sync.Once
	connection        connectionData
	setupSaved        bool
	credentialSavedFn func() bool
}

// addMissingSeed copies any seed file the workspace lacks (never
// overwrites) and returns a teacher-facing notice, empty when nothing changed.
// A failure is reported in the notice and never blocks opening the workspace.
func addMissingSeed(root string) string {
	added, err := workspace.Scaffold(root, embedded.WorkspaceTemplate, "workspace-template")
	var notice string
	if len(added) > 0 {
		notice = "Added from the new program version: " + strings.Join(added, ", ") + "."
	}
	if err != nil {
		notice += " Could not add some starter files: " + err.Error()
	}
	return strings.TrimSpace(notice)
}

// seedAdoptionNotice points teachers with customized active rules to the
// existing comparison and explicit adoption controls in Settings.
func seedAdoptionNotice(root string) string {
	loc, err := workspace.GetRulesLocation(root)
	if err != nil || loc.Mode != "sync" {
		return ""
	}
	for _, row := range playbookRows(root) {
		if row.Replaceable && row.State == "differs" {
			return "Some seed-managed instructions in Sync differ from this version. Review them in Settings → Files and choose any files to adopt. Nothing was replaced."
		}
	}
	return ""
}

func (h *Hub) headers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "same-origin")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	csp := "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'"
	if r.URL.Path == "/" || r.URL.Path == "/settings" {
		csp += "; script-src 'self'; connect-src 'self'"
	} else if r.URL.Path == "/courses" || r.URL.Path == "/students" {
		csp += "; script-src 'self'"
	}
	w.Header().Set("Content-Security-Policy", csp)
}

// checkPost enforces method, origin and secret for hub-owned forms.
func (h *Hub) checkPost(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "Method not allowed", 405)
		return false
	}
	if r.Header.Get("Origin") != h.origin {
		http.Error(w, "Form origin rejected", 403)
		return false
	}
	limit := int64(16 << 10)
	if r.URL.Path == "/students" || r.URL.Path == "/grading" {
		limit = 256 << 10
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", 400)
		return false
	}
	if subtle.ConstantTimeCompare([]byte(r.PostForm.Get("request_secret")), []byte(h.secret)) != 1 {
		http.Error(w, "Form request rejected", 403)
		return false
	}
	return true
}

func (h *Hub) home(w http.ResponseWriter, status int, message, notice string) {
	if notice == "" {
		notice, h.seedNotice = h.seedNotice, ""
	}
	data := h.pageData()
	data.Message, data.Notice = message, notice
	h.render(w, status, homeTemplate, data)
}

func (h *Hub) pageData() homeData {
	data := homeData{Secret: h.secret, Root: h.root, Program: h.programDir, Version: h.version, SyncHealth: "Local only", SyncHealthNote: "Teacher rules are stored in this Install."}
	data.ConnectionTested = h.connectionTested
	data.CanvasHealth, data.CanvasHealthNote = h.canvasHealth, h.canvasHealthNote
	if data.CanvasHealth == "" {
		data.CanvasHealth, data.CanvasHealthNote = canvasHealthNotConnected, canvasHealthNote(canvasHealthNotConnected)
	}
	if c, err := h.loadSavedCredential(); err == nil {
		data.Connected, data.CanvasOrigin, data.CanvasUser = true, c.Origin, h.canvasUser
	}
	if h.root != "" {
		loc, rulesErr := workspace.GetRulesLocation(h.root)
		data.RulesMode, data.RulesRoot, data.SyncRoot = loc.Mode, loc.RulesRoot, ""
		if loc.Mode == "sync" {
			data.SyncRoot = loc.RulesRoot
			if rulesErr != nil {
				data.SyncHealth, data.SyncHealthNote = "Unavailable", "The saved Sync folder is unavailable; CE-SE will not fall back to Install. Use Test or choose an available folder."
			} else {
				data.SyncHealth = "Available"
				data.SyncHealthNote = "Saved Sync folder exists here. Use Test to check readability. Cloud-provider delivery is not checked."
			}
		} else {
			data.SyncHealth, data.SyncHealthNote = "Local only", "Teacher rules are stored in this Install."
		}
		if rulesErr != nil {
			data.RulesError = rulesErr.Error()
		}
		if config, _, err := h.readConfig(); err == nil {
			data.Preferences, data.Aliases = true, config.Aliases
			for _, c := range config.Courses {
				if c.Current {
					data.ActiveCourses++
					nickname := strings.TrimSpace(c.Nickname)
					if nickname == "" {
						nickname = fmt.Sprintf("Course %d", c.ID)
					}
					data.CourseRows = append(data.CourseRows, homeCourse{ID: c.ID, Nickname: nickname})
				}
			}
		}
		data.Playbook = playbookRows(h.root)
		if g, _, err := readGrading(h.root); err == nil {
			data.Voice, data.FeedbackGuidance = g.Voice, g.FeedbackGuidance
		}
		if s, err := workspace.OpenStudents(h.root); err == nil {
			data.Students = len(s.Rows)
			for _, row := range s.Rows {
				if row.Monitored {
					data.Monitored++
				}
			}
		}
	}
	return data
}

func (h *Hub) settings(w http.ResponseWriter, status int, message, notice string) {
	h.settingsSection(w, status, message, notice, "connections")
}

func (h *Hub) settingsSection(w http.ResponseWriter, status int, message, notice, section string) {
	data := h.pageData()
	data.SettingsSection = validSettingsSection(section)
	data.SettingsNav = template.HTML(hubNavFor("Settings", settingsNavLabel(data.SettingsSection)))
	data.SettingsDraftClear = ""
	data.Message, data.Notice = message, notice
	h.render(w, status, settingsTemplate, data)
}

func (h *Hub) settingsRequest(w http.ResponseWriter, r *http.Request, status int, message, notice string) {
	data := h.pageData()
	query := r.URL.Query()
	data.SettingsSection = validSettingsSection(query.Get("section"))
	data.SettingsNav = template.HTML(hubNavFor("Settings", settingsNavLabel(data.SettingsSection)))
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
	h.render(w, status, settingsTemplate, data)
}

func validSettingsSection(section string) string {
	switch section {
	case "preferences", "files", "system":
		return section
	default:
		return "connections"
	}
}

func settingsNavLabel(section string) string {
	switch section {
	case "preferences":
		return "Preferences"
	case "files":
		return "Files"
	case "system":
		return "System"
	default:
		return "Connections"
	}
}

func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.headers(w, r)
	if "http://"+r.Host != h.origin {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if r.Method == http.MethodGet && (r.URL.Path == "/" || r.URL.Path == "/settings") {
		h.ensureCanvasHealth()
	}
	if r.URL.Path == "/canvas/test" {
		h.testCanvas(w, r)
		return
	}
	if r.URL.Path == "/rules/browse" {
		h.browseRules(w, r)
		return
	}
	if r.URL.Path == "/rules/test" {
		h.testRulesLocation(w, r)
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	switch r.URL.Path {
	case "/":
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "Method not allowed", 405)
			return
		}
		h.home(w, 200, "", "")
	case "/settings":
		if r.Method == http.MethodPost {
			h.saveSettings(w, r)
			return
		}
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "Method not allowed", 405)
			return
		}
		message, notice := h.takeFlash()
		h.settingsRequest(w, r, 200, message, notice)
	case "/setup":
		h.setup.initialOrigin = h.knownOrigin()
		h.setup.ServeHTTP(w, r)
	case "/setup/status":
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "Method not allowed", 405)
			return
		}
		saved := false
		if h.credentialSavedFn != nil {
			saved = h.credentialSavedFn()
		} else {
			_, credentialErr := platform.LoadCredential()
			saved = credentialErr == nil
		}
		outcome := "not_saved"
		if saved {
			outcome = "saved_not_tested"
		}
		if saved && h.setupSaved {
			outcome = "saved_this_session"
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(struct {
			CredentialSaved     bool   `json:"credential_saved"`
			SetupOutcome        string `json:"setup_outcome"`
			ValidationSucceeded bool   `json:"validation_succeeded"`
			ConnectionTested    bool   `json:"connection_tested"`
		}{saved, outcome, h.setupValidated, h.connectionTested})
	case "/_cse/hub-identity":
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "Method not allowed", 405)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(struct {
			InstallRoot string `json:"install_root"`
		}{hubcoord.CanonicalRoot(h.root)})
	case "/_cse/canvas-health":
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(struct {
			State   string `json:"state"`
			Note    string `json:"note"`
			Account string `json:"account"`
		}{h.canvasHealth, h.canvasHealthNote, h.canvasUser})
	case "/_cse/canvas-health.js":
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		_, _ = io.WriteString(w, canvasHealthScript)
	case "/_cse/drafts.js":
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		_, _ = io.WriteString(w, classroomDraftScript)
	case "/_cse/settings-drafts.js":
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		_, _ = io.WriteString(w, settingsDraftScript)
	case "/connect":
		h.connectionPage(w, r)
	case "/connect/extension":
		if r.Method != http.MethodGet || h.connection.Result.ExtensionPath == "" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Disposition", `attachment; filename="CanvasExpert-SE.mcpb"`)
		http.ServeFile(w, r, h.connection.Result.ExtensionPath)
	case "/students":
		h.studentsPage(w, r)
	case "/students/refresh":
		h.refreshStudents(w, r)
	case "/courses":
		h.coursesPage(w, r)
	case "/grading":
		h.saveGrading(w, r)
	case "/rules-location":
		h.saveRulesLocation(w, r)
	case "/how":
		h.howPage(w, r)
	case "/capabilities":
		h.capabilitiesPage(w, r)
	case "/playbook/replace":
		h.replacePlaybook(w, r)
	case "/canvas/forget":
		h.forgetCanvas(w, r)
	case "/quit":
		if !h.checkPost(w, r) {
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, "<!doctype html><html lang=\"en\"><meta charset=\"utf-8\"><title>Hub closed</title><style>"+funkyCSS+"</style><main class=\"done\"><h1>Bye!</h1><p>The hub has shut down. You can close this tab.</p></main></html>")
		h.quitOnce.Do(func() { close(h.quit) })
	default:
		http.NotFound(w, r)
	}
}

// knownOrigin prefers the workspace's configured instance, then a saved credential's.
func (h *Hub) knownOrigin() string {
	if h.root != "" {
		var config domain.CourseConfig
		if _, err := workspace.ReadDocument(filepath.Join(h.root, "context", "courses.md"), &config); err == nil && config.Instance != "" && !strings.Contains(config.Instance, "canvas.example.edu") {
			return config.Instance
		}
	}
	if c, err := platform.LoadCredential(); err == nil {
		return c.Origin
	}
	return ""
}

// RunHub serves every local page until ctx ends or the user quits. root may be
// empty for a development launch; no remembered folder is restored.
func RunHub(ctx context.Context, root, version string, output io.Writer) error {
	return RunHubWithOptions(ctx, root, version, output, HubOptions{})
}

// HubOptions selects the first browser page without changing saved settings.
type HubOptions struct {
	SetupFirst    bool
	InstallClient string
	// ReadyFile is a short-lived installer handshake containing no credentials.
	ReadyFile string
}

func RunHubWithOptions(ctx context.Context, root, version string, output io.Writer, options HubOptions) error {
	if err := platform.CredentialSelfTest(); err != nil {
		return err
	}
	if output == nil {
		output = io.Discard
	}
	var hubUnlock func()
	hubLocked := false
	defer func() {
		if hubLocked && hubUnlock != nil {
			hubUnlock()
		}
	}()
	if root != "" && options.ReadyFile == "" {
		root = hubcoord.CanonicalRoot(root)
		unlock, lockErr := hubcoord.Acquire(root, 5*time.Second)
		if lockErr != nil {
			return lockErr
		}
		hubUnlock, hubLocked = unlock, true
		if existing, ok := hubcoord.Find(root); ok {
			unlock()
			hubLocked = false
			return reportExistingHub(existing, root, output, options)
		}
	}
	listener, err := net.Listen("tcp4", preferredHubAddr)
	if err != nil {
		listener, err = net.Listen("tcp4", "127.0.0.1:0")
	}
	if err != nil {
		return errors.New("could not start the local hub")
	}
	defer listener.Close()
	var nonce [32]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		return errors.New("could not create the hub session secret")
	}
	origin := "http://" + listener.Addr().String()
	secret := hex.EncodeToString(nonce[:])
	hub := &Hub{origin: origin, secret: secret, version: version, ctx: ctx, quit: make(chan struct{}), installClient: options.InstallClient}
	if exe, linkErr := platform.LinkProgram(); linkErr == nil {
		hub.programDir = filepath.Dir(exe)
	} else if exe, exeErr := os.Executable(); exeErr == nil {
		hub.programDir = filepath.Dir(exe)
	}
	hub.setup = &setupHandler{origin: origin, secret: secret, ctx: ctx, validate: validateCredential, save: platform.SaveCredential, persistent: true, hub: true, done: make(chan struct{})}
	hub.setup.after = func(c platform.Credential) string {
		hub.canvasUser = ""
		hub.connectionTested = true
		hub.markCanvasValidated(c)
		if hub.root == "" {
			return ""
		}
		changed, err := workspace.SyncInstance(hub.root, c.Origin)
		logOutcome := "updated"
		if err != nil {
			logOutcome = "failed"
		} else if !changed {
			logOutcome = "unchanged"
		}
		rulesPath := ""
		if location, locationErr := workspace.GetRulesLocation(hub.root); locationErr == nil {
			rulesPath = location.RulesRoot
		}
		if logErr := installlog.Append(hub.root, version, installlog.Entry{Step: "origin_configuration", Client: hub.installClient, Outcome: logOutcome, ErrorCategory: map[bool]string{true: "configuration"}[err != nil], InstallPath: hub.root, RulesPath: rulesPath}); logErr != nil {
			fmt.Fprintln(output, "Install log unavailable; setup continues.")
		}
		switch {
		case err != nil:
			return "The credential was saved, but context/courses.md could not be updated; fix that file before running remote commands."
		case !changed:
			return "Saved origin differs from configured courses; update context/courses.md before remote commands."
		}
		return "CE-SE settings now use this Canvas address."
	}
	hub.setup.onValidation = func(valid bool) {
		hub.setupValidated = valid
		hub.setupSaved = false
		hub.connectionTested = false
		hub.canvasUser = ""
		hub.invalidateCanvasHealth()
	}
	hub.setup.onValidationResult = func(valid bool) {
		outcome := "failed"
		category := "validation"
		if valid {
			outcome, category = "succeeded", ""
		}
		if err := installlog.Append(hub.root, version, installlog.Entry{Step: "credential_validation", Client: hub.installClient, Outcome: outcome, ErrorCategory: category, InstallPath: hub.root}); err != nil {
			fmt.Fprintln(output, "Install log unavailable; setup continues.")
		}
	}
	hub.setup.onSaved = func() {
		hub.setupSaved = true
		if err := installlog.Append(hub.root, version, installlog.Entry{Step: "credential_save", Client: hub.installClient, Outcome: "succeeded", InstallPath: hub.root}); err != nil {
			fmt.Fprintln(output, "Install log unavailable; setup continues.")
		}
	}
	hub.setup.onSaveFailed = func() {
		if err := installlog.Append(hub.root, version, installlog.Entry{Step: "credential_save", Client: hub.installClient, Outcome: "failed", ErrorCategory: "credential_store", InstallPath: hub.root}); err != nil {
			fmt.Fprintln(output, "Install log unavailable; setup continues.")
		}
	}
	hub.root = root
	if root != "" {
		hub.seedNotice = strings.TrimSpace(strings.Join([]string{addMissingSeed(root), seedAdoptionNotice(root)}, " "))
	}
	server := &http.Server{Handler: hub, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 60 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10}
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(listener) }()
	if root != "" {
		if err = hubcoord.Write(root, origin); err != nil {
			return err
		}
		defer hubcoord.Remove(root, origin)
		// Publish the endpoint before releasing the lock so a concurrent caller
		// can validate and reuse this listener instead of binding another port.
		// The health probe may briefly precede Serve; the caller retries boundedly.
		hubUnlock()
		hubLocked = false
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
		_ = server.Close()
	}()
	_, _ = fmt.Fprintf(output, "CanvasExpert-SE hub is running: %s/ (Ctrl+C or the Quit button stops it)\n", origin)
	firstPage := "/"
	if options.SetupFirst {
		if _, credentialErr := platform.LoadCredential(); credentialErr != nil {
			firstPage = "/setup"
		} else {
			firstPage = "/settings"
		}
	}
	browserOutcome := "launch_requested"
	if err = platform.OpenBrowser(origin + firstPage); err != nil {
		browserOutcome = "launch_failed"
	}
	if root != "" {
		category := ""
		if browserOutcome == "launch_failed" {
			category = "browser"
		}
		if logErr := installlog.Append(root, version, installlog.Entry{Step: "browser", Outcome: browserOutcome, ErrorCategory: category, InstallPath: root}); logErr != nil {
			fmt.Fprintln(output, "Install log unavailable; hub continues.")
		}
	}
	if options.ReadyFile != "" {
		ready, marshalErr := json.Marshal(struct {
			URL            string `json:"url"`
			InstallRoot    string `json:"install_root"`
			SetupStatusURL string `json:"setup_status_url"`
			Reused         bool   `json:"reused"`
			BrowserOutcome string `json:"browser_outcome"`
		}{origin + firstPage, root, origin + "/setup/status", false, browserOutcome})
		if marshalErr != nil {
			return marshalErr
		}
		// Rename prevents the installing process from reading a partial marker.
		tempReady := options.ReadyFile + ".tmp"
		if err = os.WriteFile(tempReady, ready, 0o600); err != nil {
			return fmt.Errorf("could not report hub readiness: %w", err)
		}
		if err = os.Rename(tempReady, options.ReadyFile); err != nil {
			_ = os.Remove(tempReady)
			return fmt.Errorf("could not report hub readiness: %w", err)
		}
	}
	if browserOutcome == "launch_failed" {
		_, _ = fmt.Fprintln(output, "The browser could not open automatically; open the local URL above.")
	} else {
		_, _ = fmt.Fprintln(output, "A browser launch was requested for the local hub; this does not confirm that a page appeared.")
	}
	select {
	case <-hub.quit:
		return nil
	case <-ctx.Done():
		return nil
	case err = <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return errors.New("local hub stopped")
		}
		return nil
	}
}
