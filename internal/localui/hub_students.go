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

// draftNoticeScript defines showDraftNotice for the draft scripts: a restored
// draft overrides saved values, so the teacher sees it and can discard the
// edits on this page. Drafts for other courses or sections are kept.
const draftNoticeScript = `const showDraftNotice=(before,key,values,names)=>{const p=document.createElement("p");p.className="draft-restored";p.setAttribute("role","status");p.textContent="Unsaved edits from this tab are shown. Saving replaces the saved values. ";const b=document.createElement("button");b.type="button";b.className="small secondary";b.textContent="Discard unsaved edits";b.addEventListener("click",()=>{for(const n of names)delete values[n];try{if(Object.keys(values).length)sessionStorage.setItem(key,JSON.stringify(values));else sessionStorage.removeItem(key)}catch{};location.reload()});p.append(b);before.parentNode.insertBefore(p,before)};
`

// classroomDraftScript keeps unsaved edits in the current browser tab only.
// A per-hub key prevents another local hub session from restoring private notes.
const classroomDraftScript = `(()=>{
` + draftNoticeScript + `const form=document.querySelector("form[data-draft-key]");
if(!form)return;
const key=form.dataset.draftKey;
const fields=()=>Array.from(form.elements).filter(e=>e.name&&/^(active_|nick_|alias_|monitored_|extra_time_|note_)/.test(e.name));
let values={};
try{values=JSON.parse(sessionStorage.getItem(key)||"{}")}catch{}
if(form.dataset.clearDraft==="true"){for(const e of fields())delete values[e.name];try{if(Object.keys(values).length)sessionStorage.setItem(key,JSON.stringify(values));else sessionStorage.removeItem(key)}catch{};const u=new URL(location.href);u.searchParams.delete("saved");history.replaceState(null,"",u.pathname+u.search+u.hash)}
let restored=false;for(const e of fields())if(Object.prototype.hasOwnProperty.call(values,e.name)){if(e.type==="checkbox"){restored||=e.checked!==(values[e.name]===true);e.checked=values[e.name]===true}else{restored||=e.value!==values[e.name];e.value=values[e.name]}}
if(restored)showDraftNotice(form,key,values,fields().map(e=>e.name));
const save=event=>{const e=event.target;if(!e.name||! /^(active_|nick_|alias_|monitored_|extra_time_|note_)/.test(e.name))return;values[e.name]=e.type==="checkbox"?e.checked:e.value;try{sessionStorage.setItem(key,JSON.stringify(values))}catch{}};
form.addEventListener("input",save);form.addEventListener("change",save);
})();`

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
