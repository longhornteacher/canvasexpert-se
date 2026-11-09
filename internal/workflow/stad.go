package workflow

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

// STADTeamsFile is context/stad/<course>/teams.md: rounds and rosters for
// classroom team points. A round's membership is fixed once it starts; a
// reshuffle appends a new round. Role labels and the reasons behind placements
// never go here (they belong in notes.md, which the Engine never reads).
type STADTeamsFile struct {
	Schema   int         `yaml:"schema"`
	GroupSet string      `yaml:"group_set"`
	Rounds   []STADRound `yaml:"rounds"`
}

type STADRound struct {
	Name   string     `yaml:"name"`
	Starts string     `yaml:"starts"` // school date, course timezone
	Teams  []STADTeam `yaml:"teams"`
}

type STADTeam struct {
	Name    string   `yaml:"name"`
	Members []string `yaml:"members"` // student_refs
	// Joined gives the date a mid-round joiner's points start counting.
	Joined map[string]string `yaml:"joined,omitempty"`
}

func stadTeamsPath(nickname string) string {
	return filepath.ToSlash(filepath.Join("context", "stad", nickname, "teams.md"))
}

// ReadSTADTeams reads and validates the teams file through the teacher-setup
// authority, so a second PC sees the same teams.
func ReadSTADTeams(root, nickname string) (STADTeamsFile, error) {
	var f STADTeamsFile
	if _, err := workspace.ReadTeacherDocument(root, stadTeamsPath(nickname), &f); err != nil {
		return f, fmt.Errorf("%s: %w", stadTeamsPath(nickname), err)
	}
	if err := f.Validate(); err != nil {
		return f, fmt.Errorf("%s: %w", stadTeamsPath(nickname), err)
	}
	return f, nil
}

func (f STADTeamsFile) Validate() error {
	if strings.TrimSpace(f.GroupSet) == "" {
		return fmt.Errorf("group_set (the Canvas group set name) is required")
	}
	if len(f.Rounds) == 0 {
		return fmt.Errorf("at least one round is required")
	}
	roundNames := map[string]bool{}
	prev := ""
	for i, r := range f.Rounds {
		if strings.TrimSpace(r.Name) == "" || roundNames[r.Name] {
			return fmt.Errorf("rounds[%d]: each round needs its own name", i)
		}
		roundNames[r.Name] = true
		if _, err := time.Parse("2006-01-02", r.Starts); err != nil {
			return fmt.Errorf("round %q: starts must be a date like 2026-11-16", r.Name)
		}
		if r.Starts <= prev {
			return fmt.Errorf("round %q: rounds must start in date order, each after the one before", r.Name)
		}
		prev = r.Starts
		if len(r.Teams) == 0 {
			return fmt.Errorf("round %q has no teams", r.Name)
		}
		teamNames, refs := map[string]bool{}, map[string]string{}
		for _, t := range r.Teams {
			if strings.TrimSpace(t.Name) == "" || teamNames[t.Name] {
				return fmt.Errorf("round %q: each team needs its own name", r.Name)
			}
			teamNames[t.Name] = true
			for _, ref := range t.Members {
				if other, dup := refs[ref]; dup {
					return fmt.Errorf("round %q: %s is on both %q and %q", r.Name, ref, other, t.Name)
				}
				refs[ref] = t.Name
			}
			for ref, d := range t.Joined {
				if refs[ref] != t.Name {
					return fmt.Errorf("round %q, team %q: joined names %s, who is not a member", r.Name, t.Name, ref)
				}
				if _, err := time.Parse("2006-01-02", d); err != nil {
					return fmt.Errorf("round %q, team %q: joined date for %s must look like 2026-11-16", r.Name, t.Name, ref)
				}
			}
		}
	}
	return nil
}

// CurrentRound is the latest round that has started on the given school date.
func (f STADTeamsFile) CurrentRound(today string) (STADRound, bool) {
	var found STADRound
	ok := false
	for _, r := range f.Rounds {
		if r.Starts <= today {
			found, ok = r, true
		}
	}
	return found, ok
}

// STADStudent is a student reference with its privacy alias.
type STADStudent struct {
	Ref   string `json:"student_ref"`
	Alias string `json:"alias"`
}

// STADTeamsReport compares the current round with the active enrollments.
type STADTeamsReport struct {
	Course      string        `json:"course"`
	Round       string        `json:"round"`
	Teams       int           `json:"teams"`
	UnknownRefs []string      `json:"unknown_refs,omitempty"`
	Unassigned  []STADStudent `json:"active_students_on_no_team,omitempty"`
}

// OK reports whether every team member is an active student.
func (r STADTeamsReport) OK() bool { return len(r.UnknownRefs) == 0 }

// STADTeams validates teams.md against the course's active students. It reads
// Canvas and writes nothing there.
func (s *Service) STADTeams(ctx context.Context, nickname string, now time.Time) (STADTeamsReport, error) {
	rep := STADTeamsReport{Course: nickname}
	course, err := s.Course(nickname)
	if err != nil {
		return rep, err
	}
	s, err = s.ForCourse(ctx, course)
	if err != nil {
		return rep, err
	}
	f, err := ReadSTADTeams(s.Root, nickname)
	if err != nil {
		return rep, err
	}
	round, ok := f.CurrentRound(courseToday(course.Timezone, now))
	if !ok {
		return rep, fmt.Errorf("no round has started yet; the first starts %s", f.Rounds[0].Starts)
	}
	active, err := s.activeStudents(ctx, course.ID)
	if err != nil {
		return rep, err
	}
	rep.Round, rep.Teams = round.Name, len(round.Teams)
	onTeam := map[string]bool{}
	for _, t := range round.Teams {
		for _, ref := range t.Members {
			onTeam[ref] = true
			if _, isActive := active[ref]; !isActive {
				rep.UnknownRefs = append(rep.UnknownRefs, ref)
			}
		}
	}
	for ref, st := range active {
		if !onTeam[ref] {
			rep.Unassigned = append(rep.Unassigned, STADStudent{Ref: ref, Alias: st.Alias})
		}
	}
	sort.Strings(rep.UnknownRefs)
	sort.Slice(rep.Unassigned, func(i, j int) bool { return rep.Unassigned[i].Alias < rep.Unassigned[j].Alias })
	return rep, nil
}

// courseToday is the school date in the course timezone.
func courseToday(zone string, now time.Time) string {
	if zone == "" {
		zone = "America/Chicago"
	}
	loc, err := time.LoadLocation(zone)
	if err != nil {
		loc = time.UTC
	}
	return now.In(loc).Format("2006-01-02")
}

// activeStudents maps student_ref to the saved student for every active
// StudentEnrollment, registering new students in the roster as other reads do.
func (s *Service) activeStudents(ctx context.Context, courseID int64) (map[string]workspace.Student, error) {
	byUser, err := s.activeStudentsByUser(ctx, courseID)
	if err != nil {
		return nil, err
	}
	out := map[string]workspace.Student{}
	for _, st := range byUser {
		out[st.Ref] = st
	}
	return out, nil
}

// activeStudentsByUser maps Canvas user ID to the saved student.
func (s *Service) activeStudentsByUser(ctx context.Context, courseID int64) (map[int64]workspace.Student, error) {
	enrollments, err := StudentEnrollments(ctx, s.Client, courseID)
	if err != nil {
		return nil, err
	}
	store, err := workspace.OpenStudents(s.Root)
	if err != nil {
		return nil, err
	}
	out := map[int64]workspace.Student{}
	for _, en := range enrollments {
		if en.State != "active" || en.Type != "StudentEnrollment" {
			continue
		}
		id := en.UserID
		if id == 0 {
			id = en.User.ID
		}
		if id <= 0 {
			continue
		}
		st, e := store.Upsert(s.Client.Origin(), id, en.User.Name, en.User.ShortName, en.User.SISID, en.User.Login)
		if e != nil {
			return nil, e
		}
		out[id] = st
	}
	return out, store.Save()
}

// STADPushReport says what push-teams changed in Canvas and what it left alone.
type STADPushReport struct {
	Status     string   `json:"status"` // completed, partial, held
	Round      string   `json:"round"`
	GroupSetID int64    `json:"group_set_id,omitempty"`
	SetCreated bool     `json:"group_set_created,omitempty"`
	Created    []string `json:"groups_created,omitempty"`
	Renamed    []string `json:"groups_renamed,omitempty"`
	Placed     int      `json:"students_placed"`
	// LeftAlone are Canvas groups with no team in this round, and students in
	// the set who are on no team. Nothing is deleted or removed.
	LeftAloneGroups   []string `json:"groups_left_alone,omitempty"`
	LeftAloneStudents []string `json:"students_left_in_place,omitempty"`
	Held              []string `json:"held,omitempty"`
	Message           string   `json:"message,omitempty"`
}

// STADPushTeams makes one Canvas group set match the current round: one group
// per team, each team's members in it. It adds and moves; it never deletes a
// group or removes a student. Reshuffles update the same set. A rerun after a
// partial failure finds the set and groups by name and finishes the rest.
func (s *Service) STADPushTeams(ctx context.Context, nickname string, now time.Time) (STADPushReport, error) {
	var rep STADPushReport
	check, err := s.STADTeams(ctx, nickname, now)
	if err != nil {
		return rep, err
	}
	rep.Round = check.Round
	if !check.OK() {
		rep.Status = "held"
		rep.Message = fmt.Sprintf("teams.md names students who are not active in this course: %s. Nothing was written to Canvas.", strings.Join(check.UnknownRefs, ", "))
		return rep, nil
	}
	course, err := s.Course(nickname)
	if err != nil {
		return rep, err
	}
	f, err := ReadSTADTeams(s.Root, nickname)
	if err != nil {
		return rep, err
	}
	round, _ := f.CurrentRound(courseToday(course.Timezone, now))
	stamp := now.UTC().Format("20060102T150405")
	dir := func(name string) (string, error) {
		return workspace.ConstrainPath(s.Root, filepath.Join("courses", nickname, "stad", stamp+"-"+name))
	}

	// 1. The group set, found by name or created.
	sets, err := s.ListGroupSets(ctx, nickname)
	if err != nil {
		return rep, err
	}
	for _, g := range sets {
		if g.Name == f.GroupSet {
			if rep.GroupSetID != 0 {
				rep.Status, rep.Message = "held", fmt.Sprintf("two group sets are named %q; rename one in Canvas so the teams have one home", f.GroupSet)
				return rep, nil
			}
			rep.GroupSetID = g.ID
		}
	}
	if rep.GroupSetID == 0 {
		doc := domain.GroupSetDocument{Schema: 1, Kind: "group-set", Operation: "create", Course: nickname, Name: f.GroupSet, SelfSignup: domain.OptionalString{Set: true, Value: "disabled"}}
		for _, t := range round.Teams {
			doc.Groups = append(doc.Groups, domain.GroupSetRow{Key: t.Name, Name: t.Name})
		}
		path, err := dir("set.md")
		if err != nil {
			return rep, err
		}
		if err = workspace.WriteDocument(path, doc, ""); err != nil {
			return rep, err
		}
		res, err := s.PushGroupSet(ctx, path)
		if err != nil {
			return rep, err
		}
		if res.Status != "completed" {
			rep.Status, rep.Message = "held", fmt.Sprintf("creating the group set stopped (%s): %s. Run it again to finish; it picks up what exists.", res.Status, res.Message)
			return rep, nil
		}
		rep.GroupSetID, rep.SetCreated = res.GroupCategoryID, true
		for _, g := range res.Groups {
			rep.Created = append(rep.Created, g.Key)
		}
	}

	// 2. Groups: match by name, then rename a leftover group to a leftover
	// team when they share members, and create what is still missing.
	roster, err := s.ReadGroupRoster(ctx, nickname, rep.GroupSetID)
	if err != nil {
		return rep, err
	}
	groupByName := map[string]domain.GroupSummary{}
	for _, g := range roster.Groups {
		groupByName[g.Name] = g
	}
	var unmatchedTeams []STADTeam
	usedGroup := map[int64]bool{}
	for _, t := range round.Teams {
		if g, ok := groupByName[t.Name]; ok {
			usedGroup[g.ID] = true
		} else {
			unmatchedTeams = append(unmatchedTeams, t)
		}
	}
	var rows []domain.GroupSetRow
	for _, t := range unmatchedTeams {
		bestID, best := int64(0), 0
		for _, g := range roster.Groups {
			if usedGroup[g.ID] {
				continue
			}
			overlap := 0
			for _, m := range g.Members {
				for _, ref := range t.Members {
					if m.StudentRef == ref {
						overlap++
					}
				}
			}
			if overlap > best {
				bestID, best = g.ID, overlap
			}
		}
		if bestID != 0 {
			usedGroup[bestID] = true
			rows = append(rows, domain.GroupSetRow{GroupID: bestID, Name: t.Name})
			for _, g := range roster.Groups {
				if g.ID == bestID {
					rep.Renamed = append(rep.Renamed, g.Name+" -> "+t.Name)
				}
			}
		} else {
			rows = append(rows, domain.GroupSetRow{Key: t.Name, Name: t.Name})
			if !rep.SetCreated {
				rep.Created = append(rep.Created, t.Name)
			}
		}
	}
	if len(rows) > 0 {
		pulled, err := s.PullGroupSet(ctx, nickname, rep.GroupSetID)
		if err != nil {
			return rep, err
		}
		var doc domain.GroupSetDocument
		if _, err = workspace.ReadDocument(pulled, &doc); err != nil {
			return rep, err
		}
		observed := map[int64]*domain.GroupSnapshot{}
		for _, g := range doc.Groups {
			observed[g.GroupID] = g.Observed
		}
		for i := range rows {
			if rows[i].GroupID != 0 {
				rows[i].Observed = observed[rows[i].GroupID]
			}
		}
		doc.Groups = rows
		if err = workspace.WriteDocument(pulled, doc, ""); err != nil {
			return rep, err
		}
		res, err := s.PushGroupSet(ctx, pulled)
		if err != nil {
			return rep, err
		}
		if res.Status != "completed" {
			rep.Status, rep.Message = "held", fmt.Sprintf("updating the groups stopped (%s): %s. Run it again to finish.", res.Status, res.Message)
			return rep, nil
		}
		roster, err = s.ReadGroupRoster(ctx, nickname, rep.GroupSetID)
		if err != nil {
			return rep, err
		}
		groupByName = map[string]domain.GroupSummary{}
		for _, g := range roster.Groups {
			groupByName[g.Name] = g
		}
	}
	inRound := map[string]bool{}
	for _, t := range round.Teams {
		inRound[t.Name] = true
	}
	for _, g := range roster.Groups {
		if !inRound[g.Name] {
			rep.LeftAloneGroups = append(rep.LeftAloneGroups, g.Name)
		}
	}

	// 3. Memberships: move each team member into their team's group.
	mpath, err := s.PullGroupMemberships(ctx, nickname, rep.GroupSetID)
	if err != nil {
		return rep, err
	}
	var mdoc domain.GroupMembershipDocument
	if _, err = workspace.ReadDocument(mpath, &mdoc); err != nil {
		return rep, err
	}
	observed := map[string]int64{}
	for _, m := range mdoc.Observed.Memberships {
		observed[m.StudentRef] = m.GroupID
	}
	unassigned := map[string]bool{}
	for _, ref := range mdoc.Observed.UnassignedRefs {
		unassigned[ref] = true
	}
	onTeam := map[string]bool{}
	for _, t := range round.Teams {
		target := groupByName[t.Name].ID
		for _, ref := range t.Members {
			onTeam[ref] = true
			cur, assigned := observed[ref]
			switch {
			case assigned && cur == target:
			case assigned:
				mdoc.Changes = append(mdoc.Changes, domain.GroupMembershipChange{StudentRef: ref, FromGroup: domain.NullableGroupID{Set: true, Value: cur}, ToGroup: domain.NullableGroupID{Set: true, Value: target}})
			case unassigned[ref]:
				mdoc.Changes = append(mdoc.Changes, domain.GroupMembershipChange{StudentRef: ref, FromGroup: domain.NullableGroupID{Set: true, Null: true}, ToGroup: domain.NullableGroupID{Set: true, Value: target}})
			default:
				rep.Held = append(rep.Held, fmt.Sprintf("%s has a pending request or invitation in this set; settle it in Canvas first", ref))
			}
		}
	}
	for ref := range observed {
		if !onTeam[ref] {
			rep.LeftAloneStudents = append(rep.LeftAloneStudents, ref)
		}
	}
	sort.Strings(rep.LeftAloneStudents)
	rep.Status = "completed"
	if len(mdoc.Changes) > 0 {
		if err = workspace.WriteDocument(mpath, mdoc, ""); err != nil {
			return rep, err
		}
		res, err := s.PushGroupMemberships(ctx, mpath)
		if err != nil {
			return rep, err
		}
		for _, o := range res.Outcomes {
			switch o.Outcome.Status {
			case "succeeded", "preserved":
				rep.Placed++
			default:
				rep.Held = append(rep.Held, fmt.Sprintf("%s: %s %s", o.StudentRef, o.Outcome.Status, o.Outcome.Message))
			}
		}
	}
	if len(rep.Held) > 0 {
		rep.Status = "partial"
	}
	return rep, nil
}
