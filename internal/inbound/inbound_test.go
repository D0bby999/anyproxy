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
	"time"

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

func TestInboundHTTPConnectViaUpstream(t *testing.T) {
	connected := make(chan string, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodConnect {
			return
		}
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			return
		}
		defer conn.Close()
		connected <- r.Host
		_, _ = io.WriteString(conn, "HTTP/1.1 200 Connection Established\r\n\r\n")
		_, _ = io.Copy(conn, conn)
	}))
	defer upstream.Close()
	host, portStr, err := net.SplitHostPort(upstream.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	var port int
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		t.Fatal(err)
	}
	srv := NewServer(Config{ListenAddr: "127.0.0.1:0"})
	srv.SyncProfile([]pbprofile.ProxyConfig{{ID: "up", Type: pbprofile.ProxyHTTP, Host: host, Port: port, SendDomainToProxy: true}}, []pbprofile.ProxyRule{{
		ID: "proxy", Enabled: true, TargetHosts: "example.invalid", TargetPorts: "*", Action: pbprofile.ActionProxy, ProxyConfigID: "up",
	}}, "")
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	defer srv.Stop()

	client, err := net.Dial("tcp", srv.ListenAddr())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	_ = client.SetDeadline(time.Now().Add(5 * time.Second))
	_, _ = io.WriteString(client, "CONNECT example.invalid:443 HTTP/1.1\r\nHost: example.invalid:443\r\n\r\n")
	reader := bufio.NewReader(client)
	resp, err := http.ReadResponse(reader, &http.Request{Method: http.MethodConnect})
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("CONNECT response: %v, %v", resp, err)
	}
	if got := <-connected; got != "example.invalid:443" {
		t.Fatalf("upstream target = %q", got)
	}
	_, _ = io.WriteString(client, "hello")
	buf := make([]byte, 5)
	if _, err := io.ReadFull(reader, buf); err != nil || string(buf) != "hello" {
		t.Fatalf("tunnel echo = %q, %v", buf, err)
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

func TestPerProcessRoutingAndDefaultProxyFallback(t *testing.T) {
	srv := NewServer(Config{})
	proxies := []pbprofile.ProxyConfig{
		{ID: "p-chrome", Name: "Chrome Proxy", Type: pbprofile.ProxySOCKS5, Host: "127.0.0.1", Port: 1080},
		{ID: "p-default", Name: "Default Proxy", Type: pbprofile.ProxySOCKS5, Host: "127.0.0.1", Port: 1081},
	}
	ruleList := []pbprofile.ProxyRule{
		{
			ID:            "r-chrome",
			Enabled:       true,
			ProcessNames:  "Google Chrome;chrome.exe",
			TargetHosts:   "*",
			TargetPorts:   "*",
			Action:        pbprofile.ActionProxy,
			ProxyConfigID: "p-chrome",
		},
		{
			ID:           "r-curl-block",
			Enabled:      true,
			ProcessNames: "curl",
			TargetHosts:  "*",
			TargetPorts:  "*",
			Action:       pbprofile.ActionBlock,
		},
	}
	srv.SyncProfile(proxies, ruleList, "p-default")

	// 1. Chrome matches r-chrome -> p-chrome
	dec, px := srv.evaluateTargetForProcess("example.com", 443, ProcessInfo{
		PID:  752,
		Name: "Google Chrome",
		Path: "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
	})
	if dec.Action != pbprofile.ActionProxy || px == nil || px.ID != "p-chrome" {
		t.Fatalf("expected Chrome to route via p-chrome, got dec=%+v px=%+v", dec, px)
	}

	// 2. curl matches r-curl-block -> BLOCK
	dec, _ = srv.evaluateTargetForProcess("example.com", 443, ProcessInfo{PID: 800, Name: "curl"})
	if dec.Action != pbprofile.ActionBlock {
		t.Fatalf("expected curl to be blocked, got %+v", dec)
	}

	// 3. Unmatched app (Telegram) falls back to active Default Proxy (p-default)
	dec, px = srv.evaluateTargetForProcess("example.com", 443, ProcessInfo{PID: 900, Name: "Telegram"})
	if dec.Action != pbprofile.ActionProxy || px == nil || px.ID != "p-default" || dec.RuleID != "default" {
		t.Fatalf("expected unmatched app to fall back to default proxy, got dec=%+v px=%+v", dec, px)
	}

	// 4. AnyProxy itself bypasses proxy (DIRECT) to prevent self-loop
	dec, _ = srv.evaluateTargetForProcess("example.com", 443, ProcessInfo{PID: 999, Name: "AnyProxy"})
	if dec.Action != pbprofile.ActionDirect {
		t.Fatalf("expected AnyProxy self process to bypass directly, got %+v", dec)
	}
}

func TestNetstatParsersAndFriendlyProcessName(t *testing.T) {
	darwinSample := `Active Internet connections (including servers)
Proto Recv-Q Send-Q  Local Address          Foreign Address        (state)     rxbytes txbytes rhiwat shiwat process:pid state
tcp4       0      0  127.0.0.1.58937        127.0.0.1.10800        ESTABLISHED 40702   7799    388352 146988 Google Chrome He:752 00102
`
	pid, name := parseDarwinNetstatPort(darwinSample, 58937)
	if pid != 752 || name != "Google Chrome He" {
		t.Fatalf("unexpected darwin parse: pid=%d name=%q", pid, name)
	}

	winSample := `  Proto  Local Address          Foreign Address        State           PID
  TCP    127.0.0.1:58937        127.0.0.1:10800        ESTABLISHED     4321
`
	if got := parseWindowsNetstatPort(winSample, 58937); got != 4321 {
		t.Fatalf("unexpected windows parse: pid=%d", got)
	}

	if got := friendlyProcessName("/Applications/Google Chrome.app/Contents/Frameworks/Google Chrome Helper", "Google Chrome He"); got != "Google Chrome" {
		t.Fatalf("unexpected friendlyProcessName: %q", got)
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
