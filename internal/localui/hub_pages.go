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
	ID                  int64
	Name, Code, Nick    string
	State               string
	Active, Unpublished bool
}

type coursesData struct {
	Secret, Message, Notice string
	DraftKey                string
	ClearDraft              bool
	Rows                    []courseRow
}

var coursesTemplate = template.Must(template.New("courses").Parse(`<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>CanvasExpert-SE courses</title><style>` + funkyCSS + `</style><main class="wide"><div class="badge">CanvasExpert: SE</div>` + hubNavCourses + `<h1>Select active courses</h1><p>Choose current courses and set a short nickname for each.</p>
{{if .Message}}<p class="error">{{.Message}}</p>{{end}}{{if .Notice}}<p class="ok">{{.Notice}}</p>{{end}}
<form method="post" action="/courses" data-draft-key="{{.DraftKey}}" data-clear-draft="{{.ClearDraft}}"><input type="hidden" name="request_secret" value="{{.Secret}}"><div class="table-scroll"><table class="courses"><thead><tr><th>Active</th><th>Course</th><th>Canvas code</th><th>Nickname</th></tr></thead><tbody>
{{range .Rows}}<tr{{if .Unpublished}} class="unpublished"{{end}}><td><input type="hidden" name="id" value="{{.ID}}"><input type="checkbox" name="active_{{.ID}}" {{if .Active}}checked{{end}} aria-label="Active course {{.Name}} ({{.ID}})"></td><td>{{.Name}} <span class="course-state">ID {{.ID}} · {{if .State}}{{.State}}{{else}}not in Canvas list{{end}}</span>{{if .Unpublished}} <span class="pill off">unpublished</span>{{end}}</td><td><code>{{.Code}}</code></td><td><input type="text" name="nick_{{.ID}}" value="{{.Nick}}" maxlength="40" autocomplete="off" aria-label="Nickname for {{.Name}} ({{.ID}})"></td></tr>
{{else}}<tr><td colspan="4">No courses found. Connect Canvas first, then reload this page.</td></tr>{{end}}</tbody></table></div>
{{if .Rows}}<button type="submit">Save active courses</button>{{end}}</form><script src="/_cse/drafts.js" defer></script></main></html>`))

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
	body, err := workspace.ReadDocument(filepath.Join(h.root, "context", "courses.md"), &config)
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

func (h *Hub) needWorkspace(w http.ResponseWriter) bool {
	if h.root == "" {
		h.home(w, 400, "Launch with the CE-SE installation first; for development use cse --workspace PATH ui.", "")
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
		h.home(w, 400, "Could not read context/courses.md: "+err.Error(), "")
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
		row := courseRow{ID: c.ID, Name: c.Name, Code: c.Code, State: c.State, Unpublished: c.State == "unpublished", Nick: ""}
		if known, ok := configured[c.ID]; ok {
			row.Nick, row.Active = known.Nickname, known.Current
		}
		rows = append(rows, row)
	}
	// Keep Canvas' ordering within each state group while putting usable courses first.
	sort.SliceStable(rows, func(i, j int) bool {
		return !rows[i].Unpublished && rows[j].Unpublished
	})
	for _, c := range config.Courses {
		if !seen[c.ID] {
			rows = append(rows, courseRow{ID: c.ID, Name: c.Nickname, Nick: c.Nickname, Active: c.Current})
		}
	}
	h.render(w, 200, coursesTemplate, coursesData{Secret: h.secret, Message: message, Notice: notice, DraftKey: "courses:" + h.secret, ClearDraft: r.URL.Query().Get("saved") == "1", Rows: rows})
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
		h.redirect(w, r, "/courses", "", "Could not read context/courses.md: "+err.Error())
		return
	}
	origin := ""
	if h.savedOriginFn != nil {
		origin = h.savedOriginFn()
	} else if c, credErr := platform.LoadCredential(); credErr == nil {
		origin = c.Origin
	}
	if origin != "" && (config.Instance == "" || strings.Contains(config.Instance, starterInstance) || len(config.Courses) == 0) {
		config.Instance = origin
	} else if origin != "" && config.Instance != origin {
		h.redirect(w, r, "/courses", "", "context/courses.md uses a different Canvas address than the one saved on this machine. Fix one of them first.")
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
	if err = workspace.WriteDocument(filepath.Join(h.root, "context", "courses.md"), config, body); err != nil {
		h.redirect(w, r, "/courses", "", "Could not save context/courses.md.")
		return
	}
	active := 0
	for _, c := range config.Courses {
		if c.Current {
			active++
		}
	}
	h.loaded = nil
	h.redirect(w, r, "/courses?saved=1", fmt.Sprintf("Saved. %d active courses.", active), "")
}

func randomAlias(available []string) string { return available[rand.IntN(len(available))] }
