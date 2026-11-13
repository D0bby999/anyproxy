// SPDX-License-Identifier: AGPL-3.0-or-later

package rules

import (
	"bytes"
	"net/netip"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/D0bby999/anyproxy/internal/pbprofile"
)

// FlowProcess holds pre-lowercased process identity strings for a flow.
type FlowProcess struct {
	candidates []procCandidate
}

type procCandidate struct {
	path string
	file string
}

func NewFlowProcess(bundleID, processNameOrPath string) FlowProcess {
	var raw []string
	if bundleID != "" {
		raw = append(raw, bundleID)
	}
	if processNameOrPath != "" && processNameOrPath != bundleID {
		raw = append(raw, processNameOrPath)
	}
	cands := make([]procCandidate, 0, len(raw))
	for _, s := range raw {
		low := strings.ToLower(s)
		base := lastComponent(low)
		cands = append(cands, procCandidate{path: low, file: base})
	}
	return FlowProcess{candidates: cands}
}

func lastComponent(p string) string {
	if idx := strings.LastIndexAny(p, `/\`); idx >= 0 {
		return p[idx+1:]
	}
	return filepath.Base(p)
}

type procPattern struct {
	pattern  string
	fullPath bool
}

type portRange struct {
	lo uint16
	hi uint16
}

type ipRangePattern struct {
	isV6 bool
	lo   [16]byte
	hi   [16]byte
}

type v4OctetRange struct {
	any bool
	lo  uint8
	hi  uint8
}

type hostPattern struct {
	raw        string
	isCIDR     bool
	prefix     netip.Prefix
	isRange    bool
	ipRange    ipRangePattern
	isV4Octets bool
	v4Octets   [4]v4OctetRange
	isV6Prefix bool
	v6Groups   []uint16
	isExactIP  bool
	exactAddr  netip.Addr
	domainGlob string
	domainApex string
}

// Matcher is the pre-parsed, allocation-free form of a single ProxyRule.
type Matcher struct {
	Rule            pbprofile.ProxyRule
	processAll      bool
	excludeProcess  bool
	processPatterns []procPattern
	hostAll         bool
	hostPatterns    []hostPattern
	hasDomainRule   bool
	portAll         bool
	portRanges      []portRange
}

func NewMatcher(rule pbprofile.ProxyRule) *Matcher {
	m := &Matcher{Rule: rule, excludeProcess: rule.ExcludeProcess}

	// 1. Process patterns
	pRaw := strings.TrimSpace(rule.ProcessNames)
	if pRaw == "" || pRaw == "*" {
		m.processAll = true
	} else {
		for _, tok := range splitList(pRaw, false) {
			if tok == "*" {
				m.processAll = true
				m.processPatterns = nil
				break
			}
			low := strings.ToLower(tok)
			m.processPatterns = append(m.processPatterns, procPattern{
				pattern:  low,
				fullPath: strings.ContainsAny(low, `/\`),
			})
		}
	}

	// 2. Host & Domain patterns
	combined := rule.CombinedHosts()
	if combined == "" || combined == "*" {
		m.hostAll = true
	} else {
		for _, tok := range splitList(combined, true) {
			if tok == "*" {
				m.hostAll = true
				m.hostPatterns = nil
				break
			}
			hp := compileHostPattern(tok)
			if hp.domainGlob != "" {
				m.hasDomainRule = true
			}
			m.hostPatterns = append(m.hostPatterns, hp)
		}
	}

	// 3. Port patterns
	ptRaw := strings.TrimSpace(rule.TargetPorts)
	if ptRaw == "" || ptRaw == "*" {
		m.portAll = true
	} else {
		for _, tok := range splitList(ptRaw, false) {
			if tok == "*" {
				m.portAll = true
				m.portRanges = nil
				break
			}
			if dash := strings.IndexByte(tok, '-'); dash >= 0 {
				lo, err1 := strconv.ParseUint(strings.TrimSpace(tok[:dash]), 10, 16)
				hi, err2 := strconv.ParseUint(strings.TrimSpace(tok[dash+1:]), 10, 16)
				if err1 == nil && err2 == nil && lo <= hi {
					m.portRanges = append(m.portRanges, portRange{lo: uint16(lo), hi: uint16(hi)})
				}
			} else if p, err := strconv.ParseUint(tok, 10, 16); err == nil {
				m.portRanges = append(m.portRanges, portRange{lo: uint16(p), hi: uint16(p)})
			}
		}
	}

	return m
}

func compileHostPattern(raw string) hostPattern {
	hp := hostPattern{raw: raw}
	low := strings.ToLower(strings.TrimSpace(raw))

	// CIDR: e.g. 10.0.0.0/8 or 2001:db8::/32
	if strings.IndexByte(low, '/') >= 0 {
		if pfx, err := netip.ParsePrefix(low); err == nil {
			hp.isCIDR = true
			hp.prefix = pfx.Masked()
			return hp
		}
	}

	// IPv4/IPv6 range: e.g. 10.0.0.1-10.0.0.254 or 2001:db8::1-2001:db8::ff
	if dash := strings.IndexByte(low, '-'); dash >= 0 {
		a1, err1 := netip.ParseAddr(strings.TrimSpace(low[:dash]))
		a2, err2 := netip.ParseAddr(strings.TrimSpace(low[dash+1:]))
		if err1 == nil && err2 == nil {
			u1, u2 := a1.Unmap(), a2.Unmap()
			if u1.Is4() == u2.Is4() {
				hp.isRange = true
				hp.ipRange = ipRangePattern{isV6: u1.Is6(), lo: u1.As16(), hi: u2.As16()}
				return hp
			}
		}
	}

	// IPv6 pattern (contains ':')
	if strings.IndexByte(low, ':') >= 0 {
		if strings.HasSuffix(low, "*") {
			trimmed := strings.TrimSuffix(low, "*")
			trimmed = strings.TrimSuffix(trimmed, ":")
			parts := strings.Split(trimmed, ":")
			if len(parts) > 0 && len(parts) <= 8 {
				groups := make([]uint16, 0, len(parts))
				valid := true
				for _, g := range parts {
					v, err := strconv.ParseUint(g, 16, 16)
					if err != nil {
						valid = false
						break
					}
					groups = append(groups, uint16(v))
				}
				if valid {
					hp.isV6Prefix = true
					hp.v6Groups = groups
					return hp
				}
			}
		}
		if addr, err := netip.ParseAddr(low); err == nil {
			hp.isExactIP = true
			hp.exactAddr = addr.Unmap()
			return hp
		}
	}

	// Exact IPv4
	if addr, err := netip.ParseAddr(low); err == nil {
		hp.isExactIP = true
		hp.exactAddr = addr.Unmap()
		return hp
	}

	// IPv4 octet wildcard/range: e.g. 192.168.1.* or 10.1-5.*.*
	if octets, ok := parseV4Octets(low); ok {
		hp.isV4Octets = true
		hp.v4Octets = octets
		return hp
	}

	// Domain glob: e.g. *.github.com, github.com, api?.example.org
	hp.domainGlob = low
	if strings.HasPrefix(low, "*.") {
		hp.domainApex = low[2:]
	}
	return hp
}

func parseV4Octets(s string) ([4]v4OctetRange, bool) {
	var out [4]v4OctetRange
	parts := strings.Split(s, ".")
	if len(parts) != 4 {
		return out, false
	}
	for i, p := range parts {
		if p == "*" {
			out[i] = v4OctetRange{any: true, lo: 0, hi: 255}
			continue
		}
		if dash := strings.IndexByte(p, '-'); dash >= 0 {
			lo, err1 := strconv.ParseUint(p[:dash], 10, 8)
			hi, err2 := strconv.ParseUint(p[dash+1:], 10, 8)
			if err1 != nil || err2 != nil || lo > hi {
				return out, false
			}
			out[i] = v4OctetRange{lo: uint8(lo), hi: uint8(hi)}
			continue
		}
		v, err := strconv.ParseUint(p, 10, 8)
		if err != nil {
			return out, false
		}
		out[i] = v4OctetRange{lo: uint8(v), hi: uint8(v)}
	}
	return out, true
}

func (m *Matcher) MatchesProcess(flow FlowProcess) bool {
	if m.processAll {
		return !m.excludeProcess // exclude with * means "match nothing"
	}
	matched := false
	for _, cand := range flow.candidates {
		for _, p := range m.processPatterns {
			target := cand.file
			if p.fullPath {
				target = cand.path
			}
			if GlobMatch(p.pattern, target) {
				matched = true
				break
			}
		}
		if matched {
			break
		}
	}
	if m.excludeProcess {
		return !matched
	}
	return matched
}

func (m *Matcher) MatchesPort(port uint16) bool {
	if m.portAll {
		return true
	}
	for _, r := range m.portRanges {
		if port >= r.lo && port <= r.hi {
			return true
		}
	}
	return false
}

func (m *Matcher) MatchesHost(ip string, domains []string) bool {
	if m.hostAll {
		return true
	}
	addr, addrErr := netip.ParseAddr(strings.TrimSpace(ip))
	if addrErr == nil {
		addr = addr.Unmap()
	}
	for _, hp := range m.hostPatterns {
		if addrErr == nil && matchHostIP(hp, addr) {
			return true
		}
		if hp.domainGlob != "" {
			// If ip itself was passed as a hostname, check it too
			if addrErr != nil && ip != "" {
				lowIP := strings.ToLower(ip)
				if GlobMatch(hp.domainGlob, lowIP) || (hp.domainApex != "" && lowIP == hp.domainApex) {
					return true
				}
			}
			for _, d := range domains {
				lowD := strings.ToLower(d)
				if GlobMatch(hp.domainGlob, lowD) || (hp.domainApex != "" && lowD == hp.domainApex) {
					return true
				}
			}
		}
	}
	return false
}

func matchHostIP(hp hostPattern, addr netip.Addr) bool {
	switch {
	case hp.isExactIP:
		return addr == hp.exactAddr
	case hp.isCIDR:
		return hp.prefix.Contains(addr)
	case hp.isRange:
		if addr.Is6() != hp.ipRange.isV6 {
			return false
		}
		b := addr.As16()
		return bytes.Compare(b[:], hp.ipRange.lo[:]) >= 0 && bytes.Compare(b[:], hp.ipRange.hi[:]) <= 0
	case hp.isV4Octets:
		if !addr.Is4() {
			return false
		}
		b := addr.As4()
		for i := 0; i < 4; i++ {
			r := hp.v4Octets[i]
			if !r.any && (b[i] < r.lo || b[i] > r.hi) {
				return false
			}
		}
		return true
	case hp.isV6Prefix:
		if !addr.Is6() {
			return false
		}
		b := addr.As16()
		for i, want := range hp.v6Groups {
			got := (uint16(b[i*2]) << 8) | uint16(b[i*2+1])
			if got != want {
				return false
			}
		}
		return true
	}
	return false
}

// GlobMatch performs wildcard matching with '*' (0 or more chars) and '?' (any single char).
func GlobMatch(pattern, text string) bool {
	pi, ti := 0, 0
	star, mark := -1, 0
	for ti < len(text) {
		if pi < len(pattern) && (pattern[pi] == text[ti] || pattern[pi] == '?') {
			pi++
			ti++
		} else if pi < len(pattern) && pattern[pi] == '*' {
			star = pi
			mark = ti
			pi++
		} else if star != -1 {
			pi = star + 1
			mark++
			ti = mark
		} else {
			return false
		}
	}
	for pi < len(pattern) && pattern[pi] == '*' {
		pi++
	}
	return pi == len(pattern)
}

func splitList(s string, allowNewline bool) []string {
	var out []string
	for _, raw := range strings.FieldsFunc(s, func(r rune) bool {
		if allowNewline && (r == '\n' || r == '\r') {
			return true
		}
		return r == ';' || r == ','
	}) {
		t := strings.TrimSpace(raw)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

// FlowInput describes an intercepted connection or datagram flow to evaluate.
type FlowInput struct {
	Process   FlowProcess
	DestIP    string
	Domains   []string
	DestPort  uint16
	Protocol  pbprofile.RuleProtocol // ProtoTCP or ProtoUDP
	CheckHost bool                   // false on macOS UDP where NEAppProxyUDPFlow has no initial remote IP/port
}

// Decision is the result of evaluating a FlowInput against the Engine's ordered rules.
type Decision struct {
	Matched       bool
	RuleID        string
	RuleName      string
	Action        pbprofile.RuleAction
	ProxyConfigID string
}

// Engine evaluates an ordered list of compiled rules (first match wins).
type Engine struct {
	matchers      []*Matcher
	hasDomainRule bool
}

func NewEngine(rules []pbprofile.ProxyRule) *Engine {
	e := &Engine{}
	for _, r := range rules {
		if !r.Enabled {
			continue
		}
		m := NewMatcher(r)
		if m.hasDomainRule {
			e.hasDomainRule = true
		}
		e.matchers = append(e.matchers, m)
	}
	return e
}

func (e *Engine) HasDomainRules() bool { return e.hasDomainRule }

func (e *Engine) Evaluate(in FlowInput) Decision {
	isUDP := in.Protocol == pbprofile.ProtoUDP
	var excludedBy *Matcher
	for _, m := range e.matchers {
		rp := m.Rule.Protocol
		if rp != "" && rp != pbprofile.ProtoBoth && rp != in.Protocol {
			continue
		}
		if in.CheckHost {
			if !m.MatchesPort(in.DestPort) {
				continue
			}
			if !m.MatchesHost(in.DestIP, in.Domains) {
				continue
			}
		}
		if !m.MatchesProcess(in.Process) {
			// An app excluded from a PROXY rule must stay DIRECT, not fall through to the default proxy.
			if m.excludeProcess && m.Rule.Action == pbprofile.ActionProxy && excludedBy == nil {
				excludedBy = m
			}
			continue
		}
		act := m.Rule.Action
		if act == pbprofile.ActionProxy && in.CheckHost {
			if ShouldBypassProxy(in.DestIP, in.DestPort, isUDP, m.Rule.LocalhostViaProxy) {
				act = pbprofile.ActionDirect
			}
		}
		return Decision{
			Matched:       true,
			RuleID:        m.Rule.ID,
			RuleName:      m.Rule.Name,
			Action:        act,
			ProxyConfigID: m.Rule.ProxyConfigID,
		}
	}
	if excludedBy != nil {
		return Decision{Matched: true, RuleID: excludedBy.Rule.ID, RuleName: excludedBy.Rule.Name, Action: pbprofile.ActionDirect}
	}
	return Decision{Matched: false, Action: pbprofile.ActionDirect}
}
