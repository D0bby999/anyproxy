// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build windows

package driver

import (
	"os"

	"github.com/D0bby999/anyproxy/internal/dnsmap"
	"github.com/D0bby999/anyproxy/internal/flows"
)

// NewPlatformDriver returns the WinDivert driver when ProxyBridgeCore.dll + WinDivert.dll
// are bundled beside the executable, falling back to windows-sysproxy otherwise.
func NewPlatformDriver(dns *dnsmap.Store, tracker *flows.Tracker, logs *flows.LogManager) Driver {
	if os.Getenv("ANYPROXY_DRIVER") != "sysproxy" && winCoreAvailable() {
		return NewWinDivertDriver(dns, tracker, logs)
	}
	sysProxy := NewSysProxyManager(DefaultSysProxyHost, DefaultSysProxyPort)
	return NewSysProxyDriver("windows-sysproxy", sysProxy, dns, tracker, logs)
}
