package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"canvasexpertse/internal/canvas"
	"canvasexpertse/internal/domain"
)

// QuizSettings come from the QuizForge file's metadata.canvas object. Unknown
// keys are rejected so a typo does not silently leave a setting off.
type QuizSettings struct {
	ShuffleAnswers       *bool  `json:"shuffle_answers"`
	ShuffleQuestions     *bool  `json:"shuffle_questions"`
	AccessCode           string `json:"access_code"`
	AllowMultipleAttempt bool   `json:"allow_multiple_attempts"`
	ScoreToKeep          string `json:"score_to_keep"`
	AllowedAttempts      any    `json:"allowed_attempts"`
	BuildOnLastAttempt   bool   `json:"build_on_last_attempt"`
	AttemptCooldown      int    `json:"attempt_cooldown"`
	HasTimeLimit         bool   `json:"has_time_limit"`
	TimeLimitMinutes     int    `json:"time_limit_minutes"`
	CalculatorType       string `json:"calculator_type"`
	OneAtATime           bool   `json:"one_at_a_time"`
	AllowBacktracking    *bool  `json:"allow_backtracking"`
	HideResults          bool   `json:"hide_results"`
	DueAt                string `json:"due_at"`
	UnlockAt             string `json:"unlock_at"`
	LockAt               string `json:"lock_at"`
	AssignmentGroupID    string `json:"assignment_group_id"`
	PostToSIS            *bool  `json:"post_to_sis"`
	Published            bool   `json:"published"`
	ModuleID             int64  `json:"module_id"`
	// Audience restricts the quiz's associated assignment to selected club
	// members who work individually. Source IDs never become native group
	// settings. Engine-managed state lives in the sidecar, not this file.
	Audience *domain.Audience `json:"audience,omitempty"`
}

func (q QuizForge) settings() (s QuizSettings, err error) {
	raw, ok := q.Metadata["canvas"]
	if !ok {
		return s, nil
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err = dec.Decode(&s); err != nil {
		return s, fmt.Errorf("metadata.canvas is not valid: %v", err)
	}
	if !s.AllowMultipleAttempt && (s.AllowedAttempts != nil || s.ScoreToKeep != "" || s.BuildOnLastAttempt || s.AttemptCooldown != 0) {
		return s, fmt.Errorf("metadata.canvas: allowed_attempts, score_to_keep, build_on_last_attempt and attempt_cooldown require allow_multiple_attempts: true; for one attempt omit these settings and the enabling flag (see engine_reference formats)")
	}
	if s.AllowMultipleAttempt {
		n, e := s.attemptLimit()
		if e != nil {
			return s, e
		}
		if n == 1 {
			return s, fmt.Errorf("metadata.canvas: allow_multiple_attempts: true contradicts allowed_attempts: 1; for one attempt omit both settings")
		}
		if s.ScoreToKeep != "" && s.ScoreToKeep != "highest" && s.ScoreToKeep != "latest" && s.ScoreToKeep != "average" && s.ScoreToKeep != "first" {
			return s, fmt.Errorf("metadata.canvas.score_to_keep must be highest, latest, average or first")
		}
	}
	if e := domain.ValidateAudience(s.Audience); e != nil {
		return s, fmt.Errorf("metadata.canvas.audience: %w", e)
	}
	return s, nil
}

func (s QuizSettings) attemptLimit() (int, error) {
	if s.AllowedAttempts == nil {
		return -1, nil
	}
	var n int
	switch v := s.AllowedAttempts.(type) {
	case float64:
		if math.Trunc(v) != v || v > float64(math.MaxInt32) {
			break
		}
		n = int(v)
	case string:
		if v == "unlimited" {
			return -1, nil
		}
		var err error
		n, err = strconv.Atoi(v)
		if err != nil {
			n = 0
			break
		}
	}
	if n == -1 || n > 0 {
		return n, nil
	}
	return 0, fmt.Errorf("metadata.canvas.allowed_attempts must be a positive whole number, -1 or \"unlimited\"; see engine_reference formats")
}

func flag(p *bool, def bool) bool {
	if p == nil {
		return def
	}
	return *p
}

// quizSettings maps push settings to New Quizzes quiz_settings. Result viewing
// must be spelled out: unset or empty shows students no feedback at all.
func (s QuizSettings) quizSettings() map[string]any {
	qs := map[string]any{"shuffle_answers": flag(s.ShuffleAnswers, true), "shuffle_questions": flag(s.ShuffleQuestions, false)}
	if s.AccessCode != "" {
		qs["require_student_access_code"], qs["student_access_code"] = true, strings.TrimSpace(s.AccessCode)
	}
	if s.AllowMultipleAttempt {
		keep := s.ScoreToKeep
		if keep != "highest" && keep != "latest" && keep != "average" && keep != "first" {
			keep = "highest"
		}
		a := map[string]any{"multiple_attempts_enabled": true, "score_to_keep": keep, "build_on_last_attempt": s.BuildOnLastAttempt}
		n, _ := s.attemptLimit() // settings() validated the authored input.
		if n > 0 {
			a["attempt_limit"], a["max_attempts"] = true, n
		}
		if s.AttemptCooldown > 0 {
			a["cooling_period"], a["cooling_period_seconds"] = true, s.AttemptCooldown*60
		}
		qs["multiple_attempts"] = a
	}
	if s.HasTimeLimit && s.TimeLimitMinutes > 0 {
		qs["has_time_limit"], qs["session_time_limit_in_seconds"] = true, s.TimeLimitMinutes*60
	}
	if s.CalculatorType == "basic" || s.CalculatorType == "scientific" {
		qs["calculator_type"] = s.CalculatorType
	}
	if s.OneAtATime {
		qs["one_at_a_time_type"], qs["allow_backtracking"] = "question", flag(s.AllowBacktracking, true)
	}
	if s.HideResults {
		qs["result_view_settings"] = map[string]any{"result_view_restricted": true, "display_points_awarded": false, "display_points_possible": false, "display_items": false}
	} else {
		qs["result_view_settings"] = map[string]any{"result_view_restricted": true, "display_points_awarded": true, "display_points_possible": true,
			"display_items": true, "display_item_response": true, "display_item_response_correctness": true,
			"display_item_response_qualifier": "after_last_attempt", "display_item_correct_answer": true, "display_item_feedback": true}
	}
	return qs
}

type QuizPushReport struct {
	Status       string // completed or partial
	AssignmentID int64
	URL          string
	ItemsAdded   int
	ItemsTotal   int
	Published    bool
	Message      string
	Hub          string // hub tier pages, tags, and teacher actions
	Audience     string // applied individual club audience, when requested
	Path         string // existing sidecar containing the created quiz ID and resume data
	Warnings     []string
}

// quizSidecar is the resume record beside the quiz file. A New Quiz records its
// ID as assignment_id. Older sidecars without the newer fields remain usable.
type quizSidecar struct {
	AssignmentID int64 `json:"assignment_id,omitempty"`
	// TierPages records hub pages as soon as each exists, so a rerun never duplicates them.
	TierPages []TierPage `json:"tier_pages,omitempty"`
	// Instance and CourseID bind the saved quiz to one Canvas/course so a
	// copied sidecar cannot target a different deployment.
	Instance string `json:"instance,omitempty"`
	CourseID int64  `json:"course_id,omitempty"`
	// Audience is the Engine-managed individual audience for the quiz's
	// associated assignment. It is never read from the QuizForge file.
	Audience *domain.Audience `json:"audience,omitempty"`
	// Stages records accepted restricted-quiz work so a resume repeats only
	// unfinished stages. Absent for ordinary quizzes.
	Stages *quizStages `json:"stages,omitempty"`
}

// quizStages records the accepted stages of a restricted quiz. Each outcome is
// definite only after Canvas accepted that stage.
type quizStages struct {
	Items       domain.ActionOutcome `json:"items,omitempty"`
	Settings    domain.ActionOutcome `json:"settings,omitempty"`
	Audience    domain.ActionOutcome `json:"audience,omitempty"`
	Module      domain.ActionOutcome `json:"module,omitempty"`
	Publication domain.ActionOutcome `json:"publication,omitempty"`
}

func writeSidecar(path string, rec quizSidecar) error {
	b, _ := json.Marshal(rec)
	return os.WriteFile(sidecarPath(path), b, 0o644)
}

func sidecarPath(path string) string { return path + ".cse.json" }

func readSidecar(path string) quizSidecar {
	var s quizSidecar
	if b, e := os.ReadFile(sidecarPath(path)); e == nil && json.Unmarshal(b, &s) == nil {
		return s
	}
	return quizSidecar{}
}

// PushQuiz creates a New Quiz from a QuizForge file in the named course, or
// resumes the quiz recorded beside the file. The quiz ID is saved immediately
// after creation, so an interrupted push never duplicates the quiz or items
// already in Canvas. Publication, dates, assignment group and module placement
// come last, only after every item is in, and only when requested in
// metadata.canvas; otherwise the quiz stays unpublished.
func (s *Service) PushQuiz(ctx context.Context, nickname, path string) (report QuizPushReport, err error) {
	q, e := ReadQuizForge(path)
	if e != nil {
		return report, e
	}
	report.Warnings = q.Warnings
	set, e := q.settings()
	if e != nil {
		return report, e
	}
	course, e := s.Course(nickname)
	if e != nil {
		return report, e
	}
	items := q.itemPayloads()
	report.ItemsTotal = len(items)
	base := fmt.Sprintf("/api/quiz/v1/courses/%d/quizzes", course.ID)
	rec := readSidecar(path)
	restricted := set.Audience != nil || rec.Audience != nil
	if rec.Instance != "" && (rec.Instance != s.Client.Origin() || rec.CourseID != 0 && rec.CourseID != course.ID) {
		return report, fmt.Errorf("saved quiz sidecar belongs to a different Canvas/course; it was not reused")
	}
	if restricted {
		if rec.Instance == "" {
			rec.Instance, rec.CourseID = s.Client.Origin(), course.ID
		}
		if rec.Stages == nil {
			rec.Stages = &quizStages{}
		}
		if rec.Audience == nil {
			rec.Audience = set.Audience
		}
		if err := writeSidecar(path, rec); err != nil {
			return report, err
		}
	}
	id, existing := rec.AssignmentID, 0
	if id == 0 {
		var quizzes []struct {
			ID    json.Number `json:"id"`
			Title string      `json:"title"`
		}
		if e = s.Client.List(ctx, base, &quizzes); e != nil {
			return report, e
		}
		title := normalizeStudentText(strings.TrimSpace(q.Title)).(string)
		for _, x := range quizzes {
			if x.Title == title {
				return report, fmt.Errorf("a New Quiz titled %q already exists in this course (id %s); rename this quiz, or if that is an unfinished push of this file, write {\"assignment_id\": %s} to %s", title, x.ID, x.ID, sidecarPath(path))
			}
		}
		quiz := map[string]any{"title": title, "grading_type": "points", "quiz_settings": set.quizSettings()}
		total := 0.0
		for _, it := range items {
			total += it["item"].(map[string]any)["points_possible"].(float64)
		}
		quiz["points_possible"] = total
		instructions := q.Instructions
		if len(q.Tiers) > 0 {
			pages, e := s.deliverQuizHub(ctx, q, course.ID, path, false)
			report.Hub = HubSummary(pages)
			if e != nil {
				return report, fmt.Errorf("tier pages were not completed, so the quiz was not created: %w", e)
			}
			if instructions, e = BindTierLinks(instructions, pages, course.ID); e != nil {
				return report, e
			}
		}
		if instructions != "" {
			quiz["instructions"] = normalizeStudentText(instructions)
		}
		var created struct {
			ID json.Number `json:"id"` // Canvas returns the ID as a string
		}
		if e = s.Client.Write(ctx, "POST", base, map[string]any{"quiz": quiz}, &created); e != nil {
			return report, e
		}
		id, _ = created.ID.Int64()
		if id <= 0 {
			return report, &canvas.Error{Kind: "uncertain", Uncertain: true}
		}
		rec = readSidecar(path) // hub delivery may have recorded tier pages
		rec.AssignmentID = id
		if e = writeSidecar(path, rec); e != nil {
			report.AssignmentID = id
			return report, fmt.Errorf("quiz %d was created but its ID could not be saved; write {\"assignment_id\": %d} to %s before rerunning", id, id, sidecarPath(path))
		}
	} else {
		var have []json.RawMessage
		if e = s.Client.List(ctx, fmt.Sprintf("%s/%d/items", base, id), &have); e != nil {
			return report, e
		}
		existing = len(have)
	}
	report.Path = sidecarPath(path)
	report.AssignmentID = id
	report.URL = fmt.Sprintf("%s/courses/%d/assignments/%d", s.Client.Origin(), course.ID, id)
	report.ItemsAdded = existing
	labels := q.scoredIDs()
	// A restricted quiz resumes only unfinished stages; accepted items are
	// never re-posted.
	itemsAccepted := restricted && rec.Stages != nil && definite(rec.Stages.Items)
	if itemsAccepted {
		report.ItemsAdded = len(items)
	}
	for i := existing; i < len(items) && !itemsAccepted; i++ {
		if e = s.Client.Write(ctx, "POST", fmt.Sprintf("%s/%d/items", base, id), items[i], nil); e != nil {
			report.Status = "partial"
			report.Message = fmt.Sprintf("item %d of %d (%s) was not added; rerun the same file to continue this quiz", i+1, len(items), labels[i])
			return report, e
		}
		report.ItemsAdded++
	}
	if restricted && rec.Stages != nil && !definite(rec.Stages.Items) {
		rec.Stages.Items = domain.ActionOutcome{Status: "succeeded"}
		if e = writeSidecar(path, rec); e != nil {
			return report, e
		}
	}
	if len(q.Tiers) > 0 {
		// Pages are verified again and published only when the quiz itself publishes.
		pages, e := s.deliverQuizHub(ctx, q, course.ID, path, set.Published)
		report.Hub = HubSummary(pages)
		if e != nil {
			report.Status = "partial"
			report.Message = "items are in, but the tier pages were not completed; rerun the same file"
			return report, e
		}
	}
	if complete, e := s.finishQuiz(ctx, course, id, set, &report, path); e != nil {
		report.Status = "partial"
		return report, e
	} else if !complete {
		report.Status = "partial"
		return report, nil
	}
	report.Status = "completed"
	return report, nil
}

func (q QuizForge) scoredIDs() []string {
	var out []string
	for _, it := range q.preparedItems() {
		label := it.Type
		if it.ID != "" {
			label = it.ID
		}
		out = append(out, label)
	}
	return out
}

// finishQuiz applies assignment-level settings, the individual audience and
// module placement once the items are safe. Each step is idempotent, so a rerun
// repeats it harmlessly. A restricted quiz records accepted stages in the
// sidecar and resumes only unfinished work. It reports whether every requested
// stage is complete; an incomplete audience leaves the quiz unpublished.
func (s *Service) finishQuiz(ctx context.Context, course domain.Course, id int64, set QuizSettings, report *QuizPushReport, path string) (bool, error) {
	courseID := course.ID
	rec := readSidecar(path)
	restricted := set.Audience != nil || rec.Audience != nil
	var stages *quizStages
	if restricted {
		if rec.Stages == nil {
			rec.Stages = &quizStages{}
		}
		stages = rec.Stages
	}
	save := func() error {
		if stages != nil {
			rec.Stages = stages
		}
		return writeSidecar(path, rec)
	}
	fields := map[string]any{}
	for k, v := range map[string]string{"due_at": set.DueAt, "unlock_at": set.UnlockAt, "lock_at": set.LockAt, "assignment_group_id": set.AssignmentGroupID} {
		if v != "" {
			fields[k] = v
		}
	}
	if set.PostToSIS != nil {
		fields["post_to_sis"] = *set.PostToSIS
	}
	settingsDone := stages != nil && definite(stages.Settings)
	if len(fields) > 0 && !settingsDone {
		var a Assignment
		if e := s.Client.Write(ctx, "PUT", fmt.Sprintf("/api/v1/courses/%d/assignments/%d", courseID, id), map[string]any{"assignment": fields}, &a); e != nil {
			report.Message = "items are in, but assignment settings (dates, group) were not applied; rerun the same file"
			return false, e
		}
		report.Published = a.Published
	}
	if stages != nil && !definite(stages.Settings) {
		stages.Settings = domain.ActionOutcome{Status: "succeeded"}
		if e := save(); e != nil {
			return false, e
		}
	}
	if set.ModuleID > 0 {
		moduleDone := stages != nil && definite(stages.Module)
		if !moduleDone {
			var mods []struct {
				ContentID int64  `json:"content_id"`
				Type      string `json:"type"`
			}
			path := fmt.Sprintf("/api/v1/courses/%d/modules/%d/items", courseID, set.ModuleID)
			if e := s.Client.List(ctx, path+"?per_page=100", &mods); e != nil {
				report.Message = "items are in, but module placement could not be checked; rerun the same file"
				return false, e
			}
			placed := false
			for _, m := range mods {
				if m.Type == "Assignment" && m.ContentID == id {
					placed = true
					break
				}
			}
			if !placed {
				if e := s.Client.Write(ctx, "POST", path, map[string]any{"module_item": map[string]any{"type": "Assignment", "content_id": id}}, nil); e != nil {
					report.Message = "items are in, but module placement failed; rerun the same file"
					return false, e
				}
			}
		}
		if stages != nil && !definite(stages.Module) {
			stages.Module = domain.ActionOutcome{Status: "succeeded"}
			if e := save(); e != nil {
				return false, e
			}
		}
	}
	// Restrict the associated assignment before publication. A failed or
	// uncertain audience stage must never publish the quiz.
	if restricted {
		// The prepared state lives in the sidecar; the document carries intent only.
		audience := rec.Audience
		if audience == nil {
			audience = set.Audience
		}
		complete, e := s.applyQuizAudience(ctx, course, path, audience, id, stages)
		if e != nil {
			report.Message = "items are in, but the individual audience was not applied; the quiz remains unpublished"
			return false, e
		}
		if !complete {
			report.Message = "items are in, but the individual audience is incomplete; the quiz remains unpublished"
			return false, nil
		}
		report.Audience = quizAudienceSummary(audience)
	}
	if set.Published && (stages == nil || !definite(stages.Publication)) {
		var a Assignment
		if e := s.Client.Write(ctx, "PUT", fmt.Sprintf("/api/v1/courses/%d/assignments/%d", courseID, id), map[string]any{"assignment": map[string]any{"published": true}}, &a); e != nil {
			report.Message = "items and requested module placement are in, but publication failed; rerun the same file"
			return false, e
		}
		report.Published = a.Published
		if stages != nil {
			stages.Publication = domain.ActionOutcome{Status: "succeeded"}
			if e := save(); e != nil {
				return false, e
			}
		}
	}
	return true, nil
}
