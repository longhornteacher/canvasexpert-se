package workflow

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/render"
	"canvasexpertse/internal/workspace"
)

type NativeDiscussion struct {
	ID                 int64   `json:"id"`
	AssignmentID       int64   `json:"assignment_id"`
	CreatedAt          string  `json:"created_at"`
	Title              string  `json:"title"`
	Message            string  `json:"message"`
	DiscussionType     string  `json:"discussion_type"`
	RequireInitialPost bool    `json:"require_initial_post"`
	Published          bool    `json:"published"`
	DelayedPostAt      *string `json:"delayed_post_at"`
	LockAt             *string `json:"lock_at"`
	Pinned             bool    `json:"pinned"`
	IsAnnouncement     bool    `json:"is_announcement"`
	GroupCategoryID    *int64  `json:"group_category_id"`
	GroupTopicChildren []struct {
		GroupID int64 `json:"group_id"`
		TopicID int64 `json:"id"`
	} `json:"group_topic_children"`
	HTMLURL string `json:"html_url"`
}

type DiscussionReport struct {
	Path, URL, Status, Message string
	TopicID                    int64
}

func discussionBase(courseID, groupID int64) string {
	if groupID > 0 {
		return fmt.Sprintf("/api/v1/groups/%d/discussion_topics", groupID)
	}
	return fmt.Sprintf("/api/v1/courses/%d/discussion_topics", courseID)
}
func discussionPath(courseID, groupID, topicID int64) string {
	return discussionBase(courseID, groupID) + fmt.Sprintf("/%d", topicID)
}

func (s *Service) verifyDiscussionScope(ctx context.Context, nickname string, setID, groupID int64) (domain.Course, error) {
	if setID > 0 && groupID > 0 {
		return domain.Course{}, fmt.Errorf("choose a course, group_category_id, or group_id discussion scope")
	}
	c, err := s.Course(nickname)
	if err != nil {
		return c, err
	}
	if setID > 0 {
		_, err = s.groupCategory(ctx, c.ID, setID)
	}
	if groupID > 0 {
		_, err = s.verifyPageGroup(ctx, nickname, groupID)
	}
	return c, err
}

func (s *Service) PullDiscussion(ctx context.Context, nickname string, topicID, groupID int64) (string, error) {
	if topicID <= 0 || groupID < 0 {
		return "", fmt.Errorf("topic ID must be positive and group ID cannot be negative")
	}
	c, err := s.verifyDiscussionScope(ctx, nickname, 0, groupID)
	if err != nil {
		return "", err
	}
	var topic NativeDiscussion
	err = s.Client.Get(ctx, discussionPath(c.ID, groupID, topicID), &topic)
	if err != nil {
		return "", err
	}
	if topic.ID != topicID {
		return "", fmt.Errorf("Canvas returned a different discussion topic ID")
	}
	setID := int64(0)
	if topic.GroupCategoryID != nil {
		setID = *topic.GroupCategoryID
		if setID > 0 {
			if _, err = s.groupCategory(ctx, c.ID, setID); err != nil {
				return "", err
			}
		}
	}
	folder, err := writePullFolder(s.Root, c.Nickname, topic.ID, topic.Title)
	if err != nil {
		return "", err
	}
	topicPath, err := s.writePulledDiscussion(c, topic, setID, groupID, folder)
	if err != nil {
		return "", err
	}
	packet, err := s.PullDiscussionEvidence(ctx, c.Nickname, topic, c, setID, groupID, folder)
	if err != nil {
		return "", err
	}
	if _, err = writeDiscussionPacket(folder, packet); err != nil {
		return "", err
	}
	return topicPath, nil
}

func (s *Service) writePulledDiscussion(c domain.Course, t NativeDiscussion, setID, groupID int64, folder string) (string, error) {
	published := t.Published
	b := domain.DiscussionSnapshot{Instance: s.Client.Origin(), CourseID: c.ID, TopicID: t.ID, URL: t.HTMLURL, GroupCategoryID: setID, GroupID: groupID, AssignmentID: t.AssignmentID, Title: t.Title, BodyHTML: t.Message, DiscussionType: t.DiscussionType, RequireInitialPost: t.RequireInitialPost, Published: t.Published, DelayedPostAt: t.DelayedPostAt, LockAt: t.LockAt, Pinned: t.Pinned, IsAnnouncement: t.IsAnnouncement}
	d := domain.DiscussionDocument{Schema: 1, Kind: "discussion", Operation: "update", Course: c.Nickname, Instance: b.Instance, CourseID: b.CourseID, TopicID: b.TopicID, GroupCategoryID: setID, GroupID: groupID, Title: t.Title, BodyFormat: "html", DiscussionType: t.DiscussionType, RequireInitialPost: &b.RequireInitialPost, Published: &published, DelayedPostAt: b.DelayedPostAt, LockAt: b.LockAt, Pinned: &b.Pinned, IsAnnouncement: &b.IsAnnouncement, Observed: &b}
	p, err := workspace.ConstrainPath(s.Root, filepath.Join("courses", c.Nickname, "pulled", filepath.Base(folder), "topic.md"))
	if err != nil {
		return "", err
	}
	return p, workspace.WriteDocument(p, &d, t.Message)
}

func (s *Service) PushDiscussion(ctx context.Context, path string) (report DiscussionReport, err error) {
	report.Path = path
	var d domain.DiscussionDocument
	body, err := workspace.ReadDocument(path, &d)
	if err != nil {
		return report, err
	}
	c, err := s.verifyDiscussionScope(ctx, d.Course, d.GroupCategoryID, d.GroupID)
	if err != nil {
		return report, err
	}
	if (d.Instance != "" && d.Instance != s.Client.Origin()) || (d.CourseID != 0 && d.CourseID != c.ID) {
		return report, fmt.Errorf("discussion identity differs from configured Canvas/course")
	}
	if err = d.ValidateBody(body); err != nil {
		return report, err
	}
	if d.Operation == "update" {
		return s.updateDiscussion(ctx, path, c, &d, body, report)
	}
	d.Instance, d.CourseID, d.Course = s.Client.Origin(), c.ID, c.Nickname
	html, err := render.TeachingHTML(body, d.BodyFormat)
	if err != nil {
		return report, err
	}
	if d.Created == nil {
		intent := discussionIntent(d)
		d.Created = &domain.DiscussionCreated{Intent: &intent, GroupID: d.GroupID, GroupCategoryID: d.GroupCategoryID, IntendedHTML: html, IntendedTitle: d.Title, PublishedRequested: discussionBool(d.Published, false)}
	}
	created := d.Created
	if created.Intent == nil || !reflect.DeepEqual(*created.Intent, discussionIntent(d)) || created.IntendedHTML != html || created.IntendedTitle != d.Title || created.GroupID != d.GroupID || created.GroupCategoryID != d.GroupCategoryID || created.PublishedRequested != discussionBool(d.Published, false) {
		return report, fmt.Errorf("incomplete discussion create changed; recover the saved creation before editing it")
	}
	save := func() error { return workspace.WriteDocument(path, &d, body) }
	if created.Complete {
		report.Status = "completed"
		report.TopicID = created.TopicID
		report.URL = created.URL
		report.Message = "already created; pull-discussion before editing"
		return report, nil
	}
	if created.Topic.Status == "held" {
		report.Status = "held"
		report.TopicID = created.TopicID
		report.Message = created.Topic.Message
		return report, nil
	}
	if created.TopicID == 0 && (ambiguous(created.Topic)) {
		found, recoverErr := s.findUncertainDiscussion(ctx, c.ID, d.GroupID, d.GroupCategoryID, created.StartedAt, created.IntendedTitle, created.IntendedHTML, discussionBool(d.IsAnnouncement, false))
		if recoverErr != nil {
			report.Status = "uncertain"
			report.Message = recoverErr.Error()
			return report, nil
		}
		created.TopicID, created.URL, created.SavedHTML, created.AssignmentID = found.ID, found.HTMLURL, found.Message, found.AssignmentID
		created.Topic = domain.ActionOutcome{Status: "succeeded", ID: found.ID, Message: "matched the exact recent topic, including announcement intent, after an unclear response"}
		for _, child := range found.GroupTopicChildren {
			if child.GroupID > 0 && child.TopicID > 0 {
				created.GroupTopicChildren = append(created.GroupTopicChildren, domain.DiscussionChild{GroupID: child.GroupID, TopicID: child.TopicID})
			}
		}
		if err = save(); err != nil {
			return report, fmt.Errorf("recovered discussion identity could not be saved: %w", err)
		}
	}
	if created.TopicID == 0 {
		created.StartedAt = time.Now().UTC().Format(time.RFC3339Nano)
		created.Topic = domain.ActionOutcome{Status: "pending"}
		if err = save(); err != nil {
			return report, err
		}
		fields := map[string]any{"title": d.Title, "message": html, "published": false, "is_announcement": discussionBool(d.IsAnnouncement, false)}
		if d.DiscussionType != "" {
			fields["discussion_type"] = d.DiscussionType
		}
		if d.RequireInitialPost != nil {
			fields["require_initial_post"] = *d.RequireInitialPost
		}
		if d.DelayedPostAt != nil {
			fields["delayed_post_at"] = *d.DelayedPostAt
		}
		if d.LockAt != nil {
			fields["lock_at"] = *d.LockAt
		}
		if d.Pinned != nil {
			fields["pinned"] = *d.Pinned
		}
		if d.GroupCategoryID > 0 {
			fields["group_category_id"] = d.GroupCategoryID
		}
		if a := d.Assignment; a != nil {
			// The group set stays on the topic; the linked assignment is individual
			// gradebook work only.
			linked := map[string]any{"points_possible": a.Points, "grading_type": "points", "submission_types": []string{"discussion_topic"}}
			if a.DueAt != nil {
				linked["due_at"] = *a.DueAt
			}
			if a.UnlockAt != nil {
				linked["unlock_at"] = *a.UnlockAt
			}
			if a.LockAt != nil {
				linked["lock_at"] = *a.LockAt
			}
			fields["assignment"] = linked
		}
		var response NativeDiscussion
		err = s.Client.WriteAccepted(ctx, "POST", discussionBase(c.ID, d.GroupID), fields, &response)
		if response.ID > 0 {
			created.TopicID = response.ID
			created.URL = response.HTMLURL
			created.SavedHTML = response.Message
			created.AssignmentID = response.AssignmentID
			for _, child := range response.GroupTopicChildren {
				if child.GroupID > 0 && child.TopicID > 0 {
					created.GroupTopicChildren = append(created.GroupTopicChildren, domain.DiscussionChild{GroupID: child.GroupID, TopicID: child.TopicID})
				}
			}
		}
		if err != nil {
			created.Topic = failedOutcome(err)
		} else if response.ID <= 0 {
			created.Topic = domain.ActionOutcome{Status: "held", Message: "Canvas accepted topic creation but returned no topic ID; no duplicate was attempted and publication cannot continue; locate the topic in Canvas and pull its exact ID"}
		} else {
			created.Topic = domain.ActionOutcome{Status: "succeeded", ID: response.ID}
		}
		if e := save(); e != nil {
			return report, fmt.Errorf("discussion creation outcome could not be persisted: %w", e)
		}
		if !definite(created.Topic) {
			report.Status = created.Topic.Status
			report.Message = created.Topic.Message
			report.TopicID = created.TopicID
			return report, nil
		}
	}
	if created.TopicID > 0 && ambiguous(created.Topic) {
		var live NativeDiscussion
		if err = s.Client.Get(ctx, discussionPath(c.ID, d.GroupID, created.TopicID), &live); err != nil {
			return report, err
		}
		if live.ID != created.TopicID || live.Title != created.IntendedTitle || academicHTML(live.Message) != academicHTML(created.IntendedHTML) || live.IsAnnouncement != discussionBool(d.IsAnnouncement, false) || (d.GroupCategoryID > 0 && (live.GroupCategoryID == nil || *live.GroupCategoryID != d.GroupCategoryID)) {
			report.Status = "uncertain"
			report.TopicID = created.TopicID
			report.Message = "known topic ID does not match the saved discussion/announcement request; no further writes were made"
			return report, nil
		}
		created.Topic = domain.ActionOutcome{Status: "succeeded", ID: created.TopicID, Message: "recovered by exact topic ID after unclear response"}
		if err = save(); err != nil {
			return report, err
		}
	}
	if !definite(created.Topic) {
		report.Status = created.Topic.Status
		report.Message = created.Topic.Message
		report.TopicID = created.TopicID
		return report, nil
	}
	if a := created.Intent.Assignment; a != nil {
		if held, e := s.finishGradedDiscussion(ctx, c, &d, created, a, save, &report); e != nil || held {
			return report, e
		}
	}
	// Publication is a separate scoped patch. A new topic always starts unpublished.
	if !created.PublishedRequested {
		created.Publication = domain.ActionOutcome{Status: "not-requested"}
	} else if created.Publication.Status == "pending" || created.Publication.Status == "uncertain" {
		// A prior publication request had an unclear outcome. Resolve it by exact
		// topic identity before considering any further write.
		var live NativeDiscussion
		if err = s.Client.Get(ctx, discussionPath(c.ID, d.GroupID, created.TopicID), &live); err != nil {
			return report, err
		}
		if live.ID != created.TopicID {
			return report, fmt.Errorf("Canvas returned a different discussion topic ID during publication recovery")
		}
		if live.Published {
			created.Publication = domain.ActionOutcome{Status: "succeeded", Message: "publication recovered from exact topic"}
		} else {
			report.Status = "uncertain"
			report.TopicID = created.TopicID
			report.Message = "publication remains uncertain; the topic is unpublished and was not replayed"
			return report, nil
		}
	}
	if created.PublishedRequested && created.Publication.Status != "succeeded" {
		created.Publication = domain.ActionOutcome{Status: "pending"}
		if err = save(); err != nil {
			return report, err
		}
		var response NativeDiscussion
		err = s.Client.WriteAccepted(ctx, "PUT", discussionPath(c.ID, d.GroupID, created.TopicID), map[string]any{"published": true}, &response)
		if err != nil {
			created.Publication = failedOutcome(err)
			if saveErr := save(); saveErr != nil {
				return report, fmt.Errorf("publication outcome could not be persisted: %w", saveErr)
			}
			report.Status = created.Publication.Status
			report.Message = created.Publication.Message
			report.TopicID = created.TopicID
			return report, nil
		}
		// Canvas accepted the exact publication write. The known topic ID supplies
		// identity; response shape is not a post-write verification requirement.
		created.Publication = domain.ActionOutcome{Status: "succeeded"}
	}
	created.Complete = true
	if err = save(); err != nil {
		return report, err
	}
	report.Status = "completed"
	report.TopicID = created.TopicID
	report.URL = created.URL
	return report, nil
}

func discussionIntent(d domain.DiscussionDocument) domain.DiscussionIntent {
	return domain.DiscussionIntent{DiscussionType: d.DiscussionType, RequireInitialPost: d.RequireInitialPost, Published: discussionBool(d.Published, false), DelayedPostAt: d.DelayedPostAt, LockAt: d.LockAt, Pinned: d.Pinned, IsAnnouncement: discussionBool(d.IsAnnouncement, false), AnnouncementIntent: d.AnnouncementIntent, Assignment: d.Assignment}
}

func (s *Service) findUncertainDiscussion(ctx context.Context, courseID, groupID, setID int64, startedAt, title, html string, announcement bool) (NativeDiscussion, error) {
	started, err := time.Parse(time.RFC3339Nano, startedAt)
	if err != nil {
		return NativeDiscussion{}, fmt.Errorf("uncertain discussion create lacks its original start time; select the exact Canvas topic before retrying")
	}
	var topics []NativeDiscussion
	if err = s.Client.List(ctx, discussionBase(courseID, groupID)+"?per_page=100", &topics); err != nil {
		return NativeDiscussion{}, err
	}
	matches := []NativeDiscussion{}
	for _, topic := range topics {
		created, parseErr := time.Parse(time.RFC3339Nano, topic.CreatedAt)
		if parseErr != nil || topic.ID <= 0 || topic.Title != title || academicHTML(topic.Message) != academicHTML(html) || topic.IsAnnouncement != announcement {
			continue
		}
		if setID > 0 && (topic.GroupCategoryID == nil || *topic.GroupCategoryID != setID) {
			continue
		}
		if created.Before(started.Add(-time.Minute)) || created.After(started.Add(time.Hour)) {
			continue
		}
		matches = append(matches, topic)
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) == 0 {
		return NativeDiscussion{}, fmt.Errorf("no exact recent topic is visible after the unclear create; no duplicate was attempted, inspect Canvas and select/pull the exact result")
	}
	return NativeDiscussion{}, fmt.Errorf("multiple exact recent topics match this unclear create; no duplicate was attempted, select/pull the intended topic")
}

func discussionBool(v *bool, fallback bool) bool {
	if v == nil {
		return fallback
	}
	return *v
}

func (s *Service) updateDiscussion(ctx context.Context, path string, c domain.Course, d *domain.DiscussionDocument, body string, r DiscussionReport) (DiscussionReport, error) {
	r.TopicID = d.TopicID
	r.URL = d.Observed.URL
	if definite(d.UpdateOutcome) {
		r.Status = "completed"
		r.Message = "update already completed; pull-discussion before another edit"
		return r, nil
	}
	if (d.IsAnnouncement != nil && *d.IsAnnouncement) != d.Observed.IsAnnouncement && d.IsAnnouncement != nil {
		return r, fmt.Errorf("a discussion cannot be converted to an announcement or vice versa")
	}
	html, err := render.TeachingHTML(body, d.BodyFormat)
	if err != nil {
		return r, err
	}
	var live NativeDiscussion
	if err = s.Client.Get(ctx, discussionPath(c.ID, d.GroupID, d.TopicID), &live); err != nil {
		return r, err
	}
	if live.ID != d.TopicID {
		return r, fmt.Errorf("Canvas returned a different discussion topic ID")
	}
	base := d.Observed
	liveSetID := int64(0)
	if live.GroupCategoryID != nil {
		liveSetID = *live.GroupCategoryID
	}
	if liveSetID != base.GroupCategoryID || live.AssignmentID != base.AssignmentID || live.IsAnnouncement != base.IsAnnouncement {
		folder, e := writePullFolder(s.Root, c.Nickname, live.ID, live.Title)
		if e != nil {
			return r, e
		}
		fresh, e := s.writePulledDiscussion(c, live, liveSetID, d.GroupID, folder)
		if e != nil {
			return r, e
		}
		r.Status = "held"
		r.Message = "live discussion scope or identity changed; unsent edit preserved; fresh version: " + fresh
		return r, nil
	}
	contentChanged := d.Title != base.Title || html != base.BodyHTML
	if contentChanged && (live.Title != base.Title || live.Message != base.BodyHTML) {
		folder, e := writePullFolder(s.Root, c.Nickname, live.ID, live.Title)
		if e != nil {
			return r, e
		}
		fresh, e := s.writePulledDiscussion(c, live, d.GroupCategoryID, d.GroupID, folder)
		if e != nil {
			return r, e
		}
		r.Status = "held"
		r.Message = "live discussion title or body changed; unsent edit preserved; fresh version: " + fresh
		return r, nil
	}
	patch := map[string]any{}
	if d.Title != base.Title {
		patch["title"] = d.Title
	}
	if html != base.BodyHTML {
		patch["message"] = html
	}
	if d.DiscussionType != "" && d.DiscussionType != base.DiscussionType {
		patch["discussion_type"] = d.DiscussionType
	}
	if d.RequireInitialPost != nil && *d.RequireInitialPost != base.RequireInitialPost {
		patch["require_initial_post"] = *d.RequireInitialPost
	}
	if d.Published != nil && *d.Published != base.Published {
		patch["published"] = *d.Published
	}
	if d.DelayedPostAt != nil && !sameDate(d.DelayedPostAt, base.DelayedPostAt) {
		patch["delayed_post_at"] = *d.DelayedPostAt
	}
	if d.LockAt != nil && !sameDate(d.LockAt, base.LockAt) {
		patch["lock_at"] = *d.LockAt
	}
	if d.Pinned != nil && *d.Pinned != base.Pinned {
		patch["pinned"] = *d.Pinned
	}
	if len(patch) == 0 {
		d.UpdateOutcome = domain.ActionOutcome{Status: "preserved"}
		r.Status = "completed"
		return r, workspace.WriteDocument(path, d, body)
	}
	if base.IsAnnouncement && strings.TrimSpace(d.AnnouncementIntent) == "" {
		return r, fmt.Errorf("announcement updates require explicit announcement_intent describing the requested content and audience")
	}
	d.UpdateOutcome = domain.ActionOutcome{Status: "pending"}
	if err = workspace.WriteDocument(path, d, body); err != nil {
		return r, err
	}
	var response NativeDiscussion
	err = s.Client.WriteAccepted(ctx, "PUT", discussionPath(c.ID, d.GroupID, d.TopicID), patch, &response)
	if err != nil {
		d.UpdateOutcome = failedOutcome(err)
	} else {
		d.UpdateOutcome = domain.ActionOutcome{Status: "succeeded", ID: d.TopicID}
	}
	if e := workspace.WriteDocument(path, d, body); e != nil {
		return r, fmt.Errorf("discussion update outcome could not be saved: %w", e)
	}
	r.Status = d.UpdateOutcome.Status
	r.Message = d.UpdateOutcome.Message
	return r, nil
}

// finishGradedDiscussion resolves the linked assignment's identity and attaches
// the independent rubric. It runs after the topic exists and before publication;
// a failure holds publication and a resume repeats only the missing stage.
func (s *Service) finishGradedDiscussion(ctx context.Context, c domain.Course, d *domain.DiscussionDocument, created *domain.DiscussionCreated, a *domain.DiscussionAssignment, save func() error, report *DiscussionReport) (bool, error) {
	hold := func(status, message string) (bool, error) {
		report.Status, report.Message, report.TopicID = status, message, created.TopicID
		return true, nil
	}
	if created.AssignmentID == 0 {
		var live NativeDiscussion
		if err := s.Client.Get(ctx, discussionPath(c.ID, d.GroupID, created.TopicID), &live); err != nil {
			return false, err
		}
		if live.ID != created.TopicID {
			return false, fmt.Errorf("Canvas returned a different discussion topic ID")
		}
		if live.AssignmentID == 0 {
			return hold("held", "the topic exists but Canvas returned no linked assignment ID; publication and rubric were not attempted. Pull the topic to see its actual state")
		}
		created.AssignmentID = live.AssignmentID
		if err := save(); err != nil {
			return false, err
		}
	}
	if a.Rubric == nil {
		if !definite(created.Rubric) {
			created.Rubric = domain.ActionOutcome{Status: "not-requested"}
		}
		return false, nil
	}
	if definite(created.Rubric) {
		return false, nil
	}
	live, err := GetAssignment(ctx, s.Client, c.ID, created.AssignmentID)
	if err != nil {
		return false, err
	}
	pointsBefore := live.Points
	switch {
	case len(live.Rubric) > 0 && !ambiguous(created.Rubric):
		created.Rubric = domain.ActionOutcome{Status: "preserved", Message: "the linked assignment already has a rubric in Canvas; left as it is"}
	case ambiguous(created.Rubric):
		snap, e := Snapshot(live, s.Client.Origin())
		if e != nil {
			return false, e
		}
		if snap.Rubric == nil {
			if saveErr := save(); saveErr != nil {
				return false, saveErr
			}
			return hold("held", "rubric creation completion remains uncertain and no rubric is attached; attach one and run again")
		}
		created.RubricID, created.RubricAssociationID = snap.Rubric.ID, snap.Rubric.AssociationID
		created.Rubric = domain.ActionOutcome{Status: "succeeded", ID: created.RubricID}
	default:
		created.Rubric = domain.ActionOutcome{Status: "pending"}
		if err = save(); err != nil {
			return false, err
		}
		var response struct {
			Rubric struct {
				ID int64 `json:"id"`
			} `json:"rubric"`
			Association struct {
				ID int64 `json:"id"`
			} `json:"rubric_association"`
		}
		err = s.Client.Write(ctx, "POST", fmt.Sprintf("/api/v1/courses/%d/rubrics", c.ID), rubricCreateFields(*a.Rubric, created.AssignmentID), &response)
		if err != nil {
			created.Rubric = failedOutcome(err)
		} else if response.Rubric.ID <= 0 || response.Association.ID <= 0 {
			created.Rubric = domain.ActionOutcome{Status: "uncertain", Message: "rubric response lacks exact resource IDs"}
		} else {
			created.RubricID, created.RubricAssociationID = response.Rubric.ID, response.Association.ID
			created.Rubric = domain.ActionOutcome{Status: "succeeded", ID: created.RubricID}
		}
	}
	if err = save(); err != nil {
		return false, fmt.Errorf("rubric outcome could not be persisted: %w", err)
	}
	if !definite(created.Rubric) {
		return hold(created.Rubric.Status, created.Rubric.Message)
	}
	// Attaching a rubric is the author action known to change points; check the
	// scale before the discussion becomes public.
	after, err := GetAssignment(ctx, s.Client, c.ID, created.AssignmentID)
	if err != nil {
		return false, err
	}
	if after.Points != pointsBefore {
		return hold("held", "rubric attachment changed the linked assignment's points; the discussion remains unpublished")
	}
	return false, nil
}
