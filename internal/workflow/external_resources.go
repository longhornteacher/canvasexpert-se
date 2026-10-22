package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

type ExternalResource struct {
	ID              int64    `json:"id"`
	Title           string   `json:"title,omitempty"`
	Type            string   `json:"type"`
	ContextType     string   `json:"context_type"`
	ContextID       int64    `json:"context_id"`
	URL             string   `json:"url,omitempty"`
	URLStatus       string   `json:"url_status"`
	ParticipantRefs []string `json:"participant_refs,omitempty"`
}

type ExternalResourcesReport struct {
	Course    string             `json:"course"`
	CourseID  int64              `json:"course_id"`
	GroupID   int64              `json:"group_id,omitempty"`
	Resources []ExternalResource `json:"resources"`
}

type canvasCollaboration struct {
	ID                int64   `json:"id"`
	Title             string  `json:"title"`
	Name              string  `json:"name"`
	Description       string  `json:"description"`
	CollaborationType string  `json:"collaboration_type"`
	ContextType       string  `json:"context_type"`
	ContextID         int64   `json:"context_id"`
	URL               *string `json:"url"`
}

type canvasConference struct {
	ID          int64             `json:"id"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	ContextType string            `json:"context_type"`
	ContextID   int64             `json:"context_id"`
	URL         *string           `json:"url"`
	JoinURL     *string           `json:"join_url"`
	Users       []json.RawMessage `json:"users"`
	Invitees    []json.RawMessage `json:"invitees"`
	Attendees   []json.RawMessage `json:"attendees"`
}

// ListExternalResources reads Canvas metadata only. It does not provision or
// open provider resources, and it exposes participants only through existing
// private stable references.
func (s *Service) ListExternalResources(ctx context.Context, nickname string, groupID int64, resourceType string) (ExternalResourcesReport, error) {
	course, err := s.Course(nickname)
	if err != nil {
		return ExternalResourcesReport{}, err
	}
	s, err = s.ForCourse(ctx, course)
	if err != nil {
		return ExternalResourcesReport{}, err
	}
	if groupID < 0 {
		return ExternalResourcesReport{}, fmt.Errorf("group ID cannot be negative")
	}
	if resourceType != "all" && resourceType != "collaborations" && resourceType != "conferences" {
		return ExternalResourcesReport{}, fmt.Errorf("resource_type must be collaborations, conferences, or all")
	}
	contextType, contextID := "Course", course.ID
	base := fmt.Sprintf("/api/v1/courses/%d", course.ID)
	if groupID > 0 {
		groups, e := s.ListGroups(ctx, course.Nickname, 0, groupID, false)
		if e != nil || len(groups) != 1 || groups[0].ID != groupID {
			return ExternalResourcesReport{}, fmt.Errorf("group %d does not belong to this course", groupID)
		}
		contextType, contextID = "Group", groupID
		base = fmt.Sprintf("/api/v1/groups/%d", groupID)
	}
	report := ExternalResourcesReport{Course: course.Nickname, CourseID: course.ID, GroupID: groupID, Resources: []ExternalResource{}}
	if resourceType == "all" || resourceType == "collaborations" {
		var rows []canvasCollaboration
		if err := s.Client.List(ctx, base+"/collaborations?per_page=100", &rows); err != nil {
			return report, err
		}
		for _, row := range rows {
			if row.ID <= 0 || (row.ContextType != "" && row.ContextType != contextType) || (row.ContextID != 0 && row.ContextID != contextID) {
				return report, fmt.Errorf("Canvas returned a collaboration outside the selected context")
			}
			title := row.Title
			if title == "" {
				title = row.Name
			}
			report.Resources = append(report.Resources, ExternalResource{ID: row.ID, Title: title, Type: nonempty(row.CollaborationType, "collaboration"), ContextType: contextType, ContextID: contextID, URL: usableResourceURL(row.URL), URLStatus: resourceURLStatus(row.URL)})
		}
	}
	if resourceType == "all" || resourceType == "conferences" {
		var rows []canvasConference
		if err := s.Client.ListField(ctx, base+"/conferences?per_page=100", "conferences", &rows); err != nil {
			return report, err
		}
		for _, row := range rows {
			if row.ID <= 0 || (row.ContextType != "" && row.ContextType != contextType) || (row.ContextID != 0 && row.ContextID != contextID) {
				return report, fmt.Errorf("Canvas returned a conference outside the selected context")
			}
			url := usableResourceURL(row.JoinURL)
			if url == "" {
				url = usableResourceURL(row.URL)
			}
			people := append(append(append([]json.RawMessage(nil), row.Users...), row.Invitees...), row.Attendees...)
			statusURL := ptrString(url)
			report.Resources = append(report.Resources, ExternalResource{ID: row.ID, Title: row.Title, Type: "conference", ContextType: contextType, ContextID: contextID, URL: url, URLStatus: resourceURLStatus(statusURL), ParticipantRefs: s.existingParticipantRefs(people)})
		}
	}
	return report, nil
}

func (s *Service) existingParticipantRefs(raw []json.RawMessage) []string {
	store, err := workspaceOpenStudents(s.Root)
	if err != nil {
		return nil
	}
	ids := map[int64]bool{}
	for _, item := range raw {
		idRaw := item
		var participant struct {
			ID json.RawMessage `json:"id"`
		}
		if len(item) > 0 && item[0] == '{' {
			if json.Unmarshal(item, &participant) != nil {
				continue
			}
			idRaw = participant.ID
		}
		var id int64
		if json.Unmarshal(idRaw, &id) != nil {
			var text string
			if json.Unmarshal(idRaw, &text) == nil {
				id, _ = strconv.ParseInt(text, 10, 64)
			}
		}
		if id > 0 {
			ids[id] = true
		}
	}
	refs := []string{}
	for _, student := range store.Rows {
		if student.Instance == s.Client.Origin() && ids[student.UserID] {
			refs = append(refs, student.Ref)
		}
	}
	return refs
}

func workspaceOpenStudents(root string) (*workspace.StudentStore, error) {
	return workspace.OpenStudents(root)
}

func usableResourceURL(value *string) string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return ""
	}
	value = ptrString(strings.TrimSpace(*value))
	if err := domain.ValidateModuleExternalURL(*value); err != nil {
		return ""
	}
	return *value
}

func resourceURLStatus(value *string) string {
	if usableResourceURL(value) == "" {
		return "unavailable; use a teacher-provided link"
	}
	return "Canvas supplied"
}

func nonempty(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func ptrString(value string) *string { return &value }
