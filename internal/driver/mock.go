// SPDX-License-Identifier: MIT

package driver

import (
	"context"
	"strings"
	"sync"

	"github.com/D0bby999/anyproxyxa/internal/dnsmap"
	"github.com/D0bby999/anyproxyxa/internal/flows"
	"github.com/D0bby999/anyproxyxa/internal/pbprofile"
	"github.com/D0bby999/anyproxyxa/internal/rules"
)

// MockDriver is an in-process Go interception engine used in unit tests and
// unentitled development builds (`wails dev`).
type MockDriver struct {
	mu      sync.RWMutex
	started bool
	enabled bool
	backend string
	msg     string

	proxies map[string]pbprofile.ProxyConfig
	ruleSet []pbprofile.ProxyRule
	opts    RuntimeOptions
	engine  *rules.Engine
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
		engine:  rules.NewEngine(nil),
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
	d.engine = rules.NewEngine(nil)
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
		ExtensionInstalled: d.backend != "loopback-dev",
		DNSProxyActive:     d.started && d.opts.DnsViaProxy,
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
	eng := rules.NewEngine(ruleList)

	d.mu.Lock()
	d.proxies = pmap
	d.ruleSet = append([]pbprofile.ProxyRule(nil), ruleList...)
	d.opts = opts
	d.engine = eng
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

func (d *MockDriver) DrainLogs() ([]flows.ConnectionLogEntry, []flows.ActivityLogEntry, error) {
	return d.logs.ConnectionLogs(flows.MaxConnectionLogs), d.logs.ActivityLogs(flows.MaxActivityLogs), nil
}

// EvaluateAndRecord simulates or handles an intercepted flow through the rule engine,
// DNS cache, flow tracker, and connection logger.
func (d *MockDriver) EvaluateAndRecord(pid int, processName, bundleID, destIP, explicitHost string, destPort uint16, isUDP bool) (rules.Decision, string) {
	d.mu.RLock()
	enabled := d.enabled
	eng := d.engine
	proxies := d.proxies
	d.mu.RUnlock()

	var domains []string
	if explicitHost != "" {
		domains = append(domains, strings.ToLower(explicitHost))
	}
	if destIP != "" {
		for _, cached := range d.dns.DomainsForIP(destIP) {
			if cached != strings.ToLower(explicitHost) {
				domains = append(domains, cached)
			}
		}
	}

	hostLabel := explicitHost
	if hostLabel == "" && len(domains) > 0 {
		hostLabel = domains[0]
	}

	proto := pbprofile.ProtoTCP
	if isUDP {
		proto = pbprofile.ProtoUDP
	}

	var dec rules.Decision
	if !enabled {
		dec = rules.Decision{Matched: false, Action: pbprofile.ActionDirect, RuleID: "-"}
	} else {
		dec = eng.Evaluate(rules.FlowInput{
			Process:   rules.NewFlowProcess(bundleID, processName),
			DestIP:    destIP,
			Domains:   domains,
			DestPort:  destPort,
			Protocol:  proto,
			CheckHost: true,
		})
	}

	proxyLabel := string(dec.Action)
	proxyID := string(dec.Action)
	if dec.Action == pbprofile.ActionProxy {
		if p, ok := proxies[dec.ProxyConfigID]; ok {
			if isUDP && p.Type == pbprofile.ProxyHTTP {
				d.logs.WarnOnce("http-udp:"+p.ID, "HTTP proxy "+p.Label()+" does not support UDP; routing flow DIRECT")
				dec.Action = pbprofile.ActionDirect
				proxyLabel = "DIRECT"
				proxyID = "DIRECT"
			} else {
				proxyID = p.ID
				proxyLabel = p.Label()
			}
		} else {
			d.logs.WarnOnce("missing-proxy:"+dec.ProxyConfigID, "Rule "+dec.RuleID+" references missing proxy "+dec.ProxyConfigID)
			proxyID = dec.ProxyConfigID
			proxyLabel = "MISSING(" + dec.ProxyConfigID + ")"
		}
	}

	ruleID := dec.RuleID
	if ruleID == "" {
		ruleID = "default"
	}

	flowID := d.tracker.StartFlow(flows.Flow{
		PID:         pid,
		ProcessName: processName,
		DestIP:      destIP,
		Host:        hostLabel,
		Port:        destPort,
		IsUDP:       isUDP,
		ProxyID:     proxyID,
		ProxyLabel:  proxyLabel,
		RuleID:      ruleID,
	})

	d.logs.RecordConnection(flows.ConnectionLogEntry{
		PID:         pid,
		ProcessName: processName,
		DestIP:      destIP,
		Host:        hostLabel,
		Port:        destPort,
		Protocol:    string(proto),
		Action:      string(dec.Action),
		ProxyName:   proxyLabel,
		RuleID:      ruleID,
	})

	return dec, flowID
}
