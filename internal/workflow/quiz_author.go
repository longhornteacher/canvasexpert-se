package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"canvasexpertse/internal/canvas"
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
	return s, nil
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
		n := -1
		switch v := s.AllowedAttempts.(type) {
		case float64:
			n = int(v)
		case string:
			if v != "-1" && v != "unlimited" {
				fmt.Sscan(v, &n)
			}
		}
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
	Engine       string  // "classic" for a Classic Quiz; empty for a New Quiz
	QuizID       int64   // Classic only; differs from AssignmentID
	Points       float64 // Classic only: points total Canvas saved
	Hub          string  // hub tier pages, tags, and teacher actions
}

// quizSidecar is the resume record beside the quiz file. A New Quiz records its
// ID as assignment_id; a Classic Quiz records both its quiz_id and the
// assignment_id Canvas gives it.
type quizSidecar struct {
	AssignmentID int64 `json:"assignment_id,omitempty"`
	QuizID       int64 `json:"quiz_id,omitempty"`
	// TierPages records hub pages as soon as each exists, so a rerun never duplicates them.
	TierPages []TierPage `json:"tier_pages,omitempty"`
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
	set, e := q.settings()
	if e != nil {
		return report, e
	}
	course, e := s.Course(nickname)
	if e != nil {
		return report, e
	}
	if q.QuizEngine == "classic" {
		return s.pushClassicQuiz(ctx, q, set, course.ID, path)
	}
	items := q.itemPayloads()
	report.ItemsTotal = len(items)
	base := fmt.Sprintf("/api/quiz/v1/courses/%d/quizzes", course.ID)
	rec := readSidecar(path)
	if rec.QuizID != 0 {
		return report, fmt.Errorf("%s records a Classic Quiz, but this file is not quiz_engine classic; set quiz_engine back to classic, or delete %s to author a New Quiz", sidecarPath(path), sidecarPath(path))
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
		if q.Instructions != "" {
			quiz["instructions"] = normalizeStudentText(q.Instructions)
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
		b, _ := json.Marshal(quizSidecar{AssignmentID: id})
		if e = os.WriteFile(sidecarPath(path), b, 0o644); e != nil {
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
	report.AssignmentID = id
	report.URL = fmt.Sprintf("%s/courses/%d/assignments/%d", s.Client.Origin(), course.ID, id)
	report.ItemsAdded = existing
	labels := q.scoredIDs()
	for i := existing; i < len(items); i++ {
		if e = s.Client.Write(ctx, "POST", fmt.Sprintf("%s/%d/items", base, id), items[i], nil); e != nil {
			report.Status = "partial"
			report.Message = fmt.Sprintf("item %d of %d (%s) was not added; rerun the same file to continue this quiz", i+1, len(items), labels[i])
			return report, e
		}
		report.ItemsAdded++
	}
	if e = s.finishQuiz(ctx, course.ID, id, set, &report); e != nil {
		report.Status = "partial"
		return report, e
	}
	report.Status = "completed"
	return report, nil
}

func (q QuizForge) scoredIDs() []string {
	var out []string
	for _, it := range q.Items {
		if qfScoredTypes[it.Type] {
			label := it.Type
			if it.ID != "" {
				label = it.ID
			}
			out = append(out, label)
		}
	}
	return out
}

// finishQuiz applies assignment-level settings and module placement once the
// items are safe. Each step is idempotent, so a rerun repeats it harmlessly.
func (s *Service) finishQuiz(ctx context.Context, courseID, id int64, set QuizSettings, report *QuizPushReport) error {
	fields := map[string]any{}
	for k, v := range map[string]string{"due_at": set.DueAt, "unlock_at": set.UnlockAt, "lock_at": set.LockAt, "assignment_group_id": set.AssignmentGroupID} {
		if v != "" {
			fields[k] = v
		}
	}
	if set.PostToSIS != nil {
		fields["post_to_sis"] = *set.PostToSIS
	}
	if set.Published {
		fields["published"] = true
	}
	if len(fields) > 0 {
		var a Assignment
		if e := s.Client.Write(ctx, "PUT", fmt.Sprintf("/api/v1/courses/%d/assignments/%d", courseID, id), map[string]any{"assignment": fields}, &a); e != nil {
			report.Message = "items are in, but assignment settings (dates, group, publication) were not applied; rerun the same file"
			return e
		}
		report.Published = a.Published
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
			if m.Type == "Assignment" && m.ContentID == id {
				return nil
			}
		}
		if e := s.Client.Write(ctx, "POST", path, map[string]any{"module_item": map[string]any{"type": "Assignment", "content_id": id}}, nil); e != nil {
			report.Message = "items are in, but module placement failed; rerun the same file"
			return e
		}
	}
	return nil
}
