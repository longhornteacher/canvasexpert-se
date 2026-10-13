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
		h.redirect(w, r, "/settings?section=files", "", "Tick the box to confirm before replacing a file.")
		return
	}
	if err := replacePlaybookFile(h.root, rel, time.Now()); err != nil {
		h.redirect(w, r, "/settings?section=files", "", "Could not get the new version: "+err.Error())
		return
	}
	h.redirect(w, r, "/settings?section=files", rel+" is now this version's copy. Your old one is in playbook-old.", "")
}

func (h *Hub) howPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "Method not allowed", 405)
		return
	}
	h.render(w, 200, howTemplate, h.pageData())
}

// howTemplate explains storage boundaries and uses native disclosure sections for guide topics.
var howTemplate = template.Must(template.New("how").Parse(`<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>How it works</title><style>` + funkyCSS + `</style><main class="wide"><div class="badge">CanvasExpert: SE</div>` + hubNavHow + `<h1>How it works</h1><p><b>Source</b> is the development checkout. <b>Install</b> is the local runnable CE-SE folder with its Engine, teacher files, courses, student information and work. Teacher rules use Install by default. Settings can select an optional <b>Sync</b> folder as their shared authority; its provider carries those files between computers. CE-SE reads and edits the selected files but does not synchronize them itself. Student information and work stay in Install.</p>
<div class="cards">
<div class="card"><h2>The Engine</h2><p>The machinery that lets an agent work with Canvas. It includes starter playbooks, carries no private teacher data, and is replaced when you install a new version.</p><p><span class="pill on">{{.Version}}</span></p><p><code>{{if .Program}}{{.Program}}{{else}}unknown{{end}}</code></p></div>
<div class="card"><h2>Your Install</h2><p>Install keeps courses, student information, pulled work, drafts and the Engine. It also keeps teacher rules when no Sync folder is selected. With Sync selected, those rule files live in the shared folder; course and student data remain local.</p>{{if .Root}}<p><code>{{.Root}}</code></p>{{else}}<p>For development, launch with <code>cse --workspace &quot;PATH&quot; ui</code>.</p>{{end}}</div>
</div>
<p>You and the agent use teacher rules and context for judgment calls such as scores, feedback, lateness and policies. The Engine carries out supported operations safely.</p>
<h2 class="grp">How do I&hellip;?</h2>
<section class="guide-topics">
<details class="card guide-topic"><summary>Update the program</summary><ol><li>Unzip the new version somewhere else.</li><li>Copy its <code>engine</code> folder over the one in your Install. Copy nothing else; your teacher files stay yours.</li><li>Double-click <code>webui.cmd</code>. Missing starter files appear on the next hub or MCP launch. Existing copies, including <code>AGENTS.md</code> and playbooks, are not updated automatically.</li><li>To adopt a newer instruction or playbook, open <a href="/settings?section=files">Settings, Files</a> and choose Get the new version. Your previous copy is kept in <code>playbook-old</code>.</li></ol></details>
<details class="card guide-topic"><summary>Get updated instructions or a playbook</summary><ol><li>Open <a href="/settings?section=files">Settings, Files</a> and choose <b>Get the new version</b>. Your old copy is kept in <code>playbook-old</code>.</li><li>Or delete the file from your Install and reopen the hub. That removes the old copy without an archive.</li><li>Context files such as feedback voice, courses and students cannot be replaced this way.</li></ol></details>
<details class="card guide-topic"><summary>Change how the agent behaves</summary><ol><li>Feedback voice and grading habits: <a href="/settings?section=preferences">Settings, Preferences</a>.</li><li>Student accommodations and notes: <a href="/students">Students</a>.</li><li>Active courses: <a href="/courses">Courses</a>.</li><li>For other changes, ask the agent to update the appropriate playbook or edit it yourself.</li></ol></details>
<details class="card guide-topic"><summary>Use another computer</summary><ol><li>Unzip CanvasExpert-SE on that computer's own drive, not in OneDrive; some districts block syncing programs.</li><li>In Settings, select the same teacher-rules Sync folder. The provider carries rule files; CE-SE does not copy them between computers.</li><li>Point the agent at this computer's Install and ask it to set up. Connect Canvas on this computer too; its token, course and student data, and work stay local.</li></ol></details>
<details class="card guide-topic"><summary>Keep student information private</summary><ol><li>Student names and pulled work stay in your Install's private <code>.cse</code> folder and <code>courses/</code>.</li><li>The agent is instructed not to open <code>.cse</code>.</li><li>Private information does not go in the <code>engine</code> folder.</li><li>Never put your Canvas token in a file or chat.</li></ol></details>
<details class="card guide-topic"><summary>Ask the agent for work</summary><ol><li>Say <b>scoring session</b>: it pulls the work, then asks what to do.</li><li>Say <b>draft</b>: it saves scores and feedback but does not post.</li><li>For scores, new assignments, New Quizzes and assignment edits, the Engine opens a review page. Review the proposed work and approve it there before posting; say if you want to skip review.</li><li>After Canvas accepts a post, the agent reports completion in one line and stops without re-checking Canvas.</li><li>Say <b>score and post</b> to set what gets posted; the review page remains the approval step.</li></ol></details>
</section></main></html>`))
