package localui

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// driveDrafts runs the real drafts.js against a minimal fake DOM under Node.
const driveDrafts = `
const fs = require("fs"), vm = require("vm");
const mk = (name, type, value, checked) => ({ name, type, value, checked });
const fields = [mk("aliases", "checkbox", "on", false), mk("voice", "textarea", "saved voice", false)];
let notice = false;
const root = {
  tagName: "MAIN",
  dataset: { draftKey: "k", draftFields: "aliases voice", draftGroups: "prefs:aliases grading:voice", clearDraft: "grading" },
  querySelectorAll: () => fields,
  querySelector: () => null,
  firstChild: { parentNode: { insertBefore: () => { notice = true; } } },
  addEventListener: () => {},
};
const store = { k: JSON.stringify({ aliases: true, voice: "draft voice" }) };
global.document = { querySelector: () => root, createElement: () => ({ append() {}, addEventListener() {}, setAttribute() {} }) };
global.sessionStorage = { getItem: k => store[k] ?? null, setItem: (k, v) => { store[k] = v; }, removeItem: k => { delete store[k]; } };
global.location = { href: "http://127.0.0.1/settings?section=preferences&clear=grading" };
global.history = { replaceState: () => {} };
vm.runInThisContext(fs.readFileSync(process.argv[2], "utf8"));
console.log(JSON.stringify({ stored: JSON.parse(store.k || "{}"), voice: fields[1].value, aliases: fields[0].checked, notice }));
`

func TestDraftsScriptClearsOnlyTheSavedGroup(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed; skipping the drafts.js smoke test")
	}
	script := filepath.Join(t.TempDir(), "drive.js")
	if err := os.WriteFile(script, []byte(driveDrafts), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(node, script, filepath.Join("assets", "drafts.js")).CombinedOutput()
	if err != nil {
		t.Fatalf("drafts.js failed: %v\n%s", err, out)
	}
	var got struct {
		Stored  map[string]any
		Voice   string
		Aliases bool
		Notice  bool
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("unexpected output %q", out)
	}
	if _, kept := got.Stored["voice"]; kept || got.Stored["aliases"] != true {
		t.Errorf("saving the grading form should clear only its group, stored=%v", got.Stored)
	}
	if got.Voice != "saved voice" || !got.Aliases || !got.Notice {
		t.Errorf("voice=%q aliases=%v notice=%v; want saved voice kept, other group's draft restored with a notice", got.Voice, got.Aliases, got.Notice)
	}
}
