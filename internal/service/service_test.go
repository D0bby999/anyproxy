// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"errors"
	"net"
	"path/filepath"
	"testing"

	"github.com/D0bby999/anyproxy/internal/driver"
	"github.com/D0bby999/anyproxy/internal/pbprofile"
	"github.com/D0bby999/anyproxy/internal/secrets"
	"github.com/D0bby999/anyproxy/internal/store"
)

type failingSecrets struct{}

func (failingSecrets) Get(string, string) ([]byte, error) { return nil, secrets.ErrNotFound }
func (failingSecrets) Set(string, string, []byte) error   { return errors.New("vault locked") }
func (failingSecrets) Delete(string, string) error        { return nil }

func TestSaveProxyDoesNotCreateRecordWhenVaultFails(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	svc := New(st, failingSecrets{}, driver.NewMockDriver("test", nil, nil, nil))
	t.Cleanup(func() { _ = svc.Close() })
	_, err = svc.SaveProxy(pbprofile.ProxyConfig{Name: "test", Type: pbprofile.ProxyHTTP, Host: "example.com", Port: 80, Username: "user", Password: "secret"})
	if err == nil {
		t.Fatal("expected vault error")
	}
	profile, err := svc.ActiveProfile()
	if err != nil || len(profile.Proxies) != 0 {
		t.Fatalf("proxy saved despite vault error: count=%d err=%v", len(profile.Proxies), err)
	}
}

func TestStartEngineReportsInboundPortConflict(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:10800")
	if err != nil {
		t.Skipf("port already in use: %v", err)
	}
	defer ln.Close()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	svc := New(st, nil, driver.NewMockDriver("test", nil, nil, nil))
	t.Cleanup(func() { _ = svc.Close() })
	if status, err := svc.StartEngine(context.Background()); err == nil || status.State != driver.StateError {
		t.Fatalf("expected listener error, status=%+v err=%v", status, err)
	}
}

func TestServiceEndToEnd(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "anyproxyxa.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	vault := secrets.NewVault(filepath.Join(dir, "secrets.enc"), secrets.NewMemory())
	drv := driver.NewMockDriver("test-driver", nil, nil, nil)

	svc := New(st, vault, drv)
	t.Cleanup(func() { _ = svc.Close() })

	if _, err := svc.StartEngine(context.Background()); err != nil {
		t.Fatalf("StartEngine: %v", err)
	}

	// Add a proxy with a password
	prof, err := svc.SaveProxy(pbprofile.ProxyConfig{
		Name:              "Work SOCKS",
		Type:              pbprofile.ProxyType("socks5"),
		Host:              "127.0.0.1",
		Port:              1080,
		Username:          "dobby",
		Password:          "s3cr3t!",
		SendDomainToProxy: true,
	})
	if err != nil || len(prof.Proxies) != 1 {
		t.Fatalf("SaveProxy: %v, proxies=%+v", err, prof.Proxies)
	}
	if prof.Proxies[0].Password != "••••••••" {
		t.Fatalf("expected UI password indicator to be masked, got %q", prof.Proxies[0].Password)
	}
	proxyID := prof.Proxies[0].ID

	// Add two rules and reorder them
	prof, err = svc.SaveRule(pbprofile.ProxyRule{
		Name:         "Block Telemetry",
		ProcessNames: "*",
		TargetHosts:  "*.telemetry.invalid",
		TargetPorts:  "*",
		Protocol:     pbprofile.ProtoBoth,
		Action:       pbprofile.ActionBlock,
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("SaveRule 1: %v", err)
	}
	r1ID := prof.Rules[0].ID

	prof, err = svc.SaveRule(pbprofile.ProxyRule{
		Name:          "Route Curl via SOCKS",
		ProcessNames:  "curl",
		TargetHosts:   "*.github.com",
		TargetPorts:   "443",
		Protocol:      pbprofile.RuleProtocol("tcp"),
		Action:        pbprofile.RuleAction("proxy"),
		ProxyConfigID: proxyID,
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("SaveRule 2: %v", err)
	}
	r2ID := prof.Rules[1].ID

	// Move r2 above r1
	prof, err = svc.MoveRule(r2ID, -1)
	if err != nil || prof.Rules[0].ID != r2ID || prof.Rules[1].ID != r1ID {
		t.Fatalf("MoveRule failed: %+v, err=%v", prof.Rules, err)
	}

	// Simulate flow matching r2
	dec, flowID := svc.SimulateFlow(1234, "curl", "140.82.121.6", "api.github.com", 443, false)
	if !dec.Matched || dec.Action != pbprofile.ActionProxy || dec.ProxyConfigID != proxyID || flowID == "" {
		t.Fatalf("unexpected SimulateFlow decision: %+v, flowID=%s", dec, flowID)
	}

	// Export profile with passwords and import into a new profile
	exported, err := svc.ExportProfileJSON("", true)
	if err != nil {
		t.Fatalf("ExportProfileJSON: %v", err)
	}
	dup, err := svc.DuplicateProfile("Default", "Work Copy")
	if err != nil || dup.Name != "Work Copy" || len(dup.Proxies) != 1 || len(dup.Rules) != 2 {
		t.Fatalf("DuplicateProfile failed: %+v, err=%v", dup, err)
	}
	if dup.Proxies[0].ID == proxyID || dup.DefaultProxyID != dup.Proxies[0].ID || dup.Rules[0].ProxyConfigID != dup.Proxies[0].ID {
		t.Fatalf("duplicated proxy references are invalid: %+v", dup)
	}
	if _, err := svc.DuplicateProfile("Default", "Work Copy"); err == nil {
		t.Fatal("duplicate profile name should be rejected")
	}
	unchanged, err := st.GetProfileByName("Work Copy")
	if err != nil || unchanged.Proxies[0].ID != dup.Proxies[0].ID {
		t.Fatalf("existing profile was changed: %+v, err=%v", unchanged, err)
	}
	_ = exported
}
