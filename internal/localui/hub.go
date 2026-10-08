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

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/platform"
	"canvasexpertse/internal/workspace"
)

// preferredHubAddr is tried first so a bookmark keeps working; the hub falls
// back to any free loopback port when it is taken.
const preferredHubAddr = "127.0.0.1:47831"

var homeTemplate = template.Must(template.New("home").Funcs(pageFuncs).Parse(`<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>CanvasExpert-SE hub</title><style>` + funkyCSS + `</style><main class="wide"><div class="badge">SE = {{se}}</div>` + hubNav + `<h1>Command central</h1><p>Everything local lives here. This page stays open until you quit it.</p>
{{if .Message}}<p class="error">{{.Message}}</p>{{end}}{{if .Notice}}<p class="ok">{{.Notice}}</p>{{end}}
<div class="cards">
<div class="card"><h2>Canvas</h2>{{if .Connected}}<span class="pill on">connected</span><p><code>{{.CanvasOrigin}}</code></p>{{else}}<span class="pill off">not connected</span><p>No saved token on this machine.</p>{{end}}<a class="go" href="/setup">{{if .Connected}}Replace token &rarr;{{else}}Connect &rarr;{{end}}</a></div>
<div class="card"><h2>Program</h2><span class="pill on">{{.Version}}</span><p><code>{{.Program}}</code></p><p>Code only. Nothing private is stored here.</p></div>
<div class="card"><h2>Workspace</h2>{{if .Root}}<span class="pill on">found</span><p><code>{{.Root}}</code></p><p>Private data: students, settings, courses.</p>{{else}}<span class="pill off">none</span><p>Point the hub at your synced workspace folder below.</p>{{end}}</div>
<div class="card"><h2>Students</h2>{{if .Root}}<p><b>{{.Students}}</b> known, <b>{{.Monitored}}</b> monitored</p><a class="go" href="/students">Edit aliases &rarr;</a>{{else}}<p>Needs a workspace.</p>{{end}}</div>
</div>
<form method="post" action="/workspace"><input type="hidden" name="request_secret" value="{{.Secret}}"><label for="path">Workspace folder (your synced, private one)</label><div class="inline"><input id="path" name="path" type="text" value="{{.Root}}" placeholder="C:\path\to\workspace" autocomplete="off" required><button type="submit">Use it</button></div></form>
<form method="post" action="/quit"><input type="hidden" name="request_secret" value="{{.Secret}}"><button type="submit">Quit the hub</button></form></main></html>`))

type homeData struct {
	Secret, Message, Notice, CanvasOrigin, Root string
	Program, Version                            string
	Connected                                   bool
	Students, Monitored                         int
}

// Hub is one long-lived loopback server holding every local page. Pages share
// a single per-run request secret and the same origin checks as the one-shot
// commands.
type Hub struct {
	origin, secret string
	stateFile      string
	programDir     string
	version        string
	mu             sync.Mutex
	root           string
	setup          *setupHandler
	quit           chan struct{}
	quitOnce       sync.Once
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
	h.rememberRoot()
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
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
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
	data := homeData{Secret: h.secret, Message: message, Notice: notice, Root: h.root, Program: h.programDir, Version: h.version}
	if c, err := platform.LoadCredential(); err == nil {
		data.Connected, data.CanvasOrigin = true, c.Origin
	}
	if h.root != "" {
		if s, err := workspace.OpenStudents(h.root); err == nil {
			data.Students = len(s.Rows)
			for _, row := range s.Rows {
				if row.Monitored {
					data.Monitored++
				}
			}
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_ = homeTemplate.Execute(w, data)
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
	case "/setup":
		h.setup.initialOrigin = h.knownOrigin()
		h.setup.ServeHTTP(w, r)
	case "/students":
		if h.root == "" {
			h.home(w, 400, "Choose a workspace folder first; the student table lives inside it.", "")
			return
		}
		store, err := workspace.OpenStudents(h.root)
		if err != nil {
			h.home(w, 400, "Could not read the student table: "+err.Error(), "")
			return
		}
		(&studentHandler{origin: h.origin, secret: h.secret, store: store, persistent: true, hub: true}).ServeHTTP(w, r)
	case "/workspace":
		if !h.checkPost(w, r) {
			return
		}
		root, err := workspace.ResolveRoot(strings.TrimSpace(r.PostForm.Get("path")), "")
		if err == nil && h.programDir != "" && platform.Overlaps(root, h.programDir) {
			err = errors.New("The workspace and the program must be separate folders. Pick your synced workspace, not the program folder or one containing it.")
		}
		if err != nil {
			h.home(w, 400, err.Error(), "")
			return
		}
		h.setRoot(root)
		h.home(w, 200, "", "Workspace set.")
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
		if _, err := workspace.ReadDocument(filepath.Join(h.root, "settings", "courses.md"), &config); err == nil && config.Instance != "" {
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
	hub := &Hub{origin: origin, secret: secret, stateFile: hubStateFile(), version: version, quit: make(chan struct{})}
	if exe, linkErr := platform.LinkProgram(); linkErr == nil {
		hub.programDir = filepath.Dir(exe)
	} else if exe, exeErr := os.Executable(); exeErr == nil {
		hub.programDir = filepath.Dir(exe)
	}
	hub.setup = &setupHandler{origin: origin, secret: secret, ctx: ctx, validate: validateCredential, save: platform.SaveCredential, persistent: true, hub: true, done: make(chan struct{})}
	hub.setup.after = func(c platform.Credential) string {
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
