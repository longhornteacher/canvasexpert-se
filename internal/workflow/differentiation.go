package workflow

import (
	"context"
	"fmt"
	"sort"

	"canvasexpertse/internal/canvas"
)

// DifferentiationTags reads Canvas tag names and accepted memberships for display.
// Collaborative groups are excluded; this does not alter tags or assign content.
func DifferentiationTags(ctx context.Context, c *canvas.Client, courseID int64, userIDs ...int64) (map[int64][]string, error) {
	var groups []struct {
		ID               int64  `json:"id"`
		Name             string `json:"name"`
		NonCollaborative bool   `json:"non_collaborative"`
	}
	if err := c.List(ctx, fmt.Sprintf("/api/v1/courses/%d/groups?collaboration_state=non_collaborative&per_page=100", courseID), &groups); err != nil {
		return nil, err
	}
	tags := map[int64][]string{}
	namesByID := map[int64]string{}
	for _, group := range groups {
		if !group.NonCollaborative || group.ID <= 0 || group.Name == "" {
			continue
		}
		namesByID[group.ID] = group.Name
	}
	if len(namesByID) == 0 {
		return tags, nil
	}
	if len(userIDs) == 0 {
		enrollments, e := StudentEnrollments(ctx, c, courseID)
		if e != nil {
			return nil, e
		}
		seen := map[int64]bool{}
		for _, en := range enrollments {
			id := en.UserID
			if id == 0 {
				id = en.User.ID
			}
			if id > 0 && !seen[id] {
				userIDs = append(userIDs, id)
				seen[id] = true
			}
		}
	}
	memberships, e := bulkUserTagIDs(ctx, c, courseID, userIDs)
	if e != nil {
		return nil, e
	}
	for id, groupIDs := range memberships {
		for _, groupID := range groupIDs {
			if name, ok := namesByID[groupID]; ok {
				tags[id] = append(tags[id], name)
			}
		}
	}
	for id, names := range tags {
		sort.Strings(names)
		unique := names[:0]
		for _, name := range names {
			if len(unique) == 0 || unique[len(unique)-1] != name {
				unique = append(unique, name)
			}
		}
		tags[id] = unique
	}
	return tags, nil
}
