package localui

import "fmt"

// SEExpansion keeps the CLI's edition label consistent with the WebUI.
func SEExpansion() string { return "Special Edition" }

// hubNavFor returns the shared primary and secondary navigation for a hub page,
// built from the registered pages' nav slots. Its arguments are internal route
// labels, never request data.
func hubNavFor(primary, secondary string) string {
	link := func(label, href string, current bool) string {
		if current {
			return fmt.Sprintf(`<a href="%s" aria-current="page">%s</a>`, href, label)
		}
		return fmt.Sprintf(`<a href="%s">%s</a>`, href, label)
	}
	var tabs, secondaryNav string
	for _, e := range navEntries() {
		switch {
		case e.label == "":
			tabs += link(e.primary, e.href, primary == e.primary)
		case e.primary == primary:
			secondaryNav += link(e.label, e.href, secondary == e.label)
		}
	}
	if secondaryNav != "" {
		secondaryNav = `<nav class="hub-subnav" aria-label="` + primary + `">` + secondaryNav + `</nav>`
	}
	return `<header class="hub-header"><a class="hub-brand" href="/">CanvasExpert <span>SE</span></a><nav class="hub-nav" aria-label="Main navigation">` +
		tabs + `</nav></header>` + secondaryNav
}
