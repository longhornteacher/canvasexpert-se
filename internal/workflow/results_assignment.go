package workflow

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

// FillResultsAssignment lets a results or group-results file omit its
// `assignment` block: the snapshot is copied from the named packet into the
// file, so every later reader sees a complete document. A file that already has
// the block is left alone, but a results file whose grading basis differs from
// its packet is rejected with the differing field named.
func FillResultsAssignment(root, path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	kind, err := workspace.DocumentKind(b)
	if err != nil || (kind != "results" && kind != "group-results") {
		return nil // the ordinary read reports a malformed or other-kind file
	}
	packetRel, ok, err := workspace.HeaderScalar(b, "packet")
	if err != nil || !ok || strings.TrimSpace(packetRel) == "" {
		return nil
	}
	packetPath, err := workspace.ConstrainPath(root, packetRel)
	if err != nil {
		return fmt.Errorf("packet: %w", err)
	}
	present := workspace.HeaderHas(b, "assignment")
	var snapshot domain.AssignmentSnapshot
	if kind == "results" {
		var packet domain.PacketDocument
		if _, err = workspace.ReadDocument(packetPath, &packet); err != nil {
			if present {
				return nil // posting reports an unreadable packet
			}
			return err
		}
		snapshot = packet.Assignment
	} else {
		var packet domain.GroupPacketDocument
		if _, err = workspace.ReadDocument(packetPath, &packet); err != nil {
			if present {
				return nil
			}
			return err
		}
		snapshot = packet.Assignment
	}
	out, inserted, err := workspace.InsertHeaderKey(b, "assignment", snapshot)
	if err != nil {
		return err
	}
	if inserted {
		return workspace.WriteFile(path, out, 0644)
	}
	if kind != "results" {
		return nil
	}
	var doc domain.ResultsDocument
	if _, err = workspace.ParseDocument(b, &doc); err != nil {
		return nil // reported by the ordinary read
	}
	for _, row := range doc.Students {
		if row.Operation != "late-only" {
			return checkResultsBasis(snapshot, doc.Assignment)
		}
	}
	return nil
}

func checkResultsBasis(packet, results domain.AssignmentSnapshot) error {
	if diff := GradingBasisDiff(packet, results); len(diff) > 0 {
		return fmt.Errorf("results assignment differs from the packet in %s; remove the assignment block and the Engine fills it from the packet", strings.Join(diff, ", "))
	}
	return nil
}

// GradingBasisDiff names the assignment fields that make two snapshots differ
// for grading purposes, using the same comparison as SameGradingBasis.
func GradingBasisDiff(a, b domain.AssignmentSnapshot) []string {
	var names []string
	t := reflect.TypeOf(a)
	for i := 0; i < t.NumField(); i++ {
		c := b
		reflect.ValueOf(&c).Elem().Field(i).Set(reflect.ValueOf(a).Field(i))
		if !SameGradingBasis(c, b) {
			name, _, _ := strings.Cut(t.Field(i).Tag.Get("yaml"), ",")
			names = append(names, name)
		}
	}
	return names
}
