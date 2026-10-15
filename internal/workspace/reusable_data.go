package workspace

import (
	"os"
)

// initializeReusableData upgrades the former rules-only UserSync boundary. Existing
// UserSync data wins, and local originals remain intact. Session work is excluded.
func initializeReusableData(source, destination string) error {
	for _, relative := range []string{"context/courses.md", "context/students.md", ".cse/students.csv", ".cse/rosters.json", contactsFile} {
		if err := rejectLinkedPrefixes(source, relative); err != nil {
			return err
		}
		if err := rejectLinkedPrefixes(destination, relative); err != nil {
			return err
		}
		to, err := ConstrainPath(destination, relative)
		if err != nil {
			return err
		}
		if _, err := os.Stat(to); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return err
		}
		from, err := ConstrainPath(source, relative)
		if err != nil {
			return err
		}
		b, err := os.ReadFile(from)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if err := WriteFile(to, b, 0600); err != nil {
			return err
		}
	}
	return nil
}

func ReadTeacherDocument(root, relative string, out any) (string, error) {
	p, err := ResolveTeacherPath(root, relative)
	if err != nil {
		return "", err
	}
	return ReadDocument(p, out)
}

func WriteTeacherDocument(root, relative string, header any, body string) error {
	p, err := ResolveTeacherPath(root, relative)
	if err != nil {
		return err
	}
	return WriteDocument(p, header, body)
}
