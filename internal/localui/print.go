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

	"canvasexpertse/internal/platform"
	"canvasexpertse/internal/workflow"
)

var printTemplate = template.Must(template.New("print").Parse(`<!doctype html>
<html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Student reports</title>
<style>
body{margin:0;background:#f5f7fa;color:#1f2933;font-family:Segoe UI,Arial,sans-serif}
.bar{position:sticky;top:0;display:flex;gap:12px;align-items:center;padding:12px 16px;background:#fff;border-bottom:1px solid #d9e2ec}
.bar p{margin:0;flex:1}
button{font:inherit;padding:6px 14px;border:1px solid #9aa5b1;border-radius:6px;background:#fff;cursor:pointer}
button.primary{background:#1f5fbf;border-color:#1f5fbf;color:#fff}
.sheet{background:#fff;max-width:7.5in;margin:16px auto;padding:0.5in;box-shadow:0 1px 3px rgba(0,0,0,.15)}
@media print{body{background:#fff}.bar{display:none}.sheet{box-shadow:none;margin:0;padding:0;max-width:none;break-after:page;page-break-after:always}.sheet:last-child{break-after:auto;page-break-after:auto}}
</style>
<div class="bar"><p>{{len .Sheets}} student reports, one per page. Nothing here is sent anywhere.</p>
<button class="primary" type="button" onclick="window.print()">Print</button>
<form method="post" action="/done"><input type="hidden" name="request_secret" value="{{.Secret}}"><button>Done</button></form></div>
{{range .Sheets}}<section class="sheet">{{.}}</section>
{{end}}</html>`))

type printHandler struct {
	origin, secret string
	sheets         []template.HTML
	done           chan struct{}
	once           sync.Once
}

func (h *printHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "same-origin")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'")
	if "http://"+r.Host != h.origin {
		reviewError(w, http.StatusNotFound, "Page not found", "This temporary print link is not available.")
		return
	}
	switch {
	case r.Method == http.MethodGet && (r.URL.Path == "/" || r.URL.Path == "/print"):
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = printTemplate.Execute(w, struct {
			Sheets []template.HTML
			Secret string
		}{h.sheets, h.secret})
	case r.Method == http.MethodPost && r.URL.Path == "/done" && r.Header.Get("Origin") == h.origin:
		r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
		if r.ParseForm() != nil || subtle.ConstantTimeCompare([]byte(r.PostForm.Get("request_secret")), []byte(h.secret)) != 1 {
			reviewError(w, http.StatusForbidden, "Form request rejected", "Return to the print page and try again.")
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, `<!doctype html><html lang="en"><meta charset="utf-8"><title>Done</title><style>`+string(reviewCSS)+`</style><main class="end-page"><h1>Done</h1><p class="muted">You can close this tab.</p></main></html>`)
		h.once.Do(func() { close(h.done) })
	default:
		reviewError(w, http.StatusNotFound, "Page not found", "This temporary print link is not available.")
	}
}

// PrintStudentReports serves a temporary loopback page with one printable
// sheet per student and returns when the teacher clicks Done.
func PrintStudentReports(ctx context.Context, sheets []workflow.StudentSheet, output io.Writer) error {
	if output == nil {
		output = io.Discard
	}
	if len(sheets) == 0 {
		return errors.New("this run has no students to print")
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return errors.New("could not start the local print page")
	}
	defer listener.Close()
	var nonce [32]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		return errors.New("could not create the local print page")
	}
	h := &printHandler{origin: "http://" + listener.Addr().String(), secret: hex.EncodeToString(nonce[:]), done: make(chan struct{})}
	for _, s := range sheets {
		// Sheet HTML is built by the Engine with every value escaped.
		h.sheets = append(h.sheets, template.HTML(s.HTML))
	}
	server := &http.Server{Handler: h, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: time.Minute, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10}
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(listener) }()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	_, _ = fmt.Fprintf(output, "Open the local print page: %s/print\n", h.origin)
	_ = platform.OpenBrowser(h.origin + "/print")
	select {
	case <-h.done:
		return nil
	case <-ctx.Done():
		return nil
	case err = <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return errors.New("local print server stopped")
		}
		return nil
	}
}
