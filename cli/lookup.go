package cli

import (
	"fmt"
	"lota/config"
	"lota/runner"
	"strings"

	"github.com/fatih/color"
)

// FindCommandByPath finds a command by its full dot-separated path (e.g., "infra.docker.up").
func FindCommandByPath(cfg *config.AppConfig, path string) (config.SearchResult, error) {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return config.SearchResult{}, fmt.Errorf("empty command path")
	}

	result := cfg.Find(parts[0])
	if !result.Exists {
		return config.SearchResult{}, fmt.Errorf("command or group not found: %s", parts[0])
	}

	for i := 1; i < len(parts); i++ {
		if result.Command != nil {
			return config.SearchResult{}, fmt.Errorf("cannot traverse into command: %s", strings.Join(parts[:i], "."))
		}
		if len(result.Groups) == 0 {
			return config.SearchResult{}, fmt.Errorf("invalid path: %s", path)
		}
		current := result.Groups[len(result.Groups)-1]
		sub := current.Find(parts[i])
		if !sub.Exists {
			return config.SearchResult{}, fmt.Errorf("command or group not found: %s", parts[i])
		}
		sub.Groups = append(result.Groups, sub.Groups...)
		result = sub
	}

	if result.Command == nil {
		return config.SearchResult{}, fmt.Errorf("path does not resolve to a command: %s", path)
	}

	return result, nil
}

// commandPath builds the dot-separated path for a command.
func commandPath(cmd *config.Command, groups []*config.Group) string {
	parts := make([]string, 0, len(groups)+1)
	for _, g := range groups {
		parts = append(parts, g.Name)
	}
	parts = append(parts, cmd.Name)
	return strings.Join(parts, ".")
}

// ResolveDependencies resolves and topologically sorts all dependencies for a command.
// Returns the ordered list of dependency results (excluding the target command itself).
func ResolveDependencies(cfg *config.AppConfig, result config.SearchResult) ([]config.SearchResult, error) {
	if result.Command == nil {
		return nil, nil
	}

	visited := make(map[string]bool)
	completed := make(map[string]bool)
	var order []config.SearchResult

	var visit func(cmd *config.Command, groups []*config.Group) error
	visit = func(cmd *config.Command, groups []*config.Group) error {
		path := commandPath(cmd, groups)

		if completed[path] {
			return nil
		}
		if visited[path] {
			return fmt.Errorf("circular dependency detected: %s", path)
		}

		visited[path] = true

		for _, depPath := range cmd.Depends {
			depResult, err := FindCommandByPath(cfg, depPath)
			if err != nil {
				return fmt.Errorf("dependency %q of %s: %w", depPath, path, err)
			}
			if err := visit(depResult.Command, depResult.Groups); err != nil {
				return err
			}
		}

		visited[path] = false
		completed[path] = true
		order = append(order, config.SearchResult{
			Exists:  true,
			Command: cmd,
			Groups:  groups,
		})

		return nil
	}

	if err := visit(result.Command, result.Groups); err != nil {
		return nil, err
	}

	// Remove the target command itself (last in order)
	if len(order) > 0 {
		order = order[:len(order)-1]
	}

	return order, nil
}

// LoadConfig loads and indexes the configuration.
// configPath can be empty (uses default lota.yml or lota.yaml), a file path, or a directory.
func LoadConfig(configPath string) (*config.AppConfig, error) {
	fc, err := config.GetConfigPath(configPath)
	if err != nil {
		return nil, err
	}

	cfg, err := config.ParseConfig(fc.Path)
	if err != nil {
		return nil, err
	}

	// Validates the configuration (includes ExpandAllVars and BuildIndexes)
	result := config.GetValidator(cfg, fc.Path).Validate()

	// Print warnings if any
	for _, warning := range result.Warnings {
		color.Yellow("Warning: %s\n\n", warning)
	}

	if result.Error != nil {
		color.Red("Error: %v\n\n", result.Error)
		return nil, result.Error
	}

	return cfg, nil
}

// ResolveCommand greedily walks the config tree consuming CLI tokens.
// Returns the resolved result, remaining (unconsumed) arguments, and index of last found element.
// Supports arbitrary nesting: group subgroup ... command [args...]
func ResolveCommand(cfg *config.AppConfig, cliArgs []string) (config.SearchResult, []string, int) {
	if len(cliArgs) == 0 {
		return config.SearchResult{Exists: false}, cliArgs, 0
	}

	result := cfg.Find(cliArgs[0])
	if !result.Exists {
		return config.SearchResult{Exists: false}, cliArgs, 0
	}

	consumed := 1
	searchIdx := 1
	for searchIdx < len(cliArgs) {
		// Stop if we already resolved a command (leaf)
		if result.Command != nil {
			break
		}
		// Stop if there are no groups to descend into
		if len(result.Groups) == 0 {
			break
		}
		// Skip flags (tokens starting with -) during path resolution
		if len(cliArgs[searchIdx]) > 0 && cliArgs[searchIdx][0] == '-' {
			searchIdx++
			// Skip flag value if next token exists and doesn't start with -
			if searchIdx < len(cliArgs) && !strings.HasPrefix(cliArgs[searchIdx], "-") {
				searchIdx++
			}
			continue
		}
		current := result.Groups[len(result.Groups)-1]
		sub := current.Find(cliArgs[searchIdx])
		if !sub.Exists {
			break
		}
		sub.Groups = append(result.Groups, sub.Groups...)
		result = sub
		// Move consumed to searchIdx + 1 to consume the found element
		consumed = searchIdx + 1
		searchIdx++
	}

	return result, cliArgs[consumed:], consumed - 1
}

// executeSingleCommand runs a single command result with empty CLI args (for dependencies).
func executeSingleCommand(cfg *config.AppConfig, result config.SearchResult, opts runner.RunOptions) error {
	args := runner.ResolveArgs(*cfg, result.Groups, *result.Command)

	shell := runner.ResolveShell(*cfg, result.Groups, *result.Command)

	parsedArgs, err := runner.ParseArgs([]string{}, args)
	if err != nil {
		return err
	}

	vars := runner.ResolveVars(*cfg, result.Groups, *result.Command)

	context := runner.InterpolationContext{
		Vars:    vars,
		Args:    parsedArgs,
		ArgDefs: args,
	}

	dir := runner.ResolveDir(*cfg, result.Groups, *result.Command)

	return runner.ExecuteCommand(result.Command, context, opts, shell, dir)
}

// RunCommand executes a command with CLI arguments, including dependencies.
func RunCommand(cfg *config.AppConfig, result config.SearchResult, cliArgs []string, opts runner.RunOptions) error {
	if result.Command == nil {
		return fmt.Errorf("not a command")
	}

	deps, err := ResolveDependencies(cfg, result)
	if err != nil {
		return err
	}

	for _, dep := range deps {
		if opts.Verbose {
			fmt.Printf("[verbose] running dependency: %s\n", commandPath(dep.Command, dep.Groups))
		}
		if err := executeSingleCommand(cfg, dep, opts); err != nil {
			return fmt.Errorf("dependency failed: %w", err)
		}
	}

	args := runner.ResolveArgs(*cfg, result.Groups, *result.Command)

	shell := runner.ResolveShell(*cfg, result.Groups, *result.Command)

	parsedArgs, err := runner.ParseArgs(cliArgs, args)
	if err != nil {
		return err
	}

	vars := runner.ResolveVars(*cfg, result.Groups, *result.Command)

	context := runner.InterpolationContext{
		Vars:    vars,
		Args:    parsedArgs,
		ArgDefs: args,
	}

	dir := runner.ResolveDir(*cfg, result.Groups, *result.Command)

	return runner.ExecuteCommand(result.Command, context, opts, shell, dir)
}
