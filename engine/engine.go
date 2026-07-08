package engine

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/quonaro/lota/config"
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
	Stdout          io.Writer
	Stderr          io.Writer
	PrefixFormatter func(path string, cmd *config.Command, groups []*config.Group) string
	NativeHandlers  map[string]NativeFunc // map of full command path to handler
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
		Stdout:     o.Stdout,
		Stderr:     o.Stderr,
	}
}

// suggestSimilarCommand finds a command similar to the unknown one using Levenshtein distance
func suggestSimilarCommand(unknown string, commands []string) string {
	best := ""
	bestScore := 9999
	for _, cmd := range commands {
		dist := levenshteinDistanceEngine(unknown, cmd)
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

func levenshteinDistanceEngine(a, b string) int {
	if a == b {
		return 0
	}
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)
	for j := 0; j <= len(b); j++ {
		prev[j] = j
	}

	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			insertion := curr[j-1] + 1
			deletion := prev[j] + 1
			substitution := prev[j-1] + cost
			curr[j] = minEngine(insertion, deletion, substitution)
		}
		prev, curr = curr, prev
	}

	return prev[len(b)]
}

func minEngine(a, b, c int) int {
	if a <= b && a <= c {
		return a
	}
	if b <= c {
		return b
	}
	return c
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
	logger.Debugf("engine: Run called with args: %v", args)
	if len(args) == 0 {
		return fmt.Errorf("no command specified. Use --help to see available commands")
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
		return &GroupError{
			Path:   strings.Join(args, " "),
			Groups: result.Groups,
		}
	}

	return runCommand(ctx, cfg, result, remainingArgs, opts)
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

	var allWg sync.WaitGroup
	var firstErr atomic.Pointer[error]

	for _, level := range levels {
		logger.Debugf("engine: processing dependency level with %d commands", len(level))
		if f := firstErr.Load(); f != nil {
			return *f
		}
		if err := ctx.Err(); err != nil {
			return err
		}

		if parallel {
			for _, dep := range level {
				dep := dep
				path := config.CommandPath(dep.Command, dep.Groups)
				prefix := opts.formatPrefix(path, dep.Command, dep.Groups)

				allWg.Add(1)
				go func(d config.SearchResult, p string) {
					defer allWg.Done()
					if err := executeSingleCommand(ctx, cfg, d, opts, p); err != nil {
						if ctx.Err() == context.Canceled {
							return
						}
						wrapped := fmt.Errorf("dependency %s failed: %w", p, err)
						firstErr.CompareAndSwap(nil, &wrapped)
						cancel()
					}
				}(dep, prefix)
			}
		} else {
			for _, dep := range level {
				path := config.CommandPath(dep.Command, dep.Groups)
				prefix := opts.formatPrefix(path, dep.Command, dep.Groups)
				if opts.Stdout != nil {
					_, _ = fmt.Fprintf(opts.Stdout, "=> Running dependency: %s\n", path)
				} else {
					fmt.Printf("=> Running dependency: %s\n", path)
				}
				if err := executeSingleCommand(ctx, cfg, dep, opts, prefix); err != nil {
					return fmt.Errorf("dependency %s failed: %w", path, err)
				}
			}
		}
	}

	if f := firstErr.Load(); f != nil {
		return *f
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	allWg.Wait()

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

	if result.Command.Native {
		logger.Debug("engine: executing native command")
		cmdPath := config.CommandPath(result.Command, result.Groups)
		if err := runNative(ctx, cmdPath, NativeContext{
			Vars:   vars,
			Args:   parsedArgs,
			Stdout: runOpts.Stdout,
			Stderr: runOpts.Stderr,
		}, opts.NativeHandlers); err != nil {
			return err
		}
	} else {
		logger.Debug("engine: executing shell command")
		if err := runner.ExecuteCommand(ctx, result.Command, interpCtx, runOpts, shell, dir); err != nil {
			return err
		}
	}

	return nil
}

func executeSingleCommand(ctx context.Context, cfg *config.AppConfig, result config.SearchResult, opts Options, prefix string) error {
	args := runner.ResolveArgs(*cfg, result.Groups, *result.Command)
	shell := runner.ResolveShell(*cfg, result.Groups, *result.Command)
	parsedArgs, err := runner.ParseArgs([]string{}, args)
	if err != nil {
		return err
	}
	vars := runner.ResolveVars(*cfg, result.Groups, *result.Command)

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

	if result.Command.Native {
		if prefix != "" {
			_, _ = fmt.Fprintf(runOpts.Stdout, "%s ", prefix)
		}
		cmdPath := config.CommandPath(result.Command, result.Groups)
		return runNative(ctx, cmdPath, NativeContext{
			Vars:   vars,
			Args:   parsedArgs,
			Stdout: runOpts.Stdout,
			Stderr: runOpts.Stderr,
		}, opts.NativeHandlers)
	}

	if prefix != "" {
		return runner.ExecuteCommandWithPrefix(ctx, result.Command, interpCtx, runOpts, shell, dir, prefix)
	}
	return runner.ExecuteCommand(ctx, result.Command, interpCtx, runOpts, shell, dir)
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
		for _, depPath := range cmd.Depends {
			depResult, err := config.FindCommandByPath(cfg, depPath)
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
