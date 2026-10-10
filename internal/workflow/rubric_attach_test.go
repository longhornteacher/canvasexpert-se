package workflow

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

func attachmentBaseline() Assignment {
	a := fixtureAssignment()
	a.Rubric = nil
	a.RubricAssociation = nil
	a.RubricSettings.ID = 0
	a.Published = true
	a.Description = `<section data-teacher="keep"><details>Original HTML</details></section>`
	due := "2026-10-08T15:00:00Z"
	a.DueAt = &due
	return a
}
func attachmentDraft(t *testing.T, s *Service, base Assignment) string {
	t.Helper()
	observed, e := Snapshot(base, s.Client.Origin())
	if e != nil {
		t.Fatal(e)
	}
	r := domain.Rubric{Title: "Evidence", Criteria: []domain.Criterion{{ID: "draft-quality", Description: "Evidence", Points: domain.MustDecimal("100.0"), Ratings: []domain.Rating{{ID: "draft-full", Description: "Complete", Points: domain.MustDecimal("100")}, {ID: "draft-empty", Description: "Missing", Points: domain.MustDecimal("0")}}}}}
	d := domain.AssignmentDocument{Schema: 1, Kind: "assignment", Operation: "attach-rubric", Course: "TEST", Instance: s.Client.Origin(), CourseID: 1, AssignmentID: base.ID, Title: base.Name, Points: observed.Points, SubmissionTypes: base.SubmissionTypes, DueAt: base.DueAt, UnlockAt: base.UnlockAt, LockAt: base.LockAt, Published: base.Published, Rubric: &r, Observed: &observed}
	local := "2026-10-08T10:00:00-05:00"
	d.DueAt = &local
	path := filepath.Join(s.Root, "attachment.md")
	if e = workspace.WriteDocument(path, d, base.Description); e != nil {
		t.Fatal(e)
	}
	return path
}
func installFixtureRubric(a *Assignment) {
	r := fixtureAssignment()
	a.Rubric = r.Rubric
	a.RubricSettings = r.RubricSettings
	a.RubricAssociation = r.RubricAssociation
}

func TestAttachRubricPreservesExistingAssignmentAndCompletedReplay(t *testing.T) {
	live := attachmentBaseline()
	writes := 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/rubrics") {
			writes++
			var fields map[string]map[string]json.RawMessage
			if e := json.NewDecoder(r.Body).Decode(&fields); e != nil {
				t.Error(e)
			}
			if string(fields["rubric"]["skip_updating_points_possible"]) != "true" || string(fields["rubric_association"]["use_for_grading"]) != "false" || string(fields["rubric_association"]["association_id"]) != "10" {
				t.Error("unsafe rubric association fields")
			}
			installFixtureRubric(&live)
			live.Points = json.Number("20.000")
			live.UpdatedAt = "later"
			_, _ = w.Write([]byte(`{"rubric":{"id":20},"rubric_association":{"id":30}}`))
			return
		}
		if r.Method != "GET" {
			t.Errorf("attachment wrote assignment settings: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(live)
	})
	base := live
	path := attachmentDraft(t, s, base)
	report, e := s.Push(context.Background(), path)
	if e != nil {
		t.Fatal(e)
	}
	if report.Status != "completed" || writes != 1 || report.Resources == nil || report.Resources.RubricID != 20 || report.Resources.RubricAssociationID != 30 || !live.Published || live.Description != base.Description || !sameDate(live.DueAt, base.DueAt) || decimalNumber(live.Points).Cmp(domain.MustDecimal("20")) != 0 {
		t.Fatalf("attachment changed item or lost IDs: %+v", report)
	}
	before := s.Client.Requests()
	live.Points = json.Number("90")
	if _, e = s.Push(context.Background(), path); e != nil {
		t.Fatal(e)
	}
	if writes != 1 || s.Client.Requests() != before {
		t.Fatal("completed attachment replay contacted Canvas")
	}
}
func decimalNumber(n json.Number) domain.Decimal { return domain.MustDecimal(n.String()) }

func TestNewQuizPullsForRubricAttachmentOnly(t *testing.T) {
	live := attachmentBaseline()
	live.NewQuiz = true
	live.SubmissionTypes = []string{"external_tool"}
	live.Points = json.Number("15")
	writes := 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/rubrics") {
			writes++
			installFixtureRubric(&live)
			_, _ = w.Write([]byte(`{"rubric":{"id":20},"rubric_association":{"id":30}}`))
			return
		}
		if r.Method != "GET" {
			t.Errorf("New Quiz attachment wrote assignment settings: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(live)
	})
	path, e := s.PullItem(context.Background(), "TEST", "10")
	if e != nil {
		t.Fatal(e)
	}
	var doc domain.AssignmentDocument
	body, e := workspace.ReadDocument(path, &doc)
	if e != nil {
		t.Fatal(e)
	}
	doc.Title = "Renamed"
	if e = workspace.WriteDocument(path, doc, body); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Push(context.Background(), path); e == nil || !strings.Contains(e.Error(), "New Quiz") {
		t.Fatalf("New Quiz update was not refused: %v", e)
	}
	doc.Title = live.Name
	doc.Operation = "attach-rubric"
	// Matches the fixture rubric Canvas reports back after the POST.
	doc.Rubric = &domain.Rubric{Title: "Evidence", Criteria: []domain.Criterion{{ID: "draft-quality", Description: "Evidence", Points: domain.MustDecimal("100.0"), Ratings: []domain.Rating{{ID: "draft-full", Description: "Complete", Points: domain.MustDecimal("100")}, {ID: "draft-empty", Description: "Missing", Points: domain.MustDecimal("0")}}}}}
	if e = workspace.WriteDocument(path, doc, body); e != nil {
		t.Fatal(e)
	}
	report, e := s.Push(context.Background(), path)
	if e != nil {
		t.Fatal(e)
	}
	if report.Status != "completed" || writes != 1 || report.Resources.RubricID != 20 {
		t.Fatalf("New Quiz rubric not attached: %+v", report)
	}
}

func TestAttachRubricUncertainResponseRecoversGeneratedIDsWithoutReplay(t *testing.T) {
	for _, visible := range []bool{true, false} {
		t.Run(map[bool]string{true: "visible", false: "absent"}[visible], func(t *testing.T) {
			live := attachmentBaseline()
			writes := 0
			s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "POST" {
					writes++
					if visible {
						installFixtureRubric(&live)
					}
					w.WriteHeader(503)
					return
				}
				_ = json.NewEncoder(w).Encode(live)
			})
			path := attachmentDraft(t, s, live)
			first, e := s.Push(context.Background(), path)
			if e != nil {
				t.Fatal(e)
			}
			if first.Status != "uncertain" {
				t.Fatalf("ambiguous attachment lost: %+v", first)
			}
			second, e := s.Push(context.Background(), path)
			if e != nil {
				t.Fatal(e)
			}
			if writes != 1 {
				t.Fatal("uncertain attachment replayed")
			}
			if visible {
				if second.Status != "completed" || second.Resources.RubricID != 20 || second.Resources.RubricAssociationID != 30 {
					t.Fatalf("exact live association not recovered: %+v", second)
				}
			} else if second.Status != "uncertain" {
				t.Fatalf("absent lookup treated as failed create: %+v", second)
			}
		})
	}
}

func TestAttachRubricChecksRelevantAssignmentChanges(t *testing.T) {
	for _, change := range []string{"existing", "points", "directions", "title", "published"} {
		t.Run(change, func(t *testing.T) {
			live := attachmentBaseline()
			base := live
			writes := 0
			s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" {
					writes++
					installFixtureRubric(&live)
					_, _ = w.Write([]byte(`{"rubric":{"id":20},"rubric_association":{"id":30}}`))
					return
				}
				_ = json.NewEncoder(w).Encode(live)
			})
			path := attachmentDraft(t, s, base)
			switch change {
			case "existing":
				installFixtureRubric(&live)
			case "points":
				live.Points = json.Number("50")
			case "directions":
				live.Description = "Teacher changed academic requirements"
			case "title":
				live.Name = "Teacher renamed it"
			case "published":
				live.Published = false
			}
			report, e := s.Push(context.Background(), path)
			if e != nil {
				t.Fatal(e)
			}
			if change == "title" || change == "published" || change == "directions" {
				if report.Status != "completed" || writes != 1 || (change == "title" && live.Name != "Teacher renamed it") || (change == "published" && live.Published) {
					t.Fatalf("irrelevant live change blocked or overwritten: %+v", report)
				}
			} else if report.Status != "held" || writes != 0 {
				t.Fatalf("meaningful live change was overwritten: %+v", report)
			}
		})
	}
}

func TestAttachRubricMalformedDraft(t *testing.T) {
	live := attachmentBaseline()
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) { t.Error("invalid/unverified attachment reached Canvas") })
	path := attachmentDraft(t, s, live)
	var d domain.AssignmentDocument
	body, e := workspace.ReadDocument(path, &d)
	if e != nil {
		t.Fatal(e)
	}
	for _, change := range []func(*domain.AssignmentDocument){
		func(d *domain.AssignmentDocument) { d.Points = domain.MustDecimal("100") },
		func(d *domain.AssignmentDocument) { d.Rubric.UseForGrading = true },
		func(d *domain.AssignmentDocument) { d.Observed.Rubric = d.Rubric },
		func(d *domain.AssignmentDocument) { d.Rubric.Title = "" },
		func(d *domain.AssignmentDocument) { d.Rubric.ID = 20 },
	} {
		var copy domain.AssignmentDocument
		if _, e = workspace.ReadDocument(path, &copy); e != nil {
			t.Fatal(e)
		}
		change(&copy)
		if e = domain.ValidateAssignment(copy, body); e == nil {
			t.Fatal("malformed attachment accepted")
		}
	}
}

func TestAttachRubricChangedScaleAfterWriteRecordsIDsAndDoesNotRepair(t *testing.T) {
	live := attachmentBaseline()
	writes := 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			writes++
			installFixtureRubric(&live)
			live.Points = json.Number("100")
			_, _ = w.Write([]byte(`{"rubric":{"id":20},"rubric_association":{"id":30}}`))
			return
		}
		if r.Method != "GET" {
			t.Error("assignment repair write attempted")
		}
		_ = json.NewEncoder(w).Encode(live)
	})
	path := attachmentDraft(t, s, live)
	report, e := s.Push(context.Background(), path)
	if e != nil {
		t.Fatal(e)
	}
	if report.Status != "held" || writes != 1 || report.Resources == nil || report.Resources.RubricID != 20 || report.Resources.Complete {
		t.Fatalf("bad scale reported complete or lost IDs: %+v", report)
	}
	if _, e = s.Push(context.Background(), path); e != nil {
		t.Fatal(e)
	}
	if writes != 1 {
		t.Fatal("attachment replayed after scale mismatch")
	}
}
