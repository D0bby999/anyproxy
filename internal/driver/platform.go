// SPDX-License-Identifier: MIT

package driver

import (
	"runtime"

	"github.com/D0bby999/anyproxy/internal/dnsmap"
	"github.com/D0bby999/anyproxy/internal/flows"
)

// NewPlatformDriver returns the platform driver (using the OS System Proxy when no kernel driver is attached).
func NewPlatformDriver(dns *dnsmap.Store, tracker *flows.Tracker, logs *flows.LogManager) Driver {
	sysProxy := NewSysProxyManager(DefaultSysProxyHost, DefaultSysProxyPort)
	switch runtime.GOOS {
	case "darwin":
		return NewSysProxyDriver("macos-sysproxy", sysProxy, dns, tracker, logs)
	case "windows":
		return NewSysProxyDriver("windows-sysproxy", sysProxy, dns, tracker, logs)
	default:
		return newUnavailableDriver("unsupported", "System traffic interception is not supported on this platform", tracker, logs)
	}
}
