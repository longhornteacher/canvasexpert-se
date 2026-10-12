//go:build !windows

package installation

import "os/exec"

func hide(cmd *exec.Cmd) {}
