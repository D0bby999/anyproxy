// SPDX-License-Identifier: AGPL-3.0-or-later

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

	connsMu sync.Mutex
	conns   map[net.Conn]struct{}
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
		conns:      make(map[net.Conn]struct{}),
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

func (s *Server) registerConn(c net.Conn) {
	s.connsMu.Lock()
	if s.conns != nil {
		s.conns[c] = struct{}{}
	}
	s.connsMu.Unlock()
}

func (s *Server) unregisterConn(c net.Conn) {
	s.connsMu.Lock()
	if s.conns != nil {
		delete(s.conns, c)
	}
	s.connsMu.Unlock()
}

// CloseActiveConnections closes all in-flight client connections so clients re-handshake under new profile rules.
func (s *Server) CloseActiveConnections() {
	s.connsMu.Lock()
	var toClose []net.Conn
	for c := range s.conns {
		toClose = append(toClose, c)
	}
	s.connsMu.Unlock()

	for _, c := range toClose {
		_ = c.Close()
	}
}

// SyncProfile updates the active rules engine and proxy configurations.
func (s *Server) SyncProfile(proxies []pbprofile.ProxyConfig, ruleList []pbprofile.ProxyRule, defaultProxyID string) {
	s.mu.Lock()
	s.engine = rules.NewEngine(ruleList)
	s.defProxyID = defaultProxyID
	s.proxies = make(map[string]pbprofile.ProxyConfig, len(proxies))
	for _, px := range proxies {
		s.proxies[px.ID] = px
	}
	s.mu.Unlock()

	s.CloseActiveConnections()
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

	s.CloseActiveConnections()

	done := make(chan struct{})
	go func() { s.wg.Wait(); close(done) }()
	select {
	case <-done:
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
	s.registerConn(client)
	defer func() {
		s.unregisterConn(client)
		_ = client.Close()
	}()

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
	cmd := reqHeader[1]
	if cmd != 0x01 && cmd != 0x03 { // 0x01 = CONNECT, 0x03 = UDP ASSOCIATE
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

	if cmd == 0x03 {
		s.handleSOCKS5UDPAssociate(client, reader)
		return
	}

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
	} else if decision.Action == pbprofile.ActionDirect {
		targetAddr := net.JoinHostPort(destHost, strconv.Itoa(int(destPort)))
		upstream, dialErr = net.DialTimeout("tcp", targetAddr, 15*time.Second)
	} else {
		_, _ = client.Write([]byte{0x05, 0x02, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		s.recordLog(proc, destHost, destPort, "BLOCK", "", decision.RuleID)
		return
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
		} else if decision.Action == pbprofile.ActionDirect {
			targetAddr := net.JoinHostPort(destHost, strconv.Itoa(int(destPort)))
			upstream, dialErr = net.DialTimeout("tcp", targetAddr, 15*time.Second)
		} else {
			_, _ = client.Write([]byte("HTTP/1.1 403 Forbidden\r\nConnection: close\r\n\r\nBlocked by AnyProxy rules\r\n"))
			s.recordLog(proc, destHost, destPort, "BLOCK", "", decision.RuleID)
			return
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
	} else if decision.Action == pbprofile.ActionDirect {
		targetAddr := net.JoinHostPort(destHost, strconv.Itoa(int(destPort)))
		upstream, dialErr = net.DialTimeout("tcp", targetAddr, 15*time.Second)
	} else {
		_, _ = client.Write([]byte("HTTP/1.1 403 Forbidden\r\nConnection: close\r\n\r\nBlocked by AnyProxy rules\r\n"))
		s.recordLog(proc, destHost, destPort, "BLOCK", "", decision.RuleID)
		return
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

func (s *Server) handleSOCKS5UDPAssociate(client net.Conn, reader *bufio.Reader) {
	bindIP := net.IPv4(127, 0, 0, 1)
	if tcpAddr, ok := client.LocalAddr().(*net.TCPAddr); ok && len(tcpAddr.IP) > 0 {
		bindIP = tcpAddr.IP
	}
	udpConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: bindIP, Port: 0})
	if err != nil {
		_, _ = client.Write([]byte{0x05, 0x01, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}
	defer udpConn.Close()

	localUDP := udpConn.LocalAddr().(*net.UDPAddr)
	var reply []byte
	if ip4 := localUDP.IP.To4(); ip4 != nil {
		reply = make([]byte, 10)
		reply[0] = 0x05
		reply[1] = 0x00
		reply[2] = 0x00
		reply[3] = 0x01
		copy(reply[4:8], ip4)
		binary.BigEndian.PutUint16(reply[8:10], uint16(localUDP.Port))
	} else {
		ip6 := localUDP.IP.To16()
		reply = make([]byte, 22)
		reply[0] = 0x05
		reply[1] = 0x00
		reply[2] = 0x00
		reply[3] = 0x04
		copy(reply[4:20], ip6)
		binary.BigEndian.PutUint16(reply[20:22], uint16(localUDP.Port))
	}
	if _, err := client.Write(reply); err != nil {
		return
	}

	proc := s.resolveProcess(client.RemoteAddr())

	var (
		clientMu      sync.RWMutex
		clientUDPAddr *net.UDPAddr
		directConn    *net.UDPConn
		upAssocs      = make(map[string]*netproxy.UDPAssociation)
		flowIDs       = make(map[string]string)
	)
	defer func() {
		clientMu.Lock()
		if directConn != nil {
			_ = directConn.Close()
		}
		for _, a := range upAssocs {
			_ = a.Close()
		}
		ids := make([]string, 0, len(flowIDs))
		for _, id := range flowIDs {
			ids = append(ids, id)
		}
		clientMu.Unlock()
		if s.tracker != nil {
			for _, id := range ids {
				s.tracker.CloseFlow(id)
			}
		}
	}()

	// RFC 1928: close the UDP relay socket when the TCP control connection closes.
	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		var buf [1]byte
		_, _ = reader.Read(buf[:])
		_ = udpConn.Close()
	}()

	writeToClient := func(srcHost string, srcPort uint16, payload []byte, flowID string) {
		hdr, err := netproxy.PackSOCKS5UDPHeader(srcHost, srcPort)
		if err != nil {
			return
		}
		clientMu.RLock()
		cAddr := clientUDPAddr
		clientMu.RUnlock()
		if cAddr == nil {
			return
		}
		pkt := make([]byte, len(hdr)+len(payload))
		copy(pkt, hdr)
		copy(pkt[len(hdr):], payload)
		if _, err := udpConn.WriteToUDP(pkt, cAddr); err == nil && s.tracker != nil && flowID != "" {
			s.tracker.AddBytes(flowID, 0, int64(len(payload)))
		}
	}

	buf := make([]byte, 65535)
	for {
		_ = udpConn.SetReadDeadline(time.Now().Add(netproxy.UDPIdleTimeout))
		n, srcAddr, err := udpConn.ReadFromUDP(buf)
		if err != nil {
			return
		}
		if !srcAddr.IP.IsLoopback() {
			continue
		}
		clientMu.Lock()
		clientUDPAddr = srcAddr
		clientMu.Unlock()

		targetHost, targetPort, payload, err := netproxy.UnpackSOCKS5UDPDatagram(buf[:n])
		if err != nil || targetPort == 0 {
			continue
		}

		decision, targetProxy := s.evaluateTargetForProcessProto(targetHost, targetPort, proc, pbprofile.ProtoUDP)
		if decision.Action == pbprofile.ActionBlock {
			s.recordLogProto(proc, targetHost, targetPort, "UDP", "BLOCK", "", decision.RuleID)
			continue
		}

		if decision.Action == pbprofile.ActionProxy && targetProxy != nil && strings.ToUpper(string(targetProxy.Type)) != "SOCKS5" {
			if s.logs != nil {
				s.logs.WarnOnce("udp-http:"+targetProxy.ID, "HTTP/HTTPS proxy does not support UDP; blocking UDP traffic to prevent direct fallback")
			}
			s.recordLogProto(proc, targetHost, targetPort, "UDP", "BLOCK", targetProxy.Name, decision.RuleID)
			continue
		}

		flowKey := string(decision.Action) + "|" + targetHost + ":" + strconv.Itoa(int(targetPort))
		proxyName := ""
		if targetProxy != nil {
			proxyName = targetProxy.Name
		}

		clientMu.Lock()
		flowID := flowIDs[flowKey]
		if flowID == "" && s.tracker != nil {
			procLabel := proc.Name
			if procLabel == "" {
				procLabel = "inbound"
			}
			proxyLabel := proxyName
			if proxyLabel == "" {
				proxyLabel = strings.ToUpper(string(decision.Action))
			}
			ruleID := decision.RuleID
			if ruleID == "" {
				ruleID = "default"
			}
			flowID = s.tracker.StartFlow(flows.Flow{
				PID:         proc.PID,
				ProcessName: procLabel,
				DestIP:      targetHost,
				Host:        targetHost,
				Port:        targetPort,
				IsUDP:       true,
				ProxyLabel:  proxyLabel,
				RuleID:      ruleID,
				StartedAt:   time.Now().UnixMilli(),
			})
			flowIDs[flowKey] = flowID
		}
		clientMu.Unlock()

		s.recordLogProto(proc, targetHost, targetPort, "UDP", string(decision.Action), proxyName, decision.RuleID)

		if decision.Action == pbprofile.ActionProxy && targetProxy != nil {
			clientMu.Lock()
			assoc := upAssocs[targetProxy.ID]
			if assoc == nil || assoc.IsClosed() {
				up := s.buildUpstream(*targetProxy)
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				newAssoc, dialErr := up.DialUDP(ctx)
				cancel()
				if dialErr != nil {
					clientMu.Unlock()
					continue
				}
				assoc = newAssoc
				upAssocs[targetProxy.ID] = assoc
				fid := flowID
				go func(a *netproxy.UDPAssociation) {
					rbuf := make([]byte, 65535)
					for {
						sh, sp, pl, rerr := a.ReadDatagram(rbuf, time.Now().Add(netproxy.UDPIdleTimeout))
						if rerr != nil {
							return
						}
						writeToClient(sh, sp, pl, fid)
					}
				}(assoc)
			}
			clientMu.Unlock()

			destIP := ""
			if parsedIP, perr := netip.ParseAddr(targetHost); perr == nil {
				destIP = parsedIP.String()
			}
			if sent, werr := assoc.SendDatagram(destIP, targetHost, targetPort, payload); werr != nil {
				_ = assoc.Close()
			} else if s.tracker != nil && flowID != "" {
				s.tracker.AddBytes(flowID, int64(sent), 0)
			}
			continue
		}

		if decision.Action != pbprofile.ActionDirect {
			continue
		}

		// DIRECT UDP relay
		dstUDPAddr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(targetHost, strconv.Itoa(int(targetPort))))
		if err != nil {
			continue
		}
		clientMu.Lock()
		dConn := directConn
		if dConn == nil {
			dConn, err = net.ListenUDP("udp", nil)
			if err != nil {
				clientMu.Unlock()
				continue
			}
			directConn = dConn
			fid := flowID
			go func(dc *net.UDPConn) {
				rbuf := make([]byte, 65535)
				for {
					_ = dc.SetReadDeadline(time.Now().Add(netproxy.UDPIdleTimeout))
					rn, raddr, rerr := dc.ReadFromUDP(rbuf)
					if rerr != nil {
						return
					}
					writeToClient(raddr.IP.String(), uint16(raddr.Port), rbuf[:rn], fid)
				}
			}(dConn)
		}
		clientMu.Unlock()

		if _, err := dConn.WriteToUDP(payload, dstUDPAddr); err == nil && s.tracker != nil && flowID != "" {
			s.tracker.AddBytes(flowID, int64(len(payload)), 0)
		}
	}
}

func (s *Server) evaluateTarget(host string, port uint16) (rules.Decision, *pbprofile.ProxyConfig) {
	return s.evaluateTargetForProcess(host, port, ProcessInfo{Name: "anyprofile"})
}

func (s *Server) evaluateTargetForProcess(host string, port uint16, proc ProcessInfo) (rules.Decision, *pbprofile.ProxyConfig) {
	return s.evaluateTargetForProcessProto(host, port, proc, pbprofile.ProtoTCP)
}

func (s *Server) evaluateTargetForProcessProto(host string, port uint16, proc ProcessInfo, proto pbprofile.RuleProtocol) (rules.Decision, *pbprofile.ProxyConfig) {
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
		Protocol:  proto,
		CheckHost: true,
	}

	var dec rules.Decision
	if s.engine != nil {
		dec = s.engine.Evaluate(flowInput)
	} else {
		dec = rules.Decision{Action: pbprofile.ActionDirect}
	}

	// If no rule matched and the user has selected an Active/Default Proxy, route through it automatically,
	// except mandatory direct destinations (loopback, multicast, link-local, DHCP, stub DNS :53).
	if !dec.Matched && s.defProxyID != "" {
		if rules.ShouldBypassProxy(destIP, port, proto == pbprofile.ProtoUDP, false) {
			dec = rules.Decision{
				Matched: true,
				Action:  pbprofile.ActionDirect,
				RuleID:  "default-bypass",
			}
		} else {
			dec = rules.Decision{
				Matched:       true,
				Action:        pbprofile.ActionProxy,
				ProxyConfigID: s.defProxyID,
				RuleID:        "default",
			}
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
	s.recordLogProto(proc, destHost, destPort, "TCP", action, proxyName, ruleID)
}

func (s *Server) recordLogProto(proc ProcessInfo, destHost string, destPort uint16, proto, action, proxyName, ruleID string) {
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
		Protocol:    proto,
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
