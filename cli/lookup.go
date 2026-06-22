package cli

import (
	"fmt"
	"io"
	"lota/config"
	"os"
	"path/filepath"
)

// LoadConfig loads and indexes the configuration.
// configPath can be empty (uses default lota.yml or lota.yaml), a file path, or a directory.
func LoadConfig(configPath string) (*config.AppConfig, error) {
	return LoadConfigWithWriter(configPath, os.Stderr)
}

func LoadConfigWithWriter(configPath string, warnTo io.Writer) (*config.AppConfig, error) {
	fc, err := config.GetConfigPath(configPath)
	if err != nil {
		return nil, err
	}

	cfg, err := config.ParseConfigWithWriter(fc.Path, warnTo)
	if err != nil {
		return nil, fmt.Errorf("%s:%w", filepath.Base(fc.Path), err)
	}

	// Validates the configuration (includes ExpandAllVars and BuildIndexes)
	result := config.GetValidator(cfg, fc.Path).Validate()

	// Print warnings if any
	for _, warning := range result.Warnings {
		if warnTo != nil {
			_, _ = fmt.Fprintf(warnTo, "Warning: %s\n", warning)
		}
	}

	if result.Error != nil {
		if warnTo != nil {
			_, _ = fmt.Fprintf(warnTo, "Error: %v\n\n", result.Error)
		}
		return nil, result.Error
	}

	return cfg, nil
}
