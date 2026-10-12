//go:build windows

package platform

import (
	"os/exec"
	"syscall"
)

// LaunchHidden starts an independent local helper without a console window.
// The arguments are passed separately so paths with spaces remain intact.
func LaunchHidden(executable string, args ...string) error {
	cmd := exec.Command(executable, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
