// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build darwin

package driver

import (
	"testing"

	"github.com/D0bby999/anyproxy/internal/dnsmap"
	"github.com/D0bby999/anyproxy/internal/flows"
)

func TestNewPlatformDriverFallsBackWhenExtensionMissing(t *testing.T) {
	if neAvailable() {
		t.Fatal("expected neAvailable() == false in unbundled test binary")
	}
	drv := NewPlatformDriver(dnsmap.New(0), flows.NewTracker(16), flows.NewLogManager())
	if got := drv.Status().Backend; got != "macos-sysproxy" {
		t.Fatalf("expected fallback backend macos-sysproxy, got %q", got)
	}
}
