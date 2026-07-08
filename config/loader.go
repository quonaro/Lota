package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/quonaro/lota/logger"
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
	logger.Debugf("config: current directory: %s", dir)
	return dir, nil
}

func findConfigFile(dir string) (string, error) {
	logger.Debugf("config: searching for config file starting from: %s", dir)
	var checked []string
	for {
		// Try .yml first (backward compatibility)
		ymlPath := filepath.Join(dir, shared.ConfigFileName)
		logger.Debugf("config: checking for .yml: %s", ymlPath)
		if _, err := os.Stat(ymlPath); err == nil {
			logger.Debugf("config: found config file: %s", ymlPath)
			return ymlPath, nil
		}

		// Try .yaml
		yamlPath := filepath.Join(dir, shared.ConfigFileNameYAML)
		logger.Debugf("config: checking for .yaml: %s", yamlPath)
		if _, err := os.Stat(yamlPath); err == nil {
			logger.Debugf("config: found config file: %s", yamlPath)
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
	logger.Debugf("config: resolving config path for: %s", path)
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
		logger.Debugf("config: detected HTTP URL, using cache")
		// Use cached path
		cachePath := GetCachePath(path)
		return &FileConfig{Path: cachePath}, nil
	}

	if isDir(path) {
		logger.Debugf("config: path is a directory, searching for config")
		configPath, err := findConfigFile(path)
		if err != nil {
			return nil, err
		}
		return &FileConfig{Path: configPath}, nil
	}

	logger.Debugf("config: using provided path as config file: %s", path)
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
