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

// ParentDelivery is the local record for one student's email. It never holds an address.
type ParentDelivery struct {
	State  string `json:"state"` // sent | drafted | failed | uncertain
	Reason string `json:"reason,omitempty"`
	At     string `json:"at"`
}

// ParentOutcomes is schema 2: one email per student. An older or unreadable
// file stops sending instead of risking a repeat.
type ParentOutcomes struct {
	Schema   int                       `json:"schema"`
	Students map[string]ParentDelivery `json:"students"`
}

func outcomesPath(dir string) string { return filepath.Join(dir, "outcomes.json") }

func LoadParentOutcomes(dir string) (ParentOutcomes, error) {
	o := ParentOutcomes{Schema: 2, Students: map[string]ParentDelivery{}}
	b, e := os.ReadFile(outcomesPath(dir))
	if os.IsNotExist(e) {
		return o, nil
	}
	if e != nil {
		return o, e
	}
	if e = json.Unmarshal(b, &o); e != nil || o.Schema != 2 {
		return ParentOutcomes{Schema: 2, Students: map[string]ParentDelivery{}}, fmt.Errorf("outcomes.json is unreadable; keep it and ask for help rather than resending")
	}
	if o.Students == nil {
		o.Students = map[string]ParentDelivery{}
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

// parentRecipients addresses one email per student: the student's email in To
// and every enabled parent email in Cc, matched on Canvas instance and user ID.
// With no student email on file the parents are the To line. parents counts the
// enabled parent addresses; an update with none is held. Addresses never leave
// this process except through Outlook and the teacher's own review page.
func parentRecipients(root string, st workspace.Student) (to, cc []string, parents int, err error) {
	return familyRecipients(root, st, false)
}

func familyRecipients(root string, st workspace.Student, parentsOnly bool) (to, cc []string, parents int, err error) {
	contacts, err := workspace.OpenContacts(root)
	if err != nil {
		return nil, nil, 0, err
	}
	c := contacts.Students[workspace.ContactKey(st)]
	student := strings.TrimSpace(c.StudentEmail)
	if parentsOnly {
		student = ""
	}
	for _, p := range c.Parents {
		addr := strings.TrimSpace(p.Email)
		if !p.EmailEnabled || addr == "" {
			continue
		}
		parents++
		if strings.EqualFold(addr, student) {
			continue
		}
		cc = append(cc, addr)
	}
	if student == "" {
		return cc, nil, parents, nil
	}
	return []string{student}, cc, parents, nil
}

func (s *Service) parentStudent(store *workspace.StudentStore, ref string) (workspace.Student, error) {
	return store.Resolve(s.Client.Origin(), ref)
}

// BuildParentReview shows each rendered email with its real To line. Only the
// teacher's browser sees addresses.
func (s *Service) BuildParentReview(dir string) (ReviewModel, error) {
	if isFamilyContact(dir) {
		return s.buildFamilyReview(dir)
	}
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
		to, cc, parents, e := parentRecipients(s.Root, st)
		if e != nil {
			return ReviewModel{}, e
		}
		r := run.Render(sd, st)
		card.Frames = []ReviewFrame{{Label: "Email · " + r.Subject, HTML: r.HTML}}
		if parents > 0 {
			card.Facts = append(card.Facts, ReviewFact{Label: "To", Value: strings.Join(to, ", ")})
			if len(cc) > 0 {
				card.Facts = append(card.Facts, ReviewFact{Label: "Cc", Value: strings.Join(cc, ", ")})
			}
		}
		if sd.Reason != "" {
			card.Facts = append(card.Facts, ReviewFact{Label: "Why send", Value: sd.Reason})
		}
		switch state := out.Students[sd.Ref].State; {
		case parents == 0:
			card.Hold = "No enabled parent email address on file. Add one on the Students page, then ask the agent to review again."
		case state == "uncertain":
			card.Hold = "Outlook may already have sent this. Check Sent Items; nothing was resent."
		case state == "sent":
			card.Done = true
		}
		m.Cards = append(m.Cards, withDigest(card))
	}
	sort.SliceStable(m.Cards, func(i, j int) bool { return m.Cards[i].Alias < m.Cards[j].Alias })
	return m, nil
}

func withDigest(c ReviewCard) ReviewCard { c.Digest = digestOf(c); return c }

// SendParentUpdates sends (or, with draftsOnly, saves as Outlook drafts) each
// approved student's email: one message per student, the student in To and the
// enabled parents in Cc. skip lists refs the teacher did not approve. A student
// already sent (or possibly sent) is never repeated.
func (s *Service) SendParentUpdates(ctx context.Context, dir string, skip map[string]bool, draftsOnly bool, sender MailSender) (string, string, error) {
	if isFamilyContact(dir) {
		return s.sendFamilyContact(ctx, dir, skip, draftsOnly, sender)
	}
	run, e := LoadParentRun(s.Root, dir)
	if e != nil {
		return "held", "", e
	}
	tasks := make([]familyMailTask, 0, len(run.Drafts.Students))
	for _, sd := range run.Drafts.Students {
		tasks = append(tasks, familyMailTask{Ref: sd.Ref, Send: sd.Decision == "send", Render: func(st workspace.Student) (outlook.Message, error) {
			r := run.Render(sd, st)
			return outlook.Message{Subject: r.Subject, HTMLBody: r.HTML}, nil
		}})
	}
	return s.deliverFamilyMail(ctx, dir, tasks, false, skip, draftsOnly, sender)
}

// familyMailTask supplies content to the shared private recipient/send loop.
type familyMailTask struct {
	Ref    string
	Send   bool
	Render func(workspace.Student) (outlook.Message, error)
}

func (s *Service) deliverFamilyMail(ctx context.Context, dir string, tasks []familyMailTask, parentsOnly bool, skip map[string]bool, draftsOnly bool, sender MailSender) (string, string, error) {
	// Two open review pages must not submit the same run concurrently.
	lock, e := os.OpenFile(filepath.Join(dir, ".delivery.lock"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return "held", "", fmt.Errorf("this run is already delivering, or was interrupted; check Outlook and outcomes.json before removing .delivery.lock")
	}
	defer os.Remove(lock.Name())
	defer lock.Close()
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
	for _, task := range tasks {
		switch out.Students[task.Ref].State {
		case "sent", "uncertain":
			continue
		case "drafted":
			if draftsOnly {
				continue
			}
		}
		if !task.Send || skip[task.Ref] {
			// Keep accepted draft records as well as accepted/uncertain sends.
			if out.Students[task.Ref].State != "drafted" {
				out.Students[task.Ref] = ParentDelivery{State: "skipped", At: time.Now().UTC().Format(time.RFC3339)}
				if e := saveParentOutcomes(dir, out); e != nil {
					return "partial", "Could not save skipped outcomes.", e
				}
			}
			continue
		}
		if ctx.Err() != nil {
			return "partial", summarizeParent(sent, drafted, failed, uncertain, reasons), ctx.Err()
		}
		st, e := s.parentStudent(store, task.Ref)
		if e != nil {
			failed++
			reasons = append(reasons, task.Ref+": student reference could not be resolved")
			continue
		}
		to, cc, parents, e := familyRecipients(s.Root, st, parentsOnly)
		if e != nil {
			return "partial", summarizeParent(sent, drafted, failed, uncertain, reasons), e
		}
		if parents == 0 {
			continue
		}
		msg, e := task.Render(st)
		if e != nil {
			return "partial", summarizeParent(sent, drafted, failed, uncertain, reasons), e
		}
		msg.To, msg.Cc = to, cc
		// Persist the hold before calling Outlook. A process exit or a failed
		// outcome save after acceptance must never turn into a blind replay.
		out.Students[task.Ref] = ParentDelivery{State: "uncertain", At: time.Now().UTC().Format(time.RFC3339), Reason: "Delivery started; check Outlook if interrupted."}
		if e = saveParentOutcomes(dir, out); e != nil {
			return "held", "No further email was attempted.", e
		}
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
			d.Reason = "Outlook could not complete this email. Check Outlook; uncertain deliveries are held."
			reasons = append(reasons, task.Ref+": "+d.Reason)
		}
		out.Students[task.Ref] = d
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
	if isFamilyContact(dir) {
		return "", fmt.Errorf("use Drafts only to inspect a family-contact message; sending is available only through the teacher's Send button")
	}
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
// sorted by name for handing out. label names the run, for example "CS8 2026-10-16".
func (s *Service) BuildStudentSheets(dir string) (sheets []StudentSheet, label string, err error) {
	run, e := LoadParentRun(s.Root, dir)
	if e != nil {
		return nil, "", e
	}
	store, e := workspace.OpenStudents(s.Root)
	if e != nil {
		return nil, "", e
	}
	label = strings.Join(run.Report.Courses, ", ")
	if run.Report.Scope != nil {
		label += " · " + run.Report.Scope.PeriodName + " " + run.Report.Scope.PeriodStartDate + "–" + run.Report.Scope.PeriodEndDate
	}
	if t, e := time.Parse(time.RFC3339, run.Report.AsOf); e == nil {
		loc := time.UTC
		if len(run.Report.Students) > 0 && len(run.Report.Students[0].Courses) > 0 {
			if l, e := time.LoadLocation(run.Report.Students[0].Courses[0].Timezone); e == nil {
				loc = l
			}
		}
		label += " " + t.In(loc).Format("2006-01-02")
	}
	for _, sd := range run.Drafts.Students {
		st, e := s.parentStudent(store, sd.Ref)
		if e != nil {
			return nil, "", fmt.Errorf("%s: %w", sd.Ref, e)
		}
		sheets = append(sheets, StudentSheet{Name: st.Name, HTML: run.RenderStudent(sd, st).HTML})
	}
	sort.SliceStable(sheets, func(i, j int) bool { return strings.ToLower(sheets[i].Name) < strings.ToLower(sheets[j].Name) })
	return sheets, label, nil
}
