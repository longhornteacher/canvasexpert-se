package localui

import (
	"errors"
	"html/template"
	"net/http"

	"canvasexpertse/internal/installation"
)

type connectionData struct {
	Secret, Root, Message, Notice, Client, ExistingInstall string
	Result                                                 installation.Result
	Check                                                  installation.CheckResult
}

var connectionTemplate = template.Must(template.New("connect").Parse(`<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Connect your teaching assistant</title><style>` + funkyCSS + `</style><main class="wide"><div class="badge">CanvasExpert: SE</div>` + hubNav + `<h1>Connect your teaching assistant</h1><p>This connects the Engine in this Install: <code>{{.Root}}</code>. Choose the app where you want to teach.</p>
{{if .Message}}<p class="error">{{.Message}}</p>{{end}}{{if .Notice}}<p class="ok">{{.Notice}}</p>{{end}}
{{if .Root}}<form method="post" action="/connect"><input type="hidden" name="request_secret" value="{{.Secret}}"><label for="client">Teaching app</label><select id="client" name="client"><option value="work" {{if eq .Client "work"}}selected{{end}}>Work (local desktop)</option><option value="cowork" {{if eq .Client "cowork"}}selected{{end}}>Cowork (Claude desktop)</option></select>
{{if .ExistingInstall}}<p>The CE-SE connection currently points to <code>{{.ExistingInstall}}</code>. Replacing it will point this app to <code>{{.Root}}</code>; both Installs keep their files.</p><label class="check"><input type="checkbox" name="replace" required> Replace this app's CE-SE connection</label>{{end}}
<button name="action" value="register" type="submit">Connect selected app</button></form>{{else}}<p>Open the hub from the extracted Install using webui.cmd to connect an agent.</p>{{end}}
{{if .Result.State}}<div class="card"><h2>Connection outcome</h2><p>{{.Result.State}}</p>{{if .Result.NextAction}}<p>{{.Result.NextAction}}</p>{{end}}{{if .Result.ExtensionPath}}<p>Install the bundled extension using your app's local extension installer: <code>{{.Result.ExtensionPath}}</code>. <a class="go" href="/connect/extension">Download the ready-to-install extension &rarr;</a></p>{{end}}<p>Saved settings or an extension file do not establish that the destination app has used CE-SE tools.</p></div>{{end}}
<div class="card"><h2>Local tool check</h2><p>This starts this Install's Engine, discovers tools and reads instructions and configured courses. It makes no Canvas writes.</p>{{if .Root}}<form method="post" action="/connect"><input type="hidden" name="request_secret" value="{{.Secret}}"><button name="action" value="check" type="submit">Check local tools</button></form>{{end}}{{if .Check.State}}<p>{{.Check.State}}; {{.Check.ToolCount}} tools. Loaded adapter: <code>{{.Check.AdapterVersion}}</code>.</p><p>Instructions read: {{.Check.InstructionsRead}}. Configured courses read: {{.Check.ConfiguredCoursesRead}}.</p>{{end}}<p>A local check cannot verify the destination app. Reconnect or restart the app if it asks, after finishing any open review.</p></div>
<div class="card"><h2>Finish in your teaching app</h2><p>Once CE-SE tools appear, send this:</p><blockquote>Be my teaching assistant using CanvasExpert-SE. Read the current AGENTS.md through CE-SE, confirm the Install you are connected to and read my saved courses. Read my teaching profile, grading preferences and the playbook for each task. Keep those teacher files authoritative instead of copying the playbooks into project instructions. Help me carry over teaching instructions I choose, or learn my preferences gradually as we work. Tell me when the CE-SE tools are ready, then help with my first task. Drafting does not authorize Canvas writes; follow my requested scope and the ordinary review workflow.</blockquote><p>Your teaching preferences stay in Install or your configured Sync folder. You can add them progressively while working.</p><a class="go" href="/settings">Canvas connection and preferences &rarr;</a></div></main></html>`))

func (h *Hub) connectionPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.Method == http.MethodPost {
		if !h.checkPost(w, r) || h.needWorkspace(w) {
			return
		}
		h.connection.Message, h.connection.Notice = "", ""
		switch r.PostForm.Get("action") {
		case "register":
			h.connection.Result = installation.Result{}
			h.connection.ExistingInstall = ""
			h.connection.Client = r.PostForm.Get("client")
			result, err := installation.Register(h.root, h.connection.Client, r.PostForm.Get("replace") == "on")
			if err != nil {
				h.connection.Message = err.Error()
				var replacement *installation.ReplacementRequired
				if errors.As(err, &replacement) {
					h.connection.ExistingInstall = replacement.ExistingInstall
				}
			} else {
				h.connection.Result = result
				h.connection.ExistingInstall = ""
				h.connection.Notice = "Connection action completed. Follow the app's next action below."
			}
		case "check":
			ctx, cancel := h.reqCtx()
			defer cancel()
			result, err := installation.Check(ctx, h.root)
			h.connection.Check = result
			if err != nil {
				h.connection.Message = "Local MCP check did not pass: " + err.Error()
			}
		default:
			h.connection.Message = "Choose a connection action."
		}
	}
	h.connection.Secret, h.connection.Root = h.secret, h.root
	h.render(w, http.StatusOK, connectionTemplate, h.connection)
}
