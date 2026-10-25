package workflow

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"canvasexpertse/internal/domain"
)

const inboxThreadID int64 = 9007199254740993

func inboxConversationRef(s *Service, id int64) string {
	return inboxRef(s.Client.Origin(), id, "conversation")
}

func inboxMessageRef(s *Service, id int64) string {
	return inboxRef(s.Client.Origin(), id, "message")
}

func inboxFixtureConversation(id int64, state, contextCode string, messageID int64) inboxConversationLive {
	return inboxConversationLive{
		ID:            id,
		Subject:       "Late work",
		ContextCode:   contextCode,
		Participants:  []inboxParticipant{{ID: 1}, {ID: 2}},
		MessageCount:  1,
		LastMessageAt: "2026-10-25T12:00:00Z",
		LastMessage:   "I submitted it late.",
		WorkflowState: state,
		Messages:      []inboxMessageLive{{ID: messageID, AuthorID: 2, CreatedAt: "2026-10-25T12:00:00Z", Body: "I submitted it late."}},
	}
}

func inboxFixtureHandler(t *testing.T, conversations []inboxConversationLive) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/conversations" && r.Method == http.MethodGet:
			page, _ := strconv.Atoi(r.URL.Query().Get("page"))
			if page == 2 {
				_ = json.NewEncoder(w).Encode(conversations[1:])
				return
			}
			if len(conversations) > 1 {
				w.Header().Set("Link", `</api/v1/conversations?scope=`+r.URL.Query().Get("scope")+`&per_page=100&page=2>; rel="next"`)
			}
			_ = json.NewEncoder(w).Encode(conversations[:1])
		case r.URL.Path == "/api/v1/users/self/profile":
			_, _ = w.Write([]byte(`{"id":1}`))
		case r.URL.Path == "/api/v1/courses/7/enrollments":
			_, _ = w.Write([]byte(`[{"user_id":2,"type":"StudentEnrollment","user":{"id":2,"name":"Private Student Name","short_name":"Student","login_id":"private-login","email":"private@example.edu"}},{"user_id":3,"type":"ParentEnrollment","user":{"id":3,"name":"Parent Name"}},{"user_id":4,"type":"ObserverEnrollment","user":{"id":4,"name":"Observer Name"}},{"user_id":5,"type":"TeacherEnrollment","user":{"id":5,"name":"Colleague Name"}}]`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
			http.NotFound(w, r)
		}
	}
}

func inboxFixtureService(t *testing.T, handler http.HandlerFunc) (*Service, *httptest.Server) {
	t.Helper()
	s, server := workflowFixture(t, handler)
	s.Config.Courses = append(s.Config.Courses, domain.Course{Nickname: "ELA7", ID: 7})
	return s, server
}

func TestInboxListPaginatesAndReturnsPrivateSafeReferences(t *testing.T) {
	conversations := []inboxConversationLive{
		inboxFixtureConversation(inboxThreadID, "unread", "course_7", 20),
		inboxFixtureConversation(inboxThreadID+1, "read", "course_7", 21),
	}
	conversations[0].Participants = append(conversations[0].Participants, inboxParticipant{ID: 3}, inboxParticipant{ID: 4}, inboxParticipant{ID: 5})
	s, _ := inboxFixtureService(t, inboxFixtureHandler(t, conversations))
	list, err := s.InboxList(context.Background(), InboxListOptions{Course: "ELA7", Unread: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Ref != inboxConversationRef(s, inboxThreadID) || list[0].Course != "ELA7" || list[0].Read || list[0].Archived {
		t.Fatalf("unexpected Inbox list: %+v", list)
	}
	encoded, err := json.Marshal(list)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	for _, forbidden := range []string{"9007199254740993", "Private Student Name", "private-login", "avatar_url", "https://"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("Inbox output exposed %q: %s", forbidden, text)
		}
	}
	if !strings.Contains(text, list[0].Participants[0]) {
		t.Fatalf("student alias missing from output: %s", text)
	}
	for _, label := range []string{"parent", "observer", "colleague"} {
		if !strings.Contains(text, label) {
			t.Fatalf("role label %q missing: %s", label, text)
		}
	}
}

func TestInboxReadDoesNotMarkReadAndReturnsAttachmentMetadataOnly(t *testing.T) {
	conversation := inboxFixtureConversation(inboxThreadID, "unread", "course_7", 20)
	conversation.Messages[0].Attachments = []inboxAttachmentLive{{DisplayName: "Draft.docx", ContentType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document", Size: 2048}}
	s, _ := inboxFixtureService(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/conversations":
			_ = json.NewEncoder(w).Encode([]inboxConversationLive{conversation})
		case strings.HasPrefix(r.URL.Path, "/api/v1/conversations/") && r.Method == http.MethodGet:
			if r.URL.Query().Get("auto_mark_as_read") != "false" {
				t.Errorf("read changed state by default: %s", r.URL.RawQuery)
			}
			_ = json.NewEncoder(w).Encode(conversation)
		case r.URL.Path == "/api/v1/users/self/profile":
			_, _ = w.Write([]byte(`{"id":1}`))
		case r.URL.Path == "/api/v1/courses/7/enrollments":
			_, _ = w.Write([]byte(`[{"user_id":2,"type":"StudentEnrollment","user":{"id":2,"name":"Private Student Name","short_name":"Student"}}]`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})
	thread, err := s.InboxRead(context.Background(), inboxConversationRef(s, inboxThreadID), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(thread.Messages) != 1 || thread.Messages[0].Ref != inboxMessageRef(s, 20) || thread.Messages[0].Author == "Private Student Name" {
		t.Fatalf("unexpected thread: %+v", thread)
	}
	if len(thread.Messages[0].Attachments) != 1 || thread.Messages[0].Attachments[0].Name != "Draft.docx" {
		t.Fatalf("attachment metadata missing: %+v", thread.Messages[0].Attachments)
	}
	encoded, _ := json.Marshal(thread)
	for _, forbidden := range []string{"9007199254740993", "url", "private-login", "Private Student Name"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("thread output exposed %q: %s", forbidden, encoded)
		}
	}
}

func TestInboxReplySendsOnlyBodyAndRejectsNewerMessage(t *testing.T) {
	conversation := inboxFixtureConversation(inboxThreadID, "unread", "", 20)
	replies := 0
	s, _ := inboxFixtureService(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/conversations":
			_ = json.NewEncoder(w).Encode([]inboxConversationLive{conversation})
		case strings.HasPrefix(r.URL.Path, "/api/v1/conversations/") && strings.HasSuffix(r.URL.Path, "/add_message"):
			replies++
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if len(body) != 1 || body["body"] != "Thanks for letting me know." {
				t.Fatalf("reply included fields beyond body: %#v", body)
			}
			_, _ = w.Write([]byte(`{"id":9876543210123}`))
		case strings.HasPrefix(r.URL.Path, "/api/v1/conversations/") && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(conversation)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})
	reply, err := s.InboxReply(context.Background(), inboxConversationRef(s, inboxThreadID), "Thanks for letting me know.", inboxMessageRef(s, 20))
	if err != nil || reply.MessageRef != inboxMessageRef(s, 9876543210123) || reply.Status != "completed" || replies != 1 {
		t.Fatalf("reply failed: result=%+v replies=%d err=%v", reply, replies, err)
	}
	conversation.Messages = append(conversation.Messages, inboxMessageLive{ID: 22, AuthorID: 1, Body: "New teacher message"})
	if _, err = s.InboxReply(context.Background(), inboxConversationRef(s, inboxThreadID), "Thanks.", inboxMessageRef(s, 20)); err == nil || !strings.Contains(err.Error(), "newer message") {
		t.Fatalf("newer message did not hold reply: %v", err)
	}
	if replies != 1 {
		t.Fatalf("stale reply was sent: %d writes", replies)
	}
}

func TestInboxSetStateContinuesAfterOneFailure(t *testing.T) {
	conversations := []inboxConversationLive{
		inboxFixtureConversation(inboxThreadID, "unread", "", 20),
		inboxFixtureConversation(inboxThreadID+1, "unread", "", 21),
	}
	writes := 0
	s, _ := inboxFixtureService(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/conversations" && r.Method == http.MethodGet {
			page, _ := strconv.Atoi(r.URL.Query().Get("page"))
			if page == 2 {
				_ = json.NewEncoder(w).Encode(conversations[1:])
			} else {
				w.Header().Set("Link", `</api/v1/conversations?scope=all&per_page=100&page=2>; rel="next"`)
				_ = json.NewEncoder(w).Encode(conversations[:1])
			}
			return
		}
		if r.Method == http.MethodPut {
			writes++
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["workflow_state"] != "archived" {
				t.Fatalf("unexpected state body: %#v", body)
			}
			if strings.Contains(r.URL.Path, strconv.FormatInt(inboxThreadID, 10)) {
				http.Error(w, `{"errors":["denied"]}`, http.StatusForbidden)
				return
			}
			_, _ = w.Write([]byte(`{}`))
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
	})
	results := s.InboxSetState(context.Background(), []string{inboxConversationRef(s, inboxThreadID), inboxConversationRef(s, inboxThreadID+1)}, "archived")
	if len(results) != 2 || results[0].Status != "failed" || results[1].Status != "completed" || writes != 2 {
		t.Fatalf("state updates did not continue independently: %+v writes=%d", results, writes)
	}
	encoded, _ := json.Marshal(results)
	if strings.Contains(string(encoded), strconv.FormatInt(inboxThreadID, 10)) {
		t.Fatalf("state results exposed Canvas IDs: %s", encoded)
	}
}
