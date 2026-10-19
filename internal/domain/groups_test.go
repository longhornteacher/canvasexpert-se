package domain

import (
	"gopkg.in/yaml.v3"
	"testing"
)

func TestGroupSetValidationKeepsAbsentAndExplicitClearDistinct(t *testing.T) {
	var absent, clear GroupSetDocument
	if err := yaml.Unmarshal([]byte("schema: 1\nkind: group-set\noperation: update\ncourse: TEST\ninstance: https://canvas.example\ncourse_id: 1\ngroup_category_id: 2\nname: Clubs\nobserved: {instance: https://canvas.example, course_id: 1, group_category_id: 2, name: Clubs, self_signup: enabled, auto_leader: first}\n"), &absent); err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal([]byte("schema: 1\nkind: group-set\noperation: update\ncourse: TEST\ninstance: https://canvas.example\ncourse_id: 1\ngroup_category_id: 2\nname: Clubs\nself_signup: disabled\nauto_leader: none\nobserved: {instance: https://canvas.example, course_id: 1, group_category_id: 2, name: Clubs, self_signup: enabled, auto_leader: first}\n"), &clear); err != nil {
		t.Fatal(err)
	}
	if absent.SelfSignup.Set || absent.AutoLeader.Set {
		t.Fatal("omitted settings became requested changes")
	}
	if err := absent.Validate(); err != nil {
		t.Fatal(err)
	}
	if !clear.SelfSignup.Set || clear.SelfSignup.Value != "disabled" || !clear.AutoLeader.Set || clear.AutoLeader.Value != "none" {
		t.Fatal("explicit clear was lost")
	}
	if err := clear.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestGroupSetRejectsDuplicateKeysAndLimitWithoutSignup(t *testing.T) {
	d := GroupSetDocument{Schema: 1, Kind: "group-set", Operation: "create", Course: "TEST", Name: "Clubs", GroupLimit: OptionalInt{Set: true, Value: 4}, Groups: []GroupSetRow{{Key: "a", Name: "A"}, {Key: "a", Name: "B"}}}
	if err := d.Validate(); err == nil {
		t.Fatal("duplicate local key accepted")
	}
	d.Groups = d.Groups[:1]
	if err := d.Validate(); err == nil {
		t.Fatal("limit without sign-up accepted")
	}
}

func TestSharedGroupLateOnlyNeedsNoPointsOrRubric(t *testing.T) {
	individual := false
	baseline := ObservedTiming{StatusKnown: true, LateKnown: true, SecondsLateKnown: true}
	doc := GroupResultsDocument{
		Schema: 1, Kind: "group-results", Packet: "courses/TEST/pull.md",
		Assignment: AssignmentSnapshot{Instance: "https://canvas.example", CourseID: 1, AssignmentID: 2, SubmissionTypes: []string{"online_upload"}, GradingType: "points", GroupCategoryID: 4, GradeGroupStudentsIndividually: &individual},
		Groups: []GroupResult{{GroupID: 7, MemberRefs: []string{"a", "b"}, Operation: "late-only", LateOnly: &GroupLateOnly{
			LateTiming:      LateTiming{Status: "none", DueAt: "2026-10-08T12:00:00Z", Basis: "submitted before the due time"},
			ObservedMembers: map[string]ObservedTiming{"a": baseline, "b": baseline},
		}}},
	}
	if _, err := ValidateGroupResults(&doc); err != nil {
		t.Fatalf("group timing-only action incorrectly required points or a rubric: %v", err)
	}
}
