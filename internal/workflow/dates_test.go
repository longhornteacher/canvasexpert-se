package workflow

import "testing"

func TestCanvasDateChicago(t *testing.T) {
	for _, tc := range []struct {
		in, want string
		bad      bool
	}{
		{"2026-10-08T15:30", "2026-10-08T20:30:00Z", false},
		{"2026-12-08T15:30", "2026-12-08T21:30:00Z", false},
		{"2026-11-01T01:30", "", true},
		{"2026-03-08T02:30", "", true},
		{"2026-11-01T01:30:00-05:00", "2026-11-01T06:30:00Z", false},
	} {
		got, err := CanvasDate(&tc.in, "America/Chicago")
		if tc.bad {
			if err == nil {
				t.Errorf("%s should fail", tc.in)
			}
			continue
		}
		if err != nil || got == nil || *got != tc.want {
			t.Errorf("%s got %v, %v", tc.in, got, err)
		}
	}
	if got, err := CanvasDate(nil, ""); err != nil || got != nil {
		t.Fatal("omitted date was invented")
	}
}
