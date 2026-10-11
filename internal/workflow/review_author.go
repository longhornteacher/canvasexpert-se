package workflow

import (
	"encoding/json"
	"fmt"
	"html"
	"regexp"
	"strings"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/render"
	"canvasexpertse/internal/workspace"
)

// tierToken is a {{tier:LABEL}} link destination the Engine binds at push; the
// preview shows a stand-in so the link does not look broken.
var tierToken = regexp.MustCompile(`\{\{tier:[^}]*\}\}`)

const tierPreviewLink = "/courses/0/pages/created-when-posted"

func previewTierLinks(s string) string { return tierToken.ReplaceAllString(s, tierPreviewLink) }

func fact(l, v string) ReviewFact { return ReviewFact{Label: l, Value: v} }

func optional(p *string) string {
	if p == nil || *p == "" {
		return "not set"
	}
	return *p
}

// buildAssignmentReview previews a create document: the directions exactly as
// render.MarkdownHTML will write them, and each hub tier page.
func buildAssignmentReview(path string) (ReviewModel, error) {
	var doc domain.AssignmentDocument
	body, e := workspace.ReadDocument(path, &doc)
	if e != nil {
		return ReviewModel{}, e
	}
	if doc.Operation != "create" {
		return ReviewModel{}, fmt.Errorf("review of %q assignment documents is not available yet; only operation: create; edits are previewed through the credentialed review command", doc.Operation)
	}
	if doc.Created != nil && doc.Created.Complete {
		return ReviewModel{}, fmt.Errorf("this assignment was already created; nothing to review")
	}
	directions, e := render.TeachingHTML(previewTierLinks(body), doc.BodyFormat)
	if e != nil {
		return ReviewModel{}, e
	}
	m := ReviewModel{Kind: "New assignment", Title: doc.Title, AllOrNothing: true,
		Notice: "Directions are shown as the Engine will write them to Canvas. Canvas's own editor styling and student view are not simulated."}
	if doc.BodyFormat == "html" {
		// Source is escaped inside the existing sandboxed frame: agent-authored
		// HTML cannot introduce active content or external requests into review.
		directions = "<pre>" + html.EscapeString(directions) + "</pre>"
		m.Notice = "The directions below are escaped HTML source, not a visual preview. Canvas may normalize this HTML; verify saved content and student rendering in Canvas after creation."
	}
	card := ReviewCard{Ref: "assignment", Alias: doc.Title, Operation: "create-assignment", Total: doc.Points.String() + " points",
		Facts: []ReviewFact{
			fact("Course", doc.Course), fact("Submission", strings.Join(doc.SubmissionTypes, ", ")),
			fact("Due", optional(doc.DueAt)), fact("Available from", optional(doc.UnlockAt)), fact("Until", optional(doc.LockAt)),
			fact("Published", fmt.Sprint(doc.Published)),
		},
		Frames: []ReviewFrame{{Label: "Directions (HTML field)", HTML: directions}}}
	if doc.ModuleID > 0 {
		card.Facts = append(card.Facts, fact("Module", fmt.Sprint(doc.ModuleID)))
	}
	if doc.Created != nil && doc.Created.URL != "" {
		card.Facts = append(card.Facts, fact("Canvas preview", doc.Created.URL))
	}
	if doc.Rubric != nil {
		var parts []string
		for _, c := range doc.Rubric.Criteria {
			parts = append(parts, fmt.Sprintf("%s (%s)", c.Description, c.Points.String()))
		}
		card.Facts = append(card.Facts, fact("Rubric", doc.Rubric.Title+": "+strings.Join(parts, "; ")))
	}
	m.Cards = append(m.Cards, card)
	for _, t := range doc.Tiers {
		if strings.TrimSpace(t.Supports) == "" {
			continue
		}
		page, e := render.MarkdownHTML(t.Supports)
		if e != nil {
			return ReviewModel{}, e
		}
		m.Cards = append(m.Cards, ReviewCard{Ref: "tier-" + t.Label, Alias: t.PageTitle(doc.Title), Operation: "hub-tier-page",
			Facts:  []ReviewFact{fact("Visible only to Canvas tag", t.Tag), fact("Page", "created unpublished and restricted; published only if the assignment is")},
			Frames: []ReviewFrame{{Label: "Page body (HTML field)", HTML: page}}})
	}
	return m, nil
}

// buildQuizReview previews a QuizForge quiz: each item's content as the Engine
// writes it, with the answer key kept apart and marked teacher-only.
func buildQuizReview(path string) (ReviewModel, error) {
	q, e := ReadQuizForge(path)
	if e != nil {
		return ReviewModel{}, e
	}
	set, e := q.settings()
	if e != nil {
		return ReviewModel{}, e
	}
	engine := "New Quiz"
	m := ReviewModel{Kind: engine, Title: q.Title, AllOrNothing: true,
		Notice: "Item content is shown as the Engine will write it, as text and HTML. This is not a simulation of the Canvas student experience; check that in Canvas Preview or an unpublished quiz."}
	head := ReviewCard{Ref: "quiz", Alias: q.Title, Operation: "create-quiz",
		Facts: []ReviewFact{fact("Engine", engine), fact("Published", fmt.Sprint(set.Published)),
			fact("Due", orNotSet(set.DueAt)), fact("Available from", orNotSet(set.UnlockAt)), fact("Until", orNotSet(set.LockAt)),
			fact("Attempts", fmt.Sprint(set.AllowedAttempts)), fact("Time limit", timeLimit(set)), fact("Results shown to students", fmt.Sprint(!set.HideResults))}}
	if strings.TrimSpace(q.Instructions) != "" {
		head.Frames = []ReviewFrame{{Label: "Instructions (HTML field)", HTML: previewTierLinks(q.Instructions)}}
	}
	for _, t := range q.Tiers {
		head.Facts = append(head.Facts, fact("Hub tier "+t.Label, "page for Canvas tag "+t.Tag))
	}
	m.Cards = append(m.Cards, head)
	var total float64
	for i, it := range q.preparedItems() {
		p := normalizeStudentText(map[string]any{"prompt": it.Prompt}).(map[string]any)["prompt"].(string)
		pts := 0.0
		if it.Points != nil {
			pts = *it.Points
		}
		total += pts
		card := ReviewCard{Ref: fmt.Sprintf("item-%d", i+1), Alias: fmt.Sprintf("%d. %s", i+1, it.title(it.Type)), Operation: "quiz-item", Total: fmt.Sprintf("%g pts", pts),
			Facts:  []ReviewFact{fact("Type", it.Type)},
			Frames: []ReviewFrame{{Label: "Question (HTML field)", HTML: p + studentSide(it)}}}
		card.Key = answerKey(it)
		m.Cards = append(m.Cards, card)
	}
	m.Cards[0].Total = fmt.Sprintf("%g points, %d questions", total, len(m.Cards)-1)
	return m, nil
}

func orNotSet(s string) string {
	if s == "" {
		return "not set"
	}
	return s
}
func timeLimit(s QuizSettings) string {
	if !s.HasTimeLimit {
		return "none"
	}
	return fmt.Sprintf("%d minutes", s.TimeLimitMinutes)
}

func listHTML(tag string, items []string) string {
	var b strings.Builder
	b.WriteString("<" + tag + ">")
	for _, s := range items {
		b.WriteString("<li>" + s + "</li>")
	}
	return b.String() + "</" + tag + ">"
}

// studentSide is the part of an item students see besides the prompt, from the same fields the payload uses.
func studentSide(it qfItem) string {
	switch it.Type {
	case "MC", "MA":
		var c []string
		for _, ch := range it.Choices {
			c = append(c, ch.Text)
		}
		return listHTML("ul", c)
	case "TF":
		return "<p>True / False</p>"
	case "MATCHING":
		var l, r []string
		for _, p := range it.Pairs {
			l, r = append(l, p.Left), append(r, p.Right)
		}
		for _, d := range it.Distractors {
			r = append(r, d)
		}
		return "<p>Match each:</p>" + listHTML("ul", l) + "<p>with:</p>" + listHTML("ul", r)
	case "ORDERING":
		var items []string
		_ = json.Unmarshal(it.Items, &items)
		return "<p>Put in order:</p>" + listHTML("ul", items)
	case "CATEGORIZATION":
		var items []struct{ Label, Category string }
		_ = json.Unmarshal(it.Items, &items)
		var l []string
		for _, x := range items {
			l = append(l, x.Label)
		}
		return "<p>Sort into: " + html.EscapeString(strings.Join(it.Categories, ", ")) + "</p>" + listHTML("ul", l)
	case "ESSAY":
		return "<p><i>Written response.</i></p>"
	case "FILEUPLOAD":
		return "<p><i>File upload.</i></p>"
	}
	return ""
}

// answerKey is teacher-only plain text; it is never part of the student-side frame.
func answerKey(it qfItem) string {
	switch it.Type {
	case "MC", "MA":
		var c []string
		for i, ch := range it.Choices {
			if ch.Correct {
				c = append(c, fmt.Sprintf("%c. %s", 'A'+i, ch.Text))
			}
		}
		return strings.Join(c, "\n")
	case "MATCHING":
		var c []string
		for _, p := range it.Pairs {
			c = append(c, p.Left+" -> "+p.Right)
		}
		return strings.Join(c, "\n")
	case "ESSAY", "FILEUPLOAD":
		return ""
	}
	parts := []string{}
	for _, raw := range []json.RawMessage{it.Answer, it.Accept, it.Items} {
		if len(raw) > 0 {
			parts = append(parts, string(raw))
		}
	}
	if it.Evaluation.Mode != "" {
		parts = append(parts, fmt.Sprintf("%s %s %s %s", it.Evaluation.Mode, it.Evaluation.Value, it.Evaluation.Min, it.Evaluation.Max))
	}
	return strings.Join(parts, "\n")
}
