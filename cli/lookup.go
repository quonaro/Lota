package cli

import (
	"fmt"
	"lota/config"
	"lota/runner"
	"strings"

	"github.com/fatih/color"
)

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

// RunCommand executes a command with CLI arguments
func RunCommand(cfg *config.AppConfig, result config.SearchResult, cliArgs []string, opts runner.RunOptions) error {
	if result.Command == nil {
		return fmt.Errorf("not a command")
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

	return runner.ExecuteCommand(result.Command, context, opts, shell)
}
