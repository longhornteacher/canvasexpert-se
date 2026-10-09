package workflow

// Student-facing presentation for quiz prompts: dark Monokai code blocks and a
// small TEKS label. New Quizzes strips <style> and class-based CSS but keeps
// inline style attributes (verified live in the original CanvasExpert), so
// every highlighted token carries its own color.

import (
	"encoding/json"
	"html"
	"regexp"
	"strings"
)

const codeWrapper = `<pre style='background-color:#272822;color:#F8F8F2;padding:12px;border-radius:6px;font-family:Consolas,Monaco,"Courier New",monospace;font-size:14px;line-height:1.5;overflow-x:auto;white-space:pre;display:block;'><code>`

var preCode = regexp.MustCompile(`(?s)<pre[^>]*>\s*<code([^>]*)>(.*?)</code>\s*</pre>`)
var codeLanguage = regexp.MustCompile(`language-([A-Za-z0-9+#-]+)`)

// highlightCodeBlocks re-renders every <pre><code> block in the dark Monokai
// box and leaves all other HTML (prose, poetry divs, inline <code>) untouched.
// Tokens are colored as Python unless the block names another language with
// class="language-xyz"; those blocks keep the box but stay uncolored rather
// than get wrong colors.
func highlightCodeBlocks(s string) string {
	return preCode.ReplaceAllStringFunc(s, func(block string) string {
		m := preCode.FindStringSubmatch(block)
		code := strings.TrimRight(html.UnescapeString(m[2]), "\n")
		if lang := codeLanguage.FindStringSubmatch(m[1]); lang != nil && lang[1] != "python" && lang[1] != "py" {
			return codeWrapper + html.EscapeString(code) + "</code></pre>"
		}
		return codeWrapper + highlightPython(code) + "</code></pre>"
	})
}

type pyRule struct {
	re    *regexp.Regexp
	color string // empty: plain text
	kind  string
}

var pyRules = func() []pyRule {
	r := func(p string) *regexp.Regexp { return regexp.MustCompile(`^(?:` + p + `)`) }
	return []pyRule{
		{r(`#[^\n]*`), "#75715E", "comment"},
		{r(`(?s)[rRbBuUfF]{0,2}(?:"""(?:.*?)(?:"""|$)|'''(?:.*?)(?:'''|$))`), "#E6DB74", "string"},
		{r(`[rRbBuUfF]{0,2}(?:"(?:[^"\\\n]|\\.)*"?|'(?:[^'\\\n]|\\.)*'?)`), "#E6DB74", "string"},
		{r(`(?:\d+\.?\d*|\.\d+)(?:[eE][+-]?\d+)?`), "#AE81FF", "number"},
		{r(`@[A-Za-z_][\w.]*`), "#A6E22E", "decorator"},
		{r(`[A-Za-z_]\w*`), "", "name"},
		{r(`[+\-*/%=<>!&|^~]+`), "#F92672", "operator"},
		{r(`(?s).`), "", "other"},
	}
}()

var (
	pyKeywords  = set("def class return if elif else for while in not and or is break continue pass lambda try except finally raise with as from import global nonlocal yield assert del async await")
	pyConstants = set("True False None")
	pyWordOps   = set("in not and or is")
	pyImports   = set("import from as")
)

func set(words string) map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(words) {
		m[w] = true
	}
	return m
}

func span(color, text string) string {
	t := html.EscapeString(text)
	if color == "" {
		return t
	}
	return "<span style='color:" + color + "'>" + t + "</span>"
}

func highlightPython(code string) string {
	var out strings.Builder
	prev := ""
	for len(code) > 0 {
		for _, rule := range pyRules {
			m := rule.re.FindString(code)
			if m == "" {
				continue
			}
			code = code[len(m):]
			color := rule.color
			if rule.kind == "name" {
				switch {
				case prev == "def" || prev == "class":
					color = "#A6E22E"
				case pyImports[m] || pyWordOps[m]:
					color = "#F92672"
				case pyKeywords[m] || pyConstants[m]:
					color = "#66D9EF"
				}
			}
			out.WriteString(span(color, m))
			if strings.TrimSpace(m) != "" {
				prev = m
			}
			break
		}
	}
	return out.String()
}

// teksCodes reads an item's metadata.teks, a string or a list of strings.
func (it qfItem) teksCodes() []string {
	raw := it.Metadata["teks"]
	var one string
	if json.Unmarshal(raw, &one) == nil && one != "" {
		return []string{one}
	}
	var many []string
	if json.Unmarshal(raw, &many) == nil {
		return many
	}
	return nil
}

func teksLabel(codes []string) string {
	if len(codes) == 0 {
		return ""
	}
	return "<p style='font-size:12px;color:#999;margin-top:10px;'><em>TEKS: " + html.EscapeString(strings.Join(codes, ", ")) + "</em></p>"
}
