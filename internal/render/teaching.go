package render

import "fmt"

// TeachingHTML is an explicit authoring format, not a HTML sanitizer. Canvas
// owns normalization of HTML supplied by the teacher/agent.
func TeachingHTML(body, format string) (string, error) {
	switch format {
	case "", "markdown":
		return MarkdownHTML(body)
	case "html":
		return body, nil
	default:
		return "", fmt.Errorf("body_format must be markdown or html")
	}
}
