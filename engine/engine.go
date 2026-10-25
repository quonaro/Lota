package engine

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/quonaro/lota/config"
	"github.com/quonaro/lota/internal/levenshtein"
	"github.com/quonaro/lota/logger"
	"github.com/quonaro/lota/runner"
)

// Options controls execution behavior for embedded usage.
type Options struct {
	Verbose         bool
	DryRun          bool
	ConfigDir       string
	WorkingDir      string
	Timeout         time.Duration
	Stdin           io.Reader
	Stdout          io.Writer
	Stderr          io.Writer
	PrefixFormatter func(path string, cmd *config.Command, groups []*config.Group) string
	NativeHandlers  map[string]NativeFunc // map of full command path to handler
	// AutoHelp makes Run print help and return nil instead of failing when
	// there is nothing to execute: no arguments, a -h/--help flag, or a path
	// that resolves to a group rather than a command. This lets embedded
	// binaries behave like the standalone CLI without boilerplate.
	AutoHelp bool
}

func (o Options) formatPrefix(path string, cmd *config.Command, groups []*config.Group) string {
	if o.PrefixFormatter != nil {
		return o.PrefixFormatter(path, cmd, groups)
	}
	return fmt.Sprintf("[%s]", path)
}

func (o Options) toRunOptions() runner.RunOptions {
	return runner.RunOptions{
		Verbose:    o.Verbose,
		DryRun:     o.DryRun,
		ConfigDir:  o.ConfigDir,
		WorkingDir: o.WorkingDir,
		Timeout:    o.Timeout,
		Stdin:      o.Stdin,
		Stdout:     o.Stdout,
		Stderr:     o.Stderr,
	}
}

// suggestSimilarCommand finds a command similar to the unknown one using Levenshtein distance
func suggestSimilarCommand(unknown string, commands []string) string {
	best := ""
	bestScore := 9999
	for _, cmd := range commands {
		dist := levenshtein.Distance(unknown, cmd)
		if dist < bestScore {
			bestScore = dist
			best = cmd
		}
	}
	maxLen := max(len(unknown), len(best))
	if maxLen == 0 {
		return ""
	}
	normalized := float64(bestScore) / float64(maxLen)
	if normalized <= 0.5 {
		return best
	}
	return ""
}

// LoadConfig parses YAML data, builds indexes, and validates the configuration.
func LoadConfig(data []byte) (*config.AppConfig, error) {
	logger.Debug("engine: loading config from bytes")
	cfg, err := config.ParseConfigFromBytes(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if err := cfg.BuildIndexes(); err != nil {
		return nil, fmt.Errorf("failed to build indexes: %w", err)
	}
	logger.Debug("engine: indexes built successfully")

	validator := config.GetValidator(cfg, "")
	result := validator.Validate()
	if result.Error != nil {
		return nil, result.Error
	}

	return cfg, nil
}

// LoadConfigFromPath reads a config file from the given path, parses it,
// builds indexes, validates it, and returns both the config and its directory.
// The returned directory can be used as engine.Options.ConfigDir.
func LoadConfigFromPath(path string) (*config.AppConfig, string, error) {
	fc, err := config.GetConfigPath(path)
	if err != nil {
		return nil, "", fmt.Errorf("failed to resolve config path: %w", err)
	}

	cfg, err := config.ParseConfigWithWriter(fc.Path, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to parse config: %w", err)
	}

	validator := config.GetValidator(cfg, fc.Path)
	result := validator.Validate()
	if result.Error != nil {
		return nil, "", result.Error
	}

	return cfg, filepath.Dir(fc.Path), nil
}

// Run is the CLI-style entrypoint. args contains the full command line,
// for example []string{"deploy", "prod", "--force"}.
func Run(ctx context.Context, cfg *config.AppConfig, args []string, opts Options) error {
	return run(ctx, cfg, args, opts, "")
}

// run is the shared implementation behind Run and App.Run. appName is only
// used when opts.AutoHelp renders help; an empty value defaults to "app".
func run(ctx context.Context, cfg *config.AppConfig, args []string, opts Options, appName string) error {
	logger.Debugf("engine: Run called with args: %v", args)
	if len(args) == 0 {
		if opts.AutoHelp {
			PrintHelp(cfg, helpWriter(opts), appName)
			return nil
		}
		return fmt.Errorf("no command specified. Use --help to see available commands")
	}

	if opts.AutoHelp && hasHelpFlag(args) {
		return printHelpForArgs(cfg, args, opts, appName)
	}

	result, remainingArgs, _ := config.ResolveCommand(cfg, args)
	if !result.Exists {
		suggestion := suggestSimilarCommand(args[0], cfg.GetAllCommandNames())
		if suggestion != "" {
			return fmt.Errorf("command not found: %s. Did you mean: %s?", args[0], suggestion)
		}
		return fmt.Errorf("command not found: %s", args[0])
	}
	logger.Debugf("engine: command resolved, executing")
	if result.Command == nil {
		if opts.AutoHelp {
			PrintGroupHelp(cfg, result.Groups, helpWriter(opts), appName)
			return nil
		}
		return &GroupError{
			Path:   strings.Join(args, " "),
			Groups: result.Groups,
		}
	}

	return runCommand(ctx, cfg, result, remainingArgs, opts)
}

// helpWriter returns the writer help should be rendered to, defaulting to
// os.Stdout so AutoHelp never panics on an unset Options.Stdout.
func helpWriter(opts Options) io.Writer {
	if opts.Stdout != nil {
		return opts.Stdout
	}
	return os.Stdout
}

// hasHelpFlag reports whether -h or --help appears anywhere in args.
func hasHelpFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			return true
		}
	}
	return false
}

// printHelpForArgs renders the most specific help for a command line that
// contains a help flag: command help, group help, or top-level help.
func printHelpForArgs(cfg *config.AppConfig, args []string, opts Options, appName string) error {
	filtered := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			continue
		}
		filtered = append(filtered, arg)
	}
	if len(filtered) == 0 {
		PrintHelp(cfg, helpWriter(opts), appName)
		return nil
	}

	result, _, _ := config.ResolveCommand(cfg, filtered)
	switch {
	case result.Command != nil:
		PrintCommandHelp(cfg, result, helpWriter(opts), appName, opts.Verbose)
	case len(result.Groups) > 0:
		PrintGroupHelp(cfg, result.Groups, helpWriter(opts), appName)
	default:
		PrintHelp(cfg, helpWriter(opts), appName)
	}
	return nil
}

// RunCommand is the programmatic entrypoint. path is a dot-separated command
// path (e.g., "deploy.prod"), and cmdArgs are the command-specific arguments.
func RunCommand(ctx context.Context, cfg *config.AppConfig, path string, cmdArgs []string, opts Options) error {
	logger.Debugf("engine: RunCommand called with path: %s", path)
	result, err := config.FindCommandByPath(cfg, path)
	if err != nil {
		return err
	}
	return runCommand(ctx, cfg, result, cmdArgs, opts)
}

func runCommand(ctx context.Context, cfg *config.AppConfig, result config.SearchResult, cliArgs []string, opts Options) error {
	logger.Debugf("engine: runCommand for: %s", result.Command.Name)
	if result.Command == nil {
		return fmt.Errorf("not a command")
	}

	levels, err := resolveDependencyLevels(cfg, result)
	if err != nil {
		return err
	}
	logger.Debugf("engine: resolved %d dependency levels", len(levels))

	parallel := result.Command.Parallel == nil || *result.Command.Parallel

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var firstErr atomic.Pointer[error]
	// svcWg tracks service dependencies (wait: started/ready) that keep
	// running in the background past their satisfaction point.
	var svcWg sync.WaitGroup

	if parallel {
		var deps []config.SearchResult
		for _, level := range levels {
			deps = append(deps, level...)
		}
		if err := runParallelDependencies(ctx, cfg, result, deps, opts, &firstErr, cancel, &svcWg); err != nil {
			cancel()
			svcWg.Wait()
			return err
		}
	} else {
		for _, level := range levels {
			logger.Debugf("engine: processing dependency level with %d commands", len(level))
			if f := firstErr.Load(); f != nil {
				return *f
			}
			if err := ctx.Err(); err != nil {
				return err
			}

			if err := runDependencyLevel(ctx, cfg, level, opts, &firstErr, cancel, &svcWg); err != nil {
				cancel()
				svcWg.Wait()
				return err
			}
		}
	}

	if f := firstErr.Load(); f != nil {
		return *f
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	args := runner.ResolveArgs(*cfg, result.Groups, *result.Command)
	shell := runner.ResolveShell(*cfg, result.Groups, *result.Command)
	parsedArgs, err := runner.ParseArgs(cliArgs, args)
	if err != nil {
		return err
	}
	vars := runner.ResolveVars(*cfg, result.Groups, *result.Command)
	logger.Debugf("engine: resolved %d vars, shell: %s", len(vars), shell)

	interpCtx := runner.InterpolationContext{
		Vars:       vars,
		Args:       parsedArgs,
		ArgDefs:    args,
		WarnWriter: opts.Stderr,
	}

	logs := runner.ResolveLogs(*cfg, result.Groups, *result.Command)
	runOpts := opts.toRunOptions()
	runOpts.Logs = logs

	dir := runner.ResolveDir(*cfg, result.Groups, *result.Command)

	var runErr error
	if result.Command.Native {
		logger.Debug("engine: executing native command")
		cmdPath := config.CommandPath(result.Command, result.Groups)
		runErr = runNative(ctx, cmdPath, NativeContext{
			Vars:   vars,
			Args:   parsedArgs,
			Stdout: runOpts.Stdout,
			Stderr: runOpts.Stderr,
		}, opts.NativeHandlers)
	} else {
		logger.Debug("engine: executing shell command")
		runErr = runner.ExecuteCommand(ctx, result.Command, interpCtx, runOpts, shell, dir)
	}

	// The run stays alive while service dependencies (wait: started/ready)
	// are still running — like `docker compose up`. On failure we cancel
	// them explicitly; on success we wait until they exit or ctx is done.
	if runErr != nil {
		cancel()
	}
	svcWg.Wait()

	// A failed service dep predates a context-canceled main command.
	if f := firstErr.Load(); f != nil {
		return *f
	}
	if runErr != nil {
		return runErr
	}
	return ctx.Err()
}

// prepareCommand resolves args, vars, shell, logs, and working directory for
// a command — shared between normal execution and readiness probes.
func prepareCommand(cfg *config.AppConfig, result config.SearchResult, opts Options) (runner.InterpolationContext, runner.RunOptions, string, string, error) {
	args := runner.ResolveArgs(*cfg, result.Groups, *result.Command)
	shell := runner.ResolveShell(*cfg, result.Groups, *result.Command)
	parsedArgs, err := runner.ParseArgs([]string{}, args)
	if err != nil {
		return runner.InterpolationContext{}, runner.RunOptions{}, "", "", err
	}
	vars := runner.ResolveVars(*cfg, result.Groups, *result.Command)

	interpCtx := runner.InterpolationContext{
		Vars:       vars,
		Args:       parsedArgs,
		ArgDefs:    args,
		WarnWriter: opts.Stderr,
	}

	runOpts := opts.toRunOptions()
	runOpts.Logs = runner.ResolveLogs(*cfg, result.Groups, *result.Command)
	dir := runner.ResolveDir(*cfg, result.Groups, *result.Command)

	return interpCtx, runOpts, shell, dir, nil
}

func executeSingleCommand(ctx context.Context, cfg *config.AppConfig, result config.SearchResult, opts Options, prefix string) error {
	interpCtx, runOpts, shell, dir, err := prepareCommand(cfg, result, opts)
	if err != nil {
		return err
	}

	if result.Command.Native {
		if prefix != "" {
			_, _ = fmt.Fprintf(runOpts.Stdout, "%s ", prefix)
		}
		cmdPath := config.CommandPath(result.Command, result.Groups)
		return runNative(ctx, cmdPath, NativeContext{
			Vars:   interpCtx.Vars,
			Args:   interpCtx.Args,
			Stdout: runOpts.Stdout,
			Stderr: runOpts.Stderr,
		}, opts.NativeHandlers)
	}

	if prefix != "" {
		return runner.ExecuteCommandWithPrefix(ctx, result.Command, interpCtx, runOpts, shell, dir, prefix)
	}
	return runner.ExecuteCommand(ctx, result.Command, interpCtx, runOpts, shell, dir)
}

// depWaitMode resolves the effective wait mode for a dependency edge:
// explicit wait wins; otherwise "ready" when the target declares a ready
// check, "done" otherwise.
func depWaitMode(dep config.SearchResult) string {
	if dep.Wait != "" {
		return dep.Wait
	}
	if dep.Command.Ready != nil {
		return config.WaitReady
	}
	return config.WaitDone
}

// dependencyExecution tracks one command and the prerequisites that gate it.
type dependencyExecution struct {
	result        config.SearchResult
	prerequisites []*dependencyExecution
	done          chan struct{}
	err           error
}

// runParallelDependencies starts independent branches immediately while each
// command waits only for its own prerequisites. A slow sibling must not block
// a branch whose prerequisites have already been satisfied.
func runParallelDependencies(ctx context.Context, cfg *config.AppConfig, root config.SearchResult, deps []config.SearchResult, opts Options, firstErr *atomic.Pointer[error], cancel context.CancelFunc, svcWg *sync.WaitGroup) error {
	states := make(map[string]*dependencyExecution, len(deps))
	for _, dep := range deps {
		path := config.CommandPath(dep.Command, dep.Groups)
		states[path] = &dependencyExecution{
			result: dep,
			done:   make(chan struct{}),
		}
	}

	for path, state := range states {
		for _, prerequisite := range state.result.Command.Depends {
			result, err := config.FindCommandByPath(cfg, prerequisite.Name)
			if err != nil {
				return fmt.Errorf("dependency %q of %s: %w", prerequisite.Name, path, err)
			}
			prerequisitePath := config.CommandPath(result.Command, result.Groups)
			prerequisiteState, ok := states[prerequisitePath]
			if !ok {
				return fmt.Errorf("dependency %q of %s was not resolved", prerequisite.Name, path)
			}
			state.prerequisites = append(state.prerequisites, prerequisiteState)
		}
	}

	for path, state := range states {
		path, state := path, state
		go func() {
			defer close(state.done)
			for _, prerequisite := range state.prerequisites {
				select {
				case <-prerequisite.done:
					if prerequisite.err != nil {
						state.err = prerequisite.err
						return
					}
				case <-ctx.Done():
					state.err = ctx.Err()
					return
				}
			}
			if err := ctx.Err(); err != nil {
				state.err = err
				return
			}

			prefix := opts.formatPrefix(path, state.result.Command, state.result.Groups)
			if err := runDependency(ctx, cfg, state.result, opts, prefix, firstErr, cancel, svcWg); err != nil {
				if ctx.Err() != nil {
					state.err = ctx.Err()
					return
				}
				wrapped := fmt.Errorf("dependency %s failed: %w", path, err)
				state.err = wrapped
				firstErr.CompareAndSwap(nil, &wrapped)
				cancel()
			}
		}()
	}

	for _, dependency := range root.Command.Depends {
		result, err := config.FindCommandByPath(cfg, dependency.Name)
		if err != nil {
			return fmt.Errorf("dependency %q of %s: %w", dependency.Name, root.Command.Name, err)
		}
		path := config.CommandPath(result.Command, result.Groups)
		state, ok := states[path]
		if !ok {
			return fmt.Errorf("dependency %q of %s was not resolved", dependency.Name, root.Command.Name)
		}
		<-state.done
	}

	if first := firstErr.Load(); first != nil {
		return *first
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

// runDependencyLevel runs all dependencies of one level and blocks until
// each of them is satisfied: done (process exited 0), ready (probe passed),
// or started (process launched). Service dependencies keep running in the
// background; a later non-zero exit fails the whole run via firstErr.
func runDependencyLevel(ctx context.Context, cfg *config.AppConfig, level []config.SearchResult, opts Options, firstErr *atomic.Pointer[error], cancel context.CancelFunc, svcWg *sync.WaitGroup) error {
	for _, dep := range level {
		path := config.CommandPath(dep.Command, dep.Groups)
		prefix := opts.formatPrefix(path, dep.Command, dep.Groups)
		if opts.Stdout != nil {
			_, _ = fmt.Fprintf(opts.Stdout, "=> Running dependency: %s\n", path)
		} else {
			fmt.Printf("=> Running dependency: %s\n", path)
		}
		if err := runDependency(ctx, cfg, dep, opts, prefix, firstErr, cancel, svcWg); err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			return fmt.Errorf("dependency %s failed: %w", path, err)
		}
	}
	return nil
}

// runDependency satisfies a single dependency according to its wait mode and
// reports nil once satisfied. For "started"/"ready" it launches the command
// in the background and keeps monitoring it for late failures.
func runDependency(ctx context.Context, cfg *config.AppConfig, dep config.SearchResult, opts Options, prefix string, firstErr *atomic.Pointer[error], cancel context.CancelFunc, svcWg *sync.WaitGroup) error {
	path := config.CommandPath(dep.Command, dep.Groups)
	mode := depWaitMode(dep)

	if mode == config.WaitDone || opts.DryRun {
		return executeSingleCommand(ctx, cfg, dep, opts, prefix)
	}

	procErr := make(chan error, 1)
	go func() {
		procErr <- executeSingleCommand(ctx, cfg, dep, opts, prefix)
	}()

	if mode == config.WaitReady {
		if err := waitReady(ctx, cfg, dep, opts, procErr); err != nil {
			return err
		}
	}

	// The service stays alive past its satisfied point and keeps the whole
	// run alive (svcWg). If it exits with an error, the whole run fails.
	svcWg.Add(1)
	go func() {
		defer svcWg.Done()
		if err := <-procErr; err != nil && ctx.Err() == nil {
			wrapped := fmt.Errorf("dependency %s failed: %w", path, err)
			firstErr.CompareAndSwap(nil, &wrapped)
			cancel()
		}
	}()
	return nil
}

// waitReady polls the dependency's ready check until it passes, the process
// exits, attempts are exhausted, or ctx is cancelled.
func waitReady(ctx context.Context, cfg *config.AppConfig, dep config.SearchResult, opts Options, procErr chan error) error {
	path := config.CommandPath(dep.Command, dep.Groups)
	rc := dep.Command.Ready
	if rc == nil {
		return fmt.Errorf("dependency %s has no ready check", path)
	}

	interval := rc.Interval
	if interval <= 0 {
		interval = time.Second
	}
	timeout := rc.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	retries := rc.Retries
	if retries <= 0 {
		retries = 30
	}

	interpCtx, _, shell, dir, err := prepareCommand(cfg, dep, opts)
	if err != nil {
		return err
	}
	env := runner.VarsToEnv(runner.MergeVarsAndArgs(interpCtx.Vars, interpCtx.Args))

	script, err := runner.Interpolate(rc.Script, interpCtx)
	if err != nil {
		return fmt.Errorf("ready check interpolation failed: %w", err)
	}

	if opts.Stdout != nil {
		_, _ = fmt.Fprintf(opts.Stdout, "=> Waiting for %s to become ready\n", path)
	}

	startDeadline := time.Now().Add(rc.StartPeriod)
	failures := 0
	for {
		checkCtx, checkCancel := context.WithTimeout(ctx, timeout)
		err := runner.RunCheck(checkCtx, script, env, shell, opts.ConfigDir, opts.WorkingDir, dir)
		checkCancel()

		if err == nil {
			if opts.Stdout != nil {
				_, _ = fmt.Fprintf(opts.Stdout, "=> %s is ready\n", path)
			}
			return nil
		}
		if time.Now().After(startDeadline) {
			failures++
		}
		if failures > retries {
			return fmt.Errorf("did not become ready after %d attempts: %w", failures, err)
		}

		select {
		case perr := <-procErr:
			if perr == nil {
				return fmt.Errorf("exited before becoming ready")
			}
			return fmt.Errorf("exited before becoming ready: %w", perr)
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}

func resolveDependencyLevels(cfg *config.AppConfig, result config.SearchResult) ([][]config.SearchResult, error) {
	deps, err := config.ResolveDependencies(cfg, result)
	if err != nil {
		return nil, err
	}

	depth := make(map[string]int)
	visited := make(map[string]bool)

	var computeDepth func(cmd *config.Command, groups []*config.Group) int
	computeDepth = func(cmd *config.Command, groups []*config.Group) int {
		path := config.CommandPath(cmd, groups)
		if d, ok := depth[path]; ok {
			return d
		}
		if visited[path] {
			return 0 // circular — already detected by ResolveDependencies
		}
		visited[path] = true

		maxDepDepth := -1
		for _, dep := range cmd.Depends {
			depResult, err := config.FindCommandByPath(cfg, dep.Name)
			if err != nil {
				continue
			}
			d := computeDepth(depResult.Command, depResult.Groups)
			if d > maxDepDepth {
				maxDepDepth = d
			}
		}

		visited[path] = false
		d := maxDepDepth + 1
		depth[path] = d
		return d
	}

	for _, dep := range deps {
		computeDepth(dep.Command, dep.Groups)
	}

	maxDepth := -1
	for _, d := range depth {
		if d > maxDepth {
			maxDepth = d
		}
	}

	levels := make([][]config.SearchResult, maxDepth+1)
	for _, dep := range deps {
		d := depth[config.CommandPath(dep.Command, dep.Groups)]
		levels[d] = append(levels[d], dep)
	}

	var resultLevels [][]config.SearchResult
	for _, level := range levels {
		if len(level) > 0 {
			resultLevels = append(resultLevels, level)
		}
	}

	return resultLevels, nil
}
