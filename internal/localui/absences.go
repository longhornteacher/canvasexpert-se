package localui

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"canvasexpertse/internal/workspace"
)

// The Absences page is where the teacher records who was out. The Engine
// treats those dates as non-school days for that student when it counts late
// time. Nothing here reads the SIS or writes to Canvas.
type absenceRow struct {
	Ref, Name string
	Absent    bool
}

type absenceEntry struct {
	Ref, Name, Date, Label string
}

type absencesData struct {
	Appearance                       appearanceData
	Secret, Message, Notice, Version string
	Tabs                             []courseTab
	CourseID                         int64
	CourseName, Date, DateLabel      string
	Calendar, CalendarProblem        string
	Rows                             []absenceRow
	Entries                          []absenceEntry
}

const absenceRangeLimit = 60

func init() {
	register(
		page{Path: "/absences", Methods: []string{http.MethodGet, http.MethodPost}, Template: "absences", Scripts: []string{"appearance.js"},
			Nav:    []navSlot{{Primary: "Classroom", Label: "Absences", Order: 24}},
			Handle: (*Hub).absencesPage},
		page{Path: "/absences/remove", Methods: []string{http.MethodPost}, Handle: (*Hub).absenceRemove},
	)
}

func dayLabel(date string) string {
	d, err := time.Parse("2006-01-02", date)
	if err != nil {
		return date
	}
	return d.Format("Mon 1/2/2006")
}

// calendarStatus describes the school calendar the late-time count will use.
func calendarStatus(root string) (cal workspace.SchoolCalendar, status, problem string) {
	cal, err := workspace.ReadSchoolCalendar(root)
	if err == nil {
		err = cal.Validate()
	}
	if err != nil {
		return workspace.SchoolCalendar{}, "", "The school calendar (context/school-calendar.md) can't be used yet, so late time can't be corrected: " + err.Error()
	}
	return cal, fmt.Sprintf("School calendar: %s through %s, %d weekday closures. Weekends never count.", dayLabel(cal.StartDate), dayLabel(cal.EndDate), cal.WeekdayClosures()), ""
}

func (h *Hub) absencesPage(w http.ResponseWriter, r *http.Request) {
	if h.needWorkspace(w) {
		return
	}
	if r.Method == http.MethodPost && !h.checkPost(w, r) {
		return
	}
	config, _, err := h.readConfig()
	if err != nil {
		h.home(w, 400, "Could not read context/courses.md.", "")
		return
	}
	raw, date := r.URL.Query().Get("course"), r.URL.Query().Get("date")
	if r.Method == http.MethodPost {
		raw, date = r.PostForm.Get("course"), r.PostForm.Get("date")
	}
	course, selected := selectedCourse(config, raw)
	if !selected && raw != "" {
		http.Error(w, "Active course not found", http.StatusNotFound)
		return
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		date = time.Now().Format("2006-01-02")
	}
	message, notice := h.takeFlash()
	cal, calStatus, calProblem := calendarStatus(h.root)
	store, err := workspace.OpenAbsences(h.root)
	if err != nil {
		h.home(w, 400, "Could not read the saved absences.", "")
		return
	}
	students, err := workspace.OpenStudents(h.root)
	if err != nil {
		h.home(w, 400, "Could not read the student table.", "")
		return
	}
	rosters, err := workspace.ReadRosters(h.root)
	if err != nil {
		h.home(w, 400, "Could not read the saved course rosters.", "")
		return
	}
	members := rosterFor(rosters, config.Instance, course.ID).Members
	var roster []workspace.Student
	for _, v := range students.Rows {
		if _, ok := members[v.Ref]; ok && v.Instance == config.Instance {
			roster = append(roster, v)
		}
	}
	sort.SliceStable(roster, func(i, j int) bool { return strings.ToLower(roster[i].Name) < strings.ToLower(roster[j].Name) })
	to := fmt.Sprintf("/absences?course=%d&date=%s", course.ID, date)

	if r.Method == http.MethodPost {
		if !selected {
			http.Error(w, "Choose an active course before saving", http.StatusBadRequest)
			return
		}
		if r.PostForm.Get("absences_version") != store.Version {
			h.redirect(w, r, to, "", "The absence list changed after this page opened, so nothing was saved. Check the list below and save again.")
			return
		}
		through := strings.TrimSpace(r.PostForm.Get("through"))
		days := []string{date}
		if through != "" && through != date {
			end, err := time.Parse("2006-01-02", through)
			start, _ := time.Parse("2006-01-02", date)
			if err != nil || end.Before(start) || end.Sub(start) > absenceRangeLimit*24*time.Hour {
				h.redirect(w, r, to, "", fmt.Sprintf("The last day must be a date after the first, at most %d days later.", absenceRangeLimit))
				return
			}
			if calProblem != "" {
				h.redirect(w, r, to, "", "A date range needs the school calendar to skip days off. Save one day at a time, or fix the calendar first.")
				return
			}
			if days = cal.SchoolDaysBetween(date, through); len(days) == 0 {
				h.redirect(w, r, to, "", "There are no school days in that range.")
				return
			}
		} else if calProblem == "" && !cal.IsSchoolDay(date) {
			h.redirect(w, r, to, "", dayLabel(date)+" is not a school day on your calendar, so nothing was saved.")
			return
		}
		added, removed := 0, 0
		for _, v := range roster {
			checked := r.PostForm.Get("absent_"+v.Ref) == "on"
			for _, day := range days {
				if checked {
					if ok, err := store.Add(v.Instance, v.Ref, day); err != nil {
						h.redirect(w, r, to, "", "Could not save: "+err.Error())
						return
					} else if ok {
						added++
					}
				} else if len(days) == 1 && store.Remove(v.Instance, v.Ref, day) {
					// One day shows who is already marked, so unticking clears it.
					removed++
				}
			}
		}
		if err := store.Save(); err != nil {
			h.redirect(w, r, to, "", "Could not save the absences; check file access.")
			return
		}
		h.redirect(w, r, to, fmt.Sprintf("Saved: %d added, %d cleared.", added, removed), "")
		return
	}

	data := absencesData{Appearance: appearanceFor(h.root, h.secret), Secret: h.secret, Message: message, Notice: notice, Version: store.Version,
		CourseID: course.ID, CourseName: course.Nickname, Date: date, DateLabel: dayLabel(date), Calendar: calStatus, CalendarProblem: calProblem}
	for _, active := range config.Courses {
		if active.Current {
			data.Tabs = append(data.Tabs, courseTab{ID: active.ID, Name: active.Nickname, Selected: active.ID == course.ID})
		}
	}
	names := map[string]string{}
	for _, v := range roster {
		names[v.Ref] = v.Name
		row := absenceRow{Ref: v.Ref, Name: v.Name}
		for _, d := range store.Dates(v.Instance, v.Ref) {
			if d == date {
				row.Absent = true
			}
		}
		data.Rows = append(data.Rows, row)
	}
	for _, a := range store.Rows {
		if name, ok := names[a.Ref]; ok && a.Instance == config.Instance {
			data.Entries = append(data.Entries, absenceEntry{Ref: a.Ref, Name: name, Date: a.Date, Label: dayLabel(a.Date)})
		}
	}
	sort.SliceStable(data.Entries, func(i, j int) bool {
		if data.Entries[i].Date != data.Entries[j].Date {
			return data.Entries[i].Date > data.Entries[j].Date
		}
		return strings.ToLower(data.Entries[i].Name) < strings.ToLower(data.Entries[j].Name)
	})
	h.render(w, 200, tmpl("absences"), data)
}

func (h *Hub) absenceRemove(w http.ResponseWriter, r *http.Request) {
	if !h.checkPost(w, r) || h.needWorkspace(w) {
		return
	}
	config, _, err := h.readConfig()
	if err != nil {
		h.redirect(w, r, "/absences", "", "Could not read context/courses.md.")
		return
	}
	course, _ := selectedCourse(config, r.PostForm.Get("course"))
	to := fmt.Sprintf("/absences?course=%d", course.ID)
	if view := r.PostForm.Get("view"); view != "" {
		if _, err := time.Parse("2006-01-02", view); err == nil {
			to += "&date=" + view
		}
	}
	store, err := workspace.OpenAbsences(h.root)
	if err != nil {
		h.redirect(w, r, to, "", "Could not read the saved absences.")
		return
	}
	if !store.Remove(config.Instance, r.PostForm.Get("ref"), r.PostForm.Get("date")) {
		h.redirect(w, r, to, "", "That absence was already removed.")
		return
	}
	if err := store.Save(); err != nil {
		h.redirect(w, r, to, "", "Could not save the absences; check file access.")
		return
	}
	h.redirect(w, r, to, "Absence removed.", "")
}
