// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !darwin

package driver

import (
	"runtime"

	"github.com/D0bby999/anyproxy/internal/dnsmap"
	"github.com/D0bby999/anyproxy/internal/flows"
)

// NewPlatformDriver returns the platform driver (using the OS System Proxy when no kernel driver is attached).
func NewPlatformDriver(dns *dnsmap.Store, tracker *flows.Tracker, logs *flows.LogManager) Driver {
	sysProxy := NewSysProxyManager(DefaultSysProxyHost, DefaultSysProxyPort)
	if runtime.GOOS == "windows" {
		return NewSysProxyDriver("windows-sysproxy", sysProxy, dns, tracker, logs)
	}
	return newUnavailableDriver("unsupported", "System traffic interception is not supported on this platform", tracker, logs)
}
