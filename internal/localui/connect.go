package localui

import (
	"errors"
	"net/http"

	"canvasexpertse/internal/installation"
)

type connectionData struct {
	Appearance                                             appearanceData
	Secret, Root, Message, Notice, Client, ExistingInstall string
	ResultLabel, CheckLabel                                string
	Result                                                 installation.Result
	Results                                                []installation.Result
	Found                                                  installation.Availability
	Check                                                  installation.CheckResult
}

func init() {
	register(
		page{Path: "/connect", Methods: []string{http.MethodGet, http.MethodPost}, Template: "connect", Scripts: []string{"appearance.js"},
			Nav:    []navSlot{{Primary: "Settings", Label: "Agent", Order: 35}},
			Handle: (*Hub).connectionPage},
		page{Path: "/connect/extension", Handle: (*Hub).connectionExtension},
	)
}

// connectionExtension serves the prepared Cowork extension, if any.
func (h *Hub) connectionExtension(w http.ResponseWriter, r *http.Request) {
	path := h.connection.Result.ExtensionPath
	for _, r := range h.connection.Results {
		if path == "" {
			path = r.ExtensionPath
		}
	}
	if path == "" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="CanvasExpert-SE.mcpb"`)
	http.ServeFile(w, r, path)
}

func (h *Hub) connectionPage(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		if !h.checkPost(w, r) || h.needWorkspace(w) {
			return
		}
		h.connection.Message, h.connection.Notice = "", ""
		switch r.PostForm.Get("action") {
		case "register":
			h.connection.Result = installation.Result{}
			h.connection.Results = nil
			h.connection.ExistingInstall = ""
			h.connection.Client = r.PostForm.Get("client")
			replace := r.PostForm.Get("replace") == "on"
			if h.connection.Client == "" || h.connection.Client == "auto" {
				results, err := installation.RegisterAuto(h.root, replace)
				h.connection.Results = results
				switch {
				case err != nil:
					h.connection.Message = err.Error()
					h.connection.Notice = ""
				case len(results) == 0:
					h.connection.Message = "No supported app was found on this PC."
				default:
					h.connection.Notice = "Connection action completed. Follow each app's next action below."
				}
				break
			}
			result, err := installation.Register(h.root, h.connection.Client, replace)
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
	h.connection.Appearance = appearanceFor(h.root, h.secret)
	h.connection.Secret, h.connection.Root = h.secret, h.root
	h.connection.Found = installation.Inspect(h.root)
	h.connection.ResultLabel = connectionResultLabel(h.connection.Result)
	h.connection.CheckLabel = connectionCheckLabel(h.connection.Check)
	h.render(w, http.StatusOK, tmpl("connect"), h.connection)
}

func connectionResultLabel(result installation.Result) string {
	switch result.State {
	case "settings-saved":
		return labelApp(result.Client) + " registration saved"
	case "settings-already-saved":
		return labelApp(result.Client) + " already points to this UserInstall"
	case "extension-ready":
		switch result.CoworkLaunchOutcome {
		case "launch_requested":
			return "Cowork extension prepared; opening was requested"
		case "file_revealed":
			return "Cowork extension prepared; file shown in File Explorer for manual install"
		case "launch_failed":
			return "Cowork extension prepared; automatic opening did not work"
		default:
			return "Cowork extension prepared"
		}
	default:
		return "Local setup action completed"
	}
}

func connectionCheckLabel(result installation.CheckResult) string {
	if result.State == "local-check-passed" {
		return "Local checks passed"
	}
	if result.State == "local-check-failed" {
		return "Local check did not pass"
	}
	return ""
}

func appName(client string) string {
	switch client {
	case "work":
		return "ChatGPT"
	case "cowork":
		return "Claude"
	case "antigravity":
		return "Antigravity"
	}
	return client
}

// ResultLabelFor lets the template label each result of a multi-app pass.
func (c connectionData) ResultLabelFor(r installation.Result) string { return connectionResultLabel(r) }
func (c connectionData) AppName(client string) string                { return appName(client) }

func labelApp(client string) string {
	if client == "" || client == "work" {
		return "Work"
	}
	return appName(client)
}
