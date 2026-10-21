package workflow

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

func TestFillResultsAssignmentFromPacket(t *testing.T) {
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
	if err = FillResultsAssignment(root, path); err != nil {
		t.Fatal(err)
	}
	var filled domain.ResultsDocument
	if _, err = workspace.ReadDocument(path, &filled); err != nil {
		t.Fatalf("filled file does not validate: %v", err)
	}
	if filled.Assignment.AssignmentID != example.Assignment.AssignmentID || filled.Packet != example.Packet {
		t.Fatalf("assignment not copied from the packet: %+v", filled.Assignment)
	}
	// A present block is left alone, and drift names the differing field.
	if err = FillResultsAssignment(root, path); err != nil {
		t.Fatal(err)
	}
	drifted := filled.Assignment
	drifted.Points = domain.MustDecimal("999")
	if diff := GradingBasisDiff(packet.Assignment, drifted); len(diff) != 1 || diff[0] != "points" {
		t.Fatalf("drift not named: %v", diff)
	}
	if err = checkResultsBasis(packet.Assignment, drifted); err == nil || !strings.Contains(err.Error(), "points") {
		t.Fatalf("drift accepted: %v", err)
	}
}
