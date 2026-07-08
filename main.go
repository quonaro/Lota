package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"unsafe"

	"github.com/quonaro/lota/cli"
	"github.com/quonaro/lota/runner"

	"github.com/fatih/color"
)

func main() {
	exitCode := 0
	defer func() {
		os.Exit(exitCode)
	}()

	// Disable ^C echo on terminal
	var oldState syscall.Termios
	terminalRestored := false
	f := os.Stdout
	// Check if stdout is a terminal
	var termios syscall.Termios
	_, _, errno := syscall.Syscall6(
		syscall.SYS_IOCTL,
		uintptr(f.Fd()),
		uintptr(syscall.TCGETS),
		uintptr(unsafe.Pointer(&termios)),
		0, 0, 0,
	)
	if errno == 0 {
		// It's a terminal, disable signal echo
		oldState, _ = runner.DisableSignalEcho(f)
		defer func() {
			if !terminalRestored {
				_ = runner.RestoreTerminal(f, oldState)
			}
		}()
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := cli.Run(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			// Restore terminal before exit
			_ = runner.RestoreTerminal(f, oldState)
			terminalRestored = true
			return
		}
		var shellErr *runner.ShellError
		if errors.As(err, &shellErr) {
			color.Red("Error: %v\n", err)
			exitCode = shellErr.ExitCode
		} else {
			color.Red("Error: %v\n", err)
			exitCode = 1
		}
	}

	// Restore terminal on successful completion
	_ = runner.RestoreTerminal(f, oldState)
	terminalRestored = true
}
