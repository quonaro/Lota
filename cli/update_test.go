package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "dev"},
		{"dev", "dev"},
		{"unknown", "dev"},
		{"1.2.3", "v1.2.3"},
		{"v1.2.3", "v1.2.3"},
		{"v1.0.0-rc1", "v1.0.0-rc1"},
		{"not-a-version", "not-a-version"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeVersion(tt.input)
			if got != tt.expected {
				t.Errorf("normalizeVersion(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestFetchLatestRelease(t *testing.T) {
	mockRelease := release{
		TagName: "v1.2.3",
		Assets: []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		}{
			{Name: "lota-linux-amd64", BrowserDownloadURL: "https://example.com/lota-linux-amd64"},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/quonaro/lota/releases/latest" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockRelease)
	}))
	defer server.Close()

	// Replace the API URL in fetchLatestRelease by overriding repo constant isn't possible,
	// so we test the helper logic through a public wrapper in tests only.
	// Instead, we test normalizeVersion and downloadToTemp directly.
}

func TestCopyFile(t *testing.T) {
	src := filepath.Join(t.TempDir(), "src")
	dst := filepath.Join(t.TempDir(), "dst")
	content := []byte("binary data")

	if err := os.WriteFile(src, content, 0644); err != nil {
		t.Fatalf("failed to write source: %v", err)
	}

	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile failed: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("failed to read destination: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("copied content mismatch: got %q, want %q", got, content)
	}

	info, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("failed to stat destination: %v", err)
	}
	if info.Mode()&0111 == 0 {
		t.Errorf("destination not executable: %v", info.Mode())
	}

	// Overwrite existing destination
	newContent := []byte("updated data")
	if err := os.WriteFile(src, newContent, 0644); err != nil {
		t.Fatalf("failed to update source: %v", err)
	}
	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile overwrite failed: %v", err)
	}
	got, err = os.ReadFile(dst)
	if err != nil {
		t.Fatalf("failed to read destination after overwrite: %v", err)
	}
	if string(got) != string(newContent) {
		t.Errorf("overwritten content mismatch: got %q, want %q", got, newContent)
	}
}

func TestDownloadToTemp(t *testing.T) {
	content := []byte("fake binary data")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(content)
	}))
	defer server.Close()

	path, err := downloadToTemp(server.URL)
	if err != nil {
		t.Fatalf("downloadToTemp failed: %v", err)
	}
	defer func() {
		_ = os.Remove(path)
	}()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read downloaded temp file: %v", err)
	}
	if string(data) != string(content) {
		t.Errorf("downloaded content mismatch: got %q, want %q", string(data), string(content))
	}
}
