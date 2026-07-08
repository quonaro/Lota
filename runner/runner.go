package runner

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/quonaro/lota/config"
	"github.com/quonaro/lota/logger"
)

// PrefixWriter wraps an io.Writer and prefixes each line with a task name.
type PrefixWriter struct {
	Writer io.Writer
	Prefix string
	buf    []byte
}

func (pw *PrefixWriter) Write(p []byte) (n int, err error) {
	pw.buf = append(pw.buf, p...)
	if err := pw.flushLines(); err != nil {
		return len(p), err
	}
	return len(p), nil
}

// flushLines writes complete lines from buf to the writer with prefix.
// Handles both \n and \r\n line endings.
func (pw *PrefixWriter) flushLines() error {
	for {
		// Find next newline
		idx := bytes.IndexByte(pw.buf, '\n')
		if idx == -1 {
			break
		}

		// Check for \r\n and exclude the \r
		lineEnd := idx
		if idx > 0 && pw.buf[idx-1] == '\r' {
			lineEnd = idx - 1
		}

		// Write the line with prefix
		line := pw.buf[:lineEnd]
		if _, err := fmt.Fprintf(pw.Writer, "%s %s\n", pw.Prefix, line); err != nil {
			return err
		}

		// Remove the processed line (including \n, and \r if present)
		pw.buf = pw.buf[idx+1:]
	}
	return nil
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
	Verbose      bool
	DryRun       bool
	ConfigDir    string        // base directory for resolving relative dir paths
	WorkingDir   string        // caller's current working directory
	Timeout      time.Duration // 0 means no timeout
	Logs         []config.LogConfig
	ShutdownOnce *sync.Once // ensures shutdown message prints only once
	Stdin        io.Reader  // input reader; defaults to os.Stdin if nil
	Stdout       io.Writer  // output writer; defaults to os.Stdout if nil
	Stderr       io.Writer  // error writer; defaults to os.Stderr if nil
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
// Returns the file and an error if opening failed.
func openLogFile(path string, truncate bool, dryRun bool, stdout io.Writer) (*os.File, error) {
	if dryRun {
		if stdout == nil {
			stdout = os.Stdout
		}
		_, _ = fmt.Fprintf(stdout, "[dry-run] log: %s\n", path)
		return nil, nil
	}

	// Ensure parent directories exist
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("failed to create parent directories for %s: %w", path, err)
	}

	// Verify it's not a directory
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return nil, fmt.Errorf("path %s is a directory", path)
	}

	flag := os.O_CREATE | os.O_WRONLY
	if truncate {
		flag |= os.O_TRUNC
	} else {
		flag |= os.O_APPEND
	}

	f, err := os.OpenFile(path, flag, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file %s: %w", path, err)
	}

	return f, nil
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

// splitShellCommand splits a shell command string into parts, supporting quoted arguments.
// Handles both single and double quotes.
func splitShellCommand(shell string) ([]string, error) {
	var parts []string
	var current strings.Builder
	var inQuote rune
	var escape bool

	for _, r := range shell {
		if escape {
			current.WriteRune(r)
			escape = false
			continue
		}

		switch r {
		case '\\':
			escape = true
		case '"', '\'':
			switch inQuote {
			case r:
				inQuote = 0
			case 0:
				inQuote = r
			default:
				current.WriteRune(r)
			}
		case ' ', '\t':
			if inQuote == 0 {
				if current.Len() > 0 {
					parts = append(parts, current.String())
					current.Reset()
				}
			} else {
				current.WriteRune(r)
			}
		default:
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	if inQuote != 0 {
		return nil, fmt.Errorf("unclosed quote in shell command")
	}

	return parts, nil
}

// buildLogWriters resolves log paths, opens files, and builds writer lists.
// Returns the list of opened files and the combined stdout/stderr writers.
func buildLogWriters(logs []config.LogConfig, interpCtx InterpolationContext, baseDir string, dryRun bool, stdout, stderr io.Writer) ([]*os.File, []io.Writer, []io.Writer) {
	var logFiles []*os.File
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}
	stdoutWriters := []io.Writer{stdout}
	stderrWriters := []io.Writer{stderr}

	for _, logCfg := range logs {
		interpolatedPath, err := Interpolate(logCfg.Path, interpCtx)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "[log error] %s: interpolation failed: %v\n", logCfg.Path, err)
			continue
		}
		if !filepath.IsAbs(interpolatedPath) {
			interpolatedPath = filepath.Join(baseDir, interpolatedPath)
		}
		f, err := openLogFile(interpolatedPath, logCfg.Truncate, dryRun, stdout)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "[log error] %s: %v\n", logCfg.Path, err)
			continue
		}
		if f != nil {
			logFiles = append(logFiles, f)
			stdoutWriters = append(stdoutWriters, f)
			stderrWriters = append(stderrWriters, f)
		}
	}

	return logFiles, stdoutWriters, stderrWriters
}

// executeShell runs a script in shell with environment variables and optional tee logging.
// If stdout/stderr are nil, os.Stdout/os.Stderr are used.
func executeShell(ctx context.Context, script string, env []string, shell string, baseDir, workingDir, dir string, logs []config.LogConfig, interpCtx InterpolationContext, dryRun bool, stdin io.Reader, stdout, stderr io.Writer, shutdownOnce *sync.Once) error {
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}
	if stdin == nil {
		stdin = os.Stdin
	}

	// In dry-run mode, only print log targets and skip execution
	if dryRun {
		for _, logCfg := range logs {
			interpolatedPath, err := Interpolate(logCfg.Path, interpCtx)
			if err != nil {
				_, _ = fmt.Fprintf(stderr, "[dry-run] log error: %s: interpolation failed: %v\n", logCfg.Path, err)
				continue
			}
			if !filepath.IsAbs(interpolatedPath) {
				interpolatedPath = filepath.Join(baseDir, interpolatedPath)
			}
			_, _ = fmt.Fprintf(stdout, "[dry-run] log: %s\n", interpolatedPath)
		}
		return nil
	}

	// Split shell command and flags (e.g., "bash -c" -> ["bash", "-c"])
	// Supports quoted arguments for complex shell commands
	parts, err := splitShellCommand(shell)
	if err != nil {
		return err
	}
	if len(parts) == 0 {
		return fmt.Errorf("empty shell command")
	}
	cmd := exec.Command(parts[0], append(parts[1:], script)...)
	cmd.Env = append(os.Environ(), env...)
	cmd.Dir = resolveDir(baseDir, workingDir, dir)
	cmd.Stdin = stdin
	setupSysProcAttr(cmd)

	logFiles, stdoutWriters, stderrWriters := buildLogWriters(logs, interpCtx, baseDir, dryRun, stdout, stderr)
	defer closeLogFiles(logFiles)

	// If stdin is a terminal, use direct I/O for interactive commands (sudo, etc.)
	// Write script to temp file and execute directly to preserve interactive input
	if IsTerminal(stdin) {
		tmpFile, err := os.CreateTemp("", "lota-script-*.sh")
		if err != nil {
			return fmt.Errorf("create temp script file: %w", err)
		}
		tmpPath := tmpFile.Name()
		// Extract shell name for shebang (e.g., "bash" from "bash -c")
		shellName := strings.Fields(shell)[0]
		// Find the actual path to the shell
		shellPath, err := exec.LookPath(shellName)
		if err != nil {
			_ = tmpFile.Close()
			_ = os.Remove(tmpPath)
			return fmt.Errorf("find shell %s: %w", shellName, err)
		}
		if _, err := tmpFile.WriteString("#!" + shellPath + "\n" + script); err != nil {
			_ = tmpFile.Close()
			_ = os.Remove(tmpPath)
			return fmt.Errorf("write temp script file: %w", err)
		}
		if err := tmpFile.Close(); err != nil {
			_ = os.Remove(tmpPath)
			return fmt.Errorf("close temp script file: %w", err)
		}

		// Make executable
		if err := os.Chmod(tmpPath, 0755); err != nil {
			_ = os.Remove(tmpPath)
			return fmt.Errorf("chmod temp script file: %w", err)
		}

		cmd := exec.Command(tmpPath)
		cmd.Env = append(os.Environ(), env...)
		cmd.Dir = resolveDir(baseDir, workingDir, dir)
		cmd.Stdin = stdin
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		cmd.SysProcAttr = nil

		if err = cmd.Start(); err != nil {
			_ = os.Remove(tmpPath)
			return fmt.Errorf("start command: %w", err)
		}
		if err = gracefulWait(cmd, ctx, shutdownOnce, stderr); err != nil {
			_ = os.Remove(tmpPath)
			return err
		}
		_ = os.Remove(tmpPath)
	} else {
		needsPTY := len(stdoutWriters) != 1 || stdoutWriters[0] != os.Stdout ||
			len(stderrWriters) != 1 || stderrWriters[0] != os.Stderr

		if needsPTY {
			stdoutMW := io.MultiWriter(stdoutWriters...)
			stderrMW := io.MultiWriter(stderrWriters...)
			used, ptyErr := runWithPTY(cmd, stdin, stdoutMW, stderrMW, ctx, shutdownOnce)
			if !used {
				assignOutput(cmd, stdoutWriters, stderrWriters)
				cmd.Stdin = stdin
				if err = cmd.Start(); err != nil {
					return fmt.Errorf("start command: %w", err)
				}
				err = gracefulWait(cmd, ctx, shutdownOnce, stderr)
			} else {
				err = ptyErr
			}
		} else {
			assignOutput(cmd, stdoutWriters, stderrWriters)
			cmd.Stdin = stdin
			if err = cmd.Start(); err != nil {
				return fmt.Errorf("start command: %w", err)
			}
			err = gracefulWait(cmd, ctx, shutdownOnce, stderr)
		}
	}

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

// gracefulWait waits for cmd to finish. If ctx is cancelled, it sends SIGTERM
// to the process group, waits up to 10s, then SIGKILL.
func gracefulWait(cmd *exec.Cmd, ctx context.Context, shutdownOnce *sync.Once, stderr io.Writer) error {
	if stderr == nil {
		stderr = os.Stderr
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		if shutdownOnce != nil {
			shutdownOnce.Do(func() {
				_, _ = fmt.Fprintf(stderr, "\r\033[KПользователь запросил завершение выполнения...\n")
			})
		}
		if cmd.Process != nil {
			_ = terminateProcess(cmd.Process.Pid)
		}

		select {
		case <-done:
			return ctx.Err()
		case <-time.After(10 * time.Second):
			_, _ = fmt.Fprintln(stderr, "Grace period exceeded, force-killing process...")
			if cmd.Process != nil {
				_ = killProcess(cmd.Process.Pid)
			}
			return ctx.Err()
		}
	}
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

func executeCommandInternal(ctx context.Context, cmd *config.Command, interpCtx InterpolationContext, opts RunOptions, shell string, dir string, stdout, stderr io.Writer, prefix string) error {
	logger.Debugf("runner: executing command: %s", cmd.Name)
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}

	unified := MergeVarsAndArgs(interpCtx.Vars, interpCtx.Args)
	env := VarsToEnv(unified)
	envKeys := sortedMapKeys(unified)
	logger.Debugf("runner: resolved %d environment variables", len(envKeys))

	if opts.Verbose {
		_, _ = fmt.Fprintf(stdout, "[verbose] command: %s\n", cmd.Name)
		_, _ = fmt.Fprintln(stdout, "[verbose] env:")
		for _, k := range envKeys {
			_, _ = fmt.Fprintf(stdout, "  %s=%s\n", k, unified[k])
		}
	}

	if opts.DryRun {
		_, _ = fmt.Fprintln(stdout, "[dry-run] env:")
		for _, k := range envKeys {
			_, _ = fmt.Fprintf(stdout, "  %s=%s\n", k, unified[k])
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
		logger.Debugf("runner: running stage: %s", name)
		interpolated, err := Interpolate(script, interpCtx)
		if err != nil {
			return fmt.Errorf("%s hook interpolation failed: %w", name, err)
		}
		if opts.Verbose {
			_, _ = fmt.Fprintf(stdout, "[verbose] %s: %s\n", name, interpolated)
		}
		if opts.DryRun {
			_, _ = fmt.Fprintf(stdout, "[dry-run] %s:\n%s\n", name, interpolated)
		}
		return executeShell(ctx, interpolated, env, shell, opts.ConfigDir, opts.WorkingDir, dir, opts.Logs, interpCtx, opts.DryRun, opts.Stdin, stdout, stderr, opts.ShutdownOnce)
	}

	// before hook
	if cmd.Before != "" {
		logger.Debug("runner: executing before hook")
		if err := runStage("before", cmd.Before); err != nil {
			execErr = fmt.Errorf("before hook failed: %w", err)
			failed = true
		}
	}

	// script
	if !failed && cmd.Script != "" {
		logger.Debug("runner: executing main script")
		if err := runStage("script", cmd.Script); err != nil {
			execErr = err
			failed = true
		}
	}

	// after hook — runs only if before and script succeeded
	if !failed && cmd.After != "" {
		logger.Debug("runner: executing after hook")
		if err := runStage("after", cmd.After); err != nil {
			execErr = fmt.Errorf("after hook failed: %w", err)
			failed = true
		}
	}

	// fallback hook — runs on any error in before/script/after
	if failed && cmd.Fallback != "" {
		logger.Debug("runner: executing fallback hook")
		if err := runStage("fallback", cmd.Fallback); err != nil {
			if prefix != "" {
				_, _ = fmt.Fprintf(stderr, "%s fallback hook failed: %v\n", prefix, err)
			} else {
				_, _ = fmt.Fprintf(stderr, "fallback hook failed: %v\n", err)
			}
		} else {
			// fallback succeeded — command is considered recovered
			execErr = nil
		}
	}

	// finally hook — always runs at the end
	if cmd.Finally != "" {
		logger.Debug("runner: executing finally hook")
		if err := runStage("finally", cmd.Finally); err != nil {
			if prefix != "" {
				_, _ = fmt.Fprintf(stderr, "%s finally hook failed: %v\n", prefix, err)
			} else {
				_, _ = fmt.Fprintf(stderr, "finally hook failed: %v\n", err)
			}
		}
	}

	return execErr
}

func ExecuteCommand(ctx context.Context, cmd *config.Command, interpCtx InterpolationContext, opts RunOptions, shell string, dir string) error {
	logger.Debugf("runner: ExecuteCommand called for: %s", cmd.Name)
	return executeCommandInternal(ctx, cmd, interpCtx, opts, shell, dir, opts.Stdout, opts.Stderr, "")
}

// ExecuteCommandWithPrefix is like ExecuteCommand but prefixes each line of output with the given prefix.
func ExecuteCommandWithPrefix(ctx context.Context, cmd *config.Command, interpCtx InterpolationContext, opts RunOptions, shell string, dir string, prefix string) error {
	logger.Debugf("runner: ExecuteCommandWithPrefix called for: %s with prefix: %s", cmd.Name, prefix)
	stdout := opts.Stdout
	stderr := opts.Stderr
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}
	pstdout := &PrefixWriter{Writer: stdout, Prefix: prefix}
	pstderr := &PrefixWriter{Writer: stderr, Prefix: prefix}
	defer func() { _ = pstdout.Flush() }()
	defer func() { _ = pstderr.Flush() }()

	return executeCommandInternal(ctx, cmd, interpCtx, opts, shell, dir, pstdout, pstderr, prefix)
}
