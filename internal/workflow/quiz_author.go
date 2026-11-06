package workflow

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	Status         string // completed or partial
	AssignmentID   int64
	URL            string
	ItemsAdded     int
	ItemsTotal     int
	Published      bool
	Message        string
	Hub            string // hub tier pages, tags, and teacher actions
	Audience       string // applied individual club audience, when requested
	Accommodations string // private saved-settings outcome, counts only
	Path           string // existing sidecar containing the created quiz ID and resume data
	Warnings       []string
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
	// Stages records accepted restricted-quiz/accommodation work so a resume
	// repeats only unfinished stages. Older ordinary sidecars may omit it.
	Stages *quizStages `json:"stages,omitempty"`
	// Media records each accepted hot-spot image upload by item id, saved before
	// the item is posted, so a rerun reuses the image instead of uploading again.
	Media map[string]quizMedia `json:"media,omitempty"`
	// Creating is the title of a quiz whose create request was about to be sent. If the
	// process stops before the ID is saved, a rerun adopts the one quiz with this title
	// instead of holding on "already exists".
	Creating string `json:"creating,omitempty"`
}

// quizMedia holds an unsigned image URL and the hash of the uploaded bytes; an
// edited image file is uploaded again.
type quizMedia struct {
	SHA256   string `json:"sha256"`
	ImageURL string `json:"image_url"`
}

// quizStages records restricted-quiz and saved-accommodation work. Canvas
// outcomes become definite only after acceptance; skipped is not applied.
type quizStages struct {
	Items          domain.ActionOutcome    `json:"items,omitempty"`
	Settings       domain.ActionOutcome    `json:"settings,omitempty"`
	Audience       domain.ActionOutcome    `json:"audience,omitempty"`
	Module         domain.ActionOutcome    `json:"module,omitempty"`
	Publication    domain.ActionOutcome    `json:"publication,omitempty"`
	Accommodations *quizAccommodationStage `json:"accommodations,omitempty"`
}

type quizAccommodationStage struct {
	Status string `json:"status"`
	Count  int    `json:"count,omitempty"`
	Digest string `json:"digest,omitempty"`
}

type QuizPushOptions struct {
	SkipAccommodations bool
}

// writeSidecar replaces the sidecar atomically (temp file then rename), so a kill mid-write never
// leaves a torn record.
func writeSidecar(path string, rec quizSidecar) error {
	b, _ := json.Marshal(rec)
	tmp := sidecarPath(path) + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, sidecarPath(path))
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
func (s *Service) PushQuiz(ctx context.Context, nickname, path string, options ...QuizPushOptions) (report QuizPushReport, err error) {
	var opts QuizPushOptions
	if len(options) > 1 {
		return report, fmt.Errorf("only one quiz push options value is supported")
	}
	if len(options) == 1 {
		opts = options[0]
	}
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
	s, e = s.ForCourse(ctx, course)
	if e != nil {
		return report, e
	}
	items := q.itemPayloads()
	report.ItemsTotal = len(items)
	base := fmt.Sprintf("/api/quiz/v1/courses/%d/quizzes", course.ID)
	rec := readSidecar(path)
	restricted := set.Audience != nil || rec.Audience != nil
	candidates, e := s.savedQuizCandidates()
	if e != nil {
		return report, e
	}
	tracked := restricted || len(candidates) > 0 || rec.Stages != nil || opts.SkipAccommodations
	if rec.Instance != "" && (rec.Instance != s.Client.Origin() || rec.CourseID != 0 && rec.CourseID != course.ID) {
		return report, fmt.Errorf("saved quiz sidecar belongs to a different Canvas/course; it was not reused")
	}
	if tracked {
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
	if id == 0 && rec.Creating != "" {
		// An earlier push recorded that it was about to create this quiz but stopped before saving
		// its ID. Adopt the one quiz with that title; if none exists the create never landed.
		var quizzes []struct {
			ID    json.Number `json:"id"`
			Title string      `json:"title"`
		}
		if e = s.Client.List(ctx, base, &quizzes); e != nil {
			return report, e
		}
		var found []int64
		for _, x := range quizzes {
			if x.Title == rec.Creating {
				n, _ := x.ID.Int64()
				found = append(found, n)
			}
		}
		switch {
		case len(found) == 1 && found[0] > 0:
			id = found[0]
			rec.AssignmentID, rec.Creating = id, ""
			if e = writeSidecar(path, rec); e != nil {
				report.AssignmentID = id
				return report, fmt.Errorf("quiz %d was found but its ID could not be saved; write {\"assignment_id\": %d} to %s before rerunning", id, id, sidecarPath(path))
			}
		case len(found) == 0:
			rec.Creating = ""
			if e = writeSidecar(path, rec); e != nil {
				return report, e
			}
		default:
			return report, fmt.Errorf("several New Quizzes titled %q exist in this course, so the earlier unfinished push cannot be matched; ask the teacher to delete the extra copies in Canvas before pushing again", rec.Creating)
		}
	}
	fresh := false
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
				return report, fmt.Errorf("a New Quiz titled %q already exists in this course (id %s); rename this quiz; if it is an unfinished earlier push of this file, ask the teacher to rename or delete it in Canvas before pushing again", title, x.ID)
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
		// Write-ahead marker (tier pages from the hub step are kept): a stop between the create
		// and the saved ID leaves a trace the next run can match against the live quiz.
		rec = readSidecar(path)
		rec.Creating = title
		if e = writeSidecar(path, rec); e != nil {
			return report, e
		}
		if e = s.Client.Write(ctx, "POST", base, map[string]any{"quiz": quiz}, &created); e != nil {
			return report, e
		}
		id, _ = created.ID.Int64()
		if id <= 0 {
			return report, &canvas.Error{Kind: "uncertain", Uncertain: true}
		}
		fresh = true
		rec = readSidecar(path) // hub delivery may have recorded tier pages
		rec.AssignmentID, rec.Creating = id, ""
		if e = writeSidecar(path, rec); e != nil {
			report.AssignmentID = id
			return report, fmt.Errorf("quiz %d was created but its ID could not be saved; write {\"assignment_id\": %d} to %s before rerunning", id, id, sidecarPath(path))
		}
	}
	var have []quizEntryRef
	if !fresh {
		if e = s.Client.List(ctx, fmt.Sprintf("%s/%d/items", base, id), &have); e != nil {
			return report, e
		}
	}
	existing = len(have)
	report.Path = sidecarPath(path)
	report.AssignmentID = id
	report.URL = fmt.Sprintf("%s/courses/%d/assignments/%d", s.Client.Origin(), course.ID, id)
	report.ItemsAdded = existing
	labels := q.scoredIDs()
	// A tracked quiz resumes only unfinished stages; accepted items are
	// never re-posted.
	itemsAccepted := rec.Stages != nil && definite(rec.Stages.Items)
	if itemsAccepted {
		report.ItemsAdded = len(items)
	}
	native := q.hasStimuli() && !itemsAccepted && !inlinedEarlier(have, q.quizSteps())
	var build *canvas.NewQuizBuild
	if native {
		if build, e = s.Client.OpenNewQuizBuild(ctx, course.ID, id); e != nil {
			if hasStimulusEntry(have) {
				report.Status = "partial"
				report.Message = "the quiz editor session for the passages did not open; rerun the same file to continue this quiz"
				return report, e
			}
			// Questions already in the quiz come before the first passage, so
			// copying passages into the remaining questions stays consistent.
			native = false
			report.Warnings = append(report.Warnings, "Canvas's quiz editor did not open for this quiz, so each passage was copied into its questions instead of a separate stimulus block")
		}
	}
	if native {
		if done, e := s.pushQuizSteps(ctx, q, course.ID, id, path, &rec, build, have, &report); e != nil || !done {
			return report, e
		}
	} else {
		prepared := q.preparedItems()
		for i := existing; i < len(items) && !itemsAccepted; i++ {
			if StopRequested() {
				report.Status = "partial"
				report.Message = fmt.Sprintf("stopped before item %d of %d because the operation was cancelled or ran out of time; the quiz is saved and unpublished. Rerun the same file to continue", i+1, len(items))
				return report, nil
			}
			if prepared[i].Type == "HOTSPOT" {
				if e = s.attachHotspotImage(ctx, course.ID, id, path, &rec, prepared[i], items[i]); e != nil {
					report.Status = "partial"
					report.Message = fmt.Sprintf("item %d of %d (%s) was not added because its image was not uploaded; rerun the same file to continue this quiz", i+1, len(items), labels[i])
					return report, e
				}
			}
			if e = s.Client.Write(ctx, "POST", fmt.Sprintf("%s/%d/items", base, id), items[i], nil); e != nil {
				report.Status = "partial"
				report.Message = fmt.Sprintf("item %d of %d (%s) was not added; rerun the same file to continue this quiz", i+1, len(items), labels[i])
				return report, e
			}
			report.ItemsAdded++
		}
	}
	if rec.Stages != nil && !definite(rec.Stages.Items) {
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
	if StopRequested() {
		report.Status = "partial"
		report.Message = "stopped after the items were added, before the quiz settings and publication, because the operation was cancelled or ran out of time. Rerun the same file to continue"
		return report, nil
	}
	if complete, e := s.finishQuiz(ctx, course, id, set, &report, path, opts); e != nil {
		report.Status = "partial"
		return report, e
	} else if !complete {
		report.Status = "partial"
		return report, nil
	}
	report.Status = "completed"
	return report, nil
}

// quizEntryRef is one entry already in a New Quiz, as the public items list
// shows it.
type quizEntryRef struct {
	ID        json.RawMessage `json:"id"`
	EntryType string          `json:"entry_type"`
	Stimulus  json.RawMessage `json:"stimulus_quiz_entry_id"`
}

func rawID(r json.RawMessage) string {
	v := strings.Trim(strings.TrimSpace(string(r)), `"`)
	if v == "null" {
		return ""
	}
	return v
}

func hasStimulusEntry(have []quizEntryRef) bool {
	for _, h := range have {
		if h.EntryType == "Stimulus" {
			return true
		}
	}
	return false
}

// inlinedEarlier reports whether an earlier push of this file copied its
// passages into the questions: entries reach past the first stimulus's place
// but none is a stimulus.
func inlinedEarlier(have []quizEntryRef, steps []quizStep) bool {
	if hasStimulusEntry(have) {
		return false
	}
	for i, st := range steps {
		if st.stimulus != nil {
			return len(have) > i
		}
	}
	return false
}

// pushQuizSteps adds the quiz's entries in order with real stimuli: each
// STIMULUS becomes a Canvas stimulus and each question naming it is attached
// (the public items API ignores that link). Entries already in Canvas are
// matched by position, and an attachment a stopped run left undone is
// completed. It returns false when it stopped early with a partial report.
func (s *Service) pushQuizSteps(ctx context.Context, q QuizForge, courseID, id int64, path string, rec *quizSidecar, build *canvas.NewQuizBuild, have []quizEntryRef, report *QuizPushReport) (bool, error) {
	steps := q.quizSteps()
	prepared := q.prepareItems(false)
	payloads := q.buildPayloads(false)
	labels := q.scoredIDs()
	stimEntry := map[string]string{}
	report.ItemsAdded = 0
	link := func(st quizStep, entryID string, pos int) error {
		sid := prepared[st.item].StimulusID
		if sid == "" {
			return nil
		}
		if e := build.LinkToStimulus(ctx, entryID, stimEntry[sid], pos); e != nil {
			report.Status = "partial"
			report.Message = fmt.Sprintf("question %s was added but not attached to its passage %s; rerun the same file to attach it", labels[st.item], sid)
			return e
		}
		return nil
	}
	for i, h := range have {
		if i >= len(steps) {
			break
		}
		st := steps[i]
		if (st.stimulus != nil) != (h.EntryType == "Stimulus") {
			report.Status = "partial"
			report.Message = fmt.Sprintf("entry %d of the quiz in Canvas does not match this file, so nothing more was added; ask the teacher to delete the quiz in Canvas and push again", i+1)
			return false, fmt.Errorf("quiz %d no longer matches its file", id)
		}
		if st.stimulus != nil {
			stimEntry[st.stimulus.ID] = rawID(h.ID)
			continue
		}
		report.ItemsAdded++
		if prepared[st.item].StimulusID != "" && rawID(h.Stimulus) == "" {
			if e := link(st, rawID(h.ID), i+1); e != nil {
				return false, e
			}
		}
	}
	for i := len(have); i < len(steps); i++ {
		st, pos := steps[i], i+1
		if StopRequested() {
			report.Status = "partial"
			report.Message = fmt.Sprintf("stopped before entry %d of %d because the operation was cancelled or ran out of time; the quiz is saved and unpublished. Rerun the same file to continue", pos, len(steps))
			return false, nil
		}
		if st.stimulus != nil {
			title := normalizeStudentText(st.stimulus.stimulusTitle()).(string)
			body := normalizeStudentText(highlightCodeBlocks(st.stimulus.Prompt)).(string)
			entryID, e := build.CreateStimulus(ctx, title, body, st.stimulus.stimulusOrientation(), pos)
			if e != nil {
				report.Status = "partial"
				report.Message = fmt.Sprintf("passage %s was not added; rerun the same file to continue this quiz", st.stimulus.ID)
				return false, e
			}
			stimEntry[st.stimulus.ID] = entryID
			continue
		}
		it, payload := prepared[st.item], payloads[st.item]
		payload["item"].(map[string]any)["position"] = pos
		if it.Type == "HOTSPOT" {
			if e := s.attachHotspotImage(ctx, courseID, id, path, rec, it, payload); e != nil {
				report.Status = "partial"
				report.Message = fmt.Sprintf("item %d of %d (%s) was not added because its image was not uploaded; rerun the same file to continue this quiz", st.item+1, len(payloads), labels[st.item])
				return false, e
			}
		}
		var made quizEntryRef
		if e := s.Client.Write(ctx, "POST", fmt.Sprintf("/api/quiz/v1/courses/%d/quizzes/%d/items", courseID, id), payload, &made); e != nil {
			report.Status = "partial"
			report.Message = fmt.Sprintf("item %d of %d (%s) was not added; rerun the same file to continue this quiz", st.item+1, len(payloads), labels[st.item])
			return false, e
		}
		report.ItemsAdded++
		if it.StimulusID != "" && rawID(made.ID) == "" {
			report.Status = "partial"
			report.Message = fmt.Sprintf("question %s was added but Canvas did not return its ID; rerun the same file to attach it to its passage", labels[st.item])
			return false, &canvas.Error{Kind: "uncertain", Uncertain: true}
		}
		if e := link(st, rawID(made.ID), pos); e != nil {
			return false, e
		}
	}
	return true, nil
}

// attachHotspotImage uploads the item's image once and sets image_url on its
// payload. An accepted upload is recorded in the sidecar before the item is
// posted. Signed URLs are never stored or reported.
func (s *Service) attachHotspotImage(ctx context.Context, courseID, quizID int64, path string, rec *quizSidecar, it qfItem, payload map[string]any) error {
	data, e := os.ReadFile(it.imagePath)
	if e != nil {
		return fmt.Errorf("image file %q cannot be read", it.Image)
	}
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	m, ok := rec.Media[it.ID]
	if !ok || m.SHA256 != hash || m.ImageURL == "" {
		url, e := s.Client.UploadQuizMedia(ctx, courseID, quizID, data)
		if e != nil {
			return e
		}
		if rec.Media == nil {
			rec.Media = map[string]quizMedia{}
		}
		m = quizMedia{SHA256: hash, ImageURL: url}
		rec.Media[it.ID] = m
		if e = writeSidecar(path, *rec); e != nil {
			return fmt.Errorf("image for %s was uploaded but could not be recorded beside the quiz file: %v", it.ID, e)
		}
	}
	entry := payload["item"].(map[string]any)["entry"].(map[string]any)
	entry["interaction_data"].(map[string]any)["image_url"] = m.ImageURL
	return nil
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
// repeats it harmlessly. Restricted quizzes and quizzes with saved requirements
// record stages in the sidecar and resume unfinished work. Incomplete audiences
// and unapplied saved accommodations hold publication.
func (s *Service) finishQuiz(ctx context.Context, course domain.Course, id int64, set QuizSettings, report *QuizPushReport, path string, opts QuizPushOptions) (bool, error) {
	courseID := course.ID
	rec := readSidecar(path)
	restricted := set.Audience != nil || rec.Audience != nil
	var stages *quizStages
	if restricted || rec.Stages != nil {
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
	if stages != nil {
		complete, e := s.guardSavedQuizPublication(ctx, course, id, set, opts.SkipAccommodations, &rec, report, save)
		if e != nil {
			report.Message = "Saved quiz accommodations could not be prepared; publication was held."
			stages.Accommodations.Status = "held"
			if saveErr := save(); saveErr != nil {
				return false, saveErr
			}
			return false, e
		}
		if !complete {
			return false, nil
		}
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
