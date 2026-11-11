// SPDX-License-Identifier: AGPL-3.0-or-later

package netproxy

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	UDPIdleTimeout     = 120 * time.Second
	udpHeaderCacheSize = 64
)

var (
	ErrUDPNotSupportedOnHTTP = errors.New("HTTP proxies do not support raw UDP relay (SOCKS5 required)")
	ErrUDPFragmented         = errors.New("fragmented SOCKS5 UDP datagrams are not supported")
	ErrUDPAssociationClosed  = errors.New("SOCKS5 UDP association is closed")
)

type udpHeaderCacheEntry struct {
	key string
	hdr []byte
}

// UDPAssociation represents an active RFC 1928 SOCKS5 UDP ASSOCIATE session.
// It maintains both the TCP control connection and the UDP relay socket.
type UDPAssociation struct {
	upstream   Upstream
	ctrlConn   net.Conn
	udpConn    *net.UDPConn
	relayAddr  *net.UDPAddr
	sentBytes  atomic.Int64
	recvBytes  atomic.Int64
	lastActive atomic.Int64
	closed     atomic.Bool

	cacheMu sync.Mutex
	cache   []udpHeaderCacheEntry
	nextIdx int
}

// DialUDP establishes a SOCKS5 UDP ASSOCIATE session with the upstream proxy.
func (u Upstream) DialUDP(ctx context.Context) (*UDPAssociation, error) {
	if u.NormalizedType() != "SOCKS5" {
		return nil, ErrUDPNotSupportedOnHTTP
	}

	ctx, cancel := context.WithTimeout(ctx, dialTimeout)
	defer cancel()

	release, err := defaultLimiter.acquire(ctx, u.Addr())
	if err != nil {
		return nil, err
	}
	defer release()

	var d net.Dialer
	ctrlConn, err := d.DialContext(ctx, "tcp", u.Addr())
	if err != nil {
		return nil, err
	}
	if dl, ok := ctx.Deadline(); ok {
		_ = ctrlConn.SetDeadline(dl)
	}

	br := bufio.NewReader(ctrlConn)
	if err := socks5Authenticate(ctrlConn, br, u.Username, u.Password); err != nil {
		_ = ctrlConn.Close()
		return nil, err
	}

	// Send CMD = 0x03 (UDP ASSOCIATE) with 0.0.0.0:0
	assocReq := []byte{0x05, 0x03, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	if _, err := ctrlConn.Write(assocReq); err != nil {
		_ = ctrlConn.Close()
		return nil, fmt.Errorf("socks5 UDP ASSOCIATE write: %w", err)
	}

	bindHost, bindPort, err := readSOCKS5Reply(br)
	if err != nil {
		_ = ctrlConn.Close()
		return nil, fmt.Errorf("socks5 UDP ASSOCIATE reply: %w", err)
	}
	if bindPort == 0 {
		_ = ctrlConn.Close()
		return nil, errors.New("socks5 UDP ASSOCIATE returned port 0")
	}

	// RFC 1928: if BND.ADDR is 0.0.0.0 or ::, use the proxy's TCP control IP/host.
	if isUnspecifiedBindHost(bindHost) {
		if tcpAddr, ok := ctrlConn.RemoteAddr().(*net.TCPAddr); ok && tcpAddr.IP != nil {
			bindHost = tcpAddr.IP.String()
		} else {
			bindHost = u.Host
		}
	}

	_ = ctrlConn.SetDeadline(time.Time{})

	relayUDPAddr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(bindHost, strconv.Itoa(int(bindPort))))
	if err != nil {
		_ = ctrlConn.Close()
		return nil, fmt.Errorf("resolve UDP relay address: %w", err)
	}

	udpConn, err := net.DialUDP("udp", nil, relayUDPAddr)
	if err != nil {
		_ = ctrlConn.Close()
		return nil, fmt.Errorf("dial UDP relay %s: %w", relayUDPAddr.String(), err)
	}

	assoc := &UDPAssociation{
		upstream:  u,
		ctrlConn:  ctrlConn,
		udpConn:   udpConn,
		relayAddr: relayUDPAddr,
		cache:     make([]udpHeaderCacheEntry, 0, udpHeaderCacheSize),
	}
	assoc.lastActive.Store(time.Now().UnixNano())

	// Monitor the TCP control connection; RFC 1928 mandates that when the TCP
	// control connection closes, the UDP association terminates.
	go assoc.watchControlConn()

	return assoc, nil
}

func isUnspecifiedBindHost(h string) bool {
	h = strings.TrimSpace(h)
	if h == "" || h == "0.0.0.0" || h == "::" || h == "0:0:0:0:0:0:0:0" {
		return true
	}
	if ip := net.ParseIP(h); ip != nil && ip.IsUnspecified() {
		return true
	}
	return false
}

func (a *UDPAssociation) watchControlConn() {
	var buf [1]byte
	_, _ = a.ctrlConn.Read(buf[:])
	_ = a.Close()
}

// RelayAddr returns the resolved UDP relay endpoint reported by the SOCKS5 server.
func (a *UDPAssociation) RelayAddr() *net.UDPAddr {
	return a.relayAddr
}

// SentBytes returns total payload bytes sent through this UDP association.
func (a *UDPAssociation) SentBytes() int64 {
	return a.sentBytes.Load()
}

// ReceivedBytes returns total payload bytes received through this UDP association.
func (a *UDPAssociation) ReceivedBytes() int64 {
	return a.recvBytes.Load()
}

// LastActive returns the timestamp of the last datagram sent or received.
func (a *UDPAssociation) LastActive() time.Time {
	return time.Unix(0, a.lastActive.Load())
}

// IsClosed reports whether the UDP association has been torn down.
func (a *UDPAssociation) IsClosed() bool {
	return a.closed.Load()
}

// Close terminates both the TCP control connection and the UDP socket.
func (a *UDPAssociation) Close() error {
	if a.closed.Swap(true) {
		return nil
	}
	_ = a.udpConn.Close()
	return a.ctrlConn.Close()
}

func (a *UDPAssociation) headerFor(targetHost string, destPort uint16) ([]byte, error) {
	key := targetHost + ":" + strconv.Itoa(int(destPort))

	a.cacheMu.Lock()
	for i := range a.cache {
		if a.cache[i].key == key {
			hdr := a.cache[i].hdr
			a.cacheMu.Unlock()
			return hdr, nil
		}
	}
	a.cacheMu.Unlock()

	hdr, err := PackSOCKS5UDPHeader(targetHost, destPort)
	if err != nil {
		return nil, err
	}

	a.cacheMu.Lock()
	if len(a.cache) < udpHeaderCacheSize {
		a.cache = append(a.cache, udpHeaderCacheEntry{key: key, hdr: hdr})
	} else {
		a.cache[a.nextIdx] = udpHeaderCacheEntry{key: key, hdr: hdr}
		a.nextIdx = (a.nextIdx + 1) % udpHeaderCacheSize
	}
	a.cacheMu.Unlock()

	return hdr, nil
}

// SendDatagram encapsulates payload in an RFC 1928 SOCKS5 UDP header and writes it to the relay.
func (a *UDPAssociation) SendDatagram(destIP, destDomain string, destPort uint16, payload []byte) (int, error) {
	if a.IsClosed() {
		return 0, ErrUDPAssociationClosed
	}

	targetHost := a.upstream.TargetHost(destIP, destDomain)
	hdr, err := a.headerFor(targetHost, destPort)
	if err != nil {
		return 0, err
	}

	pkt := make([]byte, len(hdr)+len(payload))
	copy(pkt, hdr)
	copy(pkt[len(hdr):], payload)

	if _, err := a.udpConn.Write(pkt); err != nil {
		return 0, err
	}

	a.lastActive.Store(time.Now().UnixNano())
	a.sentBytes.Add(int64(len(payload)))
	return len(payload), nil
}

// ReadDatagram reads a single UDP datagram from the SOCKS5 relay and strips its RFC 1928 header.
func (a *UDPAssociation) ReadDatagram(buf []byte, deadline time.Time) (srcHost string, srcPort uint16, payload []byte, err error) {
	if a.IsClosed() {
		return "", 0, nil, ErrUDPAssociationClosed
	}
	if !deadline.IsZero() {
		_ = a.udpConn.SetReadDeadline(deadline)
	}
	n, err := a.udpConn.Read(buf)
	if err != nil {
		if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
			_ = a.Close()
		}
		return "", 0, nil, err
	}
	srcHost, srcPort, payload, err = UnpackSOCKS5UDPDatagram(buf[:n])
	if err != nil {
		return "", 0, nil, err
	}
	a.lastActive.Store(time.Now().UnixNano())
	a.recvBytes.Add(int64(len(payload)))
	return srcHost, srcPort, payload, nil
}

// PackSOCKS5UDPHeader builds the [RSV=0x00, 0x00][FRAG=0x00][ATYP][DST.ADDR][DST.PORT] prefix.
func PackSOCKS5UDPHeader(host string, port uint16) ([]byte, error) {
	addrBytes, err := buildSOCKS5Address(host, port)
	if err != nil {
		return nil, err
	}
	hdr := make([]byte, 3+len(addrBytes))
	hdr[0] = 0x00
	hdr[1] = 0x00
	hdr[2] = 0x00 // FRAG = 0
	copy(hdr[3:], addrBytes)
	return hdr, nil
}

// UnpackSOCKS5UDPDatagram parses an RFC 1928 SOCKS5 UDP frame and returns the origin address and inner payload.
func UnpackSOCKS5UDPDatagram(pkt []byte) (host string, port uint16, payload []byte, err error) {
	if len(pkt) < 10 {
		return "", 0, nil, fmt.Errorf("socks5 UDP datagram too short (%d bytes)", len(pkt))
	}
	if pkt[2] != 0x00 {
		return "", 0, nil, ErrUDPFragmented
	}

	atyp := pkt[3]
	offset := 4
	switch atyp {
	case 0x01: // IPv4
		if len(pkt) < offset+4+2 {
			return "", 0, nil, errors.New("truncated socks5 IPv4 UDP header")
		}
		host = net.IP(pkt[offset : offset+4]).String()
		offset += 4
	case 0x04: // IPv6
		if len(pkt) < offset+16+2 {
			return "", 0, nil, errors.New("truncated socks5 IPv6 UDP header")
		}
		host = net.IP(pkt[offset : offset+16]).String()
		offset += 16
	case 0x03: // Domain
		dlen := int(pkt[offset])
		offset++
		if len(pkt) < offset+dlen+2 {
			return "", 0, nil, errors.New("truncated socks5 domain UDP header")
		}
		host = string(pkt[offset : offset+dlen])
		offset += dlen
	default:
		return "", 0, nil, fmt.Errorf("unknown socks5 UDP ATYP 0x%02x", atyp)
	}

	port = binary.BigEndian.Uint16(pkt[offset : offset+2])
	offset += 2
	return host, port, pkt[offset:], nil
}
