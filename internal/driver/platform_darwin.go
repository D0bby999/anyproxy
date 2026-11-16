// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build darwin

package driver

import (
	"os"

	"github.com/D0bby999/anyproxy/internal/dnsmap"
	"github.com/D0bby999/anyproxy/internal/flows"
)

// NewPlatformDriver returns the macOS NetworkExtension driver when the embedded
// .systemextension and entitlements are present, falling back to macos-sysproxy
// in dev/unprovisioned builds (or when ANYPROXY_DRIVER=sysproxy).
func NewPlatformDriver(dns *dnsmap.Store, tracker *flows.Tracker, logs *flows.LogManager) Driver {
	if os.Getenv("ANYPROXY_DRIVER") != "sysproxy" && neAvailable() {
		return NewNEDriver(dns, tracker, logs)
	}
	sysProxy := NewSysProxyManager(DefaultSysProxyHost, DefaultSysProxyPort)
	return NewSysProxyDriver("macos-sysproxy", sysProxy, dns, tracker, logs)
}
