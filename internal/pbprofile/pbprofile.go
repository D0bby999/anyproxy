// SPDX-License-Identifier: AGPL-3.0-or-later

// Package pbprofile parses and serializes ProxyBridge .pbprofile files (both
// Windows/Linux PascalCase and macOS camelCase layouts) and standalone rule
// JSON arrays.
package pbprofile

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

const (
	MaxFileBytes = 4 << 20 // 4 MiB limit from upstream Linux/Windows
	MaxProxies   = 256
	MaxRules     = 4096
)

type ProxyType string

const (
	ProxySOCKS5 ProxyType = "SOCKS5"
	ProxyHTTP   ProxyType = "HTTP"
)

type RuleProtocol string

const (
	ProtoTCP  RuleProtocol = "TCP"
	ProtoUDP  RuleProtocol = "UDP"
	ProtoBoth RuleProtocol = "BOTH"
)

type RuleAction string

const (
	ActionProxy  RuleAction = "PROXY"
	ActionDirect RuleAction = "DIRECT"
	ActionBlock  RuleAction = "BLOCK"
)

type ProxyConfig struct {
	ID                string    `json:"id"`
	LegacyID          int       `json:"legacyId,omitempty"`
	Name              string    `json:"name"`
	Type              ProxyType `json:"type"`
	Host              string    `json:"host"`
	Port              int       `json:"port"`
	Username          string    `json:"username,omitempty"`
	Password          string    `json:"password,omitempty"` // only populated in memory during import/export
	SendDomainToProxy bool      `json:"sendDomainToProxy"`
}

func (p ProxyConfig) Label() string {
	s := fmt.Sprintf("%s %s:%d", strings.ToUpper(string(p.Type)), p.Host, p.Port)
	if p.Name != "" {
		return p.Name + " = " + s
	}
	return s
}

type ProxyRule struct {
	ID                string       `json:"id"`
	Name              string       `json:"name"`
	ProcessNames      string       `json:"processNames"`
	TargetHosts       string       `json:"targetHosts"`
	TargetDomains     string       `json:"targetDomains,omitempty"`
	TargetPorts       string       `json:"targetPorts"`
	Protocol          RuleProtocol `json:"protocol"`
	Action            RuleAction   `json:"action"`
	ProxyConfigID     string       `json:"proxyConfigId,omitempty"`
	Enabled           bool         `json:"enabled"`
	LocalhostViaProxy bool         `json:"localhostViaProxy"`
}

// CombinedHosts returns TargetHosts and TargetDomains joined by ';', without duplicates.
func (r ProxyRule) CombinedHosts() string {
	h := strings.TrimSpace(r.TargetHosts)
	d := strings.TrimSpace(r.TargetDomains)
	if d == "" || d == "*" {
		if h == "" {
			return "*"
		}
		return h
	}
	if h == "" || h == "*" {
		return d
	}
	return h + ";" + d
}

type LogFilter struct {
	ID       string `json:"id"`
	Mode     string `json:"mode"` // "INCLUDE" | "EXCLUDE"
	Process  string `json:"process"`
	Host     string `json:"host"`
	Port     string `json:"port"`
	Protocol string `json:"protocol"` // "ALL" | "TCP" | "UDP"
	Action   string `json:"action"`   // "ALL" | "DIRECT" | "PROXY" | "BLOCK"
}

type Profile struct {
	ID                      string        `json:"id"`
	Name                    string        `json:"name"`
	TrafficLoggingEnabled   bool          `json:"trafficLoggingEnabled"`
	DnsViaProxy             bool          `json:"dnsViaProxy"`
	LogEveryConnection      bool          `json:"logEveryConnection"`
	AutoClearConnectionLogs bool          `json:"autoClearConnectionLogs"`
	DefaultProxyID          string        `json:"defaultProxyId,omitempty"`
	FontZoom                int           `json:"fontZoom"`
	CloseToMenuBar          bool          `json:"closeToMenuBar"`
	RunAtStartup            bool          `json:"runAtStartup"`
	Proxies                 []ProxyConfig `json:"proxies"`
	Rules                   []ProxyRule   `json:"rules"`
	LogFilters              []LogFilter   `json:"logFilters,omitempty"`
}

func DefaultProfile(name string) Profile {
	if name == "" {
		name = "Default"
	}
	return Profile{
		Name:                    name,
		TrafficLoggingEnabled:   true,
		DnsViaProxy:             true,
		LogEveryConnection:      false,
		AutoClearConnectionLogs: true,
		FontZoom:                100,
		Proxies:                 []ProxyConfig{},
		Rules:                   []ProxyRule{},
	}
}

func hasControlChars(s string, allowNewline bool) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < 0x20 {
			if allowNewline && (c == '\t' || c == '\r' || c == '\n') {
				continue
			}
			return true
		}
		if c == 0x7f {
			return true
		}
	}
	return false
}

// IsDomainToken reports whether a single host token is a domain name/glob rather than an IP/CIDR/range/wildcard.
func IsDomainToken(tok string) bool {
	tok = strings.TrimSpace(tok)
	if tok == "" || tok == "*" {
		return false
	}
	if strings.Contains(tok, ":") {
		return false // IPv6 literal, range, CIDR, or prefix wildcard
	}
	for i := 0; i < len(tok); i++ {
		c := tok[i]
		if (c >= '0' && c <= '9') || c == '.' || c == '*' || c == '-' || c == '/' {
			continue
		}
		return true
	}
	return false
}

// SplitHostsAndDomains separates a combined host list into IP patterns and domain patterns
// for Windows/Linux .pbprofile export compatibility.
func SplitHostsAndDomains(combined, explicitDomains string) (hostsOut, domainsOut string) {
	var ips, doms []string
	seenDoms := map[string]bool{}
	addDom := func(d string) {
		if d != "" && !seenDoms[d] {
			seenDoms[d] = true
			doms = append(doms, d)
		}
	}
	for _, raw := range strings.FieldsFunc(combined, func(r rune) bool { return r == ';' || r == ',' || r == '\n' }) {
		t := strings.TrimSpace(raw)
		if t == "" || t == "*" {
			continue
		}
		if IsDomainToken(t) {
			addDom(t)
		} else {
			ips = append(ips, t)
		}
	}
	for _, raw := range strings.FieldsFunc(explicitDomains, func(r rune) bool { return r == ';' || r == ',' || r == '\n' }) {
		t := strings.TrimSpace(raw)
		if t != "" && t != "*" {
			addDom(t)
		}
	}
	if len(ips) == 0 {
		hostsOut = "*"
	} else {
		hostsOut = strings.Join(ips, ";")
	}
	domainsOut = strings.Join(doms, ";")
	return hostsOut, domainsOut
}

// ParseProfile reads either a Windows/Linux (.pbprofile PascalCase) or macOS (.pbprofile camelCase) JSON stream.
func ParseProfile(r io.Reader) (Profile, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxFileBytes+1))
	if err != nil {
		return Profile{}, err
	}
	if len(data) > MaxFileBytes {
		return Profile{}, fmt.Errorf("profile exceeds maximum size of %d bytes", MaxFileBytes)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return Profile{}, fmt.Errorf("invalid profile JSON: %w", err)
	}
	if len(raw) == 0 {
		return Profile{}, errors.New("empty profile JSON")
	}
	_, hasMacProxies := raw["proxyConfigs"]
	_, hasMacRules := raw["proxyRules"]
	_, hasWinProxies := raw["ProxyConfigs"]
	_, hasWinRules := raw["ProxyRules"]
	if !hasMacProxies && !hasMacRules && !hasWinProxies && !hasWinRules {
		return Profile{}, errors.New("JSON does not contain ProxyConfigs/ProxyRules or proxyConfigs/proxyRules")
	}
	if hasMacProxies || hasMacRules {
		return parseMacProfile(raw)
	}
	return parseWinLinuxProfile(data)
}

type macProfileJSON struct {
	Name                  string         `json:"name"`
	TrafficLoggingEnabled *bool          `json:"trafficLoggingEnabled"`
	DNSEnabled            *bool          `json:"dnsEnabled"`
	CloseToMenuBar        bool           `json:"closeToMenuBar"`
	RunAtStartup          bool           `json:"runAtStartup"`
	ProxyConfigs          []macProxyJSON `json:"proxyConfigs"`
	ProxyRules            []macRuleJSON  `json:"proxyRules"`
}

type macProxyJSON struct {
	ID         json.RawMessage `json:"id"`
	Name       string          `json:"name"`
	Type       string          `json:"type"`
	Host       string          `json:"host"`
	Port       json.RawMessage `json:"port"`
	Username   string          `json:"username"`
	Password   string          `json:"password"`
	SendDomain *bool           `json:"sendDomain"`
}

func parseFlexibleID(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return strings.TrimSpace(s)
	}
	var n int
	if err := json.Unmarshal(raw, &n); err == nil {
		return fmt.Sprintf("proxy-%d", n)
	}
	return ""
}

type macRuleJSON struct {
	LocalID           string `json:"localId"`
	Name              string `json:"name"`
	ProcessNames      string `json:"processNames"`
	TargetHosts       string `json:"targetHosts"`
	TargetPorts       string `json:"targetPorts"`
	Protocol          string `json:"protocol"`
	Action            string `json:"action"`
	Enabled           *bool  `json:"enabled"`
	LocalhostViaProxy *bool  `json:"localhostViaProxy"`
}

func parseMacProfile(raw map[string]json.RawMessage) (Profile, error) {
	var m macProfileJSON
	if v, ok := raw["name"]; ok {
		_ = json.Unmarshal(v, &m.Name)
	} else if v, ok := raw["Name"]; ok {
		_ = json.Unmarshal(v, &m.Name)
	}
	if v, ok := raw["trafficLoggingEnabled"]; ok {
		_ = json.Unmarshal(v, &m.TrafficLoggingEnabled)
	}
	if v, ok := raw["dnsEnabled"]; ok {
		_ = json.Unmarshal(v, &m.DNSEnabled)
	}
	if v, ok := raw["closeToMenuBar"]; ok {
		_ = json.Unmarshal(v, &m.CloseToMenuBar)
	}
	if v, ok := raw["runAtStartup"]; ok {
		_ = json.Unmarshal(v, &m.RunAtStartup)
	}
	if v, ok := raw["proxyConfigs"]; ok {
		if err := json.Unmarshal(v, &m.ProxyConfigs); err != nil {
			return Profile{}, fmt.Errorf("parse macOS proxyConfigs: %w", err)
		}
	}
	if v, ok := raw["proxyRules"]; ok {
		if err := json.Unmarshal(v, &m.ProxyRules); err != nil {
			return Profile{}, fmt.Errorf("parse macOS proxyRules: %w", err)
		}
	}
	p := DefaultProfile(m.Name)
	if m.TrafficLoggingEnabled != nil {
		p.TrafficLoggingEnabled = *m.TrafficLoggingEnabled
	}
	if m.DNSEnabled != nil {
		p.DnsViaProxy = *m.DNSEnabled
	}
	p.CloseToMenuBar = m.CloseToMenuBar
	p.RunAtStartup = m.RunAtStartup

	for i, px := range m.ProxyConfigs {
		port, err := parseFlexiblePort(px.Port)
		if err != nil {
			return Profile{}, fmt.Errorf("proxy %d: %w", i+1, err)
		}
		sendDom := true
		if px.SendDomain != nil {
			sendDom = *px.SendDomain
		}
		id := parseFlexibleID(px.ID)
		if id == "" {
			id = fmt.Sprintf("proxy-%d", i+1)
		}
		p.Proxies = append(p.Proxies, ProxyConfig{
			ID:                id,
			LegacyID:          i + 1,
			Name:              px.Name,
			Type:              normalizeProxyType(px.Type),
			Host:              strings.TrimSpace(px.Host),
			Port:              port,
			Username:          px.Username,
			Password:          px.Password,
			SendDomainToProxy: sendDom,
		})
	}
	if len(p.Proxies) > 0 {
		p.DefaultProxyID = p.Proxies[0].ID
	}

	for i, rl := range m.ProxyRules {
		en := true
		if rl.Enabled != nil {
			en = *rl.Enabled
		}
		lh := false
		if rl.LocalhostViaProxy != nil {
			lh = *rl.LocalhostViaProxy
		}
		id := rl.LocalID
		if id == "" {
			id = fmt.Sprintf("rule-%d", i+1)
		}
		actRaw := strings.TrimSpace(rl.Action)
		var act RuleAction
		var proxyID string
		switch strings.ToUpper(actRaw) {
		case "DIRECT", "":
			act = ActionDirect
		case "BLOCK":
			act = ActionBlock
		case "PROXY":
			act = ActionProxy
			proxyID = p.DefaultProxyID
		default:
			act = ActionProxy
			proxyID = actRaw
		}
		hosts, domains := SplitHostsAndDomains(rl.TargetHosts, "")
		p.Rules = append(p.Rules, ProxyRule{
			ID:                id,
			Name:              rl.Name,
			ProcessNames:      defaultWildcard(rl.ProcessNames),
			TargetHosts:       hosts,
			TargetDomains:     domains,
			TargetPorts:       defaultWildcard(rl.TargetPorts),
			Protocol:          RuleProtocol(strings.ToUpper(defaultString(rl.Protocol, "BOTH"))),
			Action:            act,
			ProxyConfigID:     proxyID,
			Enabled:           en,
			LocalhostViaProxy: lh,
		})
	}
	if err := ValidateProfile(&p); err != nil {
		return Profile{}, err
	}
	return p, nil
}

type winProfileJSON struct {
	Version                 string          `json:"Version"`
	Name                    string          `json:"Name"`
	IsTrafficLoggingEnabled *bool           `json:"IsTrafficLoggingEnabled"`
	DnsViaProxy             *bool           `json:"DnsViaProxy"`
	LogEveryConnection      *bool           `json:"LogEveryConnection"`
	AutoClearConnectionLogs *bool           `json:"AutoClearConnectionLogs"`
	DefaultProxyId          int             `json:"DefaultProxyId"`
	FontZoom                int             `json:"FontZoom"`
	LocalhostViaProxy       *bool           `json:"LocalhostViaProxy"`
	ProxyConfigs            []winProxyJSON  `json:"ProxyConfigs"`
	ProxyRules              []winRuleJSON   `json:"ProxyRules"`
	LogFilters              []winFilterJSON `json:"LogFilters"`
}

type winProxyJSON struct {
	Id                int             `json:"Id"`
	Name              string          `json:"Name"`
	Type              string          `json:"Type"`
	Host              string          `json:"Host"`
	Port              json.RawMessage `json:"Port"`
	Username          string          `json:"Username"`
	Password          string          `json:"Password"`
	SendDomainToProxy *bool           `json:"SendDomainToProxy"`
}

type winRuleJSON struct {
	Name              string `json:"Name"`
	ProcessName       string `json:"ProcessName"`
	TargetHosts       string `json:"TargetHosts"`
	TargetDomains     string `json:"TargetDomains"`
	TargetPorts       string `json:"TargetPorts"`
	Protocol          string `json:"Protocol"`
	Action            string `json:"Action"`
	IsEnabled         *bool  `json:"IsEnabled"`
	LocalhostViaProxy *bool  `json:"LocalhostViaProxy"`
	ProxyConfigId     int    `json:"ProxyConfigId"`
}

type winFilterJSON struct {
	Mode     string `json:"Mode"`
	Process  string `json:"Process"`
	Host     string `json:"Host"`
	Port     string `json:"Port"`
	Protocol string `json:"Protocol"`
	Action   string `json:"Action"`
}

func parseFlexiblePort(raw json.RawMessage) (int, error) {
	if len(raw) == 0 {
		return 0, errors.New("missing port")
	}
	var n int
	if err := json.Unmarshal(raw, &n); err == nil {
		return n, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return strconv.Atoi(strings.TrimSpace(s))
	}
	return 0, fmt.Errorf("invalid port %s", string(raw))
}

func parseWinLinuxProfile(data []byte) (Profile, error) {
	var w winProfileJSON
	if err := json.Unmarshal(data, &w); err != nil {
		return Profile{}, fmt.Errorf("parse Windows/Linux profile: %w", err)
	}
	p := DefaultProfile(w.Name)
	if w.IsTrafficLoggingEnabled != nil {
		p.TrafficLoggingEnabled = *w.IsTrafficLoggingEnabled
	}
	if w.DnsViaProxy != nil {
		p.DnsViaProxy = *w.DnsViaProxy
	}
	if w.LogEveryConnection != nil {
		p.LogEveryConnection = *w.LogEveryConnection
	}
	if w.AutoClearConnectionLogs != nil {
		p.AutoClearConnectionLogs = *w.AutoClearConnectionLogs
	}
	if w.FontZoom >= 60 && w.FontZoom <= 300 {
		p.FontZoom = w.FontZoom
	}
	defaultLocalhost := false
	if w.LocalhostViaProxy != nil {
		defaultLocalhost = *w.LocalhostViaProxy
	}

	intToID := map[int]string{}
	for i, px := range w.ProxyConfigs {
		port, err := parseFlexiblePort(px.Port)
		if err != nil {
			return Profile{}, fmt.Errorf("proxy %d: %w", i+1, err)
		}
		sendDom := true
		if px.SendDomainToProxy != nil {
			sendDom = *px.SendDomainToProxy
		}
		legID := px.Id
		if legID == 0 {
			legID = i + 1
		}
		id := fmt.Sprintf("proxy-%d", legID)
		intToID[legID] = id
		p.Proxies = append(p.Proxies, ProxyConfig{
			ID:                id,
			LegacyID:          legID,
			Name:              px.Name,
			Type:              normalizeProxyType(px.Type),
			Host:              strings.TrimSpace(px.Host),
			Port:              port,
			Username:          px.Username,
			Password:          px.Password,
			SendDomainToProxy: sendDom,
		})
	}
	if w.DefaultProxyId != 0 {
		if id, ok := intToID[w.DefaultProxyId]; ok {
			p.DefaultProxyID = id
		} else {
			return Profile{}, fmt.Errorf("DefaultProxyId %d does not match any proxy in profile", w.DefaultProxyId)
		}
	} else if len(p.Proxies) > 0 {
		p.DefaultProxyID = p.Proxies[0].ID
	}

	for i, rl := range w.ProxyRules {
		en := true
		if rl.IsEnabled != nil {
			en = *rl.IsEnabled
		}
		lh := defaultLocalhost
		if rl.LocalhostViaProxy != nil {
			lh = *rl.LocalhostViaProxy
		}
		act := RuleAction(strings.ToUpper(strings.TrimSpace(rl.Action)))
		var proxyID string
		if act == ActionProxy {
			if rl.ProxyConfigId == 0 {
				proxyID = p.DefaultProxyID
			} else if mapped, ok := intToID[rl.ProxyConfigId]; ok {
				proxyID = mapped
			} else {
				return Profile{}, fmt.Errorf("rule %d references unknown ProxyConfigId %d", i+1, rl.ProxyConfigId)
			}
		}
		hosts, domains := SplitHostsAndDomains(rl.TargetHosts, rl.TargetDomains)
		p.Rules = append(p.Rules, ProxyRule{
			ID:                fmt.Sprintf("rule-%d", i+1),
			Name:              rl.Name,
			ProcessNames:      defaultWildcard(rl.ProcessName),
			TargetHosts:       hosts,
			TargetDomains:     domains,
			TargetPorts:       defaultWildcard(rl.TargetPorts),
			Protocol:          RuleProtocol(strings.ToUpper(defaultString(rl.Protocol, "BOTH"))),
			Action:            act,
			ProxyConfigID:     proxyID,
			Enabled:           en,
			LocalhostViaProxy: lh,
		})
	}
	for i, lf := range w.LogFilters {
		p.LogFilters = append(p.LogFilters, LogFilter{
			ID:       fmt.Sprintf("filter-%d", i+1),
			Mode:     strings.ToUpper(defaultString(lf.Mode, "INCLUDE")),
			Process:  lf.Process,
			Host:     lf.Host,
			Port:     lf.Port,
			Protocol: strings.ToUpper(defaultString(lf.Protocol, "ALL")),
			Action:   strings.ToUpper(defaultString(lf.Action, "ALL")),
		})
	}
	if err := ValidateProfile(&p); err != nil {
		return Profile{}, err
	}
	return p, nil
}

// ValidateProfile enforces upstream ProxyBridge constraints across all platforms.
func ValidateProfile(p *Profile) error {
	if hasControlChars(p.Name, false) {
		return errors.New("profile name contains control characters")
	}
	if len(p.Proxies) > MaxProxies {
		return fmt.Errorf("profile has %d proxies (max %d)", len(p.Proxies), MaxProxies)
	}
	if len(p.Rules) > MaxRules {
		return fmt.Errorf("profile has %d rules (max %d)", len(p.Rules), MaxRules)
	}
	seenIDs := map[string]bool{}
	for i, px := range p.Proxies {
		if px.ID == "" {
			return fmt.Errorf("proxy %d has empty id", i+1)
		}
		if seenIDs[px.ID] {
			return fmt.Errorf("duplicate proxy id %q", px.ID)
		}
		seenIDs[px.ID] = true
		if hasControlChars(px.Name, false) || hasControlChars(px.Host, false) ||
			hasControlChars(px.Username, false) || hasControlChars(px.Password, false) {
			return fmt.Errorf("proxy %q contains control characters", px.ID)
		}
		if strings.TrimSpace(px.Host) == "" {
			return fmt.Errorf("proxy %q has empty host", px.ID)
		}
		if px.Port < 1 || px.Port > 65535 {
			return fmt.Errorf("proxy %q has invalid port %d (must be 1..65535)", px.ID, px.Port)
		}
		if px.Type != ProxySOCKS5 && px.Type != ProxyHTTP {
			return fmt.Errorf("proxy %q has invalid type %q (must be SOCKS5 or HTTP)", px.ID, px.Type)
		}
		if px.Type == ProxySOCKS5 && (len(px.Username) > 255 || len(px.Password) > 255) {
			return fmt.Errorf("proxy %q has SOCKS5 username or password exceeding 255 bytes", px.ID)
		}
	}
	for i, rl := range p.Rules {
		if hasControlChars(rl.Name, false) || hasControlChars(rl.ProcessNames, false) ||
			hasControlChars(rl.TargetHosts, true) || hasControlChars(rl.TargetDomains, true) ||
			hasControlChars(rl.TargetPorts, false) {
			return fmt.Errorf("rule %d contains control characters", i+1)
		}
		switch rl.Protocol {
		case ProtoTCP, ProtoUDP, ProtoBoth:
		default:
			return fmt.Errorf("rule %d has invalid protocol %q (must be TCP, UDP, or BOTH)", i+1, rl.Protocol)
		}
		switch rl.Action {
		case ActionProxy, ActionDirect, ActionBlock:
		default:
			return fmt.Errorf("rule %d has invalid action %q (must be PROXY, DIRECT, or BLOCK)", i+1, rl.Action)
		}
	}
	return nil
}

// MarshalProfile serializes a Profile into the cross-platform .pbprofile JSON format
// (emitting both Windows/Linux PascalCase fields and macOS camelCase fields so any
// upstream ProxyBridge app on Windows, macOS, or Linux can import the file directly).
func MarshalProfile(p Profile) ([]byte, error) {
	if err := ValidateProfile(&p); err != nil {
		return nil, err
	}
	idToInt := map[string]int{}
	winProxies := make([]map[string]any, 0, len(p.Proxies))
	macProxies := make([]map[string]any, 0, len(p.Proxies))
	for i, px := range p.Proxies {
		intID := px.LegacyID
		if intID <= 0 {
			intID = i + 1
		}
		idToInt[px.ID] = intID
		winProxies = append(winProxies, map[string]any{
			"Id":                intID,
			"Name":              px.Name,
			"Type":              strings.ToLower(string(px.Type)),
			"Host":              px.Host,
			"Port":              strconv.Itoa(px.Port),
			"Username":          px.Username,
			"Password":          px.Password,
			"SendDomainToProxy": px.SendDomainToProxy,
		})
		macPx := map[string]any{
			"id":         px.ID,
			"name":       px.Name,
			"type":       strings.ToLower(string(px.Type)),
			"host":       px.Host,
			"port":       px.Port,
			"sendDomain": px.SendDomainToProxy,
		}
		if px.Username != "" {
			macPx["username"] = px.Username
		}
		if px.Password != "" {
			macPx["password"] = px.Password
		}
		macProxies = append(macProxies, macPx)
	}
	defIntID := 0
	if p.DefaultProxyID != "" {
		defIntID = idToInt[p.DefaultProxyID]
	} else if len(p.Proxies) > 0 {
		defIntID = idToInt[p.Proxies[0].ID]
	}

	winRules := make([]map[string]any, 0, len(p.Rules))
	macRules := make([]map[string]any, 0, len(p.Rules))
	for _, rl := range p.Rules {
		hosts, domains := SplitHostsAndDomains(rl.TargetHosts, rl.TargetDomains)
		cfgInt := 0
		macAct := string(rl.Action)
		if rl.Action == ActionProxy {
			if id, ok := idToInt[rl.ProxyConfigID]; ok {
				cfgInt = id
			} else {
				cfgInt = defIntID
			}
			if rl.ProxyConfigID != "" {
				macAct = rl.ProxyConfigID
			} else if len(p.Proxies) > 0 {
				macAct = p.Proxies[0].ID
			}
		}
		winRules = append(winRules, map[string]any{
			"Name":              rl.Name,
			"ProcessName":       defaultWildcard(rl.ProcessNames),
			"TargetHosts":       hosts,
			"TargetDomains":     domains,
			"TargetPorts":       defaultWildcard(rl.TargetPorts),
			"Protocol":          string(rl.Protocol),
			"Action":            string(rl.Action),
			"IsEnabled":         rl.Enabled,
			"LocalhostViaProxy": rl.LocalhostViaProxy,
			"ProxyConfigId":     cfgInt,
		})
		macRules = append(macRules, map[string]any{
			"localId":           rl.ID,
			"name":              rl.Name,
			"processNames":      defaultWildcard(rl.ProcessNames),
			"targetHosts":       rl.CombinedHosts(),
			"targetPorts":       defaultWildcard(rl.TargetPorts),
			"protocol":          string(rl.Protocol),
			"action":            macAct,
			"enabled":           rl.Enabled,
			"localhostViaProxy": rl.LocalhostViaProxy,
		})
	}
	out := map[string]any{
		"Version":                 "1.0",
		"Name":                    p.Name,
		"IsTrafficLoggingEnabled": p.TrafficLoggingEnabled,
		"DnsViaProxy":             p.DnsViaProxy,
		"LogEveryConnection":      p.LogEveryConnection,
		"AutoClearConnectionLogs": p.AutoClearConnectionLogs,
		"DefaultProxyId":          defIntID,
		"FontZoom":                p.FontZoom,
		"ProxyConfigs":            winProxies,
		"ProxyRules":              winRules,
		// macOS keys for universal import on macOS ProxyBridge as well:
		"name":                  p.Name,
		"trafficLoggingEnabled": p.TrafficLoggingEnabled,
		"dnsEnabled":            p.DnsViaProxy,
		"closeToMenuBar":        p.CloseToMenuBar,
		"runAtStartup":          p.RunAtStartup,
		"proxyConfigs":          macProxies,
		"proxyRules":            macRules,
	}
	if len(p.LogFilters) > 0 {
		filters := make([]map[string]any, 0, len(p.LogFilters))
		for _, lf := range p.LogFilters {
			filters = append(filters, map[string]any{
				"Mode":     lf.Mode,
				"Process":  lf.Process,
				"Host":     lf.Host,
				"Port":     lf.Port,
				"Protocol": lf.Protocol,
				"Action":   lf.Action,
			})
		}
		out["LogFilters"] = filters
	}
	return json.MarshalIndent(out, "", "  ")
}

// WriteProfileFile writes a .pbprofile file with owner-only 0600 permissions.
func WriteProfileFile(path string, p Profile) error {
	b, err := MarshalProfile(p)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

// ParseRulesJSON parses a standalone JSON array of rules (exported via Rules -> Export).
func ParseRulesJSON(r io.Reader, defaultProxyID string) ([]ProxyRule, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxFileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxFileBytes {
		return nil, fmt.Errorf("rules JSON exceeds %d bytes", MaxFileBytes)
	}
	var rawList []map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(data), &rawList); err != nil {
		return nil, fmt.Errorf("invalid rules JSON array: %w", err)
	}
	if len(rawList) > MaxRules {
		return nil, fmt.Errorf("rules JSON has %d rules (max %d)", len(rawList), MaxRules)
	}
	out := make([]ProxyRule, 0, len(rawList))
	for i, m := range rawList {
		getStr := func(keys ...string) string {
			for _, k := range keys {
				if v, ok := m[k].(string); ok {
					return v
				}
			}
			return ""
		}
		getBool := func(def bool, keys ...string) bool {
			for _, k := range keys {
				if v, ok := m[k].(bool); ok {
					return v
				}
			}
			return def
		}
		proc := defaultWildcard(getStr("processNames", "ProcessName"))
		rawHosts := getStr("targetHosts", "TargetHosts")
		rawDoms := getStr("targetDomains", "TargetDomains")
		hosts, doms := SplitHostsAndDomains(rawHosts, rawDoms)
		ports := defaultWildcard(getStr("targetPorts", "TargetPorts"))
		proto := RuleProtocol(strings.ToUpper(defaultString(getStr("protocol", "Protocol"), "BOTH")))
		actRaw := strings.TrimSpace(getStr("action", "Action", "ruleAction"))
		var act RuleAction
		var proxyID string
		switch strings.ToUpper(actRaw) {
		case "DIRECT", "":
			act = ActionDirect
		case "BLOCK":
			act = ActionBlock
		case "PROXY":
			act = ActionProxy
			proxyID = defaultProxyID
		default:
			act = ActionProxy
			proxyID = actRaw
		}
		out = append(out, ProxyRule{
			ID:                defaultString(getStr("localId", "id"), fmt.Sprintf("rule-%d", i+1)),
			Name:              getStr("name", "Name"),
			ProcessNames:      proc,
			TargetHosts:       hosts,
			TargetDomains:     doms,
			TargetPorts:       ports,
			Protocol:          proto,
			Action:            act,
			ProxyConfigID:     proxyID,
			Enabled:           getBool(true, "enabled", "IsEnabled"),
			LocalhostViaProxy: getBool(false, "localhostViaProxy", "LocalhostViaProxy"),
		})
	}
	return out, nil
}

// MarshalRulesJSON exports rules in upstream rule-array JSON format.
func MarshalRulesJSON(rules []ProxyRule) ([]byte, error) {
	list := make([]map[string]any, 0, len(rules))
	for _, rl := range rules {
		act := string(rl.Action)
		if rl.Action == ActionProxy && rl.ProxyConfigID != "" {
			act = rl.ProxyConfigID
		}
		list = append(list, map[string]any{
			"name":              rl.Name,
			"action":            act,
			"enabled":           rl.Enabled,
			"processNames":      defaultWildcard(rl.ProcessNames),
			"protocol":          string(rl.Protocol),
			"targetHosts":       rl.CombinedHosts(),
			"targetPorts":       defaultWildcard(rl.TargetPorts),
			"localhostViaProxy": rl.LocalhostViaProxy,
		})
	}
	return json.MarshalIndent(list, "", "  ")
}

func normalizeProxyType(t string) ProxyType {
	switch strings.ToUpper(strings.TrimSpace(t)) {
	case "SOCKS5", "SOCKS5H", "SOCKS":
		return ProxySOCKS5
	case "HTTP", "HTTPS":
		return ProxyHTTP
	default:
		return ProxyType(strings.ToUpper(strings.TrimSpace(t)))
	}
}

func defaultWildcard(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "*"
	}
	return s
}

func defaultString(s, def string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	return s
}
