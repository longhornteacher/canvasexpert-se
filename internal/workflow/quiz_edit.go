package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

type quizContent struct {
	ID     json.Number `json:"id"`
	Fields map[string]any
}

func (q *quizContent) UnmarshalJSON(data []byte) error {
	var identity struct {
		ID json.Number `json:"id"`
	}
	if err := json.Unmarshal(data, &identity); err != nil {
		return err
	}
	q.ID = identity.ID
	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	q.Fields = quizContentFields(fields, "title", "instructions")
	return nil
}

type quizContentItem struct {
	ID            json.Number `json:"id"`
	Position      int         `json:"position"`
	EntryType     string      `json:"entry_type"`
	EntryEditable *bool       `json:"entry_editable"`
	Status        string      `json:"status"`
	Entry         struct {
		ID     json.Number `json:"id"`
		Slug   string      `json:"interaction_type_slug"`
		Fields map[string]any
	} `json:"-"`
}

func (it *quizContentItem) UnmarshalJSON(data []byte) error {
	type plain quizContentItem
	var raw struct {
		*plain
		Entry json.RawMessage `json:"entry"`
	}
	raw.plain = (*plain)(it)
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if len(raw.Entry) == 0 {
		raw.Entry = json.RawMessage("null")
	}
	var entry struct {
		ID   json.Number `json:"id"`
		Slug string      `json:"interaction_type_slug"`
	}
	if err := json.Unmarshal(raw.Entry, &entry); err != nil {
		return err
	}
	it.Entry.ID, it.Entry.Slug = entry.ID, entry.Slug
	var fields map[string]any
	if err := json.Unmarshal(raw.Entry, &fields); err != nil {
		return err
	}
	it.Entry.Fields = quizContentFields(fields, "title", "item_body", "feedback", "body", "instructions")
	return nil
}

func quizContentFields(fields map[string]any, keys ...string) map[string]any {
	out := map[string]any{}
	for _, key := range keys {
		if v, ok := fields[key]; ok {
			out[key] = v
		}
	}
	return out
}

func (it quizContentItem) editReason() string {
	if it.EntryType != "Item" {
		return "Edit this stimulus or bank content in Canvas."
	}
	if it.EntryEditable != nil && !*it.EntryEditable || it.Status == "immutable" {
		return "Canvas marks this item read-only; edit it in Canvas."
	}
	if it.Entry.ID == "" || it.Entry.Slug == "" {
		return "Canvas did not supply complete question identity."
	}
	return ""
}

func quizContentBase(courseID, assignmentID int64) string {
	return fmt.Sprintf("/api/quiz/v1/courses/%d/quizzes/%d", courseID, assignmentID)
}

func (s *Service) checkQuizContentAssignment(ctx context.Context, courseID, assignmentID int64) error {
	a, err := GetAssignment(ctx, s.Client, courseID, assignmentID)
	if err != nil {
		return err
	}
	if a.ID != assignmentID || a.CourseID != courseID || !a.NewQuiz {
		return fmt.Errorf("assignment identity does not confirm this course's exact New Quiz")
	}
	return nil
}

// PullQuiz never replaces an existing draft, including an unsent edit.
func (s *Service) PullQuiz(ctx context.Context, nickname string, assignmentID int64) (string, error) {
	if assignmentID <= 0 {
		return "", fmt.Errorf("assignment ID must be positive")
	}
	course, err := s.Course(nickname)
	if err != nil {
		return "", err
	}
	s, err = s.ForCourse(ctx, course)
	if err != nil {
		return "", err
	}
	if err = s.checkQuizContentAssignment(ctx, course.ID, assignmentID); err != nil {
		return "", err
	}
	base := quizContentBase(course.ID, assignmentID)
	var quiz quizContent
	if err = s.Client.Get(ctx, base, &quiz); err != nil {
		return "", err
	}
	if quiz.ID.String() != fmt.Sprint(assignmentID) {
		return "", fmt.Errorf("Canvas returned a different quiz")
	}
	var items []quizContentItem
	if err = s.Client.List(ctx, base+"/items?per_page=100", &items); err != nil {
		return "", err
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Position < items[j].Position })
	d := domain.NewQuizEditDocument{Schema: 1, Kind: "new-quiz-edit", Course: course.Nickname, Instance: s.Client.Origin(), CourseID: course.ID, AssignmentID: assignmentID, Changes: map[string]any{}, Items: []domain.NewQuizEditItem{}}
	d.Observed = domain.NewQuizEditSnapshot{Instance: d.Instance, CourseID: d.CourseID, AssignmentID: assignmentID, Fields: quiz.Fields}
	for _, it := range items {
		reason := it.editReason()
		d.Items = append(d.Items, domain.NewQuizEditItem{ItemID: it.ID.String(), EntryID: it.Entry.ID.String(), Position: it.Position, EntryType: it.EntryType, InteractionTypeSlug: it.Entry.Slug, Editable: reason == "", Reason: reason, Changes: map[string]any{}, Observed: domain.NewQuizItemEditSnapshot{ItemID: it.ID.String(), EntryID: it.Entry.ID.String(), EntryType: it.EntryType, InteractionTypeSlug: it.Entry.Slug, Fields: it.Entry.Fields}})
	}
	path, err := workspace.ConstrainPath(s.Root, filepath.Join("courses", course.Nickname, "drafts", fmt.Sprintf("quiz-%d-%s.md", assignmentID, time.Now().UTC().Format("20060102T150405.000000000"))))
	if err != nil {
		return "", err
	}
	return path, workspace.WriteDocument(path, &d, "")
}

type QuizEditAction struct {
	ItemID   string
	Position int
	Outcome  domain.ActionOutcome
}

type QuizEditReport struct {
	Path, Status, Message  string
	Instance               string
	CourseID, AssignmentID int64
	Actions                []QuizEditAction
}

// quizTextPatch compares only requested fields. Null and absent baselines stay
// distinct. Feedback is overlaid on the current object to preserve siblings.
func quizTextPatch(observed, live, changes map[string]any) (map[string]any, string) {
	patch := map[string]any{}
	for key, want := range changes {
		if key == "feedback" {
			requested := want.(map[string]any)
			old, _ := observed[key].(map[string]any)
			now, ok := live[key].(map[string]any)
			if live[key] != nil && !ok {
				return nil, "Current feedback cannot be preserved; pull the quiz again."
			}
			sub, held := quizTextPatch(old, now, requested)
			if held != "" {
				return nil, "feedback: " + held
			}
			if len(sub) > 0 {
				merged := map[string]any{}
				for k, v := range now {
					merged[k] = v
				}
				for k, v := range sub {
					merged[k] = v
				}
				patch[key] = merged
			}
			continue
		}
		now, present := live[key]
		if present && reflect.DeepEqual(now, want) {
			continue
		}
		old, wasPresent := observed[key]
		if present != wasPresent || !reflect.DeepEqual(now, old) {
			return nil, fmt.Sprintf("%s changed in Canvas; pull again and reapply this edit. The unsent draft is preserved.", key)
		}
		patch[key] = want
	}
	return patch, ""
}

func (s *Service) PushQuizEdit(ctx context.Context, path string) (r QuizEditReport, err error) {
	r.Path, r.Status = path, "failed"
	var d domain.NewQuizEditDocument
	body, err := workspace.ReadDocument(path, &d)
	if err != nil {
		return r, err
	}
	course, err := s.Course(d.Course)
	if err != nil {
		return r, err
	}
	s, err = s.ForCourse(ctx, course)
	if err != nil {
		return r, err
	}
	if d.Instance != s.Client.Origin() || d.CourseID != course.ID {
		return r, fmt.Errorf("New Quiz edit identity differs from configured Canvas/course")
	}
	r.Instance, r.CourseID, r.AssignmentID = d.Instance, d.CourseID, d.AssignmentID
	if err = s.checkQuizContentAssignment(ctx, course.ID, d.AssignmentID); err != nil {
		return r, err
	}
	r.Status = "completed"
	base := quizContentBase(course.ID, d.AssignmentID)
	apply := func(endpoint, wrapper string, observed, live, changes map[string]any) domain.ActionOutcome {
		patch, held := quizTextPatch(observed, live, changes)
		if held != "" {
			return domain.ActionOutcome{Status: "held", Message: held}
		}
		if len(patch) == 0 {
			return domain.ActionOutcome{Status: "preserved", Message: "Requested values are already present; no PATCH."}
		}
		payload := map[string]any{wrapper: patch}
		if wrapper == "item" {
			payload = map[string]any{"item": map[string]any{"entry": patch}}
		}
		if e := s.Client.WriteAccepted(ctx, "PATCH", endpoint, payload, nil); e != nil {
			return failedOutcome(e)
		}
		return domain.ActionOutcome{Status: "succeeded", Message: "Canvas accepted the content edit."}
	}
	record := func(id string, position int, outcome domain.ActionOutcome) {
		r.Actions = append(r.Actions, QuizEditAction{ItemID: id, Position: position, Outcome: outcome})
		if outcome.Status != "succeeded" && outcome.Status != "preserved" {
			r.Status = "partial"
		}
	}
	if len(d.Changes) > 0 {
		var live quizContent
		if e := s.Client.Get(ctx, base, &live); e != nil {
			d.Outcome = domain.ActionOutcome{Status: "failed", Message: "Could not read current quiz; retry the draft."}
		} else if live.ID.String() != fmt.Sprint(d.AssignmentID) {
			d.Outcome = domain.ActionOutcome{Status: "held", Message: "Canvas returned a different quiz."}
		} else {
			d.Outcome = apply(base, "quiz", d.Observed.Fields, live.Fields, d.Changes)
		}
		record("", 0, d.Outcome)
	}
	for i := range d.Items {
		it := &d.Items[i]
		if len(it.Changes) == 0 {
			continue
		}
		var live quizContentItem
		endpoint := base + "/items/" + it.ItemID
		if !it.Editable {
			it.Outcome = domain.ActionOutcome{Status: "held", Message: "Pulled item is read-only; edit this item in Canvas."}
		} else if e := s.Client.Get(ctx, endpoint, &live); e != nil {
			it.Outcome = domain.ActionOutcome{Status: "failed", Message: "Could not read current item; retry the draft."}
		} else if live.ID.String() != it.ItemID || live.Entry.ID.String() != it.EntryID || live.EntryType != it.EntryType || live.Entry.Slug != it.InteractionTypeSlug || live.editReason() != "" {
			it.Outcome = domain.ActionOutcome{Status: "held", Message: "Item identity, type or editability changed; pull again. The unsent draft is preserved."}
		} else {
			it.Outcome = apply(endpoint, "item", it.Observed.Fields, live.Entry.Fields, it.Changes)
		}
		record(it.ItemID, it.Position, it.Outcome)
	}
	if len(r.Actions) == 0 {
		r.Message = "No content changes requested."
		return r, nil
	}
	if e := workspace.WriteDocument(path, &d, body); e != nil {
		r.Message = "Canvas outcomes above stand, but could not be saved locally; preserve this draft and rerun to compare current values."
		return r, fmt.Errorf("%s: %w", strings.TrimSpace(r.Message), e)
	}
	return r, nil
}
