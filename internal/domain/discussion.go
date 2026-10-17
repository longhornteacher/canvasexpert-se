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
