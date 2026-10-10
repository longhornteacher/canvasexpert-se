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
	"strconv"
	"strings"
	"sync"
	"time"

	"canvasexpertse/internal/workflow"
)

// ReviewRevision is a card the teacher sent back to the agent.
type ReviewRevision struct{ Ref, Note string }

// ReviewOutcome is what the agent learns when the teacher finishes reviewing.
type ReviewOutcome struct {
	Posted      bool
	PostSummary string
	Approved    []string // approved and sent to post (or ready, when not posted)
	Revise      []ReviewRevision
	Held        []string // "ref: reason"
}

// ReviewPoster posts everything except the skipped student refs and returns a short summary.
type ReviewPoster func(ctx context.Context, skip map[string]bool) (string, error)

type reviewView struct {
	Cards                 []reviewCardView
	Title, Kind, Secret   string
	Notice                string
	AllOrNothing          bool
	Action                string
	Token, Message, Notes string
	Approved, Held, Done  int
}
type reviewCardView struct {
	workflow.ReviewCard
	Index          int
	Status, Note   string
	ChangedSinceRv bool
	Collapsed      bool
	Lines          []reviewLineView
}
type reviewLineView struct {
	workflow.ReviewLine
	Comment string // classic comments display the way Canvas shows them: one paragraph
}

type reviewHandler struct {
	origin, secret, path string
	build                func() (workflow.ReviewModel, error)
	post                 ReviewPoster
	ctx                  context.Context
	mu                   sync.Mutex
	outcome              *ReviewOutcome
	done                 chan struct{}
}

var reviewTemplate = template.Must(template.New("review").Funcs(template.FuncMap{"inc": func(i int) int { return i + 1 }}).Parse(`<!doctype html>
<html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Review before writing to Canvas</title><style>
body{font:15px/1.45 system-ui,Segoe UI,sans-serif;margin:0;background:#f3f4f6;color:#111}
header,main,footer{max-width:60rem;margin:0 auto;padding:1rem}
h1{font-size:1.4rem;margin:.2rem 0}.sub{color:#555}
.card{background:#fff;border:1px solid #cbd5e1;border-radius:8px;margin:.7rem 0;padding:.8rem 1rem}
.card.held{border:2px solid #b91c1c;background:#fef2f2}.card.revise{border:2px solid #b45309;background:#fffbeb}
.card.done{opacity:.65}.top{display:flex;gap:.8rem;align-items:baseline;flex-wrap:wrap}
.ref{font-weight:600}.total{font-size:1.1rem;font-weight:700}.tag{font-size:.8rem;border-radius:4px;padding:.05rem .4rem;background:#e2e8f0}
.tag.warn{background:#fde68a}.tag.bad{background:#fecaca;color:#7f1d1d}
pre,.comment{white-space:pre-wrap;background:#f8fafc;border:1px solid #e2e8f0;border-radius:6px;padding:.5rem .7rem;margin:.4rem 0;font:inherit}
.private{background:#fef9c3;border:1px dashed #a16207;border-radius:6px;padding:.4rem .7rem;margin:.4rem 0}
.private b{font-size:.78rem;text-transform:uppercase;letter-spacing:.05em}
iframe{width:100%;height:16rem;border:1px solid #cbd5e1;background:#fff;resize:vertical}
.facts th{text-align:left;font-weight:600;padding-right:1rem;vertical-align:top}
.hold{color:#7f1d1d;font-weight:600}.q{border-top:1px solid #e2e8f0;margin-top:.5rem;padding-top:.4rem}
.choice label{margin-right:1rem}textarea{width:100%;min-height:3.2rem;font:inherit;display:none}
.card.revise textarea{display:block}
footer{position:sticky;bottom:0;background:#fff;border-top:2px solid #94a3b8;max-width:none}
footer div{max-width:58rem;margin:0 auto;display:flex;gap:1rem;align-items:center;flex-wrap:wrap}
button{font:inherit;padding:.5rem 1rem;border-radius:6px;border:1px solid #475569;background:#e2e8f0;cursor:pointer}
button.primary{background:#166534;color:#fff;border-color:#166534}.err{background:#fee2e2;border:1px solid #b91c1c;padding:.5rem .8rem;border-radius:6px}
</style><form method="post" action="/submit">
<header><h1>Review before posting</h1><div class="sub">{{.Kind}} &middot; {{.Title}}</div>
{{if .Notice}}<p class="sub"><b>Preview limits:</b> {{.Notice}}</p>{{end}}
<p class="sub">Everything below is approved by default. Click <b>Revise</b> on anything the agent should rework, add a note, then post. Nothing has been written to Canvas yet. Yellow boxes are teacher-only and are never written. {{if not .AllOrNothing}}Text is shown as plain text, the way it will be written.{{else}}This goes in as one piece: every item must be approved, and any Revise sends all of it back.{{end}}</p>
{{if .Message}}<div class="err">{{.Message}}</div>{{end}}</header>
<input type="hidden" name="request_secret" value="{{.Secret}}"><input type="hidden" name="token" value="{{.Token}}">
<main>{{range .Cards}}<section class="card {{if .Hold}}held{{else if eq .Status "revise"}}revise{{end}}{{if .Done}} done{{end}}" data-i="{{.Index}}" data-held="{{if or .Hold .Done}}1{{end}}">
<input type="hidden" name="ref_{{.Index}}" value="{{.Ref}}">
<div class="top"><span class="ref">{{if .Alias}}{{.Alias}} &middot; {{end}}{{.Ref}}</span>{{if .Attempt}}<span>attempt {{.Attempt}}</span>{{end}}{{if .Total}}<span class="total">{{.Total}}</span>{{end}}
{{if .Late}}<span class="tag warn">{{.Late}}</span>{{end}}{{if .Done}}<span class="tag">already posted</span>{{end}}{{if .ChangedSinceRv}}<span class="tag warn">changed since you asked for a revision</span>{{end}}</div>
{{if .Hold}}<p class="hold">Held, will not post: {{.Hold}}</p>{{end}}
{{if .Facts}}<table class="facts">{{range .Facts}}<tr><th>{{.Label}}</th><td>{{.Value}}</td></tr>{{end}}</table>{{end}}
{{range .Frames}}<div class="sub">{{.Label}}</div><iframe sandbox srcdoc="{{.HTML}}" title="{{.Label}}"></iframe>{{end}}
{{if .Key}}<details class="private"><summary><b>Answer key (teacher only)</b></summary><pre>{{.Key}}</pre></details>{{end}}
{{if .Comment}}<pre>{{.Comment}}</pre>{{end}}
{{range .Lines}}<div class="q"><b>{{.Name}}</b>{{if .Score}} &middot; {{.Score}}{{end}}{{if .Done}} <span class="tag">posted</span>{{end}}
{{if .Hold}}<div class="hold">Held: {{.Hold}}</div>{{end}}{{if .Comment}}<div class="comment">{{.Comment}}</div><div class="sub">Canvas shows this to the student as one paragraph.</div>{{end}}
{{if .TeacherNote}}<div class="private"><b>Teacher only</b><br>{{.TeacherNote}}</div>{{end}}</div>{{end}}
{{if .TeacherNote}}<div class="private"><b>Teacher only (not posted)</b><br>{{.TeacherNote}}</div>{{end}}
{{if and (not .Hold) (not .Done)}}<div class="choice"><label><input type="radio" name="status_{{.Index}}" value="approved"{{if ne .Status "revise"}} checked{{end}}> Approved</label>
<label><input type="radio" name="status_{{.Index}}" value="revise"{{if eq .Status "revise"}} checked{{end}}> Revise</label></div>
<textarea name="note_{{.Index}}" placeholder="What should the agent change?">{{.Note}}</textarea>{{end}}
</section>{{end}}</main>
<footer><div><span id="summary">{{if .AllOrNothing}}Everything approved: go ahead{{else}}Post {{.Approved}} approved, {{.Held}} held{{end}}</span>
<button class="primary" id="postbtn" name="action" value="post">{{if .AllOrNothing}}{{if .Action}}{{.Action}}{{else}}Create in Canvas{{end}}{{else}}Post approved{{end}}</button><button name="action" value="revise">Send revisions only (post nothing)</button></div></footer>
</form><script>
(function(){function upd(){var a=0,r=0,h=0;document.querySelectorAll('.card').forEach(function(c){
if(c.dataset.held){h++;return}var s=c.querySelector('input[type=radio]:checked');
if(s&&s.value==='revise'){r++;c.classList.add('revise')}else{a++;c.classList.remove('revise')}});
document.getElementById('postbtn').disabled=((r>0||h>0)&&{{if .AllOrNothing}}true{{else}}false{{end}});document.getElementById('summary').textContent=({{if .AllOrNothing}}h>0?'held: see the red box; nothing can be written':r>0?r+' item(s) need revision; writing is off':'Everything approved: go ahead'{{else}}'Post '+a+' approved, '+h+' held, '+r+' revised'{{end}})}
document.addEventListener('change',upd);upd()})();
</script></html>`))

func (h *reviewHandler) view(message string, form map[string]string) (reviewView, error) {
	model, err := h.build()
	if err != nil {
		return reviewView{}, err
	}
	state, err := workflow.LoadReviewState(h.path)
	if err != nil {
		return reviewView{}, err
	}
	v := reviewView{Title: model.Title, Kind: model.Kind, Notice: model.Notice, AllOrNothing: model.AllOrNothing, Action: model.Action, Secret: h.secret, Token: model.ReviewDigest(), Message: message}
	for i, c := range model.Cards {
		cv := reviewCardView{ReviewCard: c, Index: i, Status: "approved"}
		for _, l := range c.Lines {
			lv := reviewLineView{ReviewLine: l, Comment: strings.Join(strings.Fields(l.Comment), " ")}
			cv.Lines = append(cv.Lines, lv)
		}
		cv.ReviewCard.Lines = nil
		if item, ok := state.Items[c.Ref]; ok && item.Status == "revise" {
			if item.Digest == c.Digest {
				cv.Status, cv.Note = "revise", item.Note
			} else {
				cv.ChangedSinceRv = true
			}
		}
		if form != nil {
			if s, ok := form["status_"+strconv.Itoa(i)]; ok {
				cv.Status = s
			}
			cv.Note = form["note_"+strconv.Itoa(i)]
		}
		switch {
		case c.Hold != "":
			v.Held++
		case c.Done:
			v.Done++
		case cv.Status != "revise":
			v.Approved++
		}
		v.Cards = append(v.Cards, cv)
	}
	return v, nil
}

func (h *reviewHandler) page(w http.ResponseWriter, code int, message string, form map[string]string) {
	v, err := h.view(message, form)
	if err != nil {
		http.Error(w, "Could not render the results file: "+err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	_ = reviewTemplate.Execute(w, v)
}

func (h *reviewHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "same-origin")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'; frame-src about:")
	if "http://"+r.Host != h.origin {
		http.Error(w, "Not found", 404)
		return
	}
	if r.Method == http.MethodGet && (r.URL.Path == "/" || r.URL.Path == "/review") {
		h.page(w, 200, "", nil)
		return
	}
	if r.Method != http.MethodPost || r.URL.Path != "/submit" {
		http.Error(w, "Not found", 404)
		return
	}
	if r.Header.Get("Origin") != h.origin {
		http.Error(w, "Form origin rejected", 403)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Form is too large or invalid", 400)
		return
	}
	if subtle.ConstantTimeCompare([]byte(r.PostForm.Get("request_secret")), []byte(h.secret)) != 1 {
		http.Error(w, "Form request rejected", 403)
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.outcome != nil {
		http.Error(w, "Review is already finished", 410)
		return
	}
	form := map[string]string{}
	for k := range r.PostForm {
		form[k] = r.PostForm.Get(k)
	}
	model, err := h.build()
	if err != nil {
		http.Error(w, "Could not render the results file: "+err.Error(), 500)
		return
	}
	if r.PostForm.Get("token") != model.ReviewDigest() {
		h.page(w, 409, "The results file changed after this page was drawn. Nothing was posted. Check the cards again, then press the button.", form)
		return
	}
	state, err := workflow.LoadReviewState(h.path)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	out := ReviewOutcome{}
	skip := map[string]bool{}
	if model.AllOrNothing && r.PostForm.Get("action") == "post" {
		for i, c := range model.Cards {
			if c.Hold != "" {
				h.page(w, 409, "Held, nothing was written: "+c.Hold, form)
				return
			}
			if r.PostForm.Get("status_"+strconv.Itoa(i)) == "revise" {
				h.page(w, 409, "This is created whole, so every item must be approved. Use Send revisions only, or switch the item back to Approved.", form)
				return
			}
		}
	}
	for i, c := range model.Cards {
		if r.PostForm.Get("ref_"+strconv.Itoa(i)) != c.Ref {
			http.Error(w, "Form does not match the results file", 400)
			return
		}
		switch {
		case c.Hold != "":
			out.Held = append(out.Held, c.Ref+": "+c.Hold)
			skip[c.Ref] = true
		case c.Done:
			skip[c.Ref] = true
		case r.PostForm.Get("status_"+strconv.Itoa(i)) == "revise":
			note := strings.TrimSpace(r.PostForm.Get("note_" + strconv.Itoa(i)))
			out.Revise = append(out.Revise, ReviewRevision{Ref: c.Ref, Note: note})
			state.Items[c.Ref] = workflow.ReviewItem{Status: "revise", Note: note, Digest: c.Digest}
			skip[c.Ref] = true
		default:
			out.Approved = append(out.Approved, c.Ref)
			state.Items[c.Ref] = workflow.ReviewItem{Status: "approved", Digest: c.Digest}
		}
	}
	// Revise notes are saved before anything is written, so closing loses nothing.
	if err = workflow.SaveReviewState(h.path, state); err != nil {
		http.Error(w, "Could not save review notes: "+err.Error(), 500)
		return
	}
	if r.PostForm.Get("action") == "post" && len(out.Approved) > 0 {
		ctx, cancel := context.WithTimeout(h.ctx, 10*time.Minute)
		defer cancel()
		summary, perr := h.post(ctx, skip)
		out.Posted = true
		out.PostSummary = summary
		if perr != nil {
			out.PostSummary = strings.TrimSpace(summary + " post error: " + perr.Error())
		}
	}
	h.outcome = &out
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	msg := fmt.Sprintf("%d approved, %d sent back for revision, %d held.", len(out.Approved), len(out.Revise), len(out.Held))
	if out.Posted {
		msg = "Post finished. " + msg + " Results are in the agent's session."
	} else {
		msg = "Nothing was posted. " + msg
	}
	_, _ = io.WriteString(w, "<!doctype html><html lang=\"en\"><meta charset=\"utf-8\"><title>Review finished</title><body style=\"font:16px system-ui;max-width:40rem;margin:3rem auto\"><h1>Review finished</h1><p>"+template.HTMLEscapeString(msg)+"</p><p>You can close this tab.</p></body></html>")
	close(h.done)
}

// Review serves a temporary loopback review page for a results file and returns
// when the teacher clicks Post approved or Send revisions only.
func Review(ctx context.Context, path string, build func() (workflow.ReviewModel, error), post ReviewPoster, output io.Writer) (ReviewOutcome, error) {
	if output == nil {
		output = io.Discard
	}
	model, err := build()
	if err != nil {
		return ReviewOutcome{}, err
	}
	if len(model.Cards) == 0 {
		return ReviewOutcome{}, errors.New("results file has no students to review")
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return ReviewOutcome{}, errors.New("could not start the local review page")
	}
	defer listener.Close()
	var nonce [32]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		return ReviewOutcome{}, errors.New("could not create the local review form")
	}
	origin := "http://" + listener.Addr().String()
	h := &reviewHandler{origin: origin, secret: hex.EncodeToString(nonce[:]), path: path, build: build, post: post, ctx: ctx, done: make(chan struct{})}
	server := &http.Server{Handler: h, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 11 * time.Minute, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10}
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(listener) }()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
		_ = server.Close()
	}()
	_, _ = fmt.Fprintf(output, "Open the local review page: %s/review\n", origin)
	// The page is not opened for you: the agent shows it in the desktop app's browser pane (or the teacher opens the URL).
	select {
	case <-h.done:
		h.mu.Lock()
		defer h.mu.Unlock()
		return *h.outcome, nil
	case <-ctx.Done():
		return ReviewOutcome{}, errors.New("review canceled; saved revision notes are kept")
	case err = <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return ReviewOutcome{}, errors.New("local review server stopped")
		}
		return ReviewOutcome{}, nil
	}
}
