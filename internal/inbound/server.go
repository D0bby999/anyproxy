// SPDX-License-Identifier: MIT

// Package inbound implements a local dual-protocol (SOCKS5 and HTTP CONNECT)
// forward proxy server, allowing AnyProfile, browsers, curl, and CLI tools
// to route traffic through AnyProxy's rule engine without requiring OS-level
// driver interception (NetworkExtension or WinDivert).
package inbound

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/D0bby999/anyproxy/internal/dnsmap"
	"github.com/D0bby999/anyproxy/internal/flows"
	"github.com/D0bby999/anyproxy/internal/netproxy"
	"github.com/D0bby999/anyproxy/internal/pbprofile"
	"github.com/D0bby999/anyproxy/internal/rules"
	"github.com/D0bby999/anyproxy/internal/secrets"
)

// DefaultListenAddr is the default local forward proxy endpoint.
const DefaultListenAddr = "127.0.0.1:10800"

// Config configures the Inbound Proxy Server.
type Config struct {
	ListenAddr      string
	Vault           secrets.Store
	DNS             *dnsmap.Store
	Tracker         *flows.Tracker
	Logs            *flows.LogManager
	ProcessResolver ProcessResolverFunc
}

// Server is the local proxy listener accepting SOCKS5 and HTTP CONNECT.
type Server struct {
	mu         sync.RWMutex
	listenAddr string
	listener   net.Listener
	vault      secrets.Store
	dns        *dnsmap.Store
	tracker    *flows.Tracker
	logs       *flows.LogManager
	resolver   ProcessResolverFunc
	engine     *rules.Engine
	proxies    map[string]pbprofile.ProxyConfig
	defProxyID string
	running    atomic.Bool
	stopCh     chan struct{}
	wg         sync.WaitGroup
}

// NewServer creates a new Inbound Server instance.
func NewServer(cfg Config) *Server {
	addr := cfg.ListenAddr
	if addr == "" {
		addr = DefaultListenAddr
	}
	res := cfg.ProcessResolver
	if res == nil {
		res = newPIDResolver().Resolve
	}
	return &Server{
		listenAddr: addr,
		vault:      cfg.Vault,
		dns:        cfg.DNS,
		tracker:    cfg.Tracker,
		logs:       cfg.Logs,
		resolver:   res,
		engine:     rules.NewEngine(nil),
		proxies:    make(map[string]pbprofile.ProxyConfig),
		stopCh:     make(chan struct{}),
	}
}

// ListenAddr returns the actual listening address.
func (s *Server) ListenAddr() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return s.listenAddr
}

// SyncProfile updates the active rules engine and proxy configurations.
func (s *Server) SyncProfile(proxies []pbprofile.ProxyConfig, ruleList []pbprofile.ProxyRule, defaultProxyID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.engine = rules.NewEngine(ruleList)
	s.defProxyID = defaultProxyID
	s.proxies = make(map[string]pbprofile.ProxyConfig, len(proxies))
	for _, px := range proxies {
		s.proxies[px.ID] = px
	}
}

// Start opens the local TCP listener and accepts client connections.
func (s *Server) Start() error {
	if s.running.Swap(true) {
		return nil // already running
	}

	ln, err := net.Listen("tcp", s.listenAddr)
	if err != nil {
		s.running.Store(false)
		return fmt.Errorf("inbound listen %s: %w", s.listenAddr, err)
	}

	s.mu.Lock()
	s.listener = ln
	s.stopCh = make(chan struct{})
	s.mu.Unlock()

	s.wg.Add(1)
	go s.serve(ln)

	if s.logs != nil {
		s.logs.LogActivity("INFO", fmt.Sprintf("Inbound local proxy listening on %s (SOCKS5/HTTP)", ln.Addr()))
	}
	return nil
}

// Stop closes the listener and waits for active connections to finish.
func (s *Server) Stop() error {
	if !s.running.Swap(false) {
		return nil
	}

	s.mu.Lock()
	if s.listener != nil {
		_ = s.listener.Close()
	}
	close(s.stopCh)
	s.mu.Unlock()

	done := make(chan struct{})
	go func() { s.wg.Wait(); close(done) }()
	select {
	case <-done:
	// shortcut: open relays are abandoned, not closed; fine because Stop only runs at app quit.
	case <-time.After(2 * time.Second):
	}
	return nil
}

func (s *Server) serve(ln net.Listener) {
	defer s.wg.Done()
	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-s.stopCh:
				return
			default:
				if ne, ok := err.(net.Error); ok && ne.Temporary() {
					time.Sleep(10 * time.Millisecond)
					continue
				}
				return
			}
		}

		s.wg.Add(1)
		go func(c net.Conn) {
			defer s.wg.Done()
			s.handleConnection(c)
		}(conn)
	}
}

func (s *Server) handleConnection(client net.Conn) {
	defer client.Close()

	reader := bufio.NewReader(client)
	firstByte, err := reader.Peek(1)
	if err != nil {
		return
	}

	if firstByte[0] == 0x05 {
		s.handleSOCKS5(client, reader)
	} else {
		s.handleHTTP(client, reader)
	}
}

func (s *Server) handleSOCKS5(client net.Conn, reader *bufio.Reader) {
	// Handshake: read version & auth methods
	header := make([]byte, 2)
	if _, err := io.ReadFull(reader, header); err != nil {
		return
	}
	numMethods := int(header[1])
	methods := make([]byte, numMethods)
	if _, err := io.ReadFull(reader, methods); err != nil {
		return
	}

	// Reply: version 5, no authentication required (0x00)
	if _, err := client.Write([]byte{0x05, 0x00}); err != nil {
		return
	}

	// Request: version, cmd, rsv, address type
	reqHeader := make([]byte, 4)
	if _, err := io.ReadFull(reader, reqHeader); err != nil {
		return
	}
	if reqHeader[1] != 0x01 { // 0x01 = CONNECT
		// Command not supported
		_, _ = client.Write([]byte{0x05, 0x07, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}

	var destHost string
	atyp := reqHeader[3]
	switch atyp {
	case 0x01: // IPv4
		ipBuf := make([]byte, 4)
		if _, err := io.ReadFull(reader, ipBuf); err != nil {
			return
		}
		destHost = net.IP(ipBuf).String()
	case 0x03: // Domain name
		lenBuf := make([]byte, 1)
		if _, err := io.ReadFull(reader, lenBuf); err != nil {
			return
		}
		domainBuf := make([]byte, lenBuf[0])
		if _, err := io.ReadFull(reader, domainBuf); err != nil {
			return
		}
		destHost = string(domainBuf)
	case 0x04: // IPv6
		ipBuf := make([]byte, 16)
		if _, err := io.ReadFull(reader, ipBuf); err != nil {
			return
		}
		destHost = net.IP(ipBuf).String()
	default:
		_, _ = client.Write([]byte{0x05, 0x08, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}

	var portBytes [2]byte
	if _, err := io.ReadFull(reader, portBytes[:]); err != nil {
		return
	}
	destPort := binary.BigEndian.Uint16(portBytes[:])

	// Evaluate routing rule using resolved local process identity
	proc := s.resolveProcess(client.RemoteAddr())
	decision, targetProxy := s.evaluateTargetForProcess(destHost, destPort, proc)

	if decision.Action == pbprofile.ActionBlock {
		// Connection not allowed by ruleset (0x02)
		_, _ = client.Write([]byte{0x05, 0x02, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		s.recordLog(proc, destHost, destPort, "BLOCK", "", decision.RuleID)
		return
	}

	// Dial upstream (either DIRECT or PROXY)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	var upstream net.Conn
	var dialErr error
	var proxyName string

	if decision.Action == pbprofile.ActionProxy && targetProxy != nil {
		proxyName = targetProxy.Name
		up := s.buildUpstream(*targetProxy)
		upstream, dialErr = up.DialTCP(ctx, destHost, destHost, destPort)
	} else {
		// DIRECT
		targetAddr := net.JoinHostPort(destHost, strconv.Itoa(int(destPort)))
		upstream, dialErr = net.DialTimeout("tcp", targetAddr, 15*time.Second)
	}

	if dialErr != nil {
		// Host unreachable (0x04)
		_, _ = client.Write([]byte{0x05, 0x04, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		s.recordLog(proc, destHost, destPort, string(decision.Action), proxyName, decision.RuleID)
		return
	}
	defer upstream.Close()

	// Success reply: 0x05 0x00 (success) + BND.ADDR (0.0.0.0:0)
	if _, err := client.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0}); err != nil {
		return
	}

	// Wrap client conn with unread buffer if any
	wrappedClient := wrapWithReader(client, reader)
	s.relayTraffic(wrappedClient, upstream, proc, destHost, destPort, string(decision.Action), proxyName, decision.RuleID)
}

func (s *Server) handleHTTP(client net.Conn, reader *bufio.Reader) {
	reqLine, err := reader.ReadString('\n')
	if err != nil {
		return
	}
	parts := strings.Fields(reqLine)
	if len(parts) < 2 {
		return
	}
	method := parts[0]
	target := parts[1]
	protoVer := "HTTP/1.1"
	if len(parts) >= 3 {
		protoVer = parts[2]
	}

	proc := s.resolveProcess(client.RemoteAddr())
	var destHost string
	var destPort uint16 = 80

	if strings.EqualFold(method, "CONNECT") {
		// CONNECT host:port HTTP/1.1
		h, p, err := net.SplitHostPort(target)
		if err == nil {
			destHost = h
			if parsed, err := strconv.ParseUint(p, 10, 16); err == nil {
				destPort = uint16(parsed)
			}
		} else {
			destHost = target
			destPort = 443
		}

		// Read & discard remaining HTTP headers until empty line
		for {
			line, err := reader.ReadString('\n')
			if err != nil || strings.TrimSpace(line) == "" {
				break
			}
		}

		decision, targetProxy := s.evaluateTargetForProcess(destHost, destPort, proc)
		if decision.Action == pbprofile.ActionBlock {
			_, _ = client.Write([]byte("HTTP/1.1 403 Forbidden\r\nConnection: close\r\n\r\nBlocked by AnyProxy rules\r\n"))
			s.recordLog(proc, destHost, destPort, "BLOCK", "", decision.RuleID)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		var upstream net.Conn
		var dialErr error
		var proxyName string

		if decision.Action == pbprofile.ActionProxy && targetProxy != nil {
			proxyName = targetProxy.Name
			up := s.buildUpstream(*targetProxy)
			upstream, dialErr = up.DialTCP(ctx, destHost, destHost, destPort)
		} else {
			targetAddr := net.JoinHostPort(destHost, strconv.Itoa(int(destPort)))
			upstream, dialErr = net.DialTimeout("tcp", targetAddr, 15*time.Second)
		}

		if dialErr != nil {
			_, _ = client.Write([]byte("HTTP/1.1 502 Bad Gateway\r\nConnection: close\r\n\r\nFailed to connect to upstream\r\n"))
			s.recordLog(proc, destHost, destPort, string(decision.Action), proxyName, decision.RuleID)
			return
		}
		defer upstream.Close()

		// 200 Connection Established
		if _, err := client.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
			return
		}

		wrappedClient := wrapWithReader(client, reader)
		s.relayTraffic(wrappedClient, upstream, proc, destHost, destPort, string(decision.Action), proxyName, decision.RuleID)
		return
	}

	// Plain HTTP proxy request (e.g., GET http://example.com/path HTTP/1.1)
	parsedURL, err := url.Parse(target)
	if err != nil || parsedURL.Host == "" {
		_, _ = client.Write([]byte("HTTP/1.1 400 Bad Request\r\nConnection: close\r\n\r\nAnyProxy local listener supports HTTP proxy, HTTPS CONNECT, and SOCKS5\r\n"))
		return
	}
	if h, p, err := net.SplitHostPort(parsedURL.Host); err == nil {
		destHost = h
		if parsed, err := strconv.ParseUint(p, 10, 16); err == nil {
			destPort = uint16(parsed)
		}
	} else {
		destHost = parsedURL.Host
	}

	decision, targetProxy := s.evaluateTargetForProcess(destHost, destPort, proc)
	if decision.Action == pbprofile.ActionBlock {
		_, _ = client.Write([]byte("HTTP/1.1 403 Forbidden\r\nConnection: close\r\n\r\nBlocked by AnyProxy rules\r\n"))
		s.recordLog(proc, destHost, destPort, "BLOCK", "", decision.RuleID)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	var upstream net.Conn
	var dialErr error
	var proxyName string

	if decision.Action == pbprofile.ActionProxy && targetProxy != nil {
		proxyName = targetProxy.Name
		up := s.buildUpstream(*targetProxy)
		upstream, dialErr = up.DialTCP(ctx, destHost, destHost, destPort)
	} else {
		targetAddr := net.JoinHostPort(destHost, strconv.Itoa(int(destPort)))
		upstream, dialErr = net.DialTimeout("tcp", targetAddr, 15*time.Second)
	}
	if dialErr != nil {
		_, _ = client.Write([]byte("HTTP/1.1 502 Bad Gateway\r\nConnection: close\r\n\r\nFailed to connect to upstream\r\n"))
		s.recordLog(proc, destHost, destPort, string(decision.Action), proxyName, decision.RuleID)
		return
	}
	defer upstream.Close()

	relURI := parsedURL.RequestURI()
	if relURI == "" {
		relURI = "/"
	}
	if _, err := fmt.Fprintf(upstream, "%s %s %s\r\n", method, relURI, protoVer); err != nil {
		return
	}
	wrappedClient := wrapWithReader(client, reader)
	s.relayTraffic(wrappedClient, upstream, proc, destHost, destPort, string(decision.Action), proxyName, decision.RuleID)
}

func (s *Server) resolveProcess(addr net.Addr) ProcessInfo {
	if s.resolver != nil {
		info := s.resolver(addr)
		if info.Name != "" {
			return info
		}
	}
	return ProcessInfo{Name: "unknown"}
}

func (s *Server) evaluateTarget(host string, port uint16) (rules.Decision, *pbprofile.ProxyConfig) {
	return s.evaluateTargetForProcess(host, port, ProcessInfo{Name: "anyprofile"})
}

func (s *Server) evaluateTargetForProcess(host string, port uint16, proc ProcessInfo) (rules.Decision, *pbprofile.ProxyConfig) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Prevent AnyProxy's own process from looping back through the proxy
	if strings.EqualFold(proc.Name, "AnyProxy") {
		return rules.Decision{Action: pbprofile.ActionDirect, RuleID: "self-bypass"}, nil
	}

	destIP := ""
	var domains []string
	if addr, err := netip.ParseAddr(host); err == nil {
		destIP = addr.String()
		if s.dns != nil {
			domains = s.dns.DomainsForIP(destIP)
		}
	} else {
		domains = append(domains, host)
	}

	procName := proc.Name
	if procName == "" {
		procName = "anyprofile"
	}

	flowInput := rules.FlowInput{
		Process:   rules.NewFlowProcess(procName, proc.Path),
		DestIP:    destIP,
		Domains:   domains,
		DestPort:  port,
		Protocol:  pbprofile.ProtoTCP,
		CheckHost: true,
	}

	var dec rules.Decision
	if s.engine != nil {
		dec = s.engine.Evaluate(flowInput)
	} else {
		dec = rules.Decision{Action: pbprofile.ActionDirect}
	}

	// If no rule matched and the user has selected an Active/Default Proxy, route through it automatically.
	if !dec.Matched && s.defProxyID != "" {
		dec = rules.Decision{
			Matched:       true,
			Action:        pbprofile.ActionProxy,
			ProxyConfigID: s.defProxyID,
			RuleID:        "default",
		}
	}

	var px *pbprofile.ProxyConfig
	if dec.Action == pbprofile.ActionProxy {
		targetID := dec.ProxyConfigID
		if targetID == "" {
			targetID = s.defProxyID
		}
		if p, ok := s.proxies[targetID]; ok {
			cp := p
			px = &cp
		} else {
			// A missing proxy must never silently send traffic directly.
			dec.Action = pbprofile.ActionBlock
		}
	}

	return dec, px
}

func (s *Server) buildUpstream(cfg pbprofile.ProxyConfig) netproxy.Upstream {
	password := cfg.Password
	if s.vault != nil {
		if pw, err := s.vault.Get(secrets.ServiceProxy, cfg.ID); err == nil && len(pw) > 0 {
			password = string(pw)
		}
	}
	return netproxy.Upstream{
		ID:                cfg.ID,
		Name:              cfg.Name,
		Type:              string(cfg.Type),
		Host:              cfg.Host,
		Port:              cfg.Port,
		Username:          cfg.Username,
		Password:          password,
		SendDomainToProxy: cfg.SendDomainToProxy,
	}
}

func (s *Server) relayTraffic(client, upstream net.Conn, proc ProcessInfo, destHost string, destPort uint16, action, proxyName, ruleID string) {
	start := time.Now()
	var flowID string

	procLabel := proc.Name
	if procLabel == "" {
		procLabel = "inbound"
	}
	proxyLabel := proxyName
	if proxyLabel == "" {
		proxyLabel = strings.ToUpper(action)
	}
	if ruleID == "" {
		ruleID = "default"
	}

	if s.tracker != nil {
		flowID = s.tracker.StartFlow(flows.Flow{
			PID:         proc.PID,
			ProcessName: procLabel,
			DestIP:      destHost,
			Host:        destHost,
			Port:        destPort,
			IsUDP:       false,
			ProxyLabel:  proxyLabel,
			RuleID:      ruleID,
			StartedAt:   start.UnixMilli(),
		})
	}

	onSent := func(n int64) {
		if s.tracker != nil && flowID != "" {
			s.tracker.AddBytes(flowID, n, 0)
		}
	}
	onRecv := func(n int64) {
		if s.tracker != nil && flowID != "" {
			s.tracker.AddBytes(flowID, 0, n)
		}
	}

	_, _ = netproxy.Pipe(client, upstream, onSent, onRecv)

	if s.tracker != nil && flowID != "" {
		s.tracker.CloseFlow(flowID)
	}

	s.recordLog(proc, destHost, destPort, action, proxyName, ruleID)
}

func (s *Server) recordLog(proc ProcessInfo, destHost string, destPort uint16, action, proxyName, ruleID string) {
	if s.logs == nil {
		return
	}
	procLabel := proc.Name
	if procLabel == "" {
		procLabel = "inbound"
	}
	proxyLabel := proxyName
	if proxyLabel == "" {
		proxyLabel = strings.ToUpper(action)
	}
	s.logs.RecordConnection(flows.ConnectionLogEntry{
		PID:         proc.PID,
		ProcessName: procLabel,
		DestIP:      destHost,
		Host:        destHost,
		Port:        destPort,
		Protocol:    "TCP",
		Action:      strings.ToUpper(action),
		ProxyName:   proxyLabel,
		RuleID:      ruleID,
	})
}

func parseHostPort(addr net.Addr) (string, uint16) {
	if addr == nil {
		return "127.0.0.1", 0
	}
	host, portStr, err := net.SplitHostPort(addr.String())
	if err != nil {
		return addr.String(), 0
	}
	p, _ := strconv.ParseUint(portStr, 10, 16)
	return host, uint16(p)
}

type readerConn struct {
	net.Conn
	r io.Reader
}

func (c *readerConn) Read(b []byte) (int, error) {
	return c.r.Read(b)
}

func wrapWithReader(conn net.Conn, r *bufio.Reader) net.Conn {
	return &readerConn{
		Conn: conn,
		r:    r,
	}
}
