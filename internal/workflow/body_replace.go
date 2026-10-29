package workflow

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strconv"
	"strings"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

// BodyReplacement is one exact find/replace pair. Find must match exactly once
// unless All is set.
type BodyReplacement struct {
	Find    string `json:"find"`
	Replace string `json:"replace"`
	All     bool   `json:"all,omitempty"`
}

// BodyReplaceRequest edits only the body of an existing course Page or the
// description of an existing assignment. The baseline guards are optional and
// come from a prior pull or replace report.
type BodyReplaceRequest struct {
	Course           string
	Target           string // "page" or "assignment"
	Query            string // Page ID or link; assignment ID
	Replacements     []BodyReplacement
	ExpectUpdatedAt  string
	ExpectBodySHA256 string
	Push             bool
}

type BodyReplaceReport struct {
	Path, Status, Message string
	// BodySHA256 and UpdatedAt describe the live body that was read, so a later
	// call can guard against a change made in between.
	BodySHA256, UpdatedAt string
	Counts                []int
	Page                  *PagePushReport
	Assignment            *PushReport
}

func BodySHA256(body string) string {
	sum := sha256.Sum256([]byte(body))
	return fmt.Sprintf("%x", sum)
}

// ApplyBodyReplacements applies the pairs in order. Nothing is returned unless
// every pair matched as requested, so a partial edit is never written.
func ApplyBodyReplacements(body string, pairs []BodyReplacement) (string, []int, error) {
	if len(pairs) == 0 {
		return "", nil, fmt.Errorf("at least one find/replace pair is required")
	}
	counts := make([]int, len(pairs))
	for i, p := range pairs {
		if p.Find == "" {
			return "", nil, fmt.Errorf("replacement %d: find cannot be empty", i+1)
		}
		if p.Find == p.Replace {
			return "", nil, fmt.Errorf("replacement %d: find and replace are the same", i+1)
		}
		n := strings.Count(body, p.Find)
		counts[i] = n
		if n == 0 {
			return "", counts, fmt.Errorf("replacement %d: find text matches nothing in the current body; copy it exactly from a fresh pull (Canvas stores HTML, e.g. &amp; and &nbsp;)", i+1)
		}
		if n > 1 && !p.All {
			return "", counts, fmt.Errorf("replacement %d: find text matches %d places; add surrounding text so it matches once, or ask for all occurrences", i+1, n)
		}
		body = strings.ReplaceAll(body, p.Find, p.Replace)
	}
	return body, counts, nil
}

// ReplaceBody pulls the live item into a fresh file, checks the baseline,
// applies the replacements and saves them as an ordinary edit of only the body.
// With Push, the existing Page or assignment push sends it and holds with the
// edit preserved if Canvas changed again; otherwise the file is ready for
// the teacher's go-ahead and a push. A refusal leaves the fresh pull unchanged.
func (s *Service) ReplaceBody(ctx context.Context, req BodyReplaceRequest) (BodyReplaceReport, error) {
	var r BodyReplaceReport
	var err error
	switch req.Target {
	case "page":
		r.Path, err = s.PullPage(ctx, req.Course, req.Query)
	case "assignment":
		if id, e := strconv.ParseInt(req.Query, 10, 64); e != nil || id <= 0 {
			return r, fmt.Errorf("assignment replace requires the exact assignment ID")
		}
		r.Path, err = s.PullItem(ctx, req.Course, req.Query)
	default:
		return r, fmt.Errorf("target must be page or assignment")
	}
	if err != nil {
		return r, err
	}
	var page domain.PageDocument
	var assignment domain.AssignmentDocument
	var header any
	var observed string
	if req.Target == "page" {
		header = &page
	} else {
		header = &assignment
	}
	body, err := workspace.ReadDocument(r.Path, header)
	if err != nil {
		return r, err
	}
	if req.Target == "page" {
		observed, r.UpdatedAt = page.Observed.BodyHTML, page.Observed.UpdatedAt
	} else {
		observed, r.UpdatedAt = assignment.Observed.DescriptionHTML, assignment.Observed.UpdatedAt
	}
	r.BodySHA256 = BodySHA256(observed)
	held := func(message string) (BodyReplaceReport, error) {
		r.Status, r.Message = "held", message+"; nothing was written to Canvas; fresh version: "+r.Path
		return r, nil
	}
	if req.ExpectUpdatedAt != "" && !sameDate(&req.ExpectUpdatedAt, &r.UpdatedAt) {
		return held("Canvas changed since your baseline (updated_at " + r.UpdatedAt + ")")
	}
	if req.ExpectBodySHA256 != "" && !strings.EqualFold(req.ExpectBodySHA256, r.BodySHA256) {
		return held("the live body differs from your baseline digest")
	}
	edited, counts, err := ApplyBodyReplacements(body, req.Replacements)
	r.Counts = counts
	if err != nil {
		if counts == nil {
			return r, err
		}
		return held(err.Error())
	}
	if req.Target == "assignment" {
		// PlanEdit refuses a New Quiz and confirms only the description changes.
		if _, err = PlanEdit(assignment, edited); err != nil {
			return r, err
		}
	}
	if err = workspace.WriteDocument(r.Path, header, edited); err != nil {
		return r, err
	}
	if !req.Push {
		r.Status, r.Message = "prepared", "body edit saved; push it after the teacher's go-ahead: "+r.Path
		return r, nil
	}
	if req.Target == "page" {
		report, err := s.PushPage(ctx, r.Path)
		r.Page, r.Status, r.Message = &report, report.Status, report.Message
		return r, err
	}
	report, err := s.Push(ctx, r.Path)
	r.Assignment, r.Status, r.Message = &report, report.Status, report.Message
	return r, err
}
