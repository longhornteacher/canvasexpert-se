package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"time"

	"canvasexpertse/internal/platform"
)

type installUIReady struct {
	URL            string `json:"url"`
	InstallRoot    string `json:"install_root"`
	SetupStatusURL string `json:"setup_status_url"`
}

func launchInstallUI(ctx context.Context, root string) (installUIReady, error) {
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
	if err = platform.LaunchHidden(exe, "--workspace", root, "ui", "--setup-first", "--ready-file", path); err != nil {
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
			return ready, errors.New("private WebUI did not report startup; open webui.cmd in this Install and retry connection from the hub")
		case <-ticker.C:
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			if json.Unmarshal(data, &ready) != nil || ready.URL == "" || ready.InstallRoot != root {
				return ready, errors.New("private WebUI startup returned an unexpected Install")
			}
			return ready, nil
		}
	}
}
