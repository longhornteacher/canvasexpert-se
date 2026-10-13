package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"canvasexpertse/internal/hubcoord"
	"canvasexpertse/internal/platform"
)

type installUIReady struct {
	URL            string `json:"url"`
	InstallRoot    string `json:"install_root"`
	SetupStatusURL string `json:"setup_status_url"`
	Reused         bool   `json:"reused"`
	BrowserOutcome string `json:"browser_outcome,omitempty"`
}

// installUIWarning tells agents the WebUI did not start without implying that
// registration or local checks failed; those results follow on their own lines.
type installUIWarning struct {
	UIWarning struct {
		Code     string `json:"code"`
		Message  string `json:"message"`
		Fallback string `json:"fallback"`
	} `json:"ui_warning"`
}

func newInstallUIWarning(err error) installUIWarning {
	var w installUIWarning
	w.UIWarning.Code = "webui_not_started"
	w.UIWarning.Message = err.Error() + ". Registration and local checks are reported separately."
	w.UIWarning.Fallback = "Open webui.cmd in this Install folder to start the private WebUI, then retry the connection from the hub."
	return w
}

func launchInstallUI(ctx context.Context, root, client string) (installUIReady, error) {
	var ready installUIReady
	f, err := os.CreateTemp("", "cse-ui-ready-*.json")
	if err != nil {
		return ready, err
	}
	path := f.Name()
	f.Close()
	os.Remove(path)
	defer os.Remove(path)
	exe, err := os.Executable()
	if err != nil {
		return ready, err
	}
	if err = platform.LaunchHidden(exe, "--workspace", root, "ui", "--setup-first", "--install-client", client, "--ready-file", path); err != nil {
		return ready, err
	}
	timer := time.NewTimer(8 * time.Second)
	defer timer.Stop()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ready, ctx.Err()
		case <-timer.C:
			return ready, errors.New("private WebUI did not report startup")
		case <-ticker.C:
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			if json.Unmarshal(data, &ready) != nil || ready.URL == "" || hubcoord.CanonicalRoot(ready.InstallRoot) != hubcoord.CanonicalRoot(root) {
				return ready, errors.New("private WebUI startup returned an unexpected Install")
			}
			return ready, nil
		}
	}
}

// reportInstallUI prints the WebUI start result. On failure it emits a
// structured warning and returns the error; the caller still registers and checks.
func reportInstallUI(ctx context.Context, root, client string, launch func(context.Context, string, string) (installUIReady, error), out, progress io.Writer, logEvent func(step, outcome, category, client string)) error {
	ready, err := launch(ctx, root, client)
	if err != nil {
		logEvent("hub", "failed", "startup", client)
		warning, _ := json.Marshal(newInstallUIWarning(err))
		fmt.Fprintln(out, string(warning))
		return err
	}
	outcome := "started"
	if ready.Reused {
		outcome = "reused"
	}
	logEvent("hub", outcome, "", client)
	if ready.BrowserOutcome == "launch_failed" || ready.BrowserOutcome == "launch_requested" {
		logEvent("browser", ready.BrowserOutcome, map[bool]string{true: "browser"}[ready.BrowserOutcome == "launch_failed"], client)
	}
	data, _ := json.Marshal(ready)
	fmt.Fprintln(out, string(data))
	if ready.Reused {
		fmt.Fprintln(progress, "Reused the running private WebUI for this Install; it remains open independently of this install call.")
	} else {
		fmt.Fprintln(progress, "Private WebUI launched; enter Canvas credentials only there.")
	}
	return nil
}
