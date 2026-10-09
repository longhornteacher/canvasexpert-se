package workflow

import (
	"strings"
	"testing"
)

func TestHighlightCodeBlocksColorsPythonInline(t *testing.T) {
	in := "<p>What prints?</p><pre style='x'><code>for i in range(3):\n    print(&quot;hi&quot;)  # say hi\n</code></pre><p>after</p>"
	out := highlightCodeBlocks(in)
	for _, want := range []string{
		"background-color:#272822",
		"<span style='color:#66D9EF'>for</span>",
		"<span style='color:#F92672'>in</span>",
		"<span style='color:#AE81FF'>3</span>",
		"<span style='color:#E6DB74'>&#34;hi&#34;</span>",
		"<span style='color:#75715E'># say hi</span>",
		"<p>What prints?</p>", "<p>after</p>",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %s", want, out)
		}
	}
	if !strings.Contains(out, "\n    <span") && !strings.Contains(out, "\n    print") {
		t.Errorf("indentation not preserved: %q", out)
	}
	if strings.Contains(highlightCodeBlocks("<p>no code</p>"), "272822") {
		t.Error("changed HTML without a code block")
	}
}

func TestHighlightNamesAndAngleBrackets(t *testing.T) {
	out := highlightPython("def add(a, b):\n    return a < b")
	if !strings.Contains(out, "<span style='color:#A6E22E'>add</span>") {
		t.Errorf("def name not colored: %s", out)
	}
	if strings.Contains(out, " < ") || !strings.Contains(out, "&lt;") {
		t.Errorf("code must be HTML-escaped: %s", out)
	}
}

func TestTeksLabelOnlyOnPromptItems(t *testing.T) {
	q, e := ReadQuizForge(writeQF(t, `{"title":"x","items":[
	  {"id":"a","type":"TF","points":1,"prompt":"<p>q</p>","answer":true,"metadata":{"teks":["126.34(c)(4)(A)","126.34(c)(5)(B)"]}},
	  {"id":"b","type":"ORDERING","points":1,"header":"Order","items":["x","y"],"metadata":{"teks":"126.34(c)(4)(A)"}}]}`))
	if e != nil {
		t.Fatal(e)
	}
	items := q.itemPayloads()
	if body := entryOf(items[0])["item_body"].(string); !strings.HasSuffix(body, "<em>TEKS: 126.34(c)(4)(A), 126.34(c)(5)(B)</em></p>") {
		t.Errorf("TEKS label missing: %s", body)
	}
	if body := entryOf(items[1])["item_body"].(string); body != "<p>Order</p>" {
		t.Errorf("ordering header body was disturbed: %q", body)
	}
}

func TestHighlightLeavesOtherLanguagesAndProseAlone(t *testing.T) {
	html := highlightCodeBlocks(`<pre><code class="language-html">&lt;p class="in"&gt;for and not&lt;/p&gt;</code></pre>`)
	if strings.Contains(html, "<span") || !strings.Contains(html, "272822") || !strings.Contains(html, "&lt;p class=") {
		t.Errorf("non-Python code should keep the dark box but no Python colors: %s", html)
	}
	if py := highlightCodeBlocks(`<pre><code class="language-python">if x in y: pass</code></pre>`); !strings.Contains(py, "<span") {
		t.Errorf("python-tagged block should be colored: %s", py)
	}
	prose := `<div style="white-space:pre-line">The cat and the dog, in the end, is not a poem.</div><p>Use <code>for</code> sparingly.</p>`
	if highlightCodeBlocks(prose) != prose {
		t.Error("prose, poetry divs and inline code must not change")
	}
}
