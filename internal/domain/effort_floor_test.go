package domain

import "testing"

func TestEnteredGradeRaisesOnlyHonestAttemptsBelowFloor(t *testing.T) {
	half := MustDecimal("50")
	third := MustDecimal("33.333")
	for _, tc := range []struct {
		assessment, points string
		floor              *Decimal
		honest             bool
		want               string
	}{
		{"40", "100", &half, true, "50"},
		{"40", "100", &half, false, "40"},
		{"60", "100", &half, true, "60"},
		{"50", "100", &half, true, "50"},
		{"0", "7", &half, true, "3.5"},
		{"0", "10", &third, true, "3.33"},
		{"40", "100", nil, true, "40"},
	} {
		got := EnteredGrade(MustDecimal(tc.assessment), MustDecimal(tc.points), tc.floor, tc.honest)
		if got.String() != tc.want {
			t.Errorf("EnteredGrade(%s/%s, honest=%v) = %s, want %s", tc.assessment, tc.points, tc.honest, got, tc.want)
		}
	}
}

func TestValidateConfigRejectsOutOfRangeEffortFloor(t *testing.T) {
	for _, raw := range []string{"0", "-5", "100.5"} {
		f := MustDecimal(raw)
		c := CourseConfig{Schema: 1, Instance: "https://canvas.example.edu", Courses: []Course{{Nickname: "ELA", ID: 1, EffortFloor: &f}}}
		if ValidateConfig(c) == nil {
			t.Errorf("floor %s accepted", raw)
		}
	}
	f := MustDecimal("100")
	if e := ValidateConfig(CourseConfig{Schema: 1, Instance: "https://canvas.example.edu", Courses: []Course{{Nickname: "ELA", ID: 1, EffortFloor: &f}}}); e != nil {
		t.Fatal(e)
	}
}
