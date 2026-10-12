// Package localui provides temporary command-owned loopback forms.
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
	"sync"
	"time"

	"canvasexpertse/internal/canvas"
	"canvasexpertse/internal/platform"
)

var setupTemplate = template.Must(template.New("setup").Parse(`<!doctype html>
<html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>CanvasExpert-SE setup</title><style>` + funkyCSS + `</style><main><div class="badge">CanvasExpert: SE</div>{{if .Hub}}` + hubNav + `{{end}}<h1>Plug in Canvas!</h1><p>Your access token naps inside Windows Credential Manager on this machine. It never touches a file.</p>
{{if .Message}}<p class="error">{{.Message}}</p>{{end}}{{if .Notice}}<p class="ok">{{.Notice}}</p>{{end}}
<form method="post" action="/setup" autocomplete="off">
<input type="hidden" name="request_secret" value="{{.Secret}}">
<label for="origin">Canvas address</label><input id="origin" name="origin" type="url" value="{{.Origin}}" placeholder="https://your-school.instructure.com" required autocomplete="off">
<label for="token">Secret squirrel token</label><input id="token" name="token" type="password" required autocomplete="off" spellcheck="false">
<button type="submit">Connect &amp; stash it</button></form></main></html>`))

type setupHandler struct {
	origin, secret string
	initialOrigin  string
	ctx            context.Context
	validate       func(context.Context, platform.Credential) error
	save           func(platform.Credential) error
	done           chan struct{}
	mu             sync.Mutex
	complete       bool
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

func (h *setupHandler) page(w http.ResponseWriter, status int, message string) {
	h.pageNotice(w, status, message, "")
}

func (h *setupHandler) pageNotice(w http.ResponseWriter, status int, message, notice string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_ = setupTemplate.Execute(w, struct {
		Secret, Message, Notice, Origin string
		Hub                             bool
	}{h.secret, message, notice, h.initialOrigin, h.hub})
}

func (h *setupHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "same-origin")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'")
	if "http://"+r.Host != h.origin || r.URL.Path != "/setup" {
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
	credential := platform.Credential{Origin: r.PostForm.Get("origin"), Token: r.PostForm.Get("token")}
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
				message = "Canvas rejected this token. Check the Canvas address and token. Your saved credential has not changed."
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
		notice := "Connected to " + credential.Origin + ". Token saved on this machine."
		if h.after != nil {
			if extra := h.after(credential); extra != "" {
				notice += " " + extra
			}
		}
		h.initialOrigin = credential.Origin
		h.pageNotice(w, 200, "", notice)
		return
	}
	h.complete = true
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, "<!doctype html><html lang=\"en\"><meta charset=\"utf-8\"><title>Canvas connected</title><style>"+funkyCSS+"</style><main class=\"done\"><h1>Connected!</h1><p>Credentials stashed on this machine. Go close this tab, you wizard.</p></main></html>")
	close(h.done)
}

// Setup verifies the native credential encoding, opens a private ephemeral form,
// validates credentials, and returns after saving or command cancellation.
func Setup(ctx context.Context, output io.Writer) error {
	return SetupForOrigin(ctx, output, "")
}

// SetupForOrigin pre-fills only a public Canvas origin, never a stored token.
func SetupForOrigin(ctx context.Context, output io.Writer, initialOrigin string) error {
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
	handler := &setupHandler{origin: origin, secret: hex.EncodeToString(nonce[:]), initialOrigin: initialOrigin, ctx: ctx, validate: validateCredential, save: platform.SaveCredential, done: make(chan struct{})}
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
