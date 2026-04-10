package cli

import (
	"fmt"
	"lota/config"
	"lota/runner"
)

// LoadConfig loads and indexes the configuration.
// configPath can be empty (uses default lota.yml), a file path, or a directory.
func LoadConfig(configPath string) (*config.AppConfig, error) {
	fc, err := config.GetConfigPath(configPath)
	if err != nil {
		return nil, err
	}

	cfg, err := config.ParseConfig(fc.Path)
	if err != nil {
		return nil, err
	}

	if err := cfg.BuildIndexes(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// ResolveCommand greedily walks the config tree consuming CLI tokens.
// Returns the resolved result and remaining (unconsumed) arguments.
// Supports arbitrary nesting: group subgroup ... command [args...]
func ResolveCommand(cfg *config.AppConfig, cliArgs []string) (config.SearchResult, []string) {
	if len(cliArgs) == 0 {
		return config.SearchResult{Exists: false}, cliArgs
	}

	result := cfg.Find(cliArgs[0])
	if !result.Exists {
		return config.SearchResult{Exists: false}, cliArgs
	}

	consumed := 1
	for consumed < len(cliArgs) {
		// Stop if we already resolved a command (leaf)
		if result.Command != nil {
			break
		}
		// Stop if there are no groups to descend into
		if len(result.Groups) == 0 {
			break
		}
		current := result.Groups[len(result.Groups)-1]
		sub := current.Find(cliArgs[consumed])
		if !sub.Exists {
			break
		}
		sub.Groups = append(result.Groups, sub.Groups...)
		result = sub
		consumed++
	}

	return result, cliArgs[consumed:]
}

// RunCommand executes a command with CLI arguments
func RunCommand(cfg *config.AppConfig, result config.SearchResult, cliArgs []string, opts runner.RunOptions) error {
	if result.Command == nil {
		return fmt.Errorf("not a command")
	}

	args := runner.ResolveArgs(*cfg, result.Groups, *result.Command)

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

	return runner.ExecuteCommand(result.Command, context, opts)
}
