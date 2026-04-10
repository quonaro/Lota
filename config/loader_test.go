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

func TestFileConfig_Read(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test_config.yml")
	expectedContent := []byte("name: test\nvars:\n  - KEY=value")

	if err := os.WriteFile(configPath, expectedContent, 0644); err != nil {
		t.Fatal(err)
	}

	config := &FileConfig{Path: configPath}
	content, err := config.Read()
	if err != nil {
		t.Fatalf("Read() failed: %v", err)
	}

	if string(content) != string(expectedContent) {
		t.Errorf("Read() = %v, want %v", string(content), string(expectedContent))
	}
}

func TestFileConfig_Read_FileNotFound(t *testing.T) {
	config := &FileConfig{Path: "/nonexistent/path/config.yml"}
	_, err := config.Read()
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
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
	dir := CurrentDir()
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
	config, err := GetConfigPath("")
	if err != nil {
		t.Fatalf("GetConfigPath(empty) failed: %v", err)
	}

	expectedPath := filepath.Join(CurrentDir(), shared.ConfigFileName)
	if config.Path != expectedPath {
		t.Errorf("GetConfigPath(empty) = %v, want %v", config.Path, expectedPath)
	}
}
