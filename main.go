package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

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
	oldState, terminalRestored := setupTerminal()
	defer func() {
		if !terminalRestored {
			_ = restoreTerminal(oldState)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := cli.Run(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			// Restore terminal before exit
			_ = restoreTerminal(oldState)
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
	_ = restoreTerminal(oldState)
	terminalRestored = true
}
