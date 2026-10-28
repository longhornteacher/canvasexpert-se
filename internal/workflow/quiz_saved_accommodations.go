package workflow

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

// savedQuizTimer retains presence: an unread timer is never treated as untimed.
type savedQuizTimer struct {
	Timed   *bool  `json:"has_time_limit"`
	Seconds *int64 `json:"session_time_limit_in_seconds"`
}

type savedQuizAccommodation struct {
	UserID        int64 `json:"user_id"`
	ExtraTime     *int  `json:"extra_time,omitempty"`
	ReduceChoices *bool `json:"reduce_choices_enabled,omitempty"`
}

// This plan remains private to the workflow. Only count/digest reach a sidecar;
// preview and success responses never include its student identities/settings.
type savedQuizAccommodationPlan struct {
	Changes   []savedQuizAccommodation
	Digest    string
	Published bool
}

func (s *Service) savedQuizCandidates() ([]workspace.Student, error) {
	store, err := workspace.OpenStudents(s.Root)
	if err != nil {
		return nil, err
	}
	var candidates []workspace.Student
	for _, st := range store.Rows {
		if st.Instance == s.Client.Origin() && (st.QuizTime != "" || st.ReducedChoices) {
			candidates = append(candidates, st)
		}
	}
	return candidates, nil
}

func (s *Service) savedQuizStudents(ctx context.Context, courseID int64) ([]workspace.Student, error) {
	candidates, err := s.savedQuizCandidates()
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	enrollments, err := StudentEnrollments(ctx, s.Client, courseID)
	if err != nil {
		return nil, err
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
		if id <= 0 || en.User.ID > 0 && en.User.ID != id {
			return nil, fmt.Errorf("course roster has an unconfirmed student identity")
		}
		active[id] = true
	}
	var students []workspace.Student
	for _, st := range candidates {
		if active[st.UserID] {
			students = append(students, st)
		}
	}
	return students, nil
}

// This publication stage deliberately makes no accommodation POST until the
// live setter semantics are known. Explicit skipping lasts for this call only.
func (s *Service) guardSavedQuizPublication(ctx context.Context, course domain.Course, quizID int64, set QuizSettings, skip bool, rec *quizSidecar, report *QuizPushReport, save func() error) (bool, error) {
	stage := &quizAccommodationStage{}
	rec.Stages.Accommodations = stage
	if skip {
		if err := s.checkQuizContentAssignment(ctx, course.ID, quizID); err != nil {
			return false, err
		}
		stage.Status = "skipped"
		report.Accommodations = "Saved quiz accommodations skipped by explicit request; no accommodation settings were changed."
		return true, save()
	}
	students, err := s.savedQuizStudents(ctx, course.ID)
	if err != nil {
		return false, err
	}
	if len(students) == 0 {
		stage.Status = "not-requested"
		return true, save()
	}
	timed, seconds := set.HasTimeLimit, int64(set.TimeLimitMinutes)
	if timed && (seconds <= 0 || seconds > (1<<63-1)/60) {
		return false, fmt.Errorf("quiz time limit is invalid; saved accommodations were not assumed")
	}
	if timed {
		seconds *= 60
	} else {
		seconds = 0
	}
	plan, err := s.planSavedQuizAccommodations(ctx, course.ID, quizID, savedQuizTimer{Timed: &timed, Seconds: &seconds}, students)
	if err != nil {
		return false, err
	}
	stage.Count, stage.Digest = len(plan.Changes), plan.Digest
	report.Published = plan.Published
	if len(plan.Changes) == 0 {
		stage.Status = "not-requested"
		return true, save()
	}
	stage.Status = "held"
	report.Accommodations = fmt.Sprintf("Saved accommodations planned for %d students; none applied.", stage.Count)
	report.Message = savedQuizAccommodationHold + " Publication was held; explicitly request --skip-accommodations to publish anyway."
	if plan.Published {
		report.Message = savedQuizAccommodationHold + " This quiz is already published; its publication state was preserved."
	}
	return false, save()
}

func savedQuizExtraMinutes(value string, timer savedQuizTimer) (*int, error) {
	if value == "" {
		return nil, nil
	}
	if !workspace.ValidQuizTime(value) {
		return nil, fmt.Errorf("saved quiz_time is invalid; correct it on Students")
	}
	if timer.Timed == nil || *timer.Timed && (timer.Seconds == nil || *timer.Seconds <= 0) {
		return nil, fmt.Errorf("quiz time limit is unavailable; no saved accommodation was assumed")
	}
	if !*timer.Timed {
		return nil, nil
	}
	var minutes int64
	switch value {
	case "50%", "100%":
		divisor := int64(60)
		if value == "50%" {
			divisor = 120
		}
		minutes = *timer.Seconds / divisor
		if *timer.Seconds%divisor != 0 {
			minutes++
		}
	default:
		minutes, _ = strconv.ParseInt(value, 10, 64)
	}
	if minutes > 10080 {
		return nil, fmt.Errorf("saved quiz time exceeds Canvas's 10080-minute maximum; accommodation held without capping")
	}
	n := int(minutes)
	return &n, nil
}

func (s *Service) planSavedQuizAccommodations(ctx context.Context, courseID, quizID int64, timer savedQuizTimer, students []workspace.Student) (savedQuizAccommodationPlan, error) {
	plan := savedQuizAccommodationPlan{Changes: []savedQuizAccommodation{}}
	// Canvas resolves direct, section, tag and inherited module visibility. Do
	// not approximate that audience from local membership or authored intent.
	var assignment struct {
		ID         int64          `json:"id"`
		CourseID   int64          `json:"course_id"`
		NewQuiz    bool           `json:"is_quiz_lti_assignment"`
		Published  bool           `json:"published"`
		Visibility *[]json.Number `json:"assignment_visibility"`
	}
	if err := s.Client.Get(ctx, fmt.Sprintf("/api/v1/courses/%d/assignments/%d?include[]=assignment_visibility", courseID, quizID), &assignment); err != nil {
		return plan, err
	}
	if assignment.ID != quizID || assignment.CourseID != courseID || !assignment.NewQuiz {
		return plan, fmt.Errorf("New Quiz associated assignment identity does not match this course")
	}
	plan.Published = assignment.Published
	if timer.Timed != nil && !*timer.Timed {
		// A saved timer allowance has no action on any untimed quiz, regardless
		// of audience. Fewer choices still needs a confirmed audience.
		var applicable []workspace.Student
		for _, st := range students {
			if st.ReducedChoices {
				applicable = append(applicable, st)
			}
		}
		students = applicable
	}
	if len(students) == 0 {
		return plan, nil
	}
	// Upstream's visibility SQL excludes unpublished recipients on supported
	// override paths. Even a present/nonempty list can therefore be partial.
	// Do not turn that list into a claim that no saved accommodation is needed.
	if !assignment.Published {
		return plan, fmt.Errorf("quiz's effective student audience is unavailable before publication; assignment_visibility can omit unpublished recipients")
	}
	if assignment.Visibility == nil {
		return plan, fmt.Errorf("quiz's effective student audience is unavailable; saved accommodations were not assumed")
	}
	visible := map[int64]bool{}
	for _, raw := range *assignment.Visibility {
		id, err := raw.Int64()
		if err != nil || id <= 0 {
			return plan, fmt.Errorf("quiz's effective student audience contains an unconfirmed identity")
		}
		visible[id] = true
	}
	for _, st := range students {
		if !visible[st.UserID] {
			continue
		}
		minutes, err := savedQuizExtraMinutes(st.QuizTime, timer)
		if err != nil {
			return plan, err
		}
		change := savedQuizAccommodation{UserID: st.UserID, ExtraTime: minutes}
		if st.ReducedChoices {
			yes := true
			change.ReduceChoices = &yes
		}
		if change.ExtraTime != nil || change.ReduceChoices != nil {
			plan.Changes = append(plan.Changes, change)
		}
	}
	sort.Slice(plan.Changes, func(i, j int) bool { return plan.Changes[i].UserID < plan.Changes[j].UserID })
	b, _ := json.Marshal(struct {
		CourseID, QuizID int64
		Changes          []savedQuizAccommodation
	}{courseID, quizID, plan.Changes})
	plan.Digest = fmt.Sprintf("%x", sha256.Sum256(b))
	return plan, nil
}
