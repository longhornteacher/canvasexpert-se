package localui

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"canvasexpertse/internal/workspace"
)

// The name key is a printable sheet of real names beside made-up names, for
// the teacher matching agent output to Canvas. It reads the saved rosters only.
type rosterSection struct {
	Course string
	Rows   []workspace.Student
}

type rosterKeyData struct {
	Title, Printed, ByName, ByAliasHref, Back string
	ByAlias                                   bool
	Sections                                  []rosterSection
}

func init() {
	register(page{Path: "/students/roster", Template: "students_roster", Scripts: []string{"print.js"}, Handle: (*Hub).studentsRoster})
}

func (h *Hub) studentsRoster(w http.ResponseWriter, r *http.Request) {
	if h.needWorkspace(w) {
		return
	}
	config, _, err := h.readConfig()
	if err != nil {
		h.home(w, 400, "Could not read context/courses.md.", "")
		return
	}
	raw := r.URL.Query().Get("course")
	var courses []rosterSection
	var ids []int64
	for _, c := range config.Courses {
		if c.Current && (raw == "all" || raw == strconv.FormatInt(c.ID, 10)) {
			courses = append(courses, rosterSection{Course: c.Nickname})
			ids = append(ids, c.ID)
		}
	}
	if len(courses) == 0 && raw != "all" {
		http.Error(w, "Active course not found", http.StatusNotFound)
		return
	}
	rosters, err := workspace.ReadRosters(h.root)
	if err != nil {
		h.home(w, 400, "Could not read the saved course rosters.", "")
		return
	}
	store, err := workspace.OpenStudents(h.root)
	if err != nil {
		h.home(w, 400, "Could not read the student table.", "")
		return
	}
	byAlias := r.URL.Query().Get("sort") == "alias"
	key := func(v workspace.Student) string {
		if byAlias {
			return strings.ToLower(v.Alias)
		}
		return strings.ToLower(v.Name)
	}
	for i, id := range ids {
		members := rosterFor(rosters, config.Instance, id).Members
		for _, v := range store.Rows {
			if _, ok := members[v.Ref]; ok && v.Instance == config.Instance {
				courses[i].Rows = append(courses[i].Rows, v)
			}
		}
		rows := courses[i].Rows
		sort.SliceStable(rows, func(a, b int) bool { return key(rows[a]) < key(rows[b]) })
	}
	data := rosterKeyData{Printed: time.Now().Format("January 2, 2006"), ByAlias: byAlias, Sections: courses, Back: "/students"}
	if len(courses) == 1 {
		data.Title = courses[0].Course
		data.Back = fmt.Sprintf("/students?course=%d", ids[0])
	} else {
		data.Title = "All courses"
	}
	q := url.Values{"course": {raw}}
	data.ByName = "/students/roster?" + q.Encode()
	q.Set("sort", "alias")
	data.ByAliasHref = "/students/roster?" + q.Encode()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = tmpl("students_roster").Execute(w, data)
}
