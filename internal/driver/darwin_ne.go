// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build darwin

package driver

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Foundation -framework NetworkExtension -framework SystemExtensions -framework Security
#include <stdlib.h>
#include "ne_bridge_darwin.h"
*/
import "C"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/D0bby999/anyproxy/internal/dnsmap"
	"github.com/D0bby999/anyproxy/internal/flows"
	"github.com/D0bby999/anyproxy/internal/pbprofile"
)

const (
	darwinExtBundleID = "dev.anyproxy.app.extension"
	darwinAppName     = "AnyProxy"
)

// neAvailable reports whether the embedded .systemextension and entitlement are present.
func neAvailable() bool {
	cID := C.CString(darwinExtBundleID)
	defer C.free(unsafe.Pointer(cID))
	return C.anyproxy_ne_available(cID) == 1
}

// NEDriver controls the macOS NETransparentProxyProvider + NEDNSProxyProvider System Extension.
type NEDriver struct {
	mu          sync.Mutex
	dns         *dnsmap.Store
	tracker     *flows.Tracker
	logs        *flows.LogManager
	state       EngineState
	enabled     bool
	message     string
	proxies     []pbprofile.ProxyConfig
	rules       []pbprofile.ProxyRule
	opts        RuntimeOptions
	stopPollCh  chan struct{}
	pollRunning bool
}

// NewNEDriver creates a macOS NetworkExtension driver.
func NewNEDriver(dns *dnsmap.Store, tracker *flows.Tracker, logs *flows.LogManager) *NEDriver {
	return &NEDriver{
		dns:     dns,
		tracker: tracker,
		logs:    logs,
		state:   StateStopped,
		message: "Network Extension ready",
	}
}

func (d *NEDriver) Start(ctx context.Context) error {
	d.mu.Lock()
	d.state = StateStarting
	d.message = "Activating Network Extension..."
	d.mu.Unlock()

	cID := C.CString(darwinExtBundleID)
	cName := C.CString(darwinAppName)
	defer C.free(unsafe.Pointer(cID))
	defer C.free(unsafe.Pointer(cName))

	var errBuf [1024]C.char
	if rc := C.anyproxy_ne_start(cID, cName, &errBuf[0], C.int(len(errBuf))); rc != 0 {
		msg := C.GoString(&errBuf[0])
		if msg == "" {
			msg = "Failed to start Network Extension"
		}
		d.mu.Lock()
		d.state = StateError
		d.enabled = false
		d.message = msg
		d.mu.Unlock()
		return errors.New(msg)
	}

	d.mu.Lock()
	d.state = StateRunning
	d.enabled = true
	d.message = "Network Extension active (all apps)"
	proxies := append([]pbprofile.ProxyConfig(nil), d.proxies...)
	rules := append([]pbprofile.ProxyRule(nil), d.rules...)
	opts := d.opts
	d.mu.Unlock()

	_ = d.pushConfig(proxies, rules, opts)
	_ = d.sendAction(map[string]any{"action": "setEnabled", "enabled": true})
	d.startPoller()
	return nil
}

func (d *NEDriver) Stop() error {
	d.stopPoller()

	d.mu.Lock()
	wasActive := d.state == StateRunning || d.state == StatePaused
	d.mu.Unlock()
	if !wasActive {
		d.mu.Lock()
		d.state = StateStopped
		d.enabled = false
		d.message = "Network Extension stopped"
		d.mu.Unlock()
		return nil
	}

	_ = d.sendAction(map[string]any{"action": "clearConfig"})

	var errBuf [1024]C.char
	if rc := C.anyproxy_ne_stop(&errBuf[0], C.int(len(errBuf))); rc != 0 {
		msg := C.GoString(&errBuf[0])
		d.mu.Lock()
		d.state = StateError
		d.enabled = false
		d.message = msg
		d.mu.Unlock()
		return errors.New(msg)
	}

	d.mu.Lock()
	d.state = StateStopped
	d.enabled = false
	d.message = "Network Extension stopped"
	d.mu.Unlock()
	return nil
}

func (d *NEDriver) SetEnabled(enabled bool) error {
	if err := d.sendAction(map[string]any{"action": "setEnabled", "enabled": enabled}); err != nil {
		return err
	}
	d.mu.Lock()
	d.enabled = enabled
	if enabled {
		d.state = StateRunning
		d.message = "Network Extension active (all apps)"
	} else {
		d.state = StatePaused
		d.message = "Network Extension paused"
	}
	d.mu.Unlock()
	return nil
}

func (d *NEDriver) Status() EngineStatus {
	d.mu.Lock()
	defer d.mu.Unlock()
	activeRules := 0
	for _, r := range d.rules {
		if r.Enabled {
			activeRules++
		}
	}
	activeFlows := 0
	if d.tracker != nil {
		for _, f := range d.tracker.GetFlows(0) {
			if !f.Closed {
				activeFlows++
			}
		}
	}
	return EngineStatus{
		State:              d.state,
		Enabled:            d.enabled,
		Backend:            "macos-ne",
		ExtensionInstalled: neAvailable(),
		DNSProxyActive:     d.state == StateRunning,
		ActiveProxies:      len(d.proxies),
		ActiveRules:        activeRules,
		ActiveFlows:        activeFlows,
		Message:            d.message,
	}
}

func (d *NEDriver) SyncProfile(proxies []pbprofile.ProxyConfig, ruleList []pbprofile.ProxyRule, opts RuntimeOptions) error {
	d.mu.Lock()
	d.proxies = append([]pbprofile.ProxyConfig(nil), proxies...)
	d.rules = append([]pbprofile.ProxyRule(nil), ruleList...)
	d.opts = opts
	running := d.state == StateRunning || d.state == StatePaused
	d.mu.Unlock()

	if d.logs != nil {
		d.logs.SetConfig(flows.LogConfig{
			TrafficLoggingEnabled: opts.TrafficLoggingEnabled,
			LogEveryConnection:    opts.LogEveryConnection,
			AutoClearLogs:         opts.AutoClearConnectionLogs,
			Filters:               opts.LogFilters,
		})
	}
	if !running {
		return nil
	}
	return d.pushConfig(proxies, ruleList, opts)
}

func (d *NEDriver) GetFlows(max int) ([]flows.Flow, error) {
	if d.tracker == nil {
		return nil, nil
	}
	return d.tracker.GetFlows(max), nil
}

func (d *NEDriver) ClearClosedFlows() int {
	if d.tracker == nil {
		return 0
	}
	return d.tracker.ClearClosed()
}

func (d *NEDriver) pushConfig(proxies []pbprofile.ProxyConfig, ruleList []pbprofile.ProxyRule, opts RuntimeOptions) error {
	configs := make([]map[string]any, 0, len(proxies))
	for _, p := range proxies {
		configs = append(configs, map[string]any{
			"id":                p.ID,
			"proxyType":         strings.ToLower(string(p.Type)),
			"proxyHost":         p.Host,
			"proxyPort":         int(p.Port),
			"proxyUsername":     p.Username,
			"proxyPassword":     p.Password,
			"sendDomainToProxy": p.SendDomainToProxy,
		})
	}
	if err := d.sendAction(map[string]any{
		"action":         "setProxyConfigs",
		"configs":        configs,
		"defaultProxyId": opts.DefaultProxyID,
	}); err != nil {
		return err
	}

	rulesPayload := make([]map[string]any, 0, len(ruleList))
	for _, r := range ruleList {
		action := string(r.Action)
		if r.Action == pbprofile.ActionProxy && r.ProxyConfigID != "" {
			action = r.ProxyConfigID
		}
		rulesPayload = append(rulesPayload, map[string]any{
			"processNames":   r.ProcessNames,
			"targetHosts":    r.CombinedHosts(),
			"targetPorts":    r.TargetPorts,
			"ruleProtocol":   string(r.Protocol),
			"ruleAction":     action,
			"enabled":        r.Enabled,
			"excludeProcess": r.ExcludeProcess,
		})
	}
	if err := d.sendAction(map[string]any{
		"action": "setRules",
		"rules":  rulesPayload,
	}); err != nil {
		return err
	}

	return d.sendAction(map[string]any{
		"action":  "setTrafficLogging",
		"enabled": opts.TrafficLoggingEnabled,
	})
}

func (d *NEDriver) sendAction(payload map[string]any) error {
	_, err := d.sendActionWithResponse(payload)
	return err
}

func (d *NEDriver) sendActionWithResponse(payload map[string]any) ([]byte, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	cMsg := C.CString(string(raw))
	defer C.free(unsafe.Pointer(cMsg))

	var outBuf [65536]C.char
	var errBuf [1024]C.char
	if rc := C.anyproxy_ne_send_message(cMsg, &outBuf[0], C.int(len(outBuf)), &errBuf[0], C.int(len(errBuf))); rc != 0 {
		msg := C.GoString(&errBuf[0])
		if msg == "" {
			msg = "extension IPC error"
		}
		return nil, fmt.Errorf("ne ipc: %s", msg)
	}
	resp := C.GoString(&outBuf[0])
	if resp == "" {
		return nil, nil
	}
	return []byte(resp), nil
}

func (d *NEDriver) startPoller() {
	d.mu.Lock()
	if d.pollRunning {
		d.mu.Unlock()
		return
	}
	stopCh := make(chan struct{})
	d.stopPollCh = stopCh
	d.pollRunning = true
	d.mu.Unlock()

	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				d.drainLogs()
				d.syncFlows()
			}
		}
	}()
}

func (d *NEDriver) stopPoller() {
	d.mu.Lock()
	if d.pollRunning && d.stopPollCh != nil {
		close(d.stopPollCh)
		d.stopPollCh = nil
		d.pollRunning = false
	}
	d.mu.Unlock()
}

func (d *NEDriver) drainLogs() {
	if d.logs == nil {
		return
	}
	resp, err := d.sendActionWithResponse(map[string]any{"action": "getLogs"})
	if err != nil || len(resp) == 0 {
		return
	}
	var entries []map[string]string
	if err := json.Unmarshal(resp, &entries); err != nil {
		return
	}
	for _, e := range entries {
		switch e["type"] {
		case "activity":
			d.logs.LogActivity(e["level"], e["message"])
		default:
			port, _ := strconv.Atoi(e["port"])
			pid, _ := strconv.Atoi(e["pid"])
			d.logs.RecordConnection(flows.ConnectionLogEntry{
				PID:         pid,
				ProcessName: e["process"],
				Host:        e["destination"],
				DestIP:      e["destination"],
				Port:        uint16(port),
				Protocol:    e["protocol"],
				Action:      e["proxy"],
				ProxyName:   e["proxy"],
			})
		}
	}
}

func (d *NEDriver) syncFlows() {
	if d.tracker == nil {
		return
	}
	resp, err := d.sendActionWithResponse(map[string]any{"action": "getFlows"})
	if err != nil || len(resp) == 0 {
		return
	}
	var extFlows []flows.Flow
	if err := json.Unmarshal(resp, &extFlows); err != nil {
		return
	}
	d.tracker.ReplaceFromExternal(extFlows)
}
