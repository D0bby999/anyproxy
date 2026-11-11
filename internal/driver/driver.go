// SPDX-License-Identifier: MIT

// Package driver abstracts the OS-level transparent packet/flow interception
// backend (macOS NetworkExtension System Extension, Windows WinDivert service,
// or local loopback/mock engine for dev and automated tests).
package driver

import (
	"context"

	"github.com/D0bby999/anyproxy/internal/flows"
	"github.com/D0bby999/anyproxy/internal/pbprofile"
)

// EngineState represents the operational state of the interception backend.
type EngineState string

const (
	StateStopped  EngineState = "stopped"
	StateStarting EngineState = "starting"
	StateRunning  EngineState = "running"
	StatePaused   EngineState = "paused"
	StateDegraded EngineState = "degraded"
	StateError    EngineState = "error"
)

// EngineStatus describes the current state of the OS interception driver.
type EngineStatus struct {
	State              EngineState `json:"state"`
	Enabled            bool        `json:"enabled"`
	Backend            string      `json:"backend"` // "macos-ne", "windows-windivert", "loopback-dev"
	ExtensionInstalled bool        `json:"extensionInstalled"`
	DNSProxyActive     bool        `json:"dnsProxyActive"`
	ActiveProxies      int         `json:"activeProxies"`
	ActiveRules        int         `json:"activeRules"`
	ActiveFlows        int         `json:"activeFlows"`
	Message            string      `json:"message,omitempty"`
}

// RuntimeOptions carries global runtime flags synchronized to the driver alongside proxies and rules.
type RuntimeOptions struct {
	TrafficLoggingEnabled   bool                  `json:"trafficLoggingEnabled"`
	DnsViaProxy             bool                  `json:"dnsViaProxy"`
	LogEveryConnection      bool                  `json:"logEveryConnection"`
	AutoClearConnectionLogs bool                  `json:"autoClearConnectionLogs"`
	DefaultProxyID          string                `json:"defaultProxyId,omitempty"`
	LogFilters              []pbprofile.LogFilter `json:"logFilters,omitempty"`
}

// Driver is the unified interface implemented by macOS NetworkExtension, Windows WinDivert, and MockDriver.
type Driver interface {
	// Start initializes and activates the OS interception tunnel/driver.
	Start(ctx context.Context) error
	// Stop tears down interception and wipes in-memory proxy credentials.
	Stop() error
	// SetEnabled toggles master interception without tearing down the OS driver.
	SetEnabled(enabled bool) error
	// Status returns a snapshot of the driver's operational status.
	Status() EngineStatus
	// SyncProfile atomically updates the proxy list, rule list, and runtime options.
	SyncProfile(proxies []pbprofile.ProxyConfig, rules []pbprofile.ProxyRule, opts RuntimeOptions) error
	// GetFlows returns up to max active + recently closed flows.
	GetFlows(max int) ([]flows.Flow, error)
	// ClearClosedFlows removes closed flows from the driver's live flow table.
	ClearClosedFlows() int
}
