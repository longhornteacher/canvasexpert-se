package workflow

import (
	"math/big"
	"net/http"
	"strings"
	"testing"
)

func TestSpreadLineAndPercentBands(t *testing.T) {
	var values []*big.Rat
	for _, n := range []int64{6, 12, 14, 15, 15, 17, 18, 20} {
		values = append(values, big.NewRat(n, 1))
	}
	if got := spreadLine(values); got != "median 15; middle half 14–17; lowest 6, highest 20" {
		t.Fatalf("spread: %q", got)
	}
	if got := percentBands(values, big.NewRat(20, 1)); got != "90–100%: 2, 80s: 1, 70s: 3, 60s: 1, below 60%: 1" {
		t.Fatalf("bands: %q", got)
	}
	if got := spreadLine(values[:2]); got != "median 9; lowest 6, highest 12" {
		t.Fatalf("two scores: %q", got)
	}
}

func TestResultsSpreadOrdinaryAssignment(t *testing.T) {
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
	path := writeNoRubricResults(t, s, overallBasis())
	got, err := ResultsSpread(s.Root, path)
	if err != nil || !strings.Contains(got, "out of 20): 1 scored, 0 held") || !strings.Contains(got, "median 12; lowest 12, highest 12") || !strings.Contains(got, "60s: 1") {
		t.Fatalf("spread: %q %v", got, err)
	}
}

func TestResultsSpreadNewQuizUsesItemPoints(t *testing.T) {
	s, path, _, _, _, _ := itemScoreFixture(t, "15", false, "")
	got, err := ResultsSpread(s.Root, path)
	if err != nil || !strings.Contains(got, "Q1 Essay (out of 15): median 12; lowest 12, highest 12") || strings.Contains(got, "by percent") {
		t.Fatalf("spread: %q %v", got, err)
	}
}
