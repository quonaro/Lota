package config

import (
	"fmt"
	"strings"
)

type ConfigValidator struct {
	*AppConfig
	basePath string
}

func GetValidator(config *AppConfig, basePath string) ConfigValidator {
	return ConfigValidator{config, basePath}
}

type ValidationResult struct {
	Warnings []string
	Error    error
}

type Validator interface {
	Validate() ValidationResult
}

func (c ConfigValidator) Validate() ValidationResult {
	result := ValidationResult{}

	// Process imports first
	if err := ProcessImports(c.AppConfig, c.basePath); err != nil {
		result.Error = err
		return result
	}

	// Expand all variables from env files (app, groups, commands)
	if err := ExpandAllVars(c.AppConfig, c.basePath); err != nil {
		// Check if it's a file not found error - treat as warning
		if strings.Contains(err.Error(), "env file not found") {
			result.Warnings = append(result.Warnings, err.Error())
		} else {
			result.Error = err
			return result
		}
	}

	// Build indexes
	if err := c.BuildIndexes(); err != nil {
		result.Error = err
		return result
	}

	// Check if config has any commands
	if c.countTotalCommands() == 0 {
		result.Error = fmt.Errorf("config file contains no commands. Add at least one command to your lota.yml")
		return result
	}

	// Validate dependencies: names must resolve, wait values must be valid,
	// and wait:ready requires a ready check on the target.
	if err := c.validateDependencies(); err != nil {
		result.Error = err
		return result
	}

	return result
}

// validateDependencies walks all commands (including nested groups) and
// validates each depends entry.
func (c *AppConfig) validateDependencies() error {
	for i := range c.Commands {
		if err := c.validateCommandDeps(&c.Commands[i]); err != nil {
			return err
		}
	}
	for i := range c.Groups {
		if err := c.validateGroupDeps(&c.Groups[i]); err != nil {
			return err
		}
	}
	return nil
}

func (c *AppConfig) validateGroupDeps(g *Group) error {
	for i := range g.Commands {
		if err := c.validateCommandDeps(&g.Commands[i]); err != nil {
			return err
		}
	}
	for i := range g.Groups {
		if err := c.validateGroupDeps(&g.Groups[i]); err != nil {
			return err
		}
	}
	return nil
}

func (c *AppConfig) validateCommandDeps(cmd *Command) error {
	if cmd.Ready != nil && cmd.Ready.Script == "" {
		return fmt.Errorf("command %q: ready block requires a script", cmd.Name)
	}
	for _, dep := range cmd.Depends {
		switch dep.Wait {
		case "", WaitDone, WaitReady, WaitStarted:
		default:
			return fmt.Errorf("command %q: invalid wait %q for dependency %q (valid: done, ready, started)", cmd.Name, dep.Wait, dep.Name)
		}
		target, err := FindCommandByPath(c, dep.Name)
		if err != nil {
			return fmt.Errorf("command %q: %w", cmd.Name, err)
		}
		if dep.Wait == WaitReady && target.Command.Ready == nil {
			return fmt.Errorf("command %q: dependency %q has wait %q but defines no ready block", cmd.Name, dep.Name, WaitReady)
		}
	}
	return nil
}
