package localui

import "testing"

func TestParseFloorAcceptsBlankOrPercent(t *testing.T) {
	for raw, want := range map[string]string{"": "", " 50 ": "50", "62.5%": "62.5", "100": "100"} {
		got, ok := parseFloor(raw)
		if !ok || floorText(got) != want {
			t.Errorf("parseFloor(%q) = %q, %v; want %q", raw, floorText(got), ok, want)
		}
	}
	for _, raw := range []string{"0", "-1", "101", "half"} {
		if _, ok := parseFloor(raw); ok {
			t.Errorf("parseFloor(%q) accepted", raw)
		}
	}
}
