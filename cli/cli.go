package cli

import (
	"context"
	"fmt"
	"lota/config"
	"lota/engine"
	"os"
	"path/filepath"
)

// Run executes the CLI application
func Run(ctx context.Context) error {
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

	// Hidden completion subcommand: `lota __complete`
	if len(remainingArgs) > 0 && remainingArgs[0] == "__complete" {
		RunCompleteSubcommand(remainingArgs[1:])
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
		result, _, _ := config.ResolveCommand(cfg, remainingArgs)
		if !result.Exists {
			return commandNotFoundError(cfg, remainingArgs)
		}
		verbose := flags.Verbose || hasVerboseFlag(remainingArgs)
		switch {
		case result.Command != nil:
			PrintCommandHelp(cfg, result, verbose)
		case len(result.Groups) > 0:
			ancestors := result.Groups[:len(result.Groups)-1]
			PrintGroupHelp(result.Groups[len(result.Groups)-1], ancestors, verbose)
		default:
			PrintHelp(flags.Config)
		}
		return nil
	}

	result, cmdArgs, _ := config.ResolveCommand(cfg, remainingArgs)
	if !result.Exists {
		return commandNotFoundError(cfg, remainingArgs)
	}

	verbose := flags.Verbose || hasVerboseFlag(cmdArgs)

	if len(result.Groups) > 0 && result.Command == nil {
		ancestors := result.Groups[:len(result.Groups)-1]
		PrintGroupHelp(result.Groups[len(result.Groups)-1], ancestors, verbose)
		return nil
	}

	if hasHelpFlag(cmdArgs) {
		PrintCommandHelp(cfg, result, verbose)
		return nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	opts := engine.Options{
		Verbose:    flags.Verbose,
		DryRun:     flags.DryRun,
		ConfigDir:  configDir,
		WorkingDir: cwd,
		Timeout:    flags.Timeout,
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
		PrefixFormatter: func(path string, cmd *config.Command, groups []*config.Group) string {
			colorName := resolveColor(cmd.Color, cmd.InheritColor, groups)
			if colorName == "" {
				colorName = hashColor(path)
			}
			return colorize(fmt.Sprintf("[%s]", path), colorName)
		},
	}
	return engine.Run(ctx, cfg, remainingArgs, opts)
}
