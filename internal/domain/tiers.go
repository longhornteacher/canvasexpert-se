package domain

import (
	"fmt"
	"regexp"
	"strings"
)

// TierToken matches a hub link placeholder such as {{tier:Silver}}. The Engine
// replaces it with the verified tier page path before rendering.
var TierToken = regexp.MustCompile(`\{\{tier:([^{}]*)\}\}`)

// validateTiers checks the create-time differentiated hub declaration: unique
// labels, tags present, one link token per tier that has a supplement page, and
// no token that points at a tier without a page.
func validateTiers(d AssignmentDocument, body string) error {
	tokens := map[string]int{}
	for _, m := range TierToken.FindAllStringSubmatch(body, -1) {
		tokens[m[1]]++
	}
	if len(d.Tiers) == 0 {
		if len(tokens) > 0 {
			return fmt.Errorf("body has {{tier:...}} link tokens but the document declares no tiers")
		}
		return nil
	}
	if d.Operation != "create" {
		return fmt.Errorf("tiers are only supported when creating an assignment (operation: create); differentiated pages cannot be added to an existing assignment by %s", d.Operation)
	}
	labels, titles := map[string]bool{}, map[string]bool{}
	for i, t := range d.Tiers {
		if t.Label == "" || strings.TrimSpace(t.Label) != t.Label || strings.ContainsAny(t.Label, "{}") {
			return fmt.Errorf("tiers[%d]: label is required, without surrounding spaces or braces", i)
		}
		if labels[strings.ToLower(t.Label)] {
			return fmt.Errorf("tiers[%d]: duplicate label %q", i, t.Label)
		}
		labels[strings.ToLower(t.Label)] = true
		if strings.TrimSpace(t.Tag) == "" {
			return fmt.Errorf("tiers[%d] %q: tag (the Canvas differentiation tag name) is required", i, t.Label)
		}
		if strings.TrimSpace(t.Supports) == "" {
			if tokens[t.Label] > 0 {
				return fmt.Errorf("tier %q has no supports, so it creates no page; remove its {{tier:%s}} token", t.Label, t.Label)
			}
			continue
		}
		title := strings.ToLower(t.PageTitle(d.Title))
		if titles[title] {
			return fmt.Errorf("tier %q would create a second page titled %q; give it a distinct title", t.Label, t.PageTitle(d.Title))
		}
		titles[title] = true
		switch tokens[t.Label] {
		case 0:
			return fmt.Errorf("tier %q has supports but the body has no {{tier:%s}} link token", t.Label, t.Label)
		case 1:
		default:
			return fmt.Errorf("tier %q has %d {{tier:%s}} tokens in the body; use exactly one", t.Label, tokens[t.Label], t.Label)
		}
		delete(tokens, t.Label)
	}
	for label := range tokens {
		return fmt.Errorf("body token {{tier:%s}} matches no tier label", label)
	}
	return nil
}
