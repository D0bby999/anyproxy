// SPDX-License-Identifier: MIT

package inbound

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/D0bby999/anyproxy/internal/flows"
	"github.com/D0bby999/anyproxy/internal/pbprofile"
)

func TestMissingProxyBlocksInsteadOfConnectingDirectly(t *testing.T) {
	srv := NewServer(Config{})
	srv.SyncProfile(nil, []pbprofile.ProxyRule{{
		ID: "missing-proxy", Enabled: true, TargetHosts: "*", TargetPorts: "*",
		Action: pbprofile.ActionProxy, ProxyConfigID: "missing",
	}}, "")
	decision, proxy := srv.evaluateTarget("example.com", 443)
	if decision.Action != pbprofile.ActionBlock || proxy != nil {
		t.Fatalf("missing proxy must block, got decision=%+v proxy=%+v", decision, proxy)
	}
}

func TestInboundSOCKS5Direct(t *testing.T) {
	// 1. Start a mock target echo server
	targetLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("target listen: %v", err)
	}
	defer targetLn.Close()

	go func() {
		for {
			c, err := targetLn.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				_, _ = io.Copy(conn, conn)
			}(c)
		}
	}()

	targetHost, targetPortStr, _ := net.SplitHostPort(targetLn.Addr().String())
	var targetPort uint16
	fmt.Sscanf(targetPortStr, "%d", &targetPort)

	// 2. Start Inbound Server on a random local port
	tracker := flows.NewTracker(100)
	logs := flows.NewLogManager()
	srv := NewServer(Config{
		ListenAddr: "127.0.0.1:0",
		Tracker:    tracker,
		Logs:       logs,
	})

	// Rule: DIRECT for target
	srv.SyncProfile(nil, []pbprofile.ProxyRule{
		{
			ID:          "rule-direct",
			Name:        "Direct Target",
			Enabled:     true,
			TargetHosts: targetHost,
			TargetPorts: targetPortStr,
			Action:      pbprofile.ActionDirect,
		},
	}, "")

	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start: %v", err)
	}
	defer srv.Stop()

	// 3. Connect client via SOCKS5
	client, err := net.Dial("tcp", srv.ListenAddr())
	if err != nil {
		t.Fatalf("dial inbound: %v", err)
	}
	defer client.Close()

	// SOCKS5 Handshake: \x05\x01\x00 (version 5, 1 method: no auth)
	if _, err := client.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		t.Fatalf("write handshake: %v", err)
	}
	authResp := make([]byte, 2)
	if _, err := io.ReadFull(client, authResp); err != nil {
		t.Fatalf("read auth resp: %v", err)
	}
	if authResp[0] != 0x05 || authResp[1] != 0x00 {
		t.Fatalf("bad auth resp: %v", authResp)
	}

	// SOCKS5 Connect: \x05\x01\x00\x03<len><domain><port>
	domain := "127.0.0.1"
	req := []byte{0x05, 0x01, 0x00, 0x03, byte(len(domain))}
	req = append(req, []byte(domain)...)
	var portBytes [2]byte
	binary.BigEndian.PutUint16(portBytes[:], targetPort)
	req = append(req, portBytes[:]...)

	if _, err := client.Write(req); err != nil {
		t.Fatalf("write connect req: %v", err)
	}

	connResp := make([]byte, 10)
	if _, err := io.ReadFull(client, connResp); err != nil {
		t.Fatalf("read conn resp: %v", err)
	}
	if connResp[1] != 0x00 {
		t.Fatalf("socks5 connect failed with code: %d", connResp[1])
	}

	// Echo test
	testPayload := []byte("hello anyproxyxa inbound socks5")
	if _, err := client.Write(testPayload); err != nil {
		t.Fatalf("write payload: %v", err)
	}

	echoBuf := make([]byte, len(testPayload))
	if _, err := io.ReadFull(client, echoBuf); err != nil {
		t.Fatalf("read echo: %v", err)
	}
	if string(echoBuf) != string(testPayload) {
		t.Fatalf("echo mismatch: got %q, want %q", echoBuf, testPayload)
	}
}

func TestInboundHTTPConnectDirect(t *testing.T) {
	// 1. Mock HTTPS echo server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-AnyProxyXa", "Inbound-OK")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("HTTP Connect Success"))
	}))
	defer ts.Close()

	srv := NewServer(Config{
		ListenAddr: "127.0.0.1:0",
	})
	srv.SyncProfile(nil, nil, "") // default direct

	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start: %v", err)
	}
	defer srv.Stop()

	// 2. Client issues HTTP CONNECT
	client, err := net.Dial("tcp", srv.ListenAddr())
	if err != nil {
		t.Fatalf("dial inbound: %v", err)
	}
	defer client.Close()

	connectReq := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", ts.Listener.Addr().String(), ts.Listener.Addr().String())
	if _, err := client.Write([]byte(connectReq)); err != nil {
		t.Fatalf("write connect: %v", err)
	}

	r := bufio.NewReader(client)
	respLine, err := r.ReadString('\n')
	if err != nil {
		t.Fatalf("read resp: %v", err)
	}
	if !containsString(respLine, "200") {
		t.Fatalf("connect rejected: %s", respLine)
	}
	// read until empty line
	for {
		line, _ := r.ReadString('\n')
		if line == "\r\n" || line == "\n" {
			break
		}
	}

	// Send GET request through established tunnel
	getReq := "GET / HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n"
	if _, err := client.Write([]byte(getReq)); err != nil {
		t.Fatalf("write GET: %v", err)
	}

	body, _ := io.ReadAll(r)
	if !containsString(string(body), "HTTP Connect Success") {
		t.Fatalf("body mismatch: %s", body)
	}
}

func TestInboundRuleBlock(t *testing.T) {
	srv := NewServer(Config{
		ListenAddr: "127.0.0.1:0",
	})
	srv.SyncProfile(nil, []pbprofile.ProxyRule{
		{
			ID:          "block-all",
			Name:        "Block All",
			Enabled:     true,
			TargetHosts: "*",
			TargetPorts: "*",
			Action:      pbprofile.ActionBlock,
		},
	}, "")

	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start: %v", err)
	}
	defer srv.Stop()

	client, err := net.Dial("tcp", srv.ListenAddr())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()

	// Handshake
	_, _ = client.Write([]byte{0x05, 0x01, 0x00})
	authResp := make([]byte, 2)
	_, _ = io.ReadFull(client, authResp)

	// Connect to 1.1.1.1:80
	req := []byte{0x05, 0x01, 0x00, 0x01, 1, 1, 1, 1, 0, 80}
	_, _ = client.Write(req)

	resp := make([]byte, 10)
	_, _ = io.ReadFull(client, resp)
	if resp[1] != 0x02 { // 0x02 = connection not allowed by ruleset
		t.Fatalf("expected code 0x02 for blocked rule, got %d", resp[1])
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || (len(s) > 0 && containsSub(s, substr)))
}

func containsSub(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
