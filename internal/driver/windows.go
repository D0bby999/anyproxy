// SPDX-License-Identifier: MIT

package driver

import (
	"runtime"

	"github.com/D0bby999/anyproxyxa/internal/dnsmap"
	"github.com/D0bby999/anyproxyxa/internal/flows"
)

// NewPlatformDriver returns the default platform Driver (macOS NetworkExtension driver
// on darwin, Windows WinDivert driver on windows, or loopback driver in dev/other OS).
func NewPlatformDriver(dns *dnsmap.Store, tracker *flows.Tracker, logs *flows.LogManager) Driver {
	switch runtime.GOOS {
	case "darwin":
		return NewDarwinDriver(nil, dns, tracker, logs)
	case "windows":
		return NewMockDriver("windows-windivert", dns, tracker, logs)
	default:
		return NewMockDriver("loopback-dev", dns, tracker, logs)
	}
}

