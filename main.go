package main

import (
	"context"
	"errors"
	"github.com/quonaro/lota/cli"
	"github.com/quonaro/lota/runner"
	"os"
	"os/signal"
	"syscall"

	"github.com/fatih/color"
)

func main() {
	exitCode := 0
	defer func() {
		os.Exit(exitCode)
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := cli.Run(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
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
}
