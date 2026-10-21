package workflow

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"canvasexpertse/internal/canvas"
)

// A session already needs submission history and comments. Read them once per
// assignment, concurrently, instead of listing then fetching each student.
// Group assignments retain their assignment-specific audience and evidence path.
func detailedSessionSubmissions(ctx context.Context, client *canvas.Client, courseID int64, assignments []Assignment, broad bool) (map[int64][]Submission, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	out := map[int64][]Submission{}
	var mu sync.Mutex
	var once sync.Once
	var first error
	var wg sync.WaitGroup
	for _, assignment := range assignments {
		if broad && (assignment.NeedsGradingCount != nil && *assignment.NeedsGradingCount == 0 || assignment.InClosedGradingPeriod) || assignment.GroupCategoryID != nil && *assignment.GroupCategoryID > 0 || unpullableAssignment(assignment) != "" {
			continue
		}
		wg.Add(1)
		go func(a Assignment) {
			defer wg.Done()
			rows, err := Submissions(ctx, client, courseID, a.ID, true)
			if err == nil {
				for _, sub := range rows {
					if sub.AssignmentID != a.ID || sub.UserID <= 0 || sub.User.ID != 0 && sub.User.ID != sub.UserID {
						err = fmt.Errorf("live submission identity disagrees with requested pull target")
						break
					}
				}
			}
			if err != nil {
				once.Do(func() { first = err; cancel() })
				return
			}
			mu.Lock()
			out[a.ID] = rows
			mu.Unlock()
		}(assignment)
	}
	wg.Wait()
	return out, first
}

func assignmentDueState(a Assignment, now time.Time) string {
	dates := []*string{a.DueAt}
	if a.AllDates != nil && len(*a.AllDates) > 0 {
		dates = nil
		for _, date := range *a.AllDates {
			dates = append(dates, date.DueAt)
		}
	}
	past, future, undated := false, false, false
	for _, value := range dates {
		if value == nil || *value == "" {
			undated = true
			continue
		}
		due, err := time.Parse(time.RFC3339Nano, *value)
		if err != nil {
			continue
		}
		if due.After(now) {
			future = true
		} else {
			past = true
		}
	}
	var states []string
	if past {
		states = append(states, "past due")
	}
	if future {
		states = append(states, "not due yet")
	}
	if undated || len(states) == 0 {
		states = append(states, "undated")
	}
	return strings.Join(states, " / ")
}
