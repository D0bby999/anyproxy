// SPDX-License-Identifier: AGPL-3.0-or-later

// Package netproxy implements SOCKS5 (TCP + UDP ASSOCIATE) and HTTP CONNECT
// upstream dialing, bidirectional stream piping with live byte counters, and
// the 5-step diagnostic Proxy Checker.
package netproxy

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	dialTimeout           = 20 * time.Second
	idleTimeout           = 10 * time.Minute
	maxHandshakesPerProxy = 48
)

var (
	ErrUpstreamAuth      = errors.New("upstream proxy rejected credentials")
	ErrHandshakeBusy     = errors.New("upstream proxy has too many concurrent handshakes in flight")
	ErrUnsupportedScheme = errors.New("unsupported proxy protocol")
)

// Upstream describes a configured SOCKS5 or HTTP proxy server.
type Upstream struct {
	ID                string
	Name              string
	Type              string // "SOCKS5" | "HTTP"
	Host              string
	Port              int
	Username          string
	Password          string
	SendDomainToProxy bool
}

// Addr returns host:port formatted for net.Dial.
func (u Upstream) Addr() string {
	return net.JoinHostPort(u.Host, strconv.Itoa(u.Port))
}

// NormalizedType returns "SOCKS5" or "HTTP".
func (u Upstream) NormalizedType() string {
	if strings.EqualFold(strings.TrimSpace(u.Type), "HTTP") {
		return "HTTP"
	}
	return "SOCKS5"
}

// TargetHost chooses the destination host sent to the upstream proxy based on SendDomainToProxy.
func (u Upstream) TargetHost(destIP, destDomain string) string {
	if u.SendDomainToProxy && strings.TrimSpace(destDomain) != "" {
		return strings.TrimSuffix(strings.TrimSpace(destDomain), ".")
	}
	if strings.TrimSpace(destIP) != "" {
		return strings.TrimSpace(destIP)
	}
	return strings.TrimSuffix(strings.TrimSpace(destDomain), ".")
}

// Redacted returns a log-safe representation without password.
func (u Upstream) Redacted() string {
	scheme := strings.ToLower(u.NormalizedType())
	user := ""
	if u.Username != "" {
		user = u.Username + "@"
	}
	return scheme + "://" + user + u.Addr()
}

func (u Upstream) basicAuth() string {
	if u.Username == "" && u.Password == "" {
		return ""
	}
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(u.Username+":"+u.Password))
}

// bufConn preserves bytes buffered during the proxy handshake (e.g. server-first protocols like SSH/SMTP).
type bufConn struct {
	net.Conn
	r *bufio.Reader
}

func (c *bufConn) Read(p []byte) (int, error) {
	return c.r.Read(p)
}

func (c *bufConn) CloseWrite() error {
	if tc, ok := c.Conn.(interface{ CloseWrite() error }); ok {
		return tc.CloseWrite()
	}
	return nil
}

// HandshakeLimiter caps concurrent in-flight handshakes per upstream proxy
// so a stalled proxy does not starve healthy proxies.
type HandshakeLimiter struct {
	mu   sync.Mutex
	sems map[string]chan struct{}
}

var defaultLimiter = &HandshakeLimiter{
	sems: make(map[string]chan struct{}),
}

func (l *HandshakeLimiter) acquire(ctx context.Context, key string) (func(), error) {
	l.mu.Lock()
	ch, ok := l.sems[key]
	if !ok {
		ch = make(chan struct{}, maxHandshakesPerProxy)
		l.sems[key] = ch
	}
	l.mu.Unlock()

	select {
	case ch <- struct{}{}:
		return func() { <-ch }, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("%w: %v", ErrHandshakeBusy, ctx.Err())
	}
}

// DialTCP opens a proxied TCP connection to (destIP/destDomain):destPort through u.
// If the proxy handshake fails, it returns an error immediately and never falls back to direct.
func (u Upstream) DialTCP(ctx context.Context, destIP, destDomain string, destPort uint16) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(ctx, dialTimeout)
	defer cancel()

	release, err := defaultLimiter.acquire(ctx, u.Addr())
	if err != nil {
		return nil, err
	}
	defer release()

	targetHost := u.TargetHost(destIP, destDomain)
	if targetHost == "" {
		return nil, errors.New("empty destination host")
	}

	switch u.NormalizedType() {
	case "SOCKS5":
		return u.dialSOCKS5TCP(ctx, targetHost, destPort)
	case "HTTP":
		return u.dialHTTPConnect(ctx, targetHost, destPort)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedScheme, u.Type)
	}
}

func (u Upstream) dialSOCKS5TCP(ctx context.Context, targetHost string, destPort uint16) (net.Conn, error) {
	var d net.Dialer
	c, err := d.DialContext(ctx, "tcp", u.Addr())
	if err != nil {
		return nil, err
	}
	if dl, ok := ctx.Deadline(); ok {
		_ = c.SetDeadline(dl)
	}

	br := bufio.NewReader(c)
	if err := socks5Authenticate(c, br, u.Username, u.Password); err != nil {
		_ = c.Close()
		return nil, err
	}

	req, err := buildSOCKS5Request(0x01, targetHost, destPort) // CMD = 0x01 (CONNECT)
	if err != nil {
		_ = c.Close()
		return nil, err
	}
	if _, err := c.Write(req); err != nil {
		_ = c.Close()
		return nil, err
	}

	if _, _, err := readSOCKS5Reply(br); err != nil {
		_ = c.Close()
		return nil, err
	}

	_ = c.SetDeadline(time.Time{})
	return &bufConn{Conn: c, r: br}, nil
}

func (u Upstream) dialHTTPConnect(ctx context.Context, targetHost string, destPort uint16) (net.Conn, error) {
	var d net.Dialer
	c, err := d.DialContext(ctx, "tcp", u.Addr())
	if err != nil {
		return nil, err
	}
	if dl, ok := ctx.Deadline(); ok {
		_ = c.SetDeadline(dl)
	}

	target := net.JoinHostPort(targetHost, strconv.Itoa(int(destPort)))
	var sb strings.Builder
	sb.WriteString("CONNECT ")
	sb.WriteString(target)
	sb.WriteString(" HTTP/1.1\r\nHost: ")
	sb.WriteString(target)
	sb.WriteString("\r\nProxy-Connection: keep-alive\r\n")
	if auth := u.basicAuth(); auth != "" {
		sb.WriteString("Proxy-Authorization: ")
		sb.WriteString(auth)
		sb.WriteString("\r\n")
	}
	sb.WriteString("\r\n")

	if _, err := c.Write([]byte(sb.String())); err != nil {
		_ = c.Close()
		return nil, err
	}

	br := bufio.NewReader(c)
	resp, err := http.ReadResponse(br, &http.Request{Method: http.MethodConnect})
	if err != nil {
		_ = c.Close()
		return nil, err
	}
	_ = resp.Body.Close()

	if resp.StatusCode == http.StatusProxyAuthRequired || resp.StatusCode == http.StatusUnauthorized {
		_ = c.Close()
		return nil, fmt.Errorf("%w: HTTP %d", ErrUpstreamAuth, resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		_ = c.Close()
		return nil, fmt.Errorf("HTTP CONNECT %s failed: %s", target, resp.Status)
	}

	_ = c.SetDeadline(time.Time{})
	return &bufConn{Conn: c, r: br}, nil
}

// socks5Authenticate negotiates RFC 1928 method selection and optional RFC 1929 username/password auth.
func socks5Authenticate(w io.Writer, r io.Reader, username, password string) error {
	hasAuth := username != "" || password != ""
	var greeting []byte
	if hasAuth {
		greeting = []byte{0x05, 0x02, 0x00, 0x02}
	} else {
		greeting = []byte{0x05, 0x01, 0x00}
	}
	if _, err := w.Write(greeting); err != nil {
		return fmt.Errorf("socks5 greeting write: %w", err)
	}

	var methodReply [2]byte
	if _, err := io.ReadFull(r, methodReply[:]); err != nil {
		return fmt.Errorf("socks5 greeting read: %w", err)
	}
	if methodReply[0] != 0x05 {
		return fmt.Errorf("socks5 invalid version: 0x%02x", methodReply[0])
	}

	switch methodReply[1] {
	case 0x00:
		return nil
	case 0x02:
		if len(username) > 255 || len(password) > 255 {
			return errors.New("socks5 credentials exceed 255 bytes")
		}
		authReq := make([]byte, 0, 3+len(username)+len(password))
		authReq = append(authReq, 0x01, byte(len(username)))
		authReq = append(authReq, username...)
		authReq = append(authReq, byte(len(password)))
		authReq = append(authReq, password...)
		if _, err := w.Write(authReq); err != nil {
			return fmt.Errorf("socks5 auth write: %w", err)
		}
		var authReply [2]byte
		if _, err := io.ReadFull(r, authReply[:]); err != nil {
			return fmt.Errorf("socks5 auth read: %w", err)
		}
		if authReply[1] != 0x00 {
			return fmt.Errorf("%w: socks5 status 0x%02x", ErrUpstreamAuth, authReply[1])
		}
		return nil
	case 0xFF:
		return fmt.Errorf("%w: socks5 no acceptable auth methods", ErrUpstreamAuth)
	default:
		return fmt.Errorf("socks5 unsupported auth method: 0x%02x", methodReply[1])
	}
}

// buildSOCKS5Address encodes host:port into [ATYP][ADDR][PORT].
func buildSOCKS5Address(host string, port uint16) ([]byte, error) {
	var out []byte
	if addr, err := netip.ParseAddr(host); err == nil {
		u := addr.Unmap()
		if u.Is4() {
			ip4 := u.As4()
			out = make([]byte, 1+4+2)
			out[0] = 0x01
			copy(out[1:5], ip4[:])
			binary.BigEndian.PutUint16(out[5:7], port)
			return out, nil
		}
		ip6 := u.As16()
		out = make([]byte, 1+16+2)
		out[0] = 0x04
		copy(out[1:17], ip6[:])
		binary.BigEndian.PutUint16(out[17:19], port)
		return out, nil
	}

	if len(host) == 0 || len(host) > 255 {
		return nil, fmt.Errorf("invalid socks5 domain length: %d", len(host))
	}
	out = make([]byte, 1+1+len(host)+2)
	out[0] = 0x03
	out[1] = byte(len(host))
	copy(out[2:2+len(host)], host)
	binary.BigEndian.PutUint16(out[2+len(host):], port)
	return out, nil
}

func buildSOCKS5Request(cmd byte, host string, port uint16) ([]byte, error) {
	addrBytes, err := buildSOCKS5Address(host, port)
	if err != nil {
		return nil, err
	}
	req := make([]byte, 3+len(addrBytes))
	req[0] = 0x05
	req[1] = cmd
	req[2] = 0x00
	copy(req[3:], addrBytes)
	return req, nil
}

// readSOCKS5Reply reads the exact RFC 1928 reply frame without consuming any subsequent payload bytes.
func readSOCKS5Reply(r io.Reader) (bindHost string, bindPort uint16, err error) {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return "", 0, fmt.Errorf("socks5 reply header: %w", err)
	}
	if hdr[0] != 0x05 {
		return "", 0, fmt.Errorf("socks5 invalid reply version: 0x%02x", hdr[0])
	}
	if hdr[1] != 0x00 {
		return "", 0, fmt.Errorf("socks5 command rejected with code 0x%02x", hdr[1])
	}

	switch hdr[3] {
	case 0x01: // IPv4
		var ip [4]byte
		if _, err := io.ReadFull(r, ip[:]); err != nil {
			return "", 0, err
		}
		bindHost = net.IP(ip[:]).String()
	case 0x04: // IPv6
		var ip [16]byte
		if _, err := io.ReadFull(r, ip[:]); err != nil {
			return "", 0, err
		}
		bindHost = net.IP(ip[:]).String()
	case 0x03: // Domain
		var dlen [1]byte
		if _, err := io.ReadFull(r, dlen[:]); err != nil {
			return "", 0, err
		}
		dom := make([]byte, int(dlen[0]))
		if _, err := io.ReadFull(r, dom); err != nil {
			return "", 0, err
		}
		bindHost = string(dom)
	default:
		return "", 0, fmt.Errorf("socks5 unknown address type: 0x%02x", hdr[3])
	}

	var portBuf [2]byte
	if _, err := io.ReadFull(r, portBuf[:]); err != nil {
		return "", 0, err
	}
	bindPort = binary.BigEndian.Uint16(portBuf[:])
	return bindHost, bindPort, nil
}
