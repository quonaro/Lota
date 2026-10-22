package config

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
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
	want := []byte("build:\n  script: echo hi\n")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/bad" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(want)
	}))
	defer srv.Close()

	// Unique URL per run so the shared /tmp cache can't mask a failed fetch.
	url := fmt.Sprintf("%s/cfg-%d.yml", srv.URL, time.Now().UnixNano())
	cachePath := GetCachePath(url)
	_ = os.Remove(cachePath)
	defer func() { _ = os.Remove(cachePath) }()

	data, err := FetchURL(url)
	if err != nil {
		t.Fatalf("FetchURL: %v", err)
	}
	if string(data) != string(want) {
		t.Errorf("FetchURL = %q, want %q", data, want)
	}

	// Second call should be served from cache with identical content.
	cached, err := FetchURL(url)
	if err != nil {
		t.Fatalf("FetchURL cached: %v", err)
	}
	if string(cached) != string(want) {
		t.Errorf("cached FetchURL = %q, want %q", cached, want)
	}
}

func TestFetchURL_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	url := fmt.Sprintf("%s/missing-%d.yml", srv.URL, time.Now().UnixNano())
	_ = os.Remove(GetCachePath(url))

	_, err := FetchURL(url)
	if err == nil {
		t.Fatal("expected error for non-200 response")
	}
}

func TestFetchURL_Unreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := fmt.Sprintf("%s/x-%d.yml", srv.URL, time.Now().UnixNano())
	srv.Close() // shut down before fetching
	_ = os.Remove(GetCachePath(url))

	if _, err := FetchURL(url); err == nil {
		t.Fatal("expected error for unreachable server")
	}
}
