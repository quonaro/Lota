package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"lota/shared"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/fatih/color"
	"golang.org/x/mod/semver"
)

const repo = "quonaro/lota"

// release represents the relevant fields from GitHub latest release API response
type release struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// PerformUpdate checks for the latest release, downloads the matching binary,
// and replaces the current executable.
func PerformUpdate() error {
	currentPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot locate current executable: %w", err)
	}
	realPath, err := filepath.EvalSymlinks(currentPath)
	if err != nil {
		return fmt.Errorf("cannot resolve executable path: %w", err)
	}

	color.Cyan("Checking for updates...\n")

	latest, err := fetchLatestRelease()
	if err != nil {
		return fmt.Errorf("failed to fetch latest release: %w", err)
	}

	latestVersion := normalizeVersion(latest.TagName)
	currentVersion := normalizeVersion(shared.Version)

	if currentVersion != "dev" && semver.IsValid(currentVersion) && semver.IsValid(latestVersion) {
		if semver.Compare(currentVersion, latestVersion) >= 0 {
			color.Green("You are already running the latest version (%s).\n", shared.Version)
			return nil
		}
	}

	assetName := fmt.Sprintf("lota-%s-%s", runtime.GOOS, runtime.GOARCH)
	var assetURL string
	for _, a := range latest.Assets {
		if a.Name == assetName {
			assetURL = a.BrowserDownloadURL
			break
		}
	}
	if assetURL == "" {
		return fmt.Errorf("no asset found for platform %s/%s in release %s", runtime.GOOS, runtime.GOARCH, latest.TagName)
	}

	color.Cyan("Downloading %s -> %s...\n", latest.TagName, assetName)

	tempFile, err := downloadToTemp(assetURL)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer func() { _ = os.Remove(tempFile) }()

	if err := os.Chmod(tempFile, 0755); err != nil {
		return fmt.Errorf("cannot make downloaded binary executable: %w", err)
	}

	tmpTarget := realPath + ".tmp"
	if err := copyFile(tempFile, tmpTarget); err != nil {
		_ = os.Remove(tmpTarget)
		return fmt.Errorf("cannot copy new binary: %w", err)
	}

	if err := os.Rename(tmpTarget, realPath); err != nil {
		_ = os.Remove(tmpTarget)
		return fmt.Errorf("cannot replace current binary: %w", err)
	}

	color.Green("Successfully updated to %s!\n", latest.TagName)
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func fetchLatestRelease() (*release, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API returned %d: %s", resp.StatusCode, string(body))
	}

	var r release
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, fmt.Errorf("failed to decode release JSON: %w", err)
	}
	return &r, nil
}

func downloadToTemp(url string) (string, error) {
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	tempFile, err := os.CreateTemp("", "lota-update-*")
	if err != nil {
		return "", err
	}
	defer func() { _ = tempFile.Close() }()

	if _, err := io.Copy(tempFile, resp.Body); err != nil {
		return "", err
	}
	return tempFile.Name(), nil
}

func normalizeVersion(v string) string {
	if v == "" || v == "dev" || v == "unknown" {
		return "dev"
	}
	if !semver.IsValid(v) {
		// Try adding "v" prefix if missing
		if semver.IsValid("v" + v) {
			return "v" + v
		}
	}
	return v
}
