// SPDX-License-Identifier: AGPL-3.0-or-later

package update

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
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
