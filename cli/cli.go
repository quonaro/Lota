package cli

import (
	"fmt"
	"lota/config"
	"lota/runner"
	"os"
	"path/filepath"
	"strings"
)

// Run executes the CLI application
func Run() error {
	// Check for shell completion mode first (posener/complete reads COMP_LINE and COMP_POINT)
	if os.Getenv("COMP_LINE") != "" {
		RunCompletion()
		return nil
	}

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

	fc, err := config.GetConfigPath(flags.Config)
	if err != nil {
		return fmt.Errorf("error resolving config path: %w", err)
	}
	configDir := filepath.Dir(fc.Path)

	// Check for help flag before ResolveCommand (it skips flags)
	if hasHelpFlag(remainingArgs) {
		// Resolve command to show help for it
		result, _, _ := ResolveCommand(cfg, remainingArgs)
		if !result.Exists {
			return fmt.Errorf("command not found: %s", strings.Join(remainingArgs, " "))
		}
		verbose := flags.Verbose || hasVerboseFlag(remainingArgs)
		PrintCommandHelp(cfg, result, verbose)
		return nil
	}

	result, cmdArgs, _ := ResolveCommand(cfg, remainingArgs)
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

	opts := runner.RunOptions{
		Verbose:   flags.Verbose,
		DryRun:    flags.DryRun,
		ConfigDir: configDir,
	}
	return RunCommand(cfg, result, cmdArgs, opts)
}
