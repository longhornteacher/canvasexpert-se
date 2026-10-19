package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"canvasexpertse/internal/canvas"
	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/render"
	"canvasexpertse/internal/workspace"
)

type canvasID int64

func (id *canvasID) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		*id = 0
		return nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return err
	}
	*id = canvasID(n)
	return nil
}

type discussionAttachment struct {
	ID          canvasID `json:"id"`
	Filename    string   `json:"filename"`
	DisplayName string   `json:"display_name"`
	URL         string   `json:"url"`
	ContentType string   `json:"content-type"`
	Size        int64    `json:"size"`
}

type discussionEntry struct {
	ID             canvasID               `json:"id"`
	UserID         canvasID               `json:"user_id"`
	ParentID       canvasID               `json:"parent_id"`
	Message        string                 `json:"message"`
	CreatedAt      string                 `json:"created_at"`
	UpdatedAt      string                 `json:"updated_at"`
	Deleted        bool                   `json:"deleted"`
	DeletedAt      string                 `json:"deleted_at"`
	HasMoreReplies bool                   `json:"has_more_replies"`
	RepliesCount   int                    `json:"replies_count"`
	Attachments    []discussionAttachment `json:"attachments"`
	Replies        []discussionEntry      `json:"replies"`
}

type discussionView struct {
	View       json.RawMessage   `json:"view"`
	NewEntries []discussionEntry `json:"new_entries"`
}

// PullDiscussionEvidence creates the conversation packet beside the existing
// editable topic baseline. The baseline remains topic.md for 05a compatibility.
func (s *Service) PullDiscussionEvidence(ctx context.Context, nickname string, topic NativeDiscussion, course domain.Course, setID, groupID int64, folder string) (domain.DiscussionPacket, error) {
	packet := domain.DiscussionPacket{Schema: 1, Kind: "discussion-packet", Course: course.Nickname, Instance: s.Client.Origin(), CourseID: course.ID, ParentTopicID: topic.ID, AssignmentID: topic.AssignmentID, PulledAt: time.Now().UTC().Format(time.RFC3339Nano), Status: "complete"}
	contexts := []discussionSourceTarget{}
	if groupID > 0 {
		contexts = append(contexts, discussionSourceTarget{contextType: "Group", contextID: groupID, topicID: topic.ID, groupID: groupID})
	} else if setID == 0 {
		contexts = append(contexts, discussionSourceTarget{contextType: "Course", contextID: course.ID, topicID: topic.ID})
	} else {
		var groups []canvasGroup
		if err := s.Client.List(ctx, fmt.Sprintf("/api/v1/group_categories/%d/groups?per_page=100", setID), &groups); err != nil {
			packet.Status = "partial"
			packet.Sources = append(packet.Sources, domain.DiscussionSource{ContextType: "Course", ContextID: course.ID, TopicID: topic.ID, Status: "partial", Reason: "club roster could not be verified: " + err.Error()})
		} else {
			valid := map[int64]bool{}
			for _, g := range groups {
				if g.ID <= 0 || g.ContextType != "Course" || g.CourseID != course.ID || g.GroupCategoryID != setID || nonCollaborative(g.NonCollaborative) {
					return packet, fmt.Errorf("Canvas returned a club outside collaborative group set %d", setID)
				}
				valid[g.ID] = true
			}
			seen := map[int64]bool{}
			for _, child := range topic.GroupTopicChildren {
				gid, tid := int64(child.GroupID), int64(child.TopicID)
				if gid <= 0 || tid <= 0 || seen[gid] {
					continue
				}
				seen[gid] = true
				if !valid[gid] {
					packet.Status = "partial"
					packet.Sources = append(packet.Sources, domain.DiscussionSource{ContextType: "Group", ContextID: gid, TopicID: tid, GroupID: gid, Status: "partial", Reason: "distributed child does not resolve to a collaborative club in the selected course and set"})
					continue
				}
				contexts = append(contexts, discussionSourceTarget{contextType: "Group", contextID: gid, topicID: tid, groupID: gid})
			}
			if len(topic.GroupTopicChildren) == 0 {
				packet.Status = "partial"
				packet.Sources = append(packet.Sources, domain.DiscussionSource{ContextType: "Course", ContextID: course.ID, TopicID: topic.ID, Status: "partial", Reason: "Canvas returned no distributed club child topics"})
			}
		}
	}

	authors, authorStatus := s.discussionAuthors()
	if authorStatus != "complete" {
		packet.Status = "partial"
	}
	for _, target := range contexts {
		var exactTopic NativeDiscussion
		if err := s.Client.Get(ctx, discussionPathFor(target), &exactTopic); err != nil || exactTopic.ID != target.topicID {
			packet.Status = "partial"
			packet.Sources = append(packet.Sources, domain.DiscussionSource{ContextType: target.contextType, ContextID: target.contextID, TopicID: target.topicID, GroupID: target.groupID, Status: "partial", Reason: "exact child topic could not be verified in its selected course/club context"})
			continue
		}
		entries, method, reason := s.discussionEntries(ctx, target)
		source := domain.DiscussionSource{ContextType: target.contextType, ContextID: target.contextID, TopicID: target.topicID, GroupID: target.groupID, Method: method, Status: "complete"}
		if reason != "" {
			source.Status, source.Reason, packet.Status = "partial", reason, "partial"
		}
		mapped, mapReason := s.packetEntries(ctx, folder, target, entries, authors)
		if mapReason != "" {
			source.Status, source.Reason, packet.Status = "partial", mapReason, "partial"
		}
		source.EntryCount = len(mapped)
		packet.Sources = append(packet.Sources, source)
		packet.Entries = append(packet.Entries, mapped...)
	}
	if authorStatus != "complete" {
		for i := range packet.Sources {
			packet.Sources[i].Status = "partial"
			if packet.Sources[i].Reason == "" {
				packet.Sources[i].Reason = "some authors could not be mapped to stable student refs; unresolved authors remain unknown"
			}
		}
	}
	sort.SliceStable(packet.Sources, func(i, j int) bool {
		if packet.Sources[i].ContextType != packet.Sources[j].ContextType {
			return packet.Sources[i].ContextType < packet.Sources[j].ContextType
		}
		return packet.Sources[i].ContextID < packet.Sources[j].ContextID
	})
	sort.SliceStable(packet.Entries, func(i, j int) bool {
		a, b := packet.Entries[i], packet.Entries[j]
		if a.ContextType != b.ContextType {
			return a.ContextType < b.ContextType
		}
		if a.ContextID != b.ContextID {
			return a.ContextID < b.ContextID
		}
		if a.CreatedAt != b.CreatedAt {
			return a.CreatedAt < b.CreatedAt
		}
		return a.EntryID < b.EntryID
	})
	return packet, nil
}

type discussionSourceTarget struct {
	contextType                 string
	contextID, topicID, groupID int64
}

func (s *Service) discussionEntries(ctx context.Context, target discussionSourceTarget) ([]discussionEntry, string, string) {
	base := discussionPathFor(target) + "/view?include_new_entries=1"
	var envelope discussionView
	var err error
	for attempt := 0; attempt < 2; attempt++ {
		envelope = discussionView{}
		err = s.Client.Get(ctx, base, &envelope)
		if err == nil && len(envelope.View) > 0 && string(envelope.View) != "null" {
			break
		}
	}
	var all []discussionEntry
	partial, method := "", "threaded_view"
	if err == nil && len(envelope.View) > 0 && string(envelope.View) != "null" {
		if e := json.Unmarshal(envelope.View, &all); e != nil {
			err = e
		}
	}
	if err != nil || len(envelope.View) == 0 || string(envelope.View) == "null" {
		all, err = s.paginatedDiscussionEntries(ctx, target)
		if err != nil {
			return nil, "", "threaded view unavailable and paginated entries could not be read: " + err.Error()
		}
		method = "paginated_entries"
	}
	// The threaded view is complete in normal Canvas responses. If it explicitly
	// signals omitted replies, replace those previews with the paginated reply list.
	for i := range all {
		if all[i].HasMoreReplies || all[i].RepliesCount > len(all[i].Replies) {
			var replies []discussionEntry
			path := fmt.Sprintf("%s/entries/%d/replies?per_page=100", discussionPathFor(target), all[i].ID)
			if e := s.Client.List(ctx, path, &replies); e != nil {
				partial = "one or more parent entries have unavailable replies: " + e.Error()
				continue
			}
			all[i].Replies = mergeDiscussionEntries(all[i].Replies, replies)
			all[i].HasMoreReplies = false
			all[i].RepliesCount = len(all[i].Replies)
		}
	}
	flat := flattenDiscussionEntries(target, all)
	newEntries := flattenDiscussionEntries(target, envelope.NewEntries)
	return mergeDiscussionEntries(flat, newEntries), method, partial
}

func discussionPathFor(t discussionSourceTarget) string {
	if t.contextType == "Group" {
		return fmt.Sprintf("/api/v1/groups/%d/discussion_topics/%d", t.contextID, t.topicID)
	}
	return fmt.Sprintf("/api/v1/courses/%d/discussion_topics/%d", t.contextID, t.topicID)
}

func (s *Service) paginatedDiscussionEntries(ctx context.Context, target discussionSourceTarget) ([]discussionEntry, error) {
	var roots []discussionEntry
	path := discussionPathFor(target) + "/entries?per_page=100"
	if err := s.Client.List(ctx, path, &roots); err != nil {
		return nil, err
	}
	for i := range roots {
		if !roots[i].HasMoreReplies && roots[i].RepliesCount <= len(roots[i].Replies) {
			continue
		}
		var replies []discussionEntry
		path := fmt.Sprintf("%s/entries/%d/replies?per_page=100", discussionPathFor(target), roots[i].ID)
		if err := s.Client.List(ctx, path, &replies); err != nil {
			roots[i].HasMoreReplies = true
			continue
		}
		roots[i].Replies = mergeDiscussionEntries(roots[i].Replies, replies)
		roots[i].HasMoreReplies = false
	}
	return roots, nil
}

func mergeDiscussionEntries(older, newer []discussionEntry) []discussionEntry {
	result := append([]discussionEntry(nil), older...)
	index := map[int64]int{}
	for i, e := range result {
		if e.ID > 0 {
			index[int64(e.ID)] = i
		}
	}
	for _, e := range newer {
		id := int64(e.ID)
		if id <= 0 {
			continue
		}
		if i, ok := index[id]; ok {
			// The flat new_entries response is newer than its cached view version.
			e.Replies = mergeDiscussionEntries(result[i].Replies, e.Replies)
			result[i] = e
		} else {
			index[id] = len(result)
			result = append(result, e)
		}
	}
	return result
}

func flattenDiscussionEntries(target discussionSourceTarget, roots []discussionEntry) []discussionEntry {
	var out []discussionEntry
	seen := map[int64]bool{}
	var visit func(discussionEntry, int64)
	visit = func(e discussionEntry, parent int64) {
		id := int64(e.ID)
		if id <= 0 || seen[id] {
			return
		}
		seen[id] = true
		if int64(e.ParentID) > 0 {
			parent = int64(e.ParentID)
		}
		e.ParentID = canvasID(parent)
		replies := e.Replies
		e.Replies = nil
		out = append(out, e)
		for _, child := range replies {
			visit(child, id)
		}
	}
	for _, e := range roots {
		visit(e, 0)
	}
	return out
}

type discussionAuthor struct{ kind, ref, alias string }

func (s *Service) discussionAuthors() (map[int64]discussionAuthor, string) {
	store, err := workspace.OpenStudents(s.Root)
	if err != nil {
		return map[int64]discussionAuthor{}, "author mapping unavailable: " + err.Error()
	}
	byID := map[int64]discussionAuthor{}
	for _, student := range store.Rows {
		if student.Instance == s.Client.Origin() && student.UserID > 0 {
			byID[student.UserID] = discussionAuthor{kind: "student", ref: student.Ref, alias: student.Alias}
		}
	}
	// Discussion pulls deliberately do not read the course roster. Existing
	// private mappings are enough for stable refs; other authors remain unknown.
	return byID, "complete"
}

func (s *Service) packetEntries(ctx context.Context, folder string, target discussionSourceTarget, entries []discussionEntry, authors map[int64]discussionAuthor) ([]domain.DiscussionEntry, string) {
	out := make([]domain.DiscussionEntry, 0, len(entries))
	partial := ""
	for _, entry := range entries {
		id := int64(entry.ID)
		if id <= 0 {
			continue
		}
		row := domain.DiscussionEntry{EntryID: id, ContextType: target.contextType, ContextID: target.contextID, TopicID: target.topicID, ParentEntryID: int64(entry.ParentID), CreatedAt: entry.CreatedAt, UpdatedAt: entry.UpdatedAt, Deleted: entry.Deleted || entry.DeletedAt != ""}
		if author, ok := authors[int64(entry.UserID)]; ok {
			row.AuthorKind, row.StudentRef, row.Alias = author.kind, author.ref, author.alias
		} else {
			row.AuthorKind = "unknown"
		}
		if !row.Deleted {
			row.HTML = entry.Message
			row.ReadableText = render.HTMLText(entry.Message)
		}
		attachments := make([]Attachment, 0, len(entry.Attachments))
		for _, a := range entry.Attachments {
			name := a.Filename
			if name == "" {
				name = a.DisplayName
			}
			attachments = append(attachments, Attachment{ID: int64(a.ID), Filename: name, DisplayName: a.DisplayName, URL: a.URL, ContentType: a.ContentType, Size: a.Size})
		}
		if len(attachments) > 0 {
			if row.Deleted {
				for _, a := range attachments {
					name := a.Filename
					if name == "" {
						name = a.DisplayName
					}
					row.Attachments = append(row.Attachments, domain.Attachment{ID: a.ID, Name: name, Status: "not-downloaded", Reason: "entry is deleted; attachment content was not restored"})
				}
			} else {
				// folder is an already constrained absolute pull path; derive its
				// workspace-relative form before using the shared safe downloader.
				rel, err := filepath.Rel(s.Root, folder)
				if err != nil {
					partial = "attachments could not be placed safely: " + err.Error()
				} else {
					ref := fmt.Sprintf("context-%s-%d-topic-%d-entry-%d", strings.ToLower(target.contextType), target.contextID, target.topicID, id)
					items, e := s.downloadAttachments(ctx, rel, ref, 0, attachments)
					row.Attachments = items
					if e != nil {
						partial = "one or more attachments could not be saved: " + e.Error()
					}
					for _, item := range items {
						if item.Status != "readable" && item.Status != "downloaded" {
							partial = "one or more attachments are linked but unreadable or unavailable"
						}
					}
				}
			}
		}
		if entry.HasMoreReplies {
			partial = "one or more reply lists remain incomplete"
		}
		out = append(out, row)
	}
	return out, partial
}

func writeDiscussionPacket(folder string, packet domain.DiscussionPacket) (string, error) {
	path, err := workspace.ConstrainPath(folder, "discussion-packet.yaml")
	if err != nil {
		return "", err
	}
	if err = workspace.WriteDocument(path, &packet, ""); err != nil {
		return "", err
	}
	return path, nil
}

func (s *Service) PushDiscussionReply(ctx context.Context, path string) (DiscussionReport, error) {
	report := DiscussionReport{Path: path}
	var d domain.DiscussionReplyDocument
	body, err := workspace.ReadDocument(path, &d)
	if err != nil {
		return report, err
	}
	if err = d.ValidateBody(body); err != nil {
		return report, err
	}
	course, err := s.Course(d.Course)
	if err != nil {
		return report, err
	}
	if (d.Instance != "" && d.Instance != s.Client.Origin()) || (d.CourseID != 0 && d.CourseID != course.ID) || d.ContextID != course.ID && d.ContextType == "Course" {
		return report, fmt.Errorf("discussion reply identity differs from configured Canvas/course")
	}
	if d.Outcome.Status == "succeeded" {
		report.Status, report.TopicID, report.Message = "completed", d.TopicID, "reply was already accepted and will not be repeated"
		return report, nil
	}
	if d.Outcome.Status != "" {
		report.Status, report.TopicID, report.Message = d.Outcome.Status, d.TopicID, "saved reply outcome prevents an automatic repeat; resolve the prior attempt before creating a new reply document"
		return report, nil
	}
	if d.ContextType == "Group" {
		if _, err = s.verifyPageGroup(ctx, d.Course, d.GroupID); err != nil {
			return report, err
		}
	}
	var topic NativeDiscussion
	if err = s.Client.Get(ctx, discussionPathFor(discussionSourceTarget{contextType: d.ContextType, contextID: d.ContextID, topicID: d.TopicID}), &topic); err != nil {
		return report, err
	}
	if topic.ID != d.TopicID {
		return report, fmt.Errorf("Canvas returned a different discussion topic ID")
	}
	if d.ContextType == "Group" {
		var group canvasGroup
		if err = s.Client.Get(ctx, fmt.Sprintf("/api/v1/groups/%d", d.GroupID), &group); err != nil {
			return report, err
		}
		if group.ID != d.GroupID || group.CourseID != course.ID || group.ContextType != "Course" || nonCollaborative(group.NonCollaborative) {
			return report, fmt.Errorf("reply group does not belong to the configured course")
		}
	} else if topic.GroupCategoryID != nil && *topic.GroupCategoryID > 0 {
		if _, err = s.groupCategory(ctx, course.ID, *topic.GroupCategoryID); err != nil {
			return report, err
		}
	}
	html, err := render.TeachingHTML(body, d.BodyFormat)
	if err != nil {
		return report, err
	}
	postPath := discussionPathFor(discussionSourceTarget{contextType: d.ContextType, contextID: d.ContextID, topicID: d.TopicID}) + "/entries"
	if d.ParentEntryID > 0 {
		parentPath := fmt.Sprintf("%s/entries/%d/replies", discussionPathFor(discussionSourceTarget{contextType: d.ContextType, contextID: d.ContextID, topicID: d.TopicID}), d.ParentEntryID)
		var replies []discussionEntry
		if err = s.Client.List(ctx, parentPath+"?per_page=100", &replies); err != nil {
			return report, fmt.Errorf("parent entry is not available under this exact topic; no reply was posted")
		}
		postPath = parentPath
	}
	d.Instance, d.CourseID, d.Course = s.Client.Origin(), course.ID, course.Nickname
	d.Outcome = domain.ActionOutcome{Status: "pending", Message: "teacher reply write is being attempted"}
	if err = workspace.WriteDocument(path, &d, body); err != nil {
		return report, err
	}
	var accepted struct {
		ID canvasID `json:"id"`
	}
	err = s.Client.WriteAccepted(ctx, "POST", postPath, map[string]any{"message": html}, &accepted)
	if err != nil {
		outcome := "failed"
		if canvasIsUncertain(err) {
			outcome = "uncertain"
		}
		d.Outcome = domain.ActionOutcome{Status: outcome, Message: err.Error()}
		if saveErr := workspace.WriteDocument(path, &d, body); saveErr != nil {
			return report, fmt.Errorf("reply %s and outcome could not be saved: %w", outcome, saveErr)
		}
		report.Status, report.TopicID, report.Message = outcome, d.TopicID, "reply write outcome was saved; it will not be retried automatically"
		return report, nil
	}
	d.Outcome = domain.ActionOutcome{Status: "succeeded", ID: int64(accepted.ID), Message: "Canvas accepted the teacher reply"}
	if err = workspace.WriteDocument(path, &d, body); err != nil {
		return report, fmt.Errorf("reply was accepted but its outcome could not be saved: %w", err)
	}
	report.Status, report.TopicID, report.Message = "completed", d.TopicID, "Canvas accepted the teacher reply"
	return report, nil
}

func canvasIsUncertain(err error) bool {
	var ce *canvas.Error
	return errors.As(err, &ce) && ce.Uncertain
}

func writePullFolder(root, nickname string, topicID int64, title string) (string, error) {
	name := fmt.Sprintf("discussion-%d-%s-%s", topicID, Slug(title), time.Now().UTC().Format("20060102T150405.000000000"))
	p, err := workspace.ConstrainPath(root, filepath.Join("courses", nickname, "pulled", name))
	if err != nil {
		return "", err
	}
	if err = os.MkdirAll(p, 0755); err != nil {
		return "", err
	}
	return p, nil
}
