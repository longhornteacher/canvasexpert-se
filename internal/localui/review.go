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

	"canvasexpertse/internal/platform"
	"canvasexpertse/internal/workflow"
)

// ReviewRevision is a card the teacher sent back to the agent.
type ReviewRevision struct {
	Ref  string `json:"ref"`
	Note string `json:"note"`
}

// ReviewOutcome is what the agent learns when the teacher finishes reviewing.
type ReviewOutcome struct {
	Status      string           `json:"outcome"` // completed, partial, held
	PostSummary string           `json:"summary,omitempty"`
	Path        string           `json:"result_path,omitempty"`
	Approved    []string         `json:"approved,omitempty"`
	Revise      []ReviewRevision `json:"revision_notes,omitempty"`
	Held        []string         `json:"held,omitempty"`
}

// ReviewPostResult carries the actual operation outcome and its saved artifact.
type ReviewPostResult struct{ Status, Summary, Path string }

type ReviewPoster func(ctx context.Context, skip map[string]bool) (ReviewPostResult, error)

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
	Comment string // comments display the way Canvas shows them: one paragraph
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
:root{color-scheme:light dark;--page:#f3f4f2;--surface:#fff;--surface-soft:#f8f9f7;--ink:#242824;--muted:#646b65;--line:#d8ddd8;--accent:#3e6252;--accent-soft:#e4ece7;--danger:#9a3434;--danger-soft:#f8e8e7;--good:#356347;--focus:#315e9a;--accent-ink:#fff;--private:#f5f1df;--private-line:#8f7a45}
@media(prefers-color-scheme:dark){:root{--page:#171a18;--surface:#222724;--surface-soft:#282e2a;--ink:#e7ebe7;--muted:#afb8b0;--line:#414a43;--accent:#a5c5b2;--accent-soft:#2a3b31;--danger:#ffaaaa;--danger-soft:#452c2b;--good:#a4d4b1;--focus:#8ab7f4;--accent-ink:#171a18;--private:#353326;--private-line:#9c8c54}}
*{box-sizing:border-box}body{font:15px/1.45 system-ui,-apple-system,"Segoe UI",sans-serif;margin:0;background:var(--page);color:var(--ink)}header,main{max-width:66rem;margin:0 auto;padding:1rem}header{padding-top:1.4rem;border-bottom:1px solid var(--line)}h1{font-size:1.5rem;line-height:1.2;margin:.2rem 0 .35rem}.sub{color:var(--muted)}header .sub{margin:.25rem 0}.intro{max-width:62rem}.review-card{background:var(--surface);border:1px solid var(--line);border-radius:.4rem;margin:.75rem 0;padding:.8rem 1rem;min-width:0}.review-card.held{border-left:4px solid var(--danger);background:var(--danger-soft)}.review-card.revise{border-left:4px solid #9a762e}.review-card.done{border-left:4px solid var(--good)}.top{display:flex;gap:.65rem;align-items:baseline;flex-wrap:wrap;border-bottom:1px solid var(--line);padding-bottom:.5rem}.ref{font-weight:650;overflow-wrap:anywhere}.total{font-size:1rem;font-weight:650}.tag{display:inline-block;font-size:.78rem;border:1px solid var(--line);border-radius:.25rem;padding:.05rem .4rem;background:var(--surface-soft);color:var(--muted)}.tag.warn{border-color:#9a762e;color:var(--ink)}.tag.bad{border-color:var(--danger);color:var(--danger)}pre,.comment{white-space:pre-wrap;overflow-wrap:anywhere;background:var(--surface-soft);border:1px solid var(--line);border-radius:.3rem;padding:.55rem .7rem;margin:.35rem 0;font:inherit}.work{max-height:25rem;overflow:auto}.private{background:var(--private);border:1px dashed var(--private-line);border-radius:.3rem;padding:.5rem .7rem;margin:.5rem 0;overflow-wrap:anywhere}.private b{font-size:.76rem;text-transform:uppercase;letter-spacing:.045em;color:var(--muted)}.private summary{cursor:pointer}iframe{width:100%;min-height:12rem;height:16rem;border:1px solid var(--line);border-radius:.3rem;background:var(--surface);resize:vertical}.frame-label,.evidence-label{font-size:.85rem;font-weight:650;margin:.6rem 0 .2rem}.facts{border-collapse:collapse;margin:.45rem 0;width:100%;overflow-wrap:anywhere}.facts th,.facts td{text-align:left;padding:.22rem .65rem .22rem 0;vertical-align:top;border-bottom:1px solid var(--line)}.facts th{font-weight:600;width:11rem}.hold{color:var(--danger);font-weight:650}.q{border-top:1px solid var(--line);margin-top:.55rem;padding-top:.5rem;overflow-wrap:anywhere}.choice{display:flex;gap:1rem;flex-wrap:wrap;border-top:1px solid var(--line);padding-top:.6rem;margin-top:.65rem}.choice label{display:inline-flex;align-items:center;gap:.35rem;font-weight:600}input[type=radio]{accent-color:var(--accent)}textarea{width:100%;min-height:4rem;max-width:100%;font:inherit;color:var(--ink);background:var(--surface);border:1px solid var(--line);border-radius:.3rem;padding:.5rem;display:none}textarea:focus-visible,button:focus-visible,input:focus-visible,summary:focus-visible,a:focus-visible{outline:2px solid var(--focus);outline-offset:2px}.review-card.revise textarea{display:block}.review-error{background:var(--danger-soft);border:1px solid var(--danger);border-radius:.35rem;padding:.6rem .75rem;overflow-wrap:anywhere}footer{position:sticky;bottom:0;background:var(--surface);border-top:1px solid var(--line);padding:.7rem max(1rem,calc((100vw - 64rem)/2));box-shadow:0 -3px 12px #00000012}footer>div{display:flex;gap:.6rem;align-items:center;flex-wrap:wrap;max-width:64rem;margin:0 auto}#summary{font-weight:600;margin-right:auto}.scope{color:var(--muted);font-size:.88rem;flex-basis:100%;order:2}button{font:inherit;padding:.45rem .75rem;border-radius:.3rem;border:1px solid var(--line);color:var(--ink);background:var(--surface-soft);cursor:pointer}button:hover{border-color:var(--accent)}button.primary{background:var(--accent);color:var(--accent-ink);border-color:var(--accent)}button:disabled{opacity:.5;cursor:not-allowed}main{padding-bottom:1.5rem}.done-page,.error-page{max-width:42rem;margin:3rem auto;padding:1.25rem;background:var(--surface);border:1px solid var(--line);border-radius:.4rem}.done-page h1,.error-page h1{font-size:1.35rem}.error-page{border-left:4px solid var(--danger)}.done-page p,.error-page p{overflow-wrap:anywhere}
@media(max-width:42rem){header,main{padding:.8rem}.review-card{padding:.7rem}.facts th{width:8rem}.top{gap:.45rem}footer{padding:.65rem .8rem}footer>div{gap:.45rem}#summary{flex-basis:100%}footer button{flex:1}.scope{order:3}.done-page,.error-page{margin:1rem;padding:1rem}}
</style><form method="post" action="/submit">
<header><h1>Review before posting</h1><div class="sub">{{.Kind}} &middot; {{.Title}}</div>
{{if .Notice}}<p class="sub"><b>Preview limits:</b> {{.Notice}}</p>{{end}}
<p class="intro">Everything below is approved by default. Revise an item to send it back to the agent with a note. Nothing has been written to Canvas yet. Teacher-only information is marked and is never posted. {{if not .AllOrNothing}}Text is shown as plain text, the way it will be written.{{else}}This is created as one piece: every item must be approved, and any revision sends the whole piece back.{{end}}</p>
{{if .Message}}<div class="review-error" role="alert">{{.Message}}</div>{{end}}</header>
<input type="hidden" name="request_secret" value="{{.Secret}}"><input type="hidden" name="token" value="{{.Token}}">
<main>{{range .Cards}}<section class="review-card {{if .Hold}}held{{else if eq .Status "revise"}}revise{{end}}{{if .Done}} done{{end}}" data-i="{{.Index}}" data-held="{{if or .Hold .Done}}1{{end}}">
<input type="hidden" name="ref_{{.Index}}" value="{{.Ref}}">
<div class="top"><span class="ref">{{if .Alias}}{{.Alias}} &middot; {{end}}{{.Ref}}</span>{{if .Attempt}}<span>Attempt {{.Attempt}}</span>{{end}}{{if .Total}}<span class="total">Proposed: {{.Total}}</span>{{end}}
{{if .Late}}<span class="tag warn">{{.Late}}</span>{{end}}{{if .Done}}<span class="tag">Already posted</span>{{else if .Hold}}<span class="tag bad">Held</span>{{else}}<span class="tag">Ready for review</span>{{end}}{{if .ChangedSinceRv}}<span class="tag warn">Changed since revision request</span>{{end}}</div>
{{if .Hold}}<p class="hold">Held, will not post: {{.Hold}}</p>{{end}}
{{if .Facts}}<table class="facts">{{range .Facts}}<tr><th>{{.Label}}</th><td>{{.Value}}</td></tr>{{end}}</table>{{end}}
{{range .Frames}}<div class="frame-label">{{.Label}}</div><iframe sandbox srcdoc="{{.HTML}}" title="{{.Label}}"></iframe>{{end}}
{{if .Key}}<details class="private"><summary><b>Answer key (teacher only)</b></summary><pre>{{.Key}}</pre></details>{{end}}
{{if .Submission}}<div class="evidence-label">Student submission</div><pre class="work">{{.Submission}}</pre>{{end}}
{{if .Comment}}<div class="evidence-label">Feedback to be posted</div><pre>{{.Comment}}</pre>{{end}}
{{range .Lines}}<div class="q"><b>{{.Name}}</b>{{if .Score}} &middot; {{.Score}}{{end}}{{if .Done}} <span class="tag">Posted</span>{{end}}
{{if .Hold}}<div class="hold">Held: {{.Hold}}</div>{{end}}{{if .Comment}}<div class="comment">{{.Comment}}</div><div class="sub">Canvas shows this to the student as one paragraph.</div>{{end}}
{{if .TeacherNote}}<div class="private"><b>Teacher only</b><br>{{.TeacherNote}}</div>{{end}}</div>{{end}}
{{if .TeacherNote}}<div class="private"><b>Teacher only (not posted)</b><br>{{.TeacherNote}}</div>{{end}}
{{if .LateBasis}}<div class="private"><b>Lateness, teacher only (not posted)</b><br>{{.LateBasis}}</div>{{end}}
{{if and (not .Hold) (not .Done)}}<div class="choice"><label><input type="radio" name="status_{{.Index}}" value="approved"{{if ne .Status "revise"}} checked{{end}}> Approved</label>
<label><input type="radio" name="status_{{.Index}}" value="revise"{{if eq .Status "revise"}} checked{{end}}> Revise</label></div>
<textarea name="note_{{.Index}}" placeholder="What should the agent change?">{{.Note}}</textarea>{{end}}
</section>{{end}}</main>
<footer><div><span id="summary">{{if .AllOrNothing}}Everything approved: go ahead{{else}}Post {{.Approved}} approved, {{.Held}} held{{end}}</span>
<button class="primary" id="postbtn" name="action" value="post">{{if .AllOrNothing}}{{if .Action}}{{.Action}}{{else}}Create in Canvas{{end}}{{else}}Post approved{{end}}</button><button name="action" value="revise">Send revisions only (post nothing)</button>
<span class="scope">{{if .AllOrNothing}}One-piece creation: all items must be approved; revision or hold means nothing can be written.{{else}}Only approved items can be posted. Held, already-posted and revised items are skipped.{{end}}</span></div></footer>
</form><script>
(function(){function upd(){var a=0,r=0,h=0;document.querySelectorAll('.review-card').forEach(function(c){
if(c.dataset.held){h++;return}var s=c.querySelector('input[type=radio]:checked');
if(s&&s.value==='revise'){r++;c.classList.add('revise')}else{a++;c.classList.remove('revise')}});
document.getElementById('postbtn').disabled=((r>0||h>0)&&{{if .AllOrNothing}}true{{else}}false{{end}});document.getElementById('summary').textContent=({{if .AllOrNothing}}h>0?'held: see the red item; nothing can be written':r>0?r+' item(s) need revision; writing is off':'Everything approved: go ahead'{{else}}'Post '+a+' approved, '+h+' held, '+r+' revised'{{end}})}
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
		reviewError(w, http.StatusInternalServerError, "Could not show this review", err.Error())
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	_ = reviewTemplate.Execute(w, v)
}

func reviewError(w http.ResponseWriter, status int, title, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, `<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>`+template.HTMLEscapeString(title)+`</title><style>:root{color-scheme:light dark;--page:#f3f4f2;--surface:#fff;--ink:#242824;--line:#d8ddd8;--danger:#9a3434;--danger-soft:#f8e8e7;--muted:#646b65}@media(prefers-color-scheme:dark){:root{--page:#171a18;--surface:#222724;--ink:#e7ebe7;--line:#414a43;--danger:#ffaaaa;--danger-soft:#452c2b;--muted:#afb8b0}}body{font:15px/1.45 system-ui,sans-serif;margin:0;padding:1rem;background:var(--page);color:var(--ink)}main{max-width:42rem;margin:2rem auto;padding:1.2rem;background:var(--surface);border:1px solid var(--line);border-left:4px solid var(--danger);border-radius:.4rem}h1{font-size:1.35rem;margin:.2rem 0 .6rem}.message{padding:.65rem;background:var(--danger-soft);border-radius:.3rem;overflow-wrap:anywhere}.muted{color:var(--muted)}@media(max-width:42rem){main{margin:.5rem;padding:1rem}}</style><main><h1>`+template.HTMLEscapeString(title)+`</h1><p class="message">`+template.HTMLEscapeString(message)+`</p><p class="muted">No additional review action was taken.</p></main></html>`)
}

func (h *reviewHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "same-origin")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'; frame-src about:")
	if "http://"+r.Host != h.origin {
		reviewError(w, http.StatusNotFound, "Review page not found", "This temporary review link is not available.")
		return
	}
	if r.Method == http.MethodGet && (r.URL.Path == "/" || r.URL.Path == "/review") {
		h.page(w, 200, "", nil)
		return
	}
	if r.Method != http.MethodPost || r.URL.Path != "/submit" {
		reviewError(w, http.StatusNotFound, "Review page not found", "This temporary review link is not available.")
		return
	}
	if r.Header.Get("Origin") != h.origin {
		reviewError(w, http.StatusForbidden, "Form origin rejected", "Open the local review page in the same browser tab and try again.")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := r.ParseForm(); err != nil {
		reviewError(w, http.StatusBadRequest, "Form is too large or invalid", "Nothing was posted. Return to the review page and try again.")
		return
	}
	if subtle.ConstantTimeCompare([]byte(r.PostForm.Get("request_secret")), []byte(h.secret)) != 1 {
		reviewError(w, http.StatusForbidden, "Form request rejected", "This form could not be verified. Return to the review page and try again.")
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.outcome != nil {
		reviewError(w, http.StatusGone, "Review is already finished", "This temporary review is complete. You can close this tab.")
		return
	}
	form := map[string]string{}
	for k := range r.PostForm {
		form[k] = r.PostForm.Get(k)
	}
	model, err := h.build()
	if err != nil {
		reviewError(w, http.StatusInternalServerError, "Could not show this review", err.Error())
		return
	}
	if r.PostForm.Get("token") != model.ReviewDigest() {
		h.page(w, 409, "The results file changed after this page was drawn. Nothing was posted. Check the cards again, then press the button.", form)
		return
	}
	state, err := workflow.LoadReviewState(h.path)
	if err != nil {
		reviewError(w, http.StatusInternalServerError, "Could not read saved review decisions", err.Error())
		return
	}
	out := ReviewOutcome{Status: "held", Path: h.path}
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
			reviewError(w, http.StatusBadRequest, "Review form no longer matches", "Nothing was posted. Reload the review page and check the cards again.")
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
		reviewError(w, http.StatusInternalServerError, "Could not save review notes", err.Error())
		return
	}
	if r.PostForm.Get("action") == "post" && len(out.Approved) > 0 {
		ctx, cancel := context.WithTimeout(h.ctx, 10*time.Minute)
		defer cancel()
		result, perr := h.post(ctx, skip)
		out.Status, out.PostSummary = result.Status, result.Summary
		if result.Path != "" {
			out.Path = result.Path
		}
		if out.Status == "succeeded" {
			out.Status = "completed"
		}
		if out.Status != "completed" && out.Status != "partial" && out.Status != "held" {
			out.Status = "partial"
		}
		if perr != nil {
			if out.Status == "completed" {
				out.Status = "partial"
			}
			out.PostSummary = strings.TrimSpace(result.Summary + " post error: " + perr.Error())
		}
	}
	h.outcome = &out
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	msg := fmt.Sprintf("%d approved, %d sent back for revision, %d held.", len(out.Approved), len(out.Revise), len(out.Held))
	msg = "Outcome: " + out.Status + ". " + msg + " Results are in the agent's session."
	_, _ = io.WriteString(w, `<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Review finished</title><style>:root{color-scheme:light dark;--page:#f3f4f2;--surface:#fff;--ink:#242824;--line:#d8ddd8;--muted:#646b65;--accent:#3e6252}@media(prefers-color-scheme:dark){:root{--page:#171a18;--surface:#222724;--ink:#e7ebe7;--line:#414a43;--muted:#afb8b0;--accent:#a5c5b2}}body{font:15px/1.45 system-ui,sans-serif;margin:0;padding:1rem;background:var(--page);color:var(--ink)}main{max-width:42rem;margin:2rem auto;padding:1.2rem;background:var(--surface);border:1px solid var(--line);border-left:4px solid var(--accent);border-radius:.4rem}h1{font-size:1.35rem;margin:.2rem 0 .6rem}.muted{color:var(--muted);overflow-wrap:anywhere}@media(max-width:42rem){main{margin:.5rem;padding:1rem}}</style><main><h1>Review outcome: `+template.HTMLEscapeString(out.Status)+`</h1><p>`+template.HTMLEscapeString(msg)+`</p>`)
	if out.PostSummary != "" {
		_, _ = io.WriteString(w, `<p>`+template.HTMLEscapeString(out.PostSummary)+`</p>`)
	}
	_, _ = io.WriteString(w, `<p class="muted">You can close this tab.</p></main></html>`)
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
	// Open the page for the teacher; the printed address is only a fallback.
	_ = platform.OpenBrowser(origin + "/review")
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
