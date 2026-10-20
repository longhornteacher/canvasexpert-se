package workflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"canvasexpertse/internal/outlook"
	"canvasexpertse/internal/workspace"
)

func isFamilyContact(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "contact.json"))
	// An unreadable contact file must not silently select the grade-report path.
	return !os.IsNotExist(err)
}

// ParentReviewSource preserves the existing review command for either run kind.
func ParentReviewSource(dir string) string {
	if isFamilyContact(dir) {
		return filepath.Join(dir, "contact.json")
	}
	return filepath.Join(dir, "drafts.json")
}

func (s *Service) buildFamilyReview(dir string) (ReviewModel, error) {
	doc, audience, err := LoadFamilyAudience(s.Root, dir)
	if err != nil {
		return ReviewModel{}, err
	}
	config, err := loadParentConfig(s.Root)
	if err != nil {
		return ReviewModel{}, err
	}
	store, err := workspace.OpenStudents(s.Root)
	if err != nil {
		return ReviewModel{}, err
	}
	out, err := LoadParentOutcomes(dir)
	if err != nil {
		return ReviewModel{}, err
	}
	warnings, err := ValidateFamilyContent(dir, doc)
	if err != nil {
		return ReviewModel{}, err
	}
	notices := append(append([]string{}, audience.Warnings...), warnings...)
	notices = append(notices, audience.Blocking...)
	m := ReviewModel{Kind: "Family contact", Title: doc.Title, Email: true, PostLabel: "Send approved emails", Actions: []ReviewAction{{Value: "drafts", Label: "Fill Outlook Drafts only"}}, Notice: strings.Join(notices, " · ")}
	for _, message := range doc.Messages {
		rendered, e := RenderFamilyMessage(dir, message, "{{student_name}}", "{{group_name}}", config.SignOff)
		if e != nil {
			return ReviewModel{}, e
		}
		frame := ReviewFrame{Label: fmt.Sprintf("%s · %d students · %s", message.Key, audience.Counts[message.Key], rendered.Subject), HTML: rendered.PreviewHTML}
		for _, attachment := range rendered.Attachments {
			if attachment.ContentID == "" {
				frame.Files = append(frame.Files, attachment.Name)
			}
		}
		m.Frames = append(m.Frames, frame)
		m.ContentDigest += rendered.Digest
	}
	seen := map[string]bool{}
	for _, row := range audience.Students {
		if seen[row.Ref] {
			continue
		}
		seen[row.Ref] = true
		card := ReviewCard{Ref: row.Ref, Alias: row.Alias, Operation: row.Message}
		card.Facts = []ReviewFact{{Label: "Message", Value: row.Message}, {Label: "Course", Value: row.Course}}
		if row.Group != "" {
			card.Facts = append(card.Facts, ReviewFact{Label: "Group", Value: row.Group})
		}
		st, e := s.parentStudent(store, row.Ref)
		if e != nil {
			card.Hold = "Student reference could not be resolved. Prepare this run again."
		} else {
			to, cc, parents, e := familyRecipients(s.Root, st, doc.Recipients == "parents_only")
			if e != nil {
				return ReviewModel{}, e
			}
			card.Facts = append(card.Facts, ReviewFact{Label: "Student name", Value: st.FirstName}, ReviewFact{Label: "To", Value: strings.Join(to, ", ")})
			if len(cc) > 0 {
				card.Facts = append(card.Facts, ReviewFact{Label: "Cc", Value: strings.Join(cc, ", ")})
			}
			if parents == 0 {
				card.Hold = "No enabled parent email address on file. Add one on the Students page, then review again."
			}
		}
		if len(audience.Blocking) > 0 {
			card.Hold = "Audience needs correction: " + strings.Join(audience.Blocking, " · ")
		}
		switch out.Students[row.Ref].State {
		case "sent":
			card.Done = true
		case "uncertain":
			card.Hold = "Outlook may already have sent this. Check Sent Items and Drafts; nothing was repeated."
		case "drafted":
			card.Facts = append(card.Facts, ReviewFact{Label: "Delivery", Value: "Already saved in Outlook Drafts; Drafts only will not repeat it. Send creates a fresh email."})
		}
		m.Cards = append(m.Cards, withDigest(card))
	}
	sort.SliceStable(m.Cards, func(i, j int) bool { return m.Cards[i].Alias < m.Cards[j].Alias })
	return m, nil
}

func (s *Service) sendFamilyContact(ctx context.Context, dir string, skip map[string]bool, draftsOnly bool, sender MailSender) (string, string, error) {
	doc, audience, err := s.CheckFamilyAudience(ctx, dir)
	if err != nil {
		return "held", "", err
	}
	if len(audience.Blocking) > 0 {
		return "held", "", fmt.Errorf("correct the audience and prepare again: %s", strings.Join(audience.Blocking, "; "))
	}
	config, err := loadParentConfig(s.Root)
	if err != nil {
		return "held", "", err
	}
	if _, err = ValidateFamilyContent(dir, doc); err != nil {
		return "held", "", err
	}
	byKey := map[string]FamilyMessage{}
	for _, msg := range doc.Messages {
		byKey[msg.Key] = msg
	}
	var tasks []familyMailTask
	for _, row := range audience.Students {
		message, ok := byKey[row.Message]
		if !ok {
			return "held", "", fmt.Errorf("audience references an unknown message; prepare again")
		}
		tasks = append(tasks, familyMailTask{Ref: row.Ref, Send: true, Render: func(st workspace.Student) (outlook.Message, error) {
			r, e := RenderFamilyMessage(dir, message, st.FirstName, row.Group, config.SignOff)
			return outlook.Message{Subject: r.Subject, HTMLBody: r.HTML, Attachments: r.Attachments}, e
		}})
	}
	return s.deliverFamilyMail(ctx, dir, tasks, doc.Recipients == "parents_only", skip, draftsOnly, sender)
}
