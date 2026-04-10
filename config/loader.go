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

type ConfigReader interface {
	Read() ([]byte, error)
}

func (c *FileConfig) Read() ([]byte, error) {
	return os.ReadFile(c.Path)
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func CurrentDir() string {
	dir, err := os.Getwd()
	if err != nil {
		panic(fmt.Errorf("failed to get current dir: %w", err))
	}
	return dir
}

func GetConfigPath(path string) (*FileConfig, error) {
	if path == "" {
		path = filepath.Join(CurrentDir(), shared.ConfigFileName)
	} else if isDir(path) {
		path = filepath.Join(path, shared.ConfigFileName)
	}
	return &FileConfig{Path: path}, nil
}
