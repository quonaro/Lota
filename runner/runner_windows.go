//go:build windows

package runner

import (
	"os"
	"os/exec"
)

func setupSysProcAttr(cmd *exec.Cmd) {
	// Windows does not support Setpgid via SysProcAttr.
}

func terminateProcess(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Kill()
}

func killProcess(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Kill()
}
