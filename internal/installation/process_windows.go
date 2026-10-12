package installation

import (
	"os/exec"
	"syscall"
)

func hide(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true} }
