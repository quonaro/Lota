package runner

import (
	"context"
	"fmt"
	"io"
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
	Logs       []config.LogConfig
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

// openLogFile opens a log file with the given path and truncate flag.
// Returns the file and a bool indicating success (false if skipped due to error).
func openLogFile(path string, truncate bool, dryRun bool) (*os.File, bool) {
	if dryRun {
		fmt.Printf("[dry-run] log: %s\n", path)
		return nil, false
	}

	// Ensure parent directories exist
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "[log error] %s: failed to create parent directories: %v\n", path, err)
		return nil, false
	}

	// Verify it's not a directory
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		fmt.Fprintf(os.Stderr, "[log error] %s: path is a directory\n", path)
		return nil, false
	}

	flag := os.O_CREATE | os.O_WRONLY
	if truncate {
		flag |= os.O_TRUNC
	} else {
		flag |= os.O_APPEND
	}

	f, err := os.OpenFile(path, flag, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[log error] %s: %v\n", path, err)
		return nil, false
	}

	return f, true
}

// closeLogFiles closes all opened log files, swallowing errors.
func closeLogFiles(files []*os.File) {
	for _, f := range files {
		if f != nil {
			_ = f.Close()
		}
	}
}

// executeShell runs a script in shell with environment variables and optional tee logging.
func executeShell(ctx context.Context, script string, env []string, shell string, baseDir, workingDir, dir string, logs []config.LogConfig, interpCtx InterpolationContext, dryRun bool) error {
	// In dry-run mode, only print log targets and skip execution
	if dryRun {
		for _, logCfg := range logs {
			interpolatedPath, err := Interpolate(logCfg.Path, interpCtx)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[dry-run] log error: %s: interpolation failed: %v\n", logCfg.Path, err)
				continue
			}
			if !filepath.IsAbs(interpolatedPath) {
				interpolatedPath = filepath.Join(baseDir, interpolatedPath)
			}
			fmt.Printf("[dry-run] log: %s\n", interpolatedPath)
		}
		return nil
	}

	// Split shell command and flags (e.g., "bash -c" -> ["bash", "-c"])
	parts := strings.Fields(shell)
	if len(parts) == 0 {
		return fmt.Errorf("empty shell command")
	}
	cmd := exec.CommandContext(ctx, parts[0], append(parts[1:], script)...)
	cmd.Env = append(os.Environ(), env...)
	cmd.Dir = resolveDir(baseDir, workingDir, dir)

	// Resolve log paths and open files
	var logFiles []*os.File
	stdoutWriters := []io.Writer{os.Stdout}
	stderrWriters := []io.Writer{os.Stderr}

	for _, logCfg := range logs {
		interpolatedPath, err := Interpolate(logCfg.Path, interpCtx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[log error] %s: interpolation failed: %v\n", logCfg.Path, err)
			continue
		}
		// Resolve relative paths against ConfigDir
		if !filepath.IsAbs(interpolatedPath) {
			interpolatedPath = filepath.Join(baseDir, interpolatedPath)
		}
		f, ok := openLogFile(interpolatedPath, logCfg.Truncate, dryRun)
		if ok {
			logFiles = append(logFiles, f)
			stdoutWriters = append(stdoutWriters, f)
			stderrWriters = append(stderrWriters, f)
		}
	}

	cmd.Stdout = io.MultiWriter(stdoutWriters...)
	cmd.Stderr = io.MultiWriter(stderrWriters...)

	err := cmd.Run()
	closeLogFiles(logFiles)

	if err != nil {
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
		}
		if err := executeShell(ctx, interpolatedBefore, env, shell, opts.ConfigDir, opts.WorkingDir, dir, opts.Logs, interpCtx, opts.DryRun); err != nil {
			return fmt.Errorf("before hook failed: %w", err)
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
		}
		if err := executeShell(ctx, interpolatedScript, env, shell, opts.ConfigDir, opts.WorkingDir, dir, opts.Logs, interpCtx, opts.DryRun); err != nil {
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
		}
		if err := executeShell(ctx, interpolatedAfter, env, shell, opts.ConfigDir, opts.WorkingDir, dir, opts.Logs, interpCtx, opts.DryRun); err != nil {
			if scriptErr != nil {
				fmt.Fprintf(os.Stderr, "after hook failed: %v\n", err)
			} else {
				scriptErr = fmt.Errorf("after hook failed: %w", err)
			}
		}
	}

	return scriptErr
}
