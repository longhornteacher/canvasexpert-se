package canvasexpertse

import (
	"fmt"
	"path"
	"strings"
)

// IsEngineReferenceTopic lists the offline CLI and MCP reference topics.
func IsEngineReferenceTopic(topic string) bool {
	switch topic {
	case "formats", "fields", "examples", "formats-assignment", "formats-page", "formats-module", "formats-quiz", "formats-inbox", "formats-results":
		return true
	}
	_, err := EngineExample(topic)
	return err == nil
}

// EngineExample returns one synthetic example for a topic of the form
// examples/NAME, where NAME is a file in engine-docs/examples with or without
// its extension, so an agent can read the one example it needs instead of all
// of them.
func EngineExample(topic string) (string, error) {
	name, ok := strings.CutPrefix(topic, "examples/")
	if !ok || name == "" || strings.ContainsAny(name, `/\`) {
		return "", fmt.Errorf("not an example topic: %q", topic)
	}
	entries, err := EngineDocs.ReadDir("engine-docs/examples")
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if e.Name() == name || strings.TrimSuffix(e.Name(), path.Ext(e.Name())) == name {
			b, err := EngineDocs.ReadFile("engine-docs/examples/" + e.Name())
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("# %s\n\n%s\n", e.Name(), b), nil
		}
	}
	return "", fmt.Errorf("no example named %q", name)
}

// EngineFormatSection returns focused contracts from the same embedded format
// source as the full reference, so field changes do not require a second copy.
func EngineFormatSection(section string) (string, error) {
	b, err := EngineDocs.ReadFile("engine-docs/formats.md")
	if err != nil {
		return "", err
	}
	text := strings.ReplaceAll(string(b), "\r\n", "\n")
	var headings []string
	switch section {
	case "assignment":
		headings = []string{"### Paper assignments, assignment groups and cross-course copies", "### Existing rubrics and personal library", "### Differentiated hub (assignments)", "## Individual assignment audiences from clubs"}
	case "page":
		headings = []string{"### Standalone Pages and rich bodies", "### Course Page audiences from clubs"}
	case "module":
		headings = []string{"### Ordered module assembly"}
	case "quiz":
		headings = []string{"## Quiz file (QuizForge)", "### Existing New Quiz content (`new-quiz-edit`)", "## Explicit New Quiz accommodations"}
	case "inbox":
		headings = []string{"## Canvas Inbox"}
	case "results":
		headings = []string{"### Scoring without a Canvas rubric", "### Student operations", "## Exact feedback sample (offline)"}
	default:
		return "", fmt.Errorf("unknown format section %q", section)
	}
	var out strings.Builder
	fmt.Fprintf(&out, "# %s format\n\n", section)
	paragraphs := strings.SplitN(text, "\n\n", 3)
	if len(paragraphs) > 1 {
		out.WriteString(paragraphs[1] + "\n\n")
	}
	if section != "quiz" {
		for _, line := range strings.Split(text, "\n") {
			if strings.HasPrefix(line, "| `"+section+"` |") || section == "results" && (strings.HasPrefix(line, "| `packet` |") || strings.HasPrefix(line, "| `student-work` |")) {
				out.WriteString("| Kind | Contract |\n|---|---|\n" + line + "\n\n")
			}
		}
	}
	for _, heading := range headings {
		start := strings.Index(text, "\n"+heading+"\n")
		if start < 0 {
			return "", fmt.Errorf("format reference is missing %q", heading)
		}
		start++
		depth := strings.Index(heading, " ")
		end := len(text)
		// Locate the next heading at the same or a higher level.
		cursor := start + len(heading) + 1
		for _, line := range strings.SplitAfter(text[cursor:], "\n") {
			trimmed := strings.TrimSuffix(line, "\n")
			if strings.HasPrefix(trimmed, "#") {
				n := strings.Index(trimmed, " ")
				if n > 0 && n <= depth && strings.Trim(trimmed[:n], "#") == "" {
					end = cursor
					break
				}
			}
			cursor += len(line)
		}
		out.WriteString(strings.TrimSpace(text[start:end]) + "\n\n")
	}
	if section == "assignment" {
		start := strings.Index(text, "\nA rubric has a title,")
		end := -1
		if start >= 0 {
			if next := strings.Index(text[start:], "\n#"); next >= 0 {
				end = start + next
			}
		}
		if start >= 0 && end > start {
			out.WriteString(strings.TrimSpace(text[start:end]) + "\n\n")
		}
	}
	return out.String(), nil
}
