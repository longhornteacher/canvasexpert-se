package workflow

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

// BuildEditReview previews an assignment update as a diff of Canvas Live against
// the proposed change. It reads the live assignment (no writes) so the teacher
// sees what is true now, and flags a conflict that would make push hold.
func (s *Service) BuildEditReview(ctx context.Context, path string) (ReviewModel, error) {
	var doc domain.AssignmentDocument
	body, e := workspace.ReadDocument(path, &doc)
	if e != nil {
		return ReviewModel{}, e
	}
	if doc.Operation != "update" {
		return ReviewModel{}, fmt.Errorf("edit review needs operation: update")
	}
	if definite(doc.UpdateOutcome) {
		return ReviewModel{}, fmt.Errorf("this edit was already applied; pull-item before another edit")
	}
	patch, e := PlanEdit(doc, body)
	if e != nil {
		return ReviewModel{}, e
	}
	if len(patch) == 0 {
		return ReviewModel{}, fmt.Errorf("this edit changes nothing compared with the pulled assignment")
	}
	course, e := s.Course(doc.Course)
	if e != nil {
		return ReviewModel{}, e
	}
	s, e = s.ForCourse(ctx, course)
	if e != nil {
		return ReviewModel{}, e
	}
	if doc.Instance != s.Client.Origin() || doc.CourseID != course.ID {
		return ReviewModel{}, fmt.Errorf("update identity differs from configured Canvas/course")
	}
	a, e := GetAssignment(ctx, s.Client, doc.CourseID, doc.AssignmentID)
	if e != nil {
		return ReviewModel{}, e
	}
	live, e := Snapshot(a, s.Client.Origin())
	if e != nil {
		return ReviewModel{}, e
	}
	base := *doc.Observed
	base.UpdatedAt, live.UpdatedAt = "", ""
	m := ReviewModel{Kind: "Edit to an existing assignment", Title: live.Title, AllOrNothing: true, Action: "Apply edit",
		Notice: "Only the fields listed here change; everything else in Canvas is left as it is. \"Now in Canvas\" was read just now. HTML is shown as the Engine will write it, not as Canvas's editor will render it."}
	head := ReviewCard{Ref: "edit", Alias: live.Title, Operation: "edit-assignment", Facts: []ReviewFact{fact("Assignment", fmt.Sprintf("%d in %s", doc.AssignmentID, doc.Course)), fact("Fields changing", strings.Join(patchFields(patch), ", "))}}
	if !sameEditFields(base, live, patch) {
		head.Hold = "Canvas changed these fields after this edit was pulled. Applying it would be held and your edit kept. Pull the assignment again and redo the edit."
	}
	m.Cards = append(m.Cards, head)
	cur := live
	for _, key := range patchFields(patch) {
		c := ReviewCard{Ref: "field-" + key, Alias: fieldLabel(key), Operation: "edit-field"}
		switch key {
		case "description":
			c.Frames = []ReviewFrame{{Label: "Now in Canvas", HTML: cur.DescriptionHTML}, {Label: "After this edit", HTML: body}}
		case "name":
			c.Facts = []ReviewFact{fact("Now in Canvas", cur.Title), fact("After this edit", doc.Title)}
		case "points_possible":
			c.Facts = []ReviewFact{fact("Now in Canvas", cur.Points.String()), fact("After this edit", doc.Points.String())}
			c.Total = cur.Points.String() + " -> " + doc.Points.String()
		case "submission_types":
			c.Facts = []ReviewFact{fact("Now in Canvas", strings.Join(cur.SubmissionTypes, ", ")), fact("After this edit", strings.Join(doc.SubmissionTypes, ", "))}
		case "due_at":
			c.Facts = []ReviewFact{fact("Now in Canvas", optional(cur.DueAt)), fact("After this edit", optional(doc.DueAt))}
		case "unlock_at":
			c.Facts = []ReviewFact{fact("Now in Canvas", optional(cur.UnlockAt)), fact("After this edit", optional(doc.UnlockAt))}
		case "lock_at":
			c.Facts = []ReviewFact{fact("Now in Canvas", optional(cur.LockAt)), fact("After this edit", optional(doc.LockAt))}
		case "published":
			c.Facts = []ReviewFact{fact("Now in Canvas", fmt.Sprint(cur.Published)), fact("After this edit", fmt.Sprint(doc.Published))}
			if doc.Published {
				c.Facts = append(c.Facts, fact("Note", "Publishing makes the assignment visible to students."))
			}
		case "group_category_id":
			c.Facts = []ReviewFact{fact("Now in Canvas", fmt.Sprint(cur.GroupCategoryID)), fact("After this edit", fmt.Sprint(doc.GroupCategoryID))}
		case "assignment_group_id":
			c.Facts = []ReviewFact{fact("Now in Canvas", optionalAssignmentGroup(cur.AssignmentGroupID)), fact("After this edit", optionalAssignmentGroup(doc.AssignmentGroupID)), fact("Note", "Moving this assignment between weighted assignment groups can change course grades, including grades for existing submitted work.")}
		case "grade_group_students_individually":
			c.Facts = []ReviewFact{fact("Now in Canvas", optionalBool(cur.GradeGroupStudentsIndividually)), fact("After this edit", optionalBool(doc.GradeGroupStudentsIndividually))}
		}
		m.Cards = append(m.Cards, c)
	}
	return m, nil
}

func patchFields(patch map[string]any) []string {
	var keys []string
	for k := range patch {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func fieldLabel(key string) string {
	return map[string]string{"name": "Title", "description": "Directions", "points_possible": "Points", "submission_types": "Submission types",
		"due_at": "Due date", "unlock_at": "Available from", "lock_at": "Available until", "published": "Published", "assignment_group_id": "Assignment group", "group_category_id": "Collaborative group set", "grade_group_students_individually": "Grade members individually"}[key]
}

func optionalAssignmentGroup(id *int64) string {
	if id == nil {
		return "Canvas default / not observed"
	}
	return fmt.Sprint(*id)
}

func optionalBool(value *bool) string {
	if value == nil {
		return "unknown"
	}
	return fmt.Sprint(*value)
}
