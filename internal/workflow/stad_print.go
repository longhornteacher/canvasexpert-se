package workflow

import (
	"fmt"
	"html"
	"sort"
	"strings"

	"canvasexpertse/internal/workspace"
)

// STADTeamSheet renders a round's teams with students' real names, for the
// teacher to project or post on the day teams are announced. Names come from
// the private roster, so the result goes to a printable file outside the
// workspace and never to the agent. roundName "" means the newest round in
// teams.md (announced before it starts). It reads no Canvas data.
func (s *Service) STADTeamSheet(nickname, roundName string) (doc []byte, label string, teams int, err error) {
	if _, err = s.Course(nickname); err != nil {
		return nil, "", 0, err
	}
	f, err := ReadSTADTeams(s.Root, nickname)
	if err != nil {
		return nil, "", 0, err
	}
	round := f.Rounds[len(f.Rounds)-1]
	if roundName != "" {
		found := false
		for _, r := range f.Rounds {
			if r.Name == roundName {
				round, found = r, true
			}
		}
		if !found {
			return nil, "", 0, fmt.Errorf("teams.md has no round named %q", roundName)
		}
	}
	store, err := workspace.OpenStudents(s.Root)
	if err != nil {
		return nil, "", 0, err
	}
	names := map[string]string{}
	for _, st := range store.Rows {
		names[st.Ref] = st.Name
	}
	var b strings.Builder
	b.WriteString(`<!doctype html><html lang="en"><meta charset="utf-8"><title>` + html.EscapeString(round.Name) + `</title><style>
@page{size:letter landscape;margin:0.5in}
body{margin:0;color:#1f2933;font-family:Segoe UI,Arial,sans-serif}
h1{font-size:28pt;margin:0 0 16pt}
.grid{display:grid;grid-template-columns:repeat(3,1fr);gap:14pt}
.team{border:2pt solid #52606d;border-radius:8pt;padding:10pt 14pt;break-inside:avoid}
.team h2{font-size:20pt;margin:0 0 6pt}
.team ul{list-style:none;margin:0;padding:0;font-size:16pt;line-height:1.4}
</style><h1>` + html.EscapeString(round.Name) + `</h1><div class="grid">`)
	for _, t := range round.Teams {
		var members []string
		for _, ref := range t.Members {
			n := names[ref]
			if strings.TrimSpace(n) == "" {
				return nil, "", 0, fmt.Errorf("team %q lists %s, who is not in the saved roster; run stad teams first", t.Name, ref)
			}
			members = append(members, n)
		}
		sort.Slice(members, func(i, j int) bool { return strings.ToLower(members[i]) < strings.ToLower(members[j]) })
		b.WriteString(`<section class="team"><h2>` + html.EscapeString(t.Name) + `</h2><ul>`)
		for _, n := range members {
			b.WriteString("<li>" + html.EscapeString(n) + "</li>")
		}
		b.WriteString("</ul></section>")
	}
	b.WriteString("</div></html>")
	return []byte(b.String()), nickname + " " + round.Name, len(round.Teams), nil
}
