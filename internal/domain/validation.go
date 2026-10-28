package domain

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

var unsafeNicknameDevice = regexp.MustCompile(`(?i)^(CON|PRN|AUX|NUL|CONIN\$|CONOUT\$|COM[1-9¹²³]|LPT[1-9¹²³])(?: *\..*)?$`)

func validNickname(s string) bool {
	return s != "" && strings.TrimSpace(s) == s && s != "." && s != ".." && !strings.HasSuffix(s, ".") && !strings.ContainsAny(s, `/\<>:"|?*`) && strings.IndexFunc(s, func(r rune) bool { return r < 32 }) < 0 && !unsafeNicknameDevice.MatchString(s)
}

func ValidateConfig(c CourseConfig) error {
	if c.Schema != 1 {
		return fmt.Errorf("schema must be 1")
	}
	if !validInstance(c.Instance) {
		return fmt.Errorf("instance must be an HTTPS Canvas origin")
	}
	names, ids := map[string]bool{}, map[int64]bool{}
	for i, course := range c.Courses {
		name := strings.ToLower(course.Nickname)
		if !validNickname(course.Nickname) || course.ID <= 0 || names[name] || ids[course.ID] {
			return fmt.Errorf("courses[%d]: invalid/duplicate nickname or id", i)
		}
		names[name] = true
		ids[course.ID] = true
		if course.Timezone != "" {
			if _, e := time.LoadLocation(course.Timezone); e != nil {
				return fmt.Errorf("courses[%d]: invalid timezone %q", i, course.Timezone)
			}
		}
		if f := course.EffortFloor; f != nil && (f.Sign() <= 0 || f.Cmp(MustDecimal("100")) > 0) {
			return fmt.Errorf("courses[%d]: effort_floor must be a percent above 0 and at most 100", i)
		}
	}
	return nil
}
func (c *CourseConfig) Validate() error { return ValidateConfig(*c) }
func (c *GradingConfig) Validate() error {
	if c.Schema != 1 {
		return fmt.Errorf("schema must be 1")
	}
	return nil
}
func (s *StudentNotes) Validate() error {
	if s.Schema != 1 {
		return fmt.Errorf("schema must be 1")
	}
	for i, n := range s.Notes {
		if strings.TrimSpace(n.Note) == "" || (n.StudentRef == "" && strings.TrimSpace(n.Name) == "") {
			return fmt.Errorf("notes[%d]: explicit ref or name and note required", i)
		}
	}
	return nil
}
func ValidateSnapshot(s AssignmentSnapshot) error {
	if !validInstance(s.Instance) || s.CourseID <= 0 || s.AssignmentID <= 0 || strings.TrimSpace(s.Title) == "" {
		return fmt.Errorf("assignment requires instance, positive course_id/assignment_id, and title")
	}
	if !s.Points.IsSet() || s.Points.Sign() < 0 {
		return fmt.Errorf("assignment points must be nonnegative")
	}
	if len(s.SubmissionTypes) == 0 {
		return fmt.Errorf("assignment submission_types required")
	}
	seenTypes := map[string]bool{}
	for _, kind := range s.SubmissionTypes {
		if kind == "" || seenTypes[kind] {
			return fmt.Errorf("assignment submission_types must be nonempty and unique")
		}
		seenTypes[kind] = true
	}
	if s.GroupCategoryID < 0 {
		return fmt.Errorf("assignment group_category_id cannot be negative")
	}
	if s.AssignmentGroupID != nil && *s.AssignmentGroupID <= 0 {
		return fmt.Errorf("assignment assignment_group_id must be positive when supplied")
	}
	for _, raw := range []*string{s.DueAt, s.UnlockAt, s.LockAt} {
		if raw != nil && *raw != "" {
			if _, e := time.Parse(time.RFC3339Nano, *raw); e != nil {
				return fmt.Errorf("assignment dates must be RFC3339 with explicit offset: %w", e)
			}
		}
	}
	if s.Rubric != nil {
		return ValidateRubric(*s.Rubric)
	}
	return nil
}
func ValidateAssignment(d AssignmentDocument, body string) error {
	if err := ValidateAudience(d.Audience); err != nil {
		return err
	}
	if d.Audience != nil && (d.Operation != "create" || d.GroupCategoryID != 0 || d.GradeGroupStudentsIndividually != nil || len(d.Overrides) > 0 || (d.OnlyVisibleToOverrides != nil && !*d.OnlyVisibleToOverrides)) {
		return fmt.Errorf("individual audience requires assignment create without native group settings or explicit overrides; use assignment-dates for audience updates")
	}
	if d.Schema != 1 || d.Kind != "assignment" {
		return fmt.Errorf("expected schema: 1 and kind: assignment")
	}
	if d.Operation != "create" && d.Operation != "update" && d.Operation != "attach-rubric" {
		return fmt.Errorf("operation must be create, update, or attach-rubric")
	}
	if e := ValidateBodyFormat(d.BodyFormat); e != nil {
		return e
	}
	if d.Operation != "create" && d.BodyFormat != "" && d.BodyFormat != "html" {
		return fmt.Errorf("existing assignment documents use HTML; body_format must be html or omitted")
	}
	if !validNickname(d.Course) || strings.TrimSpace(d.Title) == "" || !d.Points.IsSet() || d.Points.Sign() < 0 {
		return fmt.Errorf("safe course nickname, title, and nonnegative points required")
	}
	if d.Operation == "create" && strings.TrimSpace(body) == "" {
		return fmt.Errorf("assignment teaching body cannot be empty")
	}
	if d.Instance != "" && !validInstance(d.Instance) {
		return fmt.Errorf("instance must be an HTTPS Canvas origin")
	}
	if d.CourseID < 0 || d.AssignmentID < 0 {
		return fmt.Errorf("course_id/assignment_id cannot be negative")
	}
	if len(d.SubmissionTypes) == 0 {
		return fmt.Errorf("submission_types required")
	}
	if containsSubmissionType(d.SubmissionTypes, "on_paper") && len(d.SubmissionTypes) != 1 {
		return fmt.Errorf("on_paper must be the only submission type; it cannot be mixed with online submission types")
	}
	seen := map[string]bool{}
	for _, s := range d.SubmissionTypes {
		// PlanEdit restricts a pulled New Quiz to assignment group and base dates.
		if s != "online_text_entry" && s != "online_upload" && s != "on_paper" && (s != "external_tool" || d.Operation == "create") {
			return fmt.Errorf("unsupported submission type %q", s)
		}
		if seen[s] {
			return fmt.Errorf("duplicate submission type %q", s)
		}
		seen[s] = true
	}
	if d.ModuleID < 0 {
		return fmt.Errorf("module_id cannot be negative")
	}
	if d.AssignmentGroupID != nil && *d.AssignmentGroupID <= 0 {
		return fmt.Errorf("assignment_group_id must be positive when supplied")
	}
	if d.GroupCategoryID < 0 {
		return fmt.Errorf("group_category_id cannot be negative")
	}
	if d.GroupCategoryID > 0 && d.GradeGroupStudentsIndividually == nil && (d.Operation == "create" || d.Observed == nil || d.Observed.GroupCategoryID != d.GroupCategoryID) {
		return fmt.Errorf("a group assignment requires an explicit grade_group_students_individually value")
	}
	if d.GradeGroupStudentsIndividually != nil && d.GroupCategoryID == 0 && (d.Operation == "create" || d.Observed == nil || d.Observed.GroupCategoryID == 0) {
		return fmt.Errorf("grade_group_students_individually requires a group assignment")
	}
	if d.GroupCategoryID > 0 && (d.Operation == "create" || d.Observed == nil) && containsSubmissionType(d.SubmissionTypes, "external_tool") {
		return fmt.Errorf("group mode cannot be applied to a New Quiz")
	}
	if d.Operation != "create" && (len(d.Overrides) > 0 || d.OnlyVisibleToOverrides != nil) {
		return fmt.Errorf("assignment-date audience changes use kind: assignment-dates")
	}
	if d.Operation == "create" {
		if d.OnlyVisibleToOverrides != nil && *d.OnlyVisibleToOverrides && len(d.Overrides) == 0 && d.Audience == nil {
			return fmt.Errorf("only_visible_to_overrides requires at least one requested override")
		}
		keys := map[string]bool{}
		for i, row := range d.Overrides {
			if row.Operation != "create" || row.OverrideID != 0 || row.Observed != nil {
				return fmt.Errorf("overrides[%d] on assignment creation must be a new create row", i)
			}
			if strings.TrimSpace(row.Key) == "" || keys[row.Key] {
				return fmt.Errorf("overrides[%d] requires a unique key", i)
			}
			keys[row.Key] = true
			if row.GroupID < 0 || row.CourseSectionID < 0 {
				return fmt.Errorf("overrides[%d] target IDs cannot be negative", i)
			}
			targets := 0
			if row.GroupID > 0 {
				targets++
			}
			if len(row.StudentRefs) > 0 {
				targets++
			}
			if row.CourseSectionID > 0 {
				targets++
			}
			if targets != 1 {
				return fmt.Errorf("overrides[%d] must choose exactly one target", i)
			}
			if len(row.StudentRefs) > 0 && strings.TrimSpace(row.Title) == "" {
				return fmt.Errorf("overrides[%d] student target requires a title", i)
			}
			if d.GroupCategoryID == 0 && row.GroupID > 0 {
				return fmt.Errorf("group override requires a group assignment")
			}
			if d.GroupCategoryID > 0 && row.GroupID == 0 && len(row.StudentRefs) == 0 && row.CourseSectionID == 0 {
				return fmt.Errorf("group assignment override requires exact target")
			}
			if !row.DueAt.Set && !row.UnlockAt.Set && !row.LockAt.Set && len(row.StudentRefs) == 0 {
				return fmt.Errorf("overrides[%d] requires at least one date", i)
			}
			for _, date := range []OptionalDate{row.DueAt, row.UnlockAt, row.LockAt} {
				if date.Null && !date.Set {
					return fmt.Errorf("overrides[%d] null date must also set the field", i)
				}
				if date.Set && !date.Null {
					if _, e := time.Parse(time.RFC3339, date.Value); e != nil {
						return fmt.Errorf("overrides[%d] date requires RFC3339 with explicit offset", i)
					}
				}
			}
		}
	}
	if d.Timezone != "" {
		if _, e := time.LoadLocation(d.Timezone); e != nil {
			return fmt.Errorf("invalid timezone %q", d.Timezone)
		}
	}
	var due, unlock, lock *time.Time
	for i, raw := range []*string{d.DueAt, d.UnlockAt, d.LockAt} {
		if raw == nil || *raw == "" {
			continue
		}
		t, e := time.Parse(time.RFC3339, *raw)
		if e != nil {
			return fmt.Errorf("dates require RFC3339 with explicit offset: %w", e)
		}
		switch i {
		case 0:
			due = &t
		case 1:
			unlock = &t
		case 2:
			lock = &t
		}
	}
	if unlock != nil && lock != nil && unlock.After(*lock) {
		return fmt.Errorf("unlock_at must precede lock_at")
	}
	if due != nil && lock != nil && due.After(*lock) {
		return fmt.Errorf("due_at must not follow lock_at")
	}
	if due != nil && unlock != nil && due.Before(*unlock) {
		return fmt.Errorf("due_at must not precede unlock_at")
	}
	if d.Rubric != nil {
		if e := ValidateRubric(*d.Rubric); e != nil {
			return e
		}
		if (d.Operation == "create" || d.Operation == "attach-rubric") && d.Rubric.UseForGrading {
			return fmt.Errorf("new rubrics require use_for_grading: false (or omitted): the Engine posts independent rubric assessments and separately computed grades; existing coupled rubrics remain supported")
		}
	}
	if e := validateTiers(d, body); e != nil {
		return e
	}
	if d.Operation == "update" || d.Operation == "attach-rubric" {
		if d.Observed == nil {
			return fmt.Errorf("update requires observed live baseline")
		}
		if e := ValidateSnapshot(*d.Observed); e != nil {
			return e
		}
		if d.Instance != d.Observed.Instance || d.CourseID != d.Observed.CourseID || d.AssignmentID != d.Observed.AssignmentID {
			return fmt.Errorf("update identity differs from observed baseline")
		}
	}
	if e := validateOutcome(d.UpdateOutcome); e != nil {
		return e
	}
	if d.UpdateOutcome.ID > 0 && d.UpdateOutcome.ID != d.AssignmentID {
		return fmt.Errorf("update_outcome id disagrees with assignment_id")
	}
	if d.Operation == "create" && d.UpdateOutcome.Status != "" {
		return fmt.Errorf("create cannot carry update_outcome")
	}
	if d.Operation == "update" && d.Created != nil {
		return fmt.Errorf("update cannot carry create outcomes")
	}
	if d.Operation == "attach-rubric" {
		return validateRubricAttachment(d, body)
	}
	if d.Created != nil {
		if e := validateCreated(d); e != nil {
			return e
		}
	}
	return nil
}

func containsSubmissionType(types []string, want string) bool {
	for _, t := range types {
		if t == want {
			return true
		}
	}
	return false
}

func validateRubricAttachment(d AssignmentDocument, body string) error {
	b := d.Observed
	if b == nil || b.Rubric != nil || d.Rubric == nil {
		return fmt.Errorf("attach-rubric requires an observed assignment without a rubric and a desired rubric")
	}
	if strings.TrimSpace(d.Rubric.Title) == "" || d.Rubric.ID != 0 || d.Rubric.AssociationID != 0 {
		return fmt.Errorf("attach-rubric requires a titled new rubric without existing rubric/association IDs")
	}
	if d.ModuleID != 0 || d.UpdateOutcome.Status != "" || d.UpdateOutcome.ID != 0 {
		return fmt.Errorf("attach-rubric cannot include module placement or assignment update outcomes")
	}
	dateEqual := func(a, b *string) bool {
		if a == nil || *a == "" {
			return b == nil || *b == ""
		}
		if b == nil || *b == "" {
			return false
		}
		x, e1 := time.Parse(time.RFC3339Nano, *a)
		y, e2 := time.Parse(time.RFC3339Nano, *b)
		return e1 == nil && e2 == nil && x.Truncate(time.Second).Equal(y.Truncate(time.Second))
	}
	typesEqual := len(d.SubmissionTypes) == len(b.SubmissionTypes)
	for i := range d.SubmissionTypes {
		if !typesEqual || d.SubmissionTypes[i] != b.SubmissionTypes[i] {
			typesEqual = false
			break
		}
	}
	if d.Title != b.Title || d.Points.Cmp(b.Points) != 0 || !typesEqual || !dateEqual(d.DueAt, b.DueAt) || !dateEqual(d.UnlockAt, b.UnlockAt) || !dateEqual(d.LockAt, b.LockAt) || d.Published != b.Published || body != b.DescriptionHTML {
		return fmt.Errorf("attach-rubric must preserve assignment title, points, submission types, dates, publication, and exact HTML")
	}
	if d.AssignmentGroupID != nil && (b.AssignmentGroupID == nil || *d.AssignmentGroupID != *b.AssignmentGroupID) {
		return fmt.Errorf("attach-rubric must preserve assignment_group_id; use operation: update to change the assignment group")
	}
	if d.Created == nil {
		return nil
	}
	c := d.Created
	if c.AssignmentID != d.AssignmentID || c.RubricID < 0 || c.RubricAssociationID < 0 {
		return fmt.Errorf("attachment resource identities disagree with assignment or are negative")
	}
	if c.StartedAt != "" || c.IntendedHTML != "" || c.ModuleItemID != 0 {
		return fmt.Errorf("attach-rubric cannot carry assignment creation or module metadata")
	}
	for _, o := range []ActionOutcome{c.Assignment, c.Rubric, c.Module, c.Publication} {
		if e := validateOutcome(o); e != nil {
			return e
		}
	}
	if (c.Assignment.Status != "" && c.Assignment.Status != "preserved") || (c.Publication.Status != "" && c.Publication.Status != "preserved") || (c.Module.Status != "" && c.Module.Status != "not-requested") {
		return fmt.Errorf("attachment outcomes must preserve assignment/publication and omit module writes")
	}
	if c.Assignment.ID != 0 && c.Assignment.ID != d.AssignmentID {
		return fmt.Errorf("attachment assignment outcome id disagrees with assignment_id")
	}
	if c.Rubric.ID > 0 && c.Rubric.ID != c.RubricID {
		return fmt.Errorf("attachment rubric outcome id disagrees with rubric_id")
	}
	if (c.Rubric.Status == "succeeded" || c.Rubric.Status == "preserved" || c.Complete) && (c.RubricID <= 0 || c.RubricAssociationID <= 0) {
		return fmt.Errorf("confirmed rubric attachment requires rubric and association ids")
	}
	if c.Complete && c.Rubric.Status != "succeeded" && c.Rubric.Status != "preserved" {
		return fmt.Errorf("complete attachment requires a definite rubric outcome")
	}
	if c.Rubric.Status == "not-requested" {
		return fmt.Errorf("attach-rubric cannot mark its rubric not-requested")
	}
	return nil
}

func validateCreated(d AssignmentDocument) error {
	c := d.Created
	for _, id := range []int64{c.AssignmentID, c.RubricID, c.RubricAssociationID, c.ModuleItemID} {
		if id < 0 {
			return fmt.Errorf("created resource id cannot be negative")
		}
	}
	for _, o := range []ActionOutcome{c.Assignment, c.Rubric, c.Module, c.Publication} {
		if e := validateOutcome(o); e != nil {
			return e
		}
	}
	if c.StartedAt != "" {
		if _, e := time.Parse(time.RFC3339Nano, c.StartedAt); e != nil {
			return fmt.Errorf("create_started_at must be RFC3339")
		}
	}
	for _, item := range []struct {
		label   string
		outcome ActionOutcome
		id      int64
	}{{"assignment", c.Assignment, c.AssignmentID}, {"rubric", c.Rubric, c.RubricID}, {"module", c.Module, c.ModuleItemID}} {
		if item.outcome.ID > 0 && item.id != item.outcome.ID {
			return fmt.Errorf("created %s outcome id disagrees with resource id", item.label)
		}
		if item.outcome.Status == "succeeded" && item.id <= 0 {
			return fmt.Errorf("succeeded %s requires resource id", item.label)
		}
	}
	if (c.RubricID > 0 || c.RubricAssociationID > 0 || c.ModuleItemID > 0) && c.AssignmentID <= 0 {
		return fmt.Errorf("created rubric/module requires assignment id")
	}
	if c.Complete {
		if c.AssignmentID <= 0 || c.Assignment.Status != "succeeded" {
			return fmt.Errorf("complete create requires succeeded assignment with id")
		}
		// A rubric or module placement the teacher already settled in Canvas is
		// recorded as preserved and needs no ids of ours.
		if d.Rubric != nil && c.Rubric.Status != "preserved" && (c.RubricID <= 0 || c.RubricAssociationID <= 0 || c.Rubric.Status != "succeeded") {
			return fmt.Errorf("complete create requires succeeded requested rubric and association ids")
		}
		if d.ModuleID > 0 && c.Module.Status != "preserved" && (c.ModuleItemID <= 0 || c.Module.Status != "succeeded") {
			return fmt.Errorf("complete create requires succeeded requested module with id")
		}
		if c.Publication.Status != "succeeded" && c.Publication.Status != "preserved" {
			return fmt.Errorf("complete create requires definite publication outcome")
		}
		for _, row := range c.Overrides {
			if row.Outcome.Status != "succeeded" && row.Outcome.Status != "preserved" {
				return fmt.Errorf("complete create requires definite override outcomes")
			}
		}
		if d.OnlyVisibleToOverrides != nil && c.Visibility.Status != "succeeded" && c.Visibility.Status != "preserved" {
			return fmt.Errorf("complete create requires definite override visibility outcome")
		}
	}
	return nil
}
func (d *AssignmentDocument) ValidateBody(body string) error { return ValidateAssignment(*d, body) }
func (d *PacketDocument) Validate() error {
	if d.Schema != 1 || d.Kind != "packet" {
		return fmt.Errorf("expected schema: 1 and kind: packet")
	}
	if !validNickname(d.Course) {
		return fmt.Errorf("safe course nickname required")
	}
	refs := map[string]bool{}
	for _, ref := range d.Students {
		if strings.TrimSpace(ref) == "" || refs[ref] {
			return fmt.Errorf("packet student refs must be nonempty and unique")
		}
		refs[ref] = true
	}
	return ValidateSnapshot(d.Assignment)
}
func (d *StudentWorkDocument) Validate() error {
	if d.Schema != 1 || d.Kind != "student-work" || d.StudentRef == "" || d.AssignmentID <= 0 {
		return fmt.Errorf("invalid student-work identity/schema")
	}
	if e := validateAttempt(d.Current.Ref); e != nil {
		return e
	}
	attempts := map[int]bool{d.Current.Ref.Attempt: true}
	for _, a := range d.Prior {
		if e := validateAttempt(a.Ref); e != nil {
			return e
		}
		if attempts[a.Ref.Attempt] || a.Ref.Attempt >= d.Current.Ref.Attempt {
			return fmt.Errorf("duplicate/nonpreceding prior attempt")
		}
		attempts[a.Ref.Attempt] = true
	}
	if !d.EnteredGradeKnown && d.EnteredGrade != nil {
		return fmt.Errorf("unknown entered grade must not carry a derived entered_grade")
	}
	return nil
}
