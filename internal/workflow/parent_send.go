package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"canvasexpertse/internal/outlook"
	"canvasexpertse/internal/workspace"
)

// MailSender is the Outlook surface the parent flow needs; tests substitute it.
type MailSender interface {
	Send(outlook.Message) (outlook.Outcome, error)
	SaveDraft(outlook.Message) error
	SelfAddress() (string, error)
}

// OutlookSender drives the teacher's classic Outlook.
type OutlookSender struct{}

func (OutlookSender) Send(m outlook.Message) (outlook.Outcome, error) { return outlook.Send(m) }
func (OutlookSender) SaveDraft(m outlook.Message) error               { return outlook.SaveDraft(m) }
func (OutlookSender) SelfAddress() (string, error)                    { return outlook.SelfAddress() }

// ParentDelivery is the local record for one destination. It never holds an address.
type ParentDelivery struct {
	State  string `json:"state"` // sent | drafted | failed | uncertain
	Reason string `json:"reason,omitempty"`
	At     string `json:"at"`
}

type ParentOutcomes struct {
	Schema   int                         `json:"schema"`
	Students map[string][]ParentDelivery `json:"students"`
}

func outcomesPath(dir string) string { return filepath.Join(dir, "outcomes.json") }

func LoadParentOutcomes(dir string) (ParentOutcomes, error) {
	o := ParentOutcomes{Schema: 1, Students: map[string][]ParentDelivery{}}
	b, e := os.ReadFile(outcomesPath(dir))
	if os.IsNotExist(e) {
		return o, nil
	}
	if e != nil {
		return o, e
	}
	if e = json.Unmarshal(b, &o); e != nil {
		return ParentOutcomes{Schema: 1, Students: map[string][]ParentDelivery{}}, fmt.Errorf("outcomes.json is unreadable; keep it and ask for help rather than resending")
	}
	if o.Students == nil {
		o.Students = map[string][]ParentDelivery{}
	}
	return o, nil
}

func saveParentOutcomes(dir string, o ParentOutcomes) error {
	b, e := json.MarshalIndent(o, "", "  ")
	if e != nil {
		return e
	}
	return workspace.WriteFile(outcomesPath(dir), append(b, '\n'), 0600)
}

// destinations returns only enabled parent email addresses, matched on Canvas
// instance and user ID. They never leave this process except through Outlook
// and the teacher's own review page.
func parentDestinations(root string, st workspace.Student) ([]string, error) {
	contacts, e := workspace.OpenContacts(root)
	if e != nil {
		return nil, e
	}
	var out []string
	for _, p := range contacts.Students[workspace.ContactKey(st)].Parents {
		if p.EmailEnabled && strings.TrimSpace(p.Email) != "" {
			out = append(out, p.Email)
		}
	}
	return out, nil
}

func (s *Service) parentStudent(store *workspace.StudentStore, ref string) (workspace.Student, error) {
	return store.Resolve(s.Client.Origin(), ref)
}

// BuildParentReview shows each rendered email with its real To line. Only the
// teacher's browser sees addresses.
func (s *Service) BuildParentReview(dir string) (ReviewModel, error) {
	run, e := LoadParentRun(s.Root, dir)
	if e != nil {
		return ReviewModel{}, e
	}
	store, e := workspace.OpenStudents(s.Root)
	if e != nil {
		return ReviewModel{}, e
	}
	out, e := LoadParentOutcomes(dir)
	if e != nil {
		return ReviewModel{}, e
	}
	m := ReviewModel{Kind: "Parent updates", Title: strings.Join(run.Report.Courses, ", ") + " · run " + run.Report.RunID, PostLabel: "Send approved emails", Actions: []ReviewAction{{Value: "drafts", Label: "Fill Outlook Drafts only"}}, Test: true, Email: true}
	for _, sd := range run.Drafts.Students {
		rs := run.students[sd.Ref]
		card := ReviewCard{Ref: sd.Ref, Alias: rs.Alias, Operation: sd.Decision}
		switch sd.Decision {
		case "skip", "flag":
			card.Hold = fmt.Sprintf("Not sending (%s): %s", sd.Decision, sd.Reason)
			m.Cards = append(m.Cards, withDigest(card))
			continue
		}
		st, e := s.parentStudent(store, sd.Ref)
		if e != nil {
			card.Hold = e.Error()
			m.Cards = append(m.Cards, withDigest(card))
			continue
		}
		to, e := parentDestinations(s.Root, st)
		if e != nil {
			return ReviewModel{}, e
		}
		r := run.Render(sd, st)
		card.Frames = []ReviewFrame{{Label: "Email · " + r.Subject, HTML: r.HTML}}
		if len(to) > 0 {
			card.Facts = append(card.Facts, ReviewFact{Label: "To", Value: strings.Join(to, ", ")})
		}
		if sd.Reason != "" {
			card.Facts = append(card.Facts, ReviewFact{Label: "Why send", Value: sd.Reason})
		}
		delivered := out.Students[sd.Ref]
		sent, uncertain := 0, false
		for _, d := range delivered {
			if d.State == "sent" {
				sent++
			}
			uncertain = uncertain || d.State == "uncertain"
		}
		switch {
		case len(to) == 0:
			card.Hold = "No enabled parent email address on file. Add one on the Students page, then ask the agent to review again."
		case uncertain:
			card.Hold = "Outlook may already have sent this. Check Sent Items; nothing was resent."
		case sent == len(to):
			card.Done = true
		}
		m.Cards = append(m.Cards, withDigest(card))
	}
	sort.SliceStable(m.Cards, func(i, j int) bool { return m.Cards[i].Alias < m.Cards[j].Alias })
	return m, nil
}

func withDigest(c ReviewCard) ReviewCard { c.Digest = digestOf(c); return c }

// SendParentUpdates sends (or, with draftsOnly, saves as Outlook drafts) each
// approved student's email, one message per enabled destination. skip lists refs
// the teacher did not approve. Destinations already sent are never repeated.
func (s *Service) SendParentUpdates(ctx context.Context, dir string, skip map[string]bool, draftsOnly bool, sender MailSender) (string, string, error) {
	run, e := LoadParentRun(s.Root, dir)
	if e != nil {
		return "held", "", e
	}
	store, e := workspace.OpenStudents(s.Root)
	if e != nil {
		return "held", "", e
	}
	out, e := LoadParentOutcomes(dir)
	if e != nil {
		return "held", "", e
	}
	var sent, drafted, failed, uncertain int
	var reasons []string
	for _, sd := range run.Drafts.Students {
		if sd.Decision != "send" || skip[sd.Ref] {
			continue
		}
		if ctx.Err() != nil {
			return "partial", summarizeParent(sent, drafted, failed, uncertain, reasons), ctx.Err()
		}
		st, e := s.parentStudent(store, sd.Ref)
		if e != nil {
			failed++
			reasons = append(reasons, sd.Ref+": "+e.Error())
			continue
		}
		to, e := parentDestinations(s.Root, st)
		if e != nil {
			return "partial", summarizeParent(sent, drafted, failed, uncertain, reasons), e
		}
		r := run.Render(sd, st)
		records := out.Students[sd.Ref]
		for len(records) < len(to) {
			records = append(records, ParentDelivery{})
		}
		for i, addr := range to {
			switch records[i].State {
			case "sent", "uncertain":
				continue
			case "drafted":
				if draftsOnly {
					continue
				}
			}
			msg := outlook.Message{To: []string{addr}, Subject: r.Subject, HTMLBody: r.HTML}
			var outcome outlook.Outcome
			var err error
			if draftsOnly {
				if err = sender.SaveDraft(msg); err == nil {
					outcome = outlook.Drafted
				} else {
					outcome = outlook.Failed
				}
			} else {
				outcome, err = sender.Send(msg)
			}
			d := ParentDelivery{State: string(outcome), At: time.Now().UTC().Format(time.RFC3339)}
			if err != nil {
				d.Reason = err.Error()
				reasons = append(reasons, sd.Ref+": "+err.Error())
			}
			records[i] = d
			out.Students[sd.Ref] = records
			if e := saveParentOutcomes(dir, out); e != nil {
				return "partial", summarizeParent(sent, drafted, failed, uncertain, reasons), e
			}
			switch outcome {
			case outlook.Sent:
				sent++
			case outlook.Drafted:
				drafted++
			case outlook.Uncertain:
				uncertain++
			default:
				failed++
			}
			if err != nil && strings.Contains(err.Error(), outlook.ErrUnavailable.Error()) {
				// Outlook itself is unreachable; stop rather than fail every family.
				return "partial", summarizeParent(sent, drafted, failed, uncertain, reasons), nil
			}
		}
	}
	status := "completed"
	if failed > 0 || uncertain > 0 {
		status = "partial"
	}
	return status, summarizeParent(sent, drafted, failed, uncertain, reasons), nil
}

func summarizeParent(sent, drafted, failed, uncertain int, reasons []string) string {
	parts := []string{fmt.Sprintf("%d sent", sent)}
	if drafted > 0 {
		parts = append(parts, fmt.Sprintf("%d saved as drafts", drafted))
	}
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", failed))
	}
	if uncertain > 0 {
		parts = append(parts, fmt.Sprintf("%d uncertain (check Sent Items)", uncertain))
	}
	s := strings.Join(parts, ", ")
	if len(reasons) > 0 {
		if len(reasons) > 3 {
			reasons = append(reasons[:3], "…")
		}
		s += ". " + strings.Join(reasons, "; ")
	}
	return s
}

// SendParentTest sends the first draft email to the teacher's own address so the
// rendering and the Outlook route can be checked before real families get mail.
func (s *Service) SendParentTest(dir string, sender MailSender) (string, error) {
	run, e := LoadParentRun(s.Root, dir)
	if e != nil {
		return "", e
	}
	store, e := workspace.OpenStudents(s.Root)
	if e != nil {
		return "", e
	}
	self, e := sender.SelfAddress()
	if e != nil {
		return "", e
	}
	for _, sd := range run.Drafts.Students {
		if sd.Decision != "send" {
			continue
		}
		st, e := s.parentStudent(store, sd.Ref)
		if e != nil {
			continue
		}
		r := run.Render(sd, st)
		if _, e = sender.Send(outlook.Message{To: []string{self}, Subject: "[TEST] " + r.Subject, HTMLBody: r.HTML}); e != nil {
			return "", e
		}
		return "Sent a test copy of one email to your own address.", nil
	}
	return "", fmt.Errorf("no email to test: no student is marked send")
}

// StudentSheet is one printable student report. Name is shown only in the
// teacher's browser.
type StudentSheet struct{ Name, HTML string }

// BuildStudentSheets renders one printable sheet per student in the run,
// sorted by name for handing out.
func (s *Service) BuildStudentSheets(dir string) ([]StudentSheet, error) {
	run, e := LoadParentRun(s.Root, dir)
	if e != nil {
		return nil, e
	}
	store, e := workspace.OpenStudents(s.Root)
	if e != nil {
		return nil, e
	}
	var sheets []StudentSheet
	for _, sd := range run.Drafts.Students {
		st, e := s.parentStudent(store, sd.Ref)
		if e != nil {
			return nil, fmt.Errorf("%s: %w", sd.Ref, e)
		}
		sheets = append(sheets, StudentSheet{Name: st.Name, HTML: run.RenderStudent(sd, st).HTML})
	}
	sort.SliceStable(sheets, func(i, j int) bool { return strings.ToLower(sheets[i].Name) < strings.ToLower(sheets[j].Name) })
	return sheets, nil
}
