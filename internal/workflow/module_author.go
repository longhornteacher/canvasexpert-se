package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

type ModulePushReport struct {
	Status      string
	Course      string
	CourseID    int64
	ModuleID    int64
	Title       string
	ItemsAdded  int
	ItemsTotal  int
	Published   bool
	ChildAccess string
	Items       []domain.ModuleItemOutcome
	Message     string
}

type canvasModule struct {
	ID        int64  `json:"id"`
	CourseID  int64  `json:"course_id"`
	Name      string `json:"name"`
	Position  int    `json:"position"`
	Published bool   `json:"published"`
}

type canvasPage struct {
	PageID    int64  `json:"page_id"`
	URL       string `json:"url"`
	Title     string `json:"title"`
	Published *bool  `json:"published"`
	Locked    *bool  `json:"locked_for_user"`
}

type canvasModuleAssignment struct {
	ID        int64   `json:"id"`
	CourseID  int64   `json:"course_id"`
	Published *bool   `json:"published"`
	UnlockAt  *string `json:"unlock_at"`
	LockAt    *string `json:"lock_at"`
}

type canvasModuleItem struct {
	ID          int64  `json:"id"`
	Position    int    `json:"position"`
	Type        string `json:"type"`
	ContentID   any    `json:"content_id"`
	PageURL     string `json:"page_url"`
	ExternalURL string `json:"external_url"`
	NewTab      *bool  `json:"new_tab"`
	Title       string `json:"title"`
}

// PushModule adds native Canvas module items from an authoring document. A
// module and every item ID are saved as soon as Canvas returns them so reruns
// can verify identity and continue without replaying an uncertain write.
func (s *Service) PushModule(ctx context.Context, path string) (report ModulePushReport, err error) {
	var d domain.ModuleDocument
	body, err := workspace.ReadDocument(path, &d)
	if err != nil {
		return ModulePushReport{}, err
	}
	if d.Kind != "module" {
		return ModulePushReport{}, fmt.Errorf("expected kind: module")
	}
	if err := d.Validate(); err != nil {
		return ModulePushReport{}, err
	}
	course, err := s.Course(d.Course)
	if err != nil {
		return ModulePushReport{}, err
	}
	report = ModulePushReport{Status: "partial", Course: course.Nickname, CourseID: course.ID, Title: d.Title, ItemsTotal: len(d.Items)}
	if d.Instance != "" && d.Instance != s.Client.Origin() {
		return report, fmt.Errorf("module document belongs to a different Canvas instance")
	}
	if d.CourseID != 0 && d.CourseID != course.ID {
		return report, fmt.Errorf("module document course ID does not match configured course %s", course.Nickname)
	}
	if d.Created != nil {
		if d.Created.ModuleID > 0 && d.ModuleID > 0 && d.Created.ModuleID != d.ModuleID {
			return report, fmt.Errorf("module_id conflicts with saved created.module_id; restore the matching ID before resuming")
		}
		started := d.Created.Module.Status != "" || d.Created.ModuleID > 0 || len(d.Created.Items) > 0 || d.Created.Publication.Status != ""
		if started && (d.Instance == "" || d.CourseID == 0) {
			return report, fmt.Errorf("saved module work lacks its Canvas instance or course ID; select the exact live module before resuming")
		}
	}
	if d.Created == nil {
		d.Created = &domain.ModuleCreated{Items: make([]domain.ModuleItemOutcome, 0, len(d.Items))}
	}
	defer func() { report.Items = append([]domain.ModuleItemOutcome(nil), d.Created.Items...) }()
	for i := range d.Items {
		if i < len(d.Created.Items) && d.Created.Items[i].Index != i {
			return report, fmt.Errorf("saved module item outcomes do not match the document order; restore the original items or create a fresh document")
		}
	}
	resolvedRefs := make([]string, len(d.Items))
	accessStates := make([]string, len(d.Items))
	for i, item := range d.Items {
		resolved, access, verifyErr := s.verifyModuleReference(ctx, course.ID, item)
		if verifyErr != nil {
			return report, fmt.Errorf("items[%d]: %w", i, verifyErr)
		}
		resolvedRefs[i] = resolved
		accessStates[i] = access
	}
	d.Instance, d.CourseID = s.Client.Origin(), course.ID
	if err = workspace.WriteDocument(path, d, body); err != nil {
		return report, err
	}
	save := func() error { return workspace.WriteDocument(path, d, body) }

	moduleID := d.ModuleID
	if moduleID == 0 && d.Created.ModuleID > 0 {
		moduleID = d.Created.ModuleID
	}
	if moduleID == 0 {
		if d.Created.Module.Status == "pending" || d.Created.Module.Status == "uncertain" {
			// An unclear create is resolved by looking for the module by exact name.
			if id := s.findModuleByName(ctx, course.ID, d.Title); id > 0 {
				moduleID, d.ModuleID, d.Created.ModuleID = id, id, id
				d.Created.Module = domain.ActionOutcome{Status: "succeeded", ID: id, Message: "adopted the existing module with this name"}
				if err = save(); err != nil {
					return report, err
				}
			}
		}
		if moduleID == 0 {
			d.Created.StartedAt = time.Now().UTC().Format(time.RFC3339Nano)
			d.Created.Module = domain.ActionOutcome{Status: "pending"}
			if err = save(); err != nil {
				return report, err
			}
			var created canvasModule
			err = s.Client.Write(ctx, "POST", fmt.Sprintf("/api/v1/courses/%d/modules", course.ID), map[string]any{"module": map[string]any{"name": d.Title}}, &created)
			if err != nil {
				d.Created.Module = failedOutcome(err)
			} else if created.ID <= 0 {
				d.Created.Module = domain.ActionOutcome{Status: "uncertain", Message: "Canvas response lacks the created module ID"}
			} else {
				moduleID, d.ModuleID, d.Created.ModuleID = created.ID, created.ID, created.ID
				if created.Name != d.Title || created.Published {
					d.Created.Module = domain.ActionOutcome{Status: "uncertain", ID: created.ID, Message: "Canvas returned an ID but did not confirm the exact unpublished module"}
				} else {
					d.Created.Module = domain.ActionOutcome{Status: "succeeded", ID: created.ID}
				}
			}
			if err = save(); err != nil {
				return report, fmt.Errorf("module create outcome could not be saved: %w", err)
			}
			if !definite(d.Created.Module) && moduleID == 0 {
				report.Status, report.Message = d.Created.Module.Status, d.Created.Module.Message
				return report, nil
			}
		}
	}

	module, err := s.getModule(ctx, course.ID, moduleID)
	if err != nil {
		return report, fmt.Errorf("verify module %d in course %s: %w", moduleID, course.Nickname, err)
	}
	if module.ID != moduleID || (module.CourseID != 0 && module.CourseID != course.ID) {
		return report, fmt.Errorf("module %d is not verified in configured course %s", moduleID, course.Nickname)
	}
	if d.Created.StartedAt != "" && !definite(d.Created.Module) {
		d.Created.Module = domain.ActionOutcome{Status: "succeeded", ID: moduleID, Message: "module verified by its saved Canvas ID"}
		if err = save(); err != nil {
			return report, err
		}
	}
	if d.Created.Module.Status == "" {
		d.Created.Module = domain.ActionOutcome{Status: "preserved", ID: moduleID}
		d.Created.ModuleID = moduleID
	}
	d.ModuleID = moduleID
	report.ModuleID, report.Published = moduleID, module.Published
	if err = save(); err != nil {
		return report, err
	}

	for i, item := range d.Items {
		var saved *domain.ModuleItemOutcome
		if i < len(d.Created.Items) {
			saved = &d.Created.Items[i]
		} else {
			d.Created.Items = append(d.Created.Items, domain.ModuleItemOutcome{Index: i, Type: item.Type, ContentID: item.ContentID, ExternalURL: item.ExternalURL, NewTab: item.NewTab, Title: item.Title, Position: item.Position})
			saved = &d.Created.Items[i]
			if err = save(); err != nil {
				return report, err
			}
		}
		if saved.Index != i || saved.Type != item.Type || saved.ContentID != item.ContentID || saved.ExternalURL != item.ExternalURL || !sameBoolPointer(saved.NewTab, item.NewTab) || saved.Title != item.Title || saved.Position != item.Position {
			return report, fmt.Errorf("items[%d] changed after a prior push; create a new module document to preserve the recorded item identities", i)
		}
		if saved.Access != accessStates[i] {
			saved.Access = accessStates[i]
			if err = save(); err != nil {
				return report, err
			}
		}
		if saved.ModuleItemID > 0 {
			live, getErr := s.getModuleItem(ctx, course.ID, moduleID, saved.ModuleItemID)
			if getErr != nil || !moduleItemMatches(live, item, resolvedRefs[i]) {
				saved.Outcome = domain.ActionOutcome{Status: "preserved", ID: saved.ModuleItemID, Message: "module item was changed or removed in Canvas; left as it is now"}
				if err = save(); err != nil {
					return report, err
				}
				continue
			}
			if !definite(saved.Outcome) {
				saved.Outcome = domain.ActionOutcome{Status: "succeeded", ID: saved.ModuleItemID, Message: "placement verified by its saved Canvas ID"}
				if err = save(); err != nil {
					return report, err
				}
			}
			continue
		}
		if definite(saved.Outcome) {
			continue
		}
		if saved.Outcome.Status == "pending" || saved.Outcome.Status == "uncertain" {
			// An unclear placement is resolved by looking for the item in the module.
			var existing []canvasModuleItem
			if e := s.Client.List(ctx, fmt.Sprintf("/api/v1/courses/%d/modules/%d/items?per_page=100", course.ID, moduleID), &existing); e == nil {
				for _, candidate := range existing {
					if moduleItemMatches(candidate, item, resolvedRefs[i]) {
						saved.ModuleItemID = candidate.ID
						saved.Outcome = domain.ActionOutcome{Status: "succeeded", ID: candidate.ID, Message: "adopted the existing module item"}
						break
					}
				}
			}
			if saved.ModuleItemID > 0 {
				if err = save(); err != nil {
					return report, err
				}
				continue
			}
		}
		saved.Outcome = domain.ActionOutcome{Status: "pending"}
		if err = save(); err != nil {
			return report, err
		}
		payload := map[string]any{"type": item.Type}
		if item.Type == "SubHeader" || item.Type == "ExternalUrl" {
			payload["title"] = item.Title
			if item.Type == "ExternalUrl" {
				payload["external_url"] = item.ExternalURL
				if item.NewTab != nil {
					payload["new_tab"] = *item.NewTab
				}
			}
		} else if item.Type == "Page" {
			payload["page_url"] = resolvedRefs[i]
		} else {
			payload["content_id"] = item.ContentID
		}
		if item.Position > 0 {
			payload["position"] = item.Position
		}
		var created canvasModuleItem
		err = s.Client.Write(ctx, "POST", fmt.Sprintf("/api/v1/courses/%d/modules/%d/items", course.ID, moduleID), map[string]any{"module_item": payload}, &created)
		if err != nil {
			saved.Outcome = failedOutcome(err)
		} else if created.ID <= 0 {
			saved.Outcome = domain.ActionOutcome{Status: "uncertain", Message: "Canvas response lacks the created module item ID"}
		} else {
			saved.ModuleItemID = created.ID
			if !moduleItemMatches(created, item, resolvedRefs[i]) {
				saved.Outcome = domain.ActionOutcome{Status: "uncertain", ID: created.ID, Message: "Canvas returned an ID but did not confirm the requested module item"}
			} else {
				saved.Outcome = domain.ActionOutcome{Status: "succeeded", ID: created.ID}
				report.ItemsAdded++
			}
		}
		if err = save(); err != nil {
			return report, fmt.Errorf("module item outcome could not be saved: %w", err)
		}
		if !definite(saved.Outcome) {
			report.Status, report.Message = saved.Outcome.Status, saved.Outcome.Message
			report.Items = append([]domain.ModuleItemOutcome(nil), d.Created.Items...)
			return report, nil
		}
	}

	if d.Published != nil && *d.Published {
		if module.Published && (d.Created.Publication.Status == "pending" || d.Created.Publication.Status == "uncertain") {
			d.Created.Publication = domain.ActionOutcome{Status: "succeeded", Message: "publication recovered from live module"}
		} else if module.Published {
			d.Created.Publication = domain.ActionOutcome{Status: "preserved", Message: "module already published"}
		} else if d.Created.Publication.Status == "succeeded" || d.Created.Publication.Status == "preserved" {
			d.Created.Publication = domain.ActionOutcome{Status: "preserved", Message: "module was unpublished in Canvas after it was published; left as it is now"}
		} else {
			d.Created.Publication = domain.ActionOutcome{Status: "pending"}
			if err = save(); err != nil {
				return report, err
			}
			var updated canvasModule
			err = s.Client.Write(ctx, "PUT", fmt.Sprintf("/api/v1/courses/%d/modules/%d", course.ID, moduleID), map[string]any{"module": map[string]any{"published": true}}, &updated)
			if err != nil {
				d.Created.Publication = failedOutcome(err)
			} else if updated.ID != moduleID || !updated.Published {
				d.Created.Publication = domain.ActionOutcome{Status: "uncertain", Message: "publication response did not confirm published module"}
			} else {
				d.Created.Publication = domain.ActionOutcome{Status: "succeeded"}
				module = updated
			}
		}
		if err = save(); err != nil {
			return report, fmt.Errorf("module publication outcome could not be saved: %w", err)
		}
		if !definite(d.Created.Publication) {
			report.Status, report.Message = d.Created.Publication.Status, d.Created.Publication.Message
			return report, nil
		}
	}
	if !definite(d.Created.Publication) {
		d.Created.Publication = domain.ActionOutcome{Status: "not-requested"}
	}
	if err = save(); err != nil {
		return report, err
	}
	report.Published = module.Published
	if module.Published {
		report.ChildAccess = "module published; child states below reflect Canvas settings, not student-session visibility verification"
	} else {
		report.ChildAccess = "module unpublished; child states below reflect Canvas settings, not student-session visibility verification"
	}
	report.Items = append([]domain.ModuleItemOutcome(nil), d.Created.Items...)
	report.Status = "completed"
	return report, nil
}

// findModuleByName returns the ID of the one module in the course with this
// exact name, or zero, so an unclear create can be adopted instead of repeated.
func (s *Service) findModuleByName(ctx context.Context, courseID int64, name string) int64 {
	var modules []canvasModule
	if e := s.Client.List(ctx, fmt.Sprintf("/api/v1/courses/%d/modules?per_page=100", courseID), &modules); e != nil {
		return 0
	}
	var id int64
	n := 0
	for _, m := range modules {
		if m.Name == name && m.ID > 0 {
			id, n = m.ID, n+1
		}
	}
	if n == 1 {
		return id
	}
	return 0
}

func (s *Service) getModule(ctx context.Context, courseID, moduleID int64) (canvasModule, error) {
	var module canvasModule
	err := s.Client.Get(ctx, fmt.Sprintf("/api/v1/courses/%d/modules/%d", courseID, moduleID), &module)
	if err != nil {
		return module, err
	}
	if module.ID != moduleID {
		return module, fmt.Errorf("Canvas returned module %d for requested module %d", module.ID, moduleID)
	}
	if module.CourseID == 0 {
		module.CourseID = courseID
	}
	return module, nil
}

func (s *Service) verifyModuleReference(ctx context.Context, courseID int64, item domain.ModuleItem) (string, string, error) {
	switch item.Type {
	case "Page":
		var page canvasPage
		if err := s.Client.Get(ctx, fmt.Sprintf("/api/v1/courses/%d/pages/page_id:%d", courseID, item.ContentID), &page); err != nil {
			return "", "", err
		}
		if page.PageID != item.ContentID || strings.TrimSpace(page.URL) == "" {
			return "", "", fmt.Errorf("page ID %d is not verified in this course", item.ContentID)
		}
		return page.URL, fmt.Sprintf("published=%s, locked_for_user=%s", stateBool(page.Published), stateBool(page.Locked)), nil
	case "File":
		file, err := s.readCourseAsset(ctx, courseID, item.ContentID)
		if err != nil {
			return "", "", err
		}
		return strconv.FormatInt(item.ContentID, 10), fmt.Sprintf("hidden=%s, locked=%s, visibility=%s, unlock_at=%s, lock_at=%s", stateBool(file.Hidden), stateBool(file.Locked), file.VisibilityLevel, optionalString(&file.UnlockAt), optionalString(&file.LockAt)), nil
	case "Assignment":
		var a canvasModuleAssignment
		err := s.Client.Get(ctx, fmt.Sprintf("/api/v1/courses/%d/assignments/%d", courseID, item.ContentID), &a)
		if err != nil {
			return "", "", err
		}
		if a.ID != item.ContentID || (a.CourseID != 0 && a.CourseID != courseID) {
			return "", "", fmt.Errorf("assignment ID %d is not verified in this course", item.ContentID)
		}
		return strconv.FormatInt(item.ContentID, 10), fmt.Sprintf("published=%s, unlock_at=%s, lock_at=%s", stateBool(a.Published), optionalString(a.UnlockAt), optionalString(a.LockAt)), nil
	case "Discussion":
		var topic NativeDiscussion
		err := s.Client.Get(ctx, discussionPath(courseID, 0, item.ContentID), &topic)
		if err != nil {
			return "", "", err
		}
		if topic.ID != item.ContentID {
			return "", "", fmt.Errorf("discussion ID %d is not verified in this course", item.ContentID)
		}
		return strconv.FormatInt(item.ContentID, 10), fmt.Sprintf("published=%t", topic.Published), nil
	case "ExternalUrl":
		if err := s.verifySameCourseGroupURL(ctx, courseID, item.ExternalURL); err != nil {
			return "", "", err
		}
		return item.ExternalURL, "teacher-selected link; provider access is not verified", nil
	case "SubHeader":
		return "", "", nil
	default:
		return "", "", fmt.Errorf("unsupported item type %q", item.Type)
	}
}

func (s *Service) verifySameCourseGroupURL(ctx context.Context, courseID int64, raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	origin, err := url.Parse(s.Client.Origin())
	if err != nil || !strings.EqualFold(u.Scheme, origin.Scheme) || !strings.EqualFold(u.Host, origin.Host) {
		return nil
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i := 0; i+1 < len(parts); i++ {
		if parts[i] != "groups" {
			continue
		}
		id, parseErr := strconv.ParseInt(parts[i+1], 10, 64)
		if parseErr != nil || id <= 0 {
			return fmt.Errorf("same-origin group link does not identify a valid Canvas group")
		}
		nickname := ""
		for _, course := range s.Config.Courses {
			if course.ID == courseID {
				nickname = course.Nickname
				break
			}
		}
		if nickname == "" {
			return fmt.Errorf("selected course is not configured")
		}
		groups, groupErr := s.ListGroups(ctx, nickname, 0, id, false)
		if groupErr != nil || len(groups) != 1 || groups[0].ID != id {
			return fmt.Errorf("same-origin group link targets a group outside the selected course")
		}
		return nil
	}
	return nil
}

func sameBoolPointer(a, b *bool) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func optionalString(value *string) string {
	if value == nil || *value == "" {
		return "unset"
	}
	return *value
}

func stateBool(value *bool) string {
	if value == nil {
		return "unknown"
	}
	return strconv.FormatBool(*value)
}

func (s *Service) getModuleItem(ctx context.Context, courseID, moduleID, itemID int64) (canvasModuleItem, error) {
	var item canvasModuleItem
	err := s.Client.Get(ctx, fmt.Sprintf("/api/v1/courses/%d/modules/%d/items/%d", courseID, moduleID, itemID), &item)
	if err == nil && item.ID != itemID {
		err = fmt.Errorf("Canvas returned module item %d for requested module item %d", item.ID, itemID)
	}
	return item, err
}

func moduleItemMatches(live canvasModuleItem, spec domain.ModuleItem, resolved string) bool {
	if live.Type != spec.Type {
		return false
	}
	if spec.Type == "SubHeader" {
		return live.Title == spec.Title
	}
	if spec.Type == "Page" {
		return resolved != "" && (live.PageURL == resolved || contentString(live.ContentID) == resolved)
	}
	if spec.Type == "ExternalUrl" {
		return live.ExternalURL == spec.ExternalURL
	}
	return contentString(live.ContentID) == strconv.FormatInt(spec.ContentID, 10)
}

func contentString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatInt(int64(v), 10)
	case json.Number:
		return v.String()
	default:
		return fmt.Sprint(value)
	}
}
