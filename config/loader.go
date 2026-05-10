package config

import (
	"fmt"
	"lota/shared"
	"os"
	"path/filepath"
)

type FileConfig struct {
	Path string
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func CurrentDir() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current dir: %w", err)
	}
	return dir, nil
}

func findConfigFile(dir string) (string, error) {
	// Try .yml first (backward compatibility)
	ymlPath := filepath.Join(dir, shared.ConfigFileName)
	if _, err := os.Stat(ymlPath); err == nil {
		return ymlPath, nil
	}

	// Try .yaml
	yamlPath := filepath.Join(dir, shared.ConfigFileNameYAML)
	if _, err := os.Stat(yamlPath); err == nil {
		return yamlPath, nil
	}

	// Neither exists
	return "", fmt.Errorf("no config file found (tried %s and %s)", shared.ConfigFileName, shared.ConfigFileNameYAML)
}

func GetConfigPath(path string) (*FileConfig, error) {
	if path == "" {
		dir, err := CurrentDir()
		if err != nil {
			return nil, err
		}
		configPath, err := findConfigFile(dir)
		if err != nil {
			return nil, err
		}
		return &FileConfig{Path: configPath}, nil
	}

	if isDir(path) {
		configPath, err := findConfigFile(path)
		if err != nil {
			return nil, err
		}
		return &FileConfig{Path: configPath}, nil
	}

	return &FileConfig{Path: path}, nil
}
