package workflow

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

func TestReadResultsAssignmentFromPacketLeavesDraftUnchanged(t *testing.T) {
	root := t.TempDir()
	data, err := os.ReadFile(filepath.Join("..", "..", "engine-docs", "examples", "synthetic-results-two.md"))
	if err != nil {
		t.Fatal(err)
	}
	var example domain.ResultsDocument
	if _, err = workspace.ParseDocument(data, &example); err != nil {
		t.Fatal(err)
	}
	packet := domain.PacketDocument{Schema: 1, Kind: "packet", Course: "DEMO", Assignment: example.Assignment}
	if err = workspace.WriteDocument(filepath.Join(root, filepath.FromSlash(example.Packet)), packet, ""); err != nil {
		t.Fatal(err)
	}
	bare := regexp.MustCompile(`(?m)^assignment:\r?\n(?:[ \t].*\r?\n|\r?\n)*`).ReplaceAll(data, nil)
	if len(bare) == len(data) {
		t.Fatal("example has no assignment block to remove")
	}
	path := filepath.Join(root, "results.md")
	if err = os.WriteFile(path, bare, 0600); err != nil {
		t.Fatal(err)
	}
	var filled domain.ResultsDocument
	if _, err = ReadResultsDocument(root, path, &filled); err != nil {
		t.Fatalf("filled file does not validate: %v", err)
	}
	if filled.Assignment.AssignmentID != example.Assignment.AssignmentID || filled.Packet != example.Packet {
		t.Fatalf("assignment not copied from the packet: %+v", filled.Assignment)
	}
	if _, err = ValidateReviewInput(root, path, ""); err != nil {
		t.Fatal(err)
	}
	if _, err = BuildReview(root, path); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(bare, after) {
		t.Fatalf("validation/review changed the draft: %v", err)
	}
	invalid := bytes.Replace(bare, []byte("points: 3"), []byte("score: 3"), 1)
	if err = os.WriteFile(path, invalid, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = ReadResultsDocument(root, path, &filled); err == nil {
		t.Fatal("invalid component schema accepted")
	}
	after, err = os.ReadFile(path)
	if err != nil || !bytes.Equal(invalid, after) {
		t.Fatalf("failed validation changed the draft: %v", err)
	}
	// Drift in a supplied snapshot still names the differing field.
	drifted := filled.Assignment
	drifted.Points = domain.MustDecimal("999")
	if diff := GradingBasisDiff(packet.Assignment, drifted); len(diff) != 1 || diff[0] != "points" {
		t.Fatalf("drift not named: %v", diff)
	}
	if err = checkResultsBasis(packet.Assignment, drifted); err == nil || !strings.Contains(err.Error(), "points") {
		t.Fatalf("drift accepted: %v", err)
	}
}

func TestGradedDiscussionExampleValidates(t *testing.T) {
	var doc domain.ResultsDocument
	if _, err := workspace.ReadDocument(filepath.Join("..", "..", "engine-docs", "examples", "synthetic-graded-discussion-results.md"), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Assignment.DiscussionTopicID == nil || doc.Students[0].Grade.Evidence == nil {
		t.Fatal("example lacks the discussion topic or evidence")
	}
}

func TestGroupResultsReviewResolvesSnapshotWithoutChangingDraft(t *testing.T) {
	env := newGroupPostEnv(t, false)
	original, err := os.ReadFile(env.results)
	if err != nil {
		t.Fatal(err)
	}
	bare := regexp.MustCompile(`(?m)^assignment:\r?\n(?:[ \t].*\r?\n|\r?\n)*`).ReplaceAll(original, nil)
	if bytes.Equal(original, bare) {
		t.Fatal("fixture has no assignment snapshot")
	}
	if err = os.WriteFile(env.results, bare, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = ValidateReviewInput(env.s.Root, env.results, ""); err != nil {
		t.Fatal(err)
	}
	if _, err = BuildReview(env.s.Root, env.results); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(env.results)
	if err != nil || !bytes.Equal(bare, after) {
		t.Fatalf("group review changed the draft: %v", err)
	}
}
