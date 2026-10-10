package workflow

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"canvasexpertse/internal/domain"
)

type pointsFixture struct {
	items     []map[string]any
	submitted bool
	patches   []string
}

func (f *pointsFixture) handler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		switch {
		case r.Method == "PATCH" && strings.Contains(p, "/items/"):
			var body struct {
				Item struct {
					Points json.Number `json:"points_possible"`
				} `json:"item"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			id := p[strings.LastIndex(p, "/")+1:]
			f.patches = append(f.patches, id+"="+body.Item.Points.String())
			for _, it := range f.items {
				if it["id"] == id {
					it["points_possible"] = json.Number(body.Item.Points.String())
				}
			}
			_, _ = w.Write([]byte(`{}`))
		case r.Method != "GET":
			t.Errorf("unexpected write %s %s", r.Method, p)
		case strings.HasSuffix(p, "/items"):
			_ = json.NewEncoder(w).Encode(f.items)
		case strings.HasSuffix(p, "/submissions"):
			if f.submitted {
				_, _ = w.Write([]byte(`[{"id":1,"user_id":5,"submitted_at":"2026-10-10T12:00:00Z","workflow_state":"submitted"}]`))
				return
			}
			_, _ = w.Write([]byte(`[{"id":1,"user_id":5,"workflow_state":"unsubmitted"}]`))
		case strings.HasPrefix(p, "/api/quiz/v1/"):
			_, _ = w.Write([]byte(`{"id":"10","points_possible":100}`))
		default:
			a := fixtureAssignment()
			a.NewQuiz = true
			a.SubmissionTypes = []string{"external_tool"}
			a.Points = "100"
			_ = json.NewEncoder(w).Encode(a)
		}
	}
}

func newPointsFixture() *pointsFixture {
	return &pointsFixture{items: []map[string]any{
		{"id": "71", "position": 1, "points_possible": json.Number("1.0"), "entry_type": "Item", "entry": map[string]any{"interaction_type_slug": "essay", "item_body": "<p>Describe <b>dinner</b>.</p>"}},
		{"id": "72", "position": 2, "points_possible": json.Number("50"), "entry_type": "Item", "entry": map[string]any{"interaction_type_slug": "file-upload", "item_body": "<p>Upload it.</p>"}},
	}}
}

func TestQuizPointsChangesOnlyRequestedItemsAndReadsBack(t *testing.T) {
	f := newPointsFixture()
	s, _ := workflowFixture(t, f.handler(t))
	r, e := s.QuizPoints(context.Background(), "TEST", "10", map[int]domain.Decimal{1: domain.MustDecimal("50"), 2: domain.MustDecimal("50")})
	if e != nil {
		t.Fatal(e)
	}
	if r.Status != "completed" || r.Changed != 1 || len(f.patches) != 1 || f.patches[0] != "71=50" {
		t.Fatalf("wrong writes: %+v %v", r, f.patches)
	}
	if r.ItemTotal != "100" || r.QuizPoints != "100" || r.Items[0].Was != "1.0" || r.Items[0].Stem != "Describe dinner ." || r.Message != "" {
		t.Fatalf("wrong report: %+v", r)
	}
	if r, e = s.QuizPoints(context.Background(), "TEST", "10", map[int]domain.Decimal{2: domain.MustDecimal("10")}); e != nil || !strings.Contains(r.Message, "Items total 60 but the quiz is worth 100") {
		t.Fatalf("quiz total mismatch not reported: %+v %v", r, e)
	}
}

func TestQuizPointsHoldsOnceStudentsSubmitted(t *testing.T) {
	f := newPointsFixture()
	f.submitted = true
	s, _ := workflowFixture(t, f.handler(t))
	r, e := s.QuizPoints(context.Background(), "TEST", "10", map[int]domain.Decimal{1: domain.MustDecimal("50")})
	if e != nil || r.Status != "held" || len(f.patches) != 0 || !strings.Contains(r.Message, "regrade") {
		t.Fatalf("submitted quiz not held: %+v %v %v", r, e, f.patches)
	}
	if r, e = s.QuizPoints(context.Background(), "TEST", "10", nil); e != nil || r.Status != "listed" || len(r.Items) != 2 {
		t.Fatalf("listing failed: %+v %v", r, e)
	}
}

func TestQuizPointsRefusesUnknownPositionAndOrdinaryAssignment(t *testing.T) {
	f := newPointsFixture()
	s, _ := workflowFixture(t, f.handler(t))
	if _, e := s.QuizPoints(context.Background(), "TEST", "10", map[int]domain.Decimal{3: domain.MustDecimal("5")}); e == nil || !strings.Contains(e.Error(), "position 3") {
		t.Fatalf("unknown position accepted: %v", e)
	}
	plain, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) { _ = json.NewEncoder(w).Encode(fixtureAssignment()) })
	if _, e := plain.QuizPoints(context.Background(), "TEST", "10", nil); e == nil || !strings.Contains(e.Error(), "not a New Quiz") {
		t.Fatalf("ordinary assignment accepted: %v", e)
	}
	if len(f.patches) != 0 {
		t.Fatal("refused change wrote to Canvas")
	}
}
