package runner

import (
	"context"
	"fmt"
	"lota/config"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// RunOptions controls execution behavior
type RunOptions struct {
	Verbose   bool
	DryRun    bool
	ConfigDir string        // base directory for resolving relative dir paths
	Timeout   time.Duration // 0 means no timeout
}

// ShellError represents a non-zero exit from a shell command.
type ShellError struct {
	ExitCode int
	Command  string
}

func (e *ShellError) Error() string {
	return fmt.Sprintf("command %q exited with code %d", e.Command, e.ExitCode)
}

// executeShell runs a script in shell with environment variables
func executeShell(ctx context.Context, script string, env []string, shell string, baseDir, dir string) error {
	// Split shell command and flags (e.g., "bash -c" -> ["bash", "-c"])
	parts := strings.Fields(shell)
	if len(parts) == 0 {
		return fmt.Errorf("empty shell command")
	}
	cmd := exec.CommandContext(ctx, parts[0], append(parts[1:], script)...)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if dir != "" {
		cmd.Dir = filepath.Join(baseDir, dir)
	}
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return &ShellError{
				ExitCode: exitErr.ExitCode(),
				Command:  parts[0] + " -c",
			}
		}
		return err
	}
	return nil
}

func ExecuteCommand(ctx context.Context, cmd *config.Command, interpCtx InterpolationContext, opts RunOptions, shell string, dir string) error {
	unified := MergeVarsAndArgs(interpCtx.Vars, interpCtx.Args)
	env := VarsToEnv(unified)

	if opts.Verbose {
		fmt.Printf("[verbose] command: %s\n", cmd.Name)
		fmt.Println("[verbose] env:")
		for k, v := range unified {
			fmt.Printf("  %s=%s\n", k, v)
		}
	}

	// Apply timeout if specified
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	var scriptErr error

	// before hook
	if cmd.Before != "" {
		interpolatedBefore, err := Interpolate(cmd.Before, interpCtx)
		if err != nil {
			return fmt.Errorf("before hook interpolation failed: %w", err)
		}
		if opts.Verbose {
			fmt.Printf("[verbose] before: %s\n", interpolatedBefore)
		}
		if opts.DryRun {
			fmt.Printf("[dry-run] before:\n%s\n", interpolatedBefore)
		} else {
			if err := executeShell(ctx, interpolatedBefore, env, shell, opts.ConfigDir, dir); err != nil {
				return fmt.Errorf("before hook failed: %w", err)
			}
		}
	}

	// script
	if cmd.Script != "" {
		interpolatedScript, err := Interpolate(cmd.Script, interpCtx)
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
		if err := executeShell(ctx, interpolatedScript, env, shell, opts.ConfigDir, dir); err != nil {
			scriptErr = err
		}
	}

	// after hook — runs always unless before failed (before failure returns early)
	if cmd.After != "" {
		interpolatedAfter, err := Interpolate(cmd.After, interpCtx)
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
		if err := executeShell(ctx, interpolatedAfter, env, shell, opts.ConfigDir, dir); err != nil {
			if scriptErr != nil {
				fmt.Fprintf(os.Stderr, "after hook failed: %v\n", err)
			} else {
				scriptErr = fmt.Errorf("after hook failed: %w", err)
			}
		}
	}

	return scriptErr
}
