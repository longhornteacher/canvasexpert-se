package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"canvasexpertse/internal/canvas"
	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

type QuizAccommodationChange struct {
	Student          string `json:"student" jsonschema:"Exact stable student reference or unique saved privacy alias"`
	ExtraTimeMinutes *int   `json:"extra_time_minutes,omitempty" jsonschema:"Whole extra minutes, 0 through 10080; omission preserves intent, never means zero"`
	ExtraAttempts    *int   `json:"extra_attempts,omitempty" jsonschema:"Nonnegative extra attempts; repeat semantics remain live-unverified"`
	ReduceChoices    *bool  `json:"reduce_choices,omitempty" jsonschema:"Explicit true or false; omission does not mean false"`
}

type QuizAccommodationRequest struct {
	Students  []QuizAccommodationChange `json:"students,omitempty"`
	FromSaved bool                      `json:"from_saved,omitempty"`
}

func DecodeQuizAccommodations(r io.Reader) (QuizAccommodationRequest, error) {
	var request QuizAccommodationRequest
	d := json.NewDecoder(r)
	d.DisallowUnknownFields()
	if err := d.Decode(&request); err != nil {
		return request, fmt.Errorf("invalid accommodation request: %w", err)
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return request, fmt.Errorf("accommodation request must contain one JSON object")
	}
	return request, request.Validate()
}

func (r QuizAccommodationRequest) Validate() error {
	if r.FromSaved {
		if len(r.Students) != 0 {
			return fmt.Errorf("from_saved and students are mutually exclusive")
		}
		return nil
	}
	if len(r.Students) == 0 {
		return fmt.Errorf("at least one student change is required")
	}
	seen := map[string]bool{}
	for i, c := range r.Students {
		key := strings.ToLower(strings.TrimSpace(c.Student))
		if key == "" || seen[key] {
			return fmt.Errorf("students[%d]: student must be nonempty and unique", i)
		}
		seen[key] = true
		if c.ExtraTimeMinutes == nil && c.ExtraAttempts == nil && c.ReduceChoices == nil {
			return fmt.Errorf("students[%d]: at least one setting is required", i)
		}
		if c.ExtraTimeMinutes != nil && (*c.ExtraTimeMinutes < 0 || *c.ExtraTimeMinutes > 10080) || c.ExtraAttempts != nil && *c.ExtraAttempts < 0 {
			return fmt.Errorf("students[%d]: extra minutes must be 0–10080 and extra attempts nonnegative", i)
		}
	}
	return nil
}

type QuizAccommodationRow struct {
	StudentRef string                  `json:"student_ref"`
	Alias      string                  `json:"alias"`
	Settings   QuizAccommodationChange `json:"settings"`
	Status     string                  `json:"status"`
	Reason     string                  `json:"reason,omitempty"`
}

type QuizAccommodationReport struct {
	Status    string                 `json:"status"`
	Rows      []QuizAccommodationRow `json:"students,omitempty"`
	Planned   int                    `json:"planned"`
	Completed int                    `json:"completed"`
	Failed    int                    `json:"failed"`
	Uncertain int                    `json:"uncertain"`
	Note      string                 `json:"note,omitempty"`
}

// SetQuizAccommodations implements the documented single-request route. Until
// omission semantics are observed, a write requires all three intended values.
// There is no automatic retry, effective-value claim, or confirmation GET.
func (s *Service) SetQuizAccommodations(ctx context.Context, nickname string, quizID int64, request QuizAccommodationRequest, preview bool) (report QuizAccommodationReport, err error) {
	if err = request.Validate(); err != nil {
		return report, err
	}
	if quizID <= 0 {
		return report, fmt.Errorf("quiz_id must be the positive New Quiz assignment ID")
	}
	configured, e := canvas.New(s.Config.Instance, "origin-check-only")
	if e != nil || configured.Origin() != s.Client.Origin() {
		return report, fmt.Errorf("configured Canvas instance differs from the active connection")
	}
	course, err := s.Course(nickname)
	if err != nil {
		return report, err
	}
	s, err = s.ForCourse(ctx, course)
	if err != nil {
		return report, err
	}
	var quiz struct {
		ID       json.Number    `json:"id"`
		CourseID json.Number    `json:"course_id"`
		Settings savedQuizTimer `json:"quiz_settings"`
	}
	path := fmt.Sprintf("/api/quiz/v1/courses/%d/quizzes/%d", course.ID, quizID)
	if err = s.Client.Get(ctx, path, &quiz); err != nil {
		return report, err
	}
	liveID, idErr := quiz.ID.Int64()
	liveCourseID, courseErr := quiz.CourseID.Int64()
	if idErr != nil || liveID != quizID || quiz.CourseID != "" && (courseErr != nil || liveCourseID != course.ID) {
		return report, fmt.Errorf("New Quiz course/assignment identity does not match")
	}
	if request.FromSaved {
		return s.savedQuizAccommodationsReport(ctx, course, quizID, quiz.Settings, preview)
	}
	// Live New Quiz detail omits course_id; the associated assignment confirms
	// exact course identity and that this is a New Quiz before any write.
	if err = s.checkQuizContentAssignment(ctx, course.ID, quizID); err != nil {
		return report, err
	}
	store, err := workspace.OpenStudents(s.Root)
	if err != nil {
		return report, err
	}
	enrollments, err := StudentEnrollments(ctx, s.Client, course.ID)
	if err != nil {
		return report, err
	}
	active := map[int64]bool{}
	for _, en := range enrollments {
		if en.Type != "StudentEnrollment" || en.State != "active" {
			continue
		}
		id := en.UserID
		if id == 0 {
			id = en.User.ID
		}
		if id <= 0 || en.User.ID > 0 && id != en.User.ID {
			return report, fmt.Errorf("course roster has an unconfirmed student identity")
		}
		active[id] = true
	}
	type wireChange struct {
		UserID        int64 `json:"user_id"`
		ExtraTime     int   `json:"extra_time"`
		ExtraAttempts int   `json:"extra_attempts"`
		ReduceChoices bool  `json:"reduce_choices_enabled"`
	}
	body := []wireChange{}
	index := map[int64]int{}
	incomplete := false
	for _, change := range request.Students {
		var matches []workspace.Student
		// Exact stable references take precedence over aliases, as elsewhere.
		if st, e := store.Resolve(s.Client.Origin(), change.Student); e == nil {
			if !active[st.UserID] {
				return QuizAccommodationReport{}, fmt.Errorf("student ref is not actively enrolled in this course")
			}
			matches = []workspace.Student{st}
		} else {
			for _, st := range store.Rows {
				if st.Instance == s.Client.Origin() && active[st.UserID] && strings.EqualFold(strings.TrimSpace(st.Alias), strings.TrimSpace(change.Student)) {
					matches = append(matches, st)
				}
			}
		}
		if len(matches) != 1 {
			return QuizAccommodationReport{}, fmt.Errorf("student ref/alias %q is missing, ambiguous or not actively enrolled", change.Student)
		}
		st := matches[0]
		if _, duplicate := index[st.UserID]; duplicate {
			return QuizAccommodationReport{}, fmt.Errorf("multiple requests resolve to the same student")
		}
		index[st.UserID] = len(report.Rows)
		change.Student = st.Ref
		report.Rows = append(report.Rows, QuizAccommodationRow{StudentRef: st.Ref, Alias: st.Alias, Settings: change, Status: "preview"})
		if change.ExtraTimeMinutes == nil || change.ExtraAttempts == nil || change.ReduceChoices == nil {
			incomplete = true
			continue
		}
		body = append(body, wireChange{st.UserID, *change.ExtraTimeMinutes, *change.ExtraAttempts, *change.ReduceChoices})
	}
	report.Note = "Effective values, omission and repeated-attempt semantics remain live-unverified; inspect Moderate after any uncertain result. Never blindly rerun accepted or uncertain requests."
	if preview {
		report.Status = "preview"
		return report, nil
	}
	if incomplete {
		report.Status = "held"
		for i := range report.Rows {
			report.Rows[i].Status = "held"
			report.Rows[i].Reason = "No write: omission semantics are unverified; supply all three intended values without guessing existing settings."
		}
		return report, nil
	}
	var response struct {
		Successful []struct {
			UserID int64 `json:"user_id"`
		} `json:"successful"`
		Failed []struct {
			UserID int64 `json:"user_id"`
		} `json:"failed"`
	}
	if err = s.Client.Write(ctx, http.MethodPost, path+"/accommodations", body, &response); err != nil {
		report.Status = "uncertain"
		report.Uncertain = len(report.Rows)
		var ce *canvas.Error
		if errors.As(err, &ce) && !ce.Uncertain && ce.Status >= 400 && ce.Status < 500 {
			report.Status = "failed"
			report.Failed, report.Uncertain = len(report.Rows), 0
		}
		for i := range report.Rows {
			report.Rows[i].Status = report.Status
			report.Rows[i].Reason = "The request did not return confirmed per-student outcomes; inspect Moderate before another request."
		}
		return report, nil
	}
	success, failure := map[int64]int{}, map[int64]int{}
	for _, row := range response.Successful {
		success[row.UserID]++
		if _, ok := index[row.UserID]; !ok {
			report.Note += " Response contained an unrequested identity."
		}
	}
	for _, row := range response.Failed {
		failure[row.UserID]++
		if _, ok := index[row.UserID]; !ok {
			report.Note += " Response contained an unrequested identity."
		}
	}
	report.Status = "completed"
	for userID, i := range index {
		row := &report.Rows[i]
		switch {
		case success[userID] == 1 && failure[userID] == 0:
			row.Status = "completed"
			report.Completed++
		case failure[userID] == 1 && success[userID] == 0:
			row.Status = "failed"
			row.Reason = "Canvas rejected this student's settings; inspect Moderate."
			report.Failed++
		default:
			row.Status = "uncertain"
			row.Reason = "Missing or conflicting response identity; inspect Moderate before another request."
			report.Uncertain++
		}
	}
	if report.Completed != len(report.Rows) {
		report.Status = "partial"
	}
	return report, nil
}

// Saved settings cannot be posted until omission/repeat and course/quiz
// combination semantics are established. This limit is shared by the public
// from-saved path and the quiz publication stage; no hidden enabling flag.
const savedQuizAccommodationHold = "Saved accommodations were not applied: Canvas omission, repeat and course/quiz combination behavior is still unverified. Inspect Moderate; do not guess or clear existing settings."

func (s *Service) savedQuizAccommodationsReport(ctx context.Context, course domain.Course, quizID int64, timer savedQuizTimer, preview bool) (QuizAccommodationReport, error) {
	students, err := s.savedQuizStudents(ctx, course.ID)
	if err != nil {
		return QuizAccommodationReport{}, err
	}
	plan, err := s.planSavedQuizAccommodations(ctx, course.ID, quizID, timer, students)
	if err != nil {
		return QuizAccommodationReport{}, err
	}
	report := QuizAccommodationReport{Planned: len(plan.Changes), Status: "held", Note: savedQuizAccommodationHold}
	if preview {
		report.Status = "preview"
	} else if len(plan.Changes) == 0 {
		report.Status, report.Note = "completed", "No saved accommodations require application to this quiz; no Canvas settings were changed."
	}
	return report, nil
}
