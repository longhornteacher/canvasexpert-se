package render

import (
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// HTMLText extracts readable text from Canvas HTML without interpreting it, so
// an agent can read student work and directions without the raw markup. Block
// elements become lines, list items get "- ", preformatted text keeps its
// whitespace inside ``` fences (indentation matters in code), links keep their
// target and images become "[image: alt]". Script and style content is dropped.
func HTMLText(source string) string {
	var lines []string
	var line strings.Builder
	flush := func() {
		if text := strings.Join(strings.Fields(line.String()), " "); text != "" {
			lines = append(lines, text)
		}
		line.Reset()
	}
	var pre strings.Builder
	preDepth, skipDepth := 0, 0
	var hrefs []string
	var linkText []int // line length when each open link started
	cell := false
	z := html.NewTokenizer(strings.NewReader(source))
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			break
		}
		tok := z.Token()
		a := tok.DataAtom
		switch tt {
		case html.TextToken:
			switch {
			case skipDepth > 0:
			case preDepth > 0:
				pre.WriteString(tok.Data)
			default:
				line.WriteString(tok.Data)
			}
		case html.StartTagToken, html.SelfClosingTagToken:
			switch {
			case a == atom.Script || a == atom.Style:
				if tt == html.StartTagToken {
					skipDepth++
				}
			case skipDepth > 0:
			case a == atom.Pre:
				if preDepth == 0 {
					flush()
					pre.Reset()
				}
				preDepth++
			case preDepth > 0:
				if a == atom.Br {
					pre.WriteString("\n")
				}
			case a == atom.Br:
				flush()
			case a == atom.Img:
				alt := strings.TrimSpace(attr(tok, "alt"))
				if alt == "" {
					line.WriteString(" [image] ")
				} else {
					line.WriteString(" [image: " + alt + "] ")
				}
			case a == atom.A:
				hrefs = append(hrefs, strings.TrimSpace(attr(tok, "href")))
				linkText = append(linkText, line.Len())
			case a == atom.Li:
				flush()
				line.WriteString("- ")
			case a == atom.Td || a == atom.Th:
				if cell {
					line.WriteString(" | ")
				}
				cell = true
			case a == atom.Tr:
				flush()
				cell = false
			case isBlock(a):
				flush()
			}
		case html.EndTagToken:
			switch {
			case a == atom.Script || a == atom.Style:
				if skipDepth > 0 {
					skipDepth--
				}
			case skipDepth > 0:
			case a == atom.Pre:
				if preDepth > 0 {
					preDepth--
				}
				if preDepth == 0 {
					if code := strings.Trim(pre.String(), "\r\n"); strings.TrimSpace(code) != "" {
						lines = append(lines, "```", strings.ReplaceAll(code, "\r\n", "\n"), "```")
					}
					pre.Reset()
				}
			case preDepth > 0:
			case a == atom.A && len(hrefs) > 0:
				href, start := hrefs[len(hrefs)-1], linkText[len(linkText)-1]
				hrefs, linkText = hrefs[:len(hrefs)-1], linkText[:len(linkText)-1]
				label := ""
				if start <= line.Len() {
					label = strings.TrimSpace(line.String()[start:])
				}
				if href != "" && !strings.HasPrefix(href, "#") && !strings.HasPrefix(strings.ToLower(href), "javascript:") && href != label {
					line.WriteString(" (" + href + ")")
				}
			case a == atom.Tr:
				flush()
				cell = false
			case a == atom.Li || isBlock(a):
				flush()
			}
		}
	}
	if pre.Len() > 0 {
		lines = append(lines, "```", strings.Trim(pre.String(), "\r\n"), "```")
	}
	flush()
	return strings.Join(lines, "\n")
}

func attr(tok html.Token, name string) string {
	for _, a := range tok.Attr {
		if a.Key == name {
			return a.Val
		}
	}
	return ""
}

func isBlock(a atom.Atom) bool {
	switch a {
	case atom.P, atom.Div, atom.Section, atom.Article, atom.Ul, atom.Ol, atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6,
		atom.Blockquote, atom.Table, atom.Thead, atom.Tbody, atom.Tfoot, atom.Hr, atom.Header, atom.Footer, atom.Figure, atom.Figcaption, atom.Dl, atom.Dt, atom.Dd:
		return true
	}
	return false
}
