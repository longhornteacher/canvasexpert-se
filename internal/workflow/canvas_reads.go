// Package workflow coordinates Canvas operations and workspace documents.
// Canvas DTOs stay here so API responses are never written wholesale to agent files.
package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"canvasexpertse/internal/canvas"
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
	ID              int64       `json:"id"`
	CourseID        int64       `json:"course_id"`
	Name            string      `json:"name"`
	Description     string      `json:"description"`
	Points          json.Number `json:"points_possible"`
	SubmissionTypes []string    `json:"submission_types"`
	Published       bool        `json:"published"`
	DueAt           *string     `json:"due_at"`
	UnlockAt        *string     `json:"unlock_at"`
	LockAt          *string     `json:"lock_at"`
	UpdatedAt       string      `json:"updated_at"`
	CreatedAt       string      `json:"created_at"`
	HTMLURL         string      `json:"html_url"`
	GradingType     string      `json:"grading_type"`
	GroupCategoryID *int64      `json:"group_category_id"`
	Anonymous       bool        `json:"anonymous_grading"`
	Moderated       bool        `json:"moderated_grading"`
	QuizID          *int64      `json:"quiz_id"`
	NewQuiz         bool        `json:"is_quiz_lti_assignment"`
	Rubric          []Criterion `json:"rubric"`
	RubricSettings  struct {
		ID       int64       `json:"id"`
		Title    string      `json:"title"`
		Points   json.Number `json:"points_possible"`
		FreeForm bool        `json:"free_form_criterion_comments"`
	} `json:"rubric_settings"`
	RubricAssociation *RubricAssociation `json:"rubric_association"`
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
	Type                          string                 `json:"submission_type"`
	State                         string                 `json:"workflow_state"`
	Score                         *json.Number           `json:"score"`
	EnteredScore                  *json.Number           `json:"entered_score"`
	PointsDeducted                *json.Number           `json:"points_deducted"`
	Excused                       bool                   `json:"excused"`
	Missing                       bool                   `json:"missing"`
	Late                          bool                   `json:"late"`
	LatePolicyStatus              *string                `json:"late_policy_status"`
	SecondsLate                   int64                  `json:"seconds_late"`
	GradeMatchesCurrentSubmission *bool                  `json:"grade_matches_current_submission"`
	AssignmentVisible             *bool                  `json:"assignment_visible"`
	User                          CanvasUser             `json:"user"`
	History                       []Submission           `json:"submission_history"`
	Attachments                   []Attachment           `json:"attachments"`
	Comments                      []SubmissionComment    `json:"submission_comments"`
	RubricAssessment              map[string]RubricScore `json:"rubric_assessment"`
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
	err := c.List(ctx, fmt.Sprintf("/api/v1/courses/%d/assignments?per_page=100", courseID), &result)
	return result, err
}
func GetAssignment(ctx context.Context, c *canvas.Client, courseID, id int64) (Assignment, error) {
	var result Assignment
	err := c.Get(ctx, fmt.Sprintf("/api/v1/courses/%d/assignments/%d?include[]=rubric_assessment", courseID, id), &result)
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
	path := fmt.Sprintf("/api/v1/courses/%d/assignments/%d/submissions?per_page=100&include[]=user", courseID, id)
	if detailed {
		path += "&include[]=submission_history&include[]=submission_comments&include[]=rubric_assessment"
	}
	var result []Submission
	err := c.List(ctx, path, &result)
	return result, err
}
func GetSubmission(ctx context.Context, c *canvas.Client, courseID, id, userID int64) (Submission, error) {
	var result Submission
	err := c.Get(ctx, fmt.Sprintf("/api/v1/courses/%d/assignments/%d/submissions/%d?include[]=submission_history&include[]=submission_comments&include[]=rubric_assessment&include[]=user", courseID, id, userID), &result)
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

func UnsupportedAssignment(a Assignment) string {
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
