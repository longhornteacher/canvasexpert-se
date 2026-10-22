// Package workflow coordinates Canvas operations and workspace documents.
// Canvas DTOs stay here so API responses are never written wholesale to agent files.
package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"canvasexpertse/internal/canvas"
	"canvasexpertse/internal/domain"
)

type Course struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Code  string `json:"course_code"`
	State string `json:"workflow_state"`
}
type Module struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Position int    `json:"position"`
}
type Rating struct {
	ID              string      `json:"id"`
	Points          json.Number `json:"points"`
	Description     string      `json:"description"`
	LongDescription string      `json:"long_description"`
}
type Criterion struct {
	ID              string      `json:"id"`
	Points          json.Number `json:"points"`
	Description     string      `json:"description"`
	LongDescription string      `json:"long_description"`
	Ratings         []Rating    `json:"ratings"`
	Range           bool        `json:"criterion_use_range"`
}
type Assignment struct {
	ID                             int64       `json:"id"`
	CourseID                       int64       `json:"course_id"`
	Name                           string      `json:"name"`
	Description                    string      `json:"description"`
	Points                         json.Number `json:"points_possible"`
	SubmissionTypes                []string    `json:"submission_types"`
	Published                      bool        `json:"published"`
	DueAt                          *string     `json:"due_at"`
	UnlockAt                       *string     `json:"unlock_at"`
	LockAt                         *string     `json:"lock_at"`
	UpdatedAt                      string      `json:"updated_at"`
	CreatedAt                      string      `json:"created_at"`
	HTMLURL                        string      `json:"html_url"`
	GradingType                    string      `json:"grading_type"`
	NeedsGradingCount              *int        `json:"needs_grading_count"`
	AssignmentGroupID              *int64      `json:"assignment_group_id"`
	InClosedGradingPeriod          bool        `json:"in_closed_grading_period"`
	GroupCategoryID                *int64      `json:"group_category_id"`
	GradeGroupStudentsIndividually *bool       `json:"grade_group_students_individually"`
	DiscussionTopicID              *int64      `json:"discussion_topic_id"`
	DiscussionTopic                *struct {
		ID int64 `json:"id"`
	} `json:"discussion_topic"`
	HasSubAssignments      *bool                          `json:"has_sub_assignments"`
	Checkpoints            *[]domain.AssignmentCheckpoint `json:"checkpoints"`
	Overrides              []AssignmentOverrideLive       `json:"overrides"`
	OnlyVisibleToOverrides *bool                          `json:"only_visible_to_overrides"`
	AllDates               *[]AssignmentAllDateLive       `json:"all_dates"`
	Anonymous              bool                           `json:"anonymous_grading"`
	Moderated              bool                           `json:"moderated_grading"`
	QuizID                 *int64                         `json:"quiz_id"`
	NewQuiz                bool                           `json:"is_quiz_lti_assignment"`
	Rubric                 []Criterion                    `json:"rubric"`
	RubricSettings         struct {
		ID       int64       `json:"id"`
		Title    string      `json:"title"`
		Points   json.Number `json:"points_possible"`
		FreeForm bool        `json:"free_form_criterion_comments"`
	} `json:"rubric_settings"`
	RubricAssociation *RubricAssociation `json:"rubric_association"`
}

type AssignmentOverrideLive struct {
	CourseID           int64           `json:"course_id,omitempty"`
	NoopID             json.RawMessage `json:"noop_id,omitempty"`
	UnassignItem       *bool           `json:"unassign_item,omitempty"`
	DueAtOverridden    *bool           `json:"due_at_overridden,omitempty"`
	UnlockAtOverridden *bool           `json:"unlock_at_overridden,omitempty"`
	LockAtOverridden   *bool           `json:"lock_at_overridden,omitempty"`
	ContextModuleID    int64           `json:"context_module_id,omitempty"`
	ID                 int64           `json:"id"`
	Title              string          `json:"title"`
	GroupID            int64           `json:"group_id"`
	StudentIDs         []int64         `json:"student_ids"`
	CourseSectionID    int64           `json:"course_section_id"`
	DueAt              *string         `json:"due_at"`
	UnlockAt           *string         `json:"unlock_at"`
	LockAt             *string         `json:"lock_at"`
}

func (o *AssignmentOverrideLive) UnmarshalJSON(b []byte) error {
	type plain AssignmentOverrideLive
	var p plain
	if err := json.Unmarshal(b, &p); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(b, &fields); err != nil {
		return err
	}
	*o = AssignmentOverrideLive(p)
	for _, date := range []struct {
		key  string
		flag **bool
	}{{"due_at", &o.DueAtOverridden}, {"unlock_at", &o.UnlockAtOverridden}, {"lock_at", &o.LockAtOverridden}} {
		if *date.flag == nil {
			_, present := fields[date.key]
			v := present
			*date.flag = &v
		}
	}
	return nil
}

type AssignmentAllDateLive struct {
	ID              int64   `json:"id"`
	Base            bool    `json:"base"`
	UserID          int64   `json:"user_id"`
	StudentID       int64   `json:"student_id"`
	GroupID         int64   `json:"group_id"`
	CourseSectionID int64   `json:"course_section_id"`
	DueAt           *string `json:"due_at"`
	UnlockAt        *string `json:"unlock_at"`
	LockAt          *string `json:"lock_at"`
}
type RubricAssociation struct {
	ID              int64  `json:"id"`
	RubricID        int64  `json:"rubric_id"`
	AssociationID   int64  `json:"association_id"`
	AssociationType string `json:"association_type"`
	UseForGrading   *bool  `json:"use_for_grading"`
}
type CanvasUser struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	ShortName string `json:"short_name"`
	SISID     string `json:"sis_user_id"`
	Login     string `json:"login_id"`
}
type Enrollment struct {
	UserID int64      `json:"user_id"`
	Type   string     `json:"type"`
	State  string     `json:"enrollment_state"`
	User   CanvasUser `json:"user"`
	Grades *struct {
		CurrentScore *json.Number `json:"current_score"`
	} `json:"grades"`
}

func StudentEnrollments(ctx context.Context, c *canvas.Client, courseID int64) ([]Enrollment, error) {
	var result []Enrollment
	err := c.List(ctx, fmt.Sprintf("/api/v1/courses/%d/enrollments?type[]=StudentEnrollment&state[]=active&state[]=invited&state[]=inactive&state[]=completed&per_page=100", courseID), &result)
	return result, err
}

type Attachment struct {
	ID          int64  `json:"id"`
	Filename    string `json:"filename"`
	DisplayName string `json:"display_name"`
	URL         string `json:"url"`
	ContentType string `json:"content-type"`
	Size        int64  `json:"size"`
}
type SubmissionComment struct {
	ID        int64  `json:"id"`
	Comment   string `json:"comment"`
	Attempt   *int   `json:"attempt"`
	CreatedAt string `json:"created_at"`
	AuthorID  int64  `json:"author_id"`
}
type Submission struct {
	ID                            int64                  `json:"id"`
	UserID                        int64                  `json:"user_id"`
	AssignmentID                  int64                  `json:"assignment_id"`
	Attempt                       int                    `json:"attempt"`
	Body                          string                 `json:"body"`
	SubmittedAt                   *string                `json:"submitted_at"`
	GradedAt                      *string                `json:"graded_at"`
	CachedDueDate                 *string                `json:"cached_due_date"`
	CachedDueDatePresent          bool                   `json:"-"`
	Type                          string                 `json:"submission_type"`
	State                         string                 `json:"workflow_state"`
	Score                         *json.Number           `json:"score"`
	EnteredScore                  *json.Number           `json:"entered_score"`
	PointsDeducted                *json.Number           `json:"points_deducted"`
	Excused                       bool                   `json:"excused"`
	Missing                       bool                   `json:"missing"`
	Late                          bool                   `json:"late"`
	LateKnown                     bool                   `json:"-"`
	LatePolicyStatus              *string                `json:"late_policy_status"`
	LatePolicyStatusKnown         bool                   `json:"-"`
	CustomGradeStatusID           json.RawMessage        `json:"custom_grade_status_id,omitempty"`
	SecondsLate                   int64                  `json:"seconds_late"`
	SecondsLateKnown              bool                   `json:"-"`
	GradeMatchesCurrentSubmission *bool                  `json:"grade_matches_current_submission"`
	AssignmentVisible             *bool                  `json:"assignment_visible"`
	PostedAt                      *string                `json:"posted_at"`
	PostedAtPresent               bool                   `json:"-"`
	GroupID                       int64                  `json:"group_id"`
	Group                         *SubmissionGroup       `json:"group"`
	User                          CanvasUser             `json:"user"`
	HistoryPresent                bool                   `json:"-"`
	Grade                         *string                `json:"grade"`
	History                       []Submission           `json:"submission_history"`
	Attachments                   []Attachment           `json:"attachments"`
	Comments                      []SubmissionComment    `json:"submission_comments"`
	RubricAssessment              map[string]RubricScore `json:"rubric_assessment"`
	URL                           string                 `json:"url"`
	MediaComment                  *struct {
		URL         string `json:"url"`
		DisplayName string `json:"display_name"`
		MediaType   string `json:"media_type"`
	} `json:"media_comment"`
	DiscussionEntries []struct {
		Message   string `json:"message"`
		CreatedAt string `json:"created_at"`
	} `json:"discussion_entries"`
}

// UnmarshalJSON retains whether nullable effective-date and timing fields were
// returned at all. Canvas omitting a field is different from returning null.
func (s *Submission) UnmarshalJSON(data []byte) error {
	type plain Submission
	var decoded struct {
		*plain
		CachedDueDate    json.RawMessage `json:"cached_due_date"`
		LatePolicyStatus json.RawMessage `json:"late_policy_status"`
		Late             json.RawMessage `json:"late"`
		SecondsLate      json.RawMessage `json:"seconds_late"`
		PostedAt         json.RawMessage `json:"posted_at"`
	}
	*s = Submission{}
	decoded.plain = (*plain)(s)
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	_, s.HistoryPresent = fields["submission_history"]
	s.CachedDueDatePresent = len(decoded.CachedDueDate) > 0
	if s.CachedDueDatePresent {
		if bytes.Equal(bytes.TrimSpace(decoded.CachedDueDate), []byte("null")) {
			s.CachedDueDate = nil
		} else {
			var value string
			if err := json.Unmarshal(decoded.CachedDueDate, &value); err != nil {
				return fmt.Errorf("cached_due_date must be a timestamp or null: %w", err)
			}
			s.CachedDueDate = &value
		}
	}
	s.LatePolicyStatusKnown = len(decoded.LatePolicyStatus) > 0
	if s.LatePolicyStatusKnown {
		if bytes.Equal(bytes.TrimSpace(decoded.LatePolicyStatus), []byte("null")) {
			s.LatePolicyStatus = nil
		} else {
			var value string
			if err := json.Unmarshal(decoded.LatePolicyStatus, &value); err != nil {
				return fmt.Errorf("late_policy_status must be a string or null: %w", err)
			}
			s.LatePolicyStatus = &value
		}
	}
	s.LateKnown = len(decoded.Late) > 0
	if s.LateKnown {
		if err := json.Unmarshal(decoded.Late, &s.Late); err != nil {
			return fmt.Errorf("late must be a boolean: %w", err)
		}
	}
	s.SecondsLateKnown = len(decoded.SecondsLate) > 0
	if s.SecondsLateKnown {
		if err := json.Unmarshal(decoded.SecondsLate, &s.SecondsLate); err != nil {
			return fmt.Errorf("seconds_late must be an integer: %w", err)
		}
	}
	s.PostedAtPresent = len(decoded.PostedAt) > 0
	if s.PostedAtPresent {
		if bytes.Equal(bytes.TrimSpace(decoded.PostedAt), []byte("null")) {
			s.PostedAt = nil
		} else {
			var value string
			if err := json.Unmarshal(decoded.PostedAt, &value); err != nil {
				return fmt.Errorf("posted_at must be a timestamp or null: %w", err)
			}
			s.PostedAt = &value
		}
	}
	return nil
}

type SubmissionGroup struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}
type RubricScore struct {
	Points   *json.Number `json:"points"`
	RatingID string       `json:"rating_id"`
	Comments string       `json:"comments"`
}

func Courses(ctx context.Context, c *canvas.Client) ([]Course, error) {
	var result []Course
	err := c.List(ctx, "/api/v1/courses?enrollment_type=teacher&enrollment_state=active&per_page=100", &result)
	return result, err
}
func Modules(ctx context.Context, c *canvas.Client, courseID int64) ([]Module, error) {
	var result []Module
	err := c.List(ctx, fmt.Sprintf("/api/v1/courses/%d/modules?per_page=100", courseID), &result)
	return result, err
}
func Assignments(ctx context.Context, c *canvas.Client, courseID int64) ([]Assignment, error) {
	var result []Assignment
	err := c.List(ctx, fmt.Sprintf("/api/v1/courses/%d/assignments?per_page=100&include[]=all_dates", courseID), &result)
	return result, err
}

type AssignmentGroup struct {
	ID     int64       `json:"id"`
	Name   string      `json:"name"`
	Weight json.Number `json:"group_weight"`
}

func AssignmentGroups(ctx context.Context, c *canvas.Client, courseID int64) ([]AssignmentGroup, error) {
	var result []AssignmentGroup
	err := c.List(ctx, fmt.Sprintf("/api/v1/courses/%d/assignment_groups?per_page=100", courseID), &result)
	return result, err
}
func GetAssignment(ctx context.Context, c *canvas.Client, courseID, id int64) (Assignment, error) {
	var result Assignment
	err := c.Get(ctx, fmt.Sprintf("/api/v1/courses/%d/assignments/%d?include[]=rubric_assessment&include[]=checkpoints&include[]=all_dates&include[]=overrides&include[]=discussion_topic", courseID, id), &result)
	if err != nil {
		return result, err
	}
	if result.CourseID == 0 {
		result.CourseID = courseID
	}
	// Assignment payloads commonly omit the association setting. Read the
	// documented rubric association instead of assuming independence.
	if len(result.Rubric) > 0 && (result.RubricAssociation == nil || result.RubricAssociation.UseForGrading == nil) && result.RubricSettings.ID > 0 {
		var rubric struct {
			Associations []RubricAssociation `json:"associations"`
		}
		err = c.Get(ctx, fmt.Sprintf("/api/v1/courses/%d/rubrics/%d?include[]=assignment_associations", courseID, result.RubricSettings.ID), &rubric)
		if err != nil {
			return result, err
		}
		matches := []RubricAssociation{}
		for _, association := range rubric.Associations {
			if association.AssociationType == "Assignment" && association.AssociationID == id && association.UseForGrading != nil {
				matches = append(matches, association)
			}
		}
		if len(matches) == 1 {
			result.RubricAssociation = &matches[0]
		}
	}
	return result, nil
}
func Submissions(ctx context.Context, c *canvas.Client, courseID, id int64, detailed bool) ([]Submission, error) {
	path := fmt.Sprintf("/api/v1/courses/%d/assignments/%d/submissions?per_page=100&include[]=user&include[]=visibility", courseID, id)
	if detailed {
		path += "&include[]=submission_history&include[]=submission_comments&include[]=rubric_assessment"
	}
	var result []Submission
	err := c.List(ctx, path, &result)
	return result, err
}
func GroupSubmissions(ctx context.Context, c *canvas.Client, courseID, id int64) ([]Submission, error) {
	path := fmt.Sprintf("/api/v1/courses/%d/assignments/%d/submissions?per_page=100&include[]=user&include[]=group&include[]=visibility&include[]=submission_history&include[]=submission_comments&include[]=rubric_assessment", courseID, id)
	var result []Submission
	err := c.List(ctx, path, &result)
	return result, err
}
func GetGroupSubmission(ctx context.Context, c *canvas.Client, courseID, assignmentID, userID int64) (Submission, error) {
	var result Submission
	path := fmt.Sprintf("/api/v1/courses/%d/assignments/%d/submissions/%d?include[]=user&include[]=group&include[]=visibility&include[]=submission_history&include[]=submission_comments&include[]=rubric_assessment", courseID, assignmentID, userID)
	err := c.Get(ctx, path, &result)
	return result, err
}
func GetSubmission(ctx context.Context, c *canvas.Client, courseID, id, userID int64) (Submission, error) {
	var result Submission
	err := c.Get(ctx, fmt.Sprintf("/api/v1/courses/%d/assignments/%d/submissions/%d?include[]=submission_history&include[]=submission_comments&include[]=rubric_assessment&include[]=user&include[]=visibility", courseID, id, userID), &result)
	return result, err
}

// FindAssignment resolves only an ID/link or a unique exact title. Fuzzy choices
// are surfaced to the agent rather than silently selecting an arbitrary object.
func FindAssignment(ctx context.Context, c *canvas.Client, courseID int64, query string) (Assignment, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return Assignment{}, fmt.Errorf("assignment title, ID, or link is required")
	}
	if id, err := strconv.ParseInt(query, 10, 64); err == nil && id > 0 {
		return GetAssignment(ctx, c, courseID, id)
	}
	if u, err := url.Parse(query); err == nil && u.IsAbs() {
		origin, _ := url.Parse(c.Origin())
		if !strings.EqualFold(u.Host, origin.Host) || u.Scheme != origin.Scheme || u.User != nil {
			return Assignment{}, fmt.Errorf("assignment link must use the configured Canvas origin")
		}
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) == 4 && parts[0] == "courses" && parts[2] == "assignments" {
			cid, e1 := strconv.ParseInt(parts[1], 10, 64)
			id, e2 := strconv.ParseInt(parts[3], 10, 64)
			if e1 == nil && e2 == nil && cid == courseID && id > 0 {
				return GetAssignment(ctx, c, courseID, id)
			}
		}
		return Assignment{}, fmt.Errorf("assignment link does not identify an assignment in the requested course")
	}
	all, err := Assignments(ctx, c, courseID)
	if err != nil {
		return Assignment{}, err
	}
	var matches []Assignment
	for _, a := range all {
		if strings.EqualFold(strings.TrimSpace(a.Name), query) {
			matches = append(matches, a)
		}
	}
	if len(matches) == 1 {
		return GetAssignment(ctx, c, courseID, matches[0].ID)
	}
	if len(matches) == 0 {
		for _, a := range all {
			if strings.Contains(strings.ToLower(a.Name), strings.ToLower(query)) {
				matches = append(matches, a)
			}
		}
	}
	var candidates []string
	for _, a := range matches {
		candidates = append(candidates, fmt.Sprintf("%d: %s", a.ID, a.Name))
		if len(candidates) == 12 {
			break
		}
	}
	if len(candidates) == 0 {
		return Assignment{}, fmt.Errorf("no matching assignment; use its Canvas ID or exact title")
	}
	return Assignment{}, fmt.Errorf("select an assignment by ID; candidates: %s", strings.Join(candidates, "; "))
}

// unpullableAssignment lists only what makes reading student work pointless or
// unsafe. Every submission type and grading type is read; what the Engine
// cannot open is reported per student on the pulled work.
func unpullableAssignment(a Assignment) string {
	if a.Anonymous {
		return "anonymous grading isn't supported yet"
	}
	if a.Moderated {
		return "moderated grading isn't supported yet"
	}
	if a.GroupCategoryID != nil && *a.GroupCategoryID != 0 {
		return "group submissions aren't supported yet"
	}
	if a.QuizID != nil && !a.NewQuiz {
		return "quizzes aren't supported yet"
	}
	if a.NewQuiz || len(a.SubmissionTypes) == 0 {
		return ""
	}
	for _, t := range a.SubmissionTypes {
		if t != "none" && t != "on_paper" {
			return ""
		}
	}
	return "no student submission (none or on paper)"
}

func UnsupportedAssignment(a Assignment) string {
	if a.Anonymous {
		return "anonymous grading isn't supported yet"
	}
	if a.Moderated {
		return "moderated grading isn't supported yet"
	}
	if a.QuizID != nil && !a.NewQuiz {
		return "quizzes aren't supported yet"
	}
	if a.GradingType != "points" && a.GradingType != "percent" {
		return "requires numeric points or percent grading"
	}
	if a.NewQuiz {
		return ""
	}
	if len(a.SubmissionTypes) == 0 {
		return "no supported submission mode"
	}
	for _, t := range a.SubmissionTypes {
		if t != "online_text_entry" && t != "online_upload" {
			return "unsupported submission mode: " + t
		}
	}
	return ""
}
