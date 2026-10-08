package workflow

import (
	"context"
	"fmt"
	"sort"

	"canvasexpertse/internal/canvas"
)

// DifferentiationTags reads Canvas tag names and accepted memberships for display.
// Collaborative groups are excluded; this does not alter tags or assign content.
func DifferentiationTags(ctx context.Context, c *canvas.Client, courseID int64) (map[int64][]string, error) {
	var groups []struct {
		ID               int64  `json:"id"`
		Name             string `json:"name"`
		NonCollaborative bool   `json:"non_collaborative"`
	}
	if err := c.List(ctx, fmt.Sprintf("/api/v1/courses/%d/groups?collaboration_state=non_collaborative&per_page=100", courseID), &groups); err != nil {
		return nil, err
	}
	tags := map[int64][]string{}
	for _, group := range groups {
		if !group.NonCollaborative || group.ID <= 0 || group.Name == "" {
			continue
		}
		var members []struct {
			UserID int64  `json:"user_id"`
			State  string `json:"workflow_state"`
		}
		if err := c.List(ctx, fmt.Sprintf("/api/v1/groups/%d/memberships?filter_states[]=accepted&per_page=100", group.ID), &members); err != nil {
			return nil, err
		}
		for _, member := range members {
			if member.UserID > 0 && member.State == "accepted" {
				tags[member.UserID] = append(tags[member.UserID], group.Name)
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
