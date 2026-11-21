// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	goruntime "runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/D0bby999/anyproxy/internal/driver"
	"github.com/D0bby999/anyproxy/internal/netproxy"
	"github.com/D0bby999/anyproxy/internal/pbprofile"
	"github.com/D0bby999/anyproxy/internal/rules"
)

const (
	healthCheckInterval    = 15 * time.Second
	healthCheckTimeout     = 4 * time.Second
	healthFailureThreshold = 2
)

// RuleSimulationResult explains which rule (or Default Route) handles a hypothetical flow.
type RuleSimulationResult struct {
	Matched    bool   `json:"matched"`
	RuleID     string `json:"ruleId,omitempty"`
	RuleName   string `json:"ruleName,omitempty"`
	RuleIndex  int    `json:"ruleIndex"`
	Action     string `json:"action"`
	ProxyID    string `json:"proxyId,omitempty"`
	ProxyName  string `json:"proxyName,omitempty"`
	ProxyAddr  string `json:"proxyAddr,omitempty"`
	ViaDefault bool   `json:"viaDefault"`
	FailClosed bool   `json:"failClosed"`
	Reason     string `json:"reason"`
}

// SimulateRule evaluates processName/host/port/protocol against the active profile's
// rules using the same matcher and fail-closed semantics as the live runtime.
func (s *Service) SimulateRule(processName, host string, port int, protocol string) (RuleSimulationResult, error) {
	p, err := s.store.ActiveProfile()
	if err != nil {
		return RuleSimulationResult{}, err
	}

	proto := pbprofile.ProtoTCP
	if strings.EqualFold(strings.TrimSpace(protocol), "udp") {
		proto = pbprofile.ProtoUDP
	}
	if port <= 0 || port > 65535 {
		port = 443
	}
	host = strings.TrimSpace(host)
	if host == "" {
		host = "example.com"
	}

	destIP := host
	var domains []string
	if _, parseErr := netip.ParseAddr(host); parseErr != nil {
		domains = []string{host}
	}

	proc := strings.TrimSpace(processName)
	engine := rules.NewEngine(p.Rules)
	dec := engine.Evaluate(rules.FlowInput{
		Process:   rules.NewFlowProcess(proc, proc),
		DestIP:    destIP,
		Domains:   domains,
		DestPort:  uint16(port),
		Protocol:  proto,
		CheckHost: true,
	})

	proxyByID := make(map[string]pbprofile.ProxyConfig, len(p.Proxies))
	for _, px := range p.Proxies {
		proxyByID[px.ID] = px
	}

	res := RuleSimulationResult{
		Matched: dec.Matched,
		Action:  strings.ToLower(string(dec.Action)),
	}

	if dec.Matched {
		res.RuleID = dec.RuleID
		res.RuleName = dec.RuleName
		var matchedRule *pbprofile.ProxyRule
		for i := range p.Rules {
			if p.Rules[i].ID == dec.RuleID {
				res.RuleIndex = i + 1
				matchedRule = &p.Rules[i]
				break
			}
		}
		if matchedRule != nil && matchedRule.ExcludeProcess && dec.Action == pbprofile.ActionDirect {
			res.Reason = "Excluded by rule -> Direct"
			return res, nil
		}
		if matchedRule != nil && matchedRule.Action == pbprofile.ActionProxy && dec.Action == pbprofile.ActionDirect {
			res.Reason = "Bypassed automatically (localhost, multicast, or DNS :53)"
			return res, nil
		}
		if dec.Action == pbprofile.ActionBlock {
			res.Reason = "Blocked by rule"
			return res, nil
		}
		if dec.Action == pbprofile.ActionDirect {
			res.Reason = "Direct by rule"
			return res, nil
		}

		// ActionProxy from matched rule.
		targetID := dec.ProxyConfigID
		if targetID == "" {
			targetID = effectiveDefaultProxyID(p)
		}
		px, ok := proxyByID[targetID]
		if !ok {
			res.Action = "block"
			res.FailClosed = true
			res.Reason = "Blocked (fail-closed): assigned proxy is missing"
			return res, nil
		}
		if proto == pbprofile.ProtoUDP && strings.ToUpper(string(px.Type)) != string(pbprofile.ProxySOCKS5) {
			res.Action = "block"
			res.FailClosed = true
			res.ProxyID = px.ID
			res.ProxyName = px.Name
			res.ProxyAddr = net.JoinHostPort(px.Host, strconv.Itoa(px.Port))
			res.Reason = fmt.Sprintf("Blocked (fail-closed): %s proxy does not relay UDP", strings.ToUpper(string(px.Type)))
			return res, nil
		}
		res.Action = "proxy"
		res.ProxyID = px.ID
		res.ProxyName = px.Name
		res.ProxyAddr = net.JoinHostPort(px.Host, strconv.Itoa(px.Port))
		res.Reason = fmt.Sprintf("Routed via %s", px.Name)
		return res, nil
	}

	// No rule matched -> Default Route.
	res.ViaDefault = true
	if p.DefaultProxyID == "" {
		res.Action = "direct"
		res.Reason = "Default Route: Direct"
		return res, nil
	}
	if rules.ShouldBypassProxy(destIP, uint16(port), proto == pbprofile.ProtoUDP, false) {
		res.Action = "direct"
		res.Reason = "Default Route: bypassed automatically (localhost, multicast, or DNS :53)"
		return res, nil
	}
	px, ok := proxyByID[p.DefaultProxyID]
	if !ok {
		res.Action = "block"
		res.FailClosed = true
		res.Reason = "Blocked (fail-closed): default proxy is missing"
		return res, nil
	}
	if proto == pbprofile.ProtoUDP && strings.ToUpper(string(px.Type)) != string(pbprofile.ProxySOCKS5) {
		res.Action = "block"
		res.FailClosed = true
		res.ProxyID = px.ID
		res.ProxyName = px.Name
		res.ProxyAddr = net.JoinHostPort(px.Host, strconv.Itoa(px.Port))
		res.Reason = fmt.Sprintf("Blocked (fail-closed): Default Route uses %s proxy (no UDP)", strings.ToUpper(string(px.Type)))
		return res, nil
	}
	res.Action = "proxy"
	res.ProxyID = px.ID
	res.ProxyName = px.Name
	res.ProxyAddr = net.JoinHostPort(px.Host, strconv.Itoa(px.Port))
	res.Reason = fmt.Sprintf("Default Route via %s", px.Name)
	return res, nil
}

// LeakCheckItem represents one verification line in a LeakReport.
type LeakCheckItem struct {
	ID     string `json:"id"` // "proxy_ip" | "gateway" | "dns" | "udp" | "engine"
	Name   string `json:"name"`
	Status string `json:"status"` // "ok" | "warn" | "fail"
	Detail string `json:"detail"`
}

// LeakReport summarizes active leak-protection posture without making unproxied
// outbound probes that could expose the user's real IP.
type LeakReport struct {
	Overall   string          `json:"overall"` // "protected" | "warning" | "unprotected"
	ExitIP    string          `json:"exitIp,omitempty"`
	Country   string          `json:"country,omitempty"`
	ProxyName string          `json:"proxyName,omitempty"`
	ProxyType string          `json:"proxyType,omitempty"`
	CheckedAt string          `json:"checkedAt"`
	Items     []LeakCheckItem `json:"items"`
}

// CheckLeak verifies exit IP through the active proxy, local gateway reachability,
// DNS-via-proxy posture, UDP/QUIC fail-closed guard, and OS interception state.
func (s *Service) CheckLeak(ctx context.Context) (LeakReport, error) {
	p, err := s.store.ActiveProfile()
	if err != nil {
		return LeakReport{}, err
	}
	s.hydratePasswords(&p)

	st := s.Status()
	rep := LeakReport{
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		Items:     make([]LeakCheckItem, 0, 5),
	}

	var activePx *pbprofile.ProxyConfig
	if p.DefaultProxyID != "" {
		for i := range p.Proxies {
			if p.Proxies[i].ID == p.DefaultProxyID {
				activePx = &p.Proxies[i]
				break
			}
		}
	}

	// 1. Engine & Fail-Closed Guard
	if st.State == driver.StateRunning && st.Enabled {
		rep.Items = append(rep.Items, LeakCheckItem{
			ID:     "engine",
			Name:   "Engine & Fail-Closed Guard",
			Status: "ok",
			Detail: fmt.Sprintf("Active (%s). Unreachable proxies block traffic instead of falling back to Direct.", st.Backend),
		})
	} else {
		rep.Items = append(rep.Items, LeakCheckItem{
			ID:     "engine",
			Name:   "Engine & Fail-Closed Guard",
			Status: "fail",
			Detail: fmt.Sprintf("Engine is %s. Traffic is not intercepted until AnyProxy is enabled.", st.State),
		})
	}

	// 2. Active Proxy Exit IP (probed strictly over the proxy tunnel, never direct)
	if activePx == nil {
		rep.Items = append(rep.Items, LeakCheckItem{
			ID:     "proxy_ip",
			Name:   "Proxy Exit IP",
			Status: "warn",
			Detail: "Default Route is set to Direct. Only apps matching explicit Proxy rules use a proxy.",
		})
	} else {
		rep.ProxyName = activePx.Name
		rep.ProxyType = strings.ToUpper(string(activePx.Type))
		up := upstreamFromConfig(*activePx)
		t0 := time.Now()
		exitIP, country := netproxy.LookupExitLocation(ctx, up)
		ms := time.Since(t0).Milliseconds()
		if exitIP != "" {
			rep.ExitIP = exitIP
			rep.Country = country
			loc := exitIP
			if country != "" {
				loc = fmt.Sprintf("%s (%s)", exitIP, country)
			}
			rep.Items = append(rep.Items, LeakCheckItem{
				ID:     "proxy_ip",
				Name:   "Proxy Exit IP",
				Status: "ok",
				Detail: fmt.Sprintf("Verified %s via %s in %d ms.", loc, activePx.Name, ms),
			})
			s.setHealthSnapshot(ProxyHealthStatus{
				ProxyID:    activePx.ID,
				ProxyName:  activePx.Name,
				ProxyAddr:  up.Addr(),
				Healthy:    true,
				Checked:    true,
				FailClosed: false,
				LatencyMs:  ms,
				CheckedAt:  rep.CheckedAt,
			}, 0)
		} else {
			// Verify TCP reachability to distinguish proxy-down vs GeoIP endpoint timeout.
			d := net.Dialer{Timeout: healthCheckTimeout}
			conn, derr := d.DialContext(ctx, "tcp", up.Addr())
			if derr != nil {
				rep.Items = append(rep.Items, LeakCheckItem{
					ID:     "proxy_ip",
					Name:   "Proxy Exit IP",
					Status: "fail",
					Detail: fmt.Sprintf("Proxy %s (%s) unreachable. Fail-closed blocks traffic so your real IP does not leak.", activePx.Name, up.Addr()),
				})
				s.setHealthSnapshot(ProxyHealthStatus{
					ProxyID:    activePx.ID,
					ProxyName:  activePx.Name,
					ProxyAddr:  up.Addr(),
					Healthy:    false,
					Checked:    true,
					FailClosed: st.State == driver.StateRunning && st.Enabled,
					Error:      derr.Error(),
					CheckedAt:  rep.CheckedAt,
				}, healthFailureThreshold)
			} else {
				_ = conn.Close()
				rep.Items = append(rep.Items, LeakCheckItem{
					ID:     "proxy_ip",
					Name:   "Proxy Exit IP",
					Status: "warn",
					Detail: fmt.Sprintf("Proxy %s TCP reachable, but exit IP lookup timed out.", activePx.Name),
				})
			}
		}

		// 3. Local Gateway Verification (127.0.0.1:10800)
		if addr := s.InboundAddr(); addr != "" {
			gwHost, gwPortStr, splitErr := net.SplitHostPort(addr)
			gwPort, _ := strconv.Atoi(gwPortStr)
			if splitErr == nil && gwPort > 0 {
				gwUp := netproxy.Upstream{
					ID:                "local-gateway",
					Name:              "Local Gateway",
					Type:              "SOCKS5",
					Host:              gwHost,
					Port:              gwPort,
					SendDomainToProxy: true,
				}
				gwIP, _ := netproxy.LookupExitLocation(ctx, gwUp)
				if gwIP != "" && (rep.ExitIP == "" || gwIP == rep.ExitIP) {
					rep.Items = append(rep.Items, LeakCheckItem{
						ID:     "gateway",
						Name:   "Local Gateway Route",
						Status: "ok",
						Detail: fmt.Sprintf("Local gateway %s exits at %s (matches proxy).", addr, gwIP),
					})
				} else if gwIP != "" && rep.ExitIP != "" && gwIP != rep.ExitIP {
					rep.Items = append(rep.Items, LeakCheckItem{
						ID:     "gateway",
						Name:   "Local Gateway Route",
						Status: "warn",
						Detail: fmt.Sprintf("Gateway exited at %s while default proxy exited at %s (check higher-priority rules).", gwIP, rep.ExitIP),
					})
				} else {
					rep.Items = append(rep.Items, LeakCheckItem{
						ID:     "gateway",
						Name:   "Local Gateway Route",
						Status: "fail",
						Detail: fmt.Sprintf("Local gateway %s could not reach exit endpoint (blocked fail-closed or proxy unreachable).", addr),
					})
				}
			}
		}

		// 4. DNS Protection
		if p.DnsViaProxy && activePx.SendDomainToProxy {
			rep.Items = append(rep.Items, LeakCheckItem{
				ID:     "dns",
				Name:   "DNS Leak Protection",
				Status: "ok",
				Detail: "DNS via Proxy and Remote Domain Resolution are both enabled.",
			})
		} else {
			var missing []string
			if !p.DnsViaProxy {
				missing = append(missing, "DNS via Proxy (Settings)")
			}
			if !activePx.SendDomainToProxy {
				missing = append(missing, "Resolve domains via proxy (Proxy settings)")
			}
			rep.Items = append(rep.Items, LeakCheckItem{
				ID:     "dns",
				Name:   "DNS Leak Protection",
				Status: "warn",
				Detail: "Local DNS may see hostnames. Enable: " + strings.Join(missing, ", ") + ".",
			})
		}

		// 5. UDP / QUIC Guard
		if strings.ToUpper(string(activePx.Type)) == string(pbprofile.ProxySOCKS5) {
			uCtx, uCancel := context.WithTimeout(ctx, 5*time.Second)
			assoc, uerr := up.DialUDP(uCtx)
			uCancel()
			if uerr == nil {
				_ = assoc.Close()
				rep.Items = append(rep.Items, LeakCheckItem{
					ID:     "udp",
					Name:   "UDP & QUIC Protection",
					Status: "ok",
					Detail: "SOCKS5 UDP ASSOCIATE succeeded. UDP/QUIC traffic relays through proxy.",
				})
			} else {
				rep.Items = append(rep.Items, LeakCheckItem{
					ID:     "udp",
					Name:   "UDP & QUIC Protection",
					Status: "warn",
					Detail: "SOCKS5 server rejected UDP ASSOCIATE. UDP/QUIC fails closed instead of leaking direct.",
				})
			}
		} else {
			rep.Items = append(rep.Items, LeakCheckItem{
				ID:     "udp",
				Name:   "UDP & QUIC Protection",
				Status: "ok",
				Detail: fmt.Sprintf("%s proxy is TCP-only; UDP/QUIC is blocked fail-closed with zero direct fallback.", rep.ProxyType),
			})
		}
	}

	// Compute overall verdict
	hasFail := false
	hasWarn := false
	for _, it := range rep.Items {
		if it.Status == "fail" {
			hasFail = true
		} else if it.Status == "warn" {
			hasWarn = true
		}
	}
	switch {
	case hasFail:
		rep.Overall = "unprotected"
	case hasWarn:
		rep.Overall = "warning"
	default:
		rep.Overall = "protected"
	}
	return rep, nil
}

// ProxyHealthStatus reports live reachability of the active default proxy while
// the engine is running so the UI can clearly explain when fail-closed is blocking traffic.
type ProxyHealthStatus struct {
	ProxyID    string `json:"proxyId,omitempty"`
	ProxyName  string `json:"proxyName,omitempty"`
	ProxyAddr  string `json:"proxyAddr,omitempty"`
	Healthy    bool   `json:"healthy"`
	Checked    bool   `json:"checked"`
	FailClosed bool   `json:"failClosed"`
	LatencyMs  int64  `json:"latencyMs,omitempty"`
	Error      string `json:"error,omitempty"`
	CheckedAt  string `json:"checkedAt,omitempty"`
}

// ProxyHealth returns the latest health snapshot for the active default proxy.
func (s *Service) ProxyHealth() ProxyHealthStatus {
	s.healthMu.Lock()
	defer s.healthMu.Unlock()
	return s.health
}

// CheckActiveProxyHealth immediately probes TCP reachability of the active default proxy.
func (s *Service) CheckActiveProxyHealth(ctx context.Context) ProxyHealthStatus {
	return s.probeActiveProxyHealth(ctx, true)
}

func (s *Service) probeActiveProxyHealth(ctx context.Context, immediate bool) ProxyHealthStatus {
	p, err := s.store.ActiveProfile()
	if err != nil {
		return s.ProxyHealth()
	}
	st := s.Status()
	engineActive := st.State == driver.StateRunning && st.Enabled

	if p.DefaultProxyID == "" {
		hs := ProxyHealthStatus{Healthy: true, Checked: false, FailClosed: false}
		s.setHealthSnapshot(hs, 0)
		return hs
	}

	var activePx *pbprofile.ProxyConfig
	for i := range p.Proxies {
		if p.Proxies[i].ID == p.DefaultProxyID {
			activePx = &p.Proxies[i]
			break
		}
	}
	if activePx == nil {
		hs := ProxyHealthStatus{
			ProxyID:    p.DefaultProxyID,
			Healthy:    false,
			Checked:    true,
			FailClosed: engineActive,
			Error:      "active proxy configuration is missing",
			CheckedAt:  time.Now().UTC().Format(time.RFC3339),
		}
		s.setHealthSnapshot(hs, healthFailureThreshold)
		return hs
	}

	addr := net.JoinHostPort(activePx.Host, strconv.Itoa(activePx.Port))
	t0 := time.Now()
	d := net.Dialer{Timeout: healthCheckTimeout}
	conn, derr := d.DialContext(ctx, "tcp", addr)
	ms := time.Since(t0).Milliseconds()
	nowStr := time.Now().UTC().Format(time.RFC3339)

	if derr == nil {
		_ = conn.Close()
		hs := ProxyHealthStatus{
			ProxyID:    activePx.ID,
			ProxyName:  activePx.Name,
			ProxyAddr:  addr,
			Healthy:    true,
			Checked:    true,
			FailClosed: false,
			LatencyMs:  ms,
			CheckedAt:  nowStr,
		}
		s.setHealthSnapshot(hs, 0)
		return hs
	}

	s.healthMu.Lock()
	fails := s.healthFails + 1
	if immediate && fails < healthFailureThreshold {
		fails = healthFailureThreshold
	}
	wasHealthy := s.health.Healthy || !s.health.Checked
	s.healthMu.Unlock()

	unhealthy := fails >= healthFailureThreshold
	hs := ProxyHealthStatus{
		ProxyID:    activePx.ID,
		ProxyName:  activePx.Name,
		ProxyAddr:  addr,
		Healthy:    !unhealthy,
		Checked:    true,
		FailClosed: unhealthy && engineActive,
		Error:      derr.Error(),
		CheckedAt:  nowStr,
	}
	s.setHealthSnapshot(hs, fails)
	if unhealthy && wasHealthy && s.logs != nil {
		s.logs.LogActivity("WARN", fmt.Sprintf("Active proxy %s (%s) is unreachable; fail-closed is blocking traffic (%v)", activePx.Name, addr, derr))
	}
	return hs
}

func (s *Service) setHealthSnapshot(next ProxyHealthStatus, fails int) {
	s.healthMu.Lock()
	prev := s.health
	s.health = next
	s.healthFails = fails
	s.healthMu.Unlock()

	if prev.ProxyID != next.ProxyID || prev.Healthy != next.Healthy || prev.Checked != next.Checked || prev.FailClosed != next.FailClosed {
		s.emit("proxy:health", next)
	}
}

func (s *Service) resetHealthState() {
	s.setHealthSnapshot(ProxyHealthStatus{Healthy: true, Checked: false, FailClosed: false}, 0)
}

func (s *Service) startHealthWatchdog() {
	s.stopHealthWatchdog()
	ctx, cancel := context.WithCancel(context.Background())
	s.healthMu.Lock()
	s.healthCtx = ctx
	s.healthCancel = cancel
	s.healthFails = 0
	s.healthMu.Unlock()

	s.healthWg.Add(1)
	go func() {
		defer s.healthWg.Done()
		s.probeActiveProxyHealth(ctx, true)
		ticker := time.NewTicker(healthCheckInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				st := s.Status()
				if st.State != driver.StateRunning || !st.Enabled {
					continue
				}
				if s.logs != nil {
					s.logs.TrimMemory(time.Now())
				}
				s.probeActiveProxyHealth(ctx, false)
			}
		}
	}()
}

func (s *Service) triggerHealthProbe() {
	s.resetHealthState()
	s.healthMu.Lock()
	ctx := s.healthCtx
	s.healthMu.Unlock()
	if ctx == nil {
		return
	}
	s.healthWg.Add(1)
	go func() {
		defer s.healthWg.Done()
		s.probeActiveProxyHealth(ctx, true)
	}()
}

func (s *Service) stopHealthWatchdog() {
	s.healthMu.Lock()
	cancel := s.healthCancel
	s.healthCancel = nil
	s.healthCtx = nil
	s.healthMu.Unlock()
	if cancel != nil {
		cancel()
	}
	s.healthWg.Wait()
	s.resetHealthState()
}

// TrimMemory releases closed flow records, prunes stale log maps, and returns
// unused Go heap pages to the OS when the window is hidden to the system tray.
func (s *Service) TrimMemory() {
	if s.tracker != nil {
		s.tracker.TrimMemory()
	}
	if s.drv != nil {
		_ = s.drv.ClearClosedFlows()
	}
	if s.logs != nil {
		s.logs.TrimMemory(time.Now())
	}
	goruntime.GC()
	debug.FreeOSMemory()
}

func upstreamFromConfig(px pbprofile.ProxyConfig) netproxy.Upstream {
	return netproxy.Upstream{
		ID:                px.ID,
		Name:              px.Name,
		Type:              string(px.Type),
		Host:              px.Host,
		Port:              px.Port,
		Username:          px.Username,
		Password:          px.Password,
		SendDomainToProxy: px.SendDomainToProxy,
	}
}
