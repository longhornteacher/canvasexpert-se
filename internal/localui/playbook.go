package localui

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	embedded "canvasexpertse"
	"canvasexpertse/internal/workspace"
)

// playbookRow is one playbook or context seed file as the workspace holds it.
type playbookRow struct {
	Path, State string
	Replaceable bool
}

// playbookSeedPaths lists the seed files the teacher can compare and refresh:
// AGENTS.md, CLAUDE.md, playbooks/* and context/* (the context). README.md,
// cse.cmd and .claude are glue.
func playbookSeedPaths() []string {
	var out []string
	_ = fs.WalkDir(embedded.WorkspaceTemplate, "workspace-template", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel := strings.TrimPrefix(name, "workspace-template/")
		if rel == "AGENTS.md" || rel == "CLAUDE.md" || strings.HasPrefix(rel, "playbooks/") || strings.HasPrefix(rel, "context/") {
			out = append(out, rel)
		}
		return nil
	})
	sort.Strings(out)
	return out
}

func isSettingsPath(rel string) bool { return strings.HasPrefix(rel, "context/") }

func seedBytes(rel string) ([]byte, error) {
	return fs.ReadFile(embedded.WorkspaceTemplate, path.Join("workspace-template", rel))
}

// playbookRows compares each file with this version's starter copy.
func playbookRows(root string) []playbookRow {
	var rows []playbookRow
	for _, rel := range playbookSeedPaths() {
		row := playbookRow{Path: rel}
		seed, err := seedBytes(rel)
		have, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		switch {
		case err != nil:
			continue
		case os.IsNotExist(readErr):
			row.State = "missing"
		case readErr != nil:
			row.State = "differs"
		case bytes.Equal(have, seed):
			row.State = "same"
		default:
			row.State, row.Replaceable = "differs", !isSettingsPath(rel)
		}
		rows = append(rows, row)
	}
	return rows
}

// replacePlaybookFile moves the teacher's copy to playbook-old/<date>/<path>
// and writes this version's starter copy. Settings files are never replaced.
func replacePlaybookFile(root, rel string, now time.Time) error {
	rel = path.Clean(strings.ReplaceAll(rel, "\\", "/"))
	known := false
	for _, p := range playbookSeedPaths() {
		known = known || p == rel
	}
	if !known || isSettingsPath(rel) {
		return fmt.Errorf("%s can't be replaced from the hub", rel)
	}
	seed, err := seedBytes(rel)
	if err != nil {
		return err
	}
	target, err := workspace.ConstrainPath(root, filepath.FromSlash(rel))
	if err != nil {
		return err
	}
	if _, err = os.Lstat(target); err == nil {
		oldRel := path.Join("playbook-old", now.Format("2006-01-02"), rel)
		dest, err := workspace.ConstrainPath(root, filepath.FromSlash(oldRel))
		if err != nil {
			return err
		}
		ext := filepath.Ext(dest)
		base := strings.TrimSuffix(dest, ext)
		for n := 2; ; n++ {
			if _, statErr := os.Lstat(dest); os.IsNotExist(statErr) {
				break
			}
			dest = fmt.Sprintf("%s-%d%s", base, n, ext)
		}
		if err = os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		if err = os.Rename(target, dest); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	return os.WriteFile(target, seed, 0o644)
}

func (h *Hub) replacePlaybook(w http.ResponseWriter, r *http.Request) {
	if !h.checkPost(w, r) || h.needWorkspace(w) {
		return
	}
	rel := r.PostForm.Get("file")
	if r.PostForm.Get("confirm") != "on" {
		h.redirect(w, r, "/settings", "", "Tick the box to confirm before replacing a file.")
		return
	}
	if err := replacePlaybookFile(h.root, rel, time.Now()); err != nil {
		h.redirect(w, r, "/settings", "", "Could not get the new version: "+err.Error())
		return
	}
	h.redirect(w, r, "/settings", rel+" is now this version's copy. Your old one is in playbook-old.", "")
}

func (h *Hub) howPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "Method not allowed", 405)
		return
	}
	h.render(w, 200, howTemplate, h.pageData())
}

// howTemplate explains the workspace and Engine and answers "How do I...?" with popouts
// (native popover, no script).
var howTemplate = template.Must(template.New("how").Parse(`<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>How it works</title><style>` + funkyCSS + `</style><main class="wide"><div class="badge">CanvasExpert: SE</div>` + hubNav + `<h1>How it works</h1><p>One folder. Its <code>engine</code> folder holds the machinery; everything else is your playbooks, your context and your students.</p>
<div class="cards">
<div class="card"><h2>The Engine</h2><p>The machinery that lets the agent work with your Canvas. It also carries the starter playbooks. Nothing private lives in it. It's replaced whenever a new version comes out.</p><p><span class="pill on">{{.Version}}</span></p><p><code>{{if .Program}}{{.Program}}{{else}}unknown{{end}}</code></p></div>
<div class="card"><h2>Your CE-SE installation: playbooks, context and student information</h2><p>A <b>playbook</b> is the instruction set the agent follows for one kind of task, like scoring or creating an assignment. Say &ldquo;scoring playbook&rdquo; and it reads that one. Your <b>context</b> is what the playbooks draw on: grading voice, student notes, courses, calendar. All of it is yours to change. Student information stays here too.</p>{{if .Root}}<p><code>{{.Root}}</code></p>{{else}}<p>For development, launch with <code>cse --workspace &quot;PATH&quot; ui</code>.</p>{{end}}</div>
</div>
<p>Judgment calls (scores, feedback, lateness, policies) are made by you and the agent using your playbooks and context. The Engine just carries them out safely.</p>
<h2 class="grp">How do I&hellip;?</h2>
<div class="row">
<button type="button" popovertarget="how-update">Update the program</button>
<button type="button" popovertarget="how-file">Get the newest version of a playbook</button>
<button type="button" popovertarget="how-behave">Change how the agent behaves</button>
<button type="button" popovertarget="how-computer">Use another computer</button>
<button type="button" popovertarget="how-private">Keep student information private</button>
<button type="button" popovertarget="how-ask">Ask the agent for work</button>
</div>
<div popover id="how-update"><h2>Update the program</h2><ol><li>Unzip the new version somewhere else.</li><li>Copy its <code>engine</code> folder over the one in your CE-SE folder. Copy nothing else: your playbooks and context stay yours.</li><li>Double-click <code>webui.cmd</code>. New instruction and playbook files appear on the next hub or MCP launch; existing teacher copies, including <code>AGENTS.md</code> and playbooks, are not updated automatically.</li><li>To adopt updated instructions and playbooks, use Settings &rarr; Your playbooks and context &rarr; Get the new version. This includes the latest <code>AGENTS.md</code> directions to have the Engine open the review page and, after an accepted post, report it in one line and stop without re-checking Canvas. Your previous copies are kept in <code>playbook-old</code>.</li></ol><button type="button" class="small" popovertarget="how-update" popovertargetaction="hide">Close</button></div>
<div popover id="how-file"><h2>Get updated instructions or a playbook</h2><ol><li>Easiest: open <a href="/settings">Settings</a> and find <b>Your playbooks and context</b>. Choose <b>Get the new version</b> to replace that file with this program version's copy. Your old copy is kept in <code>playbook-old</code>.</li><li>Or delete the file from your CE-SE folder and reopen the hub. No copy is kept.</li><li>Your context files (feedback voice, courses, students) can't be replaced this way.</li></ol><button type="button" class="small" popovertarget="how-file" popovertargetaction="hide">Close</button></div>
<div popover id="how-behave"><h2>Change how the agent behaves</h2><ol><li>Feedback voice and grading habits: <a href="/settings">Settings</a>.</li><li>Student accommodations and notes: <a href="/students">Students</a>.</li><li>Active courses: <a href="/courses">Courses</a>.</li><li>Anything else: ask the agent in chat to update the right playbook, or edit the file yourself.</li></ol><button type="button" class="small" popovertarget="how-behave" popovertargetaction="hide">Close</button></div>
<div popover id="how-computer"><h2>Use another computer</h2><ol><li>Unzip CanvasExpert-SE on that computer, on its own drive, not in OneDrive (districts often block syncing programs).</li><li>Point the agent at it and ask it to set up. It will connect Canvas with you; the token is saved per computer.</li></ol><button type="button" class="small" popovertarget="how-computer" popovertargetaction="hide">Close</button></div>
<div popover id="how-private"><h2>Keep student information private</h2><ol><li>Student names and pulled work stay in your CE-SE folder: the private <code>.cse</code> folder and <code>courses/</code>.</li><li>The agent is told not to open <code>.cse</code>.</li><li>Nothing private goes in the <code>engine</code> folder.</li><li>Never put your Canvas token in a file or in chat.</li></ol><button type="button" class="small" popovertarget="how-private" popovertargetaction="hide">Close</button></div>
<div popover id="how-ask"><h2>Ask the agent for work</h2><ol><li>Say <b>scoring session</b>: it pulls the work, then asks you what to do.</li><li>Say <b>draft</b>: it saves scores and feedback but doesn't post.</li><li>For scores, new assignments, new quizzes and assignment edits, the Engine opens the review page in your browser. Review the proposed work there and approve it on the page before anything is posted; say if you want to skip review.</li><li>After Canvas accepts a post, the agent reports completion in one line and stops without re-checking Canvas.</li><li>Say <b>score and post</b> to set what gets posted; the review page is still the approval step.</li></ol><button type="button" class="small" popovertarget="how-ask" popovertargetaction="hide">Close</button></div>
</main></html>`))
