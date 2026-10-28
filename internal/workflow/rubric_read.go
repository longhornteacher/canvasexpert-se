package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"canvasexpertse/internal/canvas"
	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

type rubricRead struct {
	ID       int64        `json:"id"`
	Title    string       `json:"title"`
	Points   json.Number  `json:"points_possible"`
	FreeForm bool         `json:"free_form_criterion_comments"`
	Data     *[]Criterion `json:"data"`
}

type RubricLine struct {
	ID                     int64
	Title, Points, Context string
	CriteriaCount          *int
}

type RubricsReport struct {
	Rubrics     []RubricLine
	Limitations []string
}

func courseRubricsPath(courseID int64) string {
	return fmt.Sprintf("/api/v1/courses/%d/rubrics", courseID)
}

func (s *Service) rubricCourse(ctx context.Context, nickname string) (*Service, domain.Course, error) {
	course, err := s.Course(nickname)
	if err != nil {
		return s, course, err
	}
	s, err = s.ForCourse(ctx, course)
	return s, course, err
}

func (s *Service) rubricDetail(ctx context.Context, base string, id int64) (rubricRead, error) {
	var r rubricRead
	err := s.Client.Get(ctx, fmt.Sprintf("%s/%d", base, id), &r)
	if err == nil && r.ID != id {
		err = fmt.Errorf("Canvas returned a different rubric identity")
	}
	return r, err
}

func (s *Service) rubricAccountBase(ctx context.Context, courseID int64) (string, error) {
	var live struct {
		ID        int64 `json:"id"`
		AccountID int64 `json:"account_id"`
	}
	if err := s.Client.Get(ctx, fmt.Sprintf("/api/v1/courses/%d", courseID), &live); err != nil {
		return "", err
	}
	if live.ID != courseID || live.AccountID <= 0 {
		return "", fmt.Errorf("could not confirm this course's account")
	}
	return fmt.Sprintf("/api/v1/accounts/%d/rubrics", live.AccountID), nil
}

// ListRubrics requests definitions only, never assessments or student records.
func (s *Service) ListRubrics(ctx context.Context, nickname string, includeAccount bool) (r RubricsReport, err error) {
	s, course, err := s.rubricCourse(ctx, nickname)
	if err != nil {
		return r, err
	}
	r.Rubrics = []RubricLine{}
	appendScope := func(base, scope string) error {
		var rubrics []rubricRead
		if err := s.Client.List(ctx, base+"?per_page=100", &rubrics); err != nil {
			return err
		}
		for _, rubric := range rubrics {
			if rubric.ID <= 0 {
				return fmt.Errorf("Canvas rubric list lacks a valid rubric ID")
			}
			line := RubricLine{ID: rubric.ID, Title: rubric.Title, Points: rubric.Points.String(), Context: scope}
			if rubric.Data == nil {
				full, e := s.rubricDetail(ctx, base, rubric.ID)
				if e == nil {
					rubric = full
				}
			}
			if rubric.Data != nil {
				count := len(*rubric.Data)
				line.CriteriaCount = &count
			} else {
				r.Limitations = append(r.Limitations, fmt.Sprintf("%s rubric %d: criterion count unavailable", scope, rubric.ID))
			}
			if line.Points == "" {
				line.Points = rubric.Points.String()
			}
			if line.Title == "" {
				line.Title = rubric.Title
			}
			if line.Points == "" {
				line.Points = "unknown"
			}
			r.Rubrics = append(r.Rubrics, line)
		}
		return nil
	}
	if err = appendScope(courseRubricsPath(course.ID), "course"); err != nil {
		return r, err
	}
	if includeAccount {
		accountBase, e := s.rubricAccountBase(ctx, course.ID)
		if e != nil {
			r.Limitations = append(r.Limitations, "School rubrics unavailable: could not confirm this course's account.")
		} else if e := appendScope(accountBase, "account"); e != nil {
			var ce *canvas.Error
			if errors.As(e, &ce) && (ce.Status == 401 || ce.Status == 403) {
				r.Limitations = append(r.Limitations, "Your Canvas account can't see school rubrics.")
			} else {
				r.Limitations = append(r.Limitations, "School rubric list unavailable; course rubrics are still usable.")
			}
		}
	}
	sort.SliceStable(r.Rubrics, func(i, j int) bool {
		if r.Rubrics[i].Context != r.Rubrics[j].Context {
			return r.Rubrics[i].Context < r.Rubrics[j].Context
		}
		return r.Rubrics[i].ID < r.Rubrics[j].ID
	})
	return r, nil
}

type RubricPullReport struct {
	Path                         string
	RubricID                     int64
	UsedCourses, UsedAssignments *int
	Limitations                  []string
}

func (s *Service) rubricUsage(ctx context.Context, base string, id int64) (*int, *int, string) {
	var rows []struct {
		ID          int64 `json:"id"`
		Assignments *[]struct {
			ID int64 `json:"id"`
		} `json:"assignments"`
	}
	if err := s.Client.List(ctx, fmt.Sprintf("%s/%d/used_locations?per_page=100", base, id), &rows); err != nil {
		return nil, nil, "Rubric usage counts unavailable; the definition is still usable."
	}
	courses, assignments := map[int64]bool{}, map[int64]bool{}
	for _, row := range rows {
		if row.ID <= 0 || row.Assignments == nil {
			return nil, nil, "Canvas returned an unrecognized usage response; counts are unknown."
		}
		courses[row.ID] = true
		for _, a := range *row.Assignments {
			if a.ID <= 0 {
				return nil, nil, "Canvas returned an unrecognized usage response; counts are unknown."
			}
			assignments[a.ID] = true
		}
	}
	c, a := len(courses), len(assignments)
	return &c, &a, ""
}

func portableRubric(r rubricRead) (domain.PortableRubric, error) {
	if r.Data == nil {
		return domain.PortableRubric{}, fmt.Errorf("Canvas did not return rubric criteria; no definition was saved")
	}
	d := domain.PortableRubric{Title: r.Title, FreeFormCriterionComments: r.FreeForm}
	for i, c := range *r.Data {
		points, e := domain.ParseDecimal(c.Points.String())
		if e != nil {
			return d, fmt.Errorf("criterion %d has no usable point maximum", i+1)
		}
		criterion := domain.Criterion{ID: fmt.Sprintf("criterion-%d", i+1), Description: c.Description, LongDescription: c.LongDescription, Points: points, UseRange: c.Range}
		for j, rating := range c.Ratings {
			points, e := domain.ParseDecimal(rating.Points.String())
			if e != nil {
				return d, fmt.Errorf("criterion %d rating %d has no usable points", i+1, j+1)
			}
			criterion.Ratings = append(criterion.Ratings, domain.Rating{ID: fmt.Sprintf("rating-%d", j+1), Description: rating.Description, LongDescription: rating.LongDescription, Points: points})
		}
		d.Criteria = append(d.Criteria, criterion)
	}
	if err := domain.ValidateRubric(d.Definition()); err != nil {
		return d, err
	}
	return d, nil
}

// PullRubric exports a portable definition into a fresh local draft. Saving it
// into the teacher's library is a separate teacher-requested file operation.
func (s *Service) PullRubric(ctx context.Context, nickname string, id int64) (r RubricPullReport, err error) {
	if id <= 0 {
		return r, fmt.Errorf("rubric ID must be positive")
	}
	s, course, err := s.rubricCourse(ctx, nickname)
	if err != nil {
		return r, err
	}
	base := courseRubricsPath(course.ID)
	live, err := s.rubricDetail(ctx, base, id)
	var ce *canvas.Error
	if errors.As(err, &ce) && ce.Status == 404 {
		accountBase, e := s.rubricAccountBase(ctx, course.ID)
		if e == nil {
			base = accountBase
			live, err = s.rubricDetail(ctx, base, id)
		}
	}
	if err != nil {
		return r, err
	}
	definition, err := portableRubric(live)
	if err != nil {
		return r, err
	}
	r.RubricID = id
	var limitation string
	r.UsedCourses, r.UsedAssignments, limitation = s.rubricUsage(ctx, base, id)
	if limitation != "" {
		r.Limitations = append(r.Limitations, limitation)
	}
	r.Path, err = workspace.ConstrainPath(s.Root, filepath.Join("courses", course.Nickname, "drafts", fmt.Sprintf("rubric-%d-%s.md", id, time.Now().UTC().Format("20060102T150405.000000000"))))
	if err != nil {
		return r, err
	}
	d := domain.RubricLibraryDocument{Schema: 1, Kind: "rubric-library", Rubric: definition}
	err = workspace.WriteDocument(r.Path, &d, "Pulled rubric definition. Local criterion and rating keys are portable; this draft contains no Canvas association or student assessments.\n")
	return r, err
}
