package runner

import (
	"fmt"
	"lota/config"
	"os"
	"os/exec"
	"strings"
)

// RunOptions controls execution behavior
type RunOptions struct {
	Verbose bool
	DryRun  bool
}

// executeShell runs a script in shell with environment variables
func executeShell(script string, env []string, shell string) error {
	// Split shell command and flags (e.g., "bash -c" -> ["bash", "-c"])
	parts := strings.Fields(shell)
	if len(parts) == 0 {
		return fmt.Errorf("empty shell command")
	}
	cmd := exec.Command(parts[0], append(parts[1:], script)...)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func ExecuteCommand(cmd *config.Command, context InterpolationContext, opts RunOptions, shell string) error {
	env := VarsToEnv(context.Vars)

	if opts.Verbose {
		fmt.Printf("[verbose] command: %s\n", cmd.Name)
		fmt.Println("[verbose] vars:")
		for k, v := range context.Vars {
			fmt.Printf("  %s=%s\n", k, v)
		}
		fmt.Println("[verbose] args:")
		for k, v := range context.Args {
			fmt.Printf("  %s=%s\n", k, v)
		}
	}

	// before hook
	if cmd.Before != "" {
		interpolatedBefore, err := Interpolate(cmd.Before, context)
		if err != nil {
			return fmt.Errorf("before hook interpolation failed: %w", err)
		}
		if opts.Verbose {
			fmt.Printf("[verbose] before: %s\n", interpolatedBefore)
		}
		if opts.DryRun {
			fmt.Printf("[dry-run] before:\n%s\n", interpolatedBefore)
		} else {
			if err := executeShell(interpolatedBefore, env, shell); err != nil {
				return fmt.Errorf("before hook failed: %w", err)
			}
		}
	}

	// script
	if cmd.Script != "" {
		interpolatedScript, err := Interpolate(cmd.Script, context)
		if err != nil {
			return err
		}
		if opts.Verbose {
			fmt.Printf("[verbose] script: %s\n", interpolatedScript)
		}
		if opts.DryRun {
			fmt.Printf("[dry-run] script:\n%s\n", interpolatedScript)
			return nil
		}
		if err := executeShell(interpolatedScript, env, shell); err != nil {
			return err
		}
	}

	// after hook
	if cmd.After != "" {
		interpolatedAfter, err := Interpolate(cmd.After, context)
		if err != nil {
			return fmt.Errorf("after hook interpolation failed: %w", err)
		}
		if opts.Verbose {
			fmt.Printf("[verbose] after: %s\n", interpolatedAfter)
		}
		if opts.DryRun {
			fmt.Printf("[dry-run] after:\n%s\n", interpolatedAfter)
			return nil
		}
		if err := executeShell(interpolatedAfter, env, shell); err != nil {
			return fmt.Errorf("after hook failed: %w", err)
		}
	}

	return nil
}
