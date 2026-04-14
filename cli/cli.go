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

	// Check for help flag before ResolveCommand (it skips flags)
	if hasHelpFlag(remainingArgs) {
		// Resolve command to show help for it
		result, _ := ResolveCommand(cfg, remainingArgs)
		if !result.Exists {
			return fmt.Errorf("command not found: %s", strings.Join(remainingArgs, " "))
		}
		verbose := flags.Verbose || hasVerboseFlag(remainingArgs)
		PrintCommandHelp(cfg, result, verbose)
		return nil
	}

	result, cmdArgs := ResolveCommand(cfg, remainingArgs)
	if !result.Exists {
		return fmt.Errorf("command not found: %s", strings.Join(remainingArgs, " "))
	}

	verbose := flags.Verbose || hasVerboseFlag(cmdArgs)

	if len(result.Groups) > 0 && result.Command == nil {
		PrintGroupHelp(result.Groups[len(result.Groups)-1], verbose)
		return nil
	}

	if hasHelpFlag(cmdArgs) {
		PrintCommandHelp(cfg, result, verbose)
		return nil
	}

	// Filter out global flags from command args before parsing
	cmdArgs = filterGlobalFlags(cmdArgs)

	opts := runner.RunOptions{
		Verbose: flags.Verbose,
		DryRun:  flags.DryRun,
	}
	return RunCommand(cfg, result, cmdArgs, opts)
}

// filterGlobalFlags removes global flags from command args
// Global flags: --help, -h, --verbose, -v, --version, -V, --dry-run, --init, --config
func filterGlobalFlags(args []string) []string {
	result := make([]string, 0, len(args))
	i := 0

	for i < len(args) {
		arg := args[i]

		// Skip global flags
		switch arg {
		case "--help", "-h", "--verbose", "-v", "--version", "-V", "--dry-run", "--init":
			i++
			continue
		case "--config":
			// Skip --config and its value
			i += 2
			continue
		}

		result = append(result, arg)
		i++
	}

	return result
}
