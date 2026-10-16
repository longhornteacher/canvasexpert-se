package localui

import (
	"html/template"
	"net/http"
	"regexp"
	"sort"
	"strings"
)

// A hub page is declared once, in the file that owns the feature, with
// register in an init func. Routing, method rules, CSP, nav entries and
// templates all derive from that declaration; see README.md.
type page struct {
	Path     string   // "/courses"
	Methods  []string // allowed methods; default GET. Others get 405 + Allow
	Template string   // templates/<name>.html, parsed at init; optional
	Nav      []navSlot
	Scripts  []string // assets/*.js the page loads; drives CSP script-src
	Connect  bool     // polls Canvas health: adds connect-src and starts the check
	Unlocked bool     // runs without h.mu; the handler takes it around hub fields
	Handle   func(*Hub, http.ResponseWriter, *http.Request)
}

// navSlot places a page in the shared navigation under a primary tab. An empty
// Label makes the slot the primary tab itself (its Order sorts the tabs);
// otherwise it is a sub-tab of that primary, sorted by Order. Href overrides the
// page path, for pages that serve several tabs.
type navSlot struct {
	Primary, Label string
	Order          int
	Href           string
}

var (
	pages     = map[string]*page{}
	templates = map[string]*template.Template{}
	scriptRef = regexp.MustCompile(`/_cse/([A-Za-z0-9_.-]+\.js)`)
)

// register adds pages and fails at init when a declaration does not match its
// files, so a half-removed or half-added feature breaks the first test.
func register(ps ...page) {
	for _, p := range ps {
		p := p
		if p.Path == "" || p.Handle == nil {
			panic("localui: page needs a Path and a Handle")
		}
		if pages[p.Path] != nil {
			panic("localui: page registered twice: " + p.Path)
		}
		for _, s := range p.Scripts {
			assetText("assets/" + s)
		}
		if p.Template != "" {
			if templates[p.Template] == nil {
				templates[p.Template] = pageTemplate(p.Template)
			}
			checkScripts(p)
		}
		pages[p.Path] = &p
	}
}

// checkScripts keeps the declared scripts and the template's script tags equal:
// an undeclared script would be silently blocked by the CSP.
func checkScripts(p page) {
	used := map[string]bool{}
	for _, m := range scriptRef.FindAllStringSubmatch(assetText("templates/"+p.Template+".html"), -1) {
		used[m[1]] = true
	}
	for _, s := range p.Scripts {
		if !used[s] {
			panic("localui: " + p.Path + " declares script " + s + " that templates/" + p.Template + ".html does not load")
		}
		delete(used, s)
	}
	for s := range used {
		panic("localui: templates/" + p.Template + ".html loads " + s + " but " + p.Path + " does not declare it")
	}
}

// tmpl returns a registered page template.
func tmpl(name string) *template.Template {
	t := templates[name]
	if t == nil {
		panic("localui: no registered template " + name)
	}
	return t
}

func hasAppearance(p *page) bool {
	for _, s := range p.Scripts {
		if s == "appearance.js" {
			return true
		}
	}
	return false
}

func (p *page) allows(method string) bool {
	if len(p.Methods) == 0 {
		return method == http.MethodGet
	}
	for _, m := range p.Methods {
		if m == method {
			return true
		}
	}
	return false
}

func (p *page) allowHeader() string {
	if len(p.Methods) == 0 {
		return http.MethodGet
	}
	return strings.Join(p.Methods, ", ")
}

// servePage applies the page's method rule and locking, then its handler.
func (h *Hub) servePage(p *page, w http.ResponseWriter, r *http.Request) {
	if !p.allows(r.Method) {
		w.Header().Set("Allow", p.allowHeader())
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if p.Connect && r.Method == http.MethodGet {
		h.ensureCanvasHealth()
	}
	if p.Unlocked {
		p.Handle(h, w, r)
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	p.Handle(h, w, r)
}

type navEntry struct {
	primary, label, href string
	order                int
}

// navEntries lists the registered nav slots in display order.
func navEntries() []navEntry {
	var out []navEntry
	for _, p := range pages {
		for _, s := range p.Nav {
			href := s.Href
			if href == "" {
				href = p.Path
			}
			out = append(out, navEntry{s.Primary, s.Label, href, s.Order})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].order != out[j].order {
			return out[i].order < out[j].order
		}
		return out[i].href < out[j].href
	})
	return out
}
