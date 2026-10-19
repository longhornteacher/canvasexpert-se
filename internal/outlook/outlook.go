// Package outlook sends mail through the teacher's classic Outlook (COM
// automation). Each call creates a fresh MailItem; saved drafts are never sent.
package outlook

import "errors"

// Message is one outgoing email. To holds one or more recipient addresses; Cc
// is optional.
type Message struct {
	To       []string
	Cc       []string
	Subject  string
	HTMLBody string
	TextBody string
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
	return nil
}
