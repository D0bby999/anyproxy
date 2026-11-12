// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"net"
	"path/filepath"
	goruntime "runtime"
	"testing"

	"github.com/D0bby999/anyproxy/internal/driver"
	"github.com/D0bby999/anyproxy/internal/pbprofile"
	"github.com/D0bby999/anyproxy/internal/service"
	"github.com/D0bby999/anyproxy/internal/store"
)

type countingStopDriver struct {
	driver.Driver
	stops int
}

func (c *countingStopDriver) Stop() error {
	c.stops++
	return c.Driver.Stop()
}

func TestFormatTrayState(t *testing.T) {
	cases := []struct {
		state      driver.EngineState
		wantActive bool
		wantStatus string
		wantToggle string
	}{
		{driver.StateRunning, true, "Status: Running", "Disable AnyProxy"},
		{driver.StateStarting, true, "Status: Starting…", "Disable AnyProxy"},
		{driver.StatePaused, false, "Status: Paused", "Enable AnyProxy"},
		{driver.StateError, false, "Status: Error", "Enable AnyProxy"},
		{driver.StateStopped, false, "Status: Stopped", "Enable AnyProxy"},
		{"", false, "Status: Stopped", "Enable AnyProxy"},
	}

	for _, tc := range cases {
		got := formatTrayState(driver.EngineStatus{State: tc.state})
		if got.Active != tc.wantActive || got.StatusLabel != tc.wantStatus || got.ToggleLabel != tc.wantToggle || got.Tooltip == "" {
			t.Fatalf("formatTrayState(%q) = %+v, want active=%v status=%q toggle=%q",
				tc.state, got, tc.wantActive, tc.wantStatus, tc.wantToggle)
		}
	}
}

func TestBuildTraySnapshot(t *testing.T) {
	prof := pbprofile.DefaultProfile("Work")
	prof.DefaultProxyID = "px-2"
	prof.Proxies = []pbprofile.ProxyConfig{
		{ID: "px-1", Name: "US SOCKS", Type: pbprofile.ProxySOCKS5, Host: "1.2.3.4", Port: 1080},
		{ID: "px-2", Name: "JP HTTPS", Type: pbprofile.ProxyHTTPS, Host: "jp.example.com", Port: 443},
	}

	snap := buildTraySnapshot(
		driver.EngineStatus{State: driver.StateRunning},
		"127.0.0.1:10801",
		&prof,
		[]string{"Default", "Work", "Default"},
		true,
	)

	if !snap.Active || snap.ListenAddr != "127.0.0.1:10801" || !snap.Compact {
		t.Fatalf("unexpected basic snapshot fields: %+v", snap)
	}
	if snap.Tooltip != "AnyProxy — Running (127.0.0.1:10801)" {
		t.Fatalf("tooltip = %q", snap.Tooltip)
	}
	if snap.ActiveProxyText != "JP HTTPS" || snap.DefaultProxyID != "px-2" {
		t.Fatalf("active proxy = %q (%q), want JP HTTPS (px-2)", snap.ActiveProxyText, snap.DefaultProxyID)
	}
	if len(snap.Proxies) != 2 || snap.Proxies[1].Label != "JP HTTPS (HTTPS jp.example.com:443)" {
		t.Fatalf("unexpected proxies list: %+v", snap.Proxies)
	}
	if snap.ActiveProfile != "Work" || len(snap.Profiles) != 2 {
		t.Fatalf("unexpected profiles: active=%q list=%v", snap.ActiveProfile, snap.Profiles)
	}

	// Unknown DefaultProxyID falls back cleanly to Direct
	prof.DefaultProxyID = "missing"
	snap2 := buildTraySnapshot(driver.EngineStatus{State: driver.StateStopped}, "", &prof, nil, false)
	if snap2.ListenAddr != defaultTrayListenAddr || snap2.ActiveProxyText != "Direct" || snap2.DefaultProxyID != "" {
		t.Fatalf("stale default proxy fallback failed: %+v", snap2)
	}
}

func TestShutdownStopsServiceOnce(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	drv := &countingStopDriver{Driver: driver.NewMockDriver("test", nil, nil, nil)}
	svc := service.New(st, nil, drv)
	if _, err := svc.StartEngine(context.Background()); err != nil {
		t.Fatalf("StartEngine: %v", err)
	}

	app := NewApp()
	app.store = st
	app.svc = svc

	if v, _ := st.GetSetting("sysproxy_on"); v != "1" {
		t.Fatalf("expected sysproxy_on=1 while running, got %q", v)
	}

	if goruntime.GOOS == "darwin" {
		// On macOS, window close uses HideWindowOnClose; beforeClose is reserved for Quit and
		// must return false without tearing down the service before OnShutdown runs.
		if app.beforeClose(context.Background()) {
			t.Fatal("beforeClose must not prevent quit")
		}
		if drv.stops != 0 || drv.Status().State != driver.StateRunning {
			t.Fatalf("beforeClose on darwin should leave service running until shutdown (stops=%d state=%s)", drv.stops, drv.Status().State)
		}
	}

	app.shutdown(context.Background())
	app.shutdown(context.Background())

	if drv.stops != 1 {
		t.Fatalf("expected Stop() once across repeated shutdowns, got %d", drv.stops)
	}
	if drv.Status().State != driver.StateStopped {
		t.Fatalf("expected stopped after shutdown, got %s", drv.Status().State)
	}
	if v, _ := st.GetSetting("sysproxy_on"); v != "0" {
		t.Fatalf("expected sysproxy_on=0 after shutdown, got %q", v)
	}
	if addr := svc.InboundAddr(); addr != "" {
		if conn, err := net.Dial("tcp", addr); err == nil {
			_ = conn.Close()
			t.Fatalf("expected inbound listener %s to be closed after shutdown", addr)
		}
	}
}
