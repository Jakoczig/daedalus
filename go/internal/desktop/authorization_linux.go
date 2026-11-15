//go:build linux

package desktop

import "os/exec"

func hideAuthorizationWindow(cmd *exec.Cmd) {}
