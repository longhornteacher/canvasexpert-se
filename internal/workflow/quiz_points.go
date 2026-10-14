package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"canvasexpertse/internal/domain"
)

// QuizItemLine is one New Quiz item as the teacher sees it: position, type,
// points and the start of its prompt.
type QuizItemLine struct {
	Position int
	Type     string
	Points   string
	Was      string // previous points when this run changed the item
	Stem     string
}

// QuizPointsReport lists a New Quiz's items and any point changes made.
type QuizPointsReport struct {
	AssignmentID     int64
	Title, URL       string
	Items            []QuizItemLine
	Changed          int
	ItemTotal        string
	QuizPoints       string
	AssignmentPoints string
	Status           string // listed, completed, held, partial
	Message          string
}

type quizItem struct {
	ID        json.Number `json:"id"` // Canvas returns IDs as strings
	Position  int         `json:"position"`
	Points    json.Number `json:"points_possible"`
	EntryType string      `json:"entry_type"`
	Entry     struct {
		Slug string `json:"interaction_type_slug"`
		Body string `json:"item_body"`
	} `json:"entry"`
}

var htmlTag = regexp.MustCompile(`<[^>]+>`)

// QuizPoints lists a New Quiz's items, or sets the points of the items named by
// position. It holds before writing once any student has submitted, because
// changing points would then change their scores; Canvas's editor offers
// regrade choices for that case. Setting points is idempotent, so a rerun
// resolves an uncertain write.
func (s *Service) QuizPoints(ctx context.Context, nickname, query string, changes map[int]domain.Decimal) (r QuizPointsReport, err error) {
	course, err := s.Course(nickname)
	if err != nil {
		return r, err
	}
	a, err := FindAssignment(ctx, s.Client, course.ID, query)
	if err != nil {
		return r, err
	}
	if !a.NewQuiz {
		return r, fmt.Errorf("assignment %d is not a New Quiz", a.ID)
	}
	r.AssignmentID, r.Title, r.URL = a.ID, a.Name, a.HTMLURL
	base := fmt.Sprintf("/api/quiz/v1/courses/%d/quizzes/%d", course.ID, a.ID)
	items, err := s.quizItems(ctx, base)
	if err != nil {
		return r, err
	}
	byPos := map[int]quizItem{}
	for _, it := range items {
		byPos[it.Position] = it
	}
	positions := make([]int, 0, len(changes))
	for p, pts := range changes {
		it, ok := byPos[p]
		if !ok {
			return r, fmt.Errorf("New Quiz %d has no item at position %d", a.ID, p)
		}
		if it.EntryType != "Item" {
			return r, fmt.Errorf("position %d is a %s, not a question; change it in Canvas", p, it.EntryType)
		}
		if pts.Sign() < 0 {
			return r, fmt.Errorf("points for position %d cannot be negative", p)
		}
		positions = append(positions, p)
	}
	sort.Ints(positions)
	was := map[int]string{}
	if len(positions) == 0 {
		r.Status = "listed"
	} else {
		var subs []Submission
		if err = s.Client.List(ctx, fmt.Sprintf("/api/v1/courses/%d/assignments/%d/submissions?per_page=100", course.ID, a.ID), &subs); err != nil {
			return r, err
		}
		submitted := false
		for _, sub := range subs {
			if sub.SubmittedAt != nil || sub.State == "graded" || sub.State == "pending_review" {
				submitted = true
				break
			}
		}
		r.Status = "completed"
		if submitted {
			r.Message = "Students had already submitted. Canvas may not regrade their existing attempts; check the gradebook, and use the quiz editor's regrade option if scores should change."
		}
		var failed []string
		for _, p := range positions {
			it, target := byPos[p], changes[p]
			if now, e := domain.ParseDecimal(it.Points.String()); e == nil && now.Cmp(target) == 0 {
				continue
			}
			if e := s.Client.Write(ctx, "PATCH", base+"/items/"+it.ID.String(), map[string]any{"item": map[string]any{"points_possible": target}}, nil); e != nil {
				failed = append(failed, fmt.Sprintf("position %d: %v", p, e))
				continue
			}
			was[p] = it.Points.String()
			r.Changed++
		}
		if items, err = s.quizItems(ctx, base); err != nil {
			return r, err
		}
		for _, it := range items {
			if target, ok := changes[it.Position]; ok {
				if now, e := domain.ParseDecimal(it.Points.String()); e != nil || now.Cmp(target) != 0 {
					failed = append(failed, fmt.Sprintf("position %d reads back %s, not %s", it.Position, it.Points, target))
				}
			}
		}
		if len(failed) > 0 {
			r.Status = "partial"
			r.Message = "not confirmed: " + strings.Join(failed, "; ") + "; rerun the same command to retry"
		}
	}
	r.Items, r.ItemTotal = quizLines(items, was)
	var quiz struct {
		Points json.Number `json:"points_possible"`
	}
	if err = s.Client.Get(ctx, base, &quiz); err != nil {
		return r, err
	}
	r.QuizPoints = quiz.Points.String()
	if live, e := GetAssignment(ctx, s.Client, course.ID, a.ID); e == nil {
		r.AssignmentPoints = live.Points.String()
	}
	if total, e := domain.ParseDecimal(r.ItemTotal); e == nil {
		if qp, e := domain.ParseDecimal(r.QuizPoints); e == nil && qp.Cmp(total) != 0 {
			r.Message = strings.TrimSpace(r.Message + fmt.Sprintf("\nItems total %s but the quiz is worth %s in Canvas; tell the teacher, who can set the quiz total in Canvas.", r.ItemTotal, r.QuizPoints))
		}
	}
	return r, nil
}

func (s *Service) quizItems(ctx context.Context, base string) ([]quizItem, error) {
	var items []quizItem
	if err := s.Client.List(ctx, base+"/items", &items); err != nil {
		return nil, err
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Position < items[j].Position })
	return items, nil
}

func quizLines(items []quizItem, was map[int]string) ([]QuizItemLine, string) {
	var lines []QuizItemLine
	total := domain.MustDecimal("0").Rat()
	for _, it := range items {
		stem := compactText(htmlTag.ReplaceAllString(it.Entry.Body, " "))
		if r := []rune(stem); len(r) > 60 {
			stem = string(r[:60]) + "…"
		}
		kind := it.Entry.Slug
		if it.EntryType != "Item" {
			kind = it.EntryType
		}
		lines = append(lines, QuizItemLine{Position: it.Position, Type: kind, Points: it.Points.String(), Was: was[it.Position], Stem: stem})
		if p, e := domain.ParseDecimal(it.Points.String()); e == nil && it.EntryType == "Item" {
			total.Add(total, p.Rat())
		}
	}
	return lines, strings.TrimSuffix(strings.TrimRight(total.FloatString(4), "0"), ".")
}
