package cli

import (
	"fmt"
	"lota/config"
	"lota/runner"
	"strings"
)

// LoadConfig loads and indexes the configuration
func LoadConfig() (*config.AppConfig, error) {
	configPath, err := config.GetConfigPath("lota.yml")
	if err != nil {
		return nil, err
	}

	cfg, err := config.ParseConfig(configPath.Path)
	if err != nil {
		return nil, err
	}

	if err := cfg.BuildIndexes(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// ParseCommandPath splits ["group1", "command1", "--param1", "value"]
// into path=["group1", "command1"] and args=["--param1", "value"].
// Command path is always 1 or 2 elements, everything else are args.
func ParseCommandPath(cliArgs []string) ([]string, []string) {
	if len(cliArgs) == 0 {
		return []string{}, []string{}
	}

	// Check if second element exists and is not a flag
	if len(cliArgs) >= 2 && !strings.HasPrefix(cliArgs[1], "-") {
		return cliArgs[:2], cliArgs[2:]
	}

	return cliArgs[:1], cliArgs[1:]
}

// FindCommand finds a command by path ["group", "command"] or ["command"]
func FindCommand(cfg *config.AppConfig, path []string) config.SearchResult {
	if len(path) == 1 {
		return cfg.Find(path[0])
	}

	if len(path) == 2 {
		group := cfg.Find(path[0])
		if group.Exists && group.Group != nil {
			for _, cmd := range group.Group.Commands {
				if cmd.Name == path[1] {
					return config.SearchResult{
						Exists:  true,
						Command: &cmd,
						Group:   group.Group,
					}
				}
			}
		}
	}

	return config.SearchResult{Exists: false}
}

// RunCommand executes a command with CLI arguments
func RunCommand(cfg *config.AppConfig, result config.SearchResult, cliArgs []string) error {
	if result.Command == nil {
		return fmt.Errorf("not a command")
	}

	args := runner.ResolveArgs(*cfg, result.Group, *result.Command)

	parsedArgs, err := runner.ParseArgs(cliArgs, args)
	if err != nil {
		return err
	}

	vars := runner.ResolveVars(*cfg, result.Group, *result.Command)

	context := runner.InterpolationContext{
		Vars:    vars,
		Args:    parsedArgs,
		ArgDefs: args,
	}

	return runner.ExecuteCommand(result.Command, context)
}
