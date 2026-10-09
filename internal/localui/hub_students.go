package localui

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workflow"
	"canvasexpertse/internal/workspace"
)

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
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method not allowed", 405)
		return
	}
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
		h.home(w, 400, "Could not read settings/courses.md.", "")
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
		h.redirect(w, r, "/students", "", "Could not read settings/courses.md.")
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
