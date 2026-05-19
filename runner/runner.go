package runner

import (
	"context"
	"fmt"
	"lota/config"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// RunOptions controls execution behavior
type RunOptions struct {
	Verbose    bool
	DryRun     bool
	ConfigDir  string        // base directory for resolving relative dir paths
	WorkingDir string        // caller's current working directory
	Timeout    time.Duration // 0 means no timeout
}

// ShellError represents a non-zero exit from a shell command.
type ShellError struct {
	ExitCode int
	Command  string
}

func (e *ShellError) Error() string {
	return fmt.Sprintf("command %s exited with code %d", e.Command, e.ExitCode)
}

// resolveDir determines the working directory for a command.
// - empty dir        → ConfigDir
// - $CWD             → WorkingDir
// - $CWD/...         → WorkingDir + remainder
// - anything else    → ConfigDir + dir
func resolveDir(baseDir, workingDir, dir string) string {
	if dir == "" {
		return baseDir
	}
	if dir == "$CWD" {
		return workingDir
	}
	if strings.HasPrefix(dir, "$CWD/") {
		return filepath.Join(workingDir, strings.TrimPrefix(dir, "$CWD/"))
	}
	return filepath.Join(baseDir, dir)
}

// executeShell runs a script in shell with environment variables
func executeShell(ctx context.Context, script string, env []string, shell string, baseDir, workingDir, dir string) error {
	// Split shell command and flags (e.g., "bash -c" -> ["bash", "-c"])
	parts := strings.Fields(shell)
	if len(parts) == 0 {
		return fmt.Errorf("empty shell command")
	}
	cmd := exec.CommandContext(ctx, parts[0], append(parts[1:], script)...)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = resolveDir(baseDir, workingDir, dir)
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return &ShellError{
				ExitCode: exitErr.ExitCode(),
				Command:  summarizeShellCommand(parts, script),
			}
		}
		return err
	}
	return nil
}

func summarizeShellCommand(shellParts []string, script string) string {
	trimmed := strings.TrimSpace(script)
	if trimmed == "" {
		return strings.Join(shellParts, " ")
	}
	trimmed = strings.ReplaceAll(trimmed, "\n", " ")
	trimmed = strings.Join(strings.Fields(trimmed), " ")
	if len(trimmed) > 80 {
		trimmed = trimmed[:80] + "..."
	}
	return trimmed
}

func sortedMapKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func ExecuteCommand(ctx context.Context, cmd *config.Command, interpCtx InterpolationContext, opts RunOptions, shell string, dir string) error {
	unified := MergeVarsAndArgs(interpCtx.Vars, interpCtx.Args)
	env := VarsToEnv(unified)
	envKeys := sortedMapKeys(unified)

	if opts.Verbose {
		fmt.Printf("[verbose] command: %s\n", cmd.Name)
		fmt.Println("[verbose] env:")
		for _, k := range envKeys {
			fmt.Printf("  %s=%s\n", k, unified[k])
		}
	}

	if opts.DryRun {
		fmt.Println("[dry-run] env:")
		for _, k := range envKeys {
			fmt.Printf("  %s=%s\n", k, unified[k])
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
			if err := executeShell(ctx, interpolatedBefore, env, shell, opts.ConfigDir, opts.WorkingDir, dir); err != nil {
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
		} else if err := executeShell(ctx, interpolatedScript, env, shell, opts.ConfigDir, opts.WorkingDir, dir); err != nil {
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
		} else if err := executeShell(ctx, interpolatedAfter, env, shell, opts.ConfigDir, opts.WorkingDir, dir); err != nil {
			if scriptErr != nil {
				fmt.Fprintf(os.Stderr, "after hook failed: %v\n", err)
			} else {
				scriptErr = fmt.Errorf("after hook failed: %w", err)
			}
		}
	}

	return scriptErr
}
