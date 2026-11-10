package workflow

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"time"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

// STADStandings is the tally for one round. Fields tagged visibility "team"
// are safe for the student-facing standings Page; "teacher" fields name
// individual students and stay in the conversation.
type STADStandings struct {
	Course  string          `json:"course"`
	Round   string          `json:"round"`
	Window  [2]string       `json:"window"` // first and last school date counted
	Week    [2]string       `json:"week"`   // Monday and Sunday of the week reported
	AsOf    string          `json:"as_of"`
	Teams   []STADTeamScore `json:"teams"`
	Teacher STADTeacherView `json:"teacher"`
	Notes   []string        `json:"notes,omitempty"`
}

type STADTeamScore struct {
	Visibility    string `json:"visibility"` // team
	Name          string `json:"name"`
	RankRound     int    `json:"rank_round"`
	ScoreRound    string `json:"score_round"` // average over active members, one decimal
	RankWeek      int    `json:"rank_week"`
	ScoreWeek     string `json:"score_week"`
	ActiveMembers int    `json:"active_members"`
	Tie           bool   `json:"tie,omitempty"` // shares its round rank with another team
	exactRound    *big.Rat
	exactWeek     *big.Rat
}

type STADTeacherView struct {
	Visibility string             `json:"visibility"` // teacher
	Teams      []STADTeamDetail   `json:"teams"`
	Students   []STADStudentScore `json:"students"`
	Trimmed    []STADTrimmedAward `json:"awards_trimmed,omitempty"`
}

type STADTeamDetail struct {
	Name string `json:"name"`
	// BonusRound and BonusWeek are the all_turned_in bonuses, TeamAwards the
	// awards given to the team as a whole; all are added to the team score
	// directly, not averaged.
	BonusRound     string `json:"bonus_round"`
	BonusWeek      string `json:"bonus_week"`
	TeamAwardRound string `json:"team_awards_round"`
	TeamAwardWeek  string `json:"team_awards_week"`
	// BonusProvisional: this week is still running, so its bonus is judged on
	// the work due so far.
	BonusProvisional bool `json:"bonus_provisional,omitempty"`
}

type STADStudentScore struct {
	Ref         string `json:"student_ref"`
	Alias       string `json:"alias"`
	Team        string `json:"team"`
	EffortRound string `json:"effort_round"`
	EffortWeek  string `json:"effort_week"`
	AwardsRound string `json:"awards_round"`
	AwardsWeek  string `json:"awards_week"`
	// Improvement points, with one row per scored item saying why.
	ImprovementRound string               `json:"improvement_round"`
	ImprovementWeek  string               `json:"improvement_week"`
	Improvement      []STADImprovementRow `json:"improvement,omitempty"`
	OnTimeRound      int                  `json:"on_time_round"`
	LateRound        int                  `json:"late_round"`
	MissingRound     int                  `json:"missing_round"`
	Active           bool                 `json:"active"`
}

type STADTrimmedAward struct {
	Student string `json:"student_ref"`
	Alias   string `json:"alias"`
	Date    string `json:"date"`
	Given   string `json:"given"`
	Counted string `json:"counted"`
	Reason  string `json:"reason"`
}

// stadMember is one student's place on a team for the round.
type stadMember struct {
	ref, alias, team string
	start            string // first school date that counts
	left             string // first school date that no longer counts; "" if still enrolled
	active           bool
}

func (m stadMember) activeOn(d string) bool { return d >= m.start && (m.left == "" || d < m.left) }

type stadEvent struct {
	date string
	ref  string // "" for a team-level event
	team string
	pts  *big.Rat
	kind string // effort, award, bonus, teamaward
}

// STADStandings tallies effort points, awards and bonuses for the round into
// team scores. Points are derived from Canvas Live plus the saved files; the
// team score is the sum of active members' points, each divided by the
// members active on the day it was earned. Read-only.
func (s *Service) STADStandings(ctx context.Context, nickname, weekDate, roundName string, now time.Time) (STADStandings, error) {
	out := STADStandings{Course: nickname, AsOf: now.UTC().Format(time.RFC3339)}
	course, err := s.Course(nickname)
	if err != nil {
		return out, err
	}
	scoped, err := s.ForCourse(ctx, course)
	if err != nil {
		return out, err
	}
	s = scoped
	teams, err := ReadSTADTeams(s.Root, nickname)
	if err != nil {
		return out, err
	}
	rules, err := ReadSTADRules(s.Root, nickname)
	if err != nil {
		return out, err
	}
	awardsFile, err := ReadSTADAwards(s.Root, nickname)
	if err != nil {
		return out, err
	}
	today := courseToday(course.Timezone, now)

	var round STADRound
	roundEnd := ""
	if roundName != "" {
		found := false
		for i, r := range teams.Rounds {
			if r.Name == roundName {
				round, found = r, true
				if i+1 < len(teams.Rounds) {
					roundEnd = addDays(teams.Rounds[i+1].Starts, -1)
				}
			}
		}
		if !found {
			return out, fmt.Errorf("no round named %q in teams.md", roundName)
		}
	} else {
		var ok bool
		if round, ok = teams.CurrentRound(today); !ok {
			return out, fmt.Errorf("no round has started yet; the first starts %s", teams.Rounds[0].Starts)
		}
		for i, r := range teams.Rounds {
			if r.Name == round.Name && i+1 < len(teams.Rounds) {
				roundEnd = addDays(teams.Rounds[i+1].Starts, -1)
			}
		}
	}
	if round.Starts > today {
		return out, fmt.Errorf("round %q starts %s; there is nothing to count yet", round.Name, round.Starts)
	}
	windowEnd := today
	if roundEnd != "" && roundEnd < windowEnd {
		windowEnd = roundEnd
	}
	out.Round, out.Window = round.Name, [2]string{round.Starts, windowEnd}

	anchor := windowEnd
	if weekDate != "" {
		if _, e := time.Parse("2006-01-02", weekDate); e != nil {
			return out, fmt.Errorf("--week needs a date like 2026-11-16")
		}
		anchor = weekDate
	}
	weekStart := mondayOf(anchor)
	out.Week = [2]string{weekStart, addDays(weekStart, 6)}

	// Facts: who is on which team, and whether each is still enrolled.
	enroll, err := s.stadEnrollments(ctx, course.ID, course.Timezone)
	if err != nil {
		return out, err
	}
	var members []*stadMember
	byRef := map[string]*stadMember{}
	teamMembers := map[string][]*stadMember{}
	for _, t := range round.Teams {
		for _, ref := range t.Members {
			e, known := enroll[ref]
			if !known {
				out.Notes = append(out.Notes, fmt.Sprintf("%s is on team %s but is not on this course's roster; their points are not counted", ref, t.Name))
				continue
			}
			m := &stadMember{ref: ref, alias: e.alias, team: t.Name, start: round.Starts, active: e.active}
			if j := t.Joined[ref]; j > m.start {
				m.start = j
			}
			if !e.active && e.endDate != "" {
				m.left = e.endDate
			}
			members = append(members, m)
			byRef[ref] = m
			teamMembers[t.Name] = append(teamMembers[t.Name], m)
		}
	}

	from, _ := time.Parse("2006-01-02", round.Starts)
	to, _ := time.Parse("2006-01-02", windowEnd)
	habits, err := s.workHabitsPacket(ctx, nickname, from, to, nil, now)
	if err != nil {
		return out, err
	}
	out.Notes = append(out.Notes, habits.Flags...)

	pointsFor := map[string]*big.Rat{"on_time": rules.Effort.OnTime.Rat(), "late": rules.Effort.Late.Rat(), "missing": rules.Effort.Missing.Rat()}
	var events []stadEvent
	type counts struct{ onTime, late, missing int }
	tally := map[string]*counts{}
	missingByWeek := map[string]map[string]int{} // team -> week -> missing items
	dueByWeek := map[string]map[string]int{}     // team -> week -> due or excused items
	bump := func(m map[string]map[string]int, team, week string, n int) {
		if m[team] == nil {
			m[team] = map[string]int{}
		}
		m[team][week] += n
	}
	for _, st := range habits.Students {
		m := byRef[st.Ref]
		if m == nil {
			continue
		}
		tally[st.Ref] = &counts{}
		for _, w := range st.Weeks {
			bump(dueByWeek, m.team, w.WeekStart, w.Due+w.Excused)
			bump(missingByWeek, m.team, w.WeekStart, w.Missing)
			for _, group := range []struct {
				class string
				items []WorkHabitItem
			}{{"on_time", w.Items.OnTime}, {"late", w.Items.Late}, {"missing", w.Items.Missing}} {
				for _, it := range group.items {
					d := it.DueAt[:10]
					if !m.activeOn(d) {
						continue
					}
					events = append(events, stadEvent{date: d, ref: st.Ref, team: m.team, pts: pointsFor[group.class], kind: "effort"})
					switch group.class {
					case "on_time":
						tally[st.Ref].onTime++
					case "late":
						tally[st.Ref].late++
					default:
						tally[st.Ref].missing++
					}
				}
			}
		}
	}

	// Improvement points (true STAD), from the scored items in course order.
	improvementRows := map[string][]STADImprovementRow{}
	if rules.Improve != nil {
		items, e := s.stadScoredItems(ctx, course.ID, course.Timezone, *rules.Improve, now)
		if e != nil {
			return out, e
		}
		for _, m := range members {
			rows, e := scoreImprovement(items[m.ref], *rules.Improve)
			if e != nil {
				return out, e
			}
			for _, row := range rows {
				if row.Due < round.Starts || row.Due > windowEnd || !m.activeOn(row.Due) {
					continue
				}
				improvementRows[m.ref] = append(improvementRows[m.ref], row)
				events = append(events, stadEvent{date: row.Due, ref: m.ref, team: m.team, pts: row.exactPoints, kind: "improvement"})
			}
		}
	}

	// Awards: student awards share a weekly cap; team awards go to the team.
	cap := rules.Awards.CapPerStudentPerWeek.Rat()
	spent := map[string]*big.Rat{} // ref|week
	for _, a := range awardsFile.Awards {
		if a.Date < round.Starts || a.Date > windowEnd {
			continue
		}
		if a.Team != "" {
			if len(teamMembers[a.Team]) == 0 {
				out.Notes = append(out.Notes, fmt.Sprintf("an award on %s names team %q, which is not in this round; it was not counted", a.Date, a.Team))
				continue
			}
			events = append(events, stadEvent{date: a.Date, team: a.Team, pts: a.Points.Rat(), kind: "teamaward"})
			continue
		}
		m := byRef[a.Student]
		if m == nil {
			out.Notes = append(out.Notes, fmt.Sprintf("an award on %s names %s, who is on no team in this round; it was not counted", a.Date, a.Student))
			continue
		}
		key := a.Student + "|" + mondayOf(a.Date)
		if spent[key] == nil {
			spent[key] = new(big.Rat)
		}
		room := new(big.Rat).Sub(cap, spent[key])
		counted := new(big.Rat).Set(a.Points.Rat())
		if counted.Cmp(room) > 0 {
			counted = room
		}
		if counted.Sign() < 0 {
			counted = new(big.Rat)
		}
		spent[key].Add(spent[key], counted)
		if counted.Cmp(a.Points.Rat()) < 0 {
			out.Teacher.Trimmed = append(out.Teacher.Trimmed, STADTrimmedAward{Student: m.ref, Alias: m.alias, Date: a.Date, Given: a.Points.String(), Counted: domain.RoundRat(counted, 2).String(), Reason: a.Reason})
		}
		if m.activeOn(a.Date) && counted.Sign() > 0 {
			events = append(events, stadEvent{date: a.Date, ref: m.ref, team: m.team, pts: counted, kind: "award"})
		}
	}

	// Weekly all-turned-in bonus: every active member turned in every due item.
	bonus := rules.TeamBonus.AllTurnedIn.Rat()
	provisional := map[string]bool{}
	if bonus.Sign() > 0 {
		for _, t := range round.Teams {
			for wk := mondayOf(round.Starts); wk <= windowEnd; wk = addDays(wk, 7) {
				wkEnd := addDays(wk, 6)
				active := 0
				for _, m := range teamMembers[t.Name] {
					if m.start <= wkEnd && (m.left == "" || m.left > wk) {
						active++
					}
				}
				if active == 0 || dueByWeek[t.Name][wk] == 0 || missingByWeek[t.Name][wk] > 0 {
					continue
				}
				d := wkEnd
				if d > windowEnd {
					d = windowEnd
					provisional[t.Name] = true
				}
				events = append(events, stadEvent{date: d, team: t.Name, pts: bonus, kind: "bonus"})
			}
		}
	}

	// Scores. A member's point on day d is divided by the members active on d.
	divisor := func(team, d string) int64 {
		var n int64
		for _, m := range teamMembers[team] {
			if m.activeOn(d) {
				n++
			}
		}
		return n
	}
	score := func(team, lo, hi string, kinds ...string) *big.Rat {
		sum := new(big.Rat)
		for _, e := range events {
			if e.team != team || e.date < lo || e.date > hi {
				continue
			}
			if len(kinds) > 0 && !contains(kinds, e.kind) {
				continue
			}
			if e.ref == "" {
				sum.Add(sum, e.pts)
				continue
			}
			if n := divisor(team, e.date); n > 0 {
				sum.Add(sum, new(big.Rat).Quo(e.pts, big.NewRat(n, 1)))
			}
		}
		return sum
	}
	studentPts := func(ref, lo, hi, kind string) *big.Rat {
		sum := new(big.Rat)
		for _, e := range events {
			if e.ref == ref && e.kind == kind && e.date >= lo && e.date <= hi {
				sum.Add(sum, e.pts)
			}
		}
		return sum
	}
	str := func(r *big.Rat) string { return domain.RoundRat(r, 1).String() }
	wkLo, wkHi := out.Week[0], out.Week[1]
	for _, t := range round.Teams {
		active := 0
		for _, m := range teamMembers[t.Name] {
			if m.activeOn(windowEnd) {
				active++
			}
		}
		row := STADTeamScore{Visibility: "team", Name: t.Name, ActiveMembers: active,
			exactRound: score(t.Name, round.Starts, windowEnd), exactWeek: score(t.Name, wkLo, wkHi)}
		row.ScoreRound, row.ScoreWeek = str(row.exactRound), str(row.exactWeek)
		out.Teams = append(out.Teams, row)
		out.Teacher.Teams = append(out.Teacher.Teams, STADTeamDetail{Name: t.Name,
			BonusRound: str(score(t.Name, round.Starts, windowEnd, "bonus")), BonusWeek: str(score(t.Name, wkLo, wkHi, "bonus")),
			TeamAwardRound: str(score(t.Name, round.Starts, windowEnd, "teamaward")), TeamAwardWeek: str(score(t.Name, wkLo, wkHi, "teamaward")),
			BonusProvisional: provisional[t.Name] && windowEnd >= wkLo && windowEnd <= wkHi})
	}
	rank(out.Teams, func(t *STADTeamScore) *big.Rat { return t.exactRound }, func(t *STADTeamScore, r int) { t.RankRound = r })
	rank(out.Teams, func(t *STADTeamScore) *big.Rat { return t.exactWeek }, func(t *STADTeamScore, r int) { t.RankWeek = r })
	for i := range out.Teams {
		for j := range out.Teams {
			if i != j && out.Teams[i].RankRound == out.Teams[j].RankRound {
				out.Teams[i].Tie = true
			}
		}
	}
	sort.SliceStable(out.Teams, func(i, j int) bool { return out.Teams[i].RankRound < out.Teams[j].RankRound })
	out.Teacher.Visibility = "teacher"
	for _, m := range members {
		c := tally[m.ref]
		if c == nil {
			c = &counts{}
		}
		out.Teacher.Students = append(out.Teacher.Students, STADStudentScore{Ref: m.ref, Alias: m.alias, Team: m.team, Active: m.active,
			EffortRound: str(studentPts(m.ref, round.Starts, windowEnd, "effort")), EffortWeek: str(studentPts(m.ref, wkLo, wkHi, "effort")),
			ImprovementRound: str(studentPts(m.ref, round.Starts, windowEnd, "improvement")), ImprovementWeek: str(studentPts(m.ref, wkLo, wkHi, "improvement")), Improvement: improvementRows[m.ref],
			AwardsRound: str(studentPts(m.ref, round.Starts, windowEnd, "award")), AwardsWeek: str(studentPts(m.ref, wkLo, wkHi, "award")),
			OnTimeRound: c.onTime, LateRound: c.late, MissingRound: c.missing})
	}
	sort.Slice(out.Teacher.Students, func(i, j int) bool {
		a, b := out.Teacher.Students[i], out.Teacher.Students[j]
		if a.Team != b.Team {
			return a.Team < b.Team
		}
		return a.Alias < b.Alias
	})
	return out, nil
}

// rank gives competition ranks (1, 1, 3) from exact values.
func rank(teams []STADTeamScore, val func(*STADTeamScore) *big.Rat, set func(*STADTeamScore, int)) {
	for i := range teams {
		r := 1
		for j := range teams {
			if val(&teams[j]).Cmp(val(&teams[i])) > 0 {
				r++
			}
		}
		set(&teams[i], r)
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func addDays(date string, n int) string {
	t, _ := time.Parse("2006-01-02", date)
	return t.AddDate(0, 0, n).Format("2006-01-02")
}

func mondayOf(date string) string {
	t, _ := time.Parse("2006-01-02", date)
	return t.AddDate(0, 0, -((int(t.Weekday()) + 6) % 7)).Format("2006-01-02")
}

type stadEnrollment struct {
	alias   string
	active  bool
	endDate string // school date an inactive enrollment ended, when known
}

// stadEnrollments maps student_ref to enrollment status for every student
// enrollment in the course, active or not.
func (s *Service) stadEnrollments(ctx context.Context, courseID int64, zone string) (map[string]stadEnrollment, error) {
	enrollments, err := StudentEnrollments(ctx, s.Client, courseID)
	if err != nil {
		return nil, err
	}
	store, err := workspace.OpenStudents(s.Root)
	if err != nil {
		return nil, err
	}
	out := map[string]stadEnrollment{}
	for _, en := range enrollments {
		if en.Type != "StudentEnrollment" {
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
		cur := stadEnrollment{alias: st.Alias, active: en.State == "active"}
		if prior, seen := out[st.Ref]; seen && prior.active {
			continue // an active enrollment wins over an old inactive one
		}
		if !cur.active {
			if t, e := time.Parse(time.RFC3339, en.UpdatedAt); e == nil {
				cur.endDate = courseToday(zone, t)
			}
		}
		out[st.Ref] = cur
	}
	return out, store.Save()
}
