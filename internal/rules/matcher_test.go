// SPDX-License-Identifier: AGPL-3.0-or-later

package rules

import (
	"testing"

	"github.com/D0bby999/anyproxy/internal/pbprofile"
)

func makeMatcher(proc, hosts, ports string) *Matcher {
	return NewMatcher(pbprofile.ProxyRule{
		ID:           "r1",
		ProcessNames: proc,
		TargetHosts:  hosts,
		TargetPorts:  ports,
		Protocol:     pbprofile.ProtoBoth,
		Action:       pbprofile.ActionDirect,
		Enabled:      true,
	})
}

func TestMatcherUpstreamVectors(t *testing.T) {
	chrome := NewFlowProcess("com.google.Chrome", "Google Chrome")
	curl := NewFlowProcess("curl-abc", "curl")

	procCases := []struct {
		name string
		got  bool
		want bool
	}{
		{"star", makeMatcher("*", "*", "*").MatchesProcess(chrome), true},
		{"prefix glob must match start only", makeMatcher("chrome*", "*", "*").MatchesProcess(chrome), false},
		{"contains glob", makeMatcher("*chrome*", "*", "*").MatchesProcess(chrome), true},
		{"exact name", makeMatcher("curl", "*", "*").MatchesProcess(curl), true},
		{"list with spaces", makeMatcher("safari; curl", "*", "*").MatchesProcess(curl), true},
		{"comma list", makeMatcher("safari,curl", "*", "*").MatchesProcess(curl), true},
		{"trailing separator is not a wildcard", makeMatcher("safari;", "*", "*").MatchesProcess(curl), false},
		{"leading separator is not a wildcard", makeMatcher(";safari", "*", "*").MatchesProcess(curl), false},
		{"bundle id glob", makeMatcher("com.google.*", "*", "*").MatchesProcess(chrome), true},
		{"case insensitive", makeMatcher("CURL", "*", "*").MatchesProcess(curl), true},
		{"no match", makeMatcher("firefox", "*", "*").MatchesProcess(curl), false},
	}
	for _, tc := range procCases {
		if tc.got != tc.want {
			t.Errorf("process [%s]: got %v, want %v", tc.name, tc.got, tc.want)
		}
	}

	portCases := []struct {
		name string
		got  bool
		want bool
	}{
		{"port in list", makeMatcher("*", "*", "80,443,8000-8100").MatchesPort(8050), true},
		{"port exact", makeMatcher("*", "*", "80,443").MatchesPort(443), true},
		{"port miss", makeMatcher("*", "*", "80,443").MatchesPort(81), false},
		{"port range edge low", makeMatcher("*", "*", "8000-8100").MatchesPort(8000), true},
		{"port range edge high", makeMatcher("*", "*", "8000-8100").MatchesPort(8100), true},
		{"port range just out", makeMatcher("*", "*", "8000-8100").MatchesPort(8101), false},
		{"all ports", makeMatcher("*", "*", "*").MatchesPort(1), true},
		{"garbage port never matches", makeMatcher("*", "*", "abc").MatchesPort(1), false},
		{"trailing separator in ports is not a wildcard", makeMatcher("*", "*", "80;").MatchesPort(443), false},
		{"reversed range never matches", makeMatcher("*", "*", "9000-8000").MatchesPort(8500), false},
	}
	for _, tc := range portCases {
		if tc.got != tc.want {
			t.Errorf("port [%s]: got %v, want %v", tc.name, tc.got, tc.want)
		}
	}

	hostCases := []struct {
		name string
		got  bool
		want bool
	}{
		{"ip exact", makeMatcher("*", "10.0.0.5", "*").MatchesHost("10.0.0.5", nil), true},
		{"ip wildcard octet", makeMatcher("*", "192.168.1.*", "*").MatchesHost("192.168.1.9", nil), true},
		{"ip octet range", makeMatcher("*", "10.1-5.*.*", "*").MatchesHost("10.3.99.1", nil), true},
		{"ip cidr v4", makeMatcher("*", "10.0.0.0/8", "*").MatchesHost("10.250.1.2", nil), true},
		{"ip cidr v6", makeMatcher("*", "2001:db8::/32", "*").MatchesHost("2001:db8:abcd::1", nil), true},
		{"ip range", makeMatcher("*", "10.0.0.1-10.0.0.9", "*").MatchesHost("10.0.0.5", nil), true},
		{"ip miss", makeMatcher("*", "10.0.0.5", "*").MatchesHost("10.0.0.6", nil), false},
		{"ipv6 exact", makeMatcher("*", "::1", "*").MatchesHost("::1", nil), true},
		{"ipv6 prefix wildcard", makeMatcher("*", "2001:db8:*", "*").MatchesHost("2001:db8::5", nil), true},
		{"domain glob", makeMatcher("*", "*.github.com", "*").MatchesHost("1.1.1.1", []string{"api.github.com"}), true},
		{"domain glob covers apex", makeMatcher("*", "*.github.com", "*").MatchesHost("1.1.1.1", []string{"github.com"}), true},
		{"domain single char ?", makeMatcher("*", "api?.example.org", "*").MatchesHost("1.1.1.1", []string{"api2.example.org"}), true},
		{"domain miss", makeMatcher("*", "*.gitlab.com", "*").MatchesHost("1.1.1.1", []string{"api.github.com"}), false},
		{"domain exact", makeMatcher("*", "example.com", "*").MatchesHost("1.1.1.1", []string{"example.com"}), true},
		{"list of ip and domain", makeMatcher("*", "1.2.3.4;*.github.com", "*").MatchesHost("9.9.9.9", []string{"api.github.com"}), true},
		{"domain rule without any known name", makeMatcher("*", "*.github.com", "*").MatchesHost("9.9.9.9", nil), false},
		{"all hosts", makeMatcher("*", "*", "*").MatchesHost("9.9.9.9", nil), true},
	}
	for _, tc := range hostCases {
		if tc.got != tc.want {
			t.Errorf("host [%s]: got %v, want %v", tc.name, tc.got, tc.want)
		}
	}
}

func TestEngineMandatoryDirectAndLocalhostViaProxy(t *testing.T) {
	eng := NewEngine([]pbprofile.ProxyRule{
		{
			ID:                "r-curl",
			ProcessNames:      "curl",
			TargetHosts:       "*",
			TargetPorts:       "*",
			Protocol:          pbprofile.ProtoBoth,
			Action:            pbprofile.ActionProxy,
			ProxyConfigID:     "px1",
			Enabled:           true,
			LocalhostViaProxy: false,
		},
		{
			ID:                "r-chrome-lh",
			ProcessNames:      "chrome",
			TargetHosts:       "*",
			TargetPorts:       "*",
			Protocol:          pbprofile.ProtoTCP,
			Action:            pbprofile.ActionProxy,
			ProxyConfigID:     "px1",
			Enabled:           true,
			LocalhostViaProxy: true,
		},
	})

	curl := NewFlowProcess("", "curl")
	chrome := NewFlowProcess("", "chrome")

	// Normal internet IP -> PROXY
	d := eng.Evaluate(FlowInput{Process: curl, DestIP: "93.184.216.34", DestPort: 443, Protocol: pbprofile.ProtoTCP, CheckHost: true})
	if d.Action != pbprofile.ActionProxy {
		t.Fatalf("expected PROXY for internet IP, got %+v", d)
	}

	// Loopback with LocalhostViaProxy=false -> overridden to DIRECT
	d = eng.Evaluate(FlowInput{Process: curl, DestIP: "127.0.0.1", DestPort: 8080, Protocol: pbprofile.ProtoTCP, CheckHost: true})
	if d.Action != pbprofile.ActionDirect {
		t.Fatalf("expected DIRECT for loopback when LocalhostViaProxy=false, got %+v", d)
	}

	// Loopback with LocalhostViaProxy=true -> stays PROXY
	d = eng.Evaluate(FlowInput{Process: chrome, DestIP: "127.0.0.1", DestPort: 8080, Protocol: pbprofile.ProtoTCP, CheckHost: true})
	if d.Action != pbprofile.ActionProxy {
		t.Fatalf("expected PROXY for loopback when LocalhostViaProxy=true, got %+v", d)
	}

	// Loopback DNS port 53 -> always DIRECT even when LocalhostViaProxy=true
	d = eng.Evaluate(FlowInput{Process: chrome, DestIP: "127.0.0.53", DestPort: 53, Protocol: pbprofile.ProtoTCP, CheckHost: true})
	if d.Action != pbprofile.ActionDirect {
		t.Fatalf("expected DIRECT for loopback port 53, got %+v", d)
	}

	// Multicast, APIPA, Broadcast, DHCP -> always DIRECT
	for _, ip := range []string{"224.0.0.251", "255.255.255.255", "192.168.1.255", "169.254.10.20", "ff02::1", "fe80::1"} {
		d = eng.Evaluate(FlowInput{Process: chrome, DestIP: ip, DestPort: 443, Protocol: pbprofile.ProtoTCP, CheckHost: true})
		if d.Action != pbprofile.ActionDirect {
			t.Fatalf("expected DIRECT for %s, got %+v", ip, d)
		}
	}
}

func BenchmarkRuleEvaluate(b *testing.B) {
	eng := NewEngine([]pbprofile.ProxyRule{
		{ID: "r1", ProcessNames: "safari;firefox", TargetHosts: "10.0.0.0/8", TargetPorts: "80;443", Protocol: pbprofile.ProtoTCP, Action: pbprofile.ActionDirect, Enabled: true},
		{ID: "r2", ProcessNames: "*chrome*", TargetHosts: "192.168.*.*;*.github.com", TargetPorts: "80;443;8000-9000", Protocol: pbprofile.ProtoBoth, Action: pbprofile.ActionProxy, ProxyConfigID: "px1", Enabled: true},
	})
	in := FlowInput{
		Process:   NewFlowProcess("com.google.Chrome", "Google Chrome"),
		DestIP:    "140.82.121.6",
		Domains:   []string{"api.github.com"},
		DestPort:  443,
		Protocol:  pbprofile.ProtoTCP,
		CheckHost: true,
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = eng.Evaluate(in)
	}
}
