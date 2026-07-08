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

	return result
}
