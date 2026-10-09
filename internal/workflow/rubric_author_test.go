package workflow

import (
	"encoding/json"
	"reflect"
	"testing"

	"canvasexpertse/internal/domain"
)

func TestRubricCreatePreservesDeclaredMaximumThroughCanvasRatings(t *testing.T) {
	for _, tc := range []struct {
		name    string
		maximum string
		ratings []domain.Rating
		want    []domain.Rating
	}{
		{"omitted", "4", nil, []domain.Rating{{Points: domain.MustDecimal("4"), Description: "4 points"}, {Points: domain.MustDecimal("0"), Description: "0 points"}}},
		{"zero-descriptive-criterion", "0", nil, []domain.Rating{{Points: domain.MustDecimal("0"), Description: "0 points"}}},
		{"partial-explicit-scale", "6", []domain.Rating{{ID: "partial", Points: domain.MustDecimal("3"), Description: "Teacher's partial level"}}, []domain.Rating{{Points: domain.MustDecimal("3"), Description: "Teacher's partial level"}, {Points: domain.MustDecimal("6"), Description: "6 points"}}},
		{"complete-explicit-scale", "4.5", []domain.Rating{{ID: "full", Points: domain.MustDecimal("4.5"), Description: "Teacher's full level"}, {ID: "empty", Points: domain.MustDecimal("0"), Description: "Teacher's empty level"}}, []domain.Rating{{Points: domain.MustDecimal("4.5"), Description: "Teacher's full level"}, {Points: domain.MustDecimal("0"), Description: "Teacher's empty level"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requested := domain.Rubric{Title: "Evidence", Criteria: []domain.Criterion{{ID: "draft", Description: "Evidence", LongDescription: "Teacher's criterion", Points: domain.MustDecimal(tc.maximum), Ratings: tc.ratings}}}
			before := append([]domain.Rating(nil), requested.Criteria[0].Ratings...)
			encoded, e := json.Marshal(rubricCreateFields(requested, 10))
			if e != nil {
				t.Fatal(e)
			}
			var wire struct {
				Rubric struct {
					Criteria map[string]struct {
						Points  domain.Decimal           `json:"points"`
						Ratings map[string]domain.Rating `json:"ratings"`
					} `json:"criteria"`
				} `json:"rubric"`
			}
			if e = json.Unmarshal(encoded, &wire); e != nil {
				t.Fatal(e)
			}
			criterion := wire.Rubric.Criteria["0"]
			if criterion.Points.Cmp(requested.Criteria[0].Points) != 0 || len(criterion.Ratings) != len(tc.want) {
				t.Fatalf("authoring lost declared scale or ratings: %s", encoded)
			}
			actual := requested
			actual.Criteria = append([]domain.Criterion(nil), requested.Criteria...)
			actual.Criteria[0].Ratings = nil
			actual.Criteria[0].Points = domain.MustDecimal("0")
			for key, rating := range criterion.Ratings {
				matched := false
				for _, want := range tc.want {
					if rating.Points.Cmp(want.Points) == 0 && rating.Description == want.Description && rating.LongDescription == want.LongDescription {
						matched = true
					}
				}
				if !matched {
					t.Fatalf("unexpected generated or changed teacher rating: %+v", rating)
				}
				// Canvas generates resource IDs and derives maxima from ratings.
				rating.ID = "canvas-" + key
				actual.Criteria[0].Ratings = append(actual.Criteria[0].Ratings, rating)
				if rating.Points.Cmp(actual.Criteria[0].Points) > 0 {
					actual.Criteria[0].Points = rating.Points
				}
			}
			if actual.Criteria[0].Points.Cmp(requested.Criteria[0].Points) != 0 || !sameRubricDefinition(actual, requested) {
				t.Fatalf("Canvas-derived definition did not confirm requested scale: %+v", actual)
			}
			if !reflect.DeepEqual(before, requested.Criteria[0].Ratings) {
				t.Fatal("authoring mutated the teacher's draft ratings")
			}
			actual.Criteria[0].Ratings = actual.Criteria[0].Ratings[:len(actual.Criteria[0].Ratings)-1]
			if sameRubricDefinition(actual, requested) {
				t.Fatal("missing live rating was hidden by intended-definition normalization")
			}
		})
	}
}
