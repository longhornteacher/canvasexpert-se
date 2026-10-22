package workflow

import (
	"fmt"
	"html"
	"strings"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

// replyRouting says where family replies go. The zero value means the email
// concerns only your own courses and nothing changes.
type replyRouting struct {
	ReplyTo  []string // empty: operator-only, change nothing
	NoteHTML string   // empty: operator-only
	NoteText string   // the same note as plain text
	Hold     string   // non-empty: do not send this student's email
	Label    string   // "Your students" or "<Name>'s students", for review grouping
	WithYou  bool     // ReplyTo also includes you (review pages cannot read the address)
}

// replyFor decides Reply-To and the note for a student whose email covers the
// named courses. The note never names you, so renderers pass a nil selfAddress
// and use only NoteHTML and Hold; delivery passes the real one and uses ReplyTo.
func (s *Service) replyFor(courseNicknames []string, selfAddress func() (string, error)) replyRouting {
	type owned struct {
		nickname string
		owner    workspace.Teammate
	}
	var ownedCourses []owned
	operatorCourse := false
	for _, nick := range courseNicknames {
		var course domain.Course
		for _, c := range s.Config.Courses {
			if c.Nickname == nick {
				course = c
				break
			}
		}
		owner, ok, err := s.Owned(course)
		if err != nil {
			return replyRouting{Hold: err.Error()}
		}
		if !ok {
			operatorCourse = true
			continue
		}
		ownedCourses = append(ownedCourses, owned{nick, owner})
	}
	if len(ownedCourses) == 0 {
		return replyRouting{Label: "Your students"}
	}
	var emails []string
	distinct := map[string]bool{}
	var names []string
	for _, o := range ownedCourses {
		if strings.TrimSpace(o.owner.Email) == "" {
			return replyRouting{Hold: o.owner.Name + " has no email on the Team page, so replies couldn't be routed."}
		}
		if !distinct[o.owner.Label] {
			distinct[o.owner.Label] = true
			emails = append(emails, o.owner.Email)
			names = append(names, o.owner.Name)
		}
	}
	r := replyRouting{Label: strings.Join(names, " and ") + "'s students"}
	if len(emails) == 1 && !operatorCourse {
		o := ownedCourses[0].owner
		r.ReplyTo = emails
		r.NoteText = fmt.Sprintf("Please reply to %s, your child's teacher, at %s. Replies to this email go to %s.", o.Name, o.Email, o.Name)
		r.NoteHTML = "<p>" + html.EscapeString(r.NoteText) + "</p>"
		return r
	}
	r.ReplyTo = emails
	if operatorCourse {
		r.WithYou = true
		if selfAddress != nil {
			self, err := selfAddress()
			if err != nil || strings.TrimSpace(self) == "" {
				return replyRouting{Hold: "couldn't read your Outlook address to include you in replies."}
			}
			dup := false
			for _, e := range r.ReplyTo {
				dup = dup || strings.EqualFold(e, self)
			}
			if !dup {
				r.ReplyTo = append(r.ReplyTo, self)
			}
		}
	}
	var text, markup []string
	for _, o := range ownedCourses {
		line := fmt.Sprintf("For %s, please reply to %s at %s.", o.nickname, o.owner.Name, o.owner.Email)
		text = append(text, line)
		markup = append(markup, html.EscapeString(line))
	}
	r.NoteText = strings.Join(text, "\n")
	r.NoteHTML = "<p>" + strings.Join(markup, "<br>") + "</p>"
	return r
}

// replyFact is the review-page line for a student's reply routing, or "".
func replyFact(r replyRouting) (label, value string) {
	switch {
	case r.Hold != "":
		return "Held", r.Hold
	case len(r.ReplyTo) > 0:
		v := strings.Join(r.ReplyTo, ", ")
		if r.WithYou {
			v += ", you"
		}
		return "Replies go to", v
	}
	return "", ""
}

// applyReplyNotes hands each student's note to the run; render adds it to the
// email footer only, never to the printed student sheet.
func (s *Service) applyReplyNotes(run *ParentRun) {
	run.ReplyNotes = map[string]string{}
	for ref, rs := range run.students {
		run.ReplyNotes[ref] = s.replyFor(parentCourseNicknames(rs), nil).NoteHTML
	}
}

func parentCourseNicknames(rs ParentReportStudent) []string {
	var out []string
	for _, c := range rs.Courses {
		out = append(out, c.Course)
	}
	return out
}
