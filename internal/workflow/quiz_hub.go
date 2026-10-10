package workflow

import (
	"context"
	"fmt"
	"strings"

	"canvasexpertse/internal/domain"
)

// qfTier is one hub tier in a QuizForge file: its supports become a restricted
// Canvas Page for the tier's differentiation tag.
type qfTier struct {
	Label    string `json:"label"`
	Tag      string `json:"tag"`
	Title    string `json:"title"`
	Supports string `json:"supports"` // HTML
}

func (t qfTier) pageTitle(quizTitle string) string {
	if strings.TrimSpace(t.Title) != "" {
		return strings.TrimSpace(t.Title)
	}
	return strings.TrimSpace(quizTitle) + " - " + strings.TrimSpace(t.Tag)
}

// validateHub checks the declaration before any Canvas write. Hub is the only
// differentiation style offered, for New Quizzes: one quiz means
// one gradebook column and the same questions for every student.
func (q *QuizForge) validateHub() error {
	tokens := map[string]int{}
	for _, m := range domain.TierToken.FindAllStringSubmatch(q.Instructions, -1) {
		tokens[m[1]]++
	}
	if q.Differentiation == "" && len(q.Tiers) == 0 {
		if len(tokens) > 0 {
			return fmt.Errorf("instructions have {{tier:...}} link tokens but the file declares no tiers")
		}
		return nil
	}
	if q.Differentiation == "bridge" {
		return fmt.Errorf(`differentiation "bridge" is not offered; use "hub"`)
	}
	if q.Differentiation != "hub" || len(q.Tiers) == 0 {
		return fmt.Errorf(`tiers need differentiation: "hub" and at least one tier`)
	}
	labels, titles := map[string]bool{}, map[string]bool{}
	for i, t := range q.Tiers {
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
			return fmt.Errorf("tiers[%d] %q: a hub tier needs supports (the page content)", i, t.Label)
		}
		title := strings.ToLower(t.pageTitle(q.Title))
		if titles[title] {
			return fmt.Errorf("tier %q would create a second page titled %q; give it a distinct title", t.Label, t.pageTitle(q.Title))
		}
		titles[title] = true
		if tokens[t.Label] != 1 {
			return fmt.Errorf("instructions need exactly one {{tier:%s}} link for tier %q (found %d)", t.Label, t.Label, tokens[t.Label])
		}
		delete(tokens, t.Label)
	}
	for label := range tokens {
		return fmt.Errorf("instructions have {{tier:%s}} but no tier has that label", label)
	}
	return nil
}

// hubTiers prepares the page content: normalized text, highlighted code.
func (q *QuizForge) hubTiers() []HubTier {
	out := make([]HubTier, len(q.Tiers))
	for i, t := range q.Tiers {
		body := normalizeStudentText(highlightCodeBlocks(t.Supports)).(string)
		out[i] = HubTier{Label: t.Label, Tag: strings.TrimSpace(t.Tag), Title: normalizeStudentText(t.pageTitle(q.Title)).(string), BodyHTML: body}
	}
	return out
}

// deliverQuizHub creates or verifies the tier pages. Pages are only published
// when publish is true, which the caller sets after the quiz is complete.
func (s *Service) deliverQuizHub(ctx context.Context, q QuizForge, courseID int64, path string, publish bool) ([]TierPage, error) {
	rec := readSidecar(path)
	return s.DeliverTierPages(ctx, courseID, q.hubTiers(), publish, rec.TierPages, func(pages []TierPage) error {
		rec = readSidecar(path)
		rec.TierPages = pages
		return writeSidecar(path, rec)
	})
}
