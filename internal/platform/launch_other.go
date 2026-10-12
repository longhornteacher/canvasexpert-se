//go:build !windows

package platform

import "os/exec"

func LaunchHidden(executable string, args ...string) error {
	cmd := exec.Command(executable, args...)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
