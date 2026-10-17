package localui

import (
	"fmt"
	"net/http"
	"strings"

	"canvasexpertse/internal/workspace"
)

type homeData struct {
	Appearance                                        appearanceData
	Secret, Message, Notice, CanvasOrigin, Root       string
	Program, Version                                  string
	CanvasHealth, CanvasHealthLabel, CanvasHealthNote string
	CanvasChecking                                    bool
	SyncHealth, SyncHealthNote                        string
	ActiveCourses                                     int
	CourseRows                                        []homeCourse
	Connected                                         bool
	ConnectionTested                                  bool
	Preferences, Aliases                              bool
	Students, Monitored                               int
	CanvasUser                                        string
	RulesMode, RulesRoot, SyncRoot, RulesError        string
	Commands                                          []homeCommand
	CommandsNote, PickedCourse, FirstPrompt           string
}

type homeCourse struct {
	ID       int64
	Nickname string
}

func init() {
	register(
		page{Path: "/", Template: "home", Scripts: []string{"canvas-health.js", "home.js", "appearance.js"}, Connect: true,
			Nav:    []navSlot{{Primary: "Home", Order: 10}},
			Handle: func(h *Hub, w http.ResponseWriter, r *http.Request) { h.home(w, 200, "", "") }},
	)
}

func (h *Hub) home(w http.ResponseWriter, status int, message, notice string) {
	if notice == "" {
		notice, h.seedNotice = h.seedNotice, ""
	}
	data := h.pageData()
	data.Message, data.Notice = message, notice
	data.Commands, data.CommandsNote = homeCommands(h.root)
	if len(data.CourseRows) == 1 {
		data.PickedCourse = data.CourseRows[0].Nickname
	}
	if len(data.Commands) > 0 {
		data.FirstPrompt = fillPrompt(data.Commands[0].Prompt, data.PickedCourse)
	}
	h.render(w, status, tmpl("home"), data)
}

func (h *Hub) pageData() homeData {
	data := homeData{Appearance: appearanceFor(h.root, h.secret), Secret: h.secret, Root: h.root, Program: h.programDir, Version: h.version, SyncHealth: "Local only", SyncHealthNote: "Your teaching rules are kept on this PC."}
	data.ConnectionTested = h.connectionTested
	data.CanvasHealth, data.CanvasHealthNote = h.canvasHealth, h.canvasHealthNote
	if data.CanvasHealth == "" {
		data.CanvasHealth, data.CanvasHealthNote = canvasHealthNotConnected, canvasHealthNote(canvasHealthNotConnected)
	}
	data.CanvasHealthLabel, data.CanvasChecking = canvasHealthLabel(data.CanvasHealth), data.CanvasHealth == canvasHealthChecking
	if c, err := h.loadSavedCredential(); err == nil {
		data.Connected, data.CanvasOrigin, data.CanvasUser = true, c.Origin, h.canvasUser
	}
	if h.root != "" {
		loc, health, healthNote, rulesErr := inspectRulesLocation(h.root)
		data.RulesMode, data.RulesRoot, data.SyncRoot = loc.Mode, loc.RulesRoot, ""
		data.SyncHealth, data.SyncHealthNote = health, healthNote
		if loc.Mode == "sync" {
			data.SyncRoot = loc.RulesRoot
		}
		if rulesErr != nil {
			data.RulesError = rulesErr.Error()
		}
		if config, _, err := h.readConfig(); err == nil {
			data.Preferences, data.Aliases = true, config.Aliases
			for _, c := range config.Courses {
				if c.Current {
					data.ActiveCourses++
					nickname := strings.TrimSpace(c.Nickname)
					if nickname == "" {
						nickname = fmt.Sprintf("Course %d", c.ID)
					}
					data.CourseRows = append(data.CourseRows, homeCourse{ID: c.ID, Nickname: nickname})
				}
			}
		}
		if s, err := workspace.OpenStudents(h.root); err == nil {
			data.Students = len(s.Rows)
			for _, row := range s.Rows {
				if row.Monitored {
					data.Monitored++
				}
			}
		}
	}
	return data
}
