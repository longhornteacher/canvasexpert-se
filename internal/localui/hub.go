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

var homeTemplate = template.Must(template.New("home").Parse(`<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>CanvasExpert: Special Edition</title><style>` + funkyCSS + `</style><main class="wide"><div class="badge">CanvasExpert: SE</div>` + hubNav + `<h1>CanvasExpert: Special Edition</h1>
<p class="lead">You tell your AI agent what you need, like &ldquo;scoring session&rdquo; or &ldquo;make a new assignment&rdquo;. It reads your <b>playbooks</b> (how to do each job) and your <b>context</b> (your voice, students, courses, calendar), then uses this program to work with Canvas safely. You stay in charge: nothing is posted unless you say so.</p>
{{if .Root}}<div class="card"><h2>Active locations</h2><p>Install: <code>{{.Root}}</code></p><p>Teacher rules ({{.RulesMode}}): <code>{{.RulesRoot}}</code></p>{{if .RulesError}}<span class="pill off">{{if eq .RulesMode "sync"}}Sync unavailable{{else}}Rules location unavailable{{end}}</span><p class="error">{{.RulesError}}</p>{{end}}</div>{{end}}
<ol class="steps"><li><b>Connect Canvas</b> with an access token.</li><li><b>Pick your courses</b> and add student notes.</li><li><b>Connect CE-SE to your agent</b> and ask for work.</li></ol>
<p class="reassure">Student names and pulled work stay in this local CE-SE installation. Your token stays in Windows Credential Manager. The agent is told never to open the private student-name file, and your token is never written to a file.</p>
{{if .Message}}<p class="error">{{.Message}}</p>{{end}}{{if .Notice}}<p class="ok">{{.Notice}}</p>{{end}}
<div class="cards">
<div class="card"><h2>Your teaching assistant</h2><p>Connect this Install to Cowork or desktop Work, then check the local tools.</p><a class="go" href="/connect">Connect your agent &rarr;</a></div>
<div class="card token {{if .ConnectionTested}}t-green{{else if .Connected}}t-yellow{{else}}t-red{{end}}"><h2>Canvas token <button type="button" class="qmark" popovertarget="pat-help" aria-label="How do I get a Canvas access token?">?</button></h2>{{if .ConnectionTested}}<span class="pill on">tested this session</span><p>Canvas accepted the saved token in this hub session.</p>{{else if .Connected}}<span class="pill warn">saved, not tested</span><p>A token is saved on this computer. Test it to make sure Canvas still accepts it.</p>{{else}}<span class="pill off">not connected</span><p>The agent can't reach Canvas until you add a token.</p>{{end}}<a class="go" href="{{if .Connected}}/settings{{else}}/setup{{end}}">{{if .Connected}}Test or replace it &rarr;{{else}}Connect Canvas &rarr;{{end}}</a></div>
<div class="card"><h2>Courses</h2>{{if .Root}}<p><b>{{.ActiveCourses}}</b> active</p><a class="go" href="/courses">Pick active courses &rarr;</a>{{else}}<p>Launch CE-SE from its installation folder to get started.</p><a class="go" href="/settings">Open Settings &rarr;</a>{{end}}</div>
<div class="card"><h2>Students</h2>{{if .Root}}<p><b>{{.Students}}</b> known, <b>{{.Monitored}}</b> monitored</p><a class="go" href="/students">Open students &rarr;</a>{{else}}<p>Launch CE-SE from its installation folder to get started.</p>{{end}}</div>
<div class="card"><h2>Playbooks &amp; context</h2><p>Feedback voice, how each job is done, and your starter files.</p><a class="go" href="/settings">Open Settings &rarr;</a></div>
<div class="card"><h2>New here?</h2><p>One folder: the Engine, your playbooks and your private student information.</p><a class="go" href="/how">See how it works &rarr;</a></div>
<div class="card"><h2>Capabilities</h2><p>Green, yellow, and red lights for what CE-SE can do with Canvas Live.</p><a class="go" href="/capabilities">See the capability map &rarr;</a></div>
</div>
<div popover id="pat-help"><h2>Get a Canvas access token</h2><ol><li>Sign in to Canvas in your browser.</li><li>Click your profile picture (or <b>Account</b>), then <b>Settings</b>.</li><li>Scroll to <b>Approved Integrations</b> and click <b>+ New Access Token</b>.</li><li>Purpose: <code>CanvasExpert SE</code>. Leave the expiry blank, or pick a date you'll remember.</li><li>Click <b>Generate Token</b> and copy it right away. Canvas shows it only once.</li><li>Back here, choose <b>Connect Canvas</b> and paste it. It is kept in Windows Credential Manager, not in a file.</li></ol><p>If your district hides this option, ask your Canvas admin for a teacher access token.</p><button type="button" class="small" popovertarget="pat-help" popovertargetaction="hide">Close</button></div>
<form method="post" action="/quit"><input type="hidden" name="request_secret" value="{{.Secret}}"><button type="submit">Quit the hub</button></form></main></html>`))

var settingsTemplate = template.Must(template.New("settings").Parse(`<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>CanvasExpert-SE settings</title><style>` + funkyCSS + `</style><main class="wide"><div class="badge">CanvasExpert: SE</div>` + hubNav + `<h1>Settings</h1><p>Manage your Canvas connection and CE-SE settings.</p>
{{if .Message}}<p class="error">{{.Message}}</p>{{end}}{{if .Notice}}<p class="ok">{{.Notice}}</p>{{end}}<div class="cards">
<div class="card"><h2>Canvas</h2>{{if .Connected}}{{if .ConnectionTested}}<span class="pill on">tested this session</span>{{else}}<span class="pill warn">saved, not tested</span>{{end}}<p><code>{{.CanvasOrigin}}</code></p>{{if .CanvasUser}}<p>Signed in as <b>{{.CanvasUser}}</b></p>{{end}}<form method="post" action="/canvas/test"><input type="hidden" name="request_secret" value="{{.Secret}}"><button type="submit" class="small">Test connection</button></form>{{else}}<span class="pill off">not connected</span><p>No saved token on this machine.</p>{{end}}<a class="go" href="/setup">{{if .Connected}}Replace token &rarr;{{else}}Connect &rarr;{{end}}</a>{{if .Connected}}<details class="forget"><summary>Forget token on this computer</summary><form method="post" action="/canvas/forget"><input type="hidden" name="request_secret" value="{{.Secret}}"><label class="check"><input type="checkbox" name="confirm" required> Yes, remove it. I can reconnect later.</label><button type="submit" class="small danger">Remove token</button></form></details>{{end}}</div>
<div class="card"><h2>Engine</h2><span class="pill on">{{.Version}}</span><p><code>{{.Program}}</code></p><p>The machinery plus the starter playbooks. Nothing private.</p></div>
<div class="card"><h2>CE-SE installation</h2>{{if .Root}}<span class="pill on">found</span><p><code>{{.Root}}</code></p><p>Engine, courses, student information and work stay here.</p>{{else}}<span class="pill off">none</span><p>For a development launch, use <code>cse --workspace &quot;PATH&quot; ui</code> with the CE-SE installation containing <code>context/courses.md</code>.</p>{{end}}</div>
{{if .Root}}<div class="card"><h2>Teacher rules: Install / Sync</h2><p>Install: <code>{{.Root}}</code></p><p>Effective rules location{{if .RulesMode}} ({{.RulesMode}}){{end}}: <code>{{.RulesRoot}}</code></p>{{if .RulesError}}<span class="pill off">Sync unavailable</span><p class="error">{{.RulesError}}</p>{{end}}<p>By default, teacher rules live in Install. Sync is optional: CE-SE reads and edits the selected folder directly, and its provider (such as OneDrive or Google Drive) carries changes between computers. Courses, students, work and the Engine remain in Install.</p><p>Selecting an empty Sync folder initializes it from the current shareable teacher rules. An existing Sync folder becomes authoritative; local originals are preserved but inactive. Switching back to Install copies the shared rules there and archives differing local versions.</p><form method="post" action="/rules-location"><input type="hidden" name="request_secret" value="{{.Secret}}"><label for="sync_root">Sync folder path (leave blank to use Install)</label><input id="sync_root" name="sync_root" value="{{.SyncRoot}}" maxlength="4000" autocomplete="off"><button type="submit">Save rules location</button></form></div>{{end}}
</div>
{{if .Preferences}}<div class="card"><h2>Student aliases</h2><form method="post" action="/settings"><input type="hidden" name="request_secret" value="{{.Secret}}"><label><input type="checkbox" name="aliases" {{if .Aliases}}checked{{end}}> Use aliases in student work files</label><button type="submit">Save settings</button></form></div>{{end}}
{{if .Root}}<div class="card voice"><h2>Feedback voice</h2><p><span class="pill">Part of your context</span></p><p>The agent reads this every time it writes feedback or scores work. Plain sentences are fine.</p><form method="post" action="/grading"><input type="hidden" name="request_secret" value="{{.Secret}}">
<label for="voice">How feedback should sound</label><textarea id="voice" name="voice" rows="4" maxlength="8000" placeholder="Warm and encouraging, short sentences a 7th grader can read. Start with one glow, then one grow. Use their first name.">{{.Voice}}</textarea>
<label for="feedback_guidance">Grading habits and rubric preferences</label><textarea id="feedback_guidance" name="feedback_guidance" rows="6" maxlength="8000" placeholder="Don't take off points for spelling in CS8. Accept late work without comment. Quote one line from their work when praising it.">{{.FeedbackGuidance}}</textarea>
<button type="submit">Save feedback voice</button></form></div>{{end}}{{if .Playbook}}<div class="card voice"><h2>Your playbooks and context</h2><p>The files the agent follows, compared with this program version's starter copy. Existing teacher copies of <code>AGENTS.md</code> and playbooks are not updated automatically. Use <b>Get the new version</b> to adopt changes, including the latest <code>AGENTS.md</code> instructions for opening the review page and stopping after an accepted post without re-checking Canvas.</p><ul class="playbook">{{range .Playbook}}<li><code>{{.Path}}</code> {{if eq .State "same"}}<span class="pill same">same as this version</span>{{else if eq .State "missing"}}<span class="pill off">missing</span>{{else if eq .State "unavailable"}}<span class="pill off">unavailable</span>{{else}}<span class="pill differs">yours / differs</span> <small>differs from this version's starter copy</small>{{end}}{{if .Replaceable}}<details class="forget"><summary>Get the new version</summary><form method="post" action="/playbook/replace"><input type="hidden" name="request_secret" value="{{$.Secret}}"><input type="hidden" name="file" value="{{.Path}}"><label class="check"><input type="checkbox" name="confirm" required> Replace my copy. Keep my old one in playbook-old.</label><button type="submit" class="small danger">Replace file</button></form></details>{{end}}</li>{{end}}</ul></div>{{end}}</main></html>`))

type homeData struct {
	Secret, Message, Notice, CanvasOrigin, Root string
	Program, Version                            string
	ActiveCourses                               int
	Connected                                   bool
	ConnectionTested                            bool
	Preferences, Aliases                        bool
	Students, Monitored                         int
	CanvasUser, Voice, FeedbackGuidance         string
	RulesMode, RulesRoot, SyncRoot, RulesError  string
	Playbook                                    []playbookRow
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
	listCoursesFn func(context.Context) ([]workflow.Course, string, error)
	rosterFn      func(context.Context, int64) ([]workflow.Enrollment, string, error)
	tagsFn        func(context.Context, int64) (map[int64][]string, string, error)
	whoamiFn      func(context.Context) (string, error)
	savedOriginFn func() string
	forgetFn      func() error
	// canvasUser is the account name from the last successful connection test.
	canvasUser       string
	setupValidated   bool
	connectionTested bool
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
			return "Some seed-managed instructions in Sync differ from this version. Review them in Settings → Your playbooks and context; choose any files to adopt. Nothing was replaced."
		}
	}
	return ""
}

func (h *Hub) headers(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "same-origin")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'")
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
	data := homeData{Secret: h.secret, Root: h.root, Program: h.programDir, Version: h.version}
	data.ConnectionTested = h.connectionTested
	if c, err := platform.LoadCredential(); err == nil {
		data.Connected, data.CanvasOrigin, data.CanvasUser = true, c.Origin, h.canvasUser
	}
	if h.root != "" {
		loc, rulesErr := workspace.GetRulesLocation(h.root)
		data.RulesMode, data.RulesRoot, data.SyncRoot = loc.Mode, loc.RulesRoot, ""
		if loc.Mode == "sync" {
			data.SyncRoot = loc.RulesRoot
		}
		if rulesErr != nil {
			data.RulesError = rulesErr.Error()
		}
		if config, _, err := h.readConfig(); err == nil {
			data.Preferences, data.Aliases = true, config.Aliases
			for _, c := range config.Courses {
				if c.Current {
					data.ActiveCourses++
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
	data := h.pageData()
	data.Message, data.Notice = message, notice
	h.render(w, status, settingsTemplate, data)
}

func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.headers(w)
	if "http://"+r.Host != h.origin {
		http.Error(w, "Not found", http.StatusNotFound)
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
		h.settings(w, 200, message, notice)
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
	case "/canvas/test":
		h.testCanvas(w, r)
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
