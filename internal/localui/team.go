package localui

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"canvasexpertse/internal/canvas"
	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/platform"
	"canvasexpertse/internal/workflow"
	"canvasexpertse/internal/workspace"
)

// teamIdentity is what Canvas says about the owner of a key.
type teamIdentity struct {
	ID                     int64
	ShortName, Name, Email string
}

type teamLogRow struct{ Time, Course, Method, Path string }

type teamCard struct {
	Label, Name, Email  string
	KeyOnPC             bool
	Expiry, ExpiryState string
	Courses             []string
	CourseCount         int
	Log                 []teamLogRow
}

type teamData struct {
	Appearance              appearanceData
	Secret, Message, Notice string
	Cards                   []teamCard
}

type teamAddData struct {
	Appearance                      appearanceData
	Secret, Message, Expires, Email string
}

type teamCourseRow struct {
	ID         int64
	Name, Code string
}

type teamCoursesData struct {
	Appearance              appearanceData
	Secret, Message, Notice string
	Label, Name             string
	Rows                    []teamCourseRow
}

func init() {
	post, ap := []string{http.MethodPost}, []string{"appearance.js"}
	register(
		page{Path: "/team", Template: "team", Scripts: ap, Nav: []navSlot{{Primary: "Classroom", Label: "Team", Order: 23}}, Handle: (*Hub).teamPage},
		page{Path: "/team/add", Methods: []string{http.MethodGet, http.MethodPost}, Template: "team_add", Scripts: ap, Handle: (*Hub).teamAddPage},
		page{Path: "/team/courses", Methods: []string{http.MethodGet, http.MethodPost}, Template: "team_courses", Scripts: ap, Handle: (*Hub).teamCoursesPage},
		page{Path: "/team/test", Methods: post, Handle: (*Hub).teamTest},
		page{Path: "/team/remove", Methods: post, Handle: (*Hub).teamRemove},
		page{Path: "/team/name", Methods: post, Handle: (*Hub).teamRename},
		page{Path: "/team/email", Methods: post, Handle: (*Hub).teamEmail},
	)
}

// teamKeyStatus turns key_expires into the text shown on the Team page.
func teamKeyStatus(t workspace.Teammate, now time.Time) (text, state string) {
	switch state = t.ExpiryState(now); state {
	case workspace.KeyExpired:
		return "Expired " + t.KeyExpires, state
	case workspace.KeySoon:
		return "Expires soon: " + t.KeyExpires, state
	}
	return "Expires " + t.KeyExpires, state
}

// teamWarnings lists one line per teammate whose key expires soon or has
// expired. It reads team.md only; it never asks Canvas.
func (h *Hub) teamWarnings() []string {
	team, err := workspace.ReadTeam(h.root)
	if err != nil {
		return nil
	}
	var out []string
	for _, t := range team.Teammates {
		switch t.ExpiryState(time.Now()) {
		case workspace.KeyExpired:
			out = append(out, t.Name+"'s Canvas key expired "+t.KeyExpires+".")
		case workspace.KeySoon:
			out = append(out, t.Name+"'s Canvas key expires "+t.KeyExpires+".")
		}
	}
	return out
}

func (h *Hub) loadTeammate(id int64) (platform.Credential, error) {
	if h.loadTeammateFn != nil {
		return h.loadTeammateFn(id)
	}
	return platform.LoadTeammateCredential(id)
}

func (h *Hub) saveTeammate(id int64, c platform.Credential) error {
	if h.saveTeammateFn != nil {
		return h.saveTeammateFn(id, c)
	}
	return platform.SaveTeammateCredential(id, c)
}

func (h *Hub) deleteTeammate(id int64) error {
	if h.deleteTeammateFn != nil {
		return h.deleteTeammateFn(id)
	}
	return platform.DeleteTeammateCredential(id)
}

func (h *Hub) identity(ctx context.Context, c platform.Credential) (teamIdentity, error) {
	if h.teammateIdentityFn != nil {
		return h.teammateIdentityFn(ctx, c)
	}
	client, err := canvas.New(c.Origin, c.Token)
	if err != nil {
		return teamIdentity{}, err
	}
	var self struct {
		ID        int64  `json:"id"`
		Name      string `json:"name"`
		ShortName string `json:"short_name"`
	}
	if err = client.Get(ctx, "/api/v1/users/self", &self); err != nil {
		return teamIdentity{}, err
	}
	id := teamIdentity{ID: self.ID, Name: self.Name, ShortName: self.ShortName}
	var profile struct {
		ShortName string `json:"short_name"`
		Email     string `json:"primary_email"`
	}
	// The profile only adds the email and a nicer name; the key already works.
	if client.Get(ctx, "/api/v1/users/self/profile", &profile) == nil {
		id.Email = profile.Email
		if id.ShortName == "" {
			id.ShortName = profile.ShortName
		}
	}
	return id, nil
}

func (h *Hub) teammateCourses(ctx context.Context, c platform.Credential) ([]workflow.Course, error) {
	if h.teammateCoursesFn != nil {
		return h.teammateCoursesFn(ctx, c)
	}
	client, err := canvas.New(c.Origin, c.Token)
	if err != nil {
		return nil, err
	}
	return workflow.Courses(ctx, client)
}

// teamInstance returns the configured Canvas address, or "" when it is unset.
func teamInstance(config domain.CourseConfig) string {
	if strings.TrimSpace(config.Instance) == "" || strings.Contains(config.Instance, starterInstance) {
		return ""
	}
	return config.Instance
}

func (h *Hub) teamPage(w http.ResponseWriter, r *http.Request) {
	if h.needWorkspace(w) {
		return
	}
	message, notice := h.takeFlash()
	team, err := workspace.ReadTeam(h.root)
	if err != nil {
		message = "Could not read context/team.md: " + err.Error()
	}
	config, _, _ := h.readConfig()
	now := time.Now()
	var cards []teamCard
	for _, t := range team.Teammates {
		_, keyErr := h.loadTeammate(t.CanvasUserID)
		card := teamCard{Label: t.Label, Name: t.Name, Email: t.Email, KeyOnPC: keyErr == nil}
		card.Expiry, card.ExpiryState = teamKeyStatus(t, now)
		for _, c := range config.Courses {
			if strings.EqualFold(c.Owner, t.Label) {
				card.Courses = append(card.Courses, c.Nickname)
			}
		}
		card.CourseCount = len(card.Courses)
		entries, _ := workspace.ReadTeamLog(h.root, t.Label, 10)
		for _, e := range entries {
			card.Log = append(card.Log, teamLogRow{e.Time, e.Course, e.Method, e.Path})
		}
		cards = append(cards, card)
	}
	h.render(w, 200, tmpl("team"), teamData{Appearance: appearanceFor(h.root, h.secret), Secret: h.secret, Message: message, Notice: notice, Cards: cards})
}

// teamExpiryDefault is the school year's last day when it is still ahead.
func (h *Hub) teamExpiryDefault() string {
	cal, err := workspace.ReadSchoolCalendar(h.root)
	if err != nil {
		return ""
	}
	end, err := time.Parse("2006-01-02", cal.EndDate)
	if err != nil || !end.After(time.Now()) {
		return ""
	}
	return cal.EndDate
}

func (h *Hub) teamAddForm(w http.ResponseWriter, status int, message, expires, email string) {
	h.render(w, status, tmpl("team_add"), teamAddData{Appearance: appearanceFor(h.root, h.secret), Secret: h.secret, Message: message, Expires: expires, Email: email})
}

func (h *Hub) teamAddPage(w http.ResponseWriter, r *http.Request) {
	if h.needWorkspace(w) {
		return
	}
	if r.Method == http.MethodGet {
		h.teamAddForm(w, 200, "", h.teamExpiryDefault(), "")
		return
	}
	h.addTeammate(w, r)
}

var labelStrip = regexp.MustCompile(`[^a-z0-9-]+`)

// teamLabelFor makes a stable label from the last word of the Canvas short name:
// "Ms. Turner" becomes turner; a clash becomes turner-2, turner-3.
func teamLabelFor(name string, team workspace.TeamConfig) string {
	words := strings.Fields(name)
	base := ""
	if len(words) > 0 {
		base = labelStrip.ReplaceAllString(strings.ToLower(words[len(words)-1]), "")
	}
	base = strings.TrimLeft(base, "0123456789-")
	if base == "" {
		base = "teammate"
	}
	if len(base) > 20 {
		base = base[:20]
	}
	label := base
	for n := 2; ; n++ {
		if _, taken := team.Find(label); !taken {
			return label
		}
		label = base + "-" + strconv.Itoa(n)
	}
}

func (h *Hub) addTeammate(w http.ResponseWriter, r *http.Request) {
	if !h.checkPost(w, r) {
		return
	}
	token := strings.TrimSpace(r.PostForm.Get("token"))
	r.PostForm.Del("token")
	r.Form.Del("token")
	defer func() { token = "" }()
	expires := strings.TrimSpace(r.PostForm.Get("key_expires"))
	typedEmail := strings.TrimSpace(r.PostForm.Get("email"))
	fail := func(msg string) { h.teamAddForm(w, 400, msg, expires, typedEmail) }
	config, _, err := h.readConfig()
	instance := teamInstance(config)
	if err != nil || instance == "" {
		fail("Set up your own Canvas connection first.")
		return
	}
	if token == "" {
		fail("Paste the Canvas token.")
		return
	}
	if typedEmail != "" && !validTeamEmail(typedEmail) {
		fail("That email doesn't look like an address. Leave it blank to use the email in Canvas.")
		return
	}
	if _, err = time.Parse("2006-01-02", expires); err != nil {
		fail("Choose the day this key expires. It is the date set in Canvas.")
		return
	}
	operator, err := h.loadSavedCredential()
	if err != nil {
		fail("Set up your own Canvas connection first.")
		return
	}
	client, err := canvas.New(instance, token)
	if err != nil {
		fail("Use the token exactly as Canvas showed it.")
		return
	}
	cred := platform.Credential{Origin: client.Origin(), Token: token}
	ctx, cancel := h.reqCtx()
	defer cancel()
	who, err := h.identity(ctx, cred)
	if err != nil {
		var ce *canvas.Error
		if errors.As(err, &ce) && ce.Kind == "authentication" {
			fail("Canvas didn't accept that token. Check that the whole token was pasted and that it hasn't expired.")
		} else {
			fail("Canvas could not be reached to check that token. Try again.")
		}
		return
	}
	if who.ID <= 0 {
		fail("Canvas didn't say whose token that is. Try again.")
		return
	}
	self, err := h.identity(ctx, operator)
	if err != nil {
		fail("Canvas could not be reached to check your own connection. Try again.")
		return
	}
	if self.ID == who.ID {
		fail("That's your own key. Each teammate adds their own key.")
		return
	}
	team, err := workspace.ReadTeam(h.root)
	if err != nil {
		fail("Could not read context/team.md: " + err.Error())
		return
	}
	name := strings.TrimSpace(who.ShortName)
	if name == "" {
		name = strings.TrimSpace(who.Name)
	}
	if name == "" {
		fail("Canvas didn't give a name for that key. Try again.")
		return
	}
	for _, t := range team.Teammates {
		if t.CanvasUserID == who.ID {
			fail(t.Name + " is already on the team. Remove that entry first to replace the key.")
			return
		}
	}
	// An email typed here wins: it is where families' replies should go.
	email := typedEmail
	if email == "" {
		if email = strings.TrimSpace(who.Email); !validTeamEmail(email) {
			email = ""
		}
	}
	mate := workspace.Teammate{Label: teamLabelFor(name, team), Name: name, CanvasUserID: who.ID, Email: email, KeyExpires: expires}
	if err = h.saveTeammate(who.ID, cred); err != nil {
		fail("Windows could not save the key. Try again or check local credential access.")
		return
	}
	team.Teammates = append(team.Teammates, mate)
	if err = workspace.WriteTeam(h.root, team); err != nil {
		_ = h.deleteTeammate(who.ID)
		fail("Could not save context/team.md, so the key was not kept: " + err.Error())
		return
	}
	problem := ""
	if err = workspace.EnsureTeammateNotes(h.root, mate); err != nil {
		problem = "Could not create " + name + "'s notes file: " + err.Error()
	}
	h.redirect(w, r, "/team/courses?label="+mate.Label, name+"'s key is saved on this PC. Choose which of "+name+"'s courses to add.", problem)
}

// teamFromRequest finds the teammate named by a label, or redirects to /team.
func (h *Hub) teamFromRequest(w http.ResponseWriter, r *http.Request, label string) (workspace.TeamConfig, workspace.Teammate, bool) {
	team, err := workspace.ReadTeam(h.root)
	if err != nil {
		h.redirect(w, r, "/team", "", "Could not read context/team.md: "+err.Error())
		return team, workspace.Teammate{}, false
	}
	t, ok := team.Find(label)
	if !ok {
		h.redirect(w, r, "/team", "", "That teammate isn't on the team list.")
		return team, t, false
	}
	return team, t, true
}

func (h *Hub) teamCoursesPage(w http.ResponseWriter, r *http.Request) {
	if h.needWorkspace(w) {
		return
	}
	if r.Method == http.MethodPost {
		h.saveTeamCourses(w, r)
		return
	}
	_, mate, ok := h.teamFromRequest(w, r, r.URL.Query().Get("label"))
	if !ok {
		return
	}
	cred, err := h.loadTeammate(mate.CanvasUserID)
	if err != nil {
		h.redirect(w, r, "/team", "", mate.MissingKeyMessage())
		return
	}
	message, notice := h.takeFlash()
	config, _, _ := h.readConfig()
	configured := map[int64]bool{}
	for _, c := range config.Courses {
		configured[c.ID] = true
	}
	ctx, cancel := h.reqCtx()
	defer cancel()
	list, err := h.teammateCourses(ctx, cred)
	if err != nil {
		message = "Could not list " + mate.Name + "'s Canvas courses: " + err.Error()
	}
	var rows []teamCourseRow
	for _, c := range list {
		if !configured[c.ID] {
			rows = append(rows, teamCourseRow{ID: c.ID, Name: c.Name, Code: c.Code})
		}
	}
	h.render(w, 200, tmpl("team_courses"), teamCoursesData{Appearance: appearanceFor(h.root, h.secret), Secret: h.secret, Message: message, Notice: notice, Label: mate.Label, Name: mate.Name, Rows: rows})
}

func (h *Hub) saveTeamCourses(w http.ResponseWriter, r *http.Request) {
	if !h.checkPost(w, r) {
		return
	}
	_, mate, ok := h.teamFromRequest(w, r, r.PostForm.Get("label"))
	if !ok {
		return
	}
	back := "/team/courses?label=" + mate.Label
	config, body, err := h.readConfig()
	if err != nil {
		h.redirect(w, r, back, "", "Could not read context/courses.md: "+err.Error())
		return
	}
	if teamInstance(config) == "" {
		h.redirect(w, r, back, "", "Set up your own Canvas connection first.")
		return
	}
	have := map[int64]bool{}
	for _, c := range config.Courses {
		have[c.ID] = true
	}
	added := 0
	for _, raw := range r.PostForm["id"] {
		id, convErr := strconv.ParseInt(raw, 10, 64)
		if convErr != nil || id <= 0 || have[id] || r.PostForm.Get("pick_"+raw) != "on" {
			continue
		}
		nick := strings.TrimSpace(r.PostForm.Get("nick_" + raw))
		if nick == "" {
			h.redirect(w, r, back, "", "Every course you add needs a nickname.")
			return
		}
		config.Courses = append(config.Courses, domain.Course{Nickname: nick, ID: id, Current: true, Owner: mate.Label})
		have[id] = true
		added++
	}
	if added == 0 {
		h.redirect(w, r, back, "", "Tick at least one course to add.")
		return
	}
	if err = domain.ValidateConfig(config); err != nil {
		h.redirect(w, r, back, "", "Not saved: nicknames must be unique and file-safe (no / \\ : * ? \" < > |). "+err.Error())
		return
	}
	if err = workspace.WriteTeacherDocument(h.root, "context/courses.md", config, body); err != nil {
		h.redirect(w, r, back, "", "Could not save context/courses.md.")
		return
	}
	h.loaded = nil
	h.redirect(w, r, "/team", fmt.Sprintf("Added %d of %s's courses to your list.", added, mate.Name), "")
}

func (h *Hub) teamTest(w http.ResponseWriter, r *http.Request) {
	if h.needWorkspace(w) || !h.checkPost(w, r) {
		return
	}
	_, mate, ok := h.teamFromRequest(w, r, r.PostForm.Get("label"))
	if !ok {
		return
	}
	cred, err := h.loadTeammate(mate.CanvasUserID)
	if err != nil {
		h.redirect(w, r, "/team", "", mate.MissingKeyMessage())
		return
	}
	ctx, cancel := h.reqCtx()
	defer cancel()
	_, err = h.identity(ctx, cred)
	var ce *canvas.Error
	switch {
	case err == nil:
		h.redirect(w, r, "/team", mate.Name+"'s key works.", "")
	case errors.As(err, &ce) && ce.Kind == "authentication":
		h.redirect(w, r, "/team", "", "Canvas no longer accepts "+mate.Name+"'s key. Ask "+mate.Name+" to make a new one, then remove and re-add.")
	default:
		h.redirect(w, r, "/team", "", "Canvas could not be reached to check "+mate.Name+"'s key. Try again.")
	}
}

func (h *Hub) teamRemove(w http.ResponseWriter, r *http.Request) {
	if h.needWorkspace(w) || !h.checkPost(w, r) {
		return
	}
	team, mate, ok := h.teamFromRequest(w, r, r.PostForm.Get("label"))
	if !ok {
		return
	}
	// Courses go first: an owner missing from the team list would stop every
	// command, so a removed teammate never leaves courses behind.
	config, body, err := h.readConfig()
	if err != nil {
		h.redirect(w, r, "/team", "", "Could not read context/courses.md, so nothing was removed.")
		return
	}
	var keep []domain.Course
	for _, c := range config.Courses {
		if !strings.EqualFold(c.Owner, mate.Label) {
			keep = append(keep, c)
		}
	}
	removed := len(config.Courses) - len(keep)
	if removed > 0 {
		config.Courses = keep
		if err = workspace.WriteTeacherDocument(h.root, "context/courses.md", config, body); err != nil {
			h.redirect(w, r, "/team", "", "Could not update context/courses.md, so nothing was removed.")
			return
		}
		h.loaded = nil
	}
	line := fmt.Sprintf("Removed %s: %d courses removed from your list.", mate.Name, removed)
	if err = h.deleteTeammate(mate.CanvasUserID); err != nil {
		h.redirect(w, r, "/team", "", line+" Windows could not delete "+mate.Name+"'s key; try Remove again.")
		return
	}
	var kept []workspace.Teammate
	for _, t := range team.Teammates {
		if t.Label != mate.Label {
			kept = append(kept, t)
		}
	}
	team.Teammates = kept
	if err = workspace.WriteTeam(h.root, team); err != nil {
		h.redirect(w, r, "/team", "", line+" The key is deleted, but context/team.md could not be saved: "+err.Error())
		return
	}
	line += " Key deleted."
	h.redirect(w, r, "/team", line+" Notes and the action log are kept.", "")
}

func (h *Hub) teamRename(w http.ResponseWriter, r *http.Request) {
	if h.needWorkspace(w) || !h.checkPost(w, r) {
		return
	}
	team, mate, ok := h.teamFromRequest(w, r, r.PostForm.Get("label"))
	if !ok {
		return
	}
	name := strings.TrimSpace(r.PostForm.Get("name"))
	for i := range team.Teammates {
		if team.Teammates[i].Label == mate.Label {
			team.Teammates[i].Name = name
			team.Teammates[i].Email = strings.TrimSpace(r.PostForm.Get("email"))
		}
	}
	if err := workspace.WriteTeam(h.root, team); err != nil {
		h.redirect(w, r, "/team", "", "Not saved: "+err.Error())
		return
	}
	h.redirect(w, r, "/team", "Saved "+name+".", "")
}

// teamEmail saves where families' replies about a teammate's students go.
func (h *Hub) teamEmail(w http.ResponseWriter, r *http.Request) {
	if h.needWorkspace(w) || !h.checkPost(w, r) {
		return
	}
	team, mate, ok := h.teamFromRequest(w, r, r.PostForm.Get("label"))
	if !ok {
		return
	}
	email := strings.TrimSpace(r.PostForm.Get("email"))
	if email != "" && !validTeamEmail(email) {
		h.redirect(w, r, "/team", "", "That email doesn't look like an address, so "+mate.Name+"'s email was not changed.")
		return
	}
	for i := range team.Teammates {
		if team.Teammates[i].Label == mate.Label {
			team.Teammates[i].Email = email
		}
	}
	if err := workspace.WriteTeam(h.root, team); err != nil {
		h.redirect(w, r, "/team", "", "Not saved: "+err.Error())
		return
	}
	if email == "" {
		h.redirect(w, r, "/team", "Removed "+mate.Name+"'s email. Family emails about "+mate.Name+"'s students are held until one is added.", "")
		return
	}
	h.redirect(w, r, "/team", "Family replies about "+mate.Name+"'s students now go to "+email+".", "")
}

// validTeamEmail accepts one plain address, the form Outlook needs for Reply-To.
func validTeamEmail(email string) bool {
	at := strings.Index(email, "@")
	return at > 0 && at < len(email)-1 && strings.Count(email, "@") == 1 && !strings.ContainsAny(email, " \t<>,;")
}

// teamOwnerName shows a course owner's display name, or "" for your own courses.
func teamOwnerName(label string, team workspace.TeamConfig) string {
	if label == "" {
		return ""
	}
	if t, ok := team.Find(label); ok {
		return t.Name
	}
	return label
}
