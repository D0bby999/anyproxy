// SPDX-License-Identifier: MIT

package driver

import (
	"context"
	"sync"

	"github.com/D0bby999/anyproxy/internal/dnsmap"
	"github.com/D0bby999/anyproxy/internal/flows"
	"github.com/D0bby999/anyproxy/internal/pbprofile"
)

// MockDriver holds the in-memory driver state shared by SysProxyDriver and unit tests.
type MockDriver struct {
	mu      sync.RWMutex
	started bool
	enabled bool
	backend string
	msg     string

	proxies map[string]pbprofile.ProxyConfig
	ruleSet []pbprofile.ProxyRule
	opts    RuntimeOptions
	dns     *dnsmap.Store
	tracker *flows.Tracker
	logs    *flows.LogManager
}

// NewMockDriver creates an in-process driver using the provided DNS cache, flow tracker, and log manager.
func NewMockDriver(backend string, dns *dnsmap.Store, tracker *flows.Tracker, logs *flows.LogManager) *MockDriver {
	if backend == "" {
		backend = "loopback-dev"
	}
	if dns == nil {
		dns = dnsmap.New(0)
	}
	if tracker == nil {
		tracker = flows.NewTracker(flows.MaxLiveFlows)
	}
	if logs == nil {
		logs = flows.NewLogManager()
	}
	return &MockDriver{
		enabled: true,
		backend: backend,
		proxies: make(map[string]pbprofile.ProxyConfig),
		dns:     dns,
		tracker: tracker,
		logs:    logs,
	}
}

func (d *MockDriver) Start(_ context.Context) error {
	d.mu.Lock()
	d.started = true
	d.enabled = true
	d.mu.Unlock()
	d.logs.LogActivity("INFO", "Engine started ("+d.backend+")")
	return nil
}

func (d *MockDriver) Stop() error {
	d.mu.Lock()
	d.started = false
	clear(d.proxies)
	d.ruleSet = nil
	d.mu.Unlock()
	d.logs.LogActivity("INFO", "Engine stopped and in-memory proxy credentials cleared")
	return nil
}

func (d *MockDriver) SetEnabled(enabled bool) error {
	d.mu.Lock()
	d.enabled = enabled
	d.mu.Unlock()
	if enabled {
		d.logs.LogActivity("INFO", "Interception resumed")
	} else {
		d.logs.LogActivity("INFO", "Interception paused (all new connections bypass rules)")
	}
	return nil
}

func (d *MockDriver) SetMessage(msg string) {
	d.mu.Lock()
	d.msg = msg
	d.mu.Unlock()
}

func (d *MockDriver) Status() EngineStatus {
	d.mu.RLock()
	defer d.mu.RUnlock()

	st := StateStopped
	if d.started {
		if d.enabled {
			st = StateRunning
		} else {
			st = StatePaused
		}
	}

	activeRules := 0
	for _, r := range d.ruleSet {
		if r.Enabled {
			activeRules++
		}
	}

	activeFlows := 0
	for _, fl := range d.tracker.GetFlows(flows.MaxLiveFlows) {
		if !fl.Closed {
			activeFlows++
		}
	}

	return EngineStatus{
		State:              st,
		Enabled:            d.enabled,
		Backend:            d.backend,
		ExtensionInstalled: false,
		DNSProxyActive:     false,
		ActiveProxies:      len(d.proxies),
		ActiveRules:        activeRules,
		ActiveFlows:        activeFlows,
		Message:            d.msg,
	}
}

func (d *MockDriver) SyncProfile(proxies []pbprofile.ProxyConfig, ruleList []pbprofile.ProxyRule, opts RuntimeOptions) error {
	pmap := make(map[string]pbprofile.ProxyConfig, len(proxies))
	for _, p := range proxies {
		pmap[p.ID] = p
	}

	d.mu.Lock()
	d.proxies = pmap
	d.ruleSet = append([]pbprofile.ProxyRule(nil), ruleList...)
	d.opts = opts
	d.mu.Unlock()

	d.logs.SetConfig(flows.LogConfig{
		TrafficLoggingEnabled: opts.TrafficLoggingEnabled,
		LogEveryConnection:    opts.LogEveryConnection,
		AutoClearLogs:         opts.AutoClearConnectionLogs,
		Filters:               opts.LogFilters,
	})
	return nil
}

func (d *MockDriver) GetFlows(max int) ([]flows.Flow, error) {
	return d.tracker.GetFlows(max), nil
}

func (d *MockDriver) ClearClosedFlows() int {
	return d.tracker.ClearClosed()
}
