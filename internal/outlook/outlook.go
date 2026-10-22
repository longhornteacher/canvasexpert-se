// Package outlook sends mail through the teacher's classic Outlook (COM
// automation). Each call creates a fresh MailItem; saved drafts are never sent.
package outlook

import (
	"errors"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
)

// Attachment is a local file copied into a fresh mail item. ContentID marks an
// inline image referenced by cid: in HTMLBody; an empty ID means a regular file.
type Attachment struct {
	Path        string
	Name        string
	ContentID   string
	ContentType string
}

// Message is one outgoing email. To holds one or more recipient addresses; Cc
// is optional.
type Message struct {
	To          []string
	Cc          []string
	ReplyTo     []string // optional Reply-To addresses
	Subject     string
	HTMLBody    string
	TextBody    string
	Attachments []Attachment
}

// Outcome states whether Outlook accepted the message.
type Outcome string

const (
	Sent      Outcome = "sent"
	Drafted   Outcome = "drafted"
	Uncertain Outcome = "uncertain" // the call errored after Outlook may have accepted it
	Failed    Outcome = "failed"
)

// ErrUnavailable wraps failures with a plain reason for the teacher.
var ErrUnavailable = errors.New("outlook unavailable")

func validate(m Message) error {
	if len(m.To) == 0 {
		return errors.New("message has no recipient")
	}
	if m.Subject == "" {
		return errors.New("message has no subject")
	}
	if m.HTMLBody == "" && m.TextBody == "" {
		return errors.New("message has no body")
	}
	for _, addr := range m.ReplyTo {
		if a, err := mail.ParseAddress(addr); err != nil || a.Address != addr {
			return errors.New("message has an invalid Reply-To address")
		}
	}
	seen := map[string]bool{}
	for _, a := range m.Attachments {
		info, err := os.Stat(a.Path)
		if err != nil || !info.Mode().IsRegular() || !filepath.IsAbs(a.Path) {
			return errors.New("message attachment is unavailable")
		}
		if strings.ContainsAny(a.ContentID, "<>\r\n\" ") || (a.ContentID != "" && seen[a.ContentID]) {
			return errors.New("message has an invalid or duplicate inline attachment ID")
		}
		if a.ContentID != "" {
			seen[a.ContentID] = true
		}
	}
	return nil
}
