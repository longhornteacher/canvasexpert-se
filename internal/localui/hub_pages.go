package localui

import (
	"context"
	"fmt"
	"html/template"
	"math/rand/v2"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"canvasexpertse/internal/canvas"
	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/platform"
	"canvasexpertse/internal/workflow"
	"canvasexpertse/internal/workspace"
)

const starterInstance = "canvas.example.edu"

type courseRow struct {
	ID               int64
	Name, Code, Nick string
	State            string
	Active, InCanvas bool
}

type coursesData struct {
	Secret, Message, Notice string
	Rows                    []courseRow
}

var coursesTemplate = template.Must(template.New("courses").Funcs(pageFuncs).Parse(`<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>CanvasExpert-SE courses</title><style>` + funkyCSS + `</style><main class="wide"><div class="badge">SE = {{se}}</div>` + hubNav + `<h1>Pick your active courses</h1><p>Tick the courses you are teaching right now. Only active courses are scanned for work and Pok&eacute;mon-ified. The nickname is what you and the agent call the course.</p>
{{if .Message}}<p class="error">{{.Message}}</p>{{end}}{{if .Notice}}<p class="ok">{{.Notice}}</p>{{end}}
<form method="post" action="/courses"><input type="hidden" name="request_secret" value="{{.Secret}}"><table class="courses"><thead><tr><th>Active</th><th>Course</th><th>Canvas code</th><th>Nickname</th></tr></thead><tbody>
{{range .Rows}}<tr><td><input type="hidden" name="id" value="{{.ID}}"><input type="checkbox" name="active_{{.ID}}" {{if .Active}}checked{{end}} aria-label="Active"></td><td>{{.Name}}{{if not .InCanvas}} <span class="pill off">not in Canvas list</span>{{end}}</td><td><code>{{.Code}}</code></td><td><input type="text" name="nick_{{.ID}}" value="{{.Nick}}" maxlength="40" autocomplete="off"></td></tr>
{{else}}<tr><td colspan="4">No courses found. Connect Canvas first, then reload this page.</td></tr>{{end}}</tbody></table>
{{if .Rows}}<button type="submit">Save active courses</button>{{end}}</form></main></html>`))

type group struct {
	Title string
	Cards []card
}

type pokemonData struct {
	Secret, Message, Notice string
	Groups                  []group
	Total, Active           int
}

var pokemonTemplate = template.Must(template.New("pokemon").Funcs(pageFuncs).Parse(`<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>CanvasExpert-SE Pokemon-ify</title><style>` + funkyCSS + `</style><main class="wide"><div class="badge">SE = {{se}}</div>` + hubNav + `<h1>Pok&eacute;mon-ify!</h1><p>Every student gets a Pok&eacute;mon alias, so real names stay out of feedback and files. The Pok&eacute;mon stays the same even if you re-roll a name later.</p>
{{if .Message}}<p class="error">{{.Message}}</p>{{end}}{{if .Notice}}<p class="ok">{{.Notice}}</p>{{end}}
<input type="checkbox" id="reveal" hidden>
<div class="row"><form method="post" action="/pokemonify/summon"><input type="hidden" name="request_secret" value="{{.Secret}}"><button type="submit">Summon rosters from Canvas</button></form><label class="pill" for="reveal">Hide real names</label><span class="pill on">{{.Total}} in the party</span><span class="pill">{{.Active}} active courses</span></div>
{{range .Groups}}<section><h2 class="grp">{{.Title}}</h2><div class="deck">
{{range .Cards}}<article class="pk t-{{.Type}}{{if .Shiny}} shiny{{end}}{{if .Monitored}} mon{{end}}" style="--i:{{.Idx}}">{{if .Fresh}}<div class="ball"><i></i></div>{{end}}
<div class="top"><span class="tag">{{.Type}}</span><span>Lv. {{.Level}}</span></div><div class="art">{{.Art}}</div><h3>{{.Alias}}</h3><p class="real">{{.Name}}</p>{{if .Course}}<span class="course">{{.Course}}</span>{{end}}
<div class="acts"><form method="post" action="/pokemonify/reroll"><input type="hidden" name="request_secret" value="{{$.Secret}}"><input type="hidden" name="ref" value="{{.Ref}}"><button type="submit">Re-roll</button></form><form method="post" action="/pokemonify/monitor"><input type="hidden" name="request_secret" value="{{$.Secret}}"><input type="hidden" name="ref" value="{{.Ref}}"><button type="submit">{{if .Monitored}}Monitored &#10003;{{else}}Monitor{{end}}</button></form></div></article>
{{end}}</div></section>
{{else}}<p class="card">No students yet. Pick active courses on the Courses page, then press <b>Summon rosters from Canvas</b>.</p>{{end}}</main></html>`))

func (h *Hub) reqCtx() (context.Context, context.CancelFunc) {
	base := h.ctx
	if base == nil {
		base = context.Background()
	}
	return context.WithTimeout(base, 60*time.Second)
}

func (h *Hub) canvasClient() (*canvas.Client, error) {
	credential, err := platform.LoadCredential()
	if err != nil {
		return nil, err
	}
	return canvas.New(credential.Origin, credential.Token)
}

func (h *Hub) fetchCourses(ctx context.Context) ([]workflow.Course, string, error) {
	if h.listCoursesFn != nil {
		return h.listCoursesFn(ctx)
	}
	c, err := h.canvasClient()
	if err != nil {
		return nil, "", err
	}
	list, err := workflow.Courses(ctx, c)
	return list, c.Origin(), err
}

func (h *Hub) fetchRoster(ctx context.Context, courseID int64) ([]workflow.Enrollment, string, error) {
	if h.rosterFn != nil {
		return h.rosterFn(ctx, courseID)
	}
	c, err := h.canvasClient()
	if err != nil {
		return nil, "", err
	}
	list, err := workflow.StudentEnrollments(ctx, c, courseID)
	return list, c.Origin(), err
}

func (h *Hub) readConfig() (domain.CourseConfig, string, error) {
	var config domain.CourseConfig
	body, err := workspace.ReadDocument(filepath.Join(h.root, "settings", "courses.md"), &config)
	return config, body, err
}

func (h *Hub) takeFlash() (message, notice string) {
	message, notice = h.flashErr, h.flash
	h.flash, h.flashErr = "", ""
	return
}

func (h *Hub) redirect(w http.ResponseWriter, r *http.Request, to, notice, problem string) {
	h.flash, h.flashErr = notice, problem
	http.Redirect(w, r, to, http.StatusSeeOther)
}

func suggestNickname(code, name string) string {
	nick := strings.TrimSpace(code)
	if nick == "" {
		nick = strings.TrimSpace(name)
	}
	nick = strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`/\<>:"|?*`, r) {
			return '-'
		}
		return r
	}, nick)
	if len([]rune(nick)) > 40 {
		nick = string([]rune(nick)[:40])
	}
	return strings.Trim(strings.TrimSpace(nick), ". ")
}

func (h *Hub) needWorkspace(w http.ResponseWriter) bool {
	if h.root == "" {
		h.home(w, 400, "Choose a workspace folder first; courses and students live inside it.", "")
		return true
	}
	return false
}

func (h *Hub) coursesPage(w http.ResponseWriter, r *http.Request) {
	if h.needWorkspace(w) {
		return
	}
	if r.Method == http.MethodPost {
		h.saveCourses(w, r)
		return
	}
	message, notice := h.takeFlash()
	config, _, err := h.readConfig()
	if err != nil {
		h.home(w, 400, "Could not read settings/courses.md: "+err.Error(), "")
		return
	}
	ctx, cancel := h.reqCtx()
	defer cancel()
	list, _, fetchErr := h.fetchCourses(ctx)
	if fetchErr != nil && message == "" {
		message = "Could not list your Canvas courses (is Canvas connected?): " + fetchErr.Error()
	}
	configured := map[int64]domain.Course{}
	for _, c := range config.Courses {
		configured[c.ID] = c
	}
	var rows []courseRow
	seen := map[int64]bool{}
	for _, c := range list {
		seen[c.ID] = true
		row := courseRow{ID: c.ID, Name: c.Name, Code: c.Code, State: c.State, InCanvas: true, Nick: suggestNickname(c.Code, c.Name)}
		if known, ok := configured[c.ID]; ok {
			row.Nick, row.Active = known.Nickname, known.Current
		}
		rows = append(rows, row)
	}
	for _, c := range config.Courses {
		if !seen[c.ID] {
			rows = append(rows, courseRow{ID: c.ID, Name: c.Nickname, Nick: c.Nickname, Active: c.Current})
		}
	}
	h.render(w, 200, coursesTemplate, coursesData{Secret: h.secret, Message: message, Notice: notice, Rows: rows})
}

func (h *Hub) render(w http.ResponseWriter, status int, t *template.Template, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_ = t.Execute(w, data)
}

func (h *Hub) saveCourses(w http.ResponseWriter, r *http.Request) {
	if !h.checkPost(w, r) {
		return
	}
	config, body, err := h.readConfig()
	if err != nil {
		h.redirect(w, r, "/courses", "", "Could not read settings/courses.md: "+err.Error())
		return
	}
	origin := ""
	if c, credErr := platform.LoadCredential(); credErr == nil {
		origin = c.Origin
	}
	if origin != "" && (config.Instance == "" || strings.Contains(config.Instance, starterInstance) || len(config.Courses) == 0) {
		config.Instance = origin
	} else if origin != "" && config.Instance != origin {
		h.redirect(w, r, "/courses", "", "settings/courses.md uses a different Canvas address than the one saved on this machine. Fix one of them first.")
		return
	}
	index := map[int64]int{}
	for i, c := range config.Courses {
		index[c.ID] = i
	}
	var added []domain.Course
	for _, raw := range r.PostForm["id"] {
		id, convErr := strconv.ParseInt(raw, 10, 64)
		if convErr != nil || id <= 0 {
			continue
		}
		nick := strings.TrimSpace(r.PostForm.Get("nick_" + raw))
		active := r.PostForm.Get("active_"+raw) == "on"
		if i, ok := index[id]; ok {
			if nick != "" {
				config.Courses[i].Nickname = nick
			}
			config.Courses[i].Current = active
			continue
		}
		if !active {
			continue
		}
		if nick == "" {
			h.redirect(w, r, "/courses", "", "Every active course needs a nickname.")
			return
		}
		added = append(added, domain.Course{Nickname: nick, ID: id, Current: true})
	}
	config.Courses = append(config.Courses, added...)
	if err = domain.ValidateConfig(config); err != nil {
		h.redirect(w, r, "/courses", "", "Not saved: nicknames must be unique and file-safe (no / \\ : * ? \" < > |). "+err.Error())
		return
	}
	if err = workspace.WriteDocument(filepath.Join(h.root, "settings", "courses.md"), config, body); err != nil {
		h.redirect(w, r, "/courses", "", "Could not save settings/courses.md.")
		return
	}
	active := 0
	for _, c := range config.Courses {
		if c.Current {
			active++
		}
	}
	h.rosters, h.fresh = nil, nil
	h.redirect(w, r, "/courses", fmt.Sprintf("Saved. %d active courses.", active), "")
}

func randomAlias(available []string) string { return available[rand.IntN(len(available))] }

func (h *Hub) pokemonPage(w http.ResponseWriter, r *http.Request) {
	if h.needWorkspace(w) {
		return
	}
	message, notice := h.takeFlash()
	config, _, err := h.readConfig()
	if err != nil {
		h.home(w, 400, "Could not read settings/courses.md: "+err.Error(), "")
		return
	}
	store, err := workspace.OpenStudents(h.root)
	if err != nil {
		h.home(w, 400, "Could not read the student table: "+err.Error(), "")
		return
	}
	data := pokemonData{Secret: h.secret, Message: message, Notice: notice, Total: len(store.Rows)}
	byRef := map[string]workspace.Student{}
	for _, s := range store.Rows {
		byRef[s.Ref] = s
	}
	idx := 0
	build := func(s workspace.Student, course string) card {
		idx++
		return newCard(s.Ref, s.Alias, s.Name, course, s.Monitored, h.fresh[s.Ref], idx-1)
	}
	placed := map[string]bool{}
	for _, c := range config.Courses {
		if !c.Current {
			continue
		}
		data.Active++
		members := h.rosters[c.Nickname]
		if len(members) == 0 {
			continue
		}
		var cards []card
		for _, s := range store.Rows {
			if members[s.Ref] {
				cards = append(cards, build(s, c.Nickname))
				placed[s.Ref] = true
			}
		}
		sort.SliceStable(cards, func(i, j int) bool { return cards[i].Alias < cards[j].Alias })
		if len(cards) > 0 {
			data.Groups = append(data.Groups, group{Title: c.Nickname, Cards: cards})
		}
	}
	var rest []card
	for _, s := range store.Rows {
		if !placed[s.Ref] {
			rest = append(rest, build(s, ""))
		}
	}
	if len(rest) > 0 {
		title := "All known students"
		if len(data.Groups) > 0 {
			title = "Other known students"
		}
		sort.SliceStable(rest, func(i, j int) bool { return rest[i].Alias < rest[j].Alias })
		data.Groups = append(data.Groups, group{Title: title, Cards: rest})
	}
	h.render(w, 200, pokemonTemplate, data)
}

func (h *Hub) pokemonAction(w http.ResponseWriter, r *http.Request, action string) {
	if h.needWorkspace(w) {
		return
	}
	if !h.checkPost(w, r) {
		return
	}
	store, err := workspace.OpenStudents(h.root)
	if err != nil {
		h.redirect(w, r, "/pokemonify", "", "Could not read the student table.")
		return
	}
	store.Choose = randomAlias
	switch action {
	case "reroll":
		student, err := store.Reroll(r.PostForm.Get("ref"))
		if err == nil {
			err = store.Save()
		}
		if err != nil {
			h.redirect(w, r, "/pokemonify", "", err.Error())
			return
		}
		h.fresh = map[string]bool{student.Ref: true}
		h.redirect(w, r, "/pokemonify", "A wild "+student.Alias+" appeared!", "")
	case "monitor":
		ref := r.PostForm.Get("ref")
		found := false
		for i := range store.Rows {
			if store.Rows[i].Ref == ref {
				store.Rows[i].Monitored = !store.Rows[i].Monitored
				found = true
			}
		}
		if !found {
			h.redirect(w, r, "/pokemonify", "", "That student was not found.")
			return
		}
		if err = store.Save(); err != nil {
			h.redirect(w, r, "/pokemonify", "", "Could not save.")
			return
		}
		h.redirect(w, r, "/pokemonify", "", "")
	case "summon":
		h.summon(w, r, store)
	}
}

func (h *Hub) summon(w http.ResponseWriter, r *http.Request, store *workspace.StudentStore) {
	config, _, err := h.readConfig()
	if err != nil {
		h.redirect(w, r, "/pokemonify", "", "Could not read settings/courses.md.")
		return
	}
	ctx, cancel := h.reqCtx()
	defer cancel()
	known := map[string]bool{}
	for _, s := range store.Rows {
		known[s.Ref] = true
	}
	rosters := map[string]map[string]bool{}
	fresh := map[string]bool{}
	courses := 0
	for _, course := range config.Courses {
		if !course.Current {
			continue
		}
		courses++
		enrollments, origin, err := h.fetchRoster(ctx, course.ID)
		if err != nil {
			h.redirect(w, r, "/pokemonify", "", "Could not fetch the roster for "+course.Nickname+": "+err.Error())
			return
		}
		members := map[string]bool{}
		for _, e := range enrollments {
			id := e.UserID
			if id == 0 {
				id = e.User.ID
			}
			if e.State != "active" || id <= 0 {
				continue
			}
			student, err := store.Upsert(origin, id, e.User.Name, e.User.ShortName, e.User.SISID, e.User.Login)
			if err != nil {
				h.redirect(w, r, "/pokemonify", "", err.Error())
				return
			}
			members[student.Ref] = true
			if !known[student.Ref] {
				fresh[student.Ref] = true
			}
		}
		rosters[course.Nickname] = members
	}
	if courses == 0 {
		h.redirect(w, r, "/pokemonify", "", "No active courses yet. Pick them on the Courses page first.")
		return
	}
	if err = store.Save(); err != nil {
		h.redirect(w, r, "/pokemonify", "", "Could not save the student table: "+err.Error())
		return
	}
	h.rosters, h.fresh = rosters, fresh
	total := 0
	for _, m := range rosters {
		total += len(m)
	}
	h.redirect(w, r, "/pokemonify", fmt.Sprintf("Summoned %d students from %d courses. %d new Pokémon joined the party!", total, courses, len(fresh)), "")
}
