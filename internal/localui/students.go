package localui

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"canvasexpertse/internal/platform"
	"canvasexpertse/internal/workspace"
)


type courseTab struct {
	ID       int64
	Name     string
	Selected bool
}

type studentTag struct {
	Name, Class string
}

type studentRow struct {
	workspace.Student
	Tags []studentTag
	Note string
}

type studentsData struct {
	Secret, Message, Notice           string
	DraftKey                          string
	ClearDraft                        bool
	Rows                              []studentRow
	Hub, NotesOn                      bool
	Tabs                              []courseTab
	CourseID                          int64
	CourseName, UpdatedAt, TagMessage string
}
type studentHandler struct {
	origin, secret string
	store          *workspace.StudentStore
	done           chan struct{}
	mu             sync.Mutex
	complete       bool
	// persistent keeps the editor usable after a save (the hub stays open).
	persistent bool
	hub        bool
	data       studentsData
	// nil edits all rows in the standalone editor; the hub supplies the course roster.
	members map[string][]string
	// notesRoot, when set, enables the notes column backed by context/students.md.
	notesRoot string
}

func (h *studentHandler) page(w http.ResponseWriter, status int, message string) {
	h.pageNotice(w, status, message, "")
}

func (h *studentHandler) pageNotice(w http.ResponseWriter, status int, message, notice string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	data := h.data
	data.Secret, data.Message, data.Notice, data.Hub = h.secret, message, notice, h.hub
	if h.hub {
		data.DraftKey = "students:" + h.secret
	}
	data.Rows = nil
	var notes map[string]string
	if h.notesRoot != "" {
		if all, _, err := readNotes(h.notesRoot); err == nil {
			data.NotesOn, notes = true, notesByRef(all)
		} else if data.Message == "" {
			data.Message = "Could not read context/students.md, so notes are hidden: " + err.Error()
		}
	}
	for _, student := range h.store.Rows {
		tags, member := h.members[student.Ref]
		if h.hub && !member {
			continue
		}
		row := studentRow{Student: student, Note: notes[student.Ref]}
		for _, name := range tags {
			class := ""
			switch strings.ToLower(name) {
			case "silver", "red", "blue":
				class = strings.ToLower(name)
			}
			row.Tags = append(row.Tags, studentTag{Name: name, Class: class})
		}
		data.Rows = append(data.Rows, row)
	}
	sort.SliceStable(data.Rows, func(i, j int) bool { return strings.ToLower(data.Rows[i].Name) < strings.ToLower(data.Rows[j].Name) })
	_ = studentsTemplate.Execute(w, data)
}
func (h *studentHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "same-origin")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	csp := "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'"
	if h.hub {
		csp += "; script-src 'self'"
	}
	w.Header().Set("Content-Security-Policy", csp)
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
		h.pageNotice(w, 200, h.data.Message, h.data.Notice)
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
	edited := map[string]string{}
	for i := range h.store.Rows {
		v := &h.store.Rows[i]
		if _, member := h.members[v.Ref]; h.hub && !member {
			continue
		}
		a := strings.TrimSpace(r.PostForm.Get("alias_" + v.Ref))
		if a == "" || len([]rune(a)) > 80 || strings.ContainsAny(a, "\r\n\x00") {
			h.store.Rows = before
			h.page(w, 400, "Aliases need 1–80 characters, without line breaks.")
			return
		}
		v.Alias = a
		v.Monitored = r.PostForm.Get("monitored_"+v.Ref) == "on"
		v.ExtraTime = r.PostForm.Get("extra_time_"+v.Ref) == "on"
		if raw, sent := r.PostForm["note_"+v.Ref]; sent && h.notesRoot != "" {
			note, ok := cleanText(raw[0])
			if !ok {
				h.store.Rows = before
				h.page(w, 400, "Each note holds up to 8,000 characters.")
				return
			}
			edited[v.Ref] = note
		}
	}
	if e := h.store.Save(); e != nil {
		h.store.Rows = before
		h.page(w, 400, "Could not save. Aliases must be unique within each Canvas instance; check file access.")
		return
	}
	if len(edited) > 0 {
		if e := saveNotes(h.notesRoot, edited); e != nil {
			h.page(w, 400, "Students saved, but notes could not be saved: "+e.Error())
			return
		}
	}
	if h.persistent {
		h.data.ClearDraft = true
		h.pageNotice(w, 200, "", "Saved.")
		return
	}
	h.complete = true
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, "<!doctype html><html lang=\"en\"><meta charset=\"utf-8\"><title>Students saved</title><style>"+string(pageCSS)+"</style><main class=\"done\"><h1>Saved</h1><p>Student settings saved. You can close this page.</p></main></html>")
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
	h := &studentHandler{origin: origin, secret: hex.EncodeToString(nonce[:]), store: store, done: make(chan struct{}), notesRoot: root}
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
