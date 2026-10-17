package workflow

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

type canvasGroupCategory struct {
	ID               int64   `json:"id"`
	CourseID         int64   `json:"course_id"`
	ContextType      string  `json:"context_type"`
	Name             string  `json:"name"`
	NonCollaborative *bool   `json:"non_collaborative"`
	SelfSignup       *string `json:"self_signup"`
	GroupLimit       *int    `json:"group_limit"`
	AutoLeader       *string `json:"auto_leader"`
	GroupsCount      int     `json:"groups_count"`
	CreatedAt        string  `json:"created_at"`
}
type canvasGroup struct {
	ID               int64                `json:"id"`
	CourseID         int64                `json:"course_id"`
	GroupCategoryID  int64                `json:"group_category_id"`
	Name             string               `json:"name"`
	Description      string               `json:"description"`
	URL              string               `json:"html_url"`
	MembersCount     int                  `json:"members_count"`
	ContextType      string               `json:"context_type"`
	NonCollaborative *bool                `json:"non_collaborative"`
	Users            []domain.GroupMember `json:"users,omitempty"`
}

func (s *Service) groupCategory(ctx context.Context, courseID, setID int64) (canvasGroupCategory, error) {
	var c canvasGroupCategory
	err := s.Client.Get(ctx, fmt.Sprintf("/api/v1/group_categories/%d", setID), &c)
	if err == nil && (c.ID != setID || c.ContextType != "Course" || c.CourseID != courseID || nonCollaborative(c.NonCollaborative)) {
		return c, fmt.Errorf("group set %d does not belong to this course as a collaborative set", setID)
	}
	return c, err
}

func (s *Service) ListGroupSets(ctx context.Context, nickname string) ([]domain.GroupSetSummary, error) {
	course, err := s.Course(nickname)
	if err != nil {
		return nil, err
	}
	var cats []canvasGroupCategory
	if err = s.Client.List(ctx, fmt.Sprintf("/api/v1/courses/%d/group_categories?collaboration_state=collaborative&per_page=100", course.ID), &cats); err != nil {
		return nil, err
	}
	out := make([]domain.GroupSetSummary, 0, len(cats))
	for _, c := range cats {
		if c.ContextType != "Course" || c.CourseID != course.ID {
			continue
		}
		if nonCollaborative(c.NonCollaborative) || c.ID <= 0 {
			continue
		}
		var groups []canvasGroup
		if err = s.Client.List(ctx, fmt.Sprintf("/api/v1/group_categories/%d/groups?per_page=100", c.ID), &groups); err != nil {
			return nil, err
		}
		for _, g := range groups {
			if g.ContextType != "Course" || g.CourseID != course.ID || g.GroupCategoryID != c.ID || nonCollaborative(g.NonCollaborative) {
				return nil, fmt.Errorf("Canvas returned a group outside collaborative set %d", c.ID)
			}
		}
		out = append(out, domain.GroupSetSummary{ID: c.ID, CourseID: course.ID, Name: c.Name, Collaborative: true, SelfSignup: signup(c.SelfSignup), GroupLimit: c.GroupLimit, AutoLeader: leader(c.AutoLeader), GroupCount: len(groups)})
	}
	return out, nil
}

func (s *Service) ListGroups(ctx context.Context, nickname string, setID, groupID int64, includeMembers bool) ([]domain.GroupSummary, error) {
	course, err := s.Course(nickname)
	if err != nil {
		return nil, err
	}
	var cats []canvasGroupCategory
	var selected *canvasGroup
	if groupID > 0 {
		var g canvasGroup
		if err = s.Client.Get(ctx, fmt.Sprintf("/api/v1/groups/%d", groupID), &g); err != nil {
			return nil, err
		}
		if g.ID != groupID || g.ContextType != "Course" || g.CourseID != course.ID || g.GroupCategoryID <= 0 || nonCollaborative(g.NonCollaborative) {
			return nil, fmt.Errorf("group %d does not belong to this course", groupID)
		}
		cat, e := s.groupCategory(ctx, course.ID, g.GroupCategoryID)
		if e != nil {
			return nil, e
		}
		if setID > 0 && setID != cat.ID {
			return nil, fmt.Errorf("group %d does not belong to selected group set %d", groupID, setID)
		}
		cats = []canvasGroupCategory{cat}
		selected = &g
	} else if setID > 0 {
		c, e := s.groupCategory(ctx, course.ID, setID)
		if e != nil {
			return nil, e
		}
		cats = []canvasGroupCategory{c}
	} else if err = s.Client.List(ctx, fmt.Sprintf("/api/v1/courses/%d/group_categories?collaboration_state=collaborative&per_page=100", course.ID), &cats); err != nil {
		return nil, err
	}
	out := []domain.GroupSummary{}
	for _, cat := range cats {
		if cat.ContextType != "Course" || cat.CourseID != course.ID || nonCollaborative(cat.NonCollaborative) || cat.ID <= 0 {
			continue
		}
		path := fmt.Sprintf("/api/v1/group_categories/%d/groups?per_page=100", cat.ID)
		var groups []canvasGroup
		if selected != nil {
			groups = []canvasGroup{*selected}
		} else if err = s.Client.List(ctx, path, &groups); err != nil {
			return nil, err
		}
		for _, g := range groups {
			if g.ContextType != "Course" || g.CourseID != course.ID || g.GroupCategoryID != cat.ID || nonCollaborative(g.NonCollaborative) {
				continue
			}
			summary := domain.GroupSummary{ID: g.ID, CourseID: course.ID, GroupCategoryID: cat.ID, Name: g.Name, Description: g.Description, Collaborative: true, MembersCount: g.MembersCount}
			if includeMembers {
				var full canvasGroup
				if err = s.Client.Get(ctx, fmt.Sprintf("/api/v1/groups/%d?include[]=users", g.ID), &full); err != nil {
					return nil, err
				}
				if full.ID != g.ID || full.ContextType != "Course" || full.CourseID != course.ID || full.GroupCategoryID != cat.ID || nonCollaborative(full.NonCollaborative) {
					return nil, fmt.Errorf("group %d changed identity during member read", g.ID)
				}
				summary.Members = full.Users
				if summary.MembersCount == 0 {
					summary.MembersCount = len(full.Users)
				}
			}
			out = append(out, summary)
		}
	}
	return out, nil
}

func signup(v *string) string {
	if v == nil || *v == "" {
		return "disabled"
	}
	return *v
}
func leader(v *string) string {
	if v == nil || *v == "" {
		return "none"
	}
	return *v
}
func nonCollaborative(v *bool) bool { return v != nil && *v }

func (s *Service) PullGroupSet(ctx context.Context, nickname string, setID int64) (string, error) {
	course, err := s.Course(nickname)
	if err != nil {
		return "", err
	}
	if setID <= 0 {
		return "", fmt.Errorf("group-set ID must be positive")
	}
	set, err := s.groupCategory(ctx, course.ID, setID)
	if err != nil {
		return "", err
	}
	var groups []canvasGroup
	if err = s.Client.List(ctx, fmt.Sprintf("/api/v1/group_categories/%d/groups?per_page=100", setID), &groups); err != nil {
		return "", err
	}
	doc := domain.GroupSetDocument{Schema: 1, Kind: "group-set", Operation: "update", Course: course.Nickname, Instance: s.Client.Origin(), CourseID: course.ID, GroupCategoryID: setID, Name: set.Name, SelfSignup: domain.OptionalString{Set: true, Value: signup(set.SelfSignup)}, GroupLimit: domain.OptionalInt{Set: true, Null: set.GroupLimit == nil}, AutoLeader: domain.OptionalString{Set: true, Value: leader(set.AutoLeader)}}
	if set.GroupLimit != nil {
		doc.GroupLimit.Value = *set.GroupLimit
	}
	baseline := &domain.GroupSetSnapshot{Instance: s.Client.Origin(), CourseID: course.ID, GroupCategoryID: setID, Name: set.Name, SelfSignup: signup(set.SelfSignup), GroupLimit: set.GroupLimit, AutoLeader: leader(set.AutoLeader)}
	doc.Observed = baseline
	for _, g := range groups {
		if g.ContextType != "Course" || g.CourseID != course.ID || g.GroupCategoryID != setID || nonCollaborative(g.NonCollaborative) {
			return "", fmt.Errorf("Canvas returned a group outside group set %d", setID)
		}
		doc.Groups = append(doc.Groups, domain.GroupSetRow{GroupID: g.ID, Name: g.Name, Observed: &domain.GroupSnapshot{GroupID: g.ID, Name: g.Name, Description: g.Description}})
	}
	name := fmt.Sprintf("group-set-%d-%s.md", setID, time.Now().UTC().Format("20060102T150405.000000000"))
	path, err := workspace.ConstrainPath(s.Root, filepath.Join("courses", course.Nickname, "pulled", name))
	if err != nil {
		return "", err
	}
	if err = workspace.WriteDocument(path, doc, ""); err != nil {
		return "", err
	}
	return path, nil
}

type GroupSetReport struct {
	Path, Status, Message string
	GroupCategoryID       int64
	Groups                []domain.GroupCreateOutcome
}

func (s *Service) PushGroupSet(ctx context.Context, path string) (report GroupSetReport, err error) {
	report.Path = path
	var doc domain.GroupSetDocument
	if _, err = workspace.ReadDocument(path, &doc); err != nil {
		return report, err
	}
	if err = doc.Validate(); err != nil {
		return report, err
	}
	course, err := s.Course(doc.Course)
	if err != nil {
		return report, err
	}
	if (doc.Instance != "" && doc.Instance != s.Client.Origin()) || (doc.CourseID != 0 && doc.CourseID != course.ID) {
		return report, fmt.Errorf("group-set document belongs to a different Canvas instance or course")
	}
	doc.Course = course.Nickname
	save := func() error { return workspace.WriteDocument(path, &doc, "") }
	if doc.Created == nil {
		doc.Created = &domain.GroupSetCreated{Started: true}
		doc.Instance = s.Client.Origin()
		doc.CourseID = course.ID
		if err = save(); err != nil {
			return report, err
		}
	}
	c := doc.Created
	createdSetThisRun := false
	var set canvasGroupCategory
	if doc.Operation == "create" {
		if c.GroupCategoryID == 0 {
			if c.Set.Status == "pending" || c.Set.Status == "uncertain" {
				report.Status = "uncertain"
				report.Message = "group-set creation outcome is unclear; the existing draft was preserved"
				return report, nil
			}
			c.Set = domain.ActionOutcome{Status: "pending"}
			if err = save(); err != nil {
				return report, err
			}
			fields := map[string]any{"name": doc.Name, "non_collaborative": false, "self_signup": apiSignup(doc.SelfSignup, "disabled"), "group_limit": apiInt(doc.GroupLimit), "auto_leader": apiLeader(doc.AutoLeader, "none")}
			var created canvasGroupCategory
			err = s.Client.Write(ctx, "POST", fmt.Sprintf("/api/v1/courses/%d/group_categories", course.ID), map[string]any{"group_category": fields}, &created)
			if err != nil {
				c.Set = failedOutcome(err)
				_ = save()
				report.Status = c.Set.Status
				report.Message = err.Error()
				return report, nil
			}
			if created.ID <= 0 {
				c.Set = domain.ActionOutcome{Status: "uncertain", Message: "Canvas accepted group-set creation but returned no exact ID"}
				if e := save(); e != nil {
					return report, e
				}
				report.Status = "uncertain"
				report.Message = c.Set.Message
				return report, nil
			}
			if (created.ContextType != "" && created.ContextType != "Course") || (created.CourseID != 0 && created.CourseID != course.ID) || nonCollaborative(created.NonCollaborative) {
				return report, fmt.Errorf("Canvas returned a group set in a different course")
			}
			c.GroupCategoryID = created.ID
			set = created
			createdSetThisRun = true
			c.Set = domain.ActionOutcome{Status: "succeeded", ID: created.ID}
			doc.GroupCategoryID = created.ID
			if err = save(); err != nil {
				return report, fmt.Errorf("group set created but its ID could not be saved: %w", err)
			}
		}
	}
	if c.GroupCategoryID == 0 {
		c.GroupCategoryID = doc.GroupCategoryID
	}
	if c.GroupCategoryID <= 0 {
		return report, fmt.Errorf("exact group-set ID is unavailable")
	}
	if !createdSetThisRun {
		set, err = s.groupCategory(ctx, course.ID, c.GroupCategoryID)
		if err != nil {
			return report, err
		}
	}
	if doc.GroupCategoryID > 0 && doc.GroupCategoryID != set.ID {
		return report, fmt.Errorf("saved group-set identity changed")
	}
	doc.GroupCategoryID = set.ID
	report.GroupCategoryID = set.ID
	if doc.Operation == "update" {
		b := doc.Observed
		changed := false
		if strings.TrimSpace(doc.Name) != "" && doc.Name != b.Name {
			if set.Name != b.Name {
				report.Status = "held"
				report.Message = "group-set name changed in Canvas; the draft and baseline were preserved"
				return report, nil
			}
			changed = true
		}
		if doc.SelfSignup.Set && requestedSignup(doc.SelfSignup, b.SelfSignup) != b.SelfSignup {
			if signup(set.SelfSignup) != b.SelfSignup {
				report.Status = "held"
				report.Message = "self sign-up changed in Canvas; the draft and baseline were preserved"
				return report, nil
			}
			changed = true
		}
		if doc.GroupLimit.Set && !sameOptionalLimit(doc.GroupLimit, b.GroupLimit) {
			if !sameLimit(b.GroupLimit, set.GroupLimit) {
				report.Status = "held"
				report.Message = "group limit changed in Canvas; the draft and baseline were preserved"
				return report, nil
			}
			changed = true
		}
		if doc.AutoLeader.Set && requestedLeader(doc.AutoLeader, b.AutoLeader) != b.AutoLeader {
			if leader(set.AutoLeader) != b.AutoLeader {
				report.Status = "held"
				report.Message = "automatic leader setting changed in Canvas; the draft and baseline were preserved"
				return report, nil
			}
			changed = true
		}
		if changed {
			fields := map[string]any{}
			if strings.TrimSpace(doc.Name) != "" {
				fields["name"] = doc.Name
			}
			if doc.SelfSignup.Set {
				fields["self_signup"] = apiSignup(doc.SelfSignup, b.SelfSignup)
			}
			if doc.GroupLimit.Set {
				fields["group_limit"] = apiInt(doc.GroupLimit)
			}
			if doc.AutoLeader.Set {
				fields["auto_leader"] = apiLeader(doc.AutoLeader, b.AutoLeader)
			}
			if err = s.Client.Write(ctx, "PUT", fmt.Sprintf("/api/v1/group_categories/%d", set.ID), map[string]any{"group_category": fields}, nil); err != nil {
				report.Status = failedOutcome(err).Status
				report.Message = err.Error()
				return report, nil
			}
			doc.UpdateOutcome = domain.ActionOutcome{Status: "succeeded", ID: set.ID}
			if err = save(); err != nil {
				return report, err
			}
		}
	}
	for _, row := range doc.Groups {
		if row.GroupID > 0 {
			var live canvasGroup
			e := s.Client.Get(ctx, fmt.Sprintf("/api/v1/groups/%d", row.GroupID), &live)
			if e != nil {
				return report, e
			}
			if live.ID != row.GroupID || live.ContextType != "Course" || live.CourseID != course.ID || live.GroupCategoryID != set.ID || nonCollaborative(live.NonCollaborative) {
				return report, fmt.Errorf("group %d does not belong to this course and set", row.GroupID)
			}
			if row.Name != "" && live.Name != row.Observed.Name && live.Name != row.Name {
				report.Status = "held"
				report.Message = fmt.Sprintf("club %d changed in Canvas; the draft and baseline were preserved", row.GroupID)
				return report, nil
			}
			patch := map[string]any{}
			if row.Name != "" && live.Name == row.Observed.Name && row.Name != row.Observed.Name {
				patch["name"] = row.Name
			}
			if row.Description != nil && *row.Description != row.Observed.Description {
				if live.Description != row.Observed.Description && live.Description != *row.Description {
					report.Status = "held"
					report.Message = fmt.Sprintf("club %d description changed in Canvas; the draft and baseline were preserved", row.GroupID)
					return report, nil
				}
				if live.Description == row.Observed.Description {
					patch["description"] = *row.Description
				}
			}
			if len(patch) > 0 {
				if err = s.Client.Write(ctx, "PUT", fmt.Sprintf("/api/v1/groups/%d", row.GroupID), map[string]any{"group": patch}, nil); err != nil {
					report.Status = failedOutcome(err).Status
					report.Message = err.Error()
					return report, nil
				}
			}
			continue
		}
		found := false
		complete := false
		for _, saved := range c.Groups {
			if saved.Key == row.Key {
				found = true
				if saved.GroupID > 0 {
					var current canvasGroup
					e := s.Client.Get(ctx, fmt.Sprintf("/api/v1/groups/%d", saved.GroupID), &current)
					if e != nil {
						return report, e
					}
					if current.ID != saved.GroupID || current.ContextType != "Course" || current.CourseID != course.ID || current.GroupCategoryID != set.ID || nonCollaborative(current.NonCollaborative) {
						return report, fmt.Errorf("saved club %q no longer belongs to this course and set", row.Key)
					}
					complete = true
					break
				}
				if saved.Outcome.Status == "pending" || saved.Outcome.Status == "uncertain" {
					return report, fmt.Errorf("club %q creation outcome is unclear; do not replay it", row.Key)
				}
			}
		}
		if complete {
			continue
		}
		if found {
			for i := range c.Groups {
				if c.Groups[i].Key == row.Key && c.Groups[i].GroupID > 0 {
					continue
				}
			}
		}
		idx := -1
		for i := range c.Groups {
			if c.Groups[i].Key == row.Key {
				idx = i
				break
			}
		}
		if idx < 0 {
			c.Groups = append(c.Groups, domain.GroupCreateOutcome{Key: row.Key})
			idx = len(c.Groups) - 1
		}
		// This identity read is immediately before a distinct additive group
		// action. The accepted response ID is saved without a confirmation GET.
		if _, e := s.groupCategory(ctx, course.ID, set.ID); e != nil {
			return report, e
		}
		c.Groups[idx].Outcome = domain.ActionOutcome{Status: "pending"}
		if err = save(); err != nil {
			return report, err
		}
		fields := map[string]any{"name": row.Name}
		if row.Description != nil {
			fields["description"] = *row.Description
		}
		var created canvasGroup
		err = s.Client.Write(ctx, "POST", fmt.Sprintf("/api/v1/group_categories/%d/groups", set.ID), map[string]any{"group": fields}, &created)
		if err != nil {
			c.Groups[idx].Outcome = failedOutcome(err)
			_ = save()
			report.Status = c.Groups[idx].Outcome.Status
			report.Message = err.Error()
			return report, nil
		}
		if created.ID <= 0 {
			c.Groups[idx].Outcome = domain.ActionOutcome{Status: "uncertain", Message: "Canvas accepted club creation but returned no exact ID"}
			if e := save(); e != nil {
				return report, e
			}
			report.Status = "uncertain"
			report.Message = c.Groups[idx].Outcome.Message
			return report, nil
		}
		if (created.ContextType != "" && created.ContextType != "Course") || created.CourseID != course.ID || created.GroupCategoryID != set.ID || nonCollaborative(created.NonCollaborative) {
			return report, fmt.Errorf("Canvas returned a club outside the selected course and set")
		}
		c.Groups[idx].GroupID = created.ID
		c.Groups[idx].URL = created.URL
		c.Groups[idx].Outcome = domain.ActionOutcome{Status: "succeeded", ID: created.ID}
		if err = save(); err != nil {
			return report, fmt.Errorf("club created but its exact ID could not be saved: %w", err)
		}
	}
	report.Groups = c.Groups
	report.Status = "completed"
	return report, nil
}

func groupOptionalString(o domain.OptionalString, fallback string) string {
	if !o.Set {
		return fallback
	}
	if o.Null {
		return ""
	}
	return o.Value
}
func requestedSignup(o domain.OptionalString, fallback string) string {
	value := groupOptionalString(o, fallback)
	if value == "" {
		return "disabled"
	}
	return value
}
func requestedLeader(o domain.OptionalString, fallback string) string {
	value := groupOptionalString(o, fallback)
	if value == "" {
		return "none"
	}
	return value
}
func apiSignup(o domain.OptionalString, fallback string) any {
	if !o.Set {
		if fallback == "disabled" {
			return nil
		}
		return fallback
	}
	if o.Null || o.Value == "disabled" {
		return nil
	}
	return o.Value
}
func apiLeader(o domain.OptionalString, fallback string) any {
	if !o.Set {
		o.Value = fallback
	}
	if o.Null || o.Value == "none" {
		return nil
	}
	return o.Value
}
func apiInt(o domain.OptionalInt) any {
	if !o.Set || o.Null {
		return nil
	}
	return o.Value
}
func sameOptionalLimit(o domain.OptionalInt, b *int) bool {
	if o.Null {
		return b == nil
	}
	return b != nil && *b == o.Value
}
func sameLimit(a, b *int) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}
