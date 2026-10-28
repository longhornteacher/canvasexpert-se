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
	ExtraTimeMinutes *int   `json:"extra_time_minutes,omitempty" jsonschema:"Whole extra minutes, 0 through 10080; omitted fields keep the student's existing setting; 0 clears"`
	ExtraAttempts    *int   `json:"extra_attempts,omitempty" jsonschema:"Nonnegative extra attempts, an absolute setting (repeating it does not add more); omission keeps the existing setting"`
	ReduceChoices    *bool  `json:"reduce_choices,omitempty" jsonschema:"Explicit true or false; false clears, omission keeps the existing setting"`
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
// Observed on CS8 (2026-10-28): Canvas merges, so only supplied fields are sent.
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
		ExtraTime     *int  `json:"extra_time,omitempty"`
		ExtraAttempts *int  `json:"extra_attempts,omitempty"`
		ReduceChoices *bool `json:"reduce_choices_enabled,omitempty"`
	}
	body := []wireChange{}
	index := map[int64]int{}
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
		body = append(body, wireChange{st.UserID, change.ExtraTimeMinutes, change.ExtraAttempts, change.ReduceChoices})
	}
	report.Note = "Canvas merges a request into the student's existing setting: omitted fields are kept, and 0 or false clears one. Each value is absolute, so repeating a request changes nothing more. Canvas cannot read settings back; inspect Moderate after an uncertain result."
	if preview {
		report.Status = "preview"
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

// quizAccommodationHold is the publication hold left by a saved-settings
// application that did not fully land. Rerunning resends the whole absolute-value
// array, which Canvas applies idempotently.
const quizAccommodationHold = "Saved accommodations were not fully applied. Rerun to resend them; Canvas applies each value as an absolute setting, so a rerun is safe. Inspect Moderate if a student keeps failing."

// applySavedQuizAccommodations posts the planned array in one request and maps
// Canvas's per-student outcome. aliases names a failed student without an ID.
func (s *Service) applySavedQuizAccommodations(ctx context.Context, courseID, quizID int64, plan savedQuizAccommodationPlan, aliases map[int64]string) (applied int, failed []string, err error) {
	var response struct {
		Successful []struct {
			UserID int64 `json:"user_id"`
		} `json:"successful"`
		Failed []struct {
			UserID int64 `json:"user_id"`
		} `json:"failed"`
	}
	path := fmt.Sprintf("/api/quiz/v1/courses/%d/quizzes/%d/accommodations", courseID, quizID)
	if err = s.Client.Write(ctx, http.MethodPost, path, plan.Changes, &response); err != nil {
		return 0, nil, err
	}
	ok := map[int64]bool{}
	for _, r := range response.Successful {
		ok[r.UserID] = true
	}
	for _, c := range plan.Changes {
		if ok[c.UserID] {
			applied++
		} else {
			failed = append(failed, aliases[c.UserID])
		}
	}
	return applied, failed, nil
}

func savedAliases(students []workspace.Student) map[int64]string {
	m := map[int64]string{}
	for _, st := range students {
		m[st.UserID] = st.Alias
	}
	return m
}

// savedQuizApplyNote reports counts and failed aliases only.
func savedQuizApplyNote(applied int, failed []string) string {
	if len(failed) == 0 {
		return fmt.Sprintf("Saved accommodations applied for %d students.", applied)
	}
	return fmt.Sprintf("Saved accommodations applied for %d students; Canvas did not accept them for: %s. %s", applied, strings.Join(failed, ", "), quizAccommodationHold)
}

func (s *Service) savedQuizAccommodationsReport(ctx context.Context, course domain.Course, quizID int64, timer savedQuizTimer, preview bool) (QuizAccommodationReport, error) {
	students, err := s.savedQuizStudents(ctx, course.ID)
	if err != nil {
		return QuizAccommodationReport{}, err
	}
	plan, err := s.planSavedQuizAccommodations(ctx, course.ID, quizID, timer, students)
	if err != nil {
		return QuizAccommodationReport{}, err
	}
	report := QuizAccommodationReport{Planned: len(plan.Changes)}
	switch {
	case preview:
		report.Status = "preview"
	case len(plan.Changes) == 0:
		report.Status, report.Note = "completed", "No saved accommodations require application to this quiz; no Canvas settings were changed."
	default:
		applied, failed, e := s.applySavedQuizAccommodations(ctx, course.ID, quizID, plan, savedAliases(students))
		if e != nil {
			report.Status, report.Uncertain = "uncertain", len(plan.Changes)
			report.Note = "The request did not return confirmed outcomes. " + quizAccommodationHold
			return report, nil
		}
		report.Completed, report.Failed = applied, len(failed)
		report.Status, report.Note = "completed", savedQuizApplyNote(applied, failed)
		if len(failed) > 0 {
			report.Status = "partial"
		}
	}
	return report, nil
}
