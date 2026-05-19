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

// PrefixWriter wraps an io.Writer and prefixes each line with a task name.
type PrefixWriter struct {
	Writer io.Writer
	Prefix string
	buf    []byte
}

func (pw *PrefixWriter) Write(p []byte) (n int, err error) {
	for i, b := range p {
		pw.buf = append(pw.buf, b)
		if b == '\n' {
			if _, err := fmt.Fprintf(pw.Writer, "%s %s", pw.Prefix, pw.buf); err != nil {
				return i, err
			}
			pw.buf = pw.buf[:0]
		}
	}
	return len(p), nil
}

// Flush writes any remaining buffered bytes without a trailing newline.
func (pw *PrefixWriter) Flush() error {
	if len(pw.buf) > 0 {
		if _, err := fmt.Fprintf(pw.Writer, "%s %s\n", pw.Prefix, pw.buf); err != nil {
			return err
		}
		pw.buf = pw.buf[:0]
	}
	return nil
}

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

// assignOutput assigns stdout/stderr to cmd, preserving TTY detection when possible.
func assignOutput(cmd *exec.Cmd, stdoutWriters, stderrWriters []io.Writer) {
	if len(stdoutWriters) == 1 && stdoutWriters[0] == os.Stdout {
		cmd.Stdout = os.Stdout
	} else {
		cmd.Stdout = io.MultiWriter(stdoutWriters...)
	}

	if len(stderrWriters) == 1 && stderrWriters[0] == os.Stderr {
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stderr = io.MultiWriter(stderrWriters...)
	}
}

// executeShell runs a script in shell with environment variables and optional tee logging.
// If stdout/stderr are nil, os.Stdout/os.Stderr are used.
func executeShell(ctx context.Context, script string, env []string, shell string, baseDir, workingDir, dir string, logs []config.LogConfig, interpCtx InterpolationContext, dryRun bool, stdout, stderr io.Writer) error {
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
	if stdout != nil {
		stdoutWriters = []io.Writer{stdout}
	}
	if stderr != nil {
		stderrWriters = []io.Writer{stderr}
	}

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

	needsPTY := len(stdoutWriters) != 1 || stdoutWriters[0] != os.Stdout ||
		len(stderrWriters) != 1 || stderrWriters[0] != os.Stderr

	var err error
	if needsPTY {
		stdoutMW := io.MultiWriter(stdoutWriters...)
		stderrMW := io.MultiWriter(stderrWriters...)
		used, ptyErr := runWithPTY(cmd, stdoutMW, stderrMW)
		if !used {
			assignOutput(cmd, stdoutWriters, stderrWriters)
			err = cmd.Run()
		} else {
			err = ptyErr
		}
	} else {
		assignOutput(cmd, stdoutWriters, stderrWriters)
		err = cmd.Run()
	}
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

	var execErr error
	failed := false

	runStage := func(name, script string) error {
		interpolated, err := Interpolate(script, interpCtx)
		if err != nil {
			return fmt.Errorf("%s hook interpolation failed: %w", name, err)
		}
		if opts.Verbose {
			fmt.Printf("[verbose] %s: %s\n", name, interpolated)
		}
		if opts.DryRun {
			fmt.Printf("[dry-run] %s:\n%s\n", name, interpolated)
		}
		return executeShell(ctx, interpolated, env, shell, opts.ConfigDir, opts.WorkingDir, dir, opts.Logs, interpCtx, opts.DryRun, nil, nil)
	}

	// before hook
	if cmd.Before != "" {
		if err := runStage("before", cmd.Before); err != nil {
			execErr = fmt.Errorf("before hook failed: %w", err)
			failed = true
		}
	}

	// script
	if !failed && cmd.Script != "" {
		if err := runStage("script", cmd.Script); err != nil {
			execErr = err
			failed = true
		}
	}

	// after hook — runs only if before and script succeeded
	if !failed && cmd.After != "" {
		if err := runStage("after", cmd.After); err != nil {
			execErr = fmt.Errorf("after hook failed: %w", err)
			failed = true
		}
	}

	// fallback hook — runs on any error in before/script/after
	if failed && cmd.Fallback != "" {
		if err := runStage("fallback", cmd.Fallback); err != nil {
			fmt.Fprintf(os.Stderr, "fallback hook failed: %v\n", err)
		}
	}

	// finally hook — always runs at the end
	if cmd.Finally != "" {
		if err := runStage("finally", cmd.Finally); err != nil {
			fmt.Fprintf(os.Stderr, "finally hook failed: %v\n", err)
		}
	}

	return execErr
}

// ExecuteCommandWithPrefix is like ExecuteCommand but prefixes each line of output with the given prefix.
func ExecuteCommandWithPrefix(ctx context.Context, cmd *config.Command, interpCtx InterpolationContext, opts RunOptions, shell string, dir string, prefix string) error {
	stdout := &PrefixWriter{Writer: os.Stdout, Prefix: prefix}
	stderr := &PrefixWriter{Writer: os.Stderr, Prefix: prefix}
	defer func() { _ = stdout.Flush() }()
	defer func() { _ = stderr.Flush() }()

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

	var execErr error
	failed := false

	runStage := func(name, script string) error {
		interpolated, err := Interpolate(script, interpCtx)
		if err != nil {
			return fmt.Errorf("%s hook interpolation failed: %w", name, err)
		}
		if opts.Verbose {
			fmt.Printf("[verbose] %s: %s\n", name, interpolated)
		}
		if opts.DryRun {
			fmt.Printf("[dry-run] %s:\n%s\n", name, interpolated)
		}
		return executeShell(ctx, interpolated, env, shell, opts.ConfigDir, opts.WorkingDir, dir, opts.Logs, interpCtx, opts.DryRun, stdout, stderr)
	}

	// before hook
	if cmd.Before != "" {
		if err := runStage("before", cmd.Before); err != nil {
			execErr = fmt.Errorf("before hook failed: %w", err)
			failed = true
		}
	}

	// script
	if !failed && cmd.Script != "" {
		if err := runStage("script", cmd.Script); err != nil {
			execErr = err
			failed = true
		}
	}

	// after hook — runs only if before and script succeeded
	if !failed && cmd.After != "" {
		if err := runStage("after", cmd.After); err != nil {
			execErr = fmt.Errorf("after hook failed: %w", err)
			failed = true
		}
	}

	// fallback hook — runs on any error in before/script/after
	if failed && cmd.Fallback != "" {
		if err := runStage("fallback", cmd.Fallback); err != nil {
			fmt.Fprintf(os.Stderr, "%s fallback hook failed: %v\n", prefix, err)
		}
	}

	// finally hook — always runs at the end
	if cmd.Finally != "" {
		if err := runStage("finally", cmd.Finally); err != nil {
			fmt.Fprintf(os.Stderr, "%s finally hook failed: %v\n", prefix, err)
		}
	}

	return execErr
}
