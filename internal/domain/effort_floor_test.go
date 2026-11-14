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

func TestQuizFloorRaisesOnlyByTheShortfall(t *testing.T) {
	d := MustDecimal
	// 35 total (30 earned + 5 existing adjustment), minimum 50 of 100: add 15, keep the 5.
	if min, adj := QuizFloor(d("100"), d("35"), d("5"), d("50")); min.String() != "50" || adj.String() != "20" {
		t.Fatalf("min %s adj %s", min, adj)
	}
	// Recomputed from a total that already holds the raise: nothing more.
	if _, adj := QuizFloor(d("100"), d("50"), d("20"), d("50")); adj.String() != "20" {
		t.Fatalf("retry changed the adjustment: %s", adj)
	}
	// Above the floor: no reduction.
	if _, adj := QuizFloor(d("100"), d("70"), d("0"), d("50")); adj.String() != "0" {
		t.Fatalf("adj %s", adj)
	}
}
