// Package localui provides temporary command-owned loopback forms.
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
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"canvasexpertse/internal/canvas"
	"canvasexpertse/internal/platform"
)

type setupHandler struct {
	origin, secret string
	root           string
	initialOrigin  string
	// knownOrigin, when set, supplies the pre-filled Canvas address afresh for
	// each page instead of initialOrigin.
	knownOrigin func() string
	ctx         context.Context
	validate    func(context.Context, platform.Credential) error
	save        func(platform.Credential) error
	done        chan struct{}
	mu          sync.Mutex
	complete    bool
	// persistent keeps the form usable after a save (the hub stays open).
	persistent bool
	hub        bool
	// after runs once a credential is saved and returns a note for the page.
	after func(platform.Credential) string
	// These callbacks update non-secret state in the owning hub session.
	onValidation       func(bool)
	onValidationResult func(bool)
	onSaved            func()
	onSaveFailed       func()
}

func init() {
	register(
		// Unlocked: validating a token is a slow network call; the setup handler has
		// its own mutex and its hub callbacks lock h.mu around hub fields.
		page{Path: "/setup", Methods: []string{http.MethodGet, http.MethodPost}, Template: "setup", Scripts: []string{"appearance.js"}, Unlocked: true,
			Handle: func(h *Hub, w http.ResponseWriter, r *http.Request) { h.setup.ServeHTTP(w, r) }},
		page{Path: "/setup/status", Handle: (*Hub).setupStatus},
	)
}

// setupStatus reports non-secret setup progress for the installer to poll.
func (h *Hub) setupStatus(w http.ResponseWriter, r *http.Request) {
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
}

func validateCredential(ctx context.Context, credential platform.Credential) error {
	client, err := canvas.New(credential.Origin, credential.Token)
	if err != nil {
		return err
	}
	var identity struct {
		ID int64 `json:"id"`
	}
	if err = client.Get(ctx, "/api/v1/users/self", &identity); err != nil {
		return err
	}
	if identity.ID <= 0 {
		return &canvas.Error{Kind: "validation"}
	}
	return nil
}

var schemeLike = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*:`)

// normalizeCanvasOrigin accepts a bare host ("school.instructure.com") or a
// pasted HTTPS Canvas link and reduces it to scheme://host[:port], dropping any
// path, query or fragment. Anything unsafe (other schemes, embedded
// credentials) is returned unchanged so canvas.New rejects it.
func normalizeCanvasOrigin(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}
	if !strings.Contains(raw, "://") {
		if m := schemeLike.FindString(raw); m != "" {
			rest := raw[len(m):]
			if rest == "" || rest[0] < '0' || rest[0] > '9' {
				return raw // javascript:, mailto:, etc.
			}
		}
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
		return raw
	}
	return "https://" + u.Host
}

func (h *setupHandler) prefill() string {
	if h.knownOrigin != nil {
		return h.knownOrigin()
	}
	return h.initialOrigin
}

func (h *setupHandler) page(w http.ResponseWriter, status int, message string) {
	h.pageNotice(w, status, message, "", h.prefill())
}

func (h *setupHandler) pageNotice(w http.ResponseWriter, status int, message, notice, origin string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_ = tmpl("setup").Execute(w, struct {
		Appearance                      appearanceData
		Secret, Message, Notice, Origin string
		Hub                             bool
	}{appearanceFor(h.root, h.secret), h.secret, message, notice, origin, h.hub})
}

func (h *setupHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "same-origin")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'")
	if h.root != "" {
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; script-src 'self'; connect-src 'self'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'")
	}
	if "http://"+r.Host != h.origin {
		http.NotFound(w, r)
		return
	}
	if serveAppearance(w, r, h.root, h.origin, h.secret) {
		return
	}
	if serveScript(w, r) {
		return
	}
	if r.URL.Path != "/setup" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if r.Method == http.MethodGet {
		h.page(w, 200, "")
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method not allowed", 405)
		return
	}
	if r.Header.Get("Origin") != h.origin {
		http.Error(w, "Form origin rejected", 403)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		h.page(w, 400, "Form is too large or invalid.")
		return
	}
	if subtle.ConstantTimeCompare([]byte(r.PostForm.Get("request_secret")), []byte(h.secret)) != 1 {
		http.Error(w, "Form request rejected", 403)
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.complete {
		http.Error(w, "Setup is already complete", 410)
		return
	}
	if h.persistent && h.hub && h.onValidation != nil {
		h.onValidation(false)
	}
	credential := platform.Credential{Origin: normalizeCanvasOrigin(r.PostForm.Get("origin")), Token: r.PostForm.Get("token")}
	client, err := canvas.New(credential.Origin, credential.Token)
	if err != nil {
		h.page(w, 400, "Use the HTTPS Canvas address and a nonempty access token.")
		return
	}
	credential.Origin = client.Origin()
	ctx, cancel := context.WithTimeout(h.ctx, 45*time.Second)
	defer cancel()
	if err = h.validate(ctx, credential); err != nil {
		if h.persistent && h.hub && h.onValidation != nil {
			h.onValidation(false)
		}
		if h.onValidationResult != nil {
			h.onValidationResult(false)
		}
		message := "Canvas could not be reached to validate this token. Check your network or Canvas availability and try again. Your saved credential has not changed."
		var ce *canvas.Error
		if errors.As(err, &ce) {
			if ce.Kind == "authentication" {
				message = "Canvas rejected this token. Check the Canvas address and that you copied the whole token. Your saved credential has not changed."
			} else {
				message = ce.Error() + ". Your saved credential has not changed; this does not show that your PAT needs replacing."
			}
		}
		h.page(w, 400, message)
		return
	}
	if h.persistent && h.hub && h.onValidation != nil {
		h.onValidation(true)
	}
	if h.onValidationResult != nil {
		h.onValidationResult(true)
	}
	if err = h.save(credential); err != nil {
		if h.persistent && h.hub && h.onSaveFailed != nil {
			h.onSaveFailed()
		}
		h.page(w, 500, "Windows could not save the credential. Try again or check local credential access.")
		return
	}
	if h.persistent && h.hub && h.onSaved != nil {
		h.onSaved()
	}
	if h.persistent {
		notice := "Canvas connection ready for " + credential.Origin + ". Your token is saved on this PC. Next, choose your courses."
		if h.after != nil {
			if extra := h.after(credential); extra != "" {
				notice += " " + extra
			}
		}
		h.pageNotice(w, 200, "", notice, credential.Origin)
		return
	}
	h.complete = true
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, "<!doctype html>"+appearanceHTML(h.root)+"<meta charset=\"utf-8\"><title>Canvas connected</title><style>"+string(pageCSS)+"</style><main class=\"done\"><h1>Canvas connection ready</h1><p>Your token is saved on this PC. Next, pick your courses or go back to your teaching app. You can close this page.</p></main></html>")
	close(h.done)
}

// Setup verifies the native credential encoding, opens a private ephemeral form,
// validates credentials, and returns after saving or command cancellation.
func Setup(ctx context.Context, output io.Writer) error {
	return SetupForOrigin(ctx, output, "")
}

// SetupForOrigin pre-fills only a public Canvas origin, never a stored token.
func SetupForOrigin(ctx context.Context, output io.Writer, initialOrigin string) error {
	return SetupForRoot(ctx, output, initialOrigin, "")
}

// SetupForRoot applies the appearance of the explicitly resolved UserInstall.
func SetupForRoot(ctx context.Context, output io.Writer, initialOrigin, root string) error {
	if err := platform.CredentialSelfTest(); err != nil {
		return err
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return errors.New("could not start the local setup page")
	}
	defer listener.Close()
	var nonce [32]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		return errors.New("could not create the local setup form")
	}
	origin := "http://" + listener.Addr().String()
	handler := &setupHandler{origin: origin, secret: hex.EncodeToString(nonce[:]), initialOrigin: initialOrigin, root: root, ctx: ctx, validate: validateCredential, save: platform.SaveCredential, done: make(chan struct{})}
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 60 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10}
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(listener) }()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
		_ = server.Close()
	}()
	if output == nil {
		output = io.Discard
	}
	_, _ = fmt.Fprintf(output, "Open the local setup page: %s/setup\n", origin)
	if err = platform.OpenBrowser(origin + "/setup"); err != nil {
		_, _ = fmt.Fprintln(output, "The browser could not open automatically; open the local URL above.")
	}
	select {
	case <-handler.done:
		_, _ = fmt.Fprintln(output, "Canvas credentials saved on this machine.")
		return nil
	case <-ctx.Done():
		return errors.New("local setup canceled")
	case err = <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return errors.New("local setup server stopped")
		}
		return nil
	}
}
