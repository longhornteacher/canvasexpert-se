package render

import (
	"html"
	"regexp"
	"strings"
)

func escape(s string) string { return html.EscapeString(s) }

// inlineLink renders [text](destination), limited to Canvas course paths and
// http(s) URLs so hub links to tier pages work.
var inlineLink = regexp.MustCompile(`\[([^\[\]]+)\]\((/courses/[0-9]+/pages/[^\s()<>"]+|https?://[^\s()<>"]+)\)`)

// Emphasis: **bold**, *italic*, _italic_. Markers must hug the text, and single
// markers must not touch letters or digits, so 2*3*4 and snake_case stay as typed.
var (
	strongText = regexp.MustCompile(`\*\*([^\s*](?:[^*]*[^\s*])?)\*\*`)
	emText     = regexp.MustCompile(`(^|[^\pL\pN*_])(?:\*([^\s*](?:[^*]*[^\s*])?)\*|_([^\s_](?:[^_]*[^\s_])?)_)($|[^\pL\pN*_])`)
)

func emphasis(s string) string {
	s = strongText.ReplaceAllString(s, `<strong>$1</strong>`)
	// Repeat because adjacent spans ("*a* _b_") share the boundary character.
	for next := emText.ReplaceAllString(s, `$1<em>$2$3</em>$4`); next != s; next = emText.ReplaceAllString(s, `$1<em>$2$3</em>$4`) {
		s = next
	}
	return s
}

// inline escapes s, renders links, and applies emphasis to text but never to a
// link destination.
func inline(s string) string {
	s = escape(s)
	var out strings.Builder
	at := 0
	for _, m := range inlineLink.FindAllStringSubmatchIndex(s, -1) {
		out.WriteString(emphasis(s[at:m[0]]))
		out.WriteString(`<a href="` + s[m[4]:m[5]] + `">` + emphasis(s[m[2]:m[3]]) + `</a>`)
		at = m[1]
	}
	out.WriteString(emphasis(s[at:]))
	return out.String()
}

var (
	headingLine   = regexp.MustCompile(`^(#{1,6})\s+(.+?)\s*#*\s*$`)
	unorderedLine = regexp.MustCompile(`^\s*[-*+]\s+(.+)$`)
	orderedLine   = regexp.MustCompile(`^\s*[0-9]+[.)]\s+(.+)$`)
)

// MarkdownHTML renders a small, safe Markdown subset used for assignment
// directions: headings, paragraphs, ordered/unordered lists, fenced code, bold
// and italic emphasis, and [text](destination) links to Canvas course paths or
// http(s) URLs. Raw HTML and other inline Markdown are escaped and shown as text.
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
