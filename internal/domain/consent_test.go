package domain

import "testing"

func TestHasConsentTag(t *testing.T) {
	for name, want := range map[string]bool{"Essay 3 (CE)": true, "(ce) quiz": true, "Essay (C E)": false, "CE Essay": false, "": false} {
		if HasConsentTag(name) != want {
			t.Errorf("HasConsentTag(%q) = %v", name, !want)
		}
	}
}
