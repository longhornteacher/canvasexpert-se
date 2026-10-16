//go:build !windows

package platform

import "errors"

func CreateDesktopShortcut(root string) (string, error) {
	return "", errors.New("desktop shortcut creation is available on Windows")
}
