// SPDX-License-Identifier: MIT

// Package update checks for new AnyProxyXa releases against an update manifest
// or GitHub Releases, comparing versions and providing release metadata.
package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Current is the running build's semantic version string.
var Current = "1.0.0"

// DefaultManifestURL is the default endpoint hosting latest.json.
// Overridable at runtime via the ANYPROXY_UPDATE_URL environment variable.
var DefaultManifestURL = "https://raw.githubusercontent.com/D0bby999/anyproxyxa/main/latest.json"

const EnvManifestURL = "ANYPROXY_UPDATE_URL"

// Asset describes a single platform's downloadable artifact.
type Asset struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256,omitempty"`
}

// Manifest represents the latest.json release descriptor.
type Manifest struct {
	Version string           `json:"version"`
	Date    string           `json:"date,omitempty"`
	Notes   string           `json:"notes,omitempty"`
	Assets  map[string]Asset `json:"assets,omitempty"`
}

// ReleaseInfo is the user-facing status returned to the GUI and CLI.
type ReleaseInfo struct {
	CurrentVersion    string `json:"currentVersion"`
	LatestVersion     string `json:"latestVersion"`
	IsUpdateAvailable bool   `json:"isUpdateAvailable"`
	ReleaseDate       string `json:"releaseDate,omitempty"`
	ReleaseNotes      string `json:"releaseNotes,omitempty"`
	DownloadURL       string `json:"downloadUrl,omitempty"`
	Error             string `json:"error,omitempty"`
}

// GetManifestURL returns the configured update endpoint.
func GetManifestURL() string {
	if env := strings.TrimSpace(os.Getenv(EnvManifestURL)); env != "" {
		return env
	}
	return DefaultManifestURL
}

// CheckForUpdates retrieves the latest release manifest and compares versions.
func CheckForUpdates(ctx context.Context, customURL string) (ReleaseInfo, error) {
	targetURL := customURL
	if targetURL == "" {
		targetURL = GetManifestURL()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return ReleaseInfo{
			CurrentVersion: Current,
			Error:          err.Error(),
		}, err
	}
	req.Header.Set("User-Agent", fmt.Sprintf("AnyProxyXa/%s (%s; %s)", Current, runtime.GOOS, runtime.GOARCH))

	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ReleaseInfo{
			CurrentVersion: Current,
			Error:          fmt.Sprintf("Failed to contact update server: %v", err),
		}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("update server returned HTTP %d", resp.StatusCode)
		return ReleaseInfo{
			CurrentVersion: Current,
			Error:          err.Error(),
		}, err
	}

	var m Manifest
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return ReleaseInfo{
			CurrentVersion: Current,
			Error:          fmt.Sprintf("Invalid manifest JSON: %v", err),
		}, err
	}

	m.Version = strings.TrimPrefix(strings.TrimSpace(m.Version), "v")
	currentClean := strings.TrimPrefix(strings.TrimSpace(Current), "v")

	isNewer := CompareVersions(m.Version, currentClean) > 0

	platformKey := fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)
	var downloadURL string
	if asset, ok := m.Assets[platformKey]; ok && asset.URL != "" {
		downloadURL = asset.URL
	} else if len(m.Assets) > 0 {
		for _, a := range m.Assets {
			if a.URL != "" {
				downloadURL = a.URL
				break
			}
		}
	}

	return ReleaseInfo{
		CurrentVersion:    Current,
		LatestVersion:     m.Version,
		IsUpdateAvailable: isNewer,
		ReleaseDate:       m.Date,
		ReleaseNotes:      m.Notes,
		DownloadURL:       downloadURL,
	}, nil
}

// CompareVersions compares two semver-like strings (e.g. "1.2.3" vs "1.2.0").
// Returns 1 if v1 > v2, -1 if v1 < v2, and 0 if equal.
func CompareVersions(v1, v2 string) int {
	v1 = strings.TrimPrefix(strings.TrimSpace(v1), "v")
	v2 = strings.TrimPrefix(strings.TrimSpace(v2), "v")

	if v1 == v2 {
		return 0
	}

	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var n1, n2 int
		if i < len(parts1) {
			n1, _ = strconv.Atoi(parts1[i])
		}
		if i < len(parts2) {
			n2, _ = strconv.Atoi(parts2[i])
		}
		if n1 > n2 {
			return 1
		}
		if n1 < n2 {
			return -1
		}
	}

	return 0
}

// ValidateManifest verifies that a manifest contains required version information.
func ValidateManifest(m *Manifest) error {
	if m == nil {
		return errors.New("manifest is nil")
	}
	if strings.TrimSpace(m.Version) == "" {
		return errors.New("manifest missing version")
	}
	return nil
}

