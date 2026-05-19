//go:build !windows

package runner

import (
	"os/exec"
	"syscall"
)

func setupSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func terminateProcess(pid int) error {
	return syscall.Kill(-pid, syscall.SIGTERM)
}

func killProcess(pid int) error {
	return syscall.Kill(-pid, syscall.SIGKILL)
}
