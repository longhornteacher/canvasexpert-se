package localui

import (
	"strconv"
	"testing"

	"canvasexpertse/internal/domain"
)

func TestLateWorkLimitBox(t *testing.T) {
	for text, want := range map[string]string{"": "nil", "0": "0", " 3 ": "3"} {
		got, sent, ok := parseCutoff(map[string][]string{"k": {text}}, "k")
		if !sent || !ok || (want == "nil") != (got == nil) || got != nil && strconv.Itoa(*got) != want {
			t.Fatalf("%q -> %v %v %v", text, got, sent, ok)
		}
	}
	for _, bad := range []string{"-1", "61", "three", "1.5"} {
		if _, _, ok := parseCutoff(map[string][]string{"k": {bad}}, "k"); ok {
			t.Fatalf("accepted %q", bad)
		}
	}
	if _, sent, ok := parseCutoff(map[string][]string{}, "k"); sent || !ok {
		t.Fatal("older form must keep the saved value")
	}
	three := 3
	p := withCutoff(&domain.LateWorkPolicy{CorrectionSchoolDays: 4}, &three)
	if p == nil || *p.LateCutoffSchoolDays != 3 || p.CorrectionSchoolDays != 4 {
		t.Fatalf("%+v", p)
	}
	if withCutoff(&domain.LateWorkPolicy{LateCutoffSchoolDays: &three}, nil) != nil {
		t.Fatal("clearing the only line should drop the policy")
	}
}
