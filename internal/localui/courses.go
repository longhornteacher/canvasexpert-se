package localui

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/platform"
	"canvasexpertse/internal/workspace"
)

const starterInstance = "canvas.example.edu"

type courseRow struct {
	ID                  int64
	Name, Code, Nick    string
	State               string
	Owner               string
	Active, Unpublished bool
}

type coursesData struct {
	Appearance              appearanceData
	Secret, Message, Notice string
	DraftKey                string
	ClearDraft              bool
	Rows                    []courseRow
}

func init() {
	register(
		page{Path: "/courses", Methods: []string{http.MethodGet, http.MethodPost}, Template: "courses", Scripts: []string{"drafts.js", "appearance.js"},
			Nav:    []navSlot{{Primary: "Classroom", Order: 20}, {Primary: "Classroom", Label: "Courses", Order: 21}},
			Handle: (*Hub).coursesPage},
	)
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
			row.Nick, row.Active, row.Owner = known.Nickname, known.Current, known.Owner
		}
		rows = append(rows, row)
	}
	// Keep Canvas' ordering within each state group while putting usable courses first.
	sort.SliceStable(rows, func(i, j int) bool {
		return !rows[i].Unpublished && rows[j].Unpublished
	})
	for _, c := range config.Courses {
		if !seen[c.ID] {
			rows = append(rows, courseRow{ID: c.ID, Name: c.Nickname, Nick: c.Nickname, Active: c.Current, Owner: c.Owner})
		}
	}
	team, _ := workspace.ReadTeam(h.root)
	for i := range rows {
		rows[i].Owner = teamOwnerName(rows[i].Owner, team)
	}
	h.render(w, 200, tmpl("courses"), coursesData{Appearance: appearanceFor(h.root, h.secret), Secret: h.secret, Message: message, Notice: notice, DraftKey: "courses:" + h.secret, ClearDraft: r.URL.Query().Get("saved") == "1", Rows: rows})
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
	if err = workspace.WriteTeacherDocument(h.root, "context/courses.md", config, body); err != nil {
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
