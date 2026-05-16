package main

import (
	"errors"
	"lota/cli"
	"lota/runner"
	"os"

	"github.com/fatih/color"
)

func main() {
	if err := cli.Run(); err != nil {
		var shellErr *runner.ShellError
		if errors.As(err, &shellErr) {
			color.Red("Error: %v\n", err)
			os.Exit(shellErr.ExitCode)
		}
		color.Red("Error: %v\n", err)
		os.Exit(1)
	}
}
