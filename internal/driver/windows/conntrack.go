// SPDX-License-Identifier: MIT

package windows

import (
	"net/netip"
	"sync"
	"sync/atomic"
	"time"
)

const (
	PortDecisionUnknown uint32 = 0
	PortDecisionDirect  uint32 = 1
	PortDecisionTracked uint32 = 2
	PortDecisionBlocked uint32 = 3

	ConnIdleTimeout       = 120 * time.Second
	UDPRevalidateInterval = 30 * time.Second
)

// TrackedConn stores original destination metadata and routing assignment for a redirected flow.
type TrackedConn struct {
	Protocol      uint8
	ClientSrcIP   netip.Addr
	ClientSrcPort uint16
	OrigDstIP     netip.Addr
	OrigDstPort   uint16
	OrigDomain    string
	PID           int
	ProcessName   string
	ProxyConfigID string
	RuleID        string
	FlowID        string
	CreatedAt     time.Time
	LastSeen      time.Time
}

type udpReverseKey struct {
	clientPort uint16
	origDstIP  netip.Addr
	origPort   uint16
}

// ConnTable maintains O(1) forward and reverse connection tracking tables,
// a 65536-port atomic decision bitmap, and proxy endpoint loop exclusion.
type ConnTable struct {
	tcpDecision [65536]atomic.Uint32

	mu             sync.RWMutex
	tcpByPort      map[uint16]*TrackedConn
	udpByPort      map[uint16]*TrackedConn
	udpReverse     map[udpReverseKey]*TrackedConn
	proxyEndpoints map[netip.AddrPort]struct{}
}

// NewConnTable creates a connection tracking table.
func NewConnTable() *ConnTable {
	return &ConnTable{
		tcpByPort:      make(map[uint16]*TrackedConn, 256),
		udpByPort:      make(map[uint16]*TrackedConn, 256),
		udpReverse:     make(map[udpReverseKey]*TrackedConn, 256),
		proxyEndpoints: make(map[netip.AddrPort]struct{}, 16),
	}
}

// SetProxyEndpoints updates the set of upstream proxy TCP/UDP endpoints that must never be intercepted.
func (ct *ConnTable) SetProxyEndpoints(endpoints []netip.AddrPort) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	clear(ct.proxyEndpoints)
	for _, ep := range endpoints {
		ct.proxyEndpoints[netip.AddrPortFrom(ep.Addr().Unmap(), ep.Port())] = struct{}{}
	}
}

// AddProxyEndpoint dynamically adds a discovered UDP relay endpoint to the loop-exclusion set.
func (ct *ConnTable) AddProxyEndpoint(ep netip.AddrPort) {
	ct.mu.Lock()
	ct.proxyEndpoints[netip.AddrPortFrom(ep.Addr().Unmap(), ep.Port())] = struct{}{}
	ct.mu.Unlock()
}

// IsProxyEndpoint reports whether dstIP:dstPort belongs to a configured upstream proxy or UDP relay.
func (ct *ConnTable) IsProxyEndpoint(dstIP netip.Addr, dstPort uint16) bool {
	key := netip.AddrPortFrom(dstIP.Unmap(), dstPort)
	ct.mu.RLock()
	_, ok := ct.proxyEndpoints[key]
	ct.mu.RUnlock()
	return ok
}

// GetTCPPortDecision returns the cached fast-path decision for srcPort.
func (ct *ConnTable) GetTCPPortDecision(srcPort uint16) uint32 {
	return ct.tcpDecision[srcPort].Load()
}

// SetTCPPortDecision caches a fast-path decision (DIRECT, TRACKED, BLOCKED) for srcPort.
func (ct *ConnTable) SetTCPPortDecision(srcPort uint16, decision uint32) {
	ct.tcpDecision[srcPort].Store(decision)
}

// ResetStaleTCPState clears any cached decision and tracked connection for srcPort
// when a fresh outbound SYN (without ACK) arrives on a reused ephemeral port.
func (ct *ConnTable) ResetStaleTCPState(srcPort uint16) (evicted *TrackedConn) {
	ct.tcpDecision[srcPort].Store(PortDecisionUnknown)
	ct.mu.Lock()
	if old, ok := ct.tcpByPort[srcPort]; ok {
		evicted = old
		delete(ct.tcpByPort, srcPort)
	}
	ct.mu.Unlock()
	return evicted
}

// TrackTCP records a newly redirected TCP connection and sets its bitmap entry to PortDecisionTracked.
func (ct *ConnTable) TrackTCP(conn TrackedConn) {
	now := time.Now()
	if conn.CreatedAt.IsZero() {
		conn.CreatedAt = now
	}
	conn.LastSeen = now
	conn.ClientSrcIP = conn.ClientSrcIP.Unmap()
	conn.OrigDstIP = conn.OrigDstIP.Unmap()

	ct.mu.Lock()
	cp := conn
	ct.tcpByPort[conn.ClientSrcPort] = &cp
	ct.mu.Unlock()

	ct.tcpDecision[conn.ClientSrcPort].Store(PortDecisionTracked)
}

// LookupTCP retrieves a tracked TCP connection by its original client source port in O(1).
func (ct *ConnTable) LookupTCP(clientSrcPort uint16) (*TrackedConn, bool) {
	now := time.Now()
	ct.mu.Lock()
	defer ct.mu.Unlock()
	c, ok := ct.tcpByPort[clientSrcPort]
	if !ok {
		return nil, false
	}
	c.LastSeen = now
	cp := *c
	return &cp, true
}

// RemoveTCP removes a tracked TCP connection and resets its port bitmap entry.
func (ct *ConnTable) RemoveTCP(clientSrcPort uint16) *TrackedConn {
	ct.tcpDecision[clientSrcPort].Store(PortDecisionUnknown)
	ct.mu.Lock()
	old := ct.tcpByPort[clientSrcPort]
	delete(ct.tcpByPort, clientSrcPort)
	ct.mu.Unlock()
	return old
}

// TrackUDP records an outbound UDP flow in both forward and reverse O(1) lookup tables.
func (ct *ConnTable) TrackUDP(conn TrackedConn) {
	now := time.Now()
	if conn.CreatedAt.IsZero() {
		conn.CreatedAt = now
	}
	conn.LastSeen = now
	conn.ClientSrcIP = conn.ClientSrcIP.Unmap()
	conn.OrigDstIP = conn.OrigDstIP.Unmap()

	rkey := udpReverseKey{
		clientPort: conn.ClientSrcPort,
		origDstIP:  conn.OrigDstIP,
		origPort:   conn.OrigDstPort,
	}

	ct.mu.Lock()
	cp := conn
	ct.udpByPort[conn.ClientSrcPort] = &cp
	ct.udpReverse[rkey] = &cp
	ct.mu.Unlock()
}

// LookupUDPByClientPort looks up the latest UDP destination for clientSrcPort in O(1).
func (ct *ConnTable) LookupUDPByClientPort(clientSrcPort uint16) (*TrackedConn, bool) {
	now := time.Now()
	ct.mu.Lock()
	defer ct.mu.Unlock()
	c, ok := ct.udpByPort[clientSrcPort]
	if !ok {
		return nil, false
	}
	c.LastSeen = now
	cp := *c
	return &cp, true
}

// LookupUDPReverse looks up a tracked UDP flow by (clientSrcPort, origDstIP, origDstPort) in O(1).
func (ct *ConnTable) LookupUDPReverse(clientSrcPort uint16, origDstIP netip.Addr, origDstPort uint16) (*TrackedConn, bool) {
	now := time.Now()
	rkey := udpReverseKey{
		clientPort: clientSrcPort,
		origDstIP:  origDstIP.Unmap(),
		origPort:   origDstPort,
	}
	ct.mu.Lock()
	defer ct.mu.Unlock()
	c, ok := ct.udpReverse[rkey]
	if !ok {
		return nil, false
	}
	c.LastSeen = now
	cp := *c
	return &cp, true
}

// SweepIdle evicts TCP and UDP entries that have been idle longer than maxIdle.
func (ct *ConnTable) SweepIdle(now time.Time, maxIdle time.Duration) (evicted []*TrackedConn) {
	if maxIdle <= 0 {
		maxIdle = ConnIdleTimeout
	}
	ct.mu.Lock()
	defer ct.mu.Unlock()

	for port, c := range ct.tcpByPort {
		if now.Sub(c.LastSeen) > maxIdle {
			evicted = append(evicted, c)
			delete(ct.tcpByPort, port)
			ct.tcpDecision[port].Store(PortDecisionUnknown)
		}
	}
	for port, c := range ct.udpByPort {
		if now.Sub(c.LastSeen) > maxIdle {
			evicted = append(evicted, c)
			delete(ct.udpByPort, port)
		}
	}
	for k, c := range ct.udpReverse {
		if now.Sub(c.LastSeen) > maxIdle {
			delete(ct.udpReverse, k)
		}
	}
	return evicted
}

