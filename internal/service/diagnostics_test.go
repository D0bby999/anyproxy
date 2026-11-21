// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"net"
	"path/filepath"
	"testing"

	"github.com/D0bby999/anyproxy/internal/driver"
	"github.com/D0bby999/anyproxy/internal/flows"
	"github.com/D0bby999/anyproxy/internal/pbprofile"
	"github.com/D0bby999/anyproxy/internal/secrets"
	"github.com/D0bby999/anyproxy/internal/store"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "diag.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	vault := secrets.NewVault(filepath.Join(dir, "secrets.enc"), secrets.NewMemory())
	drv := driver.NewMockDriver("test-driver", nil, nil, nil)
	svc := NewWithAddr(st, vault, drv, "127.0.0.1:0")
	t.Cleanup(func() { _ = svc.Close() })
	return svc
}

func TestSimulateRuleCoversMatchExcludeDefaultAndFailClosedUDP(t *testing.T) {
	svc := newTestService(t)

	prof, err := svc.SaveProxy(pbprofile.ProxyConfig{
		Name: "HTTP Corp",
		Type: pbprofile.ProxyHTTP,
		Host: "127.0.0.1",
		Port: 8080,
	})
	if err != nil {
		t.Fatal(err)
	}
	httpID := prof.Proxies[0].ID

	prof, err = svc.SaveProxy(pbprofile.ProxyConfig{
		Name: "SOCKS Fast",
		Type: pbprofile.ProxySOCKS5,
		Host: "127.0.0.1",
		Port: 1080,
	})
	if err != nil {
		t.Fatal(err)
	}
	socksID := prof.Proxies[1].ID

	if _, err := svc.SetDefaultProxy(httpID); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.SaveRule(pbprofile.ProxyRule{
		Name:          "Chrome via SOCKS",
		ProcessNames:  "Google Chrome*",
		TargetHosts:   "*.example.com",
		TargetPorts:   "443",
		Protocol:      pbprofile.ProtoTCP,
		Action:        pbprofile.ActionProxy,
		ProxyConfigID: socksID,
		Enabled:       true,
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.SaveRule(pbprofile.ProxyRule{
		Name:           "All Except Telegram",
		ProcessNames:   "Telegram",
		ExcludeProcess: true,
		TargetHosts:    "chat.internal",
		TargetPorts:    "*",
		Protocol:       pbprofile.ProtoBoth,
		Action:         pbprofile.ActionProxy,
		ProxyConfigID:  socksID,
		Enabled:        true,
	}); err != nil {
		t.Fatal(err)
	}

	// 1. Matches rule #1
	res, err := svc.SimulateRule("Google Chrome", "api.example.com", 443, "tcp")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Matched || res.RuleIndex != 1 || res.Action != "proxy" || res.ProxyID != socksID {
		t.Fatalf("unexpected rule #1 match: %+v", res)
	}

	// 2. Excluded app goes Direct via rule #2
	res, err = svc.SimulateRule("Telegram", "chat.internal", 443, "tcp")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Matched || res.RuleIndex != 2 || res.Action != "direct" {
		t.Fatalf("expected exclude -> direct, got %+v", res)
	}

	// 3. Unmatched TCP falls through to Default Route (HTTP Corp)
	res, err = svc.SimulateRule("curl", "ipify.org", 443, "tcp")
	if err != nil {
		t.Fatal(err)
	}
	if res.Matched || !res.ViaDefault || res.Action != "proxy" || res.ProxyID != httpID {
		t.Fatalf("expected Default Route HTTP proxy, got %+v", res)
	}

	// 4. Unmatched UDP over HTTP default proxy fails closed (BLOCK)
	res, err = svc.SimulateRule("curl", "ipify.org", 443, "udp")
	if err != nil {
		t.Fatal(err)
	}
	if !res.ViaDefault || res.Action != "block" || !res.FailClosed {
		t.Fatalf("expected fail-closed block for UDP over HTTP default proxy, got %+v", res)
	}
}

func TestProxyHealthDetectsUnreachableAndRecovery(t *testing.T) {
	svc := newTestService(t)

	// Open a live TCP listener for healthy proxy, and pick a closed port for dead proxy.
	liveLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer liveLn.Close()
	livePort := liveLn.Addr().(*net.TCPAddr).Port

	deadLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	deadPort := deadLn.Addr().(*net.TCPAddr).Port
	_ = deadLn.Close()

	prof, err := svc.SaveProxy(pbprofile.ProxyConfig{
		Name: "Dead Proxy",
		Type: pbprofile.ProxySOCKS5,
		Host: "127.0.0.1",
		Port: deadPort,
	})
	if err != nil {
		t.Fatal(err)
	}
	deadID := prof.Proxies[0].ID

	prof, err = svc.SaveProxy(pbprofile.ProxyConfig{
		Name: "Backup Live",
		Type: pbprofile.ProxySOCKS5,
		Host: "127.0.0.1",
		Port: livePort,
	})
	if err != nil {
		t.Fatal(err)
	}
	liveID := prof.Proxies[1].ID

	if _, err := svc.SetDefaultProxy(deadID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.StartEngine(context.Background()); err != nil {
		t.Fatal(err)
	}

	hs := svc.CheckActiveProxyHealth(context.Background())
	if !hs.Checked || hs.Healthy || !hs.FailClosed || hs.ProxyID != deadID {
		t.Fatalf("expected unhealthy fail-closed status for dead proxy, got %+v", hs)
	}

	// Switch to healthy backup proxy
	if _, err := svc.SetDefaultProxy(liveID); err != nil {
		t.Fatal(err)
	}
	hs = svc.CheckActiveProxyHealth(context.Background())
	if !hs.Checked || !hs.Healthy || hs.FailClosed || hs.ProxyID != liveID {
		t.Fatalf("expected healthy status after switching to backup proxy, got %+v", hs)
	}
}

func TestCheckLeakReportsUnprotectedWhenProxyUnreachable(t *testing.T) {
	svc := newTestService(t)

	deadLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	deadPort := deadLn.Addr().(*net.TCPAddr).Port
	_ = deadLn.Close()

	prof, err := svc.SaveProxy(pbprofile.ProxyConfig{
		Name:              "Dead HTTP",
		Type:              pbprofile.ProxyHTTP,
		Host:              "127.0.0.1",
		Port:              deadPort,
		SendDomainToProxy: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetDefaultProxy(prof.Proxies[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.StartEngine(context.Background()); err != nil {
		t.Fatal(err)
	}

	rep, err := svc.CheckLeak(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rep.Overall != "unprotected" {
		t.Fatalf("expected overall=unprotected when proxy unreachable, got %+v", rep)
	}
	var sawUDP, sawEngine, sawProxy bool
	for _, it := range rep.Items {
		switch it.ID {
		case "engine":
			sawEngine = it.Status == "ok"
		case "proxy_ip":
			sawProxy = it.Status == "fail"
		case "udp":
			sawUDP = it.Status == "ok"
		}
	}
	if !sawEngine || !sawProxy || !sawUDP {
		t.Fatalf("unexpected leak check items: %+v", rep.Items)
	}
}

func TestTrimMemoryClearsClosedFlowsAndPrunesLogs(t *testing.T) {
	svc := newTestService(t)
	id := svc.Tracker().StartFlow(flows.Flow{ProcessName: "curl", DestIP: "1.1.1.1", Port: 443})
	svc.Tracker().CloseFlow(id)
	if got := len(svc.Tracker().GetFlows(10)); got != 1 {
		t.Fatalf("expected 1 closed flow before trim, got %d", got)
	}
	svc.TrimMemory()
	if got := len(svc.Tracker().GetFlows(10)); got != 0 {
		t.Fatalf("expected 0 flows after TrimMemory, got %d", got)
	}
}
