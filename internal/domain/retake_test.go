package domain

import "testing"

func TestRetakeTarget(t *testing.T) {
	floor := MustDecimal("40")
	cases := []struct {
		name                              string
		points, entered, earned, possible string
		genuine                           bool
		want                              string
		raise, capped                     bool
	}{
		{"capped", "100", "52", "88", "100", false, "70", true, true},
		{"never lowers", "100", "65", "54", "100", false, "65", false, false},
		{"floor inside the formula", "100", "25", "10", "100", true, "40", true, false},
		{"no floor when not genuine", "100", "25", "10", "100", false, "25", false, false},
		{"scaled to the original's points", "30", "12", "18", "20", false, "21", true, true},
		{"fractional points", "37.5", "10", "11", "20", false, "20.63", true, false},
		{"under the cap and above the original", "100", "52", "60", "100", false, "60", true, false},
	}
	for _, c := range cases {
		got := RetakeTarget(MustDecimal(c.points), MustDecimal(c.entered), MustDecimal(c.earned), MustDecimal(c.possible), 70, &floor, c.genuine)
		if got.Target.Cmp(MustDecimal(c.want)) != 0 || got.Raise != c.raise || got.Capped != c.capped {
			t.Errorf("%s: got %+v, want %s raise=%v capped=%v", c.name, got, c.want, c.raise, c.capped)
		}
	}
	if got := RetakeTarget(MustDecimal("100"), MustDecimal("25"), MustDecimal("10"), MustDecimal("100"), 70, nil, true); got.Raise {
		t.Error("a genuine judgment with no course floor must not raise")
	}
}
