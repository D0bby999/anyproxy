// SPDX-License-Identifier: AGPL-3.0-or-later

package driver

import (
	"context"
	"strings"
	"testing"
)

func TestUnavailableDriverDoesNotClaimToRun(t *testing.T) {
	d := newUnavailableDriver("test", "unavailable", nil, nil)
	if err := d.Start(context.Background()); err == nil {
		t.Fatal("start succeeded without a driver")
	}
	if err := d.SetEnabled(true); err == nil {
		t.Fatal("enabling succeeded without a driver")
	}
	if st := d.Status(); st.State != StateStopped || st.Enabled || st.Message == "" {
		t.Fatalf("misleading driver status: %+v", st)
	}
}

func TestSysProxyDriverTogglesOSSystemProxy(t *testing.T) {
	var calls []string
	mgr := &SysProxyManager{
		goos: "darwin",
		host: "127.0.0.1",
		port: 10800,
		run: func(name string, args ...string) ([]byte, error) {
			cmd := name + " " + strings.Join(args, " ")
			calls = append(calls, cmd)
			if len(args) > 0 && args[0] == "-listallnetworkservices" {
				return []byte("An asterisk (*) denotes that a network service is disabled.\n*Thunderbolt Bridge\nWi-Fi\n"), nil
			}
			return nil, nil
		},
	}

	drv := NewSysProxyDriver("macos-sysproxy", mgr, nil, nil, nil)
	if err := drv.Start(context.Background()); err != nil {
		t.Fatalf("Start with sysProxy failed: %v", err)
	}
	if st := drv.Status(); st.State != StateRunning || !st.Enabled || st.Backend != "macos-sysproxy" {
		t.Fatalf("unexpected running status: %+v", st)
	}

	if err := drv.SetEnabled(false); err != nil {
		t.Fatalf("SetEnabled(false) failed: %v", err)
	}
	if st := drv.Status(); st.State != StatePaused || st.Enabled {
		t.Fatalf("unexpected paused status: %+v", st)
	}

	if err := drv.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	joined := strings.Join(calls, "\n")
	if !strings.Contains(joined, "networksetup -setsocksfirewallproxy Wi-Fi 127.0.0.1 10800") {
		t.Fatalf("expected SOCKS proxy setup on Wi-Fi, got:\n%s", joined)
	}
	if !strings.Contains(joined, "networksetup -setsocksfirewallproxystate Wi-Fi off") {
		t.Fatalf("expected SOCKS proxy teardown on Wi-Fi, got:\n%s", joined)
	}
}

func TestWindowsSysProxyCommands(t *testing.T) {
	var calls []string
	mgr := &SysProxyManager{
		goos: "windows",
		host: "127.0.0.1",
		port: 10800,
		run: func(name string, args ...string) ([]byte, error) {
			calls = append(calls, name+" "+strings.Join(args, " "))
			return nil, nil
		},
	}
	drv := NewSysProxyDriver("windows-sysproxy", mgr, nil, nil, nil)
	if err := drv.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := drv.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	joined := strings.Join(calls, "\n")
	if !strings.Contains(joined, "ProxyEnable /t REG_DWORD /d 1") || !strings.Contains(joined, "ProxyEnable /t REG_DWORD /d 0") {
		t.Fatalf("expected Windows registry ProxyEnable toggle, got:\n%s", joined)
	}
}
