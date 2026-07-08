//go:build !windows

package runner

import (
	"io"
	"os"
	"os/exec"
	"syscall"
	"unsafe"
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

// IsTerminal checks if the reader is a terminal.
func IsTerminal(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}

	var termios syscall.Termios
	_, _, err := syscall.Syscall6(
		syscall.SYS_IOCTL,
		uintptr(f.Fd()),
		uintptr(getTermiosIOCTL()),
		uintptr(unsafe.Pointer(&termios)),
		0, 0, 0,
	)
	return err == 0
}

// DisableSignalEcho disables the terminal's echo of control characters like ^C.
// Returns the original termios state for restoration.
func DisableSignalEcho(f *os.File) (interface{}, error) {
	var oldState syscall.Termios
	_, _, errno := syscall.Syscall6(
		syscall.SYS_IOCTL,
		uintptr(f.Fd()),
		uintptr(getTermiosIOCTL()),
		uintptr(unsafe.Pointer(&oldState)),
		0, 0, 0,
	)
	if errno != 0 {
		return oldState, errno
	}

	newState := oldState
	// Disable ECHOCTL to prevent ^C from being displayed
	newState.Lflag &^= syscall.ECHOCTL

	_, _, errno = syscall.Syscall6(
		syscall.SYS_IOCTL,
		uintptr(f.Fd()),
		uintptr(setTermiosIOCTL()),
		uintptr(unsafe.Pointer(&newState)),
		0, 0, 0,
	)
	if errno != 0 {
		return oldState, errno
	}
	return oldState, nil
}

// RestoreTerminal restores the original terminal settings.
func RestoreTerminal(f *os.File, state interface{}) error {
	termios, ok := state.(syscall.Termios)
	if !ok {
		return nil
	}
	_, _, errno := syscall.Syscall6(
		syscall.SYS_IOCTL,
		uintptr(f.Fd()),
		uintptr(setTermiosIOCTL()),
		uintptr(unsafe.Pointer(&termios)),
		0, 0, 0,
	)
	if errno != 0 {
		return errno
	}
	return nil
}
