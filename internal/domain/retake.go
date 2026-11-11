package domain

import "math/big"

// RetakeResult is what a tutorial retake does to its original's entered grade.
type RetakeResult struct {
	Target Decimal // the grade to enter, never below the current entered grade
	Raise  bool    // Target is above the current entered grade
	Capped bool    // the cap, not the retake score, set the candidate
}

// RetakeTarget applies a tutorial retake to its original, in exact arithmetic
// rounded once at the end:
//
//	retake_percent   = max(R, floor) when the written work was a genuine try, else R
//	candidate_points = P * min(retake_percent, cap) / 100
//	target           = max(G, candidate_points)
//
// P is the original's points, G its entered grade before any late deduction,
// R the retake's earned percent, and capPercent the most a retake can add.
// floor may be nil. The result never lowers G.
func RetakeTarget(points, entered Decimal, retakeEarned, retakePossible Decimal, capPercent int, floor *Decimal, genuine bool) RetakeResult {
	hundred := big.NewRat(100, 1)
	percent := new(big.Rat)
	if retakePossible.Sign() > 0 {
		percent.Mul(new(big.Rat).Quo(retakeEarned.Rat(), retakePossible.Rat()), hundred)
	}
	if genuine && floor != nil && percent.Cmp(floor.Rat()) < 0 {
		percent = new(big.Rat).Set(floor.Rat())
	}
	limit := big.NewRat(int64(capPercent), 1)
	capped := percent.Cmp(limit) > 0
	if capped {
		percent = limit
	}
	candidate := new(big.Rat).Quo(new(big.Rat).Mul(points.Rat(), percent), hundred)
	if candidate.Cmp(entered.Rat()) <= 0 {
		return RetakeResult{Target: entered, Capped: capped}
	}
	return RetakeResult{Target: RoundRat(candidate, 2), Raise: true, Capped: capped}
}
