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

	"strconv"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/hubcoord"
	"canvasexpertse/internal/platform"
	"canvasexpertse/internal/workflow"
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
	Tags     []studentTag
	Note     string
	Contacts workspace.StudentContacts
}

type studentsData struct {
	Appearance                        appearanceData
	Secret, Message, Notice           string
	DraftKey                          string
	ClearDraft                        bool
	Rows                              []studentRow
	Hub, NotesOn                      bool
	ContactsOn                        bool
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
	notesRoot    string
	contactDraft map[string]workspace.StudentContacts
}

func (h *studentHandler) page(w http.ResponseWriter, status int, message string) {
	h.pageNotice(w, status, message, "")
}

func (h *studentHandler) pageNotice(w http.ResponseWriter, status int, message, notice string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	data := h.data
	data.Appearance = appearanceFor(h.notesRoot, h.secret)
	data.Secret, data.Message, data.Notice, data.Hub = h.secret, message, notice, h.hub
	if h.hub {
		data.DraftKey = "students:" + h.secret
	}
	data.Rows = nil
	var contacts *workspace.ContactStore
	if h.notesRoot != "" {
		var err error
		contacts, err = workspace.OpenContacts(h.notesRoot)
		if err != nil {
			if data.Message == "" {
				data.Message = "Contact details unavailable; existing data has not been replaced. " + err.Error()
			}
		} else {
			data.ContactsOn = true
		}
	}
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
		if contacts != nil {
			row.Contacts = contacts.Students[workspace.ContactKey(student)]
		}
		if draft, ok := h.contactDraft[student.Ref]; ok {
			row.Contacts = draft
		}
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
	_ = tmpl("students").Execute(w, data)
}
func (h *studentHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "same-origin")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	csp := "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'"
	csp += "; script-src 'self'; connect-src 'self'"
	w.Header().Set("Content-Security-Policy", csp)
	if "http://"+r.Host != h.origin {
		http.NotFound(w, r)
		return
	}
	if serveAppearance(w, r, h.notesRoot, h.origin, h.secret) {
		return
	}
	if serveScript(w, r) {
		return
	}
	if r.URL.Path != "/students" {
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
	h.data.ClearDraft = false
	var contacts *workspace.ContactStore
	if h.notesRoot != "" {
		var err error
		contacts, err = workspace.OpenContacts(h.notesRoot)
		if err != nil {
			h.page(w, 400, "Contact details could not be opened; nothing was saved.")
			return
		}
		// Keep raw entries on validation failure, including dynamically added rows.
		h.contactDraft = map[string]workspace.StudentContacts{}
		for _, student := range h.store.Rows {
			if _, member := h.members[student.Ref]; h.hub && !member {
				continue
			}
			if _, sent := r.PostForm["student_email_"+student.Ref]; !sent {
				continue
			}
			entry := workspace.StudentContacts{StudentEmail: r.PostForm.Get("student_email_" + student.Ref)}
			for i, phone := range r.PostForm["contact_phone_"+student.Ref] {
				value := func(field string) string {
					values := r.PostForm[field+student.Ref]
					if i < len(values) {
						return values[i]
					}
					return ""
				}
				entry.Parents = append(entry.Parents, workspace.ParentContact{Phone: phone, Email: value("contact_email_"), Text: r.PostForm.Get(fmt.Sprintf("contact_text_%s_%d", student.Ref, i)) == "yes", EmailEnabled: r.PostForm.Get(fmt.Sprintf("contact_send_email_%s_%d", student.Ref, i)) == "yes"})
			}
			h.contactDraft[student.Ref] = entry
		}
		for ref, entry := range h.contactDraft {
			var err error
			entry.StudentEmail, err = workspace.ContactEmail(entry.StudentEmail)
			if err != nil {
				h.page(w, 400, "Student email: "+err.Error())
				return
			}
			parents := []workspace.ParentContact{}
			for _, parent := range entry.Parents {
				parent.Phone, err = workspace.ContactPhone(parent.Phone)
				if err != nil {
					h.page(w, 400, "Parent phone: "+err.Error())
					return
				}
				parent.Email, err = workspace.ContactEmail(parent.Email)
				if err != nil {
					h.page(w, 400, "Parent email: "+err.Error())
					return
				}
				if parent.Phone != "" || parent.Email != "" || parent.Text || parent.EmailEnabled {
					parents = append(parents, parent)
				}
			}
			entry.Parents = parents
			for _, student := range h.store.Rows {
				if student.Ref == ref {
					contacts.Students[workspace.ContactKey(student)] = entry
					break
				}
			}
		}
	}
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
		if raw, sent := r.PostForm["goes_by_"+v.Ref]; sent {
			name := strings.TrimSpace(raw[0])
			if name == "" || len([]rune(name)) > 80 || strings.ContainsAny(raw[0], "\r\n\x00") {
				h.store.Rows = before
				h.page(w, 400, "Goes By names need 1-80 characters, without line breaks.")
				return
			}
			v.FirstName = name
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
	if contacts != nil {
		if err := contacts.Save(); err != nil {
			h.page(w, 400, "Student settings saved, but contact details could not be saved. Your entries are still shown.")
			return
		}
	}
	h.contactDraft = nil
	if h.persistent {
		h.data.ClearDraft = true
		h.pageNotice(w, 200, "", "Saved.")
		return
	}
	h.complete = true
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, "<!doctype html>"+appearanceHTML(h.notesRoot)+"<meta charset=\"utf-8\"><title>Students saved</title><style>"+string(pageCSS)+"</style><main class=\"done\"><h1>Saved</h1><p>Student settings saved. You can close this page.</p></main></html>")
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
	defer hubcoord.MarkPage(root, "students", origin, "")()
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

func init() {
	register(
		page{Path: "/students", Methods: []string{http.MethodGet, http.MethodPost}, Template: "students", Scripts: []string{"drafts.js", "contacts.js", "appearance.js"},
			Nav:    []navSlot{{Primary: "Classroom", Label: "Students", Order: 22}},
			Handle: (*Hub).studentsPage},
		page{Path: "/students/refresh", Methods: []string{http.MethodPost}, Handle: (*Hub).refreshStudents},
	)
}

func (h *Hub) fetchTags(ctx context.Context, courseID int64) (map[int64][]string, string, error) {
	if h.tagsFn != nil {
		return h.tagsFn(ctx, courseID)
	}
	c, err := h.canvasClient()
	if err != nil {
		return nil, "", err
	}
	tags, err := workflow.DifferentiationTags(ctx, c, courseID)
	return tags, c.Origin(), err
}

func selectedCourse(config domain.CourseConfig, raw string) (domain.Course, bool) {
	for _, course := range config.Courses {
		if course.Current && (raw == "" || raw == strconv.FormatInt(course.ID, 10)) {
			return course, true
		}
	}
	return domain.Course{}, false
}

func rosterFor(rosters []workspace.CourseRoster, instance string, id int64) workspace.CourseRoster {
	for _, roster := range rosters {
		if roster.Instance == instance && roster.CourseID == id {
			return roster
		}
	}
	return workspace.CourseRoster{Instance: instance, CourseID: id}
}

func (h *Hub) studentsPage(w http.ResponseWriter, r *http.Request) {
	if h.needWorkspace(w) {
		return
	}
	// Parse and authorize saves before reading course selection from the form.
	if r.Method == http.MethodPost {
		if !h.checkPost(w, r) {
			return
		}
	}
	config, _, err := h.readConfig()
	if err != nil {
		h.home(w, 400, "Could not read context/courses.md.", "")
		return
	}
	raw := r.URL.Query().Get("course")
	if r.Method == http.MethodPost {
		raw = r.PostForm.Get("course")
		if raw == "" {
			http.Error(w, "Choose an active course before saving", 400)
			return
		}
	}
	course, selected := selectedCourse(config, raw)
	if !selected && raw != "" {
		http.Error(w, "Active course not found", 404)
		return
	}
	message, notice := h.takeFlash()
	rosters, err := workspace.ReadRosters(h.root)
	if err != nil {
		h.home(w, 400, "Could not read the saved course rosters.", "")
		return
	}
	// Refresh once per course each hub session, then serve the saved snapshot.
	// POST always edits the displayed roster without an intervening Canvas read.
	if selected && r.Method == http.MethodGet && !h.loaded[course.ID] {
		rosters, err = h.syncRoster(config, course, rosters)
		if h.loaded == nil {
			h.loaded = map[int64]bool{}
		}
		h.loaded[course.ID] = true
		if err != nil {
			message = err.Error() + " Showing the last saved roster, if available."
		}
	}
	store, err := workspace.OpenStudents(h.root)
	if err != nil {
		h.home(w, 400, "Could not read the student table.", "")
		return
	}
	roster := rosterFor(rosters, config.Instance, course.ID)
	data := studentsData{CourseID: course.ID, CourseName: course.Nickname, UpdatedAt: roster.UpdatedAt, TagMessage: roster.TagMessage, Message: message, Notice: notice}
	for _, active := range config.Courses {
		if active.Current {
			data.Tabs = append(data.Tabs, courseTab{ID: active.ID, Name: active.Nickname, Selected: active.ID == course.ID})
		}
	}
	(&studentHandler{origin: h.origin, secret: h.secret, store: store, persistent: true, hub: true, data: data, members: roster.Members, notesRoot: h.root}).ServeHTTP(w, r)
}

func (h *Hub) refreshStudents(w http.ResponseWriter, r *http.Request) {
	if !h.checkPost(w, r) || h.needWorkspace(w) {
		return
	}
	config, _, err := h.readConfig()
	if err != nil {
		h.redirect(w, r, "/students", "", "Could not read context/courses.md.")
		return
	}
	course, ok := selectedCourse(config, r.PostForm.Get("course"))
	if !ok {
		h.redirect(w, r, "/students", "", "No active courses yet. Pick them on the Courses page first.")
		return
	}
	to := fmt.Sprintf("/students?course=%d", course.ID)
	rosters, err := workspace.ReadRosters(h.root)
	if err == nil {
		_, err = h.syncRoster(config, course, rosters)
	}
	if h.loaded == nil {
		h.loaded = map[int64]bool{}
	}
	h.loaded[course.ID] = true
	if err != nil {
		h.redirect(w, r, to, "", err.Error())
		return
	}
	h.redirect(w, r, to, "Roster refreshed from Canvas.", "")
}

func (h *Hub) syncRoster(config domain.CourseConfig, course domain.Course, rosters []workspace.CourseRoster) ([]workspace.CourseRoster, error) {
	ctx, cancel := h.reqCtx()
	defer cancel()
	enrollments, origin, err := h.fetchRoster(ctx, course.ID)
	if err != nil {
		return rosters, fmt.Errorf("Could not fetch the roster for %s: %w", course.Nickname, err)
	}
	if origin != config.Instance {
		return rosters, fmt.Errorf("The saved Canvas connection uses a different address than this workspace")
	}
	store, err := workspace.OpenStudents(h.root)
	if err != nil {
		return rosters, err
	}
	store.Choose = randomAlias
	tags, tagOrigin, tagErr := h.fetchTags(ctx, course.ID)
	if tagErr == nil && tagOrigin != origin {
		tagErr = fmt.Errorf("the Canvas address differs from the roster")
	}
	roster := workspace.CourseRoster{Instance: origin, CourseID: course.ID, Members: map[string][]string{}, UpdatedAt: time.Now().UTC().Format(time.RFC3339)}
	if tagErr != nil {
		roster.TagMessage = "Differentiation tags could not be pulled from Canvas: " + tagErr.Error()
	}
	for _, enrollment := range enrollments {
		id := enrollment.UserID
		if id == 0 {
			id = enrollment.User.ID
		}
		if enrollment.State != "active" || id <= 0 || (enrollment.Type != "" && enrollment.Type != "StudentEnrollment") {
			continue
		}
		student, err := store.Upsert(origin, id, enrollment.User.Name, enrollment.User.ShortName, enrollment.User.SISID, enrollment.User.Login)
		if err != nil {
			return rosters, err
		}
		roster.Members[student.Ref] = tags[id]
	}
	if err = store.Save(); err != nil {
		return rosters, fmt.Errorf("Could not save the student table: %w", err)
	}
	updated := append([]workspace.CourseRoster(nil), rosters...)
	replaced := false
	for i, old := range updated {
		if old.Instance == origin && old.CourseID == course.ID {
			updated[i], replaced = roster, true
			break
		}
	}
	if !replaced {
		updated = append(updated, roster)
	}
	if err = workspace.SaveRosters(h.root, updated); err != nil {
		return rosters, fmt.Errorf("Could not save the course roster: %w", err)
	}
	return updated, nil
}
