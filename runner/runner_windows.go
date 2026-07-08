//go:build windows

package runner

import (
	"io"
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

// IsTerminal checks if the reader is a terminal.
// Windows stub: always returns false.
func IsTerminal(r io.Reader) bool {
	return false
}

// DisableSignalEcho disables the terminal's echo of control characters like ^C.
// Windows stub: no-op.
func DisableSignalEcho(f *os.File) (interface{}, error) {
	return nil, nil
}

// RestoreTerminal restores the original terminal settings.
// Windows stub: no-op.
func RestoreTerminal(f *os.File, state interface{}) error {
	return nil
}
