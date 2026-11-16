// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !darwin && !windows

package driver

import (
	"github.com/D0bby999/anyproxy/internal/dnsmap"
	"github.com/D0bby999/anyproxy/internal/flows"
)

// NewPlatformDriver returns the unavailable stub driver on unsupported platforms.
func NewPlatformDriver(dns *dnsmap.Store, tracker *flows.Tracker, logs *flows.LogManager) Driver {
	return newUnavailableDriver("unsupported", "System traffic interception is not supported on this platform", tracker, logs)
}
