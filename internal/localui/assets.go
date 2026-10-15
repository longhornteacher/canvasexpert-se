package localui

import (
	"canvasexpertse/internal/workspace"
	"embed"
	"html/template"
	"net/http"
	"path"
	"strings"
)

// The pages' CSS, scripts and HTML live as plain files so they can be read and
// reviewed as such. Pages keep inline <style>, so the CSP needs no stylesheet
// source; scripts are served from /_cse/ with script-src 'self'.
//
//go:embed assets templates
var assetFS embed.FS

func assetText(name string) string {
	b, err := assetFS.ReadFile(name)
	if err != nil {
		panic("localui: missing embedded file " + name)
	}
	return string(b)
}

var (
	baseCSS = assetText("assets/base.css")
	// pageCSS is the shared look of the hub pages; reviewCSS the review pages'.
	pageCSS   = template.CSS(baseCSS + assetText("assets/hub.css"))
	reviewCSS = template.CSS(baseCSS + assetText("assets/review.css"))
)

var pageFuncs = template.FuncMap{
	"contactRows": func(ref string, parents []workspace.ParentContact) []contactRow {
		if len(parents) == 0 {
			parents = []workspace.ParentContact{{}}
		}
		rows := []contactRow{}
		for i, parent := range parents {
			rows = append(rows, contactRow{Ref: ref, Index: i, ParentContact: parent})
		}
		return rows
	},
	"pageCSS":   func() template.CSS { return pageCSS },
	"reviewCSS": func() template.CSS { return reviewCSS },
	// nav arguments are internal route labels, never request data.
	"nav": func(primary, secondary string) template.HTML { return template.HTML(hubNavFor(primary, secondary)) },
	"inc": func(i int) int { return i + 1 },
}

type contactRow struct {
	Ref   string
	Index int
	workspace.ParentContact
}

// pageTemplate parses templates/<name>.html once at init. Hub pages name their
// template in register; review.go parses its own.
func pageTemplate(name string) *template.Template {
	return template.Must(template.New(name).Funcs(pageFuncs).Parse(assetText("templates/" + name + ".html")))
}

// serveScript serves the embedded scripts under /_cse/. Only files in assets/
// ending in .js are reachable.
func serveScript(w http.ResponseWriter, r *http.Request) bool {
	name := strings.TrimPrefix(r.URL.Path, "/_cse/")
	if !strings.HasPrefix(r.URL.Path, "/_cse/") || path.Ext(name) != ".js" || strings.Contains(name, "/") {
		return false
	}
	body, err := assetFS.ReadFile("assets/" + name)
	if err != nil {
		http.NotFound(w, r)
		return true
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return true
	}
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	_, _ = w.Write(body)
	return true
}
