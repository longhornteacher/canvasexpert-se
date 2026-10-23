package workflow

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

// ReadResultsDocument resolves an omitted assignment from the packet in memory.
// Validation and review never rewrite unsent work or invalidate its file hash.
// Posting persists the resolved snapshot together with its action outcomes.
func ReadResultsDocument(root, path string, out any) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	b, snapshot, err := resultsAssignmentData(root, b)
	if err != nil {
		return "", fmt.Errorf("%s: %w", path, err)
	}
	body, err := workspace.ParseDocument(b, out)
	if err == nil && snapshot != nil {
		if doc, ok := out.(*domain.ResultsDocument); ok {
			for _, row := range doc.Students {
				if row.Operation != "late-only" {
					err = checkResultsBasis(*snapshot, doc.Assignment)
					break
				}
			}
		}
	}
	if err != nil {
		return "", fmt.Errorf("%s: %w", path, err)
	}
	return body, nil
}

func resultsAssignmentData(root string, b []byte) ([]byte, *domain.AssignmentSnapshot, error) {
	kind, err := workspace.DocumentKind(b)
	if err != nil || (kind != "results" && kind != "group-results") {
		return b, nil, nil // the ordinary parse reports malformed or other-kind data
	}
	packetRel, ok, err := workspace.HeaderScalar(b, "packet")
	if err != nil || !ok || strings.TrimSpace(packetRel) == "" {
		return b, nil, nil
	}
	packetPath, err := workspace.ConstrainPath(root, packetRel)
	if err != nil {
		return nil, nil, fmt.Errorf("packet: %w", err)
	}
	present := workspace.HeaderHas(b, "assignment")
	var snapshot domain.AssignmentSnapshot
	if kind == "results" {
		var packet domain.PacketDocument
		if _, err = workspace.ReadDocument(packetPath, &packet); err != nil {
			if present {
				return b, nil, nil // posting reports an unreadable packet
			}
			return nil, nil, err
		}
		snapshot = packet.Assignment
	} else {
		var packet domain.GroupPacketDocument
		if _, err = workspace.ReadDocument(packetPath, &packet); err != nil {
			if present {
				return b, nil, nil
			}
			return nil, nil, err
		}
		snapshot = packet.Assignment
	}
	out, _, err := workspace.InsertHeaderKey(b, "assignment", snapshot)
	return out, &snapshot, err
}

func checkResultsBasis(packet, results domain.AssignmentSnapshot) error {
	if diff := GradingBasisDiff(packet, results); len(diff) > 0 {
		return fmt.Errorf("results assignment differs from the packet in %s; remove the assignment block and the Engine resolves it from the packet without changing the draft", strings.Join(diff, ", "))
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
