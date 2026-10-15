package workspace

import (
	"encoding/json"
	"fmt"
	"net/mail"
	"os"
	"strings"
)

// Contacts are private teacher data, never ordinary context or scoring input.
const contactsFile = ".cse/student-contacts.json"

type ParentContact struct {
	Phone        string `json:"phone,omitempty"`
	Email        string `json:"email,omitempty"`
	Text         bool   `json:"text"`
	EmailEnabled bool   `json:"email_enabled"`
}

type StudentContacts struct {
	StudentEmail string          `json:"student_email,omitempty"`
	Parents      []ParentContact `json:"parents,omitempty"`
}

type ContactStore struct {
	Schema   int                        `json:"schema"`
	Students map[string]StudentContacts `json:"students"`
	path     string
}

// Identity uses Canvas origin and user ID because locally generated refs differ
// between UserInstall folders. Contacts therefore survive roster refresh and a new PC.
func ContactKey(student Student) string {
	return fmt.Sprintf("%s/%d", strings.ToLower(strings.TrimRight(student.Instance, "/")), student.UserID)
}

func OpenContacts(root string) (*ContactStore, error) {
	p, err := ResolveRulesPath(root, contactsFile)
	if err != nil {
		return nil, err
	}
	s := &ContactStore{Schema: 1, Students: map[string]StudentContacts{}, path: p}
	b, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, s); err != nil {
		return nil, fmt.Errorf("could not read private contacts JSON")
	}
	if s.Schema != 1 || s.Students == nil {
		return nil, fmt.Errorf("unsupported private contacts JSON")
	}
	return s, nil
}

func (s *ContactStore) Save() error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return WriteFile(s.path, append(b, '\n'), 0600)
}

func ContactEmail(raw string) (string, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return "", nil
	}
	a, err := mail.ParseAddress(v)
	if err != nil || a.Address != v || strings.ContainsAny(v, "\r\n") {
		return "", fmt.Errorf("enter one email address, or leave it blank")
	}
	return v, nil
}

func ContactPhone(raw string) (string, error) {
	var digits strings.Builder
	for _, r := range strings.TrimSpace(raw) {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
			continue
		}
		if !strings.ContainsRune(" +()-.", r) {
			return "", fmt.Errorf("enter a 10-digit US phone number, or leave it blank")
		}
	}
	v := digits.String()
	if v == "" && strings.TrimSpace(raw) == "" {
		return "", nil
	}
	if len(v) == 11 && v[0] == '1' {
		v = v[1:]
	}
	if len(v) != 10 {
		return "", fmt.Errorf("enter a 10-digit US phone number, or leave it blank")
	}
	return v[:3] + "-" + v[3:6] + "-" + v[6:], nil
}
