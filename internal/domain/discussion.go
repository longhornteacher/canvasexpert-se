package domain

import (
	"fmt"
	"strings"
)

// DiscussionDocument authors an ungraded course, distributed group-set, or
// direct group discussion topic. Graded discussion support is deliberately a
// later slice; Assignment is decoded only so 05a can reject it explicitly.
type DiscussionDocument struct {
	Schema             int                 `yaml:"schema"`
	Kind               string              `yaml:"kind"`
	Operation          string              `yaml:"operation"`
	Course             string              `yaml:"course"`
	Instance           string              `yaml:"instance,omitempty"`
	CourseID           int64               `yaml:"course_id,omitempty"`
	TopicID            int64               `yaml:"topic_id,omitempty"`
	GroupCategoryID    int64               `yaml:"group_category_id,omitempty"`
	GroupID            int64               `yaml:"group_id,omitempty"`
	Title              string              `yaml:"title"`
	BodyFormat         string              `yaml:"body_format"`
	DiscussionType     string              `yaml:"discussion_type,omitempty"`
	RequireInitialPost *bool               `yaml:"require_initial_post,omitempty"`
	Published          *bool               `yaml:"published,omitempty"`
	DelayedPostAt      *string             `yaml:"delayed_post_at,omitempty"`
	LockAt             *string             `yaml:"lock_at,omitempty"`
	Pinned             *bool               `yaml:"pinned,omitempty"`
	IsAnnouncement     *bool               `yaml:"is_announcement,omitempty"`
	AnnouncementIntent string              `yaml:"announcement_intent,omitempty"`
	Assignment         *map[string]any     `yaml:"assignment,omitempty"`
	Observed           *DiscussionSnapshot `yaml:"observed,omitempty"`
	Created            *DiscussionCreated  `yaml:"created,omitempty"`
	UpdateOutcome      ActionOutcome       `yaml:"update_outcome,omitempty"`
}

type DiscussionSnapshot struct {
	Instance           string  `yaml:"instance"`
	CourseID           int64   `yaml:"course_id"`
	TopicID            int64   `yaml:"topic_id"`
	URL                string  `yaml:"url,omitempty"`
	GroupCategoryID    int64   `yaml:"group_category_id,omitempty"`
	GroupID            int64   `yaml:"group_id,omitempty"`
	Title              string  `yaml:"title"`
	BodyHTML           string  `yaml:"body_html"`
	DiscussionType     string  `yaml:"discussion_type,omitempty"`
	RequireInitialPost bool    `yaml:"require_initial_post,omitempty"`
	Published          bool    `yaml:"published"`
	DelayedPostAt      *string `yaml:"delayed_post_at,omitempty"`
	LockAt             *string `yaml:"lock_at,omitempty"`
	Pinned             bool    `yaml:"pinned,omitempty"`
	IsAnnouncement     bool    `yaml:"is_announcement,omitempty"`
}

type DiscussionCreated struct {
	Intent             *DiscussionIntent `yaml:"intent,omitempty"`
	StartedAt          string            `yaml:"started_at,omitempty"`
	TopicID            int64             `yaml:"topic_id,omitempty"`
	GroupCategoryID    int64             `yaml:"group_category_id,omitempty"`
	GroupID            int64             `yaml:"group_id,omitempty"`
	URL                string            `yaml:"url,omitempty"`
	IntendedTitle      string            `yaml:"intended_title"`
	IntendedHTML       string            `yaml:"intended_html"`
	SavedHTML          string            `yaml:"saved_html,omitempty"`
	GroupTopicChildren []DiscussionChild `yaml:"group_topic_children,omitempty"`
	Topic              ActionOutcome     `yaml:"topic,omitempty"`
	PublishedRequested bool              `yaml:"published_requested,omitempty"`
	Publication        ActionOutcome     `yaml:"publication,omitempty"`
	Complete           bool              `yaml:"complete,omitempty"`
}

type DiscussionIntent struct {
	DiscussionType     string  `yaml:"discussion_type,omitempty"`
	RequireInitialPost *bool   `yaml:"require_initial_post,omitempty"`
	Published          bool    `yaml:"published"`
	DelayedPostAt      *string `yaml:"delayed_post_at,omitempty"`
	LockAt             *string `yaml:"lock_at,omitempty"`
	Pinned             *bool   `yaml:"pinned,omitempty"`
	IsAnnouncement     bool    `yaml:"is_announcement"`
	AnnouncementIntent string  `yaml:"announcement_intent,omitempty"`
}

type DiscussionChild struct {
	GroupID int64 `yaml:"group_id"`
	TopicID int64 `yaml:"topic_id"`
}

// DiscussionPacket is an as-of observation of Canvas conversation content.
// It intentionally contains only stable student refs and aliases, never Canvas
// user records or names.
type DiscussionPacket struct {
	Schema        int                `yaml:"schema" json:"schema"`
	Kind          string             `yaml:"kind" json:"kind"`
	Course        string             `yaml:"course" json:"course"`
	Instance      string             `yaml:"instance" json:"instance"`
	CourseID      int64              `yaml:"course_id" json:"course_id"`
	ParentTopicID int64              `yaml:"parent_topic_id" json:"parent_topic_id"`
	AssignmentID  int64              `yaml:"assignment_id,omitempty" json:"assignment_id,omitempty"`
	PulledAt      string             `yaml:"pulled_at" json:"pulled_at"`
	Status        string             `yaml:"status" json:"status"`
	TeacherNotes  []string           `yaml:"teacher_notes,omitempty" json:"teacher_notes,omitempty"`
	Sources       []DiscussionSource `yaml:"sources" json:"sources"`
	Entries       []DiscussionEntry  `yaml:"entries" json:"entries"`
}

type DiscussionSource struct {
	ContextType string `yaml:"context_type" json:"context_type"`
	ContextID   int64  `yaml:"context_id" json:"context_id"`
	TopicID     int64  `yaml:"topic_id" json:"topic_id"`
	GroupID     int64  `yaml:"group_id,omitempty" json:"group_id,omitempty"`
	Method      string `yaml:"method,omitempty" json:"method,omitempty"`
	Status      string `yaml:"status" json:"status"`
	Reason      string `yaml:"reason,omitempty" json:"reason,omitempty"`
	EntryCount  int    `yaml:"entry_count" json:"entry_count"`
}

type DiscussionEntry struct {
	EntryID       int64        `yaml:"entry_id" json:"entry_id"`
	ContextType   string       `yaml:"context_type" json:"context_type"`
	ContextID     int64        `yaml:"context_id" json:"context_id"`
	TopicID       int64        `yaml:"topic_id" json:"topic_id"`
	ParentEntryID int64        `yaml:"parent_entry_id,omitempty" json:"parent_entry_id,omitempty"`
	AuthorKind    string       `yaml:"author_kind" json:"author_kind"`
	StudentRef    string       `yaml:"student_ref,omitempty" json:"student_ref,omitempty"`
	Alias         string       `yaml:"alias,omitempty" json:"alias,omitempty"`
	CreatedAt     string       `yaml:"created_at,omitempty" json:"created_at,omitempty"`
	UpdatedAt     string       `yaml:"updated_at,omitempty" json:"updated_at,omitempty"`
	HTML          string       `yaml:"html,omitempty" json:"html,omitempty"`
	ReadableText  string       `yaml:"readable_text,omitempty" json:"readable_text,omitempty"`
	Deleted       bool         `yaml:"deleted,omitempty" json:"deleted,omitempty"`
	Attachments   []Attachment `yaml:"attachments,omitempty" json:"attachments,omitempty"`
}

func (p *DiscussionPacket) Validate() error {
	if p.Schema != 1 || p.Kind != "discussion-packet" || !validNickname(p.Course) || !validInstance(p.Instance) || p.CourseID <= 0 || p.ParentTopicID <= 0 {
		return fmt.Errorf("discussion packet requires schema 1, configured course, Canvas identity and parent topic")
	}
	if p.Status != "complete" && p.Status != "partial" {
		return fmt.Errorf("discussion packet status must be complete or partial")
	}
	sources := map[string]bool{}
	for _, source := range p.Sources {
		if (source.ContextType != "Course" && source.ContextType != "Group") || source.ContextID <= 0 || source.TopicID <= 0 || (source.Status != "complete" && source.Status != "partial") {
			return fmt.Errorf("discussion packet has an invalid source scope or coverage status")
		}
		if source.Method != "" && source.Method != "threaded_view" && source.Method != "paginated_entries" {
			return fmt.Errorf("discussion packet source method is invalid")
		}
		if source.ContextType == "Course" && source.ContextID != p.CourseID {
			return fmt.Errorf("discussion packet course source differs from its course identity")
		}
		if source.ContextType == "Group" && source.GroupID != 0 && source.GroupID != source.ContextID {
			return fmt.Errorf("discussion packet group source identity disagrees")
		}
		if source.Status == "partial" && p.Status == "complete" {
			return fmt.Errorf("complete discussion packet cannot contain a partial source")
		}
		sources[fmt.Sprintf("%s:%d:%d", source.ContextType, source.ContextID, source.TopicID)] = true
	}
	for _, entry := range p.Entries {
		key := fmt.Sprintf("%s:%d:%d", entry.ContextType, entry.ContextID, entry.TopicID)
		if entry.EntryID <= 0 || !sources[key] || (entry.ContextType != "Course" && entry.ContextType != "Group") || entry.ParentEntryID < 0 {
			return fmt.Errorf("discussion packet has an entry outside its recorded source identity")
		}
		if entry.AuthorKind != "student" && entry.AuthorKind != "instructor" && entry.AuthorKind != "unknown" {
			return fmt.Errorf("discussion packet author kind is invalid")
		}
		if entry.AuthorKind == "student" && entry.StudentRef == "" {
			return fmt.Errorf("mapped student entry requires a stable student_ref")
		}
		if entry.Deleted && (entry.HTML != "" || entry.ReadableText != "") {
			return fmt.Errorf("deleted discussion entries must not reconstruct erased text")
		}
	}
	return nil
}

type DiscussionReplyDocument struct {
	Schema        int           `yaml:"schema"`
	Kind          string        `yaml:"kind"`
	Operation     string        `yaml:"operation"`
	Course        string        `yaml:"course"`
	Instance      string        `yaml:"instance,omitempty"`
	CourseID      int64         `yaml:"course_id,omitempty"`
	ContextType   string        `yaml:"context_type"`
	ContextID     int64         `yaml:"context_id"`
	GroupID       int64         `yaml:"group_id,omitempty"`
	TopicID       int64         `yaml:"topic_id"`
	ParentEntryID int64         `yaml:"parent_entry_id,omitempty"`
	BodyFormat    string        `yaml:"body_format"`
	TeacherNote   string        `yaml:"teacher_note,omitempty"`
	Outcome       ActionOutcome `yaml:"outcome,omitempty"`
}

func (d *DiscussionReplyDocument) ValidateBody(body string) error {
	if d.Schema != 1 || d.Kind != "discussion-reply" || d.Operation != "create" {
		return fmt.Errorf("expected schema: 1, kind: discussion-reply and operation: create")
	}
	if !validNickname(d.Course) || d.TopicID <= 0 || d.ContextID <= 0 || (d.ContextType != "Course" && d.ContextType != "Group") {
		return fmt.Errorf("configured course, exact topic and course/group context identity required")
	}
	if (d.ContextType == "Group" && d.GroupID != d.ContextID) || (d.ContextType == "Course" && d.GroupID != 0) {
		return fmt.Errorf("reply context and group identity disagree")
	}
	if d.Instance != "" && !validInstance(d.Instance) {
		return fmt.Errorf("instance must be an HTTPS Canvas origin")
	}
	if d.CourseID < 0 || d.ParentEntryID < 0 || d.GroupID < 0 {
		return fmt.Errorf("invalid discussion reply identity")
	}
	if d.BodyFormat != "markdown" && d.BodyFormat != "html" {
		return fmt.Errorf("body_format is required: markdown or html")
	}
	if strings.TrimSpace(body) == "" {
		return fmt.Errorf("teacher reply body cannot be empty")
	}
	return validateOutcome(d.Outcome)
}

func (d *DiscussionDocument) ValidateBody(body string) error {
	if d.Schema != 1 || d.Kind != "discussion" || (d.Operation != "create" && d.Operation != "update") {
		return fmt.Errorf("expected schema: 1, kind: discussion and operation: create|update")
	}
	if !validNickname(d.Course) || strings.TrimSpace(d.Title) == "" {
		return fmt.Errorf("safe course nickname and discussion title required")
	}
	if d.BodyFormat != "markdown" && d.BodyFormat != "html" {
		return fmt.Errorf("body_format is required: markdown or html")
	}
	if d.Operation == "create" && strings.TrimSpace(body) == "" {
		return fmt.Errorf("discussion body cannot be empty")
	}
	if d.Instance != "" && !validInstance(d.Instance) {
		return fmt.Errorf("instance must be an HTTPS Canvas origin")
	}
	if d.CourseID < 0 || d.TopicID < 0 || d.GroupCategoryID < 0 || d.GroupID < 0 {
		return fmt.Errorf("invalid discussion/course/group identity")
	}
	if d.GroupCategoryID > 0 && d.GroupID > 0 {
		return fmt.Errorf("choose exactly one discussion scope: course, group_category_id, or group_id")
	}
	if d.DiscussionType != "" && d.DiscussionType != "threaded" && d.DiscussionType != "side_comment" && d.DiscussionType != "not_threaded" {
		return fmt.Errorf("discussion_type must be threaded, side_comment, or not_threaded")
	}
	if d.Assignment != nil {
		return fmt.Errorf("linked graded discussions are not yet supported in slice 05a; remove assignment and use an ungraded discussion")
	}
	announcement := d.IsAnnouncement != nil && *d.IsAnnouncement
	if d.Operation == "update" && d.Observed != nil && d.Observed.IsAnnouncement {
		announcement = true
	}
	if announcement && strings.TrimSpace(d.AnnouncementIntent) == "" {
		return fmt.Errorf("announcements require explicit announcement_intent describing the requested content and audience")
	}
	if announcement && d.Assignment != nil {
		return fmt.Errorf("announcements cannot carry an assignment")
	}
	if d.Operation == "create" {
		if d.Observed != nil || d.UpdateOutcome.Status != "" || d.TopicID != 0 {
			return fmt.Errorf("create cannot carry an update baseline or topic_id; created.topic_id records a resumed create")
		}
	} else {
		b := d.Observed
		if b == nil || !validInstance(b.Instance) || b.CourseID <= 0 || b.TopicID <= 0 || strings.TrimSpace(b.Title) == "" {
			return fmt.Errorf("discussion update requires a pulled live baseline")
		}
		if d.Instance != b.Instance || d.CourseID != b.CourseID || d.GroupID != b.GroupID || d.GroupCategoryID != b.GroupCategoryID || d.TopicID != b.TopicID {
			return fmt.Errorf("discussion update identity differs from baseline")
		}
		if d.Created != nil {
			return fmt.Errorf("discussion update cannot carry create state")
		}
		if d.UpdateOutcome.ID > 0 && d.UpdateOutcome.ID != d.TopicID {
			return fmt.Errorf("discussion update outcome identity disagrees")
		}
	}
	if err := validateOutcome(d.UpdateOutcome); err != nil {
		return err
	}
	if c := d.Created; c != nil {
		if c.Intent == nil {
			return fmt.Errorf("saved discussion create lacks its original settings intent; resolve the exact Canvas topic before proceeding")
		}
		if c.TopicID < 0 || c.GroupID != d.GroupID || c.GroupCategoryID != d.GroupCategoryID {
			return fmt.Errorf("created discussion scope identity disagrees")
		}
		for _, outcome := range []ActionOutcome{c.Topic, c.Publication} {
			if err := validateOutcome(outcome); err != nil {
				return err
			}
		}
		if c.Topic.ID > 0 && c.Topic.ID != c.TopicID {
			return fmt.Errorf("created discussion outcome identity disagrees")
		}
		if c.Topic.Status == "succeeded" && c.TopicID <= 0 {
			return fmt.Errorf("succeeded discussion creation requires exact topic_id")
		}
		if c.Complete && (c.TopicID <= 0 || !discussionDefinite(c.Topic) || !discussionDefinite(c.Publication)) {
			return fmt.Errorf("complete discussion requires successful topic creation")
		}
		if c.PublishedRequested && c.Publication.Status == "not-requested" {
			return fmt.Errorf("requested publication cannot be marked not requested")
		}
		if c.TopicID > 0 && (d.Instance == "" || d.CourseID <= 0) {
			return fmt.Errorf("known topic requires persisted origin/course")
		}
		if c.IntendedHTML == "" || c.IntendedTitle == "" {
			return fmt.Errorf("saved discussion create requires its original intended title and body")
		}
	}
	return nil
}

func discussionDefinite(o ActionOutcome) bool {
	return o.Status == "succeeded" || o.Status == "preserved" || o.Status == "not-requested"
}
