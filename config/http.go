package config

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxConfigSize = 100 * 1024 * 1024 // 100MB
	cacheDir      = "/tmp/lota_cache"
)

// IsURL checks if the given path is an HTTP/HTTPS URL.
func IsURL(path string) bool {
	return strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://")
}

// FetchURL downloads content from a URL and returns it as bytes.
// It skips TLS verification and enforces a 100MB size limit.
func FetchURL(url string) (data []byte, err error) {
	// Check cache first
	cachePath := GetCachePath(url)
	if cachedData, err := os.ReadFile(cachePath); err == nil {
		return cachedData, nil
	}

	// Create HTTP client with insecure TLS
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL %s: %w", url, err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("failed to close response body: %w", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP request failed for URL %s with status: %s", url, resp.Status)
	}

	// Limit response size to 100MB
	limitedReader := io.LimitReader(resp.Body, maxConfigSize)
	data, err = io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check if we hit the size limit
	if int64(len(data)) == maxConfigSize {
		return nil, fmt.Errorf("config file exceeds maximum size of 100MB")
	}

	// Ensure cache directory exists
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Write to cache
	if err := os.WriteFile(cachePath, data, 0644); err != nil {
		// Non-fatal: we have the data, just couldn't cache it
		return data, nil
	}

	return data, nil
}

// GetCachePath returns the cache file path for a given URL.
// The path is based on a SHA256 hash of the URL.
func GetCachePath(url string) string {
	hash := sha256.Sum256([]byte(url))
	hashStr := hex.EncodeToString(hash[:])
	return filepath.Join(cacheDir, hashStr+".yml")
}
