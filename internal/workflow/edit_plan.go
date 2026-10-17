package workflow

import (
	"fmt"
	"html"
	"reflect"
	"regexp"
	"strings"
	"time"
	"unicode"

	"canvasexpertse/internal/domain"
)

// PlanEdit produces the smallest explicit patch against a pulled baseline.
// It performs no network calls. A caller must compare the live baseline again
// immediately before sending it; no atomic read/write guarantee is implied.
func PlanEdit(doc domain.AssignmentDocument, html string) (map[string]any, error) {
	if err := domain.ValidateAssignment(doc, html); err != nil {
		return nil, err
	}
	if doc.Operation != "update" || doc.Observed == nil {
		return nil, fmt.Errorf("edit requires a freshly pulled assignment")
	}
	base := doc.Observed
	for _, t := range base.SubmissionTypes {
		if t == "external_tool" {
			return nil, fmt.Errorf("a New Quiz can only gain a rubric here (operation: attach-rubric); edit the quiz itself in Canvas")
		}
	}
	if !sameRubric(doc.Rubric, base.Rubric) {
		return nil, fmt.Errorf("editing an existing rubric's criteria isn't supported yet; keep the pulled rubric")
	}
	if doc.ModuleID != 0 {
		return nil, fmt.Errorf("moving an existing assignment between modules isn't supported yet")
	}
	patch := map[string]any{}
	if doc.Title != base.Title {
		patch["name"] = doc.Title
	}
	if html != base.DescriptionHTML {
		patch["description"] = html
	}
	if doc.Points.Cmp(base.Points) != 0 {
		patch["points_possible"] = doc.Points
	}
	if !reflect.DeepEqual(doc.SubmissionTypes, base.SubmissionTypes) {
		patch["submission_types"] = doc.SubmissionTypes
	}
	if !sameDate(doc.DueAt, base.DueAt) {
		patch["due_at"] = doc.DueAt
	}
	if !sameDate(doc.UnlockAt, base.UnlockAt) {
		patch["unlock_at"] = doc.UnlockAt
	}
	if !sameDate(doc.LockAt, base.LockAt) {
		patch["lock_at"] = doc.LockAt
	}
	if doc.Published != base.Published {
		patch["published"] = doc.Published
	}
	if doc.GroupCategoryID > 0 && doc.GroupCategoryID != base.GroupCategoryID {
		patch["group_category_id"] = doc.GroupCategoryID
	}
	if doc.GradeGroupStudentsIndividually != nil && !sameOptionalBool(doc.GradeGroupStudentsIndividually, base.GradeGroupStudentsIndividually) {
		patch["grade_group_students_individually"] = *doc.GradeGroupStudentsIndividually
	}
	return patch, nil
}

// SameGradingBasis compares the fields posting arithmetic or writes depend on:
// points, rubric, grading type, submission types and grading modes. Directions,
// title and scheduling are ignored.
func SameGradingBasis(a, b domain.AssignmentSnapshot) bool {
	a.UpdatedAt, b.UpdatedAt = "", ""
	a.Title, b.Title = "", ""
	a.Published, b.Published = false, false
	a.DueAt, b.DueAt = nil, nil
	a.UnlockAt, b.UnlockAt = nil, nil
	a.LockAt, b.LockAt = nil, nil
	if a.Rubric != nil {
		r := *a.Rubric
		r.UseForGrading, r.UseForGradingKnown = false, false
		a.Rubric = &r
	}
	if b.Rubric != nil {
		r := *b.Rubric
		r.UseForGrading, r.UseForGradingKnown = false, false
		b.Rubric = &r
	}
	a.DescriptionHTML, b.DescriptionHTML = "", ""
	// Old individual snapshots predate this field. Canvas commonly returns false
	// for an individual assignment, so absence is equivalent only off-group.
	if a.GroupCategoryID == 0 && b.GroupCategoryID == 0 {
		a.GradeGroupStudentsIndividually, b.GradeGroupStudentsIndividually = nil, nil
	}
	return SameSnapshot(a, b)
}

// Only fields being replaced can conflict. All other current fields are preserved.
func sameEditFields(a, b domain.AssignmentSnapshot, patch map[string]any) bool {
	if a.Instance != b.Instance || a.CourseID != b.CourseID || a.AssignmentID != b.AssignmentID {
		return false
	}
	for key := range patch {
		switch key {
		case "name":
			if a.Title != b.Title {
				return false
			}
		case "description":
			if academicHTML(a.DescriptionHTML) != academicHTML(b.DescriptionHTML) {
				return false
			}
		case "points_possible":
			if a.Points.Cmp(b.Points) != 0 {
				return false
			}
		case "submission_types":
			if !reflect.DeepEqual(a.SubmissionTypes, b.SubmissionTypes) {
				return false
			}
		case "due_at":
			if !sameDate(a.DueAt, b.DueAt) {
				return false
			}
		case "unlock_at":
			if !sameDate(a.UnlockAt, b.UnlockAt) {
				return false
			}
		case "lock_at":
			if !sameDate(a.LockAt, b.LockAt) {
				return false
			}
		case "published":
			if a.Published != b.Published {
				return false
			}
		case "group_category_id":
			if a.GroupCategoryID != b.GroupCategoryID {
				return false
			}
		case "grade_group_students_individually":
			if !sameOptionalBool(a.GradeGroupStudentsIndividually, b.GradeGroupStudentsIndividually) {
				return false
			}
		}
	}
	return true
}

var ordinaryTags = regexp.MustCompile(`(?s)<[^>]*>`)
var quotedAttribute = regexp.MustCompile(`([[:alnum:]_:-]+)\s*=\s*'([^']*)'`)
var whitespaceRun = regexp.MustCompile(`\s+`)
var tagGap = regexp.MustCompile(`\s*(/?>)$`)
var attrEquals = regexp.MustCompile(`\s*=\s*`)

// Canvas stores a course link as an absolute URL and adds data-api-* attributes
// to it; neither is a teacher edit.
var canvasLinkAttrs = regexp.MustCompile(`\s+data-api-(?:endpoint|returntype)="[^"]*"`)
var canvasAbsoluteLink = regexp.MustCompile(`href="https?://[^"/]+(/courses/)`)

// Modest representation normalization, retaining tags, attributes (including URLs)
// and code whitespace. Whitespace between text and inline tags is kept as one
// space. This deliberately does not infer semantic equivalence.
func academicHTML(source string) string {
	source = canvasAbsoluteLink.ReplaceAllString(canvasLinkAttrs.ReplaceAllString(source, ""), `href="$1`)
	var out strings.Builder
	at, literal := 0, 0
	for _, loc := range ordinaryTags.FindAllStringIndex(source, -1) {
		text := html.UnescapeString(source[at:loc[0]])
		if literal == 0 {
			text = whitespaceRun.ReplaceAllString(text, " ")
		}
		out.WriteString(text)
		tag := source[loc[0]:loc[1]]
		lower := strings.ToLower(tag)
		if strings.HasPrefix(lower, "</pre") || strings.HasPrefix(lower, "</code") {
			if literal > 0 {
				literal--
			}
		}
		if literal == 0 {
			tag = quotedAttribute.ReplaceAllString(tag, `$1="$2"`)
			tag = normalizeTag(tag)
		}
		out.WriteString(tag)
		if strings.HasPrefix(lower, "<pre>") || strings.HasPrefix(lower, "<pre ") || strings.HasPrefix(lower, "<code>") || strings.HasPrefix(lower, "<code ") {
			literal++
		}
		at = loc[1]
	}
	text := html.UnescapeString(source[at:])
	if literal == 0 {
		text = whitespaceRun.ReplaceAllString(text, " ")
	}
	out.WriteString(text)
	return strings.TrimSpace(out.String())
}

// normalizeTag collapses whitespace inside a tag outside quoted attribute values.
func normalizeTag(tag string) string {
	var out strings.Builder
	var quote rune
	space := false
	for _, r := range tag {
		switch {
		case quote != 0:
			out.WriteRune(r)
			if r == quote {
				quote = 0
			}
		case r == '"' || r == '\'':
			quote = r
			out.WriteRune(r)
		case unicode.IsSpace(r):
			space = true
		default:
			if space {
				out.WriteRune(' ')
				space = false
			}
			out.WriteRune(r)
		}
	}
	t := out.String()
	t = strings.Replace(t, "< ", "<", 1)
	t = tagGap.ReplaceAllString(t, "$1")
	return t
}

// SameSnapshot compares numeric values and date instants without mistaking API
// formatting changes for teacher edits. Other content and identities stay exact.
func SameSnapshot(a, b domain.AssignmentSnapshot) bool {
	if a.Points.Cmp(b.Points) != 0 || !sameRubric(a.Rubric, b.Rubric) {
		return false
	}
	a.Points = domain.Decimal{}
	b.Points = domain.Decimal{}
	a.Rubric = nil
	b.Rubric = nil
	a.DueAt = canonicalDate(a.DueAt)
	b.DueAt = canonicalDate(b.DueAt)
	a.UnlockAt = canonicalDate(a.UnlockAt)
	b.UnlockAt = canonicalDate(b.UnlockAt)
	a.LockAt = canonicalDate(a.LockAt)
	b.LockAt = canonicalDate(b.LockAt)
	if a.GroupCategoryID == 0 && b.GroupCategoryID == 0 {
		a.GradeGroupStudentsIndividually, b.GradeGroupStudentsIndividually = nil, nil
	}
	// These fields describe discussion/checkpoint wiring, not grading. Older
	// packets and Canvas responses may omit them; absence on either side is
	// treated as unknown for compatibility, while two observed values compare.
	if a.DiscussionTopicID == nil || b.DiscussionTopicID == nil {
		a.DiscussionTopicID, b.DiscussionTopicID = nil, nil
	}
	if a.HasSubAssignments == nil || b.HasSubAssignments == nil {
		a.HasSubAssignments, b.HasSubAssignments = nil, nil
	}
	if a.Checkpoints == nil || b.Checkpoints == nil {
		a.Checkpoints, b.Checkpoints = nil, nil
	}
	return reflect.DeepEqual(a, b)
}
func sameOptionalBool(a, b *bool) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}
func canonicalDate(v *string) *string {
	if v == nil || *v == "" {
		return nil
	}
	t, e := time.Parse(time.RFC3339Nano, *v)
	if e != nil {
		return v
	}
	s := t.UTC().Truncate(time.Second).Format(time.RFC3339)
	return &s
}
func sameRubric(a, b *domain.Rubric) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	ac, bc := *a, *b
	if len(ac.Criteria) != len(bc.Criteria) {
		return false
	}
	ac.Criteria = append([]domain.Criterion(nil), ac.Criteria...)
	bc.Criteria = append([]domain.Criterion(nil), bc.Criteria...)
	for i := range ac.Criteria {
		x, y := &ac.Criteria[i], &bc.Criteria[i]
		if x.Points.Cmp(y.Points) != 0 || len(x.Ratings) != len(y.Ratings) {
			return false
		}
		x.Points = domain.Decimal{}
		y.Points = domain.Decimal{}
		x.Ratings = append([]domain.Rating(nil), x.Ratings...)
		y.Ratings = append([]domain.Rating(nil), y.Ratings...)
		for j := range x.Ratings {
			if x.Ratings[j].Points.Cmp(y.Ratings[j].Points) != 0 {
				return false
			}
			x.Ratings[j].Points = domain.Decimal{}
			y.Ratings[j].Points = domain.Decimal{}
		}
	}
	return reflect.DeepEqual(ac, bc)
}
