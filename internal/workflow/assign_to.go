package workflow

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

// Assign To by differentiation tag. Every content kind reduces to the same
// shape: read the live audience, work out the audience the document wants,
// write only when they differ, then read back. Reconciling against live state
// (instead of replaying saved writes) means a rerun never repeats an accepted
// change, and an uncertain write is settled by looking at Canvas.

type audRow struct {
	ID         int64
	Title      string
	GroupID    int64
	SectionID  int64
	StudentIDs []int64
	Course     bool // the "Everyone" row some Canvas shapes use
	Unknown    bool // a shape this Engine cannot rewrite faithfully
	Inherited  bool // shown on an item because its module grants it
	live       *AssignmentOverrideLive
	unlockAt   *string
	lockAt     *string
}

func (r audRow) key() string {
	switch {
	case r.Course:
		return "c"
	case r.GroupID > 0:
		return "g" + strconv.FormatInt(r.GroupID, 10)
	case r.SectionID > 0:
		return "s" + strconv.FormatInt(r.SectionID, 10)
	case len(r.StudentIDs) > 0:
		ids := append([]int64(nil), r.StudentIDs...)
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		parts := make([]string, len(ids))
		for i, id := range ids {
			parts[i] = strconv.FormatInt(id, 10)
		}
		return "u" + strings.Join(parts, ",")
	}
	return "?" + strconv.FormatInt(r.ID, 10)
}

type liveAud struct {
	Everyone bool
	Rows     []audRow // the item's own rows; never the Everyone row or inherited rows
	// Inherited are rows an item shows only because its module grants them.
	// They are never rewritten or deleted through the item.
	Inherited []audRow
}

func (a liveAud) keys() map[string]bool {
	m := map[string]bool{}
	for _, r := range a.Rows {
		m[r.key()] = true
	}
	return m
}

func (a liveAud) restricted() bool { return !a.Everyone }

// hasOwn reports whether an item carries an audience of its own rather than
// only the rows it inherits from a module.
func (a liveAud) hasOwn() bool {
	return len(a.Rows) > 0 || (a.restricted() && len(a.Inherited) == 0)
}

// addRow files a row as the item's own or as inherited from its module.
func (a *liveAud) addRow(r audRow) {
	if r.Inherited {
		a.Inherited = append(a.Inherited, r)
		return
	}
	a.Rows = append(a.Rows, r)
}

func (a liveAud) describe(cat TagCatalog) string {
	var parts []string
	if a.Everyone {
		parts = append(parts, "Everyone")
	}
	var tags []string
	sections, students, other := 0, 0, 0
	for _, r := range a.Rows {
		switch {
		case r.GroupID > 0:
			tags = append(tags, cat.label(r.GroupID))
		case r.SectionID > 0:
			sections++
		case len(r.StudentIDs) > 0:
			students += len(r.StudentIDs)
		default:
			other++
		}
	}
	sort.Strings(tags)
	if len(tags) > 0 {
		parts = append(parts, "tags: "+strings.Join(tags, ", "))
	}
	if sections > 0 {
		parts = append(parts, fmt.Sprintf("%d section(s)", sections))
	}
	if students > 0 {
		parts = append(parts, fmt.Sprintf("%d student(s)", students))
	}
	if other > 0 {
		parts = append(parts, fmt.Sprintf("%d other row(s)", other))
	}
	if len(a.Inherited) > 0 {
		var via []string
		for _, r := range a.Inherited {
			if r.GroupID > 0 {
				via = append(via, cat.label(r.GroupID))
			} else {
				via = append(via, "other")
			}
		}
		sort.Strings(via)
		parts = append(parts, "via module: "+strings.Join(via, ", "))
	}
	if len(parts) == 0 {
		return "nobody"
	}
	return strings.Join(parts, " + ")
}

type wantAud struct {
	Everyone bool
	Rows     []audRow // preserved non-tag rows, then the requested tag rows
	Dates    *domain.AssignToDates
}

func (w wantAud) keys() map[string]bool {
	m := map[string]bool{}
	for _, r := range w.Rows {
		m[r.key()] = true
	}
	return m
}

type assignSurface interface {
	Kind() string
	Name() string
	Read(ctx context.Context) (liveAud, error)
	Apply(ctx context.Context, live liveAud, want wantAud) error
}

// submissionCounter is implemented by graded surfaces so removing access can be
// held when students have already turned work in.
type submissionCounter interface {
	SubmittedCount(ctx context.Context) (int, error)
}

// ---- module -------------------------------------------------------------

type moduleSurface struct {
	s    *Service
	cid  int64
	id   int64
	name string
}

func (m *moduleSurface) Kind() string { return "module" }
func (m *moduleSurface) Name() string { return m.name }

func jnum(v any) int64 {
	if n, ok := v.(float64); ok {
		return int64(n)
	}
	return 0
}

func jid(m map[string]any, flat, nested string) int64 {
	if id := jnum(m[flat]); id > 0 {
		return id
	}
	if sub, ok := m[nested].(map[string]any); ok {
		return jnum(sub["id"])
	}
	return 0
}

func parseModuleOverride(m map[string]any) audRow {
	row := audRow{ID: jnum(m["id"])}
	row.Title, _ = m["title"].(string)
	row.GroupID = jid(m, "group_id", "group")
	row.SectionID = jid(m, "course_section_id", "course_section")
	if jid(m, "course_id", "course") > 0 || m["set_type"] == "Course" {
		row.Course = true
	}
	if ids, ok := m["student_ids"].([]any); ok {
		for _, v := range ids {
			row.StudentIDs = append(row.StudentIDs, jnum(v))
		}
	}
	if list, ok := m["students"].([]any); ok {
		for _, v := range list {
			if sm, ok := v.(map[string]any); ok {
				row.StudentIDs = append(row.StudentIDs, jnum(sm["id"]))
			}
		}
	}
	if !row.Course && row.GroupID == 0 && row.SectionID == 0 && len(row.StudentIDs) == 0 {
		row.Unknown = true
	}
	return row
}

func (m *moduleSurface) path() string {
	return fmt.Sprintf("/api/v1/courses/%d/modules/%d/assignment_overrides", m.cid, m.id)
}

func (m *moduleSurface) Read(ctx context.Context) (liveAud, error) {
	var raw []map[string]any
	if err := m.s.Client.List(ctx, m.path()+"?per_page=100", &raw); err != nil {
		return liveAud{}, err
	}
	out := liveAud{Everyone: len(raw) == 0}
	for _, item := range raw {
		row := parseModuleOverride(item)
		if row.Course {
			out.Everyone = true
			continue
		}
		out.Rows = append(out.Rows, row)
	}
	return out, nil
}

func rowPayload(r audRow) (map[string]any, error) {
	switch {
	case r.GroupID > 0:
		return map[string]any{"group_id": r.GroupID}, nil
	case r.SectionID > 0:
		return map[string]any{"course_section_id": r.SectionID}, nil
	case len(r.StudentIDs) > 0:
		return map[string]any{"student_ids": r.StudentIDs}, nil
	}
	return nil, fmt.Errorf("an existing audience row has a shape this Engine cannot preserve; use replace mode or change it in Canvas")
}

func (m *moduleSurface) Apply(ctx context.Context, live liveAud, want wantAud) error {
	overrides := []map[string]any{}
	if !want.Everyone {
		for _, r := range want.Rows {
			p, err := rowPayload(r)
			if err != nil {
				return err
			}
			overrides = append(overrides, p)
		}
	}
	return m.s.Client.WriteAccepted(ctx, http.MethodPut, m.path(), map[string]any{"overrides": overrides}, nil)
}

// ---- assignment, New Quiz and graded discussion --------------------------

type assignmentSurface struct {
	s    *Service
	cid  int64
	id   int64
	name string
	kind string
}

func (a *assignmentSurface) Kind() string { return a.kind }
func (a *assignmentSurface) Name() string { return a.name }

func (a *assignmentSurface) Read(ctx context.Context) (liveAud, error) {
	as, err := GetAssignment(ctx, a.s.Client, a.cid, a.id)
	if err != nil {
		return liveAud{}, err
	}
	if as.ID != a.id || as.CourseID != a.cid {
		return liveAud{}, fmt.Errorf("assignment %d is outside this course", a.id)
	}
	if as.GroupCategoryID != nil && *as.GroupCategoryID > 0 {
		return liveAud{}, fmt.Errorf("native group assignment: a differentiation tag cannot be its audience; use the group's own audience instead")
	}
	out := liveAud{Everyone: as.OnlyVisibleToOverrides == nil || !*as.OnlyVisibleToOverrides}
	for i := range as.Overrides {
		o := as.Overrides[i]
		row := audRow{ID: o.ID, Title: o.Title, GroupID: o.GroupID, SectionID: o.CourseSectionID, StudentIDs: o.StudentIDs, live: &o, Inherited: o.ContextModuleID > 0}
		if o.CourseID > 0 {
			continue
		}
		if row.GroupID == 0 && row.SectionID == 0 && len(row.StudentIDs) == 0 {
			row.Unknown = true
		}
		out.addRow(row)
	}
	return out, nil
}

func (a *assignmentSurface) SubmittedCount(ctx context.Context) (int, error) {
	subs, err := Submissions(ctx, a.s.Client, a.cid, a.id, false)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, sub := range subs {
		if sub.SubmittedAt != nil && *sub.SubmittedAt != "" {
			n++
		}
	}
	return n, nil
}

func (a *assignmentSurface) overridesURL() string {
	return fmt.Sprintf("/api/v1/courses/%d/assignments/%d/overrides", a.cid, a.id)
}

func (a *assignmentSurface) setVisibility(ctx context.Context, onlyOverrides bool) error {
	return a.s.Client.WriteAccepted(ctx, http.MethodPut, fmt.Sprintf("/api/v1/courses/%d/assignments/%d", a.cid, a.id), map[string]any{"assignment": map[string]any{"only_visible_to_overrides": onlyOverrides}}, nil)
}

func (a *assignmentSurface) Apply(ctx context.Context, live liveAud, want wantAud) error {
	wantKeys := want.keys()
	liveByKey := map[string]audRow{}
	for _, r := range live.Rows {
		liveByKey[r.key()] = r
	}
	dateFields := func(fields map[string]any) {
		if want.Dates == nil {
			return
		}
		for _, d := range []struct{ key, value string }{{"due_at", want.Dates.DueAt}, {"unlock_at", want.Dates.UnlockAt}, {"lock_at", want.Dates.LockAt}} {
			if d.value != "" {
				putOverrideDate(fields, d.key, domain.OptionalDate{Set: true, Value: d.value})
			}
		}
	}
	for _, r := range want.Rows {
		if r.GroupID == 0 {
			continue
		}
		existing, found := liveByKey[r.key()]
		if !found {
			fields := map[string]any{"group_id": r.GroupID}
			dateFields(fields)
			if err := a.s.Client.WriteAccepted(ctx, http.MethodPost, a.overridesURL(), map[string]any{"assignment_override": fields}, nil); err != nil {
				return err
			}
			continue
		}
		if want.Dates != nil && existing.live != nil && datesDiffer(*existing.live, *want.Dates) {
			fields := map[string]any{}
			preserveOverrideDates(fields, *existing.live)
			dateFields(fields)
			if err := a.s.Client.WriteAccepted(ctx, http.MethodPut, fmt.Sprintf("%s/%d", a.overridesURL(), existing.ID), map[string]any{"assignment_override": fields}, nil); err != nil {
				return err
			}
		}
	}
	if !want.Everyone && live.Everyone {
		if err := a.setVisibility(ctx, true); err != nil {
			return err
		}
	}
	if want.Everyone && !live.Everyone {
		if err := a.setVisibility(ctx, false); err != nil {
			return err
		}
	}
	for _, r := range live.Rows {
		if !wantKeys[r.key()] {
			if err := a.s.Client.WriteAccepted(ctx, http.MethodDelete, fmt.Sprintf("%s/%d", a.overridesURL(), r.ID), nil, nil); err != nil {
				return err
			}
		}
	}
	return nil
}

func datesDiffer(o AssignmentOverrideLive, d domain.AssignToDates) bool {
	for _, c := range []struct {
		live  *string
		value string
	}{{o.DueAt, d.DueAt}, {o.UnlockAt, d.UnlockAt}, {o.LockAt, d.LockAt}} {
		if c.value == "" {
			continue
		}
		v := c.value
		if !sameDate(c.live, &v) {
			return true
		}
	}
	return false
}

// ---- date_details: pages, ungraded discussions, classic quizzes -----------

type dateDetailsSurface struct {
	s     *Service
	path  string
	name  string
	kind  string
	unlck *string
	lck   *string
}

func (d *dateDetailsSurface) Kind() string { return d.kind }
func (d *dateDetailsSurface) Name() string { return d.name }

func (d *dateDetailsSurface) Read(ctx context.Context) (liveAud, error) {
	var det pageDateDetails
	if err := d.s.Client.Get(ctx, d.path, &det); err != nil {
		return liveAud{}, err
	}
	if det.OnlyVisibleToOverrides == nil {
		return liveAud{}, fmt.Errorf("Canvas did not report who can see %s", d.name)
	}
	var overrides []pageDateOverride
	if err := d.s.Client.ListField(ctx, d.path, "overrides", &overrides); err != nil {
		return liveAud{}, fmt.Errorf("%s override collection could not be read completely: %w", d.name, err)
	}
	d.unlck, d.lck = det.UnlockAt, det.LockAt
	out := liveAud{Everyone: !*det.OnlyVisibleToOverrides}
	for _, o := range overrides {
		row := audRow{ID: o.ID, Title: o.Title, GroupID: o.GroupID, SectionID: o.CourseSectionID, StudentIDs: o.StudentIDs, unlockAt: o.UnlockAt, lockAt: o.LockAt, Inherited: o.ContextModuleID > 0}
		if row.GroupID == 0 && row.SectionID == 0 && len(row.StudentIDs) == 0 {
			row.Unknown = true
		}
		out.addRow(row)
	}
	return out, nil
}

func (d *dateDetailsSurface) Apply(ctx context.Context, live liveAud, want wantAud) error {
	liveByKey := map[string]audRow{}
	for _, r := range live.Rows {
		liveByKey[r.key()] = r
	}
	payload := []map[string]any{}
	for _, r := range want.Rows {
		if existing, ok := liveByKey[r.key()]; ok {
			r = existing
		}
		fields := map[string]any{}
		if r.ID > 0 {
			fields["id"] = r.ID
		}
		if r.Title != "" {
			fields["title"] = r.Title
		}
		switch {
		case r.GroupID > 0:
			fields["group_id"] = r.GroupID
		case r.SectionID > 0:
			fields["course_section_id"] = r.SectionID
		case len(r.StudentIDs) > 0:
			fields["student_ids"] = r.StudentIDs
		default:
			return fmt.Errorf("an existing audience row has a shape this Engine cannot preserve; use replace mode or change it in Canvas")
		}
		if r.unlockAt != nil {
			fields["unlock_at"] = *r.unlockAt
		}
		if r.lockAt != nil {
			fields["lock_at"] = *r.lockAt
		}
		payload = append(payload, fields)
	}
	body := map[string]any{"only_visible_to_overrides": !want.Everyone, "assignment_overrides": payload}
	if d.unlck != nil {
		body["unlock_at"] = *d.unlck
	}
	if d.lck != nil {
		body["lock_at"] = *d.lck
	}
	return d.s.Client.WriteAccepted(ctx, http.MethodPut, d.path, body, nil)
}

// ---- resolution ---------------------------------------------------------

type liveModule struct {
	ID         int64            `json:"id"`
	Name       string           `json:"name"`
	Position   int              `json:"position"`
	ItemsCount int              `json:"items_count"`
	Items      []liveModuleItem `json:"items"`
}

type liveModuleItem struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	Type      string `json:"type"`
	ContentID int64  `json:"content_id"`
	PageURL   string `json:"page_url"`
}

func (it liveModuleItem) key() string {
	if it.Type == "Page" {
		return "Page:" + strings.ToLower(it.PageURL)
	}
	return it.Type + ":" + strconv.FormatInt(it.ContentID, 10)
}

// assignSession caches one run's reads: tags, modules with items, and each
// module's audience.
type assignSession struct {
	s       *Service
	course  domain.Course
	cat     TagCatalog
	modules []liveModule
	loaded  bool
	modAud  map[int64]liveAud
}

func (ss *assignSession) loadModules(ctx context.Context) error {
	if ss.loaded {
		return nil
	}
	var mods []liveModule
	if err := ss.s.Client.List(ctx, fmt.Sprintf("/api/v1/courses/%d/modules?include[]=items&per_page=100", ss.course.ID), &mods); err != nil {
		return err
	}
	for i := range mods {
		if len(mods[i].Items) == 0 && mods[i].ItemsCount > 0 {
			var items []liveModuleItem
			if err := ss.s.Client.List(ctx, fmt.Sprintf("/api/v1/courses/%d/modules/%d/items?per_page=100", ss.course.ID, mods[i].ID), &items); err != nil {
				return err
			}
			mods[i].Items = items
		}
	}
	ss.modules, ss.loaded = mods, true
	return nil
}

func (ss *assignSession) moduleAudience(ctx context.Context, m liveModule) (liveAud, error) {
	if a, ok := ss.modAud[m.ID]; ok {
		return a, nil
	}
	a, err := (&moduleSurface{s: ss.s, cid: ss.course.ID, id: m.ID, name: m.Name}).Read(ctx)
	if err != nil {
		return a, err
	}
	if ss.modAud == nil {
		ss.modAud = map[int64]liveAud{}
	}
	ss.modAud[m.ID] = a
	return a, nil
}

func (ss *assignSession) findModule(ctx context.Context, ref domain.Ref) (liveModule, error) {
	if err := ss.loadModules(ctx); err != nil {
		return liveModule{}, err
	}
	var matches []liveModule
	for _, m := range ss.modules {
		if (ref.ID > 0 && m.ID == ref.ID) || (ref.Text != "" && strings.EqualFold(strings.TrimSpace(m.Name), ref.Text)) {
			matches = append(matches, m)
		}
	}
	switch len(matches) {
	case 0:
		return liveModule{}, fmt.Errorf("no module matches %q in this course", ref)
	case 1:
		return matches[0], nil
	}
	return liveModule{}, fmt.Errorf("module %q is ambiguous (%d modules share that title); use its ID", ref, len(matches))
}

func (ss *assignSession) findPage(ctx context.Context, ref domain.Ref) (surf assignSurface, err error) {
	var pages []struct {
		PageID int64  `json:"page_id"`
		URL    string `json:"url"`
		Title  string `json:"title"`
	}
	if err = ss.s.Client.List(ctx, fmt.Sprintf("/api/v1/courses/%d/pages?per_page=100", ss.course.ID), &pages); err != nil {
		return nil, err
	}
	type hit struct {
		id    int64
		title string
	}
	var matches []hit
	for _, p := range pages {
		if (ref.ID > 0 && p.PageID == ref.ID) || (ref.Text != "" && (strings.EqualFold(p.URL, ref.Text) || strings.EqualFold(strings.TrimSpace(p.Title), ref.Text))) {
			matches = append(matches, hit{p.PageID, p.Title})
		}
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("no page matches %q in this course", ref)
	case 1:
		return &dateDetailsSurface{s: ss.s, path: pageDateDetailsPath(ss.course.ID, matches[0].id), name: matches[0].title, kind: "page"}, nil
	}
	return nil, fmt.Errorf("page %q is ambiguous (%d pages share it); use its page ID", ref, len(matches))
}

func (ss *assignSession) assignmentSurfaceFor(a Assignment, kind string) assignSurface {
	if a.QuizID != nil && *a.QuizID > 0 && !a.NewQuiz {
		return &dateDetailsSurface{s: ss.s, path: fmt.Sprintf("/api/v1/courses/%d/quizzes/%d/date_details", ss.course.ID, *a.QuizID), name: a.Name, kind: "classic quiz"}
	}
	return &assignmentSurface{s: ss.s, cid: ss.course.ID, id: a.ID, name: a.Name, kind: kind}
}

func isQuizAssignment(a Assignment) bool { return a.NewQuiz || (a.QuizID != nil && *a.QuizID > 0) }

func (ss *assignSession) findAssignment(ctx context.Context, ref domain.Ref, kind string) (assignSurface, error) {
	if ref.ID > 0 {
		a, err := GetAssignment(ctx, ss.s.Client, ss.course.ID, ref.ID)
		if err != nil {
			return nil, err
		}
		if a.ID != ref.ID || a.CourseID != ss.course.ID {
			return nil, fmt.Errorf("assignment %d is outside this course", ref.ID)
		}
		if kind == "quiz" && !isQuizAssignment(a) {
			return nil, fmt.Errorf("%q is not a quiz; name it as an assignment", a.Name)
		}
		return ss.assignmentSurfaceFor(a, kind), nil
	}
	var list []Assignment
	if err := ss.s.Client.List(ctx, fmt.Sprintf("/api/v1/courses/%d/assignments?per_page=100", ss.course.ID), &list); err != nil {
		return nil, err
	}
	var matches []Assignment
	for _, a := range list {
		if strings.EqualFold(strings.TrimSpace(a.Name), ref.Text) && (kind != "quiz" || isQuizAssignment(a)) {
			matches = append(matches, a)
		}
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("no %s matches %q in this course", kind, ref)
	case 1:
		matches[0].CourseID = ss.course.ID
		return ss.assignmentSurfaceFor(matches[0], kind), nil
	}
	return nil, fmt.Errorf("%s %q is ambiguous (%d share that title); use its assignment ID", kind, ref, len(matches))
}

func (ss *assignSession) findDiscussion(ctx context.Context, ref domain.Ref) (assignSurface, error) {
	var topics []struct {
		ID           int64  `json:"id"`
		Title        string `json:"title"`
		AssignmentID *int64 `json:"assignment_id"`
	}
	if err := ss.s.Client.List(ctx, fmt.Sprintf("/api/v1/courses/%d/discussion_topics?per_page=100", ss.course.ID), &topics); err != nil {
		return nil, err
	}
	var matches []int
	for i, t := range topics {
		if (ref.ID > 0 && t.ID == ref.ID) || (ref.Text != "" && strings.EqualFold(strings.TrimSpace(t.Title), ref.Text)) {
			matches = append(matches, i)
		}
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("no discussion matches %q in this course", ref)
	case 1:
		t := topics[matches[0]]
		if t.AssignmentID != nil && *t.AssignmentID > 0 {
			return &assignmentSurface{s: ss.s, cid: ss.course.ID, id: *t.AssignmentID, name: t.Title, kind: "graded discussion"}, nil
		}
		return &dateDetailsSurface{s: ss.s, path: fmt.Sprintf("/api/v1/courses/%d/discussion_topics/%d/date_details", ss.course.ID, t.ID), name: t.Title, kind: "discussion"}, nil
	}
	return nil, fmt.Errorf("discussion %q is ambiguous (%d share that title); use its ID", ref, len(matches))
}

// itemSurface maps one module item to its audience surface; ok is false for
// entries that carry no audience of their own (headers, links, files).
func (ss *assignSession) itemSurface(ctx context.Context, it liveModuleItem) (assignSurface, bool, error) {
	switch it.Type {
	case "Assignment":
		a, err := GetAssignment(ctx, ss.s.Client, ss.course.ID, it.ContentID)
		if err != nil {
			return nil, false, err
		}
		return ss.assignmentSurfaceFor(a, "assignment"), true, nil
	case "Quiz":
		return &dateDetailsSurface{s: ss.s, path: fmt.Sprintf("/api/v1/courses/%d/quizzes/%d/date_details", ss.course.ID, it.ContentID), name: it.Title, kind: "classic quiz"}, true, nil
	case "Page":
		return &dateDetailsSurface{s: ss.s, path: fmt.Sprintf("/api/v1/courses/%d/pages/%s/date_details", ss.course.ID, url.PathEscape(it.PageURL)), name: it.Title, kind: "page"}, true, nil
	case "Discussion":
		s, err := ss.findDiscussion(ctx, domain.Ref{ID: it.ContentID})
		return s, err == nil, err
	}
	return nil, false, nil
}

// ---- engine -------------------------------------------------------------

type AssignToResult struct {
	Target  string   `json:"target"`
	Kind    string   `json:"kind"`
	Before  string   `json:"before,omitempty"`
	After   string   `json:"after,omitempty"`
	Status  string   `json:"status"` // preview, unchanged, succeeded, held
	Message string   `json:"message,omitempty"`
	Notes   []string `json:"notes,omitempty"`
}

type AssignToReport struct {
	Path    string           `json:"path,omitempty"`
	Status  string           `json:"status"` // completed, partial, preview
	Results []AssignToResult `json:"results"`
}

func (r AssignToResult) ok() bool {
	return r.Status == "succeeded" || r.Status == "unchanged" || r.Status == "preview"
}

type applyOptions struct {
	mode        string
	groups      []resolvedTag
	everyone    bool
	explicit    bool
	dates       *domain.AssignToDates
	allowRemove bool
	preview     bool
}

func sameAudience(live liveAud, want wantAud, surf assignSurface) bool {
	if len(live.Rows) == 0 && len(live.Inherited) > 0 && !live.Everyone && want.Everyone && len(want.Rows) == 0 {
		// The item has no restriction of its own; it only follows its module.
		return true
	}
	if live.Everyone != want.Everyone {
		return false
	}
	if _, isModule := surf.(*moduleSurface); isModule && want.Everyone {
		return len(live.Rows) == 0
	}
	lk, wk := live.keys(), want.keys()
	if len(lk) != len(wk) {
		return false
	}
	for k := range wk {
		if !lk[k] {
			return false
		}
	}
	if want.Dates != nil {
		for _, r := range live.Rows {
			if r.GroupID > 0 && r.live != nil && datesDiffer(*r.live, *want.Dates) {
				return false
			}
		}
	}
	return true
}

func (ss *assignSession) wanted(live liveAud, o applyOptions) wantAud {
	want := wantAud{Everyone: o.everyone, Dates: o.dates}
	if o.mode == "add" {
		if !o.explicit {
			want.Everyone = live.Everyone
		}
		want.Rows = append(want.Rows, live.Rows...)
	}
	have := want.keys()
	for _, g := range o.groups {
		row := audRow{GroupID: g.TagID, Title: g.Name}
		if !have[row.key()] {
			want.Rows = append(want.Rows, row)
			have[row.key()] = true
		}
	}
	return want
}

// applyOne reconciles one surface with the requested audience.
func (ss *assignSession) applyOne(ctx context.Context, surf assignSurface, o applyOptions) AssignToResult {
	res := AssignToResult{Target: surf.Name(), Kind: surf.Kind()}
	live, err := surf.Read(ctx)
	if err != nil {
		res.Status, res.Message = "held", err.Error()
		return res
	}
	res.Before = live.describe(ss.cat)
	for _, r := range live.Rows {
		if r.Unknown && o.mode == "add" {
			res.Status, res.Message = "held", "an existing audience row has a shape this Engine cannot preserve; use replace mode or change it in Canvas"
			return res
		}
	}
	want := ss.wanted(live, o)
	if _, isModule := surf.(*moduleSurface); isModule && want.Everyone {
		if len(o.groups) > 0 {
			res.Notes = append(res.Notes, "module is visible to everyone, so the tags were not added; set everyone: false")
		}
		want.Rows = nil
	}
	after := liveAud{Everyone: want.Everyone, Rows: want.Rows}
	res.After = after.describe(ss.cat)
	if sameAudience(live, want, surf) {
		res.Status = "unchanged"
		return res
	}
	if sc, ok := surf.(submissionCounter); ok && !o.allowRemove && removesAccess(live, want) {
		n, err := sc.SubmittedCount(ctx)
		if err != nil {
			res.Status, res.Message = "held", "could not check for submitted work before removing access: "+err.Error()
			return res
		}
		if n > 0 {
			res.Status = "held"
			res.Message = fmt.Sprintf("%d student(s) already submitted work and this change removes access; nothing changed. Set allow_remove_with_submissions: true to proceed", n)
			return res
		}
	}
	if o.preview {
		res.Status = "preview"
		return res
	}
	if err := surf.Apply(ctx, live, want); err != nil {
		res.Status, res.Message = "held", "write not confirmed: "+canvasDetail(err)+"; re-read Canvas before retrying"
		if again, rerr := surf.Read(ctx); rerr == nil && sameAudience(again, want, surf) {
			res.Status, res.Message = "succeeded", "Canvas shows the requested audience despite the error"
		}
		return res
	}
	again, err := surf.Read(ctx)
	if err != nil || !sameAudience(again, want, surf) {
		res.Status = "held"
		res.Message = "Canvas accepted the write but the read-back differs from the request; check it in Canvas"
		if err == nil {
			res.Notes = append(res.Notes, "Canvas now shows: "+again.describe(ss.cat))
		}
		return res
	}
	res.Status = "succeeded"
	return res
}

func removesAccess(live liveAud, want wantAud) bool {
	if live.Everyone && !want.Everyone {
		return true
	}
	wk := want.keys()
	for _, r := range live.Rows {
		if !wk[r.key()] {
			return true
		}
	}
	return false
}

func (ss *assignSession) resolveTags(refs []string) ([]resolvedTag, error) {
	var out []resolvedTag
	for _, ref := range refs {
		t, err := ss.cat.Resolve(ref)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

// otherModuleWarnings lists items of a module that also sit in another module
// whose audience is broader and that have no audience of their own, so they
// would stay visible through the other module.
func (ss *assignSession) otherModuleWarnings(ctx context.Context, target liveModule, groups []resolvedTag, items map[int64]assignSurface) []string {
	var notes []string
	want := map[string]bool{}
	for _, g := range groups {
		want["g"+strconv.FormatInt(g.TagID, 10)] = true
	}
	for _, it := range target.Items {
		for _, other := range ss.modules {
			if other.ID == target.ID {
				continue
			}
			found := false
			for _, o := range other.Items {
				if o.key() == it.key() && it.Type != "SubHeader" {
					found = true
				}
			}
			if !found {
				continue
			}
			aud, err := ss.moduleAudience(ctx, other)
			if err != nil {
				continue
			}
			broader := aud.Everyone
			for _, r := range aud.Rows {
				if !want[r.key()] {
					broader = true
				}
			}
			if !broader {
				continue
			}
			if surf, ok := items[it.ID]; ok {
				if own, err := surf.Read(ctx); err == nil && own.hasOwn() {
					continue
				}
			}
			notes = append(notes, fmt.Sprintf("%q is also in module %q (%s), so it stays visible there", it.Title, other.Name, aud.describe(ss.cat)))
		}
	}
	return notes
}

func (s *Service) newAssignSession(ctx context.Context, nickname string) (*assignSession, error) {
	course, err := s.Course(nickname)
	if err != nil {
		return nil, err
	}
	routed, err := s.ForCourse(ctx, course)
	if err != nil {
		return nil, err
	}
	cat, err := routed.readTagCatalog(ctx, course.ID, course.Nickname)
	if err != nil {
		return nil, err
	}
	return &assignSession{s: routed, course: course, cat: cat}, nil
}

// PreviewAssignTo resolves every name read-only and returns the audience each
// target would change from and to.
func (s *Service) PreviewAssignTo(ctx context.Context, path string) (AssignToReport, error) {
	return s.runAssignTo(ctx, path, true)
}

// PushAssignTo applies an assign-to document.
func (s *Service) PushAssignTo(ctx context.Context, path string) (AssignToReport, error) {
	return s.runAssignTo(ctx, path, false)
}

func (s *Service) runAssignTo(ctx context.Context, path string, preview bool) (AssignToReport, error) {
	report := AssignToReport{Path: path}
	var doc domain.AssignToDocument
	body, err := workspace.ReadDocument(path, &doc)
	if err != nil {
		return report, err
	}
	if err = doc.Validate(); err != nil {
		return report, err
	}
	ss, err := s.newAssignSession(ctx, doc.Course)
	if err != nil {
		return report, err
	}
	for i, target := range doc.Targets {
		results := ss.runTarget(ctx, doc, target, preview)
		report.Results = append(report.Results, results...)
		if !preview {
			if out := summarizeOutcome(results); out.Status != "unchanged" || doc.Targets[i].Outcome.Status != "succeeded" {
				doc.Targets[i].Outcome = out
			}
			if err := workspace.WriteDocument(path, &doc, body); err != nil {
				return report, fmt.Errorf("Canvas results were applied but could not be saved to the document: %w", err)
			}
		}
	}
	report.Status = "completed"
	if preview {
		report.Status = "preview"
	}
	for _, r := range report.Results {
		if !r.ok() {
			report.Status = "partial"
		}
	}
	return report, nil
}

func summarizeOutcome(results []AssignToResult) domain.AssignToOutcome {
	out := domain.AssignToOutcome{Status: "unchanged"}
	for _, r := range results {
		if r.Status == "held" {
			out.Status = "held"
			if out.Message == "" {
				out.Message = r.Message
			}
		} else if r.Status == "succeeded" && out.Status != "held" {
			out.Status = "succeeded"
		}
		out.Notes = append(out.Notes, r.Notes...)
	}
	return out
}

func (ss *assignSession) runTarget(ctx context.Context, doc domain.AssignToDocument, t domain.AssignToTarget, preview bool) []AssignToResult {
	kind, ref, _ := t.KindRef()
	held := func(msg string) []AssignToResult {
		return []AssignToResult{{Target: ref.String(), Kind: kind, Status: "held", Message: msg}}
	}
	groups, err := ss.resolveTags(t.Tags)
	if err != nil {
		return held(err.Error())
	}
	everyone, explicit := doc.EveryoneFor(t)
	opts := applyOptions{mode: doc.EffectiveMode(), groups: groups, everyone: everyone, explicit: explicit, dates: t.Dates, allowRemove: doc.AllowRemoveWithSubmissions, preview: preview}
	if kind == "module" {
		return ss.runModule(ctx, t, ref, opts)
	}
	var surf assignSurface
	switch kind {
	case "page":
		surf, err = ss.findPage(ctx, ref)
	case "quiz", "assignment":
		surf, err = ss.findAssignment(ctx, ref, kind)
	case "discussion":
		surf, err = ss.findDiscussion(ctx, ref)
	}
	if err != nil {
		return held(err.Error())
	}
	return []AssignToResult{ss.applyOne(ctx, surf, opts)}
}

func (ss *assignSession) runModule(ctx context.Context, t domain.AssignToTarget, ref domain.Ref, opts applyOptions) []AssignToResult {
	mod, err := ss.findModule(ctx, ref)
	if err != nil {
		return []AssignToResult{{Target: ref.String(), Kind: "module", Status: "held", Message: err.Error()}}
	}
	res := ss.applyOne(ctx, &moduleSurface{s: ss.s, cid: ss.course.ID, id: mod.ID, name: mod.Name}, opts)
	delete(ss.modAud, mod.ID)
	results := []AssignToResult{res}
	if !res.ok() {
		return results
	}
	cascade := t.Cascade
	if cascade == "" {
		cascade = "none"
	}
	items := map[int64]assignSurface{}
	var ownRestricted []string
	for _, it := range mod.Items {
		surf, ok, err := ss.itemSurface(ctx, it)
		if err != nil {
			results = append(results, AssignToResult{Target: it.Title, Kind: strings.ToLower(it.Type), Status: "held", Message: "could not read this item's audience: " + err.Error()})
			continue
		}
		if !ok {
			continue
		}
		items[it.ID] = surf
		switch cascade {
		case "apply_to_items":
			itemOpts := opts
			results = append(results, ss.applyOne(ctx, surf, itemOpts))
		case "clear_item_overrides":
			itemOpts := opts
			itemOpts.mode, itemOpts.groups, itemOpts.everyone, itemOpts.explicit, itemOpts.dates = "replace", nil, true, true, nil
			if own, err := surf.Read(ctx); err == nil && own.hasOwn() {
				results = append(results, ss.applyOne(ctx, surf, itemOpts))
			}
		default:
			if own, err := surf.Read(ctx); err == nil && own.hasOwn() {
				ownRestricted = append(ownRestricted, fmt.Sprintf("%q has its own audience (%s) and does not follow the module", it.Title, own.describe(ss.cat)))
			}
		}
	}
	results[0].Notes = append(results[0].Notes, ownRestricted...)
	results[0].Notes = append(results[0].Notes, ss.otherModuleWarnings(ctx, mod, opts.groups, items)...)
	return results
}
