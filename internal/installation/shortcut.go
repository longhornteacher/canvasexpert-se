package installation

import (
	"fmt"
	"path/filepath"

	"canvasexpertse/internal/platform"
)

// EnsureDesktopShortcut is a setup action, separate from read-only inspection
// and low-level client registration. A caller reports failure and continues.
func EnsureDesktopShortcut(root string) (string, error) {
	root, _, err := validate(root)
	if err != nil {
		return "", err
	}
	return platform.CreateDesktopShortcut(root)
}

func DesktopShortcutWarning(root string, err error) string {
	return fmt.Sprintf("Desktop shortcut could not be created: %v. Setup continues. Open %s to start CanvasExpert WebUI.", err, filepath.Join(root, "webui.cmd"))
}
