package workflow

// QuizForge v3.0-json -> Canvas New Quizzes items. The item shapes, scoring
// algorithms and gotchas below were verified live in the original CanvasExpert
// (api/transform.py there). Stimulus items cannot be created through the API, so
// their HTML is copied into the items that name them by stimulus_id.

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"regexp"
	"strings"
)

var qfEnvelope = regexp.MustCompile(`(?s)<QUIZFORGE_JSON>(.*?)</QUIZFORGE_JSON>`)

type QuizForge struct {
	Version      string `json:"version"`
	Title        string `json:"title"`
	Instructions string `json:"instructions"`
	QuizEngine   string `json:"quiz_engine"`
	// Differentiation "hub" (Classic only) delivers each tier's supports as a
	// restricted Canvas Page linked from instructions via {{tier:LABEL}}.
	Differentiation string                     `json:"differentiation"`
	Tiers           []qfTier                   `json:"tiers"`
	Metadata        map[string]json.RawMessage `json:"metadata"`
	Items           []qfItem                   `json:"items"`
	Rationales      []qfRationale              `json:"rationales"`
}

type qfItem struct {
	ID            string                     `json:"id"`
	Type          string                     `json:"type"`
	Prompt        string                     `json:"prompt"`
	Points        *float64                   `json:"points"`
	StimulusID    string                     `json:"stimulus_id"`
	Choices       []qfChoice                 `json:"choices"`
	Answer        json.RawMessage            `json:"answer"`
	Evaluation    qfEvaluation               `json:"evaluation"`
	Pairs         []qfPair                   `json:"pairs"`
	Distractors   []string                   `json:"distractors"`
	Items         json.RawMessage            `json:"items"` // ORDERING: [string]; CATEGORIZATION: [{label,category}]
	Header        string                     `json:"header"`
	Categories    []string                   `json:"categories"`
	Accept        json.RawMessage            `json:"accept"`
	AnswerMode    string                     `json:"answer_mode"`
	Options       []string                   `json:"options"`
	CaseSensitive bool                       `json:"case_sensitive"`
	FuzzyMatch    bool                       `json:"fuzzy_match"`
	Metadata      map[string]json.RawMessage `json:"metadata"`

	rationale *qfRationale
}

type qfChoice struct {
	ID      string `json:"id"`
	Text    string `json:"text"`
	Correct bool   `json:"correct"`
}
type qfPair struct {
	Left  string `json:"left"`
	Right string `json:"right"`
}
type qfEvaluation struct {
	Mode  string      `json:"mode"`
	Value json.Number `json:"value"`
	Min   json.Number `json:"min"`
	Max   json.Number `json:"max"`
}
type qfRationale struct {
	ItemID    string `json:"item_id"`
	Rationale string `json:"rationale"`
	Choices   []struct {
		ID        string `json:"id"`
		Rationale string `json:"rationale"`
	} `json:"choices"`
}

var qfScoredTypes = map[string]bool{"MC": true, "MA": true, "TF": true, "MATCHING": true, "FITB": true, "ORDERING": true, "CATEGORIZATION": true, "NUMERICAL": true}

// ReadQuizForge reads a QuizForge file (envelope or bare JSON) and validates it
// to the point where the transform cannot fail on shape. Unknown fields are
// ignored, as the contract allows tool-specific keys.
func ReadQuizForge(path string) (QuizForge, error) {
	var q QuizForge
	b, e := os.ReadFile(path)
	if e != nil {
		return q, e
	}
	raw := string(b)
	if m := qfEnvelope.FindStringSubmatch(raw); m != nil {
		raw = m[1]
	}
	if e = json.Unmarshal([]byte(strings.TrimSpace(raw)), &q); e != nil {
		return q, fmt.Errorf("quiz file is not valid QuizForge JSON: %v", e)
	}
	return q, q.validate()
}

func (q *QuizForge) validate() error {
	classic := q.QuizEngine == "classic"
	if q.QuizEngine != "" && q.QuizEngine != "new" && !classic {
		return fmt.Errorf(`quiz_engine must be "new" or "classic" (got %q)`, q.QuizEngine)
	}
	if strings.TrimSpace(q.Title) == "" || len(q.Items) == 0 {
		return fmt.Errorf("quiz needs a title and at least one item")
	}
	if e := q.validateHub(); e != nil {
		return e
	}
	stimuli := map[string]bool{}
	ids := map[string]int{}
	for _, it := range q.Items {
		if it.Type == "STIMULUS" {
			if it.ID == "" {
				return fmt.Errorf("a STIMULUS item needs an id")
			}
			stimuli[it.ID] = true
		}
		if it.ID != "" {
			ids[it.ID]++
		}
	}
	inStimulus := false
	scored := 0
	for i := range q.Items {
		it := &q.Items[i]
		label := fmt.Sprintf("item %d (%s %s)", i+1, it.Type, it.ID)
		switch it.Type {
		case "STIMULUS":
			inStimulus = true
			continue
		case "STIMULUS_END":
			inStimulus = false
			continue
		}
		writing := it.isWriting()
		if writing && !classic {
			return fmt.Errorf("%s: written responses are not New Quiz items; author them as a separate assignment, or set quiz_engine to classic", label)
		}
		if !qfScoredTypes[it.Type] && !writing {
			return fmt.Errorf("%s: unknown type", label)
		}
		if classic {
			if e := it.classicProblem(); e != nil {
				return fmt.Errorf("%s: %v", label, e)
			}
		}
		scored++
		if strings.TrimSpace(it.Prompt) == "" && !(it.Type == "ORDERING" && it.Header != "") {
			return fmt.Errorf("%s needs a prompt", label)
		}
		if it.Points == nil || *it.Points < 0 || math.IsNaN(*it.Points) || math.IsInf(*it.Points, 0) {
			return fmt.Errorf("%s needs an explicit nonnegative points value", label)
		}
		if it.StimulusID != "" && !stimuli[it.StimulusID] {
			return fmt.Errorf("%s names stimulus_id %q, which does not exist", label, it.StimulusID)
		}
		if inStimulus && it.StimulusID == "" {
			return fmt.Errorf("%s sits inside a stimulus but has no stimulus_id, so students would not see the passage; add stimulus_id", label)
		}
		if e := it.validateShape(); e != nil {
			return fmt.Errorf("%s: %v", label, e)
		}
	}
	if scored == 0 {
		return fmt.Errorf("quiz has no scored items")
	}
	seen := map[string]bool{}
	for _, r := range q.Rationales {
		if r.ItemID == "" || ids[r.ItemID] != 1 || seen[r.ItemID] {
			return fmt.Errorf("rationale item_id %q must match exactly one item and appear once", r.ItemID)
		}
		seen[r.ItemID] = true
	}
	for i := range q.Items {
		for j := range q.Rationales {
			if q.Items[i].ID != "" && q.Items[i].ID == q.Rationales[j].ItemID {
				if classic && q.Items[i].isWriting() {
					return fmt.Errorf("item %s takes no rationale; remove its rationales entry", q.Items[i].ID)
				}
				q.Items[i].rationale = &q.Rationales[j]
			}
		}
	}
	return nil
}

func (it *qfItem) validateShape() error {
	switch it.Type {
	case "MC", "MA":
		if len(it.Choices) < 2 {
			return fmt.Errorf("needs at least two choices")
		}
		n := 0
		for i, c := range it.Choices {
			if strings.TrimSpace(c.Text) == "" {
				return fmt.Errorf("choice %d has no text", i+1)
			}
			if c.Correct {
				n++
			}
		}
		if it.Type == "MC" && n != 1 {
			return fmt.Errorf("has %d correct choices (need 1)", n)
		}
		if it.Type == "MA" && n < 2 {
			return fmt.Errorf("has %d correct choices (need at least 2)", n)
		}
	case "TF":
		var v bool
		if json.Unmarshal(it.Answer, &v) != nil {
			return fmt.Errorf("answer must be true or false")
		}
	case "NUMERICAL":
		var n json.Number
		if json.Unmarshal(it.Answer, &n) != nil || n == "" {
			return fmt.Errorf("answer must be a number")
		}
	case "MATCHING":
		if len(it.Pairs) < 2 {
			return fmt.Errorf("needs at least two pairs")
		}
	case "ORDERING":
		var s []string
		if json.Unmarshal(it.Items, &s) != nil || len(s) < 2 {
			return fmt.Errorf("items must list at least two strings in correct order")
		}
	case "CATEGORIZATION":
		var s []struct{ Label, Category string }
		if json.Unmarshal(it.Items, &s) != nil || len(s) == 0 || len(it.Categories) < 2 {
			return fmt.Errorf("needs 2+ categories and items of {label, category}")
		}
		cats := map[string]bool{}
		for _, c := range it.Categories {
			cats[c] = true
		}
		for _, x := range s {
			if !cats[x.Category] {
				return fmt.Errorf("item %q names unknown category %q", x.Label, x.Category)
			}
		}
	case "FITB":
		_, e := it.fitbBlanks()
		return e
	}
	return nil
}

var blankToken = regexp.MustCompile(`\[blank\d*\]`)

// fitbBlanks returns the accepted answers per blank after the shape checks.
func (it *qfItem) fitbBlanks() ([][]string, error) {
	tokens := blankToken.FindAllString(it.Prompt, -1)
	mode := strings.ToLower(it.AnswerMode)
	if mode == "" {
		mode = "open_entry"
	}
	var flat []string
	var nested [][]string
	if json.Unmarshal(it.Accept, &flat) != nil {
		flat = nil
		if json.Unmarshal(it.Accept, &nested) != nil {
			return nil, fmt.Errorf("accept must be a list of answers (or one list per blank)")
		}
	}
	if len(tokens) > 1 {
		if len(tokens) > 3 {
			return nil, fmt.Errorf("multi-blank FITB supports at most 3 blanks")
		}
		if mode != "open_entry" {
			return nil, fmt.Errorf("multi-blank FITB supports open_entry only; use separate single-blank items for %s", mode)
		}
		if len(nested) != len(tokens) {
			return nil, fmt.Errorf("multi-blank FITB needs one non-empty accept array per blank")
		}
		for _, g := range nested {
			if len(g) == 0 {
				return nil, fmt.Errorf("multi-blank FITB needs one non-empty accept array per blank")
			}
		}
		return nested, nil
	}
	if len(tokens) == 1 && flat == nil && len(nested) == 1 {
		flat = nested[0]
	}
	if len(tokens) != 1 || len(flat) == 0 {
		return nil, fmt.Errorf("single-blank FITB needs exactly one [blank] and a non-empty flat accept list")
	}
	if mode == "dropdown" || mode == "wordbank" {
		if len(it.Options) == 0 {
			return nil, fmt.Errorf("%s requires a non-empty options array", mode)
		}
		if _, ok := fitbSelected(it.Options, flat); !ok {
			return nil, fmt.Errorf("%s accept answer must appear in options", mode)
		}
	}
	return [][]string{flat}, nil
}

func fitbSelected(options, accept []string) (int, bool) {
	for i, o := range options {
		for _, a := range accept {
			if strings.EqualFold(strings.TrimSpace(o), strings.TrimSpace(a)) {
				return i, true
			}
		}
	}
	return 0, false
}

func (it qfItem) isWriting() bool { return it.Type == "ESSAY" || it.Type == "FILEUPLOAD" }

// preparedItems returns every item students answer, in order, with its
// stimulus passage inlined and its prompt presented (code blocks, TEKS label).
// Both engines share it; only a Classic file holds written-response items.
func (q *QuizForge) preparedItems() []qfItem {
	stim := map[string]string{}
	for _, it := range q.Items {
		if it.Type == "STIMULUS" {
			stim[it.ID] = it.Prompt
		}
	}
	var out []qfItem
	for _, it := range q.Items {
		if !qfScoredTypes[it.Type] && !(q.QuizEngine == "classic" && it.isWriting()) {
			continue
		}
		if it.StimulusID != "" {
			it.Prompt = stim[it.StimulusID] + it.Prompt
		}
		if it.Prompt != "" {
			it.Prompt = highlightCodeBlocks(it.Prompt) + teksLabel(it.teksCodes())
		}
		out = append(out, it)
	}
	return out
}

// itemPayloads prepares and transforms every scored item in order.
func (q *QuizForge) itemPayloads() []map[string]any {
	var out []map[string]any
	for _, it := range q.preparedItems() {
		pos := len(out) + 1
		p := normalizeStudentText(it.build()).(map[string]any)
		p["item"].(map[string]any)["points_possible"] = *it.Points
		p["item"].(map[string]any)["position"] = pos
		out = append(out, p)
	}
	return out
}

var emDash = regexp.MustCompile(`(?i)\s*(?:\x{2014}|&mdash;|&#8212;|&#x2014;)\s*`)

// normalizeStudentText replaces em dashes in every string a student will see.
func normalizeStudentText(v any) any {
	switch x := v.(type) {
	case string:
		return emDash.ReplaceAllString(x, " - ")
	case []any:
		for i := range x {
			x[i] = normalizeStudentText(x[i])
		}
		return x
	case []string:
		out := make([]any, len(x))
		for i := range x {
			out[i] = normalizeStudentText(x[i])
		}
		return out
	case []map[string]any:
		out := make([]any, len(x))
		for i := range x {
			out[i] = normalizeStudentText(x[i])
		}
		return out
	case map[string]string:
		out := map[string]any{}
		for k, s := range x {
			out[k] = normalizeStudentText(s)
		}
		return out
	case map[string]any:
		for k, s := range x {
			x[k] = normalizeStudentText(s)
		}
		return x
	}
	return v
}

func para(s string) string {
	if strings.HasPrefix(strings.TrimSpace(s), "<") {
		return s
	}
	return "<p>" + s + "</p>"
}

func wrap(entry map[string]any) map[string]any {
	return map[string]any{"item": map[string]any{"entry_type": "Item", "entry": entry}}
}

func (it qfItem) neutralFeedback() map[string]any {
	if it.rationale != nil && it.rationale.Rationale != "" {
		return map[string]any{"neutral": para(it.rationale.Rationale)}
	}
	return map[string]any{}
}

func (it qfItem) title(def string) string {
	if it.ID != "" {
		return it.ID
	}
	return def
}

func (it qfItem) build() map[string]any {
	switch it.Type {
	case "MC", "MA":
		return it.buildChoice()
	case "TF":
		var v bool
		_ = json.Unmarshal(it.Answer, &v)
		return wrap(map[string]any{"title": it.title("TF"), "item_body": it.Prompt, "interaction_type_slug": "true-false",
			"interaction_data": map[string]any{"true_choice": "True", "false_choice": "False"},
			"scoring_data":     map[string]any{"value": v}, "scoring_algorithm": "Equivalence", "feedback": it.neutralFeedback()})
	case "NUMERICAL":
		return it.buildNumeric()
	case "FITB":
		return it.buildFITB()
	case "MATCHING":
		return it.buildMatching()
	case "ORDERING":
		return it.buildOrdering()
	default:
		return it.buildCategorization()
	}
}

func (it qfItem) buildChoice() map[string]any {
	rat := map[string]string{}
	if it.rationale != nil {
		for _, c := range it.rationale.Choices {
			rat[c.ID] = c.Rationale
		}
	}
	choices := make([]map[string]any, len(it.Choices))
	feedback := map[string]any{}
	var correct []string
	for i, c := range it.Choices {
		id := newUUID()
		choices[i] = map[string]any{"id": id, "position": i + 1, "item_body": para(c.Text)}
		key := c.ID
		if key == "" {
			key = string(rune('A' + i))
		}
		if r := rat[key]; r != "" {
			feedback[id] = para(r)
		}
		if c.Correct {
			correct = append(correct, id)
		}
	}
	entry := map[string]any{"title": it.title(it.Type), "item_body": it.Prompt, "interaction_data": map[string]any{"choices": choices}, "answer_feedback": feedback}
	if it.Type == "MC" {
		entry["interaction_type_slug"], entry["scoring_data"], entry["scoring_algorithm"] = "choice", map[string]any{"value": correct[0]}, "Equivalence"
	} else {
		entry["interaction_type_slug"], entry["scoring_data"], entry["scoring_algorithm"] = "multi-answer", map[string]any{"value": correct}, "AllOrNothing"
	}
	return wrap(entry)
}

// Numeric needs scoring_algorithm "Numeric" and a value ARRAY of response
// objects; Equivalence rejects the array and the response would not grade.
func (it qfItem) buildNumeric() map[string]any {
	var n json.Number
	_ = json.Unmarshal(it.Answer, &n)
	ans, ev := n.String(), it.Evaluation
	resp := map[string]any{"id": newUUID(), "type": "exactResponse", "value": ans}
	switch ev.Mode {
	case "range":
		resp = map[string]any{"id": newUUID(), "type": "withinARange", "start": ev.Min.String(), "end": ev.Max.String()}
	case "percent_margin", "absolute_margin":
		mt := "percent"
		if ev.Mode == "absolute_margin" {
			mt = "absolute"
		}
		resp = map[string]any{"id": newUUID(), "type": "marginOfError", "value": ans, "margin": ev.Value.String(), "margin_type": mt}
	case "decimal_places", "significant_digits":
		pt := "decimals"
		if ev.Mode == "significant_digits" {
			pt = "significantDigits"
		}
		resp = map[string]any{"id": newUUID(), "type": "preciseResponse", "value": ans, "precision": ev.Value.String(), "precision_type": pt}
	}
	return wrap(map[string]any{"title": it.title("NUM"), "item_body": it.Prompt, "interaction_type_slug": "numeric",
		"interaction_data": map[string]any{}, "scoring_data": map[string]any{"value": []any{resp}},
		"scoring_algorithm": "Numeric", "feedback": it.neutralFeedback()})
}

func (it qfItem) buildFITB() map[string]any {
	groups, _ := it.fitbBlanks() // validated
	mode := strings.ToLower(it.AnswerMode)
	if mode == "" {
		mode = "open_entry"
	}
	body := blankToken.ReplaceAllString(it.Prompt, "`Blank`")
	// edit_distance must be >= 1 (0 is rejected with HTTP 422).
	closeEnough := func(answer string) map[string]any {
		return map[string]any{"value": answer, "blank_text": answer, "ignore_case": !it.CaseSensitive, "edit_distance": 1}
	}
	var interaction map[string]any
	var values []any
	if len(groups) > 1 {
		blanks := []any{}
		for _, g := range groups {
			id := newUUID()
			blanks = append(blanks, map[string]any{"id": id, "answer_type": "openEntry"})
			values = append(values, map[string]any{"id": id, "scoring_data": closeEnough(g[0]), "scoring_algorithm": "TextCloseEnough"})
		}
		interaction = map[string]any{"blanks": blanks, "word_bank_choices": []any{}, "reuse_word_bank_choices": false}
	} else {
		id, answers := newUUID(), groups[0]
		switch mode {
		case "wordbank":
			bank := make([]map[string]any, len(it.Options))
			for i, o := range it.Options {
				bank[i] = map[string]any{"id": newUUID(), "item_body": o}
			}
			sel, _ := fitbSelected(it.Options, answers)
			interaction = map[string]any{"blanks": []any{map[string]any{"id": id, "choices": nil, "answer_type": "wordbank"}}, "word_bank_choices": bank, "reuse_word_bank_choices": true}
			values = []any{map[string]any{"id": id, "scoring_algorithm": "TextEquivalence",
				"scoring_data": map[string]any{"value": answers[0], "choice_id": bank[sel]["id"], "blank_text": answers[0]}}}
		case "dropdown":
			choices := make([]map[string]any, len(it.Options))
			for i, o := range it.Options {
				choices[i] = map[string]any{"id": newUUID(), "position": i + 1, "item_body": o}
			}
			sel, _ := fitbSelected(it.Options, answers)
			interaction = map[string]any{"blanks": []any{map[string]any{"id": id, "choices": choices, "answer_type": "dropdown"}}}
			values = []any{map[string]any{"id": id, "scoring_algorithm": "Equivalence",
				"scoring_data": map[string]any{"value": choices[sel]["id"], "blank_text": answers[0]}}}
		default:
			interaction = map[string]any{"blanks": []any{map[string]any{"id": id, "answer_type": "openEntry"}}, "word_bank_choices": []any{}, "reuse_word_bank_choices": false}
			values = []any{map[string]any{"id": id, "scoring_algorithm": "TextCloseEnough", "scoring_data": closeEnough(answers[0])}}
		}
	}
	return wrap(map[string]any{"title": it.title("FITB"), "item_body": body, "interaction_type_slug": "rich-fill-blank",
		"interaction_data": interaction, "scoring_data": map[string]any{"value": values, "working_item_body": body},
		"scoring_algorithm": "MultipleMethods", "feedback": it.neutralFeedback()})
}

func (it qfItem) buildMatching() map[string]any {
	answers := make([]string, 0, len(it.Pairs)+len(it.Distractors))
	questions := make([]map[string]any, len(it.Pairs))
	value := map[string]string{}
	matches := make([]map[string]string, len(it.Pairs))
	for i, p := range it.Pairs {
		qid := fmt.Sprint(10001 + i)
		answers = append(answers, p.Right)
		questions[i] = map[string]any{"id": qid, "item_body": p.Left}
		value[qid] = p.Right
		matches[i] = map[string]string{"answer_body": p.Right, "question_id": qid, "question_body": p.Left}
	}
	answers = append(answers, it.Distractors...)
	d := it.Distractors
	if d == nil {
		d = []string{}
	}
	return wrap(map[string]any{"title": it.title("MATCH"), "item_body": it.Prompt, "interaction_type_slug": "matching",
		"interaction_data":  map[string]any{"answers": answers, "questions": questions},
		"scoring_data":      map[string]any{"value": value, "edit_data": map[string]any{"matches": matches, "distractors": d}},
		"scoring_algorithm": "DeepEquals", "feedback": it.neutralFeedback()})
}

func (it qfItem) buildOrdering() map[string]any {
	var list []string
	_ = json.Unmarshal(it.Items, &list)
	choices, order := map[string]any{}, make([]string, len(list))
	for i, t := range list {
		id := newUUID()
		choices[id] = map[string]any{"id": id, "item_body": t} // plain text
		order[i] = id
	}
	body := it.Prompt
	if body == "" {
		body = para(it.Header)
	}
	return wrap(map[string]any{"title": it.title("ORDER"), "item_body": body, "interaction_type_slug": "ordering",
		"interaction_data": map[string]any{"choices": choices}, "scoring_data": map[string]any{"value": order},
		"scoring_algorithm": "DeepEquals", "feedback": it.neutralFeedback()})
}

func (it qfItem) buildCategorization() map[string]any {
	var list []struct{ Label, Category string }
	_ = json.Unmarshal(it.Items, &list)
	cats, order, byLabel := map[string]any{}, []string{}, map[string]string{}
	for _, l := range it.Categories {
		id := newUUID()
		cats[id] = map[string]any{"id": id, "item_body": l}
		order = append(order, id)
		byLabel[l] = id
	}
	distractors, members := map[string]any{}, map[string][]string{}
	for _, x := range list {
		id := newUUID()
		distractors[id] = map[string]any{"id": id, "item_body": x.Label}
		members[byLabel[x.Category]] = append(members[byLabel[x.Category]], id)
	}
	for _, d := range it.Distractors { // unsortable extras
		id := newUUID()
		distractors[id] = map[string]any{"id": id, "item_body": d}
	}
	value := make([]any, len(order))
	for i, c := range order {
		m := members[c]
		if m == nil {
			m = []string{}
		}
		value[i] = map[string]any{"id": c, "scoring_data": map[string]any{"value": m}, "scoring_algorithm": "AllOrNothing"}
	}
	return wrap(map[string]any{"title": it.title("CAT"), "item_body": it.Prompt, "interaction_type_slug": "categorization",
		"interaction_data":  map[string]any{"categories": cats, "distractors": distractors, "category_order": order},
		"scoring_data":      map[string]any{"value": value, "score_method": "all_or_nothing"},
		"scoring_algorithm": "Categorization", "feedback": it.neutralFeedback()})
}

func newUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
