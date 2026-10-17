package workflow

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"canvasexpertse/internal/canvas"
	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

type Service struct {
	Root   string
	Client *canvas.Client
	Config domain.CourseConfig
	// Skip lists student refs post must leave untouched (review: not approved).
	Skip map[string]bool
}

func (s *Service) Course(nickname string) (domain.Course, error) {
	for _, course := range s.Config.Courses {
		if strings.EqualFold(course.Nickname, nickname) {
			return course, nil
		}
	}
	return domain.Course{}, fmt.Errorf("course %q is not configured in context/courses.md", nickname)
}

// Slug is friendly decoration only. Stable remote IDs supply folder identity.
func Slug(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(unicode.ToLower(r))
		} else if b.Len() > 0 && !strings.HasSuffix(b.String(), "-") {
			b.WriteByte('-')
		}
		if b.Len() >= 64 {
			break
		}
	}
	result := strings.Trim(b.String(), "- .")
	if result == "" {
		result = "item"
	}
	return result
}

func Snapshot(a Assignment, origin string) (domain.AssignmentSnapshot, error) {
	points, err := domain.ParseDecimal(a.Points.String())
	if err != nil {
		return domain.AssignmentSnapshot{}, fmt.Errorf("assignment %d has no valid points possible", a.ID)
	}
	v := domain.AssignmentSnapshot{Instance: origin, CourseID: a.CourseID, AssignmentID: a.ID, Title: a.Name, Points: points, DescriptionHTML: a.Description, SubmissionTypes: a.SubmissionTypes, DueAt: a.DueAt, UnlockAt: a.UnlockAt, LockAt: a.LockAt, Published: a.Published, UpdatedAt: a.UpdatedAt, NewQuiz: a.NewQuiz}
	v.GradingType = a.GradingType
	v.AnonymousGrading = a.Anonymous
	v.ModeratedGrading = a.Moderated
	if a.GroupCategoryID != nil {
		v.GroupCategoryID = *a.GroupCategoryID
	}
	v.GradeGroupStudentsIndividually = a.GradeGroupStudentsIndividually
	v.DiscussionTopicID = a.DiscussionTopicID
	v.HasSubAssignments = a.HasSubAssignments
	v.Checkpoints = a.Checkpoints
	if len(a.Rubric) > 0 {
		r := domain.Rubric{ID: a.RubricSettings.ID, Title: a.RubricSettings.Title, FreeFormCriterionComments: a.RubricSettings.FreeForm}
		if a.RubricAssociation != nil {
			r.AssociationID = a.RubricAssociation.ID
			if a.RubricAssociation.UseForGrading != nil {
				r.UseForGrading = *a.RubricAssociation.UseForGrading
				r.UseForGradingKnown = true
			}
			if r.ID == 0 {
				r.ID = a.RubricAssociation.RubricID
			}
		}
		for _, criterion := range a.Rubric {
			max, err := domain.ParseDecimal(criterion.Points.String())
			if err != nil {
				return v, fmt.Errorf("rubric criterion %s has invalid maximum", criterion.ID)
			}
			c := domain.Criterion{ID: criterion.ID, Points: max, Description: criterion.Description, LongDescription: criterion.LongDescription, UseRange: criterion.Range}
			for _, rating := range criterion.Ratings {
				rp, err := domain.ParseDecimal(rating.Points.String())
				if err != nil {
					return v, fmt.Errorf("rubric rating %s has invalid points", rating.ID)
				}
				c.Ratings = append(c.Ratings, domain.Rating{ID: rating.ID, Points: rp, Description: rating.Description, LongDescription: rating.LongDescription})
			}
			r.Criteria = append(r.Criteria, c)
		}
		v.Rubric = &r
	}
	return v, nil
}

// PullItem writes a fresh file every time; an earlier unsent edit remains intact.
func (s *Service) PullItem(ctx context.Context, nickname, query string) (string, error) {
	course, err := s.Course(nickname)
	if err != nil {
		return "", err
	}
	nickname = course.Nickname
	a, err := FindAssignment(ctx, s.Client, course.ID, query)
	if err != nil {
		return "", err
	}
	// A New Quiz is pulled only so a rubric can be attached; PlanEdit refuses
	// updates to it.
	if reason := UnsupportedAssignment(a); reason != "" && !a.NewQuiz {
		return "", fmt.Errorf("assignment %d: %s", a.ID, reason)
	}
	snapshot, err := Snapshot(a, s.Client.Origin())
	if err != nil {
		return "", err
	}
	doc := domain.AssignmentDocument{Schema: 1, Kind: "assignment", Operation: "update", Course: nickname, Instance: s.Client.Origin(), CourseID: course.ID, AssignmentID: a.ID, Title: a.Name, Points: snapshot.Points, SubmissionTypes: a.SubmissionTypes, DueAt: a.DueAt, UnlockAt: a.UnlockAt, LockAt: a.LockAt, Published: a.Published, GroupCategoryID: snapshot.GroupCategoryID, GradeGroupStudentsIndividually: snapshot.GradeGroupStudentsIndividually, Rubric: snapshot.Rubric, Observed: &snapshot, Timezone: course.Timezone}
	if snapshot.GroupCategoryID == 0 {
		doc.GradeGroupStudentsIndividually = nil
	}
	name := fmt.Sprintf("%d-%s-%s.md", a.ID, Slug(a.Name), time.Now().UTC().Format("20060102T150405.000000000"))
	path, err := workspace.ConstrainPath(s.Root, filepath.Join("courses", nickname, "pulled", name))
	if err != nil {
		return "", err
	}
	if err = workspace.WriteDocument(path, doc, a.Description); err != nil {
		return "", err
	}
	return path, nil
}
