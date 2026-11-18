// SPDX-License-Identifier: AGPL-3.0-or-later

package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		v1, v2 string
		want   int
	}{
		{"1.0.0", "1.0.0", 0},
		{"v1.0.0", "1.0.0", 0},
		{"1.0.1", "1.0.0", 1},
		{"1.1.0", "1.0.9", 1},
		{"2.0.0", "1.99.99", 1},
		{"1.0.0", "1.0.1", -1},
		{"1.0.0", "2.0.0", -1},
		{"1.2", "1.2.0", 0},
		{"1.2.1", "1.2", 1},
	}

	for _, tt := range tests {
		got := CompareVersions(tt.v1, tt.v2)
		if got != tt.want {
			t.Errorf("CompareVersions(%q, %q) = %d, want %d", tt.v1, tt.v2, got, tt.want)
		}
	}
}

func TestCheckForUpdatesWithMockServer(t *testing.T) {
	manifest := Manifest{
		Version: "1.2.0",
		Date:    "2026-11-08",
		Notes:   "Performance optimizations and new local proxy listener.",
		Assets: map[string]Asset{
			runtime.GOOS + "-" + runtime.GOARCH: {
				URL:    "https://example.com/download/anyproxyxa-latest",
				SHA256: "abcdef123456",
			},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(manifest)
	}))
	defer ts.Close()

	Current = "1.0.0"
	info, err := CheckForUpdates(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("CheckForUpdates error: %v", err)
	}

	if !info.IsUpdateAvailable {
		t.Errorf("IsUpdateAvailable = false, want true")
	}
	if info.LatestVersion != "1.2.0" {
		t.Errorf("LatestVersion = %q, want 1.2.0", info.LatestVersion)
	}
	if info.DownloadURL != "https://example.com/download/anyproxyxa-latest" {
		t.Errorf("DownloadURL = %q, want https://example.com/download/anyproxyxa-latest", info.DownloadURL)
	}
}

func TestCheckForUpdatesAlreadyUpToDate(t *testing.T) {
	manifest := Manifest{
		Version: "1.0.0",
		Date:    "2026-11-08",
		Notes:   "Current release.",
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(manifest)
	}))
	defer ts.Close()

	Current = "1.0.0"
	info, err := CheckForUpdates(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("CheckForUpdates error: %v", err)
	}

	if info.IsUpdateAvailable {
		t.Errorf("IsUpdateAvailable = true, want false")
	}
	if info.LatestVersion != "1.0.0" {
		t.Errorf("LatestVersion = %q, want 1.0.0", info.LatestVersion)
	}
}

func TestDownloadVerifiesSHA256(t *testing.T) {
	payload := []byte("anyproxy-binary-payload")
	sum := sha256.Sum256(payload)
	validHex := hex.EncodeToString(sum[:])

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer ts.Close()

	dir := t.TempDir()
	var lastDone, lastTotal int64
	path, err := Download(context.Background(), ts.Client(), Asset{
		URL:    ts.URL + "/AnyProxy.dmg",
		SHA256: validHex,
	}, dir, func(done, total int64) {
		lastDone, lastTotal = done, total
	})
	if err != nil {
		t.Fatalf("Download valid: %v", err)
	}
	if lastDone != int64(len(payload)) || lastTotal != int64(len(payload)) {
		t.Fatalf("progress = (%d, %d), want (%d, %d)", lastDone, lastTotal, len(payload), len(payload))
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("downloaded file missing: %v", err)
	}

	// Bad checksum must fail and delete the file
	badPath, err := Download(context.Background(), ts.Client(), Asset{
		URL:    ts.URL + "/AnyProxy-bad.dmg",
		SHA256: strings.Repeat("0", 64),
	}, dir, nil)
	if err == nil {
		t.Fatalf("expected checksum mismatch error, got path=%q", badPath)
	}
	if _, statErr := os.Stat(dir + "/AnyProxy-bad.dmg"); !os.IsNotExist(statErr) {
		t.Fatalf("expected bad download to be removed")
	}
}

func TestAssetForValidation(t *testing.T) {
	m := Manifest{
		Version: "1.0.7",
		Assets: map[string]Asset{
			"darwin-arm64": {URL: "http://evil.example.com/app.dmg", SHA256: strings.Repeat("a", 64)},
			"darwin-amd64": {URL: "https://github.com/D0bby999/anyproxy/releases/download/v1.0.7/app.dmg", SHA256: "short"},
		},
	}
	if _, err := m.AssetFor("darwin-arm64"); err == nil {
		t.Fatalf("expected non-https URL to be rejected")
	}
	if _, err := m.AssetFor("darwin-amd64"); err == nil {
		t.Fatalf("expected invalid SHA-256 to be rejected")
	}
}
