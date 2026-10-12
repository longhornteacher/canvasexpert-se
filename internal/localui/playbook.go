package localui

import (
	"html/template"
	"net/http"
	"time"

	updates "canvasexpertse/internal/playbooks"
)

type playbookRow struct {
	Path, State, SHA256 string
	Replaceable         bool
}

func playbookRows(root string) []playbookRow {
	rows := updates.Status(root)
	out := make([]playbookRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, playbookRow{Path: r.Path, State: r.State, SHA256: r.SHA256, Replaceable: r.Replaceable})
	}
	return out
}
func seedBytes(rel string) ([]byte, error) { return updates.SeedBytes(rel) }
func replacePlaybookFile(root, rel string, now time.Time) error {
	return updates.Replace(root, rel, "", now)
}
func ReplacePlaybook(root, rel, expectedSHA256 string, now time.Time) error {
	return updates.Replace(root, rel, expectedSHA256, now)
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
var howTemplate = template.Must(template.New("how").Parse(`<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>How it works</title><style>` + funkyCSS + `</style><main class="wide"><div class="badge">CanvasExpert: SE</div>` + hubNav + `<h1>How it works</h1><p>Install holds the Engine, courses, students and work. Teacher rules (instructions, playbooks, guides and preferences) live in Install by default. Settings can point them to an optional Sync folder; CE-SE reads and edits those files there, while OneDrive, Google Drive or another provider carries them between computers.</p>
<div class="cards">
<div class="card"><h2>The Engine</h2><p>The machinery that lets the agent work with your Canvas. It also carries the starter playbooks. Nothing private lives in it. It's replaced whenever a new version comes out.</p><p><span class="pill on">{{.Version}}</span></p><p><code>{{if .Program}}{{.Program}}{{else}}unknown{{end}}</code></p></div>
<div class="card"><h2>Your CE-SE installation</h2><p>Your courses, student information, pulled work, drafts and Engine stay in Install. Teacher rules include instructions, playbooks, guides and preferences; they use Install by default and can use a selected Sync folder from Settings. Student information always stays in Install.</p>{{if .Root}}<p><code>{{.Root}}</code></p>{{else}}<p>For development, launch with <code>cse --workspace &quot;PATH&quot; ui</code>.</p>{{end}}</div>
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
<div popover id="how-computer"><h2>Use another computer</h2><ol><li>Unzip CanvasExpert-SE on that computer, on its own drive, not in OneDrive (districts often block syncing programs).</li><li>In Settings, select the same teacher-rules Sync folder used on your other computer. The provider carries rule files; CE-SE does not copy files between computers itself.</li><li>Point the agent at this Install and ask it to set up. Connect Canvas on this computer too; its token and course/student/work state stay local.</li></ol><button type="button" class="small" popovertarget="how-computer" popovertargetaction="hide">Close</button></div>
<div popover id="how-private"><h2>Keep student information private</h2><ol><li>Student names and pulled work stay in your CE-SE folder: the private <code>.cse</code> folder and <code>courses/</code>.</li><li>The agent is told not to open <code>.cse</code>.</li><li>Nothing private goes in the <code>engine</code> folder.</li><li>Never put your Canvas token in a file or in chat.</li></ol><button type="button" class="small" popovertarget="how-private" popovertargetaction="hide">Close</button></div>
<div popover id="how-ask"><h2>Ask the agent for work</h2><ol><li>Say <b>scoring session</b>: it pulls the work, then asks you what to do.</li><li>Say <b>draft</b>: it saves scores and feedback but doesn't post.</li><li>For scores, new assignments, new quizzes and assignment edits, the Engine opens the review page in your browser. Review the proposed work there and approve it on the page before anything is posted; say if you want to skip review.</li><li>After Canvas accepts a post, the agent reports completion in one line and stops without re-checking Canvas.</li><li>Say <b>score and post</b> to set what gets posted; the review page is still the approval step.</li></ol><button type="button" class="small" popovertarget="how-ask" popovertargetaction="hide">Close</button></div>
</main></html>`))
