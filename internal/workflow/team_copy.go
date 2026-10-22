package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"canvasexpertse/internal/canvas"
	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

// Copy statuses. A leftover "pending" from a crashed run counts like "uncertain".
const (
	copyPending   = "pending"
	copySucceeded = "succeeded"
	copyFailed    = "failed"
	copyUncertain = "uncertain"
)

var errPollDeadline = errors.New("poll deadline reached")

// pollUntil calls check immediately and then after every(elapsed) until it
// reports done, returns an error, the deadline passes (errPollDeadline) or ctx
// ends (ctx.Err()).
func pollUntil(ctx context.Context, every func(elapsed time.Duration) time.Duration, deadline time.Duration, check func() (bool, error)) error {
	start := time.Now()
	for {
		done, err := check()
		if err != nil {
			return err
		}
		if done {
			return nil
		}
		elapsed := time.Since(start)
		if elapsed >= deadline {
			return errPollDeadline
		}
		wait := every(elapsed)
		if remaining := deadline - elapsed; wait > remaining {
			wait = remaining
		}
		t := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-t.C:
		}
	}
}

// copyTiming is the polling schedule; tests shorten it.
type copyTiming struct {
	every          func(elapsed time.Duration) time.Duration
	selectDeadline time.Duration
	doneDeadline   time.Duration
}

func (t copyTiming) withDefaults() copyTiming {
	if t.every == nil {
		t.every = func(elapsed time.Duration) time.Duration {
			if elapsed < 30*time.Second {
				return 2 * time.Second
			}
			return 5 * time.Second
		}
	}
	if t.selectDeadline == 0 {
		t.selectDeadline = 2 * time.Minute
	}
	if t.doneDeadline == 0 {
		t.doneDeadline = 15 * time.Minute
	}
	return t
}

// CopySource and CopyDestination make up the per-module copy record.
type CopySource struct {
	Nickname    string `json:"nickname"`
	CourseID    int64  `json:"course_id"`
	ModuleID    int64  `json:"module_id"`
	ModuleTitle string `json:"module_title"`
}

type CopyDestination struct {
	CourseID     int64    `json:"course_id"`
	Nickname     string   `json:"nickname"`
	Teammate     string   `json:"teammate"`
	Status       string   `json:"status"`
	MigrationID  int64    `json:"migration_id,omitempty"`
	StartedAt    string   `json:"started_at"`
	FinishedAt   string   `json:"finished_at,omitempty"`
	DestModuleID int64    `json:"dest_module_id,omitempty"`
	Issues       []string `json:"issues"`
	Message      string   `json:"message"`
}

type CopyRecord struct {
	Schema       int               `json:"schema"`
	Source       CopySource        `json:"source"`
	Destinations []CopyDestination `json:"destinations"`
}

// CopyOutcome is one line of a copy-module result.
type CopyOutcome struct {
	Course  string
	Teacher string
	Status  string
	Issues  int
	Message string
}

type copyStore struct {
	mu   sync.Mutex
	path string
	rec  CopyRecord
	warn func(error)
}

func (st *copyStore) find(courseID int64) *CopyDestination {
	for i := range st.rec.Destinations {
		if st.rec.Destinations[i].CourseID == courseID {
			return &st.rec.Destinations[i]
		}
	}
	return nil
}

// update applies fn to a destination's entry (adding it when new) and rewrites
// the whole record file.
func (st *copyStore) update(courseID int64, fn func(*CopyDestination)) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	d := st.find(courseID)
	if d == nil {
		st.rec.Destinations = append(st.rec.Destinations, CopyDestination{CourseID: courseID, Issues: []string{}})
		d = &st.rec.Destinations[len(st.rec.Destinations)-1]
	}
	fn(d)
	b, err := json.MarshalIndent(st.rec, "", "  ")
	if err != nil {
		return err
	}
	return workspace.WriteFile(st.path, append(b, '\n'), 0600)
}

func (st *copyStore) mustUpdate(courseID int64, fn func(*CopyDestination)) {
	if err := st.update(courseID, fn); err != nil && st.warn != nil {
		st.warn(err)
	}
}

type copyItem struct {
	Title string `json:"title"`
}

// CopyModule copies one module of the source course, matched by exact title,
// once into each destination course. It never copies the same module into the
// same course twice.
func (s *Service) CopyModule(ctx context.Context, source, moduleTitle string, destinations []string) ([]CopyOutcome, error) {
	srcCourse, err := s.Course(source)
	if err != nil {
		return nil, err
	}
	var dests []domain.Course
	seen := map[int64]bool{srcCourse.ID: true}
	for _, name := range destinations {
		c, err := s.Course(name)
		if err != nil {
			return nil, err
		}
		if c.ID == srcCourse.ID {
			return nil, fmt.Errorf("%q is the source; choose other courses to copy into", srcCourse.Nickname)
		}
		if seen[c.ID] {
			continue
		}
		seen[c.ID] = true
		dests = append(dests, c)
	}
	if len(dests) == 0 {
		return nil, errors.New("name at least one course to copy into")
	}

	dir := filepath.Join(s.Root, "courses", "_team-copies")
	if err = os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	lock, err := os.OpenFile(filepath.Join(dir, ".copy.lock"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, errors.New("Another copy is running; try again when it finishes. If no copy is running (CE-SE was closed mid-copy), delete courses/_team-copies/.copy.lock and check each course's Import Course Content before copying again.")
	}
	defer os.Remove(lock.Name())
	defer lock.Close()

	src, err := s.ForCourse(ctx, srcCourse)
	if err != nil {
		return nil, err
	}
	modules, err := Modules(ctx, src.Client, srcCourse.ID)
	if err != nil {
		return nil, err
	}
	var matches []Module
	for _, m := range modules {
		if m.Name == moduleTitle {
			matches = append(matches, m)
		}
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("No module titled %q in %s.", moduleTitle, srcCourse.Nickname)
	case 1:
	default:
		return nil, fmt.Errorf("%s has %d modules titled %q; rename one first.", srcCourse.Nickname, len(matches), moduleTitle)
	}
	module := matches[0]
	var items []copyItem
	if err = src.Client.List(ctx, fmt.Sprintf("/api/v1/courses/%d/modules/%d/items?per_page=100", srcCourse.ID, module.ID), &items); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("%q in %s has no items yet.", moduleTitle, srcCourse.Nickname)
	}
	for _, it := range items {
		if domain.HasConsentTag(it.Title) {
			return nil, fmt.Errorf("Remove (CE) from %q in %s first. Each teacher adds (CE) in their own course.", it.Title, srcCourse.Nickname)
		}
	}

	recPath := filepath.Join(dir, fmt.Sprintf("course-%d-module-%d.json", srcCourse.ID, module.ID))
	store := &copyStore{path: recPath, rec: CopyRecord{Schema: 1}, warn: func(e error) {
		if s.Status != nil {
			fmt.Fprintln(s.Status, "Note: the copy record couldn't be updated:", e)
		}
	}}
	if b, e := os.ReadFile(recPath); e == nil {
		if e = json.Unmarshal(b, &store.rec); e != nil {
			return nil, fmt.Errorf("the copy record %s is unreadable; fix or remove it before copying", filepath.Base(recPath))
		}
	} else if !os.IsNotExist(e) {
		return nil, e
	}
	store.rec.Schema = 1
	store.rec.Source = CopySource{Nickname: srcCourse.Nickname, CourseID: srcCourse.ID, ModuleID: module.ID, ModuleTitle: moduleTitle}

	type job struct {
		course  domain.Course
		scoped  *Service
		teacher string
		label   string
		idx     int
	}
	outcomes := make([]CopyOutcome, len(dests))
	var jobs []job
	for i, c := range dests {
		owner, owned, e := s.Owned(c)
		teacher, label := "You", ""
		if owned {
			teacher, label = owner.Name, owner.Label
		}
		outcomes[i] = CopyOutcome{Course: c.Nickname, Teacher: teacher}
		if e != nil {
			outcomes[i].Status, outcomes[i].Message = copyFailed, e.Error()
			continue
		}
		if prior := store.find(c.ID); prior != nil {
			date := ""
			if len(prior.StartedAt) >= 10 {
				date = prior.StartedAt[:10]
			}
			outcomes[i].Status = "refused"
			outcomes[i].Message = fmt.Sprintf("%q was already sent to %s on %s (%s). CE-SE copies a module only once. Finish it in Canvas with Import Course Content, or edit that course's copy.", moduleTitle, c.Nickname, date, prior.Status)
			continue
		}
		scoped, e := s.ForCourse(ctx, c)
		if e != nil {
			outcomes[i].Status, outcomes[i].Message = copyFailed, e.Error()
			continue
		}
		jobs = append(jobs, job{c, scoped, teacher, label, i})
	}

	timing := s.copyTiming.withDefaults()
	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup
	for _, j := range jobs {
		wg.Add(1)
		sem <- struct{}{}
		go func(j job) {
			defer wg.Done()
			defer func() { <-sem }()
			status, message, issues := j.scoped.copyOne(ctx, store, timing, srcCourse, module, moduleTitle, j.course, j.label)
			outcomes[j.idx].Status, outcomes[j.idx].Message, outcomes[j.idx].Issues = status, message, issues
		}(j)
	}
	wg.Wait()
	return outcomes, nil
}

// copyOne runs one destination. s is already routed for that course.
func (s *Service) copyOne(ctx context.Context, store *copyStore, timing copyTiming, srcCourse domain.Course, module Module, title string, dest domain.Course, label string) (string, string, int) {
	c := s.Client
	id := dest.ID
	base := fmt.Sprintf("/api/v1/courses/%d/content_migrations", id)
	var issues []string
	finish := func(status, message string, patch func(*CopyDestination)) (string, string, int) {
		store.mustUpdate(id, func(d *CopyDestination) {
			d.Status, d.Message = status, message
			d.Issues = append([]string{}, issues...)
			d.FinishedAt = time.Now().UTC().Format(time.RFC3339)
			if patch != nil {
				patch(d)
			}
		})
		return status, message, len(issues)
	}
	// A write that might have reached Canvas, or a wait that ended early, is
	// uncertain; a plain Canvas refusal is a known failure.
	writeOutcome := func(err error, what string) (string, string, int) {
		var ce *canvas.Error
		if errors.As(err, &ce) && !ce.Uncertain && ce.Kind != "canceled" {
			return finish(copyFailed, what+": "+err.Error(), nil)
		}
		return finish(copyUncertain, fmt.Sprintf("Canvas may or may not have %s; check Import Course Content in %s before doing anything else.", what, dest.Nickname), nil)
	}
	waitOutcome := func(err error, deadlineMessage string) (string, string, int) {
		if errors.Is(err, errPollDeadline) {
			return finish(copyUncertain, deadlineMessage, nil)
		}
		var fe *migrationFailed
		if errors.As(err, &fe) {
			return finish(copyFailed, fmt.Sprintf("Canvas reported the copy into %s failed. The unfinished import stays in Import Course Content.", dest.Nickname), nil)
		}
		return finish(copyUncertain, fmt.Sprintf("CE-SE couldn't keep checking the copy; check Import Course Content in %s before doing anything else.", dest.Nickname), nil)
	}

	// Pending is on disk before the migration is requested.
	if err := store.update(id, func(d *CopyDestination) {
		*d = CopyDestination{CourseID: id, Nickname: dest.Nickname, Teammate: label, Status: copyPending, StartedAt: time.Now().UTC().Format(time.RFC3339), Issues: []string{}}
	}); err != nil {
		return copyFailed, "The copy record couldn't be saved, so nothing was copied: " + err.Error(), 0
	}

	var created struct {
		ID int64 `json:"id"`
	}
	body := map[string]any{
		"migration_type":   "course_copy_importer",
		"settings":         map[string]string{"source_course_id": strconv.FormatInt(srcCourse.ID, 10)},
		"selective_import": true,
	}
	if err := c.WriteAccepted(ctx, "POST", base, body, &created); err != nil {
		return writeOutcome(err, "started the copy")
	}
	if created.ID == 0 {
		return finish(copyUncertain, fmt.Sprintf("Canvas accepted the copy but didn't return its ID; check Import Course Content in %s before doing anything else.", dest.Nickname), nil)
	}
	store.mustUpdate(id, func(d *CopyDestination) { d.MigrationID = created.ID })
	migration := fmt.Sprintf("%s/%d", base, created.ID)

	state := func() (string, error) {
		var m struct {
			WorkflowState string `json:"workflow_state"`
		}
		if err := c.Get(ctx, migration, &m); err != nil {
			return "", err
		}
		return m.WorkflowState, nil
	}
	err := pollUntil(ctx, timing.every, timing.selectDeadline, func() (bool, error) {
		st, err := state()
		if err != nil {
			return false, err
		}
		if st == "failed" {
			return false, &migrationFailed{}
		}
		return st == "waiting_for_select", nil
	})
	if err != nil {
		return waitOutcome(err, fmt.Sprintf("Canvas hadn't prepared the copy after 2 minutes; check Import Course Content in %s.", dest.Nickname))
	}

	var entries []selectiveEntry
	if err = c.List(ctx, migration+"/selective_data?type=context_modules", &entries); err != nil {
		return finish(copyUncertain, fmt.Sprintf("CE-SE couldn't read Canvas's copy list; check Import Course Content in %s before doing anything else.", dest.Nickname), nil)
	}
	var found []selectiveEntry
	for _, e := range entries {
		if e.Title == title {
			found = append(found, e)
		}
	}
	if len(found) != 1 {
		return finish(copyFailed, fmt.Sprintf("Canvas's copy list didn't show exactly one %q; finish it by hand in %s. The unfinished copy stays in Import Course Content.", title, dest.Nickname), nil)
	}
	selection := map[string]any{}
	for _, p := range found[0].properties() {
		if !addSelection(selection, p) {
			return finish(copyFailed, fmt.Sprintf("Canvas's copy list had an unexpected shape; finish it by hand in %s. The unfinished copy stays in Import Course Content.", dest.Nickname), nil)
		}
	}
	if err = c.WriteAccepted(ctx, "PUT", migration, map[string]any{"copy": selection}, nil); err != nil {
		return writeOutcome(err, "applied the selection")
	}

	err = pollUntil(ctx, timing.every, timing.doneDeadline, func() (bool, error) {
		st, err := state()
		if err != nil {
			return false, err
		}
		if st == "failed" {
			return false, &migrationFailed{}
		}
		return st == "completed", nil
	})
	if err != nil {
		return waitOutcome(err, fmt.Sprintf("The copy was still running when CE-SE stopped waiting; check Import Course Content in %s before doing anything else.", dest.Nickname))
	}

	var notes []string
	var raw []struct {
		Description string `json:"description"`
	}
	if err = c.List(ctx, migration+"/migration_issues", &raw); err != nil {
		notes = append(notes, "Canvas's warnings couldn't be read.")
	}
	for _, r := range raw {
		if d := strings.TrimSpace(r.Description); d != "" {
			if rs := []rune(d); len(rs) > 200 {
				d = string(rs[:200])
			}
			issues = append(issues, d)
		}
	}
	var destModule int64
	if mods, err := Modules(ctx, c, id); err != nil {
		notes = append(notes, "The copied module couldn't be located.")
	} else {
		var ids []int64
		for _, m := range mods {
			if m.Name == title {
				ids = append(ids, m.ID)
			}
		}
		if len(ids) == 1 {
			destModule = ids[0]
		} else {
			notes = append(notes, fmt.Sprintf("%s now has %d modules titled %q.", dest.Nickname, len(ids), title))
		}
	}
	return finish(copySucceeded, strings.Join(notes, " "), func(d *CopyDestination) { d.DestModuleID = destModule })
}

type migrationFailed struct{}

func (*migrationFailed) Error() string { return "the migration failed" }

// selectiveEntry is one item of Canvas's selective_data list.
type selectiveEntry struct {
	Title    string           `json:"title"`
	Property string           `json:"property"`
	SubItems []selectiveEntry `json:"sub_items"`
}

// properties returns the entry's property and every sub-item property.
func (e selectiveEntry) properties() []string {
	var out []string
	if e.Property != "" {
		out = append(out, e.Property)
	}
	for _, s := range e.SubItems {
		out = append(out, s.properties()...)
	}
	return out
}

// addSelection turns "copy[context_modules][id_x]" into nested JSON with value
// "1" under the "copy" key, merged into root. It reports false for a shape it
// doesn't understand.
func addSelection(root map[string]any, property string) bool {
	if !strings.HasPrefix(property, "copy[") || !strings.HasSuffix(property, "]") {
		return false
	}
	parts := strings.Split(strings.TrimSuffix(strings.TrimPrefix(property, "copy["), "]"), "][")
	if len(parts) == 0 {
		return false
	}
	for _, p := range parts {
		if p == "" || strings.ContainsAny(p, "[]") {
			return false
		}
	}
	node := root
	for _, p := range parts[:len(parts)-1] {
		next, ok := node[p].(map[string]any)
		if !ok {
			if _, exists := node[p]; exists {
				return false
			}
			next = map[string]any{}
			node[p] = next
		}
		node = next
	}
	last := parts[len(parts)-1]
	if _, exists := node[last].(map[string]any); exists {
		return false
	}
	node[last] = "1"
	return true
}
