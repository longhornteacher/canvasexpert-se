package workflow

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"canvasexpertse/internal/canvas"
	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

func rubricReferenceStatus(selection *domain.RubricSelection, out domain.ActionOutcome) string {
	if out.Status == "failed" || out.Status == "uncertain" {
		return out.Status
	}
	if definite(out) {
		return "partial"
	}
	if selection != nil && (selection.Create.Status == "failed" || selection.Create.Status == "uncertain") {
		return selection.Create.Status
	}
	return "held"
}

func (s *Service) referenceRubric(ctx context.Context, courseID, id int64) (domain.Rubric, error) {
	base := courseRubricsPath(courseID)
	r, err := s.rubricDetail(ctx, base, id)
	var ce *canvas.Error
	if errors.As(err, &ce) && ce.Status == 404 {
		if account, e := s.rubricAccountBase(ctx, courseID); e == nil {
			r, err = s.rubricDetail(ctx, account, id)
		}
	}
	if err != nil {
		return domain.Rubric{}, err
	}
	portable, err := portableRubric(r)
	return portable.Definition(), err
}

func (s *Service) libraryRubric(key string) (domain.Rubric, string, error) {
	if err := domain.ValidateAuthoredRubric(domain.Rubric{Library: key}); err != nil {
		return domain.Rubric{}, "", err
	}
	path, err := workspace.ResolveRulesPath(s.Root, filepath.Join("context", "rubrics", key+".md"))
	if err != nil {
		return domain.Rubric{}, "", err
	}
	entries, e := os.ReadDir(filepath.Dir(path))
	if e != nil {
		return domain.Rubric{}, "", e
	}
	matched := ""
	for _, entry := range entries {
		if !entry.IsDir() && strings.EqualFold(entry.Name(), key+".md") {
			if matched != "" {
				return domain.Rubric{}, "", fmt.Errorf("rubric library key %q is ambiguous", key)
			}
			matched = entry.Name()
		}
	}
	if matched != "" {
		path, err = workspace.ResolveRulesPath(s.Root, filepath.Join("context", "rubrics", matched))
		if err != nil {
			return domain.Rubric{}, "", err
		}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return domain.Rubric{}, "", fmt.Errorf("rubric library entry %q is unavailable in the active teacher setup: %w", key, err)
	}
	var d domain.RubricLibraryDocument
	if _, err = workspace.ParseDocument(b, &d); err != nil {
		return domain.Rubric{}, "", err
	}
	digest := sha256.Sum256(b)
	return d.Rubric.Definition(), hex.EncodeToString(digest[:]), nil
}

// resolveRubricReference pins source intent before any content is created.
// It does not create or attach a Canvas rubric.
func (s *Service) resolveRubricReference(ctx context.Context, courseID int64, ref *domain.Rubric, selected **domain.RubricSelection, save func() error) error {
	if !domain.IsRubricReference(ref) {
		if *selected != nil {
			return fmt.Errorf("saved rubric reference changed; use a new draft")
		}
		return nil
	}
	if err := domain.ValidateAuthoredRubric(*ref); err != nil {
		return err
	}
	var definition domain.Rubric
	var digest string
	var err error
	if ref.Library != "" {
		definition, digest, err = s.libraryRubric(ref.Library)
	} else {
		definition, err = s.referenceRubric(ctx, courseID, ref.ID)
	}
	if err != nil {
		return err
	}
	prior := *selected
	if prior != nil {
		if prior.Library != ref.Library || prior.RequestedID != ref.ID || prior.LibrarySHA256 != digest || !sameRubric(&prior.Definition, &definition) {
			return fmt.Errorf("rubric reference or library definition changed during an unfinished operation; preserve this draft and create a new one")
		}
		if prior.RubricID > 0 {
			actual, e := s.referenceRubric(ctx, courseID, prior.RubricID)
			if e != nil {
				return e
			}
			matches := sameRubricDefinition(actual, definition)
			if ref.ID > 0 {
				matches = sameRubric(&actual, &definition)
			}
			if !matches {
				return fmt.Errorf("selected Canvas rubric changed; preserve it and use a new draft")
			}
			return nil
		}
	}
	selection := &domain.RubricSelection{Library: ref.Library, RequestedID: ref.ID, LibrarySHA256: digest, Definition: definition}
	if prior != nil {
		selection.Create = prior.Create
	}
	if ref.ID > 0 {
		selection.RubricID = ref.ID
		selection.Create = domain.ActionOutcome{Status: "preserved", ID: ref.ID}
	} else {
		var rubrics []rubricRead
		if err = s.Client.List(ctx, courseRubricsPath(courseID)+"?per_page=100", &rubrics); err != nil {
			return err
		}
		for _, r := range rubrics {
			if r.Title != definition.Title {
				continue
			}
			selection.NewVersion = true
			if r.ID <= 0 {
				return fmt.Errorf("course rubric catalog lacks an exact rubric ID")
			}
			if r.Data == nil {
				r, err = s.rubricDetail(ctx, courseRubricsPath(courseID), r.ID)
				if err != nil {
					return err
				}
			}
			portable, e := portableRubric(r)
			if e != nil {
				return e
			}
			if sameRubricDefinition(portable.Definition(), definition) && (selection.RubricID == 0 || r.ID < selection.RubricID) {
				selection.RubricID = r.ID
			}
		}
		if selection.RubricID > 0 {
			selection.Create = domain.ActionOutcome{Status: "preserved", ID: selection.RubricID}
			selection.NewVersion = false
		}
		if selection.RubricID == 0 && ambiguous(selection.Create) {
			return fmt.Errorf("prior rubric creation is unresolved and no matching course definition is visible; inspect Canvas before retrying")
		}
	}
	*selected = selection
	return save()
}

// pushReferencedRubric creates a missing course definition once, then attaches
// it through the public association endpoint. An accepted association has no
// confirmation read; the next invocation can recover an uncertain association.
func (s *Service) pushReferencedRubric(ctx context.Context, courseID, assignmentID int64, selection *domain.RubricSelection, out *domain.ActionOutcome, rubricID, associationID *int64, live Assignment, save func() error) error {
	if definite(*out) {
		return nil
	}
	if selection == nil {
		return fmt.Errorf("rubric reference has no resolved selection")
	}
	if live.ID != assignmentID || live.CourseID != courseID {
		return fmt.Errorf("rubric association targets a different assignment/course")
	}
	if len(live.Rubric) > 0 {
		snapshot, err := Snapshot(live, s.Client.Origin())
		if err != nil {
			return err
		}
		r := snapshot.Rubric
		if ambiguous(*out) && r != nil && r.ID == selection.RubricID && r.UseForGradingKnown && !r.UseForGrading && sameRubricDefinition(*r, selection.Definition) {
			*rubricID, *associationID = r.ID, r.AssociationID
			*out = domain.ActionOutcome{Status: "preserved", ID: r.ID, Message: "Exact independent rubric association recovered."}
			return save()
		}
		return fmt.Errorf("assignment already has a rubric; its definition and association were preserved")
	}
	if ambiguous(*out) {
		return fmt.Errorf("prior rubric association is unresolved; no association was replayed")
	}
	if selection.RubricID == 0 {
		if ambiguous(selection.Create) {
			return fmt.Errorf("prior rubric creation is unresolved; no duplicate rubric was created")
		}
		selection.Create = domain.ActionOutcome{Status: "pending"}
		if err := save(); err != nil {
			return err
		}
		payload := rubricCreateFields(selection.Definition, 0)
		payload["rubric_association"] = map[string]any{"association_id": courseID, "association_type": "Course", "purpose": "bookmark", "use_for_grading": false}
		var response struct {
			Rubric struct {
				ID int64 `json:"id"`
			} `json:"rubric"`
		}
		if err := s.Client.WriteAccepted(ctx, "POST", courseRubricsPath(courseID), payload, &response); err != nil {
			selection.Create = failedOutcome(err)
			if e := save(); e != nil {
				return e
			}
			return err
		}
		if response.Rubric.ID <= 0 {
			selection.Create = domain.ActionOutcome{Status: "uncertain", Message: "Canvas accepted rubric creation without its ID; inspect course rubrics before resuming."}
			if e := save(); e != nil {
				return e
			}
			return fmt.Errorf("%s", selection.Create.Message)
		}
		selection.RubricID = response.Rubric.ID
		selection.Create = domain.ActionOutcome{Status: "succeeded", ID: selection.RubricID}
		if err := save(); err != nil {
			return fmt.Errorf("Canvas created the course rubric; its ID could not be saved: %w", err)
		}
	}
	*out = domain.ActionOutcome{Status: "pending"}
	if err := save(); err != nil {
		return err
	}
	var response struct {
		ID int64 `json:"id"`
	}
	payload := map[string]any{"rubric_association": map[string]any{"rubric_id": selection.RubricID, "association_id": assignmentID, "association_type": "Assignment", "purpose": "grading", "use_for_grading": false}}
	if err := s.Client.WriteAccepted(ctx, "POST", fmt.Sprintf("/api/v1/courses/%d/rubric_associations", courseID), payload, &response); err != nil {
		*out = failedOutcome(err)
		if e := save(); e != nil {
			return e
		}
		return err
	}
	*rubricID, *associationID = selection.RubricID, response.ID
	message := "Canvas accepted the independent rubric association."
	if selection.NewVersion {
		message = "Library definition changed; a new course rubric version was created and attached for this assignment."
	}
	*out = domain.ActionOutcome{Status: "succeeded", ID: selection.RubricID, Message: message}
	if err := save(); err != nil {
		return fmt.Errorf("Canvas attached the rubric; its accepted outcome could not be saved: %w", err)
	}
	return nil
}
