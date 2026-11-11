// SPDX-License-Identifier: AGPL-3.0-or-later

package driver

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/D0bby999/anyproxy/internal/dnsmap"
	"github.com/D0bby999/anyproxy/internal/flows"
	"github.com/D0bby999/anyproxy/internal/pbprofile"
)

const (
	DefaultSysProxyHost = "127.0.0.1"
	DefaultSysProxyPort = 10800
)

// CommandRunner executes an external command and returns its combined output.
type CommandRunner func(name string, args ...string) ([]byte, error)

func defaultCommandRunner(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

// SysProxyManager configures and restores OS-level system proxy settings (macOS networksetup / Windows Registry).
type SysProxyManager struct {
	mu              sync.Mutex
	goos            string
	host            string
	port            int
	run             CommandRunner
	touchedServices []string
}

// NewSysProxyManager creates a system proxy controller for the current OS targeting host:port.
func NewSysProxyManager(host string, port int) *SysProxyManager {
	if host == "" {
		host = DefaultSysProxyHost
	}
	if port <= 0 {
		port = DefaultSysProxyPort
	}
	return &SysProxyManager{
		goos: runtime.GOOS,
		host: host,
		port: port,
		run:  defaultCommandRunner,
	}
}

// Enable points the OS system proxy (SOCKS5, HTTP, HTTPS) to the local inbound listener.
func (m *SysProxyManager) Enable() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch m.goos {
	case "darwin":
		return m.enableDarwin()
	case "windows":
		return m.enableWindows()
	default:
		return fmt.Errorf("system proxy is not supported on %s", m.goos)
	}
}

// Disable turns off the OS system proxy settings previously enabled by AnyProxy.
func (m *SysProxyManager) Disable() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch m.goos {
	case "darwin":
		return m.disableDarwin()
	case "windows":
		return m.disableWindows()
	default:
		return nil
	}
}

func (m *SysProxyManager) enableDarwin() error {
	services, err := m.listDarwinServices()
	if err != nil {
		return err
	}
	portStr := strconv.Itoa(m.port)
	var applied []string
	var lastErr error

	for _, svc := range services {
		cmds := [][]string{
			{"-setsocksfirewallproxy", svc, m.host, portStr},
			{"-setsocksfirewallproxystate", svc, "on"},
			{"-setwebproxy", svc, m.host, portStr},
			{"-setwebproxystate", svc, "on"},
			{"-setsecurewebproxy", svc, m.host, portStr},
			{"-setsecurewebproxystate", svc, "on"},
			{"-setproxybypassdomains", svc, "127.0.0.1", "localhost", "*.local"},
		}
		ok := true
		for _, args := range cmds {
			if out, err := m.run("networksetup", args...); err != nil {
				lastErr = fmt.Errorf("networksetup %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
				ok = false
				break
			}
		}
		if ok {
			applied = append(applied, svc)
		}
	}

	if len(applied) == 0 {
		if lastErr != nil {
			return lastErr
		}
		return fmt.Errorf("no active macOS network services found")
	}
	m.touchedServices = applied
	return nil
}

func (m *SysProxyManager) disableDarwin() error {
	services := m.touchedServices
	if len(services) == 0 {
		services, _ = m.listDarwinServices()
	}
	m.touchedServices = nil

	var firstErr error
	for _, svc := range services {
		for _, flag := range []string{"-setsocksfirewallproxystate", "-setwebproxystate", "-setsecurewebproxystate"} {
			if out, err := m.run("networksetup", flag, svc, "off"); err != nil && firstErr == nil {
				firstErr = fmt.Errorf("networksetup %s %s off: %w (%s)", flag, svc, err, strings.TrimSpace(string(out)))
			}
		}
	}
	return firstErr
}

func (m *SysProxyManager) listDarwinServices() ([]string, error) {
	out, err := m.run("networksetup", "-listallnetworkservices")
	if err != nil {
		return nil, fmt.Errorf("listallnetworkservices: %w", err)
	}
	var services []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "An asterisk") || strings.HasPrefix(line, "*") {
			continue
		}
		services = append(services, line)
	}
	return services, nil
}

const winInternetSettingsKey = `HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings`

func (m *SysProxyManager) enableWindows() error {
	addr := fmt.Sprintf("%s:%d", m.host, m.port)
	cmds := [][]string{
		{"add", winInternetSettingsKey, "/v", "ProxyServer", "/t", "REG_SZ", "/d", addr, "/f"},
		{"add", winInternetSettingsKey, "/v", "ProxyOverride", "/t", "REG_SZ", "/d", "localhost;127.*;10.*;192.168.*;<local>", "/f"},
		{"add", winInternetSettingsKey, "/v", "ProxyEnable", "/t", "REG_DWORD", "/d", "1", "/f"},
	}
	for _, args := range cmds {
		if out, err := m.run("reg", args...); err != nil {
			return fmt.Errorf("reg %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
		}
	}
	return nil
}

func (m *SysProxyManager) disableWindows() error {
	args := []string{"add", winInternetSettingsKey, "/v", "ProxyEnable", "/t", "REG_DWORD", "/d", "0", "/f"}
	if out, err := m.run("reg", args...); err != nil {
		return fmt.Errorf("reg disable proxy: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// SysProxyDriver wraps MockDriver with OS System Proxy activation for platforms without a kernel driver attached.
type SysProxyDriver struct {
	*MockDriver
	sysProxy *SysProxyManager
}

// NewSysProxyDriver creates a Driver that toggles the OS System Proxy when started, paused, or stopped.
func NewSysProxyDriver(backend string, sysProxy *SysProxyManager, dns *dnsmap.Store, tracker *flows.Tracker, logs *flows.LogManager) *SysProxyDriver {
	fb := NewMockDriver(backend, dns, tracker, logs)
	fb.SetMessage(fmt.Sprintf("System proxy ready (%s:%d)", sysProxy.host, sysProxy.port))
	return &SysProxyDriver{
		MockDriver: fb,
		sysProxy:   sysProxy,
	}
}

func (d *SysProxyDriver) Start(ctx context.Context) error {
	if err := d.sysProxy.Enable(); err != nil {
		d.SetMessage(err.Error())
		return err
	}
	d.SetMessage(fmt.Sprintf("System proxy active (%s:%d)", d.sysProxy.host, d.sysProxy.port))
	return d.MockDriver.Start(ctx)
}

func (d *SysProxyDriver) Stop() error {
	_ = d.sysProxy.Disable()
	d.SetMessage(fmt.Sprintf("System proxy stopped (%s:%d)", d.sysProxy.host, d.sysProxy.port))
	return d.MockDriver.Stop()
}

func (d *SysProxyDriver) SetEnabled(enabled bool) error {
	if enabled {
		if err := d.sysProxy.Enable(); err != nil {
			d.SetMessage(err.Error())
			return err
		}
		d.SetMessage(fmt.Sprintf("System proxy active (%s:%d)", d.sysProxy.host, d.sysProxy.port))
	} else {
		_ = d.sysProxy.Disable()
		d.SetMessage("System proxy paused")
	}
	return d.MockDriver.SetEnabled(enabled)
}

func (d *SysProxyDriver) SyncProfile(proxies []pbprofile.ProxyConfig, ruleList []pbprofile.ProxyRule, opts RuntimeOptions) error {
	return d.MockDriver.SyncProfile(proxies, ruleList, opts)
}
