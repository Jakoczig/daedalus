//go:build windows

package desktop

import (
	"os/exec"
	"syscall"
)

func hideAuthorizationWindow(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true} }
