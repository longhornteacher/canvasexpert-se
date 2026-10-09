package workflow

// Classic Quiz authoring: a QuizForge file with "quiz_engine": "classic" becomes
// a Classic Quiz through the Canvas REST API. Question shapes, the points save,
// publishing and the stale question_count were verified live in the original
// CanvasExpert (docs/reference/classic-quiz-design.md there). The resume model
// mirrors the New Quiz path: the IDs are saved beside the file straight after
// creation, only missing questions are added, and points, settings,
// publication and module placement come last.

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"canvasexpertse/internal/canvas"
)

var (
	classicBlank = regexp.MustCompile(`\[(blank\d*)\]`)
	htmlTag      = regexp.MustCompile(`<[a-zA-Z/]`)
)

// classicProblem refuses what a Classic Quiz cannot hold, in one sentence.
func (it qfItem) classicProblem() error {
	switch it.Type {
	case "ORDERING", "CATEGORIZATION":
		return fmt.Errorf("cannot be a Classic Quiz question; use MATCHING or MC instead")
	case "FITB":
		mode := strings.ToLower(it.AnswerMode)
		switch {
		case mode == "wordbank":
			return fmt.Errorf("cannot use a word bank in a Classic Quiz; use answer_mode dropdown")
		case it.FuzzyMatch:
			return fmt.Errorf("cannot use fuzzy_match in a Classic Quiz")
		case it.CaseSensitive:
			return fmt.Errorf("cannot be case_sensitive in a Classic Quiz")
		}
		seen := map[string]bool{}
		for _, m := range classicBlank.FindAllStringSubmatch(it.Prompt, -1) {
			if seen[m[1]] {
				return fmt.Errorf("needs uniquely numbered blanks, such as [blank1] and [blank2]")
			}
			seen[m[1]] = true
		}
	case "NUMERICAL":
		if m := it.Evaluation.Mode; m == "percent_margin" || m == "decimal_places" {
			return fmt.Errorf("cannot use %s in a Classic Quiz; use exact, absolute_margin, range, or significant_digits", m)
		}
	}
	return nil
}

// classicSettingProblem refuses settings Classic Quizzes cannot honor.
func (s QuizSettings) classicSettingProblem() error {
	switch {
	case s.CalculatorType != "":
		return fmt.Errorf("calculator_type has no Classic Quiz equivalent; remove it")
	case flag(s.ShuffleQuestions, false):
		return fmt.Errorf("shuffle_questions has no Classic Quiz equivalent; remove it")
	case s.BuildOnLastAttempt:
		return fmt.Errorf("build_on_last_attempt has no Classic Quiz equivalent; remove it")
	case s.AttemptCooldown > 0:
		return fmt.Errorf("attempt_cooldown has no Classic Quiz equivalent; remove it")
	case s.ScoreToKeep == "first":
		return fmt.Errorf(`score_to_keep "first" has no Classic Quiz equivalent; use highest, latest, or average`)
	}
	return nil
}

// classicFields maps push settings to Classic Quiz fields. Unlike New Quizzes,
// they are applied after the questions are in, so a half-built quiz keeps
// Canvas defaults and is never published.
func (s QuizSettings) classicFields() map[string]any {
	attempts := 1
	if s.AllowMultipleAttempt {
		attempts = -1
		switch v := s.AllowedAttempts.(type) {
		case float64:
			attempts = int(v)
		case string:
			if v != "-1" && v != "unlimited" {
				fmt.Sscan(v, &attempts)
			}
		}
	}
	policy := map[string]string{"highest": "keep_highest", "latest": "keep_latest", "average": "keep_average"}[s.ScoreToKeep]
	if policy == "" {
		policy = "keep_highest"
	}
	f := map[string]any{
		"shuffle_answers": flag(s.ShuffleAnswers, true), "allowed_attempts": attempts, "scoring_policy": policy,
		"one_question_at_a_time": s.OneAtATime, "cant_go_back": s.OneAtATime && !flag(s.AllowBacktracking, true),
		"show_correct_answers": !s.HideResults,
	}
	if s.HasTimeLimit && s.TimeLimitMinutes > 0 {
		f["time_limit"] = s.TimeLimitMinutes
	}
	if s.AccessCode != "" {
		f["access_code"] = strings.TrimSpace(s.AccessCode)
	}
	if s.HideResults {
		f["hide_results"] = "always"
	}
	for k, v := range map[string]string{"due_at": s.DueAt, "unlock_at": s.UnlockAt, "lock_at": s.LockAt, "assignment_group_id": s.AssignmentGroupID} {
		if v != "" {
			f[k] = v
		}
	}
	if s.Published {
		f["published"] = true
	}
	return f
}

// classicQuestions transforms every item students answer into a Classic
// question payload, in order, with points set from the file.
func (q *QuizForge) classicQuestions() []map[string]any {
	var out []map[string]any
	for _, it := range q.preparedItems() {
		p := normalizeStudentText(it.buildClassic()).(map[string]any)
		p["points_possible"] = *it.Points
		out = append(out, map[string]any{"question": p})
	}
	return out
}

func (it qfItem) buildClassic() map[string]any {
	qtype := map[string]string{"MC": "multiple_choice_question", "MA": "multiple_answers_question", "TF": "true_false_question",
		"MATCHING": "matching_question", "FITB": "short_answer_question", "NUMERICAL": "numerical_question",
		"ESSAY": "essay_question", "FILEUPLOAD": "file_upload_question"}[it.Type]
	q := map[string]any{"question_name": it.title(it.Type), "question_text": it.Prompt, "question_type": qtype}
	if it.rationale != nil && it.rationale.Rationale != "" && it.Type != "MC" && it.Type != "MA" {
		q["neutral_comments_html"] = para(it.rationale.Rationale)
	}
	body := func(text string) map[string]any {
		if htmlTag.MatchString(text) {
			return map[string]any{"answer_html": text}
		}
		return map[string]any{"answer_text": text}
	}
	weight := func(ok bool) int {
		if ok {
			return 100
		}
		return 0
	}
	switch it.Type {
	case "MC", "MA":
		rat := map[string]string{}
		if it.rationale != nil {
			for _, c := range it.rationale.Choices {
				rat[c.ID] = c.Rationale
			}
		}
		answers := make([]map[string]any, len(it.Choices))
		for i, c := range it.Choices {
			a := body(c.Text)
			a["answer_weight"] = weight(c.Correct)
			key := c.ID
			if key == "" {
				key = string(rune('A' + i))
			}
			if r := rat[key]; r != "" {
				a["answer_comment_html"] = para(r)
			}
			answers[i] = a
		}
		q["answers"] = answers
	case "TF":
		var v bool
		_ = json.Unmarshal(it.Answer, &v)
		q["answers"] = []map[string]any{{"answer_text": "True", "answer_weight": weight(v)}, {"answer_text": "False", "answer_weight": weight(!v)}}
	case "MATCHING":
		answers := make([]map[string]any, len(it.Pairs))
		for i, p := range it.Pairs {
			answers[i] = map[string]any{"answer_match_left": p.Left, "answer_match_right": p.Right}
		}
		q["answers"] = answers
		if len(it.Distractors) > 0 {
			q["matching_answer_incorrect_matches"] = strings.Join(it.Distractors, "\n")
		}
	case "NUMERICAL":
		var n json.Number
		_ = json.Unmarshal(it.Answer, &n)
		ev, row := it.Evaluation, map[string]any{"answer_weight": 100}
		switch ev.Mode {
		case "range":
			row["numerical_answer_type"], row["answer_range_start"], row["answer_range_end"] = "range_answer", ev.Min, ev.Max
		case "significant_digits":
			row["numerical_answer_type"], row["answer_approximate"], row["answer_precision"] = "precision_answer", n, ev.Value
		default:
			margin := json.Number("0")
			if ev.Mode == "absolute_margin" {
				margin = ev.Value
			}
			row["numerical_answer_type"], row["answer_exact"], row["answer_error_margin"] = "exact_answer", n, margin
		}
		q["answers"] = []map[string]any{row}
	case "FITB":
		groups, _ := it.fitbBlanks() // validated
		tokens := classicBlank.FindAllStringSubmatch(it.Prompt, -1)
		var answers []map[string]any
		switch {
		case len(groups) > 1:
			q["question_type"] = "fill_in_multiple_blanks_question"
			for i, g := range groups {
				for _, v := range g {
					answers = append(answers, map[string]any{"answer_text": v, "answer_weight": 100, "blank_id": tokens[i][1]})
				}
			}
		case strings.ToLower(it.AnswerMode) == "dropdown":
			q["question_type"] = "multiple_dropdowns_question"
			sel, _ := fitbSelected(it.Options, groups[0])
			for i, o := range it.Options {
				answers = append(answers, map[string]any{"answer_text": o, "answer_weight": weight(i == sel), "blank_id": tokens[0][1]})
			}
		default:
			// A short-answer question shows its own answer box, so the blank token becomes a rule.
			q["question_text"] = classicBlank.ReplaceAllString(it.Prompt, "_____")
			for _, v := range groups[0] {
				answers = append(answers, map[string]any{"answer_text": v, "answer_weight": 100})
			}
		}
		q["answers"] = answers
	}
	return q
}

type classicIDs struct{ quiz, assignment int64 }

func (s *Service) pushClassicQuiz(ctx context.Context, q QuizForge, set QuizSettings, courseID int64, path string) (report QuizPushReport, err error) {
	if e := set.classicSettingProblem(); e != nil {
		return report, e
	}
	questions := q.classicQuestions()
	total := 0.0
	for _, x := range questions {
		total += x["question"].(map[string]any)["points_possible"].(float64)
	}
	report.Engine, report.ItemsTotal = "classic", len(questions)
	base := fmt.Sprintf("/api/v1/courses/%d/quizzes", courseID)
	rec := readSidecar(path)
	var ids classicIDs
	existing := 0
	if rec.QuizID == 0 && rec.AssignmentID == 0 {
		var quizzes []struct {
			ID           json.Number `json:"id"`
			AssignmentID json.Number `json:"assignment_id"`
			Title        string      `json:"title"`
		}
		if e := s.Client.List(ctx, base+"?per_page=100", &quizzes); e != nil {
			return report, e
		}
		title := normalizeStudentText(strings.TrimSpace(q.Title)).(string)
		for _, x := range quizzes {
			if x.Title == title {
				return report, fmt.Errorf("a Classic Quiz titled %q already exists in this course (id %s); rename this quiz, or if that is an unfinished push of this file, write {\"quiz_id\": %s, \"assignment_id\": %s} to %s", title, x.ID, x.ID, x.AssignmentID, sidecarPath(path))
			}
		}
		instructions := q.Instructions
		if len(q.Tiers) > 0 {
			pages, e := s.deliverQuizHub(ctx, q, courseID, path, false)
			report.Hub = HubSummary(pages)
			if e != nil {
				return report, fmt.Errorf("tier pages were not completed, so the quiz was not created: %w", e)
			}
			if instructions, e = BindTierLinks(instructions, pages, courseID); e != nil {
				return report, e
			}
		}
		fields := map[string]any{"title": title, "quiz_type": "assignment", "published": false}
		if instructions != "" {
			fields["description"] = normalizeStudentText(instructions)
		}
		var created struct {
			ID           json.Number `json:"id"`
			AssignmentID json.Number `json:"assignment_id"`
		}
		if e := s.Client.Write(ctx, "POST", base, map[string]any{"quiz": fields}, &created); e != nil {
			return report, e
		}
		ids.quiz, _ = created.ID.Int64()
		ids.assignment, _ = created.AssignmentID.Int64()
		if ids.quiz <= 0 {
			return report, &canvas.Error{Kind: "uncertain", Uncertain: true}
		}
		rec = readSidecar(path) // hub delivery may have recorded tier pages since the first read
		rec.AssignmentID, rec.QuizID = ids.assignment, ids.quiz
		report.QuizID, report.AssignmentID = ids.quiz, ids.assignment
		if e := writeSidecar(path, rec); e != nil {
			b, _ := json.Marshal(quizSidecar{AssignmentID: ids.assignment, QuizID: ids.quiz})
			return report, fmt.Errorf("quiz %d was created but its IDs could not be saved; add %s to %s before rerunning", ids.quiz, b, sidecarPath(path))
		}
	} else {
		if rec.QuizID == 0 {
			return report, fmt.Errorf("%s has no quiz_id, so it cannot resume a Classic Quiz; write {\"quiz_id\": N, \"assignment_id\": M} or delete the file", sidecarPath(path))
		}
		ids = classicIDs{rec.QuizID, rec.AssignmentID}
		// A deleted quiz still reads 200; its assignment does not.
		if ids.assignment > 0 {
			var a struct{}
			if e := s.Client.Get(ctx, fmt.Sprintf("/api/v1/courses/%d/assignments/%d", courseID, ids.assignment), &a); e != nil {
				return report, fmt.Errorf("could not confirm the quiz recorded in %s still exists (if it was deleted, delete that file to start over): %w", sidecarPath(path), e)
			}
		}
		var have []json.RawMessage
		if e := s.Client.List(ctx, fmt.Sprintf("%s/%d/questions?per_page=100", base, ids.quiz), &have); e != nil {
			return report, e
		}
		if existing = len(have); existing > len(questions) {
			return report, fmt.Errorf("the recorded quiz already has %d questions but this file has %d; the file changed, so delete %s to start over", existing, len(questions), sidecarPath(path))
		}
	}
	report.QuizID, report.AssignmentID = ids.quiz, ids.assignment
	report.URL = fmt.Sprintf("%s/courses/%d/quizzes/%d", s.Client.Origin(), courseID, ids.quiz)
	report.ItemsAdded = existing
	labels := q.preparedLabels()
	for i := existing; i < len(questions); i++ {
		if e := s.Client.Write(ctx, "POST", fmt.Sprintf("%s/%d/questions", base, ids.quiz), questions[i], nil); e != nil {
			report.Status = "partial"
			report.Message = fmt.Sprintf("question %d of %d (%s) was not added; rerun the same file to continue this quiz", i+1, len(questions), labels[i])
			return report, e
		}
		report.ItemsAdded++
	}
	if e := s.finishClassic(ctx, courseID, ids, set, total, &report, q, path); e != nil {
		report.Status = "partial"
		return report, e
	}
	report.Status = "completed"
	return report, nil
}

func (q *QuizForge) preparedLabels() []string {
	var out []string
	for _, it := range q.preparedItems() {
		out = append(out, it.title(it.Type))
	}
	return out
}

// finishClassic runs once every question is in. Each step is idempotent, so a
// rerun repeats it harmlessly. The PUT also makes Canvas save the points total,
// which stays null until the quiz is saved after its questions are added.
// A hub step that appends links to the description belongs here, before the
// PUT that may publish.
func (s *Service) finishClassic(ctx context.Context, courseID int64, ids classicIDs, set QuizSettings, total float64, report *QuizPushReport, q QuizForge, path string) error {
	// Hub pages are verified again and published only now that every question is in
	// and only when the quiz itself is being published.
	if len(q.Tiers) > 0 {
		pages, e := s.deliverQuizHub(ctx, q, courseID, path, set.Published)
		report.Hub = HubSummary(pages)
		if e != nil {
			report.Message = "questions are in, but the tier pages were not completed; rerun the same file"
			return e
		}
	}
	var saved struct {
		Points    *float64 `json:"points_possible"`
		Published bool     `json:"published"`
	}
	quizPath := fmt.Sprintf("/api/v1/courses/%d/quizzes/%d", courseID, ids.quiz)
	if e := s.Client.Write(ctx, "PUT", quizPath, map[string]any{"quiz": set.classicFields()}, &saved); e != nil {
		report.Message = "questions are in, but quiz settings and publication were not applied; rerun the same file"
		return e
	}
	report.Published = saved.Published
	if saved.Points != nil {
		report.Points = *saved.Points
	}
	if set.PostToSIS != nil && ids.assignment > 0 {
		if e := s.Client.Write(ctx, "PUT", fmt.Sprintf("/api/v1/courses/%d/assignments/%d", courseID, ids.assignment), map[string]any{"assignment": map[string]any{"post_to_sis": *set.PostToSIS}}, nil); e != nil {
			report.Message = "questions are in, but post_to_sis was not applied; rerun the same file"
			return e
		}
	}
	if report.Points != total {
		report.Message = fmt.Sprintf("Canvas reports %v points but the file totals %v; check the question points in Canvas", report.Points, total)
	}
	if set.ModuleID > 0 {
		var mods []struct {
			ContentID int64  `json:"content_id"`
			Type      string `json:"type"`
		}
		path := fmt.Sprintf("/api/v1/courses/%d/modules/%d/items", courseID, set.ModuleID)
		if e := s.Client.List(ctx, path+"?per_page=100", &mods); e != nil {
			return e
		}
		for _, m := range mods {
			if m.Type == "Quiz" && m.ContentID == ids.quiz {
				return nil
			}
		}
		if e := s.Client.Write(ctx, "POST", path, map[string]any{"module_item": map[string]any{"type": "Quiz", "content_id": ids.quiz}}, nil); e != nil {
			report.Message = "questions are in, but module placement failed; rerun the same file"
			return e
		}
	}
	return nil
}
