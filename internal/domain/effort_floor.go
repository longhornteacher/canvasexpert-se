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
