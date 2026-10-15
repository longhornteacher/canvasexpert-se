package workspace

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func makeRulesInstall(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "context"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "context", "courses.md"), []byte("local courses"), 0644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestRulesLocationClassificationAndLocalDefault(t *testing.T) {
	root := makeRulesInstall(t)
	loc, err := GetRulesLocation(root)
	if err != nil || loc.Mode != "local" || loc.InstallRoot != loc.RulesRoot {
		t.Fatalf("local location = %#v, %v", loc, err)
	}
	for _, p := range []string{"AGENTS.md", "CLAUDE.md", "README.txt", "guide.md", "playbooks/score.md", "context/grading.md", "context/courses.md", "context/students.md", "Context/Courses.MD"} {
		if !IsSharedRulePath(p) {
			t.Errorf("%s should be shared", p)
		}
	}
	for _, p := range []string{"README.md", "install-log.jsonl", "courses/a.md", "engine/cse.exe", "playbooks/tool.exe", "context/helper.cmd", ".cse/rules-location.json", "drafts/a.md"} {
		if IsSharedRulePath(p) {
			t.Errorf("%s should remain local", p)
		}
	}
	for _, p := range []string{"README.MD"} {
		if IsSharedRulePath(p) {
			t.Errorf("case variant %s must remain local", p)
		}
	}
	if got, err := ResolveTeacherPath(root, "context/courses.md"); err != nil || got != filepath.Join(root, "context", "courses.md") {
		t.Fatalf("local-only path = %s, %v", got, err)
	}
	if _, err := ResolveTeacherPath(root, "install-log.jsonl"); err == nil {
		t.Fatal("operational log exposed as teacher rules")
	}
}

func TestInstallLogStaysLocalWhenBindingSync(t *testing.T) {
	root := makeRulesInstall(t)
	if err := os.WriteFile(filepath.Join(root, "install-log.jsonl"), []byte("operational"), 0600); err != nil {
		t.Fatal(err)
	}
	syncRoot := t.TempDir()
	if _, err := SetRulesLocation(root, syncRoot, nil, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(syncRoot, "install-log.jsonl")); !os.IsNotExist(err) {
		t.Fatal("UserInstall operational log copied to UserSync")
	}
}

func TestSetRulesLocationInitializesCustomizedRulesAndRestoresLocal(t *testing.T) {
	root := makeRulesInstall(t)
	if err := os.MkdirAll(filepath.Join(root, "playbooks"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "context"), 0755); err != nil {
		t.Fatal(err)
	}
	for p, b := range map[string]string{"AGENTS.md": "my instructions", "README.txt": "my guide", "playbooks/score.md": "custom score", "context/grading.md": "voice", "context/students.md": "private students"} {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(p)), []byte(b), 0644); err != nil {
			t.Fatal(err)
		}
	}
	syncRoot := t.TempDir()
	loc, err := SetRulesLocation(root, syncRoot, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if loc.Mode != "sync" || loc.RulesRoot != syncRoot {
		t.Fatalf("location %#v", loc)
	}
	for p, want := range map[string]string{"AGENTS.md": "my instructions", "README.txt": "my guide", "playbooks/score.md": "custom score", "context/grading.md": "voice"} {
		b, err := os.ReadFile(filepath.Join(syncRoot, filepath.FromSlash(p)))
		if err != nil || string(b) != want {
			t.Fatalf("UserSync %s=%q, %v", p, b, err)
		}
	}
	if _, err := os.Stat(filepath.Join(syncRoot, "context", "students.md")); err != nil {
		t.Fatalf("student notes missing from UserSync: %v", err)
	}
	bootstrap, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil || !strings.Contains(string(bootstrap), "Teacher rules are configured in UserSync") || !strings.Contains(string(bootstrap), "Never read, list or search `.cse/`") || !strings.Contains(string(bootstrap), "student settings") {
		t.Fatalf("bootstrap %q, %v", bootstrap, err)
	}
	// Edits through the active route resolve to UserSync; switching back carries
	// the latest shared version into UserInstall and preserves UserSync.
	p, err := ResolveTeacherPath(root, "playbooks/score.md")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("shared edit"), 0644); err != nil {
		t.Fatal(err)
	}
	loc, err = SetRulesLocation(root, "", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if loc.Mode != "local" || loc.RulesRoot != root {
		t.Fatalf("restored location %#v", loc)
	}
	b, err := os.ReadFile(filepath.Join(root, "playbooks", "score.md"))
	if err != nil || string(b) != "shared edit" {
		t.Fatalf("restored score %q, %v", b, err)
	}
	if _, err := os.Stat(filepath.Join(syncRoot, "playbooks", "score.md")); err != nil {
		t.Fatalf("UserSync removed: %v", err)
	}
}

func TestExistingSyncWinsAndArchivesLocalOriginals(t *testing.T) {
	root := makeRulesInstall(t)
	if err := os.MkdirAll(filepath.Join(root, "playbooks"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "playbooks", "score.md"), []byte("pc local"), 0644); err != nil {
		t.Fatal(err)
	}
	syncRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(syncRoot, "playbooks"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(syncRoot, "playbooks", "score.md"), []byte("shared authority"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := SetRulesLocation(root, syncRoot, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	p, err := ResolveTeacherPath(root, "playbooks/score.md")
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p)
	if err != nil || string(b) != "shared authority" {
		t.Fatalf("active rules %q, %v", b, err)
	}
	archives, err := filepath.Glob(filepath.Join(root, ".cse", "rules-archive", "*", "playbooks", "score.md"))
	if err != nil || len(archives) != 1 {
		t.Fatalf("archives %v, %v", archives, err)
	}
	b, err = os.ReadFile(archives[0])
	if err != nil || string(b) != "pc local" {
		t.Fatalf("archived local %q, %v", b, err)
	}
}

func TestMissingConfiguredSyncErrorsWithoutFallback(t *testing.T) {
	root := makeRulesInstall(t)
	missing := filepath.Join(t.TempDir(), "gone")
	if err := writeRulesBinding(root, missing); err != nil {
		t.Fatal(err)
	}
	loc, err := GetRulesLocation(root)
	if err == nil || loc.Mode != "sync" || loc.RulesRoot != missing {
		t.Fatalf("location %#v, error %v", loc, err)
	}
	if _, err := ResolveTeacherPath(root, "context/courses.md"); err == nil {
		t.Fatal("unavailable UserSync fell back to inactive course setup")
	}
	if _, err := ResolveTeacherPath(root, "AGENTS.md"); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("expected unavailable UserSync error, got %v", err)
	}
}

func TestSwitchingSyncRootsInitializesFromActiveSharedRules(t *testing.T) {
	root := makeRulesInstall(t)
	first, second := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(first, "playbooks"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(first, "AGENTS.md"), []byte("shared instructions"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(first, "playbooks", "score.md"), []byte("shared score"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("inactive stale local"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := SetRulesLocation(root, first, nil, ""); err != nil {
		t.Fatal(err)
	}
	// Initial binding to an existing authority archived and replaced local
	// instructions; the next empty UserSync must use the active shared authority.
	if _, err := SetRulesLocation(root, second, nil, ""); err != nil {
		t.Fatal(err)
	}
	for p, want := range map[string]string{"AGENTS.md": "shared instructions", "playbooks/score.md": "shared score"} {
		b, err := os.ReadFile(filepath.Join(second, filepath.FromSlash(p)))
		if err != nil || string(b) != want {
			t.Fatalf("new UserSync %s = %q, %v", p, b, err)
		}
	}
}

func TestNestedContextAndPlaybookFilesMigrateButLocalTreesDoNot(t *testing.T) {
	root := makeRulesInstall(t)
	for _, dir := range []string{"playbooks/custom/deep", "context/reference/nested", "context/engine", "playbooks/work"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(dir)), 0755); err != nil {
			t.Fatal(err)
		}
	}
	for p, body := range map[string]string{
		"playbooks/custom/deep/guide.txt":      "custom nested playbook",
		"context/reference/nested/calendar.md": "custom nested context",
		"context/engine/private.md":            "do not share",
		"playbooks/work/guide.md":              "teacher work guide",
		"playbooks/custom/tool.exe":            "do not share",
		"context/reference/helper.cmd":         "do not share",
	} {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(p)), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	syncRoot := t.TempDir()
	if _, err := SetRulesLocation(root, syncRoot, nil, ""); err != nil {
		t.Fatal(err)
	}
	for p, want := range map[string]string{
		"playbooks/custom/deep/guide.txt":      "custom nested playbook",
		"context/reference/nested/calendar.md": "custom nested context",
		"playbooks/work/guide.md":              "teacher work guide",
	} {
		b, err := os.ReadFile(filepath.Join(syncRoot, filepath.FromSlash(p)))
		if err != nil || string(b) != want {
			t.Fatalf("migrated %s = %q, %v", p, b, err)
		}
	}
	for _, p := range []string{"context/engine/private.md", "playbooks/custom/tool.exe", "context/reference/helper.cmd"} {
		if _, err := os.Stat(filepath.Join(syncRoot, filepath.FromSlash(p))); !os.IsNotExist(err) {
			t.Errorf("local tree copied %s: %v", p, err)
		}
	}
}

func TestSharedRulesRejectLinkedPrefixes(t *testing.T) {
	root := makeRulesInstall(t)
	syncRoot, outside := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(syncRoot, "context"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(outside, "nested"), 0755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(syncRoot, "context", "alias")
	if err := os.Symlink(filepath.Join(outside, "nested"), link); err != nil {
		if err := os.MkdirAll(filepath.Join(syncRoot, "context"), 0755); err != nil {
			t.Fatal(err)
		}
		command := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", `$ErrorActionPreference='Stop'; New-Item -ItemType Junction -Path $env:CSE_TEST_LINK -Target $env:CSE_TEST_TARGET | Out-Null`)
		command.Env = append(os.Environ(), "CSE_TEST_LINK="+link, "CSE_TEST_TARGET="+filepath.Join(outside, "nested"))
		if output, junctionErr := command.CombinedOutput(); junctionErr != nil {
			t.Skipf("symlink/junction unavailable: %v; %s", err, output)
		}
	}
	if _, err := SetRulesLocation(root, syncRoot, nil, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveTeacherPath(root, "context/alias/new.md"); err == nil {
		t.Fatal("resolved linked shared path")
	}
}

func TestSetRulesLocationRejectsSyncJunctionIntoInstall(t *testing.T) {
	root := makeRulesInstall(t)
	link := filepath.Join(t.TempDir(), "sync-link")
	if err := os.Symlink(root, link); err != nil {
		command := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", `$ErrorActionPreference='Stop'; New-Item -ItemType Junction -Path $env:CSE_TEST_LINK -Target $env:CSE_TEST_TARGET | Out-Null`)
		command.Env = append(os.Environ(), "CSE_TEST_LINK="+link, "CSE_TEST_TARGET="+root)
		if output, junctionErr := command.CombinedOutput(); junctionErr != nil {
			t.Skipf("symlink/junction unavailable: %v; %s", err, output)
		}
	}
	if _, err := SetRulesLocation(root, link, nil, ""); err == nil {
		t.Fatal("accepted UserSync root that resolves to UserInstall")
	}
}

func TestSetRulesLocationRequiresAbsoluteAndBindingRejectsRelative(t *testing.T) {
	root := makeRulesInstall(t)
	if _, err := SetRulesLocation(root, "relative-sync", nil, ""); err == nil {
		t.Fatal("accepted relative UserSync path")
	}
	if err := writeRulesBinding(root, "relative-sync"); err != nil {
		t.Fatal(err)
	}
	loc, err := GetRulesLocation(root)
	if err == nil || loc.Mode != "sync" {
		t.Fatalf("relative binding = %#v, %v", loc, err)
	}
}

func TestUnavailableSyncCannotSeedEmptyReplacement(t *testing.T) {
	root := makeRulesInstall(t)
	missing := filepath.Join(t.TempDir(), "gone")
	if err := writeRulesBinding(root, missing); err != nil {
		t.Fatal(err)
	}
	newSync := t.TempDir()
	if _, err := SetRulesLocation(root, newSync, nil, ""); err == nil {
		t.Fatal("initialized empty UserSync from inactive local rules")
	}
	loc, err := GetRulesLocation(root)
	if err == nil || loc.RulesRoot != missing {
		t.Fatalf("binding changed: %#v, %v", loc, err)
	}
}

func TestScaffoldRoutesSharedSeedsToActiveSync(t *testing.T) {
	root := makeRulesInstall(t)
	syncRoot := t.TempDir()
	if _, err := SetRulesLocation(root, syncRoot, nil, ""); err != nil {
		t.Fatal(err)
	}
	tpl := fstest.MapFS{
		"AGENTS.md":          &fstest.MapFile{Data: []byte("seed agent")},
		"playbooks/score.md": &fstest.MapFile{Data: []byte("seed score")},
		"context/courses.md": &fstest.MapFile{Data: []byte("do not replace")},
		"context/grading.md": &fstest.MapFile{Data: []byte("seed grading")},
	}
	if _, err := Scaffold(root, tpl, "."); err != nil {
		t.Fatal(err)
	}
	for p, want := range map[string]string{"AGENTS.md": "seed agent", "playbooks/score.md": "seed score", "context/grading.md": "seed grading"} {
		b, err := os.ReadFile(filepath.Join(syncRoot, filepath.FromSlash(p)))
		if err != nil || string(b) != want {
			t.Fatalf("shared seed %s = %q, %v", p, b, err)
		}
	}
	b, err := os.ReadFile(filepath.Join(root, "context", "courses.md"))
	if err != nil || string(b) != "local courses" {
		t.Fatalf("local course marker changed: %q, %v", b, err)
	}
}

func TestExplicitNewSyncCanReplaceStaleBinding(t *testing.T) {
	root := makeRulesInstall(t)
	if err := writeRulesBinding(root, filepath.Join(t.TempDir(), "unavailable")); err != nil {
		t.Fatal(err)
	}
	newSync := t.TempDir()
	if err := os.WriteFile(filepath.Join(newSync, "AGENTS.md"), []byte("chosen authority"), 0644); err != nil {
		t.Fatal(err)
	}
	loc, err := SetRulesLocation(root, newSync, nil, "")
	if err != nil || loc.Mode != "sync" || loc.RulesRoot != newSync {
		t.Fatalf("location %#v, %v", loc, err)
	}
}

func TestScaffoldRulesAddsOnlyMissingSharedSeeds(t *testing.T) {
	root := makeRulesInstall(t)
	tpl := fstest.MapFS{"AGENTS.md": &fstest.MapFile{Data: []byte("seed")}, "README.md": &fstest.MapFile{Data: []byte("product")}, "playbooks/score.md": &fstest.MapFile{Data: []byte("score")}, "context/courses.md": &fstest.MapFile{Data: []byte("courses")}, "context/grading.md": &fstest.MapFile{Data: []byte("grading")}}
	if err := os.MkdirAll(filepath.Join(root, "playbooks"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "playbooks", "score.md"), []byte("mine"), 0644); err != nil {
		t.Fatal(err)
	}
	created, err := ScaffoldRules(root, tpl, ".")
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 2 {
		t.Fatalf("created %v", created)
	}
	for _, p := range []string{"README.md"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(p))); !os.IsNotExist(err) {
			t.Errorf("unexpected seed %s: %v", p, err)
		}
	}
	courses, err := os.ReadFile(filepath.Join(root, "context", "courses.md"))
	if err != nil || string(courses) != "local courses" {
		t.Errorf("local course marker changed: %q %v", courses, err)
	}
	b, err := os.ReadFile(filepath.Join(root, "playbooks", "score.md"))
	if err != nil || string(b) != "mine" {
		t.Fatalf("overwrote customization: %q %v", b, err)
	}
}
