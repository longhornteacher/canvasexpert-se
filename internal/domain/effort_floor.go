package domain

import "math/big"

// EnteredGrade is the grade to enter in Canvas: the honest assessment, or the
// course's effort floor (a percent of points) when the agent judged the work an
// honest attempt and the assessment falls below that floor. The assessment, the
// rubric and the comment's score lines are never changed by the floor.
func EnteredGrade(assessment, points Decimal, floor *Decimal, honestEffort bool) Decimal {
	if floor == nil || !honestEffort || !points.IsSet() || points.Sign() <= 0 {
		return assessment
	}
	minimum := RoundRat(new(big.Rat).Quo(new(big.Rat).Mul(points.Rat(), floor.Rat()), big.NewRat(100, 1)), 2)
	if assessment.Cmp(minimum) < 0 {
		return minimum
	}
	return assessment
}

// QuizFloor is the whole-quiz honest-effort minimum for a New Quiz. projected
// is the quiz total the item scores will give before any floor adjustment
// (auto-scored and written items plus the teacher's existing adjustment),
// possible the quiz total, existing the adjustment already in the result. It
// returns the minimum and the adjustment to enter: the existing one, raised
// by only the shortfall. The result is never below existing, and recomputing
// from a total that already includes the raise adds nothing more.
func QuizFloor(possible, projected, existing, floor Decimal) (minimum, adjustment Decimal) {
	minimum = RoundRat(new(big.Rat).Quo(new(big.Rat).Mul(possible.Rat(), floor.Rat()), big.NewRat(100, 1)), 2)
	if projected.Cmp(minimum) >= 0 {
		return minimum, existing
	}
	return minimum, RoundRat(new(big.Rat).Add(existing.Rat(), new(big.Rat).Sub(minimum.Rat(), projected.Rat())), 2)
}
