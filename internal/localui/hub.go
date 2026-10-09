package localui

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
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
	"canvasexpertse/internal/platform"
	"canvasexpertse/internal/workflow"
	"canvasexpertse/internal/workspace"
)

// preferredHubAddr is tried first so a bookmark keeps working; the hub falls
// back to any free loopback port when it is taken.
const preferredHubAddr = "127.0.0.1:47831"

var homeTemplate = template.Must(template.New("home").Parse(`<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>CanvasExpert: Special Edition</title><style>` + funkyCSS + `</style><main class="wide"><div class="badge">CanvasExpert: SE</div>` + hubNav + `<h1>CanvasExpert: Special Edition</h1><p>Your courses and students, together in one place.</p>
{{if .Message}}<p class="error">{{.Message}}</p>{{end}}{{if .Notice}}<p class="ok">{{.Notice}}</p>{{end}}
<div class="cards">
<div class="card"><h2>New here?</h2><a class="go" href="/how">See how it works &rarr;</a></div>
<div class="card"><h2>Courses</h2>{{if .Root}}<p><b>{{.ActiveCourses}}</b> active</p><a class="go" href="/courses">Pick active courses &rarr;</a>{{else}}<p>Choose a workspace in Settings to get started.</p><a class="go" href="/settings">Open Settings &rarr;</a>{{end}}</div>
<div class="card"><h2>Students</h2>{{if .Root}}<p><b>{{.Students}}</b> known, <b>{{.Monitored}}</b> monitored</p><a class="go" href="/students">Open students &rarr;</a>{{else}}<p>Choose a workspace in Settings to get started.</p>{{end}}</div>
<div class="card"><h2>Settings</h2>{{if .Connected}}<span class="pill on">Canvas connected</span>{{else}}<span class="pill off">Canvas not connected</span>{{end}}<p>Canvas connection, workspace, and preferences.</p><a class="go" href="/settings">Open Settings &rarr;</a></div>
</div><form method="post" action="/quit"><input type="hidden" name="request_secret" value="{{.Secret}}"><button type="submit">Quit the hub</button></form></main></html>`))

var settingsTemplate = template.Must(template.New("settings").Parse(`<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">{{if .Picking}}<meta http-equiv="refresh" content="2">{{end}}<title>CanvasExpert-SE settings</title><style>` + funkyCSS + `</style><main class="wide"><div class="badge">CanvasExpert: SE</div>` + hubNav + `<h1>Settings</h1><p>Manage your Canvas connection and workspace.</p>
{{if .Message}}<p class="error">{{.Message}}</p>{{end}}{{if .Notice}}<p class="ok">{{.Notice}}</p>{{end}}{{if .Picking}}<p class="ok">A folder window just opened. Choose your workspace folder there (it may be behind this window). This page updates by itself. Not seeing it? Check the taskbar, or cancel and paste the path instead.</p><form method="post" action="/browse-cancel"><input type="hidden" name="request_secret" value="{{.Secret}}"><button type="submit">Cancel</button></form>{{end}}
{{if .ConfirmPath}}<div class="card"><h2>Set up a workspace here?</h2><p><code>{{.ConfirmPath}}</code></p><p>This folder has {{.ConfirmCount}} items and is not a workspace yet. I will add the starter files only; nothing existing is changed or overwritten.</p><form method="post" action="/workspace"><input type="hidden" name="request_secret" value="{{.Secret}}"><input type="hidden" name="path" value="{{.ConfirmPath}}"><input type="hidden" name="confirm" value="1"><button type="submit">Yes, set it up</button></form></div>{{end}}
<div class="cards">
<div class="card"><h2>Canvas</h2>{{if .Connected}}<span class="pill on">connected</span><p><code>{{.CanvasOrigin}}</code></p>{{if .CanvasUser}}<p>Signed in as <b>{{.CanvasUser}}</b></p>{{end}}<form method="post" action="/canvas/test"><input type="hidden" name="request_secret" value="{{.Secret}}"><button type="submit" class="small">Test connection</button></form>{{else}}<span class="pill off">not connected</span><p>No saved token on this machine.</p>{{end}}<a class="go" href="/setup">{{if .Connected}}Replace token &rarr;{{else}}Connect &rarr;{{end}}</a>{{if .Connected}}<details class="forget"><summary>Forget token on this computer</summary><form method="post" action="/canvas/forget"><input type="hidden" name="request_secret" value="{{.Secret}}"><label class="check"><input type="checkbox" name="confirm" required> Yes, remove it. I can reconnect later.</label><button type="submit" class="small danger">Remove token</button></form></details>{{end}}</div>
<div class="card"><h2>Engine (program folder)</h2><span class="pill on">{{.Version}}</span><p><code>{{.Program}}</code></p><p>The machinery plus the starter Playbook. Nothing private.</p></div>
<div class="card"><h2>Your workspace</h2>{{if .Root}}<span class="pill on">found</span><p><code>{{.Root}}</code></p><p>Your Playbook and student information.</p>{{else}}<span class="pill off">none</span><p>Choose your synced workspace folder below.</p>{{end}}</div>
</div>
<label for="path">Workspace folder (your synced, private one)</label>
<div class="inline"><form method="post" action="/browse"><input type="hidden" name="request_secret" value="{{.Secret}}"><button type="submit">Browse&hellip;</button></form>
<form class="inline" method="post" action="/workspace"><input type="hidden" name="request_secret" value="{{.Secret}}"><input id="path" name="path" type="text" placeholder="or paste a full folder path" autocomplete="off" required><button type="submit">Use it</button></form></div>
<p>Pick any folder, even a new empty one. If it isn't a workspace yet, the starter files are created for you.</p>
{{if .Preferences}}<div class="card"><h2>Student aliases</h2><form method="post" action="/settings"><input type="hidden" name="request_secret" value="{{.Secret}}"><label><input type="checkbox" name="aliases" {{if .Aliases}}checked{{end}}> Use aliases in student work files</label><button type="submit">Save settings</button></form></div>{{end}}
{{if .Root}}<div class="card voice"><h2>Feedback voice</h2><p><span class="pill">Part of your Playbook</span></p><p>The agent reads this every time it writes feedback or scores work. Plain sentences are fine.</p><form method="post" action="/grading"><input type="hidden" name="request_secret" value="{{.Secret}}">
<label for="voice">How feedback should sound</label><textarea id="voice" name="voice" rows="4" maxlength="8000" placeholder="Warm and encouraging, short sentences a 7th grader can read. Start with one glow, then one grow. Use their first name.">{{.Voice}}</textarea>
<label for="feedback_guidance">Grading habits and rubric preferences</label><textarea id="feedback_guidance" name="feedback_guidance" rows="6" maxlength="8000" placeholder="Don't take off points for spelling in CS8. Accept late work without comment. Quote one line from their work when praising it.">{{.FeedbackGuidance}}</textarea>
<button type="submit">Save feedback voice</button></form></div>{{end}}{{if .Playbook}}<div class="card voice"><h2>Your Playbook</h2><p>The files the agent follows, compared with this program version's starter copy.</p><ul class="playbook">{{range .Playbook}}<li><code>{{.Path}}</code> {{if eq .State "same"}}<span class="pill same">same as this version</span>{{else if eq .State "missing"}}<span class="pill off">missing</span>{{else}}<span class="pill differs">yours / differs</span> <small>differs from this version's starter copy</small>{{end}}{{if .Replaceable}}<details class="forget"><summary>Get the new version</summary><form method="post" action="/playbook/replace"><input type="hidden" name="request_secret" value="{{$.Secret}}"><input type="hidden" name="file" value="{{.Path}}"><label class="check"><input type="checkbox" name="confirm" required> Replace my copy. Keep my old one in playbook-old.</label><button type="submit" class="small danger">Replace file</button></form></details>{{end}}</li>{{end}}</ul></div>{{end}}</main></html>`))

type homeData struct {
	Secret, Message, Notice, CanvasOrigin, Root string
	Program, Version                            string
	ConfirmPath                                 string
	ConfirmCount                                int
	ActiveCourses                               int
	Picking, Connected                          bool
	Preferences, Aliases                        bool
	Students, Monitored                         int
	CanvasUser, Voice, FeedbackGuidance         string
	Playbook                                    []playbookRow
}

type pickResult struct {
	path string
	err  error
}

// Hub is one long-lived loopback server holding every local page. Pages share
// a single per-run request secret and the same origin checks as the one-shot
// commands.
type Hub struct {
	origin, secret string
	stateFile      string
	programDir     string
	version        string
	ctx            context.Context
	chooser        func(context.Context) (string, error)
	picking        bool
	cancelPick     context.CancelFunc
	picked         *pickResult
	// flash carries one notice or problem across a post-redirect-get.
	flash, flashErr string
	// loaded tracks the courses refreshed from Canvas during this hub session.
	loaded map[int64]bool
	// Test seams; nil means use the saved Canvas credential.
	listCoursesFn func(context.Context) ([]workflow.Course, string, error)
	rosterFn      func(context.Context, int64) ([]workflow.Enrollment, string, error)
	tagsFn        func(context.Context, int64) (map[int64][]string, string, error)
	whoamiFn      func(context.Context) (string, error)
	forgetFn      func() error
	// canvasUser is the account name from the last successful connection test.
	canvasUser string
	// seedNotice reports starter files added when the hub opened the remembered
	// workspace; Home shows it once.
	seedNotice string
	mu            sync.Mutex
	root          string
	setup         *setupHandler
	quit          chan struct{}
	quitOnce      sync.Once
}

func hubStateFile() string {
	dir, err := platform.LocalDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "hub-workspace.txt")
}

func (h *Hub) rememberRoot() {
	if h.stateFile == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(h.stateFile), 0o700); err == nil {
		_ = os.WriteFile(h.stateFile, []byte(h.root+"\n"), 0o600)
	}
}

func (h *Hub) setRoot(root string) {
	h.root = root
	h.loaded = nil
	h.rememberRoot()
}

// addMissingSeed copies any Playbook seed file the workspace lacks (never
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
	data := homeData{Picking: h.picking, Secret: h.secret, Root: h.root, Program: h.programDir, Version: h.version}
	if c, err := platform.LoadCredential(); err == nil {
		data.Connected, data.CanvasOrigin, data.CanvasUser = true, c.Origin, h.canvasUser
	}
	if h.root != "" {
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
	h.homeWith(w, status, message, notice, "", 0)
}

func (h *Hub) homeWith(w http.ResponseWriter, status int, message, notice, confirmPath string, confirmCount int) {
	data := h.pageData()
	data.Message, data.Notice = message, notice
	data.ConfirmPath, data.ConfirmCount = confirmPath, confirmCount
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
		if result := h.picked; result != nil {
			h.picked = nil
			switch {
			case result.err != nil:
				h.settings(w, 400, result.err.Error(), "")
			case result.path == "":
				h.settings(w, 200, "", "No folder chosen.")
			default:
				h.chooseWorkspace(w, result.path, false)
			}
			return
		}
		message, notice := h.takeFlash()
		h.settings(w, 200, message, notice)
	case "/browse":
		if r.Method == http.MethodGet {
			http.Redirect(w, r, "/settings", http.StatusSeeOther)
			return
		}
		if !h.checkPost(w, r) {
			return
		}
		if !h.picking {
			h.picking = true
			base := h.ctx
			if base == nil {
				base = context.Background()
			}
			pickCtx, cancel := context.WithCancel(base)
			h.cancelPick = cancel
			go h.runChooser(pickCtx)
		}
		// Auto-refresh reloads Settings with GET, not this POST URL.
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
	case "/browse-cancel":
		if !h.checkPost(w, r) {
			return
		}
		if h.cancelPick != nil {
			h.cancelPick()
		}
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
	case "/setup":
		h.setup.initialOrigin = h.knownOrigin()
		h.setup.ServeHTTP(w, r)
	case "/students":
		h.studentsPage(w, r)
	case "/students/refresh":
		h.refreshStudents(w, r)
	case "/workspace":
		if !h.checkPost(w, r) {
			return
		}
		h.chooseWorkspace(w, r.PostForm.Get("path"), r.PostForm.Get("confirm") == "1")
	case "/courses":
		h.coursesPage(w, r)
	case "/grading":
		h.saveGrading(w, r)
	case "/how":
		h.howPage(w, r)
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

func (h *Hub) runChooser(ctx context.Context) {
	choose := h.chooser
	if choose == nil {
		choose = platform.PickFolder
	}
	path, err := choose(ctx)
	if errors.Is(err, context.Canceled) {
		path, err = "", nil
	}
	h.mu.Lock()
	h.picking = false
	h.picked = &pickResult{path: path, err: err}
	h.mu.Unlock()
}

// cleanPath tolerates quotes and spaces from a pasted path.
func cleanPath(raw string) string {
	return strings.Trim(strings.TrimSpace(raw), "\"'")
}

// chooseWorkspace adopts an existing workspace, or creates the starter files in
// an empty or new folder, or asks before adding them to a non-empty one.
func (h *Hub) chooseWorkspace(w http.ResponseWriter, raw string, confirmed bool) {
	path := cleanPath(raw)
	switch {
	case path == "":
		h.settings(w, 400, "Browse to a folder or paste its full path.", "")
		return
	case !filepath.IsAbs(path):
		h.settings(w, 400, "Use a full folder path such as C:\\Users\\you\\OneDrive\\Workspace.", "")
		return
	case h.programDir != "" && platform.Overlaps(path, h.programDir):
		h.settings(w, 400, "The workspace and the program must be separate folders. Pick your synced workspace, not the program folder or one containing it.", "")
		return
	}
	if root, err := workspace.ResolveRoot(path, ""); err == nil {
		h.setRoot(root)
		h.settings(w, 200, "", strings.TrimSpace("Workspace set. "+addMissingSeed(root)))
		return
	}
	info, err := os.Stat(path)
	switch {
	case err == nil && !info.IsDir():
		h.settings(w, 400, "That is a file, not a folder.", "")
		return
	case err != nil && !os.IsNotExist(err):
		h.settings(w, 400, "Could not open that folder: "+err.Error(), "")
		return
	case err == nil && !confirmed:
		if entries, readErr := os.ReadDir(path); readErr == nil && len(entries) > 0 {
			h.homeWith(w, 200, "", "", path, len(entries))
			return
		}
	}
	created, err := workspace.Scaffold(path, embedded.WorkspaceTemplate, "workspace-template")
	if err != nil {
		h.settings(w, 400, "Could not set up the workspace: "+err.Error(), "")
		return
	}
	root, err := workspace.ResolveRoot(path, "")
	if err != nil {
		h.settings(w, 400, err.Error(), "")
		return
	}
	h.setRoot(root)
	notice := fmt.Sprintf("Workspace created: %d starter files added; nothing was overwritten.", len(created))
	if c, credErr := platform.LoadCredential(); credErr == nil {
		if changed, syncErr := workspace.SyncInstance(root, c.Origin); syncErr == nil && changed {
			notice += " It already uses your connected Canvas address."
		}
	}
	h.settings(w, 200, "", notice)
}

// knownOrigin prefers the workspace's configured instance, then a saved credential's.
func (h *Hub) knownOrigin() string {
	if h.root != "" {
		var config domain.CourseConfig
		if _, err := workspace.ReadDocument(filepath.Join(h.root, "settings", "courses.md"), &config); err == nil && config.Instance != "" && !strings.Contains(config.Instance, "canvas.example.edu") {
			return config.Instance
		}
	}
	if c, err := platform.LoadCredential(); err == nil {
		return c.Origin
	}
	return ""
}

// RunHub serves every local page until ctx ends or the user quits. root may be
// empty; the last workspace the hub used is then restored when it still exists.
func RunHub(ctx context.Context, root, version string, output io.Writer) error {
	if err := platform.CredentialSelfTest(); err != nil {
		return err
	}
	if output == nil {
		output = io.Discard
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
	hub := &Hub{origin: origin, secret: secret, stateFile: hubStateFile(), version: version, ctx: ctx, quit: make(chan struct{})}
	if exe, linkErr := platform.LinkProgram(); linkErr == nil {
		hub.programDir = filepath.Dir(exe)
	} else if exe, exeErr := os.Executable(); exeErr == nil {
		hub.programDir = filepath.Dir(exe)
	}
	hub.setup = &setupHandler{origin: origin, secret: secret, ctx: ctx, validate: validateCredential, save: platform.SaveCredential, persistent: true, hub: true, done: make(chan struct{})}
	hub.setup.after = func(c platform.Credential) string {
		hub.canvasUser = ""
		if hub.root == "" {
			return ""
		}
		changed, err := workspace.SyncInstance(hub.root, c.Origin)
		switch {
		case err != nil:
			return "Could not update settings/courses.md."
		case !changed:
			return "Saved origin differs from configured courses; update settings/courses.md before remote commands."
		}
		return "Workspace settings now use this Canvas address."
	}
	switch {
	case root != "" && platform.Overlaps(root, hub.programDir):
		_, _ = fmt.Fprintln(output, "Ignoring that workspace: it overlaps the program folder. Choose a separate synced workspace in the hub.")
	case root != "":
		hub.root = root
		hub.rememberRoot()
	case hub.stateFile != "":
		if b, err := os.ReadFile(hub.stateFile); err == nil {
			if remembered, err := workspace.ResolveRoot(strings.TrimSpace(string(b)), ""); err == nil && !platform.Overlaps(remembered, hub.programDir) {
				hub.root = remembered
				hub.seedNotice = addMissingSeed(remembered)
			}
		}
	}
	server := &http.Server{Handler: hub, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 60 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10}
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(listener) }()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
		_ = server.Close()
	}()
	_, _ = fmt.Fprintf(output, "CanvasExpert-SE hub is running: %s/ (Ctrl+C or the Quit button stops it)\n", origin)
	if err = platform.OpenBrowser(origin + "/"); err != nil {
		_, _ = fmt.Fprintln(output, "The browser could not open automatically; open the local URL above.")
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
