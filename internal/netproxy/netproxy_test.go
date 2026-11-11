// SPDX-License-Identifier: MIT

package netproxy

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type fakeSOCKS5Server struct {
	ln           net.Listener
	udpConn      *net.UDPConn
	wantUser     string
	wantPass     string
	lastATYP     atomic.Uint32
	lastDestHost atomic.Value
}

func startFakeSOCKS5(t *testing.T, user, pass string, enableUDP bool) *fakeSOCKS5Server {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp: %v", err)
	}
	s := &fakeSOCKS5Server{
		ln:       ln,
		wantUser: user,
		wantPass: pass,
	}
	t.Cleanup(func() { _ = ln.Close() })

	if enableUDP {
		uaddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("resolve udp: %v", err)
		}
		uconn, err := net.ListenUDP("udp", uaddr)
		if err != nil {
			t.Fatalf("listen udp: %v", err)
		}
		s.udpConn = uconn
		t.Cleanup(func() { _ = uconn.Close() })
		go s.serveUDP()
	}

	go s.serveTCP()
	return s
}

func (s *fakeSOCKS5Server) hostPort() (string, int) {
	addr := s.ln.Addr().(*net.TCPAddr)
	return addr.IP.String(), addr.Port
}

func (s *fakeSOCKS5Server) serveUDP() {
	buf := make([]byte, 4096)
	for {
		n, clientAddr, err := s.udpConn.ReadFromUDP(buf)
		if err != nil {
			return
		}
		host, port, payload, err := UnpackSOCKS5UDPDatagram(buf[:n])
		if err != nil {
			continue
		}
		// Echo back prefixed with the same SOCKS5 UDP header and "echo:" prefix
		hdr, err := PackSOCKS5UDPHeader(host, port)
		if err != nil {
			continue
		}
		replyPayload := append([]byte("udp-echo:"), payload...)
		pkt := append(hdr, replyPayload...)
		_, _ = s.udpConn.WriteToUDP(pkt, clientAddr)
	}
}

func (s *fakeSOCKS5Server) serveTCP() {
	for {
		c, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.handleConn(c)
	}
}

func (s *fakeSOCKS5Server) handleConn(c net.Conn) {
	defer c.Close()

	var hdr [2]byte
	if _, err := io.ReadFull(c, hdr[:]); err != nil {
		return
	}
	methods := make([]byte, int(hdr[1]))
	if _, err := io.ReadFull(c, methods); err != nil {
		return
	}

	if s.wantUser != "" {
		_, _ = c.Write([]byte{0x05, 0x02})
		var ver [1]byte
		if _, err := io.ReadFull(c, ver[:]); err != nil {
			return
		}
		var ulen [1]byte
		if _, err := io.ReadFull(c, ulen[:]); err != nil {
			return
		}
		ubuf := make([]byte, int(ulen[0]))
		if _, err := io.ReadFull(c, ubuf); err != nil {
			return
		}
		var plen [1]byte
		if _, err := io.ReadFull(c, plen[:]); err != nil {
			return
		}
		pbuf := make([]byte, int(plen[0]))
		if _, err := io.ReadFull(c, pbuf); err != nil {
			return
		}
		if string(ubuf) != s.wantUser || string(pbuf) != s.wantPass {
			_, _ = c.Write([]byte{0x01, 0x01})
			return
		}
		_, _ = c.Write([]byte{0x01, 0x00})
	} else {
		_, _ = c.Write([]byte{0x05, 0x00})
	}

	var reqHdr [4]byte
	if _, err := io.ReadFull(c, reqHdr[:]); err != nil {
		return
	}
	cmd := reqHdr[1]
	atyp := reqHdr[3]
	s.lastATYP.Store(uint32(atyp))

	var destHost string
	switch atyp {
	case 0x01:
		var ip [4]byte
		if _, err := io.ReadFull(c, ip[:]); err != nil {
			return
		}
		destHost = net.IP(ip[:]).String()
	case 0x04:
		var ip [16]byte
		if _, err := io.ReadFull(c, ip[:]); err != nil {
			return
		}
		destHost = net.IP(ip[:]).String()
	case 0x03:
		var dlen [1]byte
		if _, err := io.ReadFull(c, dlen[:]); err != nil {
			return
		}
		dom := make([]byte, int(dlen[0]))
		if _, err := io.ReadFull(c, dom); err != nil {
			return
		}
		destHost = string(dom)
	}
	s.lastDestHost.Store(destHost)

	var portBuf [2]byte
	if _, err := io.ReadFull(c, portBuf[:]); err != nil {
		return
	}
	destPort := binary.BigEndian.Uint16(portBuf[:])

	switch cmd {
	case 0x01: // CONNECT
		target, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(int(destPort))))
		if err != nil {
			_, _ = c.Write([]byte{0x05, 0x05, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
			return
		}
		defer target.Close()
		_, _ = c.Write([]byte{0x05, 0x00, 0x00, 0x01, 127, 0, 0, 1, byte(destPort >> 8), byte(destPort)})
		Pipe(c, target, nil, nil)

	case 0x03: // UDP ASSOCIATE
		if s.udpConn == nil {
			_, _ = c.Write([]byte{0x05, 0x07, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
			return
		}
		uport := uint16(s.udpConn.LocalAddr().(*net.UDPAddr).Port)
		// Intentionally return 0.0.0.0 as bind IP to verify RFC 1928 proxy-host substitution
		_, _ = c.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, byte(uport >> 8), byte(uport)})
		var b [1]byte
		_, _ = c.Read(b[:])
	}
}

func startEchoHTTPTarget(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen target: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				br := bufio.NewReader(conn)
				line, err := br.ReadString('\n')
				if err != nil {
					return
				}
				if strings.HasPrefix(line, "HEAD ") || strings.HasPrefix(line, "GET ") {
					_, _ = conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n"))
					return
				}
				_, _ = conn.Write([]byte("echo:" + line))
			}(c)
		}
	}()

	return ln.Addr().(*net.TCPAddr).Port
}

func startFakeHTTPConnectProxy(t *testing.T, wantAuthHeader string) (string, int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen http proxy: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				br := bufio.NewReader(conn)
				req, err := http.ReadRequest(br)
				if err != nil {
					return
				}
				if wantAuthHeader != "" && req.Header.Get("Proxy-Authorization") != wantAuthHeader {
					_, _ = conn.Write([]byte("HTTP/1.1 407 Proxy Authentication Required\r\n\r\n"))
					return
				}
				_, portStr, err := net.SplitHostPort(req.Host)
				if err != nil {
					_, _ = conn.Write([]byte("HTTP/1.1 400 Bad Request\r\n\r\n"))
					return
				}
				target, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", portStr))
				if err != nil {
					_, _ = conn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
					return
				}
				defer target.Close()
				_, _ = conn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
				Pipe(conn, target, nil, nil)
			}(c)
		}
	}()

	addr := ln.Addr().(*net.TCPAddr)
	return addr.IP.String(), addr.Port
}

func TestSOCKS5TCPAndSendDomainToProxy(t *testing.T) {
	targetPort := startEchoHTTPTarget(t)
	s5 := startFakeSOCKS5(t, "alice", "secret", true)
	host, port := s5.hostPort()

	up := Upstream{
		ID:                "p1",
		Name:              "Test SOCKS5",
		Type:              "SOCKS5",
		Host:              host,
		Port:              port,
		Username:          "alice",
		Password:          "secret",
		SendDomainToProxy: true,
	}

	ctx := context.Background()
	conn, err := up.DialTCP(ctx, "127.0.0.1", "example.internal", uint16(targetPort))
	if err != nil {
		t.Fatalf("DialTCP with SendDomainToProxy=true failed: %v", err)
	}
	if got := s5.lastATYP.Load(); got != 0x03 {
		t.Fatalf("expected ATYP=0x03 (domain), got 0x%02x", got)
	}
	if got, _ := s5.lastDestHost.Load().(string); got != "example.internal" {
		t.Fatalf("expected destHost=example.internal, got %q", got)
	}

	_, _ = conn.Write([]byte("hello-proxybridge\n"))
	reply, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		t.Fatalf("read reply: %v", err)
	}
	if strings.TrimSpace(reply) != "echo:hello-proxybridge" {
		t.Fatalf("unexpected reply: %q", reply)
	}
	_ = conn.Close()

	// Now with SendDomainToProxy = false -> must send IPv4 ATYP=0x01
	up.SendDomainToProxy = false
	conn2, err := up.DialTCP(ctx, "127.0.0.1", "example.internal", uint16(targetPort))
	if err != nil {
		t.Fatalf("DialTCP with SendDomainToProxy=false failed: %v", err)
	}
	_ = conn2.Close()
	if got := s5.lastATYP.Load(); got != 0x01 {
		t.Fatalf("expected ATYP=0x01 (IPv4), got 0x%02x", got)
	}

	// Wrong password must fail with ErrUpstreamAuth and never leak
	up.Password = "wrong"
	if _, err := up.DialTCP(ctx, "127.0.0.1", "", uint16(targetPort)); !errors.Is(err, ErrUpstreamAuth) {
		t.Fatalf("expected ErrUpstreamAuth, got %v", err)
	}
}

func TestSOCKS5UDPAssociateAndCheckProxyEx(t *testing.T) {
	targetPort := startEchoHTTPTarget(t)
	s5 := startFakeSOCKS5(t, "bob", "hunter2", true)
	host, port := s5.hostPort()

	up := Upstream{
		ID:                "socks-udp",
		Name:              "UDP SOCKS5",
		Type:              "SOCKS5",
		Host:              host,
		Port:              port,
		Username:          "bob",
		Password:          "hunter2",
		SendDomainToProxy: true,
	}

	ctx := context.Background()
	assoc, err := up.DialUDP(ctx)
	if err != nil {
		t.Fatalf("DialUDP failed: %v", err)
	}
	defer assoc.Close()

	payload := []byte("dns-query-payload")
	n, err := assoc.SendDatagram("8.8.8.8", "dns.google", 53, payload)
	if err != nil || n != len(payload) {
		t.Fatalf("SendDatagram failed: n=%d, err=%v", n, err)
	}

	buf := make([]byte, 2048)
	srcHost, srcPort, gotPayload, err := assoc.ReadDatagram(buf, time.Now().Add(3*time.Second))
	if err != nil {
		t.Fatalf("ReadDatagram failed: %v", err)
	}
	if srcHost != "dns.google" || srcPort != 53 {
		t.Fatalf("unexpected UDP source %s:%d", srcHost, srcPort)
	}
	if string(gotPayload) != "udp-echo:dns-query-payload" {
		t.Fatalf("unexpected UDP payload %q", string(gotPayload))
	}
	if assoc.SentBytes() != int64(len(payload)) || assoc.ReceivedBytes() != int64(len(gotPayload)) {
		t.Fatalf("byte counter mismatch: sent=%d recv=%d", assoc.SentBytes(), assoc.ReceivedBytes())
	}

	// Verify 5-step CheckProxyEx
	var steps []CheckStep
	report := CheckProxyEx(ctx, up, "127.0.0.1", uint16(targetPort), func(s CheckStep) {
		steps = append(steps, s)
	})
	if !report.Success || !report.UDPOK || len(steps) != 5 {
		t.Fatalf("expected successful 5-step report with UDPOK=true, got: %+v", report)
	}
}

func TestHTTPConnectAndPipeByteCounters(t *testing.T) {
	targetPort := startEchoHTTPTarget(t)
	upRef := Upstream{Username: "proxyuser", Password: "proxypass"}
	host, port := startFakeHTTPConnectProxy(t, upRef.basicAuth())

	up := Upstream{
		ID:                "http-1",
		Name:              "Corp HTTP",
		Type:              "HTTP",
		Host:              host,
		Port:              port,
		Username:          "proxyuser",
		Password:          "proxypass",
		SendDomainToProxy: true,
	}

	ctx := context.Background()
	if _, err := up.DialUDP(ctx); !errors.Is(err, ErrUDPNotSupportedOnHTTP) {
		t.Fatalf("expected ErrUDPNotSupportedOnHTTP for HTTP proxy, got %v", err)
	}

	report := CheckProxyEx(ctx, up, "127.0.0.1", uint16(targetPort), nil)
	if !report.Success || len(report.Steps) != 5 || report.Steps[4].Status != StepSkip {
		t.Fatalf("unexpected HTTP CheckProxyEx report: %+v", report)
	}
}
