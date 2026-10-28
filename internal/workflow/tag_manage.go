package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"canvasexpertse/internal/canvas"
	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

// bulkUserTagIDs is the observed public mapping. Missing/extra identities are
// unknown evidence, never an empty membership. Bound requests to 100 users.
func bulkUserTagIDs(ctx context.Context, c *canvas.Client, courseID int64, ids []int64) (map[int64][]int64, error) {
	out := map[int64][]int64{}
	for start := 0; start < len(ids); start += 100 {
		end := start + 100
		if end > len(ids) {
			end = len(ids)
		}
		q := url.Values{}
		wanted := map[int64]bool{}
		for _, id := range ids[start:end] {
			if id <= 0 {
				return nil, fmt.Errorf("invalid roster identity")
			}
			q.Add("user_ids[]", strconv.FormatInt(id, 10))
			wanted[id] = true
		}
		var response map[string][]int64
		if e := c.Get(ctx, fmt.Sprintf("/api/v1/courses/%d/bulk_user_tags?%s", courseID, q.Encode()), &response); e != nil {
			return nil, e
		}
		for key, groups := range response {
			id, e := strconv.ParseInt(key, 10, 64)
			if e != nil || !wanted[id] || groups == nil {
				return nil, fmt.Errorf("Canvas returned incomplete or unexpected tag membership identities")
			}
			out[id] = groups
			for _, group := range groups {
				if group <= 0 {
					return nil, fmt.Errorf("Canvas returned an invalid tag identity")
				}
			}
		}
		for id := range wanted {
			if _, ok := out[id]; !ok {
				return nil, fmt.Errorf("Canvas omitted a requested tag membership identity")
			}
		}
	}
	return out, nil
}

type tagRoster struct {
	byRef   map[string]workspace.Student
	byID    map[int64]workspace.Student
	active  map[int64]bool
	members map[int64][]string
	unknown map[int64]bool
}

func (s *Service) tagRoster(ctx context.Context, courseID int64, catalog TagCatalog) (tagRoster, error) {
	r := tagRoster{byRef: map[string]workspace.Student{}, byID: map[int64]workspace.Student{}, active: map[int64]bool{}, members: map[int64][]string{}, unknown: map[int64]bool{}}
	store, e := workspace.OpenStudents(s.Root)
	if e != nil {
		return r, e
	}
	for _, st := range store.Rows {
		if st.Instance == s.Client.Origin() {
			r.byID[st.UserID] = st
			r.byRef[st.Ref] = st
		}
	}
	enrollments, e := StudentEnrollments(ctx, s.Client, courseID)
	if e != nil {
		return r, e
	}
	ids := []int64{}
	seen := map[int64]bool{}
	for _, en := range enrollments {
		if en.Type != "StudentEnrollment" {
			continue
		}
		id := en.UserID
		if id == 0 {
			id = en.User.ID
		}
		if id <= 0 || en.User.ID > 0 && en.User.ID != id {
			return r, fmt.Errorf("course roster has an unconfirmed student identity")
		}
		if !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
		if en.State == "active" {
			r.active[id] = true
		}
	}
	memberships, e := bulkUserTagIDs(ctx, s.Client, courseID, ids)
	if e != nil {
		return r, e
	}
	for id, groups := range memberships {
		for _, group := range groups {
			st, known := r.byID[id]
			if !known || !r.active[id] {
				r.unknown[group] = true
			}
			if known {
				r.members[group] = append(r.members[group], st.Ref)
			}
		}
	}
	for _, group := range catalog.all() {
		sort.Strings(r.members[group.TagID])
		if len(r.members[group.TagID]) != group.MemberCount {
			r.unknown[group.TagID] = true
		}
	}
	return r, nil
}
func (r tagRoster) resolve(text string) (workspace.Student, error) {
	if st, ok := r.byRef[text]; ok {
		if !r.active[st.UserID] {
			return st, fmt.Errorf("student ref %s is not actively enrolled in this course", text)
		}
		return st, nil
	}
	matches := []workspace.Student{}
	for id, st := range r.byID {
		if r.active[id] && strings.EqualFold(strings.TrimSpace(st.Alias), strings.TrimSpace(text)) {
			matches = append(matches, st)
		}
	}
	if len(matches) != 1 {
		return workspace.Student{}, fmt.Errorf("student alias/ref %q is missing or ambiguous in the active saved roster", text)
	}
	return matches[0], nil
}

type tagAccess struct {
	uses          map[int64][]domain.TagUsage
	unknown       bool
	unknownGroups map[int64]bool
	assignments   map[int64]Assignment
}

// Inspect complete direct audiences and complete module item collections. Any
// unreadable coverage holds removals, while additions/structure can continue.
func (s *Service) tagAccess(ctx context.Context, courseID int64) (access tagAccess) {
	access.uses = map[int64][]domain.TagUsage{}
	access.unknownGroups = map[int64]bool{}
	access.assignments = map[int64]Assignment{}
	add := func(a liveAud, use domain.TagUsage) {
		for _, row := range a.Rows {
			if row.Unknown {
				access.unknown = true
			}
			if row.GroupID > 0 {
				access.uses[row.GroupID] = append(access.uses[row.GroupID], use)
			}
		}
	}
	assignments, e := Assignments(ctx, s.Client, courseID)
	if e != nil {
		access.unknown = true
		return access
	}
	quizAssignments := map[int64]int64{}
	for _, a := range assignments {
		if a.ID <= 0 {
			access.unknown = true
			continue
		}
		access.assignments[a.ID] = a
		if a.QuizID != nil {
			quizAssignments[*a.QuizID] = a.ID
		}
		var rows []AssignmentOverrideLive
		if e = s.Client.List(ctx, fmt.Sprintf("/api/v1/courses/%d/assignments/%d/overrides?per_page=100", courseID, a.ID), &rows); e != nil {
			access.unknown = true
			continue
		}
		aud := liveAud{}
		for _, o := range rows {
			aud.Rows = append(aud.Rows, audRow{GroupID: o.GroupID, Unknown: o.GroupID == 0 && o.CourseID == 0 && o.CourseSectionID == 0 && len(o.StudentIDs) == 0})
		}
		add(aud, domain.TagUsage{Kind: "assignment", ID: a.ID, Title: a.Name, AssignmentID: a.ID})
	}
	var pages []struct {
		ID    int64  `json:"page_id"`
		Title string `json:"title"`
	}
	if e = s.Client.List(ctx, fmt.Sprintf("/api/v1/courses/%d/pages?per_page=100", courseID), &pages); e != nil {
		access.unknown = true
	}
	for _, p := range pages {
		surf := dateDetailsSurface{s: s, path: pageDateDetailsPath(courseID, p.ID), name: p.Title, kind: "page"}
		a, e := surf.Read(ctx)
		if e != nil {
			access.unknown = true
			continue
		}
		add(a, domain.TagUsage{Kind: "page", ID: p.ID, Title: p.Title})
	}
	var topics []NativeDiscussion
	if e = s.Client.List(ctx, fmt.Sprintf("/api/v1/courses/%d/discussion_topics?per_page=100", courseID), &topics); e != nil {
		access.unknown = true
	}
	topicAssignments := map[int64]int64{}
	for _, t := range topics {
		topicAssignments[t.ID] = t.AssignmentID
		if t.AssignmentID > 0 {
			continue
		}
		surf := dateDetailsSurface{s: s, path: fmt.Sprintf("/api/v1/courses/%d/discussion_topics/%d/date_details", courseID, t.ID), name: t.Title, kind: "discussion"}
		a, e := surf.Read(ctx)
		if e != nil {
			access.unknown = true
			continue
		}
		add(a, domain.TagUsage{Kind: "discussion", ID: t.ID, Title: t.Title})
	}
	var modules []liveModule
	if e = s.Client.List(ctx, fmt.Sprintf("/api/v1/courses/%d/modules?per_page=100", courseID), &modules); e != nil {
		access.unknown = true
	}
	for _, m := range modules {
		aud, e := (&moduleSurface{s: s, cid: courseID, id: m.ID, name: m.Name}).Read(ctx)
		if e != nil {
			access.unknown = true
			continue
		}
		add(aud, domain.TagUsage{Kind: "module", ID: m.ID, Title: m.Name})
		unknownModuleWork := func() {
			for _, row := range aud.Rows {
				if row.GroupID > 0 {
					access.unknownGroups[row.GroupID] = true
				}
			}
		}
		var items []liveModuleItem
		if e = s.Client.List(ctx, fmt.Sprintf("/api/v1/courses/%d/modules/%d/items?per_page=100", courseID, m.ID), &items); e != nil {
			access.unknown = true
			continue
		}
		for _, it := range items {
			assignmentID := int64(0)
			switch it.Type {
			case "Assignment":
				assignmentID = it.ContentID
			case "Quiz":
				assignmentID = quizAssignments[it.ContentID]
				if assignmentID == 0 {
					unknownModuleWork()
				}
			case "Discussion":
				assignmentID = topicAssignments[it.ContentID]
			case "ExternalTool":
				unknownModuleWork()
			}
			if assignmentID > 0 {
				if _, ok := access.assignments[assignmentID]; !ok {
					unknownModuleWork()
				}
				add(aud, domain.TagUsage{Kind: "module assignment", ID: it.ID, Title: it.Title, AssignmentID: assignmentID})
			}
		}
	}
	for id, uses := range access.uses {
		sort.Slice(uses, func(i, j int) bool {
			if uses[i].Kind != uses[j].Kind {
				return uses[i].Kind < uses[j].Kind
			}
			return uses[i].ID < uses[j].ID
		})
		access.uses[id] = uses
	}
	return access
}

type tagPlanRow struct {
	ID                         int64
	Name, Final                string
	Delete, Create, Rename     bool
	Before, Desired            []string
	Membership                 bool
	Hold                       string
	Usage                      []domain.TagUsage
	Unknown                    bool
	Exists, MembershipConflict bool
	ObservedName               string
}
type tagPlan struct {
	set                    TagSetInfo
	newSet                 bool
	rows                   []*tagPlanRow
	create, update, remove []map[string]any
	category               map[string]any
	roster                 tagRoster
	access                 tagAccess
	structureHold          string
}
type TagsRowReport struct {
	TagID                 int64
	Name                  string
	Current, Desired      int
	Added, Removed, Usage []string
	Status, Message       string
}
type TagsReport struct {
	Path, Status, Message string
	SetID                 int64
	Structure             domain.ActionOutcome
	Rows                  []TagsRowReport
	Members               []TagMemberResult
}
type TagMemberResult struct {
	domain.TagMemberOutcome
	Alias string
}

func findTagSet(catalog TagCatalog, request domain.TagSetChange) (TagSetInfo, bool, error) {
	matches := []TagSetInfo{}
	for _, set := range catalog.Sets {
		if request.ID > 0 && request.ID == set.TagSetID || request.ID == 0 && tagKey(set.Name) == tagKey(request.Name) {
			matches = append(matches, set)
		}
	}
	if len(matches) > 1 {
		return TagSetInfo{}, false, fmt.Errorf("tag set name is ambiguous; supply its exact ID")
	}
	if len(matches) == 1 {
		return matches[0], false, nil
	}
	if request.ID > 0 {
		return TagSetInfo{}, false, fmt.Errorf("set ID is not a verified differentiation tag set in this course")
	}
	return TagSetInfo{Name: request.Name}, true, nil
}
func tagByName(set TagSetInfo, name string) (TagInfo, bool, error) {
	hits := []TagInfo{}
	for _, tag := range set.Tags {
		if tagKey(tag.Name) == tagKey(name) || strconv.FormatInt(tag.TagID, 10) == name {
			hits = append(hits, tag)
		}
	}
	if len(hits) > 1 {
		return TagInfo{}, false, fmt.Errorf("tag %q is ambiguous; supply its exact ID", name)
	}
	if len(hits) == 1 {
		return hits[0], true, nil
	}
	return TagInfo{}, false, nil
}
func observedTag(d *domain.TagsDocument, id int64) *domain.TagObserved {
	if d.Observed != nil {
		for i := range d.Observed.Tags {
			if d.Observed.Tags[i].ID == id {
				return &d.Observed.Tags[i]
			}
		}
	}
	return nil
}
func equalRefs(a, b []string) bool {
	return len(a) == len(b) && strings.Join(a, "\x00") == strings.Join(b, "\x00")
}
func (s *Service) prepareTags(ctx context.Context, course domain.Course, d *domain.TagsDocument) (p tagPlan, err error) {
	catalog, err := s.readTagCatalog(ctx, course.ID, course.Nickname)
	if err != nil {
		return p, err
	}
	p.set, p.newSet, err = findTagSet(catalog, d.Set)
	if err != nil {
		return p, err
	}
	p.roster, err = s.tagRoster(ctx, course.ID, catalog)
	if err != nil {
		return p, err
	}
	if !p.newSet {
		var live canvasGroupCategory
		if e := s.Client.Get(ctx, fmt.Sprintf("/api/v1/group_categories/%d", p.set.TagSetID), &live); e != nil || live.ID != p.set.TagSetID || live.CourseID != course.ID || live.ContextType != "Course" || live.NonCollaborative != nil && !*live.NonCollaborative {
			return p, fmt.Errorf("set is not a confirmed differentiation tag set in this course")
		}
	}
	p.category = map[string]any{"name": d.Set.Name}
	if !p.newSet {
		p.category = map[string]any{"id": p.set.TagSetID}
	}
	if d.Set.RenameTo != "" && p.set.Name != d.Set.RenameTo {
		if d.Observed != nil && d.Observed.SetName != p.set.Name {
			p.structureHold = "Tag set name changed; preview again."
		}
		p.category["name"] = d.Set.RenameTo
	}
	finalRows := map[string]*tagPlanRow{}
	usedIDs := map[int64]bool{}
	for _, change := range d.Tags {
		final := change.Name
		if change.RenameTo != "" {
			final = change.RenameTo
		}
		row := &tagPlanRow{Name: change.Name, Final: final, Delete: change.Delete, Desired: []string{}}
		var current TagInfo
		found := false
		if change.ID > 0 {
			for _, tag := range p.set.Tags {
				if tag.TagID == change.ID {
					current, found = tag, true
				}
			}
		} else {
			current, found, err = tagByName(p.set, change.Name)
			if err != nil {
				return p, err
			}
			if !found && change.RenameTo != "" {
				current, found, err = tagByName(p.set, final)
				if err != nil {
					return p, err
				}
			}
		}
		if found {
			row.Exists = true
			row.ObservedName = current.Name
			row.ID = current.TagID
			row.Before = append([]string{}, p.roster.members[row.ID]...)
			row.Unknown = p.roster.unknown[row.ID]
			row.Desired = append([]string{}, row.Before...)
			usedIDs[row.ID] = true
			if change.RenameTo != "" && current.Name != final {
				row.Rename = true
				if before := observedTag(d, row.ID); before != nil && before.Name != current.Name {
					p.structureHold = "A tag name changed; preview again."
					row.Hold = p.structureHold
					row.MembershipConflict = true
				}
			} else if change.ID > 0 && change.RenameTo == "" && tagKey(current.Name) != tagKey(change.Name) {
				p.structureHold = "A pinned tag name changed; preview again."
				row.Hold = p.structureHold
				row.MembershipConflict = true
			}
		} else if change.Delete {
			row.ID = change.ID
		} else if change.ID > 0 {
			return p, fmt.Errorf("pinned tag is missing; preserve this draft and preview again")
		} else {
			row.Create = true
		}
		if row.Delete {
			row.Desired = []string{}
		}
		if other := finalRows[tagKey(final)]; other != nil {
			return p, fmt.Errorf("duplicate final tag name %q", final)
		}
		finalRows[tagKey(final)] = row
		p.rows = append(p.rows, row)
	}
	// Include only membership-requested existing tags that have no structure row.
	if d.Members != nil {
		for name, refs := range d.Members.Tags {
			row := finalRows[tagKey(name)]
			if row == nil {
				current, found, e := tagByName(p.set, name)
				if e != nil {
					return p, e
				}
				if !found {
					return p, fmt.Errorf("membership tag %q does not resolve after requested renames", name)
				}
				if usedIDs[current.TagID] {
					return p, fmt.Errorf("membership must use the tag's final name")
				}
				row = &tagPlanRow{ID: current.TagID, Name: current.Name, ObservedName: current.Name, Exists: true, Final: current.Name, Before: append([]string{}, p.roster.members[current.TagID]...), Unknown: p.roster.unknown[current.TagID]}
				p.rows = append(p.rows, row)
				finalRows[tagKey(row.Final)] = row
				usedIDs[row.ID] = true
			}
			if row.Delete {
				return p, fmt.Errorf("membership requested for a deleted tag")
			}
			if row.Membership {
				return p, fmt.Errorf("duplicate membership target")
			}
			row.Membership = true
			row.Desired = []string{}
			seen := map[string]bool{}
			for _, text := range refs {
				st, e := p.roster.resolve(text)
				if e != nil {
					return p, e
				}
				if seen[st.Ref] {
					return p, fmt.Errorf("duplicate student identity in tag %q", name)
				}
				seen[st.Ref] = true
				row.Desired = append(row.Desired, st.Ref)
			}
			if d.Members.Mode == "add" {
				for _, ref := range row.Before {
					if !seen[ref] {
						row.Desired = append(row.Desired, ref)
					}
				}
			}
			sort.Strings(row.Desired)
			if before := observedTag(d, row.ID); before != nil && d.Members.Mode != "add" && !equalRefs(before.Members, row.Before) && !equalRefs(row.Before, row.Desired) {
				row.Hold = "Membership changed in Canvas; preview again before replacing it."
				row.MembershipConflict = true
			}
		}
	}
	// New/final names cannot collide with unlisted existing tags.
	for _, row := range p.rows {
		if row.Delete || !row.Create && !row.Rename {
			continue
		}
		for _, current := range p.set.Tags {
			if current.TagID != row.ID && tagKey(current.Name) == tagKey(row.Final) {
				return p, fmt.Errorf("final tag name %q already belongs to another tag", row.Final)
			}
		}
	}
	if len(p.set.Tags) > 0 {
		p.access = s.tagAccess(ctx, course.ID)
	} else {
		p.access = tagAccess{uses: map[int64][]domain.TagUsage{}, assignments: map[int64]Assignment{}}
	}
	for _, row := range p.rows {
		row.Usage = p.access.uses[row.ID]
		if before := observedTag(d, row.ID); before != nil && row.Membership && !reflect.DeepEqual(before.Usage, row.Usage) {
			row.Hold = "Content using this tag changed; preview again before changing membership."
			row.MembershipConflict = true
		}
		removing := difference(row.Before, row.Desired)
		if row.Delete && row.Exists {
			removing = row.Before
		}
		if len(removing) > 0 || row.Delete && row.Exists {
			if row.Unknown || p.access.unknown || p.access.unknownGroups[row.ID] {
				row.Hold = "Membership or content usage could not be read completely; removal held."
			}
			if before := observedTag(d, row.ID); before != nil && !reflect.DeepEqual(before.Usage, row.Usage) {
				row.Hold = "Content using this tag changed; preview again before removal."
			}
			if row.Hold == "" {
				for _, ref := range removing {
					if reason := s.tagRemovalHold(ctx, course.ID, ref, row.Usage, p.access.assignments, d.AllowRemoveWithSubmissions); reason != "" {
						row.Hold = reason
						break
					}
				}
			}
		}
		if row.Delete && row.Exists {
			if row.Hold != "" {
				p.structureHold = row.Hold
			}
			p.remove = append(p.remove, map[string]any{"id": row.ID})
		} else if !row.Delete && row.Create {
			p.create = append(p.create, map[string]any{"name": row.Final})
		} else if !row.Delete && row.Rename {
			p.update = append(p.update, map[string]any{"id": row.ID, "name": row.Final})
		}
	}
	sort.SliceStable(p.rows, func(i, j int) bool { return p.rows[i].Final < p.rows[j].Final })
	return p, nil
}
func difference(a, b []string) []string {
	out := []string{}
	for _, v := range a {
		if !containsRef(b, v) {
			out = append(out, v)
		}
	}
	return out
}
func (s *Service) tagRemovalHold(ctx context.Context, courseID int64, ref string, uses []domain.TagUsage, assignments map[int64]Assignment, allow bool) string {
	seen := map[int64]bool{}
	for _, use := range uses {
		if use.AssignmentID <= 0 || seen[use.AssignmentID] {
			continue
		}
		seen[use.AssignmentID] = true
		a, ok := assignments[use.AssignmentID]
		if !ok {
			return "Assignment usage is unconfirmed; removal held."
		}
		protected, _, e := s.audienceRemovalProtection(ctx, courseID, a, ref)
		if e != nil {
			return "Submission/history evidence is unavailable; removal held."
		}
		if protected && !allow {
			return "Submitted or saved work keeps this student's access; use allow_remove_with_submissions only for the teacher's explicit request."
		}
	}
	return ""
}

type managedTag struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func (t *managedTag) UnmarshalJSON(b []byte) error {
	type plain managedTag
	var raw struct {
		*plain
		Group *plain `json:"group"`
	}
	raw.plain = (*plain)(t)
	if e := json.Unmarshal(b, &raw); e != nil {
		return e
	}
	if raw.Group != nil {
		*t = managedTag(*raw.Group)
	}
	return nil
}

type managedTagsResponse struct {
	Category struct {
		ID  int64 `json:"id"`
		Non bool  `json:"non_collaborative"`
	} `json:"group_category"`
	Created []managedTag `json:"created"`
	Updated []managedTag `json:"updated"`
}

func snapshotTags(p tagPlan) *domain.TagsObserved {
	b := &domain.TagsObserved{SetID: p.set.TagSetID, SetName: p.set.Name}
	for _, row := range p.rows {
		if row.ID > 0 {
			b.Tags = append(b.Tags, domain.TagObserved{ID: row.ID, Name: row.ObservedName, Members: append([]string{}, row.Before...), UnknownMembers: row.Unknown, Usage: row.Usage, UsageUnknown: p.access.unknown || p.access.unknownGroups[row.ID]})
		}
	}
	sort.Slice(b.Tags, func(i, j int) bool { return b.Tags[i].ID < b.Tags[j].ID })
	return b
}
func tagRowsReport(p tagPlan) []TagsRowReport {
	rows := []TagsRowReport{}
	for _, row := range p.rows {
		r := TagsRowReport{TagID: row.ID, Name: row.Final, Current: len(row.Before), Desired: len(row.Desired), Status: "ready", Message: row.Hold}
		if row.Hold != "" {
			r.Status = "held"
		}
		for _, ref := range difference(row.Desired, row.Before) {
			r.Added = append(r.Added, p.roster.byRef[ref].Alias)
		}
		for _, ref := range difference(row.Before, row.Desired) {
			r.Removed = append(r.Removed, p.roster.byRef[ref].Alias)
		}
		for _, use := range row.Usage {
			r.Usage = append(r.Usage, use.Kind+": "+use.Title)
		}
		rows = append(rows, r)
	}
	return rows
}

func (s *Service) SetTags(ctx context.Context, path string, preview bool) (report TagsReport, err error) {
	report.Path = path
	var d domain.TagsDocument
	body, err := workspace.ReadDocument(path, &d)
	if err != nil {
		return report, err
	}
	course, err := s.Course(d.Course)
	if err != nil {
		return report, err
	}
	s, err = s.ForCourse(ctx, course)
	if err != nil {
		return report, err
	}
	if d.Instance != "" && d.Instance != s.Client.Origin() || d.CourseID != 0 && d.CourseID != course.ID {
		return report, fmt.Errorf("tags document targets a different Canvas instance/course")
	}
	d.Instance, d.CourseID = s.Client.Origin(), course.ID
	p, err := s.prepareTags(ctx, course, &d)
	if err != nil {
		return report, err
	}
	report.Rows = tagRowsReport(p)
	report.SetID = p.set.TagSetID
	save := func() error { return workspace.WriteDocument(path, &d, body) }
	if preview {
		d.Observed = snapshotTags(p)
		if p.set.TagSetID > 0 {
			d.Set.ID = p.set.TagSetID
		}
		for i := range d.Tags {
			for _, row := range p.rows {
				if row.Name == d.Tags[i].Name && row.ID > 0 {
					d.Tags[i].ID = row.ID
				}
			}
		}
		report.Status = "preview"
		if p.structureHold != "" {
			report.Message = p.structureHold
		}
		return report, save()
	}
	if d.Observed == nil {
		d.Observed = snapshotTags(p)
	}
	if p.set.TagSetID > 0 {
		d.Set.ID = p.set.TagSetID
	}
	for i := range d.Tags {
		for _, row := range p.rows {
			if row.Exists && row.Name == d.Tags[i].Name {
				d.Tags[i].ID = row.ID
			}
		}
	}
	report.Status = "completed"
	if p.structureHold != "" {
		d.Structure = domain.ActionOutcome{Status: "held", Message: p.structureHold}
		report.Status = "partial"
	} else if len(p.create)+len(p.update)+len(p.remove) > 0 || p.newSet || p.category["name"] != nil {
		var response managedTagsResponse
		payload := map[string]any{"group_category": p.category, "operations": map[string]any{"create": p.create, "update": p.update, "delete": p.remove}}
		if e := s.Client.WriteAccepted(ctx, "POST", fmt.Sprintf("/api/v1/courses/%d/group_categories/bulk_manage_differentiation_tag", course.ID), payload, &response); e != nil {
			d.Structure = failedOutcome(e)
			report.Status = "partial"
			p.structureHold = "Structure write did not complete; only unaffected existing tags can continue."
		} else {
			d.Structure = domain.ActionOutcome{Status: "succeeded", Message: "Canvas accepted the complete structure batch."}
			if response.Category.ID > 0 && response.Category.Non {
				d.Set.ID = response.Category.ID
				report.SetID = d.Set.ID
			}
			changed := append(response.Created, response.Updated...)
			for _, tag := range changed {
				for _, row := range p.rows {
					if row.Final == tag.Name && tag.ID > 0 {
						row.ID = tag.ID
						row.Create, row.Rename = false, false
					}
				}
				for i := range d.Tags {
					final := d.Tags[i].Name
					if d.Tags[i].RenameTo != "" {
						final = d.Tags[i].RenameTo
					}
					if final == tag.Name {
						d.Tags[i].ID = tag.ID
					}
				}
			}
		}
	} else {
		d.Structure = domain.ActionOutcome{Status: "preserved", Message: "Requested structure is already present."}
	}
	if e := save(); e != nil {
		report.Structure = d.Structure
		return report, fmt.Errorf("structure outcome could not be saved; accepted Canvas changes stand: %w", e)
	}
	blocked := map[string]bool{}
	record := func(id int64, ref, op string, out domain.ActionOutcome) {
		for i := range d.MemberOutcomes {
			r := &d.MemberOutcomes[i]
			if r.TagID == id && r.StudentRef == ref && r.Operation == op {
				r.Outcome = out
				return
			}
		}
		d.MemberOutcomes = append(d.MemberOutcomes, domain.TagMemberOutcome{TagID: id, StudentRef: ref, Operation: op, Outcome: out})
	}
	for i := range d.MemberOutcomes {
		out := &d.MemberOutcomes[i]
		if definite(out.Outcome) {
			continue
		}
		for _, row := range p.rows {
			if row.ID != out.TagID || !row.Membership {
				continue
			}
			present, wanted := containsRef(row.Before, out.StudentRef), containsRef(row.Desired, out.StudentRef)
			if out.Operation == "add" && present && wanted || out.Operation == "remove" && !present && !wanted {
				out.Outcome = domain.ActionOutcome{Status: "preserved", Message: "Requested membership is already present in Canvas."}
			}
		}
	}
	// Add every destination first; a failed destination protects that student's
	// old membership throughout the remainder of this operation.
	for _, row := range p.rows {
		if !row.Membership || row.Delete {
			continue
		}
		adds := difference(row.Desired, row.Before)
		if len(adds) == 0 {
			continue
		}
		if row.MembershipConflict || row.ID <= 0 || p.structureHold != "" && (row.Create || row.Rename) {
			report.Status = "partial"
			for _, ref := range adds {
				blocked[ref] = true
				if row.ID > 0 {
					record(row.ID, ref, "add", domain.ActionOutcome{Status: "held", Message: "Tag structure or membership changed; destination add held."})
				}
			}
			for i := range report.Rows {
				if report.Rows[i].Name == row.Final {
					report.Rows[i].Status = "held"
					report.Rows[i].Message = "Tag structure or membership changed; destination add held."
				}
			}
			continue
		}
		var group canvasGroup
		if e := s.Client.Get(ctx, fmt.Sprintf("/api/v1/groups/%d", row.ID), &group); e != nil || group.ID != row.ID || group.CourseID != course.ID || group.ContextType != "Course" || d.Set.ID <= 0 || group.GroupCategoryID != d.Set.ID || !nonCollaborative(group.NonCollaborative) {
			report.Status = "partial"
			for _, ref := range adds {
				blocked[ref] = true
				record(row.ID, ref, "add", domain.ActionOutcome{Status: "held", Message: "Exact differentiation tag identity could not be confirmed."})
			}
			continue
		}
		ids := []int64{}
		for _, ref := range adds {
			ids = append(ids, p.roster.byRef[ref].UserID)
		}
		e := s.Client.WriteAccepted(ctx, "POST", fmt.Sprintf("/api/v1/groups/%d/memberships", row.ID), map[string]any{"members": ids}, nil)
		for _, ref := range adds {
			out := domain.ActionOutcome{Status: "succeeded"}
			if e != nil {
				out = failedOutcome(e)
				blocked[ref] = true
				report.Status = "partial"
			}
			record(row.ID, ref, "add", out)
		}
		if e := save(); e != nil {
			return report, e
		}
	}
	for _, row := range p.rows {
		if !row.Membership || row.Delete {
			continue
		}
		for _, ref := range difference(row.Before, row.Desired) {
			out := domain.ActionOutcome{Status: "held"}
			reason := row.Hold
			if blocked[ref] {
				reason = "Destination add failed or was held; old membership preserved."
			}
			if p.structureHold != "" && (row.Create || row.Rename) {
				reason = "Tag structure is unfinished; membership preserved."
			}
			if reason == "" {
				reason = s.tagRemovalHold(ctx, course.ID, ref, row.Usage, p.access.assignments, d.AllowRemoveWithSubmissions)
			}
			if reason != "" {
				out.Message = reason
				report.Status = "partial"
			} else {
				st := p.roster.byRef[ref]
				var group canvasGroup
				if e := s.Client.Get(ctx, fmt.Sprintf("/api/v1/groups/%d", row.ID), &group); e != nil || group.CourseID != course.ID || group.ID != row.ID || group.GroupCategoryID != d.Set.ID || !nonCollaborative(group.NonCollaborative) {
					out.Message = "Exact differentiation tag identity changed; removal held."
					report.Status = "partial"
					record(row.ID, ref, "remove", out)
					continue
				}
				current, e := bulkUserTagIDs(ctx, s.Client, course.ID, []int64{st.UserID})
				if e != nil {
					out.Message = "Current membership unavailable; removal held."
					report.Status = "partial"
				} else if !containsID(current[st.UserID], row.ID) {
					out.Status = "preserved"
				} else if e = s.Client.WriteAccepted(ctx, "DELETE", fmt.Sprintf("/api/v1/groups/%d/users/%d", row.ID, st.UserID), nil, nil); e != nil {
					out = failedOutcome(e)
					report.Status = "partial"
				} else {
					out.Status = "succeeded"
				}
			}
			record(row.ID, ref, "remove", out)
			if e := save(); e != nil {
				return report, e
			}
		}
	}
	report.Structure = d.Structure
	for _, out := range d.MemberOutcomes {
		report.Members = append(report.Members, TagMemberResult{TagMemberOutcome: out, Alias: p.roster.byRef[out.StudentRef].Alias})
	}
	return report, save()
}

func containsID(ids []int64, id int64) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}
