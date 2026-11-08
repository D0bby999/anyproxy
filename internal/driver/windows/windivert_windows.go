// SPDX-License-Identifier: MIT

//go:build windows

package windows

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/D0bby999/anyproxyxa/internal/dnsmap"
	"github.com/D0bby999/anyproxyxa/internal/flows"
	"github.com/D0bby999/anyproxyxa/internal/netproxy"
	"github.com/D0bby999/anyproxyxa/internal/pbprofile"
	"github.com/D0bby999/anyproxyxa/internal/rules"
)

const (
	winDivertLayerNetwork  = 0
	winDivertShutdownBoth  = 3
	winDivertFilterDefault = "outbound or (inbound and udp.SrcPort == 53) or (tcp.SrcPort == 34010) or (udp.SrcPort == 34011)"
)

// winDivertAddress matches WINDIVERT_ADDRESS (80 bytes in WinDivert 2.2).
type winDivertAddress struct {
	Timestamp int64
	Layer     uint8
	Event     uint8
	Flags     uint8
	_         uint8
	Length    uint32
	Union     [64]byte
}

func (a *winDivertAddress) IsOutbound() bool {
	return (a.Flags & 0x01) != 0
}

// WinDivertEngine runs the Windows transparent packet engine and local TCP/UDP relays.
type WinDivertEngine struct {
	mu      sync.RWMutex
	running atomic.Bool
	enabled atomic.Bool

	dll          *windows.LazyDLL
	procOpen     *windows.LazyProc
	procRecv     *windows.LazyProc
	procSend     *windows.LazyProc
	procShutdown *windows.LazyProc
	procClose    *windows.LazyProc
	handle       windows.Handle

	tcpLn   net.Listener
	udpConn *net.UDPConn

	conns    *ConnTable
	resolver *ProcessResolver
	dns      *dnsmap.Store
	tracker  *flows.Tracker
	logs     *flows.LogManager

	proxies map[string]pbprofile.ProxyConfig
	engine  *rules.Engine
	selfPID int
}

// NewWinDivertEngine creates a Windows packet interception engine.
func NewWinDivertEngine(dns *dnsmap.Store, tracker *flows.Tracker, logs *flows.LogManager) *WinDivertEngine {
	if dns == nil {
		dns = dnsmap.New(0)
	}
	if tracker == nil {
		tracker = flows.NewTracker(flows.MaxLiveFlows)
	}
	if logs == nil {
		logs = flows.NewLogManager()
	}

	exePath, _ := os.Executable()
	dllPath := "WinDivert.dll"
	if exePath != "" {
		candidate := filepath.Join(filepath.Dir(exePath), "WinDivert.dll")
		if _, err := os.Stat(candidate); err == nil {
			dllPath = candidate
		}
	}
	dll := windows.NewLazyDLL(dllPath)

	e := &WinDivertEngine{
		dll:          dll,
		procOpen:     dll.NewProc("WinDivertOpen"),
		procRecv:     dll.NewProc("WinDivertRecv"),
		procSend:     dll.NewProc("WinDivertSend"),
		procShutdown: dll.NewProc("WinDivertShutdown"),
		procClose:    dll.NewProc("WinDivertClose"),
		handle:       windows.InvalidHandle,
		conns:        NewConnTable(),
		resolver:     NewProcessResolver(),
		dns:          dns,
		tracker:      tracker,
		logs:         logs,
		proxies:      make(map[string]pbprofile.ProxyConfig),
		engine:       rules.NewEngine(nil),
		selfPID:      os.Getpid(),
	}
	e.enabled.Store(true)
	return e
}

// Start opens the local TCP/UDP relay sockets and starts the WinDivert packet worker loop.
func (e *WinDivertEngine) Start(_ context.Context) error {
	if e.running.Load() {
		return nil
	}
	if err := e.dll.Load(); err != nil {
		return fmt.Errorf("load WinDivert.dll: %w", err)
	}

	tcpLn, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", DefaultTCPRelayPort))
	if err != nil {
		return fmt.Errorf("bind local TCP relay port %d: %w", DefaultTCPRelayPort, err)
	}
	udpAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("0.0.0.0:%d", DefaultUDPRelayPort))
	if err != nil {
		_ = tcpLn.Close()
		return err
	}
	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		_ = tcpLn.Close()
		return fmt.Errorf("bind local UDP relay port %d: %w", DefaultUDPRelayPort, err)
	}

	filterBytes := append([]byte(winDivertFilterDefault), 0)
	h, _, callErr := e.procOpen.Call(
		uintptr(unsafe.Pointer(&filterBytes[0])),
		winDivertLayerNetwork,
		0,
		0,
	)
	if windows.Handle(h) == windows.InvalidHandle || h == 0 {
		_ = tcpLn.Close()
		_ = udpConn.Close()
		return fmt.Errorf("WinDivertOpen failed (ensure Administrator privileges and WinDivert64.sys present): %v", callErr)
	}

	e.mu.Lock()
	e.handle = windows.Handle(h)
	e.tcpLn = tcpLn
	e.udpConn = udpConn
	e.mu.Unlock()

	e.running.Store(true)
	go e.serveTCPRelay(tcpLn)
	go e.serveUDPRelay(udpConn)
	go e.packetWorker(windows.Handle(h))

	e.logs.LogActivity("INFO", "WinDivert packet engine started on TCP:34010 / UDP:34011")
	return nil
}

// Stop shuts down the WinDivert handle, closes local relays, and clears proxy credentials.
func (e *WinDivertEngine) Stop() error {
	if !e.running.Swap(false) {
		return nil
	}

	e.mu.Lock()
	h := e.handle
	e.handle = windows.InvalidHandle
	tcpLn := e.tcpLn
	udpConn := e.udpConn
	clear(e.proxies)
	e.engine = rules.NewEngine(nil)
	e.mu.Unlock()

	if h != windows.InvalidHandle && h != 0 {
		_, _, _ = e.procShutdown.Call(uintptr(h), winDivertShutdownBoth)
		_, _, _ = e.procClose.Call(uintptr(h))
	}
	if tcpLn != nil {
		_ = tcpLn.Close()
	}
	if udpConn != nil {
		_ = udpConn.Close()
	}

	e.logs.LogActivity("INFO", "WinDivert packet engine stopped")
	return nil
}

func (e *WinDivertEngine) SetEnabled(enabled bool) {
	e.enabled.Store(enabled)
}

func (e *WinDivertEngine) IsRunning() bool {
	return e.running.Load()
}

func (e *WinDivertEngine) SyncConfig(proxies []pbprofile.ProxyConfig, ruleList []pbprofile.ProxyRule) {
	pmap := make(map[string]pbprofile.ProxyConfig, len(proxies))
	var endpoints []netip.AddrPort
	for _, p := range proxies {
		pmap[p.ID] = p
		if addr, err := netip.ParseAddr(p.Host); err == nil && p.Port > 0 && p.Port <= 65535 {
			endpoints = append(endpoints, netip.AddrPortFrom(addr.Unmap(), uint16(p.Port)))
		}
	}
	e.conns.SetProxyEndpoints(endpoints)
	eng := rules.NewEngine(ruleList)

	e.mu.Lock()
	e.proxies = pmap
	e.engine = eng
	e.mu.Unlock()
}

func (e *WinDivertEngine) packetWorker(h windows.Handle) {
	pktBuf := make([]byte, 65535)
	var addr winDivertAddress
	var recvLen uint32

	for e.running.Load() {
		ok, _, _ := e.procRecv.Call(
			uintptr(h),
			uintptr(unsafe.Pointer(&pktBuf[0])),
			uintptr(len(pktBuf)),
			uintptr(unsafe.Pointer(&recvLen)),
			uintptr(unsafe.Pointer(&addr)),
		)
		if ok == 0 || recvLen == 0 {
			if !e.running.Load() {
				return
			}
			continue
		}

		pkt := pktBuf[:recvLen]
		drop := e.processPacket(pkt, &addr)
		if drop {
			continue
		}

		var sentLen uint32
		_, _, _ = e.procSend.Call(
			uintptr(h),
			uintptr(unsafe.Pointer(&pkt[0])),
			uintptr(len(pkt)),
			uintptr(unsafe.Pointer(&sentLen)),
			uintptr(unsafe.Pointer(&addr)),
		)
	}
}

func (e *WinDivertEngine) processPacket(pkt []byte, _ *winDivertAddress) (drop bool) {
	parsed, err := ParsePacket(pkt)
	if err != nil {
		return false
	}

	// 1. Tap inbound DNS UDP/53 responses into dnsmap
	if parsed.Protocol == ProtoUDP && parsed.SrcPort == 53 && len(pkt) > parsed.PayloadOff {
		if qname, ips, ok := dnsmap.ParseResponse(pkt[parsed.PayloadOff:]); ok {
			e.dns.Record(qname, ips, dnsmap.NowSec())
		}
		return false
	}

	// 2. Reverse NAT for replies coming from our local TCP/UDP relay
	if parsed.Protocol == ProtoTCP && parsed.SrcPort == DefaultTCPRelayPort {
		if tc, ok := e.conns.LookupTCP(parsed.DstPort); ok {
			_ = RewriteFromLocalRelay(pkt, parsed, tc.ClientSrcIP, tc.OrigDstIP, tc.OrigDstPort)
		}
		return false
	}
	if parsed.Protocol == ProtoUDP && parsed.SrcPort == DefaultUDPRelayPort {
		if tc, ok := e.conns.LookupUDPByClientPort(parsed.DstPort); ok {
			_ = RewriteFromLocalRelay(pkt, parsed, tc.ClientSrcIP, tc.OrigDstIP, tc.OrigDstPort)
		}
		return false
	}

	if !e.enabled.Load() {
		return false
	}

	// 3. Never intercept packets destined to a configured upstream proxy endpoint
	if e.conns.IsProxyEndpoint(parsed.DstIP, parsed.DstPort) {
		return false
	}

	// 4. TCP handling with per-source-port bitmap + fresh SYN stale-state reset
	if parsed.Protocol == ProtoTCP {
		if parsed.IsFreshSYN() {
			if evicted := e.conns.ResetStaleTCPState(parsed.SrcPort); evicted != nil && evicted.FlowID != "" {
				e.tracker.CloseFlow(evicted.FlowID)
			}
		} else {
			switch e.conns.GetTCPPortDecision(parsed.SrcPort) {
			case PortDecisionDirect:
				return false
			case PortDecisionBlocked:
				return true
			case PortDecisionTracked:
				_ = RewriteToLocalRelay(pkt, parsed, DefaultTCPRelayPort)
				return false
			}
		}

		pid, baseName, fullPath := e.resolver.ResolveSocket(parsed.SrcPort, parsed.Version == 6, false)
		if pid == e.selfPID {
			e.conns.SetTCPPortDecision(parsed.SrcPort, PortDecisionDirect)
			return false
		}

		dec, proxyCfg, domain := e.evaluatePacket(pid, baseName, fullPath, parsed, false)
		switch dec.Action {
		case pbprofile.ActionDirect:
			e.conns.SetTCPPortDecision(parsed.SrcPort, PortDecisionDirect)
			return false
		case pbprofile.ActionBlock:
			e.conns.SetTCPPortDecision(parsed.SrcPort, PortDecisionBlocked)
			return true
		case pbprofile.ActionProxy:
			if proxyCfg == nil {
				e.conns.SetTCPPortDecision(parsed.SrcPort, PortDecisionDirect)
				return false
			}
			flowID := e.tracker.StartFlow(flows.Flow{
				PID:         pid,
				ProcessName: baseName,
				DestIP:      parsed.DstIP.String(),
				Host:        domain,
				Port:        parsed.DstPort,
				IsUDP:       false,
				IsIPv6:      parsed.Version == 6,
				ProxyID:     proxyCfg.ID,
				ProxyLabel:  proxyCfg.Label(),
				RuleID:      dec.RuleID,
			})
			e.conns.TrackTCP(TrackedConn{
				Protocol:      ProtoTCP,
				ClientSrcIP:   parsed.SrcIP,
				ClientSrcPort: parsed.SrcPort,
				OrigDstIP:     parsed.DstIP,
				OrigDstPort:   parsed.DstPort,
				OrigDomain:    domain,
				PID:           pid,
				ProcessName:   baseName,
				ProxyConfigID: proxyCfg.ID,
				RuleID:        dec.RuleID,
				FlowID:        flowID,
			})
			_ = RewriteToLocalRelay(pkt, parsed, DefaultTCPRelayPort)
			return false
		}
	}

	// 5. UDP handling
	if parsed.Protocol == ProtoUDP {
		pid, baseName, fullPath := e.resolver.ResolveSocket(parsed.SrcPort, parsed.Version == 6, true)
		if pid == e.selfPID {
			return false
		}
		dec, proxyCfg, domain := e.evaluatePacket(pid, baseName, fullPath, parsed, true)
		switch dec.Action {
		case pbprofile.ActionDirect:
			return false
		case pbprofile.ActionBlock:
			return true
		case pbprofile.ActionProxy:
			if proxyCfg == nil || proxyCfg.Type != pbprofile.ProxySOCKS5 {
				return false
			}
			flowID := e.tracker.StartFlow(flows.Flow{
				PID:         pid,
				ProcessName: baseName,
				DestIP:      parsed.DstIP.String(),
				Host:        domain,
				Port:        parsed.DstPort,
				IsUDP:       true,
				IsIPv6:      parsed.Version == 6,
				ProxyID:     proxyCfg.ID,
				ProxyLabel:  proxyCfg.Label(),
				RuleID:        dec.RuleID,
			})
			e.conns.TrackUDP(TrackedConn{
				Protocol:      ProtoUDP,
				ClientSrcIP:   parsed.SrcIP,
				ClientSrcPort: parsed.SrcPort,
				OrigDstIP:     parsed.DstIP,
				OrigDstPort:   parsed.DstPort,
				OrigDomain:    domain,
				PID:           pid,
				ProcessName:   baseName,
				ProxyConfigID: proxyCfg.ID,
				RuleID:        dec.RuleID,
				FlowID:        flowID,
			})
			_ = RewriteToLocalRelay(pkt, parsed, DefaultUDPRelayPort)
			return false
		}
	}

	return false
}

func (e *WinDivertEngine) evaluatePacket(pid int, baseName, fullPath string, parsed ParsedPacket, isUDP bool) (rules.Decision, *pbprofile.ProxyConfig, string) {
	dstIPStr := parsed.DstIP.String()
	domains := e.dns.DomainsForIP(dstIPStr)
	var domain string
	if len(domains) > 0 {
		domain = domains[0]
	}

	proto := pbprofile.ProtoTCP
	if isUDP {
		proto = pbprofile.ProtoUDP
	}

	e.mu.RLock()
	eng := e.engine
	proxies := e.proxies
	e.mu.RUnlock()

	dec := eng.Evaluate(rules.FlowInput{
		Process:   rules.NewFlowProcess(baseName, fullPath),
		DestIP:    dstIPStr,
		Domains:   domains,
		DestPort:  parsed.DstPort,
		Protocol:  proto,
		CheckHost: true,
	})

	var proxyCfg *pbprofile.ProxyConfig
	proxyLabel := string(dec.Action)
	if dec.Action == pbprofile.ActionProxy {
		if p, ok := proxies[dec.ProxyConfigID]; ok {
			if isUDP && p.Type == pbprofile.ProxyHTTP {
				e.logs.WarnOnce("http-udp:"+p.ID, "HTTP proxy "+p.Label()+" does not support UDP; routing flow DIRECT")
				dec.Action = pbprofile.ActionDirect
				proxyLabel = "DIRECT"
			} else {
				cp := p
				proxyCfg = &cp
				proxyLabel = p.Label()
			}
		} else {
			e.logs.WarnOnce("missing-proxy:"+dec.ProxyConfigID, "Rule "+dec.RuleID+" references missing proxy "+dec.ProxyConfigID)
		}
	}

	e.logs.RecordConnection(flows.ConnectionLogEntry{
		PID:         pid,
		ProcessName: baseName,
		DestIP:      dstIPStr,
		Host:        domain,
		Port:        parsed.DstPort,
		Protocol:    string(proto),
		Action:      string(dec.Action),
		ProxyName:   proxyLabel,
		RuleID:      dec.RuleID,
	})

	return dec, proxyCfg, domain
}

func (e *WinDivertEngine) serveTCPRelay(ln net.Listener) {
	for {
		clientConn, err := ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			continue
		}
		go e.handleTCPRelayConn(clientConn)
	}
}

func (e *WinDivertEngine) handleTCPRelayConn(clientConn net.Conn) {
	defer clientConn.Close()

	tcpAddr, ok := clientConn.RemoteAddr().(*net.TCPAddr)
	if !ok {
		return
	}
	clientPort := uint16(tcpAddr.Port)

	tc, ok := e.conns.LookupTCP(clientPort)
	if !ok {
		return
	}
	defer func() {
		e.conns.RemoveTCP(clientPort)
		if tc.FlowID != "" {
			e.tracker.CloseFlow(tc.FlowID)
		}
	}()

	e.mu.RLock()
	p, exists := e.proxies[tc.ProxyConfigID]
	e.mu.RUnlock()
	if !exists {
		return
	}

	up := netproxy.Upstream{
		ID:                p.ID,
		Name:              p.Name,
		Type:              string(p.Type),
		Host:              p.Host,
		Port:              p.Port,
		Username:          p.Username,
		Password:          p.Password,
		SendDomainToProxy: p.SendDomainToProxy,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	upConn, err := up.DialTCP(ctx, tc.OrigDstIP.String(), tc.OrigDomain, tc.OrigDstPort)
	cancel()
	if err != nil {
		e.logs.LogActivity("ERROR", fmt.Sprintf("TCP relay handshake to %s via %s failed: %v", tc.OrigDstIP, up.Redacted(), err))
		return
	}
	defer upConn.Close()

	netproxy.Pipe(clientConn, upConn, func(sent int64) {
		if tc.FlowID != "" {
			e.tracker.AddBytes(tc.FlowID, sent, 0)
		}
	}, func(recv int64) {
		if tc.FlowID != "" {
			e.tracker.AddBytes(tc.FlowID, 0, recv)
		}
	})
}

func (e *WinDivertEngine) serveUDPRelay(uconn *net.UDPConn) {
	buf := make([]byte, 65535)
	for {
		n, clientAddr, err := uconn.ReadFromUDP(buf)
		if err != nil {
			return
		}
		clientPort := uint16(clientAddr.Port)
		tc, ok := e.conns.LookupUDPByClientPort(clientPort)
		if !ok {
			continue
		}
		payload := append([]byte(nil), buf[:n]...)
		go e.forwardUDPDatagram(uconn, clientAddr, tc, payload)
	}
}

func (e *WinDivertEngine) forwardUDPDatagram(uconn *net.UDPConn, clientAddr *net.UDPAddr, tc *TrackedConn, payload []byte) {
	e.mu.RLock()
	p, exists := e.proxies[tc.ProxyConfigID]
	e.mu.RUnlock()
	if !exists {
		return
	}

	up := netproxy.Upstream{
		ID:                p.ID,
		Name:              p.Name,
		Type:              string(p.Type),
		Host:              p.Host,
		Port:              p.Port,
		Username:          p.Username,
		Password:          p.Password,
		SendDomainToProxy: p.SendDomainToProxy,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	assoc, err := up.DialUDP(ctx)
	cancel()
	if err != nil {
		return
	}
	defer assoc.Close()

	if raddr := assoc.RelayAddr(); raddr != nil {
		if ip, err := netip.ParseAddr(strings.Split(raddr.IP.String(), "%")[0]); err == nil {
			e.conns.AddProxyEndpoint(netip.AddrPortFrom(ip.Unmap(), uint16(raddr.Port)))
		}
	}

	if _, err := assoc.SendDatagram(tc.OrigDstIP.String(), tc.OrigDomain, tc.OrigDstPort, payload); err != nil {
		return
	}
	if tc.FlowID != "" {
		e.tracker.AddBytes(tc.FlowID, int64(len(payload)), 0)
	}

	rbuf := make([]byte, 65535)
	_, _, replyPayload, err := assoc.ReadDatagram(rbuf, time.Now().Add(5*time.Second))
	if err != nil || len(replyPayload) == 0 {
		return
	}
	if tc.FlowID != "" {
		e.tracker.AddBytes(tc.FlowID, 0, int64(len(replyPayload)))
	}
	_, _ = uconn.WriteToUDP(replyPayload, clientAddr)
	_ = strconv.IntSize
}
