// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build darwin

package driver

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/D0bby999/anyproxy/internal/dnsmap"
	"github.com/D0bby999/anyproxy/internal/flows"
	"github.com/D0bby999/anyproxy/internal/pbprofile"
)

func TestNewPlatformDriverFallsBackWhenExtensionMissing(t *testing.T) {
	if neAvailable() {
		t.Fatal("expected neAvailable() == false in unbundled test binary")
	}
	drv := NewPlatformDriver(dnsmap.New(0), flows.NewTracker(16), flows.NewLogManager())
	if got := drv.Status().Backend; got != "macos-sysproxy" {
		t.Fatalf("expected fallback backend macos-sysproxy, got %q", got)
	}
}

func TestNEDriverStopWhenStoppedIsImmediate(t *testing.T) {
	drv := NewNEDriver(dnsmap.New(0), flows.NewTracker(16), flows.NewLogManager())
	start := time.Now()
	if err := drv.Stop(); err != nil {
		t.Fatalf("Stop() unexpected error: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Fatalf("Stop() when already stopped took %v, want <200ms", elapsed)
	}
}

// the config used to be pushed once with the error dropped, so a rejected push
// left the engine reporting Running with an empty proxy/rule set
func TestPushConfigRetriesThenSucceeds(t *testing.T) {
	oldDelay := pushRetryDelay
	pushRetryDelay = time.Millisecond
	defer func() { pushRetryDelay = oldDelay }()

	calls := 0
	drv := NewNEDriver(dnsmap.New(0), flows.NewTracker(16), flows.NewLogManager())
	drv.sendMessage = func(payload []byte) ([]byte, error) {
		calls++
		if calls <= 2 {
			return nil, errors.New("extension is still starting")
		}
		return []byte(`{"status":"ok"}`), nil
	}

	if err := drv.pushConfigWithRetry(nil, nil, RuntimeOptions{}); err != nil {
		t.Fatalf("pushConfigWithRetry() unexpected error: %v", err)
	}
	// two failed attempts, then the full three-message push
	if calls != 5 {
		t.Fatalf("bridge called %d times, want 5", calls)
	}
}

func TestPushConfigReportsFailureAfterAllRetries(t *testing.T) {
	oldDelay := pushRetryDelay
	pushRetryDelay = time.Millisecond
	defer func() { pushRetryDelay = oldDelay }()

	calls := 0
	drv := NewNEDriver(dnsmap.New(0), flows.NewTracker(16), flows.NewLogManager())
	drv.sendMessage = func(payload []byte) ([]byte, error) {
		calls++
		return nil, errors.New("ipc channel closed")
	}

	err := drv.pushConfigWithRetry(nil, nil, RuntimeOptions{})
	if err == nil {
		t.Fatal("pushConfigWithRetry() = nil, want an error")
	}
	// each attempt stops on the first rejected message, so one call per attempt
	if calls != pushConfigAttempts {
		t.Fatalf("bridge called %d times, want %d", calls, pushConfigAttempts)
	}
}

// the extension answers a bad payload with rc == 0 and {"status":"error"}
func TestSendActionTreatsInBandErrorAsFailure(t *testing.T) {
	drv := NewNEDriver(dnsmap.New(0), flows.NewTracker(16), flows.NewLogManager())
	drv.sendMessage = func(payload []byte) ([]byte, error) {
		return []byte(`{"status":"error","message":"Invalid rules payload"}`), nil
	}

	err := drv.pushConfig(nil, nil, RuntimeOptions{})
	if err == nil {
		t.Fatal("pushConfig() = nil, want the in-band error")
	}
	if !strings.Contains(err.Error(), "Invalid rules payload") {
		t.Fatalf("error = %q, want it to carry the extension message", err)
	}
}

func TestPollTunnelHealthReportsDisconnect(t *testing.T) {
	drv := NewNEDriver(dnsmap.New(0), flows.NewTracker(16), flows.NewLogManager())
	drv.mu.Lock()
	drv.state = StateRunning
	drv.enabled = true
	drv.mu.Unlock()
	drv.statusReader = func() (int, string, error) { return neStatusDisconnected, "", nil }

	drv.pollTunnelHealth()

	st := drv.Status()
	if st.State != StateError {
		t.Fatalf("state = %v, want StateError", st.State)
	}
	if st.Enabled {
		t.Fatal("enabled = true, want false after the extension went away")
	}
	if st.DNSProxyActive {
		t.Fatal("DNSProxyActive = true, want false when the tunnel is down")
	}
}

func TestPollTunnelHealthClearsErrorWhenConnected(t *testing.T) {
	drv := NewNEDriver(dnsmap.New(0), flows.NewTracker(16), flows.NewLogManager())
	drv.mu.Lock()
	drv.state = StateError
	drv.enabled = true
	drv.mu.Unlock()
	drv.statusReader = func() (int, string, error) { return neStatusConnected, "", nil }

	drv.pollTunnelHealth()

	st := drv.Status()
	if st.State != StateRunning {
		t.Fatalf("state = %v, want StateRunning", st.State)
	}
	if !st.DNSProxyActive {
		t.Fatal("DNSProxyActive = false, want true while connected")
	}
}

// the Go side must send "PROXY" (not a concrete id and not an empty string) for
// rules that use the default proxy, because that is the token the extension
// resolves through defaultProxyId at connection time
func TestPushConfigSendsPROXYTokenForLateBoundRules(t *testing.T) {
	drv := NewNEDriver(dnsmap.New(0), flows.NewTracker(16), flows.NewLogManager())
	sent := make([]map[string]any, 0, 4)
	drv.sendMessage = func(raw []byte) ([]byte, error) {
		var msg map[string]any
		if err := json.Unmarshal(raw, &msg); err != nil {
			t.Fatalf("bridge got invalid json: %v", err)
		}
		sent = append(sent, msg)
		return []byte(`{"status":"ok"}`), nil
	}

	rules := []pbprofile.ProxyRule{
		{ProcessNames: "*", TargetHosts: "*", TargetPorts: "*", Action: pbprofile.ActionProxy},
		{ProcessNames: "*", TargetHosts: "*", TargetPorts: "*", Action: pbprofile.ActionProxy, ProxyConfigID: "px-1"},
	}
	if err := drv.pushConfig(
		[]pbprofile.ProxyConfig{{ID: "px-1", Type: pbprofile.ProxySOCKS5, Host: "h", Port: 1080}},
		rules,
		RuntimeOptions{DefaultProxyID: "px-1"},
	); err != nil {
		t.Fatalf("pushConfig: %v", err)
	}

	var setRules map[string]any
	for _, msg := range sent {
		if msg["action"] == "setRules" {
			setRules = msg
		}
	}
	if setRules == nil {
		t.Fatal("pushConfig never sent setRules")
	}
	rawRules, _ := setRules["rules"].([]any)
	if len(rawRules) != 2 {
		t.Fatalf("rules payload = %d entries, want 2", len(rawRules))
	}
	first, _ := rawRules[0].(map[string]any)
	second, _ := rawRules[1].(map[string]any)
	if got := first["ruleAction"]; got != "PROXY" {
		t.Fatalf("late-bound rule action = %v, want PROXY", got)
	}
	if got := second["ruleAction"]; got != "px-1" {
		t.Fatalf("pinned rule action = %v, want px-1", got)
	}
}
