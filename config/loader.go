package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/quonaro/lota/shared"
)

// ErrConfigNotFound is returned when no config file is found in the directory tree
var ErrConfigNotFound = errors.New("no config file found")

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
	var checked []string
	for {
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

		checked = append(checked, dir)

		// Stop at git root
		gitPath := filepath.Join(dir, ".git")
		if _, err := os.Stat(gitPath); err == nil {
			break
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("%w (checked: %s)", ErrConfigNotFound, strings.Join(checked, ", "))
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

	// Handle HTTP/HTTPS URLs
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		// Use cached path
		cachePath := GetCachePath(path)
		return &FileConfig{Path: cachePath}, nil
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

// ResolveImportPath resolves an import path relative to a base file.
// If the path is absolute or a URL, it's returned as-is.
// If the path is relative, it's resolved relative to the base file's directory.
func ResolveImportPath(importPath, basePath string) string {
	// URL - return as-is
	if IsURL(importPath) {
		return importPath
	}

	// Absolute path - return as-is
	if filepath.IsAbs(importPath) {
		return importPath
	}

	// Relative path - resolve relative to base file's directory
	if basePath != "" {
		baseDir := filepath.Dir(basePath)
		return filepath.Join(baseDir, importPath)
	}

	// No base path, return as-is (relative to current dir)
	return importPath
}
