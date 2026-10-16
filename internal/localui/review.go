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

// ReviewTester runs the page's "Send a test to me" button; it changes nothing the review tracks.
type ReviewTester func(ctx context.Context) (string, error)

type reviewActionKey struct{}

// ReviewActionFrom reports which extra page button (workflow.ReviewAction.Value)
// started a post; it is empty for the ordinary post button.
func ReviewActionFrom(ctx context.Context) string {
	v, _ := ctx.Value(reviewActionKey{}).(string)
	return v
}

type reviewView struct {
	Cards                 []reviewCardView
	Title, Kind, Secret   string
	Notice                string
	AllOrNothing          bool
	Action                string
	Token, Message, Notes string
	Approved, Held, Done  int
	PostLabel             string
	Actions               []workflow.ReviewAction
	Test                  bool
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
	test                 ReviewTester
	ctx                  context.Context
	mu                   sync.Mutex
	outcome              *ReviewOutcome
	done                 chan struct{}
}

var reviewTemplate = pageTemplate("review")

func (h *reviewHandler) view(message string, form map[string]string) (reviewView, error) {
	model, err := h.build()
	if err != nil {
		return reviewView{}, err
	}
	state, err := workflow.LoadReviewState(h.path)
	if err != nil {
		return reviewView{}, err
	}
	v := reviewView{Title: model.Title, Kind: model.Kind, Notice: model.Notice, AllOrNothing: model.AllOrNothing, Action: model.Action, PostLabel: model.PostLabel, Actions: model.Actions, Test: model.Test && h.test != nil, Secret: h.secret, Token: model.ReviewDigest(), Message: message}
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
	_, _ = io.WriteString(w, `<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>`+template.HTMLEscapeString(title)+`</title><style>`+string(reviewCSS)+`</style><main class="end-page error"><h1>`+template.HTMLEscapeString(title)+`</h1><p class="message">`+template.HTMLEscapeString(message)+`</p><p class="muted">No additional review action was taken.</p></main></html>`)
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
	if r.Method != http.MethodPost || (r.URL.Path != "/submit" && r.URL.Path != "/test") {
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
	if r.URL.Path == "/test" {
		if h.test == nil || h.outcome != nil {
			reviewError(w, http.StatusNotFound, "Review page not found", "This temporary review link is not available.")
			return
		}
		ctx, cancel := context.WithTimeout(h.ctx, 2*time.Minute)
		defer cancel()
		msg, err := h.test(ctx)
		if err != nil {
			msg = "Test email not sent: " + err.Error()
		}
		h.page(w, 200, msg, nil)
		return
	}
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
	action := r.PostForm.Get("action")
	extra := false
	for _, a := range model.Actions {
		extra = extra || a.Value == action
	}
	if (action == "post" || extra) && len(out.Approved) > 0 {
		ctx, cancel := context.WithTimeout(h.ctx, 10*time.Minute)
		defer cancel()
		if extra {
			ctx = context.WithValue(ctx, reviewActionKey{}, action)
		}
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
	msg = "Your review is saved: " + msg + " The agent has the results, so you can go back to your chat."
	_, _ = io.WriteString(w, `<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Review finished</title><style>`+string(reviewCSS)+`</style><main class="end-page"><h1>Review outcome: `+template.HTMLEscapeString(out.Status)+`</h1><p>`+template.HTMLEscapeString(msg)+`</p>`)
	if out.PostSummary != "" {
		_, _ = io.WriteString(w, `<p>`+template.HTMLEscapeString(out.PostSummary)+`</p>`)
	}
	_, _ = io.WriteString(w, `<p class="muted">You can close this tab.</p></main></html>`)
	close(h.done)
}

// Review serves a temporary loopback review page for a results file and returns
// when the teacher clicks Post approved or Send revisions only.
func Review(ctx context.Context, path string, build func() (workflow.ReviewModel, error), post ReviewPoster, output io.Writer) (ReviewOutcome, error) {
	return ReviewWithTest(ctx, path, build, post, nil, output)
}

// ReviewWithTest is Review plus an optional test action for the page.
func ReviewWithTest(ctx context.Context, path string, build func() (workflow.ReviewModel, error), post ReviewPoster, test ReviewTester, output io.Writer) (ReviewOutcome, error) {
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
	h := &reviewHandler{origin: origin, secret: hex.EncodeToString(nonce[:]), path: path, build: build, post: post, test: test, ctx: ctx, done: make(chan struct{})}
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
