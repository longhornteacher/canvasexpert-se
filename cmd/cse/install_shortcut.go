package main

import (
	"encoding/json"
	"fmt"
	"io"

	"canvasexpertse/internal/installation"
)

var ensureDesktopShortcut = installation.EnsureDesktopShortcut

func reportDesktopShortcut(root string, create func(string) (string, error), out, progress io.Writer) {
	path, err := create(root)
	if err != nil {
		fmt.Fprintln(progress, installation.DesktopShortcutWarning(root, err))
		return
	}
	data, _ := json.Marshal(struct {
		Path string `json:"desktop_shortcut"`
	}{path})
	fmt.Fprintln(out, string(data))
}
