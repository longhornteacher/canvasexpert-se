package canvasexpertse

import (
	"fmt"
	"strings"
)

// IsEngineReferenceTopic lists the offline CLI and MCP reference topics.
func IsEngineReferenceTopic(topic string) bool {
	switch topic {
	case "formats", "fields", "examples", "formats-assignment", "formats-page", "formats-module", "formats-quiz":
		return true
	}
	return false
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
		headings = []string{"### Paper assignments, assignment groups and cross-course copies", "### Differentiated hub (assignments)", "## Individual assignment audiences from clubs"}
	case "page":
		headings = []string{"### Standalone Pages and rich bodies", "### Course Page audiences from clubs"}
	case "module":
		headings = []string{"### Ordered module assembly"}
	case "quiz":
		headings = []string{"## Quiz file (QuizForge)"}
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
			if strings.HasPrefix(line, "| `"+section+"` |") {
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
