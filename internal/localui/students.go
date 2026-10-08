package localui

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"canvasexpertse/internal/platform"
	"canvasexpertse/internal/workspace"
)

var studentsTemplate = template.Must(template.New("students").Parse(`<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>CanvasExpert-SE students</title><style>body{font:16px system-ui;color:#16304b;margin:2rem auto;max-width:75rem;padding:0 1rem}table{border-collapse:collapse;width:100%}th,td{text-align:left;padding:.7rem;border-bottom:1px solid #ddd}input,button{font:inherit;padding:.5rem}button{margin-top:1rem}.error{color:#a32323}</style><h1>Student aliases and monitoring</h1><p>Stable references keep existing scoring files attached to the same student when an alias changes.</p>{{if .Message}}<p class="error">{{.Message}}</p>{{end}}<form action="/students" method="post"><input type="hidden" name="request_secret" value="{{.Secret}}"><table><tr><th>Name</th><th>Canvas instance</th><th>Stable reference</th><th>Alias</th><th>Monitored</th></tr>{{range .Rows}}<tr><td>{{.Name}}</td><td>{{.Instance}}</td><td>{{.Ref}}</td><td><input name="alias_{{.Ref}}" value="{{.Alias}}" maxlength="80" required></td><td><input type="checkbox" name="monitored_{{.Ref}}" {{if .Monitored}}checked{{end}}></td></tr>{{else}}<tr><td colspan="5">Pull student work first to populate this table.</td></tr>{{end}}</table><button type="submit">Save and close</button></form></html>`))

type studentHandler struct {
	origin, secret string
	store          *workspace.StudentStore
	done           chan struct{}
	mu             sync.Mutex
	complete       bool
}

func (h *studentHandler) page(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_ = studentsTemplate.Execute(w, struct {
		Secret, Message string
		Rows            []workspace.Student
	}{h.secret, message, h.store.Rows})
}
func (h *studentHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "same-origin")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'")
	if "http://"+r.Host != h.origin || r.URL.Path != "/students" {
		http.NotFound(w, r)
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.complete {
		http.Error(w, "Student editor is already closed", 410)
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
	r.Body = http.MaxBytesReader(w, r.Body, 256<<10)
	if e := r.ParseForm(); e != nil {
		h.page(w, 400, "Invalid form.")
		return
	}
	if subtle.ConstantTimeCompare([]byte(r.PostForm.Get("request_secret")), []byte(h.secret)) != 1 {
		http.Error(w, "Form request rejected", 403)
		return
	}
	before := append([]workspace.Student(nil), h.store.Rows...)
	for i := range h.store.Rows {
		v := &h.store.Rows[i]
		a := strings.TrimSpace(r.PostForm.Get("alias_" + v.Ref))
		if a == "" || len([]rune(a)) > 80 || strings.ContainsAny(a, "\r\n\x00") {
			h.store.Rows = before
			h.page(w, 400, "Aliases need 1–80 characters, without line breaks.")
			return
		}
		v.Alias = a
		v.Monitored = r.PostForm.Get("monitored_"+v.Ref) == "on"
	}
	if e := h.store.Save(); e != nil {
		h.store.Rows = before
		h.page(w, 400, "Could not save. Aliases must be unique within each Canvas instance; check file access.")
		return
	}
	h.complete = true
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, "<!doctype html><html lang=\"en\"><meta charset=\"utf-8\"><title>Students saved</title><p>Student settings saved. You can close this page.</p></html>")
	close(h.done)
}
func Students(ctx context.Context, root string, output io.Writer) error {
	store, e := workspace.OpenStudents(root)
	if e != nil {
		return e
	}
	listener, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		return fmt.Errorf("could not start the student editor")
	}
	defer listener.Close()
	var nonce [32]byte
	if _, e = rand.Read(nonce[:]); e != nil {
		return fmt.Errorf("could not create the student form")
	}
	origin := "http://" + listener.Addr().String()
	h := &studentHandler{origin: origin, secret: hex.EncodeToString(nonce[:]), store: store, done: make(chan struct{})}
	server := &http.Server{Handler: h, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10}
	errs := make(chan error, 1)
	go func() { errs <- server.Serve(listener) }()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
		_ = server.Close()
	}()
	if output == nil {
		output = io.Discard
	}
	fmt.Fprintf(output, "Open the local student editor: %s/students\n", origin)
	if e = platform.OpenBrowser(origin + "/students"); e != nil {
		fmt.Fprintln(output, "The browser could not open automatically; open the local URL above.")
	}
	select {
	case <-h.done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("student editor canceled")
	case <-errs:
		return fmt.Errorf("student editor stopped")
	}
}
