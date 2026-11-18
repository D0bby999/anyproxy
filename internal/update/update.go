// SPDX-License-Identifier: AGPL-3.0-or-later

// Package update checks for new AnyProxy releases against an update manifest
// or GitHub Releases, comparing versions and providing release metadata.
package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Current is the running build's semantic version string.
var Current = "1.0.0"

// DefaultManifestURL is the default endpoint hosting latest.json.
// Overridable at runtime via the ANYPROXY_UPDATE_URL environment variable.
var DefaultManifestURL = "https://github.com/D0bby999/anyproxy/releases/latest/download/latest.json"

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
	SHA256            string `json:"sha256,omitempty"`
	Error             string `json:"error,omitempty"`
}

// Progress reports downloaded and total bytes (total is -1 when unknown).
type Progress func(done, total int64)

// GetManifestURL returns the configured update endpoint.
func GetManifestURL() string {
	if env := strings.TrimSpace(os.Getenv(EnvManifestURL)); env != "" {
		return env
	}
	return DefaultManifestURL
}

// CheckForUpdates retrieves the latest release manifest and compares versions.
func CheckForUpdates(ctx context.Context, customURL string) (ReleaseInfo, error) {
	m, err := fetchManifest(ctx, customURL)
	if err != nil {
		return ReleaseInfo{
			CurrentVersion: Current,
			Error:          err.Error(),
		}, err
	}

	currentClean := strings.TrimPrefix(strings.TrimSpace(Current), "v")
	isNewer := CompareVersions(m.Version, currentClean) > 0

	platformKey := Platform()
	var downloadURL, sha string
	if asset, ok := m.Assets[platformKey]; ok && asset.URL != "" {
		downloadURL = asset.URL
		sha = asset.SHA256
	} else if len(m.Assets) > 0 {
		for _, a := range m.Assets {
			if a.URL != "" {
				downloadURL = a.URL
				sha = a.SHA256
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
		SHA256:            sha,
	}, nil
}

// Platform returns the manifest asset key for the current OS and architecture.
func Platform() string { return runtime.GOOS + "-" + runtime.GOARCH }

func fetchManifest(ctx context.Context, customURL string) (Manifest, error) {
	targetURL := customURL
	if targetURL == "" {
		targetURL = GetManifestURL()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return Manifest{}, err
	}
	req.Header.Set("User-Agent", fmt.Sprintf("AnyProxy/%s (%s; %s)", Current, runtime.GOOS, runtime.GOARCH))

	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return Manifest{}, fmt.Errorf("failed to contact update server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Manifest{}, fmt.Errorf("update server returned HTTP %d", resp.StatusCode)
	}

	var m Manifest
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&m); err != nil {
		return Manifest{}, fmt.Errorf("invalid manifest JSON: %w", err)
	}
	m.Version = strings.TrimPrefix(strings.TrimSpace(m.Version), "v")
	if err := ValidateManifest(&m); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

// AssetFor returns the validated platform asset from m.
func (m Manifest) AssetFor(platform string) (Asset, error) {
	a, ok := m.Assets[platform]
	if !ok || strings.TrimSpace(a.URL) == "" {
		return Asset{}, fmt.Errorf("no release package available for %s", platform)
	}
	if !isAllowedAssetURL(a.URL) {
		return Asset{}, errors.New("update download URL must use https")
	}
	if len(strings.TrimSpace(a.SHA256)) != 64 {
		return Asset{}, errors.New("release manifest missing SHA-256 checksum")
	}
	return a, nil
}

func isAllowedAssetURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	if strings.EqualFold(u.Scheme, "https") {
		return true
	}
	host := u.Hostname()
	return strings.EqualFold(u.Scheme, "http") && (host == "127.0.0.1" || host == "localhost" || host == "::1")
}

// Download fetches a into dir, verifies its SHA-256 checksum, and returns the local file path.
func Download(ctx context.Context, c *http.Client, a Asset, dir string, progress Progress) (string, error) {
	if c == nil {
		c = &http.Client{Timeout: 15 * time.Minute}
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	name := filepath.Base(strings.SplitN(a.URL, "?", 2)[0])
	if name == "" || name == "." || name == "/" || strings.ContainsAny(name, `\:`) {
		name = "update.bin"
	}
	dst := filepath.Join(dir, name)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.URL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", fmt.Sprintf("AnyProxy/%s (%s; %s)", Current, runtime.GOOS, runtime.GOARCH))

	resp, err := c.Do(req)
	if err != nil {
		return "", fmt.Errorf("download update: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download server returned HTTP %d", resp.StatusCode)
	}

	f, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	_, err = io.Copy(io.MultiWriter(f, h), &progressReader{r: resp.Body, total: resp.ContentLength, progress: progress})
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil && !strings.EqualFold(hex.EncodeToString(h.Sum(nil)), strings.TrimSpace(a.SHA256)) {
		err = errors.New("downloaded update failed SHA-256 verification")
	}
	if err != nil {
		_ = os.Remove(dst)
		return "", err
	}
	return dst, nil
}

type progressReader struct {
	r        io.Reader
	done     int64
	total    int64
	progress Progress
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.done += int64(n)
	if p.progress != nil && n > 0 {
		p.progress(p.done, p.total)
	}
	return n, err
}

// Install fetches the newest release, verifies its SHA-256, and invokes the OS installer.
// The caller should quit the running application when Install returns nil.
func Install(ctx context.Context, customURL string, stagingDir string, progress Progress) (Manifest, error) {
	m, err := fetchManifest(ctx, customURL)
	if err != nil {
		return Manifest{}, err
	}
	currentClean := strings.TrimPrefix(strings.TrimSpace(Current), "v")
	if CompareVersions(m.Version, currentClean) <= 0 {
		return m, fmt.Errorf("AnyProxy %s is already the newest version", Current)
	}
	asset, err := m.AssetFor(Platform())
	if err != nil {
		return m, err
	}
	_ = os.RemoveAll(stagingDir)
	pkg, err := Download(ctx, nil, asset, stagingDir, progress)
	if err != nil {
		return m, err
	}
	if err := Apply(pkg, os.Getpid()); err != nil {
		return m, err
	}
	return m, nil
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
