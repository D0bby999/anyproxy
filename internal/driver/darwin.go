// SPDX-License-Identifier: MIT

package driver

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/D0bby999/anyproxyxa/internal/dnsmap"
	"github.com/D0bby999/anyproxyxa/internal/flows"
	"github.com/D0bby999/anyproxyxa/internal/pbprofile"
)

const (
	MacAppBundleID       = "dev.anyprofile.anyproxyxa"
	MacExtensionBundleID = "dev.anyprofile.anyproxyxa.extension"
)

// ProviderMessageSender sends a serialized JSON message to the macOS NetworkExtension
// (via NETunnelProviderSession.sendProviderMessage) and returns the response payload.
type ProviderMessageSender func(payload []byte) ([]byte, error)

// macExtProxyConfig matches the JSON dictionary expected by AppProxyProvider.swift ("setProxyConfigs").
type macExtProxyConfig struct {
	ID                string `json:"id"`
	Type              string `json:"type"`
	Host              string `json:"host"`
	Port              int    `json:"port"`
	Username          string `json:"username,omitempty"`
	Password          string `json:"password,omitempty"`
	SendDomainToProxy bool   `json:"sendDomainToProxy"`
}

// macExtRule matches the JSON dictionary expected by AppProxyProvider.swift ("setRules").
type macExtRule struct {
	ID                string `json:"id"`
	ProcessNames      string `json:"processNames"`
	TargetHosts       string `json:"targetHosts"`
	TargetPorts       string `json:"targetPorts"`
	Protocol          string `json:"protocol"`
	Action            string `json:"action"` // "DIRECT", "BLOCK", or proxyConfigId
	Enabled           bool   `json:"enabled"`
	LocalhostViaProxy bool   `json:"localhostViaProxy"`
}

// BuildSetProxyConfigsMessage builds the atomic "setProxyConfigs" IPC payload for AppProxyProvider.
func BuildSetProxyConfigsMessage(proxies []pbprofile.ProxyConfig) ([]byte, error) {
	cfgs := make([]macExtProxyConfig, 0, len(proxies))
	for _, p := range proxies {
		cfgs = append(cfgs, macExtProxyConfig{
			ID:                p.ID,
			Type:              strings.ToUpper(string(p.Type)),
			Host:              p.Host,
			Port:              p.Port,
			Username:          p.Username,
			Password:          p.Password,
			SendDomainToProxy: p.SendDomainToProxy,
		})
	}
	return json.Marshal(map[string]any{
		"action":  "setProxyConfigs",
		"configs": cfgs,
	})
}

// BuildSetRulesMessage builds the atomic "setRules" IPC payload for AppProxyProvider.
func BuildSetRulesMessage(ruleList []pbprofile.ProxyRule) ([]byte, error) {
	items := make([]macExtRule, 0, len(ruleList))
	for _, r := range ruleList {
		action := strings.ToUpper(string(r.Action))
		if r.Action == pbprofile.ActionProxy && r.ProxyConfigID != "" {
			action = r.ProxyConfigID
		}
		items = append(items, macExtRule{
			ID:                r.ID,
			ProcessNames:      r.ProcessNames,
			TargetHosts:       r.CombinedHosts(),
			TargetPorts:       r.TargetPorts,
			Protocol:          strings.ToUpper(string(r.Protocol)),
			Action:            action,
			Enabled:           r.Enabled,
			LocalhostViaProxy: r.LocalhostViaProxy,
		})
	}
	return json.Marshal(map[string]any{
		"action": "setRules",
		"rules":  items,
	})
}

// BuildSetTrafficLoggingMessage builds the "setTrafficLogging" IPC payload.
func BuildSetTrafficLoggingMessage(enabled bool) ([]byte, error) {
	return json.Marshal(map[string]any{
		"action":  "setTrafficLogging",
		"enabled": enabled,
	})
}

// BuildSetEnabledMessage builds the "setEnabled" master switch IPC payload.
func BuildSetEnabledMessage(enabled bool) ([]byte, error) {
	return json.Marshal(map[string]any{
		"action":  "setEnabled",
		"enabled": enabled,
	})
}

// BuildGetLogsMessage builds the "getLogs" IPC payload.
func BuildGetLogsMessage() ([]byte, error) {
	return json.Marshal(map[string]any{
		"action": "getLogs",
	})
}

// BuildGetFlowsMessage builds the "getFlows" IPC payload.
func BuildGetFlowsMessage() ([]byte, error) {
	return json.Marshal(map[string]any{
		"action": "getFlows",
	})
}

// BuildClearConfigMessage builds the "clearConfig" IPC payload to wipe rules and credentials on stop.
func BuildClearConfigMessage() ([]byte, error) {
	return json.Marshal(map[string]any{
		"action": "clearConfig",
	})
}

type extLogEnvelope struct {
	Logs        []string        `json:"logs"`
	Connections []extConnRecord `json:"connections"`
}

type extConnRecord struct {
	Timestamp string `json:"timestamp"`
	PID       int    `json:"pid"`
	Process   string `json:"process"`
	DestIP    string `json:"destIp"`
	Host      string `json:"host"`
	Port      int    `json:"port"`
	Protocol  string `json:"protocol"`
	Action    string `json:"action"`
	ProxyName string `json:"proxyName"`
	RuleID    string `json:"ruleId"`
}

type extFlowsEnvelope struct {
	Flows []extFlowRecord `json:"flows"`
}

type extFlowRecord struct {
	ID         string `json:"id"`
	PID        int    `json:"pid"`
	Process    string `json:"process"`
	Dest       string `json:"dest"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	IsUDP      bool   `json:"is_udp"`
	IsIPv6     bool   `json:"is_ipv6"`
	ProxyID    string `json:"proxy_id"`
	ProxyLabel string `json:"proxy_label"`
	RuleID     string `json:"rule_id"`
	AgeMs      int64  `json:"age_ms"`
	Sent       int64  `json:"sent"`
	Received   int64  `json:"received"`
	Closed     bool   `json:"closed"`
}

// ParseExtensionLogs parses the JSON response from "getLogs" into structured connection and activity logs.
func ParseExtensionLogs(raw []byte) ([]flows.ConnectionLogEntry, []flows.ActivityLogEntry, error) {
	if len(raw) == 0 {
		return nil, nil, nil
	}
	var env extLogEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, nil, err
	}

	conns := make([]flows.ConnectionLogEntry, 0, len(env.Connections))
	for _, c := range env.Connections {
		conns = append(conns, flows.ConnectionLogEntry{
			Timestamp:   c.Timestamp,
			PID:         c.PID,
			ProcessName: c.Process,
			DestIP:      c.DestIP,
			Host:        c.Host,
			Port:        uint16(max(0, min(65535, c.Port))),
			Protocol:    c.Protocol,
			Action:      c.Action,
			ProxyName:   c.ProxyName,
			RuleID:      c.RuleID,
			Count:       1,
		})
	}

	acts := make([]flows.ActivityLogEntry, 0, len(env.Logs))
	for _, line := range env.Logs {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// Check if line encodes a legacy connection log "[CONN] proto process -> host:port [action]"
		if strings.HasPrefix(trimmed, "[CONN] ") {
			if ce, ok := parseLegacyConnLogLine(strings.TrimPrefix(trimmed, "[CONN] ")); ok {
				conns = append(conns, ce)
				continue
			}
		}
		level := "INFO"
		upper := strings.ToUpper(trimmed)
		if strings.Contains(upper, "ERROR") || strings.Contains(upper, "❌") {
			level = "ERROR"
		} else if strings.Contains(upper, "WARN") || strings.Contains(upper, "⚠️") {
			level = "WARN"
		}
		acts = append(acts, flows.ActivityLogEntry{
			Level:   level,
			Message: trimmed,
		})
	}
	return conns, acts, nil
}

func parseLegacyConnLogLine(s string) (flows.ConnectionLogEntry, bool) {
	// Format: "TCP curl -> example.com:443 [PROXY]"
	arrowIdx := strings.Index(s, " -> ")
	if arrowIdx <= 0 {
		return flows.ConnectionLogEntry{}, false
	}
	left := strings.TrimSpace(s[:arrowIdx])
	right := strings.TrimSpace(s[arrowIdx+4:])
	parts := strings.SplitN(left, " ", 2)
	if len(parts) != 2 {
		return flows.ConnectionLogEntry{}, false
	}
	proto := strings.ToUpper(strings.TrimSpace(parts[0]))
	proc := strings.TrimSpace(parts[1])

	action := "DIRECT"
	if bOpen := strings.LastIndexByte(right, '['); bOpen >= 0 && strings.HasSuffix(right, "]") {
		action = strings.TrimSpace(right[bOpen+1 : len(right)-1])
		right = strings.TrimSpace(right[:bOpen])
	}

	host := right
	var port uint16
	if colon := strings.LastIndexByte(right, ':'); colon >= 0 {
		if p, err := strconv.ParseUint(right[colon+1:], 10, 16); err == nil {
			host = right[:colon]
			port = uint16(p)
		}
	}
	return flows.ConnectionLogEntry{
		ProcessName: proc,
		DestIP:      host,
		Host:        host,
		Port:        port,
		Protocol:    proto,
		Action:      action,
		ProxyName:   action,
		Count:       1,
	}, true
}

// ParseExtensionFlows parses the JSON response from "getFlows" into flows.Flow items.
func ParseExtensionFlows(raw []byte) ([]flows.Flow, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var env extFlowsEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, err
	}
	out := make([]flows.Flow, 0, len(env.Flows))
	for _, f := range env.Flows {
		out = append(out, flows.Flow{
			ID:          f.ID,
			PID:         f.PID,
			ProcessName: f.Process,
			DestIP:      f.Dest,
			Host:        f.Host,
			Port:        uint16(max(0, min(65535, f.Port))),
			IsUDP:       f.IsUDP,
			IsIPv6:      f.IsIPv6 || strings.Contains(f.Dest, ":"),
			ProxyID:     f.ProxyID,
			ProxyLabel:  f.ProxyLabel,
			RuleID:      f.RuleID,
			AgeMs:       f.AgeMs,
			Sent:        f.Sent,
			Received:    f.Received,
			Closed:      f.Closed,
		})
	}
	return out, nil
}

// DarwinDriver manages the macOS NetworkExtension tunnel session when available
// and falls back seamlessly to the in-process Go engine in dev/test environments.
type DarwinDriver struct {
	mu           sync.RWMutex
	sender       ProviderMessageSender
	fallback     *MockDriver
	extInstalled bool
}

// NewDarwinDriver creates a macOS driver backed by sender (if non-nil) and the shared flow/log stores.
func NewDarwinDriver(sender ProviderMessageSender, dns *dnsmap.Store, tracker *flows.Tracker, logs *flows.LogManager) *DarwinDriver {
	extInstalled := sender != nil || detectMacSystemExtensionBundle()
	backend := "macos-ne"
	if sender == nil && !extInstalled {
		backend = "macos-ne-dev"
	}
	fb := NewMockDriver(backend, dns, tracker, logs)
	if !extInstalled && runtime.GOOS == "darwin" {
		fb.SetMessage("Running in dev/loopback mode (install signed AnyProxyXa.app to /Applications for System Extension kernel capture)")
	}
	return &DarwinDriver{
		sender:       sender,
		fallback:     fb,
		extInstalled: extInstalled,
	}
}

func detectMacSystemExtensionBundle() bool {
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	// .../AnyProxyXa.app/Contents/MacOS/AnyProxyXa -> .../AnyProxyXa.app/Contents/Library/SystemExtensions/dev.anyprofile.anyproxyxa.extension.systemextension
	contentsDir := filepath.Dir(filepath.Dir(exe))
	sysExtPath := filepath.Join(contentsDir, "Library", "SystemExtensions", MacExtensionBundleID+".systemextension")
	if st, err := os.Stat(sysExtPath); err == nil && st.IsDir() {
		return true
	}
	return false
}

func (d *DarwinDriver) Start(ctx context.Context) error {
	return d.fallback.Start(ctx)
}

func (d *DarwinDriver) Stop() error {
	d.mu.RLock()
	sender := d.sender
	d.mu.RUnlock()

	if sender != nil {
		if msg, err := BuildClearConfigMessage(); err == nil {
			_, _ = sender(msg)
		}
	}
	return d.fallback.Stop()
}

func (d *DarwinDriver) SetEnabled(enabled bool) error {
	d.mu.RLock()
	sender := d.sender
	d.mu.RUnlock()

	if sender != nil {
		msg, err := BuildSetEnabledMessage(enabled)
		if err != nil {
			return err
		}
		if _, err := sender(msg); err != nil {
			return fmt.Errorf("extension setEnabled: %w", err)
		}
	}
	return d.fallback.SetEnabled(enabled)
}

func (d *DarwinDriver) Status() EngineStatus {
	st := d.fallback.Status()
	d.mu.RLock()
	if d.extInstalled {
		st.ExtensionInstalled = true
	}
	d.mu.RUnlock()
	return st
}

func (d *DarwinDriver) SyncProfile(proxies []pbprofile.ProxyConfig, ruleList []pbprofile.ProxyRule, opts RuntimeOptions) error {
	if err := d.fallback.SyncProfile(proxies, ruleList, opts); err != nil {
		return err
	}

	d.mu.RLock()
	sender := d.sender
	d.mu.RUnlock()

	if sender == nil {
		return nil
	}

	pMsg, err := BuildSetProxyConfigsMessage(proxies)
	if err != nil {
		return err
	}
	if _, err := sender(pMsg); err != nil {
		return fmt.Errorf("extension setProxyConfigs: %w", err)
	}

	rMsg, err := BuildSetRulesMessage(ruleList)
	if err != nil {
		return err
	}
	if _, err := sender(rMsg); err != nil {
		return fmt.Errorf("extension setRules: %w", err)
	}

	tMsg, err := BuildSetTrafficLoggingMessage(opts.TrafficLoggingEnabled)
	if err != nil {
		return err
	}
	if _, err := sender(tMsg); err != nil {
		return fmt.Errorf("extension setTrafficLogging: %w", err)
	}

	return nil
}

func (d *DarwinDriver) GetFlows(max int) ([]flows.Flow, error) {
	d.mu.RLock()
	sender := d.sender
	d.mu.RUnlock()

	if sender != nil {
		if req, err := BuildGetFlowsMessage(); err == nil {
			if raw, err := sender(req); err == nil && len(raw) > 0 {
				if extFlows, err := ParseExtensionFlows(raw); err == nil {
					d.fallback.tracker.ReplaceFromExternal(extFlows)
				}
			}
		}
	}
	return d.fallback.GetFlows(max)
}

func (d *DarwinDriver) ClearClosedFlows() int {
	return d.fallback.ClearClosedFlows()
}

func (d *DarwinDriver) DrainLogs() ([]flows.ConnectionLogEntry, []flows.ActivityLogEntry, error) {
	d.mu.RLock()
	sender := d.sender
	d.mu.RUnlock()

	if sender != nil {
		if req, err := BuildGetLogsMessage(); err == nil {
			if raw, err := sender(req); err == nil && len(raw) > 0 {
				if conns, acts, err := ParseExtensionLogs(raw); err == nil {
					for _, c := range conns {
						d.fallback.logs.RecordConnection(c)
					}
					for _, a := range acts {
						d.fallback.logs.LogActivity(a.Level, a.Message)
					}
				}
			}
		}
	}
	return d.fallback.DrainLogs()
}

// Fallback returns the underlying MockDriver for dev simulation and diagnostics.
func (d *DarwinDriver) Fallback() *MockDriver {
	return d.fallback
}

