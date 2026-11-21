// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build windows

package driver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"github.com/D0bby999/anyproxy/internal/dnsmap"
	"github.com/D0bby999/anyproxy/internal/flows"
	"github.com/D0bby999/anyproxy/internal/pbprofile"
	"github.com/D0bby999/anyproxy/internal/rules"
)

const (
	winProxyHTTP   = 0
	winProxySOCKS5 = 1

	winActionProxy  = 0
	winActionDirect = 1
	winActionBlock  = 2

	winProtoTCP  = 0
	winProtoUDP  = 1
	winProtoBoth = 2

	pbFlowsMax = 400
)

// pbFlow mirrors PB_FLOW from ProxyBridge.h (64-bit Windows ABI).
type pbFlow struct {
	ID            uint64
	PID           uint32
	ProxyConfigID uint32
	RuleID        uint32
	AgeMs         uint32
	Sent          uint64
	Received      uint64
	Port          uint16
	IsUDP         uint8
	IsIPv6        uint8
	Dest          [16]byte
	Name          [40]byte
	Host          [64]byte
	_             [2]byte // struct tail padding to 8-byte alignment
}

func winCoreDLLPath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	dir := filepath.Dir(exe)
	core := filepath.Join(dir, "ProxyBridgeCore.dll")
	wd := filepath.Join(dir, "WinDivert.dll")
	if _, err := os.Stat(core); err != nil {
		return ""
	}
	if _, err := os.Stat(wd); err != nil {
		return ""
	}
	return core
}

func winCoreAvailable() bool {
	p := winCoreDLLPath()
	if p == "" {
		return false
	}
	dll := syscall.NewLazyDLL(p)
	return dll.Load() == nil
}

// WinDivertDriver drives ProxyBridgeCore.dll + WinDivert on Windows.
type WinDivertDriver struct {
	mu      sync.Mutex
	dll     *syscall.LazyDLL
	dns     *dnsmap.Store
	tracker *flows.Tracker
	logs    *flows.LogManager

	state   EngineState
	enabled bool
	message string

	proxies []pbprofile.ProxyConfig
	rules   []pbprofile.ProxyRule
	opts    RuntimeOptions

	proxyIDMap map[string]uint32
	ruleIDs    []uint32

	logCB  uintptr
	connCB uintptr
}

var (
	winActiveMu sync.RWMutex
	winActive   *WinDivertDriver
)

// NewWinDivertDriver creates a Windows WinDivert driver backed by ProxyBridgeCore.dll.
func NewWinDivertDriver(dns *dnsmap.Store, tracker *flows.Tracker, logs *flows.LogManager) *WinDivertDriver {
	d := &WinDivertDriver{
		dll:        syscall.NewLazyDLL(winCoreDLLPath()),
		dns:        dns,
		tracker:    tracker,
		logs:       logs,
		state:      StateStopped,
		message:    "WinDivert driver ready",
		proxyIDMap: make(map[string]uint32),
	}
	d.logCB = syscall.NewCallback(func(msgPtr uintptr) uintptr {
		winActiveMu.RLock()
		drv := winActive
		winActiveMu.RUnlock()
		if drv != nil && drv.logs != nil && msgPtr != 0 {
			drv.logs.LogActivity("INFO", cstrToGo(msgPtr, 512))
		}
		return 0
	})
	d.connCB = syscall.NewCallback(func(procPtr uintptr, pid uintptr, ipPtr uintptr, port uintptr, proxyPtr uintptr) uintptr {
		winActiveMu.RLock()
		drv := winActive
		winActiveMu.RUnlock()
		if drv != nil && drv.logs != nil {
			destIP := cstrToGo(ipPtr, 128)
			host := destIP
			if drv.dns != nil {
				if doms := drv.dns.DomainsForIP(destIP); len(doms) > 0 {
					host = doms[0]
				}
			}
			proxyInfo := cstrToGo(proxyPtr, 128)
			proto := "TCP"
			if strings.HasSuffix(proxyInfo, " (UDP)") {
				proto = "UDP"
				proxyInfo = strings.TrimSuffix(proxyInfo, " (UDP)")
			}
			action := proxyInfo
			if strings.EqualFold(proxyInfo, "Blocked") {
				action = "BLOCK"
				proxyInfo = "BLOCK"
			} else if strings.EqualFold(proxyInfo, "Direct") {
				action = "DIRECT"
			}
			drv.logs.RecordConnection(flows.ConnectionLogEntry{
				PID:         int(pid),
				ProcessName: cstrToGo(procPtr, 128),
				DestIP:      destIP,
				Host:        host,
				Port:        uint16(port),
				Protocol:    proto,
				Action:      action,
				ProxyName:   proxyInfo,
			})
		}
		return 0
	})
	return d
}

func (d *WinDivertDriver) Start(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.dll.Load(); err != nil {
		d.state = StateError
		d.message = fmt.Sprintf("Load ProxyBridgeCore.dll: %v", err)
		return err
	}

	winActiveMu.Lock()
	winActive = d
	winActiveMu.Unlock()

	_, _, _ = d.dll.NewProc("ProxyBridge_SetLogCallback").Call(d.logCB)
	_, _, _ = d.dll.NewProc("ProxyBridge_SetConnectionCallback").Call(d.connCB)

	d.syncLocked()

	r1, _, callErr := d.dll.NewProc("ProxyBridge_Start").Call()
	if r1 == 0 {
		msg := "ProxyBridge_Start failed (run as Administrator for WinDivert)"
		if callErr != nil && callErr != syscall.Errno(0) {
			msg = fmt.Sprintf("%s: %v", msg, callErr)
		}
		d.state = StateError
		d.enabled = false
		d.message = msg
		return errors.New(msg)
	}

	d.state = StateRunning
	d.enabled = true
	d.message = "WinDivert active (all apps)"
	return nil
}

func (d *WinDivertDriver) Stop() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.dll.Load() == nil {
		_, _, _ = d.dll.NewProc("ProxyBridge_Stop").Call()
		d.clearLocked()
	}
	d.state = StateStopped
	d.enabled = false
	d.message = "WinDivert stopped"
	return nil
}

func (d *WinDivertDriver) SetEnabled(enabled bool) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	var flag uintptr
	if enabled {
		flag = 1
	}
	_, _, _ = d.dll.NewProc("ProxyBridge_SetEnabled").Call(flag)
	d.enabled = enabled
	if enabled {
		d.state = StateRunning
		d.message = "WinDivert active (all apps)"
	} else {
		d.state = StatePaused
		d.message = "WinDivert paused"
	}
	return nil
}

func (d *WinDivertDriver) Status() EngineStatus {
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
		Backend:            "windows-windivert",
		ExtensionInstalled: true,
		DNSProxyActive:     d.state == StateRunning,
		ActiveProxies:      len(d.proxies),
		ActiveRules:        activeRules,
		ActiveFlows:        activeFlows,
		Message:            d.message,
	}
}

func (d *WinDivertDriver) SyncProfile(proxies []pbprofile.ProxyConfig, ruleList []pbprofile.ProxyRule, opts RuntimeOptions) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.proxies = append([]pbprofile.ProxyConfig(nil), proxies...)
	d.rules = append([]pbprofile.ProxyRule(nil), ruleList...)
	d.opts = opts

	if d.logs != nil {
		d.logs.SetConfig(flows.LogConfig{
			TrafficLoggingEnabled: opts.TrafficLoggingEnabled,
			LogEveryConnection:    opts.LogEveryConnection,
			AutoClearLogs:         opts.AutoClearConnectionLogs,
			Filters:               opts.LogFilters,
		})
	}
	if d.state == StateRunning || d.state == StatePaused {
		d.syncLocked()
	}
	return nil
}

func (d *WinDivertDriver) GetFlows(max int) ([]flows.Flow, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.state != StateRunning && d.state != StatePaused {
		return nil, nil
	}
	if max <= 0 || max > pbFlowsMax {
		max = pbFlowsMax
	}
	buf := make([]pbFlow, max)
	r1, _, _ := d.dll.NewProc("ProxyBridge_GetFlows").Call(
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(max),
	)
	n := int(r1)
	if n > max {
		n = max
	}
	revProxy := make(map[uint32]pbprofile.ProxyConfig, len(d.proxies))
	for _, px := range d.proxies {
		if cid := d.proxyIDMap[px.ID]; cid != 0 {
			revProxy[cid] = px
		}
	}
	out := make([]flows.Flow, 0, n)
	for i := 0; i < n; i++ {
		f := &buf[i]
		var ip net.IP
		if f.IsIPv6 != 0 {
			ip = net.IP(f.Dest[:16])
		} else {
			ip = net.IP(f.Dest[:4])
		}
		var proxyID, proxyLabel string
		if px, ok := revProxy[f.ProxyConfigID]; ok {
			proxyID = px.ID
			proxyLabel = px.Label()
		}
		var ruleID string
		if f.RuleID != 0 {
			ruleID = fmt.Sprintf("%d", f.RuleID)
		}
		ageMs := int64(f.AgeMs)
		if f.AgeMs == 0xFFFFFFFF {
			ageMs = 0
		}
		out = append(out, flows.Flow{
			ID:          fmt.Sprintf("ext-win-%d", f.ID),
			PID:         int(f.PID),
			ProcessName: cstrBytes(f.Name[:]),
			DestIP:      ip.String(),
			Host:        cstrBytes(f.Host[:]),
			Port:        f.Port,
			IsUDP:       f.IsUDP != 0,
			IsIPv6:      f.IsIPv6 != 0,
			ProxyID:     proxyID,
			ProxyLabel:  proxyLabel,
			RuleID:      ruleID,
			AgeMs:       ageMs,
			Sent:        int64(f.Sent),
			Received:    int64(f.Received),
		})
	}
	if d.tracker != nil {
		d.tracker.ReplaceFromExternal(out)
		return d.tracker.GetFlows(max), nil
	}
	return out, nil
}

func (d *WinDivertDriver) ClearClosedFlows() int {
	if d.tracker == nil {
		return 0
	}
	return d.tracker.ClearClosed()
}

func (d *WinDivertDriver) clearLocked() {
	delRule := d.dll.NewProc("ProxyBridge_DeleteRule")
	for _, id := range d.ruleIDs {
		_, _, _ = delRule.Call(uintptr(id))
	}
	d.ruleIDs = nil

	delProxy := d.dll.NewProc("ProxyBridge_DeleteProxyConfig")
	for _, id := range d.proxyIDMap {
		_, _, _ = delProxy.Call(uintptr(id))
	}
	d.proxyIDMap = make(map[string]uint32)
}

func (d *WinDivertDriver) syncLocked() {
	d.clearLocked()

	var bTrue uintptr = 1
	var bFalse uintptr = 0
	if d.opts.TrafficLoggingEnabled {
		_, _, _ = d.dll.NewProc("ProxyBridge_SetTrafficLoggingEnabled").Call(bTrue)
	} else {
		_, _, _ = d.dll.NewProc("ProxyBridge_SetTrafficLoggingEnabled").Call(bFalse)
	}
	if d.opts.LogEveryConnection {
		_, _, _ = d.dll.NewProc("ProxyBridge_SetLogEveryConnection").Call(bTrue)
	} else {
		_, _, _ = d.dll.NewProc("ProxyBridge_SetLogEveryConnection").Call(bFalse)
	}

	addProxy := d.dll.NewProc("ProxyBridge_AddProxyConfig")
	for _, p := range d.proxies {
		ptype := uintptr(winProxySOCKS5)
		if strings.EqualFold(string(p.Type), "HTTP") {
			ptype = uintptr(winProxyHTTP)
		}
		var sendDom uintptr
		if p.SendDomainToProxy {
			sendDom = 1
		}
		r1, _, _ := addProxy.Call(
			ptype,
			cstrPtr(p.Host),
			uintptr(p.Port),
			cstrPtr(p.Username),
			cstrPtr(p.Password),
			sendDom,
		)
		if r1 != 0 {
			d.proxyIDMap[p.ID] = uint32(r1)
		} else if d.logs != nil {
			d.logs.LogActivity("WARN", fmt.Sprintf("WinDivert could not register proxy %s (%s:%d); traffic for it will be blocked", p.Label(), p.Host, p.Port))
		}
	}

	addRule := d.dll.NewProc("ProxyBridge_AddRule")
	for _, r := range d.rules {
		if !r.Enabled {
			continue
		}
		proto := uintptr(winProtoBoth)
		switch r.Protocol {
		case pbprofile.ProtoTCP:
			proto = uintptr(winProtoTCP)
		case pbprofile.ProtoUDP:
			proto = uintptr(winProtoUDP)
		}
		action := uintptr(winActionDirect)
		switch r.Action {
		case pbprofile.ActionProxy:
			action = uintptr(winActionProxy)
		case pbprofile.ActionBlock:
			action = uintptr(winActionBlock)
		}
		// an empty ProxyConfigID means "the default proxy"
		ruleProxyID := r.ProxyConfigID
		if r.Action == pbprofile.ActionProxy && ruleProxyID == "" {
			ruleProxyID = d.opts.DefaultProxyID
		}
		proxyID := uintptr(d.proxyIDMap[ruleProxyID])
		if r.Action == pbprofile.ActionProxy && proxyID == 0 {
			// fail closed when the target proxy is missing or failed to register
			action = uintptr(winActionBlock)
		}
		var localhost uintptr
		if r.LocalhostViaProxy {
			localhost = 1
		}
		ipHosts, domHosts := rules.SplitHostAndDomainTokens(r.CombinedHosts())
		proc := r.ProcessNames
		if proc == "" {
			proc = "*"
		}
		if r.ExcludeProcess && proc != "*" {
			proc = "!" + proc
		}
		if r1, _, _ := addRule.Call(cstrPtr(proc), cstrPtr(ipHosts), cstrPtr(r.TargetPorts), cstrPtr(domHosts), proto, action, proxyID, localhost); r1 != 0 {
			d.ruleIDs = append(d.ruleIDs, uint32(r1))
		}
	}

	if d.opts.DefaultProxyID != "" {
		defID := d.proxyIDMap[d.opts.DefaultProxyID]
		defAction := uintptr(winActionProxy)
		if defID == 0 {
			// fail closed so an unresolvable/missing default proxy never leaks direct
			defAction = uintptr(winActionBlock)
		}
		if r1, _, _ := addRule.Call(cstrPtr("~*"), cstrPtr("*"), cstrPtr("*"), 0, uintptr(winProtoBoth), defAction, uintptr(defID), 0); r1 != 0 {
			d.ruleIDs = append(d.ruleIDs, uint32(r1))
		}
	}
}

func cstrPtr(s string) uintptr {
	if s == "" {
		return 0
	}
	b, err := syscall.BytePtrFromString(s)
	if err != nil {
		return 0
	}
	return uintptr(unsafe.Pointer(b))
}

func cstrBytes(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}

func cstrToGo(ptr uintptr, maxLen int) string {
	if ptr == 0 {
		return ""
	}
	p := *(*unsafe.Pointer)(unsafe.Pointer(&ptr))
	buf := unsafe.Slice((*byte)(p), maxLen)
	return cstrBytes(buf)
}
