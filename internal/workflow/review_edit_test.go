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

func editReviewFixture(t *testing.T, live Assignment) (*Service, string, domain.AssignmentDocument) {
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("review must not write: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(live)
	})
	base := fixtureAssignment()
	base.CourseID = 1
	snap, e := Snapshot(base, s.Client.Origin())
	if e != nil {
		t.Fatal(e)
	}
	doc := domain.AssignmentDocument{Schema: 1, Kind: "assignment", Operation: "update", Instance: snap.Instance, CourseID: 1, AssignmentID: 10, Course: "TEST", Title: "Evidence essay (revised)", Points: snap.Points, SubmissionTypes: snap.SubmissionTypes, Rubric: snap.Rubric, Observed: &snap}
	path := filepath.Join(s.Root, "edit.md")
	if e = workspace.WriteDocument(path, &doc, "<p>Explain your evidence with a quote.</p>"); e != nil {
		t.Fatal(e)
	}
	return s, path, doc
}

func TestEditReviewShowsLiveBeforeAndAfter(t *testing.T) {
	s, path, _ := editReviewFixture(t, fixtureAssignment())
	m, err := s.BuildEditReview(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if !m.AllOrNothing || m.Action != "Apply edit" || m.Cards[0].Hold != "" || len(m.Cards) != 3 {
		t.Fatalf("%+v", m.Cards)
	}
	var sawTitle, sawDirections bool
	for _, c := range m.Cards {
		if c.Alias == "Title" && c.Facts[0].Value == "Evidence essay" && c.Facts[1].Value == "Evidence essay (revised)" {
			sawTitle = true
		}
		if c.Alias == "Directions" && strings.Contains(c.Frames[0].HTML, "Explain your evidence.") && strings.Contains(c.Frames[1].HTML, "with a quote") {
			sawDirections = true
		}
	}
	if !sawTitle || !sawDirections {
		t.Fatalf("%+v", m.Cards)
	}
}

func TestEditReviewFlagsChangedLiveFieldAndNeverWrites(t *testing.T) {
	live := fixtureAssignment()
	live.Name = "Someone renamed this in Canvas"
	s, path, _ := editReviewFixture(t, live)
	m, err := s.BuildEditReview(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m.Cards[0].Hold, "Canvas changed") {
		t.Fatalf("%+v", m.Cards[0])
	}
}
