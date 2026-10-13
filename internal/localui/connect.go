package localui

import (
	"errors"
	"net/http"

	"canvasexpertse/internal/installation"
)

type connectionData struct {
	Secret, Root, Message, Notice, Client, ExistingInstall string
	ResultLabel, CheckLabel                                string
	Result                                                 installation.Result
	Check                                                  installation.CheckResult
}

func init() {
	register(
		page{Path: "/connect", Methods: []string{http.MethodGet, http.MethodPost}, Template: "connect",
			Nav:    []navSlot{{Primary: "Settings", Label: "Agent", Order: 35}},
			Handle: (*Hub).connectionPage},
		page{Path: "/connect/extension", Handle: (*Hub).connectionExtension},
	)
}

// connectionExtension serves the prepared Cowork extension, if any.
func (h *Hub) connectionExtension(w http.ResponseWriter, r *http.Request) {
	if h.connection.Result.ExtensionPath == "" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="CanvasExpert-SE.mcpb"`)
	http.ServeFile(w, r, h.connection.Result.ExtensionPath)
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
	h.connection.ResultLabel = connectionResultLabel(h.connection.Result)
	h.connection.CheckLabel = connectionCheckLabel(h.connection.Check)
	h.render(w, http.StatusOK, tmpl("connect"), h.connection)
}

func connectionResultLabel(result installation.Result) string {
	switch result.State {
	case "settings-saved":
		return "Work registration saved"
	case "settings-already-saved":
		return "Work already points to this Install"
	case "extension-ready":
		switch result.CoworkLaunchOutcome {
		case "launch_requested":
			return "Cowork extension prepared; opening was requested"
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
