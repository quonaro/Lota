package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func loadEnvironmentFile(basePath, path, format string) ([]Var, error) {
	// Resolve path relative to config file
	fullPath := path
	if !filepath.IsAbs(path) {
		fullPath = filepath.Join(filepath.Dir(basePath), path)
	}

	// For now, only env format is supported
	if format != "" && format != "env" {
		return nil, fmt.Errorf("unsupported format: %s (only env is supported)", format)
	}

	file, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("env file not found: %s", fullPath)
	}
	defer file.Close()

	var vars []Var
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse KEY=VALUE
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue // Skip invalid lines
		}

		vars = append(vars, Var{
			Name:  strings.TrimSpace(parts[0]),
			Value: strings.TrimSpace(parts[1]),
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read env file: %w", err)
	}

	return vars, nil
}

// ExpandVarsFromFile expands variables that reference env files
func ExpandVarsFromFile(vars []Var, basePath string) ([]Var, error) {
	result := make([]Var, 0, len(vars))

	for _, v := range vars {
		if v.FromFile != "" {
			// Load variables from file
			fileVars, err := loadEnvironmentFile(basePath, v.FromFile, v.Format)
			if err != nil {
				return nil, err
			}
			result = append(result, fileVars...)
		} else {
			result = append(result, v)
		}
	}

	return result, nil
}

// ExpandAllVars recursively expands all variables in the config
func ExpandAllVars(cfg *AppConfig, basePath string) error {
	var err error

	// Expand app-level vars
	cfg.Vars, err = ExpandVarsFromFile(cfg.Vars, basePath)
	if err != nil {
		return err
	}

	// Expand group-level vars recursively
	for i := range cfg.Groups {
		if err := expandGroupVars(&cfg.Groups[i], basePath); err != nil {
			return err
		}
	}

	// Expand command-level vars
	for i := range cfg.Commands {
		cfg.Commands[i].Vars, err = ExpandVarsFromFile(cfg.Commands[i].Vars, basePath)
		if err != nil {
			return err
		}
	}

	return nil
}

func expandGroupVars(g *Group, basePath string) error {
	var err error

	// Expand group-level vars
	g.Vars, err = ExpandVarsFromFile(g.Vars, basePath)
	if err != nil {
		return err
	}

	// Expand sub-group vars recursively
	for i := range g.Groups {
		if err := expandGroupVars(&g.Groups[i], basePath); err != nil {
			return err
		}
	}

	// Expand command-level vars
	for i := range g.Commands {
		g.Commands[i].Vars, err = ExpandVarsFromFile(g.Commands[i].Vars, basePath)
		if err != nil {
			return err
		}
	}

	return nil
}
