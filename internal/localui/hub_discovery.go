package localui

import (
	"canvasexpertse/internal/hubcoord"
	"canvasexpertse/internal/platform"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

func reportExistingHub(loc hubcoord.Location, root string, output io.Writer, options HubOptions) error {
	firstPage := "/"
	if options.SetupFirst {
		if _, err := platform.LoadCredential(); err != nil {
			firstPage = "/setup"
		} else {
			firstPage = "/settings"
		}
	}
	pageURL := loc.URL + firstPage
	_, _ = fmt.Fprintf(output, "Reusing the running CanvasExpert-SE hub for this UserInstall: %s (this command does not own or stop it)\n", pageURL)
	browserOutcome := "launch_requested"
	if err := openBrowser(pageURL); err != nil {
		browserOutcome = "launch_failed"
		_, _ = fmt.Fprintln(output, "The browser could not open automatically; open the local URL above.")
	} else {
		_, _ = fmt.Fprintln(output, "A browser launch was requested for the local hub; this does not confirm that a page appeared.")
	}
	if options.ReadyFile != "" {
		return writeHubReady(options.ReadyFile, pageURL, root, loc.URL+"/setup/status", true, browserOutcome)
	}
	return nil
}

func writeHubReady(path, pageURL, root, statusURL string, reused bool, browserOutcome string) error {
	ready, err := json.Marshal(struct {
		URL            string `json:"url"`
		InstallRoot    string `json:"install_root"`
		SetupStatusURL string `json:"setup_status_url"`
		Reused         bool   `json:"reused"`
		BrowserOutcome string `json:"browser_outcome"`
	}{pageURL, root, statusURL, reused, browserOutcome})
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err = os.WriteFile(tmp, ready, 0600); err != nil {
		return fmt.Errorf("could not report hub readiness: %w", err)
	}
	if err = os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("could not report hub readiness: %w", err)
	}
	return nil
}
