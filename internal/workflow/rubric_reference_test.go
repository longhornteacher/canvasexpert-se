package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

type referenceFixture struct {
	rubrics                                                map[int64]rubricRead
	assignments                                            map[int64]Assignment
	creates, associations, assignmentCreates, topicCreates int
	failAssociation                                        bool
	noAssociationID                                        bool
	uncertainCreate, uncertainAssociation                  bool
	lastWrite                                              bool
	readsAfterAssociation                                  int
}

func newReferenceFixture() *referenceFixture {
	a := attachmentBaseline()
	criteria := fixtureAssignment().Rubric
	r := rubricRead{ID: 20, Title: "Evidence", Points: "100", Data: &criteria}
	return &referenceFixture{rubrics: map[int64]rubricRead{20: r}, assignments: map[int64]Assignment{10: a}}
}

func (f *referenceFixture) handler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" && f.lastWrite {
			f.readsAfterAssociation++
		}
		base := "/api/v1/courses/1/"
		switch {
		case r.Method == "GET" && r.URL.Path == base+"rubrics":
			all := []rubricRead{}
			for _, rubric := range f.rubrics {
				all = append(all, rubric)
			}
			sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })
			json.NewEncoder(w).Encode(all)
		case r.Method == "GET" && strings.HasPrefix(r.URL.Path, base+"rubrics/"):
			id, _ := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, base+"rubrics/"), 10, 64)
			rubric, ok := f.rubrics[id]
			if !ok {
				http.NotFound(w, r)
				return
			}
			json.NewEncoder(w).Encode(rubric)
		case r.Method == "POST" && r.URL.Path == base+"rubrics":
			f.creates++
			var payload struct {
				Rubric struct {
					Title    string `json:"title"`
					FreeForm bool   `json:"free_form_criterion_comments"`
					Criteria map[string]struct {
						Points          json.Number       `json:"points"`
						Description     string            `json:"description"`
						LongDescription string            `json:"long_description"`
						Range           bool              `json:"criterion_use_range"`
						Ratings         map[string]Rating `json:"ratings"`
					} `json:"criteria"`
				} `json:"rubric"`
				Association struct {
					ID      int64  `json:"association_id"`
					Type    string `json:"association_type"`
					Purpose string `json:"purpose"`
					Use     bool   `json:"use_for_grading"`
				} `json:"rubric_association"`
			}
			if e := json.NewDecoder(r.Body).Decode(&payload); e != nil {
				t.Error(e)
			}
			if payload.Association.Type != "Course" || payload.Association.Purpose != "bookmark" || payload.Association.ID != 1 || payload.Association.Use {
				t.Errorf("unsafe definition create: %+v", payload.Association)
			}
			criteria := []Criterion{}
			for i := 0; i < len(payload.Rubric.Criteria); i++ {
				c := payload.Rubric.Criteria[strconv.Itoa(i)]
				criterion := Criterion{ID: fmt.Sprint("remote-c", i), Points: c.Points, Description: c.Description, LongDescription: c.LongDescription, Range: c.Range}
				for j := 0; j < len(c.Ratings); j++ {
					rating := c.Ratings[strconv.Itoa(j)]
					rating.ID = fmt.Sprint("remote-r", j)
					criterion.Ratings = append(criterion.Ratings, rating)
				}
				criteria = append(criteria, criterion)
			}
			id := int64(100 + f.creates)
			f.rubrics[id] = rubricRead{ID: id, Title: payload.Rubric.Title, FreeForm: payload.Rubric.FreeForm, Data: &criteria}
			if f.uncertainCreate {
				http.Error(w, "uncertain", 502)
				return
			}
			fmt.Fprintf(w, `{"rubric":{"id":%d},"rubric_association":{"id":9090}}`, id)
		case r.Method == "POST" && r.URL.Path == base+"rubric_associations":
			f.associations++
			var payload struct {
				Association struct {
					RubricID     int64  `json:"rubric_id"`
					AssignmentID int64  `json:"association_id"`
					Type         string `json:"association_type"`
					Purpose      string `json:"purpose"`
					Use          *bool  `json:"use_for_grading"`
				} `json:"rubric_association"`
			}
			json.NewDecoder(r.Body).Decode(&payload)
			a := payload.Association
			if a.Type != "Assignment" || a.Purpose != "grading" || a.Use == nil || *a.Use {
				t.Errorf("unsafe association: %+v", a)
			}
			if f.failAssociation {
				http.Error(w, `{"errors":"rejected"}`, 400)
				return
			}
			rubric, ok := f.rubrics[a.RubricID]
			if !ok {
				t.Error("associated unknown rubric")
				http.NotFound(w, r)
				return
			}
			assignment := f.assignments[a.AssignmentID]
			assignment.Rubric = *rubric.Data
			assignment.RubricSettings.ID = rubric.ID
			assignment.RubricSettings.Title = rubric.Title
			assignment.RubricSettings.FreeForm = rubric.FreeForm
			no := false
			assignment.RubricAssociation = &RubricAssociation{ID: 30 + int64(f.associations), RubricID: rubric.ID, AssociationID: a.AssignmentID, AssociationType: "Assignment", UseForGrading: &no}
			f.assignments[a.AssignmentID] = assignment
			f.lastWrite = true
			if f.uncertainAssociation {
				http.Error(w, "uncertain", 502)
				return
			}
			if f.noAssociationID {
				fmt.Fprint(w, `{}`)
			} else {
				fmt.Fprintf(w, `{"id":%d}`, 30+f.associations)
			}
		case r.Method == "GET" && strings.HasPrefix(r.URL.Path, base+"assignments/"):
			id, _ := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, base+"assignments/"), 10, 64)
			a, ok := f.assignments[id]
			if !ok {
				http.NotFound(w, r)
				return
			}
			json.NewEncoder(w).Encode(a)
		case r.Method == "POST" && r.URL.Path == base+"assignments":
			f.assignmentCreates++
			var payload struct {
				Assignment Assignment `json:"assignment"`
			}
			json.NewDecoder(r.Body).Decode(&payload)
			a := payload.Assignment
			a.ID = 40 + int64(f.assignmentCreates)
			a.CourseID = 1
			a.HTMLURL = "https://canvas.example.edu/assignment"
			f.assignments[a.ID] = a
			json.NewEncoder(w).Encode(a)
		case r.Method == "POST" && r.URL.Path == base+"discussion_topics":
			f.topicCreates++
			fmt.Fprint(w, `{"id":77,"assignment_id":10,"title":"Discussion","message":"<p>Respond</p>"}`)
		case r.Method == "GET" && r.URL.Path == base+"discussion_topics/77":
			fmt.Fprint(w, `{"id":77,"assignment_id":10,"title":"Discussion","message":"<p>Respond</p>"}`)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}
}

func referenceAttachment(t *testing.T, s *Service, a Assignment, ref domain.Rubric) string {
	t.Helper()
	path := attachmentDraft(t, s, a)
	var d domain.AssignmentDocument
	body, e := workspace.ReadDocument(path, &d)
	if e != nil {
		t.Fatal(e)
	}
	d.Rubric = &ref
	if e = workspace.WriteDocument(path, &d, body); e != nil {
		t.Fatal(e)
	}
	return path
}

func saveLibrary(t *testing.T, root, title string, points string) string {
	t.Helper()
	path := filepath.Join(root, "context", "rubrics", "SCR.md")
	d := domain.RubricLibraryDocument{Schema: 1, Kind: "rubric-library", Rubric: domain.PortableRubric{Title: title, Criteria: []domain.Criterion{{ID: "quality", Description: "Quality", Points: domain.MustDecimal(points)}}}}
	if e := workspace.WriteDocument(path, &d, "Teacher description.\n"); e != nil {
		t.Fatal(e)
	}
	return path
}

func TestRubricReferenceAttachesExistingAssignmentAndNewQuizWithoutReadback(t *testing.T) {
	for _, newQuiz := range []bool{false, true} {
		t.Run(fmt.Sprint(newQuiz), func(t *testing.T) {
			f := newReferenceFixture()
			a := f.assignments[10]
			a.NewQuiz = newQuiz
			if newQuiz {
				a.SubmissionTypes = []string{"external_tool"}
			}
			f.assignments[10] = a
			s, _ := workflowFixture(t, f.handler(t))
			path := referenceAttachment(t, s, a, domain.Rubric{ID: 20})
			r, e := s.Push(context.Background(), path)
			if e != nil || r.Status != "completed" || f.creates != 0 || f.associations != 1 || f.readsAfterAssociation != 0 {
				t.Fatalf("reference attach: %+v %v creates=%d associations=%d reads=%d", r, e, f.creates, f.associations, f.readsAfterAssociation)
			}
			before := s.Client.Requests()
			if _, e = s.Push(context.Background(), path); e != nil || s.Client.Requests() != before {
				t.Fatalf("replayed accepted attachment: %v", e)
			}
		})
	}
}

func TestRubricLibraryCreateResumeReuseAndVersion(t *testing.T) {
	f := newReferenceFixture()
	f.failAssociation = true
	s, _ := workflowFixture(t, f.handler(t))
	saveLibrary(t, s.Root, "SCR", "4")
	path := referenceAttachment(t, s, f.assignments[10], domain.Rubric{Library: "SCR"})
	r, e := s.Push(context.Background(), path)
	if e != nil || r.Status != "failed" || f.creates != 1 || f.associations != 1 {
		t.Fatalf("create/attach failure: %+v %v", r, e)
	}
	f.failAssociation = false
	r, e = s.Push(context.Background(), path)
	if e != nil || r.Status != "completed" || f.creates != 1 || f.associations != 2 {
		t.Fatalf("resume recreated known rubric: %+v %v creates=%d", r, e, f.creates)
	}
	for _, version := range []string{"4", "5"} {
		saveLibrary(t, s.Root, "SCR", version)
		d := domain.AssignmentDocument{Schema: 1, Kind: "assignment", Operation: "create", Course: "TEST", Title: "Next", Points: domain.MustDecimal("20"), SubmissionTypes: []string{"online_text_entry"}, Rubric: &domain.Rubric{Library: "SCR"}}
		p := filepath.Join(s.Root, "new-"+version+".md")
		if e = workspace.WriteDocument(p, &d, "Explain."); e != nil {
			t.Fatal(e)
		}
		r, e = s.Push(context.Background(), p)
		if e != nil || r.Status != "completed" {
			t.Fatalf("create using library: %+v %v", r, e)
		}
		expected := 1
		if version == "5" {
			expected = 2
		}
		if f.creates != expected {
			t.Fatalf("wrong rubric creation count %d", f.creates)
		}
		if version == "5" && !strings.Contains(r.Message, "new course rubric version") {
			t.Fatalf("new version unreported: %+v", r)
		}
	}
	if f.assignments[10].RubricSettings.ID != 101 {
		t.Fatal("library revision replaced an earlier rubric")
	}
}

func TestRubricReferenceGradedDiscussionAndChangedLibraryHold(t *testing.T) {
	f := newReferenceFixture()
	s, _ := workflowFixture(t, f.handler(t))
	d := gradedDiscussionDoc(false)
	d.GroupCategoryID = 0
	d.Title = "Discussion"
	d.Assignment.Rubric = &domain.Rubric{ID: 20}
	p := filepath.Join(s.Root, "discussion.md")
	if e := workspace.WriteDocument(p, &d, "<p>Respond</p>"); e != nil {
		t.Fatal(e)
	}
	r, e := s.PushDiscussion(context.Background(), p)
	if e != nil || r.Status != "completed" || f.topicCreates != 1 || f.associations != 1 || f.creates != 0 {
		t.Fatalf("discussion reference: %+v %v", r, e)
	}
	f = newReferenceFixture()
	f.failAssociation = true
	s, _ = workflowFixture(t, f.handler(t))
	library := saveLibrary(t, s.Root, "SCR", "4")
	p = referenceAttachment(t, s, f.assignments[10], domain.Rubric{Library: "SCR"})
	if _, e = s.Push(context.Background(), p); e != nil {
		t.Fatal(e)
	}
	before := f.associations
	if e = os.WriteFile(library, []byte("changed library"), 0600); e != nil {
		t.Fatal(e)
	}
	r2, e := s.Push(context.Background(), p)
	if e != nil || r2.Status != "held" || f.associations != before || f.creates != 1 {
		t.Fatalf("changed library not held: %+v %v", r2, e)
	}
}

func TestRubricLibraryUsesSharedAuthorityAndNoStaleFallback(t *testing.T) {
	f := newReferenceFixture()
	s, _ := workflowFixture(t, f.handler(t))
	saveLibrary(t, s.Root, "Stale local", "99")
	shared := t.TempDir()
	saveLibrary(t, shared, "Shared rubric", "4")
	b, _ := json.Marshal(map[string]any{"sync_root": shared, "data_version": 1})
	if e := workspace.WriteFile(filepath.Join(s.Root, ".cse", "rules-location.json"), b, 0600); e != nil {
		t.Fatal(e)
	}
	p := referenceAttachment(t, s, f.assignments[10], domain.Rubric{Library: "SCR"})
	r, e := s.Push(context.Background(), p)
	if e != nil || r.Status != "completed" || f.rubrics[101].Title != "Shared rubric" {
		t.Fatalf("wrong authority: %+v %v", r, e)
	}
	// Removing just the shared entry must never revive the different local copy.
	if e = os.Remove(filepath.Join(shared, "context", "rubrics", "SCR.md")); e != nil {
		t.Fatal(e)
	}
	a := attachmentBaseline()
	a.ID = 11
	f.assignments[11] = a
	p = referenceAttachment(t, s, a, domain.Rubric{Library: "SCR"})
	before := f.creates
	r, e = s.Push(context.Background(), p)
	if e != nil || r.Status != "held" || f.creates != before {
		t.Fatalf("used stale local library: %+v %v", r, e)
	}
}

func TestRubricReferencePreservesExistingAssociationAndAcceptsMissingResponseID(t *testing.T) {
	f := newReferenceFixture()
	s, _ := workflowFixture(t, f.handler(t))
	p := referenceAttachment(t, s, f.assignments[10], domain.Rubric{ID: 20})
	f.assignments[10] = fixtureAssignment()
	r, e := s.Push(context.Background(), p)
	if e != nil || r.Status != "held" || f.associations != 0 {
		t.Fatalf("existing association replaced: %+v %v", r, e)
	}
	f = newReferenceFixture()
	f.noAssociationID = true
	s, _ = workflowFixture(t, f.handler(t))
	p = referenceAttachment(t, s, f.assignments[10], domain.Rubric{ID: 20})
	r, e = s.Push(context.Background(), p)
	if e != nil || r.Status != "completed" || f.associations != 1 || f.readsAfterAssociation != 0 {
		t.Fatalf("accepted association depended on reply shape: %+v %v", r, e)
	}
}

func TestSharedRubricAssessmentUsesOrdinaryHonestPost(t *testing.T) {
	a := fixtureAssignment()
	other := a
	other.ID = 11
	live := fixtureSubmission(5)
	rubricWrites := 0
	definitionWrites := 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/submissions/5") {
			if r.Method == "PUT" {
				var payload struct {
					Rubric     map[string]RubricScore     `json:"rubric_assessment"`
					Submission map[string]json.RawMessage `json:"submission"`
				}
				json.NewDecoder(r.Body).Decode(&payload)
				if payload.Rubric != nil {
					rubricWrites++
					live.RubricAssessment = payload.Rubric
					if payload.Rubric["c1"].Points == nil || payload.Rubric["c1"].Points.String() != "60" {
						t.Errorf("wrong shared criterion assessment: %+v", payload.Rubric)
					}
				}
			}
			json.NewEncoder(w).Encode(live)
			return
		}
		if r.Method != "GET" {
			definitionWrites++
		}
		json.NewEncoder(w).Encode(a)
	})
	p := scoringFiles(t, s, 5)
	r, e := s.Post(context.Background(), p)
	if e != nil || len(r.Students) != 1 || r.Students[0].Status != "completed" || rubricWrites != 1 || definitionWrites != 0 || other.RubricSettings.ID != a.RubricSettings.ID {
		t.Fatalf("shared rubric post: %+v %v rubric=%d definition=%d", r, e, rubricWrites, definitionWrites)
	}
}

func TestRubricReferenceUncertainWritesAreRecoveredWithoutReplay(t *testing.T) {
	for _, stage := range []string{"create", "association"} {
		t.Run(stage, func(t *testing.T) {
			f := newReferenceFixture()
			f.uncertainCreate = stage == "create"
			f.uncertainAssociation = stage == "association"
			s, _ := workflowFixture(t, f.handler(t))
			saveLibrary(t, s.Root, "SCR", "4")
			p := referenceAttachment(t, s, f.assignments[10], domain.Rubric{Library: "SCR"})
			r, e := s.Push(context.Background(), p)
			if e != nil || r.Status != "uncertain" {
				t.Fatalf("uncertainty lost: %+v %v", r, e)
			}
			f.uncertainCreate, f.uncertainAssociation = false, false
			r, e = s.Push(context.Background(), p)
			if e != nil || r.Status != "completed" || f.creates != 1 || f.associations != 1 {
				t.Fatalf("uncertain write replayed: %+v %v definitions=%d associations=%d", r, e, f.creates, f.associations)
			}
		})
	}
}

func TestRubricLibraryReusesLowestIdenticalCourseID(t *testing.T) {
	f := newReferenceFixture()
	duplicate := f.rubrics[20]
	duplicate.ID = 21
	f.rubrics[21] = duplicate
	s, _ := workflowFixture(t, f.handler(t))
	definition, e := portableRubric(f.rubrics[20])
	if e != nil {
		t.Fatal(e)
	}
	d := domain.RubricLibraryDocument{Schema: 1, Kind: "rubric-library", Rubric: definition}
	if e = workspace.WriteDocument(filepath.Join(s.Root, "context", "rubrics", "SCR.md"), &d, ""); e != nil {
		t.Fatal(e)
	}
	p := referenceAttachment(t, s, f.assignments[10], domain.Rubric{Library: "SCR"})
	r, e := s.Push(context.Background(), p)
	if e != nil || r.Status != "completed" || r.Resources.RubricID != 20 || f.creates != 0 {
		t.Fatalf("duplicate definition not reused deterministically: %+v %v", r, e)
	}
}
