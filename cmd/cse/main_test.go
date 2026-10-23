package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/playbooks"
	"canvasexpertse/internal/workspace"
)

func TestHelpAndVersionNeedNoWorkspaceOrCanvas(t *testing.T) {
	for _, args := range [][]string{nil, {"help"}, {"--help"}, {"version"}} {
		var out, err bytes.Buffer
		if code := run(context.Background(), args, &out, &err); code != 0 || !strings.Contains(out.String(), "CanvasExpert-SE") {
			t.Fatalf("%v: %d %s", args, code, err.String())
		}
	}
}

func TestValidateResultsResolvesSnapshotWithoutChangingDraft(t *testing.T) {
	root := t.TempDir()
	config := domain.CourseConfig{Schema: 1, Instance: "https://canvas.example", Courses: []domain.Course{{Nickname: "TEST", ID: 1}}}
	if err := workspace.WriteDocument(filepath.Join(root, "context", "courses.md"), config, ""); err != nil {
		t.Fatal(err)
	}
	snapshot := domain.AssignmentSnapshot{Instance: "https://canvas.example", CourseID: 1, AssignmentID: 10, Title: "Reflection", Points: domain.MustDecimal("20"), SubmissionTypes: []string{"online_text_entry"}, GradingType: "points"}
	packet := domain.PacketDocument{Schema: 1, Kind: "packet", Course: "TEST", Assignment: snapshot}
	if err := workspace.WriteDocument(filepath.Join(root, "packet.md"), packet, ""); err != nil {
		t.Fatal(err)
	}
	text := "---\nschema: 1\nkind: results\npacket: packet.md\nstudents:\n  - student_ref: s1\n    operation: comment-only\n    comment_only:\n      attempt: {attempt: 1, submitted_at: '2026-10-23T12:00:00Z'}\n      header: none\n      feedback: Explain your evidence.\n---\nPreserve my unsent draft.\n"
	path := filepath.Join(root, "results.md")
	for _, tc := range []struct {
		text  string
		valid bool
	}{{text, true}, {strings.Replace(text, "header: none", "header: full", 1), false}} {
		if err := os.WriteFile(path, []byte(tc.text), 0600); err != nil {
			t.Fatal(err)
		}
		var out, progress bytes.Buffer
		code := run(context.Background(), []string{"--workspace", root, "validate", "results.md"}, &out, &progress)
		if (code == 0) != tc.valid {
			t.Fatalf("validate exit=%d valid=%v: %s %s", code, tc.valid, out.String(), progress.String())
		}
		after, err := os.ReadFile(path)
		if err != nil || string(after) != tc.text {
			t.Fatalf("validate changed draft bytes: %v", err)
		}
	}
}

func TestInstallUIEnabled(t *testing.T) {
	if installUIEnabled(true, false) || installUIEnabled(false, true) || !installUIEnabled(false, false) {
		t.Fatal("check-only or no-UI must never launch the private hub/browser")
	}
}

func TestPlaybooksCLIStatusAndExplicitReplacement(t *testing.T) {
	root := t.TempDir()
	if err := workspace.WriteDocument(filepath.Join(root, "context", "courses.md"), domain.CourseConfig{Schema: 1, Instance: "https://school.example", Courses: []domain.Course{}}, ""); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(root, "playbooks", "setup.md")
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("teacher version"), 0644); err != nil {
		t.Fatal(err)
	}
	var out, progress bytes.Buffer
	if code := run(context.Background(), []string{"--workspace", root, "playbooks", "status"}, &out, &progress); code != 0 {
		t.Fatalf("status: %d %s", code, progress.String())
	}
	var status struct {
		Files []playbooks.StatusRow `json:"files"`
	}
	if err := json.Unmarshal(out.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	var hash string
	for _, row := range status.Files {
		if row.Path == "playbooks/setup.md" {
			if row.State != "differs" || !row.Replaceable {
				t.Fatalf("unexpected status: %+v", row)
			}
			hash = row.SHA256
		}
	}
	if hash == "" {
		t.Fatal("status omitted current hash")
	}
	out.Reset()
	progress.Reset()
	if code := run(context.Background(), []string{"--workspace", root, "playbooks", "replace", "playbooks/setup.md", "--expected-sha256", "stale"}, &out, &progress); code == 0 {
		t.Fatal("stale hash was accepted")
	}
	if b, _ := os.ReadFile(p); string(b) != "teacher version" {
		t.Fatal("stale hash changed active file")
	}
	out.Reset()
	progress.Reset()
	if code := run(context.Background(), []string{"--workspace", root, "playbooks", "replace", "playbooks/setup.md", "--expected-sha256", hash}, &out, &progress); code != 0 {
		t.Fatalf("replace: %d %s", code, progress.String())
	}
	seed, _ := playbooks.SeedBytes("playbooks/setup.md")
	if b, _ := os.ReadFile(p); string(b) != string(seed) {
		t.Fatal("CLI replacement did not install seed")
	}
}
func TestInvalidResultsAreRejectedBeforeCredentials(t *testing.T) {
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, "context"), 0755)
	_ = os.WriteFile(filepath.Join(root, "context", "courses.md"), []byte("---\nschema: 1\ninstance: https://school.example\naliases: true\ncourses: []\n---\n"), 0644)
	_ = os.WriteFile(filepath.Join(root, "bad.md"), []byte("---\nschema: 1\nkind: results\nkind: results\n---\n"), 0644)
	var out, err bytes.Buffer
	code := run(context.Background(), []string{"--workspace", root, "post", "bad.md"}, &out, &err)
	if code != 2 || strings.Contains(err.String(), "credential") {
		t.Fatalf("invalid file reached environment: %d %s", code, err.String())
	}
}

func TestHelpFormatsAndExamplesNeedNoWorkspace(t *testing.T) {
	for _, topic := range []string{"formats", "fields", "examples"} {
		var out, err bytes.Buffer
		if code := run(context.Background(), []string{"help", topic}, &out, &err); code != 0 || out.Len() == 0 {
			t.Fatalf("help %s: %d %s", topic, code, err.String())
		}
	}
}

func TestRulesLocationCommandConfiguresAndReportsAuthority(t *testing.T) {
	root, syncRoot := t.TempDir(), t.TempDir()
	config := domain.CourseConfig{Schema: 1, Instance: "https://school.example", Courses: []domain.Course{}}
	if err := workspace.WriteDocument(filepath.Join(root, "context", "courses.md"), config, ""); err != nil {
		t.Fatal(err)
	}
	var out, progress bytes.Buffer
	if code := run(context.Background(), []string{"--workspace", root, "rules-location", "--sync", syncRoot}, &out, &progress); code != 0 {
		t.Fatalf("set UserSync: %d %s", code, progress.String())
	}
	for _, want := range []string{"mode: sync", "install_root: " + root, "rules_root: " + syncRoot} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("status omitted %q: %s", want, out.String())
		}
	}
	out.Reset()
	if code := run(context.Background(), []string{"--workspace", root, "rules-location"}, &out, &progress); code != 0 || !strings.Contains(out.String(), "mode: sync") {
		t.Fatalf("read status: %d %s %s", code, out.String(), progress.String())
	}
	if err := os.Rename(syncRoot, syncRoot+".offline"); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	progress.Reset()
	if code := run(context.Background(), []string{"--workspace", root, "rules-location"}, &out, &progress); code != 0 || !strings.Contains(out.String(), "mode: sync") || !strings.Contains(progress.String(), "unavailable") {
		t.Fatalf("missing UserSync status: %d %s %s", code, out.String(), progress.String())
	}
	if err := os.Rename(syncRoot+".offline", syncRoot); err != nil {
		t.Fatal(err)
	}
	if code := run(context.Background(), []string{"--workspace", root, "rules-location", "--local"}, &out, &progress); code != 0 {
		t.Fatalf("set local: %d %s", code, progress.String())
	}
}

func TestValidateResultsPreviewsComputedAssessmentTimingAndHold(t *testing.T) {
	root := t.TempDir()
	config := domain.CourseConfig{Schema: 1, Instance: "https://school.example", Courses: []domain.Course{{Nickname: "TEST", ID: 1}}}
	if e := workspace.WriteDocument(filepath.Join(root, "context", "courses.md"), config, ""); e != nil {
		t.Fatal(e)
	}
	attempt := domain.AttemptRef{Attempt: 1, SubmittedAt: "2026-10-09T12:00:00Z"}
	grade := &domain.GradeOperation{Attempt: attempt, Feedback: "Explain your evidence.", Components: []domain.ComponentScore{{Key: "answer", Evidence: "present", Criteria: []domain.CriterionScore{{CriterionID: "overall", Points: domain.MustDecimal("6.25")}}}}, LateTiming: &domain.LateTiming{Status: "late", SecondsLate: 93600, DueAt: "2026-10-08T10:00:00Z", Basis: "Teacher supplied timing"}}
	held := &domain.GradeOperation{Attempt: attempt, Feedback: "Unreadable work.", Components: []domain.ComponentScore{{Key: "answer", Evidence: "unknown"}}}
	doc := domain.ResultsDocument{Schema: 1, Kind: "results", Packet: "packet.md", Assignment: domain.AssignmentSnapshot{Instance: config.Instance, CourseID: 1, AssignmentID: 10, Title: "Synthetic reflection", Points: domain.MustDecimal("20"), GradingType: "points", SubmissionTypes: []string{"online_text_entry"}}, ScoringBasis: &domain.Rubric{Title: "Overall", Criteria: []domain.Criterion{{ID: "overall", Description: "Overall evidence", Points: domain.MustDecimal("10")}}}, Interpretation: domain.Interpretation{Mapping: "repeated", Rationale: "One response", Components: []domain.Component{{Key: "answer", Label: "Answer", Weight: domain.MustDecimal("1")}}}, Students: []domain.StudentResult{{StudentRef: "s1", Operation: "grade", Grade: grade}, {StudentRef: "s2", Operation: "grade", Grade: held}}}
	if e := workspace.WriteDocument(filepath.Join(root, "results.md"), doc, ""); e != nil {
		t.Fatal(e)
	}
	var out, progress bytes.Buffer
	if code := run(context.Background(), []string{"--workspace", root, "validate", "results.md"}, &out, &progress); code != 0 {
		t.Fatalf("validate failed: %d %s", code, progress.String())
	}
	for _, want := range []string{"Valid document:", "s1  12.5  late: 1d 2h", "s2  held: unreadable work for component answer; not scored"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q from preview: %s", want, out.String())
		}
	}
	if strings.Contains(out.String(), "s2  0") {
		t.Fatal("unknown evidence previewed as a score")
	}
	out.Reset()
	if code := run(context.Background(), []string{"--workspace", root, "validate", "context/courses.md"}, &out, &progress); code != 0 || strings.Contains(out.String(), "late:") || strings.Contains(out.String(), "held:") {
		t.Fatalf("non-results output changed: %d %s %s", code, out.String(), progress.String())
	}
}
