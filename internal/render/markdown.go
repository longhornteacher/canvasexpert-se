package render

import (
	"html"
	"regexp"
	"strings"
)

func escape(s string) string { return html.EscapeString(s) }

// inlineLink is the one inline construct rendered: [text](destination), limited to
// Canvas course paths and http(s) URLs so hub links to tier pages work.
var inlineLink = regexp.MustCompile(`\[([^\[\]]+)\]\((/courses/[0-9]+/pages/[^\s()<>"]+|https?://[^\s()<>"]+)\)`)

func inline(s string) string {
	return inlineLink.ReplaceAllString(escape(s), `<a href="$2">$1</a>`)
}

var (
	headingLine   = regexp.MustCompile(`^(#{1,6})\s+(.+?)\s*#*\s*$`)
	unorderedLine = regexp.MustCompile(`^\s*[-*+]\s+(.+)$`)
	orderedLine   = regexp.MustCompile(`^\s*[0-9]+[.)]\s+(.+)$`)
)

// MarkdownHTML renders a small, safe Markdown subset used for assignment
// directions: headings, paragraphs, ordered/unordered lists, and fenced code.
// Raw HTML and inline Markdown syntax are escaped and displayed as text, except
// [text](destination) links to Canvas course paths or http(s) URLs.
func MarkdownHTML(source string) (string, error) {
	lines := strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n")
	var out strings.Builder
	var paragraph []string
	list := ""
	code := false
	var codeLines []string
	flushParagraph := func() {
		if len(paragraph) != 0 {
			out.WriteString("<p>")
			out.WriteString(inline(strings.Join(paragraph, " ")))
			out.WriteString("</p>")
			paragraph = nil
		}
	}
	closeList := func() {
		if list != "" {
			out.WriteString("</" + list + ">")
			list = ""
		}
	}
	for _, raw := range lines {
		line := strings.TrimRight(raw, " \t")
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			flushParagraph()
			closeList()
			if code {
				out.WriteString("<pre><code>")
				out.WriteString(escape(strings.Join(codeLines, "\n")))
				out.WriteString("</code></pre>")
				codeLines = nil
				code = false
			} else {
				code = true
			}
			continue
		}
		if code {
			codeLines = append(codeLines, raw)
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			flushParagraph()
			closeList()
			continue
		}
		if match := headingLine.FindStringSubmatch(trimmed); match != nil {
			flushParagraph()
			closeList()
			level := len(match[1])
			tag := string(rune('0' + level))
			out.WriteString("<h" + tag + ">" + escape(match[2]) + "</h" + tag + ">")
			continue
		}
		kind, item := "", ""
		if match := unorderedLine.FindStringSubmatch(line); match != nil {
			kind, item = "ul", match[1]
		} else if match := orderedLine.FindStringSubmatch(line); match != nil {
			kind, item = "ol", match[1]
		}
		if kind != "" {
			flushParagraph()
			if list != kind {
				closeList()
				list = kind
				out.WriteString("<" + list + ">")
			}
			out.WriteString("<li>" + inline(strings.TrimSpace(item)) + "</li>")
			continue
		}
		closeList()
		paragraph = append(paragraph, trimmed)
	}
	flushParagraph()
	closeList()
	if code {
		out.WriteString("<pre><code>")
		out.WriteString(escape(strings.Join(codeLines, "\n")))
		out.WriteString("</code></pre>")
	}
	return out.String(), nil
}
