package domain

import (
	"fmt"
	"strings"
	"time"
)

type GroupPacketDocument struct {
	Schema         int                   `yaml:"schema"`
	Kind           string                `yaml:"kind"`
	Course         string                `yaml:"course"`
	PulledAt       string                `yaml:"pulled_at"`
	Assignment     AssignmentSnapshot    `yaml:"assignment"`
	DirectionsHTML string                `yaml:"directions_html,omitempty"`
	Groups         []GroupPacketEntry    `yaml:"groups,omitempty"`
	Unassigned     []GroupUnassignedWork `yaml:"unassigned,omitempty"`
}

type GroupPacketEntry struct {
	GroupID        int64    `yaml:"group_id"`
	Name           string   `yaml:"name"`
	Path           string   `yaml:"path"`
	EvidenceStatus string   `yaml:"evidence_status"` // complete, partial, held
	Reasons        []string `yaml:"reasons,omitempty"`
}

type GroupUnassignedWork struct {
	StudentRef          string                   `yaml:"student_ref"`
	Alias               string                   `yaml:"alias"`
	GroupIdentityStatus string                   `yaml:"group_identity_status"` // currently-unassigned or unknown
	State               string                   `yaml:"state"`
	SubmissionID        int64                    `yaml:"submission_id,omitempty"`
	Attempt             int                      `yaml:"attempt,omitempty"`
	Missing             bool                     `yaml:"missing"`
	Excused             bool                     `yaml:"excused"`
	Late                bool                     `yaml:"late"`
	AssignmentVisible   *bool                    `yaml:"assignment_visible,omitempty"`
	EnteredGradeKnown   bool                     `yaml:"entered_grade_known"`
	EnteredGrade        *Decimal                 `yaml:"entered_grade,omitempty"`
	ObservedScore       *Decimal                 `yaml:"observed_score,omitempty"`
	BodyHTML            string                   `yaml:"body_html,omitempty"`
	BodyHTMLPath        string                   `yaml:"body_html_path,omitempty"`
	ReadableText        string                   `yaml:"readable_text,omitempty"`
	Attachments         []Attachment             `yaml:"attachments,omitempty"`
	Comments            []ExistingComment        `yaml:"comments,omitempty"`
	Prior               []GroupUnassignedAttempt `yaml:"prior,omitempty"`
}

type GroupUnassignedAttempt struct {
	SubmissionID      int64        `yaml:"submission_id,omitempty"`
	SubmissionGroupID int64        `yaml:"submission_group_id,omitempty"`
	Attempt           AttemptRef   `yaml:"attempt"`
	BodyHTML          string       `yaml:"body_html,omitempty"`
	BodyHTMLPath      string       `yaml:"body_html_path,omitempty"`
	ReadableText      string       `yaml:"readable_text,omitempty"`
	Attachments       []Attachment `yaml:"attachments,omitempty"`
	EnteredGrade      *Decimal     `yaml:"entered_grade,omitempty"`
	ObservedScore     *Decimal     `yaml:"observed_score,omitempty"`
}

type GroupWorkDocument struct {
	Schema                      int                   `yaml:"schema"`
	Kind                        string                `yaml:"kind"`
	Instance                    string                `yaml:"instance"`
	Course                      string                `yaml:"course"`
	CourseID                    int64                 `yaml:"course_id"`
	AssignmentID                int64                 `yaml:"assignment_id"`
	GroupCategoryID             int64                 `yaml:"group_category_id"`
	GroupID                     int64                 `yaml:"group_id"`
	GroupName                   string                `yaml:"group_name"`
	Assignment                  AssignmentSnapshot    `yaml:"assignment"`
	PulledAt                    string                `yaml:"pulled_at"`
	RosterComplete              bool                  `yaml:"roster_complete"`
	Membership                  []GroupWorkMembership `yaml:"membership"`
	PropagationAudience         []string              `yaml:"propagation_audience,omitempty"`
	PropagationAudienceComplete bool                  `yaml:"propagation_audience_complete"`
	SharedAssessmentStatus      string                `yaml:"shared_assessment_status"` // ready, held
	SharedAssessmentReasons     []string              `yaml:"shared_assessment_reasons,omitempty"`
	Members                     []GroupWorkMember     `yaml:"members,omitempty"`
	Artifacts                   []GroupArtifact       `yaml:"artifacts,omitempty"`
}

type GroupWorkMembership struct {
	StudentRef string `yaml:"student_ref"`
	Alias      string `yaml:"alias"`
	State      string `yaml:"state"`
}

type GroupWorkMember struct {
	StudentRef          string             `yaml:"student_ref"`
	Alias               string             `yaml:"alias"`
	ExtraTime           *bool              `yaml:"extra_time,omitempty"`   // private saved fact; absent in older packets
	WorkPurpose         string             `yaml:"work_purpose,omitempty"` // score or score-and-late; late-only appears only in older packets
	CurrentMember       bool               `yaml:"current_member"`
	MembershipState     string             `yaml:"membership_state,omitempty"`
	SubmissionID        int64              `yaml:"submission_id"`
	SubmissionGroupID   int64              `yaml:"submission_group_id,omitempty"`
	Attempt             int                `yaml:"attempt"`
	SubmittedAt         string             `yaml:"submitted_at,omitempty"`
	CurrentArtifactKey  string             `yaml:"current_artifact_key,omitempty"`
	Prior               []GroupWorkAttempt `yaml:"prior,omitempty"`
	WorkflowState       string             `yaml:"workflow_state,omitempty"`
	Missing             bool               `yaml:"missing"`
	Excused             bool               `yaml:"excused"`
	Late                bool               `yaml:"late"`
	ObservedTiming      *ObservedTiming    `yaml:"observed_timing,omitempty"`
	AssignmentVisible   *bool              `yaml:"assignment_visible,omitempty"`
	GradeMatchesCurrent *bool              `yaml:"grade_matches_current_submission,omitempty"`
	EnteredGradeKnown   bool               `yaml:"entered_grade_known"`
	EnteredGrade        *Decimal           `yaml:"entered_grade,omitempty"`
	ObservedScore       *Decimal           `yaml:"observed_score,omitempty"`
	PointsDeducted      *Decimal           `yaml:"points_deducted,omitempty"`
	EffectiveDueAt      string             `yaml:"effective_due_at,omitempty"`
	RubricAssessment    []CriterionScore   `yaml:"rubric_assessment,omitempty"`
	Comments            []ExistingComment  `yaml:"comments,omitempty"`
}

type GroupWorkAttempt struct {
	SubmissionID      int64      `yaml:"submission_id,omitempty"`
	SubmissionGroupID int64      `yaml:"submission_group_id,omitempty"`
	Attempt           AttemptRef `yaml:"attempt"`
	ArtifactKey       string     `yaml:"artifact_key,omitempty"`
	EnteredGrade      *Decimal   `yaml:"entered_grade,omitempty"`
	ObservedScore     *Decimal   `yaml:"observed_score,omitempty"`
	Late              bool       `yaml:"late"`
}

type GroupArtifact struct {
	Key            string                `yaml:"key"`
	Fingerprint    string                `yaml:"fingerprint"`
	SubmissionType string                `yaml:"submission_type,omitempty"`
	URL            string                `yaml:"url,omitempty"`
	Note           string                `yaml:"note,omitempty"`
	BodyHTML       string                `yaml:"body_html,omitempty"`
	BodyHTMLPath   string                `yaml:"body_html_path,omitempty"`
	ReadableText   string                `yaml:"readable_text,omitempty"`
	Attachments    []Attachment          `yaml:"attachments,omitempty"`
	Sources        []GroupArtifactSource `yaml:"sources"`
}

type GroupArtifactSource struct {
	StudentRef        string     `yaml:"student_ref"`
	SubmissionID      int64      `yaml:"submission_id"`
	SubmissionGroupID int64      `yaml:"submission_group_id,omitempty"`
	Attempt           AttemptRef `yaml:"attempt"`
}

func (d GroupPacketDocument) Validate() error {
	if d.Schema != 1 || d.Kind != "group-packet" || !validNickname(d.Course) {
		return fmt.Errorf("invalid group-packet schema/kind/course")
	}
	if _, err := time.Parse(time.RFC3339Nano, d.PulledAt); err != nil {
		return fmt.Errorf("group-packet pulled_at must be RFC3339")
	}
	if err := ValidateSnapshot(d.Assignment); err != nil {
		return err
	}
	if d.Assignment.GroupCategoryID <= 0 || d.Assignment.CourseID <= 0 {
		return fmt.Errorf("group-packet requires group assignment identity")
	}
	seen := map[int64]bool{}
	for i, g := range d.Groups {
		if g.GroupID <= 0 || seen[g.GroupID] || strings.TrimSpace(g.Name) == "" || !strings.HasPrefix(g.Path, "courses/") {
			return fmt.Errorf("groups[%d] requires a unique exact group, label, and course path", i)
		}
		seen[g.GroupID] = true
		if g.EvidenceStatus != "complete" && g.EvidenceStatus != "partial" && g.EvidenceStatus != "held" {
			return fmt.Errorf("groups[%d] has invalid evidence_status", i)
		}
	}
	for i, u := range d.Unassigned {
		if u.GroupIdentityStatus != "currently-unassigned" && u.GroupIdentityStatus != "unknown" {
			return fmt.Errorf("unassigned[%d] requires group_identity_status", i)
		}
		if u.GroupIdentityStatus == "currently-unassigned" && u.StudentRef == "" {
			return fmt.Errorf("unassigned[%d] requires student_ref when currently unassigned", i)
		}
	}
	return nil
}

func (d GroupWorkDocument) Validate() error {
	if d.Schema != 1 || d.Kind != "group-work" || !validNickname(d.Course) || !validInstance(d.Instance) || d.CourseID <= 0 || d.AssignmentID <= 0 || d.GroupCategoryID <= 0 || d.GroupID <= 0 || strings.TrimSpace(d.GroupName) == "" || d.Assignment.Instance != d.Instance || d.Assignment.GroupCategoryID != d.GroupCategoryID || d.Assignment.AssignmentID != d.AssignmentID || d.Assignment.CourseID != d.CourseID {
		return fmt.Errorf("invalid group-work identity/schema")
	}
	if err := ValidateSnapshot(d.Assignment); err != nil {
		return err
	}
	if _, err := time.Parse(time.RFC3339Nano, d.PulledAt); err != nil {
		return fmt.Errorf("group-work pulled_at must be RFC3339")
	}
	if d.SharedAssessmentStatus != "ready" && d.SharedAssessmentStatus != "held" && d.SharedAssessmentStatus != "not-applicable" {
		return fmt.Errorf("invalid shared_assessment_status")
	}
	refs := map[string]bool{}
	for _, m := range d.Membership {
		if m.StudentRef == "" || refs[m.StudentRef] {
			return fmt.Errorf("group membership refs must be nonempty and unique")
		}
		refs[m.StudentRef] = true
	}
	seenAudience := map[string]bool{}
	for _, ref := range d.PropagationAudience {
		if ref == "" || seenAudience[ref] {
			return fmt.Errorf("propagation audience refs must be nonempty and unique")
		}
		seenAudience[ref] = true
	}
	artifacts := map[string]bool{}
	for i, a := range d.Artifacts {
		if a.Key == "" || a.Fingerprint == "" || artifacts[a.Key] || len(a.Sources) == 0 {
			return fmt.Errorf("artifacts[%d] requires unique key, fingerprint, and sources", i)
		}
		artifacts[a.Key] = true
	}
	memberRefs := map[string]bool{}
	for i, m := range d.Members {
		if m.StudentRef == "" || memberRefs[m.StudentRef] || m.SubmissionID < 0 || m.Attempt < 0 || (m.Attempt > 0 && m.SubmissionID <= 0) {
			return fmt.Errorf("members[%d] requires unique member and valid submission/attempt identity", i)
		}
		memberRefs[m.StudentRef] = true
		if m.CurrentArtifactKey != "" && !artifacts[m.CurrentArtifactKey] {
			return fmt.Errorf("members[%d] references unknown current artifact", i)
		}
		for _, p := range m.Prior {
			if p.Attempt.Attempt <= 0 {
				return fmt.Errorf("members[%d] prior attempt requires positive number", i)
			}
			if p.ArtifactKey != "" && !artifacts[p.ArtifactKey] {
				return fmt.Errorf("members[%d] references unknown prior artifact", i)
			}
		}
	}
	return nil
}
