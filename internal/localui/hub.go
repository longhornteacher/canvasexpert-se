package localui

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
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
	listCoursesFn     func(context.Context) ([]workflow.Course, string, error)
	rosterFn          func(context.Context, int64) ([]workflow.Enrollment, string, error)
	tagsFn            func(context.Context, int64) (map[int64][]string, string, error)
	whoamiFn          func(context.Context) (string, error)
	loadCredentialFn  func() (platform.Credential, error)
	savedOriginFn     func() string
	forgetFn          func() error
	pickFolderFn      func(context.Context) (string, error)
	desktopShortcutFn func(string) (string, error)
	// Team page seams; nil means Credential Manager and Canvas.
	loadTeammateFn     func(int64) (platform.Credential, error)
	saveTeammateFn     func(int64, platform.Credential) error
	deleteTeammateFn   func(int64) error
	teammateIdentityFn func(context.Context, platform.Credential) (teamIdentity, error)
	teammateCoursesFn  func(context.Context, platform.Credential) ([]workflow.Course, error)
	// canvasUser is the account name from the last successful connection test.
	canvasUser                                              string
	setupValidated                                          bool
	connectionTested                                        bool
	canvasHealth, canvasHealthNote, canvasHealthFingerprint string
	canvasHealthGeneration                                  uint64
	// canvasManualPending records a Test click made while an automatic check was
	// already running, so that check's result counts as the manual test.
	canvasManualPending bool
	canvasHealthTimeout time.Duration
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
			return "A newer version of some instruction files is available. To use one, open Settings, Files and choose Get the new version under it. Nothing was changed."
		}
	}
	return ""
}

func (h *Hub) headers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "same-origin")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	csp := "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'"
	if p := pages[r.URL.Path]; p != nil {
		if len(p.Scripts) > 0 {
			csp += "; script-src 'self'"
		}
		if p.Connect || hasAppearance(p) {
			csp += "; connect-src 'self'"
		}
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

func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.headers(w, r)
	if "http://"+r.Host != h.origin {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	// Static scripts need no hub state and are served before page lookup.
	if serveAppearance(w, r, h.root, h.origin, h.secret) {
		return
	}
	if serveScript(w, r) {
		return
	}
	p := pages[r.URL.Path]
	if p == nil {
		http.NotFound(w, r)
		return
	}
	h.servePage(p, w, r)
}

func init() {
	register(
		page{Path: "/_cse/hub-identity", Handle: (*Hub).hubIdentity},
		page{Path: "/quit", Methods: []string{http.MethodPost}, Handle: (*Hub).quitHub},
	)
}

func (h *Hub) hubIdentity(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		InstallRoot string `json:"install_root"`
	}{hubcoord.CanonicalRoot(h.root)})
}

func (h *Hub) quitHub(w http.ResponseWriter, r *http.Request) {
	if !h.checkPost(w, r) {
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, "<!doctype html>"+appearanceHTML(h.root)+"<meta charset=\"utf-8\"><title>Hub closed</title><style>"+string(pageCSS)+"</style><main class=\"done\"><h1>Bye!</h1><p>The hub has shut down. You can close this tab.</p></main></html>")
	h.quitOnce.Do(func() { close(h.quit) })
}

// knownOrigin prefers the workspace's configured instance, then a saved credential's.
func (h *Hub) knownOrigin() string {
	if h.root != "" {
		var config domain.CourseConfig
		if _, err := workspace.ReadTeacherDocument(h.root, "context/courses.md", &config); err == nil && config.Instance != "" && !strings.Contains(config.Instance, "canvas.example.edu") {
			return config.Instance
		}
	}
	if c, err := platform.LoadCredential(); err == nil {
		return c.Origin
	}
	return ""
}

// openBrowser is a seam so tests never launch a real browser.
var openBrowser = platform.OpenBrowser

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
	if root != "" {
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
	hub.setup = &setupHandler{origin: origin, secret: secret, root: root, ctx: ctx, validate: validateCredential, save: platform.SaveCredential, persistent: true, hub: true, done: make(chan struct{})}
	hub.setup.knownOrigin = hub.knownOrigin
	wireSetupCallbacks(hub, version, output)
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
	if err = openBrowser(origin + firstPage); err != nil {
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
		if err = writeHubReady(options.ReadyFile, origin+firstPage, root, origin+"/setup/status", false, browserOutcome); err != nil {
			return err
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

// wireSetupCallbacks connects the setup form to this hub's state, the UserInstall's
// course configuration and the install log. Split out so tests can drive the real
// callbacks with mocked validation and credential storage.
func wireSetupCallbacks(hub *Hub, version string, output io.Writer) {
	// The setup handler runs without hub.mu (its validation call is slow), so the
	// callbacks below take it around every hub field they touch.
	hub.setup.after = func(c platform.Credential) string {
		hub.mu.Lock()
		hub.canvasUser = ""
		hub.connectionTested = true
		hub.markCanvasValidated(c)
		hub.mu.Unlock()
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
			return "The credential was saved, but course settings were not changed. " + workspace.OriginMismatchMessage(workspace.ConfiguredInstance(hub.root), c.Origin)
		}
		return "CE-SE settings now use this Canvas address."
	}
	hub.setup.onValidation = func(valid bool) {
		hub.mu.Lock()
		defer hub.mu.Unlock()
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
		hub.mu.Lock()
		hub.setupSaved = true
		hub.mu.Unlock()
		if err := installlog.Append(hub.root, version, installlog.Entry{Step: "credential_save", Client: hub.installClient, Outcome: "succeeded", InstallPath: hub.root}); err != nil {
			fmt.Fprintln(output, "Install log unavailable; setup continues.")
		}
	}
	hub.setup.onSaveFailed = func() {
		if err := installlog.Append(hub.root, version, installlog.Entry{Step: "credential_save", Client: hub.installClient, Outcome: "failed", ErrorCategory: "credential_store", InstallPath: hub.root}); err != nil {
			fmt.Fprintln(output, "Install log unavailable; setup continues.")
		}
	}
}
