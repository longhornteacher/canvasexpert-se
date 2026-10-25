package canvasexpertse

import (
	"strings"
	"testing"
)

func TestEngineFormatSections(t *testing.T) {
	for _, section := range []string{"assignment", "page", "module", "quiz"} {
		t.Run(section, func(t *testing.T) {
			text, err := EngineFormatSection(section)
			if err != nil || len(text) < 500 || strings.Contains(text, "### Student operations") {
				t.Fatalf("section %s: size=%d err=%v", section, len(text), err)
			}
			if !IsEngineReferenceTopic("formats-" + section) {
				t.Fatal("focused topic is unavailable")
			}
			if section == "assignment" && !strings.Contains(text, "copy-assignment") {
				t.Fatal("assignment copy contract is missing")
			}
			if section == "page" && strings.Contains(text, "copies an ordinary assignment") {
				t.Fatal("assignment copy contract leaked into page reference")
			}
		})
	}
	if _, err := EngineFormatSection("missing"); err == nil {
		t.Fatal("unknown section accepted")
	}
}
