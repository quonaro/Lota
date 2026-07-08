package config

import (
	"path/filepath"
	"testing"
)

func TestIsURL(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"http://example.com", true},
		{"https://example.com", true},
		{"http://example.com/config.yml", true},
		{"https://example.com/config.yml", true},
		{"./local.yml", false},
		{"/absolute/path.yml", false},
		{"local.yml", false},
		{"ftp://example.com", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := IsURL(tt.input)
			if result != tt.expected {
				t.Errorf("IsURL(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetCachePath(t *testing.T) {
	tests := []struct {
		input string
	}{
		{"http://example.com/config.yml"},
		{"https://example.com/config.yml"},
		{"http://example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := GetCachePath(tt.input)
			// Check that the result is in the cache directory
			dir := filepath.Dir(result)
			if dir != cacheDir {
				t.Errorf("GetCachePath(%q) dir = %q, want %q", tt.input, dir, cacheDir)
			}
			// Check that the result has .yml extension
			if filepath.Ext(result) != ".yml" {
				t.Errorf("GetCachePath(%q) ext = %q, want .yml", tt.input, filepath.Ext(result))
			}
		})
	}
}

func TestResolveImportPath(t *testing.T) {
	tests := []struct {
		name       string
		importPath string
		basePath   string
		expected   string
	}{
		{
			name:       "URL remains unchanged",
			importPath: "http://example.com/config.yml",
			basePath:   "/home/user/project/lota.yml",
			expected:   "http://example.com/config.yml",
		},
		{
			name:       "HTTPS URL remains unchanged",
			importPath: "https://example.com/config.yml",
			basePath:   "/home/user/project/lota.yml",
			expected:   "https://example.com/config.yml",
		},
		{
			name:       "Absolute path remains unchanged",
			importPath: "/etc/lota.yml",
			basePath:   "/home/user/project/lota.yml",
			expected:   "/etc/lota.yml",
		},
		{
			name:       "Relative path resolved",
			importPath: "./other.yml",
			basePath:   "/home/user/project/lota.yml",
			expected:   "/home/user/project/other.yml",
		},
		{
			name:       "Relative path with subdirectory",
			importPath: "./configs/other.yml",
			basePath:   "/home/user/project/lota.yml",
			expected:   "/home/user/project/configs/other.yml",
		},
		{
			name:       "Relative path with parent directory",
			importPath: "../shared.yml",
			basePath:   "/home/user/project/lota.yml",
			expected:   "/home/user/shared.yml",
		},
		{
			name:       "No base path returns as-is",
			importPath: "./other.yml",
			basePath:   "",
			expected:   "./other.yml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ResolveImportPath(tt.importPath, tt.basePath)
			if result != tt.expected {
				t.Errorf("ResolveImportPath(%q, %q) = %q, want %q", tt.importPath, tt.basePath, result, tt.expected)
			}
		})
	}
}

func TestFetchURL(t *testing.T) {
	// This test requires a real HTTP server, so we'll skip it for now
	// In a real scenario, you'd set up a test server
	t.Skip("FetchURL requires a test HTTP server")
}
