package localui

import "fmt"

// SEExpansion keeps the CLI's edition label consistent with the WebUI.
func SEExpansion() string { return "Special Edition" }

// hubNavFor returns the shared primary and secondary navigation for a hub page.
// Its arguments are internal route labels, never request data.
func hubNavFor(primary, secondary string) string {
	primaryLink := func(label, href, key string) string {
		if primary == key {
			return fmt.Sprintf(`<a href="%s" aria-current="page">%s</a>`, href, label)
		}
		return fmt.Sprintf(`<a href="%s">%s</a>`, href, label)
	}
	var secondaryNav string
	items := map[string][][2]string{
		"Classroom": {{"Courses", "/courses"}, {"Students", "/students"}},
		"Settings":  {{"Connections", "/settings?section=connections"}, {"Preferences", "/settings?section=preferences"}, {"Files", "/settings?section=files"}, {"Agent", "/connect"}, {"System", "/settings?section=system"}},
		"Guide":     {{"How it works", "/how"}, {"Capabilities", "/capabilities"}},
	}
	if children := items[primary]; len(children) > 0 {
		secondaryNav = `<nav class="hub-subnav" aria-label="` + primary + `">`
		for _, item := range children {
			if item[0] == secondary {
				secondaryNav += fmt.Sprintf(`<a href="%s" aria-current="page">%s</a>`, item[1], item[0])
			} else {
				secondaryNav += fmt.Sprintf(`<a href="%s">%s</a>`, item[1], item[0])
			}
		}
		secondaryNav += `</nav>`
	}
	return `<header class="hub-header"><a class="hub-brand" href="/">CanvasExpert <span>SE</span></a><nav class="hub-nav" aria-label="Main navigation">` +
		primaryLink("Home", "/", "Home") + primaryLink("Classroom", "/courses", "Classroom") +
		primaryLink("Settings", "/settings", "Settings") + primaryLink("Guide", "/how", "Guide") +
		`</nav></header>` + secondaryNav
}

