package workflow

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"canvasexpertse/internal/canvas"
	"canvasexpertse/internal/workspace"
)

type InboxListOptions struct {
	Scope  string
	Unread bool
	Course string
}

type InboxAttachment struct {
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
	Size int64  `json:"size,omitempty"`
}

type InboxMessage struct {
	Ref         string            `json:"ref"`
	Author      string            `json:"author"`
	Teacher     bool              `json:"teacher"`
	Time        string            `json:"time,omitempty"`
	Body        string            `json:"body"`
	System      bool              `json:"system"`
	Attachments []InboxAttachment `json:"attachments,omitempty"`
}

type InboxConversation struct {
	Ref          string   `json:"ref"`
	Subject      string   `json:"subject"`
	Course       string   `json:"course,omitempty"`
	Participants []string `json:"participants"`
	MessageCount int      `json:"message_count"`
	LastMessage  string   `json:"last_message_at,omitempty"`
	Read         bool     `json:"read"`
	Archived     bool     `json:"archived"`
	Preview      string   `json:"preview"`
}

type InboxThread struct {
	Ref          string         `json:"ref"`
	Subject      string         `json:"subject"`
	Course       string         `json:"course,omitempty"`
	Participants []string       `json:"participants"`
	Messages     []InboxMessage `json:"messages"`
}

type InboxStateResult struct {
	Conversation string `json:"conversation"`
	State        string `json:"state"`
	Status       string `json:"status"`
	Error        string `json:"error,omitempty"`
}

type InboxReplyResult struct {
	MessageRef string `json:"message_ref,omitempty"`
	Status     string `json:"status"`
	Note       string `json:"note,omitempty"`
}

type inboxConversationLive struct {
	ID               int64                 `json:"id"`
	Subject          string                `json:"subject"`
	ContextCode      string                `json:"context_code,omitempty"`
	AudienceContexts inboxAudienceContexts `json:"audience_contexts"`
	Participants     []inboxParticipant    `json:"participants"`
	Messages         []inboxMessageLive    `json:"messages"`
	MessageCount     int                   `json:"message_count"`
	LastMessageAt    string                `json:"last_message_at"`
	LastMessage      string                `json:"last_message"`
	WorkflowState    string                `json:"workflow_state"`
}

type inboxAudienceContexts struct {
	Courses map[string][]string `json:"courses"`
	Groups  map[string][]string `json:"groups"`
}

type inboxParticipant struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
	Role string `json:"role"`
}

type inboxMessageLive struct {
	ID          int64                 `json:"id"`
	AuthorID    int64                 `json:"author_id"`
	CreatedAt   string                `json:"created_at"`
	Body        string                `json:"body"`
	Generated   bool                  `json:"generated"`
	Attachments []inboxAttachmentLive `json:"attachments"`
}

type inboxAttachmentLive struct {
	DisplayName string `json:"display_name"`
	Filename    string `json:"filename"`
	ContentType string `json:"content-type"`
	Size        int64  `json:"size"`
}

type inboxEnrollment struct {
	UserID int64      `json:"user_id"`
	Type   string     `json:"type"`
	User   CanvasUser `json:"user"`
}

type inboxIdentity struct {
	label   string
	student bool
}

func inboxRef(origin string, id int64, prefix string) string {
	sum := sha256.Sum256([]byte(origin + ":" + strconv.FormatInt(id, 10)))
	return prefix + "-" + hex.EncodeToString(sum[:12])
}

func (s *Service) inboxScope(scope string) (string, error) {
	if scope == "" {
		scope = "inbox"
	}
	switch scope {
	case "inbox", "archived", "sent":
		return scope, nil
	default:
		return "", fmt.Errorf("scope must be inbox, archived, or sent")
	}
}

func (s *Service) inboxList(ctx context.Context, scope string) ([]inboxConversationLive, error) {
	var conversations []inboxConversationLive
	path := "/api/v1/conversations?per_page=100"
	switch scope {
	case "inbox":
	case "unread", "archived", "sent":
		path += "&scope=" + url.QueryEscape(scope)
	default:
		return nil, fmt.Errorf("unsupported Canvas conversation scope")
	}
	if err := s.Client.List(ctx, path, &conversations); err != nil {
		return nil, err
	}
	for _, conversation := range conversations {
		if conversation.ID <= 0 {
			return nil, fmt.Errorf("Canvas returned a conversation without a valid ID")
		}
	}
	return conversations, nil
}

func (s *Service) inboxAllConversations(ctx context.Context) ([]inboxConversationLive, error) {
	seen := map[int64]bool{}
	var all []inboxConversationLive
	for _, scope := range []string{"inbox", "archived", "sent"} {
		conversations, err := s.inboxList(ctx, scope)
		if err != nil {
			return nil, err
		}
		for _, conversation := range conversations {
			if !seen[conversation.ID] {
				all = append(all, conversation)
				seen[conversation.ID] = true
			}
		}
	}
	return all, nil
}

func (s *Service) inboxConversationByRef(ctx context.Context, ref string) (inboxConversationLive, error) {
	if !strings.HasPrefix(ref, "conversation-") {
		return inboxConversationLive{}, errors.New("conversation must be an Inbox conversation ref")
	}
	conversations, err := s.inboxAllConversations(ctx)
	if err != nil {
		return inboxConversationLive{}, err
	}
	for _, conversation := range conversations {
		if inboxRef(s.Client.Origin(), conversation.ID, "conversation") == ref {
			return conversation, nil
		}
	}
	return inboxConversationLive{}, errors.New("conversation ref was not found in the current Canvas Inbox")
}

func (s *Service) inboxCourseIDs(conversation inboxConversationLive) []int64 {
	available := map[int64]bool{}
	for rawID := range conversation.AudienceContexts.Courses {
		id, err := strconv.ParseInt(rawID, 10, 64)
		if err == nil && id > 0 {
			available[id] = true
		}
	}
	if idText, ok := strings.CutPrefix(conversation.ContextCode, "course_"); ok {
		if id, err := strconv.ParseInt(idText, 10, 64); err == nil && id > 0 {
			available[id] = true
		}
	}
	var ids []int64
	for _, course := range s.Config.Courses {
		if available[course.ID] {
			ids = append(ids, course.ID)
		}
	}
	return ids
}

func (s *Service) inboxCourseName(conversation inboxConversationLive) string {
	ids := s.inboxCourseIDs(conversation)
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		for _, course := range s.Config.Courses {
			if course.ID == id {
				names = append(names, course.Nickname)
				break
			}
		}
	}
	return strings.Join(names, ", ")
}

func containsInt64(values []int64, target int64) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func (s *Service) inboxCourseFilter(nickname string) (int64, error) {
	if strings.TrimSpace(nickname) == "" {
		return 0, nil
	}
	course, err := s.Course(nickname)
	if err != nil {
		return 0, err
	}
	return course.ID, nil
}

func (s *Service) inboxIdentities(ctx context.Context, courseID int64, teacherID int64, participants []inboxParticipant) (map[int64]inboxIdentity, error) {
	identities := make(map[int64]inboxIdentity, len(participants))
	participantIDs := make(map[int64]bool, len(participants))
	for _, participant := range participants {
		if participant.ID > 0 {
			participantIDs[participant.ID] = true
			if participant.ID == teacherID && teacherID > 0 {
				identities[participant.ID] = inboxIdentity{label: "teacher"}
			} else {
				identities[participant.ID] = inboxIdentity{label: inboxRoleLabel(participant.Role, participant.Type)}
			}
		}
	}
	for id := range participantIDs {
		if id == teacherID && teacherID > 0 {
			identities[id] = inboxIdentity{label: "teacher"}
		}
	}
	if courseID <= 0 {
		return identities, nil
	}
	var enrollments []inboxEnrollment
	if err := s.Client.List(ctx, fmt.Sprintf("/api/v1/courses/%d/enrollments?per_page=100", courseID), &enrollments); err != nil {
		return nil, err
	}
	students, err := workspace.OpenStudents(s.Root)
	if err != nil {
		return nil, err
	}
	changed := false
	for _, enrollment := range enrollments {
		id := enrollment.UserID
		if id == 0 {
			id = enrollment.User.ID
		}
		if id <= 0 || !participantIDs[id] || id == teacherID {
			continue
		}
		switch enrollment.Type {
		case "StudentEnrollment":
			student, e := students.Upsert(s.Client.Origin(), id, enrollment.User.Name, enrollment.User.ShortName, enrollment.User.SISID, enrollment.User.Login)
			if e != nil {
				return nil, fmt.Errorf("map Inbox student to stable reference: %w", e)
			}
			identities[id] = inboxIdentity{label: student.Alias, student: true}
			changed = true
		case "ParentEnrollment":
			identities[id] = inboxIdentity{label: "parent"}
		case "ObserverEnrollment":
			identities[id] = inboxIdentity{label: "observer"}
		case "TeacherEnrollment", "TaEnrollment", "DesignerEnrollment", "AccountAdminEnrollment":
			identities[id] = inboxIdentity{label: "colleague"}
		}
	}
	if changed {
		if err := students.Save(); err != nil {
			return nil, fmt.Errorf("save Inbox student references: %w", err)
		}
	}
	return identities, nil
}

func inboxRoleLabel(values ...string) string {
	for _, value := range values {
		switch strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), "_", "")) {
		case "parent", "parentenrollment":
			return "parent"
		case "observer", "observerenrollment":
			return "observer"
		case "teacher", "teacherenrollment", "ta", "taenrollment", "designer", "designerenrollment", "accountadmin", "accountadminenrollment":
			return "colleague"
		case "student", "studentenrollment":
			return "participant"
		}
	}
	return "participant"
}

func inboxParticipantIDs(participants []inboxParticipant) []int64 {
	ids := make([]int64, 0, len(participants))
	for _, participant := range participants {
		if participant.ID > 0 {
			ids = append(ids, participant.ID)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func (s *Service) inboxLabels(ctx context.Context, conversation inboxConversationLive, teacherID int64) ([]string, map[int64]inboxIdentity, error) {
	courseIDs := s.inboxCourseIDs(conversation)
	if len(courseIDs) == 0 {
		courseIDs = []int64{0}
	}
	identities := map[int64]inboxIdentity{}
	for _, courseID := range courseIDs {
		courseIdentities, err := s.inboxIdentities(ctx, courseID, teacherID, conversation.Participants)
		if err != nil {
			return nil, nil, err
		}
		for id, identity := range courseIdentities {
			if identity.student || identities[id].label == "" || identity.label == "teacher" {
				identities[id] = identity
			}
		}
	}
	return inboxLabelsFromIdentities(conversation.Participants, identities), identities, nil
}

func inboxLabelsFromIdentities(participants []inboxParticipant, identities map[int64]inboxIdentity) []string {
	labels := make([]string, 0, len(participants))
	for _, id := range inboxParticipantIDs(participants) {
		identity := identities[id]
		if identity.label == "" {
			identity.label = "participant"
		}
		labels = append(labels, identity.label)
	}
	sort.Strings(labels)
	return labels
}

func (s *Service) InboxList(ctx context.Context, options InboxListOptions) ([]InboxConversation, error) {
	scope, err := s.inboxScope(options.Scope)
	if err != nil {
		return nil, err
	}
	courseID, err := s.inboxCourseFilter(options.Course)
	if err != nil {
		return nil, err
	}
	listScope := scope
	if options.Unread && scope == "inbox" {
		listScope = "unread"
	}
	conversations, err := s.inboxList(ctx, listScope)
	if err != nil {
		return nil, err
	}
	var teacher struct {
		ID int64 `json:"id"`
	}
	if err = s.Client.Get(ctx, "/api/v1/users/self/profile", &teacher); err != nil {
		return nil, err
	}
	if teacher.ID <= 0 {
		return nil, errors.New("Canvas did not return the current teacher identity")
	}
	participantsByCourse := map[int64][]inboxParticipant{}
	included := map[int64]bool{}
	for _, conversation := range conversations {
		courseIDs := s.inboxCourseIDs(conversation)
		if courseID > 0 && !containsInt64(courseIDs, courseID) {
			continue
		}
		stateRead := conversation.WorkflowState != "unread"
		if options.Unread && stateRead {
			continue
		}
		if len(courseIDs) == 0 {
			courseIDs = []int64{0}
		}
		for _, actualCourseID := range courseIDs {
			group := participantsByCourse[actualCourseID]
			seenIDs := map[int64]bool{}
			for _, p := range group {
				seenIDs[p.ID] = true
			}
			for _, p := range conversation.Participants {
				if p.ID > 0 && !seenIDs[p.ID] {
					group = append(group, p)
					seenIDs[p.ID] = true
				}
			}
			participantsByCourse[actualCourseID] = group
		}
		included[conversation.ID] = true
	}
	identitiesByCourse := make(map[int64]map[int64]inboxIdentity, len(participantsByCourse))
	for actualCourseID, participants := range participantsByCourse {
		identities, identityErr := s.inboxIdentities(ctx, actualCourseID, teacher.ID, participants)
		if identityErr != nil {
			return nil, identityErr
		}
		identitiesByCourse[actualCourseID] = identities
	}
	result := make([]InboxConversation, 0, len(conversations))
	for _, conversation := range conversations {
		if !included[conversation.ID] {
			continue
		}
		archived := conversation.WorkflowState == "archived"
		read := conversation.WorkflowState != "unread"
		if options.Unread && read {
			continue
		}
		mergedIdentities := map[int64]inboxIdentity{}
		courseIDs := s.inboxCourseIDs(conversation)
		if len(courseIDs) == 0 {
			courseIDs = []int64{0}
		}
		for _, actualCourseID := range courseIDs {
			for id, identity := range identitiesByCourse[actualCourseID] {
				if identity.student || mergedIdentities[id].label == "" || identity.label == "teacher" {
					mergedIdentities[id] = identity
				}
			}
		}
		labels := inboxLabelsFromIdentities(conversation.Participants, mergedIdentities)
		count := conversation.MessageCount
		if count == 0 {
			count = len(conversation.Messages)
		}
		result = append(result, InboxConversation{
			Ref:          inboxRef(s.Client.Origin(), conversation.ID, "conversation"),
			Subject:      conversation.Subject,
			Course:       s.inboxCourseName(conversation),
			Participants: labels,
			MessageCount: count,
			LastMessage:  conversation.LastMessageAt,
			Read:         read,
			Archived:     archived,
			Preview:      conversation.LastMessage,
		})
	}
	return result, nil
}

func (s *Service) InboxRead(ctx context.Context, ref string, markRead bool) (InboxThread, error) {
	conversation, err := s.inboxConversationByRef(ctx, ref)
	if err != nil {
		return InboxThread{}, err
	}
	path := fmt.Sprintf("/api/v1/conversations/%d?auto_mark_as_read=%t", conversation.ID, markRead)
	var thread inboxConversationLive
	if err = s.Client.Get(ctx, path, &thread); err != nil {
		return InboxThread{}, err
	}
	if thread.ID != 0 && thread.ID != conversation.ID {
		return InboxThread{}, errors.New("Canvas returned a different conversation")
	}
	if thread.ID == 0 {
		thread.ID = conversation.ID
	}
	var teacher struct {
		ID int64 `json:"id"`
	}
	if err = s.Client.Get(ctx, "/api/v1/users/self/profile", &teacher); err != nil {
		return InboxThread{}, err
	}
	if teacher.ID <= 0 {
		return InboxThread{}, errors.New("Canvas did not return the current teacher identity")
	}
	labels, identities, err := s.inboxLabels(ctx, thread, teacher.ID)
	if err != nil {
		return InboxThread{}, err
	}
	result := InboxThread{
		Ref:          ref,
		Subject:      thread.Subject,
		Course:       s.inboxCourseName(thread),
		Participants: labels,
		Messages:     make([]InboxMessage, 0, len(thread.Messages)),
	}
	for _, message := range thread.Messages {
		identity := identities[message.AuthorID]
		if message.Generated {
			identity.label = "system"
		} else if identity.label == "" {
			identity.label = "participant"
		}
		item := InboxMessage{
			Ref:     inboxRef(s.Client.Origin(), message.ID, "message"),
			Author:  identity.label,
			Teacher: message.AuthorID == teacher.ID,
			Time:    message.CreatedAt,
			Body:    message.Body,
			System:  message.Generated,
		}
		for _, attachment := range message.Attachments {
			name := attachment.DisplayName
			if name == "" {
				name = attachment.Filename
			}
			item.Attachments = append(item.Attachments, InboxAttachment{Name: name, Type: attachment.ContentType, Size: attachment.Size})
		}
		result.Messages = append(result.Messages, item)
	}
	return result, nil
}

func (s *Service) InboxReply(ctx context.Context, ref, body, afterMessage string) (InboxReplyResult, error) {
	if strings.TrimSpace(body) == "" {
		return InboxReplyResult{}, errors.New("body must not be empty")
	}
	if !strings.HasPrefix(afterMessage, "message-") {
		return InboxReplyResult{}, errors.New("after_message must be a message ref from inbox_read")
	}
	conversation, err := s.inboxConversationByRef(ctx, ref)
	if err != nil {
		return InboxReplyResult{}, err
	}
	var current inboxConversationLive
	if err = s.Client.Get(ctx, fmt.Sprintf("/api/v1/conversations/%d?auto_mark_as_read=false", conversation.ID), &current); err != nil {
		return InboxReplyResult{}, err
	}
	if current.ID != 0 && current.ID != conversation.ID {
		return InboxReplyResult{}, errors.New("Canvas returned a different conversation")
	}
	if len(current.Messages) == 0 {
		return InboxReplyResult{}, errors.New("conversation has no messages to reply to")
	}
	last := current.Messages[len(current.Messages)-1]
	if inboxRef(s.Client.Origin(), last.ID, "message") != afterMessage {
		return InboxReplyResult{}, errors.New("conversation has a newer message; re-read it and check with the teacher before replying")
	}
	var response struct {
		ID int64 `json:"id"`
	}
	err = s.Client.WriteAccepted(ctx, http.MethodPost, fmt.Sprintf("/api/v1/conversations/%d/add_message", conversation.ID), map[string]string{"body": body}, &response)
	if err != nil {
		var canvasErr *canvas.Error
		if errors.As(err, &canvasErr) && canvasErr.Uncertain {
			return InboxReplyResult{}, errors.New("reply outcome is uncertain; read the thread before retrying")
		}
		return InboxReplyResult{}, err
	}
	if response.ID <= 0 {
		return InboxReplyResult{Status: "completed", Note: "Canvas accepted the reply but did not return a message ref; do not retry it."}, nil
	}
	return InboxReplyResult{MessageRef: inboxRef(s.Client.Origin(), response.ID, "message"), Status: "completed"}, nil
}

func (s *Service) InboxSetState(ctx context.Context, refs []string, state string) []InboxStateResult {
	if state != "read" && state != "unread" && state != "archived" && state != "inbox" {
		return []InboxStateResult{{State: state, Status: "failed", Error: "state must be read, unread, archived, or inbox"}}
	}
	if len(refs) == 0 {
		return []InboxStateResult{{State: state, Status: "failed", Error: "conversations must contain at least one ref"}}
	}
	conversations, err := s.inboxAllConversations(ctx)
	if err != nil {
		return []InboxStateResult{{State: state, Status: "failed", Error: err.Error()}}
	}
	byRef := make(map[string]int64, len(conversations))
	for _, conversation := range conversations {
		byRef[inboxRef(s.Client.Origin(), conversation.ID, "conversation")] = conversation.ID
	}
	results := make([]InboxStateResult, 0, len(refs))
	seen := map[string]bool{}
	for _, ref := range refs {
		result := InboxStateResult{Conversation: ref, State: state}
		id, ok := byRef[ref]
		switch {
		case seen[ref]:
			result.Status, result.Error = "failed", "duplicate conversation ref"
		case !strings.HasPrefix(ref, "conversation-") || !ok:
			result.Status, result.Error = "failed", "conversation ref was not found in the current Canvas Inbox"
		default:
			var response any
			err := s.Client.WriteAccepted(ctx, http.MethodPut, fmt.Sprintf("/api/v1/conversations/%d", id), map[string]string{"workflow_state": state}, &response)
			if err != nil {
				result.Status, result.Error = "failed", err.Error()
			} else {
				result.Status = "completed"
			}
		}
		seen[ref] = true
		results = append(results, result)
	}
	return results
}
