// SPDX-License-Identifier: MIT

package driver

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/D0bby999/anyproxy/internal/dnsmap"
	"github.com/D0bby999/anyproxy/internal/flows"
	"github.com/D0bby999/anyproxy/internal/pbprofile"
)

func TestDarwinDriverIPCAndFallbackEvaluation(t *testing.T) {
	dns := dnsmap.New(0)
	dns.Record("api.github.com", []string{"140.82.121.6"}, dnsmap.NowSec())
	tracker := flows.NewTracker(100)
	logs := flows.NewLogManager()

	var sentActions []string
	fakeSender := func(payload []byte) ([]byte, error) {
		var msg map[string]any
		if err := json.Unmarshal(payload, &msg); err != nil {
			return nil, err
		}
		act, _ := msg["action"].(string)
		sentActions = append(sentActions, act)

		switch act {
		case "getFlows":
			return json.Marshal(map[string]any{
				"flows": []map[string]any{
					{
						"id":          "ne-flow-1",
						"pid":         4242,
						"process":     "curl",
						"dest":        "140.82.121.6",
						"host":        "api.github.com",
						"port":        443,
						"is_udp":      false,
						"is_ipv6":     false,
						"proxy_id":    "p1",
						"proxy_label": "SOCKS5 127.0.0.1:1080",
						"rule_id":     "r1",
						"age_ms":      120,
						"sent":        1024,
						"received":    4096,
						"closed":      false,
					},
				},
			})
		case "getLogs":
			return json.Marshal(map[string]any{
				"logs": []string{
					"[CONN] TCP curl -> api.github.com:443 [SOCKS5 127.0.0.1:1080]",
					"Tunnel started",
				},
			})
		default:
			return json.Marshal(map[string]any{"status": "ok"})
		}
	}

	drv := NewDarwinDriver(fakeSender, dns, tracker, logs)
	if err := drv.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	proxies := []pbprofile.ProxyConfig{
		{
			ID:                "p1",
			Name:              "Local SOCKS",
			Type:              pbprofile.ProxySOCKS5,
			Host:              "127.0.0.1",
			Port:              1080,
			SendDomainToProxy: true,
		},
		{
			ID:   "p2",
			Name: "Corp HTTP",
			Type: pbprofile.ProxyHTTP,
			Host: "127.0.0.1",
			Port: 8080,
		},
	}
	ruleList := []pbprofile.ProxyRule{
		{
			ID:           "r1",
			Name:         "GitHub via SOCKS",
			ProcessNames: "curl;git",
			TargetHosts:  "*.github.com",
			TargetPorts:  "443",
			Protocol:     pbprofile.ProtoTCP,
			Action:       pbprofile.ActionProxy,
			ProxyConfigID: "p1",
			Enabled:      true,
		},
		{
			ID:            "r2",
			Name:          "UDP over HTTP test",
			ProcessNames:  "quic-client",
			TargetHosts:   "*",
			TargetPorts:   "443",
			Protocol:      pbprofile.ProtoUDP,
			Action:        pbprofile.ActionProxy,
			ProxyConfigID: "p2",
			Enabled:       true,
		},
	}

	err := drv.SyncProfile(proxies, ruleList, RuntimeOptions{
		TrafficLoggingEnabled: true,
		DnsViaProxy:           true,
	})
	if err != nil {
		t.Fatalf("SyncProfile: %v", err)
	}

	// Verify setProxyConfigs, setRules, setTrafficLogging were sent
	if len(sentActions) < 3 || sentActions[0] != "setProxyConfigs" || sentActions[1] != "setRules" || sentActions[2] != "setTrafficLogging" {
		t.Fatalf("unexpected IPC actions: %v", sentActions)
	}

	// Verify GetFlows pulls from extension IPC
	gotFlows, err := drv.GetFlows(50)
	if err != nil || len(gotFlows) != 1 || gotFlows[0].ID != "ne-flow-1" || gotFlows[0].Received != 4096 {
		t.Fatalf("unexpected GetFlows result: %+v, err=%v", gotFlows, err)
	}

	// Verify DrainLogs pulls from extension IPC
	connLogs, actLogs, err := drv.DrainLogs()
	if err != nil || len(connLogs) == 0 || len(actLogs) == 0 {
		t.Fatalf("unexpected DrainLogs: conns=%+v acts=%+v err=%v", connLogs, actLogs, err)
	}

	// Verify Fallback().EvaluateAndRecord routes UDP on HTTP proxy to DIRECT with WarnOnce
	dec, _ := drv.Fallback().EvaluateAndRecord(999, "quic-client", "", "1.1.1.1", "", 443, true)
	if dec.Action != pbprofile.ActionDirect {
		t.Fatalf("expected UDP flow matched to HTTP proxy to fall back to DIRECT, got %s", dec.Action)
	}

	// Verify SetEnabled(false) sends setEnabled and pauses engine
	if err := drv.SetEnabled(false); err != nil {
		t.Fatalf("SetEnabled(false): %v", err)
	}
	if st := drv.Status(); st.State != StatePaused || st.Enabled {
		t.Fatalf("expected paused state, got %+v", st)
	}

	// Verify Stop sends clearConfig
	if err := drv.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if sentActions[len(sentActions)-1] != "clearConfig" {
		t.Fatalf("expected Stop() to send clearConfig, got %v", sentActions)
	}
}
