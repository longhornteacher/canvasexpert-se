package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func writeLocal(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func under(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	return err == nil && filepath.IsLocal(rel)
}

// With UserSync configured, durable teacher data resolves under UserSync and session/runtime
// files stay in UserInstall; a configured but missing UserSync is an error, never a fallback.
func TestDurableDataRoutesToUserSyncAndSessionFilesStayLocal(t *testing.T) {
	install, shared := makeRulesInstall(t), t.TempDir()
	if _, err := SetRulesLocation(install, shared, nil, ""); err != nil {
		t.Fatal(err)
	}
	durable := []string{
		"context/courses.md", "context/students.md", "context/grading.md", "context/reteach.md",
		"context/school-calendar.md", "context/parent-updates.md", "context/team.md", "context/team/turner.md",
		"context/courses/12/notes.md", "context/courses/12/assignments/34.md", "Teacher_Override.md", "My Own Guide.txt",
	}
	for _, rel := range durable {
		p, err := ResolveTeacherPath(install, rel)
		if err != nil || !under(shared, p) {
			t.Errorf("%s should resolve under UserSync: %s %v", rel, p, err)
		}
	}
	for _, rel := range []string{".cse/students.csv", ".cse/rosters.json", ".cse/student-contacts.json"} {
		p, err := ResolveRulesPath(install, rel)
		if err != nil || !under(shared, p) {
			t.Errorf("%s should resolve under UserSync: %s %v", rel, p, err)
		}
	}
	// Local by design: runs, review state, results/receipts, team log and Collab copy records.
	for _, rel := range []string{
		"courses/CS8/family-contact/run1/contact.json", "courses/CS8/family-contact/run1/outcomes.json",
		"courses/CS8/parent-updates/run1/drafts.json", "courses/CS8/scoring/results.md.review.json",
		"courses/_team-copies/copy.json", ".cse/team-log.jsonl",
	} {
		if _, err := ResolveTeacherPath(install, rel); err == nil {
			p := filepath.Join(shared, filepath.FromSlash(rel))
			if _, statErr := os.Stat(p); statErr == nil {
				t.Errorf("%s must not be placed in UserSync", rel)
			}
		}
	}
	if err := os.RemoveAll(shared); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveTeacherPath(install, "context/grading.md"); err == nil {
		t.Fatal("a missing configured UserSync must be an error, not a fallback to stale local data")
	}
}

// Connecting a PC to a UserSync folder that already has files must not leave this PC's other
// durable files behind (they are archived locally and then shadowed by UserSync).
func TestFirstConnectToNonEmptyUserSyncCopiesMissingDurableFilesOnly(t *testing.T) {
	install, shared := makeRulesInstall(t), t.TempDir()
	writeLocal(t, install, "context/school-calendar.md", "local calendar")
	writeLocal(t, install, "context/parent-updates.md", "local parent settings")
	writeLocal(t, install, "context/grading.md", "local grading")
	writeLocal(t, install, "context/courses/12/notes.md", "local course notes")
	writeLocal(t, install, "Teacher_Override.md", "local override")
	writeLocal(t, install, "courses/CS8/draft.md", "session work")
	// UserSync already has one of them (it must win) and is therefore not empty.
	writeLocal(t, shared, "context/grading.md", "shared grading")
	if _, err := SetRulesLocation(install, shared, nil, ""); err != nil {
		t.Fatal(err)
	}
	for rel, want := range map[string]string{
		"context/school-calendar.md": "local calendar", "context/parent-updates.md": "local parent settings",
		"context/courses/12/notes.md": "local course notes", "Teacher_Override.md": "local override",
		"context/grading.md": "shared grading",
	} {
		b, err := os.ReadFile(filepath.Join(shared, filepath.FromSlash(rel)))
		if err != nil || string(b) != want {
			t.Errorf("%s in UserSync = %q (%v), want %q", rel, b, err, want)
		}
	}
	if _, err := os.Stat(filepath.Join(shared, "courses", "CS8", "draft.md")); err == nil {
		t.Error("session work must stay in UserInstall")
	}
}
