package canvasexpertse

import (
	"strings"
	"testing"
)

func TestEngineFormatSections(t *testing.T) {
	for _, section := range []string{"assignment", "page", "module", "quiz", "inbox", "results"} {
		t.Run(section, func(t *testing.T) {
			text, err := EngineFormatSection(section)
			if err != nil || len(text) < 500 || section != "results" && strings.Contains(text, "### Student operations") {
				t.Fatalf("section %s: size=%d err=%v", section, len(text), err)
			}
			if !IsEngineReferenceTopic("formats-" + section) {
				t.Fatal("focused topic is unavailable")
			}
			if section == "assignment" && !strings.Contains(text, "copy-assignment") {
				t.Fatal("assignment copy contract is missing")
			}
			if section == "results" && (!strings.Contains(text, "### Student operations") || !strings.Contains(text, "### Scoring without a Canvas rubric") || !strings.Contains(text, "| `results` |") || strings.Contains(text, "## Quiz file (QuizForge)")) {
				t.Fatal("results contract is incomplete or carries other kinds")
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

func TestEngineExampleByName(t *testing.T) {
	for _, topic := range []string{"examples/synthetic-results-two", "examples/synthetic-results-two.md"} {
		text, err := EngineExample(topic)
		if err != nil || !strings.HasPrefix(text, "# synthetic-results-two.md") || strings.Contains(text, "synthetic-page-create") {
			t.Fatalf("%s: %v", topic, err)
		}
		if !IsEngineReferenceTopic(topic) {
			t.Fatalf("%s is not a reference topic", topic)
		}
	}
	for _, topic := range []string{"examples/", "examples/missing", "examples/../formats.md", "examples-synthetic-results-two"} {
		if IsEngineReferenceTopic(topic) {
			t.Fatalf("%s accepted", topic)
		}
	}
}
