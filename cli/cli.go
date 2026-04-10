package cli

import (
	"fmt"
	"lota/runner"
	"os"
	"strings"
)

// Run executes the CLI application
func Run() error {
	if len(os.Args) < 2 {
		PrintHelp("")
		return nil
	}

	cliArgs := os.Args[1:]

	flags, remainingArgs, err := ParseGlobalFlags(cliArgs)
	if err != nil {
		return err
	}

	if shouldExit, err := HandleGlobalFlags(flags); err != nil {
		return err
	} else if shouldExit {
		return nil
	}

	if len(remainingArgs) == 0 {
		PrintHelp(flags.Config)
		return nil
	}

	cfg, err := LoadConfig(flags.Config)
	if err != nil {
		return fmt.Errorf("error loading config: %w", err)
	}

	result, cmdArgs := ResolveCommand(cfg, remainingArgs)
	if !result.Exists {
		return fmt.Errorf("command not found: %s", strings.Join(remainingArgs, " "))
	}

	if len(result.Groups) > 0 && result.Command == nil {
		PrintGroupHelp(result.Groups[len(result.Groups)-1])
		return nil
	}

	opts := runner.RunOptions{
		Verbose: flags.Verbose,
		DryRun:  flags.DryRun,
	}
	return RunCommand(cfg, result, cmdArgs, opts)
}
