package config

import (
	"lota/shared"
	"os"
	"path/filepath"
	"testing"
)

func TestGetConfig(t *testing.T) {
	tempDir := t.TempDir()

	// Create test config file
	configPath := filepath.Join(tempDir, shared.ConfigFileName)
	if err := os.WriteFile(configPath, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		path     *string
		expected string
	}{
		{
			name:     "custom file path",
			path:     func() *string { s := configPath; return &s }(),
			expected: configPath,
		},
		{
			name:     "dir path",
			path:     func() *string { s := tempDir; return &s }(),
			expected: configPath,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := GetConfigPath(*tt.path)
			if err != nil {
				t.Fatalf("GetConfigPath() failed: %v", err)
			}
			if config.Path != tt.expected {
				t.Errorf("GetConfigPath() = %v, want %v", config.Path, tt.expected)
			}
		})
	}
}

func TestIsDir(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "existing directory",
			path:     tempDir,
			expected: true,
		},
		{
			name:     "non-existent path",
			path:     "/nonexistent/path/12345",
			expected: false,
		},
		{
			name:     "file is not dir",
			path:     filepath.Join(tempDir, "testfile"),
			expected: false,
		},
	}

	// Create test file
	if err := os.WriteFile(filepath.Join(tempDir, "testfile"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isDir(tt.path)
			if result != tt.expected {
				t.Errorf("isDir(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestCurrentDir(t *testing.T) {
	dir, err := CurrentDir()
	if err != nil {
		t.Fatalf("CurrentDir() error: %v", err)
	}
	if dir == "" {
		t.Error("CurrentDir() returned empty string")
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Errorf("CurrentDir() returned invalid path: %v", err)
	}
	if !info.IsDir() {
		t.Error("CurrentDir() returned path that is not a directory")
	}
}

func TestGetConfig_EmptyPath(t *testing.T) {
	_, err := GetConfigPath("")
	if err == nil {
		t.Error("GetConfigPath(empty) should fail when no config file exists")
	}
}

func TestFindConfigFile_Priority(t *testing.T) {
	tempDir := t.TempDir()

	// Test .yml priority when both exist
	ymlPath := filepath.Join(tempDir, shared.ConfigFileName)
	yamlPath := filepath.Join(tempDir, shared.ConfigFileNameYAML)

	if err := os.WriteFile(ymlPath, []byte("yml"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(yamlPath, []byte("yaml"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := findConfigFile(tempDir)
	if err != nil {
		t.Fatalf("findConfigFile() failed: %v", err)
	}
	if result != ymlPath {
		t.Errorf("findConfigFile() = %v, want %v (yml priority)", result, ymlPath)
	}

	// Clean up and test only .yaml exists
	if err := os.Remove(ymlPath); err != nil {
		t.Fatal(err)
	}
	result, err = findConfigFile(tempDir)
	if err != nil {
		t.Fatalf("findConfigFile() with only yaml failed: %v", err)
	}
	if result != yamlPath {
		t.Errorf("findConfigFile() = %v, want %v", result, yamlPath)
	}

	// Test neither exists
	if err := os.Remove(yamlPath); err != nil {
		t.Fatal(err)
	}
	_, err = findConfigFile(tempDir)
	if err == nil {
		t.Error("findConfigFile() should fail when neither file exists")
	}
}

func TestGetConfigPath_YamlExtension(t *testing.T) {
	tempDir := t.TempDir()

	// Create only .yaml file
	yamlPath := filepath.Join(tempDir, shared.ConfigFileNameYAML)
	if err := os.WriteFile(yamlPath, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	config, err := GetConfigPath(tempDir)
	if err != nil {
		t.Fatalf("GetConfigPath() failed: %v", err)
	}
	if config.Path != yamlPath {
		t.Errorf("GetConfigPath() = %v, want %v", config.Path, yamlPath)
	}
}
