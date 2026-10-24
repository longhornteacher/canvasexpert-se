package workflow

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"

	"gopkg.in/yaml.v3"
)

// InitResults writes a results scaffold for a pulled packet: the packet path,
// one row per student with the exact current attempt, the operation the
// assignment allows and any discussion-evidence digest. Judgment is left
// blank (interpretation, components, feedback), so the file fails validation
// until the agent has scored every student and can never post as a stub. An
// earlier results file for the same assignment supplies its interpretation and
// scoring_basis, keeping one scoring basis per assignment. It returns the
// workspace-relative path of the new file.
func InitResults(root, packetRel string, now time.Time) (string, error) {
	packetPath, err := workspace.ConstrainPath(root, filepath.FromSlash(strings.TrimSpace(packetRel)))
	if err != nil {
		return "", err
	}
	var packet domain.PacketDocument
	if _, err = workspace.ReadDocument(packetPath, &packet); err != nil {
		return "", fmt.Errorf("results-init needs an individual packet from pull-work: %w", err)
	}
	if len(packet.Students) == 0 {
		return "", errors.New("the packet has no students to score")
	}
	packetRel, err = filepath.Rel(root, packetPath)
	if err != nil {
		return "", err
	}
	assignmentDir := filepath.Dir(packetPath)
	if filepath.Base(filepath.Dir(assignmentDir)) == "pulled" {
		assignmentDir = filepath.Dir(filepath.Dir(assignmentDir))
	}
	outPath := filepath.Join(assignmentDir, "scoring", "results-"+now.Format("20060102-1504")+".md")
	if _, err = os.Stat(outPath); err == nil {
		return "", fmt.Errorf("%s already exists; finish or rename it rather than overwrite unsent work", relSlash(root, outPath))
	}
	operation, body := "grade", "grade"
	if packet.Assignment.NewQuiz {
		operation, body = "comment-only", "comment_only"
	}

	var b strings.Builder
	b.WriteString("---\nschema: 1\nkind: results\n")
	fmt.Fprintf(&b, "packet: %s\n", quoteYAML(filepath.ToSlash(packetRel)))
	writeYAMLBlock(&b, "assignment", packet.Assignment)
	b.WriteString("# Scored components (grade or comment_only):\n#   - key: response_key # from interpretation.components\n#     evidence: present # present, missing, or unknown\n#     criteria: [{criterion_id: criterion_id_from_basis, points: 0}] # assess each mapped criterion\n# Feedback without scores: operation: comment-only; comment_only: {attempt: ..., header: none, feedback: ...}; omit components.\n")
	earlier, interpretation, basis := earlierScoringBasis(root, assignmentDir, packet.Assignment.AssignmentID)
	if interpretation != nil {
		fmt.Fprintf(&b, "# interpretation and scoring basis reused from %s; keep them unless the assignment changed.\n", earlier)
		writeYAMLBlock(&b, "interpretation", interpretation)
	} else {
		b.WriteString("interpretation:\n  mapping: \"\" # repeated or direct\n  rationale: \"\"\n  components: [] # {key, label, weight} per required response unit\n")
	}
	switch {
	case basis != nil:
		writeYAMLBlock(&b, "scoring_basis", basis)
	case packet.Assignment.Rubric == nil:
		b.WriteString("# No Canvas rubric: for scored results add scoring_basis (title; criteria with id, description, points). Feedback without scores needs no scoring_basis.\n")
	}
	if criteria := criteriaHint(packet.Assignment.Rubric, basis); criteria != "" {
		fmt.Fprintf(&b, "# criteria: %s\n", criteria)
	}
	b.WriteString("students:\n")
	for _, rel := range packet.Students {
		studentPath, err := workspace.ConstrainPath(root, filepath.FromSlash(strings.TrimSpace(rel)))
		if err != nil {
			return "", err
		}
		var work domain.StudentWorkDocument
		if _, err = workspace.ReadDocument(studentPath, &work); err != nil {
			return "", fmt.Errorf("%s: %w", rel, err)
		}
		fmt.Fprintf(&b, "  - student_ref: %s\n", quoteYAML(work.StudentRef))
		fmt.Fprintf(&b, "    # work: %s (use this student_ref; Canvas identity is resolved privately)\n", rel)
		if hint := studentHint(work); hint != "" {
			fmt.Fprintf(&b, "    # %s\n", hint)
		}
		fmt.Fprintf(&b, "    operation: %s\n    %s:\n", operation, body)
		fmt.Fprintf(&b, "      attempt: {attempt: %d, submitted_at: %s}\n", work.Current.Ref.Attempt, quoteYAML(work.Current.Ref.SubmittedAt))
		if ev := work.DiscussionEvidence; ev != nil && ev.Digest != "" {
			fmt.Fprintf(&b, "      evidence: {digest: %s}\n", quoteYAML(ev.Digest))
		}
		b.WriteString("      components: []\n      feedback: \"\"\n")
	}
	b.WriteString("---\n\nResults scaffold from results-init. Fill interpretation, each student's components and feedback, then validate.\n")
	if err = workspace.WriteFile(outPath, []byte(b.String()), 0644); err != nil {
		return "", err
	}
	return relSlash(root, outPath), nil
}

// earlierScoringBasis returns the newest results file for this assignment in
// its folder that carries an interpretation, with its scoring_basis.
func earlierScoringBasis(root, dir string, assignmentID int64) (string, *domain.Interpretation, *domain.Rubric) {
	type found struct {
		path string
		mod  time.Time
		doc  domain.ResultsDocument
	}
	var all []found
	_ = walkResultFiles(dir, func(path string) error {
		info, err := os.Stat(path)
		if err != nil {
			return nil
		}
		var doc domain.ResultsDocument
		b, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if kind, err := workspace.DocumentKind(b); err != nil || kind != "results" {
			return nil
		}
		if _, err = ReadResultsDocument(root, path, &doc); err != nil {
			return nil
		}
		if doc.Assignment.AssignmentID != assignmentID || len(doc.Interpretation.Components) == 0 {
			return nil
		}
		all = append(all, found{path, info.ModTime(), doc})
		return nil
	})
	if len(all) == 0 {
		return "", nil, nil
	}
	sort.Slice(all, func(i, j int) bool { return all[i].mod.After(all[j].mod) })
	newest := all[0]
	return relSlash(root, newest.path), &newest.doc.Interpretation, newest.doc.ScoringBasis
}

func studentHint(w domain.StudentWorkDocument) string {
	var hints []string
	if w.Excused {
		hints = append(hints, "excused in Canvas")
	}
	if w.WorkPurpose == "score-and-late" {
		hints = append(hints, "needs late timing")
	} else if w.Current.Late {
		hints = append(hints, "Canvas marks late")
	}
	unreadable := 0
	for _, a := range w.Current.Attachments {
		if a.Status != "readable" {
			unreadable++
		}
	}
	for _, f := range w.Current.WrittenFiles {
		if f.Status != "" && f.Status != "readable" && f.Status != "downloaded" {
			unreadable++
		}
	}
	if unreadable > 0 {
		hints = append(hints, strconv.Itoa(unreadable)+" file(s) not readable: evidence unknown, hold")
	}
	if ev := w.DiscussionEvidence; ev != nil && ev.Status != "" && ev.Status != "complete" {
		hints = append(hints, "discussion coverage "+ev.Status)
	}
	return strings.Join(hints, "; ")
}

func criteriaHint(rubric, basis *domain.Rubric) string {
	r := basis
	if r == nil {
		r = rubric
	}
	if r == nil {
		return ""
	}
	var parts []string
	for _, c := range r.Criteria {
		parts = append(parts, fmt.Sprintf("%s %s (%s)", c.ID, c.Description, c.Points.String()))
	}
	return strings.Join(parts, ", ")
}

func writeYAMLBlock(b *strings.Builder, key string, value any) {
	out, err := yaml.Marshal(map[string]any{key: value})
	if err != nil {
		return
	}
	b.Write(out)
}

func quoteYAML(s string) string {
	out, err := yaml.Marshal(s)
	if err != nil {
		return strconv.Quote(s)
	}
	return strings.TrimSpace(string(out))
}

func relSlash(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(rel)
}
